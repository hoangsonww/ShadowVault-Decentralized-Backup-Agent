package agent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/hoangsonww/backupagent/config"
	"github.com/hoangsonww/backupagent/internal/auth"
	"github.com/hoangsonww/backupagent/internal/crypto"
	"github.com/hoangsonww/backupagent/internal/monitoring"
	"github.com/hoangsonww/backupagent/internal/p2p"
	"github.com/hoangsonww/backupagent/internal/persistence"
	"github.com/hoangsonww/backupagent/internal/protocol"
	"github.com/hoangsonww/backupagent/internal/storage"
	"github.com/hoangsonww/backupagent/internal/versioning"
	"github.com/hoangsonww/backupagent/snapshots"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

type Agent struct {
	Config    *config.Config
	DB        *persistence.DB
	Store     *storage.Store
	P2P       *p2p.P2PHost
	ACL       *auth.ACL
	SignerPub []byte
	SignerPriv []byte
}

func New(cfg *config.Config, passphrase string) (*Agent, error) {
	// Open DB
	dbPath := filepath.Join(cfg.RepositoryPath, "metadata.db")
	db, err := persistence.Open(dbPath)
	if err != nil {
		return nil, err
	}
	// derive master key
	key := crypto.DeriveKey(passphrase, nil)
	store, err := storage.New(db, key)
	if err != nil {
		return nil, err
	}
	// Load ACL
	acl := auth.NewACL(cfg.ACL.Admins)

	// Generate or load identity keypair for signing / peer identity
	pub, priv, err := crypto.GenerateEd25519Keypair()
	if err != nil {
		return nil, err
	}

	// Setup P2P with libp2p
	p2phost, err := p2p.Setup(cfg, nil, store, pub, priv)
	if err != nil {
		return nil, err
	}

	agent := &Agent{
		Config:    cfg,
		DB:        db,
		Store:     store,
		P2P:       p2phost,
		ACL:       acl,
		SignerPub: pub,
		SignerPriv: priv,
	}
	return agent, nil
}

