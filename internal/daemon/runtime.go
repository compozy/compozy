package daemon

import (
	"context"
	"errors"
	"sync"
	"time"
)

// RunMode controls daemon signal and logging ownership.
type RunMode string

const (
	RunModeForeground RunMode = "foreground"
	RunModeDetached   RunMode = "detached"
)

func resolveRunMode(mode RunMode) RunMode {
	switch mode {
	case RunModeDetached:
		return RunModeDetached
	default:
		return RunModeForeground
	}
}

func daemonRunSignalContext(ctx context.Context, mode RunMode) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, errors.New("daemon: run context is required")
	}
	if resolveRunMode(mode) != RunModeDetached {
		return ctx, func() {}, nil
	}
	return notifyDetachedDaemonContext(context.WithoutCancel(ctx))
}

func boundedLifecycleContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	base := ctx
	if base == nil {
		base = context.Background()
	}
	valueCtx := context.WithoutCancel(base)

	targetDeadline := time.Time{}
	if timeout > 0 {
		if deadline, ok := base.Deadline(); ok {
			targetDeadline = time.Now().Add(timeout)
			if deadline.Before(targetDeadline) {
				targetDeadline = deadline
			}
		} else {
			targetDeadline = time.Now().Add(timeout)
		}
	} else if deadline, ok := base.Deadline(); ok {
		targetDeadline = deadline
	}

	if targetDeadline.IsZero() {
		return valueCtx, func() {}
	}
	return newBoundedDeadlineContext(valueCtx, targetDeadline)
}

type boundedDeadlineContext struct {
	context.Context
	deadline time.Time
	done     chan struct{}

	mu  sync.RWMutex
	err error
}

func newBoundedDeadlineContext(base context.Context, deadline time.Time) (context.Context, context.CancelFunc) {
	ctx := &boundedDeadlineContext{
		Context:  base,
		deadline: deadline,
		done:     make(chan struct{}),
	}

	delay := time.Until(deadline)
	if delay <= 0 {
		ctx.expire()
		return ctx, func() {}
	}

	timer := time.AfterFunc(delay, ctx.expire)
	return ctx, func() {
		timer.Stop()
	}
}

func (c *boundedDeadlineContext) Deadline() (time.Time, bool) {
	if c == nil || c.deadline.IsZero() {
		return time.Time{}, false
	}
	return c.deadline, true
}

func (c *boundedDeadlineContext) Done() <-chan struct{} {
	if c == nil {
		return nil
	}
	return c.done
}

func (c *boundedDeadlineContext) Err() error {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.err
}

func (c *boundedDeadlineContext) expire() {
	if c == nil {
		return
	}
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return
	}
	c.err = context.DeadlineExceeded
	close(c.done)
	c.mu.Unlock()
}
