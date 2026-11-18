package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
)

// ExternalProjectionScopeKey retains every external provider's declared listing/availability scope.
func (r *RuntimeRegistry) ExternalProjectionScopeKey(scope Scope) string {
	scopes := []Scope{{ProfileID: scope.ProfileID, WorkspaceID: scope.WorkspaceID}}
	for _, provider := range r.providers {
		if _, native := provider.(*NativeProvider); native {
			continue
		}
		providerScope := scope
		if scoped, ok := provider.(ProjectionScopeProvider); ok {
			providerScope = scoped.ProjectionScope(scope)
		}
		scopes = append(scopes, providerScope)
	}
	encoded, err := json.Marshal(scopes)
	if err != nil {
		return ""
	}
	return string(encoded)
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
			descriptors, err := r.listProvider(ctx, provider, providerIndex, providerScope)
			if err != nil {
				return "", err
			}
			entries := make([]sessionProjectionEntry, 0, len(descriptors))
			for _, descriptor := range descriptors {
				entries = append(entries, sessionProjectionEntry{entry: &registryEntry{
					descriptor: descriptor, provider: provider, providerIndex: providerIndex,
				}})
			}
			shared := r.projectionAvailabilities(ctx, providerScope, entries)
			availability := make(map[ToolID]Availability, len(descriptors))
			for _, entry := range entries {
				id := entry.entry.descriptor.ID
				value, found := shared[id]
				if !found {
					value = r.availabilityFor(ctx, providerScope, entry.entry)
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
			digest := sha256.Sum256(encoded)
			fingerprint = hex.EncodeToString(digest[:])
			cache.externalStamps[key] = fingerprint
		}
		stamp.WriteString(fingerprint)
	}
	return stamp.String(), nil
}
