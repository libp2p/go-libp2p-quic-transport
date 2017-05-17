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
		laddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/12345/quic")
		Expect(err).ToNot(HaveOccurred())
		l, err = newListener(laddr, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(l.Addr().String()).To(Equal("127.0.0.1:12345"))
	})

	It("returns its multiaddr", func() {
		laddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/12346/quic")
		Expect(err).ToNot(HaveOccurred())
		l, err = newListener(laddr, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(l.Multiaddr().String()).To(Equal("/ip4/127.0.0.1/udp/12346/quic"))
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
