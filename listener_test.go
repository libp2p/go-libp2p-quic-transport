package libp2pquic

import (
	"errors"
	"net"

	tpt "github.com/libp2p/go-libp2p-transport"
	quic "github.com/lucas-clemente/quic-go"
	ma "github.com/multiformats/go-multiaddr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type mockQuicListener struct {
	connToAccept net.Conn
	serveErr     error
	closed       bool

	sessionToAccept quic.Session
	acceptErr       error
}

var _ quic.Listener = &mockQuicListener{}

func (m *mockQuicListener) Close() error                  { m.closed = true; return nil }
func (m *mockQuicListener) Accept() (quic.Session, error) { return m.sessionToAccept, m.acceptErr }
func (m *mockQuicListener) Addr() net.Addr                { panic("not implemented") }

var _ = Describe("Listener", func() {
	var (
		l            *listener
		quicListener *mockQuicListener
		transport    tpt.Transport
	)

	BeforeEach(func() {
		quicListener = &mockQuicListener{}
		transport = &QuicTransport{}
		l = &listener{
			quicListener: quicListener,
			transport:    transport,
		}
	})

	It("returns its addr", func() {
		laddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/0/quic")
		Expect(err).ToNot(HaveOccurred())
		l, err = newListener(laddr, nil)
		Expect(err).ToNot(HaveOccurred())
		as := l.Addr().String()
		Expect(as).ToNot(Equal("127.0.0.1:0)"))
		Expect(as).To(ContainSubstring("127.0.0.1:"))
	})

	It("returns its multiaddr", func() {
		laddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/0/quic")
		Expect(err).ToNot(HaveOccurred())
		l, err = newListener(laddr, nil)
		Expect(err).ToNot(HaveOccurred())
		mas := l.Multiaddr().String()
		Expect(mas).ToNot(Equal("/ip4/127.0.0.1/udp/0/quic"))
		Expect(mas).To(ContainSubstring("/ip4/127.0.0.1/udp/"))
		Expect(mas).To(ContainSubstring("/quic"))
	})

	It("closes", func() {
		err := l.Close()
		Expect(err).ToNot(HaveOccurred())
		Expect(quicListener.closed).To(BeTrue())
	})

	Context("accepting", func() {
		It("errors if no connection can be accepted", func() {
			testErr := errors.New("test error")
			quicListener.acceptErr = testErr
			_, err := l.Accept()
			Expect(err).To(MatchError(testErr))
		})
	})
})
