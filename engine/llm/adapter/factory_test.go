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
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider config must not be nil")
	})

	t.Run("Should return error for unsupported provider", func(t *testing.T) {
		config := &core.ProviderConfig{
			Provider: "unsupported",
		}
		client, err := factory.CreateClient(config)
		assert.Nil(t, client)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported LLM provider")
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

	t.Run("Should demonstrate factory pattern preventing nil panic", func(t *testing.T) {
		// This test demonstrates that the factory prevents the nil panic
		// that would occur if we tried to dereference config.Provider directly
		factory := NewDefaultFactory()

		// This would panic without the nil check
		client, err := factory.CreateClient(nil)

		// Instead of panicking, we get a proper error
		assert.Nil(t, client)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "provider config must not be nil")
	})
}
