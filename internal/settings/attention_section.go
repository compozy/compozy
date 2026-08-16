package settings

import (
	"slices"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

const attentionConfigRoot = "attention"

func buildAttentionSection(cfg *compozyconfig.Config) AttentionSection {
	return AttentionSection{Config: cloneAttentionConfig(cfg.Attention)}
}

func diffAttentionSettings(
	current compozyconfig.AttentionConfig,
	desired compozyconfig.AttentionConfig,
) []string {
	changed := make([]string, 0, 4)
	if current.Toasts != desired.Toasts {
		changed = append(changed, "attention.toasts")
	}
	if current.Sound != desired.Sound {
		changed = append(changed, "attention.sound")
	}
	if current.System != desired.System {
		changed = append(changed, "attention.system")
	}
	if !slices.Equal(current.MutedWorkspaces, desired.MutedWorkspaces) {
		changed = append(changed, "attention.muted_workspaces")
	}
	return changed
}

func applyAttentionSettings(
	editor *compozyconfig.OverlayEditor,
	settings compozyconfig.AttentionConfig,
) error {
	root := func(key string) []string { return []string{attentionConfigRoot, key} }
	return applyValueUpdates(editor, []struct {
		path  []string
		value any
	}{
		{path: root("toasts"), value: settings.Toasts},
		{path: root("sound"), value: settings.Sound},
		{path: root("system"), value: settings.System},
		{path: root("muted_workspaces"), value: cloneAttentionConfig(settings).MutedWorkspaces},
	})
}

func cloneAttentionConfig(cfg compozyconfig.AttentionConfig) compozyconfig.AttentionConfig {
	cfg.MutedWorkspaces = append([]string(nil), cfg.MutedWorkspaces...)
	return cfg
}
