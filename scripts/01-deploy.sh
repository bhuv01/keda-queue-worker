#!/usr/bin/env bash
# Deploy RabbitMQ + worker + KEDA resources for an environment (no ArgoCD).
#   Usage: ./01-deploy.sh [dev|prod]
set -euo pipefail
ENV="${1:-dev}"
kubectl apply -k "deploy/overlays/${ENV}"
echo "Waiting for RabbitMQ to be ready..."
kubectl rollout status deploy/rabbitmq -n keda-demo --timeout=180s
echo "Applied overlay: ${ENV}"
kubectl get scaledobject,deploy,po -n keda-demo
