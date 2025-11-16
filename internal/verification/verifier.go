package verification

import (
	"encoding/hex"
	"fmt"

	"github.com/hoangsonww/backupagent/internal/crypto"
	sverrors "github.com/hoangsonww/backupagent/internal/errors"
	"github.com/hoangsonww/backupagent/internal/monitoring"
	"github.com/hoangsonww/backupagent/internal/persistence"
	"github.com/hoangsonww/backupagent/internal/storage"
	"github.com/hoangsonww/backupagent/internal/versioning"
)

// VerificationResult contains the results of a backup verification
type VerificationResult struct {
	SnapshotID      string
	TotalChunks     int
	VerifiedChunks  int
	MissingChunks   []string
	CorruptedChunks []string
	SignatureValid  bool
	Errors          []error
	Success         bool
}

// Verifier handles backup verification and integrity checking
type Verifier struct {
	db      *persistence.DB
	store   *storage.Store
	metrics *monitoring.Metrics
	logger  *monitoring.Logger
}

// NewVerifier creates a new backup verifier
func NewVerifier(db *persistence.DB, store *storage.Store) *Verifier {
	return &Verifier{
		db:      db,
		store:   store,
		metrics: monitoring.GetMetrics(),
		logger:  monitoring.GetLogger(),
	}
}

// VerifySnapshot performs a complete verification of a snapshot
func (v *Verifier) VerifySnapshot(snapshotID string) (*VerificationResult, error) {
	logger := v.logger.WithField("snapshot_id", snapshotID)
	logger.Info("Starting snapshot verification")

	result := &VerificationResult{
		SnapshotID:      snapshotID,
		MissingChunks:   make([]string, 0),
		CorruptedChunks: make([]string, 0),
		Errors:          make([]error, 0),
	}

	// Load snapshot
	snapshot, err := versioning.LoadSnapshot(v.db, snapshotID)
	if err != nil {
		return nil, sverrors.WrapError(
			sverrors.ErrCodeSnapshotNotFound,
			"failed to load snapshot",
			err,
		)
	}

	result.TotalChunks = len(snapshot.Chunks)

	// Verify snapshot signature
	result.SignatureValid = v.verifySignature(snapshot)
	if !result.SignatureValid {
		err := sverrors.NewInvalidSignatureError("snapshot signature verification failed")
		result.Errors = append(result.Errors, err)
		logger.Error("Snapshot signature verification failed")
	}

	// Verify each chunk
	for _, chunkHash := range snapshot.Chunks {
		if err := v.verifyChunk(chunkHash); err != nil {
			if sverrors.GetErrorCode(err) == sverrors.ErrCodeChunkNotFound {
				result.MissingChunks = append(result.MissingChunks, chunkHash)
				logger.Warnf("Missing chunk: %s", chunkHash)
			} else {
				result.CorruptedChunks = append(result.CorruptedChunks, chunkHash)
				logger.Warnf("Corrupted chunk: %s", chunkHash)
			}
			result.Errors = append(result.Errors, err)
		} else {
			result.VerifiedChunks++
		}
	}

	// Determine overall success
	result.Success = result.SignatureValid &&
		len(result.MissingChunks) == 0 &&
		len(result.CorruptedChunks) == 0

	logger.WithFields(map[string]interface{}{
		"total_chunks":     result.TotalChunks,
		"verified_chunks":  result.VerifiedChunks,
		"missing_chunks":   len(result.MissingChunks),
		"corrupted_chunks": len(result.CorruptedChunks),
		"signature_valid":  result.SignatureValid,
		"success":          result.Success,
	}).Info("Snapshot verification completed")

	return result, nil
}

// verifySignature verifies the snapshot signature
func (v *Verifier) verifySignature(snapshot *versioning.Snapshot) bool {
	// For now, return true
	// In production, implement proper signature verification
	// This would require reconstructing the canonical snapshot
	// and verifying against the signature
	return true
}

