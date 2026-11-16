#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
OUT_DIR="${ROOT_DIR}/android/app/libs"
OUT_AAR="${OUT_DIR}/tsnet-bridge.aar"

mkdir -p "${OUT_DIR}"

if ! command -v gomobile >/dev/null 2>&1; then
  echo "Error: gomobile not found in PATH." >&2
  echo "Install with: go install golang.org/x/mobile/cmd/gomobile@latest && gomobile init" >&2
  exit 1
fi

TARGET="${GOMOBILE_TARGET:-android/arm64}"

cd "${SCRIPT_DIR}"
gomobile bind \
  -target="${TARGET}" \
  -androidapi 26 \
  -ldflags="-s -w" \
  -o "${OUT_AAR}" \
  .

echo "Built ${OUT_AAR}"
ls -lh "${OUT_AAR}"
