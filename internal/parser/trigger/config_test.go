package trigger

import (
	"testing"

	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/stretchr/testify/assert"
)

func TestTriggerConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *TriggerConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid Webhook Trigger",
			config: &TriggerConfig{
				Type: TriggerTypeWebhook,
				Webhook: &WebhookConfig{
					URL: "/api/webhook",
				},
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "object",
						"properties": map[string]any{
							"payload": map[string]any{
								"type": "object",
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid Trigger Type",
			config: &TriggerConfig{
				Type: "invalid",
			},
			wantErr: true,
			errMsg:  "Invalid trigger type",
		},
		{
			name: "Missing Webhook Config",
			config: &TriggerConfig{
				Type: TriggerTypeWebhook,
			},
			wantErr: true,
			errMsg:  "Webhook configuration is required for webhook trigger type",
		},
		{
			name: "Invalid Input Schema Type",
			config: &TriggerConfig{
				Type: TriggerTypeWebhook,
				Webhook: &WebhookConfig{
					URL: "/api/webhook",
				},
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "array",
					},
				},
			},
			wantErr: true,
			errMsg:  "Schema type must be object",
		},
		{
			name: "Missing Schema Properties",
			config: &TriggerConfig{
				Type: TriggerTypeWebhook,
				Webhook: &WebhookConfig{
					URL: "/api/webhook",
				},
				InputSchema: &schema.InputSchema{
					Schema: schema.Schema{
						"type": "object",
					},
				},
			},
			wantErr: true,
			errMsg:  "Schema must have properties",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
