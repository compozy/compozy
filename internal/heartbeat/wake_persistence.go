package heartbeat

import (
	"context"

	"errors"
	"fmt"
	"strings"

	"time"
)

func (s *ManagedWakeService) currentWakeState(ctx context.Context, req WakeRequest) (WakeState, error) {
	state, err := s.store.GetHeartbeatWakeState(ctx, req.WorkspaceID, req.AgentName, req.SessionID)
	if err != nil {
		if errors.Is(err, ErrWakeStateNotFound) {
			return WakeState{}, nil
		}
		return WakeState{}, fmt.Errorf("heartbeat: get wake state for %q: %w", req.SessionID, err)
	}
	return state, nil
}

func (s *ManagedWakeService) recordRateLimited(ctx context.Context, req WakeRequest) (WakeDecision, error) {
	if ctx == nil {
		return WakeDecision{}, errors.New("heartbeat: wake context is required")
	}
	normalized, err := normalizeWakeRequest(req)
	if err != nil {
		return WakeDecision{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.currentTime()
	state, stateErr := s.currentWakeState(ctx, normalized)
	if stateErr != nil {
		return WakeDecision{}, stateErr
	}
	decision := s.newDecision(WakeResultRateLimited, WakeReasonHeartbeatRateLimited, "", "", "")
	return s.recordDecision(ctx, normalized, decision, state, now)
}

func (s *ManagedWakeService) recordDecision(
	ctx context.Context,
	req WakeRequest,
	decision WakeDecision,
	previous WakeState,
	now time.Time,
) (WakeDecision, error) {
	if req.DryRun {
		return dryRunDecision(decision), nil
	}
	if strings.TrimSpace(decision.WakeEventID) == "" {
		decision.WakeEventID = s.newID("hwe")
	}
	if decision.Result == WakeResultSent && strings.TrimSpace(decision.SyntheticPromptID) == "" {
		decision.SyntheticPromptID = s.newID("turn")
	}
	event := wakeEventFromDecision(req, decision, now, s.config.WakeEventRetention)
	state := nextWakeState(req, decision, previous, now, s.config.WakeCooldown)
	if _, _, err := s.store.RecordHeartbeatWakeDecision(ctx, event, state); err != nil {
		return WakeDecision{}, fmt.Errorf("heartbeat: record wake decision %q: %w", event.ID, err)
	}
	return decision, nil
}

func (s *ManagedWakeService) appendDecisionEvent(
	ctx context.Context,
	req WakeRequest,
	decision WakeDecision,
	now time.Time,
) (WakeDecision, error) {
	if strings.TrimSpace(decision.WakeEventID) == "" {
		decision.WakeEventID = s.newID("hwe")
	}
	if decision.Result == WakeResultSent && strings.TrimSpace(decision.SyntheticPromptID) == "" {
		decision.SyntheticPromptID = s.newID("turn")
	}
	event := wakeEventFromDecision(req, decision, now, s.config.WakeEventRetention)
	if _, err := s.store.AppendHeartbeatWakeEvent(ctx, event); err != nil {
		return WakeDecision{}, fmt.Errorf("heartbeat: append wake event %q: %w", event.ID, err)
	}
	return decision, nil
}

func wakeEventFromDecision(req WakeRequest, decision WakeDecision, now time.Time, retention time.Duration) WakeEvent {
	return WakeEvent{
		ID:                decision.WakeEventID,
		WorkspaceID:       req.WorkspaceID,
		AgentName:         req.AgentName,
		SessionID:         req.SessionID,
		PolicySnapshotID:  decision.PolicySnapshotID,
		Source:            req.Source,
		Result:            decision.Result,
		Reason:            decision.Reason,
		SyntheticPromptID: decision.SyntheticPromptID,
		CreatedAt:         now,
		ExpiresAt:         now.Add(retention),
	}
}

func dryRunDecision(decision WakeDecision) WakeDecision {
	decision.WakeEventID = ""
	decision.SyntheticPromptID = ""
	return decision
}

func nextWakeState(
	req WakeRequest,
	decision WakeDecision,
	previous WakeState,
	now time.Time,
	cooldown time.Duration,
) WakeState {
	state := previous.Normalize()
	state.WorkspaceID = req.WorkspaceID
	state.AgentName = req.AgentName
	state.SessionID = req.SessionID
	state.PolicySnapshotID = decision.PolicySnapshotID
	state.LastResult = decision.Result
	state.LastReason = decision.Reason
	state.UpdatedAt = now
	switch decision.Result {
	case WakeResultSent:
		state.LastWakeAt = now
		state.NextAllowedAt = now.Add(cooldown)
		state.CoalescedCount = 0
	case WakeResultCoalesced:
		state.CoalescedCount++
	}
	return state
}

func (s *ManagedWakeService) newDecision(
	result WakeResult,
	reason WakeReason,
	snapshotID string,
	policyDigest string,
	configDigest string,
) WakeDecision {
	decision := WakeDecision{
		WakeEventID:      s.newID("hwe"),
		Result:           result,
		Reason:           reason,
		PolicySnapshotID: strings.TrimSpace(snapshotID),
		PolicyDigest:     strings.TrimSpace(policyDigest),
		ConfigDigest:     strings.TrimSpace(configDigest),
		Diagnostics:      nil,
	}
	if result == WakeResultSent {
		decision.SyntheticPromptID = s.newID("turn")
	}
	return decision
}

func sanitizeWakeDiagnostic(diag Diagnostic) Diagnostic {
	sanitized := sanitizeDiagnostics([]Diagnostic{diag})
	if len(sanitized) == 0 {
		return Diagnostic{}
	}
	return sanitized[0]
}
