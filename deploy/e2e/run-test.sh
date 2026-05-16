#!/bin/bash
set -euo pipefail

E2E_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$E2E_DIR"

# --- Pre-flight ---
if [ -z "${ANTHROPIC_API_KEY:-}" ] && [ -f .env ]; then
  # shellcheck disable=SC1091
  set -a; source .env; set +a
fi

if [ -z "${ANTHROPIC_API_KEY:-}" ]; then
  echo "ERROR: ANTHROPIC_API_KEY must be set (env var or .env file)" >&2
  exit 1
fi

if [ ! -f docker-compose.yml ]; then
  echo "ERROR: docker-compose.yml missing. Run ./setup.sh first." >&2
  exit 1
fi

mkdir -p agent-investigations
# Ensure the non-root container user (UID 10001) can write here.
chmod 0777 agent-investigations || true

echo "=== E2E Load-Investigation Test ==="

echo "[1/5] Cleaning up previous run..."
podman compose down -v --remove-orphans 2>/dev/null || true
rm -rf agent-investigations/*

echo "[2/5] Building images..."
podman compose build

echo "[3/5] Starting stack..."
podman compose up -d

echo "[4/5] Waiting for code-agent /ready (max 120s)..."
for i in $(seq 1 60); do
  if curl -sf http://localhost:8091/ready >/dev/null 2>&1; then
    echo "  agent ready after ${i} attempts"
    break
  fi
  sleep 2
  if [ "$i" -eq 60 ]; then
    echo "ERROR: agent never became ready" >&2
    podman compose logs code-agent | tail -50
    exit 1
  fi
done

echo "[5/5] Watching for investigation output (timeout 300s)..."
echo "  load-gen begins stress at ~30s; alerts fire at ~45s."

ELAPSED=0
TIMEOUT=300
while [ "$ELAPSED" -lt "$TIMEOUT" ]; do
  # Investigation files land at agent-investigations/investigations/<id>.json.
  # Success: at least one investigation reached a terminal state (completed or
  # escalated) AND produced findings or escalated. Other investigations in the
  # batch may still be in flight or have given up at the action budget — the
  # pipeline works as long as one alert produced a useful RCA.
  if python3 - <<'PY' 2>/dev/null
import glob, json, sys
files = glob.glob('agent-investigations/**/*.json', recursive=True)
if not files:
    sys.exit(1)
for path in files:
    try:
        with open(path) as fh:
            data = json.load(fh)
    except (OSError, json.JSONDecodeError):
        continue
    if data.get('status') not in ('completed', 'escalated'):
        continue
    if data.get('findings') or data.get('escalated'):
        sys.exit(0)
sys.exit(1)
PY
  then
    echo
    echo "=== Investigation output ==="
    while IFS= read -r -d '' f; do
      echo "--- $f ---"
      python3 -m json.tool < "$f" 2>/dev/null || cat "$f"
      echo
    done < <(find agent-investigations -type f -name '*.json' -print0)
    echo "=== Test PASSED ==="
    podman compose down -v
    exit 0
  fi
  sleep 5
  ELAPSED=$((ELAPSED + 5))
  printf '  waited %ss / %ss\n' "$ELAPSED" "$TIMEOUT"
done

echo
echo "=== TIMEOUT after ${TIMEOUT}s — no investigation produced ===" >&2
echo
echo "--- Prometheus alerts ---"
curl -sf http://localhost:9090/api/v1/alerts 2>/dev/null | python3 -m json.tool 2>/dev/null \
  || echo "(prometheus unreachable)"
echo "--- Alertmanager alerts ---"
curl -sf http://localhost:9093/api/v2/alerts 2>/dev/null | python3 -m json.tool 2>/dev/null \
  || echo "(alertmanager unreachable)"
echo "--- Investigation files (status + findings count) ---"
if compgen -G 'agent-investigations/investigations/*.json' >/dev/null; then
  for f in agent-investigations/investigations/*.json; do
    jq -c --arg f "$f" '{file:$f, status, findings_len:(.findings|length), escalated, actions_taken}' "$f" 2>/dev/null \
      || echo "(failed to parse $f)"
  done
else
  echo "(no investigation files written)"
fi
echo "--- code-agent logs (tail 80) ---"
podman compose logs --tail=80 code-agent
echo
echo "Containers left running for debugging. ./teardown.sh to clean up."
exit 1
