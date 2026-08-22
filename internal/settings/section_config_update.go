package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"
)

func (s *service) updateConfigBackedSection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	switch req.Section {
	case SectionGeneral:
		return s.updateGeneralSection(ctx, req)
	case SectionPersona:
		return s.updatePersonaSection(ctx, req)
	case SectionMemory:
		return s.updateMemorySection(ctx, req)
	case SectionRoles:
		return s.updateRolesSection(ctx, req)
	case SectionAutomation:
		return s.updateAutomationSection(ctx, req)
	case SectionNetwork:
		return s.updateNetworkSection(ctx, req)
	case SectionGateway:
		return s.updateGatewaySection(ctx, req)
	case SectionWindowManager:
		return s.updateWindowManagerSection(ctx, req)
	case SectionCmdPalette:
		return s.updateCmdPaletteSection(ctx, req)
	case SectionAttention:
		return s.updateAttentionSection(ctx, req)
	case SectionShell:
		return s.updateShellSection(ctx, req)
	case SectionObservability:
		return s.updateObservabilitySection(ctx, req)
	case SectionHooksExtensions:
		return s.updateHooksExtensionsSection(ctx, req)
	default:
		return MutationResult{}, notFoundError(fmt.Errorf("settings: unknown section %q", req.Section))
	}
}

func (s *service) updateRolesSection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	loaded, err := s.loadRolesSectionUpdate(
		ctx,
		req.Scope,
		req.WorkspaceID,
	)
	if err != nil {
		return MutationResult{}, err
	}
	if req.Roles == nil {
		return MutationResult{}, validationError(errors.New("settings: roles section payload is required"))
	}
	desired := compozyconfig.CloneRolesConfig(req.Roles)
	if err := desired.Validate("roles", &loaded.config); err != nil {
		return MutationResult{}, validationError(err)
	}
	changed := diffRolesSettings(&loaded.config.Roles, &desired)
	return s.updateScopedConfigSection(
		req.Section,
		changed,
		loaded.target,
		loaded.scope,
		loaded.workspaceID,
		loaded.workspaceRoot,
		func(editor *compozyconfig.OverlayEditor) error {
			return applyRolesSettings(editor, &desired)
		},
	)
}

func (s *service) updateGeneralSection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	cfg, target, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	if req.General == nil {
		return MutationResult{}, validationError(errors.New("settings: general section payload is required"))
	}
	desired := *req.General
	desired.Daemon.ReloadTimeouts = normalizeDaemonReloadTimeouts(desired.Daemon.ReloadTimeouts)
	changed := diffGeneralSettings(&cfg, desired)
	return s.updateConfigSection(req.Section, changed, target, func(editor *compozyconfig.OverlayEditor) error {
		return applyGeneralSettings(editor, desired)
	})
}

func (s *service) updateMemorySection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	cfg, target, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	if req.Memory == nil {
		return MutationResult{}, validationError(errors.New("settings: memory section payload is required"))
	}
	changed := diffMemorySettings(&cfg.Memory, req.Memory)
	return s.updateConfigSection(req.Section, changed, target, func(editor *compozyconfig.OverlayEditor) error {
		return applyMemorySettings(editor, req.Memory)
	})
}

func (s *service) updateAutomationSection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	cfg, target, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	if req.Automation == nil {
		return MutationResult{}, validationError(errors.New("settings: automation section payload is required"))
	}
	changed := diffAutomationSettings(&cfg, *req.Automation)
	return s.updateConfigSection(req.Section, changed, target, func(editor *compozyconfig.OverlayEditor) error {
		return applyAutomationSettings(editor, *req.Automation)
	})
}

func (s *service) updateNetworkSection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	cfg, target, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	if req.Network == nil {
		return MutationResult{}, validationError(errors.New("settings: network section payload is required"))
	}
	changed := diffNetworkSettings(cfg.Network, *req.Network)
	return s.updateConfigSection(req.Section, changed, target, func(editor *compozyconfig.OverlayEditor) error {
		return applyNetworkSettings(editor, *req.Network)
	})
}

func (s *service) updateGatewaySection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	cfg, target, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	if req.Gateway == nil {
		return MutationResult{}, validationError(errors.New("settings: gateway section payload is required"))
	}
	if err := req.Gateway.Validate(); err != nil {
		return MutationResult{}, validationError(err)
	}
	changed := diffGatewaySettings(cfg.Gateway, *req.Gateway)
	return s.updateConfigSection(req.Section, changed, target, func(editor *compozyconfig.OverlayEditor) error {
		return applyGatewaySettings(editor, *req.Gateway)
	})
}

