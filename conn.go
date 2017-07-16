package libp2pquic

import (
	"fmt"
	"net"
	"sync"

	smux "github.com/jbenet/go-stream-muxer"
	tpt "github.com/libp2p/go-libp2p-transport"
	quic "github.com/lucas-clemente/quic-go"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

type quicConn struct {
	mutex sync.RWMutex

	sess      quic.Session
	transport tpt.Transport

	laddr ma.Multiaddr
	raddr ma.Multiaddr

	closed bool
}

var _ tpt.Conn = &quicConn{}
var _ tpt.MultiStreamConn = &quicConn{}

func newQuicConn(sess quic.Session, t tpt.Transport) (*quicConn, error) {
	// analogues to manet.WrapNetConn
	laddr, err := quicMultiAddress(sess.LocalAddr())
	if err != nil {
		return nil, fmt.Errorf("failed to convert nconn.LocalAddr: %s", err)
	}

	// analogues to manet.WrapNetConn
	raddr, err := quicMultiAddress(sess.RemoteAddr())
	if err != nil {
		return nil, fmt.Errorf("failed to convert nconn.RemoteAddr: %s", err)
	}

	c := &quicConn{
		sess:      sess,
		laddr:     laddr,
		raddr:     raddr,
		transport: t,
	}
	go c.watchClosed()

	return c, nil
}

func (c *quicConn) AcceptStream() (smux.Stream, error) {
	return c.sess.AcceptStream()
}

// OpenStream opens a new stream
// It blocks until a new stream can be opened (when limited by the QUIC maximum stream limit)
func (c *quicConn) OpenStream() (smux.Stream, error) {
	return c.sess.OpenStreamSync()
}

func (c *quicConn) Serve(handler smux.StreamHandler) {
	for { // accept loop
		s, err := c.AcceptStream()
		if err != nil {
			return // err always means closed.
		}
		go handler(s)
	}
}

func (c *quicConn) Close() error {
	return c.sess.Close(nil)
}

func (c *quicConn) watchClosed() {
	c.sess.WaitUntilClosed()
	c.mutex.Lock()
	c.closed = true
	c.mutex.Unlock()
}

func (c *quicConn) IsClosed() bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return c.closed
}

func (c *quicConn) LocalAddr() net.Addr {
	return c.sess.LocalAddr()
}

func (c *quicConn) LocalMultiaddr() ma.Multiaddr {
	return c.laddr
}

func (c *quicConn) RemoteAddr() net.Addr {
	return c.sess.RemoteAddr()
}

func (c *quicConn) RemoteMultiaddr() ma.Multiaddr {
	return c.raddr
}

func (c *quicConn) Transport() tpt.Transport {
	return c.transport
}

// TODO: there must be a better way to do this
func quicMultiAddress(na net.Addr) (ma.Multiaddr, error) {
	udpMA, err := manet.FromNetAddr(na)
	if err != nil {
		return nil, err
	}
	quicMA, err := ma.NewMultiaddr(udpMA.String() + "/quic")
	if err != nil {
		return nil, err
	}
	return quicMA, nil
}
