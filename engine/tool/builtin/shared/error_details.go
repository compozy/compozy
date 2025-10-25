package shared

// ErrorDetails captures the structured error payload shared by builtin tools.
// Keeping this in a shared package ensures the JSON contract stays consistent across tools.
type ErrorDetails struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}
