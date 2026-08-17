package settings

import compozyconfig "github.com/compozy/compozy/internal/config"

const shellConfigRoot = "shell"

func buildShellSection(cfg *compozyconfig.Config) ShellSection {
	return ShellSection{Config: cfg.Shell}
}

func diffShellSettings(current compozyconfig.ShellConfig, desired compozyconfig.ShellConfig) []string {
	changed := make([]string, 0, 2)
	if current.Sessions.Sort != desired.Sessions.Sort {
		changed = append(changed, "shell.sessions.sort")
	}
	if current.Sessions.Scope != desired.Sessions.Scope {
		changed = append(changed, "shell.sessions.scope")
	}
	return changed
}

func applyShellSettings(editor *compozyconfig.OverlayEditor, settings compozyconfig.ShellConfig) error {
	return applyValueUpdates(editor, []struct {
		path  []string
		value any
	}{
		{path: []string{shellConfigRoot, "sessions", "sort"}, value: string(settings.Sessions.Sort)},
		{path: []string{shellConfigRoot, "sessions", "scope"}, value: string(settings.Sessions.Scope)},
	})
}
