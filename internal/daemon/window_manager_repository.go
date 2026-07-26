package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/compozy/agh/internal/clientstate"
	"github.com/compozy/agh/internal/windowmanager"
)

const (
	windowManagerStateDomain = "window_manager"
	windowManagerSnapshotKey = "snapshot"
)

// windowManagerRepository persists one typed aggregate per workspace.
type windowManagerRepository struct {
	service clientstate.Service

	mu    sync.Mutex
	locks map[windowmanager.WorkspaceID]*sync.Mutex
}

var _ windowmanager.Repository = (*windowManagerRepository)(nil)

func newWindowManagerRepository(service clientstate.Service) (*windowManagerRepository, error) {
	if service == nil {
		return nil, errors.New("daemon: window-manager client-state service is required")
	}
	return &windowManagerRepository{
		service: service,
		locks:   make(map[windowmanager.WorkspaceID]*sync.Mutex),
	}, nil
}

func (r *windowManagerRepository) Load(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
) (windowmanager.Snapshot, error) {
	entry, err := r.service.Get(
		ctx,
		clientstate.WorkspaceID(workspaceID),
		windowManagerStateDomain,
		windowManagerSnapshotKey,
	)
	if err != nil {
		return windowmanager.Snapshot{}, mapWindowManagerStoreError("load snapshot", err)
	}
	return decodeWindowManagerSnapshot(entry.Value, workspaceID)
}

func (r *windowManagerRepository) Commit(ctx context.Context, commit *windowmanager.Commit) error {
	if err := validateWindowManagerCommit(commit); err != nil {
		return err
	}
	lock := r.lockFor(commit.WorkspaceID)
	lock.Lock()
	defer lock.Unlock()

	current, entryRevision, exists, err := r.loadCurrent(ctx, commit.WorkspaceID)
	if err != nil {
		return err
	}
	currentRevision := windowmanager.Revision(0)
	if exists {
		currentRevision = current.Revision
	}
	if currentRevision != commit.ExpectedRevision {
		return &windowmanager.RevisionConflictError{
			Expected: commit.ExpectedRevision,
			Current:  currentRevision,
		}
	}

	encoded, err := json.Marshal(commit.Snapshot)
	if err != nil {
		return fmt.Errorf("daemon: encode window-manager snapshot: %w", err)
	}
	entries, err := r.service.Apply(
		ctx,
		clientstate.WorkspaceID(commit.WorkspaceID),
		windowManagerStateDomain,
		[]clientstate.Op{{
			Kind:  clientstate.OpPut,
			Key:   windowManagerSnapshotKey,
			Value: encoded,
			IfRev: entryRevision,
		}},
		clientstate.ApplyOptions{Origin: commit.Event.Origin},
	)
	if err != nil {
		if errors.Is(err, clientstate.ErrRevConflict) {
			return &windowmanager.RevisionConflictError{
				Expected: commit.ExpectedRevision,
				Current:  currentRevision,
			}
		}
		return mapWindowManagerStoreError("commit snapshot", err)
	}
	if len(entries) != 1 || entries[0].Deleted {
		return errors.New("daemon: window-manager snapshot commit returned an invalid result")
	}
	return nil
}

func (r *windowManagerRepository) DeleteWorkspace(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
) error {
	lock := r.lockFor(workspaceID)
	lock.Lock()
	defer lock.Unlock()

	entry, err := r.service.Get(
		ctx,
		clientstate.WorkspaceID(workspaceID),
		windowManagerStateDomain,
		windowManagerSnapshotKey,
	)
	if errors.Is(err, clientstate.ErrNotFound) {
		return nil
	}
	if err != nil {
		return mapWindowManagerStoreError("load snapshot for deletion", err)
	}
	_, err = r.service.Apply(
		ctx,
		clientstate.WorkspaceID(workspaceID),
		windowManagerStateDomain,
		[]clientstate.Op{{
			Kind: clientstate.OpDelete, Key: windowManagerSnapshotKey, IfRev: entry.Rev,
		}},
		clientstate.ApplyOptions{Origin: "window-manager.workspace.delete"},
	)
	return mapWindowManagerStoreError("delete snapshot", err)
}

