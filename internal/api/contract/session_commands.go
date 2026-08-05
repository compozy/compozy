package contract

// SessionCommandSource identifies one command owner without exposing filesystem paths.
type SessionCommandSource struct {
	Kind  string `json:"kind"`
	ID    string `json:"id,omitempty"`
	Key   string `json:"key,omitempty"`
	Scope string `json:"scope"`
}

// SessionCommandPayload is one command available in a session composer.
type SessionCommandPayload struct {
	ID             string               `json:"id"`
	CanonicalToken string               `json:"canonical_token"`
	DisplayName    string               `json:"display_name"`
	Description    string               `json:"description"`
	Lane           string               `json:"lane"`
	Source         SessionCommandSource `json:"source"`
	Placements     []string             `json:"placements"`
	InputHint      string               `json:"input_hint,omitempty"`
}

// SessionCommandsResponse is the session-scoped command catalog projection.
type SessionCommandsResponse struct {
	Commands []SessionCommandPayload `json:"commands"`
	Revision string                  `json:"revision"`
}

// SessionSkillInvocationPayload identifies one admitted inline skill marker.
type SessionSkillInvocationPayload struct {
	CommandID string               `json:"command_id"`
	Token     string               `json:"token"`
	Name      string               `json:"name"`
	Source    SessionCommandSource `json:"source"`
	Start     int                  `json:"start"`
	End       int                  `json:"end"`
}
