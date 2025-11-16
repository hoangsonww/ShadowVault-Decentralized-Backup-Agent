package versioning

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/hoangsonww/backupagent/internal/persistence"
	bolt "go.etcd.io/bbolt"
)

type Snapshot struct {
	ID         string            `json:"id"`
	Parent     string            `json:"parent,omitempty"`
	Timestamp  string            `json:"timestamp"` // RFC3339 format
	Chunks     []string          `json:"chunks"`    // hashes
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

// ListAllSnapshots returns all snapshots in the database
func ListAllSnapshots(db *persistence.DB) ([]*Snapshot, error) {
	var snapshots []*Snapshot
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(persistence.BucketSnapshots))
		return b.ForEach(func(k, v []byte) error {
			var snap Snapshot
			if err := json.Unmarshal(v, &snap); err != nil {
				return err
			}
			snapshots = append(snapshots, &snap)
			return nil
		})
	})
	return snapshots, err
}

// DeleteSnapshot removes a snapshot from the database
func DeleteSnapshot(db *persistence.DB, id string) error {
	return db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(persistence.BucketSnapshots))
		return b.Delete([]byte(id))
	})
}

// CountSnapshots returns the total number of snapshots
func CountSnapshots(db *persistence.DB) (int, error) {
	count := 0
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(persistence.BucketSnapshots))
		stats := b.Stats()
		count = stats.KeyN
		return nil
	})
	return count, err
}
