package bridges

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/toolmeta"
)

var (
	// ErrDeliveryNotFound reports that no active or retained delivery matched the lookup.
	ErrDeliveryNotFound = errors.New("bridges: delivery not found")
	// ErrDeliveryQueueSaturated reports that a bounded delivery queue could not accept more work.
	ErrDeliveryQueueSaturated = errors.New("bridges: delivery queue saturated")
	// ErrDeliveryIDConflict reports that a caller-supplied delivery id is already active.
	ErrDeliveryIDConflict = errors.New("bridges: delivery id conflict")
	// ErrDeliveryTransportUnavailable reports that the broker has no usable extension delivery transport.
	ErrDeliveryTransportUnavailable = errors.New("bridges: delivery transport unavailable")
)

// DeliveryEventType is one closed daemon-to-adapter delivery lifecycle event.
type DeliveryEventType string

const (
	// DeliveryEventTypeStart starts one progressive outbound delivery for a prompt turn.
	DeliveryEventTypeStart DeliveryEventType = DeliveryEventType(bridgecontract.DeliveryEventTypeStart)
	// DeliveryEventTypeDelta updates one progressive outbound delivery with newer full text.
	DeliveryEventTypeDelta DeliveryEventType = DeliveryEventType(bridgecontract.DeliveryEventTypeDelta)
	// DeliveryEventTypeFinal reports the terminal successful state for one delivery.
	DeliveryEventTypeFinal DeliveryEventType = DeliveryEventType(bridgecontract.DeliveryEventTypeFinal)
	// DeliveryEventTypeError reports the terminal failed state for one delivery.
	DeliveryEventTypeError DeliveryEventType = DeliveryEventType(bridgecontract.DeliveryEventTypeError)
	// DeliveryEventTypeResume rehydrates the latest delivery snapshot after adapter recovery.
	DeliveryEventTypeResume DeliveryEventType = DeliveryEventType(bridgecontract.DeliveryEventTypeResume)
	// DeliveryEventTypeDelete removes one previously delivered message.
	DeliveryEventTypeDelete DeliveryEventType = DeliveryEventType(bridgecontract.DeliveryEventTypeDelete)
	// DeliveryEventTypeProgress carries presentation-only tool lifecycle chrome.
	DeliveryEventTypeProgress DeliveryEventType = DeliveryEventType(bridgecontract.DeliveryEventTypeProgress)
)

const (
	defaultDeliveryQueueCapacity  = 4
	defaultDeliveryRetryDelay     = 25 * time.Millisecond
	defaultDeliveryRequestTimeout = 5 * time.Second
)

// DeliveryTransport delivers negotiated daemon->extension bridge requests.
// The extension name remains explicit because the broker owns routing semantics,
// while the extension manager owns the subprocess runtime.
type DeliveryTransport interface {
	DeliverBridge(ctx context.Context, extensionName string, req DeliveryRequest) (DeliveryAck, error)
}

// DeliveryProjectionEvent is the reduced session-event shape the broker needs
// to project prompt output into delivery-oriented bridge events. It remains
// ACP-agnostic so `internal/bridges` does not depend on runtime transport packages.
type DeliveryProjectionEvent struct {
	Type         string                      `json:"type"`
	TurnID       string                      `json:"turn_id"`
	Timestamp    time.Time                   `json:"timestamp"`
	Text         string                      `json:"text,omitempty"`
	Error        string                      `json:"error,omitempty"`
	Fingerprint  string                      `json:"fingerprint,omitempty"`
	Tool         *ToolProgress               `json:"tool,omitempty"`
	ToolInput    json.RawMessage             `json:"-"`
	ToolMetadata toolmeta.DescriptorMetadata `json:"-"`
}

// DeliveryRequest is the negotiated daemon->extension request payload for
// `bridges/deliver`. Regular streaming requests carry only Event. Recovery
// requests also carry Snapshot and use EventTypeResume.
type DeliveryRequest struct {
	Event    DeliveryEvent     `json:"event"`
	Snapshot *DeliverySnapshot `json:"snapshot,omitempty"`
}

// Validate reports whether the negotiated request is internally consistent.
func (r DeliveryRequest) Validate() error {
	return deliveryRequestToContract(r).Validate()
}

// DeliveryAckOutcome identifies the provider-side result of one delivery request.
type DeliveryAckOutcome string

const (
	DeliveryAckOutcomeSuccess DeliveryAckOutcome = DeliveryAckOutcome(
		bridgecontract.DeliveryAckOutcomeSuccess,
	)
	DeliveryAckOutcomeCommittedResultUnavailable DeliveryAckOutcome = DeliveryAckOutcome(
		bridgecontract.DeliveryAckOutcomeCommittedResultUnavailable,
	)
)

// DeliveryResultUnavailableMessage is the safe public explanation used when a
// provider mutation may have committed but no trustworthy result was returned.
const DeliveryResultUnavailableMessage = "provider mutation may have committed, but the bridge delivery " +
	"result is unavailable"

