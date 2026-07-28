package daemon

import (
	"context"

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
	if ctx == nil {
		ctx = context.Background()
	}
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

func restartOperationIDFromEnv(getenv func(string) string) string {
	if getenv == nil {
		return ""
	}
	return strings.TrimSpace(getenv(RestartOperationEnvKey))
}
