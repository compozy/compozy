package config

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	speedpkg "github.com/compozy/compozy/internal/speed"
)

// ErrRuntimeModelRequired identifies providers that cannot resolve a model.
var ErrRuntimeModelRequired = errors.New("runtime model required")

type runtimeModelRequiredError struct {
	provider string
}

func (e runtimeModelRequiredError) Error() string {
	return fmt.Sprintf("agent model is required when provider %q has no default model", e.provider)
}

func (e runtimeModelRequiredError) Is(target error) bool {
	return target == ErrRuntimeModelRequired
}

// ResolveProvider resolves a provider using the built-in registry and config overrides.
func (c *Config) ResolveProvider(name string) (ProviderConfig, error) {
	providerName := CanonicalProviderName(name)
	if providerName == "" {
		return ProviderConfig{}, errors.New("provider name is required")
	}

	resolved, hasBuiltin := builtinProviders[providerName]
	if hasBuiltin {
		resolved = cloneProvider(resolved)
	}
	if c != nil {
		if override, ok := c.Providers[providerName]; ok {
			resolved = mergeProvider(resolved, override)
		}
	}

	if !hasBuiltin {
		if c == nil {
			return ProviderConfig{}, newUnknownProviderError(providerName)
		}
		if _, ok := c.Providers[providerName]; !ok {
			return ProviderConfig{}, newUnknownProviderError(providerName)
		}
	}
	resolved.Models.Default = strings.TrimSpace(resolved.Models.Default)

	if err := validateResolvedProvider(providerName, resolved); err != nil {
		return ProviderConfig{}, fmt.Errorf("%w: %w", ErrProviderUnavailable, err)
	}

	return resolved, nil
}

// ResolveAgent resolves a parsed agent definition against provider config and global defaults.
func (c *Config) ResolveAgent(agent AgentDef) (ResolvedAgent, error) {
	if err := agent.Validate(); err != nil {
		return ResolvedAgent{}, err
	}

	var defaults DefaultsConfig
	var permissions PermissionsConfig
	var mcpServers []MCPServer
	if c != nil {
		defaults = c.Defaults
		permissions = c.Permissions
		mcpServers = c.MCPServers
	}

	providerName, provider, providerSource, err := c.resolveAgentProvider(agent.Provider, defaults.Provider)
	if err != nil {
		return ResolvedAgent{}, err
	}

	resolvedPermissions := cmp.Or(strings.TrimSpace(agent.Permissions), string(permissions.Mode))
	command := cmp.Or(strings.TrimSpace(agent.Command), strings.TrimSpace(provider.Command))

	model, reasoningEffort, modelSource, reasoningSource, err := resolveAgentModelRuntime(
		providerName,
		provider,
		agent,
	)
	if err != nil {
		return ResolvedAgent{}, err
	}
	speed, speedSource := resolveAgentSpeedRuntime(provider, model, agent.SpeedValue())

	resolved := resolvedAgentFromProvider(
		agent,
		providerName,
		provider,
		resolvedPermissions,
		command,
		model,
		reasoningEffort,
		speed,
		mcpServers,
	)
	resolved.RuntimeSources = ResolvedRuntimeSources{
		Provider:        providerSource,
		Model:           modelSource,
		ReasoningEffort: reasoningSource,
		Speed:           speedSource,
	}

	if err := validateResolvedAgentRuntime(providerName, resolved); err != nil {
		return ResolvedAgent{}, err
	}

	return resolved, nil
}

func resolveAgentSpeedRuntime(
	provider ProviderConfig,
	model string,
	authored speedpkg.Speed,
) (speedpkg.Speed, RuntimeValueSource) {
	if authored != "" {
		return authored, RuntimeValueSourceAgent
	}
	modelDefault := defaultSpeedForModel(provider, model)
	if modelDefault != "" {
		return modelDefault, RuntimeValueSourceModelDefault
	}
	return "", RuntimeValueSourceUnspecified
}

func (c *Config) resolveAgentProvider(
	agentProvider string,
	defaultProvider string,
) (string, ProviderConfig, RuntimeValueSource, error) {
	providerName := CanonicalProviderName(agentProvider)
	providerSource := RuntimeValueSourceAgent
	if providerName == "" {
		providerName = CanonicalProviderName(defaultProvider)
		providerSource = RuntimeValueSourceProjectDefault
	}
	if providerName == "" {
		return "", ProviderConfig{}, RuntimeValueSourceUnspecified, errors.New(
			"agent provider is required; run `compozy install` or set agent.provider/defaults.provider",
		)
	}

	provider, err := c.ResolveProvider(providerName)
	if err != nil {
		return "", ProviderConfig{}, RuntimeValueSourceUnspecified, err
	}
	return providerName, provider, providerSource, nil
}

func resolveAgentModelRuntime(
	providerName string,
	provider ProviderConfig,
	agent AgentDef,
) (string, string, RuntimeValueSource, RuntimeValueSource, error) {
	model := strings.TrimSpace(agent.Model)
	modelSource := RuntimeValueSourceAgent
	if model == "" {
		model = strings.TrimSpace(provider.Models.Default)
		modelSource = RuntimeValueSourceProviderDefault
	}
	if model == "" {
		modelSource = RuntimeValueSourceUnspecified
	}
	if model == "" && provider.RequiresRuntimeModel() {
		return "", "", RuntimeValueSourceUnspecified, RuntimeValueSourceUnspecified,
			runtimeModelRequiredError{provider: providerName}
	}

	reasoningEffort := strings.TrimSpace(agent.ReasoningEffort)
	reasoningSource := RuntimeValueSourceAgent
	if reasoningEffort == "" {
		reasoningEffort = defaultReasoningEffortForModel(provider, model)
		reasoningSource = RuntimeValueSourceModelDefault
	}
	if reasoningEffort == "" {
		reasoningSource = RuntimeValueSourceUnspecified
	}
	return model, reasoningEffort, modelSource, reasoningSource, nil
}

