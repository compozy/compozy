package gateway

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
)

func (r *Reconciler) markProviderEstablishing(
	ctx context.Context,
	plan TierPlan,
	state *runtimeTierState,
	hadAdvertisement bool,
) error {
	if hadAdvertisement {
		return nil
	}
	if err := r.markObserved(ctx, plan, ProviderEstablishing, SurfaceOff, ""); err != nil {
		return errors.Join(err, r.unbindTier(ctx, plan.Tier, state))
	}
	return nil
}

func (r *Reconciler) markObserved(
	ctx context.Context,
	plan TierPlan,
	providerState ProviderObservedState,
	surfaceState SurfaceObservedState,
	cause string,
) error {
	now := time.Now().UTC()
	updated, err := r.store.SetObserved(ctx, plan, providerState, surfaceState, now, cause)
	if err != nil {
		return err
	}
	if !updated {
		return ErrStaleGeneration
	}
	if plan.Provider != nil {
		eventErr := r.recordProviderState(ctx, *plan.Provider, providerState, cause)
		if eventErr == nil {
			return nil
		}
		if providerState != ProviderEstablishing && providerState != ProviderUp {
			return eventErr
		}
		safeCause := diagnostics.RedactAndBound(eventErr.Error(), maxPersistedGatewayDiagnosticBytes)
		rolledBack, rollbackErr := r.store.SetObserved(
			ctx,
			plan,
			ProviderDegraded,
			SurfaceOff,
			time.Now().UTC(),
			safeCause,
		)
		if rollbackErr != nil {
			return errors.Join(eventErr, fmt.Errorf("gateway: compensate provider event failure: %w", rollbackErr))
		}
		if !rolledBack {
			return errors.Join(eventErr, ErrStaleGeneration)
		}
		return eventErr
	}
	return nil
}

func (r *Reconciler) recordProviderState(
	ctx context.Context,
	activation ProviderActivation,
	state ProviderObservedState,
	cause string,
) error {
	if r.providerStates == nil {
		return nil
	}
	if err := r.providerStates.RecordProviderStateChanged(ctx, activation, state, cause); err != nil {
		return fmt.Errorf("gateway: record provider state change: %w", err)
	}
	return nil
}

func (r *Reconciler) currentProviderActivation(
	ctx context.Context,
	fallback ProviderActivation,
) (ProviderActivation, error) {
	snapshot, err := r.store.Snapshot(ctx)
	if err != nil {
		return ProviderActivation{}, fmt.Errorf("gateway: load disabled provider state: %w", err)
	}
	for _, activation := range snapshot.Providers {
		if activation.ProviderName == fallback.ProviderName && activation.Tier == fallback.Tier {
			return activation, nil
		}
	}
	return fallback, nil
}

func (r *Reconciler) markDegraded(ctx context.Context, plan TierPlan, cause error) error {
	safeCause := diagnostics.RedactAndBound(cause.Error(), maxPersistedGatewayDiagnosticBytes)
	markErr := r.markObserved(ctx, plan, ProviderDegraded, SurfaceOff, safeCause)
	if markErr != nil {
		return errors.Join(cause, markErr)
	}
	return errors.Join(ErrProviderDegraded, cause)
}
