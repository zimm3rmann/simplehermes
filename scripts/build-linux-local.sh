#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if ! pkg-config --exists gtk+-3.0; then
  echo "Missing GTK 3 development files."
  echo "Install them with: sudo apt-get update && sudo apt-get install -y libgtk-3-dev"
  exit 1
fi

if ! pkg-config --exists webkit2gtk-4.1; then
  echo "Missing WebKitGTK 4.1 development files required for the Ubuntu 24.04 Wails build."
  echo "Install them with: sudo apt-get update && sudo apt-get install -y libwebkit2gtk-4.1-dev"
  exit 1
fi

mkdir -p dist
go build -tags "production webkit2_41" -o dist/simplehermes-linux-amd64 ./cmd/simplehermes
echo "Built dist/simplehermes-linux-amd64"
