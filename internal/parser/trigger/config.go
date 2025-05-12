package trigger

import (
	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/compozy/compozy/internal/parser/transition"
	"github.com/compozy/compozy/internal/parser/validator"
)

// TriggerType represents the type of trigger
type TriggerType string

const (
	// TriggerTypeWebhook represents a webhook trigger
	TriggerTypeWebhook TriggerType = "webhook"
)

// WebhookConfig represents a webhook trigger configuration
type WebhookConfig struct {
	URL string `json:"url" yaml:"url"`
}

// TriggerConfig represents a trigger configuration
type TriggerConfig struct {
	Type        TriggerType                       `json:"type" yaml:"type"`
	Config      *WebhookConfig                    `json:"config,omitempty" yaml:"config,omitempty"`
	OnError     *transition.ErrorTransitionConfig `json:"on_error,omitempty" yaml:"on_error,omitempty"`
	InputSchema *schema.InputSchema               `json:"input,omitempty" yaml:"input,omitempty"`
}

// Validate validates the trigger configuration
func (t *TriggerConfig) Validate() error {
	validator := validator.NewCompositeValidator(
		NewTriggerTypeValidator(t.Type, t.Config),
	)
	return validator.Validate()
}
