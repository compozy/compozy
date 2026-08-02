package globaldb

import (
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func forceReleaseClaimedTaskRunParams(fence taskpkg.RunMutationFence) sqlcgen.ForceReleaseClaimedTaskRunParams {
	return sqlcgen.ForceReleaseClaimedTaskRunParams{
		ID:                     fence.RunID(),
		ExpectedTaskID:         nullableTaskString(fence.TaskID()),
		ExpectedWorkspaceID:    nullableTaskString(fence.WorkspaceID()),
		ExpectedSessionID:      nullableTaskString(fence.SessionID()),
		ExpectedClaimTokenHash: nullableTaskString(fence.ClaimTokenHash()),
		ExpectedLeaseUntil:     nullableTaskTime(fence.LeaseUntil()),
	}
}

func forceFailEligibleTaskRunParams(
	fence taskpkg.RunMutationFence,
	next taskpkg.Run,
) sqlcgen.ForceFailEligibleTaskRunParams {
	return sqlcgen.ForceFailEligibleTaskRunParams{
		EndedAt:                nullableTaskTime(next.EndedAt),
		Error:                  nullableTaskString(next.Error),
		ID:                     fence.RunID(),
		ExpectedTaskID:         nullableTaskString(fence.TaskID()),
		ExpectedWorkspaceID:    nullableTaskString(fence.WorkspaceID()),
		ExpectedStatus:         fence.Status().String(),
		ExpectedSessionID:      nullableTaskString(fence.SessionID()),
		ExpectedClaimTokenHash: nullableTaskString(fence.ClaimTokenHash()),
		ExpectedLeaseUntil:     nullableTaskTime(fence.LeaseUntil()),
	}
}

func failNeedsAttentionTaskRunForRecoveryParams(
	fence taskpkg.RunMutationFence,
	next taskpkg.Run,
) sqlcgen.FailNeedsAttentionTaskRunForRecoveryParams {
	return sqlcgen.FailNeedsAttentionTaskRunForRecoveryParams{
		EndedAt:                nullableTaskTime(next.EndedAt),
		Error:                  nullableTaskString(next.Error),
		ID:                     fence.RunID(),
		ExpectedTaskID:         nullableTaskString(fence.TaskID()),
		ExpectedWorkspaceID:    nullableTaskString(fence.WorkspaceID()),
		ExpectedSessionID:      nullableTaskString(fence.SessionID()),
		ExpectedClaimTokenHash: nullableTaskString(fence.ClaimTokenHash()),
		ExpectedLeaseUntil:     nullableTaskTime(fence.LeaseUntil()),
	}
}

func completeParentRollupTaskRunParams(
	previous taskpkg.Run,
	completed taskpkg.Run,
) sqlcgen.CompleteParentRollupTaskRunParams {
	return sqlcgen.CompleteParentRollupTaskRunParams{
		ResultJson:             nullableTaskRawJSON(completed.Result),
		EndedAt:                nullableTaskTime(completed.EndedAt),
		ID:                     previous.ID,
		ExpectedTaskID:         nullableTaskString(previous.TaskID),
		ExpectedWorkspaceID:    nullableTaskString(previous.WorkspaceID),
		ExpectedSessionID:      nullableTaskString(previous.SessionID),
		ExpectedClaimTokenHash: nullableTaskString(previous.ClaimTokenHash),
		ExpectedLeaseUntil:     nullableTaskTime(previous.LeaseUntil),
	}
}
