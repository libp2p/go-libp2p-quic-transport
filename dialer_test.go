package libp2pquic

import (
	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Listener", func() {
	var (
		d         *dialer
		transport tpt.Transport
	)

	BeforeEach(func() {
		var err error
		transport = &QuicTransport{}
		d, err = newDialer(transport)
		Expect(err).ToNot(HaveOccurred())
	})

	It("dials", func() {
		addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/8888")
		Expect(err).ToNot(HaveOccurred())

		// start a listener to connect to
		var ln *listener
		go func() {
			defer GinkgoRecover()
			ln, err = newListener(addr, nil, transport)
			Expect(err).ToNot(HaveOccurred())
			_, err = ln.Accept()
			Expect(err).ToNot(HaveOccurred())
		}()

		Eventually(func() *listener { return ln }).ShouldNot(BeNil())
		conn, err := d.Dial(addr)
		Expect(err).ToNot(HaveOccurred())
		Expect(conn.Transport()).To(Equal(d.transport))
	})

	It("matches", func() {
		invalidAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234")
		Expect(err).ToNot(HaveOccurred())
		validAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234/quic")
		Expect(err).ToNot(HaveOccurred())
		Expect(d.Matches(invalidAddr)).To(BeFalse())
		Expect(d.Matches(validAddr)).To(BeTrue())
	})
})
