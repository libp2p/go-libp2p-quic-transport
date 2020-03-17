package libp2pquic

import (
	"net"

	filter "github.com/libp2p/go-maddr-filter"
)

type filteredConn struct {
	net.PacketConn

	filters *filter.Filters
}

func newFilteredConn(c net.PacketConn, filters *filter.Filters) net.PacketConn {
	return &filteredConn{PacketConn: c, filters: filters}
}

func (c *filteredConn) ReadFrom(b []byte) (n int, addr net.Addr, rerr error) {
	for {
		n, addr, rerr = c.PacketConn.ReadFrom(b)
		// Short Header packet, see https://tools.ietf.org/html/draft-ietf-quic-invariants-07#section-4.2.
		if n < 1 || b[0]&0x80 == 0 {
			return
		}
		maddr, err := toQuicMultiaddr(addr)
		if err != nil {
			panic(err)
		}
		if !c.filters.AddrBlocked(maddr) {
			return
		}
	}
}
