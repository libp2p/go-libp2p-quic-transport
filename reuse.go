package libp2pquic

import (
	"errors"
	"fmt"
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

func (rc *reuseConn) canRef() bool {
	return rc.ref > 0
}

func (rc *reuseConn) Ref() bool {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	if rc.canRef() {
		rc.ref++
		return true
	}
	return false
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

const RuseDialRetryTime = 3

type Reuse struct {
	mutex sync.Mutex
	//unicast: the key is ip address and port number.
	unicast map[string]map[int]*reuseConn
	//unspecific: the key is port number.
	unspecific map[int]*reuseConn
	connGlobal *reuseConn
}

func NewReuse() *Reuse {
	return &Reuse{
		unicast:    make(map[string]map[int]*reuseConn),
		unspecific: make(map[int]*reuseConn),
	}
}

// dialGlobal get the global random port socket, if not exist, create
// it first.
func (r *Reuse) dialGlobal(network string) (*reuseConn, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.connGlobal == nil || !r.connGlobal.Ref() {
		var host string
		switch network {
		case "udp4":
			host = "0.0.0.0:0"
		case "udp6":
			host = "[::]:0"
		}
		addr, err := net.ResolveUDPAddr(network, host)
		if err != nil {
			return nil, err
		}
		conn, err := net.ListenUDP(network, addr)
		if err != nil {
			return nil, err
		}
		r.connGlobal = NewReuseConn(conn)
	}
	return r.connGlobal, nil
}

func (r *Reuse) dialBest(network string, raddr *net.UDPAddr) (*reuseConn, error) {
	// Find the source address which kernel use
	sip, err := srcs.Select(raddr.IP)
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if err != nil {
		return nil, err
	}

	// If we has a listener on this address, use it to dial
	if c, ok := r.unicast[sip.String()]; ok {
		for _, v := range c {
			return v, nil
		}
	}

	if len(r.unspecific) != 0 {
		// return the first value of the unspecific map
		for _, conn := range r.unspecific {
			return conn, nil
		}
	}

	return nil, errors.New("Not found the best conn")
}

func (r *Reuse) Dial(network string, raddr *net.UDPAddr) (net.PacketConn, error) {
	for i := 0; i < RuseDialRetryTime; i++ {
		conn, err := r.dialBest(network, raddr)
		if err != nil {
			// If there is no best connection which we have created. Use the global
			// connection as default.
			global, err := r.dialGlobal(network)
			if err == nil {
				return global, nil
			}
			continue
		}

		if ok := conn.Ref(); ok {
			return conn, nil
		}
	}
	return nil, errors.New("Can not reference any connection for reuse")
}

func (r *Reuse) Listen(network string, laddr *net.UDPAddr) (net.PacketConn, error) {
	conn, err := net.ListenUDP(network, laddr)
	if err != nil {
		return nil, err
	}
	rconn := NewReuseConn(conn)
	// Use the addr store in connection  to handling the situation of listening on port 0
	realAddr := rconn.LocalAddr().(*net.UDPAddr)

	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Deal with listen on a unspecific address
	if laddr.IP.IsUnspecified() {
		// The kernel already checked that the laddr is not already listen
		// so we need not check here (when we create ListenUDP).
		r.unspecific[realAddr.Port] = rconn
		return rconn, err
	}

	// Deal with listen on a unicast address
	if _, ok := r.unicast[laddr.IP.String()]; !ok {
		r.unicast[laddr.IP.String()] = make(map[int]*reuseConn)
	}

	// The kernel already checked that the laddr is not already listen
	// so we need not check here (when we create ListenUDP).
	r.unicast[laddr.IP.String()][realAddr.Port] = rconn
	return rconn, err
}

func (r *Reuse) Close(addr *net.UDPAddr) error {
	var addrNotFound = fmt.Errorf("can't find a connection with specific addr: %s", addr.String())

	r.mutex.Lock()
	defer r.mutex.Unlock()
	// deal with listen on a unspecific address
	if addr.IP.IsUnspecified() {
		if _, ok := r.unspecific[addr.Port]; ok {
			delete(r.unspecific, addr.Port)
			return nil
		}
		return addrNotFound
	}

	// deal with listen on a unicast address
	if us, ok := r.unicast[addr.IP.String()]; ok {
		if _, ok := us[addr.Port]; ok {
			delete(us, addr.Port)
		} else {
			return addrNotFound
		}

		if len(us) == 0 {
			delete(r.unicast, addr.IP.String())
		}

		return nil
	}
	return addrNotFound
}
