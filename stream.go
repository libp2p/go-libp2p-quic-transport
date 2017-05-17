package libp2pquic

import (
	"time"

	smux "github.com/jbenet/go-stream-muxer"
	quic "github.com/lucas-clemente/quic-go"
)

// The quicStream is a very thin wrapper for a quic.Stream, adding some methods
// required to fulfill the smux.Stream interface
// TODO: this can be removed once the quic.Stream supports deadlines (quic-go#514)
type quicStream struct {
	quic.Stream
}

var _ smux.Stream = &quicStream{}

func (s *quicStream) SetDeadline(time.Time) error {
	return nil
}

func (s *quicStream) SetReadDeadline(time.Time) error {
	return nil
}

func (s *quicStream) SetWriteDeadline(time.Time) error {
	return nil
}
