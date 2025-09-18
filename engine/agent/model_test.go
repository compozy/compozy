package agent

import (
	"encoding/json"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestModel_YAML_ScalarRef(t *testing.T) {
	var a struct {
		Model Model `yaml:"model"`
	}
	data := []byte("model: fast")
	require.NoError(t, yaml.Unmarshal(data, &a))
	assert.True(t, a.Model.HasRef())
	assert.Equal(t, "fast", a.Model.Ref)
	assert.False(t, a.Model.HasConfig())
}

func TestModel_YAML_InlineObject(t *testing.T) {
	var a struct {
		Model Model `yaml:"model"`
	}
	data := []byte(
		"model:\n  provider: openai\n  model: gpt-4o-mini\n  params:\n    temperature: 0.1\n    max_tokens: 64\n",
	)
	require.NoError(t, yaml.Unmarshal(data, &a))
	assert.False(t, a.Model.HasRef())
	assert.True(t, a.Model.HasConfig())
	assert.Equal(t, core.ProviderOpenAI, a.Model.Config.Provider)
	assert.Equal(t, "gpt-4o-mini", a.Model.Config.Model)
	assert.InDelta(t, 0.1, a.Model.Config.Params.Temperature, 1e-6)
	assert.EqualValues(t, 64, a.Model.Config.Params.MaxTokens)
}

func TestModel_JSON_ScalarRef(t *testing.T) {
	var m Model
	require.NoError(t, json.Unmarshal([]byte("\"fast\""), &m))
	assert.True(t, m.HasRef())
	assert.Equal(t, "fast", m.Ref)
	assert.False(t, m.HasConfig())
}

func TestModel_JSON_InlineObject(t *testing.T) {
	var m Model
	require.NoError(t, json.Unmarshal([]byte(`{"provider":"anthropic","model":"claude-3-haiku"}`), &m))
	assert.False(t, m.HasRef())
	assert.True(t, m.HasConfig())
	assert.Equal(t, core.ProviderAnthropic, m.Config.Provider)
	assert.Equal(t, "claude-3-haiku", m.Config.Model)
}

func TestAgentConfig_FromMap_ModelDecode(t *testing.T) {
	// Scalar model ref
	var cfg Config
	require.NoError(t, cfg.FromMap(map[string]any{
		"id":           "a1",
		"model":        "fast",
		"instructions": "x",
	}))
	assert.Equal(t, "a1", cfg.ID)
	assert.True(t, cfg.Model.HasRef())
	assert.Equal(t, "fast", cfg.Model.Ref)

	// Inline provider config
	var cfg2 Config
	require.NoError(t, cfg2.FromMap(map[string]any{
		"id":           "a2",
		"model":        map[string]any{"provider": "openai", "model": "gpt-4o"},
		"instructions": "y",
	}))
	assert.Equal(t, core.ProviderOpenAI, cfg2.Model.Config.Provider)
	assert.Equal(t, "gpt-4o", cfg2.Model.Config.Model)
}
