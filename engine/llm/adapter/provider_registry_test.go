package llmadapter

import (
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry(t *testing.T) {
	t.Run("ShouldRejectDuplicateRegistrations", func(t *testing.T) {
		registry := NewProviderRegistry()
		provider := &stubProvider{
			name:         core.ProviderName("duplicate"),
			client:       &stubClient{},
			capabilities: ProviderCapabilities{},
		}
		require.NoError(t, registry.Register(provider))
		err := registry.Register(provider)
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrProviderAlreadyRegistered)
		assert.Contains(t, err.Error(), "duplicate")
	})
	t.Run("ShouldRegisterAndResolveProviders", func(t *testing.T) {
		registry := NewProviderRegistry()
		provider := &stubProvider{name: core.ProviderName("test-provider"), client: &stubClient{}}
		require.NoError(t, registry.Register(provider))
		resolved, err := registry.Resolve(core.ProviderName("test-provider"))
		require.NoError(t, err)
		assert.Equal(t, provider, resolved)
		_, err = registry.Resolve(core.ProviderName("unknown"))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})
	t.Run("ShouldValidateRegistrations", func(t *testing.T) {
		registry := NewProviderRegistry()
		err := registry.Register(nil)
		assert.ErrorIs(t, err, ErrProviderNil)
		err = registry.Register(&stubProvider{name: ""})
		assert.ErrorIs(t, err, ErrProviderNameEmpty)
	})
	t.Run("ShouldTreatProviderNamesCaseInsensitive", func(t *testing.T) {
		registry := NewProviderRegistry()
		require.NoError(t, registry.Register(&stubProvider{
			name:         core.ProviderName("TestCase"),
			client:       &stubClient{},
			capabilities: ProviderCapabilities{},
		}))
		err := registry.Register(&stubProvider{
			name:         core.ProviderName("TESTCASE"),
			client:       &stubClient{},
			capabilities: ProviderCapabilities{},
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, ErrProviderAlreadyRegistered)
	})
	t.Run("ShouldCreateClientsForRegisteredProviders", func(t *testing.T) {
		registry := NewProviderRegistry()
		client := &stubClient{}
		require.NoError(t, registry.Register(&stubProvider{name: core.ProviderName("cap"), client: client}))
		created, err := registry.NewClient(
			t.Context(),
			&core.ProviderConfig{Provider: core.ProviderName("cap")},
		)
		require.NoError(t, err)
		assert.Equal(t, client, created)
		_, err = registry.NewClient(t.Context(), &core.ProviderConfig{Provider: core.ProviderName("missing")})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not registered")
	})
	t.Run("ShouldResolveProvidersCaseInsensitive", func(t *testing.T) {
		registry := NewProviderRegistry()
		provider := &stubProvider{name: core.ProviderName("MixedCase"), client: &stubClient{}}
		require.NoError(t, registry.Register(provider))

		resolved, err := registry.Resolve(core.ProviderName("mixedcase"))
		require.NoError(t, err)
		assert.Equal(t, provider, resolved)

		resolved, err = registry.Resolve(core.ProviderName("MIXEDCASE"))
		require.NoError(t, err)
		assert.Equal(t, provider, resolved)
	})
}

func TestCloneProviderConfig(t *testing.T) {
	t.Run("ShouldDeepCopyMutableFields", func(t *testing.T) {
		cfg := &core.ProviderConfig{
			Provider: core.ProviderOpenAI,
			Model:    "gpt",
			Params: core.PromptParams{
				StopWords: []string{"STOP", "END"},
			},
		}
		clone := cloneProviderConfig(cfg)
		require.NotNil(t, clone)
		require.NotSame(t, cfg, clone)
		assert.Equal(t, cfg.Provider, clone.Provider)
		assert.Equal(t, cfg.Model, clone.Model)
		assert.Equal(t, cfg.Params.StopWords, clone.Params.StopWords)
		clone.Params.StopWords[0] = "MUTATED"
		assert.Equal(t, []string{"STOP", "END"}, cfg.Params.StopWords)
	})
}
