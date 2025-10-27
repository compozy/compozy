package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/compozy/compozy/engine/core"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/testutil"
	"github.com/stretchr/testify/require"
)

func TestNewNormalizesProvider(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		expected core.ProviderName
	}{
		{name: "openai", provider: "OpenAI", expected: core.ProviderOpenAI},
		{name: "anthropic", provider: "ANTHROPIC", expected: core.ProviderAnthropic},
		{name: "google", provider: " google ", expected: core.ProviderGoogle},
		{name: "groq", provider: "Groq", expected: core.ProviderGroq},
		{name: "ollama", provider: "ollama", expected: core.ProviderOllama},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			builder := New(tc.provider, "model-id")
			if builder == nil {
				t.Fatalf("expected builder instance")
			}
			if builder.config.Provider != tc.expected {
				t.Fatalf("expected provider %q, got %q", tc.expected, builder.config.Provider)
			}
		})
	}
}

func TestNilBuilderFluentMethods(t *testing.T) {
	var builder *Builder
	require.Nil(t, builder.WithAPIKey("key"))
	require.Nil(t, builder.WithAPIURL("url"))
	require.Nil(t, builder.WithTemperature(0.5))
	require.Nil(t, builder.WithMaxTokens(1))
	require.Nil(t, builder.WithTopP(0.5))
	require.Nil(t, builder.WithFrequencyPenalty(0.1))
	require.Nil(t, builder.WithPresencePenalty(0.1))
	require.Nil(t, builder.WithDefault(true))
}

func TestWithAPIKeyAppliesTrimmedValue(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.WithAPIKey("  secret-key  ")

	cfg, err := builder.Build(testutil.NewTestContext(t))
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if cfg.APIKey != "secret-key" {
		t.Fatalf("expected api key to be trimmed, got %q", cfg.APIKey)
	}
}

func TestWithAPIURLValidatesURL(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.WithAPIURL("not a url")

	_, err := builder.Build(testutil.NewTestContext(t))
	if err == nil {
		t.Fatalf("expected validation error")
	}
	var be *sdkerrors.BuildError
	if !errors.As(err, &be) {
		t.Fatalf("expected build error, got %T", err)
	}
	if len(be.Errors) == 0 {
		t.Fatalf("expected inner errors in build error")
	}
}

func TestWithTemperatureRange(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.WithTemperature(1.5)

	cfg, err := builder.Build(testutil.NewTestContext(t))
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if !cfg.Params.IsSetTemperature() {
		t.Fatalf("expected temperature to be set")
	}
	if cfg.Params.Temperature != 1.5 {
		t.Fatalf("expected stored temperature 1.5, got %v", cfg.Params.Temperature)
	}
}

func TestWithTemperatureInvalid(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.WithTemperature(-0.1)

	_, err := builder.Build(testutil.NewTestContext(t))
	if err == nil {
		t.Fatalf("expected build to fail")
	}
	if !strings.Contains(err.Error(), "temperature") {
		t.Fatalf("expected temperature error, got %v", err)
	}
}

func TestWithMaxTokensValidation(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.WithMaxTokens(4096)

	cfg, err := builder.Build(testutil.NewTestContext(t))
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if !cfg.Params.IsSetMaxTokens() {
		t.Fatalf("expected max tokens to be set")
	}
	if cfg.Params.MaxTokens != 4096 {
		t.Fatalf("expected max tokens 4096, got %d", cfg.Params.MaxTokens)
	}
}

func TestWithMaxTokensInvalid(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.WithMaxTokens(0)

	_, err := builder.Build(testutil.NewTestContext(t))
	if err == nil {
		t.Fatalf("expected validation failure")
	}
	if !strings.Contains(err.Error(), "max tokens") {
		t.Fatalf("expected max tokens error, got %v", err)
	}
}

func TestWithTopPValidation(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.WithTopP(0.8)

	cfg, err := builder.Build(testutil.NewTestContext(t))
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if !cfg.Params.IsSetTopP() || cfg.Params.TopP != 0.8 {
		t.Fatalf("expected top_p to be set to 0.8, got %v", cfg.Params.TopP)
	}
}

func TestWithTopPInvalid(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.WithTopP(1.1)

	_, err := builder.Build(testutil.NewTestContext(t))
	if err == nil {
		t.Fatalf("expected validation failure")
	}
	if !strings.Contains(err.Error(), "top_p") {
		t.Fatalf("expected top_p error, got %v", err)
	}
}

func TestPenaltyRangeValidation(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.WithFrequencyPenalty(0.5)
	builder.WithPresencePenalty(-0.5)

	cfg, err := builder.Build(testutil.NewTestContext(t))
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if cfg.Params.FrequencyPenalty != 0.5 {
		t.Fatalf("expected frequency penalty 0.5, got %v", cfg.Params.FrequencyPenalty)
	}
	if cfg.Params.PresencePenalty != -0.5 {
		t.Fatalf("expected presence penalty -0.5, got %v", cfg.Params.PresencePenalty)
	}
}

