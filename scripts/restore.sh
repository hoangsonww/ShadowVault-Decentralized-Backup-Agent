#!/usr/bin/env bash
set -euo pipefail

SNAPID=${1:?snapshot-id}
TARGET=${2:-restored}
PASS=${3:-"changeme"}
CONFIG=${4:-config.yaml}

echo "[+] Restoring snapshot $SNAPID to $TARGET"
./restore-agent restore "$SNAPID" "$TARGET" -c "$CONFIG" -p "$PASS"
