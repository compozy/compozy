package sessiondb

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

// Close closes every pooled handle and rejects future opens.
func (p *ReadOnlyPool) Close(ctx context.Context) error {
	if p == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("store: close read-only pool context is required")
	}

	p.mu.Lock()
	if p.closed {
		shutdown := p.shutdown
		p.mu.Unlock()
		return p.waitForShutdown(ctx, shutdown)
	}
	p.closed = true
	for _, state := range p.quiescing {
		p.completeQuiescenceLocked(state, errReadOnlyPoolClosed)
	}
	newCloseOps := p.detachAllEntriesLocked(ctx)
	shutdown := &readOnlyPoolShutdown{
		ready:    make(chan struct{}),
		closings: p.allCloseOperationsLocked(),
		openings: p.allOpeningsLocked(),
	}
	p.shutdown = shutdown
	p.mu.Unlock()

	p.runCloseAll(newCloseOps)
	return p.waitForShutdown(ctx, shutdown)
}

func (p *ReadOnlyPool) detachAllEntriesLocked(ctx context.Context) []*readOnlyPoolClose {
	var closeOps []*readOnlyPoolClose
	for key, entry := range p.entries {
		delete(p.entries, key)
		if entry == nil {
			continue
		}
		if entry.idle != nil {
			close(entry.idle)
			entry.idle = nil
		}
		if entry.recorder == nil {
			p.completeEntryResponsibilityLocked(key, entry, nil)
			continue
		}
		closeOps = append(
			closeOps,
			p.startEntryCloseLocked(ctx, key, entry, "close pooled read-only recorder"),
		)
	}
	return closeOps
}

func (p *ReadOnlyPool) allOpeningsLocked() []*readOnlyPoolOpening {
	openings := make([]*readOnlyPoolOpening, 0, len(p.openings))
	for _, opening := range p.openings {
		openings = append(openings, opening)
	}
	return openings
}

func (p *ReadOnlyPool) waitForShutdown(ctx context.Context, shutdown *readOnlyPoolShutdown) error {
	if shutdown == nil {
		return errReadOnlyPoolClosed
	}

	p.mu.Lock()
	if shutdown.completed {
		err := shutdown.err
		p.mu.Unlock()
		return err
	}
	p.mu.Unlock()

	executing := p.executingCloseOperation(ctx)
	selfDependent := executing != nil && slices.Contains(shutdown.closings, executing)
	var closeErr error
	if err := p.waitForCloseOperations(ctx, shutdown.closings); err != nil {
		closeErr = errors.Join(closeErr, err)
	}
	for _, opening := range shutdown.openings {
		if executing != nil && opening == executing.opening {
			selfDependent = true
			continue
		}
		shouldWait, dependencyErr := p.beginOpeningDependency(executing, opening)
		if dependencyErr != nil {
			return errors.Join(closeErr, dependencyErr)
		}
		if !shouldWait {
			selfDependent = true
			continue
		}
		waitErr := p.waitForShutdownOpening(ctx, opening)
		if executing != nil {
			p.endOpeningDependency(executing, opening)
		}
		if waitErr != nil {
			return errors.Join(closeErr, waitErr)
		}
		closeErr = errors.Join(closeErr, opening.closeErr)
	}
	if selfDependent {
		// The caller is inside this recorder's Close callback. It cannot
		// complete the shared shutdown result until that callback returns and
		// publishes its own close result (and possibly its owning opener), but
		// it still joins every other operation in the shutdown snapshot.
		return closeErr
	}

	p.mu.Lock()
	if !shutdown.completed {
		shutdown.err = closeErr
		shutdown.completed = true
		close(shutdown.ready)
	}
	err := shutdown.err
	p.mu.Unlock()
	return err
}

func (p *ReadOnlyPool) waitForShutdownOpening(
	ctx context.Context,
	opening *readOnlyPoolOpening,
) error {
	select {
	case <-opening.ready:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("store: wait for read-only pool opener shutdown: %w", ctx.Err())
	}
}
