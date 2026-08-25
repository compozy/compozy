package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"

	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func (s *service) updateSkillsSection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	scope, workspaceID, err := s.normalizeReadScope(req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: update section %q: %w", SectionSkills, err)
	}
	agentName, err := normalizeAgentName(req.AgentName)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: update section %q: %w", SectionSkills, err)
	}

	cfg, resolved, err := s.loadConfig(ctx, scope, workspaceID, req.ProfileName)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: load section %q config: %w", SectionSkills, err)
	}

	if scope == ScopeAgent {
		if req.Skills == nil {
			return MutationResult{}, validationError(errors.New("settings: skills config is required at agent scope"))
		}
		if agentName == "" {
			return MutationResult{}, validationError(errors.New("settings: agent scope requires agent_name"))
		}
		return s.updateAgentSkillsSection(cfg.Skills, resolved, workspaceID, agentName, *req.Skills)
	}
	if scope == ScopeWorkspace {
		if req.SkillSourcesOverride == nil {
			return MutationResult{}, validationError(errors.New("settings: skills override is required at workspace scope"))
		}
		return s.updateWorkspaceSkillSources(workspaceID, cfg.Skills, resolved, *req.SkillSourcesOverride)
	}
	if req.Skills == nil {
		return MutationResult{}, validationError(errors.New("settings: skills config is required"))
	}
	next := *req.Skills
	profileName, err := normalizeSettingsProfileName(scope, req.ProfileName)
	if err != nil {
		return MutationResult{}, err
	}
	if err := next.ValidateForScope(scope.configWriteScope()); err != nil {
		return MutationResult{}, validationError(err)
	}

	target, err := compozyconfig.ResolveConfigWriteTarget(s.homePaths, "", scope.configWriteScope(), profileName)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: resolve section %q write target: %w", SectionSkills, err)
	}

	current := cfg.Skills
	changed := diffSkillsSettings(current, next)
	if len(changed) == 0 {
		return MutationResult{
			Section:     SectionSkills,
			Scope:       scope,
			ProfileName: profileName,
			Behavior:    MutationBehaviorAppliedNow,
			Applied:     true,
			Warnings:    []string{sectionsNoChangesValue},
			Lifecycle:   lifecycle.Live,
			DiffClass:   lifecycle.DiffClassLive,
			writePath:   target.Path(),
		}, nil
	}

	classification, err := ClassifyMutation(MutationDescriptor{Section: SectionSkills, ChangedFields: changed})
	if err != nil {
		return MutationResult{}, err
	}
	if slicesChanged(current.DisabledSkills, next.DisabledSkills) && s.skillsRuntime == nil {
		return MutationResult{}, errors.New("settings: skills runtime is required to apply skills.disabled_skills")
	}

	if err := s.writeSkillsConfig(target, next); err != nil {
		return MutationResult{}, fmt.Errorf("settings: write section %q: %w", SectionSkills, err)
	}

	if slicesChanged(current.DisabledSkills, next.DisabledSkills) {
		if err := s.applySkillsDisabledChanges(current.DisabledSkills, next.DisabledSkills); err != nil {
			return MutationResult{}, err
		}
	}

	return MutationResult{
		Section:         SectionSkills,
		Scope:           scope,
		WriteTarget:     target.Kind(),
		ProfileName:     profileName,
		Behavior:        classification.Behavior,
		Applied:         classification.Applied,
		RestartRequired: classification.RestartRequired,
		RestartScope:    classification.RestartScope,
		Lifecycle:       classification.Lifecycle,
		DiffClass:       classification.DiffClass,
		writePath:       target.Path(),
	}, nil
}

func (s *service) writeSkillsConfig(
	target compozyconfig.WriteTarget,
	next compozyconfig.SkillsConfig,
) error {
	_, err := compozyconfig.EditConfigOverlay(
		s.homePaths,
		"",
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			return applySkillsSettings(editor, next)
		},
	)
	return err
}

func (s *service) updateWorkspaceSkillSources(
	workspaceID string,
	current compozyconfig.SkillsConfig,
	resolved *workspacepkg.ResolvedWorkspace,
	override SkillSourcesOverride,
) (MutationResult, error) {
	if resolved == nil {
		return MutationResult{}, errors.New("settings: resolved workspace is required for skills update")
	}
	if !override.Sources.Present && !override.CustomSources.Present {
		return MutationResult{}, validationError(errors.New("settings: skills override must include sources or custom_sources"))
	}

	candidate := current
	changed := make([]string, 0, 2)
	if override.Sources.Present {
		changed = append(changed, "skills.sources")
		if !override.Sources.Null {
			candidate.Sources = append([]string(nil), override.Sources.Value...)
		}
	}
	if override.CustomSources.Present {
		changed = append(changed, "skills.custom_sources")
		if !override.CustomSources.Null {
			candidate.CustomSources = append([]string(nil), override.CustomSources.Value...)
		}
	}
	if err := candidate.ValidateForScope(compozyconfig.WriteScopeWorkspace); err != nil {
		return MutationResult{}, validationError(err)
	}

	target, err := compozyconfig.ResolveConfigWriteTarget(
		s.homePaths,
		resolved.RootDir,
		compozyconfig.WriteScopeWorkspace,
		"",
	)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: resolve section %q write target: %w", SectionSkills, err)
	}
	classification, err := ClassifyMutation(MutationDescriptor{Section: SectionSkills, ChangedFields: changed})
	if err != nil {
		return MutationResult{}, err
	}
	_, err = compozyconfig.EditConfigOverlay(
		s.homePaths,
		resolved.RootDir,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			if err := applyOptionalSkillSourceOverride(editor, "sources", override.Sources); err != nil {
				return err
			}
			return applyOptionalSkillSourceOverride(editor, "custom_sources", override.CustomSources)
		},
	)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: write section %q: %w", SectionSkills, err)
	}
	return MutationResult{
		Section: SectionSkills, Scope: ScopeWorkspace, WriteTarget: target.Kind(), WorkspaceID: workspaceID,
		Behavior: classification.Behavior, Applied: classification.Applied,
		RestartRequired: classification.RestartRequired, RestartScope: classification.RestartScope,
		Lifecycle: classification.Lifecycle, DiffClass: classification.DiffClass, writePath: target.Path(),
	}, nil
}

func applyOptionalSkillSourceOverride(
	editor *compozyconfig.OverlayEditor,
	field string,
	value OptionalStringList,
) error {
	if !value.Present {
		return nil
	}
	path := []string{string(SectionSkills), field}
	if value.Null {
		return editor.Delete(path)
	}
	return editor.SetValue(path, append([]string(nil), value.Value...))
}

func slicesChanged(current []string, next []string) bool {
	return !slices.Equal(current, next)
}

func (s *service) updateAgentSkillsSection(
	base compozyconfig.SkillsConfig,
	resolved *workspacepkg.ResolvedWorkspace,
	workspaceID string,
	agentName string,
	next compozyconfig.SkillsConfig,
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
			writePath:   agent.SourcePath,
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
		writePath:       agent.SourcePath,
	}, nil
}
