package bundles

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

func (s *Service) UpdateActivation(ctx context.Context, req UpdateActivationRequest) (ActivationPreview, error) {
	if err := s.checkReady(ctx); err != nil {
		return ActivationPreview{}, err
	}
	if req.ExpectedVersion <= 0 {
		return ActivationPreview{}, fmt.Errorf(
			"%w: expected version must be positive",
			resources.ErrValidation,
		)
	}

	s.opMu.Lock()
	defer s.opMu.Unlock()

	current, err := s.store.GetBundleActivation(ctx, strings.TrimSpace(req.ID))
	if err != nil {
		return ActivationPreview{}, err
	}
	if req.ExpectedVersion != current.Version {
		return ActivationPreview{}, fmt.Errorf(
			"%w: expected version %d",
			resources.ErrConflict,
			req.ExpectedVersion,
		)
	}
	next := cloneActivation(current)
	definition, err := s.resolveActivationDefinition(ctx, current)
	if err != nil {
		return ActivationPreview{}, err
	}
	next.SpecContentHash = definition.specContentHash
	if err := s.applyNetworkRequirementConfirmation(
		ctx,
		ActivateRequest{ConfirmNetworkRequirement: req.ConfirmNetworkRequirement},
		&current,
		&next,
	); err != nil {
		return ActivationPreview{}, err
	}
	next.UpdatedAt = s.now().UTC()

	if err := s.store.UpdateBundleActivation(ctx, next); err != nil {
		return ActivationPreview{}, err
	}
	if reconcileErr := s.reconcileLocked(ctx); reconcileErr != nil {
		return ActivationPreview{}, s.rollbackActivationAndReconcileLocked(
			ctx,
			reconcileErr,
			func(rollbackCtx context.Context) error {
				return s.restoreBundleActivation(rollbackCtx, current)
			},
			"restore bundle activation after update",
			current.ID,
		)
	}
	return s.GetActivation(ctx, next.ID)
}

func (s *Service) restoreBundleActivation(ctx context.Context, desired Activation) error {
	current, err := s.store.GetBundleActivation(ctx, desired.ID)
	if err != nil {
		return err
	}
	desired.Version = current.Version
	return s.store.UpdateBundleActivation(ctx, desired)
}
