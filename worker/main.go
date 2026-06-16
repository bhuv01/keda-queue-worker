// Command keda-queue-worker is a small message-queue application used to learn
// KEDA's event-driven autoscaling. It runs in one of two modes:
//
//	MODE=worker   (default) — consumes messages from a RabbitMQ queue and
//	                          processes them. This is the Deployment that KEDA
//	                          scales from 0 to N based on queue length.
//	MODE=producer           — publishes COUNT messages to the queue and exits.
//	                          Used by the demo Job to create a backlog.
//
// Using a single binary/image for both keeps the demo simple to deploy.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// Build metadata injected via -ldflags (see Dockerfile).
var (
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(getenv("LOG_LEVEL", "info")),
	}))
	slog.SetDefault(logger)

	cfg := loadConfig()

	// Terminate cleanly on SIGTERM (sent by Kubernetes on scale-down).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("starting keda-queue-worker",
		"mode", cfg.mode, "version", version, "commit", commit,
		"queue", cfg.queue)

	var err error
	switch cfg.mode {
	case "producer":
		err = runProducer(ctx, cfg, logger)
	case "worker":
		err = runWorker(ctx, cfg, logger)
	default:
		logger.Error("unknown MODE", "mode", cfg.mode)
		os.Exit(2)
	}

	if err != nil && ctx.Err() == nil {
		logger.Error("exited with error", "mode", cfg.mode, "error", err)
		os.Exit(1)
	}
	logger.Info("shutdown complete", "mode", cfg.mode)
}
