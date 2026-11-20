package tools

import (
	"context"
	"fmt"
	"iter"
	"slices"
	"strings"
	"sync"
)

// ProjectionObservation records the bounded changes a stream must check in addition to its epoch.
type ProjectionObservation struct {
	Policy any
	Shared projectionSharedObservation
	Scoped string
}

type projectionSharedObservation struct {
	external string
	native   string
}

type projectionPolicyObserver interface {
	// ProjectionPolicyIdentity returns a comparable token without resolving a policy.
	ProjectionPolicyIdentity(context.Context, Scope) (any, bool)
}

type nativeProjectionBuildKey struct{}

type nativeProjectionBuild struct {
	mu       sync.Mutex
	inputs   map[nativeProjectionInput]Availability
	mismatch bool
}

type nativeProjectionInput struct {
	group *NativeAvailabilityGroup
	scope Scope
}

func (b *nativeProjectionBuild) record(group *NativeAvailabilityGroup, scope Scope, value Availability) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if group.Shared {
		scope = Scope{}
	}
	key := nativeProjectionInput{group, scope}
	if previous, exists := b.inputs[key]; exists {
		b.mismatch = b.mismatch || !sameProjectionAvailability(previous, value)
		return
	}
	if b.inputs == nil {
		b.inputs = make(map[nativeProjectionInput]Availability)
	}
	value.ReasonCodes = slices.Clone(value.ReasonCodes)
	b.inputs[key] = value
}

func (b *nativeProjectionBuild) stable(r *RuntimeRegistry, scope Scope, observed ProjectionObservation) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.mismatch || len(b.inputs) == 0 {
		return !b.mismatch
	}
	// The grouped observations use the registry's fixed provider and group order.
	expected := make(map[nativeProjectionInput]string, len(b.inputs))
	r.readNativeProjectionStamps(expected, Scope{}, observed.Shared.native, true)
	r.readNativeProjectionStamps(expected, scope, observed.Scoped, false)
	for input, consumed := range b.inputs {
		var stamp strings.Builder
		appendProjectionAvailability(&stamp, consumed)
		if expected[input] != strings.TrimSuffix(stamp.String(), ";") {
			return false
		}
	}
	return true
}

func (r *RuntimeRegistry) readNativeProjectionStamps(
	stamps map[nativeProjectionInput]string, scope Scope, remaining string, shared bool,
) {
	for group := range r.nativeAvailabilityGroups(shared) {
		stamp, rest, found := strings.Cut(remaining, ";")
		if !found {
			return
		}
		stamps[nativeProjectionInput{group, scope}] = stamp
		remaining = rest
	}
}

func sameProjectionAvailability(a, b Availability) bool {
	return a.Registered == b.Registered && a.Enabled == b.Enabled && a.Available == b.Available &&
		a.Authorized == b.Authorized && a.Executable == b.Executable && a.Conflicted == b.Conflicted &&
		slices.Equal(a.ReasonCodes, b.ReasonCodes)
}

// MatchesExternal reports whether the build pinned the catalog and availability observed for this scope.
func (o ProjectionObservation) MatchesExternal(fingerprint string) bool {
	return o.Shared.external == fingerprint
}

// BeginProjectionBuild captures inputs consumed when populating the cache.
func (r *RuntimeRegistry) BeginProjectionBuild(
	ctx context.Context,
	scope Scope,
	observed ProjectionObservation,
) (context.Context, func() bool) {
	native := new(nativeProjectionBuild)
	build := context.WithValue(ctx, nativeProjectionBuildKey{}, native)
	policyStable := func() bool { return true }
	if policy, ok := r.policyResolver.(interface {
		BeginProjectionBuild(context.Context, Scope, any) (context.Context, func() bool)
	}); ok {
		build, policyStable = policy.BeginProjectionBuild(build, scope, observed.Policy)
	}
	return build, func() bool {
		return policyStable() && native.stable(r, scope, observed)
	}
}

// HasProjectionPolicyObserver reports whether an unknown policy observation must decline reuse.
func (r *RuntimeRegistry) HasProjectionPolicyObserver() bool {
	_, ok := r.policyResolver.(projectionPolicyObserver)
	return ok
}

// ObserveProjection reads the mutable inputs separately from policy resolution or projection.
// The caller rotates listings at its refresh interval and on epoch changes.
func (r *RuntimeRegistry) ObserveProjection(ctx context.Context, scope Scope) (ProjectionObservation, bool) {
	listings, ok := ctx.Value(projectionListingsKey{}).(*ProjectionListings)
	if !ok || scope.Operator || !r.usePolicyInput {
		return ProjectionObservation{}, false
	}
	if !cachedMCPProjection(ctx) {
		for _, provider := range r.providers {
			if _, mcp := provider.(*MCPProvider); mcp {
				// Without a catalog observer, a full stream read can discover new descriptors.
				return ProjectionObservation{}, false
			}
		}
	}
	var policy any
	if r.policyResolver != nil {
		observer, ok := r.policyResolver.(projectionPolicyObserver)
		if !ok {
			return ProjectionObservation{}, false
		}
		policy, ok = observer.ProjectionPolicyIdentity(ctx, scope)
		if !ok {
			return ProjectionObservation{}, false
		}
	}
	key := projectionObservationKey{r, r.ExternalProjectionScopeKey(scope)}
	shared, err := listings.externalObservations.load(ctx, key, func() (projectionSharedObservation, bool, error) {
		global, err := listings.nativeObservations.load(ctx, r, func() (string, bool, error) {
			return r.nativeProjectionObservations(ctx, Scope{}, true), true, ctx.Err()
		})
		if err != nil {
			return projectionSharedObservation{}, false, err
		}
		external, err := r.ExternalProjectionFingerprint(ctx, scope)
		return projectionSharedObservation{external: external, native: global}, err == nil, err
	})
	if err != nil {
		return ProjectionObservation{}, false
	}
	local := r.nativeProjectionObservations(ctx, scope, false)
	return ProjectionObservation{Policy: policy, Shared: shared, Scoped: local}, ctx.Err() == nil
}

type projectionObservationKey struct {
	registry *RuntimeRegistry
	scope    string
}

func (r *RuntimeRegistry) nativeProjectionObservations(ctx context.Context, scope Scope, global bool) string {
	var stamp strings.Builder
	for group := range r.nativeAvailabilityGroups(global) {
		appendProjectionAvailability(&stamp, group.Check(ctx, scope))
	}
	return stamp.String()
}

func (r *RuntimeRegistry) nativeAvailabilityGroups(shared bool) iter.Seq[*NativeAvailabilityGroup] {
	return func(yield func(*NativeAvailabilityGroup) bool) {
		for _, provider := range r.providers {
			native, ok := provider.(*NativeProvider)
			if !ok {
				continue
			}
			groups := native.scopedAvailability
			if shared {
				groups = native.sharedAvailability
			}
			for _, group := range groups {
				if !yield(group) {
					return
				}
			}
		}
	}
}

func appendProjectionAvailability(stamp *strings.Builder, availability Availability) {
	fmt.Fprintf(stamp, "%t/%t/%t/%t/%t/%t/", availability.Registered, availability.Enabled,
		availability.Available, availability.Authorized, availability.Executable, availability.Conflicted)
	for _, reason := range availability.ReasonCodes {
		appendProjectionGenerationPart(stamp, "", string(reason))
	}
	stamp.WriteByte(';')
}
