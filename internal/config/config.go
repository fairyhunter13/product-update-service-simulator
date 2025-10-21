// Package config provides runtime configuration values for the service.
package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds configuration knobs for HTTP server and workers.
type Config struct {
	HTTPAddr                string
	ShutdownTimeout         time.Duration
	InitialWorkerCount      int
	WorkerMin               int
	WorkerMax               int
	ScaleInterval           time.Duration
	ScaleUpBacklogPerWorker int
	ScaleDownIdleTicks      int
	QueueHighWatermark      int
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoienv(key string, def int) int {
	v := getenv(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func durenvms(key string, defMs int) time.Duration {
	ms := atoienv(key, defMs)
	return time.Duration(ms) * time.Millisecond
}

func durenvs(key string, defSec int) time.Duration {
	sec := atoienv(key, defSec)
	return time.Duration(sec) * time.Second
}

// Load collects configuration from environment with defaults.
func Load() Config {
	minWorkers := atoienv("WORKER_MIN", 3)
	maxWorkers := atoienv("WORKER_MAX", 8)
	initialWorkers := atoienv("WORKER_COUNT", minWorkers)
	return Config{
		HTTPAddr:                getenv("HTTP_ADDR", ":8080"),
		ShutdownTimeout:         durenvs("SHUTDOWN_TIMEOUT", 15),
		InitialWorkerCount:      initialWorkers,
		WorkerMin:               minWorkers,
		WorkerMax:               maxWorkers,
		ScaleInterval:           durenvms("SCALE_INTERVAL_MS", 500),
		ScaleUpBacklogPerWorker: atoienv("SCALE_UP_BACKLOG_PER_WORKER", 100),
		ScaleDownIdleTicks:      atoienv("SCALE_DOWN_IDLE_TICKS", 6),
		QueueHighWatermark:      atoienv("QUEUE_HIGH_WATERMARK", 5000),
	}
}
