package tools

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/redact"
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
	sources = MCPSourcesForScope(sources, scope)
	if p.consent != nil {
		admitted := make([]SourceRef, 0, len(sources))
		for _, source := range sources {
			allowed, err := p.consent(ctx, source, scope.ProfileID)
			if err != nil {
				return "", false
			}
			if allowed {
				admitted = append(admitted, source)
			}
		}
		sources = admitted
	}
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
	appendProjectionGenerationPart(&generation, "redaction", fmt.Sprintf("%x", redact.SecretFingerprint()))
	for i := range sources {
		availability, known := p.mcpSourceAvailabilityGeneration(ctx, sources[i], scope.ProfileID)
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
	profileID string,
) (string, bool) {
	if p.reliability != nil && p.reliability.dead(ctx, source, profileID) {
		return string(ReasonBackendDead), true
	}
	if _, failed := p.discoveryFailures.Load(source); failed {
		return string(ReasonBackendUnhealthy), true
	}
	if isNilInterface(p.auth) {
		return "available", true
	}
	status, err := p.auth.Status(ctx, source, "")
	if err != nil {
		return "", false
	}
	reason, blocked := MCPAuthStatusReason(status)
	if !blocked || reason == ReasonMCPAuthUnconfigured {
		return "available", true
	}
	return fmt.Sprintf("blocked:%s", reason), true
}

// cachedProjectionObservation reads the exact inputs used by cache-only MCP listings.
// Discovery owns descriptor validation and fingerprints when publishing a source snapshot.
func (p *MCPProvider) cachedProjectionObservation(ctx context.Context, scope Scope) (string, error) {
	sources, err := p.sources.ListMCPSources(ctx)
	if err != nil {
		return "", fmt.Errorf("tools: enumerate MCP projection sources: %w", err)
	}
	sources = MCPSourcesForScope(sources, scope)
	slices.SortFunc(sources, func(left, right SourceRef) int {
		return strings.Compare(sourceKey(left), sourceKey(right))
	})
	var stamp strings.Builder
	fmt.Fprintf(&stamp, "%x", redact.SecretFingerprint())
	for _, source := range sources {
		if p.consent != nil {
			allowed, err := p.consent(ctx, source, scope.ProfileID)
			if err != nil {
				return "", fmt.Errorf("tools: observe MCP source consent for %q: %w", sourceKey(source), err)
			}
			if !allowed {
				continue
			}
		}
		value, found := p.discovered.Load(source)
		if !found {
			continue
		}
		snapshot, ok := value.(*mcpDiscoverySnapshot)
		if !ok {
			panic("invariant: discovered MCP source has an unexpected type")
		}
		handle := mcpHandle{descriptor: Descriptor{Source: source}, exec: p.exec, auth: p.auth,
			reliability: p.reliability, provider: p}
		fmt.Fprintf(&stamp, "%x", snapshot.content)
		appendProjectionGenerationPart(&stamp, sourceKey(source), source.ResourceVersion)
		appendProjectionAvailability(&stamp, handle.Availability(ctx, scope))
	}
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("tools: observe MCP projection sources: %w", err)
	}
	return stamp.String(), nil
}
