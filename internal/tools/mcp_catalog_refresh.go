package tools

import (
	"context"
	"crypto/sha256"
	"log/slog"
	"maps"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/redact"
)

const (
	mcpCatalogRefreshInterval = 3 * time.Second
	mcpCatalogParallelism     = 4
)

type mcpCatalogRefresh struct {
	mu         sync.Mutex
	scopes     map[Scope]int
	authority  func(context.Context, Scope, []SourceRef) ([]SourceRef, error)
	wake       chan struct{}
	invalidate func()
	logger     *slog.Logger
}

// WithMCPBackgroundDiscovery enables cache-only hosted reads and a daemon-owned refresh worker.
func WithMCPBackgroundDiscovery(invalidate func(), logger *slog.Logger) MCPProviderOption {
	return func(p *MCPProvider) {
		p.catalog = &mcpCatalogRefresh{
			scopes: make(map[Scope]int), wake: make(chan struct{}, 1),
			invalidate: invalidate, logger: logger,
		}
	}
}

type hostedMCPProjectionKey struct{}

// IsHostedCatalogRead reports a projection backed by the daemon's discovery worker.
func IsHostedCatalogRead(ctx context.Context) bool {
	return ctx.Value(hostedMCPProjectionKey{}) != nil
}

// HostedProjectionContext projects the catalog without discovering upstream servers.
func (r *RuntimeRegistry) HostedProjectionContext(ctx context.Context) (context.Context, bool) {
	for _, provider := range r.providers {
		if mcp, ok := provider.(*MCPProvider); ok && mcp.catalog != nil {
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

// SetHostedCatalogAuthority limits background discovery to servers admitted by a bound session.
func (p *MCPProvider) SetHostedCatalogAuthority(
	authority func(context.Context, Scope, []SourceRef) ([]SourceRef, error),
) {
	if p.catalog != nil {
		p.catalog.mu.Lock()
		p.catalog.authority = authority
		p.catalog.mu.Unlock()
	}
}

// BindHostedCatalog registers an active hosted bind and returns its release hook.
func (r *RuntimeRegistry) BindHostedCatalog(scope Scope) func() {
	for _, provider := range r.providers {
		if mcp, ok := provider.(*MCPProvider); ok && mcp.catalog != nil {
			return mcp.catalog.bind(scope)
		}
	}
	return nil
}

func (c *mcpCatalogRefresh) bind(scope Scope) func() {
	c.mu.Lock()
	c.scopes[scope]++
	first := c.scopes[scope] == 1
	c.mu.Unlock()
	if first {
		c.signal()
	}
	return sync.OnceFunc(func() {
		c.mu.Lock()
		c.scopes[scope]--
		last := c.scopes[scope] == 0
		if last {
			delete(c.scopes, scope)
		}
		c.mu.Unlock()
		if last {
			c.signal()
		}
	})
}

func (c *mcpCatalogRefresh) signal() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

func (c *mcpCatalogRefresh) sourceChanged(ctx context.Context, source SourceRef, reason ReasonCode) {
	if reason != "" && c.logger != nil {
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
		if closer, ok := p.exec.(interface{ CloseCatalogConnections(context.Context) error }); ok {
			p.catalog.logSubscriptionError(ctx, closer.CloseCatalogConnections(context.WithoutCancel(ctx)))
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
	demand := p.catalogDemand(ctx, sources)
	if retainer, ok := p.exec.(interface {
		RetainCatalogSubscriptions(context.Context, map[SourceRef]struct{}) error
	}); ok {
		selected := make(map[SourceRef]struct{}, len(demand))
		for source := range demand {
			selected[source] = struct{}{}
		}
		p.catalog.logSubscriptionError(ctx, retainer.RetainCatalogSubscriptions(ctx, selected))
	}

	var workers sync.WaitGroup
	slots := make(chan struct{}, mcpCatalogParallelism)
	for source, scopes := range demand {
		if ctx.Err() != nil {
			break
		}
		slots <- struct{}{}
		workers.Go(func() {
			defer func() { <-slots }()
			if p.catalog.hasDemand(scopes) {
				p.refreshCatalogSource(ctx, source)
			}
		})
	}
	workers.Wait()
}

func (p *MCPProvider) catalogDemand(ctx context.Context, sources []SourceRef) map[SourceRef][]Scope {
	p.catalog.mu.Lock()
	scopes := maps.Clone(p.catalog.scopes)
	authority := p.catalog.authority
	p.catalog.mu.Unlock()
	demand := make(map[SourceRef][]Scope)
	for scope := range scopes {
		admitted := MCPSourcesForScope(sources, scope)
		if authority != nil {
			var err error
			admitted, err = authority(ctx, scope, admitted)
			if err != nil {
				continue
			}
		}
		for _, source := range admitted {
			if p.consent != nil {
				if admitted, err := p.consent(ctx, source, source.ProfileID); err != nil || !admitted {
					continue
				}
			}
			demand[source] = append(demand[source], scope)
		}
	}
	return demand
}

func (c *mcpCatalogRefresh) hasDemand(scopes []Scope) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, scope := range scopes {
		if c.scopes[scope] > 0 {
			return true
		}
	}
	return false
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
		if executor, ok := p.exec.(interface {
			CatalogContentRevision(context.Context, SourceRef) ([sha256.Size]byte, bool)
		}); ok {
			if content, fresh := executor.CatalogContentRevision(ctx, source); fresh && content == snapshot.content {
				// A matching cache entry cannot prove recovery from a failed explicit check.
				// Wait for notification, fallback expiry or another explicit check.
				return
			}
		}
	}
	ctx = context.WithValue(ctx, mcpOwnedDiscoveryKey{}, true)
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
