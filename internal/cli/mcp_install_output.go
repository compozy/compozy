package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

type mcpInstallOutputRecord struct {
	MCPServer mcpInstallServerOutput              `json:"mcp_server"`
	Apply     contract.SettingsApplyResponse      `json:"apply"`
	NextStep  contract.SettingsMCPInstallNextStep `json:"next_step"`
}

type mcpInstallServerOutput struct {
	Name                   string                     `json:"name"`
	Transport              string                     `json:"transport"`
	Scope                  contract.SettingsScopeKind `json:"scope"`
	WorkspaceID            string                     `json:"workspace_id,omitempty"`
	CatalogEntry           string                     `json:"catalog_entry,omitempty"`
	CatalogVersion         string                     `json:"catalog_version,omitempty"`
	SecretEnvKeys          []string                   `json:"secret_env_keys,omitempty"`
	ClientSecretConfigured bool                       `json:"client_secret_configured"`
}

func newMCPInstallOutputRecord(response *InstallSettingsMCPServerRecord) mcpInstallOutputRecord {
	secretEnvKeys := append([]string(nil), response.MCPServer.SecretEnvKeys...)
	sort.Strings(secretEnvKeys)
	clientSecretConfigured := response.MCPServer.Auth != nil && response.MCPServer.Auth.ClientSecretConfigured
	return mcpInstallOutputRecord{
		MCPServer: mcpInstallServerOutput{
			Name:                   response.MCPServer.Name,
			Transport:              response.MCPServer.Transport,
			Scope:                  response.MCPServer.Scope,
			WorkspaceID:            response.MCPServer.WorkspaceID,
			CatalogEntry:           response.MCPServer.CatalogEntry,
			CatalogVersion:         response.MCPServer.CatalogVersion,
			SecretEnvKeys:          secretEnvKeys,
			ClientSecretConfigured: clientSecretConfigured,
		},
		Apply:    response.Apply,
		NextStep: response.NextStep,
	}
}

func mcpInstallBundle(response *InstallSettingsMCPServerRecord) outputBundle {
	record := newMCPInstallOutputRecord(response)
	return outputBundle{
		jsonValue: response,
		human: func() (string, error) {
			return renderHumanSection("MCP Install", []keyValue{
				{Label: automationNameValue, Value: record.MCPServer.Name},
				{Label: "Transport", Value: record.MCPServer.Transport},
				{Label: mcpScopeValue, Value: string(record.MCPServer.Scope)},
				{Label: "Catalog Entry", Value: record.MCPServer.CatalogEntry},
				{Label: "Catalog Version", Value: record.MCPServer.CatalogVersion},
				{Label: "Secret Fields", Value: stringOrDash(strings.Join(record.MCPServer.SecretEnvKeys, ", "))},
				{Label: "OAuth Client Secret", Value: configuredLabel(record.MCPServer.ClientSecretConfigured)},
				{Label: cliAppliedValue, Value: fmt.Sprintf("%t", record.Apply.Applied)},
				{Label: cliLifecycleValue, Value: string(record.Apply.Lifecycle)},
				{Label: "Apply Record", Value: stringOrDash(record.Apply.ApplyRecordID)},
				{Label: cliActiveGenerationValue, Value: fmt.Sprintf("%d", record.Apply.ActiveGeneration)},
				{Label: cliNextActionValue, Value: string(record.Apply.NextAction)},
				{Label: "Next Step", Value: string(record.NextStep)},
			}), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"mcp_install",
				[]string{
					automationNameKey,
					"transport",
					mcpScopeKey,
					mcpWorkspaceIDKey,
					"catalog_entry",
					"catalog_version",
					"secret_env_keys",
					"client_secret_configured",
					cliAppliedKey,
					cliLifecycleKey,
					cliApplyRecordIDKey,
					cliActiveGenerationKey,
					cliNextActionKey,
					"next_step",
				},
				[]string{
					record.MCPServer.Name,
					record.MCPServer.Transport,
					string(record.MCPServer.Scope),
					record.MCPServer.WorkspaceID,
					record.MCPServer.CatalogEntry,
					record.MCPServer.CatalogVersion,
					strings.Join(record.MCPServer.SecretEnvKeys, ","),
					fmt.Sprintf("%t", record.MCPServer.ClientSecretConfigured),
					fmt.Sprintf("%t", record.Apply.Applied),
					string(record.Apply.Lifecycle),
					record.Apply.ApplyRecordID,
					fmt.Sprintf("%d", record.Apply.ActiveGeneration),
					string(record.Apply.NextAction),
					string(record.NextStep),
				},
			), nil
		},
	}
}

func configuredLabel(configured bool) string {
	if configured {
		return "configured"
	}
	return "not configured"
}
