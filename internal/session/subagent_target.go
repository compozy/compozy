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
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const subagentProviderUnavailable = "provider_unavailable"

func (r managerSubagentRuntime) Resolve(
	ctx context.Context,
	parent *Info,
	requested SubagentTarget,
) (SubagentTarget, error) {
	resolver, err := r.m.requireWorkspaceResolver()
	if err != nil {
		return SubagentTarget{}, err
	}
	workspace, err := resolver.Resolve(ctx, parent.WorkspaceID)
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

func (r managerSubagentRuntime) Capabilities(
	ctx context.Context,
	parent *Info,
) ([]SubagentAgentOption, []SubagentProviderOption, error) {
	resolver, err := r.m.requireWorkspaceResolver()
	if err != nil {
		return nil, nil, err
	}
	workspace, err := resolver.Resolve(ctx, parent.WorkspaceID)
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
		options = append(options, option)
		byName[name] = option
	}
	agents := make([]SubagentAgentOption, 0, len(workspace.Agents))
	for _, agent := range workspace.Agents {
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
				Speeds:           []string{"normal"},
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
