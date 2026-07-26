package bridges

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/agh/internal/store"
)

func (s *Service) checkReady(ctx context.Context, action string) error {
	if s == nil {
		return errors.New("bridges: registry is required")
	}
	if s.store == nil {
		return errors.New("bridges: registry store is required")
	}
	if ctx == nil {
		return fmt.Errorf("bridges: %s context is required", action)
	}
	return ctx.Err()
}

func (s *Service) loadRoutableInstance(ctx context.Context, bridgeInstanceID string) (BridgeInstance, error) {
	instance, err := s.store.GetBridgeInstance(ctx, bridgeInstanceID)
	if err != nil {
		return BridgeInstance{}, err
	}
	if !instance.Enabled || instance.Status.Normalize() == BridgeStatusDisabled {
		return BridgeInstance{}, fmt.Errorf("%w: %s", ErrBridgeInstanceUnavailable, instance.ID)
	}
	return instance, nil
}

func (s *Service) prepareRouteForWrite(route BridgeRoute, existing *BridgeRoute) BridgeRoute {
	prepared := route.normalize()

	activityAt := prepared.LastActivityAt
	if activityAt.IsZero() {
		activityAt = s.now()
	}
	prepared.LastActivityAt = activityAt

	if existing != nil && prepared.CreatedAt.IsZero() {
		prepared.CreatedAt = existing.CreatedAt
	}
	if prepared.CreatedAt.IsZero() {
		prepared.CreatedAt = activityAt
	}
	if prepared.UpdatedAt.IsZero() {
		prepared.UpdatedAt = activityAt
	}

	return prepared
}

func (r CreateInstanceRequest) toInstance(now func() time.Time) (BridgeInstance, error) {
	clock := now
	if clock == nil {
		clock = func() time.Time {
			return time.Now().UTC()
		}
	}

	instance := BridgeInstance{
		ID:                   strings.TrimSpace(r.ID),
		Scope:                r.Scope.Normalize(),
		WorkspaceID:          strings.TrimSpace(r.WorkspaceID),
		Platform:             strings.TrimSpace(r.Platform),
		ExtensionName:        strings.TrimSpace(r.ExtensionName),
		DisplayName:          strings.TrimSpace(r.DisplayName),
		Source:               r.Source.Normalize(),
		Enabled:              r.Enabled,
		Status:               r.Status.Normalize(),
		DMPolicy:             r.DMPolicy.Normalize(),
		RoutingPolicy:        r.RoutingPolicy,
		ProviderConfig:       r.ProviderConfig,
		DeliveryDefaults:     r.DeliveryDefaults,
		NotificationSuppress: r.NotificationSuppress,
		Degradation:          r.Degradation,
		CreatedAt:            r.CreatedAt,
		UpdatedAt:            r.UpdatedAt,
	}
	if instance.ID == "" {
		instance.ID = store.NewID("brg")
	}
	if instance.Source == "" {
		instance.Source = BridgeInstanceSourceDynamic
	}
	if instance.CreatedAt.IsZero() {
		instance.CreatedAt = clock()
	}
	if instance.UpdatedAt.IsZero() {
		instance.UpdatedAt = instance.CreatedAt
	}

	providerConfig, err := normalizeProviderConfigJSON(instance.ProviderConfig)
	if err != nil {
		return BridgeInstance{}, err
	}
	instance.ProviderConfig = providerConfig

	deliveryDefaults, err := NormalizeDeliveryDefaultsJSON(instance.DeliveryDefaults)
	if err != nil {
		return BridgeInstance{}, err
	}
	instance.DeliveryDefaults = deliveryDefaults
	instance = instance.normalize()

	if err := instance.Validate(); err != nil {
		return BridgeInstance{}, err
	}

	return instance, nil
}

func cloneBridgeInstance(instance BridgeInstance) *BridgeInstance {
	cloned := instance
	if instance.ProviderConfig != nil {
		cloned.ProviderConfig = append(json.RawMessage(nil), instance.ProviderConfig...)
	}
	if instance.DeliveryDefaults != nil {
		cloned.DeliveryDefaults = append(json.RawMessage(nil), instance.DeliveryDefaults...)
	}
	if instance.Degradation != nil {
		degradation := *instance.Degradation
		cloned.Degradation = &degradation
	}
	return &cloned
}

func cloneBridgeRoute(route BridgeRoute) *BridgeRoute {
	cloned := route
	return &cloned
}
