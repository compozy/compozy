package heartbeat

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"

	"github.com/compozy/compozy/internal/agentinstance"
)

func (s *CheckService) dispatch(
	ctx context.Context,
	target *CheckTarget,
	digest CheckDigest,
	occurrence, source string,
) {
	defer func() {
		s.mu.Lock()
		run := s.running[target.Instance.ID]
		if run.occurrence == occurrence {
			run.cancel()
			delete(s.running, target.Instance.ID)
		}
		s.mu.Unlock()
	}()
	outcome, err := s.dispatcher.Dispatch(ctx, target, digest, func(sessionID string) error {
		bound, err := s.store.BindTriage(ctx, target.Instance.ID, occurrence, sessionID)
		if err != nil {
			return err
		}
		if !bound {
			return errors.New("heartbeat: triage occurrence canceled before dispatch")
		}
		return nil
	})
	if err != nil {
		reason := "triage_failed"
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			reason = "triage_timeout"
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			reason = "triage_interrupted"
		}
		deniedCalls := outcome.DeniedCalls
		toolsStatus := outcome.ToolsStatus
		outcome = failedTriage(reason)
		outcome.DeniedCalls = deniedCalls
		outcome.ToolsStatus = toolsStatus
	}
	now := s.now()
	req := FinishRequest{
		HasSources:          len(target.Policy.Preferences.Context.Sources) > 0,
		SourceMore:          digest.SourceMore,
		MinInterval:         s.config.MinInterval,
		UnattendedTurnLimit: s.config.UnattendedTurnLimit,
		QuietBackoffCap:     s.config.QuietBackoffCap,
		FatigueLimit:        int64(s.config.FatigueLimit),
		Interval:            s.checkInterval(&target.Policy),
		MaxInterval:         s.config.MaxInterval,
		InstanceID:          target.Instance.ID,
		OccurrenceID:        occurrence,
		Source:              source,
		Result:              outcome.Result,
		Reason:              outcome.Reason,
		Detail:              outcome.Detail,
		ToolsStatus:         outcome.ToolsStatus,
		DeniedCalls:         outcome.DeniedCalls,
		Digest:              digest.Fingerprint,
		AutomationCursor:    digest.AutomationCursor,
		Now:                 now,
		ExpiresAt:           now.Add(s.config.Retention),
		Report:              outcome.Report,
	}
	settleCtx, settleCancel := context.WithTimeout(context.WithoutCancel(s.ctx), s.config.Timeout)
	defer settleCancel()
	if err := s.finish(settleCtx, req); err != nil {
		s.mu.Lock()
		s.pending[occurrence] = req
		s.mu.Unlock()
	}
}

// ReasonTriageConfigUnavailable marks a check whose provider rejected the configured
// triage model or runtime options. It is a configuration diagnostic, not a check failure,
// so it never advances the failure streak that pauses an instance.
const ReasonTriageConfigUnavailable = "triage_config_unavailable"

const maxConfigUnavailableDetailRunes = 2000

// ConfigUnavailableTriage reports a provider configuration rejection with its diagnostic.
func ConfigUnavailableTriage(detail string) TriageOutcome {
	detail = strings.TrimSpace(detail)
	if detail == "" {
		detail = "The provider rejected the heartbeat model or runtime options."
	}
	// Reports are capped at 64 KiB; a provider can list many choices or echo a long error.
	if runes := []rune(detail); len(runes) > maxConfigUnavailableDetailRunes {
		detail = string(runes[:maxConfigUnavailableDetailRunes]) + " (truncated)"
	}
	payload, err := json.Marshal(struct {
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	}{Reason: ReasonTriageConfigUnavailable, Detail: detail})
	if err != nil {
		return failedTriage("triage_failed")
	}
	return TriageOutcome{
		Result: string(WakeResultFailed),
		Reason: ReasonTriageConfigUnavailable,
		Detail: detail,
		Report: &agentinstance.ReportInput{Kind: "failure", Payload: payload},
	}
}

