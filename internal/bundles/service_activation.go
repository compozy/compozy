package bundles

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

// PreviewActivation resolves an activation without persisting resources.
func (s *Service) PreviewActivation(ctx context.Context, req ActivateRequest) (ActivationPreview, error) {
	if err := s.checkReady(ctx); err != nil {
		return ActivationPreview{}, err
	}

	resolved, err := s.resolveRequest(ctx, req, workspaceResolutionReadOnly)
	if err != nil {
		return ActivationPreview{}, err
	}
	digest, err := s.networkRequirementDigestForExtension(ctx, resolved.activation.ExtensionName)
	if err != nil {
		return ActivationPreview{}, err
	}
	resolved.activation = previewNetworkRequirement(resolved.activation, digest)
	return activationPreviewFromResolved(&resolved), nil
}

// Activate creates one bundle activation.
func (s *Service) Activate(ctx context.Context, req ActivateRequest) (ActivationPreview, error) {
	if err := s.checkReady(ctx); err != nil {
		return ActivationPreview{}, err
	}

	s.opMu.Lock()
	defer s.opMu.Unlock()

	digest, err := s.networkRequirementDigestForExtension(ctx, req.ExtensionName)
	if err != nil {
		return ActivationPreview{}, err
	}
	if digest != "" && !req.ConfirmNetworkRequirement {
		return ActivationPreview{}, networkRequirementConfirmationError(digest)
	}

	resolved, err := s.resolveRequest(ctx, req, workspaceResolutionRegisterPaths)
	if err != nil {
		return ActivationPreview{}, err
	}

	if _, err := s.store.GetBundleActivation(ctx, resolved.activation.ID); err == nil {
		return ActivationPreview{}, fmt.Errorf("%w: bundle activation already exists", resources.ErrConflict)
	} else if !errors.Is(err, ErrActivationNotFound) {
		return ActivationPreview{}, err
	}
	if err := s.applyNetworkRequirementConfirmationWithDigest(
		req,
		nil,
		&resolved.activation,
		digest,
	); err != nil {
		return ActivationPreview{}, err
	}

	if resolved.activation.CreatedAt.IsZero() {
		resolved.activation.CreatedAt = s.now().UTC()
	}
	resolved.activation.UpdatedAt = s.now().UTC()

	if err := s.store.CreateBundleActivation(ctx, resolved.activation); err != nil {
		return ActivationPreview{}, err
	}

	if reconcileErr := s.reconcileLocked(ctx); reconcileErr != nil {
		return ActivationPreview{}, s.rollbackActivationAndReconcileLocked(
			ctx,
			reconcileErr,
			func(rollbackCtx context.Context) error {
				return s.store.DeleteBundleActivation(rollbackCtx, resolved.activation.ID)
			},
			"delete newly-created bundle activation",
			resolved.activation.ID,
		)
	}

	return s.GetActivation(ctx, resolved.activation.ID)
}

// Deactivate removes one activation and its projected resources.
func (s *Service) Deactivate(ctx context.Context, id string) error {
	if err := s.checkReady(ctx); err != nil {
		return err
	}

	s.opMu.Lock()
	defer s.opMu.Unlock()

	current, err := s.store.GetBundleActivation(ctx, strings.TrimSpace(id))
	if err != nil {
		return err
	}
	if err := s.store.DeleteBundleActivation(ctx, current.ID); err != nil {
		return err
	}
	if reconcileErr := s.reconcileLocked(ctx); reconcileErr != nil {
		return s.rollbackActivationAndReconcileLocked(
			ctx,
			reconcileErr,
			func(rollbackCtx context.Context) error {
				return s.store.CreateBundleActivation(rollbackCtx, current)
			},
			"restore bundle activation after deactivate",
			current.ID,
		)
	}
	return nil
}

func activationPreviewFromResolved(resolved *resolvedActivation) ActivationPreview {
	return ActivationPreview{
		Activation: cloneActivation(resolved.activation),
		Bundle:     cloneBundleSpec(resolved.bundle),
		Profile:    cloneBundleProfile(resolved.profile),
		Inventory:  cloneInventoryItems(resolved.inventory),
	}
}
