package daemon

import (
	"context"

	"errors"
	"fmt"
	"os"

	"strings"
	"syscall"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/google/uuid"
)

func advanceRestartOperation(
	current RestartOperation,
	transition restartTransition,
	now time.Time,
) (RestartOperation, error) {
	nextStatus := transition.status
	if nextStatus == "" {
		return RestartOperation{}, errors.New("daemon: restart transition status is required")
	}
	if current.terminal() {
		return RestartOperation{}, fmt.Errorf(
			"%w: %s -> %s",
			errInvalidRestartTransition,
			current.Status,
			nextStatus,
		)
	}
	if !restartTransitionAllowed(current.Status, nextStatus) {
		return RestartOperation{}, fmt.Errorf(
			"%w: %s -> %s",
			errInvalidRestartTransition,
			current.Status,
			nextStatus,
		)
	}

	next := current
	next.Status = nextStatus
	next.UpdatedAt = now

	switch nextStatus {
	case RestartStatusFailed:
		reason := strings.TrimSpace(transition.failureReason)
		if reason == "" {
			return RestartOperation{}, errors.New("daemon: failed restart transition requires failure reason")
		}
		completedAt := now
		next.FailureReason = reason
		next.NewPID = 0
		next.CompletedAt = &completedAt
	case RestartStatusReady:
		if transition.newPID <= 0 {
			return RestartOperation{}, errors.New("daemon: ready restart transition requires new pid")
		}
		completedAt := now
		next.NewPID = transition.newPID
		next.FailureReason = ""
		next.CompletedAt = &completedAt
	default:
		next.NewPID = 0
		next.FailureReason = ""
		next.CompletedAt = nil
	}

	if err := next.Validate(); err != nil {
		return RestartOperation{}, err
	}
	return next, nil
}

func restartTransitionAllowed(current RestartStatus, next RestartStatus) bool {
	switch current {
	case RestartStatusPending:
		return next == RestartStatusStopping || next == RestartStatusFailed
	case RestartStatusStopping:
		return next == RestartStatusWaitingRelease || next == RestartStatusFailed
	case RestartStatusWaitingRelease:
		return next == RestartStatusStarting || next == RestartStatusFailed
	case RestartStatusStarting:
		return next == RestartStatusReady || next == RestartStatusFailed
	default:
		return false
	}
}

// RequestRestart creates the persisted restart operation, launches the detached relaunch helper,
// and signals the running daemon to enter its normal graceful shutdown path.
func (d *Daemon) RequestRestart(ctx context.Context) (RestartOperation, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	runtime, err := d.restartRequestRuntime()
	if err != nil {
		return RestartOperation{}, err
	}
	store := newRestartStore(d.homePaths, runtime.now)
	operation, err := store.Create(runtime.newOperation())
	if err != nil {
		return RestartOperation{}, err
	}

	if err := runtime.launchRelaunchHelper(ctx, d.homePaths, operation.OperationID); err != nil {
		return failRestartOperation(store, operation, "spawn relaunch helper", err)
	}

	operation, err = store.Transition(operation.OperationID, restartTransition{status: RestartStatusStopping})
	if err != nil {
		return RestartOperation{}, fmt.Errorf("daemon: mark restart operation stopping: %w", err)
	}

	if err := runtime.signalProcess(operation.OldPID, syscall.SIGTERM); err != nil {
		return failRestartOperation(store, operation, "signal daemon shutdown", err)
	}

	return operation, nil
}

func (d *Daemon) restartRequestRuntime() (restartRequestRuntime, error) {
	d.mu.Lock()
	now := d.now
	startedAt := d.startedAt
	cfg := d.config
	sessions := d.sessions
	pidFn := d.pid
	executable := d.executable
	startDetached := d.startDetached
	signalProcess := d.signalProcess
	d.mu.Unlock()

	if startedAt.IsZero() || pidFn == nil {
		return restartRequestRuntime{}, errRestartNotRunning
	}
	if executable == nil {
		return restartRequestRuntime{}, errors.New("daemon: executable resolver is required")
	}
	if startDetached == nil {
		return restartRequestRuntime{}, errors.New("daemon: detached launcher is required")
	}
	if signalProcess == nil {
		return restartRequestRuntime{}, errors.New("daemon: signal function is required")
	}

	socketPath := strings.TrimSpace(cfg.Daemon.Socket)
	if socketPath == "" {
		socketPath = d.homePaths.DaemonSocket
	}

	activeSessions := 0
	if sessions != nil {
		activeSessions = len(sessions.List())
	}

	return restartRequestRuntime{
		now:           now,
		startedAt:     startedAt,
		socketPath:    socketPath,
		activeSession: activeSessions,
		pid:           pidFn(),
		executable:    executable,
		startDetached: startDetached,
		signalProcess: signalProcess,
	}, nil
}

func (r restartRequestRuntime) newOperation() RestartOperation {
	return RestartOperation{
		OperationID:        uuid.NewString(),
		Status:             RestartStatusPending,
		OldPID:             r.pid,
		OldStartedAt:       r.startedAt,
		OldSocketPath:      r.socketPath,
		ActiveSessionCount: r.activeSession,
	}
}

func (r restartRequestRuntime) launchRelaunchHelper(
	ctx context.Context,
	homePaths aghconfig.HomePaths,
	operationID string,
) error {
	binary, err := r.executable()
	if err != nil {
		return fmt.Errorf("resolve relaunch helper executable: %w", err)
	}

	_, err = r.startDetached(ctx, detachedStartRequest{
		binary:  binary,
		args:    []string{harnessSummaryDefaultAgentName, "relaunch"},
		sandbox: withRestartOperationEnv(os.Environ(), operationID),
		logPath: homePaths.LogFile,
	})
	if err != nil {
		return fmt.Errorf("spawn relaunch helper: %w", err)
	}
	return nil
}

func failRestartOperation(
	store *restartStore,
	operation RestartOperation,
	action string,
	err error,
) (RestartOperation, error) {
	if err == nil {
		return operation, nil
	}

	failed, transitionErr := store.Transition(operation.OperationID, restartTransition{
		status:        RestartStatusFailed,
		failureReason: fmt.Sprintf("%s: %v", action, err),
	})
	if transitionErr == nil {
		operation = failed
		return operation, fmt.Errorf("daemon: %s: %w", action, err)
	}
	return operation, errors.Join(
		fmt.Errorf("daemon: %s: %w", action, err),
		fmt.Errorf("daemon: persist failed restart operation %q: %w", operation.OperationID, transitionErr),
	)
}

// GetRestartOperation reads one persisted restart operation by id.
func (d *Daemon) GetRestartOperation(_ context.Context, operationID string) (RestartOperation, error) {
	return newRestartStore(d.homePaths, d.now).Get(operationID)
}
