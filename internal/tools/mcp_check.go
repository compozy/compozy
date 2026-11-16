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
		p.forgetDiscovered(func(candidate SourceRef) bool { return candidate == source })
		if key, tracked := mcpDeadEntityKey(source, scope.ProfileID); tracked && p.reliability != nil {
			p.reliability.mu.Lock()
			delete(p.reliability.states, key)
			p.reliability.mu.Unlock()
			p.reliability.service.ForgetEntity(key)
			ctx = deadentity.WithProbeRetry(ctx, key)
		}
		ctx = context.WithValue(ctx, cachedMCPProjectionKey{}, false)
		_, _, err := p.listSourceDescriptors(ctx, source, scope.ProfileID, p.discoveryBudget)
		return err
	}
	return NewValidationError("name", ReasonSourceDisabled, "MCP server is not available in this scope")
}
