package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

var supportedProviders = map[string]core.ProviderName{
	"openai":    core.ProviderOpenAI,
	"anthropic": core.ProviderAnthropic,
	"google":    core.ProviderGoogle,
	"groq":      core.ProviderGroq,
	"ollama":    core.ProviderOllama,
}

var providerList = []string{"openai", "anthropic", "google", "groq", "ollama"}

var cloneProviderConfig = func(cfg *core.ProviderConfig) (*core.ProviderConfig, error) {
	return core.DeepCopy(cfg)
}

// Builder constructs provider configurations with a fluent API while
// collecting validation errors that are reported when Build executes.
type Builder struct {
	config *core.ProviderConfig
	errors []error
}

// New creates a model builder for the requested provider and model pair.
func New(provider, model string) *Builder {
	normalizedProvider := strings.ToLower(strings.TrimSpace(provider))
	trimmedModel := strings.TrimSpace(model)

	return &Builder{
		config: &core.ProviderConfig{
			Provider: core.ProviderName(normalizedProvider),
			Model:    trimmedModel,
		},
		errors: make([]error, 0),
	}
}

// WithAPIKey sets the API key used to authenticate with the provider.
func (b *Builder) WithAPIKey(key string) *Builder {
	if b == nil {
		return nil
	}
	b.config.APIKey = strings.TrimSpace(key)
	return b
}

// WithAPIURL configures a custom endpoint for the provider.
func (b *Builder) WithAPIURL(url string) *Builder {
	if b == nil {
		return nil
	}
	b.config.APIURL = strings.TrimSpace(url)
	return b
}

// WithTemperature assigns the sampling temperature for generation.
func (b *Builder) WithTemperature(temp float64) *Builder {
	if b == nil {
		return nil
	}
	if temp < 0 || temp > 2 {
		b.errors = append(b.errors, fmt.Errorf("temperature must be between 0 and 2 inclusive: got %v", temp))
		return b
	}
	b.config.Params.SetTemperature(temp)
	return b
}

// WithMaxTokens limits the maximum tokens returned by the provider.
func (b *Builder) WithMaxTokens(max int) *Builder {
	if b == nil {
		return nil
	}
	if max <= 0 {
		b.errors = append(b.errors, fmt.Errorf("max tokens must be positive: got %d", max))
		return b
	}
	b.config.Params.SetMaxTokens(int32(max))
	return b
}

// WithTopP configures nucleus sampling for the provider request.
func (b *Builder) WithTopP(topP float64) *Builder {
	if b == nil {
		return nil
	}
	if topP < 0 || topP > 1 {
		b.errors = append(b.errors, fmt.Errorf("top_p must be between 0 and 1 inclusive: got %v", topP))
		return b
	}
	b.config.Params.SetTopP(topP)
	return b
}

// WithFrequencyPenalty adjusts repetition behavior using frequency penalty.
func (b *Builder) WithFrequencyPenalty(penalty float64) *Builder {
	if b == nil {
		return nil
	}
	if penalty < -2 || penalty > 2 {
		b.errors = append(b.errors, fmt.Errorf("frequency penalty must be between -2 and 2 inclusive: got %v", penalty))
		return b
	}
	b.config.Params.SetFrequencyPenalty(penalty)
	return b
}

// WithPresencePenalty modifies topic diversity with a presence penalty.
func (b *Builder) WithPresencePenalty(penalty float64) *Builder {
	if b == nil {
		return nil
	}
	if penalty < -2 || penalty > 2 {
		b.errors = append(b.errors, fmt.Errorf("presence penalty must be between -2 and 2 inclusive: got %v", penalty))
		return b
	}
	b.config.Params.SetPresencePenalty(penalty)
	return b
}

// WithDefault marks whether this configuration should be treated as default.
func (b *Builder) WithDefault(isDefault bool) *Builder {
	if b == nil {
		return nil
	}
	b.config.Default = isDefault
	return b
}

// Build validates the accumulated configuration and returns a provider config.
func (b *Builder) Build(ctx context.Context) (*core.ProviderConfig, error) {
	if b == nil {
		return nil, fmt.Errorf("model builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}

	b.config.APIKey = strings.TrimSpace(b.config.APIKey)

	collected := make([]error, 0, len(b.errors)+8)
	collected = append(collected, b.errors...)
	collected = append(collected, b.validateProvider(ctx)...)
	collected = append(collected, b.validateModel(ctx)...)
	collected = append(collected, b.validateAPIURL(ctx)...)
	collected = append(collected, b.validateParams()...)

	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}

	cloned, err := cloneProviderConfig(b.config)
	if err != nil {
		return nil, fmt.Errorf("failed to clone provider config: %w", err)
	}
	cloned.Params = b.config.Params
	return cloned, nil
}

func (b *Builder) validateProvider(ctx context.Context) []error {
	provider := strings.ToLower(strings.TrimSpace(string(b.config.Provider)))
	errs := make([]error, 0, 1)
	if err := validate.ValidateNonEmpty(ctx, "provider", provider); err != nil {
		return append(errs, err)
	}
	mapped, ok := supportedProviders[provider]
	if !ok {
		return append(
			errs,
			fmt.Errorf("provider %q is not supported; must be one of %s", provider, strings.Join(providerList, ", ")),
		)
	}
	b.config.Provider = mapped
	return errs
}

func (b *Builder) validateModel(ctx context.Context) []error {
	model := strings.TrimSpace(b.config.Model)
	errs := make([]error, 0, 1)
	if err := validate.ValidateNonEmpty(ctx, "model", model); err != nil {
		return append(errs, err)
	}
	b.config.Model = model
	return errs
}

func (b *Builder) validateAPIURL(ctx context.Context) []error {
	apiURL := strings.TrimSpace(b.config.APIURL)
	b.config.APIURL = apiURL
	if apiURL == "" {
		return nil
	}
	if err := validate.ValidateURL(ctx, apiURL); err != nil {
		return []error{err}
	}
	return nil
}

func (b *Builder) validateParams() []error {
	errs := make([]error, 0, 5)
	if b.config.Params.IsSetMaxTokens() && b.config.Params.MaxTokens <= 0 {
		errs = append(errs, fmt.Errorf("max tokens must be positive: got %d", b.config.Params.MaxTokens))
	}
	if b.config.Params.IsSetTemperature() {
		temp := b.config.Params.Temperature
		if temp < 0 || temp > 2 {
			errs = append(errs, fmt.Errorf("temperature must be between 0 and 2 inclusive: got %v", temp))
		}
	}
	if b.config.Params.IsSetTopP() {
		topP := b.config.Params.TopP
		if topP < 0 || topP > 1 {
			errs = append(errs, fmt.Errorf("top_p must be between 0 and 1 inclusive: got %v", topP))
		}
	}
	if b.config.Params.IsSetFrequencyPenalty() {
		penalty := b.config.Params.FrequencyPenalty
		if penalty < -2 || penalty > 2 {
			errs = append(errs, fmt.Errorf("frequency penalty must be between -2 and 2 inclusive: got %v", penalty))
		}
	}
	if b.config.Params.IsSetPresencePenalty() {
		penalty := b.config.Params.PresencePenalty
		if penalty < -2 || penalty > 2 {
			errs = append(errs, fmt.Errorf("presence penalty must be between -2 and 2 inclusive: got %v", penalty))
		}
	}
	return errs
}

func filterErrors(errs []error) []error {
	if len(errs) == 0 {
		return nil
	}
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			filtered = append(filtered, err)
		}
	}
	return filtered
}
