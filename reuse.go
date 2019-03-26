package libp2pquic

import (
	"errors"
	"net"
	"sync"

	srcs "github.com/lnykww/go-src-select"
)

type reuseConn struct {
	net.PacketConn
	mutex sync.Mutex
	ref   int
}

func NewReuseConn(conn net.PacketConn) *reuseConn {
	return &reuseConn{
		PacketConn: conn,
		ref:        1,
	}
}

func (rc *reuseConn) Ref() bool {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	if rc.ref == 0 {
		return false
	}
	rc.ref++
	return true
}

func (rc *reuseConn) Close() error {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
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
	mutex          sync.Mutex
	unicast        map[string]map[int]*reuseConn
	unspecific     []*reuseConn
	connGlobal     *reuseConn
	connGlobalOnce sync.Once
}

func NewReuse() *Reuse {
	return &Reuse{
		unicast:    make(map[string]map[int]*reuseConn),
		unspecific: make([]*reuseConn, 0),
	}
}

// getConnGlobal get the global random port socket, if not exist, create
// it first.
func (r *Reuse) getConnGlobal(network string) (*reuseConn, error) {
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

func (r *Reuse) dial(network string, raddr *net.UDPAddr) (*reuseConn, error) {
	// Find the source address which kernel use
	sip, err := srcs.Select(raddr.IP)
	if err != nil {
		return r.getConnGlobal(network)
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()

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

	if ok := conn.Ref(); ok {
		return conn, nil
	}
	return nil, errors.New("Can not reference any connection for reuse")
}

func (r *Reuse) Listen(network string, laddr *net.UDPAddr) (net.PacketConn, error) {
	conn, err := net.ListenUDP(network, laddr)
	if err != nil {
		return nil, err
	}

	rconn := NewReuseConn(conn)

	r.mutex.Lock()
	defer r.mutex.Unlock()

	switch {
	case laddr.IP.IsUnspecified():
		r.unspecific = append(r.unspecific, rconn)
	default:
		if _, ok := r.unicast[laddr.IP.String()]; !ok {
			r.unicast[laddr.IP.String()] = make(map[int]*reuseConn)
		}
		if _, ok := r.unicast[laddr.IP.String()][laddr.Port]; ok {
			conn.Close()
			return nil, errors.New("addr already listen")
		}
		r.unicast[laddr.IP.String()][laddr.Port] = rconn
	}
	return rconn, nil
}

func (r *Reuse) Close(addr *net.UDPAddr) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
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