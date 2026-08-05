package contract

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// DeliveryMode identifies daemon-owned outbound delivery behavior.
type DeliveryMode string

const (
	DeliveryModeDirectSend DeliveryMode = "direct-send"
	DeliveryModeReply      DeliveryMode = "reply"
)

// Validate reports whether the delivery mode is supported.
func (m DeliveryMode) Validate() error {
	switch m {
	case DeliveryModeDirectSend, DeliveryModeReply:
		return nil
	case "":
		return errors.New("bridges: delivery target mode is required")
	default:
		return fmt.Errorf("bridges: unsupported delivery target mode %q", m)
	}
}

// DeliveryTarget identifies an outbound destination within one bridge instance.
type DeliveryTarget struct {
	BridgeInstanceID string       `json:"bridge_instance_id"`
	PeerID           string       `json:"peer_id,omitempty"`
	ThreadID         string       `json:"thread_id,omitempty"`
	GroupID          string       `json:"group_id,omitempty"`
	Mode             DeliveryMode `json:"mode,omitempty"`
}

// Validate reports whether the target contains the required identity.
func (t DeliveryTarget) Validate() error {
	if err := requireOpaqueIdentity(t.BridgeInstanceID, "delivery target bridge instance id"); err != nil {
		return err
	}
	for _, field := range []struct {
		value string
		label string
	}{
		{value: t.PeerID, label: "delivery target peer id"},
		{value: t.ThreadID, label: "delivery target thread id"},
		{value: t.GroupID, label: "delivery target group id"},
	} {
		if err := validateOptionalOpaqueIdentity(field.value, field.label); err != nil {
			return err
		}
	}
	if err := t.Mode.Validate(); err != nil {
		return err
	}
	if t.ThreadID != "" && t.PeerID == "" && t.GroupID == "" {
		return fmt.Errorf(
			"bridges: delivery target thread id requires peer id or group id for mode %q",
			t.Mode,
		)
	}
	if t.PeerID == "" && t.GroupID == "" {
		return fmt.Errorf(
			"bridges: delivery target mode %q requires peer id or group id",
			t.Mode,
		)
	}
	return nil
}

// IsZero reports whether the target carries any values.
func (t DeliveryTarget) IsZero() bool {
	return t.BridgeInstanceID == "" && t.PeerID == "" &&
		t.ThreadID == "" && t.GroupID == "" && t.Mode == ""
}

