package trigger

import (
	"fmt"
)

// TypeValidator validates the trigger type and its configuration
type TypeValidator struct {
	triggerType Type
	config      *WebhookConfig
}

func NewTriggerTypeValidator(triggerType Type, config *WebhookConfig) *TypeValidator {
	return &TypeValidator{
		triggerType: triggerType,
		config:      config,
	}
}

func (v *TypeValidator) Validate() error {
	switch v.triggerType {
	case TriggerTypeWebhook:
		if v.config == nil {
			return fmt.Errorf("webhook configuration is required for webhook trigger type")
		}
	default:
		return fmt.Errorf("invalid trigger type: %s", string(v.triggerType))
	}
	return nil
}
