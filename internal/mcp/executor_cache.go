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
	fingerprint string
	expiresAt   time.Time
	cacheable   bool
}

func (e *CallExecutor) cachedTools(key string, now time.Time) ([]toolspkg.MCPToolDescriptor, bool) {
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
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
		keys = append(keys, mcpProjectionSourceKey(sources[i]))
	}
	slices.Sort(keys)

	now := time.Now()
	e.toolCache.mu.Lock()
	defer e.toolCache.mu.Unlock()
	var generation strings.Builder
	for _, key := range keys {
		state, ok := e.toolCache.projectionStates[key]
		if !ok || !state.cacheable || !now.Before(state.expiresAt) {
			return "", false
		}
		fmt.Fprintf(&generation, "%d:%s%d;", len(key), key, state.generation)
	}
	return generation.String(), true
}

func (e *CallExecutor) recordToolProjection(
	source toolspkg.SourceRef,
	descriptors []toolspkg.MCPToolDescriptor,
	ttlMs int,
	now time.Time,
) error {
	encoded, err := json.Marshal(descriptors)
	if err != nil {
		return fmt.Errorf("mcp: encode tool projection generation: %w", err)
	}
	key := mcpProjectionSourceKey(source)
	e.toolCache.mu.Lock()
	if e.toolCache.projectionStates == nil {
		e.toolCache.projectionStates = make(map[string]mcpToolProjectionState)
	}
	state := e.toolCache.projectionStates[key]
	fingerprint := string(encoded)
	if state.fingerprint != fingerprint || state.cacheable != (ttlMs > 0) {
		state.generation++
	}
	if state.generation == 0 {
		state.generation = 1
	}
	state.fingerprint = fingerprint
	state.cacheable = ttlMs > 0
	if state.cacheable {
		state.expiresAt = now.Add(time.Duration(ttlMs) * time.Millisecond)
	} else {
		state.expiresAt = time.Time{}
	}
	e.toolCache.projectionStates[key] = state
	e.toolCache.mu.Unlock()
	return nil
}

func mcpProjectionSourceKey(source toolspkg.SourceRef) string {
	parts := []string{
		string(source.Kind),
		source.Owner,
		source.RawServerName,
		source.RawToolName,
		source.Scope,
		source.WorkspaceID,
		source.ResourceID,
		source.ResourceVersion,
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
	authBinding := sha256.Sum256([]byte(e.authorizationHeader(ctx, resolved)))
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
