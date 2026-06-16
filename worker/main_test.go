package main

import (
	"math/rand"
	"os"
	"testing"
	"time"
)

func TestGetms(t *testing.T) {
	os.Setenv("X_MS", "250")
	defer os.Unsetenv("X_MS")
	if got := getms("X_MS", time.Second); got != 250*time.Millisecond {
		t.Fatalf("plain int ms: want 250ms, got %s", got)
	}

	os.Setenv("X_DUR", "1s")
	defer os.Unsetenv("X_DUR")
	if got := getms("X_DUR", 0); got != time.Second {
		t.Fatalf("duration string: want 1s, got %s", got)
	}

	if got := getms("X_MISSING", 5*time.Second); got != 5*time.Second {
		t.Fatalf("default: want 5s, got %s", got)
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	for _, k := range []string{"MODE", "QUEUE_NAME", "PREFETCH", "COUNT"} {
		os.Unsetenv(k)
	}
	c := loadConfig()
	if c.mode != "worker" {
		t.Errorf("default mode: want worker, got %q", c.mode)
	}
	if c.queue != "tasks" {
		t.Errorf("default queue: want tasks, got %q", c.queue)
	}
	if c.prefetch != 1 {
		t.Errorf("default prefetch: want 1, got %d", c.prefetch)
	}
}

func TestWorkDuration(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	min, max := 100*time.Millisecond, 200*time.Millisecond
	for i := 0; i < 1000; i++ {
		d := workDuration(min, max, rng)
		if d < min || d >= max {
			t.Fatalf("workDuration out of range: %s", d)
		}
	}
	// degenerate range returns min
	if d := workDuration(max, min, rng); d != max {
		t.Fatalf("degenerate range: want %s, got %s", max, d)
	}
}

func TestShort(t *testing.T) {
	if short([]byte("hello")) != "hello" {
		t.Error("short should pass through small input")
	}
	long := make([]byte, 200)
	if len(short(long)) >= 200 {
		t.Error("short should truncate long input")
	}
}

func TestMetricsConnectedFlag(t *testing.T) {
	m := newMetrics()
	if m.connected.Load() {
		t.Error("metrics should start disconnected")
	}
	m.connected.Store(true)
	if !m.connected.Load() {
		t.Error("connected flag not set")
	}
}
