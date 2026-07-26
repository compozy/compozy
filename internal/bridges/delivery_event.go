package bridges

import (
	"encoding/json"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

// DeliveryMessageReference identifies one previously delivered message.
type DeliveryMessageReference struct {
	DeliveryID      string `json:"delivery_id,omitempty"`
	RemoteMessageID string `json:"remote_message_id,omitempty"`
}

// Validate reports whether the reference identifies at least one prior message handle.
func (r DeliveryMessageReference) Validate() error {
	return deliveryMessageReferenceToContract(r).Validate()
}

// DeliveryErrorDetail captures one typed delivery failure payload.
type DeliveryErrorDetail struct {
	Message string `json:"message"`
}

// Validate reports whether the error detail carries a message.
func (d DeliveryErrorDetail) Validate() error {
	return deliveryErrorDetailToContract(d).Validate()
}

// DeliveryResumeState captures the typed resumable delivery phase.
type DeliveryResumeState struct {
	LatestEventType DeliveryEventType `json:"latest_event_type"`
}

// Validate reports whether the resume state references a supported prior event type.
func (s DeliveryResumeState) Validate() error {
	return deliveryResumeStateToContract(s).Validate()
}

// DeliveryEvent is the daemon-owned outbound projection sent to a bridge adapter.
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

// Validate reports whether the delivery event contains the required identifiers.
func (e DeliveryEvent) Validate() error {
	return deliveryEventToContract(e).Validate()
}

func (e DeliveryEvent) normalize() DeliveryEvent {
	return deliveryEventFromContract(
		bridgecontract.NormalizeDeliveryEvent(deliveryEventToContract(e)),
	)
}

// IsLifecycleOnlyFinal reports whether the event closes a progress-only POST
// without materializing or replacing provider message content.
func (e DeliveryEvent) IsLifecycleOnlyFinal() bool {
	return deliveryEventToContract(e).IsLifecycleOnlyFinal()
}

func (r DeliveryMessageReference) normalize() DeliveryMessageReference {
	return deliveryMessageReferenceFromContract(
		bridgecontract.NormalizeDeliveryMessageReference(deliveryMessageReferenceToContract(r)),
	)
}
