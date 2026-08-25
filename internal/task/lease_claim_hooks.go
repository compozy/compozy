package task

import (
	"context"
	"fmt"
	"strings"

	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/network/participation"
)

func (m *Service) normalizeClaimCriteriaForActor(
	ctx context.Context,
	criteria ClaimCriteria,
	actor ActorContext,
) (ClaimCriteria, error) {
	normalized := criteria
	normalized.WorkspaceActiveRunCap = m.workspaceActiveRunCap
	normalized.GovernedRootActiveRunCap = m.governedRootActiveRunCap
	if strings.TrimSpace(normalized.ClaimerSessionID) == "" && actor.Actor.Kind.Normalize() == ActorKindAgentSession {
		normalized.ClaimerSessionID = strings.TrimSpace(actor.Actor.Ref)
	}
	if normalized.ClaimedBy == nil {
		normalized.ClaimedBy = &ActorIdentity{
			Kind: actor.Actor.Kind.Normalize(),
			Ref:  strings.TrimSpace(actor.Actor.Ref),
		}
	}
	if strings.TrimSpace(normalized.AgentName) == "" && actor.Actor.Kind.Normalize() == ActorKindAgentSession {
		normalized.AgentName = strings.TrimSpace(actor.Actor.Ref)
	}
	var err error
	normalized, err = normalized.Normalize(m.now().UTC())
	if err != nil {
		return ClaimCriteria{}, err
	}
	if actor.Actor.Kind.Normalize() == ActorKindAgentSession &&
		strings.TrimSpace(normalized.ClaimerSessionID) != strings.TrimSpace(actor.Scope.SessionID) {
		return ClaimCriteria{}, fmt.Errorf("%w: claimer session does not match trusted caller", ErrPermissionDenied)
	}
	if actor.Scope.Operator || actor.Actor.Kind.Normalize() == ActorKindDaemon {
		return normalized, nil
	}
	if normalized.Scope.Normalize() == ScopeWorkspace {
		targetWorkspaceID := strings.TrimSpace(normalized.WorkspaceID)
		if targetWorkspaceID != strings.TrimSpace(actor.Scope.WorkspaceID) {
			allowed, authorizeErr := taskWorkspaceAccessAllowed(ctx, m.workspaceAccess, actor, targetWorkspaceID)
			if authorizeErr != nil {
				return ClaimCriteria{}, fmt.Errorf(
					"%w: authorize claim workspace: %w",
					ErrPermissionDenied,
					authorizeErr,
				)
			}
			if !allowed {
				return ClaimCriteria{}, fmt.Errorf(
					"%w: claim workspace does not match trusted caller",
					ErrPermissionDenied,
				)
			}
		}
	}
	return normalized, nil
}

func (m *Service) dispatchTaskRunPreClaimCriteria(
	ctx context.Context,
	criteria ClaimCriteria,
	actor ActorContext,
) (ClaimCriteria, error) {
	taskContext := hookspkg.TaskRunContext{
		ProfileID:       strings.TrimSpace(actor.ReadScope.ProfileID),
		RunID:           strings.TrimSpace(criteria.RunID),
		WorkspaceID:     strings.TrimSpace(criteria.WorkspaceID),
		TargetSessionID: strings.TrimSpace(criteria.TargetSessionID),
		AgentName:       strings.TrimSpace(criteria.AgentName),
		SessionID:       strings.TrimSpace(criteria.ClaimerSessionID),
		ActorKind:       string(actor.Actor.Kind.Normalize()),
		ActorID:         strings.TrimSpace(actor.Actor.Ref),
	}
	if criteria.RunKind.Normalize() != RunKindUnknown {
		runKind := criteria.RunKind.Normalize().String()
		taskContext.RunKind = &runKind
	}
	if criteria.Soul != nil {
		taskContext.SoulSnapshotID = strings.TrimSpace(criteria.Soul.SnapshotID)
		taskContext.SoulDigest = strings.TrimSpace(criteria.Soul.Digest)
	}
	if criteria.CallerNetworkParticipation != nil {
		taskContext.ResolvedNetworkParticipation = participation.CloneSpec(*criteria.CallerNetworkParticipation)
	}
	payload := hookspkg.TaskRunPreClaimPayload{
		PayloadBase: hookspkg.PayloadBase{
			Event:     hookspkg.HookTaskRunPreClaim,
			Timestamp: m.now().UTC(),
		},
		TaskRunContext: &taskContext,
		Criteria: hookspkg.TaskRunClaimCriteria{
			RunID:                criteria.RunID,
			RunKind:              criteria.RunKind.Normalize().String(),
			WorkspaceID:          criteria.WorkspaceID,
			TargetSessionID:      criteria.TargetSessionID,
			ClaimerSessionID:     criteria.ClaimerSessionID,
			AgentName:            criteria.AgentName,
			RequiredCapabilities: append([]string(nil), criteria.RequiredCapabilities...),
			PriorityMin:          criteria.PriorityMin,
		},
	}
	result, err := m.taskHooks.DispatchTaskRunPreClaim(ctx, payload)
	if err != nil {
		return ClaimCriteria{}, err
	}
	if result.Denied {
		reason := strings.TrimSpace(result.DenyReason)
		if reason == "" {
			reason = "task run claim denied by hook"
		}
		return ClaimCriteria{}, fmt.Errorf("%w: %s", ErrPermissionDenied, reason)
	}
	patched := criteria
	patched.RequiredCapabilities = append([]string(nil), result.Criteria.RequiredCapabilities...)
	patched.PriorityMin = result.Criteria.PriorityMin
	return patched.Normalize(m.now().UTC())
}
