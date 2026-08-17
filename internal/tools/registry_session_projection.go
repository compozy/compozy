package tools

import (
	"context"
	"slices"
)

// DeferredDiscoveryProvider marks providers whose discovery may require external I/O.
type DeferredDiscoveryProvider interface {
	Provider
	DeferInitialDiscovery() bool
}

// BootstrapSessionProjection returns all callable tools from providers that do not defer discovery.
func (r *RuntimeRegistry) BootstrapSessionProjection(ctx context.Context, scope Scope) ([]ToolView, error) {
	views, err := r.sessionProjection(ctx, scope, includeInitialProvider, nil)
	if err != nil {
		return nil, err
	}
	required, err := r.bootstrapRequiresDeferredDiscovery(ctx, scope, views)
	if err != nil {
		return nil, err
	}
	if !required {
		return views, nil
	}
	return r.sessionProjection(ctx, scope, nil, nil)
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

func (r *RuntimeRegistry) bootstrapRequiresDeferredDiscovery(
	ctx context.Context,
	scope Scope,
	views []ToolView,
) (bool, error) {
	if !r.usePolicyInput {
		return false, nil
	}
	inputs, err := r.policyInputsFor(ctx, scope)
	if err != nil {
		return false, err
	}
	projected := make([]ToolID, 0, len(views))
	for i := range views {
		projected = append(projected, views[i].Descriptor.ID)
	}
	for _, pattern := range inputs.Agent.Tools {
		if !patternMatchesAnyTool(pattern, projected) {
			return true, nil
		}
	}
	if inputs.Session.Enforced {
		for _, requiredID := range inputs.Session.Tools {
			if !containsToolID(projected, requiredID) {
				return true, nil
			}
		}
	}
	return false, nil
}

func patternMatchesAnyTool(pattern ToolPattern, ids []ToolID) bool {
	return slices.ContainsFunc(ids, pattern.Match)
}

func containsToolID(ids []ToolID, target ToolID) bool {
	return slices.Contains(ids, target)
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
