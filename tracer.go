package libp2pquic

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/lucas-clemente/quic-go/logging"
	"github.com/lucas-clemente/quic-go/metrics"
	"github.com/lucas-clemente/quic-go/qlog"
)

var tracer logging.Tracer

func init() {
	tracers := []logging.Tracer{metrics.NewTracer()}
	if qlogDir := os.Getenv("QLOGDIR"); len(qlogDir) > 0 {
		if qlogger := initQlogger(qlogDir); qlogger != nil {
			tracers = append(tracers, qlogger)
		}
	}
	tracer = logging.NewMultiplexedTracer(tracers...)
}

func initQlogger(qlogDir string) logging.Tracer {
	return qlog.NewTracer(func(role logging.Perspective, connID []byte) io.WriteCloser {
		// create the QLOGDIR, if it doesn't exist
		if err := os.MkdirAll(qlogDir, 0777); err != nil {
			log.Errorf("creating the QLOGDIR failed: %s", err)
			return nil
		}
		return newQlogger(qlogDir, role, connID)
	})
}

type qlogger struct {
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
	return &qlogger{
		f:           f,
		filename:    finalFilename,
		WriteCloser: newBufferedWriteCloser(bufio.NewWriter(gz), gz),
	}
}

func (l *qlogger) Close() error {
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
