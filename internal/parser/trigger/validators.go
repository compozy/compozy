package trigger

// TriggerTypeValidator validates the trigger type and its configuration
type TriggerTypeValidator struct {
	triggerType TriggerType
	webhook     *WebhookConfig
}

func NewTriggerTypeValidator(triggerType TriggerType, webhook *WebhookConfig) *TriggerTypeValidator {
	return &TriggerTypeValidator{
		triggerType: triggerType,
		webhook:     webhook,
	}
}

func (v *TriggerTypeValidator) Validate() error {
	switch v.triggerType {
	case TriggerTypeWebhook:
		if v.webhook == nil {
			return NewMissingWebhookError()
		}
	default:
		return NewInvalidTypeError(string(v.triggerType))
	}
	return nil
}
