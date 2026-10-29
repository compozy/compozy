package contract

// ErrorPayload is the shared error response payload.
type ErrorPayload struct {
	Blockers      []WorkspaceArchiveBlocker `json:"blockers,omitempty"`
	Error         string                    `json:"error"`
	Code          string                    `json:"code,omitempty"`
	CurrentTurnID string                    `json:"current_turn_id,omitempty"`
	Details       map[string]string         `json:"details,omitempty"`
	Diagnostic    *DiagnosticItem           `json:"diagnostic,omitzero"`
	ConfigPath    string                    `json:"config_path,omitempty"`
	UnknownKeys   []string                  `json:"unknown_keys,omitempty"`
}