func (a *Agent) RunDaemon(ctx context.Context) error {
	// Subscribe to sync topic, respond to incoming updates
	sub, err := a.P2P.Topic.Subscribe()
	if err != nil {
		return err
	}
	go a.handlePubSub(sub)

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-c:
		fmt.Println("Shutting down")
		a.P2P.Cancel()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (a *Agent) handlePubSub(sub *pubsub.Subscription) {
	logger := monitoring.GetLogger()

	for {
		msg, err := sub.Next(a.P2P.Ctx)
		if err != nil {
			logger.WithError(err).Debug("PubSub subscription ended")
			return
		}

		// Record metric
		monitoring.GetMetrics().RecordMessageReceived()

		// Parse message
		var envelope map[string]interface{}
		if err := json.Unmarshal(msg.Data, &envelope); err != nil {
			logger.WithError(err).Warn("Failed to parse pubsub message")
			continue
		}

		msgType, ok := envelope["type"].(string)
		if !ok {
			logger.Warn("Message missing type field")
			continue
		}

		// Handle different message types
		switch msgType {
		case "snapshot_announcement":
			a.handleSnapshotAnnouncement(envelope, msg.GetFrom().String())
		case "chunk_request":
			a.handleChunkRequest(envelope)
		case "chunk_response":
			a.handleChunkResponse(envelope)
		case "peer_add":
			a.handlePeerAdd(envelope)
		case "peer_remove":
			a.handlePeerRemove(envelope)
		default:
			logger.Warnf("Unknown message type: %s", msgType)
		}
	}
}

func (a *Agent) handleSnapshotAnnouncement(envelope map[string]interface{}, peerID string) {
	logger := monitoring.GetLogger()

	annData, err := json.Marshal(envelope["announcement"])
	if err != nil {
		logger.WithError(err).Error("Failed to marshal snapshot announcement")
		return
	}

	var ann protocol.SnapshotAnnouncement
	if err := json.Unmarshal(annData, &ann); err != nil {
		logger.WithError(err).Error("Failed to unmarshal snapshot announcement")
		return
	}

	// Use snapshot syncer to handle announcement
	syncer := p2p.NewSnapshotSyncer(a.Store, a.P2P.ChunkFetcher, a.SignerPub, a.SignerPriv)
	if err := syncer.HandleSnapshotAnnouncement(a.P2P.Ctx, &ann, a.P2P.Topic, peerID, a.DB); err != nil {
		logger.WithError(err).Error("Failed to handle snapshot announcement")
	}
}

func (a *Agent) handleChunkRequest(envelope map[string]interface{}) {
	logger := monitoring.GetLogger()

	reqData, err := json.Marshal(envelope["request"])
	if err != nil {
		logger.WithError(err).Error("Failed to marshal chunk request")
		return
	}

	var req protocol.ChunkRequest
	if err := json.Unmarshal(reqData, &req); err != nil {
		logger.WithError(err).Error("Failed to unmarshal chunk request")
		return
	}

	// Handle request using chunk fetcher
	if err := a.P2P.ChunkFetcher.HandleChunkRequest(a.P2P.Ctx, &req, a.P2P.Topic); err != nil {
		logger.WithError(err).Error("Failed to handle chunk request")
	}
}

func (a *Agent) handleChunkResponse(envelope map[string]interface{}) {
	logger := monitoring.GetLogger()

	respData, err := json.Marshal(envelope["response"])
	if err != nil {
		logger.WithError(err).Error("Failed to marshal chunk response")
		return
	}

	var resp protocol.ChunkResponse
	if err := json.Unmarshal(respData, &resp); err != nil {
		logger.WithError(err).Error("Failed to unmarshal chunk response")
		return
	}

	// Handle response using chunk fetcher
	if err := a.P2P.ChunkFetcher.HandleChunkResponse(&resp); err != nil {
		logger.WithError(err).Error("Failed to handle chunk response")
	}
}

func (a *Agent) handlePeerAdd(envelope map[string]interface{}) {
	logger := monitoring.GetLogger()

	addData, err := json.Marshal(envelope["peer_add"])
	if err != nil {
		logger.WithError(err).Error("Failed to marshal peer add")
		return
	}

	var peerAdd protocol.PeerAdd
	if err := json.Unmarshal(addData, &peerAdd); err != nil {
		logger.WithError(err).Error("Failed to unmarshal peer add")
		return
	}

	// Validate signature
	if err := peerAdd.Validate(); err != nil {
		logger.WithError(err).Warn("Invalid peer add signature")
		return
	}

	// Check ACL
	pubKey, err := base64.StdEncoding.DecodeString(peerAdd.SignerPub)
	if err != nil || !a.ACL.IsAdmin(pubKey) {
		logger.Warn("Peer add from non-admin, ignoring")
		return
	}

	logger.Infof("Peer add validated: %s at %s", peerAdd.PeerID, peerAdd.Addr)
	monitoring.GetMetrics().RecordPeerDiscovered()
}

func (a *Agent) handlePeerRemove(envelope map[string]interface{}) {
	logger := monitoring.GetLogger()

	removeData, err := json.Marshal(envelope["peer_remove"])
	if err != nil {
		logger.WithError(err).Error("Failed to marshal peer remove")
		return
	}

	var peerRemove protocol.PeerRemove
	if err := json.Unmarshal(removeData, &peerRemove); err != nil {
		logger.WithError(err).Error("Failed to unmarshal peer remove")
		return
	}

	// Validate signature
	if err := peerRemove.Validate(); err != nil {
		logger.WithError(err).Warn("Invalid peer remove signature")
		return
	}

	// Check ACL
	pubKey, err := base64.StdEncoding.DecodeString(peerRemove.SignerPub)
	if err != nil || !a.ACL.IsAdmin(pubKey) {
		logger.Warn("Peer remove from non-admin, ignoring")
		return
	}

	logger.Infof("Peer remove validated: %s", peerRemove.PeerID)
}

func (a *Agent) CreateAndSaveSnapshot(path string) error {
	logger := monitoring.GetLogger().WithField("path", path)
	startTime := time.Now()

	logger.Info("Creating snapshot")
	snap, err := snapshots.CreateSnapshot(path, a.Store, a.SignerPub, a.SignerPriv, "", a.Config.Snapshot.MinChunkSize, a.Config.Snapshot.MaxChunkSize, a.Config.Snapshot.AvgChunkSize)
	if err != nil {
		logger.WithError(err).Error("Failed to create snapshot")
		monitoring.GetMetrics().RecordBackupFailed()
		return err
	}

	logger.WithField("snapshot_id", snap.ID).Info("Saving snapshot to database")
	if err := versioning.SaveSnapshot(a.DB, snap); err != nil {
		logger.WithError(err).Error("Failed to save snapshot")
		monitoring.GetMetrics().RecordBackupFailed()
		return err
	}

	// Calculate total bytes backed up
	var totalBytes uint64
	for _, chunkHash := range snap.Chunks {
		if data, err := a.Store.Get(chunkHash); err == nil {
			totalBytes += uint64(len(data))
		}
	}

	// Record metrics
	duration := time.Since(startTime)
	monitoring.GetMetrics().RecordBackupCreated(totalBytes, duration)

	// Broadcast metadata to peers
	logger.Info("Broadcasting snapshot to peers")
	syncer := p2p.NewSnapshotSyncer(a.Store, a.P2P.ChunkFetcher, a.SignerPub, a.SignerPriv)
	if err := syncer.BroadcastSnapshot(a.P2P.Ctx, snap, a.P2P.Topic); err != nil {
		logger.WithError(err).Warn("Failed to broadcast snapshot (snapshot saved locally)")
		// Don't fail the entire operation if broadcast fails
	}

	logger.WithFields(map[string]interface{}{
		"snapshot_id": snap.ID,
		"chunks":      len(snap.Chunks),
		"bytes":       totalBytes,
		"duration":    duration.Seconds(),
	}).Info("Snapshot created and broadcasted successfully")

	return nil
}
