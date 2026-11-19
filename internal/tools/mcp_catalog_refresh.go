package tools

import (
	"context"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/redact"
)

const (
	mcpCatalogRefreshInterval = 3 * time.Second
	mcpCatalogScopeLifetime   = time.Minute
	mcpCatalogMaxScopes       = 128
	mcpCatalogParallelism     = 4
)

type mcpCatalogRefresh struct {
	mu         sync.Mutex
	scopes     map[Scope]time.Time
	wake       chan struct{}
	invalidate func()
	logger     *slog.Logger
}

// WithMCPBackgroundDiscovery enables cache-only hosted reads and a daemon-owned refresh worker.
func WithMCPBackgroundDiscovery(invalidate func(), logger *slog.Logger) MCPProviderOption {
	return func(p *MCPProvider) {
		p.catalog = &mcpCatalogRefresh{
			scopes: make(map[Scope]time.Time), wake: make(chan struct{}, 1),
			invalidate: invalidate, logger: logger,
		}
	}
}

type hostedMCPProjectionKey struct{}

// IsHostedCatalogRead reports a projection backed by the daemon's discovery worker.
func IsHostedCatalogRead(ctx context.Context) bool {
	return ctx.Value(hostedMCPProjectionKey{}) != nil
}

// HostedProjectionContext keeps discovery active even when a hosted read hits the shared cache.
func (r *RuntimeRegistry) HostedProjectionContext(ctx context.Context, scope Scope) (context.Context, bool) {
	for _, provider := range r.providers {
		if mcp, ok := provider.(*MCPProvider); ok && mcp.catalog != nil {
			mcp.catalog.observe(mcp.ProjectionScope(scope))
			return context.WithValue(WithCachedMCPProjection(ctx), hostedMCPProjectionKey{}, true), true
		}
	}
	return ctx, false
}

// HostedCatalogEpochCoversProviders reports whether other providers need periodic revalidation.
func (r *RuntimeRegistry) HostedCatalogEpochCoversProviders() bool {
	for _, provider := range r.providers {
		if _, mcp := provider.(*MCPProvider); !mcp && provider.ID().Kind != SourceBuiltin {
			return false
		}
	}
	return true
}

func (c *mcpCatalogRefresh) observe(scope Scope) {
	now := time.Now()
	c.mu.Lock()
	previous, exists := c.scopes[scope]
	if !exists && len(c.scopes) >= mcpCatalogMaxScopes {
		var oldest Scope
		oldestAt := now
		for candidate, seen := range c.scopes {
			if seen.Before(oldestAt) {
				oldest, oldestAt = candidate, seen
			}
		}
		delete(c.scopes, oldest)
	}
	c.scopes[scope] = now
	c.mu.Unlock()
	if previous.IsZero() {
		select {
		case c.wake <- struct{}{}:
		default:
		}
	}
}

func (c *mcpCatalogRefresh) sourceChanged(ctx context.Context, source SourceRef, err error) {
	if err != nil && c.logger != nil {
		reason, _ := ReasonOf(err)
		c.logger.WarnContext(ctx, "mcp: catalog discovery failed",
			"server", redact.ExactString(mcpSourceServerName(source)),
			"scope", source.Scope, "workspace_id", source.WorkspaceID,
			"profile_id", source.ProfileID, "reason", reason,
		)
	}
	if c.invalidate != nil {
		c.invalidate()
	}
}

// RunCatalogRefresh refreshes sources for active hosted scopes until daemon shutdown.
func (p *MCPProvider) RunCatalogRefresh(ctx context.Context) {
	if p == nil || p.catalog == nil {
		return
	}
	ticker := time.NewTicker(mcpCatalogRefreshInterval)
	defer ticker.Stop()
	var changes <-chan struct{}
	if source, ok := p.exec.(interface{ CatalogChanges() <-chan struct{} }); ok {
		changes = source.CatalogChanges()
	}
	defer func() {
		if closer, ok := p.exec.(interface{ CloseCatalogConnections() error }); ok {
			p.catalog.logSubscriptionError(ctx, closer.CloseCatalogConnections())
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-p.catalog.wake:
		case <-changes:
		case <-ticker.C:
		}
		p.refreshCatalog(ctx)
	}
}

