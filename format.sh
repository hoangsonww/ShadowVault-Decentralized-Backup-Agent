#!/usr/bin/env bash
set -euo pipefail

echo "[+] Formatting Go code with go fmt..."
go fmt ./...

echo "[+] Running goimports (requires goimports installed)..."
if command -v goimports >/dev/null; then
  goimports -w .
else
  echo "WARNING: goimports not found; install via 'go install golang.org/x/tools/cmd/goimports@latest'"
fi

echo "[+] Running golangci-lint (if installed)..."
if command -v golangci-lint >/dev/null; then
  golangci-lint run
else
  echo "WARNING: golangci-lint not installed; skipping lint step"
fi

echo "[+] Formatting C sources with clang-format..."
find . -name '*.c' -o -name '*.h' | while read -r f; do
  clang-format -i "$f"
done

echo "[+] All formatting steps complete."
