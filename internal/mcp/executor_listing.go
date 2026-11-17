package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpToolNormalization struct {
	fingerprint [sha256.Size]byte
	revision    [sha256.Size]byte
	source      tools.SourceRef
	runtimeName string
	owner       string
	catalog     string
	readOnly    []string
	origin      tools.MCPReadOnlyOverrideOrigin
	raw         []*mcpsdk.Tool
	discovery   ToolDiscovery
	usedAt      time.Time
}

func (e *CallExecutor) acquireToolListing(ctx context.Context, key string) (func(), error) {
	for {
		e.toolCache.mu.Lock()
		if e.toolCache.flights == nil {
			e.toolCache.flights = make(map[string]chan struct{})
		}
		done, joined := e.toolCache.flights[key]
		if !joined {
			done = make(chan struct{})
			e.toolCache.flights[key] = done
		}
		e.toolCache.mu.Unlock()
		if !joined {
			return func() {
				e.toolCache.mu.Lock()
				delete(e.toolCache.flights, key)
				close(done)
				e.toolCache.mu.Unlock()
			}, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-done:
		}
	}
}

func (e *CallExecutor) normalizeToolListing(
	key string, source tools.SourceRef, resolved *ResolvedServer, protocolVersion string, raw []*mcpsdk.Tool,
) (ToolDiscovery, error) {
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	cached, exists := e.toolCache.normalizations[key]
	revision := redact.SecretFingerprint()
	if exists && cached.matches(source, resolved, protocolVersion, raw, revision) {
		return cached.discovery.clone(), nil
	}
	encoded, err := json.Marshal(struct {
		Source                                       tools.SourceRef
		Tools                                        []*mcpsdk.Tool
		ReadOnly                                     []string
		Origin                                       tools.MCPReadOnlyOverrideOrigin
		RuntimeName, Owner, Catalog, ProtocolVersion string
	}{source, raw, resolved.Server.ReadOnlyTools, resolved.overrideOrigin(),
		resolved.Server.EffectiveRuntimeName(), resolved.Server.Owner, resolved.Server.CatalogEntry, protocolVersion})
	if err != nil {
		return ToolDiscovery{}, fmt.Errorf("mcp: encode tool listing: %w", err)
	}
	fingerprint := sha256.Sum256(encoded)
	if exists && cached.fingerprint == fingerprint && cached.revision == revision {
		cached.usedAt = time.Now()
		e.toolCache.normalizations[key] = cached
		return cached.discovery.clone(), nil
	}
	discovery, err := e.descriptorsFromTools(source, resolved, protocolVersion, raw)
	if err != nil {
		return ToolDiscovery{}, err
	}
	if e.toolCache.normalizations == nil {
		e.toolCache.normalizations = make(map[string]mcpToolNormalization)
	}
	// Retain at most one bounded set of source revisions, including raw input for secret changes.
	if !exists && len(e.toolCache.normalizations) >= 128 {
		var oldest string
		var usedAt time.Time
		for candidate, entry := range e.toolCache.normalizations {
			if oldest == "" || entry.usedAt.Before(usedAt) {
				oldest, usedAt = candidate, entry.usedAt
			}
		}
		delete(e.toolCache.normalizations, oldest)
	}
	e.toolCache.normalizations[key] = mcpToolNormalization{
		fingerprint: fingerprint, revision: revision, raw: raw, source: source,
		readOnly: slices.Clone(resolved.Server.ReadOnlyTools), origin: resolved.overrideOrigin(),
		runtimeName: resolved.Server.EffectiveRuntimeName(), owner: resolved.Server.Owner,
		catalog:   resolved.Server.CatalogEntry,
		discovery: discovery.clone(), usedAt: time.Now(),
	}
	return discovery, nil
}

func (cached mcpToolNormalization) matches(
	source tools.SourceRef, resolved *ResolvedServer, protocolVersion string, raw []*mcpsdk.Tool,
	revision [sha256.Size]byte,
) bool {
	return cached.source == source && slices.Equal(cached.raw, raw) && cached.revision == revision &&
		cached.discovery.ProtocolVersion == protocolVersion &&
		slices.Equal(cached.readOnly, resolved.Server.ReadOnlyTools) && cached.origin == resolved.overrideOrigin() &&
		cached.runtimeName == resolved.Server.EffectiveRuntimeName() && cached.owner == resolved.Server.Owner &&
		cached.catalog == resolved.Server.CatalogEntry
}

