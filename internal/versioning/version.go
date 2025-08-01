package versioning

import (
	"encoding/json"
	"time"

	"github.com/hoangsonww/backupagent/internal/persistence"
)

type Snapshot struct {
	ID         string            `json:"id"`
	Parent     string            `json:"parent,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
	Chunks     []string          `json:"chunks"` // hashes
	Meta       map[string]string `json:"meta"`
	SignerPub  string            `json:"signer_pub"` // for authenticity
	Signature  string            `json:"signature"`
}

func SaveSnapshot(db *persistence.DB, snap *Snapshot) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(persistence.BucketSnapshots))
		data, err := json.Marshal(snap)
		if err != nil {
			return err
		}
		return b.Put([]byte(snap.ID), data)
	})
}

func LoadSnapshot(db *persistence.DB, id string) (*Snapshot, error) {
	var snap Snapshot
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(persistence.BucketSnapshots))
		v := b.Get([]byte(id))
		if v == nil {
			return ErrSnapshotNotFound
		}
		return json.Unmarshal(v, &snap)
	})
	if err != nil {
		return nil, err
	}
	return &snap, nil
}

var ErrSnapshotNotFound = errors.New("snapshot not found")
