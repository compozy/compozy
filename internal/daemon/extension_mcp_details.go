package daemon

import (
	"context"
	"fmt"
	"slices"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

type extensionMCPDetails struct {
	state   *bootState
	runtime settingspkg.MCPRuntimeProvider
}

func withDaemonExtensionMCPDetails(details *extensionMCPDetails) daemonExtensionServiceOption {
	return func(s *daemonExtensionService) { s.mcpDetails = details }
}

func (s *daemonExtensionService) populateExtensionMCPDetails(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	profile extensionpkg.ProfileLens,
	payload *contract.ExtensionPayload,
) error {
	if len(payload.MCPServers) == 0 {
		return nil
	}
	if s.mcpAllocations != nil {
		records, err := s.mcpAllocations.List(ctx, profile.ID, key.WorkspaceID)
		if err != nil {
			return err
		}
		for i := range payload.MCPServers {
			for _, record := range records {
				if record.Extension == key.Name && record.ServerName == payload.MCPServers[i].Name {
					payload.MCPServers[i].RuntimeName = record.RuntimeName
				}
			}
		}
	}
	if s.mcpDetails == nil {
		return nil
	}
	return s.mcpDetails.populate(ctx, key, profile, payload)
}

func (d *extensionMCPDetails) populate(
	ctx context.Context,
	key extensionpkg.InstanceKey,
	profile extensionpkg.ProfileLens,
	payload *contract.ExtensionPayload,
) error {
	if d.state == nil || d.state.mcpServerCatalog == nil {
		return nil
	}
	records := d.state.mcpServerCatalog.Snapshot()
	for i := range payload.MCPServers {
		item := &payload.MCPServers[i]
		if item.Status == "disabled" {
			continue
		}
		item.Status = "stopped"
		for _, record := range records {
			owner := record.Owner.Normalize()
			if owner.Kind != extensionResourceOwnerKind || owner.ID != key.Name || record.Spec.Name != item.Name {
				continue
			}
			profileID, workspaceID, err := mcpExtensionBindingOwner(ctx, d.state, record.Scope)
			if err != nil {
				return err
			}
			if profileID != profile.ID || workspaceID != key.WorkspaceID {
				continue
			}
			server, err := projectExtensionSecretHeaders(ctx, d.state, record, "")
			if err != nil {
				return err
			}
			item.RuntimeName = server.EffectiveRuntimeName()
			item.Transport = string(server.Transport)
			item.Launch = extensionpkg.MCPServerLaunchSummary(server.Command, server.URL)
			item.Auth = publishedMCPAuthSummary(server.Auth)
			if d.runtime == nil {
				item.Status = "unknown"
				break
			}
			target, err := mcpAuthTargetForResource(ctx, d.state, record.Scope, server.Name, owner)
			if err != nil {
				return err
			}
			status, err := d.runtime.MCPServerRuntimeStatus(ctx, target, server)
			if err != nil {
				return fmt.Errorf("daemon: read extension MCP status: %w", err)
			}
			item.Status = marketplaceMCPRuntimeStatus(status.State)
			break
		}
	}
	return nil
}

func publishedMCPAuthSummary(auth compozyconfig.MCPAuthConfig) *contract.MCPAuthSummary {
	if !auth.Enabled() {
		return &contract.MCPAuthSummary{Method: "none"}
	}
	registration := string(auth.Registration)
	if registration == "auto" {
		registration = "dynamic"
	}
	return &contract.MCPAuthSummary{
		Method:       "oauth",
		Registration: registration,
		IssuerURL:    auth.IssuerURL,
		Scopes:       slices.Clone(auth.Scopes),
	}
}

func marketplaceMCPRuntimeStatus(state settingspkg.MCPServerRuntimeState) string {
	switch state {
	case settingspkg.MCPServerRuntimeStateReady:
		return "running"
	case settingspkg.MCPServerRuntimeStateAuthRequired, settingspkg.MCPServerRuntimeStateAuthExpired,
		settingspkg.MCPServerRuntimeStateAuthInvalid, settingspkg.MCPServerRuntimeStateAuthRefreshFailed:
		return "needs_authorization"
	default:
		return "stopped"
	}
}
