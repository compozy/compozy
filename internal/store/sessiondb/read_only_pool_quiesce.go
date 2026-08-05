package sessiondb

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/compozy/compozy/internal/store"
)

// Quiesce prevents new leases for one session, waits for active leases to
// return, and closes the pooled database handle. The returned function must be
// called after the caller's destructive operation finishes so failed
// operations can resume read access.
func (p *ReadOnlyPool) Quiesce(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) (func(), error) {
	if p == nil {
		return nil, errors.New("store: read-only pool is required")
	}
	if ctx == nil {
		return nil, errors.New("store: quiesce read-only pool context is required")
	}
	key, err := normalizeReadOnlyPoolKey(owner, path)
	if err != nil {
		return nil, err
	}
	start, joined, err := p.beginQuiescence(key)
	if err != nil {
		return nil, err
	}
	if joined {
		return p.waitForQuiescence(ctx, key, start.state)
	}
	start.deferred = p.deferQuiescenceToExecutingClose(ctx, key, start.state)
	if start.opening != nil {
		if err := p.waitForOpeningClose(ctx, start.opening); err != nil {
			return p.failQuiescence(key, start.state, err)
		}
	}
	if start.idle != nil {
		select {
		case <-start.idle:
		case <-ctx.Done():
			return p.failQuiescence(
				key,
				start.state,
				fmt.Errorf("store: quiesce read-only pool session %q: %w", key.owner.SessionID, ctx.Err()),
			)
		}
	}
	return p.closeQuiescedEntry(ctx, key, start)
}

func (p *ReadOnlyPool) beginQuiescence(
	key readOnlyPoolKey,
) (readOnlyPoolQuiescenceStart, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return readOnlyPoolQuiescenceStart{}, false, errReadOnlyPoolClosed
	}
	if state := p.quiescing[key]; state != nil {
		state.owners++
		return readOnlyPoolQuiescenceStart{state: state}, true, nil
	}

	state := &readOnlyPoolQuiescence{
		ready:    make(chan struct{}),
		owners:   1,
		closings: make(map[*readOnlyPoolClose]struct{}),
		openings: make(map[*readOnlyPoolOpening]struct{}),
		entries:  make(map[*readOnlyPoolEntry]*readOnlyPoolQuiescenceResponsibility),
	}
	p.quiescing[key] = state
	entry := p.entries[key]
	opening := p.openings[key]
	p.addEntryResponsibilityLocked(state, entry)
	for closeOp := range p.closings[key] {
		state.closings[closeOp] = struct{}{}
	}
	if opening != nil {
		state.openings[opening] = struct{}{}
	}
	start := readOnlyPoolQuiescenceStart{
		state:   state,
		entry:   entry,
		opening: opening,
	}
	if entry != nil && entry.refs > 0 {
		if entry.idle == nil {
			entry.idle = make(chan struct{})
		}
		start.idle = entry.idle
	}
	return start, false, nil
}

func (p *ReadOnlyPool) closeQuiescedEntry(
	ctx context.Context,
	key readOnlyPoolKey,
	start readOnlyPoolQuiescenceStart,
) (func(), error) {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return p.waitForQuiescence(ctx, key, start.state)
	}
	existingCloseOps := p.closeOperationsForKeyLocked(key)
	for _, closeOp := range existingCloseOps {
		start.state.closings[closeOp] = struct{}{}
	}
	var ownedCloseOps []*readOnlyPoolClose
	if current := p.entries[key]; current != nil && current == start.entry && current.refs == 0 {
		delete(p.entries, key)
		if current.recorder != nil {
			closeOp := p.startEntryCloseLocked(
				ctx,
				key,
				current,
				fmt.Sprintf("close quiesced read-only recorder for session %q", key.owner.SessionID),
			)
			ownedCloseOps = append(ownedCloseOps, closeOp)
		} else {
			p.completeEntryResponsibilityLocked(key, current, nil)
		}
	}
	p.mu.Unlock()

	p.runCloseAll(ownedCloseOps)
	closeErr := p.waitForCloseOperations(ctx, ownedCloseOps)
	if waitErr := p.waitForCloseOperations(ctx, existingCloseOps); waitErr != nil {
		closeErr = errors.Join(closeErr, waitErr)
	}
	if closeErr != nil || !start.deferred {
		p.completeQuiescence(start.state, closeErr)
	}
	return p.waitForQuiescence(ctx, key, start.state)
}

func (p *ReadOnlyPool) waitForOpeningClose(ctx context.Context, opening *readOnlyPoolOpening) error {
	executing := p.executingCloseOperation(ctx)
	if executing != nil && executing.opening == opening {
		return nil
	}
	shouldWait, dependencyErr := p.beginOpeningDependency(executing, opening)
	if dependencyErr != nil {
		return dependencyErr
	}
	if !shouldWait {
		return nil
	}
	if executing != nil {
		defer p.endOpeningDependency(executing, opening)
	}
	select {
	case <-opening.ready:
		return opening.closeErr
	case <-ctx.Done():
		return fmt.Errorf("store: wait for read-only pool opener: %w", ctx.Err())
	}
}

