// Package network defines the AGH Network v0 protocol surface shared by the
// transport, router, and delivery layers.
package network

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ProtocolV0 is the workspace-qualified wire protocol identifier.
const ProtocolV0 = "agh-network/v0"

// Kind identifies one normative AGH Network message kind.
type Kind string

const (
	KindGreet      Kind = "greet"
	KindWhois      Kind = "whois"
	KindSay        Kind = "say"
	KindCapability Kind = "capability"
	KindReceipt    Kind = "receipt"
	KindTrace      Kind = "trace"
)

var validKinds = map[Kind]struct{}{
	KindGreet:      {},
	KindWhois:      {},
	KindSay:        {},
	KindCapability: {},
	KindReceipt:    {},
	KindTrace:      {},
}

// Validate reports whether the kind is one of the documented RFC values.
func (k Kind) Validate() error {
	if _, ok := validKinds[k]; !ok {
		return fmt.Errorf("%w: kind=%q", ErrInvalidKind, string(k))
	}
	return nil
}

// Surface identifies the conversation container class for one message.
type Surface string

const (
	SurfaceThread Surface = "thread"
	SurfaceDirect Surface = "direct"
)

var validSurfaces = map[Surface]struct{}{
	SurfaceThread: {},
	SurfaceDirect: {},
}

// Validate reports whether the surface is one of the documented values.
func (s Surface) Validate() error {
	if _, ok := validSurfaces[s]; !ok {
		return fmt.Errorf("%w: surface=%q", ErrInvalidField, string(s))
	}
	return nil
}

// ReceiptStatus identifies one receipt admission status.
type ReceiptStatus string

const (
	ReceiptStatusAccepted    ReceiptStatus = "accepted"
	ReceiptStatusRejected    ReceiptStatus = "rejected"
	ReceiptStatusDuplicate   ReceiptStatus = "duplicate"
	ReceiptStatusExpired     ReceiptStatus = "expired"
	ReceiptStatusUnsupported ReceiptStatus = "unsupported"
	ReceiptStatusCanceled    ReceiptStatus = "canceled"
)

var validReceiptStatuses = map[ReceiptStatus]struct{}{
	ReceiptStatusAccepted:    {},
	ReceiptStatusRejected:    {},
	ReceiptStatusDuplicate:   {},
	ReceiptStatusExpired:     {},
	ReceiptStatusUnsupported: {},
	ReceiptStatusCanceled:    {},
}

// Validate reports whether the receipt status is documented by the RFC.
func (s ReceiptStatus) Validate() error {
	if _, ok := validReceiptStatuses[s]; !ok {
		return fmt.Errorf("%w: receipt status=%q", ErrInvalidField, string(s))
	}
	return nil
}

// WhoisType identifies the request or response shape for `whois`.
type WhoisType string

const (
	WhoisTypeRequest  WhoisType = "request"
	WhoisTypeResponse WhoisType = "response"
)

var validWhoisTypes = map[WhoisType]struct{}{
	WhoisTypeRequest:  {},
	WhoisTypeResponse: {},
}

// Validate reports whether the whois type is a documented value.
func (t WhoisType) Validate() error {
	if _, ok := validWhoisTypes[t]; !ok {
		return fmt.Errorf("%w: whois type=%q", ErrInvalidField, string(t))
	}
	return nil
}

// ReasonCode identifies one registered protocol rejection reason.
type ReasonCode string

const (
	ReasonCodeMalformed             ReasonCode = "malformed"
	ReasonCodeExpired               ReasonCode = "expired"
	ReasonCodeDuplicate             ReasonCode = "duplicate"
	ReasonCodeUnsupportedKind       ReasonCode = "unsupported_kind"
	ReasonCodeUnsupportedProfile    ReasonCode = "unsupported_profile"
	ReasonCodeVerificationFailed    ReasonCode = "verification_failed"
	ReasonCodeNotTarget             ReasonCode = "not_target"
	ReasonCodeNotFound              ReasonCode = "not_found"
	ReasonCodeBusy                  ReasonCode = "busy"
	ReasonCodeInternal              ReasonCode = "internal"
	ReasonCodeInvalidSurface        ReasonCode = "invalid_surface"
	ReasonCodeConversationNotFound  ReasonCode = "conversation_not_found"
	ReasonCodeWorkClosed            ReasonCode = "work_closed"
	ReasonCodeWorkContainerMismatch ReasonCode = "work_container_mismatch"
	ReasonCodeLegacyFieldRejected   ReasonCode = "legacy_field_rejected"
)

var validReasonCodes = map[ReasonCode]struct{}{
	ReasonCodeMalformed:             {},
	ReasonCodeExpired:               {},
	ReasonCodeDuplicate:             {},
	ReasonCodeUnsupportedKind:       {},
	ReasonCodeUnsupportedProfile:    {},
	ReasonCodeVerificationFailed:    {},
	ReasonCodeNotTarget:             {},
	ReasonCodeNotFound:              {},
	ReasonCodeBusy:                  {},
	ReasonCodeInternal:              {},
	ReasonCodeInvalidSurface:        {},
	ReasonCodeConversationNotFound:  {},
	ReasonCodeWorkClosed:            {},
	ReasonCodeWorkContainerMismatch: {},
	ReasonCodeLegacyFieldRejected:   {},
}

