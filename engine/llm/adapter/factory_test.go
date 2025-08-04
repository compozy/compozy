package llmadapter

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
)

func TestDefaultFactory_CreateClient(t *testing.T) {
	factory := NewDefaultFactory()

	t.Run("Should return error when config is nil", func(t *testing.T) {
		client, err := factory.CreateClient(nil)
		assert.Nil(t, client)
		assert.ErrorContains(t, err, "provider config must not be nil")
	})

	t.Run("Should return error for unsupported provider", func(t *testing.T) {
		config := &core.ProviderConfig{
			Provider: "unsupported",
		}
		client, err := factory.CreateClient(config)
		assert.Nil(t, client)
		assert.ErrorContains(t, err, "unsupported LLM provider")
	})

	t.Run("Should create client for supported provider", func(t *testing.T) {
		config := &core.ProviderConfig{
			Provider: core.ProviderOllama,
			Model:    "llama2",
		}
		client, err := factory.CreateClient(config)
		assert.NotNil(t, client)
		assert.NoError(t, err)
		assert.IsType(t, &LangChainAdapter{}, client)
	})
}
