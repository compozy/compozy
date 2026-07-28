package windowmanager

import (
	"context"
	"fmt"
	"sync"
)

// MemoryWorkspaceResolver provides explicit workspace registration for tests and ephemeral use.
type MemoryWorkspaceResolver struct {
	mu         sync.RWMutex
	workspaces map[WorkspaceID]struct{}
}

var _ WorkspaceResolver = (*MemoryWorkspaceResolver)(nil)

// NewMemoryWorkspaceResolver registers the supplied workspaces.
func NewMemoryWorkspaceResolver(workspaceIDs ...WorkspaceID) *MemoryWorkspaceResolver {
	resolver := &MemoryWorkspaceResolver{workspaces: make(map[WorkspaceID]struct{}, len(workspaceIDs))}
	for _, workspaceID := range workspaceIDs {
		resolver.workspaces[workspaceID] = struct{}{}
	}
	return resolver
}

func (r *MemoryWorkspaceResolver) ResolveWorkspace(ctx context.Context, workspaceID WorkspaceID) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	r.mu.RLock()
	_, exists := r.workspaces[workspaceID]
	r.mu.RUnlock()
	if !exists {
		return fmt.Errorf("workspace %q: %w", workspaceID, ErrWorkspaceNotFound)
	}
	return nil
}

// Register makes a workspace resolvable.
func (r *MemoryWorkspaceResolver) Register(workspaceID WorkspaceID) {
	r.mu.Lock()
	r.workspaces[workspaceID] = struct{}{}
	r.mu.Unlock()
}

// Unregister makes a workspace unavailable.
func (r *MemoryWorkspaceResolver) Unregister(workspaceID WorkspaceID) {
	r.mu.Lock()
	delete(r.workspaces, workspaceID)
	r.mu.Unlock()
}
