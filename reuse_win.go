// +build windows

package libp2pquic

import (
	"net"

	filter "github.com/libp2p/go-maddr-filter"
)

type reuse struct {
	reuseBase
}

func newReuse(filters *filter.Filters) (*reuse, error) {
	return &reuse{reuseBase: newReuseBase(filters)}, nil
}

func (r *reuse) Dial(network string, raddr *net.UDPAddr) (*reuseConn, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	conn, err := r.dialLocked(network, raddr, nil)
	if err != nil {
		return nil, err
	}
	conn.IncreaseCount()
	r.maybeStartGarbageCollector()
	return conn, nil
}
