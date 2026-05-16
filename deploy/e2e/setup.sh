#!/bin/bash
# Bootstraps the deploy/e2e tree for first-time use. Everything except the
# .env (API key) and the writable agent-investigations dir is checked into
# git, so this script only handles the things that aren't.
set -euo pipefail

E2E_DIR="$(cd "$(dirname "$0")" && pwd)"

echo "=== Bootstrapping $E2E_DIR ==="

# Writable mount target for investigation JSON. Container runs as UID 10001,
# so the host dir needs to be world-writable.
mkdir -p "$E2E_DIR/agent-investigations"
chmod 0777 "$E2E_DIR/agent-investigations" 2>/dev/null || true

if [ ! -f "$E2E_DIR/.env" ]; then
  if [ ! -f "$E2E_DIR/.env.template" ]; then
    echo "ERROR: .env.template missing — repo is in an inconsistent state" >&2
    exit 1
  fi
  cp "$E2E_DIR/.env.template" "$E2E_DIR/.env"
  echo "  created .env from .env.template — EDIT IT with your ANTHROPIC_API_KEY"
else
  echo "  .env already exists, leaving it alone"
fi

chmod +x "$E2E_DIR"/*.sh "$E2E_DIR"/target/*.sh "$E2E_DIR"/load-gen/*.sh 2>/dev/null || true

echo
echo "=== Done. Next: edit .env with ANTHROPIC_API_KEY, then ./run-test.sh ==="
