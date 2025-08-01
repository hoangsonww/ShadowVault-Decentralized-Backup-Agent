package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type NATConfig struct {
	EnableAutoRelay     bool `yaml:"enable_auto_relay"`
	EnableHolePunching  bool `yaml:"enable_hole_punching"`
}

type SnapshotConfig struct {
	MinChunkSize int `yaml:"min_chunk_size"`
	MaxChunkSize int `yaml:"max_chunk_size"`
	AvgChunkSize int `yaml:"avg_chunk_size"`
}

type ACLConfig struct {
	Admins []string `yaml:"admins"`
}

type Config struct {
	RepositoryPath string        `yaml:"repository_path"`
	ListenPort     int           `yaml:"listen_port"`
	PeerBootstrap  []string      `yaml:"peer_bootstrap"`
	NATTraversal   NATConfig     `yaml:"nat_traversal"`
	Snapshot       SnapshotConfig `yaml:"snapshot"`
	ACL            ACLConfig     `yaml:"acl"`
}

func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	decoder := yaml.NewDecoder(f)
	if err := decoder.Decode(&cfg); err != nil {
		return nil, err
	}
	// Defaults if missing
	if cfg.Snapshot.MinChunkSize == 0 {
		cfg.Snapshot.MinChunkSize = 2048
	}
	if cfg.Snapshot.MaxChunkSize == 0 {
		cfg.Snapshot.MaxChunkSize = 65536
	}
	if cfg.Snapshot.AvgChunkSize == 0 {
		cfg.Snapshot.AvgChunkSize = 8192
	}
	if cfg.ListenPort == 0 {
		cfg.ListenPort = 9000
	}
	if cfg.RepositoryPath == "" {
		cfg.RepositoryPath = "./data"
	}
	return &cfg, nil
}

// Helper for snapshot naming
func SnapshotName(prefix string) string {
	return prefix + "_" + time.Now().UTC().Format("20060102T150405Z")
}
