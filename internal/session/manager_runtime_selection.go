package session

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

var (
	// ErrRuntimeSelectionConflict reports a stale compare-and-set revision.
	ErrRuntimeSelectionConflict = errors.New("session: runtime selection revision conflict")
)

// SetRuntimeSelection durably selects the default runtime for future prompts.
func (m *Manager) SetRuntimeSelection(
	ctx context.Context,
	id string,
	selection RuntimeSelection,
	expectedRevision int64,
) (*Info, error) {
	normalized, err := NormalizeRuntimeSelection(selection)
	if err != nil {
		return nil, err
	}
	return m.updateRuntimeSelection(ctx, id, &normalized, expectedRevision)
}

// ClearRuntimeSelection removes the durable runtime preference for future prompts.
func (m *Manager) ClearRuntimeSelection(ctx context.Context, id string, expectedRevision int64) (*Info, error) {
	return m.updateRuntimeSelection(ctx, id, nil, expectedRevision)
}

func (m *Manager) updateRuntimeSelection(
	ctx context.Context,
	id string,
	selection *RuntimeSelection,
	expectedRevision int64,
) (*Info, error) {
	if m == nil {
		return nil, errors.New("session: manager is required")
	}
	if ctx == nil {
		return nil, errors.New("session: runtime selection context is required")
	}
	if expectedRevision < 0 {
		return nil, fmt.Errorf("session: expected runtime selection revision must not be negative")
	}
	target := strings.TrimSpace(id)
	if target == "" {
		return nil, errors.New("session: session id is required")
	}
	if active, ok := m.Get(target); ok {
		if selection != nil {
			if err := m.validateRuntimeModelAtAdmission(ctx, active, *selection); err != nil {
				return nil, err
			}
		}
		return m.updateActiveRuntimeSelection(ctx, active, selection, expectedRevision)
	}
	return m.updateStoppedRuntimeSelection(ctx, target, selection, expectedRevision)
}

func (m *Manager) updateActiveRuntimeSelection(
	ctx context.Context,
	active *Session,
	selection *RuntimeSelection,
	expectedRevision int64,
) (*Info, error) {
	active.persistMu.Lock()
	defer active.persistMu.Unlock()

	active.mu.Lock()
	if active.RuntimeSelectionRevision != expectedRevision {
		current := active.RuntimeSelectionRevision
		active.mu.Unlock()
		return nil, runtimeSelectionConflict(expectedRevision, current)
	}
	active.SelectedRuntime = cloneRuntimeSelection(selection)
	active.RuntimeSelectionRevision++
	active.UpdatedAt = m.now()
	active.mu.Unlock()

	if err := m.persistSessionLifecycleStateLocked(ctx, active, false); err != nil {
		return nil, err
	}
	return active.Info(), nil
}

func (m *Manager) updateStoppedRuntimeSelection(
	ctx context.Context,
	target string,
	selection *RuntimeSelection,
	expectedRevision int64,
) (*Info, error) {
	m.lifecycleMu.Lock()
	defer m.lifecycleMu.Unlock()

	if _, ok := m.Get(target); ok || m.isPending(target) {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotActive, target)
	}
	meta, err := m.readMetaWithContext(ctx, target)
	if err != nil {
		return nil, err
	}
	if selection != nil && isCursorRuntimeSelection(*selection) {
		if _, err := m.resolveCursorCatalogBinding(
			ctx,
			*selection,
			modelCatalogExecutionContext(meta.ProfileID, meta.WorkspaceID),
		); err != nil {
			return nil, err
		}
	}
	_, selectionRevision := store.SessionRuntimeSelectionStateValues(meta.RuntimeSelection)
	if selectionRevision != expectedRevision {
		return nil, runtimeSelectionConflict(expectedRevision, selectionRevision)
	}
	meta.RuntimeSelection = store.NewSessionRuntimeSelectionState(
		storeSessionRuntimeSelection(selection),
		selectionRevision+1,
	)
	meta.UpdatedAt = m.now()
	metaPath := store.SessionMetaFile(filepath.Join(m.homePaths.SessionsDir, target))
	if err := store.WriteSessionMeta(metaPath, meta); err != nil {
		return nil, fmt.Errorf("session: persist runtime selection for %q: %w", target, err)
	}
	if err := m.persistSessionCatalogFromMeta(ctx, meta); err != nil {
		return nil, err
	}
	return m.sessionInfoFromMeta(ctx, meta), nil
}

func runtimeSelectionConflict(expected, current int64) error {
	return fmt.Errorf("%w: expected revision %d, current revision %d", ErrRuntimeSelectionConflict, expected, current)
}
