package daemon

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/config"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
)

type publishedMCPEnablementStore interface {
	GetAppMetadata(context.Context, string) (string, bool, error)
	SetAppMetadata(context.Context, string, string) error
}

func publishedMCPEnablementKey(record resources.Record[config.MCPServer]) string {
	if record.Source != agentSkillSyncActor().Source {
		return ""
	}
	scope, owner := record.Scope.Normalize(), record.Owner.Normalize()
	identity := record.Spec.DefinitionSource
	if owner.Kind == extensionResourceOwnerKind {
		// Extension publications already have stable, manifest-relative IDs.
		identity = record.ID
	} else if scope.Kind == resources.ResourceScopeKindProfile && scope.ID == store.DefaultProfileID {
		// The default profile republishes global files under a second resource ID.
		scope = resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
	}
	if identity == "" {
		return ""
	}
	key := strings.Join([]string{
		string(scope.Kind), scope.ID, string(owner.Kind), owner.ID, identity, record.Spec.Name,
	}, "\x00")
	return fmt.Sprintf("mcp.published.enabled.%x", sha256.Sum256([]byte(key)))
}

func applyPublishedMCPEnablement(
	ctx context.Context, state *bootState, record resources.Record[config.MCPServer], server *config.MCPServer,
) error {
	store, ok := state.registry.(publishedMCPEnablementStore)
	key := publishedMCPEnablementKey(record)
	if !ok || key == "" {
		return nil
	}
	value, found, err := store.GetAppMetadata(ctx, key)
	if found {
		server.Disabled = new(value == "false")
	}
	return err
}

func (s settingsMCPConsent) SetPublishedMCPEnabled(
	ctx context.Context, req settings.MCPAuthTargetRequest, enabled bool,
) (settings.MCPPublishedDefinition, bool, error) {
	state := s.state
	if state == nil || state.mcpServerCatalog == nil {
		return settings.MCPPublishedDefinition{}, false, nil
	}
	var selected resources.Record[config.MCPServer]
	var target mcpauth.Target
	var selectedKey string
	owner := (mcpauth.Target{Owner: req.Owner}).Normalize().Owner
	for _, record := range state.mcpServerCatalog.Snapshot() {
		if record.Spec.Name != req.Name ||
			(req.DefinitionSource != "" && req.DefinitionSource != record.Spec.DefinitionSource) {
			continue
		}
		key := publishedMCPEnablementKey(record)
		if key == "" || key == selectedKey {
			continue
		}
		candidate, err := mcpAuthTargetForResource(ctx, state, record.Scope, record.Spec.Name, record.Owner)
		if err != nil {
			return settings.MCPPublishedDefinition{}, false, err
		}
		if candidate.Owner != owner ||
			!settingsMCPExtensionTargetMatches(req, publishedMCPManagementTarget(record.Scope, candidate)) {
			continue
		}
		if selected.ID != "" {
			return settings.MCPPublishedDefinition{}, false,
				fmt.Errorf(
					"%w: MCP server %q has multiple declarations; select a definition source",
					settings.ErrConflict,
					req.Name,
				)
		}
		selected, target, selectedKey = record, candidate, key
	}
	if selected.ID == "" {
		return settings.MCPPublishedDefinition{}, false, nil
	}
	store, ok := state.registry.(publishedMCPEnablementStore)
	if !ok {
		return settings.MCPPublishedDefinition{}, false,
			fmt.Errorf("%w: published MCP enablement store is unavailable", settings.ErrUnavailable)
	}
	server, err := projectExtensionSecretHeaders(ctx, state, selected, "")
	if err != nil {
		return settings.MCPPublishedDefinition{}, false, err
	}
	if err := store.SetAppMetadata(ctx, selectedKey, fmt.Sprint(enabled)); err != nil {
		return settings.MCPPublishedDefinition{}, false, err
	}
	if state.toolProjectionEpoch != nil {
		state.toolProjectionEpoch.Advance()
	}
	server.Disabled = new(!enabled)
	return settings.MCPPublishedDefinition{
		Target: target, ManagementTarget: publishedMCPManagementTarget(selected.Scope, target), Server: server,
	}, true, nil
}

func publishedMCPManagementTarget(scope resources.ResourceScope, authTarget mcpauth.Target) mcpauth.Target {
	if scope.Kind == resources.ResourceScopeKindWorkspaceProfile {
		authTarget.Scope, authTarget.WorkspaceID = mcpauth.ScopeWorkspaceProfile, scope.ID
	}
	return authTarget
}
