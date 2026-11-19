package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	mcpToolListFloor      = time.Minute
	defaultMCPToolListTTL = 2 * time.Minute
	mcpSubscribedListTTL  = -1
)

type mcpToolListCache struct {
	normalizationCapacity int
	flights               map[string]chan struct{}
	normalizations        map[string]mcpToolNormalization
	publicationMu         sync.Mutex
	attempts              map[string]mcpListAttempt
	mu                    sync.Mutex
	entries               map[string]mcpToolListCacheEntry
	projectionStates      map[string]mcpToolProjectionState
	polls                 map[string]mcpCatalogPoll
}

type mcpListAttempt struct {
	check, list uint64
	active      int
}

func (e *CallExecutor) beginToolList(key string, fresh bool) mcpListAttempt {
	e.toolCache.publicationMu.Lock()
	defer e.toolCache.publicationMu.Unlock()
	if e.toolCache.attempts == nil {
		e.toolCache.attempts = make(map[string]mcpListAttempt)
	}
	attempt := e.toolCache.attempts[key]
	if fresh {
		attempt.check++
	}
	attempt.list++
	attempt.active++
	e.toolCache.attempts[key] = attempt
	return attempt
}

func (e *CallExecutor) finishToolList(key string) {
	e.toolCache.publicationMu.Lock()
	defer e.toolCache.publicationMu.Unlock()
	current := e.toolCache.attempts[key]
	current.active--
	if current.active == 0 {
		delete(e.toolCache.attempts, key)
	} else {
		e.toolCache.attempts[key] = current
	}
}

func (e *CallExecutor) publishToolList(key, pollKey string, attempt mcpListAttempt, fresh bool,
	listing mcpToolListCacheEntry, ttlMs int,
	projectionKey string, stdio, background bool, subscribed func() bool, owner *catalogNotificationStream,
) {
	e.toolCache.publicationMu.Lock()
	defer e.toolCache.publicationMu.Unlock()
	if owner != nil && owner.revoked.Load() {
		return
	}
	current := e.toolCache.attempts[key]
	if current.check != attempt.check || (!fresh && current.list != attempt.list) {
		return
	}
	if fresh {
		current.list++
		e.toolCache.attempts[key] = current
	}
	now := time.Now()
	fallbackTTL := e.effectiveToolListTTL(ttlMs)
	if subscribed != nil && subscribed() {
		ttlMs = mcpSubscribedListTTL
	}
	if stdio {
		e.pruneStdioCatalogRevisions(key)
	}
	e.cacheTools(key, listing, ttlMs, now)
	e.recordToolProjectionState(projectionKey, key, listing.discovery.content, e.effectiveToolListTTL(ttlMs), now)
	if background || e.catalogHTTP != nil || e.catalogPoll(pollKey).interval != 0 {
		e.recordCatalogListing(key, pollKey, fallbackTTL, ttlMs != mcpSubscribedListTTL, now)
	}
}

func (e *CallExecutor) pruneStdioCatalogRevisions(key string) {
	end := strings.LastIndexByte(key, ':') + 1
	if end == 0 {
		return
	}
	prefix := key[:end]
	revision, err := strconv.ParseUint(key[end:], 10, 64)
	if err != nil {
		return
	}
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	older := func(candidate string) bool {
		if !strings.HasPrefix(candidate, prefix) {
			return false
		}
		previous, err := strconv.ParseUint(candidate[end:], 10, 64)
		return err == nil && previous < revision
	}
	maps.DeleteFunc(e.toolCache.entries, func(candidate string, _ mcpToolListCacheEntry) bool {
		return older(candidate)
	})
	maps.DeleteFunc(e.toolCache.normalizations, func(candidate string, _ mcpToolNormalization) bool {
		return older(candidate)
	})
	maps.DeleteFunc(e.toolCache.projectionStates, func(_ string, state mcpToolProjectionState) bool {
		return older(state.listingKey)
	})
}

