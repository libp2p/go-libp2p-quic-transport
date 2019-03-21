package libp2pquic

import (
	"errors"
	"net"
	"sync"

	srcs "github.com/lnykww/go-src-select"
)

type ReuseConn struct {
	net.PacketConn
	lock sync.Mutex
	ref  int
}

func NewReuseConn(conn net.PacketConn) *ReuseConn {
	return &ReuseConn{
		PacketConn: conn,
		ref:        1,
		lock:       sync.Mutex{},
	}
}

func (rc *ReuseConn) Ref() error {
	rc.lock.Lock()
	defer rc.lock.Unlock()
	if rc.ref == 0 {
		return errors.New("conn closed")
	}
	rc.ref++
	return nil
}

func (rc *ReuseConn) Close() error {
	rc.lock.Lock()
	defer rc.lock.Unlock()
	var err error
	switch rc.ref {
	case 0: // cloesd, just return
		return nil
	case 1: // no reference, close the conn
		err = rc.PacketConn.Close()
	}
	rc.ref--
	return err
}

type Reuse struct {
	lock           sync.Mutex
	unicast        map[string]map[int]net.PacketConn
	unspecific     []net.PacketConn
	connGlobal     net.PacketConn
	connGlobalOnce sync.Once
}

func NewReuse() *Reuse {
	return &Reuse{
		unicast:    make(map[string]map[int]net.PacketConn),
		unspecific: make([]net.PacketConn, 0),
	}
}

// getConnGlobal get the global random port socket, if not exist, create
// it first.
func (r *Reuse) getConnGlobal(network string) (net.PacketConn, error) {
	var err error
	r.connGlobalOnce.Do(func() {
		var addr *net.UDPAddr
		var conn net.PacketConn
		var host string
		switch network {
		case "udp4":
			host = "0.0.0.0:0"
		case "udp6":
			host = "[::]:0"
		}
		addr, err = net.ResolveUDPAddr(network, host)
		if err != nil {
			return
		}
		conn, err = net.ListenUDP(network, addr)
		if err != nil {
			return
		}

		r.connGlobal = NewReuseConn(conn)
	})
	if r.connGlobal == nil && err == nil {
		err = errors.New("global socket init not done")
	}
	return r.connGlobal, err
}

// rueseConn Assertion the type of the conn is ReuseConn and inc the ref
func (r *Reuse) reuseConn(conn net.PacketConn) error {
	reuseConn, ok := conn.(*ReuseConn)
	if !ok {
		panic("type ReuseConn Assert failed: something wrong!")
	}
	return reuseConn.Ref()
}

func (r *Reuse) dial(network string, raddr *net.UDPAddr) (net.PacketConn, error) {
	// Find the source address which kernel use
	sip, err := srcs.Select(raddr.IP)
	if err != nil {
		return r.getConnGlobal(network)
	}
	r.lock.Lock()
	defer r.lock.Unlock()

	// If we has a listener on this address, use it to dial
	if c, ok := r.unicast[sip.String()]; ok {
		for _, v := range c {
			return v, nil
		}
	}

	if len(r.unspecific) != 0 {
		return r.unspecific[0], nil
	}

	return r.getConnGlobal(network)
}

func (r *Reuse) Dial(network string, raddr *net.UDPAddr) (net.PacketConn, error) {
	conn, err := r.dial(network, raddr)
	if err != nil {
		return nil, err
	}
	// we are reuse a conn, reference it
	if err = r.reuseConn(conn); err != nil {
		return nil, err
	}
	return conn, nil
}

func (r *Reuse) Listen(network string, laddr *net.UDPAddr) (net.PacketConn, error) {
	conn, err := net.ListenUDP(network, laddr)
	if err != nil {
		return nil, err
	}

	reuseConn := NewReuseConn(conn)

	r.lock.Lock()
	defer r.lock.Unlock()

	switch {
	case laddr.IP.IsUnspecified():
		r.unspecific = append(r.unspecific, reuseConn)
	default:
		if _, ok := r.unicast[laddr.IP.String()]; !ok {
			r.unicast[laddr.IP.String()] = make(map[int]net.PacketConn)
		}
		if _, ok := r.unicast[laddr.IP.String()][laddr.Port]; ok {
			conn.Close()
			return nil, errors.New("addr already listen")
		}
		r.unicast[laddr.IP.String()][laddr.Port] = reuseConn
	}
	return reuseConn, nil
}

func (r *Reuse) Close(addr *net.UDPAddr) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	switch {
	case addr.IP.IsUnspecified():
		for index, conn := range r.unspecific {
			recAddr := conn.LocalAddr().(*net.UDPAddr)
			if recAddr.IP.Equal(addr.IP) && recAddr.Port == addr.Port {
				r.unspecific = append(r.unspecific[:index], r.unspecific[index+1:]...)
				return nil
			}
		}
	default:
		if us, ok := r.unicast[addr.IP.String()]; ok {
			if _, ok := us[addr.Port]; ok {
				delete(us, addr.Port)
			}

			if len(us) == 0 {
				delete(r.unicast, addr.IP.String())
			}
		}
	}
	return nil
}
