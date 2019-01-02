package libp2pquic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"sync"

	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	tpt "github.com/libp2p/go-libp2p-transport"
	quic "github.com/lucas-clemente/quic-go"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/whyrusleeping/mafmt"
)

var quicConfig = &quic.Config{
	Versions:                              []quic.VersionNumber{quic.VersionMilestone0_10_0},
	MaxIncomingStreams:                    1000,
	MaxIncomingUniStreams:                 -1,              // disable unidirectional streams
	MaxReceiveStreamFlowControlWindow:     3 * (1 << 20),   // 3 MB
	MaxReceiveConnectionFlowControlWindow: 4.5 * (1 << 20), // 4.5 MB
	AcceptCookie: func(clientAddr net.Addr, cookie *quic.Cookie) bool {
		// TODO(#6): require source address validation when under load
		return true
	},
	KeepAlive: true,
}

type addrType int

const (
	addrTypeOther addrType = iota
	addrTypeLoopback
	addrTypeGlobal
	addrTypeUnspecified
)

type connManager struct {
	mu              sync.Mutex
	defaultIpv4Conn net.PacketConn
	defaultIpv6Conn net.PacketConn
	// map underhood PacketConn -> connection remote address type(usage of the conn)
	ipv4Conns map[net.PacketConn]addrType
	ipv6Conns map[net.PacketConn]addrType
}

func newConnManager() *connManager {
	return &connManager{
		ipv4Conns: make(map[net.PacketConn]addrType),
		ipv6Conns: make(map[net.PacketConn]addrType),
	}
}

func typeOfIP(ipAddr net.IP) addrType {
	if ipAddr.IsLoopback() {
		return addrTypeLoopback
	}
	if ipAddr.IsUnspecified() {
		return addrTypeUnspecified
	}
	if ipAddr.IsGlobalUnicast() {
		return addrTypeGlobal
	}
	return addrTypeOther
}

// GetConnForAddr try to reuse exist connections when possible
func (c *connManager) GetConnForAddr(network, remoteHost string) (net.PacketConn, error) {
	remoteAddr, err := net.ResolveUDPAddr(network, remoteHost)
	if err != nil {
		return nil, err
	}

	var listenHost string
	var conns map[net.PacketConn]addrType
	var defaultConn func() net.PacketConn
	var setDefaultConn func(conn net.PacketConn)
	switch network {
	case "udp4":
		listenHost = "0.0.0.0:0"
		conns = c.ipv4Conns
		defaultConn = func() net.PacketConn {
			return c.defaultIpv4Conn
		}
		setDefaultConn = func(conn net.PacketConn) {
			c.defaultIpv4Conn = conn
		}
	case "udp6":
		listenHost = ":0"
		conns = c.ipv6Conns
		defaultConn = func() net.PacketConn {
			return c.defaultIpv6Conn
		}
		setDefaultConn = func(conn net.PacketConn) {
			c.defaultIpv6Conn = conn
		}
	default:
		return nil, fmt.Errorf("unsupported network: %s", network)
	}

	// check if there exists a connection of expected type
	var pickedConn net.PacketConn
	c.mu.Lock()
	remoteAddrType := typeOfIP(remoteAddr.IP)
	for conn, connAddrType := range conns {
		if defaultConn() == nil && connAddrType == addrTypeUnspecified {
			setDefaultConn(conn)
		}
		if connAddrType == remoteAddrType {
			pickedConn = conn
			break
		}
	}
	if pickedConn == nil {
		pickedConn = defaultConn()
	}
	c.mu.Unlock()
	if pickedConn != nil {
		return pickedConn, nil
	}
	// could not reuse an exist connection, create a new one
	conn, err := c.createConn(network, listenHost)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	conns[conn] = remoteAddrType
	c.mu.Unlock()
	return conn, nil
}

