package gc

import (
	"context"
	"fmt"
	"time"

	"github.com/hoangsonww/backupagent/internal/monitoring"
	"github.com/hoangsonww/backupagent/internal/persistence"
	"github.com/hoangsonww/backupagent/internal/storage"
	"github.com/hoangsonww/backupagent/internal/versioning"
)

// Collector handles garbage collection of old snapshots and unreferenced chunks
type Collector struct {
	db            *persistence.DB
	store         *storage.Store
	retentionDays int
	gcInterval    time.Duration
	metrics       *monitoring.Metrics
	ctx           context.Context
	cancel        context.CancelFunc
}

// NewCollector creates a new garbage collector
func NewCollector(db *persistence.DB, store *storage.Store, retentionDays int, gcInterval time.Duration) *Collector {
	ctx, cancel := context.WithCancel(context.Background())
	return &Collector{
		db:            db,
		store:         store,
		retentionDays: retentionDays,
		gcInterval:    gcInterval,
		metrics:       monitoring.GetMetrics(),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Start begins the garbage collection routine
func (gc *Collector) Start() {
	logger := monitoring.GetLogger()
	logger.Infof("Starting garbage collector (retention: %d days, interval: %s)",
		gc.retentionDays, gc.gcInterval)

	go func() {
		ticker := time.NewTicker(gc.gcInterval)
		defer ticker.Stop()

		// Run immediately on start
		if err := gc.Run(); err != nil {
			logger.WithError(err).Error("Initial garbage collection failed")
		}

		for {
			select {
			case <-gc.ctx.Done():
				logger.Info("Garbage collector stopped")
				return
			case <-ticker.C:
				if err := gc.Run(); err != nil {
					logger.WithError(err).Error("Garbage collection failed")
				}
			}
		}
	}()
}

// Stop stops the garbage collector
func (gc *Collector) Stop() {
	gc.cancel()
}

// Run performs a garbage collection cycle
func (gc *Collector) Run() error {
	logger := monitoring.GetLogger()
	startTime := time.Now()

	logger.Info("Starting garbage collection cycle")

	// Step 1: Find and delete old snapshots
	deletedSnapshots, err := gc.deleteOldSnapshots()
	if err != nil {
		return fmt.Errorf("failed to delete old snapshots: %w", err)
	}

	logger.Infof("Deleted %d old snapshots", deletedSnapshots)

	// Step 2: Find referenced chunks
	referencedChunks, err := gc.findReferencedChunks()
	if err != nil {
		return fmt.Errorf("failed to find referenced chunks: %w", err)
	}

	logger.Infof("Found %d referenced chunks", len(referencedChunks))

	// Step 3: Delete unreferenced chunks
	deletedChunks, bytesFreed, err := gc.deleteUnreferencedChunks(referencedChunks)
	if err != nil {
		return fmt.Errorf("failed to delete unreferenced chunks: %w", err)
	}

	// Record metrics
	gc.metrics.RecordGarbageCollection(uint64(deletedChunks), int64(bytesFreed))

	duration := time.Since(startTime)
	logger.WithFields(map[string]interface{}{
		"deleted_snapshots": deletedSnapshots,
		"deleted_chunks":    deletedChunks,
		"bytes_freed":       bytesFreed,
		"duration":          duration.Seconds(),
	}).Info("Garbage collection completed")

	return nil
}

// deleteOldSnapshots deletes snapshots older than retention period
func (gc *Collector) deleteOldSnapshots() (int, error) {
	logger := monitoring.GetLogger()
	cutoffTime := time.Now().AddDate(0, 0, -gc.retentionDays)

	// Get all snapshots
	snapshots, err := gc.getAllSnapshots()
	if err != nil {
		return 0, fmt.Errorf("failed to get snapshots: %w", err)
	}

	deletedCount := 0
	for _, snap := range snapshots {
		// Parse snapshot timestamp
		snapTime, err := time.Parse(time.RFC3339, snap.Timestamp)
		if err != nil {
			logger.WithError(err).Warnf("Failed to parse snapshot timestamp: %s", snap.ID)
			continue
		}

		// Delete if older than cutoff
		if snapTime.Before(cutoffTime) {
			if err := versioning.DeleteSnapshot(gc.db, snap.ID); err != nil {
				logger.WithError(err).Warnf("Failed to delete snapshot: %s", snap.ID)
				continue
			}
			logger.Infof("Deleted old snapshot: %s (age: %s)", snap.ID, time.Since(snapTime))
			deletedCount++
		}
	}

	return deletedCount, nil
}

// findReferencedChunks returns a set of all chunk hashes referenced by active snapshots
func (gc *Collector) findReferencedChunks() (map[string]bool, error) {
	snapshots, err := gc.getAllSnapshots()
	if err != nil {
		return nil, err
	}

	referenced := make(map[string]bool)
	for _, snap := range snapshots {
		for _, chunkHash := range snap.Chunks {
			referenced[chunkHash] = true
		}
	}

	return referenced, nil
}

// deleteUnreferencedChunks deletes chunks not referenced by any snapshot
func (gc *Collector) deleteUnreferencedChunks(referenced map[string]bool) (int, int64, error) {
	logger := monitoring.GetLogger()

	// Get all stored chunks
	allChunks, err := gc.store.ListAll()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to list chunks: %w", err)
	}

	deletedCount := 0
	var bytesFreed int64

	for _, chunkHash := range allChunks {
		if !referenced[chunkHash] {
			// Get chunk size before deletion
			data, err := gc.store.Get(chunkHash)
			if err != nil {
				logger.WithError(err).Warnf("Failed to get chunk for size: %s", chunkHash)
				continue
			}
			chunkSize := int64(len(data))

			// Delete unreferenced chunk
			if err := gc.store.Delete(chunkHash); err != nil {
				logger.WithError(err).Warnf("Failed to delete chunk: %s", chunkHash)
				continue
			}

			deletedCount++
			bytesFreed += chunkSize
		}
	}

	return deletedCount, bytesFreed, nil
}

// getAllSnapshots returns all snapshots from the database
func (gc *Collector) getAllSnapshots() ([]*versioning.Snapshot, error) {
	return versioning.ListAllSnapshots(gc.db)
}

// RunOnce performs a single garbage collection cycle (for manual triggers)
func (gc *Collector) RunOnce() error {
	return gc.Run()
}
