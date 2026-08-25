package mcp

import (
	"context"

	"github.com/compozy/compozy/internal/tools"
)

type hostedBootstrapRegistry interface {
	BootstrapSessionProjection(context.Context, tools.Scope) ([]tools.ToolView, error)
}

type hostedBootstrapCallRegistry interface {
	BootstrapSessionCall(context.Context, tools.Scope, tools.CallRequest) (tools.ToolResult, error)
}

func (s *HostedService) bootstrapProjection(
	ctx context.Context,
	record *hostedBindRecord,
) (HostedProjectionResponse, error) {
	if record == nil {
		return HostedProjectionResponse{}, ErrHostedBindNotFound
	}
	registry := s.currentRegistry()
	if registry == nil {
		return HostedProjectionResponse{}, ErrHostedRegistryRequired
	}
	bootstrap, ok := registry.(hostedBootstrapRegistry)
	if !ok {
		return s.projection(ctx, record)
	}
	generation, known := s.bootstrapProjectionGeneration(ctx, record)
	views, err := bootstrap.BootstrapSessionProjection(ctx, record.scope())
	if err != nil {
		return HostedProjectionResponse{}, err
	}
	if s.projectionDecorator != nil {
		views, err = s.projectionDecorator(ctx, record.scope(), cloneToolViews(views))
		if err != nil {
			return HostedProjectionResponse{}, err
		}
	}
	response := hostedProjectionResponse(views)
	if !known {
		return response, nil
	}
	after, stillKnown := s.bootstrapProjectionGeneration(ctx, record)
	if !stillKnown || after != generation {
		return response, nil
	}
	key := hostedProjectionFlightKey{bindID: record.bindID, generation: generation}
	if !s.finishProjectionGeneration(key, record, response, true) {
		return HostedProjectionResponse{}, ErrHostedBindNotFound
	}
	return cloneHostedProjectionResponse(response), nil
}

func (s *HostedService) bootstrapProjectionGeneration(
	ctx context.Context,
	record *hostedBindRecord,
) (string, bool) {
	if s == nil || s.projectionGeneration == nil || record == nil {
		return "", false
	}
	return s.projectionGeneration(ctx, record.scope())
}
