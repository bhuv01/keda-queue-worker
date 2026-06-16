# Architecture

How a message in a queue becomes a scaling decision.

## Components

| Component | Role |
|-----------|------|
| **RabbitMQ** | The message broker. Holds the `tasks` queue. |
| **producer** | Publishes messages into `tasks` (a demo Job). Same image, `MODE=producer`. |
| **queue-worker** | Long-running consumer Deployment. Pulls + processes messages. **This is what KEDA scales 0→N.** |
| **KEDA operator** (`keda` ns) | Polls RabbitMQ, manages a Kubernetes HPA, and drives the worker's replica count. |
| **ScaledObject** | The scaling policy: queue=`tasks`, min=0, max=N, target msgs/replica, cooldown. |
| **TriggerAuthentication** + Secret | Supplies the broker connection string to KEDA without putting it in the ScaledObject. |

## Flow

```
 producer ──publish──▶ [ tasks queue ]  RabbitMQ
                            │  ▲
                 poll depth │  │ consume + ack
            (Management API)│  │
                            ▼  │
   KEDA ◀────────── reads queue length ──────────  queue-worker pods (0..N)
     │
     ├─ queue empty for cooldownPeriod ─▶ HPA sets replicas = 0
     └─ messages waiting                ─▶ HPA sets replicas = ceil(msgs / value)
```

1. The **producer** enqueues messages into `tasks`.
2. **KEDA** polls the queue length every `pollingInterval` seconds via the RabbitMQ
   Management API (HTTP).
3. KEDA maintains an **HPA** from the `ScaledObject`. With `minReplicaCount: 0`:
   - queue non-empty ⇒ scale **0 → 1 → N**,
   - queue empty for `cooldownPeriod` ⇒ scale back to **0**.
4. Each **worker** pod consumes messages with **manual ack**, processes, then acks.

## The two thresholds (the key KEDA concept)

- **`value`** (the `QueueLength` target) governs **1 → N**:
  `desiredReplicas = ceil(messages / value)`, clamped to `[min, max]`.
  With `value: 5`, a backlog of 32 → `ceil(32/5)` = **7 replicas**.
- **`activationValue`** governs **0 → 1**. KEDA only "activates" (leaves zero) when
  the metric is strictly above it. With `activationValue: 0`, a single message
  wakes a pod.

This split is why a scaler can sit at 0 yet still know when to wake: the metric
source (the broker) is always running, independent of the worker pods.

## Why `protocol: http` (Management API), not `amqp`

With `amqp`, the scaler counts only **ready** messages. A worker that prefetches
holds messages as **unacked** while processing — those wouldn't be counted, so
KEDA could scale in too early and stall throughput. The **HTTP Management API**
reports ready **and** unacked, giving a truer picture of outstanding work.

## No message loss on scale-down

Workers use **manual acks** and ack only after successful processing. On
scale-down Kubernetes sends `SIGTERM`; the worker stops consuming and closes the
channel. RabbitMQ automatically **requeues** any delivered-but-unacked message,
so another (or a future) pod processes it. `terminationGracePeriodSeconds` gives
in-flight work time to finish first.

## ScaledObject vs ScaledJob

- **ScaledObject** (used here): scales a persistent Deployment of consumers.
  Best when a warm consumer pool is efficient.
- **ScaledJob** (`deploy/keda/alternatives/`): spawns a Job per batch of
  messages; each pod runs to completion and exits. Best for discrete,
  run-to-completion tasks (encode, ETL, report generation).
