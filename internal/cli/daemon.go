package cli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"strings"
	"sync"
	"syscall"
	"time"

	aghdaemon "github.com/compozy/agh/internal/daemon"

	"github.com/compozy/agh/internal/procutil"

	"github.com/spf13/cobra"
)

const (
	daemonStartedValue = "Started"
	daemonStatusValue  = "Status"
	versionValue       = "Version"
	daemonDaemonKey    = "daemon"
	daemonDisabledKey  = "disabled"
	daemonStartKey     = "start"
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
}

func newDaemonCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   daemonDaemonKey,
		Short: "Manage the AGH daemon",
	}

	cmd.AddCommand(newDaemonStartCommand(deps))
	cmd.AddCommand(newDaemonRelaunchCommand(deps))
	cmd.AddCommand(newDaemonStopCommand(deps))
	return cmd
}

func newDaemonStartCommand(deps commandDeps) *cobra.Command {
	var (
		foreground       bool
		internalChild    bool
		exitWhenOrphaned bool
	)

	cmd := &cobra.Command{
		Use:   daemonStartKey,
		Short: "Start the AGH daemon",
		Example: `  # Start AGH in the background and wait for readiness
  agh daemon start

  # Keep logs attached to the current terminal
  agh daemon start --foreground`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if foreground || internalChild {
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

			return deps.runRelaunchHelper(cmd.Context(), aghdaemon.RelaunchHelperConfig{
				HomePaths:   homePaths,
				OperationID: strings.TrimSpace(os.Getenv(aghdaemon.RestartOperationEnvKey)),
				Executable:  deps.executable,
				Sandbox:     os.Environ(),
			})
		},
	}
}

func newDaemonStopCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the AGH daemon",
		Example: `  # Ask the running daemon to stop
  agh daemon stop`,
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
	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return err
	}
	if err := deps.ensureHome(runtime.HomePaths); err != nil {
		return err
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

func runDaemonDetached(ctx context.Context, deps commandDeps) (DaemonStatus, error) {
	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return DaemonStatus{}, err
	}
	if err := deps.ensureHome(runtime.HomePaths); err != nil {
		return DaemonStatus{}, err
	}

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

	status, err := waitForDaemonStart(ctx, deps, child)
	if err != nil {
		return DaemonStatus{}, err
	}
	return status, nil
}

func waitForDaemonStart(ctx context.Context, deps commandDeps, child daemonProcess) (DaemonStatus, error) {
	if child == nil {
		return DaemonStatus{}, errors.New("cli: detached daemon process is required")
	}
	childPID := child.PID()
	if childPID <= 0 {
		return DaemonStatus{}, errors.New("cli: detached daemon process pid is required")
	}
	waitCtx := ctx
	if waitCtx == nil {
		waitCtx = context.Background()
	}
	if _, hasDeadline := waitCtx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(waitCtx, deps.startTimeout)
		defer cancel()
	}

	client, err := clientFromDeps(deps)
	if err != nil {
		return DaemonStatus{}, err
	}

	processAlive := deps.processAlive
	if processAlive == nil {
		processAlive = procutil.Alive
	}

	ticker := time.NewTicker(deps.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-child.Done():
			if err := child.Wait(); err != nil {
				return DaemonStatus{}, fmt.Errorf("cli: detached daemon exited before readiness: %w", err)
			}
			return DaemonStatus{}, errors.New("cli: detached daemon exited before readiness")
		case <-waitCtx.Done():
			return DaemonStatus{}, fmt.Errorf("cli: daemon did not become ready before timeout: %w", waitCtx.Err())
		case <-ticker.C:
			status, statusErr := client.DaemonStatus(waitCtx)
			if statusErr == nil && status.PID == childPID {
				return status, nil
			}
			if !processAlive(childPID) {
				return DaemonStatus{}, errors.New("cli: detached daemon exited before readiness")
			}
		}
	}
}
