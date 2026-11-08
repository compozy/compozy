package tools

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
)

type checkInDispatch struct {
	recorder   CheckInRecorder
	policy     *CheckInPolicy
	decision   *CheckInDecision
	descriptor Descriptor
	input      json.RawMessage
	invoked    bool
	attempted  bool
	denyReason string
	started    time.Time
}

func (a *checkInDispatch) begin(ctx context.Context, reason string) error {
	if a == nil || a.policy == nil || a.decision != nil || a.attempted {
		return nil
	}
	a.attempted = true
	if a.recorder == nil {
		return NewToolError(
			ErrorCodeDenied,
			a.descriptor.ID,
			string(ReasonCheckInAuditUnavailable),
			ErrToolDenied,
			ReasonCheckInAuditUnavailable,
		)
	}
	preview := diagnostics.RedactAndBound(string(a.input), 512)
	decision, err := a.recorder.Begin(
		ctx,
		CheckInAttempt{
			OccurrenceID: a.policy.OccurrenceID,
			ToolID:       a.descriptor.ID,
			Backend:      a.descriptor.Backend.Kind,
			MCPServer:    a.descriptor.Backend.MCPServer,
			Access:       AccessOf(a.descriptor),
			DenyReason:   reason,
			ArgsPreview:  preview,
		},
		*a.policy,
	)
	if err != nil {
		return NewToolError(
			ErrorCodeDenied,
			a.descriptor.ID,
			string(ReasonCheckInAuditUnavailable),
			err,
			ReasonCheckInAuditUnavailable,
		)
	}
	a.decision = &decision
	if !decision.Allowed {
		return NewToolError(
			ErrorCodeDenied,
			a.descriptor.ID,
			decision.Reason,
			ErrToolDenied,
			ReasonCode(decision.Reason),
		)
	}
	return nil
}
func (a *checkInDispatch) finish(ctx context.Context, callErr error) error {
	if a == nil || a.policy == nil {
		return nil
	}
	reason := a.denyReason
	if callErr != nil && reason == "" {
		reason = string(ReasonCheckInInputInvalid)
		if r, ok := ReasonOf(callErr); ok {
			reason = string(r)
		}
	}
	if a.decision == nil && !a.attempted {
		if err := a.begin(ctx, reason); err != nil && a.decision == nil {
			return err
		}
	}
	if a.decision == nil {
		return nil
	}
	outcome := "denied"
	if a.invoked {
		outcome = "ok"
		if callErr != nil {
			outcome = "error"
			if errors.Is(callErr, context.Canceled) || errors.Is(callErr, context.DeadlineExceeded) ||
				errors.Is(callErr, ErrToolCanceled) ||
				errors.Is(callErr, ErrToolTimedOut) {
				outcome = "cancelled"
			}
		}
	}
	return a.recorder.Finish(ctx, a.policy.OccurrenceID, a.decision.Seq, outcome, reason, time.Since(a.started))
}
func WithCheckInRecorder(recorder CheckInRecorder) RegistryOption {
	return func(r *RuntimeRegistry) { r.checkInRecorder = recorder }
}
