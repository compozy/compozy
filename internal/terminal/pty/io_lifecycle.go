package pty

import (
	"errors"
	"sync"
)

var errIOClosed = errors.New("terminal pty: input/output lifecycle is closed")

type ioLifecycle struct {
	mu     sync.Mutex
	active sync.WaitGroup
	closed bool
}

func (l *ioLifecycle) begin() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return errIOClosed
	}
	l.active.Add(1)
	return nil
}

func (l *ioLifecycle) end() {
	l.active.Done()
}

func (l *ioLifecycle) seal() {
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()
}

func (l *ioLifecycle) wait() {
	l.active.Wait()
}
