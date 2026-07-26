package core

import (
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/api/contract"

	aghconfig "github.com/compozy/agh/internal/config"

	settingspkg "github.com/compozy/agh/internal/settings"
	"github.com/gin-gonic/gin"
)

func parsePutSettingsMCPServerRequest(c *gin.Context) (settingspkg.CollectionItemPutRequest, error) {
	var body struct {
		Server          *contract.SettingsMCPServerPayload             `json:"server"`
		SecretValues    *contract.SettingsMCPSecretValuesPayload       `json:"secret_values,omitempty"`
		PreserveSecrets *contract.SettingsMCPSecretPreservationPayload `json:"preserve_secrets,omitempty"`
		PreserveEnv     []string                                       `json:"preserve_env,omitempty"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return settingspkg.CollectionItemPutRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode MCP server settings request: %w", err),
		)
	}
	if body.Server == nil {
		return settingspkg.CollectionItemPutRequest{}, NewSettingsValidationError(
			errors.New("mcp-servers.server is required"),
		)
	}
	req, err := parseSettingsCollectionRequest(c, settingspkg.CollectionMCPServers)
	if err != nil {
		return settingspkg.CollectionItemPutRequest{}, err
	}
	name, err := requiredSettingsPathValue(c.Param("name"), "name")
	if err != nil {
		return settingspkg.CollectionItemPutRequest{}, err
	}
	bodyName := strings.TrimSpace(body.Server.Name)
	if bodyName != "" && bodyName != name {
		return settingspkg.CollectionItemPutRequest{}, NewSettingsValidationError(
			fmt.Errorf("mcp-servers.server.name must match path name %q", name),
		)
	}
	target, err := parseSettingsTarget(c.Query("target"))
	if err != nil {
		return settingspkg.CollectionItemPutRequest{}, err
	}
	server := aghconfig.MCPServer{
		Name:      name,
		Transport: aghconfig.MCPServerTransport(strings.TrimSpace(body.Server.Transport)),
		Command:   strings.TrimSpace(body.Server.Command),
		Args:      cloneStrings(body.Server.Args),
		Env:       cloneStringMap(body.Server.Env),
		SecretEnv: cloneStringMap(body.Server.SecretEnv),
		URL:       strings.TrimSpace(body.Server.URL),
	}
	if body.Server.Auth != nil {
		server.Auth = aghconfig.MCPAuthConfig{
			Type:             aghconfig.MCPAuthType(strings.TrimSpace(body.Server.Auth.Type)),
			IssuerURL:        strings.TrimSpace(body.Server.Auth.IssuerURL),
			MetadataURL:      strings.TrimSpace(body.Server.Auth.MetadataURL),
			AuthorizationURL: strings.TrimSpace(body.Server.Auth.AuthorizationURL),
			TokenURL:         strings.TrimSpace(body.Server.Auth.TokenURL),
			RevocationURL:    strings.TrimSpace(body.Server.Auth.RevocationURL),
			ClientID:         strings.TrimSpace(body.Server.Auth.ClientID),
			ClientSecretRef:  strings.TrimSpace(body.Server.Auth.ClientSecretRef),
			Scopes:           cloneStrings(body.Server.Auth.Scopes),
		}
	}
	if err := server.Validate("server"); err != nil {
		return settingspkg.CollectionItemPutRequest{}, NewSettingsValidationError(err)
	}
	return settingspkg.CollectionItemPutRequest{
		CollectionRequest:     req,
		Name:                  name,
		Target:                target,
		MCPServer:             &server,
		MCPSecrets:            mcpSecretValuesFromPayload(body.SecretValues),
		MCPSecretPreservation: mcpSecretPreservationFromPayload(body.PreserveSecrets),
		MCPEnvPreservation:    cloneStrings(body.PreserveEnv),
	}, nil
}

func parsePutSettingsSandboxRequest(c *gin.Context) (settingspkg.CollectionItemPutRequest, error) {
	var body struct {
		Profile *contract.SettingsSandboxProfilePayload `json:"profile"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return settingspkg.CollectionItemPutRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode sandbox settings request: %w", err),
		)
	}
	if body.Profile == nil {
		return settingspkg.CollectionItemPutRequest{}, NewSettingsValidationError(
			errors.New("sandboxes.profile is required"),
		)
	}
	req, err := parseSettingsCollectionRequest(c, settingspkg.CollectionSandboxes)
	if err != nil {
		return settingspkg.CollectionItemPutRequest{}, err
	}
	name, err := requiredSettingsPathValue(c.Param("name"), "name")
	if err != nil {
		return settingspkg.CollectionItemPutRequest{}, err
	}
	profile, err := sandboxProfileFromPayload(*body.Profile)
	if err != nil {
		return settingspkg.CollectionItemPutRequest{}, err
	}
	return settingspkg.CollectionItemPutRequest{
		CollectionRequest: req,
		Name:              name,
		Sandbox:           &profile,
	}, nil
}

func parsePutSettingsHookRequest(c *gin.Context) (settingspkg.CollectionItemPutRequest, error) {
	var body struct {
		Declaration *contract.SettingsHookDeclarationPayload `json:"declaration"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		return settingspkg.CollectionItemPutRequest{}, NewSettingsValidationError(
			fmt.Errorf("decode hook settings request: %w", err),
		)
	}
	if body.Declaration == nil {
		return settingspkg.CollectionItemPutRequest{}, NewSettingsValidationError(
			errors.New("hooks.declaration is required"),
		)
	}
	req, err := parseSettingsCollectionRequest(c, settingspkg.CollectionHooks)
	if err != nil {
		return settingspkg.CollectionItemPutRequest{}, err
	}
	name, err := requiredSettingsPathValue(c.Param("name"), "name")
	if err != nil {
		return settingspkg.CollectionItemPutRequest{}, err
	}
	bodyName := strings.TrimSpace(body.Declaration.Name)
	if bodyName != "" && bodyName != name {
		return settingspkg.CollectionItemPutRequest{}, NewSettingsValidationError(
			fmt.Errorf("hooks.declaration.name must match path name %q", name),
		)
	}
	declaration := *body.Declaration
	declaration.Name = name
	decl, err := hookDeclarationFromPayload(declaration)
	if err != nil {
		return settingspkg.CollectionItemPutRequest{}, err
	}
	return settingspkg.CollectionItemPutRequest{
		CollectionRequest: req,
		Name:              name,
		Hook:              &decl,
	}, nil
}

func parseSettingsScope(rawScope string, rawWorkspaceID string) (settingspkg.ScopeKind, string, error) {
	scope := settingspkg.ScopeKind(strings.TrimSpace(rawScope))
	if scope == "" {
		scope = settingspkg.ScopeGlobal
	}
	if err := scope.Validate(); err != nil {
		return "", "", NewSettingsValidationError(err)
	}
	return scope, strings.TrimSpace(rawWorkspaceID), nil
}

func parseSettingsTarget(raw string) (settingspkg.TargetSelector, error) {
	target := settingspkg.TargetSelector(strings.TrimSpace(raw))
	if target == "" {
		target = settingspkg.TargetAuto
	}
	switch target {
	case settingspkg.TargetAuto, settingspkg.TargetConfig, settingspkg.TargetSidecar:
		return target, nil
	default:
		return "", NewSettingsValidationError(
			fmt.Errorf("settings.target must be one of %q, %q, %q", "auto", "config", "sidecar"),
		)
	}
}

func requiredSettingsPathValue(raw string, field string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", NewSettingsValidationError(fmt.Errorf("%s is required", field))
	}
	return value, nil
}
