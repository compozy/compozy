package cli

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

const (
	configShellKey         = "shell"
	configShellSessionsKey = "sessions"
	configShellSortKey     = "sort"
)

func shellConfigSetPathKinds() map[string]configSetValueKind {
	return map[string]configSetValueKind{
		configShellKey + "." + configShellSessionsKey + "." + configShellSortKey: configSetString,
		configShellKey + "." + configShellSessionsKey + "." + automationScopeKey: configSetString,
	}
}

func applyShellConfigValue(cfg *compozyconfig.ShellConfig, path []string, value any) error {
	if cfg == nil || len(path) != 3 || path[0] != configShellKey || path[1] != configShellSessionsKey {
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
	switch path[2] {
	case configShellSortKey:
		cfg.Sessions.Sort = compozyconfig.ShellSessionSort(text)
	case automationScopeKey:
		cfg.Sessions.Scope = compozyconfig.ShellSessionScope(text)
	default:
		return fmt.Errorf("cli: unsupported shell config path %q", strings.Join(path, "."))
	}
	return cfg.Validate()
}

func settingsShellPayloadFromConfig(cfg compozyconfig.ShellConfig) contract.SettingsShellPayload {
	return contract.SettingsShellPayload{Sessions: contract.SettingsShellSessionsPayload{
		Sort:  contract.SettingsShellSessionSort(cfg.Sessions.Sort),
		Scope: contract.SettingsShellSessionScope(cfg.Sessions.Scope),
	}}
}
