package libp2pquic

import (
	"fmt"
	"sync"

	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/whyrusleeping/mafmt"
)

// QuicTransport implements a QUIC Transport
type QuicTransport struct {
	lmutex    sync.Mutex
	listeners map[string]tpt.Listener

	dmutex  sync.Mutex
	dialers map[string]tpt.Dialer
}

var _ tpt.Transport = &QuicTransport{}

// NewQuicTransport creates a new QUIC Transport
// it tracks dialers and listeners created
func NewQuicTransport() *QuicTransport {
	// utils.SetLogLevel(utils.LogLevelDebug)
	return &QuicTransport{
		listeners: make(map[string]tpt.Listener),
		dialers:   make(map[string]tpt.Dialer),
	}
}

func (t *QuicTransport) Dialer(laddr ma.Multiaddr, opts ...tpt.DialOpt) (tpt.Dialer, error) {
	if !t.Matches(laddr) {
		return nil, fmt.Errorf("quic transport cannot dial %q", laddr)
	}

	t.dmutex.Lock()
	defer t.dmutex.Unlock()

	s := laddr.String()
	d, ok := t.dialers[s]
	if ok {
		return d, nil
	}

	// TODO: read opts
	quicd, err := newDialer(t)
	if err != nil {
		return nil, err
	}
	t.dialers[s] = quicd
	return quicd, nil
}

// Listen starts listening on laddr
func (t *QuicTransport) Listen(laddr ma.Multiaddr) (tpt.Listener, error) {
	if !t.Matches(laddr) {
		return nil, fmt.Errorf("quic transport cannot listen on %q", laddr)
	}

	t.lmutex.Lock()
	defer t.lmutex.Unlock()

	l, ok := t.listeners[laddr.String()]
	if ok {
		return l, nil
	}

	ln, err := newListener(laddr, t)
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
