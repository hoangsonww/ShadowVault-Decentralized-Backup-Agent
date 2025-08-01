#!/usr/bin/env bash
set -euo pipefail

CONFIG=${1:-config.yaml}
PASS=${2:-"changeme"}

echo "[+] Using config: $CONFIG"
if [ ! -f "$CONFIG" ]; then
  cat <<EOF > "$CONFIG"
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
  echo "[+] Created default config at $CONFIG"
fi

echo "[+] Initializing identity & DB by spinning up agent briefly..."
./backup-agent daemon -c "$CONFIG" -p "$PASS" &
PID=$!
sleep 2
kill $PID || true
echo "[+] Bootstrap complete. Identity persisted and DB initialized."
