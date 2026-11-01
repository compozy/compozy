package tools

import "context"

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
	sources := make(map[string]Availability)
	for _, descriptor := range descriptors {
		if _, requested := result[descriptor.ID]; !requested {
			continue
		}
		source := descriptor.Source
		source.RawToolName = ""
		key := sourceKey(source)
		availability, found := sources[key]
		if !found {
			handle := &mcpHandle{descriptor: descriptor, exec: p.exec, auth: p.auth, scope: scope, reliability: p.reliability}
			availability = handle.Availability(ctx, scope)
			sources[key] = availability
		}
		result[descriptor.ID] = availability
	}
	return result
}
