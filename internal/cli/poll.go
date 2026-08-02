package cli

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type pollEvent uint8

const (
	pollEventTick pollEvent = iota
	pollEventInterrupt
)

type pollStep[T any] func(context.Context, pollEvent) (T, bool, error)

func requirePollingContext(ctx context.Context) error {
	if ctx == nil {
		return errors.New("cli: polling context is required")
	}
	return nil
}

func pollUntil[T any](
	ctx context.Context,
	timeout time.Duration,
	interval time.Duration,
	interrupt <-chan struct{},
	timeoutMessage string,
	step pollStep[T],
) (T, error) {
	var zero T
	if err := requirePollingContext(ctx); err != nil {
		return zero, err
	}
	if interval <= 0 {
		return zero, errors.New("cli: polling interval must be greater than zero")
	}
	if step == nil {
		return zero, errors.New("cli: polling step is required")
	}

	pollCtx := ctx
	cancel := func() {}
	if _, hasDeadline := pollCtx.Deadline(); !hasDeadline {
		if timeout <= 0 {
			return zero, errors.New("cli: polling timeout must be greater than zero")
		}
		pollCtx, cancel = context.WithTimeout(pollCtx, timeout)
	}
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		if err := pollCtx.Err(); err != nil {
			return zero, fmt.Errorf("%s: %w", timeoutMessage, err)
		}
		select {
		case <-pollCtx.Done():
			return zero, fmt.Errorf("%s: %w", timeoutMessage, pollCtx.Err())
		case <-interrupt:
			interrupt = nil
			value, done, err := step(pollCtx, pollEventInterrupt)
			if err != nil || done {
				return value, err
			}
		case <-ticker.C:
			value, done, err := step(pollCtx, pollEventTick)
			if err != nil || done {
				return value, err
			}
		}
	}
}
