package daemon

import (
	"context"
	"errors"
	"sync"
)

// lifecycleRWGate is a context-aware, writer-preferring reader/writer gate.
type lifecycleRWGate struct {
	mu             sync.Mutex
	changed        chan struct{}
	readers        int
	writer         bool
	waitingWriters int
}

func (g *lifecycleRWGate) lock(ctx context.Context, shared bool) (func(), error) {
	if g == nil {
		return nil, errors.New("daemon: lifecycle gate is required")
	}
	if ctx == nil {
		return nil, errors.New("daemon: lifecycle gate context is required")
	}

	waitingWriter := false
	for {
		if err := ctx.Err(); err != nil {
			g.cancelWriterWait(waitingWriter)
			return nil, err
		}

		g.mu.Lock()
		unlock, acquired := g.tryAcquireLocked(shared, &waitingWriter)
		if acquired {
			g.mu.Unlock()
			return unlock, nil
		}
		changed := g.changed
		g.mu.Unlock()

		select {
		case <-ctx.Done():
			g.cancelWriterWait(waitingWriter)
			return nil, ctx.Err()
		case <-changed:
		}
	}
}

func (g *lifecycleRWGate) lockUncancelable(shared bool) func() {
	waitingWriter := false
	for {
		g.mu.Lock()
		unlock, acquired := g.tryAcquireLocked(shared, &waitingWriter)
		if acquired {
			g.mu.Unlock()
			return unlock
		}
		changed := g.changed
		g.mu.Unlock()
		<-changed
	}
}

func (g *lifecycleRWGate) tryAcquireLocked(shared bool, waitingWriter *bool) (func(), bool) {
	g.ensureChangedLocked()
	if shared {
		if g.writer || g.waitingWriters > 0 {
			return nil, false
		}
		g.readers++
		return g.unlockShared, true
	}

	if !*waitingWriter {
		g.waitingWriters++
		*waitingWriter = true
		g.notifyLocked()
	}
	if g.writer || g.readers > 0 {
		return nil, false
	}
	g.waitingWriters--
	*waitingWriter = false
	g.writer = true
	return g.unlockExclusive, true
}

func (g *lifecycleRWGate) cancelWriterWait(waiting bool) {
	if !waiting {
		return
	}
	g.mu.Lock()
	g.waitingWriters--
	g.notifyLocked()
	g.mu.Unlock()
}

func (g *lifecycleRWGate) unlockShared() {
	g.mu.Lock()
	g.readers--
	g.notifyLocked()
	g.mu.Unlock()
}

func (g *lifecycleRWGate) unlockExclusive() {
	g.mu.Lock()
	g.writer = false
	g.notifyLocked()
	g.mu.Unlock()
}

func (g *lifecycleRWGate) ensureChangedLocked() {
	if g.changed == nil {
		g.changed = make(chan struct{})
	}
}

func (g *lifecycleRWGate) notifyLocked() {
	g.ensureChangedLocked()
	close(g.changed)
	g.changed = make(chan struct{})
}
