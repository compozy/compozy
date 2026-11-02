package session

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/providers"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const subagentProviderUnavailable = "provider_unavailable"

// PermissionLimit returns the caller's mode, or the child's configured level in an instance lineage.
func (r managerSubagentRuntime) PermissionLimit(
	ctx context.Context,
	parent *Info,
	requested SubagentTarget,
) (SubagentPermissionLimit, error) {
	return r.m.SubagentPermissionLimit(ctx, parent, requested)
}

// SubagentPermissionLimit resolves the limit for one delegation from parent to the requested target.
func (m *Manager) SubagentPermissionLimit(
	ctx context.Context,
	parent *Info,
	requested SubagentTarget,
) (SubagentPermissionLimit, error) {
	inherited := SubagentPermissionLimit{Mode: permissionMode(parent)}
	if m.instanceCeiling == nil {
		return inherited, nil
	}
	ceiling, err := m.instanceCeiling(ctx, parent.ID, "")
	if err != nil || ceiling == nil {
		return inherited, err
	}
	runtime := managerSubagentRuntime{m: m}
	target, err := runtime.Resolve(ctx, parent, requested)
	if err != nil {
		return SubagentPermissionLimit{}, err
	}
	workspace, err := m.resolveResumeWorkspace(
		ctx,
		&store.SessionMeta{WorkspaceID: parent.WorkspaceID, ProfileID: parent.ProfileID},
	)
	if err != nil {
		return SubagentPermissionLimit{}, err
	}
	resolved, err := resolveWorkspaceSessionAgentForType(
		target.Agent, target.Provider, SessionTypeSpawned, &workspace, m.agentResolver,
	)
	if err != nil {
		return SubagentPermissionLimit{}, err
	}
	return SubagentPermissionLimit{Mode: m.startPermissions(resolved.Permissions), ChildOwnsLevel: true}, nil
}

func (r managerSubagentRuntime) Resolve(
	ctx context.Context,
	parent *Info,
	requested SubagentTarget,
) (SubagentTarget, error) {
	_, err := r.m.requireWorkspaceResolver()
	if err != nil {
		return SubagentTarget{}, err
	}
	workspace, err := r.m.resolveResumeWorkspace(
		ctx,
		&store.SessionMeta{WorkspaceID: parent.WorkspaceID, ProfileID: parent.ProfileID},
	)
	if err != nil {
		return SubagentTarget{}, err
	}
	inherited := subagentInherited(parent)
	agent, provider := requested.Agent, requested.Provider
	if agent == "" {
		agent = inherited.Agent
	}
	if provider == "" {
		provider = inherited.Provider
	}
	resolved, err := resolveWorkspaceSessionAgentForType(
		agent,
		provider,
		SessionTypeSpawned,
		&workspace,
		r.m.agentResolver,
	)
	if err != nil {
		if errors.Is(err, workspacepkg.ErrAgentNotAvailable) {
			return SubagentTarget{}, &SubagentError{
				Code:    "agent_not_found",
				Message: fmt.Sprintf("Agent %s not found.", agent),
				Err:     ErrSubagentTargetUnavailable,
			}
		}
		return SubagentTarget{}, &SubagentError{
			Code:    subagentProviderUnavailable,
			Message: err.Error(),
			Err:     ErrSubagentTargetUnavailable,
		}
	}
	option, err := r.providerOption(ctx, &workspace.Config, provider)
	if err != nil {
		return SubagentTarget{}, err
	}
	if !option.CanDelegate {
		return SubagentTarget{}, &SubagentError{
			Code:    subagentProviderUnavailable,
			Message: fmt.Sprintf("Provider %s is unavailable: %s", provider, strings.Join(option.Constraints, " ")),
			Err:     ErrSubagentTargetUnavailable,
		}
	}
	target := resolveSubagentTarget(
		inherited,
		requested,
		SubagentTarget{
			Agent:           agent,
			Provider:        provider,
			Model:           resolved.Model,
			ReasoningEffort: resolved.ReasoningEffort,
			Speed:           string(resolved.SpeedValue()),
		},
	)
	if err := validateSubagentModel(target.Model, target.Provider, option.Models); err != nil {
		return SubagentTarget{}, err
	}
	return target, nil
}

