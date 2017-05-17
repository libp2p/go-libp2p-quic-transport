package libp2pquic

import (
	"net"

	tpt "github.com/libp2p/go-libp2p-transport"
	quic "github.com/lucas-clemente/quic-go"
	testdata "github.com/lucas-clemente/quic-go/testdata"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

type listener struct {
	laddr        ma.Multiaddr
	quicListener quic.Listener

	transport tpt.Transport
}

var _ tpt.Listener = &listener{}

func newListener(laddr ma.Multiaddr, t tpt.Transport) (*listener, error) {
	qconf := &quic.Config{
		// we need to provide a certificate here
		// use the demo certificate from quic-go
		TLSConfig: testdata.GetTLSConfig(),
	}

	_, host, err := manet.DialArgs(laddr)
	if err != nil {
		return nil, err
	}
	qln, err := quic.ListenAddr(host, qconf)
	if err != nil {
		return nil, err
	}

	return &listener{
		laddr:        laddr,
		quicListener: qln,
		transport:    t,
	}, nil
}

func (l *listener) Accept() (tpt.Conn, error) {
	sess, err := l.quicListener.Accept()
	if err != nil {
		return nil, err
	}
	return newQuicConn(sess, l.transport)
}

func (l *listener) Close() error {
	return l.quicListener.Close()
}

func (l *listener) Addr() net.Addr {
	return l.quicListener.Addr()
}

func (l *listener) Multiaddr() ma.Multiaddr {
	return l.laddr
}
