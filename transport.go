package libp2pquic

import (
	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"
)

type QuicTransport struct{}

func NewQuicTransport() *QuicTransport {
	return &QuicTransport{}
}

func (t *QuicTransport) Dialer(laddr ma.Multiaddr, opts ...tpt.DialOpt) (tpt.Dialer, error) {
	panic("not implemented")
}

func (t *QuicTransport) Listen(laddr ma.Multiaddr) (tpt.Listener, error) {
	panic("not implemented")
}

func (t *QuicTransport) Matches(ma.Multiaddr) bool {
	panic("not implemented")
}

var _ tpt.Transport = &QuicTransport{}
