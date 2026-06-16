package main

import (
	"log/slog"
	"os"
	"strconv"
	"time"
)

// config is sourced entirely from environment variables so it can be set
// declaratively from the Kubernetes manifests. One binary, two modes
// (worker | producer), selected by MODE.
type config struct {
	mode    string // MODE: "worker" (default) or "producer"
	amqpURI string // AMQP_URI, e.g. amqp://app:pass@rabbitmq:5672/
	queue   string // QUEUE_NAME

	// worker tuning
	prefetch int           // PREFETCH: unacked messages held per consumer
	workMin  time.Duration // WORK_MS_MIN: min simulated processing time
	workMax  time.Duration // WORK_MS_MAX: max simulated processing time
	failRate float64       // FAIL_RATE: 0..1, fraction of messages to nack+requeue
	port     string        // PORT: health/metrics HTTP port

	// producer tuning
	count        int           // COUNT: number of messages to publish
	publishDelay time.Duration // PUBLISH_DELAY between messages
}

func loadConfig() config {
	return config{
		mode:    getenv("MODE", "worker"),
		amqpURI: getenv("AMQP_URI", "amqp://guest:guest@localhost:5672/"),
		queue:   getenv("QUEUE_NAME", "tasks"),

		prefetch: getint("PREFETCH", 1),
		workMin:  getms("WORK_MS_MIN", 250*time.Millisecond),
		workMax:  getms("WORK_MS_MAX", 750*time.Millisecond),
		failRate: getfloat("FAIL_RATE", 0),
		port:     getenv("PORT", "8080"),

		count:        getint("COUNT", 100),
		publishDelay: getms("PUBLISH_DELAY", 0),
	}
}

func getenv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func getint(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getfloat(key string, def float64) float64 {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}

// getms reads either a plain integer of milliseconds ("250") or a Go duration
// string ("250ms", "1s") for convenience.
func getms(key string, def time.Duration) time.Duration {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	if n, err := strconv.Atoi(v); err == nil {
		return time.Duration(n) * time.Millisecond
	}
	if d, err := time.ParseDuration(v); err == nil {
		return d
	}
	return def
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
