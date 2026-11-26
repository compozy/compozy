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
	provider    ProjectionAvailabilityProvider
	ids         []ToolID
	descriptors []Descriptor
}

func (r *RuntimeRegistry) projectionAvailabilities(
	ctx context.Context,
	scope Scope,
	entries []sessionProjectionEntry,
) map[ToolID]Availability {
	cache, cached := ctx.Value(policyResolutionKey{}).(*policyResolutionCache)
	key := policyResolutionScope{registry: r, scope: scope}
	availability := make(map[ToolID]Availability)
	if cached {
		cache.availabilityMu.Lock()
		maps.Copy(availability, cache.availabilities[key])
		cache.availabilityMu.Unlock()
	}
	var batches map[int]*projectionAvailabilityBatch
	for _, candidate := range entries {
		entry := candidate.entry
		if len(entry.conflicts) > 0 {
			delete(availability, entry.descriptor.ID)
			continue
		}
		if _, found := availability[entry.descriptor.ID]; found {
			continue
		}
		if value, found := r.pinnedAvailability(ctx, scope, entry); found {
			availability[entry.descriptor.ID] = value
			continue
		}
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
		if _, mcp := provider.(*MCPProvider); mcp {
			batch.descriptors = append(batch.descriptors, entry.descriptor)
		}
	}
	if len(batches) == 0 {
		return availability
	}
	for _, providerIndex := range slices.Sorted(maps.Keys(batches)) {
		batch := batches[providerIndex]
		var resolved map[ToolID]Availability
		if mcp, ok := batch.provider.(*MCPProvider); ok {
			resolved = mcp.projectionAvailability(ctx, scope, batch.descriptors)
		} else {
			resolved = batch.provider.ProjectionAvailability(ctx, scope, batch.ids)
		}
		for _, id := range batch.ids {
			value, found := resolved[id]
			if !found {
				value = Availability{Enabled: true, ReasonCodes: []ReasonCode{ReasonBackendNotExecutable}}
			}
			availability[id] = validateProjectionAvailability(value)
		}
	}
	if cached {
		cache.availabilityMu.Lock()
		if cache.availabilities == nil {
			cache.availabilities = make(map[policyResolutionScope]map[ToolID]Availability)
		}
		cache.availabilities[key] = maps.Clone(availability)
		cache.availabilityMu.Unlock()
	}
	return availability
}

// PolicyProjection projects an already resolved scope without reentering its policy resolver.
func (r *RuntimeRegistry) PolicyProjection(ctx context.Context, scope Scope, inputs PolicyInputs) ([]ToolView, error) {
	if listings, ok := ctx.Value(projectionListingsKey{}).(*ProjectionListings); ok &&
		cachedMCPProjection(ctx) && !scope.Operator && inputs.CheckIn == nil &&
		ctx.Value(pinnedProjectionInputsKey{}) == nil && ctx.Value(nativeProjectionBuildKey{}) == nil {
		return listings.loadPolicy(ctx, r, scope, inputs)
	}
	return r.policyProjection(ctx, scope, inputs)
}

func (r *RuntimeRegistry) policyProjection(
	ctx context.Context, scope Scope, inputs PolicyInputs,
) ([]ToolView, error) {
	views, _, err := r.policyProjectionWithHealth(ctx, scope, inputs)
	return views, err
}

