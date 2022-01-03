package libp2pquic

import (
	mrand "math/rand"
	"testing"
	"time"

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
)

var _ = BeforeEach(func() {
	garbageCollectIntervalOrig = garbageCollectInterval
	maxUnusedDurationOrig = maxUnusedDuration
	garbageCollectInterval = 50 * time.Millisecond
	maxUnusedDuration = 0
	origQuicConfig = quicConfig
	quicConfig = quicConfig.Clone()
})

var _ = AfterEach(func() {
	garbageCollectInterval = garbageCollectIntervalOrig
	maxUnusedDuration = maxUnusedDurationOrig
	quicConfig = origQuicConfig
})
