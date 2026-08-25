package settings

import (
	"context"

	"fmt"

	"sort"

	compozyconfig "github.com/compozy/compozy/internal/config"

	skillspkg "github.com/compozy/compozy/internal/skills"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (s *service) buildMemorySection(ctx context.Context, cfg *compozyconfig.Config) (MemorySection, error) {
	health := MemoryHealthStatus{}
	if s.memoryRuntime != nil {
		status, err := s.memoryRuntime.MemoryHealthStatus(ctx)
		if err != nil {
			return MemorySection{}, fmt.Errorf("settings: memory health: %w", err)
		}
		health = status
	}

	return MemorySection{
		Config: cfg.Memory,
		Health: health,
		Actions: MemoryActions{
			Consolidate: ActionMetadata{
				Name:      sectionsConsolidateKey,
				Available: s.consolidateActionAvailable,
				Behavior:  MutationBehaviorActionTrigger,
			},
		},
	}, nil
}

func (s *service) buildSkillsSection(
	ctx context.Context,
	cfg *compozyconfig.Config,
	resolved *workspacepkg.ResolvedWorkspace,
	scope ScopeKind,
	profileName string,
	agentName string,
) (SkillsSection, error) {
	section := SkillsSection{
		Config: cfg.Skills,
	}
	section.Links = buildSkillsOperationalLinks()

	if scope == ScopeAgent {
		agent, _, err := s.resolveEffectiveAgent(resolved, agentName)
		if err != nil {
			return SkillsSection{}, err
		}
		section.Config.DisabledSkills = append([]string(nil), agent.Skills.Disabled...)
	}

	if s.skillsRuntime == nil {
		section.DisabledCount = len(section.Config.DisabledSkills)
		var sourceErr error
		section.Sources, section.Inherits, sourceErr = s.buildSkillSourceReadModel(
			section.Config, resolved, scope, nil, false,
		)
		if sourceErr != nil {
			return SkillsSection{}, sourceErr
		}
		return section, nil
	}
	projection, err := s.skillProjectionWorkspace(ctx, cfg, resolved, scope, profileName)
	if err != nil {
		return SkillsSection{}, err
	}

	var (
		skills  []*skillspkg.Skill
		loadErr error
	)
	if scope == ScopeAgent {
		skills, loadErr = s.skillsRuntime.ForAgent(ctx, projection, agentName)
	} else if projection != nil {
		if workspaceRuntime, ok := s.skillsRuntime.(SkillsWorkspaceRuntime); ok {
			skills, loadErr = workspaceRuntime.ForWorkspace(ctx, projection)
		} else {
			skills = s.skillsRuntime.List()
		}
	} else {
		skills = s.skillsRuntime.List()
	}
	if loadErr != nil {
		return SkillsSection{}, mapSkillsSettingsError(loadErr)
	}
	section.RuntimeAvailable = true
	section.DiscoveredCount = len(skills)
	for _, skill := range skills {
		if skill != nil && !skill.Enabled {
			section.DisabledCount++
		}
	}
	if diagnosticsRuntime, ok := s.skillsRuntime.(SkillsDiagnosticsRuntime); ok {
		diagnostics, diagnosticsErr := diagnosticsRuntime.SkillDiagnostics(ctx, projection, agentName)
		if diagnosticsErr != nil {
			return SkillsSection{}, mapSkillsSettingsError(diagnosticsErr)
		}
		section.Diagnostics = diagnostics
	}
	var statuses []skillspkg.SkillSourceRootStatus
	if sourcesRuntime, ok := s.skillsRuntime.(SkillsSourcesRuntime); ok {
		statuses, err = sourcesRuntime.SkillSourceRoots(ctx, projection)
		if err != nil {
			return SkillsSection{}, mapSkillsSettingsError(err)
		}
	}
	section.Sources, section.Inherits, err = s.buildSkillSourceReadModel(
		section.Config, projection, scope, statuses, true,
	)
	if err != nil {
		return SkillsSection{}, err
	}

	return section, nil
}

func (s *service) buildAutomationSection(
	ctx context.Context,
	cfg *compozyconfig.Config,
) (AutomationSection, error) {
	runtime := AutomationRuntimeStatus{}
	if s.automationRuntime != nil {
		status, err := s.automationRuntime.AutomationRuntimeStatus(ctx)
		if err != nil {
			return AutomationSection{}, fmt.Errorf("settings: automation runtime: %w", err)
		}
		runtime = status
	}

	return AutomationSection{
		Config: AutomationSettings{
			Enabled:           cfg.Automation.Enabled,
			Timezone:          cfg.Automation.Timezone,
			MaxConcurrentJobs: cfg.Automation.MaxConcurrentJobs,
			DefaultFireLimit:  cfg.Automation.DefaultFireLimit,
		},
		Runtime: runtime,
		Links: []OperationalLink{
			{Label: string(SectionAutomation), Path: "/automation"},
		},
	}, nil
}

func (s *service) buildNetworkSection(ctx context.Context, cfg *compozyconfig.Config) (NetworkSection, error) {
	runtime := NetworkRuntimeStatus{}
	if s.networkRuntime != nil {
		status, err := s.networkRuntime.NetworkRuntimeStatus(ctx)
		if err != nil {
			return NetworkSection{}, fmt.Errorf("settings: network runtime: %w", err)
		}
		runtime = status
	}

	return NetworkSection{
		Config:  cfg.Network,
		Runtime: runtime,
		Links: []OperationalLink{
			{Label: string(SectionNetwork), Path: "/network"},
		},
	}, nil
}

func (s *service) buildObservabilitySection(
	ctx context.Context,
	cfg *compozyconfig.Config,
) (ObservabilitySection, error) {
	runtime := ObservabilityRuntimeStatus{}
	if s.observabilityRuntime != nil {
		status, err := s.observabilityRuntime.ObservabilityRuntimeStatus(ctx)
		if err != nil {
			return ObservabilitySection{}, fmt.Errorf("settings: observability runtime: %w", err)
		}
		runtime = status
	}

	return ObservabilitySection{
		Config:         cfg.Observability,
		Runtime:        runtime,
		LogTailSupport: CapabilityStatus{Available: s.logTailAvailable},
	}, nil
}

func (s *service) buildHooksExtensionsSection(
	ctx context.Context,
	cfg *compozyconfig.Config,
) (HooksExtensionsSection, error) {
	hooks, err := s.buildHookItems(ScopeUser, "", "", "")
	if err != nil {
		return HooksExtensionsSection{}, err
	}

	installed := []InstalledExtension{}
	if s.extensions != nil {
		values, err := s.extensions.InstalledExtensions(ctx)
		if err != nil {
			return HooksExtensionsSection{}, fmt.Errorf("settings: installed extensions: %w", err)
		}
		installed = append(installed, values...)
		sort.Slice(installed, func(i, j int) bool {
			return installed[i].Name < installed[j].Name
		})
	}

	parity := TransportParityStatus{}
	if s.transportParity != nil {
		status, err := s.transportParity.TransportParityStatus(ctx)
		if err != nil {
			return HooksExtensionsSection{}, fmt.Errorf("settings: transport parity status: %w", err)
		}
		parity = status
	}

	return HooksExtensionsSection{
		Hooks:           hooks,
		Extensions:      cloneExtensionsConfig(cfg.Extensions),
		Installed:       installed,
		TransportParity: parity,
	}, nil
}
