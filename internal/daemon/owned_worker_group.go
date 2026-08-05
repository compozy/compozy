package daemon

import (
	"context"
	"sync"
)

// ownedWorkerGroup orders work admission before shutdown starts waiting.
// Begin reserves one operation; the returned completion function is idempotent.
type ownedWorkerGroup struct {
	mu       sync.Mutex
	stopping bool
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	done     chan struct{}
}

func newOwnedWorkerGroup(cancel context.CancelFunc) *ownedWorkerGroup {
	return &ownedWorkerGroup{
		cancel: cancel,
		done:   make(chan struct{}),
	}
}

func (g *ownedWorkerGroup) Begin() (complete func(), ok bool) {
	if g == nil {
		return nil, false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopping {
		return nil, false
	}
	g.wg.Add(1)
	return sync.OnceFunc(g.wg.Done), true
}

func (g *ownedWorkerGroup) Stop() <-chan struct{} {
	if g == nil {
		done := make(chan struct{})
		close(done)
		return done
	}
	g.mu.Lock()
	if g.stopping {
		done := g.done
		g.mu.Unlock()
		return done
	}
	g.stopping = true
	done := g.done
	cancel := g.cancel
	g.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	go func() {
		g.wg.Wait()
		close(done)
	}()
	return done
}
