package core

import (
	"context"
	"fmt"
	"sync"
)

type terminalStreamLifecycle struct {
	mu        sync.Mutex
	stop      chan struct{}
	drained   chan struct{}
	active    int
	accepting bool
}

func newTerminalStreamLifecycle() *terminalStreamLifecycle {
	lifecycle := &terminalStreamLifecycle{}
	lifecycle.reset()
	return lifecycle
}

func (l *terminalStreamLifecycle) reset() {
	l.mu.Lock()
	if l.stop != nil && l.accepting {
		close(l.stop)
	}
	l.stop = make(chan struct{})
	l.accepting = true
	if l.active == 0 {
		l.drained = closedWindowManagerChannel()
	}
	l.mu.Unlock()
}

func (l *terminalStreamLifecycle) begin() (<-chan struct{}, func(), bool) {
	l.mu.Lock()
	if !l.accepting {
		l.mu.Unlock()
		return nil, nil, false
	}
	if l.active == 0 {
		l.drained = make(chan struct{})
	}
	l.active++
	stop := l.stop
	l.mu.Unlock()
	return stop, sync.OnceFunc(l.end), true
}

func (l *terminalStreamLifecycle) end() {
	l.mu.Lock()
	l.active--
	if l.active == 0 {
		close(l.drained)
	}
	l.mu.Unlock()
}

func (l *terminalStreamLifecycle) shutdown(ctx context.Context) error {
	l.mu.Lock()
	if l.accepting {
		l.accepting = false
		close(l.stop)
	}
	drained := l.drained
	l.mu.Unlock()
	select {
	case <-drained:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("api: wait for terminal streams: %w", ctx.Err())
	}
}
