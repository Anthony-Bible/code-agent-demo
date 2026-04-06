# Makefile for Kubernetes deployment automation with minikube
# Usage: make <target>

# Variables (override with: make IMAGE_TAG=v1.0.0 <target>)
IMAGE_NAME ?= ghcr.io/anthony-bible/code-agent-demo
IMAGE_TAG ?= latest
FULL_IMAGE := $(IMAGE_NAME):$(IMAGE_TAG)
NAMESPACE ?= code-agent

# Kubernetes resources
K8S_DIR := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))
CONTEXT := $(K8S_DIR)

# Colors for output
BLUE := \033[0;34m
GREEN := \033[0;32m
YELLOW := \033[0;33m
RED := \033[0;31m
NC := \033[0m # No Color

.PHONY: help build load deploy test clean logs status all

help: ## Show this help message
	@echo "$(BLUE)Kubernetes Deployment Makefile for minikube$(NC)"
	@echo ""
	@echo "$(GREEN)Available targets:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-15s$(NC) %s\n", $$1, $$2}'
	@echo ""
	@echo "$(GREEN)Variables:$(NC)"
	@echo "  IMAGE_NAME    Image name (default: $(IMAGE_NAME))"
	@echo "  IMAGE_TAG     Image tag (default: $(IMAGE_TAG))"
	@echo "  NAMESPACE     Kubernetes namespace (default: $(NAMESPACE))"

check-minikube: ## Check if minikube is running
	@minikube status > /dev/null 2>&1 || { echo "$(RED)Error: minikube is not running$(NC)"; exit 1; }

build: check-minikube ## Build Docker image
	@echo "$(BLUE)Building Docker image: $(FULL_IMAGE)$(NC)"
	docker build -t $(FULL_IMAGE) \
		--build-arg REVISION=$$(git rev-parse --short HEAD 2>/dev/null || echo "unknown") \
		--build-arg CREATED=$$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
		-f $(K8S_DIR)/../../Dockerfile $(K8S_DIR)/../..
	@echo "$(GREEN)Build complete: $(FULL_IMAGE)$(NC)"

load: build ## Load image into minikube
	@echo "$(BLUE)Loading image into minikube...$(NC)"
	minikube image load $(FULL_IMAGE)
	@echo "$(GREEN)Image loaded to minikube$(NC)"
	# Update deployment image reference
	@kubectl set image deployment/code-agent-demo \
		code-agent-demo=$(FULL_IMAGE) -n $(NAMESPACE) --context=$$(minikube kubectl -- profile) 2>/dev/null || echo "$(YELLOW)Note: Run 'make deploy' first to create deployment$(NC)"

deploy: ## Create Kubernetes resources
	@echo "$(BLUE)Deploying to Kubernetes...$(NC)"
	kubectl create namespace $(NAMESPACE) --dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f $(K8S_DIR)namespace.yaml
	kubectl apply -f $(K8S_DIR)configmap.yaml
	kubectl apply -f $(K8S_DIR)configmap-alert-sources.yaml
	# Create secret if it doesn't exist
	kubectl create secret generic code-agent-secrets \
		-n $(NAMESPACE) \
		--from-literal=ANTHROPIC_API_KEY=test-key \
		--dry-run=client -o yaml | kubectl apply -f -
	kubectl apply -f $(K8S_DIR)deployment.yaml
	kubectl apply -f $(K8S_DIR)service.yaml
	@echo "$(GREEN)Deployment applied$(NC)"

wait-ready: ## Wait for deployment to be ready
	@echo "$(BLUE)Waiting for deployment to be ready...$(NC)"
	kubectl wait --for=condition=available --timeout=60s \
		deployment/code-agent-demo -n $(NAMESPACE)
	@echo "$(GREEN)Deployment is ready$(NC)"

test: wait-ready ## Test the deployed service
	@echo "$(BLUE)Running health checks...$(NC)"
	@POD=$$(kubectl get pod -n $(NAMESPACE) -l app.kubernetes.io/name=code-agent-demo -o jsonpath='{.items[0].metadata.name}'); \
	echo "$(YELLOW)Testing health endpoint...$(NC)"; \
	kubectl exec -n $(NAMESPACE) $$POD -- wget -qO- http://localhost:8080/health | jq .; \
	echo "$(YELLOW)Testing ready endpoint...$(NC)"; \
	kubectl exec -n $(NAMESPACE) $$POD -- wget -qO- http://localhost:8080/ready | jq .; \
	echo "$(YELLOW)Verifying non-root user...$(NC)"; \
	kubectl exec -n $(NAMESPACE) $$POD -- id; \
	echo "$(YELLOW)Verifying read-only root filesystem...$(NC)"; \
	kubectl exec -n $(NAMESPACE) $$POD -- touch /app/test.txt 2>&1 | grep -q "Read-only" && echo "  ✓ Read-only enforced" || echo "  ✗ Write allowed (unexpected)"; \
	echo "$(GREEN)All checks passed!$(NC)"

port-forward: ## Port-forward service to localhost:8080
	@echo "$(BLUE)Port-forwarding service to localhost:8080...$(NC)"
	@kubectl port-forward -n $(NAMESPACE) svc/code-agent-demo 8080:8080

logs: ## Show deployment logs (follow mode)
	kubectl logs -n $(NAMESPACE) -l app.kubernetes.io/name=code-agent-demo -f

status: ## Show deployment status
	@echo "$(BLUE)Deployment Status:$(NC)"
	@echo ""
	@echo "$(YELLOW)Pods:$(NC)"
	kubectl get pods -n $(NAMESPACE) -l app.kubernetes.io/name=code-agent-demo
	@echo ""
	@echo "$(YELLOW)Service:$(NC)"
	kubectl get svc -n $(NAMESPACE) code-agent-demo
	@echo ""
	@echo "$(YELLOW)Security Context:$(NC)"
	@POD=$$(kubectl get pod -n $(NAMESPACE) -l app.kubernetes.io/name=code-agent-demo -o jsonpath='{.items[0].metadata.name}'); \
	kubectl exec -n $(NAMESPACE) $$POD -- id

clean: ## Remove all Kubernetes resources
	@echo "$(YELLOW)Removing Kubernetes resources...$(NC)"
	kubectl delete namespace $(NAMESPACE) --ignore-not-found=true
	@echo "$(GREEN)Cleanup complete$(NC)"

all: clean build load deploy test ## Full workflow: clean, build, load, deploy, test

.DEFAULT_GOAL := help
