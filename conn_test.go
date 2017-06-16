package libp2pquic

import (
	"errors"
	"net"

	smux "github.com/jbenet/go-stream-muxer"
	quic "github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/protocol"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type mockStream struct {
	id protocol.StreamID
}

func (s *mockStream) Close() error                { return nil }
func (s *mockStream) Reset(error)                 { return }
func (s *mockStream) Read([]byte) (int, error)    { return 0, nil }
func (s *mockStream) Write([]byte) (int, error)   { return 0, nil }
func (s *mockStream) StreamID() protocol.StreamID { return s.id }

var _ quic.Stream = &mockStream{}

type mockQuicSession struct {
	closed              bool
	waitUntilClosedChan chan struct{} // close this chan to make WaitUntilClosed return

	localAddr  net.Addr
	remoteAddr net.Addr

	streamToAccept  quic.Stream
	streamAcceptErr error

	streamToOpen  quic.Stream
	streamOpenErr error
}

var _ quic.Session = &mockQuicSession{}

func (s *mockQuicSession) AcceptStream() (quic.Stream, error) {
	return s.streamToAccept, s.streamAcceptErr
}
func (s *mockQuicSession) OpenStream() (quic.Stream, error) { return s.streamToOpen, s.streamOpenErr }
func (s *mockQuicSession) OpenStreamSync() (quic.Stream, error) {
	return s.streamToOpen, s.streamOpenErr
}
func (s *mockQuicSession) Close(error) error    { s.closed = true; return nil }
func (s *mockQuicSession) LocalAddr() net.Addr  { return s.localAddr }
func (s *mockQuicSession) RemoteAddr() net.Addr { return s.remoteAddr }
func (s *mockQuicSession) WaitUntilClosed()     { <-s.waitUntilClosedChan }

var _ = Describe("Conn", func() {
	var (
		conn *quicConn
		sess *mockQuicSession
	)

	BeforeEach(func() {
		sess = &mockQuicSession{
			localAddr:           &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1337},
			remoteAddr:          &net.UDPAddr{IP: net.IPv4(192, 168, 13, 37), Port: 1234},
			waitUntilClosedChan: make(chan struct{}),
		}
		var err error
		conn, err = newQuicConn(sess, nil)
		Expect(err).ToNot(HaveOccurred())
	})

	It("has the correct local address", func() {
		Expect(conn.LocalAddr()).To(Equal(sess.localAddr))
		Expect(conn.LocalMultiaddr().String()).To(Equal("/ip4/127.0.0.1/udp/1337/quic"))
	})

	It("has the correct remote address", func() {
		Expect(conn.RemoteAddr()).To(Equal(sess.remoteAddr))
		Expect(conn.RemoteMultiaddr().String()).To(Equal("/ip4/192.168.13.37/udp/1234/quic"))
	})

	It("closes", func() {
		err := conn.Close()
		Expect(err).ToNot(HaveOccurred())
		Expect(sess.closed).To(BeTrue())
	})

	It("says if it is closed", func() {
		Consistently(func() bool { return conn.IsClosed() }).Should(BeFalse())
		close(sess.waitUntilClosedChan)
		Eventually(func() bool { return conn.IsClosed() }).Should(BeTrue())
	})

	Context("opening streams", func() {
		It("opens streams", func() {
			s := &mockStream{id: 1337}
			sess.streamToOpen = s
			str, err := conn.OpenStream()
			Expect(err).ToNot(HaveOccurred())
			Expect(str.(*quicStream).Stream).To(Equal(s))
		})

		It("errors when it can't open a stream", func() {
			testErr := errors.New("stream open err")
			sess.streamOpenErr = testErr
			_, err := conn.OpenStream()
			Expect(err).To(MatchError(testErr))
		})
	})

	Context("accepting streams", func() {
		It("accepts streams", func() {
			s := &mockStream{id: 1337}
			sess.streamToAccept = s
			str, err := conn.AcceptStream()
			Expect(err).ToNot(HaveOccurred())
			Expect(str.(*quicStream).Stream).To(Equal(s))
		})

		It("errors when it can't open a stream", func() {
			testErr := errors.New("stream open err")
			sess.streamAcceptErr = testErr
			_, err := conn.AcceptStream()
			Expect(err).To(MatchError(testErr))
		})
	})

	Context("serving", func() {
		var (
			handler           func(smux.Stream)
			handlerCalled     bool
			handlerCalledWith smux.Stream
		)

		BeforeEach(func() {
			handlerCalled = false
			handlerCalledWith = nil
			handler = func(s smux.Stream) {
				handlerCalledWith = s
				handlerCalled = true
			}
		})

		It("calls the handler", func() {
			str := &mockStream{id: 5}
			sess.streamToAccept = str
			var returned bool
			go func() {
				conn.Serve(handler)
				returned = true
			}()
			Eventually(func() bool { return handlerCalled }).Should(BeTrue())
			Expect(handlerCalledWith.(*quicStream).Stream).To(Equal(str))
			// make the go-routine return
			sess.streamAcceptErr = errors.New("stop test")
		})

		It("returns when accepting a stream errors", func() {
			sess.streamAcceptErr = errors.New("accept err")
			var returned bool
			go func() {
				conn.Serve(handler)
				returned = true
			}()
			Eventually(func() bool { return returned }).Should(BeTrue())
			Expect(handlerCalled).To(BeFalse())
		})
	})

})
