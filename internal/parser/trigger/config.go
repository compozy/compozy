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
	switch t.Type {
	case TriggerTypeWebhook:
		if t.Webhook == nil {
			return &TriggerError{
				Message: "Webhook configuration is required for webhook trigger type",
				Code:    "INVALID_TRIGGER_TYPE",
			}
		}
	default:
		return &TriggerError{
			Message: "Invalid trigger type: " + string(t.Type),
			Code:    "INVALID_TRIGGER_TYPE",
		}
	}

	if t.InputSchema != nil {
		if err := t.InputSchema.Validate(); err != nil {
			return &TriggerError{
				Message: "Invalid input schema: " + err.Error(),
				Code:    "INVALID_INPUT_SCHEMA",
			}
		}
	}

	return nil
}

// TriggerError represents errors that can occur during trigger configuration
type TriggerError struct {
	Message string
	Code    string
}

func (e *TriggerError) Error() string {
	return e.Message
}
