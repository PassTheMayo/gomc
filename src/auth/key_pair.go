// Package auth implements the online-mode login handshake: RSA key exchange
// and player verification against Mojang's session server.
package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
)

// KeyPair is the server's RSA key used to exchange the shared secret.
type KeyPair struct {
	private   *rsa.PrivateKey
	PublicDER []byte
}

// GenerateKeyPair creates a 1024-bit RSA key pair, the size the client expects.
func GenerateKeyPair() (*KeyPair, error) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)

	if err != nil {
		return nil, err
	}

	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)

	if err != nil {
		return nil, err
	}

	return &KeyPair{private: key, PublicDER: der}, nil
}

// Decrypt decrypts data the client encrypted with the public key.
func (k *KeyPair) Decrypt(data []byte) ([]byte, error) {
	//lint:ignore SA1019 the Minecraft client encrypts the shared secret with PKCS #1 v1.5.
	return rsa.DecryptPKCS1v15(rand.Reader, k.private, data)
}
