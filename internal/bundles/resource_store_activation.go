package bundles

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/compozy/agh/internal/resources"
)

func (s *ResourceStore) CreateBundleActivation(ctx context.Context, activation Activation) error {
	activation, err := activation.Validated()
	if err != nil {
		return err
	}
	if activation.CreatedAt.IsZero() {
		activation.CreatedAt = s.now().UTC()
	}
	if activation.UpdatedAt.IsZero() {
		activation.UpdatedAt = activation.CreatedAt
	}
	_, err = s.activations.Put(ctx, s.actor, resources.Draft[ActivationResourceSpec]{
		ID:              strings.TrimSpace(activation.ID),
		Scope:           resourceScopeForActivation(activation),
		ExpectedVersion: 0,
		Spec:            activationResourceSpecFromActivation(activation),
	})
	if err != nil {
		return fmt.Errorf("bundles: create activation resource %q: %w", activation.ID, err)
	}
	return nil
}

func (s *ResourceStore) UpdateBundleActivation(ctx context.Context, activation Activation) error {
	activation, err := activation.Validated()
	if err != nil {
		return err
	}
	if activation.Version <= 0 {
		return fmt.Errorf("bundles: update activation resource %q: expected version must be positive", activation.ID)
	}
	if _, err := s.activations.Get(ctx, s.actor, strings.TrimSpace(activation.ID)); err != nil {
		return mapActivationResourceError("get", activation.ID, err)
	}
	if activation.UpdatedAt.IsZero() {
		activation.UpdatedAt = s.now().UTC()
	}
	_, err = s.activations.Put(ctx, s.actor, resources.Draft[ActivationResourceSpec]{
		ID:              strings.TrimSpace(activation.ID),
		Scope:           resourceScopeForActivation(activation),
		ExpectedVersion: activation.Version,
		Spec:            activationResourceSpecFromActivation(activation),
	})
	if err != nil {
		return fmt.Errorf("bundles: update activation resource %q: %w", activation.ID, err)
	}
	return nil
}

func (s *ResourceStore) DeleteBundleActivation(ctx context.Context, id string) error {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return errors.New("bundles: activation id is required")
	}
	current, err := s.activations.Get(ctx, s.actor, trimmed)
	if err != nil {
		return mapActivationResourceError("get", trimmed, err)
	}
	if err := s.activations.Delete(ctx, s.actor, trimmed, current.Version); err != nil {
		return mapActivationResourceError("delete", trimmed, err)
	}
	return nil
}

func (s *ResourceStore) GetBundleActivation(ctx context.Context, id string) (Activation, error) {
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return Activation{}, errors.New("bundles: activation id is required")
	}
	record, err := s.activations.Get(ctx, s.actor, trimmed)
	if err != nil {
		return Activation{}, mapActivationResourceError("get", trimmed, err)
	}
	return activationFromResourceRecord(record), nil
}

func (s *ResourceStore) ListBundleActivations(ctx context.Context) ([]Activation, error) {
	records, err := s.activations.List(ctx, s.actor, resources.ResourceFilter{
		Kind: BundleActivationResourceKind,
	})
	if err != nil {
		return nil, fmt.Errorf("bundles: list activation resources: %w", err)
	}
	activations := make([]Activation, 0, len(records))
	for _, record := range records {
		activations = append(activations, activationFromResourceRecord(record))
	}
	slices.SortFunc(activations, compareActivations)
	return activations, nil
}
