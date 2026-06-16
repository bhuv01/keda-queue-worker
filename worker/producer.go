package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// task is the message payload. Kept tiny on purpose.
type task struct {
	ID     int       `json:"id"`
	Issued time.Time `json:"issued"`
	Note   string    `json:"note"`
}

// runProducer publishes cfg.count messages to the queue and exits. It is used by
// the demo Job to create a backlog that KEDA reacts to.
func runProducer(ctx context.Context, cfg config, logger *slog.Logger) error {
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

	q, err := declareQueue(ch, cfg.queue)
	if err != nil {
		return err
	}

	logger.Info("publishing messages", "count", cfg.count, "queue", q.Name)

	for i := 1; i <= cfg.count; i++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		body, _ := json.Marshal(task{
			ID:     i,
			Issued: time.Now().UTC(),
			Note:   fmt.Sprintf("demo task %d/%d", i, cfg.count),
		})

		pubCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err := ch.PublishWithContext(pubCtx, "", q.Name, false, false, amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent, // survive broker restart
			Timestamp:    time.Now(),
			Body:         body,
		})
		cancel()
		if err != nil {
			return fmt.Errorf("publish %d: %w", i, err)
		}

		if cfg.publishDelay > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(cfg.publishDelay):
			}
		}
		if i%50 == 0 {
			logger.Info("published", "so_far", i, "of", cfg.count)
		}
	}

	logger.Info("done publishing", "count", cfg.count)
	return nil
}