func (e *CallExecutor) invalidateToolListing(key string, notified bool) {
	e.toolCache.publicationMu.Lock()
	defer e.toolCache.publicationMu.Unlock()
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	_, published := e.toolCache.entries[key]
	_, listing := e.toolCache.attempts[key]
	_, polling := e.toolCache.polls[key]
	// Establishment before the first list starts only wakes the worker. Invalidation
	// matters once a list can have been fetched before this subscription was live.
	if !notified && !published && !listing && !polling {
		return
	}
	for _, binding := range []string{key, catalogPollAttemptKey(key)} {
		if attempt, ok := e.toolCache.attempts[binding]; ok {
			attempt.check++
			e.toolCache.attempts[binding] = attempt
		}
	}
	delete(e.toolCache.polls, key)
	e.dropToolListingLocked(key)
}

func (e *CallExecutor) dropToolListingLocked(key string) {
	delete(e.toolCache.entries, key)
	maps.DeleteFunc(e.toolCache.projectionStates, func(_ string, state mcpToolProjectionState) bool {
		return state.listingKey == key
	})
}

type mcpToolListCacheEntry struct {
	source    toolspkg.SourceRef
	discovery ToolDiscovery
	raw       []*mcpsdk.Tool
	expiresAt time.Time
	listedAt  time.Time
}

type mcpToolProjectionState struct {
	listingKey  string
	generation  uint64
	fingerprint [sha256.Size]byte
	expiresAt   time.Time
}

func (e *CallExecutor) cachedTools(key string, now time.Time) (ToolDiscovery, bool) {
	entry, ok := e.cachedToolListing(key, now)
	return entry.discovery.clone(), ok
}

func (e *CallExecutor) cachedToolListing(key string, now time.Time) (mcpToolListCacheEntry, bool) {
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	e.pruneToolProjectionStatesLocked(now)
	entry, ok := e.toolCache.entries[key]
	if !ok || toolListExpired(entry.expiresAt, now) {
		delete(e.toolCache.entries, key)
		return mcpToolListCacheEntry{}, false
	}
	return entry, true
}

func (e *CallExecutor) cacheTools(
	key string, listing mcpToolListCacheEntry, ttlMs int, now time.Time,
) {
	ttl := e.effectiveToolListTTL(ttlMs)
	e.toolCache.mu.Lock()
	if e.toolCache.entries == nil {
		e.toolCache.entries = make(map[string]mcpToolListCacheEntry)
	}
	e.pruneToolProjectionStatesLocked(now)
	maps.DeleteFunc(e.toolCache.entries, func(_ string, entry mcpToolListCacheEntry) bool {
		return toolListExpired(entry.expiresAt, now)
	})
	if _, exists := e.toolCache.entries[key]; !exists &&
		len(e.toolCache.entries) >= defaultToolNormalizationCacheCapacity {
		var oldest string
		var eviction mcpToolListCacheEntry
		for candidate, entry := range e.toolCache.entries {
			if oldest == "" || expiresBefore(entry, eviction) {
				oldest, eviction = candidate, entry
			}
		}
		e.dropToolListingLocked(oldest)
		delete(e.toolCache.normalizations, oldest)
	}
	listing.discovery = listing.discovery.clone()
	listing.expiresAt = toolListExpiry(now, ttl)
	listing.listedAt = now
	e.toolCache.entries[key] = listing
	e.toolCache.mu.Unlock()
}

func expiresBefore(candidate, current mcpToolListCacheEntry) bool {
	switch {
	case candidate.expiresAt.IsZero() && current.expiresAt.IsZero():
		return candidate.listedAt.Before(current.listedAt)
	case candidate.expiresAt.IsZero():
		return false
	case current.expiresAt.IsZero():
		return true
	default:
		return candidate.expiresAt.Before(current.expiresAt)
	}
}

// RetainCatalogSources releases descriptors owned by removed or superseded declarations.
func (e *CallExecutor) RetainCatalogSources(sources []toolspkg.SourceRef) {
	active := make(map[toolspkg.SourceRef]bool, len(sources))
	for _, source := range sources {
		active[source] = true
	}
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	for key, entry := range e.toolCache.entries {
		if !active[entry.source] {
			e.dropToolListingLocked(key)
		}
	}
	maps.DeleteFunc(e.toolCache.normalizations, func(_ string, normalization mcpToolNormalization) bool {
		return !active[normalization.source]
	})
}

