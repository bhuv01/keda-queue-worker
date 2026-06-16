#!/usr/bin/env bash
# Install KEDA core via Helm. No HTTP add-on needed for queue scaling — the
# RabbitMQ scaler is built into KEDA core.
set -euo pipefail
helm repo add kedacore https://kedacore.github.io/charts
helm repo update
helm upgrade --install keda kedacore/keda \
  --namespace keda --create-namespace --wait
kubectl rollout status deploy/keda-operator -n keda --timeout=180s
echo "KEDA core installed."
kubectl get pods -n keda
