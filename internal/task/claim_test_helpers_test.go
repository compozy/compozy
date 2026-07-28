package task

import (
	"context"
	"strings"
)

func claimExactRunForTest(
	ctx context.Context,
	manager *Service,
	runID string,
	actor ActorContext,
) (*Run, error) {
	run, err := manager.store.GetTaskRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	taskRecord, err := manager.store.GetTask(ctx, run.TaskID)
	if err != nil {
		return nil, err
	}

	claimerSessionID := "test-claim:" + run.ID
	agentName := ""
	if actor.Actor.Kind.Normalize() == ActorKindAgentSession {
		claimerSessionID = strings.TrimSpace(actor.Actor.Ref)
		agentName = strings.TrimSpace(actor.Actor.Ref)
	}
	if taskRecord.Owner != nil {
		switch taskRecord.Owner.Kind.Normalize() {
		case OwnerKindAgentSession:
			claimerSessionID = strings.TrimSpace(taskRecord.Owner.Ref)
		case OwnerKindPool:
			agentName = strings.TrimSpace(taskRecord.Owner.Ref)
		}
	}

	result, err := manager.ClaimNextRun(ctx, ClaimCriteria{
		RunID:                run.ID,
		WorkspaceID:          taskRecord.WorkspaceID,
		ClaimerSessionID:     claimerSessionID,
		AgentName:            agentName,
		RequiredCapabilities: append([]string(nil), run.RequiredCapabilities...),
	}, actor)
	if err != nil {
		return nil, err
	}
	return &result.Run, nil
}

// seedNonLeasedClaimedRunForTest prepares the internal precondition owned by
// StartRun, attach, and boot-recovery tests. Exact agent claims are lease-bound
// and deliberately use claimExactRunForTest instead.
func seedNonLeasedClaimedRunForTest(
	ctx context.Context,
	manager *Service,
	runID string,
	actor ActorContext,
) (*Run, error) {
	run, err := manager.store.GetTaskRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	run.Status = TaskRunStatusClaimed
	run.ClaimedBy = cloneActorIdentity(&actor.Actor)
	run.ClaimedAt = manager.now().UTC()
	run.SessionID = ""
	run.ClaimTokenHash = ""
	if err := manager.store.UpdateTaskRun(ctx, run); err != nil {
		return nil, err
	}
	return &run, nil
}
