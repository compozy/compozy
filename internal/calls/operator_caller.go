package calls

import (
	"context"
	"errors"
	"time"
)

type operatorCallerReopener interface {
	ReopenOperatorCaller(context.Context, string, time.Time) error
}

// ResolveOperatorCaller atomically converges concurrent first-call candidates on one durable caller session.
func (s *Service) ResolveOperatorCaller(
	ctx context.Context,
	candidate OperatorCallerBinding,
) (OperatorCallerBinding, error) {
	return s.store.ResolveOperatorCaller(ctx, candidate)
}

// ReopenOperatorCaller removes a completed subtree-drain fence from the
// stable logical caller before its session is resumed for new operator work.
func (s *Service) ReopenOperatorCaller(ctx context.Context, sessionID string) error {
	reopener, ok := s.store.(operatorCallerReopener)
	if !ok {
		return errors.New("calls: operator caller store cannot reopen drained sessions")
	}
	return reopener.ReopenOperatorCaller(ctx, sessionID, s.now().UTC())
}
