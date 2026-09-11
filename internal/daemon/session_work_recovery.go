package daemon

import (
	"context"

	"github.com/compozy/compozy/internal/session"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func installSupervisedWorkRecovery(state *bootState, tasks *taskpkg.Service) {
	manager, ok := state.sessions.(*session.Manager)
	if !ok {
		return
	}
	manager.SetSupervisedWorkRecovery(func(ctx context.Context, info *session.Info, eventID string) error {
		return tasks.RecoverSupervisedWork(ctx, taskpkg.SupervisedStop{
			SessionID: info.ID, WorkspaceID: info.WorkspaceID, ProfileID: info.ProfileID, EventID: eventID,
		})
	})
}

func (d *Daemon) installBootSupervisedWorkRecovery(state *bootState) error {
	store, ok := taskStoreForBoot(state)
	if !ok {
		return nil
	}
	manager, err := taskpkg.NewManager(taskpkg.WithStore(store), taskpkg.WithManagerNow(d.now))
	if err != nil {
		return err
	}
	installSupervisedWorkRecovery(state, manager)
	return nil
}
