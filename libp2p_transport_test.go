package libp2pquic_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"testing"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	peer "github.com/libp2p/go-libp2p-core/peer"
	tpt "github.com/libp2p/go-libp2p-core/transport"
	tptt "github.com/libp2p/go-libp2p-testing/suites/transport"

	quic "github.com/libp2p/go-libp2p-quic-transport"
)

func TestLibp2pTransportSuite(t *testing.T) {
	create := func() (tpt.Transport, peer.ID, ic.PrivKey) {
		key, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			t.Fatal(err)
		}

		priv, err := ic.UnmarshalRsaPrivateKey(x509.MarshalPKCS1PrivateKey(key))
		if err != nil {
			t.Fatal(err)
		}

		id, err := peer.IDFromPrivateKey(priv)
		if err != nil {
			t.Fatal(err)
		}

		transport, err := quic.NewTransport(priv)
		if err != nil {
			t.Fatal(err)
		}
		return transport, id, priv
	}
	clientTpt, _, _ := create()
	serverTpt, serverID, _ := create()

	tptt.SubtestTransport(t, serverTpt, clientTpt, "/ip4/127.0.0.1/udp/0/quic", serverID)
}
