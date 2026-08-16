package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

func (m *Manager) prepareSessionStartStorage(spec *sessionStartSpec) (sessionStartStorage, error) {
	sessionDir := filepath.Join(m.homePaths.SessionsDir, spec.sessionID)
	if spec.cleanupSessionDir {
		if err := os.MkdirAll(sessionDir, 0o755); err != nil {
			return sessionStartStorage{}, fmt.Errorf("session: create session directory %q: %w", sessionDir, err)
		}
	}

	return sessionStartStorage{
		sessionDir: sessionDir,
		metaPath:   store.SessionMetaFile(sessionDir),
		dbPath:     store.SessionDBFile(sessionDir),
	}, nil
}

func (m *Manager) openSessionStartRecorder(
	ctx context.Context,
	spec *sessionStartSpec,
	storage sessionStartStorage,
) (sessionStartStorage, error) {
	recorder, err := m.openStore(ctx, store.SessionDBOwner{
		SessionID:   spec.sessionID,
		WorkspaceID: spec.workspace.ID,
	}, storage.dbPath)
	if err != nil {
		return storage, fmt.Errorf("session: open session store %q: %w", storage.dbPath, err)
	}
	if spec.clearEventStoreOnOpen {
		if err := clearSessionStartRecorder(ctx, recorder, storage.dbPath); err != nil {
			closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), defaultLifecycleTimeout)
			defer cancel()
			return storage, errors.Join(err, recorder.Close(closeCtx))
		}
	}

	storage.recorder = recorder
	return storage, nil
}

type clearableEventRecorder interface {
	Clear(context.Context) error
}

func clearSessionStartRecorder(ctx context.Context, recorder EventRecorder, dbPath string) error {
	clearable, ok := recorder.(clearableEventRecorder)
	if !ok {
		return fmt.Errorf("session: event store %q does not support reset", dbPath)
	}
	if err := clearable.Clear(ctx); err != nil {
		return fmt.Errorf("session: reset event store %q: %w", dbPath, err)
	}
	return nil
}

func (m *Manager) normalizeCreateLineage(
	ctx context.Context,
	sessionID string,
	sessionType Type,
	workspaceID string,
	lineage *store.SessionLineage,
	provenanceParentSessionID string,
) (*store.SessionLineage, error) {
	normalizedType := normalizeSessionType(sessionType)
	prepared, systemProvenance, err := m.prepareInternalSystemProvenance(
		ctx,
		sessionID,
		normalizedType,
		workspaceID,
		lineage,
		provenanceParentSessionID,
	)
	if err != nil {
		return nil, err
	}
	prepared, err = m.prepareProvenanceLineage(ctx, normalizedType, workspaceID, prepared)
	if err != nil {
		return nil, err
	}
	normalized := store.NormalizeSessionLineage(sessionID, prepared)
	if err := store.ValidateSessionLineage(sessionID, normalized); err != nil {
		return nil, fmt.Errorf("session: validate session lineage: %w", err)
	}

	hasParent := strings.TrimSpace(normalized.ParentSessionID) != ""
	if err := validateCreateLineageParentType(normalizedType, hasParent, systemProvenance); err != nil {
		return nil, err
	}

	requiresTTL := normalizedType == SessionTypeSpawned || normalizedType == SessionTypeCoordinator
	if requiresTTL && normalized.TTLExpiresAt == nil {
		return nil, errors.New("session: spawned and coordinator sessions require a ttl deadline")
	}
	if normalized.TTLExpiresAt != nil {
		now := m.now()
		if !normalized.TTLExpiresAt.After(now) {
			return nil, errors.New("session: ttl deadline must be in the future")
		}
		if normalized.SpawnBudget.TTLSeconds <= 0 {
			ttlSeconds := int64(normalized.TTLExpiresAt.Sub(now).Seconds())
			if ttlSeconds <= 0 {
				ttlSeconds = 1
			}
			normalized.SpawnBudget.TTLSeconds = ttlSeconds
		}
	}
	if !systemProvenance {
		if err := m.validateCreateLineageReferences(ctx, normalized); err != nil {
			return nil, fmt.Errorf("session: validate lineage references for %q: %w", sessionID, err)
		}
	}

	return normalized, nil
}

