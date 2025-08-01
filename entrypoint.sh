#!/usr/bin/env bash
# entrypoint.sh — build, bootstrap, and get the backup agent running quickly.
# Usage:
#   ./entrypoint.sh [config.yaml] [optional: path-to-snapshot]
# Environment:
#   PASSPHRASE - if unset, you'll be prompted interactively.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG=${1:-config.yaml}
SNAP_PATH=${2:-}
PASS=${PASSPHRASE:-}

BIN_DIR="$REPO_ROOT/bin"
BACKUP_BIN="$BIN_DIR/backup-agent"
RESTORE_BIN="$BIN_DIR/restore-agent"
PEERCTL_BIN="$BIN_DIR/peerctl"

cleanup() {
    if [[ -n "${DAEMON_PID-}" ]]; then
        echo "[+] Shutting down daemon (pid $DAEMON_PID)"
        kill "$DAEMON_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

# Prompt for passphrase if not provided
if [[ -z "$PASS" ]]; then
    read -rsp "Enter encryption passphrase: " PASS
    echo
    if [[ -z "$PASS" ]]; then
        echo "Passphrase required" >&2
        exit 1
    fi
fi

# Build binaries
echo "[+] Building binaries..."
mkdir -p "$BIN_DIR"
go build -o "$BACKUP_BIN" ./cmd/backup-agent
go build -o "$RESTORE_BIN" ./cmd/restore-agent-restore
go build -o "$PEERCTL_BIN" ./cmd/peerctl
chmod +x "$BACKUP_BIN" "$RESTORE_BIN" "$PEERCTL_BIN"

# Ensure config exists
if [[ ! -f "$CONFIG" ]]; then
    echo "[+] Creating default config at $CONFIG"
    cat > "$CONFIG" <<'EOF'
repository_path: "./data"
listen_port: 9000
peer_bootstrap: []
nat_traversal:
  enable_auto_relay: true
  enable_hole_punching: true
snapshot:
  min_chunk_size: 2048
  max_chunk_size: 65536
  avg_chunk_size: 8192
acl:
  admins: []
EOF
fi

# Start daemon in background
echo "[+] Starting backup-agent daemon..."
"$BACKUP_BIN" daemon -c "$CONFIG" -p "$PASS" &
DAEMON_PID=$!
sleep 2

echo "[+] Daemon started (PID: $DAEMON_PID)"

# Optional initial snapshot
if [[ -n "$SNAP_PATH" ]]; then
    if [[ ! -d "$SNAP_PATH" && ! -f "$SNAP_PATH" ]]; then
        echo "[!] Snapshot path '$SNAP_PATH' does not exist" >&2
    else
        echo "[+] Taking initial snapshot of $SNAP_PATH"
        "$BACKUP_BIN" snapshot "$SNAP_PATH" -c "$CONFIG" -p "$PASS"
    fi
fi

cat <<EOF

✅ Setup complete.

Useful commands:
  List peers:      "$PEERCTL_BIN" -c "$CONFIG" -p "$PASS" list
  Add peer:        "$PEERCTL_BIN" -c "$CONFIG" -p "$PASS" add /ip4/1.2.3.4/tcp/9000/p2p/<peerID>
  Remove peer:     "$PEERCTL_BIN" -c "$CONFIG" -p "$PASS" remove <peerID>
  Restore snapshot:"$RESTORE_BIN" restore <snapshot-id> <target-dir> -c "$CONFIG" -p "$PASS"

To keep daemon alive, don't exit this script (or run it with nohup / systemd if you want persistence).

EOF

# Wait on daemon so script stays up unless killed
wait "$DAEMON_PID"
