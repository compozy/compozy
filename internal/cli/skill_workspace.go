package cli

import (
	"context"

	"errors"
	"fmt"

	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/skills"
	skillbundled "github.com/compozy/compozy/skills"
	"github.com/spf13/cobra"
)

const (
	agentLocalSkillSource = "agent-local"
)

const (
	additionalSkillSource  = "additional"
	bundledSkillSource     = "bundled"
	marketplaceSkillSource = "marketplace"
	userSkillSource        = "user"
)

type skillCommandScope struct {
	query     SkillQuery
	useDaemon bool
}

func loadSkillCommandContext(ctx context.Context, deps commandDeps, agentName string) (skillCommandContext, error) {
	runtime, err := loadRuntimeContext(deps)
	if err != nil {
		return skillCommandContext{}, fmt.Errorf("cli: load skill runtime context: %w", err)
	}

	registry := skills.NewRegistry(skills.RegistryConfig{
		BundledFS:      skillbundled.FS(),
		UserAgentsDir:  runtime.HomePaths.AgentsDir,
		UserSkillsDir:  runtime.HomePaths.SkillsDir,
		DisabledSkills: append([]string(nil), runtime.Config.Skills.DisabledSkills...),
	})
	if err := registry.LoadAll(ctx); err != nil {
		return skillCommandContext{}, fmt.Errorf("cli: load skill registry: %w", err)
	}

	var skillList []*skills.Skill
	if strings.TrimSpace(agentName) != "" {
		skillList, err = registry.ForAgent(ctx, nil, agentName)
		if err != nil {
			return skillCommandContext{}, fmt.Errorf("cli: load agent skills: %w", err)
		}
	} else {
		skillList, err = registry.ForWorkspace(ctx, nil)
		if err != nil {
			return skillCommandContext{}, fmt.Errorf("cli: load global skills: %w", err)
		}
	}

	return skillCommandContext{
		bundledFS: skillbundled.FS(),
		registry:  registry,
		skills:    skillList,
	}, nil
}

func resolveSkillCommandScope(
	ctx context.Context,
	cmd *cobra.Command,
	deps commandDeps,
) (skillCommandScope, error) {
	workspaceRef, err := commandWorkspaceFlag(cmd)
	if err != nil {
		return skillCommandScope{}, err
	}
	agentRef, err := commandSkillAgentFlag(cmd)
	if err != nil {
		return skillCommandScope{}, err
	}

	client, err := clientFromDeps(deps)
	if err != nil {
		if workspaceRef != "" || strings.TrimSpace(deps.getenv(workspaceEnvName)) != "" {
			return skillCommandScope{}, err
		}
		return skillCommandScope{query: SkillQuery{ForAgent: agentRef}}, nil
	}
	resolution, resolved, err := resolveContextualCommandWorkspace(ctx, cmd, deps, client, workspaceRef)
	if err != nil {
		if workspaceRef != "" || strings.TrimSpace(deps.getenv(workspaceEnvName)) != "" {
			return skillCommandScope{}, err
		}
		return skillCommandScope{query: SkillQuery{ForAgent: agentRef}}, nil
	}

	scope := skillCommandScope{
		query: SkillQuery{
			ForAgent: agentRef,
		},
		useDaemon: resolved,
	}
	if resolved {
		scope.query.Workspace = resolution.ID
		if scope.query.ForAgent == "" && resolution.Source == workspaceResolutionSessionIdentity {
			scope.query.ForAgent = resolution.IdentityAgent
		}
	}
	return scope, nil
}

func skillShadowsRecordFromDomain(snapshot skills.SkillShadows) SkillShadowsRecord {
	return SkillShadowsRecord{
		Name:    strings.TrimSpace(snapshot.Name),
		Winner:  skillShadowEntryRecordFromDomain(snapshot.Winner),
		Shadows: skillShadowEntryRecordsFromDomain(snapshot.Shadows),
	}
}

