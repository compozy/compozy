package store

import "time"

// SessionLifecycleTimes keeps optional catalog timestamps off the hot SessionInfo value.
type SessionLifecycleTimes struct {
	AttachExpiresAt *time.Time
	ArchivedAt      *time.Time
	ParkedAt        *time.Time
	IdleExpiresAt   *time.Time
	DrainingAt      *time.Time
}

// AttachExpiresAtValue returns the optional attachment expiry.
func (s SessionInfo) AttachExpiresAtValue() *time.Time {
	if s.Lifecycle == nil {
		return nil
	}
	return s.Lifecycle.AttachExpiresAt
}

// ArchivedAtValue returns the optional archive timestamp.
func (s SessionInfo) ArchivedAtValue() *time.Time {
	if s.Lifecycle == nil {
		return nil
	}
	return s.Lifecycle.ArchivedAt
}

// ParkedAtValue returns the optional parked timestamp.
func (s SessionInfo) ParkedAtValue() *time.Time {
	if s.Lifecycle == nil {
		return nil
	}
	return s.Lifecycle.ParkedAt
}

// IdleExpiresAtValue returns the optional parked-child expiry.
func (s SessionInfo) IdleExpiresAtValue() *time.Time {
	if s.Lifecycle == nil {
		return nil
	}
	return s.Lifecycle.IdleExpiresAt
}

// DrainingAtValue returns the optional subtree-drain fence timestamp.
func (s SessionInfo) DrainingAtValue() *time.Time {
	if s.Lifecycle == nil {
		return nil
	}
	return s.Lifecycle.DrainingAt
}

// SetLifecycleTimes replaces all optional catalog lifecycle timestamps.
func (s *SessionInfo) SetLifecycleTimes(times SessionLifecycleTimes) {
	s.Lifecycle = &times
}

func (s *SessionInfo) ensureLifecycleTimes() *SessionLifecycleTimes {
	if s.Lifecycle == nil {
		s.Lifecycle = &SessionLifecycleTimes{}
	}
	return s.Lifecycle
}

// SetAttachExpiresAt replaces the optional attachment expiry.
func (s *SessionInfo) SetAttachExpiresAt(value *time.Time) {
	s.ensureLifecycleTimes().AttachExpiresAt = value
}

// SetArchivedAt replaces the optional archive timestamp.
func (s *SessionInfo) SetArchivedAt(value *time.Time) {
	s.ensureLifecycleTimes().ArchivedAt = value
}

// SetParkedAt replaces the optional parked timestamp.
func (s *SessionInfo) SetParkedAt(value *time.Time) {
	s.ensureLifecycleTimes().ParkedAt = value
}

// SetIdleExpiresAt replaces the optional parked-child expiry.
func (s *SessionInfo) SetIdleExpiresAt(value *time.Time) {
	s.ensureLifecycleTimes().IdleExpiresAt = value
}

// SetDrainingAt replaces the optional subtree-drain fence timestamp.
func (s *SessionInfo) SetDrainingAt(value *time.Time) {
	s.ensureLifecycleTimes().DrainingAt = value
}
