package queue

import (
	"context"
	"testing"

	"github.com/fairyhunter13/product-update-service-simulator/internal/config"
	"github.com/fairyhunter13/product-update-service-simulator/internal/model"
	"github.com/fairyhunter13/product-update-service-simulator/internal/obs"
	"github.com/fairyhunter13/product-update-service-simulator/internal/store"
)

func TestQueueNonBlockingEnqueue(t *testing.T) {
	q := New(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	q.Start(ctx, 0)
	for i := 0; i < 1000; i++ {
		p := float64(i)
		ev := model.Event{ProductID: "x", Price: &p}
		ok := q.Enqueue(ev)
		if !ok {
			t.Fatalf("enqueue failed at %d", i)
		}
	}
	if q.BacklogSize() == 0 {
		t.Fatalf("expected backlog > 0")
	}
}

func TestQueueShutdownIntake(t *testing.T) {
	q := New(1)
	q.CloseIntake()
	if !q.IsShuttingDown() {
		t.Fatalf("expected shutting down true")
	}
	p := 1.0
	ok := q.Enqueue(model.Event{ProductID: "x", Price: &p})
	if ok {
		t.Fatalf("expected enqueue false when shutting down")
	}
}

func TestManagerDrain(t *testing.T) {
	cfg := config.Load()
	obs.InitLogger()
	st := store.New()
	q := New(16)
	mgr := NewManager(cfg, q, st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Start(ctx)
	for i := 0; i < 100; i++ {
		p := float64(i)
		_ = mgr.Enqueue(model.Event{ProductID: "xx", Price: &p})
	}
	ctxDrain, cancelDrain := context.WithCancel(context.Background())
	defer cancelDrain()
	if ok := mgr.DrainUntil(ctxDrain); !ok {
		t.Fatalf("expected drain true")
	}
}
