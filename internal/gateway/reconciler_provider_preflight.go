package gateway

import "context"

func (r *Reconciler) preflightProvider(ctx context.Context, provider ProviderActivation) error {
	preflight, ok := r.effects.(ProviderPreflight)
	if !ok {
		return nil
	}
	return preflight.PreflightProvider(ctx, provider)
}
