package tools

import (
	"context"
	"slices"
)

var _ ProjectionAvailabilityProvider = (*MCPProvider)(nil)

// ProjectionAvailability discovers once per batch and shares source-level availability within that request.
func (p *MCPProvider) ProjectionAvailability(ctx context.Context, scope Scope, ids []ToolID) map[ToolID]Availability {
	result := make(map[ToolID]Availability, len(ids))
	for _, id := range ids {
		result[id] = Unavailable(ReasonBackendNotExecutable)
	}
	descriptors, err := p.List(ctx, scope)
	if err != nil {
		for _, id := range ids {
			result[id] = Unavailable(ReasonBackendUnhealthy)
		}
		return result
	}
	for id, availability := range p.projectionAvailability(ctx, scope, descriptors) {
		if _, requested := result[id]; requested {
			result[id] = availability
		}
	}
	return result
}

// projectionAvailability checks current authorization against this request's successful discovery.
// Call-time resolution still revalidates the source; a second discovery here could discard healthy tools.
func (p *MCPProvider) projectionAvailability(
	ctx context.Context,
	scope Scope,
	descriptors []Descriptor,
) map[ToolID]Availability {
	result := make(map[ToolID]Availability, len(descriptors))
	sources := make(map[string]Availability)
	for idx := range descriptors {
		descriptor := &descriptors[idx]
		source := descriptor.Source
		source.RawToolName = ""
		key := sourceKey(source)
		availability, found := sources[key]
		if !found {
			handle := &mcpHandle{
				descriptor:  *descriptor,
				exec:        p.exec,
				auth:        p.auth,
				scope:       scope,
				reliability: p.reliability,
			}
			availability = handle.Availability(ctx, scope)
			sources[key] = availability
		}
		availability.ReasonCodes = slices.Clone(availability.ReasonCodes)
		result[descriptor.ID] = availability
	}
	return result
}
