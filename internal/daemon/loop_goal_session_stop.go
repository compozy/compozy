package daemon

import (
	"context"
	"errors"
	"fmt"

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

var _ session.GoalSessionStopHandler = (*daemonLoopAPIService)(nil)

// StopSessionGoals pauses session-origin Goals before their runtime is stopped.
func (s *daemonLoopAPIService) StopSessionGoals(ctx context.Context, info *session.Info) error {
	return s.controlStoppedSessionGoals(ctx, info, false)
}

// RemoveSessionGoals cancels retained Goals before their origin session is deleted.
func (s *daemonLoopAPIService) RemoveSessionGoals(ctx context.Context, info *session.Info) error {
	return s.controlStoppedSessionGoals(ctx, info, true)
}

func (s *daemonLoopAPIService) controlStoppedSessionGoals(ctx context.Context, info *session.Info, remove bool) error {
	reader, ok := s.persistence.(sessionLoopWorkReader)
	if !ok {
		return errors.New("daemon: session Goal lifecycle reader is unavailable")
	}
	rows, err := reader.ListSessionLoopWork(
		ctx,
		store.ReadScope{ProfileID: info.ProfileID},
		info.WorkspaceID,
		info.ID,
		"",
	)
	if err != nil {
		return err
	}
	actor := taskpkg.ActorContext{
		Actor:  taskpkg.ActorIdentity{Kind: taskpkg.ActorKindDaemon, Ref: "session-goal-lifecycle"},
		Origin: taskpkg.Origin{Kind: taskpkg.OriginKindDaemon, Ref: "session-goal-lifecycle"},
	}
	var errs []error
	for _, row := range rows {
		if !row.OwnsGoal {
			continue
		}
		var controlErr error
		if remove {
			controlErr = s.aggregate.CancelRun(ctx, looppkg.WorkspaceID(info.WorkspaceID), row.RunID,
				"Goal session removed by the operator", actor)
		} else {
			controlErr = s.aggregate.Pause(ctx, looppkg.WorkspaceID(info.WorkspaceID), row.RunID, actor)
		}
		if controlErr != nil {
			errs = append(errs, fmt.Errorf("settle stopped session Goal: %w", controlErr))
		}
	}
	return errors.Join(errs...)
}
