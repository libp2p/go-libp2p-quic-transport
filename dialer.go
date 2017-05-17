package libp2pquic

import (
	"context"
	"crypto/tls"

	tpt "github.com/libp2p/go-libp2p-transport"
	quic "github.com/lucas-clemente/quic-go"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/whyrusleeping/mafmt"
)

type dialer struct {
	transport tpt.Transport
}

var _ tpt.Dialer = &dialer{}

func newDialer(transport tpt.Transport) (*dialer, error) {
	return &dialer{
		transport: transport,
	}, nil
}

func (d *dialer) Dial(raddr ma.Multiaddr) (tpt.Conn, error) {
	// TODO: check that raddr is a QUIC address
	tlsConf := &tls.Config{InsecureSkipVerify: true}
	_, host, err := manet.DialArgs(raddr)
	if err != nil {
		return nil, err
	}

	qsess, err := quic.DialAddr(host, &quic.Config{TLSConfig: tlsConf})
	if err != nil {
		return nil, err
	}

	return newQuicConn(qsess, d.transport)
}

func (d *dialer) DialContext(ctx context.Context, raddr ma.Multiaddr) (tpt.Conn, error) {
	// TODO: implement the ctx
	return d.Dial(raddr)
}

func (d *dialer) Matches(a ma.Multiaddr) bool {
	return mafmt.QUIC.Matches(a)
}
