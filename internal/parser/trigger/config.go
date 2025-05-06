package trigger

import (
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/transition"
)

// TriggerType represents the type of trigger
type TriggerType string

const (
	// TriggerTypeWebhook represents a webhook trigger
	TriggerTypeWebhook TriggerType = "webhook"
)

// WebhookConfig represents a webhook trigger configuration
type WebhookConfig struct {
	URL WebhookURL `json:"url" yaml:"url"`
}

// TriggerConfig represents a trigger configuration
type TriggerConfig struct {
	Type        TriggerType                       `json:"type" yaml:"type"`
	Webhook     *WebhookConfig                    `json:"webhook,omitempty" yaml:"webhook,omitempty"`
	OnError     *transition.ErrorTransitionConfig `json:"on_error,omitempty" yaml:"on_error,omitempty"`
	InputSchema *common.InputSchema               `json:"input,omitempty" yaml:"input,omitempty"`
}

// Validate validates the trigger configuration
func (t *TriggerConfig) Validate() error {
	validator := common.NewCompositeValidator(
		NewTriggerTypeValidator(t.Type, t.Webhook),
	)
	return validator.Validate()
}
