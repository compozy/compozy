package calls

import "context"

// ResolveOperatorCaller atomically converges concurrent first-call candidates on one durable caller session.
func (s *Service) ResolveOperatorCaller(
	ctx context.Context,
	candidate OperatorCallerBinding,
) (OperatorCallerBinding, error) {
	return s.store.ResolveOperatorCaller(ctx, candidate)
}
