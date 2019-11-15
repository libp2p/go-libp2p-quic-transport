// +build !windows

package libp2pquic

import (
	"net"

	"github.com/vishvananda/netlink"
)

type reuse struct {
	reuseBase

	handle *netlink.Handle // Only set on Linux. nil on other systems.
}

func newReuse() (*reuse, error) {
	handle, err := netlink.NewHandle(SupportedNlFamilies...)
	if err == netlink.ErrNotImplemented {
		handle = nil
	} else if err != nil {
		return nil, err
	}
	return &reuse{
		reuseBase: newReuseBase(),
		handle:    handle,
	}, nil
}

// Get the source IP that the kernel would use for dialing.
// This only works on Linux.
// On other systems, this returns an empty slice of IP addresses.
func (r *reuse) getSourceIPs(network string, raddr *net.UDPAddr) ([]net.IP, error) {
	if r.handle == nil {
		return nil, nil
	}

	routes, err := r.handle.RouteGet(raddr.IP)
	if err != nil {
		return nil, err
	}

	ips := make([]net.IP, 0, len(routes))
	for _, route := range routes {
		ips = append(ips, route.Src)
	}
	return ips, nil
}

func (r *reuse) Dial(network string, raddr *net.UDPAddr) (*reuseConn, error) {
	ips, err := r.getSourceIPs(network, raddr)
	if err != nil {
		return nil, err
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	conn, err := r.dialLocked(network, raddr, ips)
	if err != nil {
		return nil, err
	}
	conn.IncreaseCount()
	r.maybeStartGarbageCollector()
	return conn, nil
}