func TestPenaltyInvalid(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.WithFrequencyPenalty(3)
	builder.WithPresencePenalty(3)

	_, err := builder.Build(testutil.NewTestContext(t))
	if err == nil {
		t.Fatalf("expected frequency penalty validation failure")
	}
	if !strings.Contains(err.Error(), "frequency penalty") {
		t.Fatalf("expected frequency penalty error, got %v", err)
	}
	if !strings.Contains(err.Error(), "presence penalty") {
		t.Fatalf("expected presence penalty error, got %v", err)
	}
}

func TestWithDefaultSetsFlag(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.WithDefault(true)

	cfg, err := builder.Build(testutil.NewTestContext(t))
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if !cfg.Default {
		t.Fatalf("expected default flag to be true")
	}
}

func TestBuildValidConfiguration(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.WithAPIKey("key")
	builder.WithAPIURL("https://api.openai.com/v1")
	builder.WithTemperature(0.7)
	builder.WithMaxTokens(2048)
	builder.WithTopP(0.9)
	builder.WithFrequencyPenalty(0.2)
	builder.WithPresencePenalty(0.1)
	builder.WithDefault(true)

	cfg, err := builder.Build(testutil.NewTestContext(t))
	if err != nil {
		t.Fatalf("build failed: %v", err)
	}
	if cfg == builder.config {
		t.Fatalf("expected config to be cloned")
	}
	if cfg.Provider != core.ProviderOpenAI {
		t.Fatalf("expected provider openai, got %q", cfg.Provider)
	}
}

func TestBuildRequiresProviderAndModel(t *testing.T) {
	builder := New("", " ")

	_, err := builder.Build(testutil.NewTestContext(t))
	if err == nil {
		t.Fatalf("expected validation error")
	}
	var be *sdkerrors.BuildError
	if !errors.As(err, &be) {
		t.Fatalf("expected build error, got %T", err)
	}
	if len(be.Errors) < 2 {
		t.Fatalf("expected provider and model errors, got %d", len(be.Errors))
	}
}

func TestBuildRejectsUnsupportedProvider(t *testing.T) {
	builder := New("unsupported", "gpt-4")

	_, err := builder.Build(testutil.NewTestContext(t))
	if err == nil {
		t.Fatalf("expected error for unsupported provider")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("expected unsupported provider message, got %v", err)
	}
}

func TestBuildRequiresContext(t *testing.T) {
	builder := New("openai", "gpt-4")

	var missingCtx context.Context
	_, err := builder.Build(missingCtx)
	if err == nil {
		t.Fatalf("expected context error")
	}
	if !strings.Contains(err.Error(), "context is required") {
		t.Fatalf("expected context error message, got %v", err)
	}
}

func TestBuildHandlesNilBuilder(t *testing.T) {
	var builder *Builder
	_, err := builder.Build(testutil.NewTestContext(t))
	if err == nil {
		t.Fatalf("expected nil builder error")
	}
	if !strings.Contains(err.Error(), "model builder is required") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestBuildCloneFailure(t *testing.T) {
	ctx := testutil.NewTestContext(t)
	builder := New("openai", "gpt-4")
	original := cloneProviderConfig
	cloneProviderConfig = func(*core.ProviderConfig) (*core.ProviderConfig, error) {
		return nil, fmt.Errorf("clone failed")
	}
	t.Cleanup(func() {
		cloneProviderConfig = original
	})
	cfg, err := builder.Build(ctx)
	require.Nil(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to clone provider config")
}

func TestValidateParamsDetectsMutations(t *testing.T) {
	builder := New("openai", "gpt-4")
	builder.config.Params.SetMaxTokens(10)
	builder.config.Params.MaxTokens = 0
	builder.config.Params.SetTemperature(0.5)
	builder.config.Params.Temperature = 3
	builder.config.Params.SetTopP(0.5)
	builder.config.Params.TopP = -0.1
	builder.config.Params.SetFrequencyPenalty(0.5)
	builder.config.Params.FrequencyPenalty = 3
	builder.config.Params.SetPresencePenalty(0.5)
	builder.config.Params.PresencePenalty = -3
	_, err := builder.Build(testutil.NewTestContext(t))
	require.Error(t, err)
	require.Contains(t, err.Error(), "max tokens")
	require.Contains(t, err.Error(), "temperature")
	require.Contains(t, err.Error(), "top_p")
	require.Contains(t, err.Error(), "frequency penalty")
	require.Contains(t, err.Error(), "presence penalty")
}

func TestFilterErrorsHandlesNil(t *testing.T) {
	require.Nil(t, filterErrors(nil))
	require.Empty(t, filterErrors([]error{nil}))
}
