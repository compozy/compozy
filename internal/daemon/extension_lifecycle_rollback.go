package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"
)

const extensionLifecycleRollbackTimeout = 15 * time.Second

// Compensation restores only the selected package while unrelated runtime failures remain diagnosable.
func (s *daemonExtensionService) rollbackGlobalExtensionLifecycle(
	ctx context.Context,
	name string,
	previous bool,
	cause error,
) error {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extensionLifecycleRollbackTimeout)
	defer cancel()
	profileID := extensionDefaultProfileLens().ID
	rollbackErr := errors.Join(
		s.registry.SetEnabledForProfile(name, profileID, previous),
		s.reloadExtensionProfile(rollbackCtx, name, profileID, previous),
	)
	if rollbackErr != nil {
		rollbackErr = fmt.Errorf("daemon: rollback extension lifecycle %q: %w", name, rollbackErr)
	}
	return errors.Join(cause, rollbackErr)
}
