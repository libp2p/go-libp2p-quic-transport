package libp2pquic

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	n "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	tpt "github.com/libp2p/go-libp2p-core/transport"

	p2ptls "github.com/libp2p/go-libp2p-tls"

	quic "github.com/lucas-clemente/quic-go"
	ma "github.com/multiformats/go-multiaddr"
)

// A listener listens for QUIC connections.
type listener struct {
	quicListener   quic.Listener
	conn           *reuseConn
	transport      *transport
	privKey        ic.PrivKey
	localPeer      peer.ID
	localMultiaddr ma.Multiaddr
}

var _ tpt.Listener = &listener{}

func newListener(rconn *reuseConn, t *transport, localPeer peer.ID, key ic.PrivKey, identity *p2ptls.Identity) (tpt.Listener, error) {
	var tlsConf tls.Config
	tlsConf.GetConfigForClient = func(_ *tls.ClientHelloInfo) (*tls.Config, error) {
		// return a tls.Config that verifies the peer's certificate chain.
		// Note that since we have no way of associating an incoming QUIC connection with
		// the peer ID calculated here, we don't actually receive the peer's public key
		// from the key chan.
		conf, _ := identity.ConfigForAny()
		return conf, nil
	}
	ln, err := quic.Listen(rconn, &tlsConf, t.serverConfig)
	if err != nil {
		return nil, err
	}
	localMultiaddr, err := toQuicMultiaddr(ln.Addr())
	if err != nil {
		return nil, err
	}
	return &listener{
		conn:           rconn,
		quicListener:   ln,
		transport:      t,
		privKey:        key,
		localPeer:      localPeer,
		localMultiaddr: localMultiaddr,
	}, nil
}

// Accept accepts new connections.
func (l *listener) Accept() (tpt.CapableConn, error) {
	for {
		sess, err := l.quicListener.Accept(context.Background())
		if err != nil {
			return nil, err
		}
		conn, err := l.setupConn(sess)
		if err != nil {
			sess.CloseWithError(0, err.Error())
			continue
		}
		return conn, nil
	}
}

func (l *listener) setupConn(sess quic.Session) (tpt.CapableConn, error) {
	// The tls.Config used to establish this connection already verified the certificate chain.
	// Since we don't have any way of knowing which tls.Config was used though,
	// we have to re-determine the peer's identity here.
	// Therefore, this is expected to never fail.
	remotePubKey, err := p2ptls.PubKeyFromCertChain(sess.ConnectionState().PeerCertificates)
	if err != nil {
		return nil, err
	}

	remotePeerID, err := peer.IDFromPublicKey(remotePubKey)
	if err != nil {
		return nil, err
	}
	remoteMultiaddr, err := toQuicMultiaddr(sess.RemoteAddr())
	if err != nil {
		return nil, err
	}

	connaddrs := &connAddrs{lmAddr: l.localMultiaddr, rmAddr: remoteMultiaddr}
	if l.transport.gater != nil && !l.transport.gater.InterceptSecured(n.DirInbound, remotePeerID, connaddrs) {
		return nil, fmt.Errorf("secured connection gated")
	}

	return &conn{
		sess:            sess,
		transport:       l.transport,
		localPeer:       l.localPeer,
		localMultiaddr:  l.localMultiaddr,
		privKey:         l.privKey,
		remoteMultiaddr: remoteMultiaddr,
		remotePeerID:    remotePeerID,
		remotePubKey:    remotePubKey,
	}, nil
}

// Close closes the listener.
func (l *listener) Close() error {
	defer l.conn.DecreaseCount()
	return l.quicListener.Close()
}

// Addr returns the address of this listener.
func (l *listener) Addr() net.Addr {
	return l.quicListener.Addr()
}

// Multiaddr returns the multiaddress of this listener.
func (l *listener) Multiaddr() ma.Multiaddr {
	return l.localMultiaddr
}
