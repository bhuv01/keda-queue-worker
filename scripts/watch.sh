#!/usr/bin/env bash
# Watch KEDA scale the worker as the queue grows and drains.
set -euo pipefail
watch -n 1 '
echo "== ScaledObject ==";  kubectl get scaledobject -n keda-demo;
echo; echo "== HPA (managed by KEDA) =="; kubectl get hpa -n keda-demo;
echo; echo "== Worker Deployment ==";     kubectl get deploy queue-worker -n keda-demo;
echo; echo "== Pods ==";                  kubectl get pods -n keda-demo -l app.kubernetes.io/name=queue-worker
'
