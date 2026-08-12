package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

type mcpProjectionGenerationSource interface {
	ProjectionGeneration(context.Context, []SourceRef) (string, bool)
}

// ProjectionGeneration reports the exact descriptor and availability state for scoped MCP sources.
func (p *MCPProvider) ProjectionGeneration(ctx context.Context, scope Scope) (string, bool) {
	if p == nil || isNilInterface(p.sources) || isNilInterface(p.exec) {
		return "", false
	}
	sources, err := p.sources.ListMCPSources(ctx)
	if err != nil {
		return "", false
	}
	sources = mcpSourcesForScope(sources, scope)
	if len(sources) == 0 {
		return "0", true
	}
	generationSource, ok := p.exec.(mcpProjectionGenerationSource)
	if !ok {
		return "", false
	}
	descriptorGeneration, known := generationSource.ProjectionGeneration(ctx, sources)
	if !known {
		return "", false
	}

	slices.SortFunc(sources, func(left SourceRef, right SourceRef) int {
		return strings.Compare(sourceKey(left), sourceKey(right))
	})
	var generation strings.Builder
	appendProjectionGenerationPart(&generation, "descriptors", descriptorGeneration)
	for i := range sources {
		availability, known := p.mcpSourceAvailabilityGeneration(ctx, sources[i])
		if !known {
			return "", false
		}
		appendProjectionGenerationPart(&generation, sourceKey(sources[i]), availability)
	}
	return generation.String(), true
}

func (p *MCPProvider) mcpSourceAvailabilityGeneration(
	ctx context.Context,
	source SourceRef,
) (string, bool) {
	if p.reliability != nil && p.reliability.dead(ctx, source) {
		return string(ReasonBackendDead), true
	}
	if isNilInterface(p.auth) {
		return "available", true
	}
	status, err := p.auth.Status(ctx, source)
	if err != nil {
		return "", false
	}
	reason, blocked := MCPAuthStatusReason(status)
	if !blocked || reason == ReasonMCPAuthUnconfigured {
		return "available", true
	}
	return fmt.Sprintf("blocked:%s", reason), true
}
