package persistence

import (
	"time"

	bolt "go.etcd.io/bbolt"
)

const (
	BucketBlocks    = "blocks"
	BucketSnapshots = "snapshots"
	BucketPeers     = "peers"
	BucketACLs      = "acls"
)

type DB struct {
	db *bolt.DB
}

func Open(path string) (*DB, error) {
	b, err := bolt.Open(path, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, err
	}
	err = b.Update(func(tx *bolt.Tx) error {
		for _, bucket := range []string{BucketBlocks, BucketSnapshots, BucketPeers, BucketACLs} {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &DB{db: b}, nil
}

func (d *DB) View(fn func(tx *bolt.Tx) error) error {
	return d.db.View(fn)
}

func (d *DB) Update(fn func(tx *bolt.Tx) error) error {
	return d.db.Update(fn)
}

func (d *DB) Close() error {
	return d.db.Close()
}
