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

func (s *daemonLoopAPIService) StopSessionGoals(ctx context.Context, info *session.Info) error {
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
		if err := s.aggregate.CancelRun(
			ctx,
			looppkg.WorkspaceID(info.WorkspaceID),
			row.RunID,
			"Goal session stopped by the operator",
			actor,
		); err != nil {
			errs = append(errs, fmt.Errorf("cancel session Goal: %w", err))
		}
	}
	return errors.Join(errs...)
}
