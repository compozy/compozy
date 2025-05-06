package trigger

import (
	"github.com/compozy/compozy/internal/parser/common"
	"github.com/compozy/compozy/internal/parser/schema"
	"github.com/compozy/compozy/internal/parser/transition"
	"gopkg.in/yaml.v3"
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
	InputSchema *schema.InputSchema               `json:"input,omitempty" yaml:"input,omitempty"`
}

// Load loads a trigger configuration from a YAML file
func Load(path string) (*TriggerConfig, error) {
	data, err := common.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config TriggerConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// Validate validates the trigger configuration
func (t *TriggerConfig) Validate() error {
	validator := common.NewCompositeValidator(
		NewTriggerTypeValidator(t.Type, t.Webhook),
		schema.NewSchemaValidator(nil, t.InputSchema, nil),
	)
	return validator.Validate()
}
