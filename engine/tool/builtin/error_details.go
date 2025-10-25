package builtin

// ErrorDetails captures the structured error payload shared by builtin tools.
// Keeping this in the root package ensures the JSON contract stays consistent.
type ErrorDetails struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}
