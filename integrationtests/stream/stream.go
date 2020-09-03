package stream

import (
	"io"
	"time"

	"github.com/libp2p/go-libp2p-core/mux"
)

type Stream interface {
	io.Reader
	io.Writer
	io.Closer

	CloseWrite() error
	CloseRead() error
	Reset() error

	SetDeadline(time.Time) error
	SetReadDeadline(time.Time) error
	SetWriteDeadline(time.Time) error
}

type stream struct {
	mux.MuxedStream
}

func WrapStream(str mux.MuxedStream) *stream {
	return &stream{MuxedStream: str}
}
