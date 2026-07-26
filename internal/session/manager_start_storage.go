package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/store"
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
	recorder, err := m.openStore(ctx, spec.sessionID, storage.dbPath)
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
	lineage *store.SessionLineage,
) (*store.SessionLineage, error) {
	normalizedType := normalizeSessionType(sessionType)
	normalized := store.NormalizeSessionLineage(sessionID, lineage)
	if err := store.ValidateSessionLineage(sessionID, normalized); err != nil {
		return nil, fmt.Errorf("session: validate session lineage: %w", err)
	}

	hasParent := strings.TrimSpace(normalized.ParentSessionID) != ""
	switch {
	case normalizedType == SessionTypeSpawned && !hasParent:
		return nil, errors.New("session: spawned session lineage requires a parent session id")
	case hasParent && normalizedType != SessionTypeSpawned:
		return nil, errors.New("session: only spawned sessions may have a parent session id")
	case normalizedType == SessionTypeCoordinator && hasParent:
		return nil, errors.New("session: coordinator sessions must be root sessions")
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
	if err := m.validateCreateLineageReferences(ctx, normalized); err != nil {
		return nil, fmt.Errorf("session: validate lineage references for %q: %w", sessionID, err)
	}

	return normalized, nil
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
