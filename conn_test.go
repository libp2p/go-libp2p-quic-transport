package libp2pquic

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

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

	runServer := func(tr tpt.Transport) (ma.Multiaddr, <-chan tpt.Conn) {
		addrChan := make(chan ma.Multiaddr)
		connChan := make(chan tpt.Conn)
		go func() {
			defer GinkgoRecover()
			addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/0/quic")
			Expect(err).ToNot(HaveOccurred())
			ln, err := tr.Listen(addr)
			Expect(err).ToNot(HaveOccurred())
			addrChan <- ln.Multiaddr()
			conn, err := ln.Accept()
			Expect(err).ToNot(HaveOccurred())
			connChan <- conn
		}()
		return <-addrChan, connChan
	}

	// modify the cert chain such that verificiation will fail
	invalidateCertChain := func(tlsConf *tls.Config) {
		tlsConf.Certificates[0].Certificate = [][]byte{tlsConf.Certificates[0].Certificate[0]}
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
		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport)

		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
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

	It("opens and accepts streams", func() {
		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport)

		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		conn, err := clientTransport.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		serverConn := <-serverConnChan

		str, err := conn.OpenStream()
		Expect(err).ToNot(HaveOccurred())
		_, err = str.Write([]byte("foobar"))
		Expect(err).ToNot(HaveOccurred())
		str.Close()
		sstr, err := serverConn.AcceptStream()
		Expect(err).ToNot(HaveOccurred())
		data, err := ioutil.ReadAll(sstr)
		Expect(err).ToNot(HaveOccurred())
		Expect(data).To(Equal([]byte("foobar")))
	})

	It("fails if the peer ID doesn't match", func() {
		thirdPartyID, err := peer.IDFromPrivateKey(createPeer())
		Expect(err).ToNot(HaveOccurred())

		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport)

		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		// dial, but expect the wrong peer ID
		_, err = clientTransport.Dial(context.Background(), serverAddr, thirdPartyID)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("TLS handshake error: bad certificate"))
		Consistently(serverConnChan).ShouldNot(Receive())
	})

	It("fails if the client presents an invalid cert chain", func() {
		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport)

		clientTransport, err := NewTransport(clientKey)
		invalidateCertChain(clientTransport.(*transport).tlsConf)
		Expect(err).ToNot(HaveOccurred())
		conn, err := clientTransport.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		Eventually(func() bool { return conn.IsClosed() }).Should(BeTrue())
		Consistently(serverConnChan).ShouldNot(Receive())
	})

	It("fails if the server presents an invalid cert chain", func() {
		serverTransport, err := NewTransport(serverKey)
		invalidateCertChain(serverTransport.(*transport).tlsConf)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport)

		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		_, err = clientTransport.Dial(context.Background(), serverAddr, serverID)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("TLS handshake error: bad certificate"))
		Consistently(serverConnChan).ShouldNot(Receive())
	})

	It("keeps accepting connections after a failed connection attempt", func() {
		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport)

		// first dial with an invalid cert chain
		clientTransport1, err := NewTransport(clientKey)
		invalidateCertChain(clientTransport1.(*transport).tlsConf)
		Expect(err).ToNot(HaveOccurred())
		_, err = clientTransport1.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		Consistently(serverConnChan).ShouldNot(Receive())

		// then dial with a valid client
		clientTransport2, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		_, err = clientTransport2.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		Eventually(serverConnChan).Should(Receive())
	})
})
