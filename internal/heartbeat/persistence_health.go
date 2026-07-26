package heartbeat

import (
	"fmt"
	"strings"
)

// Normalize trims session health metadata fields.
func (h SessionHealth) Normalize() SessionHealth {
	h.SessionID = strings.TrimSpace(h.SessionID)
	h.WorkspaceID = strings.TrimSpace(h.WorkspaceID)
	h.AgentName = strings.TrimSpace(h.AgentName)
	h.State = SessionHealthState(strings.TrimSpace(string(h.State)))
	h.Health = SessionHealthStatus(strings.TrimSpace(string(h.Health)))
	h.IneligibilityReason = strings.TrimSpace(h.IneligibilityReason)
	h.LastError = strings.TrimSpace(h.LastError)
	return h
}

// Validate ensures session health remains metadata-only runtime state.
func (h SessionHealth) Validate() error {
	normalized := h.Normalize()
	switch {
	case normalized.SessionID == "":
		return fmt.Errorf("%w: session id is required", ErrInvalidSessionHealth)
	case normalized.WorkspaceID == "":
		return fmt.Errorf("%w: workspace id is required", ErrInvalidSessionHealth)
	case normalized.AgentName == "":
		return fmt.Errorf("%w: agent name is required", ErrInvalidSessionHealth)
	case !ValidSessionHealthState(normalized.State):
		return fmt.Errorf("%w: invalid state %q", ErrInvalidSessionHealth, normalized.State)
	case !ValidSessionHealthStatus(normalized.Health):
		return fmt.Errorf("%w: invalid health %q", ErrInvalidSessionHealth, normalized.Health)
	case normalized.IneligibilityReason != "" &&
		!ValidSessionHealthIneligibilityReason(normalized.IneligibilityReason):
		return fmt.Errorf(
			"%w: invalid ineligibility reason %q",
			ErrInvalidSessionHealth,
			normalized.IneligibilityReason,
		)
	case normalized.UpdatedAt.IsZero():
		return fmt.Errorf("%w: updated_at is required", ErrInvalidSessionHealth)
	}
	return nil
}

// Validate ensures the health query uses sane bounds and enum filters.
func (q SessionHealthListQuery) Validate() error {
	if q.Limit < 0 {
		return fmt.Errorf("%w: invalid session health limit %d", ErrInvalidSessionHealth, q.Limit)
	}
	if q.State != "" && !ValidSessionHealthState(q.State) {
		return fmt.Errorf("%w: invalid state %q", ErrInvalidSessionHealth, q.State)
	}
	if q.Health != "" && !ValidSessionHealthStatus(q.Health) {
		return fmt.Errorf("%w: invalid health %q", ErrInvalidSessionHealth, q.Health)
	}
	return nil
}

// Normalize trims wake state metadata fields.
func (s WakeState) Normalize() WakeState {
	s.WorkspaceID = strings.TrimSpace(s.WorkspaceID)
	s.AgentName = strings.TrimSpace(s.AgentName)
	s.SessionID = strings.TrimSpace(s.SessionID)
	s.PolicySnapshotID = strings.TrimSpace(s.PolicySnapshotID)
	s.LastResult = WakeResult(strings.TrimSpace(string(s.LastResult)))
	s.LastReason = WakeReason(strings.TrimSpace(string(s.LastReason)))
	return s
}

// Validate ensures wake state cannot become claimable work.
func (s WakeState) Validate() error {
	normalized := s.Normalize()
	switch {
	case normalized.WorkspaceID == "":
		return fmt.Errorf("%w: workspace id is required", ErrInvalidWakeState)
	case normalized.AgentName == "":
		return fmt.Errorf("%w: agent name is required", ErrInvalidWakeState)
	case normalized.SessionID == "":
		return fmt.Errorf("%w: session id is required", ErrInvalidWakeState)
	case normalized.CoalescedCount < 0:
		return fmt.Errorf("%w: coalesced count must be non-negative", ErrInvalidWakeState)
	case !ValidWakeResult(normalized.LastResult):
		return fmt.Errorf("%w: invalid result %q", ErrInvalidWakeState, normalized.LastResult)
	case normalized.LastReason != "" && !ValidWakeReason(normalized.LastReason):
		return fmt.Errorf("%w: invalid reason %q", ErrInvalidWakeState, normalized.LastReason)
	case normalized.UpdatedAt.IsZero():
		return fmt.Errorf("%w: updated_at is required", ErrInvalidWakeState)
	}
	return nil
}

// Validate ensures the wake state query uses sane bounds.
func (q WakeStateListQuery) Validate() error {
	if q.Limit < 0 {
		return fmt.Errorf("%w: invalid wake state limit %d", ErrInvalidWakeState, q.Limit)
	}
	return nil
}

// Normalize trims wake event metadata fields.
func (e WakeEvent) Normalize() WakeEvent {
	e.ID = strings.TrimSpace(e.ID)
	e.WorkspaceID = strings.TrimSpace(e.WorkspaceID)
	e.AgentName = strings.TrimSpace(e.AgentName)
	e.SessionID = strings.TrimSpace(e.SessionID)
	e.PolicySnapshotID = strings.TrimSpace(e.PolicySnapshotID)
	e.Source = WakeSource(strings.TrimSpace(string(e.Source)))
	e.Result = WakeResult(strings.TrimSpace(string(e.Result)))
	e.Reason = WakeReason(strings.TrimSpace(string(e.Reason)))
	e.SyntheticPromptID = strings.TrimSpace(e.SyntheticPromptID)
	return e
}

// Validate ensures the wake event is retained audit state, not claimable work.
func (e WakeEvent) Validate() error {
	normalized := e.Normalize()
	switch {
	case normalized.ID == "":
		return fmt.Errorf("%w: id is required", ErrInvalidWakeEvent)
	case normalized.WorkspaceID == "":
		return fmt.Errorf("%w: workspace id is required", ErrInvalidWakeEvent)
	case normalized.AgentName == "":
		return fmt.Errorf("%w: agent name is required", ErrInvalidWakeEvent)
	case !ValidWakeSource(normalized.Source):
		return fmt.Errorf("%w: invalid source %q", ErrInvalidWakeEvent, normalized.Source)
	case !ValidWakeResult(normalized.Result):
		return fmt.Errorf("%w: invalid result %q", ErrInvalidWakeEvent, normalized.Result)
	case !ValidWakeReason(normalized.Reason):
		return fmt.Errorf("%w: invalid reason %q", ErrInvalidWakeEvent, normalized.Reason)
	case normalized.CreatedAt.IsZero():
		return fmt.Errorf("%w: created_at is required", ErrInvalidWakeEvent)
	case normalized.ExpiresAt.IsZero():
		return fmt.Errorf("%w: expires_at is required", ErrInvalidWakeEvent)
	}
	return nil
}

// Validate ensures the wake event query uses sane bounds and enum filters.
func (q WakeEventListQuery) Validate() error {
	if q.Limit < 0 {
		return fmt.Errorf("%w: invalid wake event limit %d", ErrInvalidWakeEvent, q.Limit)
	}
	if q.Source != "" && !ValidWakeSource(q.Source) {
		return fmt.Errorf("%w: invalid source %q", ErrInvalidWakeEvent, q.Source)
	}
	if q.Result != "" && !ValidWakeResult(q.Result) {
		return fmt.Errorf("%w: invalid result %q", ErrInvalidWakeEvent, q.Result)
	}
	if q.Reason != "" && !ValidWakeReason(q.Reason) {
		return fmt.Errorf("%w: invalid reason %q", ErrInvalidWakeEvent, q.Reason)
	}
	return nil
}