func (e *CallExecutor) listResolvedTools(
	ctx context.Context, source tools.SourceRef,
) (discovery ToolDiscovery, err error) {
	resolved, err := e.resolveServer(ctx, source, "")
	if err != nil {
		return ToolDiscovery{}, err
	}
	if e.consent != nil {
		if err := e.consent(ctx, source, resolved); err != nil {
			return ToolDiscovery{}, err
		}
	}
	healthObservation := e.runtimeHealth.Begin(resolved.HealthKey)
	var runtimeHealthErr error
	servedFromCache := false
	defer func() {
		if servedFromCache {
			// A cached answer never touched the server, so it proves nothing about its health.
			return
		}
		if runtimeHealthErr != nil {
			e.runtimeHealth.RecordFailure(healthObservation, runtimeHealthErr)
			return
		}
		e.runtimeHealth.RecordSuccess(healthObservation)
	}()
	if err := e.ensureAuthorized(ctx, &resolved); err != nil {
		runtimeHealthErr = err
		return ToolDiscovery{}, err
	}
	authorizationHeader := e.authorizationHeader(ctx, &resolved)
	if err = e.prepareToolListing(ctx, &resolved); err != nil {
		runtimeHealthErr = err
		return ToolDiscovery{}, err
	}
	if !tools.FreshMCPDiscovery(ctx) {
		if cached, ok := e.cachedHTTPTools(ctx, &resolved, source, authorizationHeader); ok {
			servedFromCache = true
			return cached, nil
		}
	}
	if cached, ok := e.cachedPooledStdioTools(
		ctx,
		&resolved,
		source,
		authorizationHeader,
	); ok &&
		!tools.FreshMCPDiscovery(ctx) {
		servedFromCache = true
		return cached, nil
	}
	client, err := e.openRenderedClient(ctx, &resolved, authorizationHeader)
	if err != nil {
		runtimeHealthErr = err
		return ToolDiscovery{}, normalizeMCPErrorWithContext(ctx, "", err, true)
	}
	defer func() {
		err = joinMCPClientCleanup(err, client)
	}()
	discovery, runtimeHealthErr, err = e.listToolsFromClient(
		ctx, client, source, &resolved, negotiatedProtocolVersion(client.session), authorizationHeader,
	)
	if err != nil {
		return ToolDiscovery{}, err
	}
	return discovery, nil
}

// listToolsFromClient answers discovery over an open client, preferring a cached projection. The
// returned health error is the failure, if any, that the server itself caused.
func (e *CallExecutor) listToolsFromClient(
	ctx context.Context,
	client *managedMCPClient,
	source tools.SourceRef,
	resolved *ResolvedServer,
	protocolVersion string,
	authorizationHeader string,
) (ToolDiscovery, error, error) {
	cacheKey, err := e.toolCacheKey(ctx, resolved, protocolVersion, authorizationHeader)
	if err != nil {
		return ToolDiscovery{}, nil, err
	}
	fresh := tools.FreshMCPDiscovery(ctx)
	if !fresh {
		release, err := e.acquireToolListing(ctx, cacheKey)
		if err != nil {
			return ToolDiscovery{}, nil, err
		}
		defer release()
		if entry, ok := e.cachedToolListing(
			cacheKey,
			time.Now(),
		); ok &&
			entry.discovery.ProtocolVersion == protocolVersion {
			discovery, err := e.normalizeToolListing(cacheKey, source, resolved, protocolVersion, entry.raw)
			return discovery, nil, err
		}
	}
	attempt := e.beginToolList(cacheKey, fresh)
	defer e.finishToolList(cacheKey)
	raw, ttlMs, err := listMCPTools(ctx, client.session)
	if err != nil {
		return ToolDiscovery{}, err, normalizeMCPErrorWithContext(ctx, "", err, true)
	}
	discovery, err := e.normalizeToolListing(cacheKey, source, resolved, protocolVersion, raw)
	if err != nil {
		return ToolDiscovery{}, err, err
	}
	if err := e.publishToolList(
		cacheKey,
		attempt,
		fresh,
		discovery,
		raw,
		ttlMs,
		mcpProjectionSourceKey(source, authorizationHeader, resolved.listingBinding),
	); err != nil {
		return ToolDiscovery{}, nil, err
	}
	return discovery, nil, nil
}

func (e *CallExecutor) prepareToolListing(ctx context.Context, resolved *ResolvedServer) error {
	if resolved.Server.EffectiveTransport() == compozyconfig.MCPServerTransportHTTP {
		if err := e.renderHTTPServer(ctx, resolved); err != nil {
			return err
		}
		if len(resolved.Server.SecretHeaders) == 0 && len(resolved.Server.InputValues) == 0 {
			return nil
		}
		data, err := json.Marshal(struct {
			Server  compozyconfig.MCPServer
			Headers map[string][]string
		}{resolved.Server, resolved.requestHeaders})
		if err != nil {
			return fmt.Errorf("mcp: encode resolved listing identity: %w", err)
		}
		// Only the digest enters cache keys; the transport uses this same resolved material.
		resolved.listingBinding = fmt.Sprintf("%x", sha256.Sum256(data))
		return nil
	}
	spec, err := e.resolveMCPStdioSpec(ctx, resolved.Server, resolved.Target)
	if err != nil {
		return err
	}
	resolved.listingSpec = &spec
	resolved.listingBinding = spec.digest
	return nil
}

func (e *CallExecutor) cachedHTTPTools(
	ctx context.Context, resolved *ResolvedServer, source tools.SourceRef, authorizationHeader string,
) (ToolDiscovery, bool) {
	if resolved.Server.EffectiveTransport() != compozyconfig.MCPServerTransportHTTP {
		return ToolDiscovery{}, false
	}
	key, err := e.toolCacheKey(ctx, resolved, "", authorizationHeader)
	if err != nil {
		return ToolDiscovery{}, false
	}
	entry, ok := e.cachedToolListing(key, time.Now())
	if !ok {
		return ToolDiscovery{}, false
	}
	discovery, err := e.normalizeToolListing(key, source, resolved, entry.discovery.ProtocolVersion, entry.raw)
	return discovery, err == nil
}