// MessageSender identifies the upstream actor that produced a message.
type MessageSender struct {
	ID          string `json:"id,omitempty"`
	Username    string `json:"username,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
}

// MessageContent carries normalized text content.
type MessageContent struct {
	Text string `json:"text,omitempty"`
}

// MessageAttachment captures normalized attachment metadata.
type MessageAttachment struct {
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	MIMEType string `json:"mime_type,omitempty"`
	URL      string `json:"url,omitempty"`
}

// DeliveryOperation identifies post, edit, or delete behavior.
type DeliveryOperation string

const (
	DeliveryOperationPost   DeliveryOperation = "post"
	DeliveryOperationEdit   DeliveryOperation = "edit"
	DeliveryOperationDelete DeliveryOperation = "delete"
)

// Normalize returns the canonical delivery operation.
func (o DeliveryOperation) Normalize() DeliveryOperation {
	return DeliveryOperation(strings.ToLower(strings.TrimSpace(string(o))))
}

// Validate reports whether the delivery operation is supported.
func (o DeliveryOperation) Validate() error {
	switch o.Normalize() {
	case "", DeliveryOperationPost, DeliveryOperationEdit, DeliveryOperationDelete:
		return nil
	default:
		return fmt.Errorf("bridges: unsupported delivery operation %q", strings.TrimSpace(string(o)))
	}
}

// DeliveryEventType is one closed daemon-to-adapter lifecycle event.
type DeliveryEventType string

const (
	DeliveryEventTypeStart    DeliveryEventType = "start"
	DeliveryEventTypeDelta    DeliveryEventType = "delta"
	DeliveryEventTypeFinal    DeliveryEventType = "final"
	DeliveryEventTypeError    DeliveryEventType = "error"
	DeliveryEventTypeResume   DeliveryEventType = "resume"
	DeliveryEventTypeDelete   DeliveryEventType = "delete"
	DeliveryEventTypeProgress DeliveryEventType = "progress"
)

// DeliveryEventTypeValues returns the closed wire values in stable order.
func DeliveryEventTypeValues() []string {
	return []string{
		string(DeliveryEventTypeStart), string(DeliveryEventTypeDelta),
		string(DeliveryEventTypeFinal), string(DeliveryEventTypeError),
		string(DeliveryEventTypeResume), string(DeliveryEventTypeDelete),
		string(DeliveryEventTypeProgress),
	}
}

// NormalizeDeliveryEventType returns the canonical lifecycle event type.
func NormalizeDeliveryEventType(value DeliveryEventType) DeliveryEventType {
	return normalizeDeliveryEventType(value)
}

// IsTerminalDeliveryEventType reports whether a lifecycle event is terminal.
func IsTerminalDeliveryEventType(value DeliveryEventType) bool {
	return isTerminalDeliveryEventType(value)
}

func normalizeDeliveryEventType(value DeliveryEventType) DeliveryEventType {
	return DeliveryEventType(strings.ToLower(strings.TrimSpace(string(value))))
}

func isTerminalDeliveryEventType(value DeliveryEventType) bool {
	switch normalizeDeliveryEventType(value) {
	case DeliveryEventTypeFinal, DeliveryEventTypeError, DeliveryEventTypeDelete:
		return true
	default:
		return false
	}
}

func validateDeliveryEventType(value DeliveryEventType, final bool) error {
	switch normalizeDeliveryEventType(value) {
	case DeliveryEventTypeStart:
		if final {
			return errors.New("bridges: delivery start event cannot be final")
		}
	case DeliveryEventTypeDelta:
		if final {
			return errors.New("bridges: delivery delta event cannot be final")
		}
	case DeliveryEventTypeProgress:
		if final {
			return errors.New("bridges: delivery progress event cannot be final")
		}
	case DeliveryEventTypeFinal:
		if !final {
			return errors.New("bridges: delivery final event must set final=true")
		}
	case DeliveryEventTypeError:
		if !final {
			return errors.New("bridges: delivery error event must set final=true")
		}
	case DeliveryEventTypeResume:
		return nil
	case DeliveryEventTypeDelete:
		if !final {
			return errors.New("bridges: delivery delete event must set final=true")
		}
	case "":
		return errors.New("bridges: delivery event type is required")
	default:
		return fmt.Errorf("bridges: unsupported delivery event type %q", strings.TrimSpace(string(value)))
	}
	return nil
}

// DeliveryMessageReference identifies one previously delivered message.
type DeliveryMessageReference struct {
	DeliveryID      string `json:"delivery_id,omitempty"`
	RemoteMessageID string `json:"remote_message_id,omitempty"`
}

// NormalizeDeliveryMessageReference returns a canonical prior-message handle.
func NormalizeDeliveryMessageReference(reference DeliveryMessageReference) DeliveryMessageReference {
	return reference.normalize()
}

// Validate reports whether the reference carries a prior message handle.
func (r DeliveryMessageReference) Validate() error {
	normalized := r.normalize()
	if normalized.DeliveryID != "" && !utf8.ValidString(normalized.DeliveryID) {
		return errors.New("bridges: delivery reference delivery id must be valid UTF-8")
	}
	if normalized.DeliveryID == "" && normalized.RemoteMessageID == "" {
		return errors.New("bridges: delivery reference requires delivery id or remote message id")
	}
	return nil
}

func (r DeliveryMessageReference) normalize() DeliveryMessageReference {
	return DeliveryMessageReference{
		DeliveryID: r.DeliveryID, RemoteMessageID: strings.TrimSpace(r.RemoteMessageID),
	}
}

// DeliveryErrorDetail captures a typed delivery failure.
type DeliveryErrorDetail struct {
	Message string `json:"message"`
}

// Validate reports whether the error carries a message.
func (d DeliveryErrorDetail) Validate() error {
	return requireField(strings.TrimSpace(d.Message), "delivery error message")
}

func (d DeliveryErrorDetail) normalize() DeliveryErrorDetail {
	return DeliveryErrorDetail{Message: strings.TrimSpace(d.Message)}
}

// DeliveryResumeState captures the resumable delivery phase.
type DeliveryResumeState struct {
	LatestEventType DeliveryEventType `json:"latest_event_type"`
}

// Validate reports whether the resume state references a supported event.
func (s DeliveryResumeState) Validate() error {
	normalized := s.normalize()
	if normalized.LatestEventType == "" {
		return errors.New("bridges: delivery resume latest event type is required")
	}
	if normalized.LatestEventType == DeliveryEventTypeResume {
		return errors.New("bridges: delivery resume latest event type cannot itself be resume")
	}
	return validateDeliveryEventType(
		normalized.LatestEventType,
		isTerminalDeliveryEventType(normalized.LatestEventType),
	)
}

func (s DeliveryResumeState) normalize() DeliveryResumeState {
	return DeliveryResumeState{LatestEventType: normalizeDeliveryEventType(s.LatestEventType)}
}

// DeliveryAckOutcome identifies the provider-side result of one delivery request.
type DeliveryAckOutcome string

const (
	DeliveryAckOutcomeSuccess                    DeliveryAckOutcome = "success"
	DeliveryAckOutcomeCommittedResultUnavailable DeliveryAckOutcome = "committed_result_unavailable"
)

// DeliveryAckOutcomeValues returns the closed acknowledgement outcomes in stable order.
func DeliveryAckOutcomeValues() []string {
	return []string{
		string(DeliveryAckOutcomeSuccess),
		string(DeliveryAckOutcomeCommittedResultUnavailable),
	}
}

// Normalize returns the canonical acknowledgement outcome; empty means success.
func (o DeliveryAckOutcome) Normalize() DeliveryAckOutcome {
	normalized := DeliveryAckOutcome(strings.ToLower(strings.TrimSpace(string(o))))
	if normalized == "" {
		return DeliveryAckOutcomeSuccess
	}
	return normalized
}

// Validate reports whether the acknowledgement outcome is supported.
func (o DeliveryAckOutcome) Validate() error {
	switch o.Normalize() {
	case DeliveryAckOutcomeSuccess, DeliveryAckOutcomeCommittedResultUnavailable:
		return nil
	default:
		return fmt.Errorf("bridges: unsupported delivery ack outcome %q", strings.TrimSpace(string(o)))
	}
}

// DeliveryAck acknowledges one delivery request without conflating receipt with provider success.
type DeliveryAck struct {
	DeliveryID             string               `json:"delivery_id"`
	Seq                    int64                `json:"seq"`
	RemoteMessageID        string               `json:"remote_message_id,omitempty"`
	ReplaceRemoteMessageID string               `json:"replace_remote_message_id,omitempty"`
	Outcome                DeliveryAckOutcome   `json:"outcome,omitempty"`
	Error                  *DeliveryErrorDetail `json:"error,omitempty"`
}

// NormalizeDeliveryAck returns a canonical acknowledgement.
func NormalizeDeliveryAck(ack DeliveryAck) DeliveryAck {
	return ack.normalize()
}

// ValidateFor reports whether the acknowledgement belongs to an event.
func (a DeliveryAck) ValidateFor(event DeliveryEvent) error {
	normalized := a.normalize()
	if err := normalized.Outcome.Validate(); err != nil {
		return err
	}
	if err := requireOpaqueIdentity(normalized.DeliveryID, "delivery ack delivery id"); err != nil {
		return err
	}
	if normalized.DeliveryID != event.DeliveryID {
		return fmt.Errorf(
			"bridges: delivery ack delivery id %q does not match event %q",
			normalized.DeliveryID,
			event.DeliveryID,
		)
	}
	if normalized.Seq != event.Seq {
		return fmt.Errorf("bridges: delivery ack sequence %d does not match event %d", normalized.Seq, event.Seq)
	}
	return normalized.validateOutcome()
}

func (a DeliveryAck) normalize() DeliveryAck {
	a.RemoteMessageID = strings.TrimSpace(a.RemoteMessageID)
	a.ReplaceRemoteMessageID = strings.TrimSpace(a.ReplaceRemoteMessageID)
	a.Outcome = a.Outcome.Normalize()
	if a.Error != nil {
		normalizedError := a.Error.normalize()
		a.Error = &normalizedError
	}
	return a
}

func (a DeliveryAck) validateOutcome() error {
	switch a.Outcome {
	case DeliveryAckOutcomeSuccess:
		if a.Error != nil {
			return errors.New("bridges: successful delivery ack cannot carry an error")
		}
		return nil
	case DeliveryAckOutcomeCommittedResultUnavailable:
		if a.Error == nil {
			return errors.New("bridges: committed-result-unavailable ack requires an error")
		}
		if err := a.Error.Validate(); err != nil {
			return fmt.Errorf("bridges: validate committed-result-unavailable ack error: %w", err)
		}
		if a.RemoteMessageID != "" || a.ReplaceRemoteMessageID != "" {
			return errors.New("bridges: committed-result-unavailable ack cannot carry remote message ids")
		}
		return nil
	default:
		return fmt.Errorf("bridges: unsupported delivery ack outcome %q", a.Outcome)
	}
}
