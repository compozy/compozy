package sessiondb

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

func (p *ReadOnlyPool) startCloseLocked(
	ctx context.Context,
	key readOnlyPoolKey,
	recorder store.EventReadCloser,
	op string,
) *readOnlyPoolClose {
	closeOp := &readOnlyPoolClose{
		key:      key,
		recorder: recorder,
		op:       op,
		runCtx:   ctx,
		ready:    make(chan struct{}),
	}
	if p.closings[key] == nil {
		p.closings[key] = make(map[*readOnlyPoolClose]struct{})
	}
	p.closings[key][closeOp] = struct{}{}
	p.attachCloseToQuiescenceLocked(closeOp)
	return closeOp
}

func (p *ReadOnlyPool) startEntryCloseLocked(
	ctx context.Context,
	key readOnlyPoolKey,
	entry *readOnlyPoolEntry,
	op string,
) *readOnlyPoolClose {
	closeOp := p.startCloseLocked(ctx, key, entry.recorder, op)
	p.attachEntryCloseLocked(entry, closeOp)
	return closeOp
}

func (p *ReadOnlyPool) runClose(closeOp *readOnlyPoolClose) {
	if !p.claimCloseExecution(closeOp) {
		return
	}
	closeCtx := context.WithValue(closeOp.runCtx, readOnlyPoolCloseContextKey{}, readOnlyPoolCloseExecution{
		pool:      p,
		operation: closeOp,
	})
	var err error
	if closeErr := closeOp.recorder.Close(closeCtx); closeErr != nil {
		err = fmt.Errorf("store: %s: %w", closeOp.op, closeErr)
	}

	p.mu.Lock()
	closeOp.err = err
	p.recordCloseTerminalLocked(closeOp, err)
	p.completeDeferredQuiescenceLocked(closeOp, err)
	if byKey := p.closings[closeOp.key]; byKey != nil {
		delete(byKey, closeOp)
		if len(byKey) == 0 {
			delete(p.closings, closeOp.key)
		}
	}
	close(closeOp.ready)
	p.mu.Unlock()
}

func (p *ReadOnlyPool) claimCloseExecution(closeOp *readOnlyPoolClose) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if closeOp.started {
		return false
	}
	closeOp.started = true
	return true
}

func (p *ReadOnlyPool) runCloseAll(closeOps []*readOnlyPoolClose) {
	for _, closeOp := range closeOps {
		p.runClose(closeOp)
	}
}

func (p *ReadOnlyPool) closeOperationsForKeyLocked(key readOnlyPoolKey) []*readOnlyPoolClose {
	byKey := p.closings[key]
	closeOps := make([]*readOnlyPoolClose, 0, len(byKey))
	for closeOp := range byKey {
		closeOps = append(closeOps, closeOp)
	}
	return closeOps
}

func (p *ReadOnlyPool) allCloseOperationsLocked() []*readOnlyPoolClose {
	var closeOps []*readOnlyPoolClose
	for _, byKey := range p.closings {
		for closeOp := range byKey {
			closeOps = append(closeOps, closeOp)
		}
	}
	return closeOps
}

func (p *ReadOnlyPool) waitForCloseOperations(ctx context.Context, closeOps []*readOnlyPoolClose) error {
	executing := p.executingCloseOperation(ctx)
	var closeErr error
	for _, closeOp := range closeOps {
		if err := p.waitForCloseOperation(ctx, executing, closeOp); err != nil {
			closeErr = errors.Join(closeErr, err)
			if ctx.Err() != nil || errors.Is(err, errReadOnlyPoolCloseDependencyCycle) {
				return closeErr
			}
		}
	}
	return closeErr
}

func (p *ReadOnlyPool) waitForCloseOperation(
	ctx context.Context,
	executing *readOnlyPoolClose,
	closeOp *readOnlyPoolClose,
) error {
	shouldWait, shouldRun, dependencyErr := p.beginCloseDependency(executing, closeOp)
	if dependencyErr != nil {
		return dependencyErr
	}
	if !shouldWait {
		return nil
	}
	if executing != nil {
		defer p.endCloseDependency(executing, closeOp)
	}
	if shouldRun {
		p.runClose(closeOp)
	}
	select {
	case <-closeOp.ready:
		return closeOp.err
	default:
	}
	select {
	case <-closeOp.ready:
		return closeOp.err
	case <-ctx.Done():
		return fmt.Errorf("store: wait for read-only pool recorder close: %w", ctx.Err())
	}
}

func (p *ReadOnlyPool) executingCloseOperation(ctx context.Context) *readOnlyPoolClose {
	execution, ok := ctx.Value(readOnlyPoolCloseContextKey{}).(readOnlyPoolCloseExecution)
	if !ok || execution.pool != p {
		return nil
	}
	return execution.operation
}
