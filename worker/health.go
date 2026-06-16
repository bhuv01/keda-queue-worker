package main

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// metrics holds lock-free counters exposed in Prometheus text format. The worker
// is a background consumer, but it still serves probes and metrics so it behaves
// like a first-class citizen in the cluster (and so you can scrape throughput).
type metrics struct {
	startTime time.Time
	connected atomic.Bool

	processed atomic.Int64
	failed    atomic.Int64
	inFlight  atomic.Int64
}

func newMetrics() *metrics { return &metrics{startTime: time.Now()} }

// startHealthServer runs a tiny HTTP server with /healthz, /readyz, /metrics and
// returns a shutdown func. Readiness reflects the broker connection so the pod
// is only "Ready" once it can actually consume.
func startHealthServer(ctx context.Context, port string, m *metrics, logger *slog.Logger) func(context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"alive"}`))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if m.connected.Load() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ready"}`))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"disconnected"}`))
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		write := func(s string) { _, _ = w.Write([]byte(s)) }
		i64 := func(v int64) string { return strconv.FormatInt(v, 10) }
		up := int64(0)
		if m.connected.Load() {
			up = 1
		}
		write("# HELP app_info Build info.\n# TYPE app_info gauge\n")
		write("app_info{version=\"" + version + "\",commit=\"" + commit + "\"} 1\n")
		write("# HELP broker_connection_up Whether the broker connection is established.\n# TYPE broker_connection_up gauge\n")
		write("broker_connection_up " + i64(up) + "\n")
		write("# HELP messages_processed_total Successfully processed messages.\n# TYPE messages_processed_total counter\n")
		write("messages_processed_total " + i64(m.processed.Load()) + "\n")
		write("# HELP messages_failed_total Messages that failed and were requeued.\n# TYPE messages_failed_total counter\n")
		write("messages_failed_total " + i64(m.failed.Load()) + "\n")
		write("# HELP messages_in_flight Messages currently being processed.\n# TYPE messages_in_flight gauge\n")
		write("messages_in_flight " + i64(m.inFlight.Load()) + "\n")
	})

	srv := &http.Server{Addr: ":" + port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		logger.Info("health server listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("health server error", "error", err)
		}
	}()

	return srv.Shutdown
}
