package globaldb

import (
	"database/sql"
	"strings"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/agh/internal/task"
)

func forceTaskRunSnapshotParams(
	previous taskpkg.Run,
	next taskpkg.Run,
) (sqlcgen.ForceUpdateTaskRunSnapshotParams, error) {
	lineage := taskRunReviewLineage(next)
	network, err := encodeParticipationSnapshot(next.WorkspaceID, next.NetworkSpecSnapshot())
	if err != nil {
		return sqlcgen.ForceUpdateTaskRunSnapshotParams{}, err
	}
	wakeID, targetSessionID, ownerKey := next.NetworkWakeCorrelation()
	return sqlcgen.ForceUpdateTaskRunSnapshotParams{
		TaskID:                 nullableTaskString(next.TaskID),
		WorkspaceID:            nullableTaskString(next.WorkspaceID),
		Status:                 next.Status.String(),
		Attempt:                int64(next.Attempt),
		PreviousRunID:          nullableTaskString(next.PreviousRunID),
		FailureKind:            strings.TrimSpace(next.FailureKind),
		ClaimedByKind:          nullableTaskActorKind(next.ClaimedBy),
		ClaimedByRef:           nullableTaskActorRef(next.ClaimedBy),
		SessionID:              nullableTaskString(next.SessionID),
		OriginKind:             string(next.Origin.Kind),
		OriginRef:              next.Origin.Ref,
		IdempotencyKey:         nullableTaskString(next.IdempotencyKey),
		NetworkSpecJson:        network.JSON,
		NetworkMode:            network.Mode,
		NetworkChannel:         network.Channel,
		NetworkSource:          network.Source,
		ClaimTokenHash:         nullableTaskString(next.ClaimTokenHash),
		LeaseUntil:             nullableTaskTime(next.LeaseUntil),
		HeartbeatAt:            nullableTaskTime(next.HeartbeatAt),
		QueuedAt:               store.FormatTimestamp(next.QueuedAt),
		ClaimedAt:              nullableTaskTime(next.ClaimedAt),
		StartedAt:              nullableTaskTime(next.StartedAt),
		EndedAt:                nullableTaskTime(next.EndedAt),
		Error:                  nullableTaskString(next.Error),
		MetadataJson:           nullableTaskRawJSON(next.Metadata),
		ResultJson:             nullableTaskRawJSON(next.Result),
		ReviewRequired:         lineage.Required,
		ReviewRequestRound:     int64(lineage.RequestRound),
		ReviewPolicySnapshot:   string(lineage.PolicySnapshot),
		ReviewRequestID:        nullableTaskString(lineage.RequestID),
		ParentRunID:            nullableTaskString(lineage.ParentRunID),
		ReviewID:               nullableTaskString(lineage.ReviewID),
		ReviewRound:            int64(lineage.ReviewRound),
		ContinuationReason:     lineage.ContinuationReason,
		MissingWorkJson:        string(lineage.MissingWork),
		NextRoundGuidance:      lineage.NextRoundGuidance,
		NetworkWakeID:          nullableTaskString(wakeID),
		NetworkTargetSessionID: nullableTaskString(targetSessionID),
		NetworkOwnerKey:        nullableTaskString(ownerKey),
		ID:                     next.ID,
		PreviousStatus:         previous.Status.Normalize().String(),
		PreviousSessionID:      sql.NullString{String: strings.TrimSpace(previous.SessionID), Valid: true},
		PreviousClaimTokenHash: sql.NullString{String: strings.TrimSpace(previous.ClaimTokenHash), Valid: true},
		PreviousLeaseUntil:     sql.NullString{String: forceRunCASTimestamp(previous.LeaseUntil), Valid: true},
	}, nil
}
