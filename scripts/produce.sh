#!/usr/bin/env bash
# Enqueue a backlog of messages to trigger KEDA scale-up.
#   Usage: ./produce.sh [count] [image]
# Defaults: 200 messages using the dev image tag.
set -euo pipefail
COUNT="${1:-200}"
IMAGE="${2:-ghcr.io/OWNER/keda-queue-worker:dev}"

# Apply a templated one-off producer Job:
kubectl apply -n keda-demo -f - <<JOB
apiVersion: batch/v1
kind: Job
metadata:
  generateName: producer-
  namespace: keda-demo
spec:
  backoffLimit: 3
  ttlSecondsAfterFinished: 300
  template:
    spec:
      restartPolicy: Never
      securityContext: { runAsNonRoot: true, runAsUser: 65532, seccompProfile: { type: RuntimeDefault } }
      containers:
        - name: producer
          image: ${IMAGE}
          env:
            - { name: MODE, value: "producer" }
            - { name: QUEUE_NAME, value: "tasks" }
            - { name: COUNT, value: "${COUNT}" }
            - name: AMQP_URI
              valueFrom: { secretKeyRef: { name: rabbitmq-credentials, key: amqp-uri } }
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: { drop: ["ALL"] }
JOB
echo "Producer Job created to enqueue ${COUNT} messages. Watch scaling with ./scripts/watch.sh"
