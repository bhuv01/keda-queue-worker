package main

import (
	"context"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// dialWithRetry connects to RabbitMQ, retrying with capped backoff. A worker pod
// started from zero by KEDA may briefly race ahead of the broker being routable,
// so we never crash on the first failed dial — we back off and retry until the
// context is cancelled.
func dialWithRetry(ctx context.Context, uri string, logger *slog.Logger) (*amqp.Connection, error) {
	const (
		base = 500 * time.Millisecond
		max  = 10 * time.Second
	)
	backoff := base
	for attempt := 1; ; attempt++ {
		conn, err := amqp.Dial(uri)
		if err == nil {
			logger.Info("connected to broker", "attempt", attempt)
			return conn, nil
		}
		logger.Warn("broker dial failed, retrying", "attempt", attempt, "backoff", backoff.String(), "error", err)

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(backoff):
		}
		if backoff < max {
			backoff *= 2
			if backoff > max {
				backoff = max
			}
		}
	}
}

// declareQueue makes sure the queue exists and is durable. Idempotent: both the
// producer and the worker call it so neither depends on start-up order.
func declareQueue(ch *amqp.Channel, name string) (amqp.Queue, error) {
	return ch.QueueDeclare(
		name,
		true,  // durable — survives broker restart
		false, // autoDelete
		false, // exclusive
		false, // noWait
		nil,   // args
	)
}
