package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/v2/internal/errors"
	"github.com/compozy/compozy/sdk/v2/internal/validate"
)

var supportedProviders = map[string]core.ProviderName{
	"openai":     core.ProviderOpenAI,
	"anthropic":  core.ProviderAnthropic,
	"google":     core.ProviderGoogle,
	"groq":       core.ProviderGroq,
	"ollama":     core.ProviderOllama,
	"deepseek":   core.ProviderDeepSeek,
	"xai":        core.ProviderXAI,
	"cerebras":   core.ProviderCerebras,
	"openrouter": core.ProviderOpenRouter,
}

var providerList = []string{
	"openai",
	"anthropic",
	"google",
	"groq",
	"ollama",
	"deepseek",
	"xai",
	"cerebras",
	"openrouter",
}

// New creates a provider configuration using functional options
func New(ctx context.Context, provider string, model string, opts ...Option) (*core.ProviderConfig, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating provider configuration", "provider", provider, "model", model)
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	cfg := &core.ProviderConfig{
		Provider: core.ProviderName(normalizedProvider),
		Model:    strings.TrimSpace(model),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0, 8)
	if err := validateProvider(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateModel(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if err := validateAPIURL(ctx, cfg); err != nil {
		collected = append(collected, err)
	}
	if errs := validateParams(cfg); len(errs) > 0 {
		collected = append(collected, errs...)
	}
	if len(collected) > 0 {
		return nil, &sdkerrors.BuildError{Errors: collected}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone provider config: %w", err)
	}
	return cloned, nil
}

func validateProvider(ctx context.Context, cfg *core.ProviderConfig) error {
	provider := strings.ToLower(strings.TrimSpace(string(cfg.Provider)))
	if err := validate.NonEmpty(ctx, "provider", provider); err != nil {
		return err
	}
	mapped, ok := supportedProviders[provider]
	if !ok {
		return fmt.Errorf("provider %q is not supported; must be one of %s", provider, strings.Join(providerList, ", "))
	}
	cfg.Provider = mapped
	return nil
}

func validateModel(ctx context.Context, cfg *core.ProviderConfig) error {
	model := strings.TrimSpace(cfg.Model)
	if err := validate.NonEmpty(ctx, "model", model); err != nil {
		return err
	}
	cfg.Model = model
	return nil
}

func validateAPIURL(ctx context.Context, cfg *core.ProviderConfig) error {
	apiURL := strings.TrimSpace(cfg.APIURL)
	cfg.APIURL = apiURL
	if apiURL == "" {
		return nil
	}
	if err := validate.URL(ctx, apiURL); err != nil {
		return err
	}
	return nil
}

func validateParams(cfg *core.ProviderConfig) []error {
	errs := make([]error, 0, 5)
	if cfg.Params.IsSetMaxTokens() && cfg.Params.MaxTokens <= 0 {
		errs = append(errs, fmt.Errorf("max tokens must be positive: got %d", cfg.Params.MaxTokens))
	}
	if cfg.Params.IsSetTemperature() {
		temp := cfg.Params.Temperature
		if temp < 0 || temp > 2 {
			errs = append(errs, fmt.Errorf("temperature must be between 0 and 2 inclusive: got %v", temp))
		}
	}
	if cfg.Params.IsSetTopP() {
		topP := cfg.Params.TopP
		if topP < 0 || topP > 1 {
			errs = append(errs, fmt.Errorf("top_p must be between 0 and 1 inclusive: got %v", topP))
		}
	}
	if cfg.Params.IsSetFrequencyPenalty() {
		penalty := cfg.Params.FrequencyPenalty
		if penalty < -2 || penalty > 2 {
			errs = append(errs, fmt.Errorf("frequency penalty must be between -2 and 2 inclusive: got %v", penalty))
		}
	}
	if cfg.Params.IsSetPresencePenalty() {
		penalty := cfg.Params.PresencePenalty
		if penalty < -2 || penalty > 2 {
			errs = append(errs, fmt.Errorf("presence penalty must be between -2 and 2 inclusive: got %v", penalty))
		}
	}
	return errs
}
