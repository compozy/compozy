package core

import (
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

// TaskRunPayloadFromRun converts one task run into the shared payload.
func TaskRunPayloadFromRun(run *taskpkg.Run) contract.TaskRunPayload {
	if run == nil {
		return contract.TaskRunPayload{}
	}

	networkSpec := run.NetworkSpecSnapshot()
	var designation *taskpkg.RunDesignationSummary
	if runDesignation, ok := taskpkg.DesignationFromRun(*run); ok {
		designation = runDesignation.Summary()
	}
	return contract.TaskRunPayload{
		ID:                           run.ID,
		TaskID:                       run.TaskID,
		Status:                       run.Status,
		Attempt:                      int(run.Attempt),
		RecoveryCount:                int(run.RecoveryCount),
		PreviousRunID:                run.PreviousRunID,
		FailureKind:                  run.FailureKind,
		ClaimedBy:                    cloneActorIdentity(run.ClaimedBy),
		SessionID:                    run.SessionID,
		Origin:                       run.Origin,
		IdempotencyKey:               run.IdempotencyKey,
		ResolvedNetworkParticipation: participation.CloneSpec(networkSpec),
		DesignationGroupID:           run.DesignationGroupID,
		Designation:                  designation,
		ClaimTokenHash:               run.ClaimTokenHash,
		LeaseUntil:                   optionalTime(run.LeaseUntil),
		HeartbeatAt:                  optionalTime(run.HeartbeatAt),
		QueuedAt:                     run.QueuedAt,
		ClaimedAt:                    optionalTime(run.ClaimedAt),
		StartedAt:                    optionalTime(run.StartedAt),
		EndedAt:                      optionalTime(run.EndedAt),
		Error:                        taskpkg.RedactClaimTokens(run.Error),
		Metadata:                     taskpkg.RedactClaimTokenJSON(run.Metadata),
		Result:                       taskpkg.RedactClaimTokenJSON(run.Result),
	}
}
