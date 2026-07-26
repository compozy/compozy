package core

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	settingspkg "github.com/compozy/agh/internal/settings"
)

func agentMCPAuthPayload(value aghconfig.MCPAuthConfig) *contract.SettingsMCPAuthConfigViewPayload {
	if value.IsZero() {
		return nil
	}
	return &contract.SettingsMCPAuthConfigViewPayload{
		Type:                   strings.TrimSpace(string(value.Type)),
		IssuerURL:              strings.TrimSpace(value.IssuerURL),
		MetadataURL:            strings.TrimSpace(value.MetadataURL),
		AuthorizationURL:       strings.TrimSpace(value.AuthorizationURL),
		TokenURL:               strings.TrimSpace(value.TokenURL),
		RevocationURL:          strings.TrimSpace(value.RevocationURL),
		ClientID:               strings.TrimSpace(value.ClientID),
		ClientSecretConfigured: strings.TrimSpace(value.ClientSecretRef) != "",
		Scopes:                 cloneStrings(value.Scopes),
	}
}

func settingsMCPServerItemPayloads(values []settingspkg.MCPServerItem) []contract.SettingsMCPServerItemPayload {
	if len(values) == 0 {
		return []contract.SettingsMCPServerItemPayload{}
	}
	payloads := make([]contract.SettingsMCPServerItemPayload, 0, len(values))
	for _, value := range values {
		payloads = append(payloads, contract.SettingsMCPServerItemPayload{
			Name:           strings.TrimSpace(value.Name),
			Transport:      strings.TrimSpace(string(value.Transport)),
			Command:        strings.TrimSpace(value.Command),
			Args:           cloneStrings(value.Args),
			EnvKeys:        cloneStrings(value.EnvKeys),
			SecretEnvKeys:  cloneStrings(value.SecretEnvKeys),
			URL:            strings.TrimSpace(value.URL),
			Auth:           settingsMCPAuthConfigPayload(value.Auth, value.ClientSecretConfigured),
			AuthStatus:     settingsMCPAuthStatusPayload(value.AuthStatus),
			RuntimeStatus:  settingsMCPServerRuntimeStatusPayload(value.RuntimeStatus),
			Scope:          contract.SettingsScopeKind(value.Scope),
			WorkspaceID:    strings.TrimSpace(value.WorkspaceID),
			CatalogEntry:   strings.TrimSpace(value.CatalogEntry),
			CatalogVersion: strings.TrimSpace(value.CatalogVersion),
			SourceMetadata: settingsSourceMetadataPayload(value.SourceMetadata),
		})
	}
	return payloads
}

func settingsMCPServerRuntimeStatusPayload(
	value *settingspkg.MCPServerRuntimeStatus,
) *contract.SettingsMCPServerRuntimeStatusPayload {
	if value == nil {
		return nil
	}
	return &contract.SettingsMCPServerRuntimeStatusPayload{
		Configured:  value.Configured,
		Initialized: value.Initialized,
		State:       strings.TrimSpace(string(value.State)),
		Probe:       strings.TrimSpace(string(value.Probe)),
		ToolCount:   value.ToolCount,
		Reason:      strings.TrimSpace(value.Reason),
		Diagnostic:  strings.TrimSpace(value.Diagnostic),
	}
}

func settingsMCPAuthConfigPayload(
	value aghconfig.MCPAuthConfig,
	clientSecretConfigured bool,
) *contract.SettingsMCPAuthConfigViewPayload {
	if value.IsZero() {
		return nil
	}
	return &contract.SettingsMCPAuthConfigViewPayload{
		Type:                   strings.TrimSpace(string(value.Type)),
		IssuerURL:              strings.TrimSpace(value.IssuerURL),
		MetadataURL:            strings.TrimSpace(value.MetadataURL),
		AuthorizationURL:       strings.TrimSpace(value.AuthorizationURL),
		TokenURL:               strings.TrimSpace(value.TokenURL),
		RevocationURL:          strings.TrimSpace(value.RevocationURL),
		ClientID:               strings.TrimSpace(value.ClientID),
		ClientSecretConfigured: clientSecretConfigured,
		Scopes:                 cloneStrings(value.Scopes),
	}
}

func mcpSecretPreservationFromPayload(
	payload *contract.SettingsMCPSecretPreservationPayload,
) settingspkg.MCPSecretPreservation {
	if payload == nil {
		return settingspkg.MCPSecretPreservation{}
	}
	return settingspkg.MCPSecretPreservation{
		SecretEnv:         cloneStrings(payload.SecretEnv),
		OAuthClientSecret: payload.OAuthClientSecret,
	}
}

func mcpSecretValuesFromPayload(payload *contract.SettingsMCPSecretValuesPayload) settingspkg.MCPSecretValues {
	if payload == nil {
		return settingspkg.MCPSecretValues{}
	}
	var oauthClientSecret *string
	if payload.OAuthClientSecret != nil {
		value := *payload.OAuthClientSecret
		oauthClientSecret = &value
	}
	return settingspkg.MCPSecretValues{
		SecretEnv:         cloneStringMap(payload.SecretEnv),
		OAuthClientSecret: oauthClientSecret,
	}
}
