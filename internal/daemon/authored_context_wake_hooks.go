package daemon

import (
	"context"
	"errors"

	"strings"

	core "github.com/compozy/agh/internal/api/core"

	"github.com/compozy/agh/internal/heartbeat"
	hookspkg "github.com/compozy/agh/internal/hooks"

	"github.com/compozy/agh/internal/soul"
)

func hookHeartbeatWakeService(next core.HeartbeatWakeService, hooks *hooksNotifier) core.HeartbeatWakeService {
	if next == nil {
		return nil
	}
	return hookedHeartbeatWakeService{next: next, hooks: hooks}
}

type hookedHeartbeatWakeService struct {
	next  core.HeartbeatWakeService
	hooks *hooksNotifier
}

func (s hookedHeartbeatWakeService) Wake(
	ctx context.Context,
	req heartbeat.WakeRequest,
) (heartbeat.WakeDecision, error) {
	if s.next == nil {
		return heartbeat.WakeDecision{}, errors.New("daemon: heartbeat wake service is not configured")
	}
	s.dispatchWakeBefore(ctx, req)
	decision, err := s.next.Wake(ctx, req)
	if err == nil {
		s.dispatchWakeAfter(ctx, req, decision)
	}
	return decision, err
}

func (s hookedHeartbeatWakeService) dispatchWakeBefore(ctx context.Context, req heartbeat.WakeRequest) {
	if s.hooks == nil {
		return
	}
	payload := hookspkg.AgentHeartbeatWakeBeforePayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookAgentHeartbeatWakeBefore,
			Timestamp: s.hooks.timestamp(),
		},
		SessionContext: hookspkg.SessionContext{
			SessionID:   strings.TrimSpace(req.SessionID),
			AgentName:   strings.TrimSpace(req.AgentName),
			WorkspaceID: strings.TrimSpace(req.WorkspaceID),
		},
		Source: string(req.Source),
		DryRun: req.DryRun,
	}
	if _, err := s.hooks.DispatchAgentHeartbeatWakeBefore(ctx, payload); err != nil {
		logAuthoredContextDependencyError(s.hooks.logger, "daemon: dispatch heartbeat wake before hook", err)
	}
}

func (s hookedHeartbeatWakeService) dispatchWakeAfter(
	ctx context.Context,
	req heartbeat.WakeRequest,
	decision heartbeat.WakeDecision,
) {
	if s.hooks == nil {
		return
	}
	payload := hookspkg.AgentHeartbeatWakeAfterPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookAgentHeartbeatWakeAfter,
			Timestamp: s.hooks.timestamp(),
		},
		SessionContext: hookspkg.SessionContext{
			SessionID:   strings.TrimSpace(req.SessionID),
			AgentName:   strings.TrimSpace(req.AgentName),
			WorkspaceID: strings.TrimSpace(req.WorkspaceID),
		},
		WakeEventID:       strings.TrimSpace(decision.WakeEventID),
		Result:            string(decision.Result),
		Reason:            string(decision.Reason),
		PolicySnapshotID:  strings.TrimSpace(decision.PolicySnapshotID),
		PolicyDigest:      strings.TrimSpace(decision.PolicyDigest),
		ConfigDigest:      strings.TrimSpace(decision.ConfigDigest),
		SyntheticPromptID: strings.TrimSpace(decision.SyntheticPromptID),
		Source:            string(req.Source),
	}
	if _, err := s.hooks.DispatchAgentHeartbeatWakeAfter(ctx, payload); err != nil {
		logAuthoredContextDependencyError(s.hooks.logger, "daemon: dispatch heartbeat wake after hook", err)
	}
}

func dispatchHeartbeatPolicyResolved(
	ctx context.Context,
	hooks *hooksNotifier,
	workspaceID string,
	agentName string,
	policy *heartbeat.ResolvedPolicy,
	snapshotID string,
) {
	if hooks == nil || policy == nil {
		return
	}
	payload := hookspkg.AgentHeartbeatPolicyResolvedPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookAgentHeartbeatPolicyResolved,
			Timestamp: hooks.timestamp(),
		},
		AuthoredContextProvenance: hookspkg.AuthoredContextProvenance{
			WorkspaceID:      strings.TrimSpace(workspaceID),
			AgentName:        strings.TrimSpace(agentName),
			SourcePath:       strings.TrimSpace(policy.SourcePath),
			SnapshotID:       strings.TrimSpace(snapshotID),
			Digest:           strings.TrimSpace(policy.Digest),
			ConfigDigest:     strings.TrimSpace(policy.ConfigDigest),
			ValidationStatus: authoredValidationStatus(policy.Present, policy.Active, policy.Valid),
			Valid:            policy.Valid,
			Active:           policy.Active,
			Reason:           firstHeartbeatDiagnosticCode(policy.Diagnostics),
		},
		Summary: strings.TrimSpace(policy.Summary),
	}
	if _, err := hooks.DispatchAgentHeartbeatPolicyResolved(ctx, payload); err != nil {
		logAuthoredContextDependencyError(hooks.logger, "daemon: dispatch heartbeat policy hook", err)
	}
}

func authoredValidationStatus(present bool, active bool, valid bool) string {
	switch {
	case !present:
		return taskRecoverySessionMissing
	case !valid:
		return "invalid"
	case !active:
		return "inactive"
	default:
		return authoredContextRuntimeValidKey
	}
}

func firstSoulDiagnosticCode(diagnostics []soul.Diagnostic) string {
	for _, diagnostic := range diagnostics {
		if code := strings.TrimSpace(diagnostic.Code); code != "" {
			return code
		}
	}
	return ""
}

func firstHeartbeatDiagnosticCode(diagnostics []heartbeat.Diagnostic) string {
	for _, diagnostic := range diagnostics {
		if code := strings.TrimSpace(diagnostic.Code); code != "" {
			return code
		}
	}
	return ""
}
