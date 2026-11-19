package session

import (
	"context"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// sideFamilyMember mirrors the durable recursive visibility filter for live overlays.
// Parent reads are non-repairing; legacy descendants remain hidden even while resumed.
func (m *Manager) sideFamilyMember(ctx context.Context, lineage *store.SessionLineage) bool {
	seen := map[string]bool{}
	for lineage != nil {
		if isSideLineage(lineage) {
			return true
		}
		parent := strings.TrimSpace(lineage.ParentSessionID)
		if parent == "" || seen[parent] {
			return false
		}
		seen[parent] = true
		meta, err := m.readSessionMetaReadOnly(ctx, parent)
		if err != nil {
			return false
		}
		lineage = meta.Lineage
	}
	return false
}
