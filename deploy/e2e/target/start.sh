#!/bin/bash
set -e

/usr/bin/node_exporter \
  --web.listen-address=0.0.0.0:9100 \
  --collector.cpu \
  --collector.meminfo \
  --collector.loadavg \
  --collector.diskstats \
  --collector.filesystem \
  --collector.netdev &
NODE_EXPORTER_PID=$!

# Backgrounded processes don't trip `set -e`. Verify node_exporter actually
# stayed up — otherwise Prometheus gets no metrics and the test fails opaquely.
sleep 2
if ! kill -0 "$NODE_EXPORTER_PID" 2>/dev/null; then
  echo "FATAL: node_exporter failed to start" >&2
  exit 1
fi

exec /app/server.sh
