package p2p

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/hoangsonww/backupagent/internal/crypto"
	"github.com/hoangsonww/backupagent/internal/monitoring"
	"github.com/hoangsonww/backupagent/internal/protocol"
	"github.com/hoangsonww/backupagent/internal/storage"
	"github.com/hoangsonww/backupagent/internal/versioning"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// ChunkFetcher handles fetching missing chunks from peers
type ChunkFetcher struct {
	store          *storage.Store
	signerPub      []byte
	signerPriv     []byte
	maxConcurrent  int
	timeout        time.Duration
	pendingFetches sync.Map // hash -> chan []byte
	metrics        *monitoring.Metrics
}

// NewChunkFetcher creates a new chunk fetcher
func NewChunkFetcher(store *storage.Store, signerPub, signerPriv []byte, maxConcurrent int, timeout time.Duration) *ChunkFetcher {
	return &ChunkFetcher{
		store:         store,
		signerPub:     signerPub,
		signerPriv:    signerPriv,
		maxConcurrent: maxConcurrent,
		timeout:       timeout,
		metrics:       monitoring.GetMetrics(),
	}
}

// FetchChunk fetches a chunk from peers
func (cf *ChunkFetcher) FetchChunk(ctx context.Context, hash string, topic *pubsub.Topic, peerID string) ([]byte, error) {
	logger := monitoring.GetLogger().WithField("chunk_hash", hash)
	logger.Debug("Fetching chunk from peers")

	startTime := time.Now()
	defer func() {
		cf.metrics.RecordChunkFetched(time.Since(startTime))
	}()

	// Check if chunk already exists locally
	if data, err := cf.store.Get(hash); err == nil {
		logger.Debug("Chunk found in local storage")
		return data, nil
	}

	// Create request
	req := &protocol.ChunkRequest{
		Hash:      hash,
		Requestor: peerID,
		SignerPub: base64.StdEncoding.EncodeToString(cf.signerPub),
	}

	// Sign request
	payload := req.Hash + "|" + req.Requestor
	sig := crypto.Sign([]byte(payload), cf.signerPriv)
	req.Signature = base64.StdEncoding.EncodeToString(sig)

	// Encode request
	reqBytes, err := json.Marshal(map[string]interface{}{
		"type":    "chunk_request",
		"request": req,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	// Create response channel
	respChan := make(chan []byte, 1)
	cf.pendingFetches.Store(hash, respChan)
	defer cf.pendingFetches.Delete(hash)

	// Publish request
	if err := topic.Publish(ctx, reqBytes); err != nil {
		logger.WithError(err).Error("Failed to publish chunk request")
		cf.metrics.RecordChunkRequest(true, true)
		return nil, fmt.Errorf("failed to publish request: %w", err)
	}

	cf.metrics.RecordChunkRequest(true, false)
	logger.Debug("Chunk request published")

	// Wait for response with timeout
	select {
	case data := <-respChan:
		logger.Debug("Chunk received from peer")
		return data, nil
	case <-time.After(cf.timeout):
		logger.Warn("Chunk fetch timeout")
		cf.metrics.RecordChunkRequest(true, true)
		return nil, errors.New("chunk fetch timeout")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// HandleChunkResponse processes a chunk response
func (cf *ChunkFetcher) HandleChunkResponse(resp *protocol.ChunkResponse) error {
	logger := monitoring.GetLogger().WithField("chunk_hash", resp.Hash)

	// Validate response
	if err := resp.Validate(); err != nil {
		logger.WithError(err).Warn("Invalid chunk response signature")
		return fmt.Errorf("invalid chunk response: %w", err)
	}

	// Decode chunk data
	data, err := base64.StdEncoding.DecodeString(resp.Data)
	if err != nil {
		logger.WithError(err).Error("Failed to decode chunk data")
		return fmt.Errorf("failed to decode chunk data: %w", err)
	}

	// Verify chunk hash
	actualHash := hex.EncodeToString(crypto.Hash(data))
	if actualHash != resp.Hash {
		logger.Errorf("Chunk hash mismatch: expected %s, got %s", resp.Hash, actualHash)
		return errors.New("chunk hash mismatch")
	}

	// Store chunk
	if err := cf.store.Put(resp.Hash, data); err != nil {
		logger.WithError(err).Error("Failed to store chunk")
		return fmt.Errorf("failed to store chunk: %w", err)
	}

	// Notify waiting fetchers
	if ch, ok := cf.pendingFetches.Load(resp.Hash); ok {
		select {
		case ch.(chan []byte) <- data:
		default:
		}
	}

	logger.Debug("Chunk response processed successfully")
	return nil
}

// HandleChunkRequest processes a chunk request and sends response
func (cf *ChunkFetcher) HandleChunkRequest(ctx context.Context, req *protocol.ChunkRequest, topic *pubsub.Topic) error {
	logger := monitoring.GetLogger().WithField("chunk_hash", req.Hash)

	cf.metrics.RecordChunkRequest(false, false)

	// Validate request
	if err := req.Validate(); err != nil {
		logger.WithError(err).Warn("Invalid chunk request signature")
		cf.metrics.RecordChunkRequest(false, true)
		return fmt.Errorf("invalid chunk request: %w", err)
	}

	// Get chunk from storage
	data, err := cf.store.Get(req.Hash)
	if err != nil {
		logger.WithError(err).Debug("Chunk not found in local storage")
		// Don't respond if we don't have the chunk
		return nil
	}

	// Create response
	resp := &protocol.ChunkResponse{
		Hash:      req.Hash,
		Data:      base64.StdEncoding.EncodeToString(data),
		SignerPub: base64.StdEncoding.EncodeToString(cf.signerPub),
	}

	// Sign response
	payload := resp.Hash + "|" + resp.Data
	sig := crypto.Sign([]byte(payload), cf.signerPriv)
	resp.Signature = base64.StdEncoding.EncodeToString(sig)

	// Encode response
	respBytes, err := json.Marshal(map[string]interface{}{
		"type":     "chunk_response",
		"response": resp,
	})
	if err != nil {
		logger.WithError(err).Error("Failed to encode chunk response")
		cf.metrics.RecordChunkRequest(false, true)
		return fmt.Errorf("failed to encode response: %w", err)
	}

	// Publish response
	if err := topic.Publish(ctx, respBytes); err != nil {
		logger.WithError(err).Error("Failed to publish chunk response")
		cf.metrics.RecordChunkRequest(false, true)
		return fmt.Errorf("failed to publish response: %w", err)
	}

	logger.Debug("Chunk response sent successfully")
	return nil
}

// SnapshotSyncer handles snapshot synchronization
type SnapshotSyncer struct {
	store      *storage.Store
	fetcher    *ChunkFetcher
	signerPub  []byte
	signerPriv []byte
	metrics    *monitoring.Metrics
}

// NewSnapshotSyncer creates a new snapshot syncer
func NewSnapshotSyncer(store *storage.Store, fetcher *ChunkFetcher, signerPub, signerPriv []byte) *SnapshotSyncer {
	return &SnapshotSyncer{
		store:      store,
		fetcher:    fetcher,
		signerPub:  signerPub,
		signerPriv: signerPriv,
		metrics:    monitoring.GetMetrics(),
	}
}

// BroadcastSnapshot broadcasts a snapshot to peers
func (ss *SnapshotSyncer) BroadcastSnapshot(ctx context.Context, snapshot *versioning.Snapshot, topic *pubsub.Topic) error {
	logger := monitoring.GetLogger().WithField("snapshot_id", snapshot.ID)
	logger.Info("Broadcasting snapshot to peers")

	// Create announcement
	announcement := &protocol.SnapshotAnnouncement{
		Snapshot: *snapshot,
	}

	// Encode announcement
	annBytes, err := json.Marshal(map[string]interface{}{
		"type":         "snapshot_announcement",
		"announcement": announcement,
	})
	if err != nil {
		logger.WithError(err).Error("Failed to encode snapshot announcement")
		return fmt.Errorf("failed to encode announcement: %w", err)
	}

	// Publish announcement
	if err := topic.Publish(ctx, annBytes); err != nil {
		logger.WithError(err).Error("Failed to publish snapshot announcement")
		return fmt.Errorf("failed to publish announcement: %w", err)
	}

	ss.metrics.RecordMessageSent()
	logger.Info("Snapshot announcement broadcasted successfully")
	return nil
}

// HandleSnapshotAnnouncement processes a snapshot announcement
func (ss *SnapshotSyncer) HandleSnapshotAnnouncement(ctx context.Context, ann *protocol.SnapshotAnnouncement, topic *pubsub.Topic, peerID string, db interface{}) error {
	logger := monitoring.GetLogger().WithField("snapshot_id", ann.Snapshot.ID)
	logger.Info("Processing snapshot announcement")

	// Validate announcement
	if err := ann.Validate(); err != nil {
		logger.WithError(err).Warn("Invalid snapshot announcement signature")
		return fmt.Errorf("invalid announcement: %w", err)
	}

	// Check if we already have this snapshot
	// This would require a DB interface to check, simplified here
	logger.Infof("Received valid snapshot announcement: %s", ann.Snapshot.ID)

	// Fetch missing chunks in the background
	go ss.fetchMissingChunks(ctx, &ann.Snapshot, topic, peerID)

	return nil
}

// fetchMissingChunks fetches chunks that are missing locally
func (ss *SnapshotSyncer) fetchMissingChunks(ctx context.Context, snapshot *versioning.Snapshot, topic *pubsub.Topic, peerID string) {
	logger := monitoring.GetLogger().WithField("snapshot_id", snapshot.ID)

	// Create semaphore for concurrent fetches
	sem := make(chan struct{}, ss.fetcher.maxConcurrent)
	var wg sync.WaitGroup

	missingCount := 0
	for _, chunkHash := range snapshot.Chunks {
		// Check if chunk exists locally
		if _, err := ss.store.Get(chunkHash); err == nil {
			continue
		}

		missingCount++
		wg.Add(1)

		go func(hash string) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Fetch chunk
			if _, err := ss.fetcher.FetchChunk(ctx, hash, topic, peerID); err != nil {
				logger.WithError(err).Warnf("Failed to fetch chunk %s", hash)
			}
		}(chunkHash)
	}

	wg.Wait()
	logger.Infof("Finished fetching %d missing chunks for snapshot %s", missingCount, snapshot.ID)
}
