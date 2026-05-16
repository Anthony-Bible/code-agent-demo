#!/bin/bash
set -euo pipefail

echo "[load-gen] Phase 1: baseline idle (30s)..."
sleep 30

echo "[load-gen] Phase 2a: CPU stress (120s, all cores)..."
stress-ng --cpu 0 --timeout 120s --metrics-brief &

echo "[load-gen] Phase 2b: memory pressure (120s, 4x1G, --vm-keep to actually hold pages)..."
stress-ng --vm 4 --vm-bytes 1G --vm-keep --timeout 120s &

echo "[load-gen] Phase 2c: HTTP flood against target:8080 (500 reqs, 20 parallel)..."
seq 1 500 | xargs -n 1 -P 20 -I{} curl -s -m 2 -o /dev/null http://target:8080/ || true

wait
echo "[load-gen] Stress phase complete. Sleeping forever for log inspection."
sleep infinity
