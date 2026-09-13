package daemon

import (
	"context"
	"errors"

	extensionpkg "github.com/compozy/compozy/internal/extension"
)

func (s *daemonExtensionService) prepareInstallMCPAllocations(
	ctx context.Context, prepared preparedDaemonExtensionInstall,
) (*extensionMCPAllocationSnapshot, error) {
	if s.mcpAllocations == nil {
		return nil, nil
	}
	if _, err := s.registry.Get(prepared.name); err == nil {
		return nil, extensionpkg.ErrExtensionExists
	} else if !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		return nil, err
	}
	records, err := s.mcpAllocations.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return snapshotExtensionMCPAllocations(prepared.target.key(prepared.name), records), nil
}

func (s *daemonExtensionService) rollbackInstallMCPAllocations(
	ctx context.Context,
	plan *extensionMCPAllocationSnapshot,
) error {
	if plan == nil || s.mcpAllocations == nil {
		return nil
	}
	rollbackCtx, cancel := extensionSecretRollbackContext(ctx)
	defer cancel()
	// Preserve reservations if package/registry compensation itself failed.
	if _, err := s.registry.Get(plan.key.Name); err == nil {
		return nil
	} else if !errors.Is(err, extensionpkg.ErrExtensionNotFound) {
		return err
	}
	return s.rollbackExtensionMCPAllocations(rollbackCtx, plan)
}
