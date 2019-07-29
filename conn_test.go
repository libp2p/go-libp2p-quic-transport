package libp2pquic

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	mrand "math/rand"
	"time"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	tpt "github.com/libp2p/go-libp2p-core/transport"
	ma "github.com/multiformats/go-multiaddr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Connection", func() {
	var (
		serverKey, clientKey ic.PrivKey
		serverID, clientID   peer.ID
	)

	createPeer := func() (peer.ID, ic.PrivKey) {
		var priv ic.PrivKey
		var err error
		switch mrand.Int() % 4 {
		case 0:
			fmt.Fprintf(GinkgoWriter, " using an ECDSA key: ")
			priv, _, err = ic.GenerateECDSAKeyPair(rand.Reader)
		case 1:
			fmt.Fprintf(GinkgoWriter, " using an RSA key: ")
			priv, _, err = ic.GenerateRSAKeyPair(1024, rand.Reader)
		case 2:
			fmt.Fprintf(GinkgoWriter, " using an Ed25519 key: ")
			priv, _, err = ic.GenerateEd25519Key(rand.Reader)
		case 3:
			fmt.Fprintf(GinkgoWriter, " using an secp256k1 key: ")
			priv, _, err = ic.GenerateSecp256k1Key(rand.Reader)
		}
		Expect(err).ToNot(HaveOccurred())
		id, err := peer.IDFromPrivateKey(priv)
		Expect(err).ToNot(HaveOccurred())
		fmt.Fprintln(GinkgoWriter, id.Pretty())
		return id, priv
	}

	runServer := func(tr tpt.Transport, multiaddr string) (ma.Multiaddr, <-chan tpt.CapableConn) {
		addrChan := make(chan ma.Multiaddr)
		connChan := make(chan tpt.CapableConn)
		go func() {
			defer GinkgoRecover()
			addr, err := ma.NewMultiaddr(multiaddr)
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

	BeforeEach(func() {
		serverID, serverKey = createPeer()
		clientID, clientKey = createPeer()
	})

	It("handshakes on IPv4", func() {
		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")

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

	It("handshakes on IPv6", func() {
		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport, "/ip6/::1/udp/0/quic")

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
		serverAddr, serverConnChan := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")

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
		thirdPartyID, _ := createPeer()

		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")

		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		// dial, but expect the wrong peer ID
		_, err = clientTransport.Dial(context.Background(), serverAddr, thirdPartyID)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("CRYPTO_ERROR"))
		Consistently(serverConnChan).ShouldNot(Receive())
	})

	It("dials to two servers at the same time", func() {
		serverID2, serverKey2 := createPeer()

		serverTransport, err := NewTransport(serverKey)
		Expect(err).ToNot(HaveOccurred())
		serverAddr, serverConnChan := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")
		serverTransport2, err := NewTransport(serverKey2)
		Expect(err).ToNot(HaveOccurred())
		serverAddr2, serverConnChan2 := runServer(serverTransport2, "/ip4/127.0.0.1/udp/0/quic")

		data := bytes.Repeat([]byte{'a'}, 5*1<<20) // 5 MB
		// wait for both servers to accept a connection
		// then send some data
		go func() {
			for _, c := range []tpt.CapableConn{<-serverConnChan, <-serverConnChan2} {
				go func(conn tpt.CapableConn) {
					defer GinkgoRecover()
					str, err := conn.OpenStream()
					Expect(err).ToNot(HaveOccurred())
					defer str.Close()
					_, err = str.Write(data)
					Expect(err).ToNot(HaveOccurred())
				}(c)
			}
		}()

		clientTransport, err := NewTransport(clientKey)
		Expect(err).ToNot(HaveOccurred())
		c1, err := clientTransport.Dial(context.Background(), serverAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		c2, err := clientTransport.Dial(context.Background(), serverAddr2, serverID2)
		Expect(err).ToNot(HaveOccurred())

		done := make(chan struct{}, 2)
		// receive the data on both connections at the same time
		for _, c := range []tpt.CapableConn{c1, c2} {
			go func(conn tpt.CapableConn) {
				defer GinkgoRecover()
				str, err := conn.AcceptStream()
				Expect(err).ToNot(HaveOccurred())
				str.Close()
				d, err := ioutil.ReadAll(str)
				Expect(err).ToNot(HaveOccurred())
				Expect(d).To(Equal(data))
				conn.Close()
				done <- struct{}{}
			}(c)
		}

		Eventually(done, 5*time.Second).Should(Receive())
		Eventually(done, 5*time.Second).Should(Receive())
	})
})
