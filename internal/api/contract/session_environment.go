package contract

// SessionEnvBindingPayload contains an authored reference, never its resolved value.
type SessionEnvBindingPayload struct {
	Ref      string `json:"ref"`
	Required bool   `json:"required"`
}
