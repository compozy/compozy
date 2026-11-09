package config

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/runtimeoption"
	speedpkg "github.com/compozy/compozy/internal/speed"
	toolspkg "github.com/compozy/compozy/internal/tools"
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
	var override ProviderConfig
	overridden := false
	if c != nil {
		override, overridden = c.Providers[providerName]
	}
	retiredBase := false
	if hasBuiltin {
		resolved = cloneProvider(resolved)
		resolved.CommandIsBuiltin = true
	} else {
		resolved, retiredBase = retiredProviderBase(providerName, override, overridden)
		hasBuiltin = retiredBase
	}
	if overridden {
		baseRuntime := resolved.RuntimeProviderName(providerName)
		resolved = mergeProvider(resolved, override)
		// A builtin reviewer mode names a mode of the builtin runtime. Routing the provider to
		// another runtime drops that inherited default, which can mean something else there.
		if strings.TrimSpace(override.ReviewerMode) == "" &&
			!strings.EqualFold(resolved.RuntimeProviderName(providerName), baseRuntime) {
			resolved.ReviewerMode = ""
		}
	}
	if retiredBase {
		warnRetiredBuiltinProvider(providerName, override, overridden)
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

func (c *Config) resolveAgentDefinition(agent AgentDef, applied agentOverrideFields) (ResolvedAgent, error) {
	toolPolicyEnforced := len(agent.Tools) > 0 || len(agent.Toolsets) > 0
	policy, dropped := toolspkg.DropRetiredToolReferences(toolspkg.ToolPolicy{
		Tools: agent.Tools, Toolsets: agent.Toolsets, DenyTools: agent.DenyTools,
	})
	agent.Tools, agent.Toolsets, agent.DenyTools = policy.Tools, policy.Toolsets, policy.DenyTools
	owner := "agent:" + cmp.Or(agent.SourcePath, agent.Name)
	toolspkg.WarnRetiredToolReferences(owner, dropped)
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

	configuredMCP := MergeMCPServers(mcpServers, provider.MCPServers)
	agent.MCPServers = ResolveAgentMCPReferences(agent.MCPServers, configuredMCP)

	resolvedPermissions := cmp.Or(strings.TrimSpace(agent.Permissions), string(permissions.Mode))
	command := cmp.Or(strings.TrimSpace(agent.Command), strings.TrimSpace(provider.Command))

	model, reasoningEffort, modelSource, reasoningSource, err := resolveAgentModelRuntime(
		providerName,
		provider,
		agent,
		defaults.sessionRuntimeFor(providerSource),
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
	resolved.ToolPolicyEnforced = toolPolicyEnforced
	resolved.CommandIsBuiltin = provider.CommandIsBuiltin && strings.TrimSpace(agent.Command) == ""
	resolved.RuntimeSources = ResolvedRuntimeSources{
		Provider:        providerSource,
		Model:           modelSource,
		ReasoningEffort: reasoningSource,
		Speed:           speedSource,
		Permissions:     permissionsSource(agent.Permissions),
	}
	applied.markOverrideSources(&resolved.RuntimeSources)
	if err := applyProviderFullAccessPreference(&resolved, providerName, provider, permissions); err != nil {
		return ResolvedAgent{}, err
	}

	if err := validateResolvedAgentRuntime(providerName, provider, resolved); err != nil {
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

// resolveAgentModel applies the model cascade: the definition, then [defaults].model for a
// project-default provider, then the provider default.
func resolveAgentModel(
	provider ProviderConfig,
	agent AgentDef,
	session sessionRuntimeDefaults,
) (string, RuntimeValueSource) {
	if model := strings.TrimSpace(agent.Model); model != "" {
		return model, RuntimeValueSourceAgent
	}
	if session.model != "" {
		return session.model, RuntimeValueSourceProjectDefault
	}
	if model := strings.TrimSpace(provider.Models.Default); model != "" {
		return model, RuntimeValueSourceProviderDefault
	}
	return "", RuntimeValueSourceUnspecified
}

// resolveAgentModelRuntime selects model and effort with source attribution,
// omitting defaults that the chosen model does not advertise.
func resolveAgentModelRuntime(
	providerName string,
	provider ProviderConfig,
	agent AgentDef,
	session sessionRuntimeDefaults,
) (string, string, RuntimeValueSource, RuntimeValueSource, error) {
	model, modelSource := resolveAgentModel(provider, agent, session)
	if model == "" && provider.RequiresRuntimeModel() {
		return "", "", RuntimeValueSourceUnspecified, RuntimeValueSourceUnspecified,
			runtimeModelRequiredError{provider: providerName}
	}

	reasoningEffort := strings.TrimSpace(agent.ReasoningEffort)
	reasoningSource := RuntimeValueSourceAgent
	// The default effort belongs to the default model: an agent-authored model
	// keeps its own curated default instead.
	if reasoningEffort == "" && modelSource != RuntimeValueSourceAgent &&
		providerAcceptsReasoningEffort(provider, model, session.reasoningEffort) {
		reasoningEffort = session.reasoningEffort
		reasoningSource = RuntimeValueSourceProjectDefault
	}
	if reasoningEffort == "" {
		modelDefault := defaultReasoningEffortForModel(provider, model)
		if providerAcceptsReasoningEffort(provider, model, modelDefault) {
			reasoningEffort = modelDefault
			reasoningSource = RuntimeValueSourceModelDefault
		}
	}
	if reasoningEffort == "" {
		reasoningSource = RuntimeValueSourceUnspecified
	}
	return model, reasoningEffort, modelSource, reasoningSource, nil
}

// validateResolvedAgentRuntime rejects authoritative static capability absence.
// Partial curation cannot deny effort choices advertised later by live ACP.
func validateResolvedAgentRuntime(
	providerName string,
	provider ProviderConfig,
	resolved ResolvedAgent,
) error {
	if strings.TrimSpace(resolved.Command) == "" {
		return fmt.Errorf("provider %q command is required", providerName)
	}
	if strings.TrimSpace(resolved.Permissions) != "" {
		if err := PermissionMode(resolved.Permissions).Validate("agent.permissions"); err != nil {
			return err
		}
	}
	if !providerModelsConfigIsZero(provider.Models) &&
		resolved.Reasoning.EffectiveReasoningApply() == ReasoningApplyNone &&
		strings.TrimSpace(resolved.ReasoningEffort) != "" {
		return fmt.Errorf("provider %q does not support reasoning effort", providerName)
	}
	if effort := strings.TrimSpace(resolved.ReasoningEffort); effort != "" {
		for _, model := range provider.Models.Curated {
			if strings.TrimSpace(model.ID) == resolved.Model && len(model.ReasoningEfforts) == 0 &&
				builtinModelHasNoSelectableReasoning(providerName, resolved.Model) {
				return modelReasoningUnavailableError(providerName, resolved.Model, effort)
			}
		}
	}
	return nil
}

// modelReasoningUnavailableError preserves the public structured validation code
// when static model capabilities rule out an effort before ACP negotiation.
func modelReasoningUnavailableError(provider, model, effort string) error {
	cause := fmt.Errorf(
		"reasoning effort %q is unavailable for provider model %q/%q; omit reasoning_effort or select an advertised value",
		effort,
		provider,
		model,
	)
	item := diagnostics.NewItem(diagnostics.ItemSpec{
		ID:            "provider.models.reasoning_effort_unsupported",
		Code:          diagnosticcontract.CodeReasoningEffortUnsupported,
		Category:      diagnosticcontract.CategoryProvider,
		Title:         "Reasoning effort is unsupported",
		Message:       cause.Error(),
		Severity:      diagnosticcontract.SeverityError,
		DataFreshness: diagnosticcontract.FreshnessLive,
	}, diagnostics.WithEvidence(map[string]any{
		"provider_id": provider, "model_id": model, "requested": effort,
	}))
	return diagnostics.NewStructuredError(item, cause)
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
		ReviewerMode:          provider.ReviewerMode,
		Name:                  agent.Name,
		Provider:              providerName,
		Command:               command,
		DisplayName:           provider.DisplayName,
		Model:                 model,
		ReasoningEffort:       reasoningEffort,
		Tools:                 cloneStrings(agent.Tools),
		Toolsets:              cloneStrings(agent.Toolsets),
		DenyTools:             cloneStrings(agent.DenyTools),
		Permissions:           resolvedPermissions,
		PermissionsConfigured: strings.TrimSpace(agent.Permissions) != "",
		Harness:               provider.EffectiveHarness(),
		RuntimeProvider:       provider.RuntimeProviderName(providerName),
		Transport:             strings.TrimSpace(provider.Transport),
		BaseURL:               strings.TrimSpace(provider.BaseURL),
		AuthMode:              provider.EffectiveAuthMode(),
		EnvPolicy:             provider.EffectiveEnvPolicy(),
		HomePolicy:            provider.EffectiveHomePolicy(),
		NoneSecurity:          provider.EffectiveNoneSecurity(),
		AuthStatusCmd:         strings.TrimSpace(provider.AuthStatusCmd),
		AuthLoginCmd:          strings.TrimSpace(provider.AuthLoginCmd),
		AuthACPMethod:         provider.EffectiveACPMethod(),
		SessionMCP:            provider.SessionMCPEnabled(),
		SessionMCPOverHTTP:    provider.SessionMCPOverHTTP(),
		SessionEnv:            MergeSessionEnv(provider.SessionEnv, agent.SessionEnv),
		CredentialSlots:       provider.EffectiveCredentialSlots(),
		MCPServers:            mergeMCPServerLayers(mcpServers, provider.MCPServers, agent.MCPServers),
		Reasoning:             provider.Models.Reasoning,
		Prompt:                agent.Prompt,
	}
	resolved.setAutoCompactWindow(cmp.Or(agent.AutoCompactWindow, provider.AutoCompactWindow))
	resolved.SetSpeed(speed)
	resolved.SetACPOptions(agent.ACPOptionsValue())
	return resolved
}

func applyProviderFullAccessPreference(
	resolved *ResolvedAgent,
	providerName string,
	provider ProviderConfig,
	permissions PermissionsConfig,
) error {
	if !permissions.ProviderFullAccess || resolved.Permissions != string(PermissionModeApproveAll) {
		return nil
	}
	// Copilot keeps approvals in its own allow_all option; its modes are task modes.
	optionID := "mode"
	if providerName == providerCopilotKey {
		optionID = CopilotAllowAllOptionID
	}
	for _, option := range resolved.ACPOptionsValue() {
		if option.ID == optionID {
			return nil // The agent explicitly chose this ACP option.
		}
	}
	if provider.EffectiveHarness() != ProviderHarnessACP {
		return fmt.Errorf(
			"permissions.provider_full_access requires an ACP provider; %q uses %q",
			providerName,
			provider.EffectiveHarness(),
		)
	}
	var valueID string
	switch providerName {
	case providerCodexKey:
		valueID = "agent-full-access"
	case "claude":
		valueID = "bypassPermissions"
	case providerAgyKey:
		valueID = "yolo"
	case providerCopilotKey:
		valueID = CopilotAllowAllOn
	default:
		return fmt.Errorf("permissions.provider_full_access is unsupported for provider %q", providerName)
	}
	options, _ := runtimeoption.MergeSelections(
		[]ACPOptionSelection{{ID: optionID, ValueID: valueID}}, resolved.ACPOptionsValue(),
	)
	resolved.SetACPOptions(options)
	return nil
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
	SessionEnv SessionEnv
	Provider   string
	Model      string
	Reasoning  string
	// Command is an explicit launch command for this attempt (a fallback route's account).
	// It is applied after the provider-aware resolution: when non-empty it replaces the
	// resolved command on any provider; when empty the existing rule holds (the agent
	// command is kept only for a same-provider selection, else the selected provider's).
	Command string
}

// ResolveSessionAgentWithRuntime resolves one session agent with runtime-level overrides.
func (c *Config) ResolveSessionAgentWithRuntime(
	agent AgentDef,
	overrides RuntimeOverrides,
) (ResolvedAgent, error) {
	resolved, err := c.resolveSessionAgentRuntime(agent, overrides)
	if err != nil {
		return ResolvedAgent{}, err
	}
	if command := strings.TrimSpace(overrides.Command); command != "" {
		resolved.Command = command
		resolved.CommandIsBuiltin = false
	}
	if err := overrides.SessionEnv.Validate("runtime.session_env"); err != nil {
		return ResolvedAgent{}, err
	}
	resolved.SessionEnv = MergeSessionEnv(resolved.SessionEnv, overrides.SessionEnv)
	return resolved, nil
}

func (c *Config) resolveSessionAgentRuntime(
	agent AgentDef,
	overrides RuntimeOverrides,
) (ResolvedAgent, error) {
	agent, applied := c.applyAgentOverride(agent)
	rawProviderOverride := strings.TrimSpace(overrides.Provider)
	override := CanonicalProviderName(rawProviderOverride)
	model := strings.TrimSpace(overrides.Model)
	reasoningEffort := strings.TrimSpace(overrides.Reasoning)
	if rawProviderOverride == "" && model == "" && reasoningEffort == "" {
		return c.resolveAgentDefinition(agent, applied)
	}

	effectiveProvider := CanonicalProviderName(agent.Provider)
	if effectiveProvider == "" && c != nil {
		effectiveProvider = CanonicalProviderName(c.Defaults.Provider)
	}
	// Selecting the model the agent already resolves to is not a model change, so its effort stays.
	modelChanged := model != "" && model != c.EffectiveAgentModel(agent)
	applied = applied.withoutSessionSelections(
		rawProviderOverride != "" && override != effectiveProvider, model != "", modelChanged, reasoningEffort != "",
	)
	if rawProviderOverride == "" || override == effectiveProvider {
		sessionAgent := agent
		if model != "" {
			if modelChanged && reasoningEffort == "" {
				sessionAgent.ReasoningEffort = ""
			}
			sessionAgent.Model = model
		}
		if reasoningEffort != "" {
			sessionAgent.ReasoningEffort = reasoningEffort
		}
		return c.resolveAgentDefinition(sessionAgent, applied)
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

	resolved, err := c.resolveAgentDefinition(sessionAgent, applied)
	if err != nil {
		return ResolvedAgent{}, fmt.Errorf("resolve session agent with provider %q: %w", override, err)
	}

	return resolved, nil
}
