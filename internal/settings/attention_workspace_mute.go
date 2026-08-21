package settings

import (
	"context"
	"errors"
	"fmt"
	"slices"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/config/lifecycle"
)

// SetAttentionWorkspaceMuted atomically changes one workspace mute without replacing concurrent settings.
func (s *service) SetAttentionWorkspaceMuted(
	ctx context.Context,
	workspaceID string,
	muted bool,
) (bool, error) {
	identity := compozyconfig.AttentionConfig{MutedWorkspaces: []string{workspaceID}}
	if err := identity.Validate(); err != nil {
		return false, fmt.Errorf("settings: validate attention workspace registration id: %w", err)
	}
	normalizedID := workspaceID

	s.applyMu.Lock()
	defer s.applyMu.Unlock()

	cfg, target, err := s.loadGlobalSectionUpdate(ctx, SectionAttention, ScopeUser, "")
	if err != nil {
		return false, err
	}
	desired := cloneAttentionConfig(cfg.Attention)
	present := slices.Contains(desired.MutedWorkspaces, normalizedID)
	if present == muted {
		return false, nil
	}
	if muted {
		desired.MutedWorkspaces = append(desired.MutedWorkspaces, normalizedID)
	} else {
		desired.MutedWorkspaces = slices.DeleteFunc(
			desired.MutedWorkspaces,
			func(candidate string) bool { return candidate == normalizedID },
		)
	}
	if err := desired.Validate(); err != nil {
		return false, validationError(err)
	}

	result, err := s.updateConfigSection(
		SectionAttention,
		[]string{"attention.muted_workspaces"},
		target,
		func(editor *compozyconfig.OverlayEditor) error {
			return applyAttentionSettings(editor, desired)
		},
	)
	if err != nil {
		return false, err
	}
	result, err = s.finalizeSectionUpdate(ctx, result, nil)
	if err != nil {
		return true, err
	}
	applyResult, err := s.recordAttentionSectionApply(ctx, result)
	if err != nil {
		return true, err
	}
	if !applyResult.Applied {
		return true, errors.New("settings: attention workspace mute change was not applied live")
	}
	if applyResult.Record.Lifecycle != lifecycle.Live {
		return true, fmt.Errorf(
			"settings: attention workspace mute lifecycle = %q, want %q",
			applyResult.Record.Lifecycle,
			lifecycle.Live,
		)
	}
	return true, nil
}
