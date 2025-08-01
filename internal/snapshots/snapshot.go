package snapshots

import (
	"io"
	"os"
	"path/filepath"

	"github.com/hoangsonww/backupagent/internal/chunker"
	"github.com/hoangsonww/backupagent/internal/storage"
	"github.com/hoangsonww/backupagent/internal/versioning"
	"github.com/hoangsonww/backupagent/internal/crypto"
	"github.com/hoangsonww/backupagent/internal/versioning"
)

func CreateSnapshot(path string, store *storage.Store, signerPub, signerPriv []byte, parent string, cfgSnapshotMin, cfgSnapshotMax, cfgSnapshotAvg int) (*versioning.Snapshot, error) {
	var chunkHashes []string

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() {
			f, err := os.Open(p)
			if err != nil {
				return err
			}
			defer f.Close()
			ch := chunker.New(f, cfgSnapshotMin, cfgSnapshotMax, cfgSnapshotAvg)
			for {
				chunk, err := ch.Next()
				if err == io.EOF {
					break
				}
				if err != nil {
					return err
				}
				hash, err := store.PutChunk(chunk)
				if err != nil {
					return err
				}
				chunkHashes = append(chunkHashes, hash)
				if len(chunk) == 0 {
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	snap := &versioning.Snapshot{
		ID:        versioning.GenerateSnapshotID(), // implement deterministic or random
		Parent:    parent,
		Timestamp: time.Now().UTC(),
		Chunks:    chunkHashes,
		Meta:      map[string]string{"source": path},
		SignerPub: crypto.EncodeKey(signerPub),
	}
	// Sign it
	raw, _ := json.Marshal(snapWithoutSignature(snap))
	sig := crypto.Sign(raw, signerPriv)
	snap.Signature = base64.StdEncoding.EncodeToString(sig)

	return snap, nil
}

func snapWithoutSignature(s *versioning.Snapshot) *versioning.Snapshot {
	return &versioning.Snapshot{
		ID:        s.ID,
		Parent:    s.Parent,
		Timestamp: s.Timestamp,
		Chunks:    s.Chunks,
		Meta:      s.Meta,
		SignerPub: s.SignerPub,
	}
}
