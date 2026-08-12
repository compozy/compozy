package core

import (
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/network/participation"
	taskpkg "github.com/compozy/compozy/internal/task"
)

// TaskRunSummaryPayloadFromSummary converts one operator-facing run summary into the shared payload.
func TaskRunSummaryPayloadFromSummary(summary *taskpkg.RunSummary) *contract.TaskRunSummaryPayload {
	if summary == nil {
		return nil
	}

	return &contract.TaskRunSummaryPayload{
		ID:                           summary.ID,
		TaskID:                       summary.TaskID,
		Status:                       summary.Status,
		Attempt:                      summary.Attempt,
		RecoveryCount:                summary.RecoveryCount,
		PreviousRunID:                summary.PreviousRunID,
		FailureKind:                  summary.FailureKind,
		MaxAttempts:                  summary.MaxAttempts,
		SessionID:                    summary.SessionID,
		WorktreeID:                   summary.WorktreeID,
		ResolvedWorktreeMode:         summary.ResolvedWorktreeMode,
		ResolvedWorktreeRef:          summary.ResolvedWorktreeRef,
		ClaimedBy:                    cloneActorIdentity(summary.ClaimedBy),
		ClaimTokenHash:               summary.ClaimTokenHash,
		LeaseUntil:                   optionalTime(summary.LeaseUntil),
		HeartbeatAt:                  optionalTime(summary.HeartbeatAt),
		ResolvedNetworkParticipation: resolvedParticipationFromRunSummary(summary),
		DesignationGroupID:           summary.DesignationGroupID,
		Designation:                  cloneRunDesignationSummary(summary.Designation),
		QueuedAt:                     summary.QueuedAt,
		ClaimedAt:                    optionalTime(summary.ClaimedAt),
		StartedAt:                    optionalTime(summary.StartedAt),
		EndedAt:                      optionalTime(summary.EndedAt),
		Error:                        taskpkg.RedactClaimTokens(summary.Error),
	}
}

func resolvedParticipationFromRunSummary(summary *taskpkg.RunSummary) *participation.Spec {
	if summary == nil || summary.ResolvedNetworkParticipation == nil {
		return nil
	}
	return cloneResolvedParticipation(summary.ResolvedNetworkParticipation)
}

func cloneResolvedParticipation(spec *participation.Spec) *participation.Spec {
	if spec == nil {
		return nil
	}
	return participation.CloneSpec(*spec)
}
