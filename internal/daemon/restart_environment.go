package daemon

import (
	"context"
	"errors"

	"fmt"
	"os"

	"strings"

	"time"
)

func withRestartOperationEnv(sandbox []string, operationID string) []string {
	if len(sandbox) == 0 {
		sandbox = os.Environ()
	}

	prefix := RestartOperationEnvKey + "="
	result := make([]string, 0, len(sandbox)+1)
	replaced := false
	for _, entry := range sandbox {
		if strings.HasPrefix(entry, prefix) {
			result = append(result, prefix+operationID)
			replaced = true
			continue
		}
		result = append(result, entry)
	}
	if !replaced {
		result = append(result, prefix+operationID)
	}
	return result
}

func withTimeoutCap(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		return ctx, func() {}
	}

	deadline := time.Now().Add(timeout)
	if currentDeadline, hasDeadline := ctx.Deadline(); hasDeadline && !deadline.Before(currentDeadline) {
		return ctx, func() {}
	}

	return context.WithDeadline(ctx, deadline)
}

func (d *Daemon) markRestartReadyIfRequested(info Info) error {
	operationID := strings.TrimSpace(restartOperationIDFromEnv(d.getenv))
	if operationID == "" {
		return nil
	}

	store := newRestartStore(d.homePaths, d.now)
	operation, err := store.Get(operationID)
	if err != nil {
		return fmt.Errorf("daemon: load restart operation %q: %w", operationID, err)
	}
	if !operation.hasFreshDaemonInfo(info) {
		return fmt.Errorf("daemon: restart operation %q did not observe fresh daemon discovery state", operationID)
	}
	if _, err := store.Transition(operationID, restartTransition{
		status: RestartStatusReady,
		newPID: info.PID,
	}); err != nil {
		return fmt.Errorf("daemon: mark restart operation %q ready: %w", operationID, err)
	}
	return nil
}

// reconcileSupersededRestarts closes abandoned observations once another daemon owns
// this home's lock and has completed boot. It never infers failure from elapsed time.
func (d *Daemon) reconcileSupersededRestarts() error {
	entries, err := os.ReadDir(d.homePaths.RestartsDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("daemon: list restart operations for recovery: %w", err)
	}
	currentID := restartOperationIDFromEnv(d.getenv)
	store := newRestartStore(d.homePaths, d.now)
	var failures []error
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		operationID := strings.TrimSuffix(entry.Name(), ".json")
		if operationID == currentID {
			continue
		}
		operation, err := store.Get(operationID)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if operation.Status != RestartStatusStarting {
			continue
		}
		if _, err := store.Transition(operationID, restartTransition{
			status:        RestartStatusFailed,
			failureReason: "restart observation was superseded by another daemon startup; the installed runtime was retained",
		}); err != nil {
			failures = append(failures, fmt.Errorf("daemon: reconcile superseded restart %q: %w", operationID, err))
		}
	}
	return errors.Join(failures...)
}

func restartOperationIDFromEnv(getenv func(string) string) string {
	if getenv == nil {
		return ""
	}
	return strings.TrimSpace(getenv(RestartOperationEnvKey))
}
