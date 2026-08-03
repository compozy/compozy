package contract

// ErrorPayload is the shared error response payload.
type ErrorPayload struct {
	Error      string            `json:"error"`
	Code       string            `json:"code,omitempty"`
	Details    map[string]string `json:"details,omitempty"`
	Diagnostic *DiagnosticItem   `json:"diagnostic,omitempty"`
}
