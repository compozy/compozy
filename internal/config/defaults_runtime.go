package config

import (
	"fmt"
	"slices"
	"strings"

	burnttoml "github.com/BurntSushi/toml"
)

var (
	defaultsProviderPath        = []string{bootstrapDefaultsKey, bootstrapProviderKey}
	defaultsModelPath           = []string{bootstrapDefaultsKey, "model"}
	defaultsReasoningEffortPath = []string{bootstrapDefaultsKey, "reasoning_effort"}
)

// sessionRuntimeDefaults is the `[defaults]` model and reasoning that apply to
// one agent resolution. It is empty unless the provider came from
// defaults.provider.
type sessionRuntimeDefaults struct {
	model           string
	reasoningEffort string
}

// sessionRuntimeFor returns the model and reasoning refinements for a provider
// resolved from source. An agent-authored provider never inherits them: the
// default model was chosen for the default provider only.
func (c DefaultsConfig) sessionRuntimeFor(source RuntimeValueSource) sessionRuntimeDefaults {
	if source != RuntimeValueSourceProjectDefault {
		return sessionRuntimeDefaults{}
	}
	return sessionRuntimeDefaults{
		model:           strings.TrimSpace(c.Model),
		reasoningEffort: strings.TrimSpace(c.ReasoningEffort),
	}
}

// validateSessionRuntime requires the model and reasoning defaults to refine a
// configured provider and the reasoning value to be a known effort.
func (c DefaultsConfig) validateSessionRuntime() error {
	if strings.TrimSpace(c.Provider) == "" {
		if strings.TrimSpace(c.Model) != "" {
			return ValidationError{Path: configDefaultsModelPath, Message: "requires defaults.provider"}
		}
		if strings.TrimSpace(c.ReasoningEffort) != "" {
			return ValidationError{Path: configDefaultsReasoningEffortPath, Message: "requires defaults.provider"}
		}
	}
	return validateRoleReasoningEffort(configDefaultsReasoningEffortPath, c.ReasoningEffort)
}

// providerAcceptsReasoningEffort reports whether a resolved provider can apply
// a reasoning effort. A default effort is skipped for providers that cannot,
// rather than failing every session that inherits it.
func providerAcceptsReasoningEffort(provider ProviderConfig, modelID, effort string) bool {
	if !providerModelsConfigIsZero(provider.Models) && provider.Models.EffectiveReasoningApply() == ReasoningApplyNone {
		return false
	}
	for _, model := range provider.Models.Curated {
		if strings.TrimSpace(model.ID) == modelID {
			return slices.Contains(model.ReasoningEfforts, effort)
		}
	}
	return true
}

// builtinModelHasNoSelectableReasoning requires a declared reasoning capability;
// name-only builtin entries and partial curation leave live ACP choices unknown.
func builtinModelHasNoSelectableReasoning(providerName, modelID string) bool {
	for _, model := range builtinProviders[providerName].Models.Curated {
		if strings.TrimSpace(model.ID) == modelID {
			return model.SupportsReasoning != nil && len(model.ReasoningEfforts) == 0
		}
	}
	return false
}

// dropStaleDefaultsRuntime removes this document's defaults.model and
// defaults.reasoning_effort before defaults.provider changes to provider, so a
// single-key provider write (CLI, native tool, install, profile seed) never
// leaves the previous provider's model behind. Writers that set the model in
// the same edit write it after the provider.
func (e *OverlayEditor) dropStaleDefaultsRuntime(path []string, provider any) error {
	if !slices.Equal(path, defaultsProviderPath) {
		return nil
	}
	next, ok := provider.(string)
	if !ok {
		return nil
	}
	var document struct {
		Defaults struct {
			Provider *string `toml:"provider"`
		} `toml:"defaults"`
	}
	if _, err := burnttoml.Decode(string(e.content), &document); err != nil {
		return fmt.Errorf("config: read defaults.provider from %q: %w", e.source, err)
	}
	current := document.Defaults.Provider
	if current != nil && CanonicalProviderName(*current) == CanonicalProviderName(next) {
		return nil
	}
	for _, stale := range [][]string{defaultsModelPath, defaultsReasoningEffortPath} {
		if err := e.Delete(stale); err != nil {
			return err
		}
	}
	return nil
}
