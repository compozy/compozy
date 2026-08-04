package bridges

import (
	"context"
	"encoding/json"
	"fmt"

	bridgecontract "github.com/compozy/compozy/internal/bridges/contract"
)

// DeliveryMode identifies the daemon-owned outbound delivery behavior requested
// for one canonical bridge target.
type DeliveryMode string

const (
	// DeliveryModeDirectSend sends a fresh outbound message into the target conversation.
	DeliveryModeDirectSend DeliveryMode = DeliveryMode(bridgecontract.DeliveryModeDirectSend)
	// DeliveryModeReply sends an outbound reply within the resolved conversation context.
	DeliveryModeReply DeliveryMode = DeliveryMode(bridgecontract.DeliveryModeReply)
)

// Validate reports whether the delivery mode belongs to the supported mode set.
func (m DeliveryMode) Validate() error {
	return bridgecontract.DeliveryMode(m).Validate()
}

// ResolveDeliveryTargetRequest captures one outbound target request before
// bridge-instance defaults have been merged in.
type ResolveDeliveryTargetRequest struct {
	BridgeInstanceID string       `json:"bridge_instance_id"`
	PeerID           string       `json:"peer_id,omitempty"`
	ThreadID         string       `json:"thread_id,omitempty"`
	GroupID          string       `json:"group_id,omitempty"`
	Mode             DeliveryMode `json:"mode,omitempty"`
}

// Validate reports whether the request carries valid opaque identities and an
// exact supported mode when one is explicitly supplied.
func (r ResolveDeliveryTargetRequest) Validate() error {
	if err := requireOpaqueIdentity(r.BridgeInstanceID, "delivery target request bridge instance id"); err != nil {
		return err
	}
	for _, field := range []struct {
		value string
		label string
	}{
		{value: r.PeerID, label: "delivery target request peer id"},
		{value: r.ThreadID, label: "delivery target request thread id"},
		{value: r.GroupID, label: "delivery target request group id"},
	} {
		if isBlank(field.value) {
			continue
		}
		if err := requireOpaqueIdentity(field.value, field.label); err != nil {
			return err
		}
	}
	if r.Mode != "" {
		return r.Mode.Validate()
	}
	return nil
}

// TargetResolver resolves one canonical outbound delivery target from bridge
// instance metadata plus explicit destination overrides.
type TargetResolver interface {
	ResolveDeliveryTarget(ctx context.Context, req ResolveDeliveryTargetRequest) (*DeliveryTarget, error)
}

var _ TargetResolver = (*Service)(nil)

type deliveryTargetDefaults struct {
	PeerID   string       `json:"peer_id,omitempty"`
	ThreadID string       `json:"thread_id,omitempty"`
	GroupID  string       `json:"group_id,omitempty"`
	Mode     DeliveryMode `json:"mode,omitempty"`
}

// BuildDeliveryTarget merges bridge-instance delivery defaults with explicit
// request overrides and returns one canonical outbound target.
func BuildDeliveryTarget(instance BridgeInstance, req ResolveDeliveryTargetRequest) (DeliveryTarget, error) {
	normalizedInstance := instance.normalize()
	if err := normalizedInstance.Validate(); err != nil {
		return DeliveryTarget{}, err
	}

	if err := req.Validate(); err != nil {
		return DeliveryTarget{}, err
	}
	if req.BridgeInstanceID != normalizedInstance.ID {
		return DeliveryTarget{}, fmt.Errorf(
			"bridges: delivery target request bridge instance id %q does not match instance %q",
			req.BridgeInstanceID,
			normalizedInstance.ID,
		)
	}

	defaults, err := decodeDeliveryTargetDefaults(normalizedInstance.DeliveryDefaults)
	if err != nil {
		return DeliveryTarget{}, err
	}

	target := DeliveryTarget{
		BridgeInstanceID: normalizedInstance.ID,
		PeerID:           firstNonEmpty(req.PeerID, defaults.PeerID),
		ThreadID:         firstNonEmpty(req.ThreadID, defaults.ThreadID),
		GroupID:          firstNonEmpty(req.GroupID, defaults.GroupID),
		Mode:             req.Mode,
	}
	if target.Mode == "" {
		target.Mode = defaults.Mode
	}
	if target.Mode == "" {
		target.Mode = DeliveryModeDirectSend
	}

	if err := target.Validate(); err != nil {
		return DeliveryTarget{}, err
	}
	return target, nil
}

// ResolveDeliveryTarget loads the owning bridge instance and resolves the
// canonical outbound target under that instance's delivery defaults.
func (s *Service) ResolveDeliveryTarget(
	ctx context.Context,
	req ResolveDeliveryTargetRequest,
) (*DeliveryTarget, error) {
	if err := s.checkReady(ctx, "resolve delivery target"); err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	instance, err := s.loadRoutableInstance(ctx, req.BridgeInstanceID)
	if err != nil {
		return nil, err
	}

	target, err := BuildDeliveryTarget(instance, req)
	if err != nil {
		return nil, err
	}
	return new(target), nil
}

func decodeDeliveryTargetDefaults(raw json.RawMessage) (deliveryTargetDefaults, error) {
	normalized, err := normalizeRawJSON(raw, "bridge instance delivery defaults")
	if err != nil {
		return deliveryTargetDefaults{}, err
	}
	if len(normalized) == 0 {
		return deliveryTargetDefaults{}, nil
	}

	var defaults deliveryTargetDefaults
	if err := json.Unmarshal(normalized, &defaults); err != nil {
		return deliveryTargetDefaults{}, fmt.Errorf("bridges: decode bridge instance delivery defaults: %w", err)
	}

	if defaults.Mode != "" {
		if err := defaults.Mode.Validate(); err != nil {
			return deliveryTargetDefaults{}, err
		}
	}
	return defaults, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if !isBlank(value) {
			return value
		}
	}
	return ""
}