// Normalize returns the canonical acknowledgement outcome; empty means success.
func (o DeliveryAckOutcome) Normalize() DeliveryAckOutcome {
	return DeliveryAckOutcome(bridgecontract.DeliveryAckOutcome(o).Normalize())
}

// Validate reports whether the acknowledgement outcome is supported.
func (o DeliveryAckOutcome) Validate() error {
	return bridgecontract.DeliveryAckOutcome(o).Validate()
}

// DeliveryAck is the negotiated extension->daemon result for one `bridges/deliver` request.
type DeliveryAck struct {
	DeliveryID             string               `json:"delivery_id"`
	Seq                    int64                `json:"seq"`
	RemoteMessageID        string               `json:"remote_message_id,omitempty"`
	ReplaceRemoteMessageID string               `json:"replace_remote_message_id,omitempty"`
	Outcome                DeliveryAckOutcome   `json:"outcome,omitempty"`
	Error                  *DeliveryErrorDetail `json:"error,omitempty"`
	wireDecoded            bool
	wireSeqPresent         bool
}

// UnmarshalJSON preserves whether the required sequence was explicitly sent.
// Sequence zero is valid, so the Go zero value cannot distinguish it from an
// absent or null JSON field without this wire-presence bit.
func (a *DeliveryAck) UnmarshalJSON(data []byte) error {
	type deliveryAckWire struct {
		DeliveryID             string               `json:"delivery_id"`
		Seq                    json.RawMessage      `json:"seq"`
		RemoteMessageID        string               `json:"remote_message_id,omitempty"`
		ReplaceRemoteMessageID string               `json:"replace_remote_message_id,omitempty"`
		Outcome                DeliveryAckOutcome   `json:"outcome,omitempty"`
		Error                  *DeliveryErrorDetail `json:"error,omitempty"`
	}

	var decoded deliveryAckWire
	if err := json.Unmarshal(data, &decoded); err != nil {
		return fmt.Errorf("bridges: decode delivery acknowledgement: %w", err)
	}

	*a = DeliveryAck{
		DeliveryID:             decoded.DeliveryID,
		RemoteMessageID:        decoded.RemoteMessageID,
		ReplaceRemoteMessageID: decoded.ReplaceRemoteMessageID,
		Outcome:                decoded.Outcome,
		Error:                  decoded.Error,
		wireDecoded:            true,
	}
	sequence := bytes.TrimSpace(decoded.Seq)
	if len(sequence) == 0 || bytes.Equal(sequence, []byte("null")) {
		return nil
	}
	if err := json.Unmarshal(sequence, &a.Seq); err != nil {
		return fmt.Errorf("bridges: decode delivery acknowledgement sequence: %w", err)
	}
	a.wireSeqPresent = true
	return nil
}

// ValidateFor reports whether the acknowledgement still belongs to the request
// that triggered it.
func (a DeliveryAck) ValidateFor(event DeliveryEvent) error {
	if a.wireDecoded && !a.wireSeqPresent {
		return errors.New("bridges: delivery ack sequence is required")
	}
	return deliveryAckToContract(a).ValidateFor(deliveryEventToContract(event))
}

// DeliverySnapshot captures the current progressive state for one active
// delivery so the broker can resume it after adapter recovery.
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

// Validate reports whether the snapshot contains the state needed to resume a
// negotiated bridge delivery.
func (s DeliverySnapshot) Validate() error {
	return deliverySnapshotToContract(s).Validate()
}

// PromptDeliveryRegistration binds one session prompt turn to a routed bridge
// delivery stream before or shortly after the prompt begins emitting events.
type PromptDeliveryRegistration struct {
	SessionID      string                    `json:"session_id"`
	TurnID         string                    `json:"turn_id"`
	ExtensionName  string                    `json:"extension_name"`
	DeliveryID     string                    `json:"delivery_id,omitempty"`
	RoutingKey     RoutingKey                `json:"routing_key"`
	DeliveryTarget DeliveryTarget            `json:"delivery_target"`
	Progress       ProgressConfig            `json:"progress"`
	SeedEvents     []DeliveryProjectionEvent `json:"seed_events,omitempty"`
}

// Validate reports whether the registration contains enough routed context to
// project session output into a negotiated delivery stream.
func (r PromptDeliveryRegistration) Validate() error {
	normalized := r.normalize()
	if err := requireField(normalized.SessionID, "prompt delivery registration session id"); err != nil {
		return err
	}
	if err := requireField(normalized.TurnID, "prompt delivery registration turn id"); err != nil {
		return err
	}
	if err := requireField(normalized.ExtensionName, "prompt delivery registration extension name"); err != nil {
		return err
	}
	if err := normalized.RoutingKey.Validate(); err != nil {
		return err
	}
	if err := normalized.DeliveryTarget.Validate(); err != nil {
		return err
	}
	if normalized.DeliveryTarget.BridgeInstanceID != normalized.RoutingKey.BridgeInstanceID {
		return errors.New("bridges: prompt delivery registration target must match routing key bridge instance")
	}
	if err := normalized.Progress.Validate(); err != nil {
		return err
	}
	return nil
}

