package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ProviderConfig_Conversions(t *testing.T) {
	t.Run("Should construct and marshal/unmarshal via helpers", func(t *testing.T) {
		pc := NewProviderConfig(ProviderOpenAI, "gpt-4-turbo", "{{ .env.OPENAI_API_KEY }}")
		b, err := pc.AsJSON()
		require.NoError(t, err)
		var tmp map[string]any
		require.NoError(t, json.Unmarshal(b, &tmp))
		m, err := pc.AsMap()
		require.NoError(t, err)
		assert.Equal(t, tmp["model"], m["model"])
	})
	t.Run("Should merge from map with override (empty values overwrite)", func(t *testing.T) {
		pc := &ProviderConfig{Provider: ProviderMock, Model: "m1", APIKey: "k1", Params: PromptParams{MaxTokens: 10}}
		err := pc.FromMap(map[string]any{"model": "m2", "params": map[string]any{"max_tokens": 20}})
		require.NoError(t, err)
		assert.Equal(t, "m2", pc.Model)
		assert.Equal(t, int32(20), pc.Params.MaxTokens)
		// WithOverwriteWithEmptyValue means unspecified fields may become zero values
		assert.Equal(t, ProviderName(""), pc.Provider)
		assert.Equal(t, "", pc.APIKey)
	})
}
