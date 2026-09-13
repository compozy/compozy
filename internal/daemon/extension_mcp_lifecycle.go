package daemon

import (
	"context"

	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensionmcp"
)

// The caller holds the instance lifecycle lock until publication or compensation completes.
type extensionMCPAllocationSnapshot struct {
	key    extensionpkg.InstanceKey
	before map[extensionmcp.Target]struct{}
}

func snapshotExtensionMCPAllocations(
	key extensionpkg.InstanceKey,
	records []extensionmcp.Record,
) *extensionMCPAllocationSnapshot {
	snapshot := &extensionMCPAllocationSnapshot{key: key.Normalize(), before: map[extensionmcp.Target]struct{}{}}
	for _, record := range records {
		if record.Extension == snapshot.key.Name && record.WorkspaceID == snapshot.key.WorkspaceID {
			snapshot.before[record.Target] = struct{}{}
		}
	}
	return snapshot
}

func (s *daemonExtensionService) snapshotMCPAllocations(
	ctx context.Context,
	key extensionpkg.InstanceKey,
) (*extensionMCPAllocationSnapshot, error) {
	if s.mcpAllocations == nil {
		return nil, nil
	}
	records, err := s.mcpAllocations.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	return snapshotExtensionMCPAllocations(key, records), nil
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
		if record.Extension != snapshot.key.Name || record.WorkspaceID != snapshot.key.WorkspaceID {
			continue
		}
		if _, existed := snapshot.before[record.Target]; !existed {
			created = append(created, record.Target)
		}
	}
	return s.mcpAllocations.DeleteTargets(rollbackCtx, created)
}