func (r *windowManagerRepository) loadCurrent(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
) (windowmanager.Snapshot, uint64, bool, error) {
	entry, err := r.service.Get(
		ctx,
		clientstate.WorkspaceID(workspaceID),
		windowManagerStateDomain,
		windowManagerSnapshotKey,
	)
	if errors.Is(err, clientstate.ErrNotFound) {
		return windowmanager.Snapshot{}, 0, false, nil
	}
	if err != nil {
		return windowmanager.Snapshot{}, 0, false, mapWindowManagerStoreError(
			"load current snapshot",
			err,
		)
	}
	snapshot, err := decodeWindowManagerSnapshot(entry.Value, workspaceID)
	if err != nil {
		return windowmanager.Snapshot{}, 0, false, err
	}
	return snapshot, entry.Rev, true, nil
}

func (r *windowManagerRepository) lockFor(workspaceID windowmanager.WorkspaceID) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	lock := r.locks[workspaceID]
	if lock == nil {
		lock = &sync.Mutex{}
		r.locks[workspaceID] = lock
	}
	return lock
}

func (r *windowManagerRepository) forgetWorkspace(workspaceID windowmanager.WorkspaceID) {
	r.mu.Lock()
	delete(r.locks, workspaceID)
	r.mu.Unlock()
}

func validateWindowManagerCommit(commit *windowmanager.Commit) error {
	if commit == nil {
		return fmt.Errorf("daemon: window-manager commit is required: %w", windowmanager.ErrInvalidTopology)
	}
	if commit.WorkspaceID == "" ||
		commit.Snapshot.WorkspaceID != commit.WorkspaceID ||
		commit.Event.WorkspaceID != commit.WorkspaceID {
		return fmt.Errorf("daemon: window-manager commit workspace identity: %w", windowmanager.ErrInvalidTopology)
	}
	nextRevision, err := windowmanager.NextTopologyRevision(commit.ExpectedRevision)
	if err != nil {
		return fmt.Errorf("daemon: window-manager commit revision: %w", err)
	}
	if commit.Snapshot.Revision != nextRevision ||
		commit.Event.Revision != commit.Snapshot.Revision {
		return fmt.Errorf(
			"daemon: window-manager commit must advance exactly once: %w",
			windowmanager.ErrInvalidTopology,
		)
	}
	if err := windowmanager.ValidateSnapshot(commit.Snapshot); err != nil {
		return fmt.Errorf("daemon: validate window-manager commit: %w", err)
	}
	return nil
}

func decodeWindowManagerSnapshot(
	encoded []byte,
	workspaceID windowmanager.WorkspaceID,
) (windowmanager.Snapshot, error) {
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	var snapshot windowmanager.Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return windowmanager.Snapshot{}, fmt.Errorf(
			"daemon: decode window-manager snapshot: %w",
			errors.Join(windowmanager.ErrInvalidTopology, err),
		)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return windowmanager.Snapshot{}, fmt.Errorf(
			"daemon: decode trailing window-manager snapshot data: %w",
			errors.Join(windowmanager.ErrInvalidTopology, err),
		)
	}
	if snapshot.WorkspaceID != workspaceID {
		return windowmanager.Snapshot{}, fmt.Errorf(
			"daemon: window-manager snapshot workspace mismatch: %w",
			windowmanager.ErrInvalidTopology,
		)
	}
	if err := windowmanager.ValidateSnapshot(snapshot); err != nil {
		return windowmanager.Snapshot{}, fmt.Errorf("daemon: validate stored window-manager snapshot: %w", err)
	}
	return snapshot, nil
}

func mapWindowManagerStoreError(operation string, err error) error {
	if err == nil {
		return nil
	}
	var mapped error
	switch {
	case errors.Is(err, clientstate.ErrNotFound):
		mapped = windowmanager.ErrSnapshotNotFound
	case errors.Is(err, clientstate.ErrWorkspaceNotFound):
		mapped = windowmanager.ErrWorkspaceNotFound
	case errors.Is(err, clientstate.ErrRevConflict):
		mapped = windowmanager.ErrRevisionConflict
	case errors.Is(err, clientstate.ErrClosed):
		mapped = windowmanager.ErrClosed
	default:
		mapped = err
	}
	return fmt.Errorf("daemon: %s: %w", operation, mapped)
}
