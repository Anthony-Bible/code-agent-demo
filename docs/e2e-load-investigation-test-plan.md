# E2E Load-Investigation Test Plan

## Overview

End-to-end test that simulates a production incident: a target VM is placed under artificial load, Prometheus detects the anomaly, Alertmanager fires a webhook to the agent, and the agent autonomously investigates and produces a root cause analysis.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Docker Network                           │
│                                                                 │
│  ┌──────────────┐    ┌──────────────┐    ┌───────────────────┐  │
│  │  Load Gen    │───▶│  Target VM   │◀───│  Prometheus       │  │
│  │  (hey/wrk2) │    │  (node_exporter│  │  (scrape :9100)   │  │
│  └──────────────┘    │   + demo svc) │    └───────┬───────────┘  │
│                      └──────────────┘            │              │
│                                                  ▼              │
│                                          ┌──────────────┐      │
│                                          │ Alertmanager │      │
│                                          │ (webhook)    │      │
│                                          └──────┬───────┘      │
│                                                 │              │
│                                                 ▼              │
│                                          ┌──────────────┐      │
│                                          │ code-agent    │      │
│                                          │ (webhook srv)│      │
│                                          └──────────────┘      │
└─────────────────────────────────────────────────────────────────┘
```

### Component Inventory

| Component | Image / Binary | Port | Purpose |
|-----------|---------------|------|---------|
| Target VM | `alpine:3.21` + node_exporter | :9100, :8080 | Simulated production host |
| Prometheus | `prom/prometheus:v3.3.1` | :9090 | Metric scraping & rule evaluation |
| Alertmanager | `prom/alertmanager:v0.28.1` | :9093 | Alert routing & webhook dispatch |
| Code Agent | Built from `Dockerfile` | :8080, :8081 | Webhook receiver & AI investigator |
| Load Generator | `rakyll/hey` or `williamyao/wrk2` | N/A | HTTP stress / CPU stress |

---

## Phase 1: Infrastructure Setup

### 1.1 Docker Compose File

Create `deploy/e2e/docker-compose.yml`:

```yaml
version: "3.9"

services:
  target:
    build:
      context: ./target
      dockerfile: Dockerfile
    container_name: e2e-target
    ports:
      - "9100:9100"   # node_exporter
      - "8080:8080"   # demo HTTP service
    networks:
      - e2e-net
    labels:
      env: "e2e-test"

  prometheus:
    image: prom/prometheus:v3.3.1
    container_name: e2e-prometheus
    volumes:
      - ./prometheus:/etc/prometheus
    ports:
      - "9090:9090"
    networks:
      - e2e-net
    depends_on:
      - target

  alertmanager:
    image: prom/alertmanager:v0.28.1
    container_name: e2e-alertmanager
    volumes:
      - ./alertmanager:/etc/alertmanager
    ports:
      - "9093:9093"
    networks:
      - e2e-net
    depends_on:
      - prometheus

  code-agent:
    build:
      context: ../..
      dockerfile: Dockerfile
    container_name: e2e-code-agent
    env_file:
      - .env
    environment:
      - AGENT_SERVE_ADDR=:8080
      - AGENT_AI_PROVIDER=anthropic
      - AGENT_SAFETY_AUTO_APPROVE_SAFE=true
      - AGENT_SAFETY_COMMAND_VALIDATION_MODE=blacklist
      - AGENT_LOG_LEVEL=debug
      - AGENT_LOG_FORMAT=json
    volumes:
      - ./agent-config:/app/config
      - ./agent-investigations:/app/.agent
    ports:
      - "8090:8080"   # webhook (avoid clash with target :8080)
      - "8091:8081"   # health/ready
    networks:
      - e2e-net
    depends_on:
      - alertmanager

  load-generator:
    build:
      context: ./load-gen
      dockerfile: Dockerfile
    container_name: e2e-load-gen
    networks:
      - e2e-net
    depends_on:
      - target

networks:
  e2e-net:
    driver: bridge
```

### 1.2 Target VM Container

Create `deploy/e2e/target/Dockerfile`:

```dockerfile
FROM alpine:3.21

RUN apk add --no-cache node_exporter bash curl coreutils

