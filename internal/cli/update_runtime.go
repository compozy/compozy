package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"
	"github.com/compozy/compozy/internal/procutil"
	compozyupdate "github.com/compozy/compozy/internal/update"
)

type cliUpdateRuntime struct {
	deps      commandDeps
	homePaths compozyconfig.HomePaths
}

var _ compozyupdate.CoordinatorRuntime = (*cliUpdateRuntime)(nil)

func newCLIUpdateRuntime(deps commandDeps, homePaths compozyconfig.HomePaths) *cliUpdateRuntime {
	return &cliUpdateRuntime{deps: deps, homePaths: homePaths}
}

func newUpdateHolder(surface compozyupdate.Actor, now time.Time) (compozyupdate.Holder, error) {
	startedAt, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		return compozyupdate.Holder{}, fmt.Errorf("cli: resolve update executor identity: %w", err)
	}
	generation, err := compozyupdate.NewExecutorGeneration()
	if err != nil {
		return compozyupdate.Holder{}, err
	}
	return compozyupdate.Holder{
		PID: os.Getpid(), PIDStartTime: startedAt, Surface: surface,
		ExecutorGeneration: generation, LeaseExpiresAt: now.UTC().Add(compozyupdate.DefaultLeaseDuration),
	}, nil
}

func (r *cliUpdateRuntime) AcquireMutationLock(context.Context) (compozyupdate.MutationLock, error) {
	startedAt, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		return nil, fmt.Errorf("cli: resolve update mutation identity: %w", err)
	}
	lock, err := compozydaemon.AcquireUpdateLock(
		daemonStartUpdateLockPath(r.homePaths),
		compozydaemon.UpdateLockOwner{PID: os.Getpid(), StartedAt: startedAt},
	)
	if err != nil {
		return nil, err
	}
	return lock, nil
}

func (r *cliUpdateRuntime) RestartDaemon(ctx context.Context) error {
	_, running, err := daemonInfo(r.homePaths, r.deps)
	if err != nil {
		return err
	}
	if !running {
		_, err = runDaemonDetached(ctx, r.deps)
		return err
	}
	client, err := clientFromDeps(r.deps)
	if err != nil {
		return err
	}
	action, err := client.TriggerSettingsRestart(ctx)
	if err != nil {
		return err
	}
	_, err = waitForSettingsRestart(ctx, r.deps, client, action.OperationID)
	return err
}

func (r *cliUpdateRuntime) HealthCheck(ctx context.Context) error {
	client, err := clientFromDeps(r.deps)
	if err != nil {
		return err
	}
	status, err := client.DaemonStatus(ctx)
	if err != nil {
		return err
	}
	if status.PID <= 0 || strings.TrimSpace(status.Status) != daemonRunningStatus {
		return fmt.Errorf("cli: replacement daemon is not healthy (status=%q pid=%d)", status.Status, status.PID)
	}
	return nil
}

func (r *cliUpdateRuntime) InstalledApp(ctx context.Context) (compozyupdate.InstalledApp, error) {
	if r.deps.resolveAppInstallation == nil {
		return compozyupdate.InstalledApp{}, errors.New("cli: app installation resolver is required")
	}
	installation, err := r.deps.resolveAppInstallation(ctx, r.homePaths)
	if err != nil {
		return compozyupdate.InstalledApp{}, err
	}
	return compozyupdate.InstalledApp{
		Present: installation.Installed,
		Version: strings.TrimSpace(installation.Version),
	}, nil
}

func waitForAppUpdateCompletion(
	ctx context.Context,
	deps commandDeps,
	store *compozyupdate.OperationStore,
	operationID string,
	deadline time.Time,
) (*compozyupdate.Operation, error) {
	if store == nil {
		return nil, errors.New("cli: update operation store is required")
	}
	timeout := time.Until(deadline)
	if deps.now != nil {
		timeout = deadline.Sub(deps.now())
	}
	return pollUntil(
		ctx,
		timeout,
		deps.pollInterval,
		nil,
		"cli: desktop app update did not complete before its deadline",
		func(pollCtx context.Context, _ pollEvent) (*compozyupdate.Operation, bool, error) {
			if archived, done, err := readArchivedAppCompletion(pollCtx, store, operationID); done || err != nil {
				return archived, done, err
			}
			live, err := store.Read(pollCtx)
			if err != nil {
				return nil, true, err
			}
			if live == nil || live.ID != operationID {
				if archived, done, archiveErr := readArchivedAppCompletion(
					pollCtx,
					store,
					operationID,
				); done || archiveErr != nil {
					return archived, done, archiveErr
				}
				return nil, true, errors.New("cli: update operation disappeared before archival")
			}
			return nil, false, nil
		},
	)
}

func readArchivedAppCompletion(
	ctx context.Context,
	store *compozyupdate.OperationStore,
	operationID string,
) (*compozyupdate.Operation, bool, error) {
	archived, err := store.ReadArchived(ctx, operationID)
	if err != nil {
		return nil, true, err
	}
	if archived == nil {
		return nil, false, nil
	}
	return archivedAppCompletion(archived)
}

func archivedAppCompletion(operation *compozyupdate.Operation) (*compozyupdate.Operation, bool, error) {
	if operation.App != nil && operation.App.Phase == compozyupdate.PhaseFailed {
		message := strings.TrimSpace(operation.LastError)
		if message == "" {
			message = "desktop app update failed"
		}
		return operation, true, errors.New("cli: " + message)
	}
	return operation, true, nil
}
