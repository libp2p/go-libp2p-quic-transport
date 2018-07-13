package libp2pquic

import (
	"crypto/rand"

	ic "github.com/libp2p/go-libp2p-crypto"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Crypto", func() {
	var (
		serverKey, clientKey ic.PrivKey
		serverID, clientID   peer.ID
	)

	Describe("keyToCertificate", func() {
		It("Ed25519", func() {
			priv, _, err := ic.GenerateEd25519Key(rand.Reader)
			Expect(err).ToNot(HaveOccurred())
			key, cert, err := keyToCertificate(priv)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
