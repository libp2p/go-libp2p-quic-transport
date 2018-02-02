package libp2pquic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"

	ic "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	tpt "github.com/libp2p/go-libp2p-transport"
	ma "github.com/multiformats/go-multiaddr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Connection", func() {
	var (
		serverKey, clientKey ic.PrivKey
		serverID, clientID   peer.ID
	)

	createPeer := func() ic.PrivKey {
		key, err := rsa.GenerateKey(rand.Reader, 1024)
		Expect(err).ToNot(HaveOccurred())
		priv, err := ic.UnmarshalRsaPrivateKey(x509.MarshalPKCS1PrivateKey(key))
		Expect(err).ToNot(HaveOccurred())
		return priv
	}

	runServer := func() (<-chan ma.Multiaddr, <-chan tpt.Conn) {
		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		addrChan := make(chan ma.Multiaddr)
		connChan := make(chan tpt.Conn)
		go func() {
			defer GinkgoRecover()
			addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/0/quic")
			Expect(err).ToNot(HaveOccurred())
			ln, err := serverTransport.Listen(addr)
			Expect(err).ToNot(HaveOccurred())
			addrChan <- ln.Multiaddr()
			conn, err := ln.Accept()
			Expect(err).ToNot(HaveOccurred())
			connChan <- conn
		}()
		return addrChan, connChan
	}

	BeforeEach(func() {
		var err error
		serverKey = createPeer()
		serverID, err = peer.IDFromPrivateKey(serverKey)
		Expect(err).ToNot(HaveOccurred())
		clientKey = createPeer()
		clientID, err = peer.IDFromPrivateKey(clientKey)
		Expect(err).ToNot(HaveOccurred())
	})

	It("handshakes", func() {
		serverAddrChan, serverConnChan := runServer()
		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr := <-serverAddrChan
		conn, err := clientTransport.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		serverConn := <-serverConnChan
		Expect(conn.LocalPeer()).To(Equal(clientID))
		Expect(conn.LocalPrivateKey()).To(Equal(clientKey))
		Expect(conn.RemotePeer()).To(Equal(serverID))
		Expect(conn.RemotePublicKey()).To(Equal(serverKey.GetPublic()))
		Expect(serverConn.LocalPeer()).To(Equal(serverID))
		Expect(serverConn.LocalPrivateKey()).To(Equal(serverKey))
		Expect(serverConn.RemotePeer()).To(Equal(clientID))
		Expect(serverConn.RemotePublicKey()).To(Equal(clientKey.GetPublic()))
	})

	It("fails if the peer ID doesn't match", func() {
		thirdPartyID, err := peer.IDFromPrivateKey(createPeer())
		Expect(err).ToNot(HaveOccurred())

		serverAddrChan, serverConnChan := runServer()
		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr := <-serverAddrChan
		// dial, but expect the wrong peer ID
		_, err = clientTransport.Dial(context.Background(), serverAddr, thirdPartyID)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("TLS handshake error: bad certificate"))
		Consistently(serverConnChan).ShouldNot(Receive())
	})
})
