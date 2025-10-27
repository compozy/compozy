package model

import (
	"context"
	"errors"
	"testing"

	"github.com/compozy/compozy/engine/core"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
)

func TestNew(t *testing.T) {
	t.Run("Should create model with minimal configuration", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "openai", "gpt-4")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg == nil {
			t.Fatal("expected config, got nil")
		}
		if cfg.Provider != core.ProviderOpenAI {
			t.Errorf("expected provider 'openai', got '%s'", cfg.Provider)
		}
		if cfg.Model != "gpt-4" {
			t.Errorf("expected model 'gpt-4', got '%s'", cfg.Model)
		}
	})
	t.Run("Should trim whitespace from provider and model", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "  openai  ", "  gpt-4  ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Provider != core.ProviderOpenAI {
			t.Errorf("expected trimmed provider 'openai', got '%s'", cfg.Provider)
		}
		if cfg.Model != "gpt-4" {
			t.Errorf("expected trimmed model 'gpt-4', got '%s'", cfg.Model)
		}
	})
	t.Run("Should normalize provider to lowercase", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "OpenAI", "gpt-4")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.Provider != core.ProviderOpenAI {
			t.Errorf("expected provider 'openai', got '%s'", cfg.Provider)
		}
	})
	t.Run("Should fail when context is nil", func(t *testing.T) {
		_, err := New(nil, "openai", "gpt-4")
		if err == nil {
			t.Fatal("expected error for nil context")
		}
		if err.Error() != "context is required" {
			t.Errorf("unexpected error message: %v", err)
		}
	})
	t.Run("Should fail when provider is empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "", "gpt-4")
		if err == nil {
			t.Fatal("expected error for empty provider")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when provider is invalid", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "invalid-provider", "gpt-4")
		if err == nil {
			t.Fatal("expected error for invalid provider")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should fail when model is empty", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "openai", "")
		if err == nil {
			t.Fatal("expected error for empty model")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should create model with all options", func(t *testing.T) {
		ctx := context.Background()
		params := core.PromptParams{}
		params.SetTemperature(0.7)
		params.SetMaxTokens(1000)
		cfg, err := New(ctx, "openai", "gpt-4",
			WithAPIKey("test-key"),
			WithAPIURL("https://api.openai.com/v1"),
			WithParams(params),
			WithOrganization("org-123"),
			WithDefault(true),
			WithMaxToolIterations(10),
			WithContextWindow(8000),
		)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.APIKey != "test-key" {
			t.Errorf("expected api_key 'test-key', got '%s'", cfg.APIKey)
		}
		if cfg.APIURL != "https://api.openai.com/v1" {
			t.Errorf("expected api_url, got '%s'", cfg.APIURL)
		}
		if cfg.Organization != "org-123" {
			t.Errorf("expected organization 'org-123', got '%s'", cfg.Organization)
		}
		if !cfg.Default {
			t.Error("expected default to be true")
		}
		if cfg.MaxToolIterations != 10 {
			t.Errorf("expected max_tool_iterations 10, got %d", cfg.MaxToolIterations)
		}
		if cfg.ContextWindow != 8000 {
			t.Errorf("expected context_window 8000, got %d", cfg.ContextWindow)
		}
		if cfg.Params.Temperature != 0.7 {
			t.Errorf("expected temperature 0.7, got %v", cfg.Params.Temperature)
		}
		if cfg.Params.MaxTokens != 1000 {
			t.Errorf("expected max_tokens 1000, got %d", cfg.Params.MaxTokens)
		}
	})
	t.Run("Should validate temperature range", func(t *testing.T) {
		ctx := context.Background()
		params := core.PromptParams{}
		params.SetTemperature(3.0)
		_, err := New(ctx, "openai", "gpt-4", WithParams(params))
		if err == nil {
			t.Fatal("expected error for invalid temperature")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should validate max_tokens positive", func(t *testing.T) {
		ctx := context.Background()
		params := core.PromptParams{}
		params.SetMaxTokens(-1)
		_, err := New(ctx, "openai", "gpt-4", WithParams(params))
		if err == nil {
			t.Fatal("expected error for negative max_tokens")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should validate top_p range", func(t *testing.T) {
		ctx := context.Background()
		params := core.PromptParams{}
		params.SetTopP(1.5)
		_, err := New(ctx, "openai", "gpt-4", WithParams(params))
		if err == nil {
			t.Fatal("expected error for invalid top_p")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should validate frequency_penalty range", func(t *testing.T) {
		ctx := context.Background()
		params := core.PromptParams{}
		params.SetFrequencyPenalty(3.0)
		_, err := New(ctx, "openai", "gpt-4", WithParams(params))
		if err == nil {
			t.Fatal("expected error for invalid frequency_penalty")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should validate presence_penalty range", func(t *testing.T) {
		ctx := context.Background()
		params := core.PromptParams{}
		params.SetPresencePenalty(-3.0)
		_, err := New(ctx, "openai", "gpt-4", WithParams(params))
		if err == nil {
			t.Fatal("expected error for invalid presence_penalty")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should validate API URL format", func(t *testing.T) {
		ctx := context.Background()
		_, err := New(ctx, "openai", "gpt-4", WithAPIURL("not-a-valid-url"))
		if err == nil {
			t.Fatal("expected error for invalid API URL")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Errorf("expected BuildError, got %T", err)
		}
	})
	t.Run("Should accept valid API URL", func(t *testing.T) {
		ctx := context.Background()
		cfg, err := New(ctx, "openai", "gpt-4", WithAPIURL("https://api.openai.com/v1"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.APIURL != "https://api.openai.com/v1" {
			t.Errorf("expected api_url, got '%s'", cfg.APIURL)
		}
	})
	t.Run("Should support all provider types", func(t *testing.T) {
		ctx := context.Background()
		providers := []struct {
			name     string
			expected core.ProviderName
		}{
			{"openai", core.ProviderOpenAI},
			{"anthropic", core.ProviderAnthropic},
			{"google", core.ProviderGoogle},
			{"groq", core.ProviderGroq},
			{"ollama", core.ProviderOllama},
			{"deepseek", core.ProviderDeepSeek},
			{"xai", core.ProviderXAI},
			{"cerebras", core.ProviderCerebras},
			{"openrouter", core.ProviderOpenRouter},
		}
		for _, p := range providers {
			cfg, err := New(ctx, p.name, "test-model")
			if err != nil {
				t.Errorf("provider %s: unexpected error: %v", p.name, err)
				continue
			}
			if cfg.Provider != p.expected {
				t.Errorf("provider %s: expected %s, got %s", p.name, p.expected, cfg.Provider)
			}
		}
	})
	t.Run("Should perform deep copy", func(t *testing.T) {
		ctx := context.Background()
		params := core.PromptParams{}
		params.SetTemperature(0.5)
		cfg1, err := New(ctx, "openai", "gpt-4", WithParams(params))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		cfg1.Model = "modified"
		cfg2, err := New(ctx, "openai", "gpt-4", WithParams(params))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg2.Model == "modified" {
			t.Error("deep copy failed: configuration was modified")
		}
	})
	t.Run("Should accumulate multiple validation errors", func(t *testing.T) {
		ctx := context.Background()
		params := core.PromptParams{}
		params.SetTemperature(3.0)
		params.SetTopP(2.0)
		_, err := New(ctx, "", "", WithParams(params))
		if err == nil {
			t.Fatal("expected error for multiple validation failures")
		}
		var buildErr *sdkerrors.BuildError
		if !errors.As(err, &buildErr) {
			t.Fatalf("expected BuildError, got %T", err)
		}
		if len(buildErr.Errors) < 2 {
			t.Errorf("expected multiple errors, got %d", len(buildErr.Errors))
		}
	})
}
