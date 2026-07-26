package daemon

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
)

type hookAwareParticipationResolver struct {
	inner participation.Resolver
	hooks atomic.Pointer[hookspkg.Hooks]
}

var _ participation.ResolvedObserver = (*hookAwareParticipationResolver)(nil)

func wrapParticipationResolverWithHooks(
	resolver participation.Resolver,
	hooks *hookspkg.Hooks,
) participation.Resolver {
	if resolver == nil {
		return nil
	}
	if existing, ok := resolver.(*hookAwareParticipationResolver); ok {
		existing.attachHooks(hooks)
		return existing
	}
	wrapped := &hookAwareParticipationResolver{inner: resolver}
	wrapped.attachHooks(hooks)
	return wrapped
}

func attachParticipationResolverHooks(resolver participation.Resolver, hooks *hookspkg.Hooks) {
	if wrapped, ok := resolver.(*hookAwareParticipationResolver); ok {
		wrapped.attachHooks(hooks)
	}
}

func (r *hookAwareParticipationResolver) attachHooks(hooks *hookspkg.Hooks) {
	if r == nil || hooks == nil {
		return
	}
	r.hooks.Store(hooks)
}

func (r *hookAwareParticipationResolver) Resolve(
	ctx context.Context,
	in participation.ResolveInput,
) (participation.Spec, error) {
	hooks := r.hooks.Load()
	if hooks == nil {
		return r.inner.Resolve(ctx, in)
	}
	intentResolver, selectsIntent := r.inner.(participation.IntentResolver)
	intent := participation.Intent{
		Request: participation.CloneRequest(in.Request),
		Source:  in.RequestSource,
	}
	if selectsIntent {
		selected, err := intentResolver.SelectIntent(ctx, in)
		if err != nil {
			return participation.Spec{}, err
		}
		intent = selected
	}
	prePayload := hookspkg.NetworkParticipationPreResolvePayload{
		WorkspaceID: strings.TrimSpace(in.WorkspaceID),
		Owner:       in.Owner,
		Request:     participation.CloneRequest(intent.Request),
		Source:      intent.Source,
		OwnerKey:    participationOwnerKey(in.Owner),
	}
	patched, err := hooks.DispatchNetworkParticipationPreResolve(ctx, prePayload)
	if err != nil {
		return participation.Spec{}, err
	}
	intent.Request = participation.CloneRequest(patched.Request)
	if selectsIntent {
		return intentResolver.ResolveIntent(ctx, in, intent)
	}
	if intent.Request != nil {
		in.Request = intent.Request
	}
	spec, err := r.inner.Resolve(ctx, in)
	if err != nil {
		return participation.Spec{}, err
	}
	return spec, nil
}

func (r *hookAwareParticipationResolver) ObserveParticipationResolved(
	ctx context.Context,
	observation participation.ResolvedObservation,
) error {
	if r == nil {
		return nil
	}
	hooks := r.hooks.Load()
	if hooks == nil {
		return nil
	}
	_, resolvedErr := hooks.DispatchNetworkParticipationResolved(ctx, hookspkg.NetworkParticipationResolvedPayload{
		WorkspaceID: strings.TrimSpace(observation.WorkspaceID),
		Owner:       observation.Owner,
		OwnerKey:    participationOwnerKey(observation.Owner),
		Spec:        observation.Spec,
	})
	if resolvedErr != nil {
		return fmt.Errorf("dispatch network.participation.resolved: %w", resolvedErr)
	}
	return nil
}

func participationOwnerKey(owner participation.OwnerRef) string {
	kind := strings.TrimSpace(string(owner.Kind))
	id := strings.TrimSpace(owner.ID)
	if kind == "" || id == "" {
		return ""
	}
	return kind + ":" + id
}
