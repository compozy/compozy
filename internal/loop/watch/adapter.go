package watch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrPollerRequired reports a missing watch-source poll dependency.
	ErrPollerRequired = errors.New("loop watch poller is required")
	// ErrSpecRequired reports an empty watch-source spec.
	ErrSpecRequired = errors.New("loop watch spec is required")
	// ErrSpecInvalid reports a malformed watch-source spec.
	ErrSpecInvalid = errors.New("loop watch spec is invalid")
)

// Adapter runs one callback-free poll -> ready -> settle -> confirm claim cycle.
type Adapter struct {
	poller Poller
}

// NewAdapter constructs a watch-source adapter for coordinator claims.
func NewAdapter(poller Poller) (*Adapter, error) {
	if poller == nil {
		return nil, ErrPollerRequired
	}
	return &Adapter{poller: poller}, nil
}

// Tick runs exactly one watch-source claim cycle. It never parks a goroutine.
func (a *Adapter) Tick(ctx context.Context, req TickRequest) (TickResult, error) {
	if ctx == nil {
		return TickResult{}, errors.New("loop watch context is required")
	}
	if a == nil || a.poller == nil {
		return TickResult{}, ErrPollerRequired
	}
	if len(req.Spec) == 0 {
		return TickResult{}, ErrSpecRequired
	}
	if !json.Valid(req.Spec) {
		return TickResult{}, ErrSpecInvalid
	}
	now := req.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	machine := newWatchFSM()
	transitions := make([]Transition, 0, 4)
	if err := recordTransition(ctx, machine, &transitions, watchEventPoll); err != nil {
		return TickResult{}, err
	}
	response, err := a.poller.Poll(ctx, PollRequest{
		Spec:                cloneRawMessage(req.Spec),
		ExpectedStateDigest: req.ExpectedStateDigest,
	})
	if err != nil {
		return TickResult{}, fmt.Errorf("loop watch poll: %w", err)
	}
	if !response.Ready {
		return waitOrStall(ctx, machine, &transitions, req, response, now)
	}
	if err := recordTransition(ctx, machine, &transitions, watchEventReady); err != nil {
		return TickResult{}, err
	}
	if response.SettledAt != nil && now.Before(response.SettledAt.UTC()) {
		return waitOrStall(ctx, machine, &transitions, req, response, now)
	}
	if err := recordTransition(ctx, machine, &transitions, watchEventSettle); err != nil {
		return TickResult{}, err
	}
	if err := recordTransition(ctx, machine, &transitions, watchEventConfirm); err != nil {
		return TickResult{}, err
	}
	return TickResult{Outcome: OutcomeReady, Response: response, Transitions: transitions}, nil
}

func recordTransition(
	ctx context.Context,
	machine *watchFSM,
	transitions *[]Transition,
	event string,
) error {
	transition, err := machine.transition(ctx, event)
	if err != nil {
		return err
	}
	*transitions = append(*transitions, transition)
	return nil
}

func waitOrStall(
	ctx context.Context,
	machine *watchFSM,
	transitions *[]Transition,
	req TickRequest,
	response PollResponse,
	now time.Time,
) (TickResult, error) {
	if silenceExceeded(req.LastProgressAt, req.SilenceWindow, now) {
		if err := recordTransition(ctx, machine, transitions, watchEventStall); err != nil {
			return TickResult{}, err
		}
		return TickResult{Outcome: OutcomeStalled, Response: response, Transitions: *transitions}, nil
	}
	if err := recordTransition(ctx, machine, transitions, watchEventWait); err != nil {
		return TickResult{}, err
	}
	return TickResult{Outcome: OutcomeWaiting, Response: response, Transitions: *transitions}, nil
}

func silenceExceeded(lastProgressAt time.Time, window time.Duration, now time.Time) bool {
	if window <= 0 || lastProgressAt.IsZero() {
		return false
	}
	return !now.UTC().Before(lastProgressAt.UTC().Add(window))
}
