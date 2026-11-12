package store

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// WriteAttribution names the turn a write counts toward and whether the write is a shared coalesced batch. It rides
// on the write's context as a value only; it never changes which context executes the write.
type WriteAttribution struct {
	Turn      *TurnWriteAccount // nil outside a turn
	Coalesced bool
}

type writeAttributionKey struct{}

// WithWriteAttribution attaches attribution to a context the caller owns.
func WithWriteAttribution(ctx context.Context, attribution WriteAttribution) context.Context {
	return context.WithValue(ctx, writeAttributionKey{}, attribution)
}

// WriteAttributionFrom returns the attribution attached to ctx.
func WriteAttributionFrom(ctx context.Context) (WriteAttribution, bool) {
	attribution, ok := ctx.Value(writeAttributionKey{}).(WriteAttribution)
	return attribution, ok
}

// TurnWriteAccount accumulates one turn's executed writes; safe for concurrent use. A nil account ignores Add.
type TurnWriteAccount struct {
	mu      sync.Mutex
	flushes int64
	total   time.Duration
	closed  bool
}

// NewTurnWriteAccount returns an open account.
func NewTurnWriteAccount() *TurnWriteAccount {
	return &TurnWriteAccount{}
}

// Add counts one write of duration d; it is a no-op after Close.
func (a *TurnWriteAccount) Add(d time.Duration) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return
	}
	a.flushes++
	a.total += d
}

// Close stops accounting and returns the counted writes and their summed duration.
func (a *TurnWriteAccount) Close() (flushes int64, total time.Duration) {
	if a == nil {
		return 0, 0
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.closed = true
	return a.flushes, a.total
}

// WriteObservation describes one finished ExecuteWrite or ExecuteWriteOperation call. Duration spans admission,
// every retry, and commit; AdmissionWait is the part spent waiting for write admission.
type WriteObservation struct {
	Operation     string
	Coalesced     bool
	Started       time.Time
	AdmissionWait time.Duration
	Duration      time.Duration
	Err           error
}

// WriteObserver receives every finished write with the write caller's context. It runs on the writing goroutine and
// must not block. internal/store cannot import telemetry, so the composition root supplies it.
type WriteObserver func(context.Context, WriteObservation)

var writeObserver atomic.Pointer[WriteObserver]

// SetWriteObserver installs the process-wide write observer; nil removes it.
func SetWriteObserver(observer WriteObserver) {
	if observer == nil {
		writeObserver.Store(nil)
		return
	}
	writeObserver.Store(&observer)
}

// observeWrite updates the turn account of a write whose transaction began and reports the write to the observer.
func observeWrite(ctx context.Context, operation string, started, admitted time.Time, began bool, err error) {
	duration := time.Since(started)
	attribution, _ := WriteAttributionFrom(ctx)
	wait := duration
	if !admitted.IsZero() {
		wait = admitted.Sub(started)
	}
	if began {
		attribution.Turn.Add(duration)
	}
	observer := writeObserver.Load()
	if observer == nil {
		return
	}
	(*observer)(ctx, WriteObservation{
		Operation:     operation,
		Coalesced:     attribution.Coalesced,
		Started:       started,
		AdmissionWait: wait,
		Duration:      duration,
		Err:           err,
	})
}
