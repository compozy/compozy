package heartbeat

import (
	"context"

	"errors"
	"fmt"
	"strings"

	"time"
)

func (s *ManagedWakeService) wakeLocked(ctx context.Context, req WakeRequest) (WakeDecision, error) {
	now := s.currentTime()
	if err := ctx.Err(); err != nil {
		return WakeDecision{}, err
	}
	if !s.config.Enabled {
		decision := s.newDecision(WakeResultSkipped, WakeReasonHeartbeatDisabled, "", "", "")
		return s.recordDecision(ctx, req, decision, WakeState{}, now)
	}

	snapshot, envelope, decision, hasPolicy, err := s.loadWakePolicy(ctx, req)
	if err != nil {
		return WakeDecision{}, err
	}
	state, stateErr := s.currentWakeState(ctx, req)
	if stateErr != nil {
		return WakeDecision{}, stateErr
	}
	if !hasPolicy {
		return s.recordDecision(ctx, req, decision, state, now)
	}
	if decision, done := s.policySkipDecision(snapshot, &envelope, now); done {
		return s.recordDecision(ctx, req, decision, state, now)
	}

	if !state.NextAllowedAt.IsZero() && state.NextAllowedAt.After(now) {
		result, reason := cooldownDecisionForSource(req.Source)
		decision := s.newDecision(result, reason, snapshot.ID, snapshot.Digest, snapshot.ConfigDigest)
		return s.recordDecision(ctx, req, decision, state, now)
	}

	healthDecision, done, err := s.healthSkipDecision(ctx, req, snapshot)
	if err != nil {
		return WakeDecision{}, err
	}
	if done {
		return s.recordDecision(ctx, req, healthDecision, state, now)
	}

	return s.dispatchWakePrompt(ctx, req, snapshot, &envelope, state, now)
}

func (s *ManagedWakeService) policySkipDecision(
	snapshot Snapshot,
	envelope *SnapshotEnvelope,
	now time.Time,
) (WakeDecision, bool) {
	if err := s.ensureCurrentConfigDigest(envelope, snapshot); err != nil {
		decision := s.newDecision(
			WakeResultSkipped,
			WakeReasonHeartbeatInvalid,
			snapshot.ID,
			snapshot.Digest,
			snapshot.ConfigDigest,
		)
		decision.Diagnostics = []Diagnostic{sanitizeWakeDiagnostic(Diagnostic{
			Code:     "heartbeat_config_digest_stale",
			Severity: diagnosticWarning,
			Message:  err.Error(),
			Field:    "config_digest",
		})}
		return decision, true
	}
	if envelope == nil || !envelope.Active || !envelope.Prompt.Active {
		return s.newDecision(
			WakeResultSkipped,
			WakeReasonHeartbeatDisabled,
			snapshot.ID,
			snapshot.Digest,
			snapshot.ConfigDigest,
		), true
	}
	allowed, err := envelope.Preferences.AllowsAt(now)
	if err != nil {
		decision := s.newDecision(
			WakeResultSkipped,
			WakeReasonHeartbeatInvalid,
			snapshot.ID,
			snapshot.Digest,
			snapshot.ConfigDigest,
		)
		decision.Diagnostics = []Diagnostic{diagnosticForError(
			"heartbeat_time_window_invalid",
			snapshot.SourcePath,
			err,
			ErrInvalid,
		)}
		return decision, true
	}
	if !allowed {
		return s.newDecision(
			WakeResultSkipped,
			WakeReasonQuietWindow,
			snapshot.ID,
			snapshot.Digest,
			snapshot.ConfigDigest,
		), true
	}
	return WakeDecision{}, false
}

func (s *ManagedWakeService) healthSkipDecision(
	ctx context.Context,
	req WakeRequest,
	snapshot Snapshot,
) (WakeDecision, bool, error) {
	health, err := s.healthReader.GetSessionHealth(ctx, req.SessionID)
	if err != nil {
		if errors.Is(err, ErrSessionHealthNotFound) {
			decision := s.newDecision(
				WakeResultSkipped,
				WakeReasonSessionNotFound,
				snapshot.ID,
				snapshot.Digest,
				snapshot.ConfigDigest,
			)
			return decision, true, nil
		}
		return WakeDecision{}, false, fmt.Errorf("heartbeat: read session health for %q: %w", req.SessionID, err)
	}
	if reason := ineligibleWakeReason(req, health); reason != "" {
		decision := s.newDecision(
			WakeResultSkipped,
			reason,
			snapshot.ID,
			snapshot.Digest,
			snapshot.ConfigDigest,
		)
		return decision, true, nil
	}
	return WakeDecision{}, false, nil
}

