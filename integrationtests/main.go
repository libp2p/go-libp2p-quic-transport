package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	libp2pquic "github.com/libp2p/go-libp2p-quic-transport"
	ma "github.com/multiformats/go-multiaddr"
)

func main() {
	hostKeyFile := flag.String("key", "", "file containing the libp2p private key")
	peerKeyFile := flag.String("peerkey", "", "file containing the libp2p private key of the peer")
	addrStr := flag.String("addr", "", "address to listen on (for the server) or to dial (for the client)")
	role := flag.String("role", "", "server or client")
	flag.Parse()

	hostKey, peerPubKey, err := readKeys(*hostKeyFile, *peerKeyFile)
	if err != nil {
		log.Fatal(err)
	}
	addr, err := ma.NewMultiaddr(*addrStr)
	if err != nil {
		log.Fatal(err)
	}

	switch *role {
	case "server":
		if err := runServer(hostKey, peerPubKey, addr); err != nil {
			log.Fatal(err)
		}
	case "client":
		if err := runClient(hostKey, peerPubKey, addr); err != nil {
			log.Fatal(err)
		}
	}
}

// We pass in both the private keys of host and peer.
// We never use the private key of the peer though.
// That's why this function returns the peer's public key.
func readKeys(hostKeyFile, peerKeyFile string) (crypto.PrivKey, crypto.PubKey, error) {
	// read the host key
	hostKeyBytes, err := ioutil.ReadFile(hostKeyFile)
	if err != nil {
		return nil, nil, err
	}
	hostKey, err := crypto.UnmarshalPrivateKey(hostKeyBytes)
	if err != nil {
		return nil, nil, err
	}
	// read the peers key
	peerKeyBytes, err := ioutil.ReadFile(peerKeyFile)
	if err != nil {
		return nil, nil, err
	}
	peerKey, err := crypto.UnmarshalPrivateKey(peerKeyBytes)
	if err != nil {
		return nil, nil, err
	}
	return hostKey, peerKey.GetPublic(), nil
}

func runServer(hostKey crypto.PrivKey, peerKey crypto.PubKey, addr ma.Multiaddr) error {
	tr, err := libp2pquic.NewTransport(hostKey, nil, nil)
	if err != nil {
		return err
	}
	ln, err := tr.Listen(addr)
	if err != nil {
		return err
	}
	conn, err := ln.Accept()
	if err != nil {
		return err
	}
	defer ln.Close()
	if !conn.RemotePublicKey().Equals(peerKey) {
		return errors.New("mismatching public keys")
	}
	clientPeerID, err := peer.IDFromPublicKey(peerKey)
	if err != nil {
		return err
	}
	if conn.RemotePeer() != clientPeerID {
		return fmt.Errorf("remote Peer ID mismatch. Got %s, expected %s", conn.RemotePeer().Pretty(), clientPeerID.Pretty())
	}
	for {
		str, err := conn.AcceptStream()
		if err != nil {
			return nil
		}
		defer str.Close()
		data, err := ioutil.ReadAll(str)
		if err != nil {
			return err
		}
		if _, err := str.Write(data); err != nil {
			return err
		}
		if err := str.Close(); err != nil {
			return err
		}
	}
}

func runClient(hostKey crypto.PrivKey, peerKey crypto.PubKey, addr ma.Multiaddr) error {
	tr, err := libp2pquic.NewTransport(hostKey, nil, nil)
	if err != nil {
		return err
	}
	serverPeerID, err := peer.IDFromPublicKey(peerKey)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, err := tr.Dial(ctx, addr, serverPeerID)
	if err != nil {
		return err
	}
	defer conn.Close()
	if !conn.RemotePublicKey().Equals(peerKey) {
		return errors.New("mismatching public keys")
	}
	if conn.RemotePeer() != serverPeerID {
		return fmt.Errorf("remote Peer ID mismatch. Got %s, expected %s", conn.RemotePeer().Pretty(), serverPeerID.Pretty())
	}
	str, err := conn.OpenStream()
	if err != nil {
		return err
	}
	data := make([]byte, 1<<15)
	rand.Read(data)
	if _, err := str.Write(data); err != nil {
		return err
	}
	if err := str.Close(); err != nil {
		return err
	}
	echoed, err := ioutil.ReadAll(str)
	if err != nil {
		return err
	}
	if !bytes.Equal(data, echoed) {
		return errors.New("echoed data does not match")
	}
	return nil
}
