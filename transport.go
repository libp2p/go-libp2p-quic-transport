package libp2pquic

import (
	"context"
	"errors"

	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	tpt "github.com/libp2p/go-libp2p-transport"
	quic "github.com/lucas-clemente/quic-go"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/whyrusleeping/mafmt"
)

var quicDialAddr = quic.DialAddr

// The Transport implements the tpt.Transport interface for QUIC connections.
type transport struct {
	privKey   ic.PrivKey
	localPeer peer.ID
}

var _ tpt.Transport = &transport{}

// NewTransport creates a new QUIC transport
func NewTransport(key ic.PrivKey) (tpt.Transport, error) {
	localPeer, err := peer.IDFromPrivateKey(key)
	if err != nil {
		return nil, err
	}
	return &transport{
		privKey:   key,
		localPeer: localPeer,
	}, nil
}

// Dial dials a new QUIC connection
func (t *transport) Dial(ctx context.Context, raddr ma.Multiaddr, p peer.ID) (tpt.Conn, error) {
	_, host, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}
	tlsConf, err := GenerateConfig(t.privKey)
	if err != nil {
		return nil, err
	}
	sess, err := quicDialAddr(host, tlsConf, &quic.Config{Versions: []quic.VersionNumber{101}})
	if err != nil {
		return nil, err
	}
	remotePubKey, err := getRemotePubKey(sess)
	if err != nil {
		return nil, err
	}
	localMultiaddr, err := quicMultiaddr(sess.LocalAddr())
	if err != nil {
		return nil, err
	}
	if !p.MatchesPublicKey(remotePubKey) {
		err := errors.New("peer IDs don't match")
		sess.Close(err)
		return nil, err
	}
	return &conn{
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
	return newListener(addr, t, t.localPeer, t.privKey)
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
