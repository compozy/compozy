package session

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/store"
)

// sideFamilyMember mirrors the durable recursive visibility filter for live overlays.
// Parent reads are non-repairing; legacy descendants remain hidden even while resumed.
func (m *Manager) sideFamilyMember(ctx context.Context, lineage *store.SessionLineage) bool {
	hidden, err := m.sideFamilyMembership(ctx, lineage)
	if err != nil {
		return false
	}
	return hidden
}

// sideFamilyMembership distinguishes unreadable ancestry from a proven public family.
func (m *Manager) sideFamilyMembership(ctx context.Context, lineage *store.SessionLineage) (bool, error) {
	seen := map[string]bool{}
	for lineage != nil {
		if isSideLineage(lineage) {
			return true, nil
		}
		if err := ctx.Err(); err != nil {
			return false, err
		}
		parent := store.HierarchyParentSessionID(lineage)
		if parent == "" || seen[parent] {
			return false, nil
		}
		seen[parent] = true
		meta, err := m.readSessionMetaReadOnly(ctx, parent)
		if err != nil {
			return false, fmt.Errorf("session: read side ancestry %s: %w", parent, err)
		}
		lineage = meta.Lineage
	}
	return false, nil
}
