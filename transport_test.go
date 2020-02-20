package libp2pquic

import (
	"crypto/rand"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/pnet"
	tpt "github.com/libp2p/go-libp2p-core/transport"
	ma "github.com/multiformats/go-multiaddr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transport", func() {
	var t tpt.Transport

	BeforeEach(func() {
		t = &transport{}
	})

	It("says if it can dial an address", func() {
		invalidAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234")
		Expect(err).ToNot(HaveOccurred())
		validAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234/quic")
		Expect(err).ToNot(HaveOccurred())
		Expect(t.CanDial(invalidAddr)).To(BeFalse())
		Expect(t.CanDial(validAddr)).To(BeTrue())
	})

	It("supports the QUIC protocol", func() {
		protocols := t.Protocols()
		Expect(protocols).To(HaveLen(1))
		Expect(protocols[0]).To(Equal(ma.P_QUIC))
	})

	It("rejects empty PSKs if pnet.ForcePrivateNetwork is set", func() {
		pnet.ForcePrivateNetwork = true
		defer func() {
			pnet.ForcePrivateNetwork = false
		}()

		priv, _, err := ic.GenerateECDSAKeyPair(rand.Reader)
		Expect(err).ToNot(HaveOccurred())
		_, err = NewTransport(priv, nil)
		Expect(err).To(HaveOccurred())
		Expect(err).To(MatchError(pnet.ErrNotInPrivateNetwork))
	})
})
