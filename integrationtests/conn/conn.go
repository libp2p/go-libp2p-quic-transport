// +build !stream_open_no_context

package conn

import (
	"context"

	"github.com/libp2p/go-libp2p-core/mux"
	tpt "github.com/libp2p/go-libp2p-core/transport"
)

func OpenStream(ctx context.Context, c tpt.CapableConn)  (mux.MuxedStream, error) {
	return c.OpenStream(ctx)
}
