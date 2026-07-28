package situation

import (
	"context"

	"errors"

	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"

	skillspkg "github.com/compozy/compozy/internal/skills"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (s *Service) resolveWorkspace(
	ctx context.Context,
	workspaceID string,
	rootDir string,
	provided *workspacepkg.ResolvedWorkspace,
) (*workspacepkg.ResolvedWorkspace, error) {
	if provided != nil {
		clone := *provided
		return &clone, nil
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
		Workspace: workspacepkg.Workspace{
			ID:      strings.TrimSpace(workspaceID),
			RootDir: strings.TrimSpace(rootDir),
		},
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
		provider := firstTrimmed(providerOverride, agent.Provider, workspaceSnapshot.Config.Defaults.Provider)
		model := strings.TrimSpace(agent.Model)
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

func (s *Service) limits(ctx context.Context, workspaceID string) (contract.AgentLimitsPayload, error) {
	limits := contract.AgentLimitsPayload{
		MaxChildren:         compozyconfig.DefaultCoordinatorMaxChildren,
		MaxSpawnDepth:       defaultMaxSpawnDepth,
		MaxActiveTaskLeases: defaultMaxActiveTaskLeases,
		ContextSectionLimit: s.limit(),
	}
	if resolver := s.coordinatorRoleValue(); resolver != nil {
		cfg, err := resolver.ResolveCoordinatorRole(ctx, strings.TrimSpace(workspaceID))
		if err != nil && isContextError(err) {
			return contract.AgentLimitsPayload{}, err
		}
		if err == nil && cfg.MaxChildren > 0 {
			limits.MaxChildren = cfg.MaxChildren
		}
	}
	return limits, nil
}
