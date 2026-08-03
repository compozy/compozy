package daemon

import (
	"context"
	"errors"
	"fmt"
	"time"

	extensionpkg "github.com/compozy/compozy/internal/extension"
)

const extensionLifecycleRollbackTimeout = 15 * time.Second

type globalExtensionLifecycleSnapshot struct {
	enabled      bool
	confirmation extensionpkg.NetworkConfirmation
}

func snapshotGlobalExtensionLifecycle(info *extensionpkg.ExtensionInfo) globalExtensionLifecycleSnapshot {
	if info == nil {
		return globalExtensionLifecycleSnapshot{}
	}
	return globalExtensionLifecycleSnapshot{
		enabled: info.Enabled,
		confirmation: extensionpkg.NetworkConfirmation{
			Digest:      info.NetworkRequirementDigest,
			ConfirmedBy: info.NetworkConfirmedBy,
			ConfirmedAt: info.NetworkConfirmedAt,
		},
	}
}

func (s *daemonExtensionService) rollbackGlobalExtensionLifecycle(
	ctx context.Context,
	name string,
	snapshot globalExtensionLifecycleSnapshot,
	cause error,
) error {
	rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), extensionLifecycleRollbackTimeout)
	defer cancel()
	var rollbackErr error
	if snapshot.enabled {
		rollbackErr = errors.Join(rollbackErr, s.registry.Enable(name))
	} else {
		rollbackErr = errors.Join(rollbackErr, s.registry.Disable(name))
	}
	rollbackErr = errors.Join(rollbackErr, s.registry.RestoreNetworkConfirmation(
		extensionpkg.GlobalInstanceKey(name),
		snapshot.confirmation,
	))
	rollbackErr = errors.Join(rollbackErr, s.reload(rollbackCtx))
	if rollbackErr != nil {
		rollbackErr = fmt.Errorf("daemon: rollback extension lifecycle %q: %w", name, rollbackErr)
	}
	return errors.Join(cause, rollbackErr)
}
