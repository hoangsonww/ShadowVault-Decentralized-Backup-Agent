package monitoring

import (
	"context"
	"fmt"
	"net/http"
	"net/http/pprof"
	"time"
)

// MetricsServer serves metrics and health endpoints
type MetricsServer struct {
	metricsServer    *http.Server
	healthServer     *http.Server
	profilingServer  *http.Server
	metrics          *Metrics
	healthChecker    *HealthChecker
}

// NewMetricsServer creates a new metrics server
func NewMetricsServer(metricsPort, healthPort, profilingPort int, enableProfiling bool) *MetricsServer {
	ms := &MetricsServer{
		metrics:       GetMetrics(),
		healthChecker: GetHealthChecker(),
	}

	// Metrics server
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/metrics", ms.metricsHandler())
	ms.metricsServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", metricsPort),
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Health server
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", ms.healthChecker.HTTPHandler())
	healthMux.HandleFunc("/ready", ms.healthChecker.ReadinessHandler())
	healthMux.HandleFunc("/live", ms.healthChecker.LivenessHandler())
	ms.healthServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", healthPort),
		Handler:      healthMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Profiling server (optional)
	if enableProfiling {
		profilingMux := http.NewServeMux()
		profilingMux.HandleFunc("/debug/pprof/", pprof.Index)
		profilingMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		profilingMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		profilingMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		profilingMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		ms.profilingServer = &http.Server{
			Addr:         fmt.Sprintf(":%d", profilingPort),
			Handler:      profilingMux,
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 60 * time.Second,
		}
	}

	return ms
}

// Start starts the metrics servers
func (ms *MetricsServer) Start() error {
	logger := GetLogger()

	// Start metrics server
	go func() {
		logger.Infof("Starting metrics server on %s", ms.metricsServer.Addr)
		if err := ms.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Error("Metrics server error")
		}
	}()

	// Start health server
	go func() {
		logger.Infof("Starting health check server on %s", ms.healthServer.Addr)
		if err := ms.healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.WithError(err).Error("Health server error")
		}
	}()

	// Start profiling server if enabled
	if ms.profilingServer != nil {
		go func() {
			logger.Infof("Starting profiling server on %s", ms.profilingServer.Addr)
			if err := ms.profilingServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.WithError(err).Error("Profiling server error")
			}
		}()
	}

	return nil
}

// Stop gracefully stops the metrics servers
func (ms *MetricsServer) Stop(ctx context.Context) error {
	logger := GetLogger()
	logger.Info("Stopping metrics servers...")

	errChan := make(chan error, 3)

	// Stop metrics server
	go func() {
		if err := ms.metricsServer.Shutdown(ctx); err != nil {
			errChan <- fmt.Errorf("metrics server shutdown: %w", err)
		} else {
			errChan <- nil
		}
	}()

	// Stop health server
	go func() {
		if err := ms.healthServer.Shutdown(ctx); err != nil {
			errChan <- fmt.Errorf("health server shutdown: %w", err)
		} else {
			errChan <- nil
		}
	}()

	// Stop profiling server
	go func() {
		if ms.profilingServer != nil {
			if err := ms.profilingServer.Shutdown(ctx); err != nil {
				errChan <- fmt.Errorf("profiling server shutdown: %w", err)
			} else {
				errChan <- nil
			}
		} else {
			errChan <- nil
		}
	}()

	// Wait for all shutdowns
	for i := 0; i < 3; i++ {
		if err := <-errChan; err != nil {
			return err
		}
	}

	logger.Info("All metrics servers stopped")
	return nil
}

