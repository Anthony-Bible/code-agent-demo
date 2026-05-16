#!/bin/bash
set -euo pipefail

E2E_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$E2E_DIR"

echo "Stopping containers and removing volumes..."
podman compose down -v --remove-orphans 2>/dev/null || true

echo "Clearing investigation artifacts..."
rm -rf agent-investigations/*

echo "Done."
