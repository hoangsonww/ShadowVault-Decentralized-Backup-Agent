#!/usr/bin/env bash
set -euo pipefail

echo "[+] Checking gofmt..."
gofmt_output=$(gofmt -l .)
if [[ -n "$gofmt_output" ]]; then
  echo "The following Go files need formatting (gofmt):"
  echo "$gofmt_output"
  exit 1
fi

echo "[+] Checking goimports..."
if command -v goimports >/dev/null; then
  goimports_output=$(goimports -l .)
  if [[ -n "$goimports_output" ]]; then
    echo "The following Go files need goimports fixes:"
    echo "$goimports_output"
    exit 1
  fi
else
  echo "WARNING: goimports not installed; skipping its check"
fi

echo "[+] Running golangci-lint..."
if command -v golangci-lint >/dev/null; then
  if ! golangci-lint run; then
    echo "golangci-lint found issues"
    exit 1
  fi
else
  echo "WARNING: golangci-lint not installed; skipping lint check"
fi

echo "[+] Verifying C formatting with clang-format..."
bad=0
while IFS= read -r f; do
  formatted=$(clang-format "$f")
  if ! diff -u <(printf '%s' "$formatted") "$f" >/dev/null; then
    echo "C file not properly formatted: $f"
    bad=1
  fi
done < <(find . -name '*.c' -o -name '*.h')
if [[ $bad -ne 0 ]]; then
  exit 1
fi

echo "✅ All formatting checks passed."
