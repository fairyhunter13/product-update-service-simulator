// Package store provides an in-memory product state store for the simulator.
package store

import (
	"sync"

	"github.com/fairyhunter13/product-update-service-simulator/internal/model"
)

// productState holds a product and its last sequence number.
type productState struct {
	p            model.Product
	lastSequence uint64
}

// Store holds product states with concurrency protection.
type Store struct {
	mu sync.RWMutex
	m  map[string]productState
}

// New creates a new in-memory Store.
func New() *Store {
	return &Store{m: make(map[string]productState)}
}

// Get retrieves a product by ID.
func (s *Store) Get(id string) (model.Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.m[id]
	if !ok {
		return model.Product{}, false
	}
	return st.p, true
}

// Upsert applies an event to the product state with simple sequence checks.
func (s *Store) Upsert(ev model.Event) {
	if ev.ProductID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.m[ev.ProductID]
	if ok {
		if ev.Sequence < st.lastSequence {
			return
		}
		if ev.Sequence == st.lastSequence {
			return
		}
		if ev.Price != nil {
			st.p.Price = *ev.Price
		}
		if ev.Stock != nil {
			st.p.Stock = *ev.Stock
		}
		st.lastSequence = ev.Sequence
		s.m[ev.ProductID] = st
		return
	}
	// new entry
	p := model.Product{ProductID: ev.ProductID}
	if ev.Price != nil {
		p.Price = *ev.Price
	}
	if ev.Stock != nil {
		p.Stock = *ev.Stock
	}
	s.m[ev.ProductID] = productState{p: p, lastSequence: ev.Sequence}
}
