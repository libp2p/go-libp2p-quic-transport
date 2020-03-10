package libp2pquic

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"golang.org/x/crypto/chacha20"
	"golang.org/x/crypto/hkdf"
)

// This should match *exactly* the list of supported versions of quic-go.
var supportedVersions = [...]uint32{0xff00001b /* draft-27 */}

var (
	// errVersionNegotiation = errors.New("version negotiation packet")
	errUnknownVersion = errors.New("unknown version")
)

type protectedConn struct {
	net.PacketConn

	key *[32]byte

	// used as a buffer, to avoid allocations
	readCounter  [16]byte
	writeCounter [16]byte
}

func protectConn(conn net.PacketConn, psk *[32]byte) net.PacketConn {
	r := hkdf.Expand(sha256.New, psk[:], []byte("libp2p protector"))
	var key [32]byte
	if _, err := io.ReadFull(r, key[:]); err != nil {
		panic(err)
	}
	return &protectedConn{PacketConn: conn, key: &key}
}

func (c *protectedConn) ReadFrom(p []byte) (n int, addr net.Addr, rerr error) {
	n, addr, rerr = c.PacketConn.ReadFrom(p)
	if rerr != nil || n == 0 {
		return
	}
	data := p[:n]
	for len(data) > 0 {
		length, err := c.maybeProtectPacket(data)
		if err != nil {
			if err == errUnknownVersion {
				// TODO: send version negotiation packet
				return c.ReadFrom(p)
			}
			// let the QUIC stack handle the packet
			return
		}
		data = data[length:]
	}
	return
}

func (c *protectedConn) maybeProtectPacket(p []byte) (uint64, error) {
	if p[0]&0x80 == 0 {
		return uint64(len(p)), nil
	}

	r := bytes.NewReader(p)
	startLen := r.Len()
	typeByte, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	ver := make([]byte, 4)
	if _, err := r.Read(ver); err != nil {
		return 0, err
	}
	version := binary.BigEndian.Uint32(ver)
	// Version Negotiation Packet, see https://tools.ietf.org/html/draft-ietf-quic-invariants-07#section-5.
	if version == 0 {
		return uint64(len(p)), nil
	}
	var versionSupported bool
	for _, v := range supportedVersions {
		if v == version {
			versionSupported = true
			break
		}
	}
	if !versionSupported {
		return 0, errUnknownVersion
	}

	// skip DCID
	if err := c.skipConnID(r); err != nil {
		return 0, err
	}
	// skip SCID
	if err := c.skipConnID(r); err != nil {
		return 0, err
	}
	if (typeByte&0x30)>>4 == 0x3 { // Retry
		return uint64(len(p)), nil
	}

	length, err := readVarInt(r)
	if err != nil {
		return 0, err
	}
	payloadOffset := uint64(startLen - r.Len())
	payloadOffsetEnd := uint64(payloadOffset) + length
	if uint64(len(p)) < payloadOffsetEnd {
		return 0, io.EOF
	}
	if (typeByte&0x30)>>4 == 0x2 { // Handshake
		// We only encrypt / decrypt Handshake packets.
		if length < 16 {
			return 0, fmt.Errorf("payload length too small: %d", length)
		}
		c.encryptPayload(p[payloadOffset:payloadOffsetEnd])
	}
	return payloadOffset + length, nil
}

func (c *protectedConn) skipConnID(r *bytes.Reader) error {
	l, err := r.ReadByte()
	if err != nil {
		return err
	}
	_, err = r.Seek(int64(l), io.SeekCurrent)
	return err
}

// encryptPayload encrypts the payload.
// payload must be at least 16 bytes long.
func (c *protectedConn) encryptPayload(payload []byte) {
	sample := payload[len(payload)-16:]
	cipher, err := chacha20.NewUnauthenticatedCipher(c.key[:], sample[4:])
	if err != nil {
		panic(err)
	}
	cipher.SetCounter(binary.LittleEndian.Uint32(sample[:4]))
	// TODO: implement the ChaCha20 crash workaround
	cipher.XORKeyStream(payload[:16], payload[:16])
}

func (c *protectedConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	if err := c.writeTo(p, addr); err != nil {
		// We expect the QUIC stack to only send valid QUIC packets.
		panic(err)
	}
	return c.PacketConn.WriteTo(p, addr)
}

func (c *protectedConn) writeTo(p []byte, addr net.Addr) error {
	data := p
	for len(data) > 0 {
		length, err := c.maybeProtectPacket(data)
		if err != nil {
			if err == errUnknownVersion {
				return errors.New("wrote packet with unsupported version")
			}
			return fmt.Errorf("error parsing packet: %s", err)
		}
		data = data[length:]
	}

	return nil
}

// readVarInt reads a number in the QUIC varint format
func readVarInt(b io.ByteReader) (uint64, error) {
	firstByte, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	// the first two bits of the first byte encode the length
	len := 1 << ((firstByte & 0xc0) >> 6)
	b1 := firstByte & (0xff - 0xc0)
	if len == 1 {
		return uint64(b1), nil
	}
	b2, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	if len == 2 {
		return uint64(b2) + uint64(b1)<<8, nil
	}
	b3, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	b4, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	if len == 4 {
		return uint64(b4) + uint64(b3)<<8 + uint64(b2)<<16 + uint64(b1)<<24, nil
	}
	b5, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	b6, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	b7, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	b8, err := b.ReadByte()
	if err != nil {
		return 0, err
	}
	return uint64(b8) + uint64(b7)<<8 + uint64(b6)<<16 + uint64(b5)<<24 + uint64(b4)<<32 + uint64(b3)<<40 + uint64(b2)<<48 + uint64(b1)<<56, nil
}
