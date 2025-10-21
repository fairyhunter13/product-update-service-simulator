package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("SHUTDOWN_TIMEOUT", "")
	t.Setenv("WORKER_MIN", "")
	t.Setenv("WORKER_MAX", "")
	t.Setenv("WORKER_COUNT", "")
	t.Setenv("SCALE_INTERVAL_MS", "")
	t.Setenv("SCALE_UP_BACKLOG_PER_WORKER", "")
	t.Setenv("SCALE_DOWN_IDLE_TICKS", "")
	t.Setenv("QUEUE_HIGH_WATERMARK", "")
	c := Load()
	if c.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr default")
	}
	if c.ShutdownTimeout != 15*time.Second {
		t.Fatalf("ShutdownTimeout default")
	}
	if c.WorkerMin != 3 || c.WorkerMax != 5 {
		t.Fatalf("worker bounds default")
	}
	if c.ScaleInterval != 500*time.Millisecond {
		t.Fatalf("ScaleInterval default")
	}
	if c.ScaleUpBacklogPerWorker != 100 || c.ScaleDownIdleTicks != 6 {
		t.Fatalf("scale thresholds default")
	}
	if c.QueueHighWatermark != 5000 {
		t.Fatalf("high watermark default")
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("SHUTDOWN_TIMEOUT", "2")
	t.Setenv("WORKER_MIN", "2")
	t.Setenv("WORKER_MAX", "3")
	t.Setenv("WORKER_COUNT", "2")
	t.Setenv("SCALE_INTERVAL_MS", "250")
	t.Setenv("SCALE_UP_BACKLOG_PER_WORKER", "10")
	t.Setenv("SCALE_DOWN_IDLE_TICKS", "2")
	t.Setenv("QUEUE_HIGH_WATERMARK", "99")
	c := Load()
	if c.HTTPAddr != ":9090" {
		t.Fatalf("HTTPAddr env")
	}
	if c.ShutdownTimeout != 2*time.Second {
		t.Fatalf("ShutdownTimeout env")
	}
	if c.WorkerMin != 2 || c.WorkerMax != 3 || c.InitialWorkerCount != 2 {
		t.Fatalf("workers env")
	}
	if c.ScaleInterval != 250*time.Millisecond {
		t.Fatalf("ScaleInterval env")
	}
	if c.ScaleUpBacklogPerWorker != 10 || c.ScaleDownIdleTicks != 2 {
		t.Fatalf("scale thresholds env")
	}
	if c.QueueHighWatermark != 99 {
		t.Fatalf("high watermark env")
	}
	_ = os.Unsetenv("HTTP_ADDR")
}
