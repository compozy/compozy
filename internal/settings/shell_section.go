package settings

import compozyconfig "github.com/compozy/compozy/internal/config"

const (
	shellConfigRoot    = "shell"
	shellSessionsKey   = "sessions"
	shellAppearanceKey = "appearance"
)

func buildShellSection(cfg *compozyconfig.Config) ShellSection {
	warnings := []string{}
	if cfg.Shell.Sessions.ScopeExplicit {
		warnings = append(warnings, compozyconfig.ShellScopeDeprecationWarning)
	}
	return ShellSection{Config: cfg.Shell, Warnings: warnings}
}

func diffShellSettings(current compozyconfig.ShellConfig, desired compozyconfig.ShellConfig) []string {
	changed := make([]string, 0, 5)
	if current.Sessions.GroupBy != desired.Sessions.GroupBy ||
		current.Sessions.GroupByExplicit != desired.Sessions.GroupByExplicit {
		changed = append(changed, "shell.sessions.group_by")
	}
	if current.Sessions.Sort != desired.Sessions.Sort {
		changed = append(changed, "shell.sessions.sort")
	}
	if desired.Sessions.ScopeExplicit || current.Sessions.Scope != desired.Sessions.Scope {
		changed = append(changed, "shell.sessions.scope")
	}
	if shellFontChanged(current.Appearance.UIFont, current.Appearance.UIFontExplicit,
		desired.Appearance.UIFont, desired.Appearance.UIFontExplicit) {
		changed = append(changed, "shell.appearance.ui_font")
	}
	if shellFontChanged(current.Appearance.MonoFont, current.Appearance.MonoFontExplicit,
		desired.Appearance.MonoFont, desired.Appearance.MonoFontExplicit) {
		changed = append(changed, "shell.appearance.mono_font")
	}
	return changed
}

// shellFontChanged also reports an explicit reset of a key the file stores as
// "", so the reset deletes it instead of leaving the empty key behind.
func shellFontChanged(current string, currentExplicit bool, desired string, desiredExplicit bool) bool {
	if current != desired {
		return true
	}
	return desired == "" && desiredExplicit && currentExplicit
}

func applyShellSettings(editor *compozyconfig.OverlayEditor, settings compozyconfig.ShellConfig) error {
	if err := editor.SetValue(
		[]string{shellConfigRoot, shellSessionsKey, "sort"},
		string(settings.Sessions.Sort),
	); err != nil {
		return err
	}
	if settings.Sessions.GroupByExplicit {
		if err := editor.SetValue(
			[]string{shellConfigRoot, shellSessionsKey, "group_by"},
			string(settings.Sessions.GroupBy),
		); err != nil {
			return err
		}
	}
	if settings.Sessions.ScopeExplicit {
		if err := editor.SetValue(
			[]string{shellConfigRoot, shellSessionsKey, "scope"},
			string(settings.Sessions.Scope),
		); err != nil {
			return err
		}
	}
	if settings.Appearance.UIFontExplicit {
		if err := applyShellFont(editor, "ui_font", settings.Appearance.UIFont); err != nil {
			return err
		}
	}
	if settings.Appearance.MonoFontExplicit {
		return applyShellFont(editor, "mono_font", settings.Appearance.MonoFont)
	}
	return nil
}

// applyShellFont persists a font family; an empty family deletes the key so the default applies.
func applyShellFont(editor *compozyconfig.OverlayEditor, key string, family string) error {
	path := []string{shellConfigRoot, shellAppearanceKey, key}
	if family == "" {
		return editor.Delete(path)
	}
	return editor.SetValue(path, family)
}
