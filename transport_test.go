package libp2pquic

import (
	tpt "github.com/libp2p/go-libp2p-transport"
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
})

var _ = Describe("Port reuse", func() {
	var c *connManager
	BeforeEach(func() {
		c = newConnManager()
	})
	It("reuse IPv4 port", func() {
		addr, _ := ma.NewMultiaddr("/ip4/0.0.0.0/udp/40000/quic")
		listenConn, err := c.listenUDP(addr)
		Expect(err).ToNot(HaveOccurred())

		dialConn, err := c.GetConnForAddr("udp4", "127.0.0.1:40004")
		Expect(err).ToNot(HaveOccurred())
		Expect(dialConn).To(Equal(listenConn))
		listenConn.Close()
	})
	It("reuse IPv6 port", func() {
		addr, _ := ma.NewMultiaddr("/ip6/::/udp/40001/quic")
		listenConn, err := c.listenUDP(addr)
		Expect(err).ToNot(HaveOccurred())

		dialConn, err := c.GetConnForAddr("udp6", "[::1]:40004")
		Expect(err).ToNot(HaveOccurred())
		Expect(dialConn).To(Equal(listenConn))
		listenConn.Close()
	})
	It("listen after dial won't reuse conn", func() {
		dialConn, err := c.GetConnForAddr("udp4", "127.0.0.1:40004")
		Expect(err).ToNot(HaveOccurred())

		addr, _ := ma.NewMultiaddr("/ip4/0.0.0.0/udp/40002/quic")
		listenConn, err := c.listenUDP(addr)
		Expect(err).ToNot(HaveOccurred())
		Expect(listenConn).ToNot(Equal(dialConn))

		dialConn.Close()
		listenConn.Close()
	})
	It("use listen conn by default", func() {
		dialConn, err := c.GetConnForAddr("udp4", "127.0.0.1:40004")
		Expect(err).ToNot(HaveOccurred())

		addr, _ := ma.NewMultiaddr("/ip4/0.0.0.0/udp/40003/quic")
		listenConn, err := c.listenUDP(addr)
		Expect(err).ToNot(HaveOccurred())

		dialConn2, err := c.GetConnForAddr("udp4", "1.2.3.4:40005")
		Expect(err).ToNot(HaveOccurred())
		Expect(dialConn2).To(Equal(listenConn))

		dialConn.Close()
		listenConn.Close()
	})
})