func (e *CallExecutor) effectiveToolListTTL(ttlMs int) time.Duration {
	if ttlMs == mcpSubscribedListTTL {
		return 0
	}
	floor := e.toolListFloor
	if floor <= 0 {
		if ttlMs <= 0 {
			return defaultMCPToolListTTL
		}
		floor = mcpToolListFloor
	}
	return max(floor, time.Duration(ttlMs)*time.Millisecond)
}

func toolListExpiry(now time.Time, ttl time.Duration) time.Time {
	if ttl == 0 {
		return time.Time{}
	}
	return now.Add(ttl)
}

func toolListExpired(expiresAt, now time.Time) bool {
	return !expiresAt.IsZero() && !now.Before(expiresAt)
}

func cloneMCPToolDescriptors(descriptors []toolspkg.MCPToolDescriptor) []toolspkg.MCPToolDescriptor {
	cloned := append([]toolspkg.MCPToolDescriptor(nil), descriptors...)
	for index := range cloned {
		cloned[index].InputSchema = append(json.RawMessage(nil), cloned[index].InputSchema...)
		cloned[index].OutputSchema = append(json.RawMessage(nil), cloned[index].OutputSchema...)
	}
	return cloned
}

// ProjectionGeneration reports the exact live tool-list cache generation for configured sources.
func (e *CallExecutor) ProjectionGeneration(
	ctx context.Context,
	sources []toolspkg.SourceRef,
) (string, bool) {
	if e == nil || ctx == nil || ctx.Err() != nil {
		return "", false
	}
	keys := make([]string, 0, len(sources))
	for i := range sources {
		key, ok := e.projectionSourceKey(ctx, sources[i])
		if !ok {
			return "", false
		}
		keys = append(keys, key)
	}
	slices.Sort(keys)

	now := time.Now()
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	e.pruneToolProjectionStatesLocked(now)
	var generation strings.Builder
	for _, key := range keys {
		state, ok := e.toolCache.projectionStates[key]
		if !ok {
			return "", false
		}
		fmt.Fprintf(&generation, "%d:%s%d:%x;", len(key), key, state.generation, state.fingerprint)
	}
	return generation.String(), true
}

func (e *CallExecutor) recordToolProjectionState(
	key, listingKey string, fingerprint [sha256.Size]byte, ttl time.Duration, now time.Time,
) {
	e.toolCache.mu.Lock()
	if e.toolCache.projectionStates == nil {
		e.toolCache.projectionStates = make(map[string]mcpToolProjectionState)
	}
	e.pruneToolProjectionStatesLocked(now)
	state := e.toolCache.projectionStates[key]
	state.listingKey = listingKey
	if state.fingerprint != fingerprint {
		state.generation++
	}
	if state.generation == 0 {
		state.generation = 1
	}
	state.fingerprint = fingerprint
	state.expiresAt = toolListExpiry(now, ttl)
	e.toolCache.projectionStates[key] = state
	e.toolCache.mu.Unlock()
}

// CatalogContentRevision returns the current cached content for this exact authorized binding.
func (e *CallExecutor) CatalogContentRevision(
	ctx context.Context, source toolspkg.SourceRef,
) ([sha256.Size]byte, bool) {
	key, ok := e.projectionSourceKey(ctx, source)
	if !ok {
		return [sha256.Size]byte{}, false
	}
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	state, found := e.toolCache.projectionStates[key]
	return state.fingerprint, found && !toolListExpired(state.expiresAt, time.Now())
}

func (e *CallExecutor) projectionSourceKey(
	ctx context.Context,
	source toolspkg.SourceRef,
) (string, bool) {
	if e.servers == nil {
		return mcpProjectionSourceKey(source, "", ""), true
	}
	resolved, err := e.resolveServer(ctx, source, "")
	if err != nil {
		return "", false
	}
	header, err := e.authorizationHeader(ctx, &resolved)
	if err != nil {
		return "", false
	}
	if err := e.prepareToolListing(ctx, &resolved); err != nil {
		return "", false
	}
	if e.stdioPool != nil && resolved.listingSpec != nil {
		if _, ok := e.stdioPool.knownProtocol(*resolved.listingSpec); !ok {
			return "", false
		}
	}
	return mcpProjectionSourceKey(source, header, resolved.listingBinding), true
}

