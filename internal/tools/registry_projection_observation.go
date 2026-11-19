package tools

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
)

// ProjectionObservation records the bounded changes a stream must check in addition to its epoch.
type ProjectionObservation struct {
	Policy any
	Shared [sha256.Size]byte
	Scoped string
}

type projectionPolicyObserver interface {
	// ProjectionPolicyIdentity returns a comparable token without resolving a policy.
	ProjectionPolicyIdentity(context.Context, Scope) (any, bool)
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
	shared, err := listings.externalObservations.load(ctx, key, func() ([sha256.Size]byte, bool, error) {
		global, err := listings.nativeObservations.load(ctx, r, func() (string, bool, error) {
			return r.nativeProjectionObservations(ctx, Scope{}, true), true, ctx.Err()
		})
		if err != nil {
			return [sha256.Size]byte{}, false, err
		}
		external, err := r.ExternalProjectionFingerprint(ctx, scope)
		var stamp strings.Builder
		appendProjectionGenerationPart(&stamp, global, external)
		return sha256.Sum256([]byte(stamp.String())), err == nil, err
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
	for _, provider := range r.providers {
		native, ok := provider.(*NativeProvider)
		if !ok {
			continue
		}
		checks := native.scopedAvailability
		if global {
			checks = native.sharedAvailability
		}
		for _, check := range checks {
			appendProjectionAvailability(&stamp, check(ctx, scope))
		}
	}
	return stamp.String()
}

func appendProjectionAvailability(stamp *strings.Builder, availability Availability) {
	fmt.Fprintf(stamp, "%t/%t/%t/%t/%t/%t/", availability.Registered, availability.Enabled,
		availability.Available, availability.Authorized, availability.Executable, availability.Conflicted)
	for _, reason := range availability.ReasonCodes {
		appendProjectionGenerationPart(stamp, "", string(reason))
	}
	stamp.WriteByte(';')
}
