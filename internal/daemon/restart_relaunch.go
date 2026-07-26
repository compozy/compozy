package daemon

import (
	"context"

	"errors"
	"fmt"
	"os"

	"strings"

	"time"

	aghconfig "github.com/compozy/agh/internal/config"

	"github.com/compozy/agh/internal/procutil"
)

// RelaunchHelperConfig configures one internal `agh daemon relaunch` execution.
type RelaunchHelperConfig struct {
	HomePaths      aghconfig.HomePaths
	OperationID    string
	Executable     func() (string, error)
	Sandbox        []string
	PollInterval   time.Duration
	ReleaseTimeout time.Duration
	ReadyTimeout   time.Duration
	ExitDrainWait  time.Duration
}

type relaunchHelper struct {
	cfg           RelaunchHelperConfig
	now           func() time.Time
	processAlive  func(int) bool
	acquireLock   func(string, int) (*Lock, error)
	readInfo      func(string) (Info, error)
	startDetached detachedStartFunc
}

// RunRelaunchHelper runs the detached helper that waits for daemon shutdown resources to release
// and then launches the replacement daemon.
func RunRelaunchHelper(ctx context.Context, cfg RelaunchHelperConfig) error {
	return newRelaunchHelper(cfg).run(ctx)
}

func newRelaunchHelper(cfg RelaunchHelperConfig) *relaunchHelper {
	if cfg.Executable == nil {
		cfg.Executable = os.Executable
	}
	if len(cfg.Sandbox) == 0 {
		cfg.Sandbox = os.Environ()
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = defaultRestartPollInterval
	}
	if cfg.ReleaseTimeout <= 0 {
		cfg.ReleaseTimeout = defaultRestartReleaseWait
	}
	if cfg.ReadyTimeout <= 0 {
		cfg.ReadyTimeout = defaultRestartReadyWait
	}
	if cfg.ExitDrainWait <= 0 {
		cfg.ExitDrainWait = defaultRestartExitDrainWait
	}

	return &relaunchHelper{
		cfg:           cfg,
		now:           func() time.Time { return time.Now().UTC() },
		processAlive:  procutil.Alive,
		acquireLock:   AcquireLock,
		readInfo:      ReadInfo,
		startDetached: defaultDetachedStart,
	}
}

func (h *relaunchHelper) run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	operationID := strings.TrimSpace(h.cfg.OperationID)
	if operationID == "" {
		return errors.New("daemon: restart operation id is required")
	}

	store := newRestartStore(h.cfg.HomePaths, h.now)
	operation, err := h.waitForStopping(ctx, store)
	if err != nil {
		return h.fail(store, operationID, fmt.Errorf("daemon: wait for restart stopping: %w", err))
	}
	if operation.terminal() {
		return nil
	}

	operation, err = store.Transition(operationID, restartTransition{
		status: RestartStatusWaitingRelease,
	})
	if err != nil {
		return err
	}

	operation, err = h.waitForRelease(ctx, store, operation)
	if err != nil {
		return h.fail(store, operationID, fmt.Errorf("daemon: wait for daemon release: %w", err))
	}
	if operation.terminal() {
		return nil
	}

	operation, err = store.Transition(operationID, restartTransition{
		status: RestartStatusStarting,
	})
	if err != nil {
		return err
	}

	binary, err := h.cfg.Executable()
	if err != nil {
		return h.fail(
			store,
			operationID,
			fmt.Errorf("daemon: resolve replacement executable: %w", err),
		)
	}

	replacement, err := h.startDetached(ctx, detachedStartRequest{
		binary:  binary,
		args:    []string{harnessSummaryDefaultAgentName, "start", "--foreground"},
		sandbox: withRestartOperationEnv(h.cfg.Sandbox, operation.OperationID),
		logPath: h.cfg.HomePaths.LogFile,
	})
	if err != nil {
		return h.fail(
			store,
			operationID,
			fmt.Errorf("daemon: spawn replacement daemon: %w", err),
		)
	}

	return h.waitForReady(ctx, store, operationID, replacement)
}

func (h *relaunchHelper) waitForStopping(
	ctx context.Context,
	store *restartStore,
) (RestartOperation, error) {
	waitCtx, cancel := withTimeoutCap(ctx, h.cfg.ReleaseTimeout)
	defer cancel()

	ticker := time.NewTicker(h.cfg.PollInterval)
	defer ticker.Stop()

	for {
		operation, err := store.Get(h.cfg.OperationID)
		if err != nil {
			return RestartOperation{}, err
		}
		if operation.Status == RestartStatusStopping || operation.terminal() {
			return operation, nil
		}

		select {
		case <-waitCtx.Done():
			return RestartOperation{}, errors.New("daemon: restart operation did not enter stopping before timeout")
		case <-ticker.C:
		}
	}
}

func (h *relaunchHelper) waitForRelease(
	ctx context.Context,
	store *restartStore,
	operation RestartOperation,
) (RestartOperation, error) {
	waitCtx, cancel := withTimeoutCap(ctx, h.cfg.ReleaseTimeout)
	defer cancel()

	ticker := time.NewTicker(h.cfg.PollInterval)
	defer ticker.Stop()

	for {
		current, err := store.Get(operation.OperationID)
		if err != nil {
			return RestartOperation{}, err
		}
		if current.terminal() {
			return current, nil
		}

		released, err := h.releaseConditionsMet(current)
		if err != nil {
			return RestartOperation{}, err
		}
		if released {
			return current, nil
		}

		select {
		case <-waitCtx.Done():
			return RestartOperation{}, errors.New("daemon: release wait timed out")
		case <-ticker.C:
		}
	}
}