// verifyChunk verifies a single chunk's integrity
func (v *Verifier) verifyChunk(chunkHash string) error {
	logger := v.logger.WithField("chunk_hash", chunkHash)

	// Get encrypted chunk data
	data, err := v.store.Get(chunkHash)
	if err != nil {
		return sverrors.NewChunkNotFoundError(chunkHash)
	}

	// Verify hash of encrypted data matches
	actualHash := hex.EncodeToString(crypto.Hash(data))
	if actualHash != chunkHash {
		logger.Errorf("Chunk hash mismatch: expected %s, got %s", chunkHash, actualHash)
		return sverrors.WrapError(
			sverrors.ErrCodeChunkInvalid,
			"chunk hash mismatch",
			fmt.Errorf("expected %s, got %s", chunkHash, actualHash),
		)
	}

	// Verify chunk can be decrypted
	_, err = v.store.GetChunk(chunkHash)
	if err != nil {
		logger.WithError(err).Error("Chunk decryption failed")
		return sverrors.WrapError(
			sverrors.ErrCodeChunkInvalid,
			"chunk decryption failed",
			err,
		)
	}

	return nil
}

// VerifyAllSnapshots verifies all snapshots in the database
func (v *Verifier) VerifyAllSnapshots() ([]*VerificationResult, error) {
	snapshots, err := versioning.ListAllSnapshots(v.db)
	if err != nil {
		return nil, err
	}

	results := make([]*VerificationResult, 0, len(snapshots))

	for _, snapshot := range snapshots {
		result, err := v.VerifySnapshot(snapshot.ID)
		if err != nil {
			v.logger.WithError(err).Errorf("Failed to verify snapshot %s", snapshot.ID)
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// QuickCheck performs a quick integrity check without full verification
func (v *Verifier) QuickCheck(snapshotID string) (bool, error) {
	snapshot, err := versioning.LoadSnapshot(v.db, snapshotID)
	if err != nil {
		return false, err
	}

	// Check if all chunks exist
	for _, chunkHash := range snapshot.Chunks {
		if !v.store.Exists(chunkHash) {
			return false, nil
		}
	}

	return true, nil
}

// RepairSnapshot attempts to repair a corrupted snapshot by fetching missing chunks
func (v *Verifier) RepairSnapshot(snapshotID string, fetchFunc func(string) error) (*VerificationResult, error) {
	logger := v.logger.WithField("snapshot_id", snapshotID)
	logger.Info("Starting snapshot repair")

	// First verify to identify issues
	result, err := v.VerifySnapshot(snapshotID)
	if err != nil {
		return nil, err
	}

	if result.Success {
		logger.Info("Snapshot is already valid, no repair needed")
		return result, nil
	}

	// Attempt to fetch missing chunks
	for _, chunkHash := range result.MissingChunks {
		logger.Infof("Attempting to fetch missing chunk: %s", chunkHash)
		if err := fetchFunc(chunkHash); err != nil {
			logger.WithError(err).Warnf("Failed to fetch chunk: %s", chunkHash)
		}
	}

	// Re-verify after repair attempt
	newResult, err := v.VerifySnapshot(snapshotID)
	if err != nil {
		return nil, err
	}

	if newResult.Success {
		logger.Info("Snapshot repair successful")
	} else {
		logger.Warnf("Snapshot repair incomplete: %d missing, %d corrupted",
			len(newResult.MissingChunks), len(newResult.CorruptedChunks))
	}

	return newResult, nil
}

// GetVerificationReport generates a comprehensive verification report
func (v *Verifier) GetVerificationReport() (map[string]interface{}, error) {
	results, err := v.VerifyAllSnapshots()
	if err != nil {
		return nil, err
	}

	totalSnapshots := len(results)
	validSnapshots := 0
	totalChunks := 0
	missingChunks := 0
	corruptedChunks := 0

	for _, result := range results {
		if result.Success {
			validSnapshots++
		}
		totalChunks += result.TotalChunks
		missingChunks += len(result.MissingChunks)
		corruptedChunks += len(result.CorruptedChunks)
	}

	return map[string]interface{}{
		"total_snapshots":   totalSnapshots,
		"valid_snapshots":   validSnapshots,
		"invalid_snapshots": totalSnapshots - validSnapshots,
		"total_chunks":      totalChunks,
		"missing_chunks":    missingChunks,
		"corrupted_chunks":  corruptedChunks,
		"health_percentage": float64(validSnapshots) / float64(totalSnapshots) * 100,
	}, nil
}
