package libp2pquic

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	mrand "math/rand"
	"net"
	"sync/atomic"
	"time"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	tpt "github.com/libp2p/go-libp2p-core/transport"
	filter "github.com/libp2p/go-maddr-filter"
	quicproxy "github.com/lucas-clemente/quic-go/integrationtests/tools/proxy"
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
			priv, _, err = ic.GenerateRSAKeyPair(2048, rand.Reader)
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

	runServer := func(tr tpt.Transport, multiaddr string) tpt.Listener {
		addr, err := ma.NewMultiaddr(multiaddr)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
		ln, err := tr.Listen(addr)
		ExpectWithOffset(1, err).ToNot(HaveOccurred())
		return ln
	}

	BeforeEach(func() {
		serverID, serverKey = createPeer()
		clientID, clientKey = createPeer()
	})

	It("handshakes on IPv4", func() {
		serverTransport, err := NewTransport(serverKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		ln := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")
		defer ln.Close()

		clientTransport, err := NewTransport(clientKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		conn, err := clientTransport.Dial(context.Background(), ln.Multiaddr(), serverID)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		serverConn, err := ln.Accept()
		Expect(err).ToNot(HaveOccurred())
		defer serverConn.Close()
		Expect(conn.LocalPeer()).To(Equal(clientID))
		Expect(conn.LocalPrivateKey()).To(Equal(clientKey))
		Expect(conn.RemotePeer()).To(Equal(serverID))
		Expect(conn.RemotePublicKey().Equals(serverKey.GetPublic())).To(BeTrue())
		Expect(serverConn.LocalPeer()).To(Equal(serverID))
		Expect(serverConn.LocalPrivateKey()).To(Equal(serverKey))
		Expect(serverConn.RemotePeer()).To(Equal(clientID))
		Expect(serverConn.RemotePublicKey().Equals(clientKey.GetPublic())).To(BeTrue())
	})

	It("handshakes on IPv6", func() {
		serverTransport, err := NewTransport(serverKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		ln := runServer(serverTransport, "/ip6/::1/udp/0/quic")
		defer ln.Close()

		clientTransport, err := NewTransport(clientKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		conn, err := clientTransport.Dial(context.Background(), ln.Multiaddr(), serverID)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		serverConn, err := ln.Accept()
		Expect(err).ToNot(HaveOccurred())
		defer serverConn.Close()
		Expect(conn.LocalPeer()).To(Equal(clientID))
		Expect(conn.LocalPrivateKey()).To(Equal(clientKey))
		Expect(conn.RemotePeer()).To(Equal(serverID))
		Expect(conn.RemotePublicKey().Equals(serverKey.GetPublic())).To(BeTrue())
		Expect(serverConn.LocalPeer()).To(Equal(serverID))
		Expect(serverConn.LocalPrivateKey()).To(Equal(serverKey))
		Expect(serverConn.RemotePeer()).To(Equal(clientID))
		Expect(serverConn.RemotePublicKey().Equals(clientKey.GetPublic())).To(BeTrue())
	})

	It("opens and accepts streams", func() {
		serverTransport, err := NewTransport(serverKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		ln := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")
		defer ln.Close()

		clientTransport, err := NewTransport(clientKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		conn, err := clientTransport.Dial(context.Background(), ln.Multiaddr(), serverID)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		serverConn, err := ln.Accept()
		Expect(err).ToNot(HaveOccurred())
		defer serverConn.Close()

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

		serverTransport, err := NewTransport(serverKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		ln := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")

		clientTransport, err := NewTransport(clientKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		// dial, but expect the wrong peer ID
		_, err = clientTransport.Dial(context.Background(), ln.Multiaddr(), thirdPartyID)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("CRYPTO_ERROR"))

		done := make(chan struct{})
		go func() {
			defer GinkgoRecover()
			defer close(done)
			ln.Accept()
		}()
		Consistently(done).ShouldNot(BeClosed())
		ln.Close()
		Eventually(done).Should(BeClosed())
	})

	It("filters addresses", func() {
		filters := filter.NewFilters()
		ipNet := net.IPNet{
			IP:   net.IPv4(127, 0, 0, 1),
			Mask: net.IPv4Mask(255, 255, 255, 255),
		}
		filters.AddFilter(ipNet, filter.ActionDeny)
		testMA, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/1234/quic")
		Expect(err).ToNot(HaveOccurred())
		Expect(filters.AddrBlocked(testMA)).To(BeTrue())

		serverTransport, err := NewTransport(serverKey, nil, filters)
		Expect(err).ToNot(HaveOccurred())
		ln := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")
		defer ln.Close()

		clientTransport, err := NewTransport(clientKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())

		// make sure that connection attempts fails
		clientTransport.(*transport).config.HandshakeTimeout = 250 * time.Millisecond
		_, err = clientTransport.Dial(context.Background(), ln.Multiaddr(), serverID)
		Expect(err).To(HaveOccurred())
		Expect(err.(net.Error).Timeout()).To(BeTrue())

		// now allow the address and make sure the connection goes through
		clientTransport.(*transport).config.HandshakeTimeout = 2 * time.Second
		filters.AddFilter(ipNet, filter.ActionAccept)
		conn, err := clientTransport.Dial(context.Background(), ln.Multiaddr(), serverID)
		Expect(err).ToNot(HaveOccurred())
		conn.Close()
	})

	It("dials to two servers at the same time", func() {
		serverID2, serverKey2 := createPeer()

		serverTransport, err := NewTransport(serverKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		ln1 := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")
		defer ln1.Close()
		serverTransport2, err := NewTransport(serverKey2, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		ln2 := runServer(serverTransport2, "/ip4/127.0.0.1/udp/0/quic")
		defer ln2.Close()

		data := bytes.Repeat([]byte{'a'}, 5*1<<20) // 5 MB
		// wait for both servers to accept a connection
		// then send some data
		go func() {
			serverConn1, err := ln1.Accept()
			Expect(err).ToNot(HaveOccurred())
			serverConn2, err := ln2.Accept()
			Expect(err).ToNot(HaveOccurred())

			for _, c := range []tpt.CapableConn{serverConn1, serverConn2} {
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

		clientTransport, err := NewTransport(clientKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		c1, err := clientTransport.Dial(context.Background(), ln1.Multiaddr(), serverID)
		Expect(err).ToNot(HaveOccurred())
		defer c1.Close()
		c2, err := clientTransport.Dial(context.Background(), ln2.Multiaddr(), serverID2)
		Expect(err).ToNot(HaveOccurred())
		defer c2.Close()

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
				done <- struct{}{}
			}(c)
		}

		Eventually(done, 5*time.Second).Should(Receive())
		Eventually(done, 5*time.Second).Should(Receive())
	})

	It("sends stateless resets", func() {
		serverTransport, err := NewTransport(serverKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		ln := runServer(serverTransport, "/ip4/127.0.0.1/udp/0/quic")

		var drop uint32
		serverPort := ln.Addr().(*net.UDPAddr).Port
		proxy, err := quicproxy.NewQuicProxy("localhost:0", &quicproxy.Opts{
			RemoteAddr: fmt.Sprintf("localhost:%d", serverPort),
			DropPacket: func(quicproxy.Direction, []byte) bool {
				return atomic.LoadUint32(&drop) > 0
			},
		})
		Expect(err).ToNot(HaveOccurred())
		defer proxy.Close()

		// establish a connection
		clientTransport, err := NewTransport(clientKey, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		proxyAddr, err := toQuicMultiaddr(proxy.LocalAddr())
		Expect(err).ToNot(HaveOccurred())
		conn, err := clientTransport.Dial(context.Background(), proxyAddr, serverID)
		Expect(err).ToNot(HaveOccurred())
		str, err := conn.OpenStream()
		Expect(err).ToNot(HaveOccurred())

		// Stop forwarding packets and close the server.
		// This prevents the CONNECTION_CLOSE from reaching the client.
		atomic.StoreUint32(&drop, 1)
		Expect(ln.Close()).To(Succeed())
		time.Sleep(100 * time.Millisecond) // give the kernel some time to free the UDP port
		ln = runServer(serverTransport, fmt.Sprintf("/ip4/127.0.0.1/udp/%d/quic", serverPort))
		defer ln.Close()
		// Now that the new server is up, re-enable packet forwarding.
		atomic.StoreUint32(&drop, 0)

		// The new server doesn't have any state for the previously established connection.
		// We expect it to send a stateless reset.
		_, rerr := str.Write([]byte("foobar"))
		if rerr == nil {
			_, rerr = str.Read([]byte{0, 0})
		}
		Expect(rerr).To(HaveOccurred())
		Expect(rerr.Error()).To(ContainSubstring("received a stateless reset"))
	})
})
