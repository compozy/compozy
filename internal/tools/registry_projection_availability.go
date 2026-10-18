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
