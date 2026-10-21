package settings

import compozyconfig "github.com/compozy/compozy/internal/config"

const shellConfigRoot = "shell"

func buildShellSection(cfg *compozyconfig.Config) ShellSection {
	warnings := []string{}
	if cfg.Shell.Sessions.ScopeExplicit {
		warnings = append(warnings, compozyconfig.ShellScopeDeprecationWarning)
	}
	return ShellSection{Config: cfg.Shell, Warnings: warnings}
}

func diffShellSettings(current compozyconfig.ShellConfig, desired compozyconfig.ShellConfig) []string {
	changed := make([]string, 0, 3)
	if current.Sessions.GroupBy != desired.Sessions.GroupBy || current.Sessions.GroupByExplicit != desired.Sessions.GroupByExplicit {
		changed = append(changed, "shell.sessions.group_by")
	}
	if current.Sessions.Sort != desired.Sessions.Sort {
		changed = append(changed, "shell.sessions.sort")
	}
	if desired.Sessions.ScopeExplicit || current.Sessions.Scope != desired.Sessions.Scope {
		changed = append(changed, "shell.sessions.scope")
	}
	return changed
}

func applyShellSettings(editor *compozyconfig.OverlayEditor, settings compozyconfig.ShellConfig) error {
	if err := editor.SetValue([]string{shellConfigRoot, "sessions", "sort"}, string(settings.Sessions.Sort)); err != nil {
		return err
	}
	if settings.Sessions.GroupByExplicit {
		if err := editor.SetValue([]string{shellConfigRoot, "sessions", "group_by"}, string(settings.Sessions.GroupBy)); err != nil {
			return err
		}
	}
	if settings.Sessions.ScopeExplicit {
		return editor.SetValue([]string{shellConfigRoot, "sessions", "scope"}, string(settings.Sessions.Scope))
	}
	return nil
}
