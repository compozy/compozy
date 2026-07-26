package settings

import (
	"context"

	"fmt"

	"sort"

	aghconfig "github.com/compozy/agh/internal/config"

	skillspkg "github.com/compozy/agh/internal/skills"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (s *service) buildMemorySection(ctx context.Context, cfg *aghconfig.Config) (MemorySection, error) {
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
	cfg *aghconfig.Config,
	resolved *workspacepkg.ResolvedWorkspace,
	scope ScopeKind,
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
		return section, nil
	}

	var (
		skills []*skillspkg.Skill
		err    error
	)
	if scope == ScopeAgent {
		skills, err = s.skillsRuntime.ForAgent(ctx, resolved, agentName)
	} else {
		skills = s.skillsRuntime.List()
	}
	if err != nil {
		return SkillsSection{}, mapSkillsSettingsError(err)
	}
	section.RuntimeAvailable = true
	section.DiscoveredCount = len(skills)
	for _, skill := range skills {
		if skill != nil && !skill.Enabled {
			section.DisabledCount++
		}
	}
	if diagnosticsRuntime, ok := s.skillsRuntime.(SkillsDiagnosticsRuntime); ok {
		diagnostics, diagnosticsErr := diagnosticsRuntime.SkillDiagnostics(ctx, resolved, agentName)
		if diagnosticsErr != nil {
			return SkillsSection{}, mapSkillsSettingsError(diagnosticsErr)
		}
		section.Diagnostics = diagnostics
	}

	return section, nil
}

func (s *service) buildAutomationSection(
	ctx context.Context,
	cfg *aghconfig.Config,
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

func (s *service) buildNetworkSection(ctx context.Context, cfg *aghconfig.Config) (NetworkSection, error) {
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
	cfg *aghconfig.Config,
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
	cfg *aghconfig.Config,
) (HooksExtensionsSection, error) {
	hooks := buildHookItems(cfg.Hooks.Declarations)

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