// DeliveryBrokerOption customizes delivery-broker construction.
type DeliveryBrokerOption func(*Broker)

// WithDeliveryBrokerDescriptorLookup resolves registry-owned tool presentation metadata.
func WithDeliveryBrokerDescriptorLookup(lookup toolmeta.DescriptorLookup) DeliveryBrokerOption {
	return func(b *Broker) {
		b.descriptors = lookup
	}
}

// WithDeliveryBrokerNow overrides the broker clock, mainly for tests.
func WithDeliveryBrokerNow(now func() time.Time) DeliveryBrokerOption {
	return func(b *Broker) {
		if now != nil {
			b.now = now
		}
	}
}

// WithDeliveryBrokerQueueCapacity overrides the bounded queue length per routed
// delivery worker. Values below 2 are raised to 2 so `start` and one terminal
// event can coexist under pressure.
func WithDeliveryBrokerQueueCapacity(capacity int) DeliveryBrokerOption {
	return func(b *Broker) {
		if capacity < 2 {
			capacity = 2
		}
		b.queueCapacity = capacity
	}
}

// WithDeliveryBrokerRetryDelay overrides the backoff between retry attempts
// after a delivery-transport failure.
func WithDeliveryBrokerRetryDelay(delay time.Duration) DeliveryBrokerOption {
	return func(b *Broker) {
		if delay > 0 {
			b.retryDelay = delay
		}
	}
}

// WithDeliveryBrokerRequestTimeout overrides the timeout applied to one
// negotiated `bridges/deliver` call.
func WithDeliveryBrokerRequestTimeout(timeout time.Duration) DeliveryBrokerOption {
	return func(b *Broker) {
		if timeout > 0 {
			b.requestTimeout = timeout
		}
	}
}

// WithDeliveryBrokerLifecycleContext injects the broker-owned lifecycle context
// used by background route workers.
func WithDeliveryBrokerLifecycleContext(ctx context.Context) DeliveryBrokerOption {
	return func(b *Broker) {
		if ctx != nil {
			b.lifecycleCtx = ctx
		}
	}
}

func normalizeDeliveryEventType(value DeliveryEventType) DeliveryEventType {
	return DeliveryEventType(bridgecontract.NormalizeDeliveryEventType(bridgecontract.DeliveryEventType(value)))
}

func isTerminalDeliveryEventType(value DeliveryEventType) bool {
	return bridgecontract.IsTerminalDeliveryEventType(bridgecontract.DeliveryEventType(value))
}

func (a DeliveryAck) normalize() DeliveryAck {
	return deliveryAckFromContract(bridgecontract.NormalizeDeliveryAck(deliveryAckToContract(a)))
}

func (s DeliverySnapshot) normalize() DeliverySnapshot {
	return deliverySnapshotFromContract(
		bridgecontract.NormalizeDeliverySnapshot(deliverySnapshotToContract(s)),
	)
}

func (r PromptDeliveryRegistration) normalize() PromptDeliveryRegistration {
	normalized := r
	normalized.SessionID = strings.TrimSpace(normalized.SessionID)
	normalized.TurnID = strings.TrimSpace(normalized.TurnID)
	normalized.ExtensionName = strings.TrimSpace(normalized.ExtensionName)
	normalized.DeliveryID = strings.TrimSpace(normalized.DeliveryID)
	normalized.RoutingKey = normalized.RoutingKey.normalize()
	normalized.DeliveryTarget = normalized.DeliveryTarget.normalize()
	if normalized.Progress.ToolProgress == "" || normalized.Progress.Grouping == "" {
		normalized.Progress = ResolveProgressConfig(nil, "")
	} else {
		normalized.Progress = normalized.Progress.effective()
	}
	if len(normalized.SeedEvents) > 0 {
		normalized.SeedEvents = append([]DeliveryProjectionEvent(nil), normalized.SeedEvents...)
		for idx := range normalized.SeedEvents {
			normalized.SeedEvents[idx] = normalized.SeedEvents[idx].normalize()
		}
	}
	return normalized
}

func (e DeliveryProjectionEvent) normalize() DeliveryProjectionEvent {
	normalized := e
	normalized.Type = strings.TrimSpace(normalized.Type)
	normalized.TurnID = strings.TrimSpace(normalized.TurnID)
	normalized.Error = strings.TrimSpace(normalized.Error)
	normalized.Fingerprint = strings.TrimSpace(normalized.Fingerprint)
	normalized.Tool = cloneToolProgress(normalized.Tool)
	normalized.ToolInput = cloneRawJSON(normalized.ToolInput)
	return normalized
}
