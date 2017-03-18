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

	Context("listening", func() {
		It("creates a new listener", func() {
			maddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234/quic")
			Expect(err).ToNot(HaveOccurred())
			ln, err := t.Listen(maddr)
			Expect(err).ToNot(HaveOccurred())
			Expect(ln.Multiaddr()).To(Equal(maddr))
		})

		It("returns an existing listener", func() {
			maddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1235/quic")
			Expect(err).ToNot(HaveOccurred())
			ln, err := t.Listen(maddr)
			Expect(err).ToNot(HaveOccurred())
			Expect(ln.Multiaddr()).To(Equal(maddr))
			ln2, err := t.Listen(maddr)
			Expect(err).ToNot(HaveOccurred())
			Expect(ln2).To(Equal(ln))
			Expect(t.listeners).To(HaveLen(1))
		})
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
