package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

type sideFamilyReader interface {
	SessionSideAncestry(context.Context, string, string, string) (*store.SessionLineage, error)
}

func (m *Manager) sideFamilyMember(
	ctx context.Context, sessionID, workspaceID, profileID string, lineage *store.SessionLineage,
) bool {
	hidden, err := m.sideFamilyMembership(ctx, sessionID, workspaceID, profileID, lineage)
	return err == nil && hidden
}

// sideFamilyMembership uses durable ancestry when a parent's metadata is missing.
func (m *Manager) sideFamilyMembership(
	ctx context.Context, sessionID, workspaceID, profileID string, lineage *store.SessionLineage,
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
		if errors.Is(err, ErrSessionNotFound) || errors.Is(err, store.ErrSessionNotFound) {
			lineage = nil
			if reader, ok := m.sessionCatalog.(sideFamilyReader); ok {
				var readErr error
				lineage, readErr = reader.SessionSideAncestry(ctx, parent, workspaceID, profileID)
				if readErr != nil {
					return false, fmt.Errorf("session: read durable side ancestry %s: %w", parent, readErr)
				}
			}
			if lineage == nil {
				m.warnMissingAncestor(ctx, child, parent, err)
			}
			child = parent
			continue
		}
		if err != nil {
			return false, fmt.Errorf("session: read side ancestry %s: %w", parent, err)
		}
		if meta.WorkspaceID != workspaceID || meta.ProfileID != profileID {
			return false, nil
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
