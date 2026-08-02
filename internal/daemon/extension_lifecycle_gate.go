package daemon

import (
	"context"
	"errors"
	"sync"
)

// extensionLifecycleGate is a context-aware reader/writer gate. Named
// mutations hold a read lease; install flows with an unknown name hold the
// exclusive write lease.
type extensionLifecycleGate struct {
	mu             sync.Mutex
	changed        chan struct{}
	readers        int
	writer         bool
	waitingWriters int
}

func newExtensionLifecycleGate() *extensionLifecycleGate {
	return &extensionLifecycleGate{changed: make(chan struct{})}
}

func (g *extensionLifecycleGate) acquireRead(ctx context.Context) error {
	if g == nil {
		return errors.New("extension lifecycle gate is required")
	}
	for {
		g.mu.Lock()
		if !g.writer && g.waitingWriters == 0 {
			g.readers++
			g.mu.Unlock()
			return nil
		}
		changed := g.changed
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

func (g *extensionLifecycleGate) releaseRead() {
	g.mu.Lock()
	g.readers--
	g.signalLocked()
	g.mu.Unlock()
}

func (g *extensionLifecycleGate) acquireWrite(ctx context.Context) error {
	if g == nil {
		return errors.New("extension lifecycle gate is required")
	}
	g.mu.Lock()
	g.waitingWriters++
	for {
		if !g.writer && g.readers == 0 {
			g.waitingWriters--
			g.writer = true
			g.mu.Unlock()
			return nil
		}
		changed := g.changed
		g.mu.Unlock()
		select {
		case <-ctx.Done():
			g.mu.Lock()
			g.waitingWriters--
			g.signalLocked()
			g.mu.Unlock()
			return ctx.Err()
		case <-changed:
			g.mu.Lock()
		}
	}
}

func (g *extensionLifecycleGate) releaseWrite() {
	g.mu.Lock()
	g.writer = false
	g.signalLocked()
	g.mu.Unlock()
}

func (g *extensionLifecycleGate) signalLocked() {
	close(g.changed)
	g.changed = make(chan struct{})
}
