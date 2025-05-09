package common

import "maps"

// WithParams represents parameters for a component
type WithParams map[string]any

type Input map[string]any
type Output map[string]any
type TriggerInput map[string]any

func (t *TriggerInput) ToInput() Input {
	input := make(Input)
	maps.Copy(input, *t)
	return input
}

type ErrorResponse struct {
	Msg string `json:"name"`
}
