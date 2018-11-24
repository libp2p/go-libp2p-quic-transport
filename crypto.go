package libp2pquic

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"math/big"
	"time"

	"github.com/gogo/protobuf/proto"
	ic "github.com/libp2p/go-libp2p-crypto"
	pb "github.com/libp2p/go-libp2p-crypto/pb"
	peer "github.com/libp2p/go-libp2p-peer"
)

// mint certificate selection is broken.
const hostname = "quic.ipfs"

// Identity is used to secure connections
type Identity struct {
	*tls.Config
}

// NewIdentity creates a new identity
func NewIdentity(privKey ic.PrivKey) (*Identity, error) {
	conf, err := generateConfig(privKey)
	if err != nil {
		return nil, err
	}
	return &Identity{conf}, nil
}

// ConfigForPeer creates a new tls.Config that verifies the peers certificate chain.
// It should be used to create a new tls.Config before dialing.
func (i *Identity) ConfigForPeer(remote peer.ID) *tls.Config {
	// We need to check the peer ID in the VerifyPeerCertificate callback.
	// The tls.Config it is also used for listening, and we might also have concurrent dials.
	// Clone it so we can check for the specific peer ID we're dialing here.
	conf := i.Config.Clone()
	// We're using InsecureSkipVerify, so the verifiedChains parameter will always be empty.
	// We need to parse the certificates ourselves from the raw certs.
	conf.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		chain := make([]*x509.Certificate, len(rawCerts))
		for i := 0; i < len(rawCerts); i++ {
			cert, err := x509.ParseCertificate(rawCerts[i])
			if err != nil {
				return err
			}
			chain[i] = cert
		}
		pubKey, err := getRemotePubKey(chain)
		if err != nil {
			return err
		}
		if !remote.MatchesPublicKey(pubKey) {
			return errors.New("peer IDs don't match")
		}
		return nil
	}
	return conf
}

// KeyFromChain takes a chain of x509.Certificates and returns the peer's public key.
func KeyFromChain(chain []*x509.Certificate) (ic.PubKey, error) {
	return getRemotePubKey(chain)
}

const certValidityPeriod = 180 * 24 * time.Hour

func generateConfig(privKey ic.PrivKey) (*tls.Config, error) {
	key, cert, err := keyToCertificate(privKey)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		ServerName:         hostname,
		InsecureSkipVerify: true, // This is not insecure here. We will verify the cert chain ourselves.
		ClientAuth:         tls.RequireAnyClientCert,
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{cert.Raw},
			PrivateKey:  key,
		}},
	}, nil
}

func getRemotePubKey(chain []*x509.Certificate) (ic.PubKey, error) {
	if len(chain) != 1 {
		return nil, errors.New("expected one certificates in the chain")
	}
	pool := x509.NewCertPool()
	pool.AddCert(chain[0])
	if _, err := chain[0].Verify(x509.VerifyOptions{Roots: pool}); err != nil {
		return nil, err
	}
	remotePubKey, err := x509.MarshalPKIXPublicKey(chain[0].PublicKey)
	if err != nil {
		return nil, err
	}
	return ic.UnmarshalRsaPublicKey(remotePubKey)
}

func keyToCertificate(sk ic.PrivKey) (interface{}, *x509.Certificate, error) {
	sn, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: sn,
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     time.Now().Add(certValidityPeriod),
		DNSNames:     []string{hostname},
	}

	var publicKey, privateKey interface{}
	keyBytes, err := sk.Bytes()
	if err != nil {
		return nil, nil, err
	}
	pbmes := new(pb.PrivateKey)
	if err := proto.Unmarshal(keyBytes, pbmes); err != nil {
		return nil, nil, err
	}
	switch pbmes.GetType() {
	case pb.KeyType_RSA:
		k, err := x509.ParsePKCS1PrivateKey(pbmes.GetData())
		if err != nil {
			return nil, nil, err
		}
		publicKey = &k.PublicKey
		privateKey = k
	// TODO: add support for ECDSA
	default:
		return nil, nil, errors.New("unsupported key type for TLS")
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, publicKey, privateKey)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, err
	}
	return privateKey, cert, nil
}
