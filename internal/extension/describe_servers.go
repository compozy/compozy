package extensionpkg

import (
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
)

func extensionServerPayloads(ext *Extension, profileName string) []contract.MarketplaceServerPayload {
	servers := make([]contract.MarketplaceServerPayload, 0)
	if ext.Manifest == nil {
		return servers
	}
	scope := "global"
	if ext.Status.WorkspaceID != "" {
		scope = "workspace"
	}
	for _, name := range sortedMapKeys(ext.Manifest.Resources.MCPServers) {
		config := ext.Manifest.Resources.MCPServers[name]
		if !manifestPlacementVisible(config.Profile, profileName) {
			continue
		}
		transport := strings.TrimSpace(config.Transport)
		if transport == "" {
			transport = "stdio"
			if config.URL != "" {
				transport = forgeURLSchemeHTTP
			}
		}
		status := "unknown"
		if !ext.Info.Enabled {
			status = "disabled"
		}
		servers = append(servers, contract.MarketplaceServerPayload{
			Name: name, Owner: "extension:" + ext.Info.Name, Scope: scope,
			Transport: transport, Launch: MCPServerLaunchSummary(config.Command, config.URL), Status: status,
			Auth: manifestMCPAuthSummary(config.Auth), Profile: profileName, WorkspaceID: ext.Status.WorkspaceID,
		})
	}
	return servers
}

// MCPServerLaunchSummary excludes arguments, userinfo, paths and query values that may contain secrets.
func MCPServerLaunchSummary(command, remoteURL string) string {
	if remoteURL != "" {
		endpoint, err := url.Parse(remoteURL)
		if err != nil || endpoint.Hostname() == "" {
			return ""
		}
		return endpoint.Scheme + "://" + endpoint.Host
	}
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	return filepath.Base(command)
}

func manifestMCPAuthSummary(auth *MCPServerAuthConfig) *contract.MCPAuthSummary {
	if auth == nil {
		return nil
	}
	return &contract.MCPAuthSummary{
		Method:       auth.Method,
		Registration: auth.Registration,
		IssuerURL:    auth.IssuerURL,
		Scopes:       slices.Clone(auth.Scopes),
	}
}
