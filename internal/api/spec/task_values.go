package spec

import (
	"github.com/compozy/agh/internal/api/contract"

	taskpkg "github.com/compozy/agh/internal/task"
)

func taskScopeValues() []string {
	return []string{
		string(taskpkg.ScopeGlobal),
		string(taskpkg.ScopeWorkspace),
	}
}

func taskCatalogScopeValues() []string {
	return []string{
		string(taskpkg.CatalogScopeAll),
		string(taskpkg.CatalogScopeGlobal),
		string(taskpkg.CatalogScopeWorkspace),
	}
}

func taskCatalogSortValues() []string {
	return []string{
		string(taskpkg.CatalogSortRecent),
		string(taskpkg.CatalogSortPriority),
	}
}

func taskStatusValues() []string {
	return []string{
		string(taskpkg.TaskStatusDraft),
		string(taskpkg.TaskStatusPending),
		string(taskpkg.TaskStatusBlocked),
		string(taskpkg.TaskStatusNeedsAttention),
		string(taskpkg.TaskStatusReady),
		string(taskpkg.TaskStatusInProgress),
		string(taskpkg.TaskStatusCompleted),
		string(taskpkg.TaskStatusFailed),
		string(taskpkg.TaskStatusCanceled),
	}
}

func taskPriorityValues() []string {
	return []string{
		string(taskpkg.PriorityLow),
		string(taskpkg.PriorityMedium),
		string(taskpkg.PriorityHigh),
		string(taskpkg.PriorityUrgent),
	}
}

func taskApprovalPolicyValues() []string {
	return []string{
		string(taskpkg.ApprovalPolicyNone),
		string(taskpkg.ApprovalPolicyManual),
	}
}

func taskApprovalStateValues() []string {
	return []string{
		string(taskpkg.ApprovalStateNotRequired),
		string(taskpkg.ApprovalStatePending),
		string(taskpkg.ApprovalStateApproved),
		string(taskpkg.ApprovalStateRejected),
	}
}

func taskRunStatusValues() []string {
	return []string{
		taskpkg.TaskRunStatusQueued.String(),
		taskpkg.TaskRunStatusClaimed.String(),
		taskpkg.TaskRunStatusStarting.String(),
		taskpkg.TaskRunStatusRunning.String(),
		taskpkg.TaskRunStatusCompleted.String(),
		taskpkg.TaskRunStatusFailed.String(),
		taskpkg.TaskRunStatusCanceled.String(),
		taskpkg.TaskRunStatusNeedsAttention.String(),
	}
}

func taskActorKindValues() []string {
	return []string{
		string(taskpkg.ActorKindHuman),
		string(taskpkg.ActorKindAgentSession),
		string(taskpkg.ActorKindAutomation),
		string(taskpkg.ActorKindExtension),
		string(taskpkg.ActorKindNetworkPeer),
		string(taskpkg.ActorKindDaemon),
	}
}

func taskOwnerKindValues() []string {
	return []string{
		string(taskpkg.OwnerKindHuman),
		string(taskpkg.OwnerKindAgentSession),
		string(taskpkg.OwnerKindAutomation),
		string(taskpkg.OwnerKindExtension),
		string(taskpkg.OwnerKindNetworkPeer),
		string(taskpkg.OwnerKindPool),
	}
}

func taskOriginKindValues() []string {
	return []string{
		string(taskpkg.OriginKindCLI),
		string(taskpkg.OriginKindWeb),
		string(taskpkg.OriginKindUDS),
		string(taskpkg.OriginKindHTTP),
		string(taskpkg.OriginKindAutomation),
		string(taskpkg.OriginKindExtension),
		string(taskpkg.OriginKindNetwork),
		string(taskpkg.OriginKindAgentSession),
		string(taskpkg.OriginKindDaemon),
	}
}

func taskDependencyKindValues() []string {
	return []string{
		string(taskpkg.DependencyKindBlocks),
	}
}

func taskBlockKindValues() []string {
	return []string{
		string(taskpkg.BlockKindNeedsInput),
		string(taskpkg.BlockKindCapability),
		string(taskpkg.BlockKindTransient),
	}
}

func taskBlockedSourceValues() []string {
	return []string{
		string(taskpkg.BlockedSourceDependency),
		string(taskpkg.BlockedSourceApproval),
		string(taskpkg.BlockedSourcePaused),
		string(taskpkg.BlockedSourceBlock),
	}
}

func taskCoordinatorModeValues() []string {
	return []string{
		string(taskpkg.CoordinatorModeInherit),
		string(taskpkg.CoordinatorModeGuided),
	}
}

func taskWorkerModeValues() []string {
	return []string{
		string(taskpkg.WorkerModeInherit),
		string(taskpkg.WorkerModeSelect),
	}
}

func taskSandboxModeValues() []string {
	return []string{
		string(taskpkg.SandboxModeInherit),
		string(taskpkg.SandboxModeNone),
		string(taskpkg.SandboxModeRef),
	}
}

func taskRuntimeModeValues() []string {
	return []string{
		string(taskpkg.RuntimeModeDefault),
		string(taskpkg.RuntimeModeEvidence),
	}
}

func taskReviewPolicyValues() []string {
	return []string{
		string(taskpkg.ReviewPolicyNone),
		string(taskpkg.ReviewPolicyAlways),
		string(taskpkg.ReviewPolicyOnSuccess),
		string(taskpkg.ReviewPolicyOnFailure),
	}
}

func taskRunReviewStatusValues() []string {
	return []string{
		string(taskpkg.RunReviewStatusRequested),
		string(taskpkg.RunReviewStatusRouted),
		string(taskpkg.RunReviewStatusInReview),
		string(taskpkg.RunReviewStatusRecorded),
		string(taskpkg.RunReviewStatusCircuitOpened),
		string(taskpkg.RunReviewStatusCanceled),
	}
}

func taskRunReviewOutcomeValues() []string {
	return []string{
		string(taskpkg.RunReviewOutcomeApproved),
		string(taskpkg.RunReviewOutcomeRejected),
		string(taskpkg.RunReviewOutcomeBlocked),
		string(taskpkg.RunReviewOutcomeError),
		string(taskpkg.RunReviewOutcomeTimeout),
		string(taskpkg.RunReviewOutcomeInvalidOutput),
	}
}

func taskInboxLaneValues() []string {
	return []string{
		string(contract.TaskInboxLaneMyWork),
		string(contract.TaskInboxLaneApprovals),
		string(contract.TaskInboxLaneFailedRuns),
		string(contract.TaskInboxLaneBlocked),
		string(contract.TaskInboxLaneArchived),
	}
}

func coordinationMessageKindValues() []string {
	kinds := contract.CoordinationMessageKinds()
	values := make([]string, 0, len(kinds))
	for _, kind := range kinds {
		values = append(values, string(kind))
	}
	return values
}