func (s *ManagedWakeService) dispatchWakePrompt(
	ctx context.Context,
	req WakeRequest,
	snapshot Snapshot,
	envelope *SnapshotEnvelope,
	state WakeState,
	now time.Time,
) (WakeDecision, error) {
	decision := s.newDecision(WakeResultSent, WakeReasonSent, snapshot.ID, snapshot.Digest, snapshot.ConfigDigest)
	if req.DryRun {
		return dryRunDecision(decision), nil
	}
	decision, err := s.recordDecision(ctx, req, decision, state, now)
	if err != nil {
		return WakeDecision{}, err
	}
	reservedPromptID := decision.SyntheticPromptID
	promptResult, promptErr := s.prompter.PromptHeartbeatWake(ctx, SyntheticWakePromptRequest{
		SessionID:            req.SessionID,
		Message:              heartbeatWakePrompt(envelope),
		TurnID:               decision.SyntheticPromptID,
		WakeEventID:          decision.WakeEventID,
		PolicySnapshotID:     snapshot.ID,
		PolicyDigest:         snapshot.Digest,
		ConfigDigest:         snapshot.ConfigDigest,
		Summary:              envelope.Summary,
		SyntheticCorrelation: req.SyntheticCorrelation,
	})
	if promptErr != nil {
		result := WakeResultFailed
		reason := WakeReasonSyntheticPromptFailed
		if errors.Is(promptErr, ErrSyntheticPromptBusy) {
			result = WakeResultSkipped
			reason = WakeReasonSessionPromptRace
		}
		decision = s.newDecision(result, reason, snapshot.ID, snapshot.Digest, snapshot.ConfigDigest)
		decision.Diagnostics = []Diagnostic{sanitizeWakeDiagnostic(Diagnostic{
			Code:     string(reason),
			Severity: diagnosticWarning,
			Message:  promptErr.Error(),
		})}
		decision.SyntheticPromptID = reservedPromptID
		return s.appendDecisionEvent(ctx, req, decision, now)
	}
	if promptID := strings.TrimSpace(promptResult.SyntheticPromptID); promptID != "" {
		decision.SyntheticPromptID = promptID
	}
	return decision, nil
}

func (s *ManagedWakeService) loadWakePolicy(
	ctx context.Context,
	req WakeRequest,
) (Snapshot, SnapshotEnvelope, WakeDecision, bool, error) {
	snapshot, err := s.store.GetLatestValidHeartbeatSnapshot(ctx, req.WorkspaceID, req.AgentName)
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			decision := s.newDecision(WakeResultSkipped, WakeReasonHeartbeatNoPolicy, "", "", "")
			return Snapshot{}, SnapshotEnvelope{}, decision, false, nil
		}
		return Snapshot{}, SnapshotEnvelope{}, WakeDecision{}, false, fmt.Errorf(
			"heartbeat: load latest valid snapshot for %q/%q: %w",
			req.WorkspaceID,
			req.AgentName,
			err,
		)
	}
	envelope, err := snapshot.ResolvedEnvelope()
	if err != nil {
		decision := s.newDecision(
			WakeResultSkipped,
			WakeReasonHeartbeatInvalid,
			snapshot.ID,
			snapshot.Digest,
			snapshot.ConfigDigest,
		)
		decision.Diagnostics = []Diagnostic{sanitizeWakeDiagnostic(Diagnostic{
			Code:     "heartbeat_snapshot_invalid",
			Severity: diagnosticError,
			Message:  err.Error(),
		})}
		return snapshot, SnapshotEnvelope{}, decision, false, nil
	}
	if !envelope.Valid {
		decision := s.newDecision(
			WakeResultSkipped,
			WakeReasonHeartbeatInvalid,
			snapshot.ID,
			snapshot.Digest,
			snapshot.ConfigDigest,
		)
		decision.Diagnostics = cloneDiagnostics(envelope.Diagnostics)
		return snapshot, envelope, decision, false, nil
	}
	if !envelope.Present {
		decision := s.newDecision(WakeResultSkipped, WakeReasonHeartbeatNoPolicy, "", "", "")
		return snapshot, envelope, decision, false, nil
	}
	return snapshot, envelope, WakeDecision{}, true, nil
}

func (s *ManagedWakeService) ensureCurrentConfigDigest(envelope *SnapshotEnvelope, snapshot Snapshot) error {
	provenance, err := ConfigProvenanceFor(s.config)
	if err != nil {
		return err
	}
	current := strings.TrimSpace(provenance.Digest)
	envelopeDigest := ""
	if envelope != nil {
		envelopeDigest = envelope.ConfigProvenance.Digest
	}
	stored := strings.TrimSpace(firstNonEmpty(snapshot.ConfigDigest, envelopeDigest))
	if current == "" || stored == "" || current == stored {
		return nil
	}
	return fmt.Errorf("heartbeat policy config digest is stale: current %s differs from snapshot %s", current, stored)
}
