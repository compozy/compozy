package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) normalizeCreateOptsLineage(
	ctx context.Context,
	sessionID string,
	sessionType Type,
	workspaceID string,
	opts CreateOpts,
) (*store.SessionLineage, error) {
	return m.normalizeCreateLineage(
		ctx, sessionID, sessionType, workspaceID, opts.Lineage, opts.ProvenanceParentSessionID,
	)
}

func (m *Manager) prepareInternalSystemProvenance(
	ctx context.Context,
	sessionID string,
	sessionType Type,
	workspaceID string,
	lineage *store.SessionLineage,
	parentSessionID string,
) (*store.SessionLineage, bool, error) {
	parentSessionID = strings.TrimSpace(parentSessionID)
	if parentSessionID == "" {
		return lineage, false, nil
	}
	if lineage != nil {
		return nil, false, fmt.Errorf("%w: internal system provenance cannot be combined with lineage", ErrValidation)
	}
	if normalizeSessionType(sessionType) != SessionTypeSystem {
		return nil, false, fmt.Errorf("%w: internal provenance requires a system session", ErrValidation)
	}

	parent, err := m.Status(ctx, parentSessionID)
	if err != nil {
		if errors.Is(err, ErrSessionNotFound) {
			m.logger.WarnContext(ctx, "session provenance parent is unavailable",
				"session_id", strings.TrimSpace(sessionID),
				"parent_session_id", parentSessionID,
				"workspace_id", strings.TrimSpace(workspaceID),
			)
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("session: resolve internal provenance parent: %w", err)
	}
	if strings.TrimSpace(parent.WorkspaceID) != strings.TrimSpace(workspaceID) {
		return nil, false, fmt.Errorf("%w: internal provenance parent belongs to another workspace", ErrValidation)
	}
	parentLineage := store.NormalizeSessionLineage(parent.ID, parent.Lineage)
	rootSessionID := strings.TrimSpace(parentLineage.RootSessionID)
	if rootSessionID == "" {
		rootSessionID = parent.ID
	}
	return &store.SessionLineage{
		ParentSessionID: parent.ID,
		RootSessionID:   rootSessionID,
		SpawnDepth:      parentLineage.SpawnDepth + 1,
	}, true, nil
}
