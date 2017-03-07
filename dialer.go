package libp2pquic

import (
	"context"

	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"
)

type dialer struct{}

func (d *dialer) Dial(raddr ma.Multiaddr) (tpt.Conn, error) {
	panic("not implemented")
}

func (d *dialer) DialContext(ctx context.Context, raddr ma.Multiaddr) (tpt.Conn, error) {
	panic("not implemented")
}

func (d *dialer) Matches(ma.Multiaddr) bool {
	panic("not implemented")
}

var _ tpt.Dialer = &dialer{}
