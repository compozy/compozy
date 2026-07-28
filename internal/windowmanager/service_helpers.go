package windowmanager

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

func (m *Manager) resolveWorkspace(ctx context.Context, workspaceID WorkspaceID) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if err := validateWorkspaceID(workspaceID); err != nil {
		return err
	}
	m.mu.Lock()
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return ErrClosed
	}
	if err := m.workspaces.ResolveWorkspace(ctx, workspaceID); err != nil {
		return fmt.Errorf("resolve workspace %q: %w", workspaceID, err)
	}
	return nil
}

func (m *Manager) lockFor(workspaceID WorkspaceID) (*sync.Mutex, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil, ErrClosed
	}
	lock := m.workspaceLocks[workspaceID]
	if lock == nil {
		lock = &sync.Mutex{}
		m.workspaceLocks[workspaceID] = lock
	}
	return lock, nil
}

// Close terminates live subscriptions and rejects later operations.
func (m *Manager) Close() error {
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return nil
	}
	m.closed = true
	hubs := make([]*subscriptionHub, 0, len(m.hubs))
	for _, hub := range m.hubs {
		hubs = append(hubs, hub)
	}
	m.mu.Unlock()
	for _, hub := range hubs {
		hub.closeAll(nil)
	}
	return nil
}

func validateCommandRequest(request CommandRequest) (CommandID, error) {
	if request.Payload == nil {
		return "", fmt.Errorf("command payload is required: %w", ErrInvalidCommand)
	}
	commandID := request.Payload.CommandID()
	if request.CommandID != "" && request.CommandID != commandID {
		return "", fmt.Errorf(
			"command ID %q does not match payload %q: %w",
			request.CommandID,
			commandID,
			ErrInvalidCommand,
		)
	}
	return commandID, nil
}

func resolveExpectedRevision(snapshot *Snapshot, request CommandRequest) (*Revision, error) {
	if request.ExpectedRevision == snapshot.Revision {
		return nil, nil
	}
	conflict := &RevisionConflictError{Expected: request.ExpectedRevision, Current: snapshot.Revision}
	if request.ExpectedRevision > snapshot.Revision || request.Rebase == nil {
		return nil, conflict
	}
	conflict.Details = validateRebaseGuard(snapshot, request.Rebase)
	if len(conflict.Details) > 0 {
		return nil, conflict
	}
	rebased := request.ExpectedRevision
	return &rebased, nil
}

func validateRebaseGuard(snapshot *Snapshot, guard *RebaseGuard) []Conflict {
	var conflicts []Conflict
	guarded := false
	if guard.WindowID != nil {
		guarded = true
		placement, exists := findWindowPlacement(snapshot, *guard.WindowID)
		if !exists {
			conflicts = append(conflicts, Conflict{Code: "rebase.window_missing", EntityID: string(*guard.WindowID)})
		} else if guard.SourceNodeID != nil && placement.nodeID != *guard.SourceNodeID {
			conflicts = append(
				conflicts,
				Conflict{
					Code:      "rebase.source_changed",
					EntityID:  string(*guard.SourceNodeID),
					CurrentID: string(placement.nodeID),
				},
			)
		}
	}
	if guard.TargetNodeID != nil {
		guarded = true
		if _, exists := findNodeInSnapshot(snapshot, *guard.TargetNodeID); !exists {
			conflicts = append(
				conflicts,
				Conflict{Code: "rebase.target_changed", EntityID: string(*guard.TargetNodeID)},
			)
		}
	}
	if guard.SplitID != nil {
		guarded = true
		node, exists := findNodeInSnapshot(snapshot, *guard.SplitID)
		if !exists || node.Kind != NodeKindSplit {
			conflicts = append(conflicts, Conflict{Code: "rebase.split_changed", EntityID: string(*guard.SplitID)})
		} else if guard.BoundaryIndex == nil || *guard.BoundaryIndex < 0 || *guard.BoundaryIndex >= len(node.Children)-1 {
			conflicts = append(conflicts, Conflict{Code: "rebase.boundary_changed", EntityID: string(*guard.SplitID)})
		}
	}
	if !guarded {
		conflicts = append(conflicts, Conflict{Code: "rebase.guard_required"})
	}
	return conflicts
}

func (m *Manager) repositoryConflict(ctx context.Context, workspaceID WorkspaceID, expected Revision) error {
	current, err := m.repository.Load(ctx, workspaceID)
	if err != nil && !errors.Is(err, ErrSnapshotNotFound) {
		return fmt.Errorf("load revision after conflict: %w", err)
	}
	return &RevisionConflictError{Expected: expected, Current: current.Revision}
}

type emptyLayoutRegistry struct{}

func (emptyLayoutRegistry) Resolve(context.Context, WorkspaceID, string) (LayoutDocument, error) {
	return LayoutDocument{}, ErrLayoutResourceNotFound
}
func (emptyLayoutRegistry) List(context.Context, WorkspaceID) ([]LayoutResource, error) {
	return []LayoutResource{}, nil
}

var _ LayoutResourceRegistry = emptyLayoutRegistry{}
