package conn

import "crypto/cipher"

// cfb8 implements the 8-bit cipher feedback mode Minecraft uses for its
// encrypted connections. The standard library only provides full-block CFB.
type cfb8 struct {
	block   cipher.Block
	window  []byte
	pos     int
	out     []byte
	decrypt bool
}

// newCFB8 returns a CFB8 stream. The IV must be one block long.
func newCFB8(block cipher.Block, iv []byte, decrypt bool) cipher.Stream {
	size := block.BlockSize()
	window := make([]byte, size*16)

	copy(window, iv)

	return &cfb8{block: block, window: window, out: make([]byte, size), decrypt: decrypt}
}

// XORKeyStream encrypts or decrypts src into dst one byte at a time. The shift
// register slides over a larger window so it only needs compacting once every
// few blocks instead of on every byte.
func (c *cfb8) XORKeyStream(dst, src []byte) {
	size := len(c.out)

	for i, b := range src {
		if c.pos+size >= len(c.window) {
			copy(c.window, c.window[c.pos:c.pos+size])
			c.pos = 0
		}

		c.block.Encrypt(c.out, c.window[c.pos:c.pos+size])

		x := b ^ c.out[0]

		if c.decrypt {
			c.window[c.pos+size] = b
		} else {
			c.window[c.pos+size] = x
		}

		c.pos++
		dst[i] = x
	}
}