func validateCreateLineageParentType(sessionType Type, hasParent, systemProvenance bool) error {
	switch {
	case sessionType == SessionTypeSpawned && !hasParent:
		return errors.New("session: spawned session lineage requires a parent session id")
	case sessionType == SessionTypeCoordinator && hasParent:
		return errors.New("session: coordinator sessions must be root sessions")
	case hasParent && sessionType != SessionTypeSpawned && sessionType != SessionTypeUser &&
		(sessionType != SessionTypeSystem || !systemProvenance):
		return errors.New("session: only spawned or user sessions may have a parent session id")
	default:
		return nil
	}
}

// prepareProvenanceLineage fills server-owned root/depth for user sessions that
// record a creation parent. Provenance links carry no spawn governance: shape
// violations are rejected and the parent must live in the target workspace.
func (m *Manager) prepareProvenanceLineage(
	ctx context.Context,
	sessionType Type,
	workspaceID string,
	lineage *store.SessionLineage,
) (*store.SessionLineage, error) {
	if lineage == nil || sessionType != SessionTypeUser {
		return lineage, nil
	}
	parentID := strings.TrimSpace(lineage.ParentSessionID)
	if parentID == "" {
		return lineage, nil
	}
	if err := validateProvenanceLineageShape(lineage); err != nil {
		return nil, err
	}
	parent, err := m.Status(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("session: resolve parent session %q: %w", parentID, err)
	}
	targetWorkspaceID := strings.TrimSpace(workspaceID)
	if targetWorkspaceID != "" && strings.TrimSpace(parent.WorkspaceID) != targetWorkspaceID {
		return nil, fmt.Errorf(
			"%w: parent session %q belongs to another workspace",
			ErrValidation,
			parentID,
		)
	}
	parentLineage := store.NormalizeSessionLineage(parent.ID, parent.Lineage)
	rootID := strings.TrimSpace(parentLineage.RootSessionID)
	if rootID == "" {
		rootID = parent.ID
	}
	filled := *lineage
	filled.ParentSessionID = parentID
	filled.RootSessionID = rootID
	filled.SpawnDepth = parentLineage.SpawnDepth + 1
	return &filled, nil
}

func validateProvenanceLineageShape(lineage *store.SessionLineage) error {
	switch {
	case lineage.TTLExpiresAt != nil:
		return fmt.Errorf("%w: provenance lineage cannot carry a ttl deadline", ErrValidation)
	case lineage.AutoStopOnParent:
		return fmt.Errorf("%w: provenance lineage cannot auto-stop on parent", ErrValidation)
	case strings.TrimSpace(lineage.SpawnRole) != "":
		return fmt.Errorf("%w: provenance lineage cannot carry a spawn role", ErrValidation)
	case lineage.SpawnBudget != (store.SessionSpawnBudget{}):
		return fmt.Errorf("%w: provenance lineage cannot carry a spawn budget", ErrValidation)
	case !sessionPermissionPolicyIsEmpty(lineage.PermissionPolicy):
		return fmt.Errorf("%w: provenance lineage cannot carry a permission policy", ErrValidation)
	}
	return nil
}

func sessionPermissionPolicyIsEmpty(policy store.SessionPermissionPolicy) bool {
	normalized := store.NormalizeSessionPermissionPolicy(policy)
	return len(normalized.Tools) == 0 &&
		len(normalized.Skills) == 0 &&
		len(normalized.MCPServers) == 0 &&
		len(normalized.WorkspacePaths) == 0 &&
		len(normalized.NetworkChannels) == 0 &&
		len(normalized.SandboxProfiles) == 0
}

func (m *Manager) validateCreateLineageReferences(ctx context.Context, lineage *store.SessionLineage) error {
	if lineage == nil || strings.TrimSpace(lineage.ParentSessionID) == "" {
		return nil
	}
	if _, err := m.Status(ctx, lineage.ParentSessionID); err != nil {
		return fmt.Errorf("session: validate parent lineage %q: %w", lineage.ParentSessionID, err)
	}
	rootID := strings.TrimSpace(lineage.RootSessionID)
	if rootID == "" || rootID == strings.TrimSpace(lineage.ParentSessionID) {
		return nil
	}
	if _, err := m.Status(ctx, rootID); err != nil {
		return fmt.Errorf("session: validate root lineage %q: %w", rootID, err)
	}
	return nil
}
