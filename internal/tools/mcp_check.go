package tools

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/deadentity"
)

// CheckMCPServer refreshes only the selected effective source; consent is still checked before discovery.
func (p *MCPProvider) CheckMCPServer(ctx context.Context, scope Scope, name string) error {
	sources, err := p.sources.ListMCPSources(ctx)
	if err != nil {
		return fmt.Errorf("tools: list MCP sources for check: %w", err)
	}
	for _, source := range MCPSourcesForScope(sources, scope) {
		if mcpSourceServerName(source) != name {
			continue
		}
		// A rejected listing is shown as a failed source in the refreshed session catalog.
		_, err := p.CheckMCPSource(ctx, source, func(ctx context.Context) ([]MCPToolDescriptor, error) {
			return p.discoverSource(ctx, source, p.discoveryBudget)
		})
		return err
	}
	return NewValidationError("name", ReasonSourceDisabled, "MCP server is not available in this scope")
}

// CheckMCPSource reports a catalog rejection separately from an error running the check.
func (p *MCPProvider) CheckMCPSource(
	ctx context.Context, source SourceRef, discover func(context.Context) ([]MCPToolDescriptor, error),
) (MCPSourceDiscovery, error) {
	if key, tracked := mcpDeadEntityKey(source, source.ProfileID); tracked && p.reliability != nil {
		ctx = deadentity.WithProbeRetry(ctx, key)
	}
	ctx = WithFreshMCPDiscovery(context.WithValue(ctx, cachedMCPProjectionKey{}, false))
	return p.listSourceDiscovery(ctx, source, source.ProfileID, discover)
}

type freshMCPDiscoveryKey struct{}

// WithFreshMCPDiscovery bypasses executor tool-list caches for an explicit probe.
func WithFreshMCPDiscovery(ctx context.Context) context.Context {
	return context.WithValue(ctx, freshMCPDiscoveryKey{}, true)
}

func FreshMCPDiscovery(ctx context.Context) bool {
	fresh, ok := ctx.Value(freshMCPDiscoveryKey{}).(bool)
	return ok && fresh
}
