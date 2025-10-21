// Package queue implements an in-memory event queue and worker manager.
package queue

import (
	"context"
	"sync"
	"time"

	"github.com/fairyhunter13/product-update-service-simulator/internal/config"
	"github.com/fairyhunter13/product-update-service-simulator/internal/model"
	"github.com/fairyhunter13/product-update-service-simulator/internal/obs"
	"github.com/fairyhunter13/product-update-service-simulator/internal/store"
)

// Manager coordinates workers processing queued events and scaling.
type Manager struct {
	cfg    config.Config
	q      *Queue
	st     *store.Store
	seq    Sequencer
	ctx    context.Context
	cancel context.CancelFunc

	mu            sync.Mutex
	workerCancels []context.CancelFunc
}

// NewManager constructs a Manager with the given config, queue, and store.
func NewManager(cfg config.Config, q *Queue, st *store.Store) *Manager {
	return &Manager{cfg: cfg, q: q, st: st}
}

// Start begins processing and autoscaling in the background.
func (m *Manager) Start(parent context.Context) {
	m.ctx, m.cancel = context.WithCancel(parent)
	m.q.Start(m.ctx, m.cfg.QueueHighWatermark)
	m.addWorkers(m.cfg.InitialWorkerCount)
	go m.scaler()
}

// Stop cancels background routines and stops workers.
func (m *Manager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Lock()
	for _, c := range m.workerCancels {
		c()
	}
	m.workerCancels = nil
	m.mu.Unlock()
}

// scaler adjusts worker count based on backlog and configuration.
func (m *Manager) scaler() {
	t := time.NewTicker(m.cfg.ScaleInterval)
	defer t.Stop()
	idleTicks := 0
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-t.C:
			backlog := m.q.BacklogSize()
			wc := m.WorkerCount()
			if backlog > wc*m.cfg.ScaleUpBacklogPerWorker && wc < m.cfg.WorkerMax {
				m.addWorkers(1)
				idleTicks = 0
				continue
			}
			if backlog == 0 {
				idleTicks++
				if idleTicks >= m.cfg.ScaleDownIdleTicks && wc > m.cfg.WorkerMin {
					m.removeWorkers(1)
					idleTicks = 0
				}
			} else {
				idleTicks = 0
			}
		}
	}
}

// addWorkers spawns n workers.
func (m *Manager) addWorkers(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := 0; i < n; i++ {
		wctx, cancel := context.WithCancel(m.ctx)
		m.workerCancels = append(m.workerCancels, cancel)
		go m.worker(wctx)
	}
	obs.Logger.Info("workers scaled", "worker_count", len(m.workerCancels))
}

// removeWorkers stops up to n workers.
func (m *Manager) removeWorkers(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n > len(m.workerCancels) {
		n = len(m.workerCancels)
	}
	for i := 0; i < n; i++ {
		c := m.workerCancels[len(m.workerCancels)-1]
		m.workerCancels = m.workerCancels[:len(m.workerCancels)-1]
		c()
	}
	obs.Logger.Info("workers scaled", "worker_count", len(m.workerCancels))
}

// worker drains events from the queue and updates the store.
func (m *Manager) worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-m.q.Out():
			m.st.Upsert(ev)
			m.q.MarkProcessed()
		}
	}
}

// Enqueue proxies to the underlying queue.
func (m *Manager) Enqueue(ev model.Event) bool { return m.q.Enqueue(ev) }

// BacklogSize returns pending items in the queue.
func (m *Manager) BacklogSize() int { return m.q.BacklogSize() }

// QueueDepth returns backlog plus buffered output items.
func (m *Manager) QueueDepth() int { return m.q.QueueDepth() }

// WorkerCount returns the current number of workers.
func (m *Manager) WorkerCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.workerCancels)
}

// NextSequence returns the next sequence number.
func (m *Manager) NextSequence() uint64 { return m.seq.Next() }

// IsShuttingDown reports whether new enqueues are rejected.
func (m *Manager) IsShuttingDown() bool { return m.q.IsShuttingDown() }

// CloseIntake disallows future enqueues.
func (m *Manager) CloseIntake() { m.q.CloseIntake() }

// QueueMetrics exposes the underlying queue metrics.
func (m *Manager) QueueMetrics() (enq, proc uint64, backlog, depth int) {
	return m.q.Metrics()
}

// DrainUntil blocks until the queue is fully drained or context is done.
func (m *Manager) DrainUntil(ctx context.Context) bool {
	for {
		enq, proc, backlog, depth := m.q.Metrics()
		if backlog == 0 && depth == 0 && enq == proc {
			return true
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(50 * time.Millisecond):
		}
	}
}
