package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type NATConfig struct {
	EnableAutoRelay    bool `yaml:"enable_auto_relay"`
	EnableHolePunching bool `yaml:"enable_hole_punching"`
}

type SnapshotConfig struct {
	MinChunkSize int  `yaml:"min_chunk_size"`
	MaxChunkSize int  `yaml:"max_chunk_size"`
	AvgChunkSize int  `yaml:"avg_chunk_size"`
	Compression  bool `yaml:"compression"`
}

type ACLConfig struct {
	Admins []string `yaml:"admins"`
}

type P2PConfig struct {
	MaxPeers            int           `yaml:"max_peers"`
	ConnectionTimeout   time.Duration `yaml:"connection_timeout"`
	DiscoveryInterval   time.Duration `yaml:"discovery_interval"`
	HeartbeatInterval   time.Duration `yaml:"heartbeat_interval"`
	MaxConcurrentFetch  int           `yaml:"max_concurrent_fetch"`
	ChunkFetchTimeout   time.Duration `yaml:"chunk_fetch_timeout"`
	ReconnectBackoff    time.Duration `yaml:"reconnect_backoff"`
	MaxReconnectBackoff time.Duration `yaml:"max_reconnect_backoff"`
}

type StorageConfig struct {
	MaxCacheSize       int64         `yaml:"max_cache_size"`
	GCInterval         time.Duration `yaml:"gc_interval"`
	RetentionDays      int           `yaml:"retention_days"`
	VerifyOnRestore    bool          `yaml:"verify_on_restore"`
	EnableDeduplication bool         `yaml:"enable_deduplication"`
}

type MonitoringConfig struct {
	EnableMetrics     bool   `yaml:"enable_metrics"`
	MetricsPort       int    `yaml:"metrics_port"`
	EnableProfiling   bool   `yaml:"enable_profiling"`
	ProfilingPort     int    `yaml:"profiling_port"`
	HealthCheckPort   int    `yaml:"health_check_port"`
	LogLevel          string `yaml:"log_level"`
	LogFormat         string `yaml:"log_format"` // "json" or "text"
	EnableTracing     bool   `yaml:"enable_tracing"`
	TracingEndpoint   string `yaml:"tracing_endpoint"`
}

type SchedulerConfig struct {
	EnableAutoBackup bool          `yaml:"enable_auto_backup"`
	BackupInterval   time.Duration `yaml:"backup_interval"`
	BackupPaths      []string      `yaml:"backup_paths"`
	MaxBackupRetries int           `yaml:"max_backup_retries"`
}

type SecurityConfig struct {
	EnableRateLimiting bool   `yaml:"enable_rate_limiting"`
	RequestsPerSecond  int    `yaml:"requests_per_second"`
	BurstSize          int    `yaml:"burst_size"`
	EnableIPWhitelist  bool   `yaml:"enable_ip_whitelist"`
	WhitelistedIPs     []string `yaml:"whitelisted_ips"`
	MaxRequestSize     int64  `yaml:"max_request_size"`
}