func (s *service) updateAttentionSection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	cfg, target, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	if req.Attention == nil {
		return MutationResult{}, validationError(errors.New("settings: attention section payload is required"))
	}
	desired := cloneAttentionConfig(*req.Attention)
	if err := desired.Validate(); err != nil {
		return MutationResult{}, validationError(err)
	}
	changed := diffAttentionSettings(cfg.Attention, desired)
	return s.updateConfigSection(req.Section, changed, target, func(editor *compozyconfig.OverlayEditor) error {
		return applyAttentionSettings(editor, desired)
	})
}

func (s *service) updateShellSection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	cfg, target, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	if req.Shell == nil {
		return MutationResult{}, validationError(errors.New("settings: shell section payload is required"))
	}
	desired := *req.Shell
	if err := desired.Validate(); err != nil {
		return MutationResult{}, validationError(err)
	}
	changed := diffShellSettings(cfg.Shell, desired)
	return s.updateConfigSection(req.Section, changed, target, func(editor *compozyconfig.OverlayEditor) error {
		return applyShellSettings(editor, desired)
	})
}

func (s *service) updateObservabilitySection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	cfg, target, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	if req.Observability == nil {
		return MutationResult{}, validationError(
			errors.New("settings: observability section payload is required"),
		)
	}
	changed := diffObservabilitySettings(cfg.Observability, *req.Observability)
	return s.updateConfigSection(req.Section, changed, target, func(editor *compozyconfig.OverlayEditor) error {
		return applyObservabilitySettings(editor, *req.Observability)
	})
}

func (s *service) updateHooksExtensionsSection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	cfg, target, err := s.loadGlobalSectionUpdate(ctx, req.Section, req.Scope, req.WorkspaceID)
	if err != nil {
		return MutationResult{}, err
	}
	if req.HooksExtensions == nil {
		return MutationResult{}, validationError(
			errors.New("settings: hooks-extensions section payload is required"),
		)
	}
	changed := diffExtensionsSettings(cfg.Extensions, *req.HooksExtensions)
	return s.updateConfigSection(req.Section, changed, target, func(editor *compozyconfig.OverlayEditor) error {
		return applyExtensionsSettings(editor, *req.HooksExtensions)
	})
}

func (s *service) loadGlobalSectionUpdate(
	ctx context.Context,
	section SectionName,
	scope ScopeKind,
	workspaceID string,
) (compozyconfig.Config, compozyconfig.WriteTarget, error) {
	loaded, err := s.loadScopedSectionUpdate(ctx, section, scope, workspaceID, ScopeUser)
	if err != nil {
		return compozyconfig.Config{}, compozyconfig.WriteTarget{}, err
	}
	return loaded.config, loaded.target, nil
}

type scopedSectionUpdate struct {
	config        compozyconfig.Config
	target        compozyconfig.WriteTarget
	scope         ScopeKind
	workspaceID   string
	profileName   string
	workspaceRoot string
}

func (s *service) loadScopedSectionUpdate(
	ctx context.Context,
	section SectionName,
	scope ScopeKind,
	workspaceID string,
	allowedScopes ...ScopeKind,
) (scopedSectionUpdate, error) {
	return s.loadScopedSectionUpdateForProfile(
		ctx, section, scope, workspaceID, "", allowedScopes...,
	)
}

