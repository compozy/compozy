package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	compozyconfig "github.com/compozy/compozy/internal/config"
	compozydaemon "github.com/compozy/compozy/internal/daemon"

	"github.com/compozy/compozy/internal/procutil"
	compozyupdate "github.com/compozy/compozy/internal/update"

	"github.com/spf13/cobra"
)

const (
	daemonStartedValue = "Started"
	daemonStatusValue  = "Status"
	versionValue       = "Version"
	daemonDaemonKey    = "daemon"
	daemonDisabledKey  = "disabled"
	daemonStartKey     = "start"
	daemonStopKey      = "stop"
	daemonStartedAtKey = "started_at"
	daemonStatusKey    = "status"
	versionKey         = "version"
)

const internalChildFlagName = "internal-child"
const exitWhenOrphanedFlagName = "exit-when-orphaned"

type daemonProcess interface {
	PID() int
	Done() <-chan struct{}
	Wait() error
	Terminate() error
}

func newDaemonCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:               daemonDaemonKey,
		Short:             "Manage the CompozyOS daemon",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error { return rejectMachineProfileFlag(cmd) },
	}
	configureMachineProfileFlag(cmd, true)

	cmd.AddCommand(newDaemonStartCommand(deps))
	cmd.AddCommand(newDaemonBootstrapCommand(deps))
	cmd.AddCommand(newDaemonRelaunchCommand(deps))
	cmd.AddCommand(newDaemonUpdateCoordinatorCommand(deps))
	cmd.AddCommand(newDaemonAppTransitionCommand(deps))
	cmd.AddCommand(newDaemonAppDiagnosticBundleCommand(deps))
	cmd.AddCommand(newDaemonStopCommand(deps))
	return cmd
}

func newDaemonUpdateCoordinatorCommand(deps commandDeps) *cobra.Command {
	var operationID string
	var executorGeneration string
	cmd := &cobra.Command{
		Use:    "update-coordinator",
		Short:  "Internal detached update coordinator",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDaemonUpdateCoordinator(cmd.Context(), deps, operationID, executorGeneration)
		},
	}
	cmd.Flags().StringVar(&operationID, "operation-id", "", "Update operation id")
	cmd.Flags().StringVar(&executorGeneration, "executor-generation", "", "Update executor generation")
	mustMarkFlagRequired(cmd, "operation-id")
	mustMarkFlagRequired(cmd, "executor-generation")
	return cmd
}

func runDaemonUpdateCoordinator(
	ctx context.Context,
	deps commandDeps,
	operationID string,
	executorGeneration string,
) error {
	manager, homePaths, err := resolveUpdateManager(deps)
	if err != nil {
		return err
	}
	operation, err := manager.OperationStore().Read(ctx)
	if err != nil {
		return err
	}
	if operation == nil || operation.ID != strings.TrimSpace(operationID) || operation.Holder == nil ||
		operation.Holder.ExecutorGeneration != strings.TrimSpace(executorGeneration) {
		return compozyupdate.ErrExecutorFenced
	}
	startedAt, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		return fmt.Errorf("cli: resolve detached update executor identity: %w", err)
	}
	holder := *operation.Holder
	holder.PID = os.Getpid()
	holder.PIDStartTime = startedAt
	holder.Surface = compozyupdate.ActorDaemon
	holder.LeaseExpiresAt = deps.now().UTC().Add(compozyupdate.DefaultLeaseDuration)
	transitionKind := compozyupdate.TransitionRenewLease
	fenceGeneration := executorGeneration
	if !manager.OperationStore().HolderLive(operation.Holder) {
		transitionKind = compozyupdate.TransitionAcquireLease
		fenceGeneration = ""
	}
	operation, err = manager.OperationStore().Transition(
		ctx,
		operation.ID,
		fenceGeneration,
		operation.Revision,
		compozyupdate.Transition{
			Kind: transitionKind, Actor: compozyupdate.ActorDaemon, Target: operation.ActiveTarget,
			Holder: &holder, Percent: operation.Percent,
		},
	)
	if err != nil {
		return err
	}
	runtime := newCLIUpdateRuntime(deps, homePaths)
	coordinator, err := compozyupdate.NewCoordinator(compozyupdate.CoordinatorConfig{
		Store: manager.OperationStore(), ReleaseManager: manager, BinaryManager: manager, Runtime: runtime,
	})
	if err != nil {
		return err
	}
	return coordinator.Run(ctx, operation.ID, holder.ExecutorGeneration)
}

