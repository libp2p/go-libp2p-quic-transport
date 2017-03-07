package libp2pquic

import (
	"net"

	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"
)

type listener struct{}

func (l *listener) Accept() (tpt.Conn, error) {
	panic("not implemented")
}

func (l *listener) Close() error {
	panic("not implemented")
}

func (l *listener) Addr() net.Addr {
	panic("not implemented")
}

func (l *listener) Multiaddr() ma.Multiaddr {
	panic("not implemented")
}

var _ tpt.Listener = &listener{}