// Capabilities describes provider and agent targets available in the parent session's profile.
func (r managerSubagentRuntime) Capabilities(
	ctx context.Context,
	parent *Info,
) ([]SubagentAgentOption, []SubagentProviderOption, error) {
	_, err := r.m.requireWorkspaceResolver()
	if err != nil {
		return nil, nil, err
	}
	workspace, err := r.m.resolveResumeWorkspace(
		ctx,
		&store.SessionMeta{WorkspaceID: parent.WorkspaceID, ProfileID: parent.ProfileID},
	)
	if err != nil {
		return nil, nil, err
	}
	names := make(map[string]bool)
	for name := range compozyconfig.BuiltinProviders() {
		names[name] = true
	}
	for name := range workspace.Config.Providers {
		names[name] = true
	}
	ordered := make([]string, 0, len(names))
	for name := range names {
		ordered = append(ordered, name)
	}
	slices.Sort(ordered)
	options := make([]SubagentProviderOption, 0, len(names))
	byName := make(map[string]SubagentProviderOption)
	for _, name := range ordered {
		option, err := r.providerOption(ctx, &workspace.Config, name)
		if err != nil {
			return nil, nil, err
		}
		provider, err := workspace.Config.ResolveProvider(name)
		if err != nil {
			return nil, nil, err
		}
		current := ""
		if inherited := subagentInherited(parent); inherited.Provider == name {
			current = inherited.Model
		}
		option = subagentCapabilityModels(option, current, provider.Models.Default)
		options = append(options, option)
		byName[name] = option
	}
	definitions := workspace.Agents
	if catalog, ok := r.m.agentResolver.(AgentCatalogResolver); ok {
		definitions = catalog.ListAgentDefinitions(&workspace)
	}
	agents := make([]SubagentAgentOption, 0, len(definitions))
	for _, agent := range definitions {
		resolved, err := workspace.Config.ResolveAgent(agent)
		option := SubagentAgentOption{Name: agent.Name, Constraints: []string{}}
		if err != nil {
			option.Constraints = []string{err.Error()}
		} else {
			provider := byName[resolved.Provider]
			option.Provider = resolved.Provider
			option.CanDelegate = provider.CanDelegate
			option.Constraints = slices.Clone(provider.Constraints)
		}
		agents = append(agents, option)
	}
	return agents, options, nil
}

func (r managerSubagentRuntime) providerOption(
	ctx context.Context,
	cfg *compozyconfig.Config,
	name string,
) (SubagentProviderOption, error) {
	option := SubagentProviderOption{
		Provider:    name,
		CanDelegate: true,
		Constraints: []string{},
		Models:      []SubagentModelOption{},
	}
	provider, err := cfg.ResolveProvider(name)
	if err != nil {
		return option, &SubagentError{
			Code:    subagentProviderUnavailable,
			Message: err.Error(),
			Err:     ErrSubagentTargetUnavailable,
		}
	}
	option.DisplayName = provider.DisplayName
	probeCtx, cancel := context.WithTimeout(ctx, defaultLifecycleTimeout)
	defer cancel()
	probe := providers.NewPreStarter().
		PreStart(probeCtx, provider, &providers.ProbeEnv{ProviderName: name, HomePaths: r.m.homePaths})
	if probe.Item != nil {
		option.CanDelegate = false
		constraint := probe.Item.Message
		switch probe.Item.Code {
		case diagnosticcontract.CodeProviderCLIMissing:
			constraint = "Provider is not installed."
		case diagnosticcontract.CodeProviderNotAuthenticated, diagnosticcontract.CodeProviderCredentialUnresolved:
			constraint = "Provider is not authenticated."
		}
		option.Constraints = append(option.Constraints, constraint)
	}
	if r.m.modelCatalog != nil {
		models, err := r.m.modelCatalog.ListModels(ctx, modelcatalog.ListOptions{ProviderID: name})
		if err != nil {
			return option, err
		}
		for _, model := range models {
			item := SubagentModelOption{
				ID:               model.ModelID,
				Label:            model.DisplayName,
				ReasoningEfforts: []string{},
				Speeds:           []string{string(speedpkg.SpeedNormal)},
			}
			for _, effort := range model.ReasoningEfforts {
				item.ReasoningEfforts = append(item.ReasoningEfforts, string(effort))
			}
			for _, binding := range model.TransportBindings {
				if binding.Fast != nil && *binding.Fast {
					item.Speeds = append(item.Speeds, "fast")
					break
				}
			}
			option.Models = append(option.Models, item)
		}
	}
	if option.CanDelegate && len(option.Models) == 0 {
		option.Constraints = append(
			option.Constraints,
			"Model catalog unavailable; the agent default model will be used.",
		)
	}
	return option, nil
}

func validateSubagentModel(model, provider string, options []SubagentModelOption) error {
	if model == "" || len(options) == 0 {
		return nil
	}
	ids := make([]string, 0, len(options))
	for _, option := range options {
		if option.ID == model {
			return nil
		}
		ids = append(ids, option.ID)
	}
	return &SubagentError{
		Code: "model_unavailable",
		Message: fmt.Sprintf(
			"Model %s is not available on %s. Available: %s.",
			model, provider,
			strings.Join(ids[:min(10, len(ids))], ", "),
		),
		Err: ErrSubagentTargetUnavailable,
	}
}

// Capabilities previews must not restrict the full catalog used by Resolve.
func subagentCapabilityModels(option SubagentProviderOption, current, fallback string) SubagentProviderOption {
	const limit = 40
	option.ModelsTotal = len(option.Models)
	models := make([]SubagentModelOption, 0, min(limit, option.ModelsTotal))
	if option.CanDelegate {
		for _, preferred := range []string{current, fallback} {
			if preferred == "" || (len(models) > 0 && models[0].ID == preferred) {
				continue
			}
			for _, model := range option.Models {
				if model.ID == preferred {
					models = append(models, model)
					break
				}
			}
		}
		for _, model := range option.Models {
			if len(models) == limit {
				break
			}
			if model.ID != current && model.ID != fallback {
				models = append(models, model)
			}
		}
	}
	option.Models = models
	option.ModelsTruncated = len(models) < option.ModelsTotal
	return option
}
