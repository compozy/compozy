package settings

import (
	"context"
	"errors"
	"fmt"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func (s *service) updateAttentionSection(
	ctx context.Context,
	req SectionUpdateRequest,
) (MutationResult, error) {
	s.attentionMu.Lock()
	defer s.attentionMu.Unlock()

	if req.Attention == nil {
		return MutationResult{}, validationError(errors.New("settings: attention section payload is required"))
	}
	scope, profileName, profileID, err := s.resolveAttentionProfile(ctx, req.Scope, req.ProfileName)
	if err != nil {
		return MutationResult{}, err
	}
	desired := *req.Attention
	cfg, target, err := s.loadGlobalSectionUpdate(ctx, req.Section, ScopeUser, "")
	if err != nil {
		return MutationResult{}, err
	}
	currentMutes, err := s.attentionWorkspaceMutes.ListAttentionWorkspaceMutes(ctx, profileID)
	if err != nil {
		return MutationResult{}, fmt.Errorf("settings: list attention workspace mutes: %w", err)
	}
	if !req.ReplaceAttentionWorkspaceMutes {
		desired.MutedWorkspaces = currentMutes
	}
	desired = normalizeAttentionSettings(desired)
	if err := desired.Validate(); err != nil {
		return MutationResult{}, validationError(err)
	}
	configChanges := diffAttentionRequest(cfg.Attention, desired)
	mutesChanged := attentionWorkspaceMutesChanged(currentMutes, desired.MutedWorkspaces)
	result, err := s.updateConfigSection(
		req.Section,
		configChanges,
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			return applyAttentionSettings(editor, desired)
		},
	)
	if err != nil {
		return MutationResult{}, err
	}
	if mutesChanged {
		if err := s.attentionWorkspaceMutes.ReplaceAttentionWorkspaceMutes(
			ctx,
			profileID,
			desired.MutedWorkspaces,
		); err != nil {
			rollbackErr := s.rollbackAttentionConfig(target, cfg.Attention, len(configChanges) > 0)
			return MutationResult{}, errors.Join(
				fmt.Errorf("settings: replace attention workspace mutes: %w", err),
				rollbackErr,
			)
		}
		if len(configChanges) == 0 {
			result.Warnings = nil
		}
	}
	result.Scope = scope
	result.ProfileName = profileName
	return result, nil
}

func (s *service) rollbackAttentionConfig(
	target compozyconfig.WriteTarget,
	previous compozyconfig.AttentionConfig,
	needed bool,
) error {
	if !needed {
		return nil
	}
	_, err := s.updateConfigSection(
		SectionAttention,
		[]string{"attention.toasts", "attention.sound", "attention.system"},
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			return applyAttentionSettings(editor, AttentionSettings{
				Toasts: previous.Toasts,
				Sound:  previous.Sound,
				System: previous.System,
			})
		},
	)
	if err != nil {
		return fmt.Errorf("settings: roll back attention delivery config: %w", err)
	}
	return nil
}
