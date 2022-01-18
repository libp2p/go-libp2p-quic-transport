//go:build new_transport_no_rcmgr
// +build new_transport_no_rcmgr

package transport

import (
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/transport"
)

func New(key crypto.PrivKey) (transport.Transport, error) {
	return libp2pquic.NewTransport(key, nil, nil)
}
