package trigger

import (
	"fmt"
)

// TriggerTypeValidator validates the trigger type and its configuration
type TriggerTypeValidator struct {
	triggerType TriggerType
	config      *WebhookConfig
}

func NewTriggerTypeValidator(triggerType TriggerType, config *WebhookConfig) *TriggerTypeValidator {
	return &TriggerTypeValidator{
		triggerType: triggerType,
		config:      config,
	}
}

func (v *TriggerTypeValidator) Validate() error {
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
