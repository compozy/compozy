package journal

import (
	"context"
	"errors"
	"fmt"
)

func (l *terminalLane) close(ctx context.Context) error {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return l.waitUntilClosed(ctx)
	}
	l.sealed = true
	close(l.reservationsChanged)
	l.reservationsChanged = make(chan struct{})
	l.mu.Unlock()
	if err := l.waitForInputReservations(ctx); err != nil {
		return err
	}
	l.mu.Lock()
	if l.idleTimer != nil {
		l.idleTimer.Stop()
	}
	l.idleGeneration++
	candidates := append([]idleCandidate(nil), l.idle...)
	l.idle = nil
	l.mu.Unlock()
	for _, candidate := range candidates {
		l.finishIdleCandidate(candidate)
	}
	l.finishAssembly(nil, l.service.now())
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()
	select {
	case l.wake <- struct{}{}:
	default:
	}
	return l.waitUntilClosed(ctx)
}

func (l *terminalLane) waitForInputReservations(ctx context.Context) error {
	for {
		l.mu.Lock()
		active := l.inputReservations
		changed := l.reservationsChanged
		l.mu.Unlock()
		if active == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("terminal journal: wait for %q input delivery: %w", l.info.ID, context.Cause(ctx))
		case <-changed:
		}
	}
}

func (l *terminalLane) waitUntilClosed(ctx context.Context) error {
	select {
	case <-ctx.Done():
		flushErr := fmt.Errorf("terminal journal: flush %q: %w", l.info.ID, context.Cause(ctx))
		return flushErr
	case <-l.done:
		l.mu.Lock()
		defer l.mu.Unlock()
		return l.err
	}
}

func (s *Service) closeLanes(ctx context.Context, matches func(*terminalLane) bool) error {
	s.mu.Lock()
	type laneEntry struct {
		key  string
		lane *terminalLane
	}
	lanes := make([]laneEntry, 0)
	for key, lane := range s.lanes {
		if matches(lane) {
			lanes = append(lanes, laneEntry{key: key, lane: lane})
		}
	}
	s.mu.Unlock()
	var errs []error
	for _, entry := range lanes {
		laneCtx, cancelLane := independentLaneCloseContext(ctx)
		err := entry.lane.close(laneCtx)
		s.removeStoppedLane(entry.key, entry.lane)
		if err != nil {
			errs = append(errs, err)
			cancelLane()
			continue
		}
		cancelLane()
	}
	return errors.Join(errs...)
}

func (s *Service) removeStoppedLane(key string, lane *terminalLane) {
	select {
	case <-lane.done:
	default:
		return
	}
	s.mu.Lock()
	if s.lanes[key] == lane {
		delete(s.lanes, key)
	}
	s.mu.Unlock()
}

func independentLaneCloseContext(parent context.Context) (context.Context, context.CancelFunc) {
	timeoutCtx, cancelTimeout := context.WithTimeout(context.WithoutCancel(parent), laneCleanupTimeout)
	closeCtx, cancelCause := context.WithCancelCause(timeoutCtx)
	stopParent := context.AfterFunc(parent, func() { cancelCause(context.Cause(parent)) })
	return closeCtx, func() {
		stopParent()
		cancelCause(context.Canceled)
		cancelTimeout()
	}
}
