package libp2pquic

import (
	"net"
	"runtime"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reuse", func() {
	var reuse *reuse

	BeforeEach(func() {
		var err error
		reuse, err = newReuse()
		Expect(err).ToNot(HaveOccurred())
	})

	It("creates a new global connection when listening on 0.0.0.0", func() {
		addr, err := net.ResolveUDPAddr("udp4", "0.0.0.0:0")
		Expect(err).ToNot(HaveOccurred())
		conn, err := reuse.Listen("udp4", addr)
		Expect(err).ToNot(HaveOccurred())
		Expect(conn.GetCount()).To(Equal(1))
	})

	It("creates a new global connection when listening on [::]", func() {
		addr, err := net.ResolveUDPAddr("udp6", "[::]:1234")
		Expect(err).ToNot(HaveOccurred())
		conn, err := reuse.Listen("udp6", addr)
		Expect(err).ToNot(HaveOccurred())
		Expect(conn.GetCount()).To(Equal(1))
	})

	It("creates a new global connection when dialing", func() {
		addr, err := net.ResolveUDPAddr("udp4", "1.1.1.1:1234")
		Expect(err).ToNot(HaveOccurred())
		conn, err := reuse.Dial("udp4", addr)
		Expect(err).ToNot(HaveOccurred())
		Expect(conn.GetCount()).To(Equal(1))
		laddr := conn.LocalAddr().(*net.UDPAddr)
		Expect(laddr.IP.String()).To(Equal("0.0.0.0"))
		Expect(laddr.Port).ToNot(BeZero())
	})

	It("reuses a connection it created for listening when dialing", func() {
		// listen
		addr, err := net.ResolveUDPAddr("udp4", "0.0.0.0:0")
		Expect(err).ToNot(HaveOccurred())
		lconn, err := reuse.Listen("udp4", addr)
		Expect(err).ToNot(HaveOccurred())
		Expect(lconn.GetCount()).To(Equal(1))
		// dial
		raddr, err := net.ResolveUDPAddr("udp4", "1.1.1.1:1234")
		Expect(err).ToNot(HaveOccurred())
		conn, err := reuse.Dial("udp4", raddr)
		Expect(err).ToNot(HaveOccurred())
		Expect(conn.GetCount()).To(Equal(2))
	})

	if runtime.GOOS == "linux" {
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
	}
})
