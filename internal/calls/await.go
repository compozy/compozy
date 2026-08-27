package calls

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Get returns one call from an exact ownership scope.
func (s *Service) Get(
	ctx context.Context,
	profileID string,
	workspaceID string,
	callID string,
) (CallRecord, error) {
	scope, err := NormalizeCallScope(CallScope{ProfileID: profileID, WorkspaceID: workspaceID})
	if err != nil {
		return CallRecord{}, err
	}
	return s.store.GetCall(ctx, scope, callID)
}

// Await waits until at least one selected call settles or the bounded timeout expires.
func (s *Service) Await(ctx context.Context, input AwaitInput) (AwaitOutcome, error) {
	ids, err := normalizeAwaitIDs(input.CallIDs)
	if err != nil {
		return AwaitOutcome{}, err
	}
	timeout := input.Timeout
	if timeout < 0 {
		return AwaitOutcome{}, newError(CodeValidation, "timeout must be zero or positive", nil)
	}
	if timeout > MaxAwaitDuration {
		timeout = MaxAwaitDuration
	}
	wake, unregister, err := s.registerWaiters(ids)
	if err != nil {
		return AwaitOutcome{}, err
	}
	defer unregister()
	scope, err := NormalizeCallScope(CallScope{
		ProfileID: input.ProfileID, Scope: input.Scope, WorkspaceID: input.WorkspaceID,
	})
	if err != nil {
		return AwaitOutcome{}, err
	}
	snapshot := func() (AwaitOutcome, error) {
		outcome := AwaitOutcome{ClampedTimeout: timeout}
		for _, callID := range ids {
			record, getErr := s.store.GetCall(ctx, scope, callID)
			if getErr != nil {
				return AwaitOutcome{}, getErr
			}
			if record.State.Terminal() {
				outcome.Settled = append(outcome.Settled, record)
			} else {
				outcome.Pending = append(outcome.Pending, callID)
			}
		}
		outcome.Outcome = classifyAwaitOutcome(outcome)
		return outcome, nil
	}
	outcome, err := snapshot()
	if err != nil || len(outcome.Settled) > 0 || timeout == 0 {
		return s.withResume(outcome, err)
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return AwaitOutcome{}, awaitContextError(ctx.Err())
		case <-timer.C:
			outcome, err = snapshot()
			return s.withResume(outcome, err)
		case <-wake:
		}
		outcome, err = snapshot()
		if err != nil || len(outcome.Settled) > 0 {
			return s.withResume(outcome, err)
		}
	}
}

func classifyAwaitOutcome(outcome AwaitOutcome) AwaitOutcomeKind {
	switch {
	case len(outcome.Pending) == 0:
		return AwaitOutcomeComplete
	case len(outcome.Settled) > 0:
		return AwaitOutcomePartial
	default:
		return AwaitOutcomeTimeout
	}
}

func normalizeAwaitIDs(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, newError(CodeValidation, "call_ids is required", nil)
	}
	seen := make(map[string]struct{}, len(values))
	ids := make([]string, 0, len(values))
	for _, value := range values {
		id := strings.TrimSpace(value)
		if id == "" {
			return nil, newError(CodeValidation, "call_ids cannot contain blanks", nil)
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Service) registerWaiters(callIDs []string) (<-chan struct{}, func(), error) {
	s.waitMu.Lock()
	for _, callID := range callIDs {
		if len(s.waiters[callID]) >= MaxConcurrentAwait {
			s.waitMu.Unlock()
			return nil, nil, newError(
				CodeValidation,
				fmt.Sprintf("call %q already has %d active awaiters", callID, MaxConcurrentAwait),
				nil,
			)
		}
	}
	s.nextWaiterID++
	id := s.nextWaiterID
	wake := make(chan struct{}, 1)
	for _, callID := range callIDs {
		if s.waiters[callID] == nil {
			s.waiters[callID] = make(map[uint64]chan struct{})
		}
		s.waiters[callID][id] = wake
	}
	s.waitMu.Unlock()
	return wake, func() {
		s.waitMu.Lock()
		defer s.waitMu.Unlock()
		for _, callID := range callIDs {
			delete(s.waiters[callID], id)
			if len(s.waiters[callID]) == 0 {
				delete(s.waiters, callID)
			}
		}
	}, nil
}

func (s *Service) notifyWaiters(callID string) {
	s.waitMu.Lock()
	defer s.waitMu.Unlock()
	for _, wake := range s.waiters[strings.TrimSpace(callID)] {
		select {
		case wake <- struct{}{}:
		default:
		}
	}
}

func (s *Service) withResume(outcome AwaitOutcome, outcomeErr error) (AwaitOutcome, error) {
	if outcomeErr != nil || len(outcome.Pending) == 0 {
		return outcome, outcomeErr
	}
	resume, err := s.newID("cawait")
	if err != nil {
		return AwaitOutcome{}, fmt.Errorf("calls: allocate await resume: %w", err)
	}
	outcome.Resume = resume
	return outcome, nil
}

func awaitContextError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return newError(CodeTimedOut, "call await timed out", err)
	case errors.Is(err, context.Canceled):
		return newError(CodeCanceled, "call await canceled", err)
	default:
		return err
	}
}
