package settings

import "context"

// HasPendingConfigRestart reports whether persisted desired config still
// differs from the daemon's active runtime projection.
func (s *service) HasPendingConfigRestart(ctx context.Context) (bool, error) {
	state, err := s.ensureActiveConfigState(ctx)
	if err != nil {
		return false, err
	}
	desiredHash, _, err := s.currentDesiredConfigHash()
	if err != nil {
		return false, err
	}
	return desiredHash != state.hash, nil
}
