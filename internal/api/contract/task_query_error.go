package contract

// TaskQueryErrorPayload reports one invalid typed task-list query field.
type TaskQueryErrorPayload struct {
	ErrorPayload
	Field string `json:"field"`
}
