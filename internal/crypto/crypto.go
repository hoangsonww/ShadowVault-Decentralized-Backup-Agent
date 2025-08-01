package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"

	"golang.org/x/crypto/argon2"
)

const (
	saltSize = 16
)

func DeriveKey(passphrase string, salt []byte) []byte {
	if salt == nil {
		salt = make([]byte, saltSize)
		rand.Read(salt)
	}
	// Using Argon2id
	return argon2.IDKey([]byte(passphrase), salt, 1, 64*1024, 4, 32)
}

func Encrypt(plaintext, key []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, aesgcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	ciphertext = aesgcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func Decrypt(ciphertext, key, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

func Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

func GenerateEd25519Keypair() (pub, priv []byte, err error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return public, private, nil
}

func Sign(message, priv []byte) []byte {
	return ed25519.Sign(priv, message)
}

func Verify(message, sig, pub []byte) bool {
	return ed25519.Verify(pub, message, sig)
}

func EncodeKey(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

func DecodeKey(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

func ValidateKeySizes(key []byte) error {
	if len(key) != 32 {
		return errors.New("encryption key must be 32 bytes")
	}
	return nil
}
