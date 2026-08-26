package daemon

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"time"

	core "github.com/compozy/compozy/internal/api/core"
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
	targets []compozyupdate.Target,
) (core.SettingsUpdateApply, error) {
	if c.manager == nil || c.spawn == nil || c.holder == nil {
		return core.SettingsUpdateApply{}, errors.New("daemon: settings update apply is not configured")
	}
	holder, err := c.holder(compozyupdate.ActorDaemon)
	if err != nil {
		return core.SettingsUpdateApply{}, err
	}
	requestedTargets := slices.Clone(targets)
	request, err := c.manager.PlanOperation(ctx, compozyupdate.ActorWeb, targets, holder)
	if err != nil {
		return core.SettingsUpdateApply{
			Targets: requestedTargets, Status: compozyupdate.ApplyStatusFailed, Message: err.Error(),
		}, err
	}
	operation, err := c.manager.AcquireOperation(ctx, request)
	if err != nil {
		if blocked, ok := errors.AsType[*compozyupdate.BlockedError](err); ok {
			return core.SettingsUpdateApply{
				Targets: requestedTargets, Status: compozyupdate.ApplyStatusBlocked,
				OperationID: blocked.Operation.ID, Message: "An update operation is already active.",
				Holder: blocked.Operation.Holder,
			}, nil
		}
		return core.SettingsUpdateApply{
			Targets: requestedTargets, Status: compozyupdate.ApplyStatusFailed, Message: err.Error(),
		}, err
	}
	detachedCtx := context.WithoutCancel(ctx)
	if err := c.spawn(detachedCtx, operation); err != nil {
		_, transitionErr := c.manager.OperationStore().Transition(
			detachedCtx, operation.ID, operation.Holder.ExecutorGeneration, operation.Revision,
			compozyupdate.Transition{
				Kind: compozyupdate.TransitionPhase, Actor: compozyupdate.ActorDaemon,
				Target: operation.ActiveTarget, Phase: compozyupdate.PhaseFailed, Percent: -1,
				LastError: err.Error(), Outcome: string(compozyupdate.StatusFailed),
			},
		)
		result := core.SettingsUpdateApply{
			Targets: requestedTargets, Status: compozyupdate.ApplyStatusAccepted, OperationID: operation.ID,
			Message: "Update accepted.",
		}
		if transitionErr != nil {
			return result, errors.Join(err, transitionErr)
		}
		return result, nil
	}
	return core.SettingsUpdateApply{
		Targets: requestedTargets, Status: compozyupdate.ApplyStatusAccepted, OperationID: operation.ID,
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
	if errors.Is(err, compozyupdate.ErrOperationNotCancelable) {
		return core.SettingsUpdateCancel{
			Status: compozyupdate.StatusBlocked, OperationID: operation.ID,
			Message: fmt.Sprintf(
				"The update operation cannot be canceled during the %s phase.",
				settingsUpdateOperationPhase(operation),
			),
			Holder: operation.Holder,
		}, nil
	}
	if errors.Is(err, compozyupdate.ErrExecutorFenced) {
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

func settingsUpdateOperationPhase(operation *compozyupdate.Operation) compozyupdate.OperationPhase {
	if operation == nil {
		return ""
	}
	if operation.ActiveTarget == compozyupdate.TargetRuntime && operation.Runtime != nil {
		return operation.Runtime.Phase
	}
	if operation.ActiveTarget == compozyupdate.TargetApp && operation.App != nil {
		return operation.App.Phase
	}
	return ""
}

func newSettingsUpdateManager(d *Daemon, logger *slog.Logger) (*compozyupdate.Manager, error) {
	if d == nil || logger == nil {
		return nil, errors.New("daemon: settings update daemon is required")
	}
	return compozyupdate.NewManager(&compozyupdate.Config{
		HomePaths:       d.homePaths,
		CurrentVersion:  version.Current().Version,
		ExecutablePath:  d.executable,
		Getenv:          os.Getenv,
		OperationEvents: compozyupdate.NewSlogOperationEventEmitter(logger),
	})
}

func newSettingsUpdateController(d *Daemon, manager settingsUpdateManager) settingsUpdateController {
	return settingsUpdateController{
		manager: manager,
		holder: func(surface compozyupdate.Actor) (compozyupdate.Holder, error) {
			return newDaemonUpdateHolder(d, surface)
		},
		spawn: func(ctx context.Context, operation *compozyupdate.Operation) error {
			return spawnDetachedUpdateCoordinator(context.WithoutCancel(ctx), d, operation)
		},
	}
}

func newDaemonUpdateHolder(d *Daemon, surface compozyupdate.Actor) (compozyupdate.Holder, error) {
	if d == nil || d.pid == nil || d.processStartedAt == nil || d.now == nil {
		return compozyupdate.Holder{}, errors.New("daemon: update executor identity dependencies are required")
	}
	pid := d.pid()
	startedAt, err := d.processStartedAt(pid)
	if err != nil {
		return compozyupdate.Holder{}, fmt.Errorf("daemon: resolve update executor identity: %w", err)
	}
	generation, err := compozyupdate.NewExecutorGeneration()
	if err != nil {
		return compozyupdate.Holder{}, err
	}
	return compozyupdate.Holder{
		PID: pid, PIDStartTime: startedAt, Surface: surface,
		ExecutorGeneration: generation, LeaseExpiresAt: d.now().UTC().Add(2 * time.Minute),
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
