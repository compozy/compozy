package tools

import (
	"context"
	"maps"
	"slices"
)

// ProjectionAvailabilityProvider resolves one request-local snapshot without changing call-time checks.
type ProjectionAvailabilityProvider interface {
	// Missing IDs are unresolved; backend failures must return unavailable entries.
	ProjectionAvailability(context.Context, Scope, []ToolID) map[ToolID]Availability
}

type sessionProjectionEntry struct {
	entry    *registryEntry
	decision EffectiveToolDecision
}

type projectionAvailabilityBatch struct {
	provider ProjectionAvailabilityProvider
	ids      []ToolID
}

func projectionAvailabilities(
	ctx context.Context,
	scope Scope,
	entries []sessionProjectionEntry,
) map[ToolID]Availability {
	var batches map[int]*projectionAvailabilityBatch
	for _, candidate := range entries {
		entry := candidate.entry
		provider, ok := entry.provider.(ProjectionAvailabilityProvider)
		if !ok || len(entry.conflicts) > 0 {
			continue
		}
		if batches == nil {
			batches = make(map[int]*projectionAvailabilityBatch)
		}
		batch := batches[entry.providerIndex]
		if batch == nil {
			batch = &projectionAvailabilityBatch{provider: provider}
			batches[entry.providerIndex] = batch
		}
		batch.ids = append(batch.ids, entry.descriptor.ID)
	}
	if len(batches) == 0 {
		return nil
	}
	availability := make(map[ToolID]Availability)
	for _, providerIndex := range slices.Sorted(maps.Keys(batches)) {
		batch := batches[providerIndex]
		resolved := batch.provider.ProjectionAvailability(ctx, scope, batch.ids)
		for _, id := range batch.ids {
			value, found := resolved[id]
			if !found {
				value = Availability{Enabled: true, ReasonCodes: []ReasonCode{ReasonBackendNotExecutable}}
			}
			availability[id] = validateProjectionAvailability(value)
		}
	}
	return availability
}

// DiagnosticProjection returns all indexed tools with effective diagnostics for the supplied scope.
func (r *RuntimeRegistry) DiagnosticProjection(ctx context.Context, scope Scope) ([]ToolView, error) {
	index, err := r.buildIndex(ctx, scope)
	if err != nil {
		return nil, err
	}
	evaluator, err := r.evaluatorFor(ctx, scope, index.ids())
	if err != nil {
		return nil, err
	}
	candidates := make([]sessionProjectionEntry, 0, len(index.entries))
	for _, entry := range index.entries {
		if _, ok := entry.provider.(*MCPProvider); ok {
			candidates = append(candidates, sessionProjectionEntry{entry: entry})
		}
	}
	batched := projectionAvailabilities(ctx, scope, candidates)
	views := make([]ToolView, 0, len(index.entries))
	for _, entry := range index.entries {
		availability, found := batched[entry.descriptor.ID]
		if !found {
			availability = r.availabilityFor(ctx, scope, entry)
		}
		view, err := r.viewForAvailability(ctx, scope, evaluator, entry, availability)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (r *RuntimeRegistry) viewFor(
	ctx context.Context,
	scope Scope,
	evaluator PolicyEvaluator,
	entry *registryEntry,
) (ToolView, error) {
	return r.viewForAvailability(ctx, scope, evaluator, entry, r.availabilityFor(ctx, scope, entry))
}

func (r *RuntimeRegistry) viewForAvailability(
	ctx context.Context,
	scope Scope,
	evaluator PolicyEvaluator,
	entry *registryEntry,
	availability Availability,
) (ToolView, error) {
	decision, err := evaluateIndexedDescriptor(ctx, scope, evaluator, entry.descriptor)
	if err != nil {
		return ToolView{}, err
	}
	decision = applyAvailabilityDecision(decision, availability)
	descriptor := entry.descriptor
	if !entry.owned {
		// Shared native descriptors must not leak mutable fields; index-owned clones already are private.
		descriptor = cloneDescriptor(descriptor)
	}
	return ToolView{
		Descriptor:   descriptor,
		Availability: availability,
		Decision:     decision,
	}, nil
}
