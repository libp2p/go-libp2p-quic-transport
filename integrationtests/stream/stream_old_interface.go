// +build old_stream_close

package stream

import (
	"log"

	"github.com/lucas-clemente/quic-go"
)

func init() {
	log.Println("Using old stream interface wrapper.")
}

const reset quic.ErrorCode = 0

func (s *stream) CloseWrite() error {
	return s.MuxedStream.Close()
}

func (s *stream) CloseRead() error {
	s.MuxedStream.(quic.Stream).CancelRead(reset)
	return nil
}

func (s *stream) Close() error {
	s.MuxedStream.(quic.Stream).CancelRead(reset)
	return s.MuxedStream.Close()
}
