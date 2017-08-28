package libp2pquic

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"math/big"
	"net"

	tpt "github.com/libp2p/go-libp2p-transport"
	quic "github.com/lucas-clemente/quic-go"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

type listener struct {
	laddr        ma.Multiaddr
	quicListener quic.Listener

	transport tpt.Transport
}

var _ tpt.Listener = &listener{}

func newListener(laddr ma.Multiaddr, t tpt.Transport) (*listener, error) {
	_, host, err := manet.DialArgs(laddr)
	if err != nil {
		return nil, err
	}
	tlsConf, err := generateTLSConfig()
	if err != nil {
		return nil, err
	}
	qln, err := quic.ListenAddr(host, tlsConf, nil)
	if err != nil {
		return nil, err
	}
	addr, err := quicMultiAddress(qln.Addr())
	if err != nil {
		return nil, err
	}

	return &listener{
		laddr:        addr,
		quicListener: qln,
		transport:    t,
	}, nil
}

func (l *listener) Accept() (tpt.Conn, error) {
	sess, err := l.quicListener.Accept()
	if err != nil {
		return nil, err
	}
	return newQuicConn(sess, l.transport)
}

func (l *listener) Close() error {
	return l.quicListener.Close()
}

func (l *listener) Addr() net.Addr {
	return l.quicListener.Addr()
}

func (l *listener) Multiaddr() ma.Multiaddr {
	return l.laddr
}

// Generate a bare-bones TLS config for the server.
// The client doesn't verify the certificate yet.
func generateTLSConfig() (*tls.Config, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{tlsCert}}, nil
}
