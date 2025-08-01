package crypto_test

import (
	"bytes"
	"testing"

	"github.com/hoangsonww/backupagent/internal/crypto"
)

func TestEncryptDecrypt(t *testing.T) {
	pass := "testpass"
	salt := []byte("testsalt01234567")
	key := crypto.DeriveKey(pass, salt)
	plaintext := []byte("hello world, secret backup data")

	ciphertext, nonce, err := crypto.Encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("encrypt failed: %v", err)
	}
	decoded, err := crypto.Decrypt(ciphertext, key, nonce)
	if err != nil {
		t.Fatalf("decrypt failed: %v", err)
	}
	if !bytes.Equal(decoded, plaintext) {
		t.Fatalf("roundtrip mismatch: got %s want %s", string(decoded), string(plaintext))
	}
}

func TestHashing(t *testing.T) {
	data := []byte("some data to hash")
	h1 := crypto.Hash(data)
	h2 := crypto.Hash(data)
	if !bytes.Equal(h1, h2) {
		t.Fatalf("hashes differ")
	}
}
