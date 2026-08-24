package settings

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

const (
	attentionConfigRoot          = "attention"
	attentionMutedWorkspacesPath = "muted_workspaces"
)

// AttentionSettings combines user-wide delivery channels with one profile's workspace mutes.
type AttentionSettings struct {
	Toasts          bool
	Sound           bool
	System          bool
	MutedWorkspaces []string
}

func (s AttentionSettings) Validate() error {
	seen := make(map[string]struct{}, len(s.MutedWorkspaces))
	for index, workspaceID := range s.MutedWorkspaces {
		if !workspacepkg.IsRegistrationID(workspaceID) {
			return compozyconfig.ValidationError{
				Code:    "attention_workspace_mute_invalid",
				Path:    attentionMutedWorkspacesPath,
				Message: fmt.Sprintf("entry %d must be a public workspace registration id", index),
			}
		}
		if _, exists := seen[workspaceID]; exists {
			return compozyconfig.ValidationError{
				Code:    "attention_workspace_mute_invalid",
				Path:    attentionMutedWorkspacesPath,
				Message: fmt.Sprintf("entry %d must not duplicate a workspace", index),
			}
		}
		seen[workspaceID] = struct{}{}
	}
	return nil
}

func normalizeAttentionSettings(settings AttentionSettings) AttentionSettings {
	settings.MutedWorkspaces = append([]string(nil), settings.MutedWorkspaces...)
	for index := range settings.MutedWorkspaces {
		settings.MutedWorkspaces[index] = strings.TrimSpace(settings.MutedWorkspaces[index])
	}
	sort.Strings(settings.MutedWorkspaces)
	return settings
}

func buildAttentionSection(cfg *compozyconfig.Config, mutedWorkspaces []string) AttentionSection {
	return AttentionSection{Config: AttentionSettings{
		Toasts:          cfg.Attention.Toasts,
		Sound:           cfg.Attention.Sound,
		System:          cfg.Attention.System,
		MutedWorkspaces: append([]string(nil), mutedWorkspaces...),
	}}
}

func diffAttentionSettings(
	current compozyconfig.AttentionConfig,
	desired compozyconfig.AttentionConfig,
) []string {
	changed := make([]string, 0, 3)
	if current.Toasts != desired.Toasts {
		changed = append(changed, "attention.toasts")
	}
	if current.Sound != desired.Sound {
		changed = append(changed, "attention.sound")
	}
	if current.System != desired.System {
		changed = append(changed, "attention.system")
	}
	return changed
}

func diffAttentionRequest(current compozyconfig.AttentionConfig, desired AttentionSettings) []string {
	return diffAttentionSettings(current, compozyconfig.AttentionConfig{
		Toasts: desired.Toasts,
		Sound:  desired.Sound,
		System: desired.System,
	})
}

func attentionWorkspaceMutesChanged(current []string, desired []string) bool {
	return !slices.Equal(current, desired)
}

func applyAttentionSettings(
	editor *compozyconfig.OverlayEditor,
	settings AttentionSettings,
) error {
	root := func(key string) []string { return []string{attentionConfigRoot, key} }
	return applyValueUpdates(editor, []struct {
		path  []string
		value any
	}{
		{path: root("toasts"), value: settings.Toasts},
		{path: root("sound"), value: settings.Sound},
		{path: root("system"), value: settings.System},
	})
}

func cloneAttentionConfig(cfg compozyconfig.AttentionConfig) compozyconfig.AttentionConfig {
	return cfg
}
