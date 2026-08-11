package tools

import (
	"context"
	"fmt"
	"strings"
)

// WithProjectionInvalidator invalidates cached projections after any dispatch attempt.
func WithProjectionInvalidator(invalidate func()) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.projectionInvalidator = invalidate
	}
}

// WithProjectionGeneration wires authoritative non-provider projection state.
func WithProjectionGeneration(generation ProjectionGenerationResolver) RegistryOption {
	return func(registry *RuntimeRegistry) {
		registry.projectionGeneration = generation
	}
}

// ProjectionGeneration returns one exact generation token for all scoped projection inputs.
func (r *RuntimeRegistry) ProjectionGeneration(ctx context.Context, scope Scope) (string, bool) {
	if r == nil || r.projectionGeneration == nil {
		return "", false
	}
	base, known := r.projectionGeneration(ctx, scope)
	if !known {
		return "", false
	}
	var generation strings.Builder
	appendProjectionGenerationPart(&generation, "registry", base)
	for _, provider := range r.providers {
		source, ok := provider.(ProjectionGenerationProvider)
		if !ok {
			return "", false
		}
		providerGeneration, providerKnown := source.ProjectionGeneration(ctx, scope)
		if !providerKnown {
			return "", false
		}
		appendProjectionGenerationPart(&generation, sourceKey(provider.ID()), providerGeneration)
	}
	return generation.String(), true
}

func appendProjectionGenerationPart(dst *strings.Builder, name string, generation string) {
	fmt.Fprintf(dst, "%d:%s%d:%s", len(name), name, len(generation), generation)
}
