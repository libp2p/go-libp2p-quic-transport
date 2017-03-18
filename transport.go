package libp2pquic

import (
	"fmt"
	"sync"

	pstore "github.com/libp2p/go-libp2p-peerstore"
	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/whyrusleeping/mafmt"
)

// QuicTransport implements a QUIC Transport
type QuicTransport struct {
	mutex sync.Mutex

	peers pstore.Peerstore

	listeners map[string]tpt.Listener
}

// NewQuicTransport creates a new QUIC Transport
// it tracks dialers and listeners created
func NewQuicTransport(peers pstore.Peerstore) *QuicTransport {
	return &QuicTransport{
		peers:     peers,
		listeners: make(map[string]tpt.Listener),
	}
}

func (t *QuicTransport) Dialer(laddr ma.Multiaddr, opts ...tpt.DialOpt) (tpt.Dialer, error) {
	panic("not implemented")
}

// Listen starts listening on laddr
func (t *QuicTransport) Listen(laddr ma.Multiaddr) (tpt.Listener, error) {
	if !t.Matches(laddr) {
		return nil, fmt.Errorf("quic transport cannot listen on %q", laddr)
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()

	l, ok := t.listeners[laddr.String()]
	if ok {
		return l, nil
	}

	ln, err := newListener(laddr, t.peers, t)
	if err != nil {
		return nil, err
	}

	t.listeners[laddr.String()] = ln
	return ln, nil
}

func (t *QuicTransport) Matches(a ma.Multiaddr) bool {
	return mafmt.QUIC.Matches(a)
}

var _ tpt.Transport = &QuicTransport{}
