package libp2pquic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	tpt "github.com/libp2p/go-libp2p-core/transport"
	quic "github.com/lucas-clemente/quic-go"
	ma "github.com/multiformats/go-multiaddr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Transport", func() {
	var t tpt.Transport

	BeforeEach(func() {
		rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
		Expect(err).ToNot(HaveOccurred())
		key, err := ic.UnmarshalRsaPrivateKey(x509.MarshalPKCS1PrivateKey(rsaKey))
		Expect(err).ToNot(HaveOccurred())
		t, err = NewTransport(key, nil, nil)
		Expect(err).ToNot(HaveOccurred())
	})

	It("says if it can dial an address", func() {
		invalidAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234")
		Expect(err).ToNot(HaveOccurred())
		validAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234/quic")
		Expect(err).ToNot(HaveOccurred())
		Expect(t.CanDial(invalidAddr)).To(BeFalse())
		Expect(t.CanDial(validAddr)).To(BeTrue())
	})

	It("says that it cannot dial /dns addresses", func() {
		addr, err := ma.NewMultiaddr("/dns/google.com/udp/443/quic")
		Expect(err).ToNot(HaveOccurred())
		Expect(t.CanDial(addr)).To(BeFalse())
	})

	It("supports the QUIC protocol", func() {
		protocols := t.Protocols()
		Expect(protocols).To(HaveLen(1))
		Expect(protocols[0]).To(Equal(ma.P_QUIC))
	})

	It("uses a conn that can interface assert to a UDPConn for dialing", func() {
		origQuicDialContext := quicDialContext
		defer func() { quicDialContext = origQuicDialContext }()

		var conn net.PacketConn
		quicDialContext = func(_ context.Context, c net.PacketConn, _ net.Addr, _ string, _ *tls.Config, _ *quic.Config) (quic.Session, error) {
			conn = c
			return nil, errors.New("listen error")
		}
		remoteAddr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/0/quic")
		Expect(err).ToNot(HaveOccurred())
		_, err = t.Dial(context.Background(), remoteAddr, "remote peer id")
		Expect(err).To(MatchError("listen error"))
		Expect(conn).ToNot(BeNil())
		defer conn.Close()
		_, ok := conn.(udpConn)
		Expect(ok).To(BeTrue())
	})
})
