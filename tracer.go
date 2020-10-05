package libp2pquic

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/francoispqt/gojay"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/lucas-clemente/quic-go/logging"
	"github.com/lucas-clemente/quic-go/metrics"
	"github.com/lucas-clemente/quic-go/qlog"
)

var metricsTracer logging.Tracer
var qlogDir string

func init() {
	metricsTracer = metrics.NewTracer()
	qlogDir = os.Getenv("QLOGDIR")
}

func getTracer(qlogCallback func(qlog.ConnectionTracer)) logging.Tracer {
	var tracers []logging.Tracer
	if qlogger := maybeGetQlogger(qlogCallback); qlogger != nil {
		tracers = append(tracers, qlogger)
	}
	return logging.NewMultiplexedTracer(tracers...)
}

func maybeGetQlogger(qlogCallback func(qlog.ConnectionTracer)) logging.Tracer {
	if len(qlogDir) == 0 {
		return nil
	}
	return initQlogger(qlogDir, qlogCallback)
}

type qlogTracer struct {
	logging.Tracer

	cb func(qlog.ConnectionTracer)
}

func (t *qlogTracer) TracerForConnection(p logging.Perspective, odcid logging.ConnectionID) logging.ConnectionTracer {
	ct := t.Tracer.TracerForConnection(p, odcid)
	if ct != nil {
		t.cb(ct.(qlog.ConnectionTracer))
	}
	return ct
}

func initQlogger(qlogDir string, qlogCallback func(qlog.ConnectionTracer)) logging.Tracer {
	tracer := qlog.NewTracer(func(role logging.Perspective, connID []byte) io.WriteCloser {
		// create the QLOGDIR, if it doesn't exist
		if err := os.MkdirAll(qlogDir, 0777); err != nil {
			log.Errorf("creating the QLOGDIR failed: %s", err)
			return nil
		}
		return newQlogger(qlogDir, role, connID)
	})
	if tracer != nil {
		tracer = &qlogTracer{Tracer: tracer, cb: qlogCallback}
	}
	return tracer
}

type connectionStartEvent struct {
	local, remote peer.ID
}

var _ qlog.ExternalEvent = &connectionStartEvent{}

func (e *connectionStartEvent) Name() string     { return "connection_start" }
func (e *connectionStartEvent) Category() string { return "libp2p" }
func (e *connectionStartEvent) IsNil() bool      { return e == nil }
func (e *connectionStartEvent) MarshalJSONObject(enc *gojay.Encoder) {
	fmt.Println("Marshal", e.local, len(e.local))
	if len(e.local) > 0 {
		enc.StringKey("local", e.local.Pretty())
	}
	if len(e.remote) > 0 {
		enc.StringKey("remote", e.remote.Pretty())
	}
}

type qlogExporter struct {
	f        *os.File // QLOGDIR/.log_xxx.qlog.gz.swp
	filename string   // QLOGDIR/log_xxx.qlog.gz
	io.WriteCloser
}

func newQlogger(qlogDir string, role logging.Perspective, connID []byte) io.WriteCloser {
	t := time.Now().UTC().Format("2006-01-02T15-04-05.999999999UTC")
	r := "server"
	if role == logging.PerspectiveClient {
		r = "client"
	}
	finalFilename := fmt.Sprintf("%s%clog_%s_%s_%x.qlog.gz", qlogDir, os.PathSeparator, t, r, connID)
	filename := fmt.Sprintf("%s%c.log_%s_%s_%x.qlog.gz.swp", qlogDir, os.PathSeparator, t, r, connID)
	f, err := os.Create(filename)
	if err != nil {
		log.Errorf("unable to create qlog file %s: %s", filename, err)
		return nil
	}
	gz := gzip.NewWriter(f)
	return &qlogExporter{
		f:           f,
		filename:    finalFilename,
		WriteCloser: newBufferedWriteCloser(bufio.NewWriter(gz), gz),
	}
}

func (l *qlogExporter) Close() error {
	if err := l.WriteCloser.Close(); err != nil {
		return err
	}
	path := l.f.Name()
	if err := l.f.Close(); err != nil {
		return err
	}
	return os.Rename(path, l.filename)
}

type bufferedWriteCloser struct {
	*bufio.Writer
	io.Closer
}

func newBufferedWriteCloser(writer *bufio.Writer, closer io.Closer) io.WriteCloser {
	return &bufferedWriteCloser{
		Writer: writer,
		Closer: closer,
	}
}

func (h bufferedWriteCloser) Close() error {
	if err := h.Writer.Flush(); err != nil {
		return err
	}
	return h.Closer.Close()
}
