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
	result, err := claimExactRunResultForTest(ctx, manager, runID, actor)
	if err != nil {
		return nil, err
	}
	return &result.Run, nil
}

func claimExactRunResultForTest(
	ctx context.Context,
	manager *Service,
	runID string,
	actor ActorContext,
) (*ClaimResult, error) {
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
	return result, nil
}

// admitRunDirectlyForTest exercises the production tokenless admission while
// leaving the run claimed for focused lifecycle tests.
func admitRunDirectlyForTest(
	ctx context.Context,
	manager *Service,
	runID string,
	actor ActorContext,
) (*Run, error) {
	run, _, err := manager.loadAuthorizedRunWithTask(ctx, runID, actor)
	if err != nil {
		return nil, err
	}
	admitted, _, err := manager.admitQueuedRunForDirectExecution(ctx, run, actor)
	if err != nil {
		return nil, err
	}
	return &admitted, nil
}
