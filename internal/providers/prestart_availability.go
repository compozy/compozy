package providers

import (
	"context"
	"strings"
)

// ProfileAvailabilityChecker rejects provider startup while the owning profile
// is archived or reserved by an unfinished lifecycle operation.
type ProfileAvailabilityChecker interface {
	EnsureAvailableID(context.Context, string) error
}

// SetProfileAvailabilityChecker installs the daemon-owned lifecycle gate.
func (s *PreStarter) SetProfileAvailabilityChecker(checker ProfileAvailabilityChecker) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.profileAvailability = checker
	s.clearLocked()
	s.mu.Unlock()
}

func (s *PreStarter) clearLocked() {
	s.entries = make(map[preStartCacheKey]preStartCacheEntry)
	s.generation++
}

func (s *PreStarter) checkProfileAvailability(ctx context.Context, env *ProbeEnv) error {
	if s == nil || env == nil {
		return nil
	}
	profileID := strings.TrimSpace(env.PreStartScope.ProfileID)
	if profileID == "" {
		return nil
	}
	s.mu.Lock()
	checker := s.profileAvailability
	s.mu.Unlock()
	if checker == nil {
		return nil
	}
	return checker.EnsureAvailableID(ctx, profileID)
}
