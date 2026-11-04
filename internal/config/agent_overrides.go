package config

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/reasoning"
	speedpkg "github.com/compozy/compozy/internal/speed"
)

// AgentOverridesConfigKey is the [agents] subtable that holds per-agent overrides.
const AgentOverridesConfigKey = "overrides"

// AgentOverride replaces selected runtime fields of the effective agent definition with the
// same name while its prompt, appearance, and every other field keep tracking the source.
// It is read only from operator-authored config.toml layers; extension and marketplace
// packages cannot ship config, and native config tools treat the table as a trust root.
type AgentOverride struct {
	Permissions     string         `toml:"permissions,omitempty"`
	Provider        string         `toml:"provider,omitempty"`
	Model           string         `toml:"model,omitempty"`
	ReasoningEffort string         `toml:"reasoning_effort,omitempty"`
	Speed           speedpkg.Speed `toml:"speed,omitempty"`
}

type agentOverrideOverlay struct {
	Permissions     *string         `toml:"permissions"`
	Provider        *string         `toml:"provider"`
	Model           *string         `toml:"model"`
	ReasoningEffort *string         `toml:"reasoning_effort"`
	Speed           *speedpkg.Speed `toml:"speed"`
}

// agentOverrideFields records which effective runtime fields an override supplied.
type agentOverrideFields struct {
	permissions, provider, model, reasoningEffort, speed bool
}

// applyAgentOverrideOverlays merges each layer per agent and per field, so a workspace layer
// can replace one field of a global override without restating the others.
func applyAgentOverrideOverlays(layer map[string]agentOverrideOverlay, dst *map[string]AgentOverride) {
	if len(layer) == 0 {
		return
	}
	if *dst == nil {
		*dst = make(map[string]AgentOverride, len(layer))
	}
	// Sorted iteration keeps the result stable when two raw names trim to the same agent.
	for _, name := range slices.Sorted(maps.Keys(layer)) {
		overlay := layer[name]
		key := NormalizeAgentName(name)
		current := (*dst)[key]
		assignTrimmed(&current.Permissions, overlay.Permissions)
		assignTrimmed(&current.Provider, overlay.Provider)
		assignTrimmed(&current.Model, overlay.Model)
		assignTrimmed(&current.ReasoningEffort, overlay.ReasoningEffort)
		if overlay.Speed != nil {
			current.Speed = normalizeAgentSpeed(*overlay.Speed)
		}
		(*dst)[key] = current
	}
}

func assignTrimmed(dst *string, value *string) {
	if value != nil {
		*dst = strings.TrimSpace(*value)
	}
}

func validateAgentOverrides(overrides map[string]AgentOverride) error {
	for _, name := range slices.Sorted(maps.Keys(overrides)) {
		override := overrides[name]
		path := "agents." + AgentOverridesConfigKey + "." + name
		if err := ValidateAgentName(name); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if override.Permissions != "" {
			if err := PermissionMode(override.Permissions).Validate(path + ".permissions"); err != nil {
				return err
			}
		}
		if override.ReasoningEffort != "" && !reasoning.IsValid(override.ReasoningEffort) {
			return &reasoning.InvalidEffortError{Path: path + ".reasoning_effort", Value: override.ReasoningEffort}
		}
		if err := validateAgentSpeed(override.Speed, path+".speed"); err != nil {
			return err
		}
	}
	return nil
}

// ResolveAgent resolves a parsed agent definition against its operator override, provider
// config, and global defaults.
func (c *Config) ResolveAgent(agent AgentDef) (ResolvedAgent, error) {
	overridden, applied := c.applyAgentOverride(agent)
	return c.resolveAgentDefinition(overridden, applied)
}

// ResolveAgentDefinition resolves a definition that already passed through ApplyAgentOverride.
func (c *Config) ResolveAgentDefinition(agent AgentDef) (ResolvedAgent, error) {
	return c.resolveAgentDefinition(agent, agentOverrideFields{})
}

// AgentOverride returns the operator override configured for one agent name.
func (c *Config) AgentOverride(name string) (AgentOverride, bool) {
	if c == nil || len(c.Agents.Overrides) == 0 {
		return AgentOverride{}, false
	}
	override, ok := c.Agents.Overrides[NormalizeAgentName(name)]
	return override, ok && override != AgentOverride{}
}

// ApplyAgentOverride returns the definition with its configured operator override applied.
// Callers that layer narrower runtime selections (roles) apply them afterwards and resolve
// the result with ResolveAgentDefinition so the override is not applied twice.
func (c *Config) ApplyAgentOverride(agent AgentDef) AgentDef {
	overridden, _ := c.applyAgentOverride(agent)
	return overridden
}

// applyAgentOverride mirrors the session runtime rule: a changed provider owns the complete
// provider-derived runtime, and a changed model drops an authored effort the override did
// not restate.
func (c *Config) applyAgentOverride(agent AgentDef) (AgentDef, agentOverrideFields) {
	override, ok := c.AgentOverride(agent.Name)
	if !ok {
		return agent, agentOverrideFields{}
	}
	agent = CloneAgentDef(agent)
	var applied agentOverrideFields
	if provider := CanonicalProviderName(override.Provider); provider != "" {
		// Repeating the inherited provider leaves the definition unchanged, so [defaults].model
		// and reasoning_effort still apply; only a different provider becomes agent-owned.
		if provider != CanonicalProviderName(cmp.Or(strings.TrimSpace(agent.Provider), c.Defaults.Provider)) {
			agent.Command, agent.Model, agent.ReasoningEffort = "", "", ""
			agent.Provider = provider
		}
		applied.provider = true
	}
	if override.Model != "" {
		if override.Model != agent.Model && override.ReasoningEffort == "" {
			agent.ReasoningEffort = ""
		}
		agent.Model, applied.model = override.Model, true
	}
	if override.ReasoningEffort != "" {
		agent.ReasoningEffort, applied.reasoningEffort = override.ReasoningEffort, true
	}
	if override.Speed != "" {
		agent.SetSpeed(override.Speed)
		applied.speed = true
	}
	if override.Permissions != "" {
		agent.Permissions, applied.permissions = override.Permissions, true
	}
	return agent, applied
}

// withoutSessionSelections drops attribution for fields a session runtime selection replaced,
// because session selections outrank the operator override.
// A selected model replaces the override effort only when it differs from the current model,
// matching the session rule that keeps the effort for a same-model selection.
func (applied agentOverrideFields) withoutSessionSelections(
	providerChanged, modelSelected, modelChanged, effortSelected bool,
) agentOverrideFields {
	applied.provider = applied.provider && !providerChanged
	applied.model = applied.model && !modelSelected && !providerChanged
	applied.reasoningEffort = applied.reasoningEffort && !effortSelected && !modelChanged && !providerChanged
	return applied
}

// markOverrideSources attributes the overridden effective values to the operator override.
func (applied agentOverrideFields) markOverrideSources(sources *ResolvedRuntimeSources) {
	mark := func(set bool, source *RuntimeValueSource) {
		if set {
			*source = RuntimeValueSourceConfigOverride
		}
	}
	mark(applied.provider, &sources.Provider)
	mark(applied.model, &sources.Model)
	mark(applied.reasoningEffort, &sources.ReasoningEffort)
	mark(applied.speed, &sources.Speed)
	mark(applied.permissions, &sources.Permissions)
}
