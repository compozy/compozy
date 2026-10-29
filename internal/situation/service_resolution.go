package situation

import (
	"context"

	"errors"

	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/compozy/compozy/internal/session"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (s *Service) resolveWorkspace(
	ctx context.Context,
	workspaceID string,
	rootDir string,
	provided *workspacepkg.ResolvedWorkspace,
) (*workspacepkg.ResolvedWorkspace, error) {
	if provided != nil {
		return new(*provided), nil
	}

	target := firstTrimmed(workspaceID, rootDir)
	if resolver := s.workspaceResolverValue(); resolver != nil && target != "" {
		resolved, err := resolver.Resolve(ctx, target)
		if err == nil {
			return &resolved, nil
		}
		if isContextError(err) {
			return nil, err
		}
	}

	if strings.TrimSpace(workspaceID) == "" && strings.TrimSpace(rootDir) == "" {
		return nil, nil
	}
	return &workspacepkg.ResolvedWorkspace{
		ID:      strings.TrimSpace(workspaceID),
		RootDir: strings.TrimSpace(rootDir),
	}, nil
}

func (s *Service) resolveAgent(
	name string,
	providerOverride string,
	provided compozyconfig.AgentDef,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
) (compozyconfig.ResolvedAgent, compozyconfig.AgentDef, error) {
	agent := provided
	agentName := firstTrimmed(name, agent.Name)
	if resolver := s.agentResolverValue(); resolver != nil && agentName != "" {
		resolved, err := resolver.ResolveAgent(agentName, workspaceSnapshot)
		switch {
		case err == nil:
			agent = resolved
		case isContextError(err):
			return compozyconfig.ResolvedAgent{}, compozyconfig.AgentDef{}, err
		case !errors.Is(err, workspacepkg.ErrAgentNotAvailable) && agent.Name == "":
			return compozyconfig.ResolvedAgent{}, compozyconfig.AgentDef{}, err
		}
	}
	if strings.TrimSpace(agent.Name) == "" {
		agent.Name = agentName
	}
	if agentName == "" &&
		strings.TrimSpace(providerOverride) == "" &&
		strings.TrimSpace(agent.Provider) == "" &&
		strings.TrimSpace(agent.Model) == "" {
		return compozyconfig.ResolvedAgent{}, agent, nil
	}

	if workspaceSnapshot != nil {
		defaults := workspaceSnapshot.Config.Defaults
		provider := firstTrimmed(providerOverride, agent.Provider, defaults.Provider)
		model := strings.TrimSpace(agent.Model)
		if model == "" && strings.TrimSpace(agent.Provider) == "" &&
			compozyconfig.CanonicalProviderName(provider) == compozyconfig.CanonicalProviderName(defaults.Provider) {
			// defaults.model belongs to defaults.provider only, as in session resolution.
			model = strings.TrimSpace(defaults.Model)
		}
		if provider != "" && model == "" {
			if providerConfig, err := workspaceSnapshot.Config.ResolveProvider(provider); err == nil {
				model = strings.TrimSpace(providerConfig.Models.Default)
			}
		}
		return compozyconfig.ResolvedAgent{
			Name:     strings.TrimSpace(agent.Name),
			Provider: provider,
			Model:    model,
		}, agent, nil
	}

	return compozyconfig.ResolvedAgent{
		Name:     strings.TrimSpace(agent.Name),
		Provider: firstTrimmed(providerOverride, agent.Provider),
		Model:    strings.TrimSpace(agent.Model),
	}, agent, nil
}

func (s *Service) capabilitiesSection(
	ctx context.Context,
	workspaceSnapshot *workspacepkg.ResolvedWorkspace,
	agent compozyconfig.AgentDef,
) (contract.AgentCapabilitySectionPayload, error) {
	capabilities := make([]contract.AgentCapabilityPayload, 0)
	if catalog := agent.Capabilities; catalog != nil {
		for _, capability := range catalog.Capabilities {
			id := strings.TrimSpace(capability.ID)
			if id == "" {
				continue
			}
			capabilities = append(capabilities, contract.AgentCapabilityPayload{
				ID:      id,
				Summary: strings.TrimSpace(capability.Summary),
				Source:  "agent",
			})
		}
	}

	if registry := s.skillRegistryValue(); registry != nil {
		var (
			skills []*skillspkg.Skill
			err    error
		)
		if strings.TrimSpace(agent.Name) != "" {
			skills, err = registry.ForAgent(ctx, workspaceSnapshot, agent.Name)
		} else {
			skills, err = registry.ForWorkspace(ctx, workspaceSnapshot)
		}
		if err != nil {
			if isContextError(err) {
				return contract.AgentCapabilitySectionPayload{}, err
			}
		} else {
			for _, skill := range skills {
				if skill == nil || !skill.Enabled {
					continue
				}
				name := strings.TrimSpace(skill.Meta.Name)
				if name == "" {
					continue
				}
				capabilities = append(capabilities, contract.AgentCapabilityPayload{
					ID:      "skill:" + name,
					Summary: strings.TrimSpace(skill.Meta.Description),
					Source:  "skill",
				})
			}
		}
	}

	slices.SortStableFunc(capabilities, func(left, right contract.AgentCapabilityPayload) int {
		if left.Source != right.Source {
			return strings.Compare(left.Source, right.Source)
		}
		return strings.Compare(left.ID, right.ID)
	})
	return contract.AgentCapabilitySectionPayload{
		Section:      sectionMeta(len(capabilities), s.limit()),
		Capabilities: boundedCapabilities(capabilities, s.limit()),
	}, nil
}

// limits reports the calling session's own effective spawn budget, the same values the
// spawn engine enforces for its governed children.
func (s *Service) limits(sessionType session.Type, budget store.SessionSpawnBudget) contract.AgentLimitsPayload {
	allowance := session.ChildSpawnAllowanceFor(sessionType, budget)
	return contract.AgentLimitsPayload{
		MaxChildren:           allowance.Budget.MaxChildren,
		MaxSpawnDepth:         allowance.Budget.MaxDepth,
		MaxActivePerWorkspace: allowance.Budget.MaxActivePerWorkspace,
		GovernedSpawnDepth:    allowance.GovernedDepth,
		CanSpawnChildren:      allowance.Allowed,
		SpawnBlockedReason:    allowance.BlockedReason,
		MaxActiveTaskLeases:   defaultMaxActiveTaskLeases,
		ContextSectionLimit:   s.limit(),
	}
}
