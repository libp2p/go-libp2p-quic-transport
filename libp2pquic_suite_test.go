package libp2pquic

import (
	"bytes"
	mrand "math/rand"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/lucas-clemente/quic-go"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestLibp2pQuicTransport(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "libp2p QUIC Transport Suite")
}

var _ = BeforeSuite(func() {
	mrand.Seed(GinkgoRandomSeed())
})

var (
	garbageCollectIntervalOrig time.Duration
	maxUnusedDurationOrig      time.Duration
	origQuicConfig             *quic.Config
	mockCtrl                   *gomock.Controller
)

func isGarbageCollectorRunning() bool {
	var b bytes.Buffer
	pprof.Lookup("goroutine").WriteTo(&b, 1)
	return strings.Contains(b.String(), "go-libp2p-quic-transport.(*reuse).runGarbageCollector")
}

var _ = BeforeEach(func() {
	mockCtrl = gomock.NewController(GinkgoT())

	Expect(isGarbageCollectorRunning()).To(BeFalse())
	garbageCollectIntervalOrig = garbageCollectInterval
	maxUnusedDurationOrig = maxUnusedDuration
	garbageCollectInterval = 50 * time.Millisecond
	maxUnusedDuration = 0
	origQuicConfig = quicConfig
	quicConfig = quicConfig.Clone()
})

var _ = AfterEach(func() {
	mockCtrl.Finish()

	Eventually(isGarbageCollectorRunning).Should(BeFalse())
	garbageCollectInterval = garbageCollectIntervalOrig
	maxUnusedDuration = maxUnusedDurationOrig
	quicConfig = origQuicConfig
})
