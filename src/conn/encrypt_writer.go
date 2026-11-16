package conn

import (
	"crypto/cipher"
	"io"
)

// encryptWriter encrypts everything written to it before passing it on. The
// scratch buffer is owned by the Conn and reused between writes.
type encryptWriter struct {
	w      io.Writer
	stream cipher.Stream
	buf    *[]byte
}

func (e *encryptWriter) Write(p []byte) (int, error) {
	if cap(*e.buf) < len(p) {
		*e.buf = make([]byte, len(p))
	}

	out := (*e.buf)[:len(p)]
	e.stream.XORKeyStream(out, p)

	return e.w.Write(out)
}
