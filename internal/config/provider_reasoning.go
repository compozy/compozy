package config

import (
	"fmt"
	"strings"
	"time"
)

const (
	modelGPT56SolID             = "gpt-5.6-sol"
	modelGPT56TerraID           = "gpt-5.6-terra"
	modelGPT56LunaID            = "gpt-5.6-luna"
	modelClaudeFable5ID         = "claude-fable-5"
	modelClaudeOpus48ID         = "claude-opus-4-8"
	modelClaudeSonnet5ID        = "claude-sonnet-5"
	modelClaudeHaiku45CurrentID = "claude-haiku-4-5-20251001"
	providerReasoningLowKey     = "low"
	providerReasoningMaxKey     = "max"
	providerReasoningNoneKey    = "none"
	providerReasoningXHighKey   = "xhigh"
	codexModelsReleaseDate      = "2026-06-26"
)

// ReasoningApplyStrategy identifies how AGH applies a provider reasoning effort.
type ReasoningApplyStrategy string

const (
	// ReasoningApplyACPOption applies effort through an advertised ACP session config option.
	ReasoningApplyACPOption ReasoningApplyStrategy = "acp_option"
	// ReasoningApplyNone exposes no selectable reasoning effort for the provider.
	ReasoningApplyNone ReasoningApplyStrategy = "none"
)

// ProviderReasoningConfig describes provider-level reasoning application.
type ProviderReasoningConfig struct {
	Apply ReasoningApplyStrategy `toml:"apply,omitempty"`
}

type providerReasoningOverlay struct {
	Apply *ReasoningApplyStrategy `toml:"apply"`
}

type providerModelsDiscoveryOverlay struct {
	Enabled  *bool   `toml:"enabled"`
	Command  *string `toml:"command"`
	Endpoint *string `toml:"endpoint"`
	Timeout  *string `toml:"timeout"`
}

func (o providerReasoningOverlay) applyTo(dst *ProviderReasoningConfig) {
	if o.Apply != nil {
		dst.Apply = *o.Apply
	}
}

func (o providerModelsDiscoveryOverlay) Apply(dst *ProviderModelsDiscoveryConfig) {
	if o.Enabled != nil {
		dst.Enabled = new(*o.Enabled)
	}
	if o.Command != nil {
		dst.Command = *o.Command
	}
	if o.Endpoint != nil {
		dst.Endpoint = *o.Endpoint
	}
	if o.Timeout != nil {
		dst.Timeout = *o.Timeout
	}
}

// EffectiveReasoningApply returns the provider's explicit strategy or none.
func (p ProviderModelsConfig) EffectiveReasoningApply() ReasoningApplyStrategy {
	if p.Reasoning.Apply == "" {
		return ReasoningApplyNone
	}
	return p.Reasoning.Apply
}

// Validate reports whether the reasoning strategy is supported.
func (r ProviderReasoningConfig) Validate(path string) error {
	switch r.Apply {
	case "", ReasoningApplyACPOption, ReasoningApplyNone:
		return nil
	default:
		return fmt.Errorf("%s.apply %q is invalid; expected acp_option or none", path, r.Apply)
	}
}

func mergeProviderReasoning(base ProviderReasoningConfig, override ProviderReasoningConfig) ProviderReasoningConfig {
	if override.Apply != "" {
		base.Apply = override.Apply
	}
	return base
}

func providerReasoningConfigIsZero(value ProviderReasoningConfig) bool {
	return value.Apply == ""
}

func builtinClaudeModelsConfig() ProviderModelsConfig {
	return ProviderModelsConfig{
		Default: modelClaudeSonnet5ID,
		Reasoning: ProviderReasoningConfig{
			Apply: ReasoningApplyACPOption,
		},
		Curated: []ProviderModelConfig{
			claudeReasoningModel(modelClaudeFable5ID, "Claude Fable 5", 1_000_000, 128_000, 10, 50, true, "2026-06-09"),
			claudeReasoningModel(modelClaudeOpus48ID, "Claude Opus 4.8", 1_000_000, 128_000, 5, 25, false, ""),
			claudeReasoningModel(modelClaudeSonnet5ID, "Claude Sonnet 5", 1_000_000, 128_000, 3, 15, false, ""),
			{
				ID:                   modelClaudeHaiku45CurrentID,
				DisplayName:          "Claude Haiku 4.5",
				ContextWindow:        new(int64(200_000)),
				MaxOutputTokens:      new(int64(64_000)),
				SupportsTools:        new(true),
				SupportsReasoning:    new(true),
				CostInputPerMillion:  new(1.0),
				CostOutputPerMillion: new(5.0),
			},
		},
	}
}

func claudeReasoningModel(
	id string,
	displayName string,
	contextWindow int64,
	maxOutput int64,
	inputCost float64,
	outputCost float64,
	featured bool,
	releaseDate string,
) ProviderModelConfig {
	return ProviderModelConfig{
		ID:                id,
		DisplayName:       displayName,
		ContextWindow:     new(contextWindow),
		MaxOutputTokens:   new(maxOutput),
		SupportsTools:     new(true),
		SupportsReasoning: new(true),
		ReasoningEfforts: []string{
			providerReasoningLowKey,
			providerMediumKey,
			providerHighKey,
			providerReasoningXHighKey,
			providerReasoningMaxKey,
		},
		DefaultReasoningEffort: providerHighKey,
		CostInputPerMillion:    new(inputCost),
		CostOutputPerMillion:   new(outputCost),
		Featured:               new(featured),
		ReleaseDate:            releaseDate,
	}
}

func builtinCodexModelsConfig() ProviderModelsConfig {
	return ProviderModelsConfig{
		Default: modelGPT56SolID,
		Reasoning: ProviderReasoningConfig{
			Apply: ReasoningApplyACPOption,
		},
		Curated: []ProviderModelConfig{
			codexReasoningModel(modelGPT56SolID, "GPT-5.6 Sol", 5, 30, true),
			codexReasoningModel(modelGPT56TerraID, "GPT-5.6 Terra", 2.5, 15, false),
			codexReasoningModel(modelGPT56LunaID, "GPT-5.6 Luna", 1, 6, false),
		},
	}
}

func codexReasoningModel(
	id string,
	displayName string,
	inputCost float64,
	outputCost float64,
	featured bool,
) ProviderModelConfig {
	return ProviderModelConfig{
		ID:                id,
		DisplayName:       displayName,
		ContextWindow:     new(int64(1_050_000)),
		MaxOutputTokens:   new(int64(128_000)),
		SupportsTools:     new(true),
		SupportsReasoning: new(true),
		ReasoningEfforts: []string{
			providerReasoningNoneKey,
			providerReasoningLowKey,
			providerMediumKey,
			providerHighKey,
			providerReasoningXHighKey,
			providerReasoningMaxKey,
		},
		DefaultReasoningEffort: providerMediumKey,
		CostInputPerMillion:    new(inputCost),
		CostOutputPerMillion:   new(outputCost),
		Featured:               new(featured),
		ReleaseDate:            codexModelsReleaseDate,
	}
}

func validateProviderModelReleaseDate(path string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	layout := "2006-01-02"
	if len(trimmed) == len("2006-01") {
		layout = "2006-01"
	}
	if _, err := time.Parse(layout, trimmed); err != nil {
		return fmt.Errorf("%s.release_date %q must be YYYY-MM or YYYY-MM-DD: %w", path, trimmed, err)
	}
	return nil
}
