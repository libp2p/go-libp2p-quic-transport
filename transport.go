package libp2pquic

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"

	logging "github.com/ipfs/go-log"
	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	tpt "github.com/libp2p/go-libp2p-transport"
	quic "github.com/lucas-clemente/quic-go"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/whyrusleeping/mafmt"
)

var log = logging.Logger("libp2pquic")

// Types of IP addresses. Used as a key to the map.
const (
	AddrTypeGlobalUnicast uint8 = iota
	AddrTypeInterfaceLocalMulticast
	AddrTypeLinkLocalMulticast
	AddrTypeLinkLocalUnicast
	AddrTypeLoopback
	AddrTypeMulticast
	AddrTypeUnspecified
	AddrTypeInvalid
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

type connManager struct {
	// conn keeps track of all "PacketConn" created.
	// maps string(Network_Type:Addr_Type) -> PacketConn
	// Network_Type could be "udp4" or "udp6"
	// Addr_Type could be one of the const from AddrType* depending on the type of IP address.
	conn      map[string][]net.PacketConn
	connMutex *sync.Mutex

	// interfaceAddrs maps string(Network_Type:Addr_Type) -> []string(IP Addresses)
	// Its contains all the IP addresses associated with an interface
	// Eg: 127.0.0.1/8 is stored as ["udp4:4"]->"127.0.0.1"
	// Eg: 192.168.0.102 as			["udp4:0"]->"192.168.0.102"
	// Used when dialing and an appropriate socket isn't available/open.
	interfaceAddrs map[string][]string
}

func resolveNetworkAndAddrType(addr *net.IPAddr) (string, error) {
	addrType := AddrTypeInvalid
	if addr.IP.IsGlobalUnicast() {
		addrType = AddrTypeGlobalUnicast
	} else if addr.IP.IsInterfaceLocalMulticast() {
		addrType = AddrTypeInterfaceLocalMulticast
	} else if addr.IP.IsLinkLocalMulticast() {
		addrType = AddrTypeLinkLocalMulticast
	} else if addr.IP.IsLinkLocalUnicast() {
		addrType = AddrTypeLinkLocalUnicast
	} else if addr.IP.IsLoopback() {
		addrType = AddrTypeLoopback
	} else if addr.IP.IsMulticast() {
		addrType = AddrTypeMulticast
	} else if addr.IP.IsUnspecified() || addr.IP == nil {
		addrType = AddrTypeUnspecified
	}

	if addrType == AddrTypeInvalid {
		return "", fmt.Errorf("could not determine IP address type (eg: loopback, unicast, multicast etc)")
	} else {
		// addr.Network() returns "udp" (not "udp4" or "udp6")
		if addr.IP.To4() != nil {
			return "udp4:" + strconv.Itoa(int(addrType)), nil
		} else {
			return "udp6:" + strconv.Itoa(int(addrType)), nil
		}

	}

}

// return conn which accepts connection on all interface
func (c *connManager) connForAllInterface(network string) net.PacketConn {
	key := network + ":" + strconv.Itoa(int(AddrTypeUnspecified))
	if len(c.conn[key]) != 0 {
		return c.conn[key][0]
	} else {
		return nil
	}
}

func (c *connManager) GetConnForAddr(network, host string) (net.PacketConn, error) {
	udpAddr, err := net.ResolveUDPAddr(network, host)
	if err != nil {
		return nil, err
	}

	// check if we already have a "packetConn" corresponding to "0.0.0.0" or "::"
	// If yes, no need to check for any other "packetConn"
	upc := c.connForAllInterface(network)
	if upc != nil {
		return upc, nil
	}

	netAddrType, err := resolveNetworkAndAddrType(&net.IPAddr{IP: udpAddr.IP, Zone: udpAddr.Zone})
	if err != nil {
		return nil, err
	}

	availablePacketConnList := c.conn[netAddrType]

	// no "packetConn" available for this type of IP address.
	// create a new "packetConn" and store it for future use.
	if len(availablePacketConnList) == 0 {
		pc, err := c.createConn(network, host)
		if err != nil {
			return nil, err
		}
		c.connMutex.Lock()
		c.conn[netAddrType] = append(c.conn[netAddrType], pc)
		c.connMutex.Unlock()
		log.Debug("Creating a new packetConn with local addr:", pc.LocalAddr())
		return pc, nil
	}

	// This network has at least one PacketConn.
	// Return the first one.
	log.Debug("Found an existing packetConn with local addr: ", availablePacketConnList[0].LocalAddr())
	return availablePacketConnList[0], nil
}

func (c *connManager) createConn(network, host string) (net.PacketConn, error) {
	addr, err := net.ResolveUDPAddr(network, host)
	if err != nil {
		return nil, err
	}
	return net.ListenUDP(network, addr)
}

func (c *connManager) queryAllInterfaceAddrs() {
	ias, err := net.InterfaceAddrs()
	if err != nil {
		return // no error. If we could not query the interfaces we will use "0.0.0.0" or "::" while dialling.
	}

	for _, ia := range ias {
		ipAddr := &net.IPAddr{
			IP: net.ParseIP(strings.Split(ia.String(), "/")[0]),
		}
		if err != nil {
			continue // no error
		}
		ty, err := resolveNetworkAndAddrType(ipAddr)
		if err != nil {
			return // no error
		}
		c.interfaceAddrs[ty] = append(c.interfaceAddrs[ty], ipAddr.String())
	}
}

// getLocalInterfaceToDialOn will return the IP address associated
// to an interface corresponding to an IP address type.
// Eg: If we dial to "127.0.0.2", it returns "127.0.0.1"
// Eg: If we dial to 192.168.0.104", it returns "192.168.0.102"
// This is useful if we are dialling to "192.168.0.104" [non localhost] but listening on "127.0.0.1" [localhost]
// In this case we have to create a new "packetConn" [socket] using "192.168.0.102"
func (c *connManager) getLocalInterfaceToDialOn(network, host string) string {
	host = strings.Split(host, ":")[0]
	ty, err := resolveNetworkAndAddrType(&net.IPAddr{IP: net.ParseIP(host)})
	var list []string
	if err == nil {
		list = c.interfaceAddrs[ty]
	}

	// len(list) == 0 would mean an error is returned by "resolveNetworkAndAddrType"
	// So could not determine the best interface to dial from.
	// Return the "all interfaces" IP.
	if len(list) == 0 {
		switch network {
		case "udp4":
			return "0.0.0.0"
		case "udp6":
			return "::"
		}
	}
	return list[0]
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

	cm := &connManager{
		conn:           make(map[string][]net.PacketConn),
		connMutex:      &sync.Mutex{},
		interfaceAddrs: make(map[string][]string),
	}

	// query and fill "interfaceAddrs" map->list with all the available interface IPs.
	// This will be used while dialling to pick the best interface if one is not already available.
	cm.queryAllInterfaceAddrs()

	return &transport{
		privKey:     key,
		localPeer:   localPeer,
		tlsConf:     tlsConf,
		connManager: cm,
	}, nil
}

// Dial dials a new QUIC connection
func (t *transport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (tpt.Conn, error) {
	network, host, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	// get the IP address of the interface which could dial to "raddr".
	intf := t.connManager.getLocalInterfaceToDialOn(network, host)
	// check if we have "packetConn" corresponding to this interface. If yes, dial using it. If not, create a new one.
	pconn, err := t.connManager.GetConnForAddr(network, intf+":")
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
