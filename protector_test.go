package libp2pquic

import (
	"context"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	mrand "math/rand"
	"net"
	"time"

	ic "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	tpt "github.com/libp2p/go-libp2p-core/transport"
	ma "github.com/multiformats/go-multiaddr"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type proxy struct {
	remoteAddr net.Addr
	localAddr  net.Addr
	conn       *net.UDPConn
	callback   func([]byte) []byte
}

func newProxy(remoteAddr net.Addr, callback func([]byte) []byte) *proxy {
	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	Expect(err).ToNot(HaveOccurred())
	conn, err := net.ListenUDP("udp", addr)
	Expect(err).ToNot(HaveOccurred())
	p := &proxy{
		remoteAddr: remoteAddr,
		conn:       conn,
		callback:   callback,
	}
	go func() {
		defer GinkgoRecover()
		p.run()
	}()
	return p
}

func (p *proxy) run() {
	b := make([]byte, 2000)
	for {
		b = b[:2000]
		n, addr, err := p.conn.ReadFrom(b)
		if err != nil {
			return
		}
		b = p.callback(b[:n])
		if p.localAddr == nil { // first packet from client
			p.localAddr = addr
		}
		if addr.String() == p.localAddr.String() {
			p.conn.WriteTo(b, p.remoteAddr)
		} else if addr.String() == p.remoteAddr.String() {
			p.conn.WriteTo(b, p.localAddr)
		} else {
			Fail(fmt.Sprintf("unexpected address: %s (local: %s, remote %s)", addr, p.localAddr, p.remoteAddr))
		}
	}
}

func (p *proxy) Multiaddr() ma.Multiaddr {
	addr := p.conn.LocalAddr().(*net.UDPAddr)
	maddr, err := ma.NewMultiaddr(fmt.Sprintf("/ip4/%s/udp/%d/quic", addr.IP.String(), addr.Port))
	Expect(err).ToNot(HaveOccurred())
	return maddr
}

var _ = Describe("Protector", func() {
	var psk []byte

	BeforeEach(func() {
		psk = make([]byte, 32)
		rand.Read(psk)
		fmt.Fprintf(GinkgoWriter, "PSK: %#x\n", psk)
	})

	runServer := func() (tpt.Listener, peer.ID) {
		priv, _, err := ic.GenerateRSAKeyPair(2048, rand.Reader)
		Expect(err).ToNot(HaveOccurred())
		addr, err := ma.NewMultiaddr("/ip4/127.0.0.1/udp/0/quic")
		Expect(err).ToNot(HaveOccurred())
		tr, err := NewTransport(priv, psk)
		Expect(err).ToNot(HaveOccurred())
		ln, err := tr.Listen(addr)
		Expect(err).ToNot(HaveOccurred())

		go func() {
			for {
				defer GinkgoRecover()
				conn, err := ln.Accept()
				if err != nil {
					return
				}
				str, err := conn.OpenStream()
				Expect(err).ToNot(HaveOccurred())
				_, err = str.Write([]byte("foobar"))
				Expect(err).ToNot(HaveOccurred())
				Expect(str.Close()).To(Succeed())
			}
		}()

		id, err := peer.IDFromPrivateKey(priv)
		Expect(err).ToNot(HaveOccurred())
		return ln, id
	}

	It("handshakes", func() {
		ln, serverID := runServer()
		defer ln.Close()

		priv, _, err := ic.GenerateECDSAKeyPair(rand.Reader)
		Expect(err).ToNot(HaveOccurred())
		tr, err := NewTransport(priv, psk)
		Expect(err).ToNot(HaveOccurred())

		conn, err := tr.Dial(context.Background(), ln.Multiaddr(), serverID)
		Expect(err).ToNot(HaveOccurred())
		defer conn.Close()
		str, err := conn.AcceptStream()
		Expect(err).ToNot(HaveOccurred())
		defer str.Close()
		data, err := ioutil.ReadAll(str)
		Expect(err).ToNot(HaveOccurred())
		Expect(data).To(Equal([]byte("foobar")))
	})

	It("fails the handshake with mismatching PSKs", func() {
		quicConfigOrig := quicConfig.Clone()
		defer func() {
			quicConfig = quicConfigOrig
		}()

		quicConfig.HandshakeTimeout = 500 * time.Millisecond
		ln, serverID := runServer()
		defer ln.Close()

		psk2 := make([]byte, 32)
		rand.Read(psk2)
		priv, _, err := ic.GenerateECDSAKeyPair(rand.Reader)
		Expect(err).ToNot(HaveOccurred())
		tr, err := NewTransport(priv, psk2)
		Expect(err).ToNot(HaveOccurred())

		_, err = tr.Dial(context.Background(), ln.Multiaddr(), serverID)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Handshake did not complete in time"))
	})

	It("handshakes with cut packets", func() {
		ln, serverID := runServer()
		defer ln.Close()

		proxy := newProxy(ln.Addr(), func(p []byte) []byte {
			if mrand.Int()%5 != 0 {
				return p
			}
			return p[:1+mrand.Intn(len(p)-2)]
		})
		priv, _, err := ic.GenerateECDSAKeyPair(rand.Reader)
		Expect(err).ToNot(HaveOccurred())
		tr, err := NewTransport(priv, psk)
		Expect(err).ToNot(HaveOccurred())

		conn, err := tr.Dial(context.Background(), proxy.Multiaddr(), serverID)
		Expect(err).ToNot(HaveOccurred())
		str, err := conn.AcceptStream()
		Expect(err).ToNot(HaveOccurred())
		data, err := ioutil.ReadAll(str)
		Expect(err).ToNot(HaveOccurred())
		Expect(data).To(Equal([]byte("foobar")))
		Expect(conn.Close()).To(Succeed())
	})

	It("handshakes with corrupted packets", func() {
		ln, serverID := runServer()
		defer ln.Close()

		proxy := newProxy(ln.Addr(), func(p []byte) []byte {
			if mrand.Int()%5 != 0 {
				return p
			}
			p[mrand.Intn(len(p))] ^= 0xff // invert one byte
			return p
		})
		priv, _, err := ic.GenerateECDSAKeyPair(rand.Reader)
		Expect(err).ToNot(HaveOccurred())
		tr, err := NewTransport(priv, psk)
		Expect(err).ToNot(HaveOccurred())

		for i := 0; i < 20; i++ {
			conn, err := tr.Dial(context.Background(), proxy.Multiaddr(), serverID)
			Expect(err).ToNot(HaveOccurred())
			str, err := conn.AcceptStream()
			Expect(err).ToNot(HaveOccurred())
			data, err := ioutil.ReadAll(str)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(Equal([]byte("foobar")))
			Expect(conn.Close()).To(Succeed())
		}
	})
})