func newDaemonStartCommand(deps commandDeps) *cobra.Command {
	var (
		foreground       bool
		internalChild    bool
		exitWhenOrphaned bool
	)

	cmd := &cobra.Command{
		Use:   daemonStartKey,
		Short: "Start the CompozyOS daemon",
		Example: `  # Start CompozyOS in the background and wait for readiness
  compozy daemon start

  # Keep logs attached to the current terminal
  compozy daemon start --foreground`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if internalChild {
				return runDaemonForegroundChild(cmd.Context(), deps, exitWhenOrphaned)
			}
			if foreground {
				return runDaemonForeground(cmd.Context(), deps, exitWhenOrphaned)
			}
			status, err := runDaemonDetached(cmd.Context(), deps)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, daemonStatusBundle(status, deps.now))
		},
	}
	cmd.Flags().BoolVar(&foreground, "foreground", false, "Run the daemon in the foreground")
	cmd.Flags().BoolVar(&internalChild, internalChildFlagName, false, "Internal detached child mode")
	mustMarkFlagHidden(cmd, internalChildFlagName)
	cmd.Flags().BoolVar(
		&exitWhenOrphaned,
		exitWhenOrphanedFlagName,
		false,
		"Exit when the parent process exits (test harness use)",
	)
	mustMarkFlagHidden(cmd, exitWhenOrphanedFlagName)
	return cmd
}

func newDaemonRelaunchCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:    "relaunch",
		Short:  "Internal daemon relaunch helper",
		Hidden: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			homePaths, err := deps.resolveHome()
			if err != nil {
				return err
			}

			return deps.runRelaunchHelper(cmd.Context(), &compozydaemon.RelaunchHelperConfig{
				HomePaths:   homePaths,
				OperationID: strings.TrimSpace(os.Getenv(compozydaemon.RestartOperationEnvKey)),
				Executable:  deps.executable,
				Sandbox:     os.Environ(),
			})
		},
	}
}

func newDaemonStopCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   daemonStopKey,
		Short: "Stop the CompozyOS daemon",
		Example: `  # Ask the running daemon to stop
  compozy daemon stop`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			runtime, err := loadRuntimeContext(deps)
			if err != nil {
				return err
			}

			info, running, err := daemonInfo(runtime.HomePaths, deps)
			if err != nil {
				return err
			}
			if !running {
				return errors.New("cli: daemon is not running")
			}

			if err := deps.signalProcess(info.PID, syscall.SIGTERM); err != nil {
				return err
			}

			status, err := waitForDaemonStop(cmd.Context(), deps, runtime, info)
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, daemonStatusBundle(status, deps.now))
		},
	}
}

func runDaemonForeground(ctx context.Context, deps commandDeps, exitWhenOrphaned bool) error {
	return runDaemonForegroundMode(ctx, deps, exitWhenOrphaned, true)
}

func runDaemonForegroundChild(ctx context.Context, deps commandDeps, exitWhenOrphaned bool) error {
	return runDaemonForegroundMode(ctx, deps, exitWhenOrphaned, false)
}

func runDaemonForegroundMode(
	ctx context.Context,
	deps commandDeps,
	exitWhenOrphaned bool,
	acquireUpdateLock bool,
) (returnErr error) {
	deps = deps.withDaemonRuntimeDefaults()
	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return err
	}
	if err := deps.ensureHome(runtime.HomePaths); err != nil {
		return err
	}
	refreshDesktopOwnedRuntimePathBeforeStart(ctx, deps, runtime.HomePaths)
	var updateLock *compozydaemon.UpdateLock
	if acquireUpdateLock {
		updateLock, err = acquireDaemonStartUpdateLock(daemonStartUpdateLockPath(runtime.HomePaths))
		if err != nil {
			return err
		}
		defer func() {
			if updateLock != nil {
				returnErr = errors.Join(returnErr, updateLock.Release())
			}
		}()
	}

	if _, running, err := daemonInfo(runtime.HomePaths, deps); err != nil {
		return err
	} else if running {
		return errors.New("cli: daemon already running")
	}

	runner, err := deps.newDaemon()
	if err != nil {
		return err
	}
	if updateLock != nil {
		if err := updateLock.Release(); err != nil {
			return err
		}
		updateLock = nil
	}

	// A harness-launched daemon self-terminates gracefully when its launcher
	// dies abruptly (SIGKILL / test timeout), which would otherwise orphan it.
	if exitWhenOrphaned {
		watchCtx, cancel := context.WithCancel(ctx)
		var watchers sync.WaitGroup
		watchers.Go(func() {
			procutil.WatchParentExit(watchCtx, 0, cancel)
		})
		defer func() {
			cancel()
			watchers.Wait()
		}()
		ctx = watchCtx
	}
	return runner.Run(ctx)
}