// Validate reports whether the reason code belongs to the v0 registry.
func (r ReasonCode) Validate() error {
	if _, ok := validReasonCodes[r]; !ok {
		return fmt.Errorf("%w: reason_code=%q", ErrInvalidField, string(r))
	}
	return nil
}

// WorkState identifies one RFC work lifecycle state.
type WorkState string

const (
	WorkStateSubmitted  WorkState = "submitted"
	WorkStateWorking    WorkState = "working"
	WorkStateNeedsInput WorkState = "needs_input"
	WorkStateCompleted  WorkState = "completed"
	WorkStateFailed     WorkState = "failed"
	WorkStateCanceled   WorkState = "canceled"
)

var validWorkStates = map[WorkState]struct{}{
	WorkStateSubmitted:  {},
	WorkStateWorking:    {},
	WorkStateNeedsInput: {},
	WorkStateCompleted:  {},
	WorkStateFailed:     {},
	WorkStateCanceled:   {},
}

// Validate reports whether the work state is documented by the RFC.
func (s WorkState) Validate() error {
	if _, ok := validWorkStates[s]; !ok {
		return fmt.Errorf("%w: work state=%q", ErrInvalidField, string(s))
	}
	return nil
}

// Proof preserves the opaque protocol proof payload for forward compatibility.
type Proof map[string]json.RawMessage

// ExtensionMap preserves opaque extension payloads without interpreting them.
type ExtensionMap map[string]json.RawMessage

// Envelope is the shared AGH Network v0 wire envelope.
type Envelope struct {
	Protocol    string          `json:"protocol"`
	ID          string          `json:"id"`
	WorkspaceID string          `json:"workspace_id"`
	Kind        Kind            `json:"kind"`
	Channel     string          `json:"channel"`
	Surface     *Surface        `json:"surface,omitempty"`
	ThreadID    *string         `json:"thread_id,omitempty"`
	DirectID    *string         `json:"direct_id,omitempty"`
	From        string          `json:"from"`
	To          *string         `json:"to,omitempty"`
	Mentions    []string        `json:"mentions,omitempty"`
	WorkID      *string         `json:"work_id,omitempty"`
	ReplyTo     *string         `json:"reply_to,omitempty"`
	TraceID     *string         `json:"trace_id,omitempty"`
	CausationID *string         `json:"causation_id,omitempty"`
	TS          int64           `json:"ts"`
	ExpiresAt   *int64          `json:"expires_at,omitempty"`
	Body        json.RawMessage `json:"body"`
	Proof       *Proof          `json:"proof"`
	Ext         ExtensionMap    `json:"ext,omitempty"`
}

// UnmarshalJSON rejects obsolete hard-cut wire fields before decoding.
func (e *Envelope) UnmarshalJSON(data []byte) error {
	if err := rejectLegacyEnvelopeFields(data); err != nil {
		return err
	}
	type envelopeAlias Envelope
	var decoded envelopeAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*e = Envelope(decoded)
	return nil
}

// IsDirected reports whether the envelope targets a specific peer.
func (e Envelope) IsDirected() bool {
	return e.To != nil
}

// IsBroadcast reports whether the envelope is channel-broadcast.
func (e Envelope) IsBroadcast() bool {
	return e.To == nil
}

// DecodeBody parses and validates the envelope body using the envelope kind.
func (e Envelope) DecodeBody() (Body, error) {
	return DecodeBody(e.Kind, e.Body)
}

// ConversationRef identifies exactly one conversation container.
type ConversationRef struct {
	WorkspaceID string
	Channel     string
	Surface     Surface
	ThreadID    string
	DirectID    string
}

// Validate reports whether the reference identifies exactly one container.
func (r ConversationRef) Validate() error {
	workspaceID := strings.TrimSpace(r.WorkspaceID)
	if workspaceID == "" {
		return fmt.Errorf("%w: workspace_id is required", ErrMissingField)
	}
	if err := ValidateWorkspaceID(workspaceID); err != nil {
		return fmt.Errorf("validate conversation workspace_id: %w", err)
	}
	channel := strings.TrimSpace(r.Channel)
	if channel == "" {
		return fmt.Errorf("%w: channel is required", ErrMissingField)
	}
	if err := ValidateChannel(channel); err != nil {
		return fmt.Errorf("validate conversation channel: %w", err)
	}
	surface := Surface(strings.TrimSpace(string(r.Surface)))
	if err := surface.Validate(); err != nil {
		return err
	}
	threadID := strings.TrimSpace(r.ThreadID)
	directID := strings.TrimSpace(r.DirectID)
	switch surface {
	case SurfaceThread:
		if err := ValidateConversationID(threadID, "thread_id"); err != nil {
			return err
		}
		if directID != "" {
			return fmt.Errorf("%w: direct_id must be absent for surface=%q", ErrInvalidField, surface)
		}
	case SurfaceDirect:
		if err := ValidateConversationID(directID, "direct_id"); err != nil {
			return err
		}
		if threadID != "" {
			return fmt.Errorf("%w: thread_id must be absent for surface=%q", ErrInvalidField, surface)
		}
	}
	return nil
}

