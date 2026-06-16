# KEDA Queue Scaling Demo (RabbitMQ)

An enterprise-grade, end-to-end example of **event-driven autoscaling on a message
queue** with [KEDA](https://keda.sh) — a worker that scales **from 0 to N pods
based on RabbitMQ queue depth** — deployed via **GitHub Actions + ArgoCD (GitOps)**.

> Empty queue ⇒ **0 worker pods**. Messages arrive ⇒ workers **scale up** to chew
> through the backlog. Queue drains ⇒ workers **scale back to 0**. No message is
> lost on scale-down (manual acks + requeue).

This is KEDA's original, canonical use case, and the repo is built to **learn it**:
manifests are heavily commented and `docs/ARCHITECTURE.md` walks the full
queue→scale flow, including the two thresholds people most often confuse
(`value` vs `activationValue`).

---

## What's inside

```
.
├── worker/                  # Go app: RabbitMQ consumer + producer (one image, MODE switch)
│   ├── *.go                 # config, amqp, worker, producer, health/metrics, tests
│   └── Dockerfile           # multi-stage, distroless, nonroot
├── deploy/
│   ├── base/                # Namespace, ServiceAccount, worker Deployment
│   ├── rabbitmq/            # Demo broker (Deployment+Service) + credentials Secret
│   ├── keda/                # TriggerAuthentication + ScaledObject (the scaling brain)
│   │   └── alternatives/    # ScaledJob variant + Redis-list variant (for comparison)
│   ├── demo/                # Producer Job to create a backlog
│   └── overlays/{dev,prod}/ # Kustomize overlays (max replicas, cooldown, etc.)
├── argocd/                  # AppProject + dev/prod Applications (GitOps)
├── .github/workflows/       # ci.yaml (test+validate), release.yaml (build+bump)
├── scripts/                 # install KEDA, deploy, produce, watch, queue-status
├── docs/ARCHITECTURE.md
└── Makefile
```

## Prerequisites

- A running Kubernetes cluster + `kubectl` (you said it's already set up ✅)
- `helm` 3
- (For GitOps) ArgoCD installed, and this repo pushed to GitHub
- Replace `OWNER` placeholders with your GitHub org/user

```bash
grep -rl "OWNER" . | xargs sed -i 's#OWNER#your-gh-username#g'
```

---

## Quick start (no ArgoCD) — see scaling in ~5 minutes

```bash
# 1. Install KEDA core (RabbitMQ scaler is built in — no add-on required)
./scripts/00-install-keda.sh

# 2. Deploy RabbitMQ + worker + KEDA resources (dev overlay)
./scripts/01-deploy.sh dev
#    The worker settles to 0 replicas once the empty queue passes cooldown.

# 3. Terminal A — watch it scale
./scripts/watch.sh

# 4. Terminal B — enqueue a backlog
./scripts/produce.sh 200
#    Watch Terminal A: 0 -> 1 -> several worker pods, then back to 0 as the
#    queue drains and cooldown elapses.

# (optional) see live queue depth
./scripts/queue-status.sh
```

> If you build/push your own image, pass it to produce.sh:
> `./scripts/produce.sh 200 ghcr.io/you/keda-queue-worker:dev`

---

## GitOps deployment (GitHub Actions + ArgoCD)

1. Push to GitHub and replace `OWNER`.
2. **CI** (`.github/workflows/ci.yaml`): Go vet + tests, Docker build,
   `kustomize build`, and `kubeconform` schema validation on every PR/push.
3. **Release** (`.github/workflows/release.yaml`) on a `v*` tag: builds & pushes
   the image to **GHCR**, then `kustomize edit set image` bumps the prod tag and
   commits it back.
4. **ArgoCD** syncs:

   ```bash
   kubectl apply -f argocd/project.yaml
   kubectl apply -f argocd/application-dev.yaml
   kubectl apply -f argocd/application-prod.yaml
   ```

   Apps use automated sync (prune + selfHeal) and **ignore drift on the worker's
   `spec.replicas`**, because KEDA owns it.

```bash
git tag v0.1.0 && git push origin v0.1.0   # cut a release
```

---

## How scaling is configured

`deploy/keda/scaledobject.yaml` is the heart of it:

```yaml
minReplicaCount: 0        # scale to zero
maxReplicaCount: 10       # the N (overlay: dev=5, prod=30)
triggers:
  - type: rabbitmq
    metadata:
      protocol: http       # Management API: counts ready + unacked messages
      queueName: tasks
      mode: QueueLength
      value: "5"           # target messages per replica  -> governs 1..N
      activationValue: "0" # governs 0..1
    authenticationRef:
      name: keda-trigger-auth-rabbitmq
```

| Setting | dev | prod |
|---|---|---|
| `maxReplicaCount` (N) | 5 | 30 |
| `cooldownPeriod` (to zero) | 30s | 300s |
| worker `PREFETCH` | 1 | 5 |
| PodDisruptionBudget | – | ✅ |

See `docs/ARCHITECTURE.md` for the scaling math and the `value` vs
`activationValue` distinction.

---

## The app (worker/)

One small Go binary, two modes:

- `MODE=worker` (default) — consumes `tasks`, simulates processing
  (`WORK_MS_MIN..WORK_MS_MAX`), **manual ack** after success, requeue on failure.
- `MODE=producer` — publishes `COUNT` messages and exits (used by the demo Job).

Production touches: structured JSON logging (`slog`), broker dial **retry with
backoff**, graceful shutdown that lets in-flight messages finish or be safely
requeued, `/healthz` + `/readyz` (readiness tracks the broker connection) +
Prometheus `/metrics`, distroless nonroot image, read-only root FS, dropped caps.

---

## Learn-more knobs

- Switch to **ScaledJob** (one Job per batch): `deploy/keda/alternatives/scaledjob-rabbitmq.yaml`.
- Switch broker to **Redis**: `deploy/keda/alternatives/scaledobject-redis.yaml`.
- Set `FAIL_RATE=0.2` on the worker to watch nack/requeue behaviour.
- Lower `value` or raise `COUNT` to force more aggressive scale-out.

## Troubleshooting

- **Never scales up** — check `kubectl get scaledobject -n keda-demo` (READY/ACTIVE)
  and `kubectl describe scaledobject queue-worker -n keda-demo`; verify the
  Management API URI/credentials in the Secret.
- **Scales up but never to zero** — make sure only ONE autoscaler targets the
  Deployment, no static `replicas:` is set, and the queue is actually empty
  (`./scripts/queue-status.sh`).
- **Worker CrashLoop on start** — usually the broker isn't ready yet; the worker
  retries with backoff, but confirm `kubectl get pods -n keda-demo` shows
  RabbitMQ Ready.

## Clean up

```bash
kubectl delete -k deploy/overlays/dev   # or prod
helm uninstall keda -n keda
```

## Notes on versions

Uses the current KEDA RabbitMQ scaler spec (`mode: QueueLength` + `value`; the old
`queueLength` field is deprecated). The bundled RabbitMQ is a single-node demo
broker — use the RabbitMQ Cluster Operator or a managed service in production, and
keep secrets out of Git (Sealed Secrets / External Secrets Operator). License: MIT.
