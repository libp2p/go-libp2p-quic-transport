package libp2pquic

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"net"

	"golang.org/x/crypto/hkdf"

	logging "github.com/ipfs/go-log"
	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/pnet"
	tpt "github.com/libp2p/go-libp2p-core/transport"
	p2ptls "github.com/libp2p/go-libp2p-tls"
	filter "github.com/libp2p/go-maddr-filter"
	quic "github.com/lucas-clemente/quic-go"
	ma "github.com/multiformats/go-multiaddr"
	mafmt "github.com/multiformats/go-multiaddr-fmt"
	manet "github.com/multiformats/go-multiaddr-net"
)

var log = logging.Logger("quic-transport")

var quicConfig = &quic.Config{
	MaxIncomingStreams:                    1000,
	MaxIncomingUniStreams:                 -1,             // disable unidirectional streams
	MaxReceiveStreamFlowControlWindow:     10 * (1 << 20), // 10 MB
	MaxReceiveConnectionFlowControlWindow: 15 * (1 << 20), // 15 MB
	AcceptToken: func(clientAddr net.Addr, _ *quic.Token) bool {
		// TODO(#6): require source address validation when under load
		return true
	},
	KeepAlive: true,
}

type connManager struct {
	reuseUDP4 *reuse
	reuseUDP6 *reuse
}

func newConnManager(filters *filter.Filters) (*connManager, error) {
	reuseUDP4, err := newReuse(filters)
	if err != nil {
		return nil, err
	}
	reuseUDP6, err := newReuse(filters)
	if err != nil {
		return nil, err
	}
	return &connManager{
		reuseUDP4: reuseUDP4,
		reuseUDP6: reuseUDP6,
	}, nil
}

func (c *connManager) getReuse(network string) (*reuse, error) {
	switch network {
	case "udp4":
		return c.reuseUDP4, nil
	case "udp6":
		return c.reuseUDP6, nil
	default:
		return nil, errors.New("invalid network: must be either udp4 or udp6")
	}
}

func (c *connManager) Listen(network string, laddr *net.UDPAddr) (*reuseConn, error) {
	reuse, err := c.getReuse(network)
	if err != nil {
		return nil, err
	}
	return reuse.Listen(network, laddr)
}

func (c *connManager) Dial(network string, raddr *net.UDPAddr) (*reuseConn, error) {
	reuse, err := c.getReuse(network)
	if err != nil {
		return nil, err
	}
	return reuse.Dial(network, raddr)
}

// The Transport implements the tpt.Transport interface for QUIC connections.
type transport struct {
	privKey     ic.PrivKey
	localPeer   peer.ID
	identity    *p2ptls.Identity
	connManager *connManager
	config      *quic.Config
}

var _ tpt.Transport = &transport{}

// NewTransport creates a new QUIC transport
func NewTransport(key ic.PrivKey, psk pnet.PSK, filters *filter.Filters) (tpt.Transport, error) {
	if len(psk) > 0 {
		log.Error("QUIC doesn't support private networks yet.")
		return nil, errors.New("QUIC doesn't support private networks yet")
	}
	localPeer, err := peer.IDFromPrivateKey(key)
	if err != nil {
		return nil, err
	}
	identity, err := p2ptls.NewIdentity(key)
	if err != nil {
		return nil, err
	}
	connManager, err := newConnManager(filters)
	if err != nil {
		return nil, err
	}
	config := quicConfig.Clone()
	keyBytes, err := key.Raw()
	if err != nil {
		return nil, err
	}
	keyReader := hkdf.New(sha256.New, keyBytes, nil, []byte("libp2p quic stateless reset key"))
	config.StatelessResetKey = make([]byte, 32)
	if _, err := io.ReadFull(keyReader, config.StatelessResetKey); err != nil {
		return nil, err
	}

	return &transport{
		privKey:     key,
		localPeer:   localPeer,
		identity:    identity,
		connManager: connManager,
		config:      config,
	}, nil
}

// Dial dials a new QUIC connection
func (t *transport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (tpt.CapableConn, error) {
	network, host, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}
	addr, err := net.ResolveUDPAddr(network, host)
	if err != nil {
		return nil, err
	}
	remoteMultiaddr, err := toQuicMultiaddr(addr)
	if err != nil {
		return nil, err
	}
	tlsConf, keyCh := t.identity.ConfigForPeer(p)
	pconn, err := t.connManager.Dial(network, addr)
	if err != nil {
		return nil, err
	}
	sess, err := quic.DialContext(ctx, pconn, addr, host, tlsConf, t.config)
	if err != nil {
		pconn.DecreaseCount()
		return nil, err
	}
	// Should be ready by this point, don't block.
	var remotePubKey ic.PubKey
	select {
	case remotePubKey = <-keyCh:
	default:
	}
	if remotePubKey == nil {
		pconn.DecreaseCount()
		return nil, errors.New("go-libp2p-quic-transport BUG: expected remote pub key to be set")
	}
	go func() {
		<-sess.Context().Done()
		pconn.DecreaseCount()
	}()

	localMultiaddr, err := toQuicMultiaddr(pconn.LocalAddr())
	if err != nil {
		pconn.DecreaseCount()
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
		remoteMultiaddr: remoteMultiaddr,
	}, nil
}

// CanDial determines if we can dial to an address
func (t *transport) CanDial(addr ma.Multiaddr) bool {
	return mafmt.QUIC.Matches(addr)
}

// Listen listens for new QUIC connections on the passed multiaddr.
func (t *transport) Listen(addr ma.Multiaddr) (tpt.Listener, error) {
	lnet, host, err := manet.DialArgs(addr)
	if err != nil {
		return nil, err
	}
	laddr, err := net.ResolveUDPAddr(lnet, host)
	if err != nil {
		return nil, err
	}
	conn, err := t.connManager.Listen(lnet, laddr)
	if err != nil {
		return nil, err
	}
	return newListener(conn, t, t.localPeer, t.privKey, t.identity)
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