type Config struct {
	RepositoryPath string           `yaml:"repository_path"`
	ListenPort     int              `yaml:"listen_port"`
	PeerBootstrap  []string         `yaml:"peer_bootstrap"`
	NATTraversal   NATConfig        `yaml:"nat_traversal"`
	Snapshot       SnapshotConfig   `yaml:"snapshot"`
	ACL            ACLConfig        `yaml:"acl"`
	P2P            P2PConfig        `yaml:"p2p"`
	Storage        StorageConfig    `yaml:"storage"`
	Monitoring     MonitoringConfig `yaml:"monitoring"`
	Scheduler      SchedulerConfig  `yaml:"scheduler"`
	Security       SecurityConfig   `yaml:"security"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	var cfg Config
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Override with environment variables
	cfg.applyEnvironmentOverrides()

	// Apply defaults
	cfg.applyDefaults()

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &cfg, nil
}

// applyEnvironmentOverrides overrides config values with environment variables if set
func (c *Config) applyEnvironmentOverrides() {
	if val := os.Getenv("SHADOWVAULT_REPO_PATH"); val != "" {
		c.RepositoryPath = val
	}
	if val := os.Getenv("SHADOWVAULT_LISTEN_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			c.ListenPort = port
		}
	}
	if val := os.Getenv("SHADOWVAULT_LOG_LEVEL"); val != "" {
		c.Monitoring.LogLevel = val
	}
	if val := os.Getenv("SHADOWVAULT_LOG_FORMAT"); val != "" {
		c.Monitoring.LogFormat = val
	}
	if val := os.Getenv("SHADOWVAULT_METRICS_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			c.Monitoring.MetricsPort = port
		}
	}
	if val := os.Getenv("SHADOWVAULT_BOOTSTRAP_PEERS"); val != "" {
		c.PeerBootstrap = strings.Split(val, ",")
	}
	if val := os.Getenv("SHADOWVAULT_ENABLE_COMPRESSION"); val != "" {
		c.Snapshot.Compression = val == "true" || val == "1"
	}
}

// applyDefaults sets default values for unset configuration fields
func (c *Config) applyDefaults() {
	// Snapshot defaults
	if c.Snapshot.MinChunkSize == 0 {
		c.Snapshot.MinChunkSize = 2048
	}
	if c.Snapshot.MaxChunkSize == 0 {
		c.Snapshot.MaxChunkSize = 65536
	}
	if c.Snapshot.AvgChunkSize == 0 {
		c.Snapshot.AvgChunkSize = 8192
	}

	// Network defaults
	if c.ListenPort == 0 {
		c.ListenPort = 9000
	}

	// Repository defaults
	if c.RepositoryPath == "" {
		c.RepositoryPath = "./data"
	}

	// P2P defaults
	if c.P2P.MaxPeers == 0 {
		c.P2P.MaxPeers = 50
	}
	if c.P2P.ConnectionTimeout == 0 {
		c.P2P.ConnectionTimeout = 30 * time.Second
	}
	if c.P2P.DiscoveryInterval == 0 {
		c.P2P.DiscoveryInterval = 5 * time.Minute
	}
	if c.P2P.HeartbeatInterval == 0 {
		c.P2P.HeartbeatInterval = 30 * time.Second
	}
	if c.P2P.MaxConcurrentFetch == 0 {
		c.P2P.MaxConcurrentFetch = 10
	}
	if c.P2P.ChunkFetchTimeout == 0 {
		c.P2P.ChunkFetchTimeout = 60 * time.Second
	}
	if c.P2P.ReconnectBackoff == 0 {
		c.P2P.ReconnectBackoff = 5 * time.Second
	}
	if c.P2P.MaxReconnectBackoff == 0 {
		c.P2P.MaxReconnectBackoff = 5 * time.Minute
	}

	// Storage defaults
	if c.Storage.MaxCacheSize == 0 {
		c.Storage.MaxCacheSize = 1024 * 1024 * 1024 // 1GB
	}
	if c.Storage.GCInterval == 0 {
		c.Storage.GCInterval = 24 * time.Hour
	}
	if c.Storage.RetentionDays == 0 {
		c.Storage.RetentionDays = 30
	}
	c.Storage.VerifyOnRestore = true // Always verify by default
	c.Storage.EnableDeduplication = true

	// Monitoring defaults
	if c.Monitoring.MetricsPort == 0 {
		c.Monitoring.MetricsPort = 9090
	}
	if c.Monitoring.ProfilingPort == 0 {
		c.Monitoring.ProfilingPort = 6060
	}
	if c.Monitoring.HealthCheckPort == 0 {
		c.Monitoring.HealthCheckPort = 8080
	}
	if c.Monitoring.LogLevel == "" {
		c.Monitoring.LogLevel = "info"
	}
	if c.Monitoring.LogFormat == "" {
		c.Monitoring.LogFormat = "json"
	}

	// Scheduler defaults
	if c.Scheduler.BackupInterval == 0 {
		c.Scheduler.BackupInterval = 24 * time.Hour
	}
	if c.Scheduler.MaxBackupRetries == 0 {
		c.Scheduler.MaxBackupRetries = 3
	}

	// Security defaults
	if c.Security.RequestsPerSecond == 0 {
		c.Security.RequestsPerSecond = 100
	}
	if c.Security.BurstSize == 0 {
		c.Security.BurstSize = 200
	}
	if c.Security.MaxRequestSize == 0 {
		c.Security.MaxRequestSize = 100 * 1024 * 1024 // 100MB
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate chunk sizes
	if c.Snapshot.MinChunkSize <= 0 {
		return fmt.Errorf("min_chunk_size must be > 0, got %d", c.Snapshot.MinChunkSize)
	}
	if c.Snapshot.MaxChunkSize <= c.Snapshot.MinChunkSize {
		return fmt.Errorf("max_chunk_size (%d) must be > min_chunk_size (%d)",
			c.Snapshot.MaxChunkSize, c.Snapshot.MinChunkSize)
	}
	if c.Snapshot.AvgChunkSize < c.Snapshot.MinChunkSize ||
	   c.Snapshot.AvgChunkSize > c.Snapshot.MaxChunkSize {
		return fmt.Errorf("avg_chunk_size (%d) must be between min (%d) and max (%d)",
			c.Snapshot.AvgChunkSize, c.Snapshot.MinChunkSize, c.Snapshot.MaxChunkSize)
	}

	// Validate ports
	if c.ListenPort < 1 || c.ListenPort > 65535 {
		return fmt.Errorf("listen_port must be 1-65535, got %d", c.ListenPort)
	}
	if c.Monitoring.EnableMetrics && (c.Monitoring.MetricsPort < 1 || c.Monitoring.MetricsPort > 65535) {
		return fmt.Errorf("metrics_port must be 1-65535, got %d", c.Monitoring.MetricsPort)
	}
	if c.Monitoring.EnableProfiling && (c.Monitoring.ProfilingPort < 1 || c.Monitoring.ProfilingPort > 65535) {
		return fmt.Errorf("profiling_port must be 1-65535, got %d", c.Monitoring.ProfilingPort)
	}
	if c.Monitoring.HealthCheckPort < 1 || c.Monitoring.HealthCheckPort > 65535 {
		return fmt.Errorf("health_check_port must be 1-65535, got %d", c.Monitoring.HealthCheckPort)
	}

	// Validate repository path
	if c.RepositoryPath == "" {
		return fmt.Errorf("repository_path cannot be empty")
	}

	// Validate P2P settings
	if c.P2P.MaxPeers < 1 {
		return fmt.Errorf("max_peers must be >= 1, got %d", c.P2P.MaxPeers)
	}
	if c.P2P.MaxConcurrentFetch < 1 {
		return fmt.Errorf("max_concurrent_fetch must be >= 1, got %d", c.P2P.MaxConcurrentFetch)
	}

	// Validate storage settings
	if c.Storage.RetentionDays < 0 {
		return fmt.Errorf("retention_days must be >= 0, got %d", c.Storage.RetentionDays)
	}
	if c.Storage.MaxCacheSize < 0 {
		return fmt.Errorf("max_cache_size must be >= 0, got %d", c.Storage.MaxCacheSize)
	}

	// Validate log level
	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true, "fatal": true,
	}
	if !validLogLevels[strings.ToLower(c.Monitoring.LogLevel)] {
		return fmt.Errorf("invalid log_level: %s (must be debug, info, warn, error, or fatal)",
			c.Monitoring.LogLevel)
	}

	// Validate log format
	if c.Monitoring.LogFormat != "json" && c.Monitoring.LogFormat != "text" {
		return fmt.Errorf("invalid log_format: %s (must be json or text)", c.Monitoring.LogFormat)
	}

	// Validate security settings
	if c.Security.EnableRateLimiting {
		if c.Security.RequestsPerSecond < 1 {
			return fmt.Errorf("requests_per_second must be >= 1, got %d", c.Security.RequestsPerSecond)
		}
		if c.Security.BurstSize < c.Security.RequestsPerSecond {
			return fmt.Errorf("burst_size (%d) must be >= requests_per_second (%d)",
				c.Security.BurstSize, c.Security.RequestsPerSecond)
		}
	}

	return nil
}

// Helper for snapshot naming
func SnapshotName(prefix string) string {
	return prefix + "_" + time.Now().UTC().Format("20060102T150405Z")
}
