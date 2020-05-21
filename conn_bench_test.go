package libp2pquic

import (
	"context"
	"crypto/rand"
	"testing"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
)

func BenchmarkInterceptSecured(b *testing.B) {
	priv1, _, _ := ic.GenerateEd25519Key(rand.Reader)
	priv2, _, _ := ic.GenerateEd25519Key(rand.Reader)

	id2, _ := peer.IDFromPrivateKey(priv2)
	gater := &mockGater{blockedPeer: id2, blockAccept: false}

	t1, _ := NewTransport(priv1, nil, gater)
	t2, _ := NewTransport(priv2, nil, nil)

	laddr, _ := ma.NewMultiaddr("/ip4/127.0.0.1/udp/0/quic")
	l, _ := t2.Listen(laddr)
	defer l.Close()

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	raddr := l.Multiaddr()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := t1.Dial(context.Background(), raddr, id2)
		if err == nil {
			b.Fatal("expected error")
		}
	}
}
