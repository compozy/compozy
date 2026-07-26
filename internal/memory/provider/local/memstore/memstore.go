package memstore

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/memory/provider/local"
)

// Adapter exposes memory.Store through the local provider's contract-typed backend.
type Adapter struct {
	store *memory.Store
}

var _ local.Backend = (*Adapter)(nil)

// New wraps a memory Store for the bundled local provider.
func New(store *memory.Store) *Adapter {
	return &Adapter{store: store}
}

// EnsureDirs creates the underlying memory directories.
func (a *Adapter) EnsureDirs() error {
	store, err := a.requireStore()
	if err != nil {
		return err
	}
	return store.EnsureDirs()
}

// LoadPromptIndex returns the prompt-safe MEMORY.md content for a scope.
func (a *Adapter) LoadPromptIndex(
	scope memcontract.Scope,
) (content string, truncated bool, err error) {
	store, err := a.requireStore()
	if err != nil {
		return "", false, err
	}
	return store.LoadPromptIndex(scope)
}

// List returns memory headers for one scope.
func (a *Adapter) List(scope memcontract.Scope) ([]memcontract.Header, error) {
	store, err := a.requireStore()
	if err != nil {
		return nil, err
	}
	return store.List(scope)
}

// Recall delegates to Store.Recall.
func (a *Adapter) Recall(
	ctx context.Context,
	query memcontract.Query,
	opts memcontract.RecallOptions,
) (memcontract.Packaged, error) {
	store, err := a.requireStore()
	if err != nil {
		return memcontract.Packaged{}, err
	}
	return store.Recall(ctx, query, opts)
}

// ApplyDecision persists and applies a controller decision through Store.
func (a *Adapter) ApplyDecision(ctx context.Context, decision memcontract.Decision) error {
	store, err := a.requireStore()
	if err != nil {
		return err
	}
	if _, err := store.ApplyDecision(ctx, decision); err != nil {
		return fmt.Errorf("memory provider local store: apply decision: %w", err)
	}
	return nil
}

// ForWorkspace returns a backend bound to the requested workspace memory root.
func (a *Adapter) ForWorkspace(workspaceRoot string) local.Backend {
	store, err := a.requireStore()
	if err != nil {
		return &Adapter{}
	}
	return &Adapter{store: store.ForWorkspace(workspaceRoot)}
}

// ForAgent returns a backend bound to the requested agent memory tier.
func (a *Adapter) ForAgent(
	workspaceID string,
	agentName string,
	tier memcontract.AgentTier,
) local.Backend {
	store, err := a.requireStore()
	if err != nil {
		return &Adapter{}
	}
	return &Adapter{store: store.ForAgent(workspaceID, agentName, tier)}
}

func (a *Adapter) requireStore() (*memory.Store, error) {
	if a == nil || a.store == nil {
		return nil, errors.New("memory provider local store: store is required")
	}
	return a.store, nil
}