func (h *relaunchHelper) releaseConditionsMet(operation RestartOperation) (bool, error) {
	if h.processAlive(operation.OldPID) {
		return false, nil
	}
	if _, err := os.Stat(operation.OldSocketPath); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("daemon: stat old socket %q: %w", operation.OldSocketPath, err)
	}
	if _, err := os.Stat(h.cfg.HomePaths.DaemonInfo); err == nil {
		return false, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Errorf("daemon: stat daemon info %q: %w", h.cfg.HomePaths.DaemonInfo, err)
	}

	lock, err := h.acquireLock(h.cfg.HomePaths.DaemonLock, os.Getpid())
	switch {
	case err == nil:
		if releaseErr := lock.Release(); releaseErr != nil {
			return false, fmt.Errorf("daemon: release lock probe %q: %w", h.cfg.HomePaths.DaemonLock, releaseErr)
		}
		return true, nil
	case errors.Is(err, ErrAlreadyRunning):
		return false, nil
	default:
		return false, fmt.Errorf("daemon: probe daemon lock %q: %w", h.cfg.HomePaths.DaemonLock, err)
	}
}

func (h *relaunchHelper) waitForReady(
	ctx context.Context,
	store *restartStore,
	operationID string,
	process restartProcess,
) error {
	if ctx == nil {
		ctx = context.Background()
	}
	waitCtx, cancel := withTimeoutCap(ctx, h.cfg.ReadyTimeout)
	defer cancel()

	processErrCh := make(chan error, 1)
	go func() {
		processErrCh <- process.Wait()
	}()

	ticker := time.NewTicker(h.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case err := <-processErrCh:
			if err != nil {
				return h.fail(
					store,
					operationID,
					fmt.Errorf("%w: %w", errReplacementDaemonExitedBeforeReady, err),
				)
			}
			return h.fail(
				store,
				operationID,
				errReplacementDaemonExitedBeforeReady,
			)
		case <-waitCtx.Done():
			return h.handleReadyWaitDone(ctx, waitCtx, store, operationID, processErrCh)
		case <-ticker.C:
			operation, err := store.Get(operationID)
			if err != nil {
				return h.fail(
					store,
					operationID,
					fmt.Errorf("daemon: load restart operation %q: %w", operationID, err),
				)
			}
			switch operation.Status {
			case RestartStatusReady:
				return nil
			case RestartStatusFailed:
				return fmt.Errorf("daemon: restart operation failed: %s", operation.FailureReason)
			}
		}
	}
}

func (h *relaunchHelper) handleReadyWaitDone(
	ctx context.Context,
	waitCtx context.Context,
	store *restartStore,
	operationID string,
	processErrCh <-chan error,
) error {
	if err := replacementReadinessCanceledError(waitCtx); err != nil {
		return h.fail(store, operationID, err)
	}
	exited, err := waitForProcessExitAfterReadyTimeout(ctx, processErrCh, h.cfg.ExitDrainWait)
	if exited {
		if err != nil {
			return h.fail(
				store,
				operationID,
				fmt.Errorf("%w: %w", errReplacementDaemonExitedBeforeReady, err),
			)
		}
		return h.fail(store, operationID, errReplacementDaemonExitedBeforeReady)
	}
	if err != nil {
		if cancelErr := replacementReadinessCanceledError(ctx); cancelErr != nil {
			return h.fail(store, operationID, cancelErr)
		}
		return h.fail(
			store,
			operationID,
			fmt.Errorf("daemon: wait for replacement daemon exit after readiness timeout: %w", err),
		)
	}
	return h.fail(store, operationID, errors.New("daemon: replacement daemon did not become ready before timeout"))
}

func replacementReadinessCanceledError(ctx context.Context) error {
	if ctx == nil || !errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	cause := context.Cause(ctx)
	if cause == nil {
		cause = ctx.Err()
	} else if !errors.Is(cause, ctx.Err()) {
		cause = errors.Join(ctx.Err(), cause)
	}
	return fmt.Errorf("daemon: replacement daemon readiness canceled: %w", cause)
}

func waitForProcessExitAfterReadyTimeout(
	ctx context.Context,
	processErrCh <-chan error,
	grace time.Duration,
) (bool, error) {
	select {
	case err := <-processErrCh:
		return true, err
	default:
	}
	if grace <= 0 {
		return false, nil
	}

	timer := time.NewTimer(grace)
	defer timer.Stop()

	select {
	case err := <-processErrCh:
		return true, err
	case <-timer.C:
		return false, nil
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (h *relaunchHelper) fail(store *restartStore, operationID string, err error) error {
	if err == nil {
		return nil
	}
	_, transitionErr := store.Transition(operationID, restartTransition{
		status:        RestartStatusFailed,
		failureReason: err.Error(),
	})
	if transitionErr != nil && !errors.Is(transitionErr, errInvalidRestartTransition) {
		return errors.Join(err, transitionErr)
	}
	return err
}
