package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (h *BaseHandlers) agentTaskClaimCriteria(
	ctx context.Context,
	req contract.AgentTaskClaimNextRequest,
	caller agentidentity.Caller,
) (taskpkg.ClaimCriteria, error) {
	workspaceID := strings.TrimSpace(req.WorkspaceID)
	callerWorkspaceID := strings.TrimSpace(caller.Session.WorkspaceID)
	if workspaceID == "" {
		workspaceID = callerWorkspaceID
	}
	if callerWorkspaceID == "" && workspaceID != "" {
		return taskpkg.ClaimCriteria{}, fmt.Errorf(
			"%w: agent session has no workspace for workspace task claims",
			taskpkg.ErrPermissionDenied,
		)
	}
	if callerWorkspaceID != "" && workspaceID != callerWorkspaceID {
		return taskpkg.ClaimCriteria{}, fmt.Errorf(
			"%w: agent session %q cannot claim workspace %q",
			taskpkg.ErrPermissionDenied,
			caller.Session.ID,
			workspaceID,
		)
	}

	leaseDuration, err := agentTaskLeaseDuration(req.LeaseSeconds)
	if err != nil {
		return taskpkg.ClaimCriteria{}, err
	}
	capabilities, err := h.agentTaskClaimCapabilities(ctx, req.RequiredCapabilities, caller)
	if err != nil {
		return taskpkg.ClaimCriteria{}, err
	}

	return taskpkg.ClaimCriteria{
		RunID:            strings.TrimSpace(req.RunID),
		WorkspaceID:      workspaceID,
		ClaimerSessionID: strings.TrimSpace(caller.Session.ID),
		ClaimedBy: &taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindAgentSession,
			Ref:  strings.TrimSpace(caller.Session.ID),
		},
		AgentName:            strings.TrimSpace(caller.Session.AgentName),
		RequiredCapabilities: capabilities,
		PriorityMin:          req.PriorityMin,
		CallerNetworkParticipation: participation.CloneSpec(
			caller.Session.NetworkSpecSnapshot(),
		),
		Soul:          soulClaimProvenanceFromCaller(caller),
		LeaseDuration: leaseDuration,
	}, nil
}

func soulClaimProvenanceFromCaller(caller agentidentity.Caller) *taskpkg.SoulClaimProvenance {
	digest := strings.TrimSpace(caller.Session.SoulDigest)
	if digest == "" {
		return nil
	}
	return &taskpkg.SoulClaimProvenance{
		SnapshotID: strings.TrimSpace(caller.Session.SoulSnapshotID),
		Digest:     digest,
		AgentName:  strings.TrimSpace(caller.Session.AgentName),
	}
}

func (h *BaseHandlers) agentTaskClaimCapabilities(
	ctx context.Context,
	requested []string,
	caller agentidentity.Caller,
) ([]string, error) {
	capabilities := append([]string(nil), requested...)
	if h.AgentContextService == nil {
		return capabilities, nil
	}
	contextPayload, err := h.AgentContextService.ContextForSession(ctx, sessionInfoFromAgentCaller(caller))
	if err != nil {
		return nil, err
	}
	for _, capability := range contextPayload.Capabilities.Capabilities {
		if id := strings.TrimSpace(capability.ID); id != "" {
			capabilities = append(capabilities, id)
		}
	}
	return capabilities, nil
}
