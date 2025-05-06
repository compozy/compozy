package transition

import "github.com/compozy/compozy/internal/parser/common"

// SuccessTransitionConfig represents a success transition configuration
type SuccessTransitionConfig struct {
	Next *string            `json:"next,omitempty" yaml:"next,omitempty"`
	With *common.WithParams `json:"with,omitempty" yaml:"with,omitempty"`
}

// ErrorTransitionConfig represents an error transition configuration
type ErrorTransitionConfig struct {
	Next *string            `json:"next,omitempty" yaml:"next,omitempty"`
	With *common.WithParams `json:"with,omitempty" yaml:"with,omitempty"`
}
