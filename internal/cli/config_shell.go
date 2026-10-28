package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	configShellKey           = "shell"
	configShellSessionsKey   = "sessions"
	configShellSortKey       = "sort"
	configShellAppearanceKey = "appearance"
	configShellUIFontKey     = "ui_font"
	configShellMonoFontKey   = "mono_font"
)

func shellConfigSetPathKinds() map[string]configSetValueKind {
	return map[string]configSetValueKind{
		"shell.sessions.group_by": configSetString,
		configShellKey + "." + configShellSessionsKey + "." + configShellSortKey:       configSetString,
		configShellKey + "." + configShellSessionsKey + "." + automationScopeKey:       configSetString,
		configShellKey + "." + configShellAppearanceKey + "." + configShellUIFontKey:   configSetString,
		configShellKey + "." + configShellAppearanceKey + "." + configShellMonoFontKey: configSetString,
	}
}

func applyShellConfigValue(cfg *compozyconfig.ShellConfig, path []string, value any) error {
	if cfg == nil || len(path) != 3 || path[0] != configShellKey ||
		(path[1] != configShellSessionsKey && path[1] != configShellAppearanceKey) {
		return fmt.Errorf("cli: unsupported shell config path %q", strings.Join(path, "."))
	}
	text, ok := value.(string)
	if !ok {
		return fmt.Errorf(
			"cli: config set %q expects a string, got %T",
			strings.Join(path, "."),
			value,
		)
	}
	if path[1] == configShellAppearanceKey {
		return applyShellAppearanceValue(cfg, path, text)
	}
	switch path[2] {
	case "group_by":
		cfg.Sessions.GroupBy = compozyconfig.ShellSessionGroupBy(text)
		cfg.Sessions.GroupByExplicit = true
	case configShellSortKey:
		cfg.Sessions.Sort = compozyconfig.ShellSessionSort(text)
	case automationScopeKey:
		cfg.Sessions.Scope = compozyconfig.ShellSessionScope(text)
	default:
		return fmt.Errorf("cli: unsupported shell config path %q", strings.Join(path, "."))
	}
	return cfg.Validate()
}

func applyShellAppearanceValue(cfg *compozyconfig.ShellConfig, path []string, family string) error {
	switch path[2] {
	case configShellUIFontKey:
		cfg.Appearance.UIFont = family
		cfg.Appearance.UIFontExplicit = true
	case configShellMonoFontKey:
		cfg.Appearance.MonoFont = family
		cfg.Appearance.MonoFontExplicit = true
	default:
		return fmt.Errorf("cli: unsupported shell config path %q", strings.Join(path, "."))
	}
	return cfg.Validate()
}

// settingsShellUpdatePayload sends only the selected shell field so the daemon preserves the others.
func settingsShellUpdatePayload(cfg compozyconfig.ShellConfig, path []string) contract.SettingsShellPayload {
	if path[1] == configShellAppearanceKey {
		appearance := &contract.SettingsShellAppearancePayload{}
		if path[2] == configShellUIFontKey {
			appearance.UIFont = &cfg.Appearance.UIFont
		} else {
			appearance.MonoFont = &cfg.Appearance.MonoFont
		}
		return contract.SettingsShellPayload{Appearance: appearance}
	}
	sessions := contract.SettingsShellSessionsPayload{}
	switch path[2] {
	case configShellSortKey:
		sessions.Sort = contract.SettingsShellSessionSort(cfg.Sessions.Sort)
	case automationScopeKey:
		sessions.Scope = contract.SettingsShellSessionScope(cfg.Sessions.Scope)
	case "group_by":
		sessions.GroupBy = contract.SettingsShellSessionGroupBy(cfg.Sessions.GroupBy)
	}
	return contract.SettingsShellPayload{Sessions: sessions}
}
