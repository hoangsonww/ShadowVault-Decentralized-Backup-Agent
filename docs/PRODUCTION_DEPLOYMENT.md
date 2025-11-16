# ShadowVault Production Deployment Guide

## Table of Contents
- [Prerequisites](#prerequisites)
- [System Requirements](#system-requirements)
- [Installation](#installation)
- [Configuration](#configuration)
- [Security Hardening](#security-hardening)
- [Monitoring Setup](#monitoring-setup)
- [High Availability](#high-availability)
- [Backup & Recovery](#backup--recovery)
- [Troubleshooting](#troubleshooting)

## Prerequisites

### Required Software
- **Go**: 1.21 or higher
- **Docker**: 20.10+ (optional, for containerized deployment)
- **Prometheus**: For metrics collection (recommended)
- **Grafana**: For visualization (recommended)

### Network Requirements
- Open ports:
  - `9000`: P2P networking (configurable)
  - `8080`: Health checks
  - `9090`: Metrics endpoint
  - `6060`: Profiling (disable in production)

## System Requirements

### Minimum Requirements
- **CPU**: 2 cores
- **RAM**: 4GB
- **Disk**: 50GB SSD (for metadata and chunks)
- **Network**: 10 Mbps upload/download

### Recommended for Production
- **CPU**: 4+ cores
- **RAM**: 8GB+
- **Disk**: 500GB+ NVMe SSD
- **Network**: 100 Mbps+ with low latency

## Installation

### Method 1: Binary Installation

```bash
# Download latest release
wget https://github.com/hoangsonww/ShadowVault/releases/latest/download/shadowvault-linux-amd64.tar.gz

# Extract
tar -xzf shadowvault-linux-amd64.tar.gz

# Move binaries to system path
sudo mv shadowvault-* /usr/local/bin/

# Verify installation
shadowvault-backup-agent --version
```

### Method 2: Build from Source

```bash
# Clone repository
git clone https://github.com/hoangsonww/ShadowVault.git
cd ShadowVault

# Build
make build

# Install
sudo make install
```

### Method 3: Docker Deployment

```bash
# Pull image
docker pull ghcr.io/hoangsonww/shadowvault:latest

# Run container
docker run -d \
  --name shadowvault \
  -v /data/shadowvault:/data \
  -p 9000:9000 \
  -p 8080:8080 \
  -p 9090:9090 \
  -e SHADOWVAULT_LOG_LEVEL=info \
  ghcr.io/hoangsonww/shadowvault:latest
```

## Configuration

### Production Configuration Template

Create `/etc/shadowvault/config.yaml`:

```yaml
repository_path: "/var/lib/shadowvault/data"
listen_port: 9000

peer_bootstrap:
  - "/dns4/bootstrap1.shadowvault.io/tcp/9000/p2p/..."
  - "/dns4/bootstrap2.shadowvault.io/tcp/9000/p2p/..."

nat_traversal:
  enable_auto_relay: true
  enable_hole_punching: true

snapshot:
  min_chunk_size: 2048
  max_chunk_size: 65536
  avg_chunk_size: 8192
  compression: true

acl:
  admins:
    - "YOUR_ED25519_PUBLIC_KEY_BASE64"

p2p:
  max_peers: 100
  connection_timeout: 60s
  discovery_interval: 5m
  heartbeat_interval: 30s
  max_concurrent_fetch: 20
  chunk_fetch_timeout: 120s
  reconnect_backoff: 10s
  max_reconnect_backoff: 5m

storage:
  max_cache_size: 5368709120  # 5GB
  gc_interval: 6h
  retention_days: 90
  verify_on_restore: true
  enable_deduplication: true

monitoring:
  enable_metrics: true
  metrics_port: 9090
  enable_profiling: false  # DISABLE IN PRODUCTION
  profiling_port: 6060
  health_check_port: 8080
  log_level: info
  log_format: json
  enable_tracing: false
  tracing_endpoint: ""

scheduler:
  enable_auto_backup: true
  backup_interval: 24h
  backup_paths:
    - "/data/important"
    - "/var/lib/app"
  max_backup_retries: 3

security:
  enable_rate_limiting: true
  requests_per_second: 200
  burst_size: 400
  enable_ip_whitelist: false
  whitelisted_ips: []
  max_request_size: 104857600  # 100MB
```

### Environment Variables

```bash
# Core configuration
export SHADOWVAULT_REPO_PATH=/var/lib/shadowvault/data
export SHADOWVAULT_LISTEN_PORT=9000

# Logging
export SHADOWVAULT_LOG_LEVEL=info
export SHADOWVAULT_LOG_FORMAT=json

# Monitoring
export SHADOWVAULT_METRICS_PORT=9090

# Features
export SHADOWVAULT_ENABLE_COMPRESSION=true

# Bootstrap peers
export SHADOWVAULT_BOOTSTRAP_PEERS="peer1,peer2,peer3"
```

## Security Hardening

### 1. System User Setup

```bash
# Create dedicated user
sudo useradd -r -s /bin/false shadowvault

# Set ownership
sudo chown -R shadowvault:shadowvault /var/lib/shadowvault
sudo chmod 750 /var/lib/shadowvault
```

### 2. Systemd Service

Create `/etc/systemd/system/shadowvault.service`:

```ini
[Unit]
Description=ShadowVault Backup Agent
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=shadowvault
Group=shadowvault
ExecStart=/usr/local/bin/shadowvault-backup-agent \
  --config /etc/shadowvault/config.yaml
Restart=on-failure
RestartSec=10s
LimitNOFILE=65536
EnvironmentFile=-/etc/shadowvault/environment

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/shadowvault
ProtectKernelTunables=true
ProtectControlGroups=true
RestrictNamespaces=true
RestrictRealtime=true
RestrictSUIDSGID=true

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable shadowvault
sudo systemctl start shadowvault
sudo systemctl status shadowvault
```

### 3. Firewall Configuration

```bash
# Using UFW
sudo ufw allow 9000/tcp comment 'ShadowVault P2P'
sudo ufw allow from 10.0.0.0/8 to any port 8080 proto tcp comment 'Health checks internal'
sudo ufw allow from 10.0.0.0/8 to any port 9090 proto tcp comment 'Metrics internal'

# Using iptables
sudo iptables -A INPUT -p tcp --dport 9000 -j ACCEPT
sudo iptables -A INPUT -s 10.0.0.0/8 -p tcp --dport 8080 -j ACCEPT
sudo iptables -A INPUT -s 10.0.0.0/8 -p tcp --dport 9090 -j ACCEPT
```

### 4. TLS/SSL (Recommended for API)

Generate certificates:

```bash
# Self-signed for testing
openssl req -x509 -newkey rsa:4096 -keyout key.pem -out cert.pem -days 365 -nodes

# Production: Use Let's Encrypt
certbot certonly --standalone -d backup.yourdomain.com
```

## Monitoring Setup

### Prometheus Configuration

`/etc/prometheus/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'shadowvault'
    static_configs:
      - targets: ['localhost:9090']
    metric_relabel_configs:
      - source_labels: [__name__]
        regex: 'shadowvault_.*'
        action: keep
```

### Grafana Dashboard

1. Import dashboard from `docs/grafana-dashboard.json`
2. Configure Prometheus data source
3. Key metrics to monitor:
   - Backup success/failure rate
   - Storage usage
   - P2P peer count
   - Network throughput
   - Error rates

### Health Check Monitoring

```bash
# Add to monitoring system
curl http://localhost:8080/health

# Expected response:
{
  "status": "healthy",
  "timestamp": "2024-01-01T00:00:00Z",
  "uptime": "72h30m15s",
  "components": {
    "database": {"status": "healthy"},
    "p2p": {"status": "healthy"},
    "storage": {"status": "healthy"}
  }
}
```

## High Availability

### Multi-Node Setup

1. **Deploy multiple nodes**:
   ```bash
   # Node 1
   SHADOWVAULT_LISTEN_PORT=9000 shadowvault-backup-agent &

   # Node 2
   SHADOWVAULT_LISTEN_PORT=9001 shadowvault-backup-agent &

   # Node 3
   SHADOWVAULT_LISTEN_PORT=9002 shadowvault-backup-agent &
   ```

2. **Load balancer configuration** (HAProxy):
   ```
   frontend shadowvault_api
       bind *:8080
       default_backend shadowvault_nodes

   backend shadowvault_nodes
       balance roundrobin
       option httpchk GET /health
       server node1 10.0.1.1:8080 check
       server node2 10.0.1.2:8080 check
       server node3 10.0.1.3:8080 check
   ```

### Disaster Recovery

1. **Regular database backups**:
   ```bash
   # Backup metadata
   cp /var/lib/shadowvault/data/metadata.db /backups/metadata-$(date +%Y%m%d).db
   ```

2. **Snapshot exports**:
   ```bash
   # Export snapshots
   shadowvault-backup-agent export --snapshot-id <id> --output /backups/
   ```

## Backup & Recovery

### Creating Backups

```bash
# Manual backup
shadowvault-backup-agent snapshot /path/to/data

# Via API
curl -X POST http://localhost:8080/api/v1/backup \
  -H "Content-Type: application/json" \
  -d '{"path": "/path/to/data"}'
```

### Restoring Backups

```bash
# Command line
shadowvault-restore-agent --snapshot-id <id> --target /restore/path

# Via API
curl -X POST http://localhost:8080/api/v1/restore \
  -H "Content-Type: application/json" \
  -d '{"snapshot_id": "<id>", "target_path": "/restore/path"}'
```

### Verification

```bash
# Verify snapshot integrity
shadowvault-backup-agent verify --snapshot-id <id>

# Check metrics
curl http://localhost:9090/metrics | grep shadowvault_backups
```

## Troubleshooting

### Common Issues

#### 1. P2P Connection Failures

**Symptoms**: No peers connected, isolated node

**Solutions**:
```bash
# Check connectivity
nc -zv bootstrap.shadowvault.io 9000

# Verify NAT configuration
curl https://icanhazip.com

# Check logs
journalctl -u shadowvault -f | grep -i "p2p\|peer\|connection"
```

#### 2. High Memory Usage

**Symptoms**: OOM kills, slow performance

**Solutions**:
```yaml
# Reduce cache size in config
storage:
  max_cache_size: 1073741824  # 1GB

# Limit concurrent operations
p2p:
  max_concurrent_fetch: 5
```

#### 3. Backup Failures

**Symptoms**: Failed backups in logs

**Solutions**:
```bash
# Check disk space
df -h /var/lib/shadowvault

# Verify permissions
ls -la /var/lib/shadowvault

# Review error logs
journalctl -u shadowvault --since "1 hour ago" | grep -i error
```

### Log Analysis

```bash
# View JSON logs with jq
journalctl -u shadowvault -o cat | jq 'select(.level == "error")'

# Filter by component
journalctl -u shadowvault -o cat | jq 'select(.fields.component == "p2p")'

# Monitor in real-time
journalctl -u shadowvault -f --output=json | jq -r '.message'
```

### Performance Tuning

```yaml
# For high-throughput environments
p2p:
  max_concurrent_fetch: 50
  chunk_fetch_timeout: 30s

storage:
  max_cache_size: 10737418240  # 10GB

# For low-resource environments
p2p:
  max_concurrent_fetch: 3
  max_peers: 10

storage:
  max_cache_size: 536870912  # 512MB
```

## Support

- **Documentation**: https://github.com/hoangsonww/ShadowVault/docs
- **Issues**: https://github.com/hoangsonww/ShadowVault/issues
- **Discussions**: https://github.com/hoangsonww/ShadowVault/discussions
