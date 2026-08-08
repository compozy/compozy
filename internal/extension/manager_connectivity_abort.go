package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const connectivityAbortStartTimeout = 100 * time.Millisecond

// abortConnectivityProvider starts process-tree shutdown after an unsafe teardown response.
// Process.Shutdown owns escalation beyond this caller bound, and normal supervision replaces it.
func (m *Manager) abortConnectivityProvider(
	ctx context.Context,
	name string,
	expected processHandle,
) error {
	if m == nil {
		return ErrManagerRequired
	}
	if expected == nil {
		return nil
	}
	extension, ok := m.lookupManaged(name)
	if !ok || extension == nil {
		return nil
	}
	m.mu.RLock()
	process := extension.process
	m.mu.RUnlock()
	if process == nil || process != expected {
		return nil
	}
	abortCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), connectivityAbortStartTimeout)
	defer cancel()
	err := process.Shutdown(abortCtx)
	if err == nil || processTerminal(process) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	return fmt.Errorf("extension: abort unsafe connectivity provider %q: %w", name, err)
}
