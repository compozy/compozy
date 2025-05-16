package trigger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_TriggerConfigValidation(t *testing.T) {
	t.Run("Should validate valid webhook trigger", func(t *testing.T) {
		config := Config{
			Type: TriggerTypeWebhook,
			Config: &WebhookConfig{
				URL: "/api/webhook",
			},
		}

		err := config.Validate()
		assert.NoError(t, err)
	})

	t.Run("Should return error when webhook config is missing", func(t *testing.T) {
		config := Config{
			Type: TriggerTypeWebhook,
		}

		err := config.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "webhook configuration is required for webhook trigger type")
	})
}
