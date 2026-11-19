package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ExternalProjectionScopeKey retains every external provider's declared listing/availability scope.
func (r *RuntimeRegistry) ExternalProjectionScopeKey(scope Scope) string {
	var key strings.Builder
	appendProjectionGenerationPart(&key, scope.ProfileID, scope.WorkspaceID)
	for _, provider := range r.providers {
		if _, native := provider.(*NativeProvider); native {
			continue
		}
		providerScope := scope
		if scoped, ok := provider.(ProjectionScopeProvider); ok {
			providerScope = scoped.ProjectionScope(scope)
		}
		for _, part := range []string{
			providerScope.ProfileID, providerScope.WorkspaceID, providerScope.AgentName,
			providerScope.SessionID, providerScope.RunID, providerScope.ActorKind,
		} {
			appendProjectionGenerationPart(&key, "", part)
		}
		fmt.Fprintf(&key, "%d/%t/%t/%t;", providerScope.Generation, providerScope.Operator,
			providerScope.AllProfiles, providerScope.ProfileSwitchAuthorized)
	}
	return key.String()
}

// ExternalProjectionFingerprint validates external catalogs and health once per declared provider scope in a read.
func (r *RuntimeRegistry) ExternalProjectionFingerprint(ctx context.Context, scope Scope) (string, error) {
	ctx = r.ReadContext(EnsureProjectionListings(ctx))
	cache, ok := ctx.Value(policyResolutionKey{}).(*policyResolutionCache)
	if !ok {
		return "", errors.New("tools: projection read context is unavailable")
	}
	cache.externalMu.Lock()
	defer cache.externalMu.Unlock()
	if cache.externalStamps == nil {
		cache.externalStamps = make(map[projectionListingKey]string)
	}
	var stamp strings.Builder
	for providerIndex, provider := range r.providers {
		if _, native := provider.(*NativeProvider); native {
			continue
		}
		providerScope := scope
		if scoped, ok := provider.(ProjectionScopeProvider); ok {
			providerScope = scoped.ProjectionScope(scope)
		}
		key := projectionListingKey{r, providerIndex, providerScope}
		fingerprint, found := cache.externalStamps[key]
		if !found {
			var err error
			fingerprint, err = r.externalProviderFingerprint(ctx, provider, providerIndex, providerScope)
			if err != nil {
				return "", err
			}
			cache.externalStamps[key] = fingerprint
		}
		stamp.WriteString(fingerprint)
	}
	return stamp.String(), nil
}

func (r *RuntimeRegistry) externalProviderFingerprint(
	ctx context.Context, provider Provider, index int, scope Scope,
) (string, error) {
	if mcp, ok := provider.(*MCPProvider); ok && cachedMCPProjection(ctx) {
		generation, err := mcp.cachedProjectionObservation(ctx, scope)
		return projectionFingerprint([]byte(generation)), err
	}
	if source, ok := provider.(ProjectionGenerationProvider); ok {
		if generation, known := source.ProjectionGeneration(ctx, scope); known {
			return projectionFingerprint([]byte(generation)), nil
		}
	}
	descriptors, err := r.listProvider(ctx, provider, index, scope)
	if err != nil {
		return "", err
	}
	entries := make([]sessionProjectionEntry, 0, len(descriptors))
	for _, descriptor := range descriptors {
		entries = append(entries, sessionProjectionEntry{entry: &registryEntry{
			descriptor: descriptor, provider: provider, providerIndex: index,
		}})
	}
	shared := r.projectionAvailabilities(ctx, scope, entries)
	availability := make(map[ToolID]Availability, len(descriptors))
	for _, entry := range entries {
		id := entry.entry.descriptor.ID
		value, found := shared[id]
		if !found {
			value = r.availabilityFor(ctx, scope, entry.entry)
		}
		availability[id] = value
	}
	encoded, err := json.Marshal(struct {
		Descriptors  []Descriptor
		Availability map[ToolID]Availability
	}{descriptors, availability})
	if err != nil {
		return "", err
	}
	return projectionFingerprint(encoded), nil
}

func projectionFingerprint(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
