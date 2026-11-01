package inputqueue

import "context"

type syntheticAdmissionStore interface {
	SyntheticSessionInputAdmitted(ctx context.Context, sessionID, messageID string) (bool, error)
}

// SyntheticAdmitted reports whether a synthetic input with messageID was already queued, dispatched, or
// sent for the session. Stores without the lookup report false.
func (s *Service) SyntheticAdmitted(ctx context.Context, sessionID, messageID string) (bool, error) {
	lookup, ok := s.store.(syntheticAdmissionStore)
	if !ok {
		return false, nil
	}
	return lookup.SyntheticSessionInputAdmitted(ctx, sessionID, messageID)
}
