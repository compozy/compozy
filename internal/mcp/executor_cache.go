package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpToolListCache struct {
	mu      sync.Mutex
	entries map[string]mcpToolListCacheEntry
}

type mcpToolListCacheEntry struct {
	descriptors []toolspkg.MCPToolDescriptor
	expiresAt   time.Time
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