// ContainerKey returns a stable workspace/channel/surface/container key.
func (r ConversationRef) ContainerKey() string {
	workspaceID := strings.TrimSpace(r.WorkspaceID)
	channel := strings.TrimSpace(r.Channel)
	surface := Surface(strings.TrimSpace(string(r.Surface)))
	switch surface {
	case SurfaceThread:
		return workspaceID + "\x00" + channel + "\x00" + string(surface) + "\x00" + strings.TrimSpace(r.ThreadID)
	case SurfaceDirect:
		return workspaceID + "\x00" + channel + "\x00" + string(surface) + "\x00" + strings.TrimSpace(r.DirectID)
	default:
		return workspaceID + "\x00" + channel + "\x00" + string(surface)
	}
}

// IsThread reports whether the reference targets a public thread.
func (r ConversationRef) IsThread() bool {
	return Surface(strings.TrimSpace(string(r.Surface))) == SurfaceThread
}

// IsDirect reports whether the reference targets a direct room.
func (r ConversationRef) IsDirect() bool {
	return Surface(strings.TrimSpace(string(r.Surface))) == SurfaceDirect
}

// Body is the typed representation of one envelope body.
type Body interface {
	Kind() Kind
}

var (
	_ Body = GreetBody{}
	_ Body = WhoisBody{}
	_ Body = SayBody{}
	_ Body = CapabilityBody{}
	_ Body = ReceiptBody{}
	_ Body = TraceBody{}
)

// GreetBody advertises peer presence and capabilities in a channel.
type GreetBody struct {
	PeerCard PeerCard `json:"peer_card"`
	Summary  string   `json:"summary,omitempty"`
}

// Kind returns the wire kind for the body.
func (GreetBody) Kind() Kind { return KindGreet }

// PeerCard advertises one peer's identity and capabilities.
type PeerCard struct {
	PeerID              string       `json:"peer_id"`
	DisplayName         *string      `json:"display_name,omitempty"`
	ProfilesSupported   []string     `json:"profiles_supported"`
	Capabilities        []string     `json:"capabilities"`
	ArtifactsSupported  []string     `json:"artifacts_supported"`
	TrustModesSupported []string     `json:"trust_modes_supported"`
	Ext                 ExtensionMap `json:"ext,omitempty"`
}

// WhoisBody requests or returns peer card information.
type WhoisBody struct {
	Type     WhoisType `json:"type"`
	Query    string    `json:"query,omitempty"`
	PeerCard *PeerCard `json:"peer_card,omitempty"`
}

// Kind returns the wire kind for the body.
func (WhoisBody) Kind() Kind { return KindWhois }

// SayBody carries broadcast chat-first communication.
type SayBody struct {
	Text      string            `json:"text"`
	Artifacts []json.RawMessage `json:"artifacts,omitempty"`
	Intent    string            `json:"intent,omitempty"`
}

// Kind returns the wire kind for the body.
func (SayBody) Kind() Kind { return KindSay }

// CapabilityBody carries or advertises one transferable capability artifact.
type CapabilityBody struct {
	Capability CapabilityEnvelopePayload `json:"capability"`
}

// Kind returns the wire kind for the body.
func (CapabilityBody) Kind() Kind { return KindCapability }

// CapabilityEnvelopePayload is the transferable unified capability document.
type CapabilityEnvelopePayload struct {
	ID                string   `json:"id"`
	Summary           string   `json:"summary"`
	Outcome           string   `json:"outcome"`
	Version           string   `json:"version,omitempty"`
	Digest            string   `json:"digest"`
	ContextNeeded     []string `json:"context_needed,omitempty"`
	ArtifactsExpected []string `json:"artifacts_expected,omitempty"`
	ExecutionOutline  []string `json:"execution_outline,omitempty"`
	Constraints       []string `json:"constraints,omitempty"`
	Examples          []string `json:"examples,omitempty"`
	Requirements      []string `json:"requirements,omitempty"`
}

// ReceiptBody acknowledges or rejects protocol-level admission.
type ReceiptBody struct {
	ForID      string        `json:"for_id"`
	Status     ReceiptStatus `json:"status"`
	ReasonCode *ReasonCode   `json:"reason_code,omitempty"`
	Detail     *string       `json:"detail,omitempty"`
}

// Kind returns the wire kind for the body.
func (ReceiptBody) Kind() Kind { return KindReceipt }

// TraceBody reports progress or terminal outcome for work.
type TraceBody struct {
	State        WorkState         `json:"state"`
	Message      string            `json:"message,omitempty"`
	Result       json.RawMessage   `json:"result,omitempty"`
	ArtifactRefs []json.RawMessage `json:"artifact_refs,omitempty"`
}

// Kind returns the wire kind for the body.
func (TraceBody) Kind() Kind { return KindTrace }
