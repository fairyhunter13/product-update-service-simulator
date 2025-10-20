package store

import (
	"sync"

	"github.com/fairyhunter13/product-update-service-simulator/internal/model"
)

type productState struct {
	p            model.Product
	lastSequence uint64
}

type Store struct {
	mu sync.RWMutex
	m  map[string]productState
}

func New() *Store {
	return &Store{m: make(map[string]productState)}
}

func (s *Store) Get(id string) (model.Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st, ok := s.m[id]
	if !ok {
		return model.Product{}, false
	}
	return st.p, true
}

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
