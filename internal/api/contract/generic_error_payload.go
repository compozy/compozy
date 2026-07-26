package contract

// ErrorPayload is the shared error response payload.
type ErrorPayload struct {
	Error      string          `json:"error"`
	Diagnostic *DiagnosticItem `json:"diagnostic,omitempty"`
}
