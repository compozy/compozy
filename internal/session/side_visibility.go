package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

// sideFamilyMember mirrors the durable recursive visibility filter for live overlays.
// Parent reads are non-repairing; legacy descendants remain hidden even while resumed.
func (m *Manager) sideFamilyMember(ctx context.Context, sessionID string, lineage *store.SessionLineage) bool {
	hidden, err := m.sideFamilyMembership(ctx, sessionID, lineage)
	if err != nil {
		return false
	}
	return hidden
}

// sideFamilyMembership distinguishes unreadable ancestry from a proven public family.
// A deleted ancestor ends the walk like a root, as the durable filter's join does:
// deletion keeps a parent's ordinary children and their parent_session_id, so the
// edge is expected history, not a read failure. Any other read error still fails,
// including metadata whose identity does not match its directory.
func (m *Manager) sideFamilyMembership(
	ctx context.Context, sessionID string, lineage *store.SessionLineage,
) (bool, error) {
	child := strings.TrimSpace(sessionID)
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
		// Mismatched metadata is corrupt, not absent: fail closed so it cannot expose
		// a side descendant.
		if errors.Is(err, errSessionMetaIdentityMismatch) {
			return false, fmt.Errorf("session: read side ancestry %s: %w", parent, err)
		}
		// A deleted parent keeps its ordinary children, so a missing ancestor is
		// expected state: treat the edge as absent, as the visibility SQL filter does.
		// The child cannot tell whether that parent was a side session; keeping such
		// orphans hidden needs side deletion to detach them (tracked in #1546).
		if errors.Is(err, ErrSessionNotFound) || errors.Is(err, store.ErrSessionNotFound) {
			m.warnMissingAncestor(ctx, child, parent, err)
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("session: read side ancestry %s: %w", parent, err)
		}
		lineage = meta.Lineage
		child = parent
	}
	return false, nil
}

// warnMissingAncestor logs each dangling child-to-parent edge once per manager.
func (m *Manager) warnMissingAncestor(ctx context.Context, child, parent string, err error) {
	if m.logger == nil {
		return
	}
	if _, warned := m.missingAncestorWarned.LoadOrStore(child+"\x00"+parent, struct{}{}); warned {
		return
	}
	m.logger.WarnContext(
		ctx,
		"session: side ancestry parent is missing; treating the edge as absent",
		"session_id", child, "parent_session_id", parent, "error", err,
	)
}