func (c *connManager) createConn(network, host string) (net.PacketConn, error) {
	addr, err := net.ResolveUDPAddr(network, host)
	if err != nil {
		return nil, err
	}
	return net.ListenUDP(network, addr)
}

func (c *connManager) listenUDP(addr ma.Multiaddr) (net.PacketConn, error) {
	network, host, err := manet.DialArgs(addr)
	if err != nil {
		return nil, err
	}
	conn, err := c.createConn(network, host)
	if err != nil {
		return conn, err
	}
	c.mu.Lock()
	switch network {
	case "udp4":
		c.ipv4Conns[conn] = addrTypeUnspecified
	case "udp6":
		c.ipv6Conns[conn] = addrTypeUnspecified
	}
	c.mu.Unlock()
	return conn, nil
}

// The Transport implements the tpt.Transport interface for QUIC connections.
type transport struct {
	privKey     ic.PrivKey
	localPeer   peer.ID
	tlsConf     *tls.Config
	connManager *connManager
}

var _ tpt.Transport = &transport{}

// NewTransport creates a new QUIC transport
func NewTransport(key ic.PrivKey) (tpt.Transport, error) {
	localPeer, err := peer.IDFromPrivateKey(key)
	if err != nil {
		return nil, err
	}
	tlsConf, err := generateConfig(key)
	if err != nil {
		return nil, err
	}

	return &transport{
		privKey:     key,
		localPeer:   localPeer,
		tlsConf:     tlsConf,
		connManager: newConnManager(),
	}, nil
}

// Dial dials a new QUIC connection
func (t *transport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (tpt.Conn, error) {
	network, host, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}
	pconn, err := t.connManager.GetConnForAddr(network, host)
	if err != nil {
		return nil, err
	}
	addr, err := fromQuicMultiaddr(raddr)
	if err != nil {
		return nil, err
	}
	var remotePubKey ic.PubKey
	tlsConf := t.tlsConf.Clone()
	// We need to check the peer ID in the VerifyPeerCertificate callback.
	// The tls.Config it is also used for listening, and we might also have concurrent dials.
	// Clone it so we can check for the specific peer ID we're dialing here.
	tlsConf.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		chain := make([]*x509.Certificate, len(rawCerts))
		for i := 0; i < len(rawCerts); i++ {
			cert, err := x509.ParseCertificate(rawCerts[i])
			if err != nil {
				return err
			}
			chain[i] = cert
		}
		var err error
		remotePubKey, err = getRemotePubKey(chain)
		if err != nil {
			return err
		}
		if !p.MatchesPublicKey(remotePubKey) {
			return errors.New("peer IDs don't match")
		}
		return nil
	}
	sess, err := quic.DialContext(ctx, pconn, addr, host, tlsConf, quicConfig)
	if err != nil {
		return nil, err
	}
	localMultiaddr, err := toQuicMultiaddr(sess.LocalAddr())
	if err != nil {
		return nil, err
	}
	return &conn{
		sess:            sess,
		transport:       t,
		privKey:         t.privKey,
		localPeer:       t.localPeer,
		localMultiaddr:  localMultiaddr,
		remotePubKey:    remotePubKey,
		remotePeerID:    p,
		remoteMultiaddr: raddr,
	}, nil
}

// CanDial determines if we can dial to an address
func (t *transport) CanDial(addr ma.Multiaddr) bool {
	return mafmt.QUIC.Matches(addr)
}

// Listen listens for new QUIC connections on the passed multiaddr.
func (t *transport) Listen(addr ma.Multiaddr) (tpt.Listener, error) {
	return newListener(addr, t, t.localPeer, t.privKey, t.tlsConf)
}

// Proxy returns true if this transport proxies.
func (t *transport) Proxy() bool {
	return false
}

// Protocols returns the set of protocols handled by this transport.
func (t *transport) Protocols() []int {
	return []int{ma.P_QUIC}
}

func (t *transport) String() string {
	return "QUIC"
}
