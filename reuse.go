package libp2pquic

import (
	"net"
	"sync"
	"sync/atomic"

	"github.com/vishvananda/netlink"
)

type reuseConn struct {
	net.PacketConn
	refCount int32 // to be used as an atomic
}

func newReuseConn(conn net.PacketConn) *reuseConn {
	return &reuseConn{PacketConn: conn}
}

func (c *reuseConn) IncreaseCount() { atomic.AddInt32(&c.refCount, 1) }
func (c *reuseConn) DecreaseCount() { atomic.AddInt32(&c.refCount, -1) }
func (c *reuseConn) GetCount() int  { return int(atomic.LoadInt32(&c.refCount)) }

type reuse struct {
	mutex sync.Mutex

	unicast map[string] /* IP.String() */ map[int] /* port */ *reuseConn
	// global contains connections that are listening on 0.0.0.0 / ::
	global map[int]*reuseConn
}

func newReuse() *reuse {
	return &reuse{
		unicast: make(map[string]map[int]*reuseConn),
		global:  make(map[int]*reuseConn),
	}
}

func (r *reuse) getSourceIPs(network string, raddr *net.UDPAddr) ([]net.IP, error) {
	// Determine the source address that the kernel would use for this IP address.
	// Note: This only works on Linux.
	// On other OSes, this will return a netlink.ErrNotImplemetned.
	routes, err := (&netlink.Handle{}).RouteGet(raddr.IP)
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
	if err != nil && err != netlink.ErrNotImplemented {
		return nil, err
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	conn, err := r.dialLocked(network, raddr, ips)
	if err != nil {
		return nil, err
	}
	conn.IncreaseCount()
	return conn, nil
}

func (r *reuse) dialLocked(network string, raddr *net.UDPAddr, ips []net.IP) (*reuseConn, error) {
	for _, ip := range ips {
		// We already have at least one suitable connection...
		if conns, ok := r.unicast[ip.String()]; ok {
			// ... we don't care which port we're dialing from. Just use the first.
			for _, c := range conns {
				return c, nil
			}
		}
	}

	// Use a connection listening on 0.0.0.0 (or ::).
	// Again, we don't care about the port number.
	for _, conn := range r.global {
		return conn, nil
	}

	// We don't have a connection that we can use for dialing.
	// Dial a new connection from a random port.
	var addr *net.UDPAddr
	switch network {
	case "udp4":
		addr = &net.UDPAddr{IP: net.IPv4zero, Port: 0}
	case "udp6":
		addr = &net.UDPAddr{IP: net.IPv6zero, Port: 0}
	}
	conn, err := net.ListenUDP(network, addr)
	if err != nil {
		return nil, err
	}
	rconn := newReuseConn(conn)
	r.global[conn.LocalAddr().(*net.UDPAddr).Port] = rconn
	return rconn, nil
}

func (r *reuse) Listen(network string, laddr *net.UDPAddr) (*reuseConn, error) {
	conn, err := net.ListenUDP(network, laddr)
	if err != nil {
		return nil, err
	}
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	rconn := newReuseConn(conn)
	rconn.IncreaseCount()

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Deal with listen on a global address
	if laddr.IP.IsUnspecified() {
		// The kernel already checked that the laddr is not already listen
		// so we need not check here (when we create ListenUDP).
		r.global[laddr.Port] = rconn
		return rconn, err
	}

	// Deal with listen on a unicast address
	if _, ok := r.unicast[localAddr.IP.String()]; !ok {
		r.unicast[laddr.IP.String()] = make(map[int]*reuseConn)
	}

	// The kernel already checked that the laddr is not already listen
	// so we need not check here (when we create ListenUDP).
	r.unicast[laddr.IP.String()][localAddr.Port] = rconn
	return rconn, err
}
