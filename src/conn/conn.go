// Package conn implements the framing layer of the Minecraft protocol: length
// prefixes, optional zlib compression and optional AES/CFB8 encryption.
package conn

import (
	"bytes"
	"compress/zlib"
	"crypto/aes"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"main/src/protocol"
)

const (
	// MaxPacketSize is the largest frame length a three byte VarInt can describe.
	MaxPacketSize = 2097151

	// MaxUncompressedSize is the largest decompressed packet the client accepts.
	MaxUncompressedSize = 8388608
)

// Conn is a Minecraft protocol connection. ReadPacket must only be called from
// one goroutine at a time; WritePacket is safe for concurrent use.
type Conn struct {
	raw       net.Conn
	in        *reader
	threshold int

	wmu    sync.Mutex
	out    io.Writer
	frame  []byte
	zbuf   bytes.Buffer
	zw     *zlib.Writer
	zr     io.ReadCloser
	encOut []byte
}

// New wraps a network connection. Compression and encryption start disabled.
func New(raw net.Conn) *Conn {
	return &Conn{raw: raw, in: newReader(raw), out: raw, threshold: -1}
}

// Close closes the underlying connection.
func (c *Conn) Close() error {
	return c.raw.Close()
}

// SetReadDeadline sets the deadline for future reads.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.raw.SetReadDeadline(t)
}

// SetCompression enables compression of packets at least threshold bytes long.
// A negative threshold disables compression.
func (c *Conn) SetCompression(threshold int) {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	c.threshold = threshold
}

// EnableEncryption encrypts all further traffic with the shared secret, which
// serves as both the AES key and the CFB8 initialization vector.
func (c *Conn) EnableEncryption(secret []byte) error {
	block, err := aes.NewCipher(secret)

	if err != nil {
		return err
	}

	c.wmu.Lock()
	defer c.wmu.Unlock()

	c.in.enableDecryption(newCFB8(block, secret, true))
	c.out = &encryptWriter{w: c.raw, stream: newCFB8(block, secret, false), buf: &c.encOut}

	return nil
}

// ReadPacket reads the next packet and returns its ID and payload.
func (c *Conn) ReadPacket() (int32, []byte, error) {
	length, err := protocol.ReadVarInt(c.in)

	if err != nil {
		return 0, nil, err
	}

	if length <= 0 || length > MaxPacketSize {
		return 0, nil, fmt.Errorf("conn: invalid packet length %d", length)
	}

	body := make([]byte, length)

	if err = c.in.ReadFull(body); err != nil {
		return 0, nil, err
	}

	if c.threshold >= 0 {
		if body, err = c.decompress(body); err != nil {
			return 0, nil, err
		}
	}

	r := protocol.NewReader(body)
	id := r.VarInt()

	if err = r.Err(); err != nil {
		return 0, nil, err
	}

	return id, r.Rest(), nil
}

func (c *Conn) decompress(body []byte) ([]byte, error) {
	r := protocol.NewReader(body)
	size := int(r.VarInt())

	if err := r.Err(); err != nil {
		return nil, err
	}

	if size == 0 {
		return r.Rest(), nil
	}

	if size < c.threshold || size > MaxUncompressedSize {
		return nil, fmt.Errorf("conn: invalid uncompressed size %d", size)
	}

	src := bytes.NewReader(r.Rest())

	if c.zr == nil {
		zr, err := zlib.NewReader(src)

		if err != nil {
			return nil, err
		}

		c.zr = zr
	} else if err := c.zr.(zlib.Resetter).Reset(src, nil); err != nil {
		return nil, err
	}

	out := make([]byte, size)

	if _, err := io.ReadFull(c.zr, out); err != nil {
		return nil, err
	}

	if n, _ := c.zr.Read(make([]byte, 1)); n != 0 {
		return nil, errors.New("conn: compressed packet larger than declared")
	}

	return out, nil
}

// WritePacket frames, compresses and encrypts a packet and writes it.
func (c *Conn) WritePacket(id int32, payload []byte) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	size := protocol.VarIntSize(id) + len(payload)

	if size > MaxUncompressedSize {
		return fmt.Errorf("conn: packet 0x%02X too large (%d bytes)", id, size)
	}

	var length int
	var compressed []byte

	switch {
	case c.threshold < 0:
		length = size
	case size < c.threshold:
		length = size + 1
	default:
		var err error

		if compressed, err = c.compress(id, payload); err != nil {
			return err
		}

		length = protocol.VarIntSize(int32(size)) + len(compressed)
	}

	if length > MaxPacketSize {
		return fmt.Errorf("conn: packet 0x%02X exceeds the maximum frame size (%d bytes)", id, length)
	}

	frame := protocol.AppendVarInt(c.frame[:0], int32(length))

	switch {
	case compressed != nil:
		frame = protocol.AppendVarInt(frame, int32(size))
		frame = append(frame, compressed...)
	case c.threshold >= 0:
		frame = append(frame, 0)
		fallthrough
	default:
		frame = protocol.AppendVarInt(frame, id)
		frame = append(frame, payload...)
	}

	c.frame = frame
	_, err := c.out.Write(frame)

	return err
}

func (c *Conn) compress(id int32, payload []byte) ([]byte, error) {
	c.zbuf.Reset()

	if c.zw == nil {
		c.zw = zlib.NewWriter(&c.zbuf)
	} else {
		c.zw.Reset(&c.zbuf)
	}

	var idBuf [protocol.MaxVarIntLen]byte

	if _, err := c.zw.Write(protocol.AppendVarInt(idBuf[:0], id)); err != nil {
		return nil, err
	}

	if _, err := c.zw.Write(payload); err != nil {
		return nil, err
	}

	if err := c.zw.Close(); err != nil {
		return nil, err
	}

	return c.zbuf.Bytes(), nil
}