func (s *service) loadScopedSectionUpdateForProfile(
	ctx context.Context,
	section SectionName,
	scope ScopeKind,
	workspaceID string,
	profileName string,
	allowedScopes ...ScopeKind,
) (scopedSectionUpdate, error) {
	normalizedScope, normalizedWorkspaceID, err := s.normalizeReadScope(scope, workspaceID)
	if err != nil {
		return scopedSectionUpdate{}, fmt.Errorf("settings: update section %q: %w", section, err)
	}
	supported := slices.Contains(allowedScopes, normalizedScope)
	if !supported {
		if len(allowedScopes) == 1 && allowedScopes[0] == ScopeUser {
			return scopedSectionUpdate{}, conflictError(
				fmt.Errorf("settings: section %q does not support workspace scope", section),
			)
		}
		return scopedSectionUpdate{}, conflictError(
			fmt.Errorf("settings: section %q does not support %s scope", section, normalizedScope),
		)
	}

	cfg, resolved, err := s.loadConfig(ctx, normalizedScope, normalizedWorkspaceID, profileName)
	if err != nil {
		return scopedSectionUpdate{}, fmt.Errorf(
			"settings: load section %q config: %w",
			section,
			err,
		)
	}

	writeScope := compozyconfig.WriteScopeUser
	workspaceRoot := ""
	if resolved != nil {
		workspaceRoot = resolved.RootDir
	}
	if normalizedScope == ScopeWorkspace {
		if resolved == nil {
			return scopedSectionUpdate{}, errors.New("settings: resolved workspace is required for section update")
		}
		writeScope = compozyconfig.WriteScopeWorkspace
	}
	if normalizedScope == ScopeProfile {
		writeScope = compozyconfig.WriteScopeProfile
	}
	target, err := compozyconfig.ResolveConfigWriteTarget(s.homePaths, workspaceRoot, writeScope, profileName)
	if err != nil {
		return scopedSectionUpdate{}, fmt.Errorf(
			"settings: resolve section %q write target: %w",
			section,
			err,
		)
	}

	return scopedSectionUpdate{
		config:        cfg,
		target:        target,
		scope:         normalizedScope,
		workspaceID:   normalizedWorkspaceID,
		profileName:   strings.TrimSpace(profileName),
		workspaceRoot: workspaceRoot,
	}, nil
}

func (s *service) loadRolesSectionUpdate(
	ctx context.Context,
	scope ScopeKind,
	workspaceID string,
) (scopedSectionUpdate, error) {
	return s.loadScopedSectionUpdate(ctx, SectionRoles, scope, workspaceID, ScopeUser, ScopeWorkspace)
}

func (s *service) updateConfigSection(
	section SectionName,
	changed []string,
	target compozyconfig.WriteTarget,
	mutate func(*compozyconfig.OverlayEditor) error,
) (MutationResult, error) {
	return s.updateScopedConfigSection(section, changed, target, ScopeUser, "", "", mutate)
}

func (s *service) updateScopedConfigSection(
	section SectionName,
	changed []string,
	target compozyconfig.WriteTarget,
	scope ScopeKind,
	workspaceID string,
	workspaceRoot string,
	mutate func(*compozyconfig.OverlayEditor) error,
) (MutationResult, error) {
	return s.updateScopedConfigSectionForProfile(
		section, changed, target, scope, workspaceID, "", workspaceRoot, mutate,
	)
}

func (s *service) updateScopedConfigSectionForProfile(
	section SectionName,
	changed []string,
	target compozyconfig.WriteTarget,
	scope ScopeKind,
	workspaceID string,
	profileName string,
	workspaceRoot string,
	mutate func(*compozyconfig.OverlayEditor) error,
) (MutationResult, error) {
	if len(changed) == 0 {
		return MutationResult{
			Section:     section,
			Scope:       scope,
			WriteTarget: target.Kind(),
			WorkspaceID: workspaceID,
			ProfileName: strings.TrimSpace(profileName),
			Behavior:    MutationBehaviorAppliedNow,
			Applied:     true,
			Warnings:    []string{sectionsNoChangesValue},
			Lifecycle:   lifecycle.Live,
			DiffClass:   lifecycle.DiffClassForRoot(string(section)),
			writePath:   target.Path(),
		}, nil
	}

	classification, err := ClassifyMutation(MutationDescriptor{Section: section, ChangedFields: changed})
	if err != nil {
		return MutationResult{}, err
	}

	if _, err := compozyconfig.EditConfigOverlay(s.homePaths, workspaceRoot, target, mutate); err != nil {
		return MutationResult{}, fmt.Errorf("settings: write section %q: %w", section, err)
	}

	return MutationResult{
		Section:         section,
		Scope:           scope,
		WriteTarget:     target.Kind(),
		WorkspaceID:     workspaceID,
		ProfileName:     strings.TrimSpace(profileName),
		Behavior:        classification.Behavior,
		Applied:         classification.Applied,
		RestartRequired: classification.RestartRequired,
		RestartScope:    classification.RestartScope,
		Lifecycle:       classification.Lifecycle,
		DiffClass:       classification.DiffClass,
		writePath:       target.Path(),
	}, nil
}
