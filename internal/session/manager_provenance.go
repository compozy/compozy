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
			return &store.SessionLineage{
				ParentSessionID: parentSessionID,
				RootSessionID:   parentSessionID,
				SpawnDepth:      1,
			}, true, nil
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
	if rootSessionID != parent.ID {
		root, statusErr := m.Status(ctx, rootSessionID)
		switch {
		case errors.Is(statusErr, ErrSessionNotFound):
			rootSessionID = parent.ID
		case statusErr != nil:
			return nil, false, fmt.Errorf("session: resolve internal provenance root %q: %w", rootSessionID, statusErr)
		case strings.TrimSpace(root.WorkspaceID) != strings.TrimSpace(workspaceID):
			return nil, false, fmt.Errorf("%w: internal provenance root belongs to another workspace", ErrValidation)
		}
	}
	return &store.SessionLineage{
		ParentSessionID: parent.ID,
		RootSessionID:   rootSessionID,
		SpawnDepth:      parentLineage.SpawnDepth + 1,
	}, true, nil
}

func (m *Manager) prepareSpawnedLineageReferences(
	ctx context.Context,
	lineage *store.SessionLineage,
) (*store.SessionLineage, error) {
	parentID := strings.TrimSpace(lineage.ParentSessionID)
	parent, err := m.Status(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("session: validate parent lineage %q: %w", parentID, err)
	}
	rootID := strings.TrimSpace(lineage.RootSessionID)
	if rootID == "" || rootID == parentID {
		return lineage, nil
	}
	root, err := m.Status(ctx, rootID)
	if err == nil {
		if strings.TrimSpace(root.WorkspaceID) != strings.TrimSpace(parent.WorkspaceID) {
			return nil, fmt.Errorf("%w: spawned root belongs to another workspace", ErrValidation)
		}
		return lineage, nil
	}
	if !errors.Is(err, ErrSessionNotFound) {
		return nil, fmt.Errorf("session: validate root lineage %q: %w", rootID, err)
	}

	if lineage.SpawnRole == store.SubagentSpawnRole {
		rebased := *lineage
		rebased.RootSessionID = parent.ID
		return &rebased, nil
	}
	governance, err := m.spawnGovernanceForParent(ctx, parent)
	if err != nil {
		return nil, err
	}
	rebased := *lineage
	rebased.RootSessionID = governance.rootID
	return &rebased, nil
}

func (m *Manager) revalidateSpawnedLineageForStart(ctx context.Context, spec *sessionStartSpec) error {
	if spec.startAction != sessionStartActionCreate || normalizeSessionType(spec.sessionType) != SessionTypeSpawned {
		return nil
	}
	lineage, err := m.prepareSpawnedLineageReferences(ctx, spec.lineage)
	if err != nil {
		return fmt.Errorf("session: revalidate spawned lineage for %q: %w", spec.sessionID, err)
	}
	spec.lineage = lineage
	return nil
}

// refreshStartLineage runs under lifecycleMu so reparenting cannot stale an accepted start.
func (m *Manager) refreshStartLineage(ctx context.Context, spec *sessionStartSpec) error {
	if spec.startAction == sessionStartActionResume {
		meta, err := m.readSessionMetaReadOnly(ctx, spec.sessionID)
		if err != nil {
			return fmt.Errorf("session: refresh resumed lineage: %w", err)
		}
		spec.sessionType = normalizeSessionType(Type(meta.SessionType))
		spec.lineage = store.NormalizeSessionLineage(spec.sessionID, meta.Lineage)
		m.releaseResumedParentStopBinding(ctx, spec)
		return store.ValidateSessionLineage(spec.sessionID, spec.lineage)
	}
	if spec.lineage == nil || spec.lineage.ParentSessionID == "" {
		return nil
	}
	if spec.sessionType == SessionTypeUser {
		lineage, err := m.prepareProvenanceLineage(ctx, spec.sessionType, spec.workspace.ID, spec.lineage)
		if err != nil {
			return err
		}
		spec.lineage = lineage
		return store.ValidateSessionLineage(spec.sessionID, lineage)
	}
	if spec.sessionType == SessionTypeSystem {
		lineage, _, err := m.prepareInternalSystemProvenance(
			ctx, spec.sessionID, spec.sessionType, spec.workspace.ID, nil, spec.lineage.ParentSessionID,
		)
		if err != nil {
			return err
		}
		refreshed := store.CloneSessionLineage(spec.lineage)
		refreshed.RootSessionID = lineage.RootSessionID
		refreshed.SpawnDepth = lineage.SpawnDepth
		spec.lineage = refreshed
		return store.ValidateSessionLineage(spec.sessionID, refreshed)
	}
	parent, err := m.Status(ctx, spec.lineage.ParentSessionID)
	if err != nil {
		return fmt.Errorf("session: refresh parent lineage: %w", err)
	}
	lineage := store.CloneSessionLineage(spec.lineage)
	parentLineage := store.NormalizeSessionLineage(parent.ID, parent.Lineage)
	lineage.RootSessionID = parentLineage.RootSessionID
	lineage.SpawnDepth = parentLineage.SpawnDepth + 1
	if err := store.ValidateSessionLineage(spec.sessionID, lineage); err != nil {
		return err
	}
	spec.lineage = lineage
	return nil
}

// releaseResumedParentStopBinding drops auto_stop_on_parent from a child resumed
// after its parent stopped. The parent-stop binding already fired; reviving the
// child is a deliberate act, and keeping the binding would let the spawn reaper
// cancel the launch on its next sweep. Lineage, TTL, and permission narrowing stay.
func (m *Manager) releaseResumedParentStopBinding(ctx context.Context, spec *sessionStartSpec) {
	lineage := spec.lineage
	if lineage == nil || !lineage.AutoStopOnParent || lineage.ParentSessionID == "" {
		return
	}
	live, err := m.sessionLive(ctx, lineage.ParentSessionID)
	if err != nil {
		// Unknown parent state keeps the binding: the reaper decides from the same state.
		spec.startLogger(m).Warn(
			"session.resume.parent_state_unavailable",
			"parent_session_id", lineage.ParentSessionID, "error", err,
		)
		return
	}
	if live {
		return
	}
	lineage.AutoStopOnParent = false
	spec.startLogger(m).Info(
		"session.resume.parent_stop_binding_released",
		"parent_session_id", lineage.ParentSessionID,
	)
}

// sessionLive reports whether id is starting, active, or stopping, by the same
// rule the spawn reaper applies: the in-memory run when there is one, else the
// persisted state. A deleted session is not live.
func (m *Manager) sessionLive(ctx context.Context, id string) (bool, error) {
	state, _, err := m.sessionStopState(ctx, id)
	if err != nil {
		return false, err
	}
	return liveState(state), nil
}
