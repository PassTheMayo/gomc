package auth

import (
	"crypto/sha1"
	"encoding/hex"
	"strings"
)

// ServerHash computes the server ID hash sent to the session server: a SHA-1
// digest printed as a signed two's complement hexadecimal number, like Java's
// new BigInteger(digest).toString(16).
func ServerHash(serverID string, secret, publicKey []byte) string {
	h := sha1.New()
	h.Write([]byte(serverID))
	h.Write(secret)
	h.Write(publicKey)

	digest := h.Sum(nil)
	negative := digest[0]&0x80 != 0

	if negative {
		carry := true

		for i := len(digest) - 1; i >= 0; i-- {
			digest[i] = ^digest[i]

			if carry {
				digest[i]++
				carry = digest[i] == 0
			}
		}
	}

	s := strings.TrimLeft(hex.EncodeToString(digest), "0")

	if negative {
		return "-" + s
	}

	return s
}
