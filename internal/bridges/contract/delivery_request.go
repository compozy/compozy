package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// DeliveryRequest is the negotiated daemon-to-extension request payload.
type DeliveryRequest struct {
	Event    DeliveryEvent     `json:"event"`
	Snapshot *DeliverySnapshot `json:"snapshot,omitempty"`
}

// Validate reports whether the negotiated request is internally consistent.
func (r DeliveryRequest) Validate() error {
	eventType := normalizeDeliveryEventType(r.Event.EventType)
	if err := r.Event.Validate(); err != nil {
		return err
	}
	if r.Snapshot == nil {
		if eventType == DeliveryEventTypeResume {
			return errors.New("bridges: resume delivery request requires a snapshot")
		}
		return nil
	}
	if eventType != DeliveryEventTypeResume {
		return errors.New("bridges: only resume delivery requests may include a snapshot")
	}
	if err := r.Snapshot.Validate(); err != nil {
		return err
	}
	if r.Snapshot.DeliveryID != r.Event.DeliveryID {
		return errors.New("bridges: delivery request snapshot must match event delivery id")
	}
	if r.Snapshot.BridgeInstanceID != r.Event.BridgeInstanceID {
		return errors.New("bridges: delivery request snapshot must match event bridge instance id")
	}
	return nil
}

// DeliverySnapshot captures resumable state for one active delivery.
type DeliverySnapshot struct {
	DeliveryID             string                    `json:"delivery_id"`
	SessionID              string                    `json:"session_id"`
	TurnID                 string                    `json:"turn_id"`
	BridgeInstanceID       string                    `json:"bridge_instance_id"`
	RoutingKey             RoutingKey                `json:"routing_key"`
	DeliveryTarget         DeliveryTarget            `json:"delivery_target"`
	LatestSeq              int64                     `json:"latest_seq"`
	LatestEventType        DeliveryEventType         `json:"latest_event_type"`
	CurrentContent         MessageContent            `json:"current_content"`
	Operation              DeliveryOperation         `json:"operation,omitempty"`
	Reference              *DeliveryMessageReference `json:"reference,omitempty"`
	ProviderMetadata       json.RawMessage           `json:"provider_metadata,omitempty"`
	LastSentSeq            int64                     `json:"last_sent_seq,omitempty"`
	LastAckedSeq           int64                     `json:"last_acked_seq,omitempty"`
	RemoteMessageID        string                    `json:"remote_message_id,omitempty"`
	ReplaceRemoteMessageID string                    `json:"replace_remote_message_id,omitempty"`
	Final                  bool                      `json:"final"`
	Error                  string                    `json:"error,omitempty"`
	UpdatedAt              time.Time                 `json:"updated_at"`
}

// NormalizeDeliverySnapshot returns the canonical resumable wire snapshot.
func NormalizeDeliverySnapshot(snapshot DeliverySnapshot) DeliverySnapshot {
	normalized := snapshot.normalize()
	normalized.ProviderMetadata = slices.Clone(normalized.ProviderMetadata)
	return normalized
}

// Validate reports whether the snapshot can resume a delivery.
func (s DeliverySnapshot) Validate() error {
	normalized := s.normalize()
	if err := normalized.validateIdentity(); err != nil {
		return err
	}
	if err := normalized.validateRouting(); err != nil {
		return err
	}
	if err := normalized.validateSequences(); err != nil {
		return err
	}
	if err := normalized.validateOperationReference(); err != nil {
		return err
	}
	if _, err := normalizeRawJSON(normalized.ProviderMetadata, "delivery snapshot provider metadata"); err != nil {
		return err
	}
	return validateDeliveryEventType(normalized.LatestEventType, normalized.Final)
}

func (s DeliverySnapshot) validateIdentity() error {
	for _, required := range []struct {
		value string
		label string
	}{
		{s.DeliveryID, "delivery snapshot id"},
		{s.SessionID, "delivery snapshot session id"},
		{s.TurnID, "delivery snapshot turn id"},
		{s.BridgeInstanceID, "delivery snapshot bridge instance id"},
	} {
		if err := requireField(required.value, required.label); err != nil {
			return err
		}
	}
	if s.UpdatedAt.IsZero() {
		return errors.New("bridges: delivery snapshot updated at is required")
	}
	return nil
}

func (s DeliverySnapshot) validateRouting() error {
	if err := s.RoutingKey.Validate(); err != nil {
		return err
	}
	if s.RoutingKey.BridgeInstanceID != s.BridgeInstanceID {
		return errors.New("bridges: delivery snapshot bridge instance id must match routing key")
	}
	if err := s.DeliveryTarget.Validate(); err != nil {
		return err
	}
	if s.DeliveryTarget.BridgeInstanceID != s.BridgeInstanceID {
		return errors.New("bridges: delivery snapshot bridge instance id must match delivery target")
	}
	return nil
}

func (s DeliverySnapshot) validateSequences() error {
	if s.LatestSeq < 0 {
		return fmt.Errorf("bridges: invalid delivery snapshot latest sequence %d", s.LatestSeq)
	}
	if s.LastSentSeq < 0 {
		return fmt.Errorf("bridges: invalid delivery snapshot last sent sequence %d", s.LastSentSeq)
	}
	if s.LastAckedSeq < 0 {
		return fmt.Errorf("bridges: invalid delivery snapshot last acked sequence %d", s.LastAckedSeq)
	}
	if s.LastAckedSeq > s.LastSentSeq {
		return errors.New("bridges: delivery snapshot last acked sequence cannot exceed last sent sequence")
	}
	if s.LastSentSeq > s.LatestSeq {
		return errors.New("bridges: delivery snapshot last sent sequence cannot exceed latest sequence")
	}
	return nil
}

func (s DeliverySnapshot) validateOperationReference() error {
	if err := s.Operation.Validate(); err != nil {
		return err
	}
	if s.Operation == DeliveryOperationPost {
		if s.Reference != nil {
			return errors.New("bridges: delivery snapshot post operation cannot include a reference")
		}
		return nil
	}
	if s.Reference == nil {
		return fmt.Errorf("bridges: delivery snapshot %s operation requires a reference", s.Operation)
	}
	return s.Reference.Validate()
}

func (s DeliverySnapshot) normalize() DeliverySnapshot {
	s.DeliveryID = strings.TrimSpace(s.DeliveryID)
	s.SessionID = strings.TrimSpace(s.SessionID)
	s.TurnID = strings.TrimSpace(s.TurnID)
	s.BridgeInstanceID = strings.TrimSpace(s.BridgeInstanceID)
	s.RoutingKey = s.RoutingKey.normalize()
	s.DeliveryTarget = s.DeliveryTarget.normalize()
	s.LatestEventType = normalizeDeliveryEventType(s.LatestEventType)
	s.Operation = s.Operation.Normalize()
	if s.Operation == "" {
		s.Operation = DeliveryOperationPost
	}
	s.ProviderMetadata = bytes.TrimSpace(s.ProviderMetadata)
	s.RemoteMessageID = strings.TrimSpace(s.RemoteMessageID)
	s.ReplaceRemoteMessageID = strings.TrimSpace(s.ReplaceRemoteMessageID)
	s.Error = strings.TrimSpace(s.Error)
	if s.Reference != nil {
		reference := s.Reference.normalize()
		s.Reference = &reference
	}
	return s
}
