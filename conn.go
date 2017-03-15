package libp2pquic

import (
	"net"
	"time"

	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"
)

type conn struct {
	quicConn  net.Conn
	transport tpt.Transport
}

func (c *conn) Read(p []byte) (int, error) {
	return c.quicConn.Read(p)
}

func (c *conn) Write(p []byte) (int, error) {
	return c.quicConn.Write(p)
}

func (c *conn) Close() error {
	return c.quicConn.Close()
}

func (c *conn) LocalAddr() net.Addr {
	return c.quicConn.LocalAddr()
}

func (c *conn) RemoteAddr() net.Addr {
	return c.quicConn.RemoteAddr()
}

func (c *conn) LocalMultiaddr() ma.Multiaddr {
	panic("not implemented")
}

func (c *conn) RemoteMultiaddr() ma.Multiaddr {
	panic("not implemented")
}

func (c *conn) Transport() tpt.Transport {
	return c.transport
}

func (c *conn) SetDeadline(t time.Time) error {
	return nil
}

func (c *conn) SetReadDeadline(t time.Time) error {
	return nil
}

func (c *conn) SetWriteDeadline(t time.Time) error {
	return nil
}

var _ tpt.Conn = &conn{}
