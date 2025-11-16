package monitoring

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics holds all application metrics
type Metrics struct {
	// Backup metrics
	BackupsCreated       atomic.Uint64
	BackupsFailed        atomic.Uint64
	RestoresCompleted    atomic.Uint64
	RestoresFailed       atomic.Uint64
	BytesBackedUp        atomic.Uint64
	BytesRestored        atomic.Uint64
	ChunksStored         atomic.Uint64
	ChunksFetched        atomic.Uint64
	DeduplicatedChunks   atomic.Uint64

	// P2P metrics
	PeersConnected       atomic.Int64
	PeersDiscovered      atomic.Uint64
	MessagesReceived     atomic.Uint64
	MessagesSent         atomic.Uint64
	ChunkRequestsReceived atomic.Uint64
	ChunkRequestsSent    atomic.Uint64
	ChunkRequestsFailed  atomic.Uint64

	// Storage metrics
	TotalStorageUsed     atomic.Int64
	BlocksStored         atomic.Uint64
	BlocksDeleted        atomic.Uint64
	GarbageCollectionRuns atomic.Uint64

	// Performance metrics
	BackupDuration       *DurationHistogram
	RestoreDuration      *DurationHistogram
	ChunkFetchDuration   *DurationHistogram

	// Error metrics
	TotalErrors          atomic.Uint64
	NetworkErrors        atomic.Uint64
	StorageErrors        atomic.Uint64
	CryptoErrors         atomic.Uint64
}

// DurationHistogram tracks duration distributions
type DurationHistogram struct {
	mu      sync.RWMutex
	buckets map[string]uint64
	sum     time.Duration
	count   uint64
}

// NewMetrics creates a new Metrics instance
func NewMetrics() *Metrics {
	return &Metrics{
		BackupDuration:     NewDurationHistogram(),
		RestoreDuration:    NewDurationHistogram(),
		ChunkFetchDuration: NewDurationHistogram(),
	}
}

// NewDurationHistogram creates a new duration histogram
func NewDurationHistogram() *DurationHistogram {
	return &DurationHistogram{
		buckets: make(map[string]uint64),
	}
}

// Observe records a duration observation
func (h *DurationHistogram) Observe(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.sum += d
	h.count++

	// Categorize into buckets
	bucket := h.getBucket(d)
	h.buckets[bucket]++
}

// getBucket returns the bucket name for a duration
func (h *DurationHistogram) getBucket(d time.Duration) string {
	switch {
	case d < time.Second:
		return "0-1s"
	case d < 5*time.Second:
		return "1-5s"
	case d < 10*time.Second:
		return "5-10s"
	case d < 30*time.Second:
		return "10-30s"
	case d < time.Minute:
		return "30-60s"
	case d < 5*time.Minute:
		return "1-5m"
	default:
		return "5m+"
	}
}

// Average returns the average duration
func (h *DurationHistogram) Average() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.count == 0 {
		return 0
	}
	return h.sum / time.Duration(h.count)
}

// Snapshot returns a snapshot of the histogram
func (h *DurationHistogram) Snapshot() map[string]uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()

	snapshot := make(map[string]uint64, len(h.buckets))
	for k, v := range h.buckets {
		snapshot[k] = v
	}
	return snapshot
}

// RecordBackupCreated increments backup created counter
func (m *Metrics) RecordBackupCreated(bytes uint64, duration time.Duration) {
	m.BackupsCreated.Add(1)
	m.BytesBackedUp.Add(bytes)
	m.BackupDuration.Observe(duration)
}

// RecordBackupFailed increments backup failed counter
func (m *Metrics) RecordBackupFailed() {
	m.BackupsFailed.Add(1)
	m.TotalErrors.Add(1)
}

// RecordRestoreCompleted increments restore completed counter
func (m *Metrics) RecordRestoreCompleted(bytes uint64, duration time.Duration) {
	m.RestoresCompleted.Add(1)
	m.BytesRestored.Add(bytes)
	m.RestoreDuration.Observe(duration)
}

// RecordRestoreFailed increments restore failed counter
func (m *Metrics) RecordRestoreFailed() {
	m.RestoresFailed.Add(1)
	m.TotalErrors.Add(1)
}

// RecordChunkStored increments chunks stored counter
func (m *Metrics) RecordChunkStored(size uint64, deduplicated bool) {
	m.ChunksStored.Add(1)
	if deduplicated {
		m.DeduplicatedChunks.Add(1)
	} else {
		m.TotalStorageUsed.Add(int64(size))
		m.BlocksStored.Add(1)
	}
}

// RecordChunkFetched increments chunks fetched counter
func (m *Metrics) RecordChunkFetched(duration time.Duration) {
	m.ChunksFetched.Add(1)
	m.ChunkFetchDuration.Observe(duration)
}

// RecordPeerConnected increments peer counter
func (m *Metrics) RecordPeerConnected() {
	m.PeersConnected.Add(1)
}

// RecordPeerDisconnected decrements peer counter
func (m *Metrics) RecordPeerDisconnected() {
	m.PeersConnected.Add(-1)
}

// RecordPeerDiscovered increments peer discovered counter
func (m *Metrics) RecordPeerDiscovered() {
	m.PeersDiscovered.Add(1)
}

// RecordMessageReceived increments message received counter
func (m *Metrics) RecordMessageReceived() {
	m.MessagesReceived.Add(1)
}

// RecordMessageSent increments message sent counter
func (m *Metrics) RecordMessageSent() {
	m.MessagesSent.Add(1)
}

// RecordChunkRequest tracks chunk request metrics
func (m *Metrics) RecordChunkRequest(sent bool, failed bool) {
	if sent {
		m.ChunkRequestsSent.Add(1)
	} else {
		m.ChunkRequestsReceived.Add(1)
	}
	if failed {
		m.ChunkRequestsFailed.Add(1)
		m.NetworkErrors.Add(1)
		m.TotalErrors.Add(1)
	}
}

// RecordGarbageCollection increments GC counter
func (m *Metrics) RecordGarbageCollection(blocksDeleted uint64, bytesFreed int64) {
	m.GarbageCollectionRuns.Add(1)
	m.BlocksDeleted.Add(blocksDeleted)
	m.TotalStorageUsed.Add(-bytesFreed)
}

// RecordError increments error counters
func (m *Metrics) RecordError(errorType string) {
	m.TotalErrors.Add(1)
	switch errorType {
	case "network":
		m.NetworkErrors.Add(1)
	case "storage":
		m.StorageErrors.Add(1)
	case "crypto":
		m.CryptoErrors.Add(1)
	}
}

// Global metrics instance
var globalMetrics = NewMetrics()

// GetMetrics returns the global metrics instance
func GetMetrics() *Metrics {
	return globalMetrics
}