func validateResolvedAgentRuntime(providerName string, resolved ResolvedAgent) error {
	if strings.TrimSpace(resolved.Command) == "" {
		return fmt.Errorf("provider %q command is required", providerName)
	}
	if strings.TrimSpace(resolved.Permissions) == "" {
		return nil
	}
	return PermissionMode(resolved.Permissions).Validate("agent.permissions")
}

func resolvedAgentFromProvider(
	agent AgentDef,
	providerName string,
	provider ProviderConfig,
	resolvedPermissions string,
	command string,
	model string,
	reasoningEffort string,
	speed speedpkg.Speed,
	mcpServers []MCPServer,
) ResolvedAgent {
	resolved := ResolvedAgent{
		Name:            agent.Name,
		Provider:        providerName,
		Command:         command,
		DisplayName:     provider.DisplayName,
		Model:           model,
		ReasoningEffort: reasoningEffort,
		Tools:           cloneStrings(agent.Tools),
		Toolsets:        cloneStrings(agent.Toolsets),
		DenyTools:       cloneStrings(agent.DenyTools),
		Permissions:     resolvedPermissions,
		Harness:         provider.EffectiveHarness(),
		RuntimeProvider: provider.RuntimeProviderName(providerName),
		Transport:       strings.TrimSpace(provider.Transport),
		BaseURL:         strings.TrimSpace(provider.BaseURL),
		AuthMode:        provider.EffectiveAuthMode(),
		EnvPolicy:       provider.EffectiveEnvPolicy(),
		HomePolicy:      provider.EffectiveHomePolicy(),
		NoneSecurity:    provider.EffectiveNoneSecurity(),
		AuthStatusCmd:   strings.TrimSpace(provider.AuthStatusCmd),
		AuthLoginCmd:    strings.TrimSpace(provider.AuthLoginCmd),
		SessionMCP:      provider.SessionMCPEnabled(),
		CredentialSlots: provider.EffectiveCredentialSlots(),
		MCPServers:      mergeMCPServerLayers(mcpServers, provider.MCPServers, agent.MCPServers),
		Reasoning:       provider.Models.Reasoning,
		Prompt:          agent.Prompt,
	}
	resolved.SetSpeed(speed)
	resolved.SetACPOptions(agent.ACPOptionsValue())
	return resolved
}

// ResolveSessionAgent resolves a parsed agent definition for one session.
// When providerOverride is set, the selected provider becomes canonical and
// provider-owned runtime fields are re-resolved from that provider to avoid
// mixed runtimes from the original agent definition.
func (c *Config) ResolveSessionAgent(agent AgentDef, providerOverride string) (ResolvedAgent, error) {
	return c.ResolveSessionAgentWithRuntime(agent, RuntimeOverrides{Provider: providerOverride})
}

// RuntimeOverrides are the engine-resolved runtime fields applied to one session.
type RuntimeOverrides struct {
	Provider  string
	Model     string
	Reasoning string
}

// ResolveSessionAgentWithRuntime resolves one session agent with runtime-level overrides.
func (c *Config) ResolveSessionAgentWithRuntime(
	agent AgentDef,
	overrides RuntimeOverrides,
) (ResolvedAgent, error) {
	rawProviderOverride := strings.TrimSpace(overrides.Provider)
	override := CanonicalProviderName(rawProviderOverride)
	model := strings.TrimSpace(overrides.Model)
	reasoningEffort := strings.TrimSpace(overrides.Reasoning)
	if rawProviderOverride == "" && model == "" && reasoningEffort == "" {
		return c.ResolveAgent(agent)
	}

	effectiveProvider := CanonicalProviderName(agent.Provider)
	if effectiveProvider == "" && c != nil {
		effectiveProvider = CanonicalProviderName(c.Defaults.Provider)
	}
	if rawProviderOverride == "" || override == effectiveProvider {
		sessionAgent := agent
		if model != "" {
			if model != strings.TrimSpace(agent.Model) && reasoningEffort == "" {
				sessionAgent.ReasoningEffort = ""
			}
			sessionAgent.Model = model
		}
		if reasoningEffort != "" {
			sessionAgent.ReasoningEffort = reasoningEffort
		}
		return c.ResolveAgent(sessionAgent)
	}

	// A changed provider owns the complete provider-derived runtime. Clearing
	// these fields prevents an agent model or command from leaking across
	// providers while same-provider selections retain valid agent-specific
	// commands and apply only the explicitly selected runtime fields above.
	sessionAgent := agent
	sessionAgent.Provider = override
	sessionAgent.Command = ""
	sessionAgent.Model = ""
	sessionAgent.ReasoningEffort = ""
	if model != "" {
		sessionAgent.Model = model
	}
	if reasoningEffort != "" {
		sessionAgent.ReasoningEffort = reasoningEffort
	}

	resolved, err := c.ResolveAgent(sessionAgent)
	if err != nil {
		return ResolvedAgent{}, fmt.Errorf("resolve session agent with provider %q: %w", override, err)
	}

	return resolved, nil
}
