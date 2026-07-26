package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// DeliveryEvent is the daemon-owned outbound projection sent to an adapter.
type DeliveryEvent struct {
	DeliveryID       string                    `json:"delivery_id"`
	BridgeInstanceID string                    `json:"bridge_instance_id"`
	RoutingKey       RoutingKey                `json:"routing_key"`
	DeliveryTarget   DeliveryTarget            `json:"delivery_target"`
	Seq              int64                     `json:"seq"`
	EventType        DeliveryEventType         `json:"event_type"`
	Content          MessageContent            `json:"content"`
	Final            bool                      `json:"final"`
	Operation        DeliveryOperation         `json:"operation,omitempty"`
	Reference        *DeliveryMessageReference `json:"reference,omitempty"`
	Error            *DeliveryErrorDetail      `json:"error,omitempty"`
	Resume           *DeliveryResumeState      `json:"resume,omitempty"`
	Progress         *ToolProgress             `json:"progress,omitempty"`
	ProviderMetadata json.RawMessage           `json:"provider_metadata,omitempty"`
}

// NormalizeDeliveryEvent returns the canonical wire event with independently
// owned typed payloads.
func NormalizeDeliveryEvent(event DeliveryEvent) DeliveryEvent {
	normalized := event.normalize()
	normalized.ProviderMetadata = slices.Clone(normalized.ProviderMetadata)
	return normalized
}

// Validate reports whether the delivery event is internally consistent.
func (e DeliveryEvent) Validate() error {
	normalized := e.normalize()
	if err := requireField(normalized.DeliveryID, "delivery event id"); err != nil {
		return err
	}
	if err := requireField(normalized.BridgeInstanceID, "delivery event bridge instance id"); err != nil {
		return err
	}
	if err := normalized.RoutingKey.Validate(); err != nil {
		return err
	}
	if normalized.RoutingKey.BridgeInstanceID != normalized.BridgeInstanceID {
		return errors.New("bridges: delivery event bridge instance id must match routing key")
	}
	if !normalized.DeliveryTarget.IsZero() {
		if err := normalized.DeliveryTarget.Validate(); err != nil {
			return err
		}
		if normalized.DeliveryTarget.BridgeInstanceID != normalized.BridgeInstanceID {
			return errors.New("bridges: delivery target bridge instance id must match delivery event")
		}
	}
	if normalized.Seq < 0 {
		return fmt.Errorf("bridges: invalid delivery event sequence %d", normalized.Seq)
	}
	if err := normalized.Operation.Validate(); err != nil {
		return err
	}
	if err := validateDeliveryEventType(normalized.EventType, normalized.Final); err != nil {
		return err
	}
	if _, err := normalizeRawJSON(normalized.ProviderMetadata, "delivery event provider metadata"); err != nil {
		return err
	}
	if err := normalized.validateOperation(); err != nil {
		return err
	}
	return normalized.validateTypedFields()
}

// IsLifecycleOnlyFinal reports whether the event closes progress without text.
func (e DeliveryEvent) IsLifecycleOnlyFinal() bool {
	operation := e.Operation.Normalize()
	if operation == "" {
		operation = DeliveryOperationPost
	}
	return normalizeDeliveryEventType(e.EventType) == DeliveryEventTypeFinal &&
		operation == DeliveryOperationPost &&
		strings.TrimSpace(e.Content.Text) == ""
}

func (e DeliveryEvent) normalize() DeliveryEvent {
	e.DeliveryID = strings.TrimSpace(e.DeliveryID)
	e.BridgeInstanceID = strings.TrimSpace(e.BridgeInstanceID)
	e.RoutingKey = e.RoutingKey.normalize()
	e.DeliveryTarget = e.DeliveryTarget.normalize()
	e.EventType = normalizeDeliveryEventType(e.EventType)
	e.Content = MessageContent{Text: strings.TrimSpace(e.Content.Text)}
	e.Operation = e.Operation.Normalize()
	if e.Operation == "" {
		e.Operation = DeliveryOperationPost
	}
	e.ProviderMetadata = bytes.TrimSpace(e.ProviderMetadata)
	if e.Reference != nil {
		reference := e.Reference.normalize()
		e.Reference = &reference
	}
	if e.Error != nil {
		detail := e.Error.normalize()
		e.Error = &detail
	}
	if e.Resume != nil {
		resume := e.Resume.normalize()
		e.Resume = &resume
	}
	e.Progress = cloneToolProgress(e.Progress)
	return e
}

func (e DeliveryEvent) validateOperation() error {
	switch e.Operation {
	case DeliveryOperationPost:
		if e.Reference != nil {
			return errors.New("bridges: delivery post operation cannot include a reference")
		}
	case DeliveryOperationEdit, DeliveryOperationDelete:
		if e.Reference == nil {
			return fmt.Errorf("bridges: delivery %s operation requires a reference", e.Operation)
		}
		if err := e.Reference.Validate(); err != nil {
			return err
		}
	}
	if e.EventType == DeliveryEventTypeDelete && e.Operation != DeliveryOperationDelete {
		return errors.New("bridges: delete delivery events must use delete operation")
	}
	if e.EventType != DeliveryEventTypeDelete && e.Operation == DeliveryOperationDelete {
		return errors.New("bridges: delete operation requires delete event type")
	}
	if e.EventType == DeliveryEventTypeProgress && e.Operation != DeliveryOperationPost {
		return errors.New("bridges: progress delivery events must use post operation")
	}
	return nil
}
