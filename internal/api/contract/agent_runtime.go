package contract

// AgentACPOptionSelection captures one typed ACP option selection in a public
// runtime contract. Exactly one of ValueID or BoolValue must be set.
type AgentACPOptionSelection struct {
	ID        string `json:"id"`
	ValueID   string `json:"value_id,omitempty"`
	BoolValue *bool  `json:"bool_value,omitempty"`
}
