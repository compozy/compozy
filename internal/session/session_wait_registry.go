package session

import (
	"fmt"
	"sync"
	"time"
)

type waitEdge struct {
	badge    Badge
	epoch    int64
	revision int64
	gone     bool
}

type waitTimer interface {
	Channel() <-chan time.Time
	Stop() bool
}

type waitTimerFactory func(time.Duration) waitTimer
type waitAfterFunc func(time.Duration, func()) waitTimer

type realWaitTimer struct {
	timer *time.Timer
}

func newRealWaitTimer(duration time.Duration) waitTimer {
	return &realWaitTimer{timer: time.NewTimer(duration)}
}

func newRealWaitAfterFunc(duration time.Duration, callback func()) waitTimer {
	return &realWaitTimer{timer: time.AfterFunc(duration, callback)}
}

func (t *realWaitTimer) Channel() <-chan time.Time {
	return t.timer.C
}

func (t *realWaitTimer) Stop() bool {
	return t.timer.Stop()
}

type sessionWaitRegistration struct {
	id        string
	sessionID string
	epoch     int64
	pinned    bool
	revision  int64
	until     map[Badge]struct{}
	edges     []waitEdge
	signal    chan struct{}
	attached  bool
	terminal  *WaitOutcome
	timer     waitTimer
}

type sessionWaitRegistry struct {
	mu        sync.Mutex
	byID      map[string]*sessionWaitRegistration
	bySession map[string]map[string]*sessionWaitRegistration
	newID     IDGenerator
	now       func() time.Time
	grace     time.Duration
	afterFunc waitAfterFunc
	closed    bool
}

func newSessionWaitRegistry(
	newID IDGenerator,
	now func() time.Time,
	afterFunc waitAfterFunc,
) *sessionWaitRegistry {
	return &sessionWaitRegistry{
		byID:      make(map[string]*sessionWaitRegistration),
		bySession: make(map[string]map[string]*sessionWaitRegistration),
		newID:     newID,
		now:       now,
		grace:     WaitResumeGrace,
		afterFunc: afterFunc,
	}
}

func (r *sessionWaitRegistry) register(
	sessionID string,
	epoch int64,
	until map[Badge]struct{},
) (*sessionWaitRegistration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, fmt.Errorf("%w: registry is closed", ErrWaitValidation)
	}
	registrations := r.bySession[sessionID]
	if len(registrations) >= WaitMaxPerSession {
		return nil, fmt.Errorf("%w: session %q permits at most %d concurrent waits", ErrWaitLimit, sessionID, WaitMaxPerSession)
	}
	id, err := r.newID()
	if err != nil {
		return nil, fmt.Errorf("session: generate wait registration id: %w", err)
	}
	registration := &sessionWaitRegistration{
		id: id, sessionID: sessionID, epoch: epoch, pinned: epoch != 0, until: cloneWaitBadges(until),
		edges: make([]waitEdge, 0, WaitEdgeBufferSize), signal: make(chan struct{}, 1), attached: true,
	}
	if registrations == nil {
		registrations = make(map[string]*sessionWaitRegistration)
		r.bySession[sessionID] = registrations
	}
	registrations[id] = registration
	r.byID[id] = registration
	return registration, nil
}

func (r *sessionWaitRegistry) resume(
	req WaitRequest,
	until map[Badge]struct{},
) (*sessionWaitRegistration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	registration := r.byID[req.ResumeID]
	if registration == nil {
		return nil, fmt.Errorf("%w: %s", ErrWaitExpired, req.ResumeID)
	}
	if registration.sessionID != req.SessionID || !sameWaitBadges(registration.until, until) ||
		(req.Epoch != 0 && registration.epoch != req.Epoch) {
		return nil, fmt.Errorf("%w: resume request does not match its original registration", ErrWaitValidation)
	}
	if registration.attached {
		return nil, fmt.Errorf("%w: %s", ErrWaitInUse, req.ResumeID)
	}
	registration.attached = true
	if registration.timer != nil {
		registration.timer.Stop()
		registration.timer = nil
	}
	return registration, nil
}

func (r *sessionWaitRegistry) pin(registration *sessionWaitRegistration, epoch int64, revision int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if registration == nil {
		return
	}
	registration.epoch = epoch
	registration.pinned = true
	registration.revision = revision
}

func (r *sessionWaitRegistry) publish(sessionID string, edge waitEdge) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, registration := range r.bySession[sessionID] {
		if registration.terminal != nil {
			continue
		}
		if registration.pinned && registration.epoch != edge.epoch {
			r.finishLocked(registration, outcomeForWaitEdge(registration, waitEdge{
				gone: true, epoch: edge.epoch, revision: edge.revision,
			}))
			continue
		}
		if len(registration.edges) >= WaitEdgeBufferSize {
			r.finishLocked(registration, WaitOutcome{
				Outcome: WaitResultOverflow, Revision: max(registration.revision, edge.revision),
			})
			continue
		}
		registration.edges = append(registration.edges, edge)
		registration.revision = max(registration.revision, edge.revision)
		terminal := edge.gone || edge.badge == BadgeStopped || edge.badge == BadgeFailed
		if terminal {
			r.removeSessionIndexLocked(registration)
		}
		notifyWaitRegistration(registration)
	}
}

