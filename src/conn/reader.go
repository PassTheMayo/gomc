package conn

import (
	"crypto/cipher"
	"io"
)

// reader is a buffered reader that decrypts bytes as they arrive from the
// socket. Enabling decryption also decrypts bytes that were already buffered
// but not yet consumed, so read-ahead never corrupts the stream.
type reader struct {
	src    io.Reader
	buf    []byte
	r, w   int
	stream cipher.Stream
}

func newReader(src io.Reader) *reader {
	return &reader{src: src, buf: make([]byte, 64*1024)}
}

func (b *reader) fill() error {
	if b.r > 0 {
		copy(b.buf, b.buf[b.r:b.w])
		b.w -= b.r
		b.r = 0
	}

	n, err := b.src.Read(b.buf[b.w:])

	if n > 0 {
		if b.stream != nil {
			b.stream.XORKeyStream(b.buf[b.w:b.w+n], b.buf[b.w:b.w+n])
		}

		b.w += n
	}

	if n == 0 && err == nil {
		return io.ErrNoProgress
	}

	if n > 0 {
		return nil
	}

	return err
}

// ReadByte implements io.ByteReader.
func (b *reader) ReadByte() (byte, error) {
	for b.r == b.w {
		if err := b.fill(); err != nil {
			return 0, err
		}
	}

	c := b.buf[b.r]
	b.r++

	return c, nil
}

// ReadFull fills p completely.
func (b *reader) ReadFull(p []byte) error {
	for len(p) > 0 {
		if b.r == b.w {
			if err := b.fill(); err != nil {
				if err == io.EOF {
					return io.ErrUnexpectedEOF
				}

				return err
			}
		}

		n := copy(p, b.buf[b.r:b.w])
		b.r += n
		p = p[n:]
	}

	return nil
}

// enableDecryption decrypts all future input, including bytes already buffered.
func (b *reader) enableDecryption(s cipher.Stream) {
	s.XORKeyStream(b.buf[b.r:b.w], b.buf[b.r:b.w])
	b.stream = s
}
