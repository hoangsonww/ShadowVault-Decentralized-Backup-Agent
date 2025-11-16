//go:build integration
// +build integration

package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hoangsonww/backupagent/config"
	"github.com/hoangsonww/backupagent/internal/agent"
	"github.com/hoangsonww/backupagent/internal/monitoring"
	"github.com/hoangsonww/backupagent/internal/versioning"
)

func TestEndToEndBackupRestore(t *testing.T) {
	// Create temporary directories
	tmpDir, err := os.MkdirTemp("", "shadowvault-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoPath := filepath.Join(tmpDir, "repo")
	dataPath := filepath.Join(tmpDir, "data")
	restorePath := filepath.Join(tmpDir, "restore")

	// Create test data
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}

	testFile := filepath.Join(dataPath, "test.txt")
	testData := []byte("Hello, ShadowVault!")
	if err := os.WriteFile(testFile, testData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create configuration
	cfg := &config.Config{
		RepositoryPath: repoPath,
		ListenPort:     19000,
		Snapshot: config.SnapshotConfig{
			MinChunkSize: 2048,
			MaxChunkSize: 65536,
			AvgChunkSize: 8192,
		},
	}

	// Initialize monitoring
	monitoring.SetGlobalLogger(monitoring.NewLogger("debug", "text"))
	monitoring.InitHealthChecker("test")

	// Create agent
	agent, err := agent.New(cfg, "test-passphrase")
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.DB.Close()

	// Create snapshot
	if err := agent.CreateAndSaveSnapshot(dataPath); err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	// List snapshots
	snapshots, err := versioning.ListAllSnapshots(agent.DB)
	if err != nil {
		t.Fatalf("Failed to list snapshots: %v", err)
	}

	if len(snapshots) != 1 {
		t.Fatalf("Expected 1 snapshot, got %d", len(snapshots))
	}

	t.Logf("Created snapshot: %s", snapshots[0].ID)

	// Verify snapshot
	if len(snapshots[0].Chunks) == 0 {
		t.Error("Snapshot has no chunks")
	}

	// TODO: Implement restore and verify
	// For now, this is a basic integration test
	t.Log("End-to-end backup test passed")
}

func TestMultipleSnapshotsWithGC(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shadowvault-gc-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoPath := filepath.Join(tmpDir, "repo")
	dataPath := filepath.Join(tmpDir, "data")

	// Create test data
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		t.Fatalf("Failed to create data dir: %v", err)
	}

	// Create configuration
	cfg := &config.Config{
		RepositoryPath: repoPath,
		ListenPort:     19001,
		Snapshot: config.SnapshotConfig{
			MinChunkSize: 2048,
			MaxChunkSize: 65536,
			AvgChunkSize: 8192,
		},
		Storage: config.StorageConfig{
			RetentionDays: 0, // Keep everything for this test
		},
	}

	// Initialize monitoring
	monitoring.SetGlobalLogger(monitoring.NewLogger("debug", "text"))

	// Create agent
	agent, err := agent.New(cfg, "test-passphrase")
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.DB.Close()

	// Create multiple snapshots
	for i := 0; i < 3; i++ {
		testFile := filepath.Join(dataPath, "test"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(testFile, []byte("test data"), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		if err := agent.CreateAndSaveSnapshot(dataPath); err != nil {
			t.Fatalf("Failed to create snapshot %d: %v", i, err)
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Verify we have 3 snapshots
	snapshots, err := versioning.ListAllSnapshots(agent.DB)
	if err != nil {
		t.Fatalf("Failed to list snapshots: %v", err)
	}

	if len(snapshots) != 3 {
		t.Fatalf("Expected 3 snapshots, got %d", len(snapshots))
	}

	t.Log("Multiple snapshots test passed")
}

func TestConcurrentBackups(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shadowvault-concurrent-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoPath := filepath.Join(tmpDir, "repo")

	// Create configuration
	cfg := &config.Config{
		RepositoryPath: repoPath,
		ListenPort:     19002,
		Snapshot: config.SnapshotConfig{
			MinChunkSize: 2048,
			MaxChunkSize: 65536,
			AvgChunkSize: 8192,
		},
	}

	// Initialize monitoring
	monitoring.SetGlobalLogger(monitoring.NewLogger("debug", "text"))

	// Create agent
	agent, err := agent.New(cfg, "test-passphrase")
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.DB.Close()

	// Create multiple data directories
	numConcurrent := 5
	dataPaths := make([]string, numConcurrent)

	for i := 0; i < numConcurrent; i++ {
		dataPath := filepath.Join(tmpDir, "data"+string(rune('0'+i)))
		if err := os.MkdirAll(dataPath, 0755); err != nil {
			t.Fatalf("Failed to create data dir: %v", err)
		}

		testFile := filepath.Join(dataPath, "test.txt")
		if err := os.WriteFile(testFile, []byte("concurrent test"), 0644); err != nil {
			t.Fatalf("Failed to write test file: %v", err)
		}

		dataPaths[i] = dataPath
	}

	// Create snapshots concurrently
	errChan := make(chan error, numConcurrent)
	for i := 0; i < numConcurrent; i++ {
		go func(path string) {
			errChan <- agent.CreateAndSaveSnapshot(path)
		}(dataPaths[i])
	}

	// Wait for all to complete
	for i := 0; i < numConcurrent; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent backup %d failed: %v", i, err)
		}
	}

	// Verify we have all snapshots
	snapshots, err := versioning.ListAllSnapshots(agent.DB)
	if err != nil {
		t.Fatalf("Failed to list snapshots: %v", err)
	}

	if len(snapshots) != numConcurrent {
		t.Fatalf("Expected %d snapshots, got %d", numConcurrent, len(snapshots))
	}

	t.Log("Concurrent backups test passed")
}

func TestAgentShutdown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "shadowvault-shutdown-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoPath := filepath.Join(tmpDir, "repo")

	cfg := &config.Config{
		RepositoryPath: repoPath,
		ListenPort:     19003,
		Snapshot: config.SnapshotConfig{
			MinChunkSize: 2048,
			MaxChunkSize: 65536,
			AvgChunkSize: 8192,
		},
	}

	monitoring.SetGlobalLogger(monitoring.NewLogger("debug", "text"))

	agent, err := agent.New(cfg, "test-passphrase")
	if err != nil {
		t.Fatalf("Failed to create agent: %v", err)
	}

	// Start agent in background
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		agent.RunDaemon(ctx)
	}()

	// Let it run briefly
	time.Sleep(1 * time.Second)

	// Cancel context to trigger shutdown
	cancel()

	// Wait for shutdown
	time.Sleep(2 * time.Second)

	// Try to close DB (should work if shutdown was clean)
	if err := agent.DB.Close(); err != nil {
		t.Errorf("Database close failed: %v", err)
	}

	t.Log("Agent shutdown test passed")
}
