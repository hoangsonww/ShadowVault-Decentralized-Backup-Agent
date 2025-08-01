#!/usr/bin/env bash
set -euo pipefail

SNAP_PATH=${1:?snapshot path}
PASS=${2:-"changeme"}
CONFIG=${3:-config.yaml}

echo "[+] Taking snapshot of $SNAP_PATH"
./backup-agent snapshot "$SNAP_PATH" -c "$CONFIG" -p "$PASS"
