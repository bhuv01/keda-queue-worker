package main

import (
	"context"
	"log/slog"
	"math/rand"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// runWorker is the consumer that KEDA scales. It connects to RabbitMQ, consumes
// from the queue with manual acks, and simulates processing work per message.
//
// Correctness on scale-down: messages use MANUAL ack and are only acked AFTER
// successful processing. When KEDA scales this pod away, Kubernetes sends
// SIGTERM; we stop consuming and close the channel/connection. Any message that
// was delivered but not yet acked is automatically requeued by RabbitMQ and
// picked up by another (or a future) pod — so no message is lost.
func runWorker(ctx context.Context, cfg config, logger *slog.Logger) error {
	m := newMetrics()
	shutdownHealth := startHealthServer(ctx, cfg.port, m, logger)
	defer func() {
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = shutdownHealth(shCtx)
	}()

	conn, err := dialWithRetry(ctx, cfg.amqpURI, logger)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	if _, err := declareQueue(ch, cfg.queue); err != nil {
		return err
	}

	// Limit how many unacked messages this consumer holds at once. With a small
	// prefetch, queue "ready" length stays a clean scaling signal.
	if err := ch.Qos(cfg.prefetch, 0, false); err != nil {
		return err
	}

	deliveries, err := ch.Consume(
		cfg.queue,
		"",    // consumer tag (auto)
		false, // autoAck = false -> manual ack
		false, // exclusive
		false, // noLocal
		false, // noWait
		nil,
	)
	if err != nil {
		return err
	}

	m.connected.Store(true)
	logger.Info("worker ready, consuming", "queue", cfg.queue, "prefetch", cfg.prefetch)

	// Watch for broker-side closure so we can exit and let Kubernetes restart us.
	closeErr := conn.NotifyClose(make(chan *amqp.Error, 1))

	pod, _ := os.Hostname()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	for {
		select {
		case <-ctx.Done():
			logger.Info("signal received, draining: closing channel requeues unacked messages")
			m.connected.Store(false)
			return nil

		case e := <-closeErr:
			m.connected.Store(false)
			if e != nil {
				logger.Error("broker connection closed", "error", e)
			}
			return nil

		case d, ok := <-deliveries:
			if !ok {
				m.connected.Store(false)
				logger.Warn("deliveries channel closed")
				return nil
			}
			m.process(d, cfg, rng, pod, logger)
		}
	}
}

// process simulates handling a single message and acks (or nacks+requeues on a
// simulated failure).
func (m *metrics) process(d amqp.Delivery, cfg config, rng *rand.Rand, pod string, logger *slog.Logger) {
	m.inFlight.Add(1)
	defer m.inFlight.Add(-1)

	dur := workDuration(cfg.workMin, cfg.workMax, rng)
	time.Sleep(dur)

	if cfg.failRate > 0 && rng.Float64() < cfg.failRate {
		m.failed.Add(1)
		logger.Warn("processing failed, requeueing", "pod", pod, "body", short(d.Body))
		_ = d.Nack(false, true) // requeue
		return
	}

	m.processed.Add(1)
	logger.Info("processed message",
		"pod", pod, "took", dur.String(),
		"body", short(d.Body), "total", m.processed.Load())
	_ = d.Ack(false)
}

// workDuration returns a random duration in [min, max].
func workDuration(min, max time.Duration, rng *rand.Rand) time.Duration {
	if max <= min {
		return min
	}
	return min + time.Duration(rng.Int63n(int64(max-min)))
}

func short(b []byte) string {
	const n = 80
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
