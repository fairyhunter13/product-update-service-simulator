package store

import (
	"sync"
	"testing"

	"github.com/fairyhunter13/product-update-service-simulator/internal/model"
)

func TestStorePartialUpdates(t *testing.T) {
	s := New()
	p := 10.5
	s.Upsert(model.Event{ProductID: "p1", Price: &p, Sequence: 1})
	s7 := int64(7)
	s.Upsert(model.Event{ProductID: "p1", Stock: &s7, Sequence: 2})
	got, ok := s.Get("p1")
	if !ok {
		t.Fatalf("not found")
	}
	if got.Price != 10.5 || got.Stock != 7 {
		t.Fatalf("unexpected: %+v", got)
	}
}

func TestStoreLastWriteWins(t *testing.T) {
	s := New()
	p := 1.0
	s.Upsert(model.Event{ProductID: "p2", Price: &p, Sequence: 2})
	pOld := 99.0
	s.Upsert(model.Event{ProductID: "p2", Price: &pOld, Sequence: 1})
	got, _ := s.Get("p2")
	if got.Price != 1.0 {
		t.Fatalf("expected 1.0, got %v", got.Price)
	}
}

func TestStoreConcurrentUpserts(t *testing.T) {
	s := New()
	id := "p3"
	var wg sync.WaitGroup
	for i := 1; i <= 100; i++ {
		seq := uint64(i)
		st := int64(i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Upsert(model.Event{ProductID: id, Stock: &st, Sequence: seq})
		}()
	}
	wg.Wait()
	got, ok := s.Get(id)
	if !ok {
		t.Fatalf("not found")
	}
	if got.Stock != 100 {
		t.Fatalf("expected 100, got %d", got.Stock)
	}
}
