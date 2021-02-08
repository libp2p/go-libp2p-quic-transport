package libp2pquic

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/klauspost/compress/zstd"

	"github.com/lucas-clemente/quic-go/logging"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("qlogger", func() {
	BeforeEach(func() {
		d, err := ioutil.TempDir("", "libp2p-quic-transport-test")
		Expect(err).ToNot(HaveOccurred())
		qlogDir = d
		fmt.Fprintf(GinkgoWriter, "Creating temporary directory: %s\n", qlogDir)
	})

	AfterEach(func() {
		qlogDir = ""
		Expect(os.RemoveAll(qlogDir)).To(Succeed())
	})

	getFile := func() os.FileInfo {
		files, err := ioutil.ReadDir(qlogDir)
		Expect(err).ToNot(HaveOccurred())
		Expect(files).To(HaveLen(1))
		return files[0]
	}

	It("saves a qlog", func() {
		logger := newQlogger(logging.PerspectiveServer, []byte{0xde, 0xad, 0xbe, 0xef})
		file := getFile()
		Expect(string(file.Name()[0])).To(Equal("."))
		Expect(file.Name()).To(HaveSuffix(".qlog.swp"))
		// close the logger. This should move the file.
		Expect(logger.Close()).To(Succeed())
		file = getFile()
		Expect(string(file.Name()[0])).ToNot(Equal("."))
		Expect(file.Name()).To(HaveSuffix(".qlog.zst"))
		Expect(file.Name()).To(And(
			ContainSubstring("server"),
			ContainSubstring("deadbeef"),
		))
	})

	It("buffers", func() {
		logger := newQlogger(logging.PerspectiveServer, []byte("connid"))
		initialSize := getFile().Size()
		// Do a small write.
		// Since the writter is buffered, this should not be written to disk yet.
		logger.Write([]byte("foobar"))
		Expect(getFile().Size()).To(Equal(initialSize))
		// Close the logger. This should flush the buffer to disk.
		Expect(logger.Close()).To(Succeed())
		finalSize := getFile().Size()
		fmt.Fprintf(GinkgoWriter, "initial log file size: %d, final log file size: %d\n", initialSize, finalSize)
		Expect(finalSize).To(BeNumerically(">", initialSize))
	})

	It("compresses", func() {
		logger := newQlogger(logging.PerspectiveServer, []byte("connid"))
		logger.Write([]byte("foobar"))
		Expect(logger.Close()).To(Succeed())
		compressed, err := ioutil.ReadFile(qlogDir + "/" + getFile().Name())
		Expect(err).ToNot(HaveOccurred())
		Expect(compressed).ToNot(Equal("foobar"))
		c, err := zstd.NewReader(bytes.NewReader(compressed))
		Expect(err).ToNot(HaveOccurred())
		data, err := ioutil.ReadAll(c)
		Expect(err).ToNot(HaveOccurred())
		Expect(data).To(Equal([]byte("foobar")))
	})
})
