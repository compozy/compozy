package trigger

import (
	"github.com/compozy/compozy/internal/parser/common"
)

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

// SchemaValidator validates input schema
type SchemaValidator struct {
	schema *common.InputSchema
}

func NewSchemaValidator(schema *common.InputSchema) *SchemaValidator {
	return &SchemaValidator{schema: schema}
}

func (v *SchemaValidator) Validate() error {
	if v.schema == nil {
		return nil
	}
	if err := v.schema.Validate(); err != nil {
		return NewInvalidInputSchemaError(err)
	}
	return nil
}
