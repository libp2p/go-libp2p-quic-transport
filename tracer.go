package libp2pquic

import (
	"context"
	"net"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/lucas-clemente/quic-go/logging"
	"github.com/lucas-clemente/quic-go/qlog"
)

type tracer struct {
	node peer.ID
}

var _ logging.Tracer = &tracer{}

func newTracer(node peer.ID) logging.Tracer {
	return logging.NewMultiplexedTracer(&metricsTracer{}, &tracer{node: node})
}

func (t *tracer) SentPacket(net.Addr, *logging.Header, logging.ByteCount, []logging.Frame) {}
func (t *tracer) DroppedPacket(net.Addr, logging.PacketType, logging.ByteCount, logging.PacketDropReason) {
}
func (t *tracer) TracerForConnection(ctx context.Context, p logging.Perspective, odcid logging.ConnectionID) logging.ConnectionTracer {
	var tracers []logging.ConnectionTracer
	if qlogWriter := newQlogger(p, odcid); qlogWriter != nil {
		if q := qlog.NewConnectionTracer(qlogWriter, p, odcid); q != nil {
			tracers = append(tracers, q)
		}
	}
	if m := newStatsConnectionTracer(ctx, p, odcid, t.node); m != nil {
		tracers = append(tracers, m)
	}
	if len(tracers) == 0 {
		return nil
	}
	return logging.NewMultiplexedConnectionTracer(tracers...)
}
