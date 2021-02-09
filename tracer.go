package libp2pquic

import (
	"crypto/rand"
	"encoding/binary"
	"math"
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
func (t *tracer) TracerForConnection(p logging.Perspective, odcid logging.ConnectionID) logging.ConnectionTracer {
	var (
		qlogPath string
		tracers  []logging.ConnectionTracer
	)

	if t.shouldRecordQlog() {
		qlogWriter := newQlogger(p, odcid)
		if qlogWriter != nil {
			qlogPath = qlogWriter.GetPath()
			if q := qlog.NewConnectionTracer(qlogWriter, p, odcid); q != nil {
				tracers = append(tracers, q)
			}
		}
	}
	if m := newStatsConnectionTracer(p, odcid, t.node, qlogPath); m != nil {
		tracers = append(tracers, m)
	}
	if len(tracers) == 0 {
		return nil
	}
	return logging.NewMultiplexedConnectionTracer(tracers...)
}

// We only enable qlog on a fraction (50%) of the connections.
func (t *tracer) shouldRecordQlog() bool {
	b := make([]byte, 2)
	rand.Read(b)
	return binary.BigEndian.Uint16(b) > math.MaxUint16/2
}
