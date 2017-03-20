package libp2pquic

import (
	"context"
	"crypto/tls"

	tpt "github.com/libp2p/go-libp2p-transport"
	quicconn "github.com/marten-seemann/quic-conn"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
	"github.com/whyrusleeping/mafmt"
)

type dialer struct {
	transport tpt.Transport
}

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
	c, err := quicconn.Dial(host, tlsConf)
	if err != nil {
		return nil, err
	}

	mnc, err := manet.WrapNetConn(c)
	if err != nil {
		return nil, err
	}

	return &tpt.ConnWrap{
		Conn: mnc,
		Tpt:  d.transport,
	}, nil
}

func (d *dialer) DialContext(ctx context.Context, raddr ma.Multiaddr) (tpt.Conn, error) {
	return d.Dial(raddr)
}

func (d *dialer) Matches(a ma.Multiaddr) bool {
	return mafmt.QUIC.Matches(a)
}

var _ tpt.Dialer = &dialer{}
