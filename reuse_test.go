package libp2pquic

import (
	"net"
	"strings"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Reuse", func() {
	var reuse *Reuse

	BeforeEach(func() {
		reuse = NewReuse()
	})

	It("IPv4 Reuse correct socket", func() {
		host := "0.0.0.0"
		network := "udp4"
		daddr := "8.8.8.8"
		laddr := "127.0.0.1"

		unspecific, err := net.ResolveUDPAddr(network, strings.Join([]string{host, ":1004"}, ""))
		Expect(err).ToNot(HaveOccurred())
		unspecificConn, err := reuse.Listen(network, unspecific)
		Expect(err).ToNot(HaveOccurred())

		// Only has unspecific listener, dial to dest("8.8.8.8")
		// expect use the unspecific socket
		dest, err := net.ResolveUDPAddr(network, strings.Join([]string{daddr, ":1004"}, ""))
		Expect(err).ToNot(HaveOccurred())
		dialConn, err := reuse.Dial(network, dest)
		Expect(err).ToNot(HaveOccurred())
		Expect(dialConn.(*ReuseConn).PacketConn).To(Equal(unspecificConn.(*ReuseConn).PacketConn))

		// Add 127.0.0.1 addr, dial to dest("8.8.8.8")
		// expect use the unspecific socket
		localAddr, err := net.ResolveUDPAddr(network, strings.Join([]string{laddr, ":1005"}, ""))
		Expect(err).ToNot(HaveOccurred())
		localHostConn, err := reuse.Listen(network, localAddr)
		Expect(err).ToNot(HaveOccurred())

		dialConn, err = reuse.Dial(network, dest)
		Expect(err).ToNot(HaveOccurred())
		Expect(dialConn.(*ReuseConn).PacketConn).To(Equal(unspecificConn.(*ReuseConn).PacketConn))

		// dial to localhost expcet use the localHostConn
		localhost, err := net.ResolveUDPAddr(network, strings.Join([]string{laddr, ":1006"}, ""))
		Expect(err).ToNot(HaveOccurred())
		localHostDialConn, err := reuse.Dial(network, localhost)
		Expect(err).ToNot(HaveOccurred())
		Expect(localHostDialConn.(*ReuseConn).PacketConn).To(Equal(localHostConn.(*ReuseConn).PacketConn))

		// close the unspecific listener
		reuse.Close(unspecific)
		// dial to dest("8.8.8.8"), expect use the global conn
		dialConnGlobal, err := reuse.Dial(network, dest)
		Expect(err).ToNot(HaveOccurred())
		connLocalAddr, ok := dialConnGlobal.LocalAddr().(*net.UDPAddr)
		Expect(ok).To(BeTrue())
		Expect(connLocalAddr.Port).NotTo(Equal(1004))
		Expect(connLocalAddr.IP.IsUnspecified()).To(BeTrue())

		// dial to localhost also use the localHostConn
		localHostDialConn2, err := reuse.Dial(network, localhost)
		Expect(err).ToNot(HaveOccurred())
		Expect(localHostDialConn2.(*ReuseConn).PacketConn).To(Equal(localHostConn.(*ReuseConn).PacketConn))
		// close the localAddr listener
		reuse.Close(localAddr)

		// dial to localhost expect use the global conn
		dialConnGlobal2, err := reuse.Dial(network, localhost)
		Expect(err).ToNot(HaveOccurred())
		connLocalAddr2, ok := dialConnGlobal2.LocalAddr().(*net.UDPAddr)
		Expect(ok).To(BeTrue())
		Expect(connLocalAddr2.Port).NotTo(Equal(1004))
		Expect(connLocalAddr2.IP.IsUnspecified()).To(BeTrue())
	})

	It("IPv6 Reuse correct socket", func() {
		host := "[::]"
		network := "udp6"
		daddr := "[2001:4860:4860::8888]"
		laddr := "[::1]"

		unspecific, err := net.ResolveUDPAddr(network, strings.Join([]string{host, ":1004"}, ""))
		Expect(err).ToNot(HaveOccurred())
		unspecificConn, err := reuse.Listen(network, unspecific)
		Expect(err).ToNot(HaveOccurred())

		// Only has unspecific listener, dial to dest("2001:4860:4860::8888")
		// expect use the unspecific socket
		dest, err := net.ResolveUDPAddr(network, strings.Join([]string{daddr, ":1004"}, ""))
		Expect(err).ToNot(HaveOccurred())
		dialConn, err := reuse.Dial(network, dest)
		Expect(err).ToNot(HaveOccurred())
		Expect(dialConn.(*ReuseConn).PacketConn).To(Equal(unspecificConn.(*ReuseConn).PacketConn))

		// Add [::1] addr, dial to dest("2001:4860:4860::8888")
		// expect use the unspecific socket
		localAddr, err := net.ResolveUDPAddr(network, strings.Join([]string{laddr, ":1005"}, ""))
		Expect(err).ToNot(HaveOccurred())
		localHostConn, err := reuse.Listen(network, localAddr)
		Expect(err).ToNot(HaveOccurred())

		dialConn, err = reuse.Dial(network, dest)
		Expect(err).ToNot(HaveOccurred())
		// for ipv6 will use localhost or unspecific connection
		// if there is no default ipv6 route, will use localhost
		// what ever never use globalConnection
		Expect(reuse.connGlobal).To(BeNil())

		// dial to localhost expcet use the localHostConn
		localhost, err := net.ResolveUDPAddr(network, strings.Join([]string{laddr, ":1006"}, ""))
		Expect(err).ToNot(HaveOccurred())
		localHostDialConn, err := reuse.Dial(network, localhost)
		Expect(err).ToNot(HaveOccurred())
		Expect(localHostDialConn.(*ReuseConn).PacketConn).To(Equal(localHostConn.(*ReuseConn).PacketConn))

		// close the unspecific listener
		reuse.Close(unspecific)

		// dial to localhost also use the localHostConn
		localHostDialConn2, err := reuse.Dial(network, localhost)
		Expect(err).ToNot(HaveOccurred())
		Expect(localHostDialConn2.(*ReuseConn).PacketConn).To(Equal(localHostConn.(*ReuseConn).PacketConn))

		// close the localAddr listener
		reuse.Close(localAddr)
		// dial to dest("2001:4860:4860::8888"), expect use the global conn
		Expect(err).ToNot(HaveOccurred())
		dialConnGlobal, err := reuse.Dial(network, dest)
		Expect(err).ToNot(HaveOccurred())
		connLocalAddr, ok := dialConnGlobal.LocalAddr().(*net.UDPAddr)
		Expect(ok).To(BeTrue())
		Expect(connLocalAddr.Port).NotTo(Equal(1004))
		Expect(connLocalAddr.IP.IsUnspecified()).To(BeTrue())

	})

	It("ReuseConn test", func() {
		network := "udp4"
		addr1, err := net.ResolveUDPAddr(network, "127.0.0.1:4444")
		Expect(err).ToNot(HaveOccurred())
		addr2, err := net.ResolveUDPAddr(network, "127.0.0.1:4445")
		Expect(err).ToNot(HaveOccurred())
		conn1, err := net.ListenUDP(network, addr1)
		Expect(err).ToNot(HaveOccurred())
		conn2, err := net.ListenUDP(network, addr2)
		Expect(err).ToNot(HaveOccurred())

		reuseConn1 := NewReuseConn(conn1)
		reuseConn2 := NewReuseConn(conn2)

		TestData := "ReuseConnTest"

		sendData := func() {
			n, err := conn1.WriteTo([]byte(TestData), addr2)
			Expect(err).ToNot(HaveOccurred())
			Expect(n).To(Equal(len(TestData)))
		}

		go sendData()

		reuseConn2.SetReadDeadline(time.Now().Add(5 * time.Second))
		data := make([]byte, len(TestData))
		_, _, err = reuseConn2.ReadFrom(data)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(data[:])).To(Equal(TestData))

		err = reuseConn2.Ref()
		Expect(err).ToNot(HaveOccurred())

		err = reuseConn2.Close()
		Expect(err).ToNot(HaveOccurred())

		go sendData()

		reuseConn2.SetReadDeadline(time.Now().Add(5 * time.Second))
		data = make([]byte, len(TestData))
		_, _, err = reuseConn2.ReadFrom(data)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(data[:])).To(Equal(TestData))

		err = reuseConn2.Close()
		Expect(err).ToNot(HaveOccurred())
		reuseConn2.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, _, err = reuseConn2.ReadFrom(data)
		Expect(strings.Contains(err.Error(), "use of closed network connection")).To(BeTrue())

		err = reuseConn1.Close()
		Expect(err).ToNot(HaveOccurred())
		reuseConn1.SetReadDeadline(time.Now().Add(5 * time.Second))
		_, _, err = reuseConn1.ReadFrom(data)
		Expect(strings.Contains(err.Error(), "use of closed network connection")).To(BeTrue())
	})
})
