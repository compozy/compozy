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
	views, err := bootstrap.BootstrapSessionProjection(ctx, record.scope())
	if err != nil {
		return HostedProjectionResponse{}, err
	}
	return hostedProjectionResponse(views), nil
}
