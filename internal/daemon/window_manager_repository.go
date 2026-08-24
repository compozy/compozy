package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/clientstate"
	"github.com/compozy/compozy/internal/windowmanager"
)

const (
	windowManagerStateDomain     = "window_manager"
	windowManagerSnapshotKeyStem = "snapshot"
)

// windowManagerSnapshotKey partitions the per-workspace aggregate by profile:
// desktops are per-profile working state, so one workspace holds one document
// per profile rather than one document overall (US-026).
func windowManagerSnapshotKey(profileID string) string {
	return windowManagerSnapshotKeyStem + ":" + profileID
}

// windowManagerSnapshotProfile reports the profile a stored aggregate key owns.
func windowManagerSnapshotProfile(key string) (string, bool) {
	profileID, partitioned := strings.CutPrefix(key, windowManagerSnapshotKeyStem+":")
	if !partitioned || profileID == "" {
		return "", false
	}
	return profileID, true
}

// windowManagerRepository persists one typed aggregate per workspace, for one profile.
type windowManagerRepository struct {
	service   clientstate.Service
	profileID string
	logger    *slog.Logger

	mu    sync.Mutex
	locks map[windowmanager.WorkspaceID]*sync.Mutex

	// writeMu guards the durable-write section, not just the flag: a commit holds
	// it shared from before its seal check until after its store write returns, so
	// sealing — which takes it exclusively — cannot overtake a write already in
	// flight.
	writeMu sync.RWMutex
	sealed  bool
}

var errWindowManagerSnapshotDiscardable = errors.New("daemon: discardable window-manager snapshot")

// errWindowManagerProfileDeleted refuses durable writes for a profile whose
// desktops are being removed.
var errWindowManagerProfileDeleted = errors.New("daemon: window-manager profile was deleted")

// seal closes this profile's write path for good and waits for the writes already
// inside it.
//
// Deletion enumerates and removes stored arrangements, so a commit that is midway
// through its store write must finish before the enumeration starts — otherwise it
// lands afterwards and resurrects the partition that was just removed. Taking the
// write lock exclusively is that barrier: it returns only once every earlier commit
// has left the section, and every later one fails before it writes.
func (r *windowManagerRepository) seal() {
	r.writeMu.Lock()
	r.sealed = true
	r.writeMu.Unlock()
}

type windowManagerRepositoryOption func(*windowManagerRepository)

func withWindowManagerRepositoryLogger(logger *slog.Logger) windowManagerRepositoryOption {
	return func(repository *windowManagerRepository) {
		if logger != nil {
			repository.logger = logger
		}
	}
}

var _ windowmanager.Repository = (*windowManagerRepository)(nil)

func newWindowManagerRepository(
	service clientstate.Service,
	profileID string,
	options ...windowManagerRepositoryOption,
) (*windowManagerRepository, error) {
	if service == nil {
		return nil, errors.New("daemon: window-manager client-state service is required")
	}
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return nil, errors.New("daemon: window-manager profile is required")
	}
	repository := &windowManagerRepository{
		service:   service,
		profileID: profileID,
		logger:    slog.Default(),
		locks:     make(map[windowmanager.WorkspaceID]*sync.Mutex),
	}
	for _, option := range options {
		if option != nil {
			option(repository)
		}
	}
	return repository, nil
}

func (r *windowManagerRepository) Load(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
) (windowmanager.Snapshot, error) {
	lock := r.lockFor(workspaceID)
	lock.Lock()
	defer lock.Unlock()
	// Shared for the whole body: the discard path below writes, and a sealed
	// profile has no arrangement left to read anyway. Reporting it as absent is
	// what the stack-activation coalescer needs to drop its pending flush quietly
	// while the manager closes.
	r.writeMu.RLock()
	defer r.writeMu.RUnlock()
	if r.sealed {
		return windowmanager.Snapshot{}, fmt.Errorf(
			"daemon: load snapshot: %w",
			windowmanager.ErrSnapshotNotFound,
		)
	}
	entry, err := r.service.Get(
		ctx,
		clientstate.WorkspaceID(workspaceID),
		windowManagerStateDomain,
		windowManagerSnapshotKey(r.profileID),
	)
	if err != nil {
		return windowmanager.Snapshot{}, mapWindowManagerStoreError("load snapshot", err)
	}
	snapshot, err := decodeWindowManagerSnapshot(entry.Value, workspaceID)
	if !errors.Is(err, errWindowManagerSnapshotDiscardable) {
		return snapshot, err
	}
	r.logger.Warn(
		"discarding incompatible window-manager snapshot",
		"workspace_id", workspaceID,
		"error", err,
	)
	if deleteErr := r.deleteSnapshotEntry(
		ctx,
		workspaceID,
		entry.Rev,
		"window-manager.snapshot.discard",
	); deleteErr != nil {
		return windowmanager.Snapshot{}, deleteErr
	}
	return windowmanager.Snapshot{}, fmt.Errorf(
		"discarded window-manager snapshot: %w",
		windowmanager.ErrSnapshotNotFound,
	)
}

func (r *windowManagerRepository) Commit(ctx context.Context, commit *windowmanager.Commit) error {
	if err := validateWindowManagerCommit(commit); err != nil {
		return err
	}
	lock := r.lockFor(commit.WorkspaceID)
	lock.Lock()
	defer lock.Unlock()
	// Held shared across the whole durable write below, so a seal cannot slip
	// between this check and the store write it guards.
	r.writeMu.RLock()
	defer r.writeMu.RUnlock()
	if r.sealed {
		return errWindowManagerProfileDeleted
	}

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
			Key:   windowManagerSnapshotKey(r.profileID),
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
	r.writeMu.RLock()
	defer r.writeMu.RUnlock()
	// Deleting a workspace's copy of an already-removed profile is a no-op.
	if r.sealed {
		return nil
	}

	entry, err := r.service.Get(
		ctx,
		clientstate.WorkspaceID(workspaceID),
		windowManagerStateDomain,
		windowManagerSnapshotKey(r.profileID),
	)
	if errors.Is(err, clientstate.ErrNotFound) {
		return nil
	}
	if err != nil {
		return mapWindowManagerStoreError("load snapshot for deletion", err)
	}
	return r.deleteSnapshotEntry(ctx, workspaceID, entry.Rev, "window-manager.workspace.delete")
}

func (r *windowManagerRepository) deleteSnapshotEntry(
	ctx context.Context,
	workspaceID windowmanager.WorkspaceID,
	revision uint64,
	origin string,
) error {
	_, err := r.service.Apply(
		ctx,
		clientstate.WorkspaceID(workspaceID),
		windowManagerStateDomain,
		[]clientstate.Op{{Kind: clientstate.OpDelete, Key: windowManagerSnapshotKey(r.profileID), IfRev: revision}},
		clientstate.ApplyOptions{Origin: origin},
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
		windowManagerSnapshotKey(r.profileID),
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
			errors.Join(windowmanager.ErrInvalidTopology, errWindowManagerSnapshotDiscardable, err),
		)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("multiple JSON values")
		}
		return windowmanager.Snapshot{}, fmt.Errorf(
			"daemon: decode trailing window-manager snapshot data: %w",
			errors.Join(windowmanager.ErrInvalidTopology, errWindowManagerSnapshotDiscardable, err),
		)
	}
	if snapshot.Version != windowmanager.SnapshotVersion {
		return windowmanager.Snapshot{}, fmt.Errorf(
			"daemon: window-manager snapshot version %d is unsupported: %w",
			snapshot.Version,
			errors.Join(windowmanager.ErrInvalidTopology, errWindowManagerSnapshotDiscardable),
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
