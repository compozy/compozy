package daemon

import (
	"context"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensionmcp"
)

type extensionMCPAllocationScope uint8

const (
	extensionMCPWorkspaceAllocations extensionMCPAllocationScope = iota
	extensionMCPPackageAllocations
)

// The lifecycle lock covers the snapshot scope through publication or compensation.
type extensionMCPAllocationSnapshot struct {
	key    extensionpkg.InstanceKey
	scope  extensionMCPAllocationScope
	before map[extensionmcp.Target]struct{}
}

func snapshotExtensionMCPAllocations(
	key extensionpkg.InstanceKey,
	scope extensionMCPAllocationScope,
	records []extensionmcp.Record,
) *extensionMCPAllocationSnapshot {
	snapshot := &extensionMCPAllocationSnapshot{
		key: key.Normalize(), scope: scope, before: map[extensionmcp.Target]struct{}{},
	}
	for _, record := range records {
		if snapshot.contains(record.Target) {
			snapshot.before[record.Target] = struct{}{}
		}
	}
	return snapshot
}

func (s *daemonExtensionService) snapshotMCPAllocations(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	scope extensionMCPAllocationScope,
) (*extensionMCPAllocationSnapshot, error) {
	if s.mcpAllocations == nil {
		return nil, nil
	}
	records, err := s.mcpAllocations.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return snapshotExtensionMCPAllocations(key, scope, records), nil
}

// Run only after the owning package/link has been restored or removed successfully.
func (s *daemonExtensionService) rollbackExtensionMCPAllocations(
	ctx context.Context,
	snapshot *extensionMCPAllocationSnapshot,
) error {
	if snapshot == nil || s.mcpAllocations == nil {
		return nil
	}
	rollbackCtx, cancel := extensionSecretRollbackContext(ctx)
	defer cancel()
	records, err := s.mcpAllocations.ListAll(rollbackCtx)
	if err != nil {
		return err
	}
	var created []extensionmcp.Target
	for _, record := range records {
		if !snapshot.contains(record.Target) {
			continue
		}
		if _, existed := snapshot.before[record.Target]; !existed {
			created = append(created, record.Target)
		}
	}
	return s.mcpAllocations.DeleteTargets(rollbackCtx, created)
}

func (s *extensionMCPAllocationSnapshot) contains(target extensionmcp.Target) bool {
	return target.Extension == s.key.Name &&
		(s.scope == extensionMCPPackageAllocations || target.WorkspaceID == s.key.WorkspaceID)
}
