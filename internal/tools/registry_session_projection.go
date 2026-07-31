package tools

import (
	"context"
)

// DeferredDiscoveryProvider marks providers whose discovery may require external I/O.
type DeferredDiscoveryProvider interface {
	Provider
	DeferInitialDiscovery() bool
}

// BootstrapSessionProjection returns all callable tools from providers that do not defer discovery.
func (r *RuntimeRegistry) BootstrapSessionProjection(ctx context.Context, scope Scope) ([]ToolView, error) {
	return r.sessionProjection(ctx, scope, includeInitialProvider, nil)
}

// BootstrapSessionCall dispatches through the complete registry to enforce policy and conflict checks.
func (r *RuntimeRegistry) BootstrapSessionCall(
	ctx context.Context,
	scope Scope,
	req CallRequest,
) (ToolResult, error) {
	return r.Call(ctx, scope, req)
}

func includeInitialProvider(provider Provider) bool {
	deferred, ok := provider.(DeferredDiscoveryProvider)
	return !ok || !deferred.DeferInitialDiscovery()
}

func (r *RuntimeRegistry) sessionProjection(
	ctx context.Context,
	scope Scope,
	includeProvider func(Provider) bool,
	includeEntry func(*registryEntry) bool,
) ([]ToolView, error) {
	index, err := r.buildIndexMatching(ctx, scope, includeProvider)
	if err != nil {
		return nil, err
	}
	entries := index.entries
	if includeEntry != nil {
		entries = make([]*registryEntry, 0, len(index.entries))
		for _, entry := range index.entries {
			if includeEntry(entry) {
				entries = append(entries, entry)
			}
		}
	}
	ids := make([]ToolID, 0, len(entries))
	for _, entry := range entries {
		ids = append(ids, entry.descriptor.ID)
	}
	evaluator, err := r.evaluatorFor(ctx, scope, ids)
	if err != nil {
		return nil, err
	}
	views := make([]ToolView, 0, len(entries))
	for _, entry := range entries {
		view, err := r.viewFor(ctx, scope, evaluator, entry)
		if err != nil {
			return nil, err
		}
		if view.Decision.VisibleToSession && view.Decision.Callable {
			views = append(views, view)
		}
	}
	return views, nil
}
