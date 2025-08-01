package identity

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	peer "github.com/libp2p/go-libp2p/core/peer"
)

const keyFileName = "identity.key"

// LoadOrCreate loads a libp2p identity key from repoPath or creates & persists a new one.
func LoadOrCreate(repoPath string) (libp2pcrypto.PrivKey, string, error) {
	if err := os.MkdirAll(repoPath, 0700); err != nil {
		return nil, "", err
	}
	keyPath := filepath.Join(repoPath, keyFileName)
	if _, err := os.Stat(keyPath); err == nil {
		b, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, "", err
		}
		priv, err := libp2pcrypto.UnmarshalPrivateKey(b)
		if err != nil {
			return nil, "", err
		}
		pid, err := peer.IDFromPrivateKey(priv)
		if err != nil {
			return nil, "", err
		}
		return priv, pid.String(), nil
	}
	// generate new Ed25519 key
	priv, _, err := libp2pcrypto.GenerateEd25519Key(nil)
	if err != nil {
		return nil, "", err
	}
	marshaled, err := libp2pcrypto.MarshalPrivateKey(priv)
	if err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(keyPath, marshaled, 0600); err != nil {
		return nil, "", err
	}
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return nil, "", err
	}
	return priv, pid.String(), nil
}

// ExportPublicKeyBase64 exports a libp2p public key to base64 string.
func ExportPublicKeyBase64(priv libp2pcrypto.PrivKey) (string, error) {
	pub := priv.GetPublic()
	raw, err := libp2pcrypto.MarshalPublicKey(pub)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// ImportPublicKeyBase64 imports a base64-encoded libp2p public key.
func ImportPublicKeyBase64(s string) (libp2pcrypto.PubKey, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return libp2pcrypto.UnmarshalPublicKey(raw)
}

func PeerIDFromPriv(priv libp2pcrypto.PrivKey) (string, error) {
	pid, err := peer.IDFromPrivateKey(priv)
	if err != nil {
		return "", err
	}
	return pid.String(), nil
}

func DescribeIdentity(priv libp2pcrypto.PrivKey) (string, string, error) {
	pid, err := PeerIDFromPriv(priv)
	if err != nil {
		return "", "", err
	}
	pubB64, err := ExportPublicKeyBase64(priv)
	if err != nil {
		return "", "", err
	}
	return pid, pubB64, nil
}

func ValidatePeerID(priv libp2pcrypto.PrivKey, expected string) error {
	pid, err := PeerIDFromPriv(priv)
	if err != nil {
		return err
	}
	if pid != expected {
		return fmt.Errorf("peer ID mismatch: got %s expected %s", pid, expected)
	}
	return nil
}