func (p *MCPProvider) refreshCatalog(ctx context.Context) {
	sources, err := p.sources.ListMCPSources(ctx)
	if err != nil {
		return
	}
	p.retainDiscoveredSources(sources)
	if retainer, ok := p.exec.(interface{ RetainCatalogSources([]SourceRef) }); ok {
		retainer.RetainCatalogSources(sources)
	}
	now := time.Now()
	p.catalog.mu.Lock()
	maps.DeleteFunc(p.catalog.scopes, func(_ Scope, seen time.Time) bool {
		return now.Sub(seen) > mcpCatalogScopeLifetime
	})
	scopes := maps.Clone(p.catalog.scopes)
	p.catalog.mu.Unlock()
	selected := make(map[SourceRef]struct{})
	for scope := range scopes {
		for _, source := range MCPSourcesForScope(sources, scope) {
			if p.consent != nil {
				if admitted, err := p.consent(ctx, source, source.ProfileID); err != nil || !admitted {
					continue
				}
			}
			selected[source] = struct{}{}
		}
	}
	if retainer, ok := p.exec.(interface {
		RetainCatalogSubscriptions(context.Context, map[SourceRef]struct{}) error
	}); ok {
		p.catalog.logSubscriptionError(ctx, retainer.RetainCatalogSubscriptions(ctx, selected))
	}

	var workers sync.WaitGroup
	slots := make(chan struct{}, mcpCatalogParallelism)
	for source := range selected {
		if ctx.Err() != nil {
			break
		}
		slots <- struct{}{}
		workers.Go(func() {
			defer func() { <-slots }()
			p.refreshCatalogSource(ctx, source)
		})
	}
	workers.Wait()
}

func (c *mcpCatalogRefresh) logSubscriptionError(ctx context.Context, err error) {
	if err != nil && c.logger != nil {
		c.logger.WarnContext(ctx, "mcp: close catalog subscription failed")
	}
}

func (p *MCPProvider) refreshCatalogSource(ctx context.Context, source SourceRef) {
	if value, found := p.discovered.Load(source); found {
		snapshot, ok := value.(*mcpDiscoverySnapshot)
		if !ok {
			panic("invariant: discovered MCP source has an unexpected type")
		}
		snapshot.currentDescriptors()
		if generation, ok := p.exec.(mcpProjectionGenerationSource); ok {
			if _, fresh := generation.ProjectionGeneration(ctx, []SourceRef{source}); fresh {
				return
			}
		}
	}
	if _, _, err := p.listSourceDescriptors(ctx, source, source.ProfileID, p.discoveryBudget); err != nil &&
		ctx.Err() == nil {
		p.recordDiscoveryFailure(ctx, source, err)
	}
}

// CatalogDiscoveryReason reports the last discovery failure for the effective scoped server.
func (p *MCPProvider) CatalogDiscoveryReason(ctx context.Context, scope Scope, name string) string {
	sources, err := p.sources.ListMCPSources(ctx)
	if err != nil {
		return ""
	}
	for _, source := range MCPSourcesForScope(sources, scope) {
		if mcpSourceServerName(source) != name {
			continue
		}
		if failure, found := p.discoveryFailures.Load(source); found {
			switch failure {
			case ReasonCallTimedOut:
				return "Tool discovery timed out"
			case ReasonMCPUnreachable:
				return "Server unavailable during tool discovery"
			default:
				return "Tool discovery failed"
			}
		}
		if _, found := p.discovered.Load(source); !found {
			return "Waiting for tool discovery"
		}
	}
	return ""
}
