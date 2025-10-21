package queue

import "sync/atomic"

// Sequencer provides monotonically increasing sequence numbers.
type Sequencer struct{ n atomic.Uint64 }

// Next returns the next sequence number.
func (s *Sequencer) Next() uint64 { return s.n.Add(1) }
