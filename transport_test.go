package libp2pquic

import (
	ma "github.com/multiformats/go-multiaddr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transport", func() {
	var t *QuicTransport

	BeforeEach(func() {
		t = NewQuicTransport(nil)
	})

	It("matches", func() {
		invalidAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234")
		Expect(err).ToNot(HaveOccurred())
		validAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234/quic")
		Expect(err).ToNot(HaveOccurred())
		Expect(t.Matches(invalidAddr)).To(BeFalse())
		Expect(t.Matches(validAddr)).To(BeTrue())
	})
})