# Simple HTTP server that can be stressed
COPY <<'EOF' /app/server.sh
#!/bin/bash
# Demo HTTP service — intentionally inefficient for load testing
# Simulates a "production service" with CPU-spikey endpoints
while true; do
  echo -e "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\nok $(date)" \
    | nc -l -p 8080 -q 0
done &
EOF

# Startup: node_exporter + demo service
COPY <<'EOF' /app/start.sh
#!/bin/bash
/usr/bin/node_exporter \
  --web.listen-address=0.0.0.0:9100 \
  --collector.cpu \
  --collector.meminfo \
  --collector.loadavg \
  --collector.diskstats \
  --collector.filesystem \
  --collector.netdev &
/app/server.sh
EOF

RUN chmod +x /app/start.sh /app/server.sh
EXPOSE 9100 8080
CMD ["/app/start.sh"]
```

### 1.3 Load Generator

Create `deploy/e2e/load-gen/Dockerfile`:

```dockerfile
FROM alpine:3.21

RUN apk add --no-cache bash curl coreutils stress-ng

# Two-phase load generator:
# Phase 1: idle (baseline metrics)
# Phase 2: CPU + memory stress (trigger alert)
COPY <<'EOF' /app/generate-load.sh
#!/bin/bash
echo "Phase 1: Baseline idle (30s)..."
sleep 30

echo "Phase 2: CPU stress (120s)..."
# Stress 2 CPU cores for 2 minutes — enough to trip >80% alert
stress-ng --cpu 2 --timeout 120s --metrics-brief &

echo "Phase 2: Memory pressure (120s)..."
# Allocate ~256MB to spike memory
stress-ng --vm 2 --vm-bytes 128M --timeout 120s &

echo "Phase 2: HTTP load against target (120s)..."
# Bombard the demo service
for i in $(seq 1 500); do
  curl -s http://target:8080/ &>/dev/null &
done

echo "Load generation complete. Containers remain running."
sleep infinity
EOF

RUN chmod +x /app/generate-load.sh
CMD ["/app/generate-load.sh"]
```

---

## Phase 2: Monitoring Configuration

### 2.1 Prometheus Config

Create `deploy/e2e/prometheus/prometheus.yml`:

```yaml
global:
  scrape_interval: 5s        # Fast scraping for e2e (prod would be 15-30s)
  evaluation_interval: 5s    # Fast rule evaluation

scrape_configs:
  - job_name: "target"
    static_configs:
      - targets: ["target:9100"]
        labels:
          env: "e2e-test"
          host: "e2e-target"

rule_files:
  - "/etc/prometheus/alert_rules.yml"

alerting:
  alertmanagers:
    - static_configs:
        - targets: ["alertmanager:9093"]
