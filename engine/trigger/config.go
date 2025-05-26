package trigger

import (
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
)

// Type represents the type of trigger
type Type string

const (
	// TriggerTypeWebhook represents a webhook trigger
	TriggerTypeWebhook Type = "webhook"
)

// WebhookConfig represents a webhook trigger configuration
type WebhookConfig struct {
	URL string `json:"url" yaml:"url"`
}

// Config represents a trigger configuration
type Config struct {
	Type        Type                        `json:"type"               yaml:"type"`
	Config      *WebhookConfig              `json:"config,omitempty"   yaml:"config,omitempty"`
	OnError     *task.ErrorTransitionConfig `json:"on_error,omitempty" yaml:"on_error,omitempty"`
	InputSchema *schema.InputSchema         `json:"input,omitempty"    yaml:"input,omitempty"`
}

// Validate validates the trigger configuration
func (t *Config) Validate() error {
	validator := schema.NewCompositeValidator(
		NewTriggerTypeValidator(t.Type, t.Config),
	)
	return validator.Validate()
}
