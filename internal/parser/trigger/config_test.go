package trigger

import (
	"testing"

	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/stretchr/testify/assert"
)

func Test_TriggerConfigValidation(t *testing.T) {
	t.Run("Should validate valid webhook trigger", func(t *testing.T) {
		config := TriggerConfig{
			Type: TriggerTypeWebhook,
			Config: &WebhookConfig{
				URL: "/api/webhook",
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when webhook config is missing", func(t *testing.T) {
		config := TriggerConfig{
			Type: TriggerTypeWebhook,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "webhook configuration is required for webhook trigger type")
	})

	t.Run("Should validate valid webhook trigger with input schema", func(t *testing.T) {
		config := TriggerConfig{
			Type: TriggerTypeWebhook,
			Config: &WebhookConfig{
				URL: "/api/webhook",
			},
			InputSchema: &schema.InputSchema{
				Schema: schema.Schema{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type": "string",
						},
					},
				},
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error for invalid input schema type", func(t *testing.T) {
		config := TriggerConfig{
			Type: TriggerTypeWebhook,
			Config: &WebhookConfig{
				URL: "/api/webhook",
			},
			InputSchema: &schema.InputSchema{
				Schema: schema.Schema{
					"type": "array",
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Schema type must be object")
	})

	t.Run("Should return error when schema properties are missing", func(t *testing.T) {
		config := TriggerConfig{
			Type: TriggerTypeWebhook,
			Config: &WebhookConfig{
				URL: "/api/webhook",
			},
			InputSchema: &schema.InputSchema{
				Schema: schema.Schema{
					"type": "object",
				},
			},
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Schema must have properties")
	})
}
