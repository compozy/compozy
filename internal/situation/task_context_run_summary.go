package situation

import (
	"strings"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

func runSummaryFromTaskRun(run taskpkg.Run, maxAttempts int) taskpkg.RunSummary {
	summary := taskpkg.RunSummary{
		ID:                           strings.TrimSpace(run.ID),
		TaskID:                       strings.TrimSpace(run.TaskID),
		Status:                       run.Status.Normalize(),
		Attempt:                      int(run.Attempt),
		RecoveryCount:                int(run.RecoveryCount),
		MaxAttempts:                  maxAttempts,
		SessionID:                    strings.TrimSpace(run.SessionID),
		ClaimedBy:                    cloneActorIdentity(run.ClaimedBy),
		ClaimTokenHash:               strings.TrimSpace(run.ClaimTokenHash),
		LeaseUntil:                   run.LeaseUntil.UTC(),
		HeartbeatAt:                  run.HeartbeatAt.UTC(),
		ResolvedNetworkParticipation: participation.CloneSpec(run.NetworkSpecSnapshot()),
		DesignationGroupID:           strings.TrimSpace(run.DesignationGroupID),
		QueuedAt:                     run.QueuedAt.UTC(),
		ClaimedAt:                    run.ClaimedAt.UTC(),
		StartedAt:                    run.StartedAt.UTC(),
		EndedAt:                      run.EndedAt.UTC(),
		Error:                        safeTaskContextText(run.Error, maxReviewReasonFallback),
	}
	if designation, ok := taskpkg.DesignationFromRun(run); ok {
		summary.DesignationGroupID = designation.GroupID
		summary.Designation = designation.Summary()
	}
	return summary
}
