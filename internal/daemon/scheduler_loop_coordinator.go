package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	taskpkg "github.com/compozy/agh/internal/task"
)

func (s schedulerTaskSource) RunLoopCoordinatorBackstop(
	ctx context.Context,
	now time.Time,
	actor taskpkg.ActorContext,
) (int, error) {
	if s.coordinatorBackstop != nil {
		return s.coordinatorBackstop.RunLoopCoordinatorBackstop(ctx, now, actor)
	}
	if err := s.enqueueWatchEventsGapWakes(ctx, actor.Origin, now); err != nil {
		return 0, err
	}
	scopes, err := s.loopCoordinatorClaimScopes(ctx)
	if err != nil {
		return 0, err
	}
	started := 0
	for _, scope := range scopes {
		startedForScope := 0
		for startedForScope < defaultLoopCoordinatorBackstopLimit {
			claim, err := s.manager.ClaimNextRun(ctx, taskpkg.ClaimCriteria{
				Scope:            scope.scope,
				WorkspaceID:      scope.workspaceID,
				RunKind:          taskpkg.RunKindCoordinator,
				ClaimerSessionID: "daemon-loop-coordinator",
				ClaimedBy:        &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "loop-coordinator"},
				LeaseDuration:    taskpkg.DefaultRunLeaseDuration,
				Now:              now,
			}, actor)
			if errors.Is(err, taskpkg.ErrNoClaimableRun) ||
				errors.Is(err, taskpkg.ErrWorkspaceActiveRunCapReached) {
				break
			}
			if err != nil {
				return started, err
			}
			startKey := strings.TrimSpace(claim.Run.IdempotencyKey)
			if startKey == "" {
				startKey = "coordinator-start:" + claim.Run.ID
			}
			if _, err := s.manager.StartRun(ctx, claim.Run.ID, taskpkg.StartRun{
				IdempotencyKey: startKey,
				ClaimToken:     claim.ClaimToken,
			}, actor); err != nil {
				return started, err
			}
			started++
			startedForScope++
		}
	}
	return started, nil
}

type loopCoordinatorClaimScope struct {
	scope       taskpkg.Scope
	workspaceID string
}

func (s schedulerTaskSource) loopCoordinatorClaimScopes(ctx context.Context) ([]loopCoordinatorClaimScope, error) {
	runs, err := s.store.ListTaskRunsByStatus(ctx, []taskpkg.RunStatus{taskpkg.TaskRunStatusQueued})
	if err != nil {
		return nil, err
	}
	scopes := make([]loopCoordinatorClaimScope, 0, len(runs))
	seen := make(map[string]struct{}, len(runs))
	for _, run := range runs {
		if run.RunKind.Normalize() != taskpkg.RunKindCoordinator {
			continue
		}
		taskRecord, err := s.store.GetTask(ctx, run.TaskID)
		if err != nil {
			return nil, fmt.Errorf("daemon: load coordinator task %q for run %q: %w", run.TaskID, run.ID, err)
		}
		scope := loopCoordinatorClaimScope{
			scope:       taskRecord.Scope.Normalize(),
			workspaceID: strings.TrimSpace(taskRecord.WorkspaceID),
		}
		if scope.scope == "" {
			scope.scope = taskpkg.ScopeGlobal
		}
		key := string(scope.scope) + "\x00" + scope.workspaceID
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		scopes = append(scopes, scope)
	}
	return scopes, nil
}
