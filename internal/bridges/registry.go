package bridges

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// CreateInstanceRequest captures the persisted configuration for a new bridge instance.
type CreateInstanceRequest struct {
	ID                   string               `json:"id,omitempty"`
	Scope                Scope                `json:"scope"`
	WorkspaceID          string               `json:"workspace_id,omitempty"`
	Platform             string               `json:"platform"`
	ExtensionName        string               `json:"extension_name"`
	DisplayName          string               `json:"display_name"`
	Source               BridgeInstanceSource `json:"source,omitempty"`
	Enabled              bool                 `json:"enabled"`
	Status               BridgeStatus         `json:"status"`
	DMPolicy             BridgeDMPolicy       `json:"dm_policy,omitempty"`
	RoutingPolicy        RoutingPolicy        `json:"routing_policy"`
	ProviderConfig       json.RawMessage      `json:"provider_config,omitempty"`
	DeliveryDefaults     json.RawMessage      `json:"delivery_defaults,omitempty"`
	NotificationSuppress bool                 `json:"notification_suppress"`
	Degradation          *BridgeDegradation   `json:"degradation,omitempty"`
	CreatedAt            time.Time            `json:"created_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
}

// Validate reports whether the creation request contains a valid instance definition.
func (r CreateInstanceRequest) Validate() error {
	_, err := r.toInstance(nil)
	return err
}

// UpdateInstanceRequest captures one mutation of bridge-instance fields that
// do not change the lifecycle state machine.
type UpdateInstanceRequest struct {
	ID                   string             `json:"id"`
	DisplayName          *string            `json:"display_name,omitempty"`
	DMPolicy             *BridgeDMPolicy    `json:"dm_policy,omitempty"`
	RoutingPolicy        *RoutingPolicy     `json:"routing_policy,omitempty"`
	ProviderConfig       *json.RawMessage   `json:"provider_config,omitempty"`
	DeliveryDefaults     *json.RawMessage   `json:"delivery_defaults,omitempty"`
	NotificationSuppress *bool              `json:"notification_suppress,omitempty"`
	Degradation          *BridgeDegradation `json:"degradation,omitempty"`
	ClearDegradation     bool               `json:"clear_degradation,omitempty"`
	UpdatedAt            time.Time          `json:"updated_at"`
}

// Validate reports whether the request contains at least one mutable field and
// each supplied value is internally consistent.
func (r UpdateInstanceRequest) Validate() error {
	if err := requireField(strings.TrimSpace(r.ID), "bridge instance id"); err != nil {
		return err
	}
	if !r.hasMutableField() {
		return errors.New("bridges: bridge instance update requires at least one mutable field")
	}
	return r.validateOptionalFields()
}

func (r UpdateInstanceRequest) hasMutableField() bool {
	return r.DisplayName != nil ||
		r.DMPolicy != nil ||
		r.RoutingPolicy != nil ||
		r.ProviderConfig != nil ||
		r.DeliveryDefaults != nil ||
		r.NotificationSuppress != nil ||
		r.Degradation != nil ||
		r.ClearDegradation
}

func (r UpdateInstanceRequest) validateOptionalFields() error {
	if r.DisplayName != nil {
		if err := requireField(strings.TrimSpace(*r.DisplayName), "bridge instance display name"); err != nil {
			return err
		}
	}
	if r.DMPolicy != nil {
		if err := r.DMPolicy.Validate(); err != nil {
			return err
		}
	}
	if r.RoutingPolicy != nil {
		if err := r.RoutingPolicy.Validate(); err != nil {
			return err
		}
	}
	if r.ProviderConfig != nil {
		if _, err := normalizeProviderConfigJSON(*r.ProviderConfig); err != nil {
			return err
		}
	}
	if r.DeliveryDefaults != nil {
		if _, err := NormalizeDeliveryDefaultsJSON(*r.DeliveryDefaults); err != nil {
			return err
		}
	}
	if r.Degradation != nil {
		if err := r.Degradation.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// UpdateInstanceStateRequest captures one daemon-owned lifecycle transition.
type UpdateInstanceStateRequest struct {
	ID               string             `json:"id"`
	Enabled          bool               `json:"enabled"`
	Status           BridgeStatus       `json:"status"`
	Degradation      *BridgeDegradation `json:"degradation,omitempty"`
	ClearDegradation bool               `json:"clear_degradation,omitempty"`
	UpdatedAt        time.Time          `json:"updated_at"`
}

// Validate reports whether the request contains the fields needed for a lifecycle update.
func (r UpdateInstanceStateRequest) Validate() error {
	if err := requireField(strings.TrimSpace(r.ID), "bridge instance id"); err != nil {
		return err
	}
	if r.ClearDegradation && r.Degradation != nil && !r.Degradation.IsZero() {
		return errors.New("bridges: bridge instance state update cannot clear and set degradation together")
	}
	if r.Degradation != nil {
		if err := r.Degradation.Validate(); err != nil {
			return err
		}
	}
	return validateInstanceLifecycle(r.Enabled, r.Status.Normalize())
}

// Service is the concrete daemon-owned registry implementation.
type Service struct {
	store RegistryStore
	now   func() time.Time
}

// RegistryOption customizes Service construction.
type RegistryOption func(*Service)

var _ Registry = (*Service)(nil)

// NewRegistry constructs the bridge registry over the supplied persistence surface.
func NewRegistry(store RegistryStore, opts ...RegistryOption) *Service {
	service := &Service{
		store: store,
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

// WithNow overrides the clock used for default timestamps in tests.
func WithNow(now func() time.Time) RegistryOption {
	return func(service *Service) {
		if now != nil {
			service.now = now
		}
	}
}

// CreateInstance persists a new bridge instance after applying lifecycle validation.
func (s *Service) CreateInstance(ctx context.Context, req CreateInstanceRequest) (*BridgeInstance, error) {
	if err := s.checkReady(ctx, "create bridge instance"); err != nil {
		return nil, err
	}

	instance, err := req.toInstance(s.now)
	if err != nil {
		return nil, fmt.Errorf("bridges: create bridge instance: build request: %w", err)
	}
	if err := s.store.InsertBridgeInstance(ctx, instance); err != nil {
		return nil, fmt.Errorf("bridges: create bridge instance %q: insert: %w", instance.ID, err)
	}
	return cloneBridgeInstance(instance), nil
}

// GetInstance returns one persisted bridge instance by primary key.
func (s *Service) GetInstance(ctx context.Context, id string) (*BridgeInstance, error) {
	if err := s.checkReady(ctx, "get bridge instance"); err != nil {
		return nil, err
	}

	trimmedID := strings.TrimSpace(id)
	instance, err := s.store.GetBridgeInstance(ctx, trimmedID)
	if err != nil {
		return nil, fmt.Errorf("bridges: get bridge instance %q: %w", trimmedID, err)
	}
	return cloneBridgeInstance(instance), nil
}

// UpdateInstance updates one persisted bridge instance without changing its
// lifecycle state.
func (s *Service) UpdateInstance(ctx context.Context, req UpdateInstanceRequest) (*BridgeInstance, error) {
	if err := s.checkReady(ctx, "update bridge instance"); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf(
			"bridges: update bridge instance %q: validate request: %w",
			strings.TrimSpace(req.ID),
			err,
		)
	}

	trimmedID := strings.TrimSpace(req.ID)
	instance, err := s.store.GetBridgeInstance(ctx, trimmedID)
	if err != nil {
		return nil, fmt.Errorf("bridges: update bridge instance %q: load current state: %w", trimmedID, err)
	}
	if instance.Source == BridgeInstanceSourcePackage {
		return nil, fmt.Errorf("bridges: update bridge instance %q: %w", trimmedID, ErrBridgeInstanceReadOnly)
	}
	if req.DisplayName != nil {
		instance.DisplayName = strings.TrimSpace(*req.DisplayName)
	}
	if req.DMPolicy != nil {
		instance.DMPolicy = req.DMPolicy.Normalize()
	}
	if req.RoutingPolicy != nil {
		instance.RoutingPolicy = *req.RoutingPolicy
	}
	if req.ProviderConfig != nil {
		normalized, err := normalizeProviderConfigJSON(*req.ProviderConfig)
		if err != nil {
			return nil, fmt.Errorf("bridges: update bridge instance %q: normalize provider config: %w", trimmedID, err)
		}
		instance.ProviderConfig = normalized
	}
	if req.DeliveryDefaults != nil {
		normalized, err := NormalizeDeliveryDefaultsJSON(*req.DeliveryDefaults)
		if err != nil {
			return nil, fmt.Errorf(
				"bridges: update bridge instance %q: normalize delivery defaults: %w",
				trimmedID,
				err,
			)
		}
		instance.DeliveryDefaults = normalized
	}
	if req.NotificationSuppress != nil {
		instance.NotificationSuppress = *req.NotificationSuppress
	}
	if req.ClearDegradation {
		instance.Degradation = nil
	}
	if req.Degradation != nil {
		degradation := req.Degradation.normalize()
		if degradation.IsZero() {
			instance.Degradation = nil
		} else {
			instance.Degradation = &degradation
		}
	}
	instance.UpdatedAt = req.UpdatedAt
	if instance.UpdatedAt.IsZero() {
		instance.UpdatedAt = s.now()
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("bridges: update bridge instance %q: validate updated state: %w", trimmedID, err)
	}
	if err := s.store.UpdateBridgeInstance(ctx, instance); err != nil {
		return nil, fmt.Errorf("bridges: update bridge instance %q: persist: %w", trimmedID, err)
	}
	return cloneBridgeInstance(instance), nil
}

// UpdateInstanceState applies one validated lifecycle transition to a persisted instance.
func (s *Service) UpdateInstanceState(ctx context.Context, req UpdateInstanceStateRequest) (*BridgeInstance, error) {
	if err := s.checkReady(ctx, "update bridge instance state"); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf(
			"bridges: update bridge instance state %q: validate request: %w",
			strings.TrimSpace(req.ID),
			err,
		)
	}

	trimmedID := strings.TrimSpace(req.ID)
	instance, err := s.store.GetBridgeInstance(ctx, trimmedID)
	if err != nil {
		return nil, fmt.Errorf("bridges: update bridge instance state %q: load current state: %w", trimmedID, err)
	}
	if err := ValidateInstanceStateTransition(instance, req.Enabled, req.Status); err != nil {
		return nil, fmt.Errorf("bridges: update bridge instance state %q: validate transition: %w", trimmedID, err)
	}

	instance.Enabled = req.Enabled
	instance.Status = req.Status.Normalize()
	switch {
	case req.ClearDegradation:
		instance.Degradation = nil
	case req.Degradation != nil:
		degradation := req.Degradation.normalize()
		if degradation.IsZero() {
			instance.Degradation = nil
		} else {
			instance.Degradation = &degradation
		}
	case instance.Status.Normalize() != BridgeStatusDegraded &&
		instance.Status.Normalize() != BridgeStatusAuthRequired &&
		instance.Status.Normalize() != BridgeStatusError:
		instance.Degradation = nil
	}
	instance.UpdatedAt = req.UpdatedAt
	if instance.UpdatedAt.IsZero() {
		instance.UpdatedAt = s.now()
	}
	if err := instance.Validate(); err != nil {
		return nil, fmt.Errorf("bridges: update bridge instance state %q: validate updated state: %w", trimmedID, err)
	}

	if err := s.store.UpdateBridgeInstance(ctx, instance); err != nil {
		return nil, fmt.Errorf("bridges: update bridge instance state %q: persist: %w", trimmedID, err)
	}
	return cloneBridgeInstance(instance), nil
}
