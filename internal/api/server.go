package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hoangsonww/backupagent/internal/agent"
	"github.com/hoangsonww/backupagent/internal/gc"
	"github.com/hoangsonww/backupagent/internal/monitoring"
	"github.com/hoangsonww/backupagent/internal/versioning"
)

// Server provides HTTP API for management
type Server struct {
	agent         *agent.Agent
	gc            *gc.Collector
	metrics       *monitoring.Metrics
	healthChecker *monitoring.HealthChecker
	server        *http.Server
}

// NewServer creates a new API server
func NewServer(agent *agent.Agent, gcCollector *gc.Collector, port int) *Server {
	s := &Server{
		agent:         agent,
		gc:            gcCollector,
		metrics:       monitoring.GetMetrics(),
		healthChecker: monitoring.GetHealthChecker(),
	}

	mux := http.NewServeMux()

	// Snapshot management
	mux.HandleFunc("/api/v1/snapshots", s.handleSnapshots)
	mux.HandleFunc("/api/v1/snapshots/create", s.handleCreateSnapshot)
	mux.HandleFunc("/api/v1/snapshots/", s.handleSnapshotDetail)

	// Backup operations
	mux.HandleFunc("/api/v1/backup", s.handleBackup)
	mux.HandleFunc("/api/v1/restore", s.handleRestore)

	// Garbage collection
	mux.HandleFunc("/api/v1/gc/run", s.handleRunGC)
	mux.HandleFunc("/api/v1/gc/status", s.handleGCStatus)

	// Metrics and monitoring
	mux.HandleFunc("/api/v1/metrics/summary", s.handleMetricsSummary)
	mux.HandleFunc("/api/v1/status", s.handleStatus)

	// Peer management
	mux.HandleFunc("/api/v1/peers", s.handlePeers)

	s.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      s.loggingMiddleware(s.corsMiddleware(mux)),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return s
}

// Start starts the API server
func (s *Server) Start() error {
	logger := monitoring.GetLogger()
	logger.Infof("Starting API server on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Stop gracefully stops the API server
func (s *Server) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// Middleware for logging
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		logger := monitoring.GetLogger()

		logger.WithFields(map[string]interface{}{
			"method": r.Method,
			"path":   r.URL.Path,
			"remote": r.RemoteAddr,
		}).Debug("API request")

		next.ServeHTTP(w, r)

		logger.WithFields(map[string]interface{}{
			"method":   r.Method,
			"path":     r.URL.Path,
			"duration": time.Since(start).Milliseconds(),
		}).Info("API request completed")
	})
}

// CORS middleware
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleSnapshots lists all snapshots
func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshots, err := versioning.ListAllSnapshots(s.agent.DB)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list snapshots: %v", err), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"snapshots": snapshots,
		"count":     len(snapshots),
	})
}

// handleCreateSnapshot creates a new snapshot
func (s *Server) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path string `json:"path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	go func() {
		if err := s.agent.CreateAndSaveSnapshot(req.Path); err != nil {
			monitoring.GetLogger().WithError(err).Error("Failed to create snapshot")
		}
	}()

	respondJSON(w, http.StatusAccepted, map[string]string{
		"status":  "accepted",
		"message": "Snapshot creation started",
	})
}

// handleSnapshotDetail returns details of a specific snapshot
func (s *Server) handleSnapshotDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Path[len("/api/v1/snapshots/"):]
	if id == "" {
		http.Error(w, "Snapshot ID required", http.StatusBadRequest)
		return
	}

	snapshot, err := versioning.LoadSnapshot(s.agent.DB, id)
	if err != nil {
		if err == versioning.ErrSnapshotNotFound {
			http.Error(w, "Snapshot not found", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to load snapshot: %v", err), http.StatusInternalServerError)
		}
		return
	}

	respondJSON(w, http.StatusOK, snapshot)
}

// handleBackup handles backup operations
func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	s.handleCreateSnapshot(w, r)
}

// handleRestore handles restore operations
func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SnapshotID string `json:"snapshot_id"`
		TargetPath string `json:"target_path"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SnapshotID == "" || req.TargetPath == "" {
		http.Error(w, "snapshot_id and target_path are required", http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]string{
		"status":  "accepted",
		"message": "Restore operation started",
	})
}

// handleRunGC triggers garbage collection
func (s *Server) handleRunGC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	go func() {
		if err := s.gc.RunOnce(); err != nil {
			monitoring.GetLogger().WithError(err).Error("Manual GC failed")
		}
	}()

	respondJSON(w, http.StatusAccepted, map[string]string{
		"status":  "accepted",
		"message": "Garbage collection started",
	})
}

// handleGCStatus returns GC status
func (s *Server) handleGCStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"gc_runs": s.metrics.GarbageCollectionRuns.Load(),
		"blocks_deleted": s.metrics.BlocksDeleted.Load(),
	})
}

// handleMetricsSummary returns metrics summary
func (s *Server) handleMetricsSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	summary := map[string]interface{}{
		"backups": map[string]interface{}{
			"created": s.metrics.BackupsCreated.Load(),
			"failed":  s.metrics.BackupsFailed.Load(),
		},
		"restores": map[string]interface{}{
			"completed": s.metrics.RestoresCompleted.Load(),
			"failed":    s.metrics.RestoresFailed.Load(),
		},
		"data": map[string]interface{}{
			"bytes_backed_up": s.metrics.BytesBackedUp.Load(),
			"bytes_restored":  s.metrics.BytesRestored.Load(),
		},
		"storage": map[string]interface{}{
			"total_used":    s.metrics.TotalStorageUsed.Load(),
			"blocks_stored": s.metrics.BlocksStored.Load(),
			"blocks_deleted": s.metrics.BlocksDeleted.Load(),
		},
		"p2p": map[string]interface{}{
			"peers_connected":  s.metrics.PeersConnected.Load(),
			"peers_discovered": s.metrics.PeersDiscovered.Load(),
			"messages_sent":    s.metrics.MessagesSent.Load(),
			"messages_received": s.metrics.MessagesReceived.Load(),
		},
		"errors": map[string]interface{}{
			"total":   s.metrics.TotalErrors.Load(),
			"network": s.metrics.NetworkErrors.Load(),
			"storage": s.metrics.StorageErrors.Load(),
			"crypto":  s.metrics.CryptoErrors.Load(),
		},
	}

	respondJSON(w, http.StatusOK, summary)
}

// handleStatus returns overall system status
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	health := s.healthChecker.GetHealth()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"health":   health,
		"p2p_id":   s.agent.P2P.Host.ID().String(),
		"peers":    len(s.agent.P2P.Host.Network().Peers()),
	})
}

// handlePeers returns connected peers
func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	peers := s.agent.P2P.Host.Network().Peers()
	peerList := make([]map[string]interface{}, 0, len(peers))

	for _, peerID := range peers {
		peerInfo := s.agent.P2P.Host.Peerstore().PeerInfo(peerID)
		peerList = append(peerList, map[string]interface{}{
			"id":    peerID.String(),
			"addrs": peerInfo.Addrs,
		})
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"peers": peerList,
		"count": len(peerList),
	})
}

// respondJSON writes a JSON response
func respondJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
