package globaldb

import (
	"sync"
	"sync/atomic"
)

// outboxSignalKind identifies which durable outbox a transaction wrote.
type outboxSignalKind uint32

const (
	outboxSignalLoopEffect outboxSignalKind = 1 << iota
	outboxSignalGoalSession
)

// outboxSignalCollector is implemented by write-transaction executors that
// report committed outbox writes after COMMIT.
type outboxSignalCollector interface {
	markOutboxSignal(kind outboxSignalKind)
}

// markOutboxSignal records that the active transaction enqueued outbox work.
// Executors that do not collect signals are ignored; the relay fallback sweep
// still delivers those rows.
func markOutboxSignal(exec any, kind outboxSignalKind) {
	collector, ok := exec.(outboxSignalCollector)
	if !ok {
		return
	}
	collector.markOutboxSignal(kind)
}

// outboxSignalSet accumulates signal kinds within one transaction attempt.
type outboxSignalSet struct {
	kinds atomic.Uint32
}

func (s *outboxSignalSet) markOutboxSignal(kind outboxSignalKind) {
	s.kinds.Or(uint32(kind))
}

func (s *outboxSignalSet) load() outboxSignalKind {
	return outboxSignalKind(s.kinds.Load())
}

type outboxSignalExecutor struct {
	globalSQLExecutor
	outboxSignalSet
}

// outboxSignalHub owns the post-commit wake callbacks shared by all repositories.
type outboxSignalHub struct {
	mu          sync.RWMutex
	loopEffect  func()
	goalSession func()
}

// SetLoopEffectCommitSignal installs the callback invoked after a transaction
// that enqueued Loop effect deliveries commits. Nil clears it.
func (g *GlobalDB) SetLoopEffectCommitSignal(signal func()) {
	if g == nil || g.LoopRepo == nil || g.LoopRepo.repoBase == nil {
		return
	}
	hub := g.LoopRepo.outboxSignals
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.loopEffect = signal
}

// SetGoalSessionOutboxCommitSignal installs the callback invoked after a
// transaction that enqueued Goal session projections or Loop session cleanup
// obligations commits. Nil clears it.
func (g *GlobalDB) SetGoalSessionOutboxCommitSignal(signal func()) {
	if g == nil || g.LoopRepo == nil || g.LoopRepo.repoBase == nil {
		return
	}
	hub := g.LoopRepo.outboxSignals
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.goalSession = signal
}

func (h *outboxSignalHub) fire(kinds outboxSignalKind) {
	if h == nil || kinds == 0 {
		return
	}
	h.mu.RLock()
	loopEffect, goalSession := h.loopEffect, h.goalSession
	h.mu.RUnlock()
	if kinds&outboxSignalLoopEffect != 0 && loopEffect != nil {
		loopEffect()
	}
	if kinds&outboxSignalGoalSession != 0 && goalSession != nil {
		goalSession()
	}
}