// metricsHandler returns Prometheus-style metrics
func (ms *MetricsServer) metricsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		// Backup metrics
		fmt.Fprintf(w, "# HELP shadowvault_backups_created_total Total number of backups created\n")
		fmt.Fprintf(w, "# TYPE shadowvault_backups_created_total counter\n")
		fmt.Fprintf(w, "shadowvault_backups_created_total %d\n", ms.metrics.BackupsCreated.Load())

		fmt.Fprintf(w, "# HELP shadowvault_backups_failed_total Total number of failed backups\n")
		fmt.Fprintf(w, "# TYPE shadowvault_backups_failed_total counter\n")
		fmt.Fprintf(w, "shadowvault_backups_failed_total %d\n", ms.metrics.BackupsFailed.Load())

		fmt.Fprintf(w, "# HELP shadowvault_restores_completed_total Total number of completed restores\n")
		fmt.Fprintf(w, "# TYPE shadowvault_restores_completed_total counter\n")
		fmt.Fprintf(w, "shadowvault_restores_completed_total %d\n", ms.metrics.RestoresCompleted.Load())

		fmt.Fprintf(w, "# HELP shadowvault_restores_failed_total Total number of failed restores\n")
		fmt.Fprintf(w, "# TYPE shadowvault_restores_failed_total counter\n")
		fmt.Fprintf(w, "shadowvault_restores_failed_total %d\n", ms.metrics.RestoresFailed.Load())

		fmt.Fprintf(w, "# HELP shadowvault_bytes_backed_up_total Total bytes backed up\n")
		fmt.Fprintf(w, "# TYPE shadowvault_bytes_backed_up_total counter\n")
		fmt.Fprintf(w, "shadowvault_bytes_backed_up_total %d\n", ms.metrics.BytesBackedUp.Load())

		fmt.Fprintf(w, "# HELP shadowvault_bytes_restored_total Total bytes restored\n")
		fmt.Fprintf(w, "# TYPE shadowvault_bytes_restored_total counter\n")
		fmt.Fprintf(w, "shadowvault_bytes_restored_total %d\n", ms.metrics.BytesRestored.Load())

		// Chunk metrics
		fmt.Fprintf(w, "# HELP shadowvault_chunks_stored_total Total number of chunks stored\n")
		fmt.Fprintf(w, "# TYPE shadowvault_chunks_stored_total counter\n")
		fmt.Fprintf(w, "shadowvault_chunks_stored_total %d\n", ms.metrics.ChunksStored.Load())

		fmt.Fprintf(w, "# HELP shadowvault_chunks_fetched_total Total number of chunks fetched\n")
		fmt.Fprintf(w, "# TYPE shadowvault_chunks_fetched_total counter\n")
		fmt.Fprintf(w, "shadowvault_chunks_fetched_total %d\n", ms.metrics.ChunksFetched.Load())

		fmt.Fprintf(w, "# HELP shadowvault_deduplicated_chunks_total Total number of deduplicated chunks\n")
		fmt.Fprintf(w, "# TYPE shadowvault_deduplicated_chunks_total counter\n")
		fmt.Fprintf(w, "shadowvault_deduplicated_chunks_total %d\n", ms.metrics.DeduplicatedChunks.Load())

		// P2P metrics
		fmt.Fprintf(w, "# HELP shadowvault_peers_connected Current number of connected peers\n")
		fmt.Fprintf(w, "# TYPE shadowvault_peers_connected gauge\n")
		fmt.Fprintf(w, "shadowvault_peers_connected %d\n", ms.metrics.PeersConnected.Load())

		fmt.Fprintf(w, "# HELP shadowvault_peers_discovered_total Total number of peers discovered\n")
		fmt.Fprintf(w, "# TYPE shadowvault_peers_discovered_total counter\n")
		fmt.Fprintf(w, "shadowvault_peers_discovered_total %d\n", ms.metrics.PeersDiscovered.Load())

		fmt.Fprintf(w, "# HELP shadowvault_messages_received_total Total messages received\n")
		fmt.Fprintf(w, "# TYPE shadowvault_messages_received_total counter\n")
		fmt.Fprintf(w, "shadowvault_messages_received_total %d\n", ms.metrics.MessagesReceived.Load())

		fmt.Fprintf(w, "# HELP shadowvault_messages_sent_total Total messages sent\n")
		fmt.Fprintf(w, "# TYPE shadowvault_messages_sent_total counter\n")
		fmt.Fprintf(w, "shadowvault_messages_sent_total %d\n", ms.metrics.MessagesSent.Load())

		// Storage metrics
		fmt.Fprintf(w, "# HELP shadowvault_storage_used_bytes Current storage usage in bytes\n")
		fmt.Fprintf(w, "# TYPE shadowvault_storage_used_bytes gauge\n")
		fmt.Fprintf(w, "shadowvault_storage_used_bytes %d\n", ms.metrics.TotalStorageUsed.Load())

		fmt.Fprintf(w, "# HELP shadowvault_blocks_stored_total Total blocks stored\n")
		fmt.Fprintf(w, "# TYPE shadowvault_blocks_stored_total counter\n")
		fmt.Fprintf(w, "shadowvault_blocks_stored_total %d\n", ms.metrics.BlocksStored.Load())

		fmt.Fprintf(w, "# HELP shadowvault_blocks_deleted_total Total blocks deleted\n")
		fmt.Fprintf(w, "# TYPE shadowvault_blocks_deleted_total counter\n")
		fmt.Fprintf(w, "shadowvault_blocks_deleted_total %d\n", ms.metrics.BlocksDeleted.Load())

		fmt.Fprintf(w, "# HELP shadowvault_gc_runs_total Total garbage collection runs\n")
		fmt.Fprintf(w, "# TYPE shadowvault_gc_runs_total counter\n")
		fmt.Fprintf(w, "shadowvault_gc_runs_total %d\n", ms.metrics.GarbageCollectionRuns.Load())

		// Error metrics
		fmt.Fprintf(w, "# HELP shadowvault_errors_total Total errors by type\n")
		fmt.Fprintf(w, "# TYPE shadowvault_errors_total counter\n")
		fmt.Fprintf(w, "shadowvault_errors_total{type=\"total\"} %d\n", ms.metrics.TotalErrors.Load())
		fmt.Fprintf(w, "shadowvault_errors_total{type=\"network\"} %d\n", ms.metrics.NetworkErrors.Load())
		fmt.Fprintf(w, "shadowvault_errors_total{type=\"storage\"} %d\n", ms.metrics.StorageErrors.Load())
		fmt.Fprintf(w, "shadowvault_errors_total{type=\"crypto\"} %d\n", ms.metrics.CryptoErrors.Load())

		// Duration histograms
		fmt.Fprintf(w, "# HELP shadowvault_backup_duration_seconds Backup duration distribution\n")
		fmt.Fprintf(w, "# TYPE shadowvault_backup_duration_seconds histogram\n")
		for bucket, count := range ms.metrics.BackupDuration.Snapshot() {
			fmt.Fprintf(w, "shadowvault_backup_duration_seconds{le=\"%s\"} %d\n", bucket, count)
		}
		fmt.Fprintf(w, "shadowvault_backup_duration_seconds_avg %.2f\n", ms.metrics.BackupDuration.Average().Seconds())
	}
}
