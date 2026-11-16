package conn

import (
	"bytes"
	"crypto/aes"
	"encoding/hex"
	"net"
	"testing"
)

func TestCFB8Vector(t *testing.T) {
	// NIST SP 800-38A F.3.7, CFB8-AES128.Encrypt, also verified with openssl.
	key, _ := hex.DecodeString("2b7e151628aed2a6abf7158809cf4f3c")
	iv, _ := hex.DecodeString("000102030405060708090a0b0c0d0e0f")
	plain, _ := hex.DecodeString("6bc1bee22e409f96e93d7e117393172aae2d")
	want := "3b79424c9c0dd436bace9e0ed4586a4f32b9"

	block, _ := aes.NewCipher(key)
	out := make([]byte, len(plain))

	// Encrypt in uneven pieces to exercise the sliding window.
	enc := newCFB8(block, iv, false)
	enc.XORKeyStream(out[:5], plain[:5])
	enc.XORKeyStream(out[5:], plain[5:])

	if got := hex.EncodeToString(out); got != want {
		t.Fatalf("encrypted %s, want %s", got, want)
	}

	dec := newCFB8(block, iv, true)
	back := make([]byte, len(out))
	dec.XORKeyStream(back, out)

	if !bytes.Equal(back, plain) {
		t.Fatal("decryption did not restore the plaintext")
	}

	long := bytes.Repeat([]byte("minecraft"), 100)
	encLong, decLong := newCFB8(block, iv, false), newCFB8(block, iv, true)
	buf := make([]byte, len(long))
	encLong.XORKeyStream(buf, long)
	decLong.XORKeyStream(buf, buf)

	if !bytes.Equal(buf, long) {
		t.Fatal("long round trip failed")
	}
}

func TestPacketsWithCompressionAndEncryption(t *testing.T) {
	a, b := net.Pipe()
	client, server := New(a), New(b)
	secret := []byte("0123456789abcdef")
	small := []byte("small")
	big := bytes.Repeat([]byte("chunk data "), 1000)

	errs := make(chan error, 1)

	go func() {
		for _, p := range [][]byte{small, big} {
			if err := client.WritePacket(0x10, p); err != nil {
				errs <- err
				return
			}
		}

		client.SetCompression(256)

		if err := client.EnableEncryption(secret); err != nil {
			errs <- err
			return
		}

		for _, p := range [][]byte{small, big} {
			if err := client.WritePacket(0x2E, p); err != nil {
				errs <- err
				return
			}
		}

		errs <- nil
	}()

	for i, want := range [][]byte{small, big, small, big} {
		if i == 2 {
			server.SetCompression(256)

			if err := server.EnableEncryption(secret); err != nil {
				t.Fatal(err)
			}
		}

		id, payload, err := server.ReadPacket()

		if err != nil {
			t.Fatalf("packet %d: %v", i, err)
		}

		if (i < 2 && id != 0x10) || (i >= 2 && id != 0x2E) || !bytes.Equal(payload, want) {
			t.Fatalf("packet %d: id 0x%02X, %d bytes", i, id, len(payload))
		}
	}

	if err := <-errs; err != nil {
		t.Fatal(err)
	}
}
