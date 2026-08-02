package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func extensionSecretsBundle(item ExtensionSecretsRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		jsonl: func(cmd *cobra.Command) error {
			return writeJSONLine(cmd, item)
		},
		human: func() (string, error) {
			stale := staleExtensionSecretNames(item)
			return renderHumanSection("Extension Secrets", []keyValue{
				{Label: "Declared Env", Value: stringOrDash(strings.Join(item.DeclaredEnv, ", "))},
				{Label: "Bound Env", Value: stringOrDash(strings.Join(item.BoundEnvKeys, ", "))},
				{Label: "Stale Bindings", Value: stringOrDash(strings.Join(stale, ", "))},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject("extension_secrets", []string{
				"declared_env", "bound_env_keys", "stale_env_keys",
			}, []string{
				strings.Join(item.DeclaredEnv, "|"),
				strings.Join(item.BoundEnvKeys, "|"),
				strings.Join(staleExtensionSecretNames(item), "|"),
			}), nil
		},
	}
}

func extensionSecretUnsetBundle(item extensionSecretUnsetRecord) outputBundle {
	return outputBundle{
		jsonValue: item,
		jsonl:     func(cmd *cobra.Command) error { return writeJSONLine(cmd, item) },
		human: func() (string, error) {
			return fmt.Sprintf("✓ unbound %s", item.EnvName), nil
		},
		toon: func() (string, error) {
			return renderToonObject("extension_secret", []string{"env_name", automationStatusKey}, []string{
				item.EnvName, item.Status,
			}), nil
		},
	}
}

func staleExtensionSecretNames(item ExtensionSecretsRecord) []string {
	if item.Bindings != nil {
		stale := make([]string, 0, len(item.Bindings))
		for _, binding := range item.Bindings {
			if binding.Stale {
				stale = append(stale, strings.TrimSpace(binding.EnvName))
			}
		}
		return stale
	}
	declared := make(map[string]struct{}, len(item.DeclaredEnv))
	for _, name := range item.DeclaredEnv {
		declared[strings.TrimSpace(name)] = struct{}{}
	}
	stale := make([]string, 0)
	for _, name := range item.BoundEnvKeys {
		if _, ok := declared[strings.TrimSpace(name)]; !ok {
			stale = append(stale, name)
		}
	}
	return stale
}
