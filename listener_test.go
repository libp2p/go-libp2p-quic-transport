package libp2pquic

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"net"

	ic "github.com/libp2p/go-libp2p-crypto"
	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Listener", func() {
	var (
		t         tpt.Transport
		localAddr ma.Multiaddr
	)

	BeforeEach(func() {
		rsaKey, err := rsa.GenerateKey(rand.Reader, 1024)
		Expect(err).ToNot(HaveOccurred())
		key, err := ic.UnmarshalRsaPrivateKey(x509.MarshalPKCS1PrivateKey(rsaKey))
		Expect(err).ToNot(HaveOccurred())
		t, err = NewTransport(key)
		Expect(err).ToNot(HaveOccurred())
		localAddr, err = ma.NewMultiaddr("/ip4/127.0.0.1/udp/0/quic")
		Expect(err).ToNot(HaveOccurred())
	})

	It("returns the address it is listening on", func() {
		ln, err := t.Listen(localAddr)
		Expect(err).ToNot(HaveOccurred())
		netAddr := ln.Addr()
		Expect(netAddr).To(BeAssignableToTypeOf(&net.UDPAddr{}))
		port := netAddr.(*net.UDPAddr).Port
		Expect(port).ToNot(BeZero())
		Expect(ln.Multiaddr().String()).To(Equal(fmt.Sprintf("/ip4/127.0.0.1/udp/%d/quic", port)))
	})

	It("returns Accept when it is closed", func() {
		addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/0/quic")
		Expect(err).ToNot(HaveOccurred())
		ln, err := t.Listen(addr)
		Expect(err).ToNot(HaveOccurred())
		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			ln.Accept()
			close(done)
		}()
		Consistently(done).ShouldNot(BeClosed())
		Expect(ln.Close()).To(Succeed())
		Eventually(done).Should(BeClosed())
	})

	It("doesn't accept Accept calls after it is closed", func() {
		ln, err := t.Listen(localAddr)
		Expect(err).ToNot(HaveOccurred())
		Expect(ln.Close()).To(Succeed())
		_, err = ln.Accept()
		Expect(err).To(HaveOccurred())
	})
})
