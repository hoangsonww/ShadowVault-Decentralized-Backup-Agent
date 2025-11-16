package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	content := `
repository_path: "./test-data"
listen_port: 9001
peer_bootstrap:
  - "/ip4/127.0.0.1/tcp/9002/p2p/TestPeer"
nat_traversal:
  enable_auto_relay: true
  enable_hole_punching: true
snapshot:
  min_chunk_size: 4096
  max_chunk_size: 32768
  avg_chunk_size: 16384
  compression: true
acl:
  admins:
    - "test-admin-key"
p2p:
  max_peers: 100
  connection_timeout: 60s
  discovery_interval: 10m
monitoring:
  enable_metrics: true
  metrics_port: 9091
  log_level: debug
  log_format: json
storage:
  retention_days: 90
  verify_on_restore: true
security:
  enable_rate_limiting: true
  requests_per_second: 50
  burst_size: 100
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test loaded values
	if cfg.RepositoryPath != "./test-data" {
		t.Errorf("Expected repository_path './test-data', got '%s'", cfg.RepositoryPath)
	}
	if cfg.ListenPort != 9001 {
		t.Errorf("Expected listen_port 9001, got %d", cfg.ListenPort)
	}
	if cfg.Snapshot.MinChunkSize != 4096 {
		t.Errorf("Expected min_chunk_size 4096, got %d", cfg.Snapshot.MinChunkSize)
	}
	if cfg.Snapshot.Compression != true {
		t.Error("Expected compression to be enabled")
	}
	if cfg.P2P.MaxPeers != 100 {
		t.Errorf("Expected max_peers 100, got %d", cfg.P2P.MaxPeers)
	}
	if cfg.Storage.RetentionDays != 90 {
		t.Errorf("Expected retention_days 90, got %d", cfg.Storage.RetentionDays)
	}
}

func TestConfigDefaults(t *testing.T) {
	content := `
repository_path: "./data"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Test defaults
	if cfg.ListenPort != 9000 {
		t.Errorf("Expected default listen_port 9000, got %d", cfg.ListenPort)
	}
	if cfg.Snapshot.MinChunkSize != 2048 {
		t.Errorf("Expected default min_chunk_size 2048, got %d", cfg.Snapshot.MinChunkSize)
	}
	if cfg.Snapshot.MaxChunkSize != 65536 {
		t.Errorf("Expected default max_chunk_size 65536, got %d", cfg.Snapshot.MaxChunkSize)
	}
	if cfg.P2P.MaxPeers != 50 {
		t.Errorf("Expected default max_peers 50, got %d", cfg.P2P.MaxPeers)
	}
	if cfg.P2P.ConnectionTimeout != 30*time.Second {
		t.Errorf("Expected default connection_timeout 30s, got %v", cfg.P2P.ConnectionTimeout)
	}
	if cfg.Storage.RetentionDays != 30 {
		t.Errorf("Expected default retention_days 30, got %d", cfg.Storage.RetentionDays)
	}
	if cfg.Monitoring.LogLevel != "info" {
		t.Errorf("Expected default log_level 'info', got '%s'", cfg.Monitoring.LogLevel)
	}
	if cfg.Monitoring.LogFormat != "json" {
		t.Errorf("Expected default log_format 'json', got '%s'", cfg.Monitoring.LogFormat)
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	content := `
repository_path: "./data"
listen_port: 9000
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(content); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	// Set environment variables
	os.Setenv("SHADOWVAULT_REPO_PATH", "/custom/path")
	os.Setenv("SHADOWVAULT_LISTEN_PORT", "9999")
	os.Setenv("SHADOWVAULT_LOG_LEVEL", "debug")
	os.Setenv("SHADOWVAULT_ENABLE_COMPRESSION", "true")
	defer func() {
		os.Unsetenv("SHADOWVAULT_REPO_PATH")
		os.Unsetenv("SHADOWVAULT_LISTEN_PORT")
		os.Unsetenv("SHADOWVAULT_LOG_LEVEL")
		os.Unsetenv("SHADOWVAULT_ENABLE_COMPRESSION")
	}()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if cfg.RepositoryPath != "/custom/path" {
		t.Errorf("Expected repository_path '/custom/path', got '%s'", cfg.RepositoryPath)
	}
	if cfg.ListenPort != 9999 {
		t.Errorf("Expected listen_port 9999, got %d", cfg.ListenPort)
	}
	if cfg.Monitoring.LogLevel != "debug" {
		t.Errorf("Expected log_level 'debug', got '%s'", cfg.Monitoring.LogLevel)
	}
	if !cfg.Snapshot.Compression {
		t.Error("Expected compression to be enabled")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config",
			config: `
repository_path: "./data"
listen_port: 9000
`,
			expectError: false,
		},
		{
			name: "invalid port - too low",
			config: `
repository_path: "./data"
listen_port: -1
`,
			expectError: true,
			errorMsg:    "listen_port must be 1-65535",
		},
		{
			name: "invalid port - too high",
			config: `
repository_path: "./data"
listen_port: 70000
`,
			expectError: true,
			errorMsg:    "listen_port must be 1-65535",
		},
		{
			name: "invalid chunk sizes",
			config: `
repository_path: "./data"
snapshot:
  min_chunk_size: 10000
  max_chunk_size: 5000
`,
			expectError: true,
			errorMsg:    "max_chunk_size",
		},
		{
			name: "invalid log level",
			config: `
repository_path: "./data"
monitoring:
  log_level: invalid
`,
			expectError: true,
			errorMsg:    "invalid log_level",
		},
		{
			name: "invalid log format",
			config: `
repository_path: "./data"
monitoring:
  log_format: xml
`,
			expectError: true,
			errorMsg:    "invalid log_format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "config-*.yaml")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.config); err != nil {
				t.Fatalf("Failed to write temp file: %v", err)
			}
			tmpFile.Close()

			_, err = Load(tmpFile.Name())
			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.errorMsg != "" && !contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestSnapshotName(t *testing.T) {
	name := SnapshotName("test")
	if !contains(name, "test_") {
		t.Errorf("Expected snapshot name to start with 'test_', got '%s'", name)
	}
	if len(name) < 20 {
		t.Errorf("Expected snapshot name to include timestamp, got '%s'", name)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && hasSubstring(s, substr)))
}

func hasSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
