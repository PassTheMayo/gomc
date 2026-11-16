package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestServerHash(t *testing.T) {
	// Examples from the protocol documentation, hashing only the name.
	cases := map[string]string{
		"Notch": "4ed1f46bbe04bc756bcb17c0c7ce3e4632f06a48",
		"jeb_":  "-7c9d5b0044c130109a5d7b5fb5c317c02b4e28c1",
		"simon": "88e16a1019277b15d58faf0541e11910eb756f6",
	}

	for name, want := range cases {
		if got := ServerHash(name, nil, nil); got != want {
			t.Errorf("ServerHash(%q) = %s, want %s", name, got, want)
		}
	}
}

func TestKeyPairDecrypt(t *testing.T) {
	keys, err := GenerateKeyPair()

	if err != nil {
		t.Fatal(err)
	}

	pub, err := x509.ParsePKIXPublicKey(keys.PublicDER)

	if err != nil {
		t.Fatal(err)
	}

	secret := []byte("0123456789abcdef")
	//lint:ignore SA1019 the Minecraft client encrypts the shared secret with PKCS #1 v1.5.
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pub.(*rsa.PublicKey), secret)

	if err != nil {
		t.Fatal(err)
	}

	decrypted, err := keys.Decrypt(encrypted)

	if err != nil || string(decrypted) != string(secret) {
		t.Fatalf("Decrypt = %q, %v", decrypted, err)
	}
}

func TestHasJoined(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("username") != "Steve" || r.URL.Query().Get("serverId") != "abc" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		_, _ = w.Write([]byte(`{"id":"b50ad385829d3141a2167e7d7539ba7f","name":"Steve","properties":[{"name":"textures","value":"v","signature":"s"}]}`))
	}))
	defer srv.Close()

	c := NewSessionClient()
	c.BaseURL = srv.URL

	p, err := c.HasJoined(context.Background(), "Steve", "abc")

	if err != nil {
		t.Fatal(err)
	}

	if p.Name != "Steve" || p.ID.String() != "b50ad385-829d-3141-a216-7e7d7539ba7f" || len(p.Properties) != 1 || p.Properties[0].Signature != "s" {
		t.Fatalf("unexpected profile %+v", p)
	}

	if _, err = c.HasJoined(context.Background(), "Alex", "abc"); err != ErrNotAuthenticated {
		t.Fatalf("expected ErrNotAuthenticated, got %v", err)
	}
}