func (r *sessionWaitRegistry) next(registration *sessionWaitRegistration) (WaitOutcome, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if registration == nil {
		return WaitOutcome{}, false
	}
	if registration.terminal != nil {
		outcome := *registration.terminal
		r.removeIndexesLocked(registration)
		return outcome, true
	}
	for len(registration.edges) > 0 {
		edge := registration.edges[0]
		registration.edges = registration.edges[1:]
		if registration.pinned && edge.epoch != registration.epoch {
			return r.finishLocked(registration, outcomeForWaitEdge(registration, waitEdge{
				gone: true, epoch: edge.epoch, revision: edge.revision,
			})), true
		}
		if edge.gone {
			return r.finishLocked(registration, outcomeForWaitEdge(registration, edge)), true
		}
		if waitBadgeMatches(registration.until, edge.badge) {
			return r.finishLocked(registration, outcomeForWaitEdge(registration, edge)), true
		}
		if edge.badge == BadgeStopped || edge.badge == BadgeFailed {
			return r.finishLocked(registration, WaitOutcome{
				Outcome: WaitResultSessionGone, State: edge.badge, Revision: edge.revision,
			}), true
		}
	}
	return WaitOutcome{}, false
}

func (r *sessionWaitRegistry) complete(
	registration *sessionWaitRegistration,
	edge waitEdge,
) WaitOutcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.finishLocked(registration, outcomeForWaitEdge(registration, edge))
}

func (r *sessionWaitRegistry) cancel(registration *sessionWaitRegistration) WaitOutcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.finishLocked(registration, WaitOutcome{
		Outcome: WaitResultCanceled, Revision: registration.revision,
	})
}

func (r *sessionWaitRegistry) detach(registration *sessionWaitRegistration) WaitOutcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	if registration.terminal != nil {
		return *registration.terminal
	}
	registration.attached = false
	registration.timer = r.afterFunc(r.grace, func() { r.expire(registration.id) })
	return WaitOutcome{
		Outcome:  WaitResultTimeout,
		Revision: registration.revision,
		ResumeID: registration.id,
	}
}

func (r *sessionWaitRegistry) remove(registration *sessionWaitRegistration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removeIndexesLocked(registration)
}

func (r *sessionWaitRegistry) finishLocked(
	registration *sessionWaitRegistration,
	outcome WaitOutcome,
) WaitOutcome {
	if registration == nil {
		return outcome
	}
	if registration.terminal != nil {
		return *registration.terminal
	}
	registration.terminal = &outcome
	r.removeSessionIndexLocked(registration)
	if registration.attached {
		delete(r.byID, registration.id)
		if registration.timer != nil {
			registration.timer.Stop()
			registration.timer = nil
		}
	}
	notifyWaitRegistration(registration)
	return outcome
}

func (r *sessionWaitRegistry) removeIndexesLocked(registration *sessionWaitRegistration) {
	if registration == nil {
		return
	}
	delete(r.byID, registration.id)
	r.removeSessionIndexLocked(registration)
}

func (r *sessionWaitRegistry) removeSessionIndexLocked(registration *sessionWaitRegistration) {
	if registration == nil {
		return
	}
	registrations := r.bySession[registration.sessionID]
	delete(registrations, registration.id)
	if len(registrations) == 0 {
		delete(r.bySession, registration.sessionID)
	}
}

func (r *sessionWaitRegistry) expire(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	registration := r.byID[id]
	if registration == nil || registration.attached {
		return
	}
	r.removeIndexesLocked(registration)
}

func (r *sessionWaitRegistry) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.closed = true
	for _, registration := range r.byID {
		if registration.timer != nil {
			registration.timer.Stop()
		}
		outcome := WaitOutcome{Outcome: WaitResultCanceled, Revision: registration.revision}
		registration.terminal = &outcome
		notifyWaitRegistration(registration)
	}
	r.byID = make(map[string]*sessionWaitRegistration)
	r.bySession = make(map[string]map[string]*sessionWaitRegistration)
}

func outcomeForWaitEdge(registration *sessionWaitRegistration, edge waitEdge) WaitOutcome {
	if edge.gone || (registration.pinned && edge.epoch != registration.epoch) {
		return WaitOutcome{Outcome: WaitResultSessionGone, Revision: edge.revision}
	}
	return WaitOutcome{Outcome: WaitResultStateReached, State: edge.badge, Revision: edge.revision}
}

func notifyWaitRegistration(registration *sessionWaitRegistration) {
	select {
	case registration.signal <- struct{}{}:
	default:
	}
}

func cloneWaitBadges(input map[Badge]struct{}) map[Badge]struct{} {
	cloned := make(map[Badge]struct{}, len(input))
	for badge := range input {
		cloned[badge] = struct{}{}
	}
	return cloned
}

func sameWaitBadges(left map[Badge]struct{}, right map[Badge]struct{}) bool {
	if len(left) != len(right) {
		return false
	}
	for badge := range left {
		if _, ok := right[badge]; !ok {
			return false
		}
	}
	return true
}
