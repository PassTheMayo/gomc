// Package uuid implements the 128-bit identifiers Minecraft uses for players
// and entities.
package uuid

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
)

// UUID is a 128-bit universally unique identifier in big-endian byte order.
type UUID [16]byte

// ErrInvalid is returned when a string is not a valid UUID.
var ErrInvalid = errors.New("uuid: invalid format")

// New returns a random (version 4) UUID.
func New() UUID {
	var u UUID

	_, _ = rand.Read(u[:])

	u[6] = u[6]&0x0F | 0x40
	u[8] = u[8]&0x3F | 0x80

	return u
}

// NameBased returns a name-based (version 3, MD5) UUID of data, matching
// Java's UUID.nameUUIDFromBytes.
func NameBased(data []byte) UUID {
	u := UUID(md5.Sum(data))

	u[6] = u[6]&0x0F | 0x30
	u[8] = u[8]&0x3F | 0x80

	return u
}

// OfflinePlayer returns the UUID the vanilla server assigns to a player when
// online mode is disabled.
func OfflinePlayer(name string) UUID {
	return NameBased([]byte("OfflinePlayer:" + name))
}

// Parse parses a UUID with or without dashes.
func Parse(s string) (UUID, error) {
	var u UUID

	s = strings.ReplaceAll(s, "-", "")

	if len(s) != 32 {
		return u, ErrInvalid
	}

	if _, err := hex.Decode(u[:], []byte(s)); err != nil {
		return u, ErrInvalid
	}

	return u, nil
}

// String returns the canonical dashed representation of u.
func (u UUID) String() string {
	var b [36]byte

	hex.Encode(b[0:8], u[0:4])
	b[8] = '-'
	hex.Encode(b[9:13], u[4:6])
	b[13] = '-'
	hex.Encode(b[14:18], u[6:8])
	b[18] = '-'
	hex.Encode(b[19:23], u[8:10])
	b[23] = '-'
	hex.Encode(b[24:], u[10:])

	return string(b[:])
}