// policyProjectionWithHealth also reports whether a failed provider was left out of the views.
func (r *RuntimeRegistry) policyProjectionWithHealth(
	ctx context.Context, scope Scope, inputs PolicyInputs,
) ([]ToolView, bool, error) {
	cache := &policyResolutionCache{values: map[policyResolutionScope]policyResolution{
		{registry: r, scope: scope}: {inputs: clonePolicyInputs(inputs)},
	}}
	ctx = context.WithValue(ctx, policyResolutionKey{}, cache)
	index, err := r.buildIndex(ctx, scope)
	if err != nil {
		return nil, false, err
	}
	evaluator, err := r.evaluatorFor(ctx, scope, index.ids())
	if err != nil {
		return nil, false, err
	}
	views := make([]ToolView, 0, len(index.entries))
	for _, entry := range index.entries {
		decision, err := evaluateIndexedDescriptor(ctx, scope, evaluator, entry.descriptor)
		if err != nil {
			return nil, false, err
		}
		views = append(
			views,
			ToolView{
				Descriptor: Descriptor{ID: entry.descriptor.ID, Source: entry.descriptor.Source},
				Decision:   decision,
			},
		)
	}
	return views, index.degraded, nil
}

// DiagnosticProjection returns all indexed tools with effective diagnostics for the supplied scope.
func (r *RuntimeRegistry) DiagnosticProjection(ctx context.Context, scope Scope) ([]ToolView, error) {
	ctx = r.ReadContext(ctx)
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
		_, sessionBatch := entry.provider.(ProjectionAvailabilityProvider)
		_, operatorBatch := entry.provider.(*MCPProvider)
		if sessionBatch && (!scope.Operator || operatorBatch) {
			candidates = append(candidates, sessionProjectionEntry{entry: entry})
		}
	}
	batched := r.projectionAvailabilities(ctx, scope, candidates)
	views := make([]ToolView, 0, len(index.entries))
	for _, entry := range index.entries {
		availability, found := batched[entry.descriptor.ID]
		if !found || len(entry.conflicts) > 0 {
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
	descriptor := projectionDescriptor(ctx, entry.descriptor)
	return ToolView{
		Descriptor:   descriptor,
		Availability: availability,
		Decision:     decision,
	}, nil
}

func (r *RuntimeRegistry) availabilityFor(ctx context.Context, scope Scope, entry *registryEntry) Availability {
	if len(entry.conflicts) > 0 {
		return r.uncachedAvailabilityFor(ctx, scope, entry)
	}
	if value, found := r.pinnedAvailability(ctx, scope, entry); found {
		return value
	}
	cache, cached := ctx.Value(policyResolutionKey{}).(*policyResolutionCache)
	key := policyResolutionScope{registry: r, scope: scope}
	if cached {
		cache.availabilityMu.Lock()
		availability, found := cache.availabilities[key][entry.descriptor.ID]
		cache.availabilityMu.Unlock()
		if found {
			return availability
		}
	}
	availability := r.uncachedAvailabilityFor(ctx, scope, entry)
	if cached {
		cache.availabilityMu.Lock()
		if cache.availabilities == nil {
			cache.availabilities = make(map[policyResolutionScope]map[ToolID]Availability)
		}
		if cache.availabilities[key] == nil {
			cache.availabilities[key] = make(map[ToolID]Availability)
		}
		cache.availabilities[key][entry.descriptor.ID] = availability
		cache.availabilityMu.Unlock()
	}
	return availability
}

func (r *RuntimeRegistry) uncachedAvailabilityFor(ctx context.Context, scope Scope, entry *registryEntry) Availability {
	if len(entry.conflicts) > 0 {
		return Availability{
			Registered:  true,
			Enabled:     true,
			Conflicted:  true,
			ReasonCodes: append([]ReasonCode(nil), entry.conflicts...),
		}
	}
	handle, ok, err := entry.provider.Resolve(ctx, scope, entry.descriptor.ID)
	if err != nil {
		LogBackendFailure(ctx, entry.descriptor.ID, "tool_registry_resolve", err)
		return Availability{
			Registered:  true,
			Enabled:     true,
			ReasonCodes: []ReasonCode{ReasonBackendUnhealthy},
		}
	}
	if !ok || isNilInterface(handle) {
		return Availability{
			Registered:  true,
			Enabled:     true,
			ReasonCodes: []ReasonCode{ReasonBackendNotExecutable},
		}
	}
	availability := handle.Availability(ctx, scope)
	return validateProjectionAvailability(availability)
}