```

### 2.2 Alert Rules

Create `deploy/e2e/prometheus/alert_rules.yml`:

```yaml
groups:
  - name: e2e-load-alerts
    rules:
      # High CPU — primary trigger for investigation
      - alert: HighCPUUtilization
        expr: 100 - (avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[30s])) * 100) > 80
        for: 15s
        labels:
          severity: critical
          team: sre
        annotations:
          summary: "High CPU on {{ $labels.instance }}"
          description: "CPU usage is {{ $value }}% on {{ $labels.instance }} — above 80% threshold"

      # High memory — secondary signal
      - alert: HighMemoryUtilization
        expr: (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100 > 85
        for: 15s
        labels:
          severity: warning
          team: sre
        annotations:
          summary: "High memory on {{ $labels.instance }}"
          description: "Memory usage is {{ $value }}% on {{ $labels.instance }}"

      # High load average — corroborating signal
      - alert: HighLoadAverage
        expr: node_load1 > 2
        for: 15s
        labels:
          severity: warning
          team: sre
        annotations:
          summary: "High load average on {{ $labels.instance }}"
          description: "1-minute load average is {{ $value }} on {{ $labels.instance }}"
```

**Why these thresholds:**
- CPU > 80% for 15s — aggressive enough to fire during stress-ng with 5s scrape interval
- Memory > 85% — 2x 128M stressors should push this on a container with ~512M
- Load > 2 — guaranteed with 2 CPU stressors on a 1-core container

---

## Phase 3: Alert Routing

### 3.1 Alertmanager Config

Create `deploy/e2e/alertmanager/alertmanager.yml`:

```yaml
global:
  resolve_timeout: 5m

route:
  receiver: "code-agent"
  group_wait: 5s          # Fast grouping for e2e
  group_interval: 10s
  repeat_interval: 1m
  routes:
    - match:
        severity: critical
      receiver: "code-agent"
    - match:
        severity: warning
      receiver: "code-agent"

receivers:
  - name: "code-agent"
    webhook_configs:
      - url: "http://code-agent:8080/alerts/prometheus"
        send_resolved: true
        http_config:
          # Optional: add basic auth if configured
          # basic_auth:
          #   username: e2e
          #   password: test
```

### 3.2 Agent Alert Sources Config

Create `deploy/e2e/agent-config/alert-sources.yaml`:

```yaml
addr: ":8080"

sources:
  - type: prometheus
    name: alertmanager
    webhook_path: /alerts/prometheus
```

### 3.3 Environment File

Create `deploy/e2e/.env` (gitignored):

```bash
ANTHROPIC_API_KEY=sk-ant-xxx  # MUST BE SET BEFORE RUNNING
```

---

## Phase 4: Agent Configuration for Investigation

### 4.1 Safety Settings for E2E

The agent's investigation executor needs to access the target VM. Two approaches:

**Option A: Allow SSH into target container (recommended for e2e)**

Add a skill that gives the agent context about the target VM:

Create `deploy/e2e/agent-config/skills/e2e-target/SKILL.md`:

```yaml
---
name: e2e-target
description: "E2E test target VM access — provides connection details and context for the load test target"
allowed-tools:
  - bash
  - read_file
  - list_files
  - fetch
---
```

```markdown
# E2E Target VM

You have access to the following test infrastructure:

- **Target VM**: `e2e-target` (Docker container on the `e2e-net` network)
  - node_exporter metrics: `http://target:9100/metrics`
  - Demo HTTP service: `http://target:8080/`
  - SSH: `ssh root@target` (if SSH enabled)

- **Prometheus**: `http://prometheus:9090`
  - Query API: `http://prometheus:9090/api/v1/query?query=...`

- **Alertmanager**: `http://alertmanager:9093`

## Useful Investigation Commands

```bash
# Check CPU usage via Prometheus API
curl -s 'http://prometheus:9090/api/v1/query?query=100-avg(rate(node_cpu_seconds_total{mode="idle"}[1m]))*100'

# Check memory usage
curl -s 'http://prometheus:9090/api/v1/query?query=(1-node_memory_MemAvailable_bytes/node_memory_MemTotal_bytes)*100'

# Check load average
curl -s 'http://prometheus:9090/api/v1/query?query=node_load1'

# List top processes on target (if SSH available)
ssh root@target "ps aux --sort=-%cpu | head -20"

# Check disk I/O
curl -s 'http://prometheus:9090/api/v1/query?query=rate(node_disk_read_bytes_total[1m])'
```
```

**Option B: Use the `fetch` tool with Prometheus HTTP API**

The agent's fetch tool can query Prometheus directly. No SSH needed — the agent queries metrics via HTTP API, which is how a real investigation might work in a cloud environment.

### 4.2 Investigation Prompt Enhancement

The existing `GenericPromptBuilder` builds investigation prompts from alert data. For e2e, ensure the system prompt includes:

- The target VM hostname and accessible endpoints
- That the agent should use the `fetch` tool to query Prometheus
- That the agent should use `bash` to inspect the target if accessible

This is already handled by the skill system — activating the `e2e-target` skill gives the agent all needed context.

---

## Phase 5: Execution Procedure

### 5.1 Prerequisites

```bash
# 1. Set your API key
export ANTHROPIC_API_KEY="sk-ant-xxx"

# 2. Build the code-agent Docker image
cd deploy/e2e
docker compose build code-agent

# 3. Verify all images build
docker compose build
```

### 5.2 Run the Test

```bash
cd deploy/e2e

# Start all services
docker compose up -d

# Wait for agent to be ready
until curl -s http://localhost:8091/ready | grep -q '"status":"ok"'; do
  echo "Waiting for agent readiness..."
  sleep 2
done

# Watch the load generator (should already be running)
docker compose logs -f load-generator

# Watch the agent's investigation logs
docker compose logs -f code-agent
```

### 5.3 Expected Timeline

| Time | Event |
|------|-------|
| 0:00 | All containers start |
| 0:05 | Agent reports ready (`/ready` → 200) |
| 0:10 | Prometheus begins scraping target |
| 0:30 | Load generator enters Phase 2 (stress) |
| 0:45 | CPU alert fires (80%+ for 15s) |
| 0:45 | Alertmanager receives alert, dispatches webhook |
| 0:45 | Agent receives webhook, starts investigation |
| 1:00-2:00 | Agent investigates using tools (bash, fetch) |
| 2:00 | Agent completes investigation with RCA |
| 2:30 | Load generator stops, alerts resolve |
| 3:00 | Optional: resolved alerts sent to agent |

### 5.4 Verify Investigation Results

```bash
# Check investigation output files
ls -la deploy/e2e/agent-investigations/

# Read the investigation result
cat deploy/e2e/agent-investigations/*.json | jq .

# Expected fields in the result:
# - investigation_id
# - status: "completed" or "escalated"
# - confidence: 0.0-1.0
# - findings: array of RCA findings
# - actions_taken: number of tool calls
# - duration: time spent investigating
```

---

## Phase 6: Success Criteria

### Must Have (P0)
- [ ] Alert fires in Prometheus when load is applied
- [ ] Alertmanager delivers webhook to code-agent
- [ ] Agent receives and parses the alert
- [ ] Agent starts an investigation (investigation_id created)
- [ ] Agent uses at least one tool (bash or fetch) to gather evidence
- [ ] Agent produces an investigation result (status: completed or escalated)
- [ ] Investigation result includes RCA findings with root cause and confidence

### Should Have (P1)
- [ ] Agent correctly identifies CPU stress as the root cause
- [ ] Agent's confidence score > 0.5
- [ ] Agent uses multiple tools (fetch to Prometheus + bash to inspect)
- [ ] Investigation completes within 3 minutes
- [ ] Multiple alerts are correlated (CPU + memory + load)

### Nice to Have (P2)
- [ ] Agent correctly identifies `stress-ng` as the specific process
- [ ] Agent provides actionable remediation steps
- [ ] Escalation triggers if confidence is low
- [ ] Resolved alert is processed after load stops
- [ ] Agent uses a subagent for part of the investigation
- [ ] Full e2e test is automated as a Go test or shell script

---

## Phase 7: Automation

There are two levels of automation:

1. **`setup.sh`** — Scaffold the entire `deploy/e2e/` directory tree with all config files, Dockerfiles, and scripts. Run once after cloning.
2. **`run-test.sh`** — Execute the test: build, start, wait, validate, tear down. Run every time you want to test.
3. **`teardown.sh`** — Clean up containers, volumes, and generated artifacts.

### 7.1 Setup Script — `deploy/e2e/setup.sh`

Creates the entire e2e directory from scratch so `run-test.sh` can work:

```bash
#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
E2E_DIR="$SCRIPT_DIR"

echo "=== E2E Test Setup ==="
echo "Creating directory structure and config files in $E2E_DIR"
echo ""

# --- Directory tree ---
mkdir -p "$E2E_DIR"/{target,load-gen,prometheus,alertmanager,agent-config/skills/e2e-target,agent-investigations}

# --- .gitignore ---
cat > "$E2E_DIR/.gitignore" <<'EOF'
.env
agent-investigations/
*.log
EOF

# --- .env template ---
if [ ! -f "$E2E_DIR/.env" ]; then
  cat > "$E2E_DIR/.env" <<'EOF'
# Set your Anthropic API key here, or export ANTHROPIC_API_KEY before running
ANTHROPIC_API_KEY=
EOF
  echo "Created .env template — EDIT IT with your API key before running!"
else
  echo ".env already exists, skipping"
fi

# --- Target VM Dockerfile ---
cat > "$E2E_DIR/target/Dockerfile" <<'DOCKERFILE'
FROM alpine:3.21

RUN apk add --no-cache node_exporter bash curl coreutils

# Simple HTTP server that can be stressed — intentionally inefficient
COPY server.sh /app/server.sh
COPY start.sh /app/start.sh
RUN chmod +x /app/start.sh /app/server.sh

EXPOSE 9100 8080
CMD ["/app/start.sh"]
DOCKERFILE

cat > "$E2E_DIR/target/start.sh" <<'STARTSH'
#!/bin/bash
/usr/bin/node_exporter \
  --web.listen-address=0.0.0.0:9100 \
  --collector.cpu \
  --collector.meminfo \
  --collector.loadavg \
  --collector.diskstats \
  --collector.filesystem \
  --collector.netdev &
/app/server.sh
STARTSH

cat > "$E2E_DIR/target/server.sh" <<'SERVERSH'
#!/bin/bash
# Demo HTTP service — one connection at a time, easy to overload
while true; do
  echo -e "HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\n\r\nok $(date)" \
    | nc -l -p 8080 -q 0
done &
SERVERSH

chmod +x "$E2E_DIR/target/start.sh" "$E2E_DIR/target/server.sh"

# --- Load Generator Dockerfile ---
cat > "$E2E_DIR/load-gen/Dockerfile" <<'DOCKERFILE'
FROM alpine:3.21

RUN apk add --no-cache bash curl coreutils stress-ng

COPY generate-load.sh /app/generate-load.sh
RUN chmod +x /app/generate-load.sh
CMD ["/app/generate-load.sh"]
DOCKERFILE

cat > "$E2E_DIR/load-gen/generate-load.sh" <<'LOADSH'
#!/bin/bash
echo "Phase 1: Baseline idle (30s)..."
sleep 30

echo "Phase 2: CPU stress (120s)..."
stress-ng --cpu 2 --timeout 120s --metrics-brief &

echo "Phase 2: Memory pressure (120s)..."
stress-ng --vm 2 --vm-bytes 128M --timeout 120s &

echo "Phase 2: HTTP load against target (120s)..."
for i in $(seq 1 500); do
  curl -s http://target:8080/ &>/dev/null &
done

echo "Load generation complete. Containers remain running."
sleep infinity
LOADSH

chmod +x "$E2E_DIR/load-gen/generate-load.sh"

# --- Prometheus config ---
cat > "$E2E_DIR/prometheus/prometheus.yml" <<'PROMYML'
global:
  scrape_interval: 5s
  evaluation_interval: 5s

scrape_configs:
  - job_name: "target"
    static_configs:
      - targets: ["target:9100"]
        labels:
          env: "e2e-test"
          host: "e2e-target"

rule_files:
  - "/etc/prometheus/alert_rules.yml"

alerting:
  alertmanagers:
    - static_configs:
        - targets: ["alertmanager:9093"]
PROMYML

cat > "$E2E_DIR/prometheus/alert_rules.yml" <<'RULESYML'
groups:
  - name: e2e-load-alerts
    rules:
      - alert: HighCPUUtilization
        expr: 100 - (avg by(instance) (rate(node_cpu_seconds_total{mode="idle"}[30s])) * 100) > 80
        for: 15s
        labels:
          severity: critical
          team: sre
        annotations:
          summary: "High CPU on {{ $labels.instance }}"
          description: "CPU usage is {{ $value }}% on {{ $labels.instance }} — above 80% threshold"

      - alert: HighMemoryUtilization
        expr: (1 - (node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes)) * 100 > 85
        for: 15s
        labels:
          severity: warning
          team: sre
        annotations:
          summary: "High memory on {{ $labels.instance }}"
          description: "Memory usage is {{ $value }}% on {{ $labels.instance }}"

      - alert: HighLoadAverage
        expr: node_load1 > 2
        for: 15s
        labels:
          severity: warning
          team: sre
        annotations:
          summary: "High load average on {{ $labels.instance }}"
          description: "1-minute load average is {{ $value }} on {{ $labels.instance }}"
RULESYML

# --- Alertmanager config ---
cat > "$E2E_DIR/alertmanager/alertmanager.yml" <<'AMYML'
global:
  resolve_timeout: 5m

route:
  receiver: "code-agent"
  group_wait: 5s
  group_interval: 10s
  repeat_interval: 1m
  routes:
    - match:
        severity: critical
      receiver: "code-agent"
    - match:
        severity: warning
      receiver: "code-agent"

receivers:
  - name: "code-agent"
    webhook_configs:
      - url: "http://code-agent:8080/alerts/prometheus"
        send_resolved: true
AMYML

# --- Agent alert sources config ---
cat > "$E2E_DIR/agent-config/alert-sources.yaml" <<'AGENTYML'
addr: ":8080"

sources:
  - type: prometheus
    name: alertmanager
    webhook_path: /alerts/prometheus
AGENTYML

# --- E2E target skill for the agent ---
cat > "$E2E_DIR/agent-config/skills/e2e-target/SKILL.md" <<'SKILLMD'
---
name: e2e-target
description: "E2E test target VM access — provides connection details for the load test target"
allowed-tools:
  - bash
  - read_file
  - list_files
  - fetch
---

# E2E Target VM

You have access to the following test infrastructure:

- **Target VM**: `e2e-target` (Docker container on the `e2e-net` network)
  - node_exporter metrics: `http://target:9100/metrics`
  - Demo HTTP service: `http://target:8080/`

- **Prometheus**: `http://prometheus:9090`
  - Query API: `http://prometheus:9090/api/v1/query?query=...`

- **Alertmanager**: `http://alertmanager:9093`

## Useful Investigation Commands

```bash
# Check CPU usage via Prometheus API
curl -s 'http://prometheus:9090/api/v1/query?query=100-avg(rate(node_cpu_seconds_total{mode="idle"}[1m]))*100'

# Check memory usage
curl -s 'http://prometheus:9090/api/v1/query?query=(1-node_memory_MemAvailable_bytes/node_memory_MemTotal_bytes)*100'

# Check load average
curl -s 'http://prometheus:9090/api/v1/query?query=node_load1'

# Check disk I/O
curl -s 'http://prometheus:9090/api/v1/query?query=rate(node_disk_read_bytes_total[1m])'
```
SKILLMD

# --- Docker Compose ---
cat > "$E2E_DIR/docker-compose.yml" <<'COMPOSE'
version: "3.9"

services:
  target:
    build:
      context: ./target
      dockerfile: Dockerfile
    container_name: e2e-target
    ports:
      - "9100:9100"
      - "8080:8080"
    networks:
      - e2e-net
    labels:
      env: "e2e-test"

  prometheus:
    image: prom/prometheus:v3.3.1
    container_name: e2e-prometheus
    volumes:
      - ./prometheus:/etc/prometheus
    ports:
      - "9090:9090"
    networks:
      - e2e-net
    depends_on:
      - target

  alertmanager:
    image: prom/alertmanager:v0.28.1
    container_name: e2e-alertmanager
    volumes:
      - ./alertmanager:/etc/alertmanager
    ports:
      - "9093:9093"
    networks:
      - e2e-net
    depends_on:
      - prometheus

  code-agent:
    build:
      context: ../..
      dockerfile: Dockerfile
    container_name: e2e-code-agent
    env_file:
      - .env
    environment:
      - AGENT_SERVE_ADDR=:8080
      - AGENT_AI_PROVIDER=anthropic
      - AGENT_SAFETY_AUTO_APPROVE_SAFE=true
      - AGENT_SAFETY_COMMAND_VALIDATION_MODE=blacklist
      - AGENT_LOG_LEVEL=debug
      - AGENT_LOG_FORMAT=json
    volumes:
      - ./agent-config:/app/config
      - ./agent-investigations:/app/.agent
    ports:
      - "8090:8080"
      - "8091:8081"
    networks:
      - e2e-net
    depends_on:
      - alertmanager

  load-generator:
    build:
      context: ./load-gen
      dockerfile: Dockerfile
    container_name: e2e-load-gen
    networks:
      - e2e-net
    depends_on:
      - target

networks:
  e2e-net:
    driver: bridge
COMPOSE

echo ""
echo "=== Setup complete! ==="
echo ""
echo "Next steps:"
echo "  1. Edit deploy/e2e/.env and set ANTHROPIC_API_KEY"
echo "  2. Run: cd deploy/e2e && ./run-test.sh"
echo ""
echo "Or if you prefer manual control:"
echo "  cd deploy/e2e && docker compose up -d"
```

### 7.2 Test Runner — `deploy/e2e/run-test.sh`

```bash
#!/bin/bash
set -euo pipefail

E2E_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$E2E_DIR"

# --- Pre-flight checks ---
if [ -z "${ANTHROPIC_API_KEY:-}" ]; then
  # Try loading from .env
  if [ -f .env ]; then
    # shellcheck disable=SC1091
    source .env
  fi
fi

if [ -z "${ANTHROPIC_API_KEY:-}" ]; then
  echo "ERROR: ANTHROPIC_API_KEY must be set (env var or .env file)"
  exit 1
fi

if [ ! -f docker-compose.yml ]; then
  echo "ERROR: docker-compose.yml not found. Run ./setup.sh first."
  exit 1
fi

echo "=== E2E Load-Investigation Test ==="
echo ""

# --- Cleanup any previous run ---
echo "Cleaning up previous run..."
docker compose down -v --remove-orphans 2>/dev/null || true
rm -rf agent-investigations/*

# --- Build & Start ---
echo "Building containers..."
docker compose build

echo "Starting services..."
docker compose up -d

# --- Wait for readiness ---
echo "Waiting for code-agent to be ready..."
RETRY=0
MAX_RETRIES=60
while [ $RETRY -lt $MAX_RETRIES ]; do
  if curl -sf http://localhost:8091/ready > /dev/null 2>&1; then
    echo "Agent is ready!"
    break
  fi
  RETRY=$((RETRY + 1))
  echo "  Attempt $RETRY/$MAX_RETRIES..."
  sleep 2
done

if [ $RETRY -eq $MAX_RETRIES ]; then
  echo "ERROR: Agent never became ready"
  docker compose logs code-agent
  exit 1
fi

# --- Verify Prometheus is scraping ---
echo ""
echo "Verifying Prometheus is scraping the target..."
sleep 5
TARGETS=$(curl -sf http://localhost:9090/api/v1/targets 2>/dev/null || echo "{}")
if echo "$TARGETS" | grep -q "e2e-target"; then
  echo "Prometheus is scraping the target!"
else
  echo "WARNING: Prometheus may not be scraping yet. Check http://localhost:9090/targets"
fi

# --- Wait for load to trigger alerts ---
echo ""
echo "Load generator is running. Waiting for alerts to fire..."
echo "  (Load generator starts stress at ~30s mark)"
echo ""

# Monitor for investigation files
echo "Watching for investigation output..."
TIMEOUT=300  # 5 minutes max
ELAPSED=0
while [ $ELAPSED -lt $TIMEOUT ]; do
  FILES=$(find agent-investigations -name "*.json" 2>/dev/null | wc -l | tr -d ' ')
  if [ "$FILES" -gt 0 ]; then
    echo ""
    echo "=== Investigation complete! ==="
    echo ""
    cat agent-investigations/*.json | python3 -m json.tool 2>/dev/null || cat agent-investigations/*.json
    echo ""
    echo "=== Test PASSED ==="
    docker compose down -v
    exit 0
  fi
  sleep 5
  ELAPSED=$((ELAPSED + 5))
  echo "  Waiting... (${ELAPSED}s / ${TIMEOUT}s)"
done

echo ""
echo "=== TIMEOUT: No investigation completed within ${TIMEOUT}s ==="
echo ""
echo "Debugging info:"
echo "--- Prometheus targets ---"
curl -sf http://localhost:9090/api/v1/targets 2>/dev/null | python3 -m json.tool || echo "Prometheus unreachable"
echo "--- Alertmanager alerts ---"
curl -sf http://localhost:9093/api/v2/alerts 2>/dev/null | python3 -m json.tool || echo "Alertmanager unreachable"
echo "--- Agent logs (last 50 lines) ---"
docker compose logs --tail=50 code-agent

echo ""
echo "Leaving containers running for manual debugging. Run ./teardown.sh to clean up."
exit 1
```

### 7.3 Teardown Script — `deploy/e2e/teardown.sh`

```bash
#!/bin/bash
set -euo pipefail

E2E_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$E2E_DIR"

echo "=== Tearing down E2E test environment ==="

docker compose down -v --remove-orphans 2>/dev/null || true

echo "Cleaning investigation artifacts..."
rm -rf agent-investigations/*

echo "Done. Containers stopped and volumes removed."
```

### 7.4 One-Command Workflow

```bash
# From project root:
cd deploy/e2e

# First time: scaffold everything
./setup.sh

# Edit .env with your key
$EDITOR .env

# Run the full test (builds, starts, waits, validates, tears down)
./run-test.sh

# Or if the test failed and containers are still running:
./teardown.sh
```

---

## Phase 8: Optional Enhancements

### 8.1 Real VM (instead of Docker container)

For a more realistic test, use a real VM:
- Spin up a GCP/GCP/AWS instance
- Install node_exporter + a demo service
- Point Prometheus at the VM's public IP
- SSH from the agent into the VM for investigation

### 8.2 GCP Monitoring Integration

Instead of Prometheus, use GCP Monitoring:
- Deploy the agent to Cloud Run
- Configure a GCP Monitoring notification channel (webhook to the agent)
- Use the `gcp_monitoring` alert source type
- Stress a GCE instance to trigger the GCP alert

### 8.3 Multi-Alert Correlation

Test that the agent can correlate multiple related alerts:
- Fire CPU, memory, and load alerts simultaneously
- Verify the RCA correlates them to a single root cause (stress test)

### 8.4 Go Integration Test

Write a Go test in `internal/integration/e2e_load_test.go`:
- Programmatically start the Docker containers
- Wait for alerts and investigation results
- Assert on the investigation output
- Gate with `ANTHROPIC_API_KEY` env var (skip if not set)

### 8.5 CI Pipeline

Add to `.github/workflows/`:
- Trigger on `deploy/e2e/**` changes
- Requires `ANTHROPIC_API_KEY` secret
- Runs the e2e test script
- Uploads investigation results as artifacts

---

## File Structure Summary

```
deploy/e2e/
├── setup.sh                        # Scaffold entire directory (run once)
├── run-test.sh                     # Build, start, wait, validate, teardown
├── teardown.sh                     # Stop containers, remove volumes/artifacts
├── docker-compose.yml              # Orchestrates all services (generated by setup.sh)
├── .env                            # API key (gitignored, generated by setup.sh)
├── .gitignore                      # Ignore .env and investigations
├── target/
│   ├── Dockerfile                  # Target VM with node_exporter
│   ├── start.sh                    # Entrypoint: node_exporter + demo svc
│   └── server.sh                   # Naive HTTP server to stress
├── load-gen/
│   ├── Dockerfile                  # Stress generator
│   └── generate-load.sh            # Two-phase: idle → CPU/mem/HTTP stress
├── prometheus/
│   ├── prometheus.yml              # Scrape config (5s interval)
│   └── alert_rules.yml            # CPU > 80%, mem > 85%, load > 2
├── alertmanager/
│   └── alertmanager.yml            # Webhook routing to code-agent
└── agent-config/
    ├── alert-sources.yaml          # Agent alert source config
    └── skills/
        └── e2e-target/
            └── SKILL.md            # Target VM context for agent
```

## Risks & Mitigations

| Risk | Mitigation |
|------|-----------|
| Agent costs (API calls) | Use debug logging to estimate token spend; set `AGENT_MAX_TOKENS` conservatively |
| Flaky alert timing | Use short `for:` durations (15s) and fast scrape intervals (5s) |
| Agent fails to investigate | Debug via agent logs; check safety settings blocking bash commands |
| Docker networking issues | Use named services on shared `e2e-net` bridge network |
| `stress-ng` not available in Alpine | Fall back to `yes > /dev/null &` for CPU, `dd if=/dev/zero` for memory |
| Webhook auth mismatch | Start without basic auth; add after basic flow works |