func skillShadowEntryRecordsFromDomain(entries []skills.ShadowEntry) []contract.SkillShadowEntryPayload {
	if len(entries) == 0 {
		return nil
	}
	records := make([]contract.SkillShadowEntryPayload, 0, len(entries))
	for _, entry := range entries {
		records = append(records, skillShadowEntryRecordFromDomain(entry))
	}
	return records
}

func skillShadowEntryRecordFromDomain(entry skills.ShadowEntry) contract.SkillShadowEntryPayload {
	return contract.SkillShadowEntryPayload{
		Path:             strings.TrimSpace(entry.Path),
		Tier:             strings.TrimSpace(entry.Tier),
		ResolvedToWinner: entry.ResolvedToWinner,
		DetectedAt:       entry.DetectedAt.UTC(),
	}
}

func skillProvenanceRecordFromSkill(skill *skills.Skill, shadows skills.SkillShadows) SkillProvenanceRecord {
	provenance := SkillProvenanceRecord{
		InstalledFromBundle:    strings.TrimSpace(skill.InstalledFromBundle),
		InstalledFromExtension: strings.TrimSpace(skill.InstalledFromExtension),
		PrecedenceTier:         skills.SkillPrecedenceTierName(skill.Source),
	}
	if skill.Provenance != nil {
		provenance.Slug = strings.TrimSpace(skill.Provenance.Slug)
		provenance.Registry = strings.TrimSpace(skill.Provenance.Registry)
		provenance.Version = strings.TrimSpace(skill.Provenance.Version)
		if !skill.Provenance.InstalledAt.IsZero() {
			installedAt := skill.Provenance.InstalledAt.UTC()
			provenance.InstalledAt = &installedAt
		}
	}
	if len(shadows.Shadows) > 1 {
		provenance.ShadowedBy = skillShadowEntryRecordsFromDomain(shadows.Shadows[1:])
	}
	return provenance
}

func commandWorkspaceFlag(cmd *cobra.Command) (string, error) {
	workspace, err := cmd.Flags().GetString("workspace")
	if err != nil {
		return "", fmt.Errorf("read workspace flag: %w", err)
	}
	trimmed := strings.TrimSpace(workspace)
	if cmd.Flags().Changed("workspace") && trimmed == "" {
		return "", fmt.Errorf("workspace flag cannot be empty")
	}
	return trimmed, nil
}

func commandSkillAgentFlag(cmd *cobra.Command) (string, error) {
	agentName, err := cmd.Flags().GetString("for-agent")
	if err != nil {
		return "", fmt.Errorf("read for-agent flag: %w", err)
	}
	trimmed := strings.TrimSpace(agentName)
	if cmd.Flags().Changed("for-agent") && trimmed == "" {
		return "", fmt.Errorf("for-agent flag cannot be empty")
	}
	if trimmed != "" {
		if err := compozyconfig.ValidateAgentName(trimmed); err != nil {
			return "", err
		}
	}
	return trimmed, nil
}

func normalizeSkillSourceFilter(sourceFilter string) (string, error) {
	filter := strings.ToLower(strings.TrimSpace(sourceFilter))
	switch filter {
	case "":
		return "", nil
	case bundledSkillSource,
		marketplaceSkillSource,
		userSkillSource,
		additionalSkillSource,
		workspaceSkillSource,
		agentLocalSkillSource:
		return filter, nil
	default:
		return "", fmt.Errorf("cli: invalid skill source %q", sourceFilter)
	}
}

func findSkillByName(allSkills []*skills.Skill, name string) (*skills.Skill, error) {
	skillName := strings.TrimSpace(name)
	if skillName == "" {
		return nil, errors.New("skill name is required")
	}

	for _, skill := range allSkills {
		if skill == nil {
			continue
		}
		if skill.Meta.Name == skillName {
			return skill, nil
		}
	}

	return nil, fmt.Errorf("skill %q not found", skillName)
}
