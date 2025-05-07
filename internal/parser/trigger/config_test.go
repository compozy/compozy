package trigger

import (
	"testing"

	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/stretchr/testify/assert"
)

func TestTriggerConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      TriggerConfig
		expectError bool
		errMsg      string
	}{
		{
			name: "Valid Webhook Trigger",
			config: TriggerConfig{
				Type: TriggerTypeWebhook,
				Config: &WebhookConfig{
					URL: "/api/webhook",
				},
			},
			expectError: false,
		},
		{
			name: "Missing Webhook Config",
			config: TriggerConfig{
				Type: TriggerTypeWebhook,
			},
			expectError: true,
			errMsg:      "Webhook configuration is required for webhook trigger type",
		},
		{
			name: "Invalid Trigger Type",
			config: TriggerConfig{
				Type: TriggerTypeWebhook,
				Config: &WebhookConfig{
					URL: "/api/webhook",
				},
			},
			expectError: false,
		},
		{
			name: "Valid Webhook Trigger with Input Schema",
			config: TriggerConfig{
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
			},
			expectError: false,
		},
		{
			name: "Invalid Input Schema Type",
			config: TriggerConfig{
				Type: TriggerTypeWebhook,
				Config: &WebhookConfig{
					URL: "/api/webhook",
				},
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "array",
					},
				},
			},
			expectError: true,
			errMsg:      "Schema type must be object",
		},
		{
			name: "Missing Schema Properties",
			config: TriggerConfig{
				Type: TriggerTypeWebhook,
				Config: &WebhookConfig{
					URL: "/api/webhook",
				},
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "object",
					},
				},
			},
			expectError: true,
			errMsg:      "Schema must have properties",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
