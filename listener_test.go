package libp2pquic

import (
	"errors"
	"net"

	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type mockNetListener struct {
	connToAccept net.Conn
	acceptErr    error
	closed       bool
}

func (m *mockNetListener) Accept() (net.Conn, error) {
	if m.acceptErr != nil {
		return nil, m.acceptErr
	}
	return m.connToAccept, nil
}

func (m *mockNetListener) Close() error {
	m.closed = true
	return nil
}

func (m *mockNetListener) Addr() net.Addr {
	panic("not implemented")
}

var _ net.Listener = &mockNetListener{}

var _ = Describe("Listener", func() {
	var (
		l           *listener
		netListener *mockNetListener
		transport   tpt.Transport
	)

	BeforeEach(func() {
		netListener = &mockNetListener{}
		transport = &QuicTransport{}
		l = &listener{
			quicListener: netListener,
			transport:    transport,
		}
	})

	It("returns its addr", func() {
		laddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/12345/quic")
		Expect(err).ToNot(HaveOccurred())
		l, err = newListener(laddr, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(l.Addr().String()).To(Equal("127.0.0.1:12345"))
	})

	It("returns its multiaddr", func() {
		laddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/12346/quic")
		Expect(err).ToNot(HaveOccurred())
		l, err = newListener(laddr, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(l.Multiaddr().String()).To(Equal("/ip4/127.0.0.1/udp/12346/quic"))
	})

	It("closes", func() {
		err := l.Close()
		Expect(err).ToNot(HaveOccurred())
		Expect(netListener.closed).To(BeTrue())
	})

	Context("accepting", func() {
		It("accepts a new conn", func() {
			remoteAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:1234")
			Expect(err).ToNot(HaveOccurred())
			localAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:4321")
			Expect(err).ToNot(HaveOccurred())
			udpConn, err := net.DialUDP("udp", localAddr, remoteAddr)
			netListener.connToAccept = udpConn
			conn, err := l.Accept()
			Expect(err).ToNot(HaveOccurred())
			Expect(conn.LocalMultiaddr().String()).To(Equal("/ip4/127.0.0.1/udp/4321"))
			Expect(conn.RemoteMultiaddr().String()).To(Equal("/ip4/127.0.0.1/udp/1234"))
			Expect(conn.Transport()).To(Equal(transport))
		})

		It("errors if it can't read the muliaddresses of a conn", func() {
			netListener.connToAccept = &net.UDPConn{}
			_, err := l.Accept()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("nil multiaddr"))
		})

		It("errors if no connection can be accepted", func() {
			testErr := errors.New("test error")
			netListener.acceptErr = testErr
			_, err := l.Accept()
			Expect(err).To(MatchError(testErr))
		})
	})
})
