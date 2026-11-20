package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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
		inputs, err := cachedMCPProviderInputs(ctx, mcp, scope)
		return projectionFingerprint([]byte(inputs.stamp)), err
	}
	inputs, err := r.cachedExternalProviderInputs(ctx, provider, projectionListingKey{r, index, scope})
	return inputs.stamp, err
}

type pinnedProjectionInputsKey struct{}

type pinnedProjectionInputs struct {
	mcp   map[pinnedMCPProviderInputs]mcpProjectionInputs
	other map[projectionListingKey]externalProviderInputs
}

type externalProviderInputs struct {
	descriptors  []Descriptor
	availability map[ToolID]Availability
	stamp        string
	complete     bool
}

func pinnedProviderInputs(ctx context.Context, key projectionListingKey) (externalProviderInputs, bool) {
	pinned, ok := ctx.Value(pinnedProjectionInputsKey{}).(pinnedProjectionInputs)
	if !ok {
		return externalProviderInputs{}, false
	}
	inputs, found := pinned.other[key]
	return inputs, found
}

func (r *RuntimeRegistry) pinnedAvailability(
	ctx context.Context,
	scope Scope,
	entry *registryEntry,
) (Availability, bool) {
	if ctx.Value(pinnedProjectionInputsKey{}) == nil {
		return Availability{}, false
	}
	switch entry.provider.(type) {
	case *NativeProvider, *MCPProvider:
		return Availability{}, false
	}
	if provider, ok := entry.provider.(ProjectionScopeProvider); ok {
		scope = provider.ProjectionScope(scope)
	}
	inputs, found := pinnedProviderInputs(ctx, projectionListingKey{r, entry.providerIndex, scope})
	if !found {
		return Availability{}, false
	}
	value, found := inputs.availability[entry.descriptor.ID]
	return value, found
}

func (r *RuntimeRegistry) cachedExternalProviderInputs(
	ctx context.Context, provider Provider, key projectionListingKey,
) (externalProviderInputs, error) {
	if pinned, found := pinnedProviderInputs(ctx, key); found {
		return pinned, nil
	}
	if listings, found := ctx.Value(projectionListingsKey{}).(*ProjectionListings); found {
		return listings.otherInputs.load(ctx, key, func() (externalProviderInputs, bool, error) {
			inputs, err := r.readExternalProviderInputs(ctx, provider, key)
			return inputs, inputs.complete && err == nil, err
		})
	}
	return r.readExternalProviderInputs(ctx, provider, key)
}

func (r *RuntimeRegistry) readExternalProviderInputs(
	ctx context.Context, provider Provider, key projectionListingKey,
) (externalProviderInputs, error) {
	descriptors, complete, err := r.listValidatedProvider(ctx, provider, key, key.scope)
	if err != nil {
		return externalProviderInputs{}, err
	}
	availability := make(map[ToolID]Availability, len(descriptors))
	if batch, ok := provider.(ProjectionAvailabilityProvider); ok && len(descriptors) != 0 {
		ids := make([]ToolID, 0, len(descriptors))
		for i := range descriptors {
			ids = append(ids, descriptors[i].ID)
		}
		values := batch.ProjectionAvailability(ctx, key.scope, ids)
		for _, id := range ids {
			value, found := values[id]
			if !found {
				value = Unavailable(ReasonBackendNotExecutable)
			}
			value = validateProjectionAvailability(value)
			value.ReasonCodes = slices.Clone(value.ReasonCodes)
			availability[id] = value
		}
	} else {
		for i := range descriptors {
			d := &descriptors[i]
			value := r.uncachedAvailabilityFor(ctx, key.scope, &registryEntry{
				descriptor: *d, provider: provider, providerIndex: key.providerIndex,
			})
			value.ReasonCodes = slices.Clone(value.ReasonCodes)
			availability[d.ID] = value
		}
	}
	encoded, err := json.Marshal(struct {
		Descriptors  []Descriptor
		Availability map[ToolID]Availability
	}{descriptors, availability})
	if err != nil {
		return externalProviderInputs{}, err
	}
	return externalProviderInputs{descriptors, availability, projectionFingerprint(encoded), complete}, ctx.Err()
}

func projectionFingerprint(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func cachedMCPProviderInputs(ctx context.Context, provider *MCPProvider, scope Scope) (mcpProjectionInputs, error) {
	if pinned, found := provider.pinnedProjectionInputs(ctx, scope); found {
		return pinned, nil
	}
	listings, found := ctx.Value(projectionListingsKey{}).(*ProjectionListings)
	if !found {
		return provider.readCachedProjectionInputs(ctx, scope)
	}
	key := pinnedMCPProviderInputs{provider, provider.ProjectionScope(scope)}
	return listings.mcpInputs.load(ctx, key, func() (mcpProjectionInputs, bool, error) {
		inputs, err := provider.readCachedProjectionInputs(ctx, key.scope)
		return inputs, err == nil, err
	})
}

// PinProjectionInputs makes a validated build use exactly the cached MCP catalog and health
// read by its observation. Other providers can be served without admitting a persistent entry.
func (r *RuntimeRegistry) PinProjectionInputs(
	build, observed context.Context, scope Scope,
) (context.Context, bool, error) {
	var pinned pinnedProjectionInputs
	complete := true
	for index, provider := range r.providers {
		switch source := provider.(type) {
		case *NativeProvider:
		case *MCPProvider:
			if !cachedMCPProjection(observed) {
				complete = false
				continue
			}
			inputs, err := cachedMCPProviderInputs(observed, source, scope)
			if err != nil {
				return build, false, err
			}
			if pinned.mcp == nil {
				pinned.mcp = make(map[pinnedMCPProviderInputs]mcpProjectionInputs)
			}
			pinned.mcp[pinnedMCPProviderInputs{source, source.ProjectionScope(scope)}] = inputs
		default:
			scoped, ok := provider.(ProjectionScopeProvider)
			if !ok {
				complete = false
				continue
			}
			key := projectionListingKey{r, index, scoped.ProjectionScope(scope)}
			inputs, err := r.cachedExternalProviderInputs(observed, provider, key)
			if err != nil {
				return build, false, err
			}
			if !inputs.complete {
				complete = false
				continue
			}
			if pinned.other == nil {
				pinned.other = make(map[projectionListingKey]externalProviderInputs)
			}
			pinned.other[key] = inputs
		}
	}
	return context.WithValue(build, pinnedProjectionInputsKey{}, pinned), complete, nil
}
