package queue

import (
	"context"
	"testing"
	"time"

	"github.com/fairyhunter13/product-update-service-simulator/internal/config"
	"github.com/fairyhunter13/product-update-service-simulator/internal/model"
	"github.com/fairyhunter13/product-update-service-simulator/internal/obs"
	"github.com/fairyhunter13/product-update-service-simulator/internal/store"
)

func TestManagerScaler_UpAndDown(t *testing.T) {
	// Configure aggressive scaling
	t.Setenv("WORKER_MIN", "1")
	t.Setenv("WORKER_MAX", "3")
	t.Setenv("WORKER_COUNT", "1")
	t.Setenv("SCALE_INTERVAL_MS", "50")
	t.Setenv("SCALE_UP_BACKLOG_PER_WORKER", "1")
	t.Setenv("SCALE_DOWN_IDLE_TICKS", "1")

	cfg := config.Load()
	obs.InitLogger()
	st := store.New()
	q := New(8)
	mgr := NewManager(cfg, q, st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Start(ctx)

	// Enqueue backlog to trigger scale up
	for i := 0; i < 50; i++ {
		p := float64(i)
		_ = mgr.Enqueue(model.Event{ProductID: "scale", Price: &p})
	}

	// Wait until worker count increases
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if wc := mgr.WorkerCount(); wc > 1 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if wc := mgr.WorkerCount(); wc <= 1 {
		t.Fatalf("expected scale up, worker_count=%d", wc)
	}

	// Wait for drain
	ctxDrain, cancelDrain := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelDrain()
	if ok := mgr.DrainUntil(ctxDrain); !ok {
		t.Fatalf("drain timeout")
	}
	// Allow scaler to tick and scale down to min
	deadline2 := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline2) {
		if wc := mgr.WorkerCount(); wc == cfg.WorkerMin {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if wc := mgr.WorkerCount(); wc != cfg.WorkerMin {
		t.Fatalf("expected scale down to %d, got %d", cfg.WorkerMin, wc)
	}
}
