package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpToolListCache struct {
	publicationMu    sync.Mutex
	attempts         map[string]mcpListAttempt
	mu               sync.Mutex
	entries          map[string]mcpToolListCacheEntry
	projectionStates map[string]mcpToolProjectionState
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

func (e *CallExecutor) publishToolList(key string, attempt mcpListAttempt, fresh bool,
	source toolspkg.SourceRef, authorizationHeader string, discovery ToolDiscovery, ttlMs int,
) error {
	e.toolCache.publicationMu.Lock()
	defer e.toolCache.publicationMu.Unlock()
	current := e.toolCache.attempts[key]
	if current.check != attempt.check || (!fresh && current.list != attempt.list) {
		return nil
	}
	if fresh {
		current.list++
		e.toolCache.attempts[key] = current
	}
	now := time.Now()
	e.cacheTools(key, discovery, ttlMs, now)
	return e.recordToolProjection(source, authorizationHeader, discovery.Descriptors, ttlMs, now)
}

type mcpToolListCacheEntry struct {
	discovery ToolDiscovery
	expiresAt time.Time
}

type mcpToolProjectionState struct {
	generation  uint64
	fingerprint [sha256.Size]byte
	expiresAt   time.Time
}

func (e *CallExecutor) cachedTools(key string, now time.Time) (ToolDiscovery, bool) {
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	e.pruneToolProjectionStatesLocked(now)
	entry, ok := e.toolCache.entries[key]
	if !ok || !now.Before(entry.expiresAt) {
		delete(e.toolCache.entries, key)
		return ToolDiscovery{}, false
	}
	discovery := entry.discovery
	discovery.Descriptors = cloneMCPToolDescriptors(discovery.Descriptors)
	return discovery, true
}

func (e *CallExecutor) cacheTools(key string, discovery ToolDiscovery, ttlMs int, now time.Time) {
	if ttlMs <= 0 {
		// An uncacheable answer still supersedes what an earlier listing cached for this key.
		e.toolCache.mu.Lock()
		delete(e.toolCache.entries, key)
		e.toolCache.mu.Unlock()
		return
	}
	e.toolCache.mu.Lock()
	if e.toolCache.entries == nil {
		e.toolCache.entries = make(map[string]mcpToolListCacheEntry)
	}
	e.pruneToolProjectionStatesLocked(now)
	maps.DeleteFunc(e.toolCache.entries, func(_ string, entry mcpToolListCacheEntry) bool {
		return !now.Before(entry.expiresAt)
	})
	discovery.Descriptors = cloneMCPToolDescriptors(discovery.Descriptors)
	e.toolCache.entries[key] = mcpToolListCacheEntry{
		discovery: discovery,
		expiresAt: now.Add(time.Duration(ttlMs) * time.Millisecond),
	}
	e.toolCache.mu.Unlock()
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

func (e *CallExecutor) recordToolProjection(
	source toolspkg.SourceRef,
	authorizationHeader string,
	descriptors []toolspkg.MCPToolDescriptor,
	ttlMs int,
	now time.Time,
) error {
	key := mcpProjectionSourceKey(source, authorizationHeader)
	if ttlMs <= 0 {
		e.toolCache.mu.Lock()
		e.pruneToolProjectionStatesLocked(now)
		delete(e.toolCache.projectionStates, key)
		e.toolCache.mu.Unlock()
		return nil
	}
	fingerprint, err := fingerprintMCPToolDescriptors(descriptors)
	if err != nil {
		return err
	}
	e.toolCache.mu.Lock()
	if e.toolCache.projectionStates == nil {
		e.toolCache.projectionStates = make(map[string]mcpToolProjectionState)
	}
	e.pruneToolProjectionStatesLocked(now)
	state := e.toolCache.projectionStates[key]
	if state.fingerprint != fingerprint {
		state.generation++
	}
	if state.generation == 0 {
		state.generation = 1
	}
	state.fingerprint = fingerprint
	state.expiresAt = now.Add(time.Duration(ttlMs) * time.Millisecond)
	e.toolCache.projectionStates[key] = state
	e.toolCache.mu.Unlock()
	return nil
}

func fingerprintMCPToolDescriptors(descriptors []toolspkg.MCPToolDescriptor) ([sha256.Size]byte, error) {
	canonical := make([]json.RawMessage, 0, len(descriptors))
	for index := range descriptors {
		encoded, err := json.Marshal(descriptors[index])
		if err != nil {
			return [sha256.Size]byte{}, fmt.Errorf("mcp: encode tool projection descriptor: %w", err)
		}
		canonical = append(canonical, encoded)
	}
	slices.SortFunc(canonical, func(left, right json.RawMessage) int {
		return bytes.Compare(left, right)
	})
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return [sha256.Size]byte{}, fmt.Errorf("mcp: encode tool projection generation: %w", err)
	}
	return sha256.Sum256(encoded), nil
}

func (e *CallExecutor) projectionSourceKey(
	ctx context.Context,
	source toolspkg.SourceRef,
) (string, bool) {
	if e.servers == nil {
		return mcpProjectionSourceKey(source, ""), true
	}
	resolved, err := e.resolveServer(ctx, source, "")
	if err != nil {
		return "", false
	}
	return mcpProjectionSourceKey(source, e.authorizationHeader(ctx, &resolved)), true
}

func (e *CallExecutor) pruneToolProjectionStatesLocked(now time.Time) {
	maps.DeleteFunc(e.toolCache.projectionStates, func(_ string, state mcpToolProjectionState) bool {
		return !now.Before(state.expiresAt)
	})
}

func mcpProjectionSourceKey(source toolspkg.SourceRef, authorizationHeader string) string {
	authBinding := sha256.Sum256([]byte(authorizationHeader))
	parts := []string{
		string(source.Kind),
		source.Owner,
		source.RawServerName,
		source.RawToolName,
		source.Scope,
		source.WorkspaceID,
		source.ResourceID,
		source.ResourceVersion,
		hex.EncodeToString(authBinding[:]),
	}
	var key strings.Builder
	for _, part := range parts {
		fmt.Fprintf(&key, "%d:%s", len(part), part)
	}
	return key.String()
}

func (e *CallExecutor) toolCacheKey(
	ctx context.Context,
	resolved *ResolvedServer,
	protocolVersion string,
	authorizationHeader string,
) (string, error) {
	targetKey, err := resolved.Target.Key()
	if err != nil {
		return "", fmt.Errorf("mcp: cache target key: %w", err)
	}
	var fingerprint string
	if resolved.Server.Auth.Enabled() {
		cfg, err := mcpauth.ServerConfigFromMCP(ctx, resolved.Target, resolved.Server, nil)
		if err != nil {
			return "", fmt.Errorf("mcp: cache server fingerprint: %w", err)
		}
		fingerprint, err = mcpauth.ServerDefinitionFingerprint(cfg)
		if err != nil {
			return "", fmt.Errorf("mcp: cache server fingerprint: %w", err)
		}
	} else {
		definition, err := json.Marshal(resolved.Server)
		if err != nil {
			return "", fmt.Errorf("mcp: encode cache server definition: %w", err)
		}
		sum := sha256.Sum256(definition)
		fingerprint = hex.EncodeToString(sum[:])
	}
	authBinding := sha256.Sum256([]byte(authorizationHeader))
	key := targetKey + "\x00" +
		fingerprint + "\x00" + string(resolved.overrideOrigin()) + "\x00" +
		protocolVersion + "\x00" +
		hex.EncodeToString(authBinding[:])
	return key, nil
}

// cachedPooledStdioTools answers discovery from the tool cache without touching the child, using
// the protocol the pool negotiated with the same launch identity.
func (e *CallExecutor) cachedPooledStdioTools(
	ctx context.Context,
	resolved *ResolvedServer,
	authorizationHeader string,
) (ToolDiscovery, bool) {
	if e.stdioPool == nil || resolved.Server.EffectiveTransport() != compozyconfig.MCPServerTransportStdio {
		return ToolDiscovery{}, false
	}
	spec, err := e.resolveMCPStdioSpec(ctx, resolved.Server, resolved.Target)
	if err != nil {
		return ToolDiscovery{}, false
	}
	protocolVersion, ok := e.stdioPool.knownProtocol(spec)
	if !ok {
		return ToolDiscovery{}, false
	}
	key, err := e.toolCacheKey(ctx, resolved, protocolVersion, authorizationHeader)
	if err != nil {
		return ToolDiscovery{}, false
	}
	return e.cachedTools(key, time.Now())
}

func negotiatedProtocolVersion(session *mcpsdk.ClientSession) string {
	if session == nil || session.InitializeResult() == nil {
		return ""
	}
	return session.InitializeResult().ProtocolVersion
}
