// +build linux

package libp2pquic

import (
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reuse (on Linux)", func() {
	var reuse *reuse

	BeforeEach(func() {
		var err error
		reuse, err = newReuse(nil)
		Expect(err).ToNot(HaveOccurred())
	})

	Context("creating and reusing connections", func() {
		AfterEach(func() { closeAllConns(reuse) })

		It("reuses a connection it created for listening on a specific interface", func() {
			raddr, err := net.ResolveUDPAddr("udp4", "1.1.1.1:1234")
			Expect(err).ToNot(HaveOccurred())
			ips, err := reuse.getSourceIPs("udp4", raddr)
			Expect(err).ToNot(HaveOccurred())
			Expect(ips).ToNot(BeEmpty())
			// listen
			addr, err := net.ResolveUDPAddr("udp4", ips[0].String()+":0")
			Expect(err).ToNot(HaveOccurred())
			lconn, err := reuse.Listen("udp4", addr)
			Expect(err).ToNot(HaveOccurred())
			Expect(lconn.GetCount()).To(Equal(1))
			// dial
			conn, err := reuse.Dial("udp4", raddr)
			Expect(err).ToNot(HaveOccurred())
			Expect(conn.GetCount()).To(Equal(2))
		})
	})
})
