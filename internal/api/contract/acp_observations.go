package contract

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
	SupportsLoadSession bool                         `json:"supports_load_session"`
	SupportedModes      []string                     `json:"supported_modes,omitempty"`
	ConfigOptions       []SessionConfigOptionPayload `json:"config_options,omitempty"`
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
