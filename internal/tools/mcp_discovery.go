package tools

import (
	"context"
	"time"
)

// DeferInitialDiscovery keeps vendor MCP startup out of the hosted session handshake.
func (*MCPProvider) DeferInitialDiscovery() bool {
	return true
}

// WithMCPDiscoveryBudget bounds one catalog discovery pass without shortening tool calls.
// Sources share the remaining budget, so a slow peer cannot consume later peers' time.
func WithMCPDiscoveryBudget(budget time.Duration) MCPProviderOption {
	return func(provider *MCPProvider) {
		if budget > 0 {
			provider.discoveryBudget = budget
		}
	}
}

func (p *MCPProvider) discoverSource(
	ctx context.Context,
	source SourceRef,
	timeout time.Duration,
) ([]MCPToolDescriptor, error) {
	if timeout <= 0 {
		return p.exec.ListTools(ctx, source)
	}
	discoveryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return p.exec.ListTools(discoveryCtx, source)
}
