package core

import (
	"context"
	"fmt"
	"sync"
)

type windowManagerStreamLifecycle struct {
	mu        sync.Mutex
	stop      chan struct{}
	drained   chan struct{}
	active    int
	accepting bool
}

func newWindowManagerStreamLifecycle() *windowManagerStreamLifecycle {
	lifecycle := &windowManagerStreamLifecycle{}
	lifecycle.reset()
	return lifecycle
}

func (l *windowManagerStreamLifecycle) reset() {
	if l == nil {
		return
	}
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

func (l *windowManagerStreamLifecycle) begin() (<-chan struct{}, func(), bool) {
	if l == nil {
		return nil, nil, false
	}
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
	var once sync.Once
	return stop, func() {
		once.Do(func() { l.end() })
	}, true
}

func (l *windowManagerStreamLifecycle) end() {
	l.mu.Lock()
	if l.active > 0 {
		l.active--
		if l.active == 0 {
			close(l.drained)
		}
	}
	l.mu.Unlock()
}

func (l *windowManagerStreamLifecycle) shutdown(ctx context.Context) error {
	if l == nil {
		return nil
	}
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
		return fmt.Errorf("api: wait for window-manager streams: %w", ctx.Err())
	}
}

func closedWindowManagerChannel() chan struct{} {
	channel := make(chan struct{})
	close(channel)
	return channel
}
