package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"
)

func (p *MCPProvider) listProjection(ctx context.Context, scope Scope) ([]Descriptor, bool, error) {
	if p == nil || isNilInterface(p.exec) || isNilInterface(p.sources) {
		return nil, false, NewValidationError(
			"provider",
			ReasonDependencyMissing,
			"mcp provider is required",
		)
	}
	if err := contextErr(ctx, ""); err != nil {
		return nil, false, err
	}
	sources, err := p.sources.ListMCPSources(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("tools: list mcp sources: %w", err)
	}
	sources = MCPSourcesForScope(sources, scope)
	if p.reliability != nil {
		p.reliability.retainActiveSources(scope, sources)
	}
	descriptors := make([]Descriptor, 0)
	complete := true
	var deadline time.Time
	if p.discoveryBudget > 0 {
		deadline = time.Now().Add(p.discoveryBudget)
		if callerDeadline, ok := ctx.Deadline(); ok {
			remaining := time.Now().Add(time.Until(callerDeadline) / 2)
			if remaining.Before(deadline) {
				deadline = remaining
			}
		}
	}
	for idx, source := range sources {
		var timeout time.Duration
		if !deadline.IsZero() {
			timeout = max(time.Until(deadline)/time.Duration(len(sources)-idx), time.Nanosecond)
		}
		sourceDescriptors, sourceComplete, err := p.listSourceDescriptors(ctx, source, scope.ProfileID, timeout)
		if err != nil {
			return nil, false, err
		}
		complete = complete && sourceComplete
		descriptors = append(descriptors, sourceDescriptors...)
	}
	slices.SortFunc(descriptors, func(left, right Descriptor) int {
		return strings.Compare(left.ID.String(), right.ID.String())
	})
	return descriptors, complete, nil
}
