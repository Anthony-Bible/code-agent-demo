# Kubernetes Deployment

Kubernetes manifests and automation for deploying the code-agent-demo webhook server.

## Quick Start (minikube)

### Prerequisites

- [minikube](https://minikube.sigs.k8s.io/docs/start/) installed and running
- kubectl configured for minikube
- Docker CLI (for building images)

### One-Command Deployment

```bash
cd deploy/k8s
make all
```

This will:
1. Build the Docker image
2. Load it into minikube
3. Create all Kubernetes resources
4. Run health and security checks

### Individual Steps

```bash
# Build the Docker image
make build

# Load image into minikube (required for local testing)
make load

# Deploy to Kubernetes
make deploy

# Wait for deployment to be ready
make wait-ready

# Run health and security checks
make test

# View logs
make logs

# Check deployment status
make status

# Port-forward service to localhost:8080
make port-forward
```

### Cleanup

```bash
make clean
```

## Available Make Targets

Run `make help` to see all available targets:

| Target | Description |
|--------|-------------|
| `help` | Show all available targets |
| `build` | Build Docker image |
| `load` | Load image into minikube |
| `deploy` | Create Kubernetes resources |
| `test` | Run health and security checks |
| `logs` | Show deployment logs |
| `status` | Show deployment status |
| `port-forward` | Port-forward service to localhost |
| `clean` | Remove all Kubernetes resources |
| `all` | Full workflow: clean → build → load → deploy → test |

## Configuration

### Variables

Override Makefile variables:

```bash
# Custom image tag
make all IMAGE_TAG=v1.0.0

# Custom namespace
make all NAMESPACE=my-namespace
```

Default values:
- `IMAGE_NAME`: `ghcr.io/anthony-bible/code-agent-demo`
- `IMAGE_TAG`: `latest`
- `NAMESPACE`: `code-agent`

### Image Pull Policy

For minikube testing with locally-loaded images, the deployment uses `imagePullPolicy: Never` to avoid pulling from a registry.

**For production deployment** to a container registry:
1. Build, tag, and push the image:
   ```bash
   docker build -t ghcr.io/anthony-bible/code-agent-demo:latest .
   docker push ghcr.io/anthony-bible/code-agent-demo:latest
   ```
2. Update `imagePullPolicy` in `deployment.yaml` from `Never` to `IfNotPresent` or `Always`

## Security Configuration

The deployment follows Kubernetes security best practices:

| Security Feature | Value |
|------------------|-------|
| Run as non-root | `runAsUser: 10001` |
| Run as non-root group | `runAsGroup: 10001` |
| Filesystem group | `fsGroup: 10001` |
| Read-only root filesystem | `readOnlyRootFilesystem: true` |
| Privilege escalation | `allowPrivilegeEscalation: false` |
| Capabilities | `drop: ["ALL"]` |
| Seccomp profile | `type: RuntimeDefault` |
| Service account token | `automountServiceAccountToken: false` |

### Verification

Run `make test` to verify:
- Non-root user execution
- Read-only root filesystem
- All capabilities dropped
- Health and readiness endpoints

## Accessing the Service

### Via Port-Forward

```bash
make port-forward

# In another terminal:
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

### Via NodePort (minikube)

The service is configured as ClusterIP. For NodePort access, modify `service.yaml`:

```yaml
spec:
  type: NodePort
  ports:
    - port: 8080
      targetPort: 8080
      nodePort: 30080
```

Then access via:
```bash
minikube service code-agent-demo -n code-agent --url
```

## Secrets

The deployment requires a `code-agent-secrets` secret with the Anthropic API key:

```bash
kubectl create secret generic code-agent-secrets \
  -n code-agent \
  --from-literal=ANTHROPIC_API_KEY=your-api-key-here
```

For testing, `make deploy` creates a placeholder secret with `test-key`.

## Alert Webhook Endpoints

Once deployed, the service accepts alert webhooks at:

- `POST /alerts/prometheus` - Prometheus Alertmanager
- `POST /alerts/gcp` - GCP Monitoring

Example usage with port-forward:
```bash
# Start port-forward
make port-forward

# Send test alert
curl -X POST http://localhost:8080/alerts/prometheus \
  -H "Content-Type: application/json" \
  -d '{
    "status": "firing",
    "alerts": [
      {
        "labels": {
          "alertname": "TestAlert",
          "severity": "warning"
        },
        "annotations": {
          "description": "This is a test alert"
        }
      }
    ]
  }'
```

## Troubleshooting

### Image Pull Errors

If you see `ImagePullBackOff`, ensure the image is loaded into minikube:
```bash
make load
```

### Pod Not Ready

Check pod status and logs:
```bash
kubectl get pods -n code-agent
kubectl describe pod -n code-agent <pod-name>
make logs
```

### Permission Errors

Verify security context is active:
```bash
kubectl exec -n code-agent <pod-name> -- id
# Should show: uid=10001(agent) gid=10001(agent)
```

## Production Deployment

For production deployment to a real Kubernetes cluster:

1. **Push image to registry**
   ```bash
   docker build -t ghcr.io/anthony-bible/code-agent-demo:latest .
   docker push ghcr.io/anthony-bible/code-agent-demo:latest
   ```

2. **Update image pull policy** in `deployment.yaml`:
   ```yaml
   imagePullPolicy: IfNotPresent  # or Always
   ```

3. **Configure real secrets**:
   ```bash
   kubectl create secret generic code-agent-secrets \
     -n code-agent \
     --from-literal=ANTHROPIC_API_KEY=<real-api-key>
   ```

4. **Deploy**:
   ```bash
   kubectl apply -f deploy/k8s/
   ```

5. **Verify**:
   ```bash
   kubectl rollout status deployment/code-agent-demo -n code-agent
   ```

## Resources

- [minikube Documentation](https://minikube.sigs.k8s.io/docs/)
- [Kubernetes Security Best Practices](https://kubernetes.io/docs/concepts/security/pod-security-standards/)