func failedTriage(reason string) TriageOutcome {
	payload := `{"reason":"triage_failed","detail":"The proactive check failed."}`
	switch reason {
	case "triage_output_invalid":
		payload = `{"reason":"triage_output_invalid","detail":"The proactive check returned an invalid response."}`
	case "triage_unsupported":
		payload = `{"reason":"triage_unsupported","detail":"The provider does not support read-only triage."}`
	case "triage_timeout":
		payload = `{"reason":"triage_timeout","detail":"The proactive check timed out."}`
	case "triage_interrupted":
		payload = `{"reason":"triage_interrupted","detail":"The proactive check was interrupted."}`
	}
	return TriageOutcome{
		Result: "failed",
		Reason: reason,
		Report: &agentinstance.ReportInput{Kind: "failure", Payload: []byte(payload)},
	}
}

// Recover runs before traffic and preserves already committed outcomes.
func (s *CheckService) Recover(ctx context.Context) error {
	s.recoveryMu.Lock()
	defer s.recoveryMu.Unlock()
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("heartbeat: checks are stopped")
	}
	if s.ready {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()
	inflight, err := s.store.Inflight(ctx)
	if err != nil {
		return err
	}
	for _, state := range inflight {
		if state.TriageSessionID != "" {
			if err := s.dispatcher.Stop(ctx, state.TriageSessionID); err != nil {
				return err
			}
		}
		now := s.now()
		source := checkTimerSource
		if strings.HasPrefix(state.OccurrenceID, "instance_check-") {
			source = checkManualSource
		}
		// Keep the cadence already committed at admission, including per-agent policy.
		var nextWake time.Time
		if state.NextWakeAt != "" {
			nextWake, err = store.ParseTimestamp(state.NextWakeAt)
			if err != nil {
				return err
			}
		}
		outcome := failedTriage("triage_interrupted")
		if err := s.finish(ctx, FinishRequest{
			UnattendedTurnLimit: s.config.UnattendedTurnLimit,
			QuietBackoffCap:     s.config.QuietBackoffCap,
			FatigueLimit:        int64(s.config.FatigueLimit),
			NextWakeAt:          nextWake,
			InstanceID:          state.InstanceID,
			OccurrenceID:        state.OccurrenceID,
			Source:              source,
			Result:              outcome.Result,
			Reason:              outcome.Reason,
			Report:              outcome.Report,
			Now:                 now,
			ExpiresAt:           now.Add(s.config.Retention),
		}); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.ready = !s.closed
	s.mu.Unlock()
	return nil
}

func (s *CheckService) admitCheck(
	ctx context.Context,
	target *CheckTarget,
	digest CheckDigest,
	request AdmitRequest,
	source string,
) (CheckResult, error) {
	occurrence, err := store.NewID(source)
	if err != nil {
		return CheckResult{}, err
	}
	request.OccurrenceID = occurrence
	request.EventIDs = digest.EventIDs
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return CheckResult{}, errors.New("heartbeat: checks are stopped")
	}
	admitted, err := s.store.Admit(ctx, request)
	if err != nil {
		return CheckResult{}, err
	}
	if !admitted {
		return CheckResult{Gate: checkNoSlot}, nil
	}
	dispatchCtx, cancel := context.WithTimeout(s.ctx, s.config.Timeout)
	s.running[target.Instance.ID] = checkRun{occurrence: occurrence, cancel: cancel}
	s.wg.Go(func() { s.dispatch(dispatchCtx, target, digest, occurrence, source) })
	return CheckResult{OccurrenceID: occurrence}, nil
}

// CancelInstance stops the check owned by the instance after its pause commits.
func (s *CheckService) CancelInstance(ctx context.Context, id string) error {
	s.mu.Lock()
	if cancel := s.collecting[id]; cancel != nil {
		cancel()
	}
	s.mu.Unlock()
	state, err := s.store.WakeState(ctx, id)
	if err != nil {
		return err
	}
	if !state.Canceled || state.OccurrenceID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if run, ok := s.running[id]; ok && run.occurrence == state.OccurrenceID {
		run.cancel()
	}
	return nil
}
