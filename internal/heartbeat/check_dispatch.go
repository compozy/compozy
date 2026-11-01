package heartbeat

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/agentinstance"
)

func (s *CheckService) dispatch(target CheckTarget, digest CheckDigest, occurrence, source string) {
	ctx, cancel := context.WithTimeout(s.ctx, s.config.Timeout)
	defer cancel()
	outcome, err := s.dispatcher.Dispatch(ctx, target, digest, func(sessionID string) error {
		bound, err := s.store.BindTriage(ctx, target.Instance.ID, occurrence, sessionID)
		if err != nil {
			return err
		}
		if !bound {
			return errors.New("heartbeat: triage occurrence cancelled before dispatch")
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
		outcome = failedTriage(reason)
		outcome.DeniedCalls = deniedCalls
	}
	now := s.now()
	req := FinishRequest{
		QuietBackoffCap:  s.config.QuietBackoffCap,
		FatigueLimit:     int64(s.config.FatigueLimit),
		Interval:         s.checkInterval(target.Policy),
		MaxInterval:      s.config.MaxInterval,
		InstanceID:       target.Instance.ID,
		OccurrenceID:     occurrence,
		Source:           source,
		Result:           outcome.Result,
		Reason:           outcome.Reason,
		Detail:           outcome.Detail,
		DeniedCalls:      outcome.DeniedCalls,
		Digest:           digest.Fingerprint,
		AutomationCursor: digest.AutomationCursor,
		Now:              now,
		ExpiresAt:        now.Add(s.config.Retention),
		Report:           outcome.Report,
	}
	settleCtx, settleCancel := context.WithTimeout(context.WithoutCancel(s.ctx), s.config.Timeout)
	defer settleCancel()
	if _, err := s.store.Finish(settleCtx, req); err != nil {
		s.mu.Lock()
		s.pending[occurrence] = req
		s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return errors.New("heartbeat: checks are stopped")
	}
	if s.ready {
		return nil
	}
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
		source := "timer"
		if strings.HasPrefix(state.OccurrenceID, "instance_check-") {
			source = "instance_check"
		}
		outcome := failedTriage("triage_interrupted")
		if _, err := s.store.Finish(ctx, FinishRequest{
			QuietBackoffCap: s.config.QuietBackoffCap,
			FatigueLimit:    int64(s.config.FatigueLimit),
			Interval:        s.config.Interval,
			MaxInterval:     s.config.MaxInterval,
			InstanceID:      state.InstanceID,
			OccurrenceID:    state.OccurrenceID,
			Source:          source,
			Result:          outcome.Result,
			Reason:          outcome.Reason,
			Report:          outcome.Report,
			Now:             now,
			ExpiresAt:       now.Add(s.config.Retention),
		}); err != nil {
			return err
		}
	}
	s.ready = true
	return nil
}
