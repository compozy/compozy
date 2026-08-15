package contract

import (
	"strings"

	"github.com/compozy/compozy/internal/acp"
)

// ACPPromptStopReason is the closed turn-completion vocabulary exposed by ACP sessions.
type ACPPromptStopReason string

const (
	ACPStopReasonEndTurn         ACPPromptStopReason = "end_turn"
	ACPStopReasonMaxTokens       ACPPromptStopReason = "max_tokens"
	ACPStopReasonMaxTurnRequests ACPPromptStopReason = "max_turn_requests"
	ACPStopReasonRefusal         ACPPromptStopReason = "refusal"
	// ACP spells the cancellation wire value with a doubled l.
	ACPStopReasonCancelled ACPPromptStopReason = "cancel" + "led"
)

// ACPPromptStopReasonValues returns the closed ACP prompt stop vocabulary.
func ACPPromptStopReasonValues() []string {
	return []string{
		string(ACPStopReasonEndTurn), string(ACPStopReasonMaxTokens),
		string(ACPStopReasonMaxTurnRequests), string(ACPStopReasonRefusal),
		string(ACPStopReasonCancelled),
	}
}

// ACPAvailableCommandPayload is one command in the session's current replacement set.
type ACPAvailableCommandPayload struct {
	Name        string                           `json:"name"`
	Description string                           `json:"description"`
	Input       *ACPAvailableCommandInputPayload `json:"input,omitempty"`
}

// ACPAvailableCommandInputPayload describes the optional unstructured command argument.
type ACPAvailableCommandInputPayload struct {
	Hint string `json:"hint"`
}

// ACPCapsPayload is the JSON representation of ACP capabilities.
type ACPCapsPayload struct {
	SupportsLoadSession   bool                         `json:"supports_load_session"`
	PromptImage           bool                         `json:"prompt_image"`
	PromptAudio           bool                         `json:"prompt_audio"`
	PromptEmbeddedContext bool                         `json:"prompt_embedded_context"`
	SupportedModes        []string                     `json:"supported_modes,omitempty"`
	ConfigOptions         []SessionConfigOptionPayload `json:"config_options,omitempty"`
}

// ACPCapsPayloadFromACP projects negotiated ACP capabilities into the wire contract.
func ACPCapsPayloadFromACP(caps acp.Caps, known bool) *ACPCapsPayload {
	if !known {
		return nil
	}

	return &ACPCapsPayload{
		SupportsLoadSession:   caps.SupportsLoadSession,
		PromptImage:           caps.PromptImage,
		PromptAudio:           caps.PromptAudio,
		PromptEmbeddedContext: caps.PromptEmbeddedContext,
		SupportedModes:        append([]string(nil), caps.SupportedModes...),
		ConfigOptions:         sessionConfigOptionPayloadsFromACP(caps.ConfigOptions),
	}
}

func sessionConfigOptionPayloadsFromACP(options []acp.SessionConfigOption) []SessionConfigOptionPayload {
	if len(options) == 0 {
		return nil
	}
	payloads := make([]SessionConfigOptionPayload, 0, len(options))
	for _, option := range options {
		payloads = append(payloads, SessionConfigOptionPayload{
			ID:          strings.TrimSpace(option.ID),
			Label:       strings.TrimSpace(option.Label),
			Description: strings.TrimSpace(option.Description),
			Kind:        string(option.Kind),
			Current:     strings.TrimSpace(option.Current),
			Values:      sessionConfigOptionValuePayloadsFromACP(option.Values),
		})
	}
	return payloads
}

func sessionConfigOptionValuePayloadsFromACP(
	values []acp.SessionConfigOptionValue,
) []SessionConfigOptionValuePayload {
	if len(values) == 0 {
		return nil
	}
	payloads := make([]SessionConfigOptionValuePayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, SessionConfigOptionValuePayload{
			Value:       strings.TrimSpace(value.Value),
			Label:       strings.TrimSpace(value.Label),
			Description: strings.TrimSpace(value.Description),
		})
	}
	return payloads
}

// SessionConfigOptionPayload is one active ACP session config option.
type SessionConfigOptionPayload struct {
	ID          string                            `json:"id"`
	Label       string                            `json:"label,omitempty"`
	Description string                            `json:"description,omitempty"`
	Kind        string                            `json:"kind"`
	Current     string                            `json:"current,omitempty"`
	Values      []SessionConfigOptionValuePayload `json:"values,omitempty"`
}

// SessionConfigOptionValuePayload is one selectable value for an active ACP config option.
type SessionConfigOptionValuePayload struct {
	Value       string `json:"value"`
	Label       string `json:"label,omitempty"`
	Description string `json:"description,omitempty"`
}
