package pty

import (
	"context"
	"errors"
	"os/exec"
	"sync"
)

type waitResult struct {
	exit Exit
	err  error
}

type processWaiter struct {
	once   sync.Once
	done   chan struct{}
	mu     sync.RWMutex
	result waitResult
}

func newProcessWaiter() processWaiter {
	return processWaiter{done: make(chan struct{})}
}

func (w *processWaiter) start(wait func() waitResult) {
	w.once.Do(func() {
		go func() {
			result := wait()
			w.mu.Lock()
			w.result = result
			w.mu.Unlock()
			close(w.done)
		}()
	})
}

func (w *processWaiter) wait(ctx context.Context) (Exit, error) {
	select {
	case <-ctx.Done():
		return Exit{}, ctx.Err()
	case <-w.done:
		w.mu.RLock()
		defer w.mu.RUnlock()
		return w.result.exit, w.result.err
	}
}

func classifyExit(err error, command *exec.Cmd) Exit {
	if command == nil || command.ProcessState == nil {
		return Exit{Cause: "unknown"}
	}
	if signal := processSignal(command.ProcessState); signal != "" {
		return Exit{Cause: "signaled", Signal: &signal}
	}
	if err == nil || errors.As(err, new(*exec.ExitError)) {
		code := command.ProcessState.ExitCode()
		return Exit{Cause: "exited", Code: &code}
	}
	return Exit{Cause: "unknown"}
}
