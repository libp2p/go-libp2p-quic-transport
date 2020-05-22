package libp2pquic

import (
	"net"

	"github.com/libp2p/go-libp2p-core/connmgr"

	ma "github.com/multiformats/go-multiaddr"
)

type connAddrs struct {
	lmAddr ma.Multiaddr
	rmAddr ma.Multiaddr
}

func (c *connAddrs) LocalMultiaddr() ma.Multiaddr {
	return c.lmAddr
}

func (c *connAddrs) RemoteMultiaddr() ma.Multiaddr {
	return c.rmAddr
}

type filteredConn struct {
	net.PacketConn

	lmAddr ma.Multiaddr
	gater  connmgr.ConnectionGater
}

func newFilteredConn(c net.PacketConn, gater connmgr.ConnectionGater) net.PacketConn {
	lmAddr, err := toQuicMultiaddr(c.LocalAddr())
	if err != nil {
		panic(err)
	}

	return &filteredConn{PacketConn: c, gater: gater, lmAddr: lmAddr}
}

func (c *filteredConn) ReadFrom(b []byte) (n int, addr net.Addr, rerr error) {
	for {
		n, addr, rerr = c.PacketConn.ReadFrom(b)
		// Short Header packet, see https://tools.ietf.org/html/draft-ietf-quic-invariants-07#section-4.2.
		if n < 1 || b[0]&0x80 == 0 {
			return
		}
		rmAddr, err := toQuicMultiaddr(addr)
		if err != nil {
			panic(err)
		}

		connAddrs := &connAddrs{lmAddr: c.lmAddr, rmAddr: rmAddr}

		if c.gater.InterceptAccept(connAddrs) {
			return
		}
	}
}
