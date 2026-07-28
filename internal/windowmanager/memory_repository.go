package windowmanager

import (
	"context"
	"fmt"
	"sync"
)

// MemoryRepository is an atomic repository for composition tests and ephemeral runtimes.
type MemoryRepository struct {
	mu        sync.RWMutex
	snapshots map[WorkspaceID]Snapshot
	commits   map[WorkspaceID][]Commit
}

var _ Repository = (*MemoryRepository)(nil)

// NewMemoryRepository creates an empty workspace-partitioned repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{snapshots: make(map[WorkspaceID]Snapshot), commits: make(map[WorkspaceID][]Commit)}
}

func (r *MemoryRepository) Load(ctx context.Context, workspaceID WorkspaceID) (Snapshot, error) {
	if err := validateContext(ctx); err != nil {
		return Snapshot{}, err
	}
	r.mu.RLock()
	snapshot, exists := r.snapshots[workspaceID]
	r.mu.RUnlock()
	if !exists {
		return Snapshot{}, fmt.Errorf("workspace %q: %w", workspaceID, ErrSnapshotNotFound)
	}
	return cloneSnapshot(snapshot), nil
}

func (r *MemoryRepository) Commit(ctx context.Context, commit *Commit) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if commit.WorkspaceID == "" || commit.Snapshot.WorkspaceID != commit.WorkspaceID ||
		commit.Event.WorkspaceID != commit.WorkspaceID {
		return fmt.Errorf("commit workspace identity: %w", ErrInvalidTopology)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	currentRevision := Revision(0)
	if current, exists := r.snapshots[commit.WorkspaceID]; exists {
		currentRevision = current.Revision
	}
	if currentRevision != commit.ExpectedRevision {
		return &RevisionConflictError{Expected: commit.ExpectedRevision, Current: currentRevision}
	}
	nextRevision, err := NextTopologyRevision(currentRevision)
	if err != nil {
		return fmt.Errorf("commit revision: %w", err)
	}
	if commit.Snapshot.Revision != nextRevision || commit.Event.Revision != commit.Snapshot.Revision {
		return fmt.Errorf("commit revision must advance exactly once: %w", ErrInvalidTopology)
	}
	r.snapshots[commit.WorkspaceID] = cloneSnapshot(commit.Snapshot)
	cloned := *commit
	cloned.Snapshot = cloneSnapshot(commit.Snapshot)
	cloned.Event = cloneEvent(commit.Event)
	r.commits[commit.WorkspaceID] = append(r.commits[commit.WorkspaceID], cloned)
	return nil
}

func (r *MemoryRepository) DeleteWorkspace(ctx context.Context, workspaceID WorkspaceID) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.snapshots, workspaceID)
	delete(r.commits, workspaceID)
	r.mu.Unlock()
	return nil
}

// Commits returns an immutable copy of a workspace's transaction log.
func (r *MemoryRepository) Commits(workspaceID WorkspaceID) []Commit {
	r.mu.RLock()
	defer r.mu.RUnlock()
	stored := r.commits[workspaceID]
	commits := make([]Commit, len(stored))
	for index := range stored {
		commits[index] = stored[index]
		commits[index].Snapshot = cloneSnapshot(stored[index].Snapshot)
		commits[index].Event = cloneEvent(stored[index].Event)
	}
	return commits
}
