package cli

import (
	"strconv"
	"strings"
	"time"
)

const (
	mcpAuthExpiresValue = "Expires"
	mcpAuthExpiresAtKey = "expires_at"
)

func mcpAuthStatusBundle(status SettingsMCPAuthStatusRecord) outputBundle {
	return outputBundle{
		jsonValue: status,
		human: func() (string, error) {
			return renderHumanSection("MCP Auth", mcpAuthStatusRows(status)), nil
		},
		toon: func() (string, error) {
			fields, values := mcpAuthStatusFields(status)
			return renderToonObject("mcp_auth", fields, values), nil
		},
	}
}

func mcpAuthStatusListBundle(statuses []SettingsMCPAuthStatusRecord) outputBundle {
	return listBundle(
		statuses,
		statuses,
		"MCP Auth",
		[]string{"Server", mcpScopeValue, automationStatusValue, "Client", mcpAuthExpiresValue, "Diagnostic"},
		"mcp_auth",
		[]string{"server", mcpScopeKey, automationStatusKey, "client", mcpAuthExpiresAtKey, "diagnostic"},
		func(item SettingsMCPAuthStatusRecord) []string {
			return []string{
				stringOrDash(item.ServerName),
				stringOrDash(item.Scope),
				stringOrDash(item.Status),
				stringOrDash(item.ClientID),
				stringOrDash(timePtrString(item.ExpiresAt)),
				stringOrDash(item.Diagnostic),
			}
		},
		func(item SettingsMCPAuthStatusRecord) []string {
			return []string{
				item.ServerName,
				item.Scope,
				item.Status,
				item.ClientID,
				timePtrString(item.ExpiresAt),
				item.Diagnostic,
			}
		},
	)
}

func mcpAuthStatusRows(status SettingsMCPAuthStatusRecord) []keyValue {
	return []keyValue{
		{Label: "Server", Value: stringOrDash(status.ServerName)},
		{Label: mcpScopeValue, Value: stringOrDash(status.Scope)},
		{Label: configWorkspaceValue, Value: stringOrDash(status.WorkspaceID)},
		{Label: automationStatusValue, Value: stringOrDash(status.Status)},
		{Label: "Remote URL", Value: stringOrDash(status.RemoteURL)},
		{Label: "Auth Type", Value: stringOrDash(status.AuthType)},
		{Label: "Client ID", Value: stringOrDash(status.ClientID)},
		{Label: "Scopes", Value: stringOrDash(strings.Join(status.Scopes, ", "))},
		{Label: mcpAuthExpiresValue, Value: stringOrDash(timePtrString(status.ExpiresAt))},
		{Label: "Token Present", Value: boolString(status.TokenPresent)},
		{Label: "Refreshable", Value: boolString(status.Refreshable)},
		{Label: "Diagnostic", Value: stringOrDash(status.Diagnostic)},
	}
}

func mcpAuthStatusFields(status SettingsMCPAuthStatusRecord) ([]string, []string) {
	fields := []string{
		"server",
		mcpScopeKey,
		mcpWorkspaceIDKey,
		automationStatusKey,
		"remote_url",
		"auth_type",
		"client_id",
		"scopes",
		mcpAuthExpiresAtKey,
		"token_present",
		"refreshable",
		"diagnostic",
	}
	values := []string{
		status.ServerName,
		status.Scope,
		status.WorkspaceID,
		status.Status,
		status.RemoteURL,
		status.AuthType,
		status.ClientID,
		strings.Join(status.Scopes, "|"),
		timePtrString(status.ExpiresAt),
		boolString(status.TokenPresent),
		boolString(status.Refreshable),
		status.Diagnostic,
	}
	return fields, values
}

func timePtrString(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func boolString(value bool) string {
	return strconv.FormatBool(value)
}
