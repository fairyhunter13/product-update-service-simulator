package queue

import "sync/atomic"

type Sequencer struct{ n atomic.Uint64 }

func (s *Sequencer) Next() uint64 { return s.n.Add(1) }
