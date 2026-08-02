package sessiondb

import (
	"context"
	"errors"
	"fmt"
)

// readOnlyPoolQuiescenceResponsibility owns the complete lifecycle of an
// entry captured by quiescence. Becoming idle is only an intermediate state;
// ready closes after the entry's successor recorder close publishes.
type readOnlyPoolQuiescenceResponsibility struct {
	state     *readOnlyPoolQuiescence
	entry     *readOnlyPoolEntry
	closeOp   *readOnlyPoolClose
	ready     chan struct{}
	err       error
	completed bool
}

func (p *ReadOnlyPool) addEntryResponsibilityLocked(
	state *readOnlyPoolQuiescence,
	entry *readOnlyPoolEntry,
) *readOnlyPoolQuiescenceResponsibility {
	if state == nil || entry == nil || state.completed {
		return nil
	}
	if responsibility := state.entries[entry]; responsibility != nil {
		return responsibility
	}
	responsibility := &readOnlyPoolQuiescenceResponsibility{
		state: state,
		entry: entry,
		ready: make(chan struct{}),
	}
	state.entries[entry] = responsibility
	return responsibility
}

// attachCloseToQuiescenceLocked publishes the close edge before its recorder
// callback can run. Dependency checks therefore observe every materialized
// close in the same atomic state transition that creates its reservation.
func (p *ReadOnlyPool) attachCloseToQuiescenceLocked(closeOp *readOnlyPoolClose) {
	state := p.quiescing[closeOp.key]
	if state == nil || state.completed {
		return
	}
	state.closings[closeOp] = struct{}{}
	closeOp.quiescence = state
}

func (p *ReadOnlyPool) attachEntryCloseLocked(
	entry *readOnlyPoolEntry,
	closeOp *readOnlyPoolClose,
) {
	state := p.quiescing[closeOp.key]
	if state == nil {
		return
	}
	responsibility := state.entries[entry]
	if responsibility == nil || responsibility.completed || responsibility.closeOp != nil {
		return
	}
	responsibility.closeOp = closeOp
	closeOp.responsibility = responsibility
}

func (p *ReadOnlyPool) completeEntryResponsibilityLocked(
	key readOnlyPoolKey,
	entry *readOnlyPoolEntry,
	err error,
) {
	state := p.quiescing[key]
	if state == nil {
		return
	}
	p.completeResponsibilityLocked(state.entries[entry], err)
}

func (p *ReadOnlyPool) completeResponsibilityLocked(
	responsibility *readOnlyPoolQuiescenceResponsibility,
	err error,
) {
	if responsibility == nil || responsibility.completed {
		return
	}
	responsibility.err = err
	responsibility.completed = true
	if state := responsibility.state; state != nil {
		delete(state.entries, responsibility.entry)
	}
	close(responsibility.ready)
}

func (p *ReadOnlyPool) recordCloseTerminalLocked(closeOp *readOnlyPoolClose, err error) {
	states := make(map[*readOnlyPoolQuiescence]struct{}, 3)
	if state := closeOp.quiescence; state != nil {
		states[state] = struct{}{}
	}
	if state := closeOp.deferredQuiescence; state != nil {
		states[state] = struct{}{}
	}
	if responsibility := closeOp.responsibility; responsibility != nil && responsibility.state != nil {
		states[responsibility.state] = struct{}{}
	}
	if state := p.quiescing[closeOp.key]; state != nil {
		if _, tracked := state.closings[closeOp]; tracked {
			states[state] = struct{}{}
		}
	}
	for state := range states {
		if !state.completed && err != nil {
			state.err = errors.Join(state.err, err)
		}
		delete(state.closings, closeOp)
	}
	p.completeResponsibilityLocked(closeOp.responsibility, err)
}

func (p *ReadOnlyPool) waitForResponsibility(
	ctx context.Context,
	state *readOnlyPoolQuiescence,
	responsibility *readOnlyPoolQuiescenceResponsibility,
) error {
	select {
	case <-responsibility.ready:
		return responsibility.err
	case <-state.ready:
		return state.err
	default:
	}
	select {
	case <-responsibility.ready:
		return responsibility.err
	case <-state.ready:
		return state.err
	case <-ctx.Done():
		return fmt.Errorf("store: wait for read-only pool entry close responsibility: %w", ctx.Err())
	}
}
