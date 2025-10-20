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

func NewManager(cfg config.Config, q *Queue, st *store.Store) *Manager {
	return &Manager{cfg: cfg, q: q, st: st}
}

func (m *Manager) Start(parent context.Context) {
	m.ctx, m.cancel = context.WithCancel(parent)
	m.q.Start(m.ctx, m.cfg.QueueHighWatermark)
	m.addWorkers(m.cfg.InitialWorkerCount)
	go m.scaler()
}

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

func (m *Manager) Enqueue(ev model.Event) bool { return m.q.Enqueue(ev) }

func (m *Manager) BacklogSize() int { return m.q.BacklogSize() }

func (m *Manager) QueueDepth() int { return m.q.QueueDepth() }

func (m *Manager) WorkerCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.workerCancels)
}

func (m *Manager) NextSequence() uint64 { return m.seq.Next() }

func (m *Manager) IsShuttingDown() bool { return m.q.IsShuttingDown() }

func (m *Manager) CloseIntake() { m.q.CloseIntake() }

func (m *Manager) QueueMetrics() (enq, proc uint64, backlog, depth int) {
	return m.q.Metrics()
}

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
