package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const defaultToolNormalizationCacheCapacity = 128

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
	source = tools.MCPDeclarationSource(source)
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	for range 2 {
		snapshot := redact.RegisteredSecretSnapshot()
		discovery, err := e.normalizeToolListingAt(key, source, resolved, protocolVersion, raw, snapshot)
		if err != nil || snapshot.MaterialFingerprint() == redact.RegisteredSecretSnapshot().MaterialFingerprint() {
			return discovery, err
		}
	}
	return ToolDiscovery{}, tools.NewValidationError(
		"listing", tools.ReasonSecretMetadata, "mcp tool metadata changed during redaction",
	)
}

func (e *CallExecutor) normalizeToolListingAt(
	key string, source tools.SourceRef, resolved *ResolvedServer, protocolVersion string, raw []*mcpsdk.Tool,
	snapshot *redact.SecretSnapshot,
) (ToolDiscovery, error) {
	cached, exists := e.toolCache.normalizations[key]
	revision := snapshot.MaterialFingerprint()
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
	discovery, err := e.descriptorsFromTools(source, resolved, protocolVersion, raw, snapshot)
	if err != nil {
		return ToolDiscovery{}, err
	}
	if e.toolCache.normalizations == nil {
		e.toolCache.normalizations = make(map[string]mcpToolNormalization)
	}
	// Retain at most one bounded set of source revisions, including raw input for secret changes.
	if !exists && len(e.toolCache.normalizations) >= e.toolCache.normalizationCapacity {
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
		e.recordBearerRejection(ctx, &resolved, runtimeHealthErr)
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
	authorizationHeader, err := e.prepareAuthorization(ctx, &resolved)
	if err != nil {
		runtimeHealthErr = err
		return ToolDiscovery{}, err
	}
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
	discovery, runtimeHealthErr, err = e.listToolsWithCatalogClient(ctx, source, &resolved, authorizationHeader)
	return discovery, err
}

func (e *CallExecutor) listToolsWithCatalogClient(
	ctx context.Context, source tools.SourceRef, resolved *ResolvedServer, authorizationHeader string,
) (discovery ToolDiscovery, healthErr error, err error) {
	client, err := e.openToolListingClient(ctx, resolved, authorizationHeader)
	if err != nil && ctx.Err() == nil && e.catalogHTTP != nil &&
		resolved.Server.EffectiveTransport() == compozyconfig.MCPServerTransportHTTP {
		client, err = e.openRenderedClient(ctx, resolved, authorizationHeader)
	}
	if err != nil {
		return ToolDiscovery{}, err, normalizeMCPErrorWithContext(ctx, "", err, true)
	}
	defer func() {
		err = joinMCPClientCleanup(err, client)
	}()
	discovery, healthErr, err = e.listToolsFromClient(
		ctx, client, source, resolved, negotiatedProtocolVersion(client.session), authorizationHeader,
	)
	if client.borrowed && healthErr != nil && ctx.Err() == nil {
		retireErr := e.catalogHTTP.forget(client.session)
		if reason, ok := tools.ReasonOf(err); ok && reason == tools.ReasonMCPAuthRequired {
			return discovery, healthErr, errors.Join(err, retireErr)
		}
		replacement, openErr := e.openRenderedClient(ctx, resolved, authorizationHeader)
		if openErr != nil {
			return ToolDiscovery{}, openErr, errors.Join(
				normalizeMCPErrorWithContext(ctx, "", openErr, true),
				retireErr,
			)
		}
		client = replacement
		discovery, healthErr, err = e.listToolsFromClient(
			ctx, client, source, resolved, negotiatedProtocolVersion(client.session), authorizationHeader,
		)
		if err != nil {
			err = errors.Join(err, retireErr)
		}
	}
	return discovery, healthErr, err
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
			return ToolDiscovery{}, nil, normalizeMCPErrorWithContext(ctx, "", err, true)
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
	exchangeCtx, finishExchange := e.exchangeContext(ctx, client)
	defer finishExchange()
	raw, ttlMs, err := listMCPTools(exchangeCtx, client.session)
	client.observeExchange(exchangeCtx, err)
	if err != nil {
		return ToolDiscovery{}, err, normalizeMCPErrorWithContext(exchangeCtx, "", err, true)
	}
	discovery, err := e.normalizeToolListing(cacheKey, source, resolved, protocolVersion, raw)
	if err != nil {
		return ToolDiscovery{}, err, err
	}
	stdio := e.stdioPool != nil && resolved.listingSpec != nil
	if client.borrowed || (stdio && e.stdioPool.catalogSubscribed(*resolved.listingSpec)) {
		ttlMs = mcpSubscribedListTTL
	}
	if err := e.publishToolList(
		cacheKey,
		attempt,
		fresh,
		discovery,
		raw,
		ttlMs,
		mcpProjectionSourceKey(source, authorizationHeader, resolved.listingBinding),
		stdio,
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
			Server                  compozyconfig.MCPServer
			Headers                 map[string][]string
			RequireResourceMetadata bool
		}{resolved.Server, resolved.requestHeaders, resolved.requireResourceMetadata})
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
	if e.stdioPool != nil {
		resolved.listingBinding += ":" + strconv.FormatUint(e.stdioPool.catalogRevision(spec), 10)
	}
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
