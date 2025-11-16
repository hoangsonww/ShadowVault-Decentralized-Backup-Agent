package protocol

import (
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/hoangsonww/backupagent/internal/crypto"
	"github.com/hoangsonww/backupagent/internal/versioning"
)

// SnapshotAnnouncement carries a signed snapshot metadata.
type SnapshotAnnouncement struct {
	Snapshot versioning.Snapshot `json:"snapshot"`
}

// Validate verifies the embedded snapshot signature.
func (sa *SnapshotAnnouncement) Validate() error {
	// Reconstruct canonical snapshot without signature for verification
	rawSnap := versioning.Snapshot{
		ID:        sa.Snapshot.ID,
		Parent:    sa.Snapshot.Parent,
		Timestamp: sa.Snapshot.Timestamp,
		Chunks:    sa.Snapshot.Chunks,
		Meta:      sa.Snapshot.Meta,
		SignerPub: sa.Snapshot.SignerPub,
	}
	data, err := json.Marshal(rawSnap)
	if err != nil {
		return err
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sa.Snapshot.Signature)
	if err != nil {
		return err
	}
	pubKeyBytes, err := base64.StdEncoding.DecodeString(sa.Snapshot.SignerPub)
	if err != nil {
		return err
	}
	if !crypto.Verify(data, sigBytes, pubKeyBytes) {
		return errors.New("snapshot signature invalid")
	}
	return nil
}

// ChunkRequest asks for a block by hash. Signed by requester.
type ChunkRequest struct {
	Hash      string `json:"hash"`
	Requestor string `json:"requestor"`  // peer ID
	SignerPub string `json:"signer_pub"` // base64 ed25519 pubkey
	Signature string `json:"signature"`  // base64 signature over Hash+Requestor
}

// Validate ensures the signature on the request is correct.
func (cr *ChunkRequest) Validate() error {
	payload := cr.Hash + "|" + cr.Requestor
	sig, err := base64.StdEncoding.DecodeString(cr.Signature)
	if err != nil {
		return err
	}
	pub, err := base64.StdEncoding.DecodeString(cr.SignerPub)
	if err != nil {
		return err
	}
	if !crypto.Verify([]byte(payload), sig, pub) {
		return errors.New("chunk request signature invalid")
	}
	return nil
}

// ChunkResponse carries the requested block. Signed by responder.
type ChunkResponse struct {
	Hash      string `json:"hash"`
	Data      string `json:"data"`       // base64 encrypted chunk
	SignerPub string `json:"signer_pub"` // base64 ed25519 pubkey
	Signature string `json:"signature"`  // base64 signature over Hash+Data
}

// Validate ensures the response signature is correct.
func (cr *ChunkResponse) Validate() error {
	payload := cr.Hash + "|" + cr.Data
	sig, err := base64.StdEncoding.DecodeString(cr.Signature)
	if err != nil {
		return err
	}
	pub, err := base64.StdEncoding.DecodeString(cr.SignerPub)
	if err != nil {
		return err
	}
	if !crypto.Verify([]byte(payload), sig, pub) {
		return errors.New("chunk response signature invalid")
	}
	return nil
}

// PeerAdd is a message to introduce/add a peer.
type PeerAdd struct {
	Addr      string `json:"addr"`
	PeerID    string `json:"peer_id"`
	SignerPub string `json:"signer_pub"` // base64 ed25519 pubkey of introducer
	Signature string `json:"signature"`  // signature over Addr+PeerID
}

// Validate verifies introduction signature.
func (pa *PeerAdd) Validate() error {
	payload := pa.Addr + "|" + pa.PeerID
	sig, err := base64.StdEncoding.DecodeString(pa.Signature)
	if err != nil {
		return err
	}
	pub, err := base64.StdEncoding.DecodeString(pa.SignerPub)
	if err != nil {
		return err
	}
	if !crypto.Verify([]byte(payload), sig, pub) {
		return errors.New("peer add signature invalid")
	}
	return nil
}

// PeerRemove signals removal of a peer.
type PeerRemove struct {
	PeerID    string `json:"peer_id"`
	SignerPub string `json:"signer_pub"`
	Signature string `json:"signature"`
}

// Validate verifies removal signature.
func (pr *PeerRemove) Validate() error {
	payload := pr.PeerID
	sig, err := base64.StdEncoding.DecodeString(pr.Signature)
	if err != nil {
		return err
	}
	pub, err := base64.StdEncoding.DecodeString(pr.SignerPub)
	if err != nil {
		return err
	}
	if !crypto.Verify([]byte(payload), sig, pub) {
		return errors.New("peer remove signature invalid")
	}
	return nil
}