func runDaemonDetached(ctx context.Context, deps commandDeps) (
	status DaemonStatus,
	returnErr error,
) {
	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return DaemonStatus{}, err
	}
	if err := deps.ensureHome(runtime.HomePaths); err != nil {
		return DaemonStatus{}, err
	}
	updateLock, err := acquireDaemonStartUpdateLock(daemonStartUpdateLockPath(runtime.HomePaths))
	if err != nil {
		return DaemonStatus{}, err
	}
	defer func() {
		returnErr = errors.Join(returnErr, updateLock.Release())
	}()

	if info, running, err := daemonInfo(runtime.HomePaths, deps); err != nil {
		return DaemonStatus{}, err
	} else if running {
		return DaemonStatus{}, fmt.Errorf("cli: daemon already running (pid=%d)", info.PID)
	}

	child, err := deps.spawnDetached(ctx, runtime.HomePaths)
	if err != nil {
		return DaemonStatus{}, err
	}
	if child == nil {
		return DaemonStatus{}, errors.New("cli: detached daemon process is required")
	}

	status, err = waitForDaemonStart(ctx, deps, child)
	if err != nil {
		return DaemonStatus{}, err
	}
	return status, nil
}

func daemonStartUpdateLockPath(homePaths compozyconfig.HomePaths) string {
	return filepath.Join(homePaths.HomeDir, compozyconfig.UpdateLockName)
}

func acquireDaemonStartUpdateLock(path string) (*compozydaemon.UpdateLock, error) {
	startedAt, err := procutil.StartedAt(os.Getpid())
	if err != nil {
		return nil, fmt.Errorf("cli: resolve daemon start identity: %w", err)
	}
	lock, err := compozydaemon.AcquireUpdateLock(path, compozydaemon.UpdateLockOwner{
		PID:       os.Getpid(),
		StartedAt: startedAt,
	})
	if err != nil {
		return nil, fmt.Errorf("cli: acquire daemon start mutation lock: %w", err)
	}
	return lock, nil
}

func waitForDaemonStart(ctx context.Context, deps commandDeps, child daemonProcess) (DaemonStatus, error) {
	if err := requirePollingContext(ctx); err != nil {
		return DaemonStatus{}, err
	}
	if child == nil {
		return DaemonStatus{}, errors.New("cli: detached daemon process is required")
	}
	childPID := child.PID()
	if childPID <= 0 {
		return DaemonStatus{}, errors.New("cli: detached daemon process pid is required")
	}
	client, err := clientFromDeps(deps)
	if err != nil {
		return DaemonStatus{}, err
	}

	processAlive := deps.processAlive
	if processAlive == nil {
		processAlive = procutil.Alive
	}

	return pollUntil(
		ctx,
		deps.startTimeout,
		deps.pollInterval,
		child.Done(),
		"cli: daemon did not become ready before timeout",
		func(pollCtx context.Context, event pollEvent) (DaemonStatus, bool, error) {
			if event == pollEventInterrupt {
				if err := child.Wait(); err != nil {
					return DaemonStatus{}, true, fmt.Errorf("cli: detached daemon exited before readiness: %w", err)
				}
				return DaemonStatus{}, true, errors.New("cli: detached daemon exited before readiness")
			}

			status, statusErr := client.DaemonStatus(pollCtx)
			if statusErr == nil && status.PID == childPID && strings.TrimSpace(status.Status) == daemonRunningStatus {
				return status, true, nil
			}
			if !processAlive(childPID) {
				return DaemonStatus{}, true, errors.New("cli: detached daemon exited before readiness")
			}
			return DaemonStatus{}, false, nil
		},
	)
}