func (p *ReadOnlyPool) failQuiescence(
	key readOnlyPoolKey,
	state *readOnlyPoolQuiescence,
	err error,
) (func(), error) {
	p.completeQuiescence(state, err)
	p.releaseQuiescence(key, state)
	return nil, err
}

func (p *ReadOnlyPool) waitForQuiescence(
	ctx context.Context,
	key readOnlyPoolKey,
	state *readOnlyPoolQuiescence,
) (func(), error) {
	executing := p.executingCloseOperation(ctx)
	dependency, dependencyErr := p.beginQuiescenceDependency(executing, state)
	if dependencyErr != nil {
		p.releaseQuiescence(key, state)
		return nil, dependencyErr
	}
	if dependency.registered {
		defer p.endQuiescenceDependency(executing, state)
	}
	if dependency.selfDependent {
		if err := p.waitForQuiescencePeers(ctx, dependency); err != nil {
			p.releaseQuiescence(key, state)
			return nil, err
		}
		p.mu.Lock()
		stateErr := state.err
		p.mu.Unlock()
		if stateErr != nil {
			p.releaseQuiescence(key, state)
			return nil, stateErr
		}
		return p.newQuiescenceRelease(key, state), nil
	}
	select {
	case <-state.ready:
	case <-ctx.Done():
		p.releaseQuiescence(key, state)
		return nil, fmt.Errorf("store: wait for read-only pool quiescence for %q: %w", key.owner.SessionID, ctx.Err())
	}

	p.mu.Lock()
	err := state.err
	p.mu.Unlock()
	if err != nil {
		p.releaseQuiescence(key, state)
		return nil, err
	}
	return p.newQuiescenceRelease(key, state), nil
}

func (p *ReadOnlyPool) waitForQuiescencePeers(
	ctx context.Context,
	dependency readOnlyPoolQuiescenceDependency,
) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("store: wait for read-only pool quiescence peers: %w", err)
	}
	peerErr := p.waitForCloseOperations(ctx, dependency.closings)
	if ctx.Err() != nil || errors.Is(peerErr, errReadOnlyPoolCloseDependencyCycle) {
		return peerErr
	}
	for _, responsibility := range dependency.responsibilities {
		if err := p.waitForResponsibility(ctx, dependency.state, responsibility); err != nil {
			peerErr = errors.Join(peerErr, err)
			if ctx.Err() != nil || errors.Is(err, errReadOnlyPoolCloseDependencyCycle) {
				return peerErr
			}
		}
	}
	for _, opening := range dependency.openings {
		if err := p.waitForOpeningClose(ctx, opening); err != nil {
			peerErr = errors.Join(peerErr, err)
			if ctx.Err() != nil || errors.Is(err, errReadOnlyPoolCloseDependencyCycle) {
				return peerErr
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return errors.Join(peerErr, fmt.Errorf("store: wait for read-only pool quiescence peers: %w", err))
	}
	return peerErr
}

func (p *ReadOnlyPool) deferQuiescenceToExecutingClose(
	ctx context.Context,
	key readOnlyPoolKey,
	state *readOnlyPoolQuiescence,
) bool {
	executing := p.executingCloseOperation(ctx)
	if executing == nil || executing.key != key {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if executing.quiescence == state {
		return false
	}
	if executing.deferredQuiescence != nil && executing.deferredQuiescence != state {
		if !executing.deferredQuiescence.completed {
			return false
		}
		executing.deferredQuiescence = nil
	}
	executing.deferredQuiescence = state
	state.closings[executing] = struct{}{}
	return true
}

func (p *ReadOnlyPool) newQuiescenceRelease(key readOnlyPoolKey, state *readOnlyPoolQuiescence) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			p.releaseQuiescence(key, state)
		})
	}
}

func (p *ReadOnlyPool) completeQuiescence(state *readOnlyPoolQuiescence, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completeQuiescenceLocked(state, err)
}

func (p *ReadOnlyPool) completeQuiescenceLocked(state *readOnlyPoolQuiescence, err error) {
	if state == nil || state.completed {
		return
	}
	if err != nil && !errors.Is(state.err, err) {
		state.err = errors.Join(state.err, err)
	}
	state.completed = true
	clear(state.closings)
	clear(state.openings)
	close(state.ready)
}

func (p *ReadOnlyPool) completeDeferredQuiescenceLocked(closeOp *readOnlyPoolClose, err error) {
	state := closeOp.deferredQuiescence
	if state == nil {
		return
	}
	closeOp.deferredQuiescence = nil
	p.completeQuiescenceLocked(state, err)
	if state.owners == 0 && p.quiescing[closeOp.key] == state {
		delete(p.quiescing, closeOp.key)
	}
}

func (p *ReadOnlyPool) releaseQuiescence(key readOnlyPoolKey, state *readOnlyPoolQuiescence) {
	p.mu.Lock()
	defer p.mu.Unlock()
	current := p.quiescing[key]
	if current == nil || current != state {
		return
	}
	if current.owners > 0 {
		current.owners--
	}
	if current.owners == 0 && current.completed {
		delete(p.quiescing, key)
	}
}