func (e *CallExecutor) pruneToolProjectionStatesLocked(now time.Time) {
	maps.DeleteFunc(e.toolCache.projectionStates, func(_ string, state mcpToolProjectionState) bool {
		return toolListExpired(state.expiresAt, now)
	})
}

func mcpProjectionSourceKey(source toolspkg.SourceRef, authorizationHeader, binding string) string {
	source = toolspkg.MCPDeclarationSource(source)
	authBinding := sha256.Sum256([]byte(authorizationHeader))
	parts := []string{
		string(source.Kind),
		source.Owner,
		source.RawServerName,
		source.RawToolName,
		source.Scope,
		source.WorkspaceID,
		source.ProfileID,
		source.ResourceID,
		source.ResourceVersion,
		hex.EncodeToString(authBinding[:]),
	}
	var key strings.Builder
	for _, part := range parts {
		fmt.Fprintf(&key, "%d:%s", len(part), part)
	}
	if binding != "" {
		fmt.Fprintf(&key, "%d:%s", len(binding), binding)
	}
	return key.String()
}

func (e *CallExecutor) toolCacheKey(
	ctx context.Context,
	resolved *ResolvedServer,
	protocolVersion string,
	authorizationHeader string,
) (string, error) {
	if resolved.Server.EffectiveTransport() == compozyconfig.MCPServerTransportHTTP {
		// An HTTP listing carries its negotiated protocol until this exact binding expires.
		protocolVersion = ""
	}
	targetKey, err := resolved.Target.Key()
	if err != nil {
		return "", fmt.Errorf("mcp: cache target key: %w", err)
	}
	definition, err := json.Marshal(resolved.Server)
	if err != nil {
		return "", fmt.Errorf("mcp: encode cache server definition: %w", err)
	}
	sum := sha256.Sum256(definition)
	fingerprint := hex.EncodeToString(sum[:])
	if resolved.Server.Auth.Enabled() {
		cfg, err := mcpauth.ServerConfigFromMCP(ctx, resolved.Target, resolved.Server, nil)
		if err != nil {
			return "", fmt.Errorf("mcp: cache server fingerprint: %w", err)
		}
		authFingerprint, err := mcpauth.ServerDefinitionFingerprint(cfg)
		if err != nil {
			return "", fmt.Errorf("mcp: cache server fingerprint: %w", err)
		}
		fingerprint += ":" + authFingerprint
	}
	authBinding := sha256.Sum256([]byte(authorizationHeader))
	key := targetKey + "\x00" +
		fingerprint + "\x00" + string(resolved.overrideOrigin()) + "\x00" +
		protocolVersion + "\x00" +
		hex.EncodeToString(authBinding[:]) + "\x00" + resolved.listingBinding
	return key, nil
}

// cachedPooledStdioTools answers discovery from the tool cache without touching the child, using
// the protocol the pool negotiated with the same launch identity.
func (e *CallExecutor) cachedPooledStdioTools(
	ctx context.Context,
	resolved *ResolvedServer,
	source toolspkg.SourceRef,
	authorizationHeader string,
) (ToolDiscovery, bool) {
	if e.stdioPool == nil || resolved.Server.EffectiveTransport() != compozyconfig.MCPServerTransportStdio {
		return ToolDiscovery{}, false
	}
	spec := resolved.listingSpec
	if spec == nil {
		return ToolDiscovery{}, false
	}
	protocolVersion, ok := e.stdioPool.knownProtocol(*spec)
	if !ok {
		return ToolDiscovery{}, false
	}
	key, err := e.toolCacheKey(ctx, resolved, protocolVersion, authorizationHeader)
	if err != nil {
		return ToolDiscovery{}, false
	}
	entry, ok := e.cachedToolListing(key, time.Now())
	if !ok {
		return ToolDiscovery{}, false
	}
	discovery, err := e.normalizeToolListing(key, source, resolved, protocolVersion, entry.raw)
	return discovery, err == nil
}

func negotiatedProtocolVersion(session *mcpsdk.ClientSession) string {
	if session == nil || session.InitializeResult() == nil {
		return ""
	}
	return session.InitializeResult().ProtocolVersion
}
