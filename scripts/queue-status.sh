#!/usr/bin/env bash
# Show the live RabbitMQ queue depth via the Management API (port-forwarded).
set -euo pipefail
USER="$(kubectl get secret rabbitmq-credentials -n keda-demo -o jsonpath='{.data.username}' | base64 -d)"
PASS="$(kubectl get secret rabbitmq-credentials -n keda-demo -o jsonpath='{.data.password}' | base64 -d)"
kubectl port-forward -n keda-demo svc/rabbitmq 15672:15672 >/tmp/rmq-pf.log 2>&1 &
PF=$!; trap 'kill $PF 2>/dev/null || true' EXIT
sleep 3
echo "Queue depth (messages ready/unacked):"
curl -s -u "${USER}:${PASS}" http://localhost:15672/api/queues/%2F/tasks \
  | grep -o '"messages[_a-z]*":[0-9]*' || echo "queue 'tasks' not found yet"
