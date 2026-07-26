package bridges

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
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

// Normalize returns the canonical delivery mode representation.
func (m DeliveryMode) Normalize() DeliveryMode {
	return DeliveryMode(bridgecontract.DeliveryMode(m).Normalize())
}

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

// Validate reports whether the request identifies the owning bridge instance.
func (r ResolveDeliveryTargetRequest) Validate() error {
	return requireField(strings.TrimSpace(r.BridgeInstanceID), "delivery target request bridge instance id")
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

	normalizedReq := req.normalize()
	if err := normalizedReq.Validate(); err != nil {
		return DeliveryTarget{}, err
	}
	if normalizedReq.BridgeInstanceID != normalizedInstance.ID {
		return DeliveryTarget{}, fmt.Errorf(
			"bridges: delivery target request bridge instance id %q does not match instance %q",
			normalizedReq.BridgeInstanceID,
			normalizedInstance.ID,
		)
	}

	defaults, err := decodeDeliveryTargetDefaults(normalizedInstance.DeliveryDefaults)
	if err != nil {
		return DeliveryTarget{}, err
	}

	target := DeliveryTarget{
		BridgeInstanceID: normalizedInstance.ID,
		PeerID:           firstNonEmpty(normalizedReq.PeerID, defaults.PeerID),
		ThreadID:         firstNonEmpty(normalizedReq.ThreadID, defaults.ThreadID),
		GroupID:          firstNonEmpty(normalizedReq.GroupID, defaults.GroupID),
		Mode:             normalizedReq.Mode,
	}
	if target.Mode == "" {
		target.Mode = defaults.Mode
	}
	if target.Mode == "" {
		target.Mode = DeliveryModeDirectSend
	}

	canonical := target.normalize()
	if err := canonical.Validate(); err != nil {
		return DeliveryTarget{}, err
	}
	return canonical, nil
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

	normalizedReq := req.normalize()
	if err := normalizedReq.Validate(); err != nil {
		return nil, err
	}

	instance, err := s.loadRoutableInstance(ctx, normalizedReq.BridgeInstanceID)
	if err != nil {
		return nil, err
	}

	target, err := BuildDeliveryTarget(instance, normalizedReq)
	if err != nil {
		return nil, err
	}
	return cloneDeliveryTarget(target), nil
}

func (r ResolveDeliveryTargetRequest) normalize() ResolveDeliveryTargetRequest {
	normalized := r
	normalized.BridgeInstanceID = strings.TrimSpace(normalized.BridgeInstanceID)
	normalized.PeerID = strings.TrimSpace(normalized.PeerID)
	normalized.ThreadID = strings.TrimSpace(normalized.ThreadID)
	normalized.GroupID = strings.TrimSpace(normalized.GroupID)
	normalized.Mode = normalized.Mode.Normalize()
	return normalized
}

func (d deliveryTargetDefaults) normalize() deliveryTargetDefaults {
	return deliveryTargetDefaults{
		PeerID:   strings.TrimSpace(d.PeerID),
		ThreadID: strings.TrimSpace(d.ThreadID),
		GroupID:  strings.TrimSpace(d.GroupID),
		Mode:     d.Mode.Normalize(),
	}
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

	defaults = defaults.normalize()
	if defaults.Mode != "" {
		if err := defaults.Mode.Validate(); err != nil {
			return deliveryTargetDefaults{}, err
		}
	}
	return defaults, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func cloneDeliveryTarget(target DeliveryTarget) *DeliveryTarget {
	cloned := target.normalize()
	return &cloned
}
