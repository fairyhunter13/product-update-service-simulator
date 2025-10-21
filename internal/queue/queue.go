package queue

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fairyhunter13/product-update-service-simulator/internal/model"
	"github.com/fairyhunter13/product-update-service-simulator/internal/obs"
)

// Queue is a simple buffered event queue with a background broker.
type Queue struct {
	mu           sync.Mutex
	backlog      []model.Event
	notify       chan struct{}
	out          chan model.Event
	shuttingDown atomic.Bool

	enqueued  atomic.Uint64
	processed atomic.Uint64
}

// New creates a Queue with a buffered output channel.
func New(outBuffer int) *Queue {
	if outBuffer <= 0 {
		outBuffer = 64
	}
	return &Queue{
		notify: make(chan struct{}, 1),
		out:    make(chan model.Event, outBuffer),
	}
}

// Start runs the broker loop.
func (q *Queue) Start(ctx context.Context, highWatermark int) {
	go q.broker(ctx, highWatermark)
}

// broker moves backlog items to the output channel.
func (q *Queue) broker(ctx context.Context, highWatermark int) {
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		q.flushOnce()
		if highWatermark > 0 {
			if sz := q.BacklogSize(); sz > highWatermark {
				obs.Logger.Warn("queue backlog exceeds high watermark", "backlog_size", sz, "high_watermark", highWatermark)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-q.notify:
		case <-ticker.C:
		}
	}
}

// flushOnce drains backlog into the output buffer.
func (q *Queue) flushOnce() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.backlog) > 0 && len(q.out) < cap(q.out) {
		item := q.backlog[0]
		q.backlog = q.backlog[1:]
		q.out <- item
	}
}

// Enqueue appends an event into the backlog and notifies the broker.
func (q *Queue) Enqueue(ev model.Event) bool {
	if q.shuttingDown.Load() {
		return false
	}
	q.enqueued.Add(1)
	q.mu.Lock()
	q.backlog = append(q.backlog, ev)
	q.mu.Unlock()
	select {
	case q.notify <- struct{}{}:
	default:
	}
	return true
}

// Out exposes the output channel of events.
func (q *Queue) Out() <-chan model.Event { return q.out }

// BacklogSize returns the number of enqueued-but-not-yet-output events.
func (q *Queue) BacklogSize() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.backlog)
}

// QueueDepth returns backlog plus buffered output items.
func (q *Queue) QueueDepth() int { // backlog + out buffered items
	q.mu.Lock()
	bl := len(q.backlog)
	q.mu.Unlock()
	return bl + len(q.out)
}

// MarkProcessed increases the processed counter.
func (q *Queue) MarkProcessed() { q.processed.Add(1) }

// Metrics returns counters and sizes for observability.
func (q *Queue) Metrics() (enq, proc uint64, backlog, depth int) {
	enq = q.enqueued.Load()
	proc = q.processed.Load()
	backlog = q.BacklogSize()
	depth = q.QueueDepth()
	return enq, proc, backlog, depth
}

// CloseIntake disallows future enqueues.
func (q *Queue) CloseIntake() { q.shuttingDown.Store(true) }

// IsShuttingDown reports if intake has been closed.
func (q *Queue) IsShuttingDown() bool { return q.shuttingDown.Load() }
