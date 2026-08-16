package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/procutil"
	compozyupdate "github.com/compozy/compozy/internal/update"
	"github.com/compozy/compozy/internal/version"
)

type settingsUpdateController struct {
	manager settingsUpdateManager
	spawn   func(context.Context, *compozyupdate.Operation) error
	holder  func(compozyupdate.Actor) (compozyupdate.Holder, error)
}

var _ core.SettingsUpdateController = settingsUpdateController{}

type settingsUpdateManager interface {
	Snapshot(context.Context) (compozyupdate.MultiState, error)
	PlanOperation(context.Context, compozyupdate.Actor, []compozyupdate.Target, compozyupdate.Holder) (
		compozyupdate.OperationRequest,
		error,
	)
	AcquireOperation(context.Context, compozyupdate.OperationRequest) (*compozyupdate.Operation, error)
	OperationStore() *compozyupdate.OperationStore
}

func (c settingsUpdateController) GetUpdate(ctx context.Context) (compozyupdate.MultiState, error) {
	if c.manager == nil {
		return compozyupdate.MultiState{}, errors.New("daemon: settings update manager is required")
	}
	return c.manager.Snapshot(ctx)
}

func (c settingsUpdateController) ApplyUpdate(
	ctx context.Context,
	target compozyupdate.Target,
) (core.SettingsUpdateApply, error) {
	if c.manager == nil || c.spawn == nil || c.holder == nil {
		return core.SettingsUpdateApply{}, errors.New("daemon: settings update apply is not configured")
	}
	holder, err := c.holder(compozyupdate.ActorDaemon)
	if err != nil {
		return core.SettingsUpdateApply{}, err
	}
	request, err := c.manager.PlanOperation(ctx, compozyupdate.ActorWeb, []compozyupdate.Target{target}, holder)
	if err != nil {
		return core.SettingsUpdateApply{
			Target: target, Status: compozyupdate.ApplyStatusFailed, Message: err.Error(),
		}, err
	}
	operation, err := c.manager.AcquireOperation(ctx, request)
	if err != nil {
		var blocked *compozyupdate.BlockedError
		if errors.As(err, &blocked) {
			return core.SettingsUpdateApply{
				Target: target, Status: compozyupdate.ApplyStatusBlocked,
				OperationID: blocked.Operation.ID, Message: "An update operation is already active.",
				Holder: blocked.Operation.Holder,
			}, nil
		}
		return core.SettingsUpdateApply{
			Target: target, Status: compozyupdate.ApplyStatusFailed, Message: err.Error(),
		}, err
	}
	if err := c.spawn(ctx, operation); err != nil {
		_, transitionErr := c.manager.OperationStore().Transition(
			ctx, operation.ID, operation.Holder.ExecutorGeneration, operation.Revision,
			compozyupdate.Transition{
				Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorDaemon,
				Target: target, Phase: compozyupdate.PhaseFailed, Percent: -1,
				LastError: err.Error(), Outcome: string(compozyupdate.StatusFailed),
			},
		)
		return core.SettingsUpdateApply{
			Target: target, Status: compozyupdate.ApplyStatusFailed, OperationID: operation.ID,
			Message: err.Error(),
		}, errors.Join(err, transitionErr)
	}
	return core.SettingsUpdateApply{
		Target: target, Status: compozyupdate.ApplyStatusAccepted, OperationID: operation.ID,
		Message: "Update accepted.",
	}, nil
}

func (c settingsUpdateController) CancelUpdate(ctx context.Context) (core.SettingsUpdateCancel, error) {
	if c.manager == nil {
		return core.SettingsUpdateCancel{}, errors.New("daemon: settings update manager is required")
	}
	operation, err := c.manager.OperationStore().Read(ctx)
	if err != nil {
		return core.SettingsUpdateCancel{}, err
	}
	if operation == nil {
		return core.SettingsUpdateCancel{
			Status: compozyupdate.StatusCanceled, Message: "No update operation is active.",
		}, nil
	}
	canceled, err := c.manager.OperationStore().Transition(
		ctx, operation.ID, "", operation.Revision,
		compozyupdate.Transition{
			Kind: compozyupdate.TransitionCancel, Actor: compozyupdate.ActorWeb,
			Target: operation.ActiveTarget, Percent: -1, Outcome: string(compozyupdate.StatusCanceled),
		},
	)
	if errors.Is(err, compozyupdate.ErrOperationNotCancelable) || errors.Is(err, compozyupdate.ErrExecutorFenced) {
		return core.SettingsUpdateCancel{
			Status: compozyupdate.StatusBlocked, OperationID: operation.ID,
			Message: "The update operation has a live executor and was not canceled.", Holder: operation.Holder,
		}, nil
	}
	if err != nil {
		return core.SettingsUpdateCancel{}, err
	}
	return core.SettingsUpdateCancel{
		Status: compozyupdate.StatusCanceled, OperationID: canceled.ID,
		Message: "Canceled dormant update operation; the update channel is free.",
	}, nil
}

func newSettingsUpdateManager(d *Daemon) (*compozyupdate.Manager, error) {
	if d == nil {
		return nil, errors.New("daemon: settings update daemon is required")
	}
	return compozyupdate.NewManager(&compozyupdate.Config{
		HomePaths:       d.homePaths,
		CurrentVersion:  version.Current().Version,
		ExecutablePath:  d.executable,
		Getenv:          os.Getenv,
		OperationEvents: compozyupdate.NewSlogOperationEventEmitter(d.logger),
	})
}

func newSettingsUpdateController(d *Daemon, manager settingsUpdateManager) settingsUpdateController {
	return settingsUpdateController{
		manager: manager,
		holder:  newDaemonUpdateHolder,
		spawn: func(ctx context.Context, operation *compozyupdate.Operation) error {
			return spawnDetachedUpdateCoordinator(ctx, d, operation)
		},
	}
}

func newDaemonUpdateHolder(surface compozyupdate.Actor) (compozyupdate.Holder, error) {
	startedAt, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		return compozyupdate.Holder{}, fmt.Errorf("daemon: resolve update executor identity: %w", err)
	}
	generation, err := compozyupdate.NewExecutorGeneration()
	if err != nil {
		return compozyupdate.Holder{}, err
	}
	return compozyupdate.Holder{
		PID: os.Getpid(), PIDStartTime: startedAt, Surface: surface,
		ExecutorGeneration: generation, LeaseExpiresAt: time.Now().UTC().Add(2 * time.Minute),
	}, nil
}

func spawnDetachedUpdateCoordinator(
	ctx context.Context,
	d *Daemon,
	operation *compozyupdate.Operation,
) error {
	if d == nil || operation == nil || operation.Holder == nil {
		return errors.New("daemon: update coordinator operation is required")
	}
	binary, err := d.executable()
	if err != nil {
		return fmt.Errorf("daemon: resolve update coordinator executable: %w", err)
	}
	process, err := d.startDetached(ctx, detachedStartRequest{
		binary: binary,
		args: []string{
			string(compozyupdate.ActorDaemon), "update-coordinator",
			"--operation-id", operation.ID,
			"--executor-generation", operation.Holder.ExecutorGeneration,
		},
		sandbox: os.Environ(),
		logPath: d.homePaths.LogFile,
	})
	if err != nil {
		return fmt.Errorf("daemon: spawn update coordinator: %w", err)
	}
	if process == nil || process.PID() <= 0 {
		return errors.New("daemon: detached update coordinator process is invalid")
	}
	return nil
}
