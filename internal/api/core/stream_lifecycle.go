package core

import (
	"context"
	"fmt"
	"sync"
)

type streamLifecycle struct {
	mu        sync.Mutex
	stop      chan struct{}
	drained   chan struct{}
	active    int
	accepting bool
	label     string
}

func newStreamLifecycle(label string) *streamLifecycle {
	lifecycle := &streamLifecycle{label: label}
	lifecycle.reset()
	return lifecycle
}

func (l *streamLifecycle) reset() {
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
		l.drained = closedChannel()
	}
	l.mu.Unlock()
}

func (l *streamLifecycle) begin() (<-chan struct{}, func(), bool) {
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
	return stop, sync.OnceFunc(l.end), true
}

func (l *streamLifecycle) end() {
	l.mu.Lock()
	if l.active > 0 {
		l.active--
		if l.active == 0 {
			close(l.drained)
		}
	}
	l.mu.Unlock()
}

func (l *streamLifecycle) shutdown(ctx context.Context) error {
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
		return fmt.Errorf("api: wait for %s streams: %w", l.label, ctx.Err())
	}
}

func closedChannel() chan struct{} {
	channel := make(chan struct{})
	close(channel)
	return channel
}
