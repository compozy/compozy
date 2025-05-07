package trigger

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
			return NewMissingWebhookError()
		}
	default:
		return NewInvalidTypeError(string(v.triggerType))
	}
	return nil
}
