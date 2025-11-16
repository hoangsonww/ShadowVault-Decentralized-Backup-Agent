package storage

import (
	"bytes"
	"encoding/hex"
	"errors"
	"sync"

	"github.com/hoangsonww/backupagent/internal/crypto"
	"github.com/hoangsonww/backupagent/internal/persistence"
)

type Store struct {
	db      *persistence.DB
	baseKey []byte // master encryption key
	mu      sync.Mutex
}

func New(db *persistence.DB, masterKey []byte) (*Store, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("master key must be 32 bytes")
	}
	return &Store{
		db:      db,
		baseKey: masterKey,
	}, nil
}

// PutChunk stores deduped encrypted chunk. Returns its hash.
func (s *Store) PutChunk(plaintext []byte) (string, error) {
	hash := crypto.Hash(plaintext)
	hashStr := hex.EncodeToString(hash)

	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.db.Update(func(tx *boltTx) error {
		b := tx.Bucket([]byte(persistence.BucketBlocks))
		if b.Get([]byte(hashStr)) != nil {
			// Already exists (dedup)
			return nil
		}
		enc, nonce, err := crypto.Encrypt(plaintext, s.baseKey)
		if err != nil {
			return err
		}
		// Store as nonce || ciphertext
		stored := append(nonce, enc...)
		return b.Put([]byte(hashStr), stored)
	})
	if err != nil {
		return "", err
	}
	return hashStr, nil
}

// GetChunk returns decrypted chunk by hash string
func (s *Store) GetChunk(hashStr string) ([]byte, error) {
	var stored []byte
	err := s.db.View(func(tx *boltTx) error {
		b := tx.Bucket([]byte(persistence.BucketBlocks))
		v := b.Get([]byte(hashStr))
		if v == nil {
			return errors.New("chunk not found")
		}
		stored = append([]byte(nil), v...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// assume nonce size 12 for GCM
	if len(stored) < 12 {
		return nil, errors.New("stored chunk malformed")
	}
	nonce := stored[:12]
	ciphertext := stored[12:]
	return crypto.Decrypt(ciphertext, s.baseKey, nonce)
}

// Get retrieves encrypted chunk data by hash (for P2P transfer)
func (s *Store) Get(hashStr string) ([]byte, error) {
	var stored []byte
	err := s.db.View(func(tx *boltTx) error {
		b := tx.Bucket([]byte(persistence.BucketBlocks))
		v := b.Get([]byte(hashStr))
		if v == nil {
			return errors.New("chunk not found")
		}
		stored = append([]byte(nil), v...)
		return nil
	})
	return stored, err
}

// Put stores encrypted chunk data directly (for P2P received chunks)
func (s *Store) Put(hashStr string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *boltTx) error {
		b := tx.Bucket([]byte(persistence.BucketBlocks))
		return b.Put([]byte(hashStr), data)
	})
}

// Delete removes a chunk from storage
func (s *Store) Delete(hashStr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *boltTx) error {
		b := tx.Bucket([]byte(persistence.BucketBlocks))
		return b.Delete([]byte(hashStr))
	})
}

// ListAll returns all chunk hashes in storage
func (s *Store) ListAll() ([]string, error) {
	var hashes []string
	err := s.db.View(func(tx *boltTx) error {
		b := tx.Bucket([]byte(persistence.BucketBlocks))
		return b.ForEach(func(k, v []byte) error {
			hashes = append(hashes, string(k))
			return nil
		})
	})
	return hashes, err
}

// Exists checks if a chunk exists in storage
func (s *Store) Exists(hashStr string) bool {
	err := s.db.View(func(tx *boltTx) error {
		b := tx.Bucket([]byte(persistence.BucketBlocks))
		if b.Get([]byte(hashStr)) == nil {
			return errors.New("not found")
		}
		return nil
	})
	return err == nil
}
