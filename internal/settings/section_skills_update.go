package settings

import (
	"context"
	"errors"
	"fmt"

	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/config/lifecycle"

	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func (s *service) updateSkillsSection(
	ctx context.Context,
	req SectionRequest,
	next aghconfig.SkillsConfig,
) (MutationResult, error) {
	scope, workspaceID, err := s.normalizeReadScope(req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: update section %q: %w", SectionSkills, err)
	}
	if scope == ScopeWorkspace {
		return MutationResult{}, conflictError(
			errors.New("settings: section \"skills\" does not support workspace scope"),
		)
	}
	agentName, err := normalizeAgentName(req.AgentName)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: update section %q: %w", SectionSkills, err)
	}

	cfg, resolved, err := s.loadConfig(ctx, scope, workspaceID)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: load section %q config: %w", SectionSkills, err)
	}

	if scope == ScopeAgent {
		if agentName == "" {
			return MutationResult{}, validationError(errors.New("settings: agent scope requires agent_name"))
		}
		return s.updateAgentSkillsSection(cfg.Skills, resolved, workspaceID, agentName, next)
	}

	target, err := aghconfig.ResolveConfigWriteTarget(s.homePaths, "", aghconfig.WriteScopeGlobal)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: resolve section %q write target: %w", SectionSkills, err)
	}

	current := cfg.Skills
	changed := diffSkillsSettings(current, next)
	if len(changed) == 0 {
		return MutationResult{
			Section:   SectionSkills,
			Scope:     ScopeGlobal,
			Behavior:  MutationBehaviorAppliedNow,
			Applied:   true,
			Warnings:  []string{sectionsNoChangesValue},
			Lifecycle: lifecycle.Live,
			DiffClass: lifecycle.DiffClassLive,
		}, nil
	}

	classification, err := ClassifyMutation(MutationDescriptor{Section: SectionSkills, ChangedFields: changed})
	if err != nil {
		return MutationResult{}, err
	}
	if classification.Behavior == MutationBehaviorAppliedNow && s.skillsRuntime == nil {
		return MutationResult{}, errors.New("settings: skills runtime is required to apply skills.disabled_skills")
	}

	if _, err := aghconfig.EditConfigOverlay(s.homePaths, "", target, func(editor *aghconfig.OverlayEditor) error {
		return applySkillsSettings(editor, next)
	}); err != nil {
		return MutationResult{}, fmt.Errorf("settings: write section %q: %w", SectionSkills, err)
	}

	if classification.Behavior == MutationBehaviorAppliedNow {
		if err := s.applySkillsDisabledChanges(current.DisabledSkills, next.DisabledSkills); err != nil {
			return MutationResult{}, err
		}
	}

	return MutationResult{
		Section:         SectionSkills,
		Scope:           ScopeGlobal,
		WriteTarget:     target.Kind(),
		Behavior:        classification.Behavior,
		Applied:         classification.Applied,
		RestartRequired: classification.RestartRequired,
		RestartScope:    classification.RestartScope,
		Lifecycle:       classification.Lifecycle,
		DiffClass:       classification.DiffClass,
	}, nil
}

func (s *service) updateAgentSkillsSection(
	base aghconfig.SkillsConfig,
	resolved *workspacepkg.ResolvedWorkspace,
	workspaceID string,
	agentName string,
	next aghconfig.SkillsConfig,
) (MutationResult, error) {
	agent, targetKind, err := s.resolveEffectiveAgent(resolved, agentName)
	if err != nil {
		return MutationResult{}, err
	}
	if immutable := diffAgentImmutableSkillsSettings(base, next); len(immutable) > 0 {
		return MutationResult{}, validationError(
			fmt.Errorf(
				"settings: agent scope only supports skills.disabled_skills, got %s",
				strings.Join(immutable, ", "),
			),
		)
	}

	current := base
	current.DisabledSkills = append([]string(nil), agent.Skills.Disabled...)
	changed := diffSkillsSettings(current, next)
	if len(changed) == 0 {
		return MutationResult{
			Section:     SectionSkills,
			Scope:       ScopeAgent,
			WriteTarget: targetKind,
			WorkspaceID: workspaceID,
			AgentName:   agentName,
			Behavior:    MutationBehaviorAppliedNow,
			Applied:     true,
			Warnings:    []string{sectionsNoChangesValue},
			Lifecycle:   lifecycle.Live,
			DiffClass:   lifecycle.DiffClassLive,
		}, nil
	}

	classification, err := ClassifyMutation(MutationDescriptor{Section: SectionSkills, ChangedFields: changed})
	if err != nil {
		return MutationResult{}, err
	}
	if classification.Behavior == MutationBehaviorAppliedNow {
		if err := s.applyAgentSkillsDisabledChanges(
			resolved,
			agentName,
			current.DisabledSkills,
			next.DisabledSkills,
		); err != nil {
			return MutationResult{}, err
		}
	}

	return MutationResult{
		Section:         SectionSkills,
		Scope:           ScopeAgent,
		WriteTarget:     targetKind,
		WorkspaceID:     workspaceID,
		AgentName:       agentName,
		Behavior:        classification.Behavior,
		Applied:         classification.Applied,
		RestartRequired: classification.RestartRequired,
		RestartScope:    classification.RestartScope,
		Lifecycle:       classification.Lifecycle,
		DiffClass:       classification.DiffClass,
	}, nil
}
