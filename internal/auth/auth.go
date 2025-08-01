package auth

import (
	"errors"

	"github.com/hoangsonww/backupagent/internal/crypto"
)

type ACL struct {
	Admins map[string]bool // base64-encoded pub keys
}

// Load from list
func NewACL(admins []string) *ACL {
	m := make(map[string]bool)
	for _, a := range admins {
		m[a] = true
	}
	return &ACL{Admins: m}
}

func (a *ACL) IsAdmin(pubKey string) bool {
	return a.Admins[pubKey]
}

// Peer authentication: verifying signed messages

type SignedMessage struct {
	Payload   []byte
	Signature []byte
	PubKey    []byte
}

func VerifySignedMessage(msg *SignedMessage) bool {
	return crypto.Verify(msg.Payload, msg.Signature, msg.PubKey)
}

func SignPayload(payload []byte, priv []byte) []byte {
	return crypto.Sign(payload, priv)
}

func PubKeyToString(pub []byte) string {
	return crypto.EncodeKey(pub)
}

func StringToPubKey(s string) ([]byte, error) {
	return crypto.DecodeKey(s)
}

// Authorization error
var ErrNotAuthorized = errors.New("not authorized")
