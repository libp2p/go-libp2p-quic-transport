package libp2pquic

import (
	"errors"
	"net"

	addrutil "github.com/libp2p/go-addr-util"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

var quicMA ma.Multiaddr

func init() {
	var err error
	quicMA, err = ma.NewMultiaddr("/quic")
	if err != nil {
		panic(err)
	}
}

func toQuicMultiaddr(na net.Addr) (ma.Multiaddr, error) {
	udpMA, err := manet.FromNetAddr(na)
	if err != nil {
		return nil, err
	}
	return udpMA.Encapsulate(quicMA), nil
}

func fromQuicMultiaddr(addr ma.Multiaddr) (net.Addr, error) {
	return manet.ToNetAddr(addr.Decapsulate(quicMA))
}

// this is a bit hacky, but it fixes a problem with "unspecified addresses" (0.0.0.0)
// appearing in connections. (this breaks libp2p and ipfs assumptions -- listeners
// may have unspecified addrs, but connections should not -- should use loopback ip).
func toQuicMultiaddrLocal(na net.Addr) (ma.Multiaddr, error) {
	qma, err := toQuicMultiaddr(na)
	if err != nil {
		return nil, err
	}
	rma, err := addrutil.ResolveUnspecifiedAddresses([]ma.Multiaddr{qma}, nil)
	if err != nil {
		return nil, err
	}
	if len(rma) < 1 {
		return nil, errors.New("failed to resolve multiaddr")
	}
	return rma[0], nil
}
