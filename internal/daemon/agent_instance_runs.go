package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/agentinstance"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store/globaldb"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

func reconcileInstanceState(
	ctx context.Context,
	state *bootState,
	repo *globaldb.InstanceRepo,
	instance agentinstance.Instance,
) error {
	if instance.ArchivedAt != nil {
		return nil
	}
	var failures []error
	if instance.State == string(looppkg.StatusPaused) {
		if err := cancelInstanceCheck(ctx, state, instance.ID); err != nil {
			failures = append(failures, err)
		}
	}
	actor, err := taskpkg.DeriveDaemonActorContext("agent_instance:"+instance.ID, "")
	if err != nil {
		return err
	}
	after := ""
	for {
		runs, err := repo.ManagedRuns(ctx, instance.ID, after)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, run := range runs {
			after = run.ID
			if err := reconcileInstanceRun(ctx, state, instance, run, actor); err != nil {
				failures = append(failures, fmt.Errorf("reconcile instance run %s: %w", run.ID, err))
			}
		}
		if len(runs) < 100 {
			break
		}
	}
	return errors.Join(
		append(
			failures,
			repo.ForgetSettledRuns(ctx, instance.ID),
			releaseInstanceInputs(ctx, state, repo, instance),
		)...)
}

func reconcileInstanceRun(ctx context.Context, state *bootState, instance agentinstance.Instance,
	run globaldb.InstanceManagedRun, actor taskpkg.ActorContext) error {
	pause := instance.State == string(looppkg.StatusPaused) && run.Status == string(looppkg.StatusRunning) &&
		!run.PauseRequested
	resume := instance.State == nativeFilterActiveValue && run.Recorded &&
		(run.Status == string(looppkg.StatusPaused) ||
			(run.Status == string(looppkg.StatusRunning) && run.PauseRequested))
	if !pause && !resume {
		return nil
	}
	if state.deps.Loops == nil {
		return errors.New("instance run controls unavailable")
	}
	workspace := run.WorkspaceID
	if workspace == "" {
		workspace = workspaceaccess.UserSelector
	}
	if pause {
		return state.deps.Loops.PauseLoopRun(ctx, workspace, run.ID, actor)
	}
	err := state.deps.Loops.ResumeLoopRun(ctx, workspace, run.ID, actor)
	if err != nil && run.Status == string(looppkg.StatusRunning) && errors.Is(err, looppkg.ErrTransitionConflict) {
		// The run is still settling the pause this instance requested. It stays recorded, so instance
		// maintenance resumes it once paused instead of failing the already committed instance resume.
		return nil
	}
	return err
}

func releaseInstanceInputs(
	ctx context.Context,
	state *bootState,
	repo *globaldb.InstanceRepo,
	instance agentinstance.Instance,
) error {
	if instance.State != nativeFilterActiveValue &&
		(instance.State != string(looppkg.StatusPaused) || agentinstance.OperatorOnlyResume(instance.PauseReason)) {
		return nil
	}
	releaser, ok := state.sessions.(interface {
		ReleaseInstanceInputs(context.Context, string) error
	})
	if !ok {
		return errors.New("instance input release unavailable")
	}
	after := ""
	var failures []error
	for {
		sessions, err := repo.QueuedSessions(ctx, instance.ID, after)
		if err != nil {
			return errors.Join(append(failures, err)...)
		}
		for _, id := range sessions {
			after = id
			if err := releaser.ReleaseInstanceInputs(ctx, id); err != nil {
				failures = append(failures, err)
			}
		}
		if len(sessions) < 100 {
			return errors.Join(failures...)
		}
	}
}
