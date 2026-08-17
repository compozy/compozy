package cli

import (
	"encoding/json"
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
				{Label: "Bound Env", Value: stringOrDash(strings.Join(extensionSecretBindingLabels(item), ", "))},
				{Label: "Stale Bindings", Value: stringOrDash(strings.Join(stale, ", "))},
			}), nil
		},
		toon: func() (string, error) {
			type approvedBinding struct {
				EnvName    string `json:"env_name"`
				Stale      bool   `json:"stale"`
				MCPServer  string `json:"mcp_server,omitempty"`
				HeaderName string `json:"header_name,omitempty"`
			}
			approvedBindings := make([]approvedBinding, 0, len(item.Bindings))
			for _, binding := range item.Bindings {
				approvedBindings = append(approvedBindings, approvedBinding{
					EnvName:    binding.EnvName,
					Stale:      binding.Stale,
					MCPServer:  binding.MCPServer,
					HeaderName: binding.HeaderName,
				})
			}
			bindings, err := json.Marshal(approvedBindings)
			if err != nil {
				return "", fmt.Errorf("cli: marshal extension secret bindings: %w", err)
			}
			return renderToonObject("extension_secrets", []string{
				"declared_env", "bound_env_keys", "stale_env_keys", cliBindingsKey,
			}, []string{
				strings.Join(item.DeclaredEnv, "|"),
				strings.Join(item.BoundEnvKeys, "|"),
				strings.Join(staleExtensionSecretNames(item), "|"),
				string(bindings),
			}), nil
		},
	}
}

func extensionSecretBindingLabels(item ExtensionSecretsRecord) []string {
	labels := make([]string, 0, len(item.Bindings))
	for _, binding := range item.Bindings {
		label := strings.TrimSpace(binding.EnvName)
		server := strings.TrimSpace(binding.MCPServer)
		header := strings.TrimSpace(binding.HeaderName)
		if server != "" && header != "" {
			label += " (remote-header " + server + ":" + header + ")"
		}
		labels = append(labels, label)
	}
	if len(labels) == 0 {
		return append([]string(nil), item.BoundEnvKeys...)
	}
	return labels
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
	stale := make([]string, 0, len(item.Bindings))
	for _, binding := range item.Bindings {
		if binding.Stale {
			stale = append(stale, strings.TrimSpace(binding.EnvName))
		}
	}
	return stale
}
