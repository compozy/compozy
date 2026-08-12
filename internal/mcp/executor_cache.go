package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpToolListCache struct {
	mu               sync.Mutex
	entries          map[string]mcpToolListCacheEntry
	projectionStates map[string]mcpToolProjectionState
}

type mcpToolListCacheEntry struct {
	descriptors []toolspkg.MCPToolDescriptor
	expiresAt   time.Time
}

type mcpToolProjectionState struct {
	generation  uint64
	fingerprint [sha256.Size]byte
	expiresAt   time.Time
}

func (e *CallExecutor) cachedTools(key string, now time.Time) ([]toolspkg.MCPToolDescriptor, bool) {
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	e.pruneToolProjectionStatesLocked(now)
	entry, ok := e.toolCache.entries[key]
	if !ok || !now.Before(entry.expiresAt) {
		delete(e.toolCache.entries, key)
		return nil, false
	}
	return cloneMCPToolDescriptors(entry.descriptors), true
}

func (e *CallExecutor) cacheTools(key string, descriptors []toolspkg.MCPToolDescriptor, ttlMs int, now time.Time) {
	if ttlMs <= 0 {
		return
	}
	e.toolCache.mu.Lock()
	if e.toolCache.entries == nil {
		e.toolCache.entries = make(map[string]mcpToolListCacheEntry)
	}
	e.pruneToolProjectionStatesLocked(now)
	for cachedKey, entry := range e.toolCache.entries {
		if !now.Before(entry.expiresAt) {
			delete(e.toolCache.entries, cachedKey)
		}
	}
	e.toolCache.entries[key] = mcpToolListCacheEntry{
		descriptors: cloneMCPToolDescriptors(descriptors),
		expiresAt:   now.Add(time.Duration(ttlMs) * time.Millisecond),
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
		fmt.Fprintf(&generation, "%d:%s%d;", len(key), key, state.generation)
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
	encoded, err := json.Marshal(descriptors)
	if err != nil {
		return fmt.Errorf("mcp: encode tool projection generation: %w", err)
	}
	fingerprint := sha256.Sum256(encoded)
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

func (e *CallExecutor) projectionSourceKey(
	ctx context.Context,
	source toolspkg.SourceRef,
) (string, bool) {
	if e.servers == nil {
		return mcpProjectionSourceKey(source, ""), true
	}
	resolved, err := e.resolveServer(ctx, source)
	if err != nil {
		return "", false
	}
	return mcpProjectionSourceKey(source, e.authorizationHeader(ctx, resolved)), true
}

func (e *CallExecutor) pruneToolProjectionStatesLocked(now time.Time) {
	for key, state := range e.toolCache.projectionStates {
		if !now.Before(state.expiresAt) {
			delete(e.toolCache.projectionStates, key)
		}
	}
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
	resolved ResolvedServer,
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
		fingerprint + "\x00" +
		protocolVersion + "\x00" +
		hex.EncodeToString(authBinding[:])
	return key, nil
}

func negotiatedProtocolVersion(session *mcpsdk.ClientSession) string {
	if session == nil || session.InitializeResult() == nil {
		return ""
	}
	return session.InitializeResult().ProtocolVersion
}
