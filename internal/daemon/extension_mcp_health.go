package daemon

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/resources"
)

func extensionMCPHealthKey(
	ctx context.Context,
	state *bootState,
	record resources.Record[compozyconfig.MCPServer],
) mcppkg.RuntimeHealthKey {
	owner := record.Owner.Normalize()
	if state == nil || state.extensions == nil || owner.Kind != extensionResourceOwnerKind {
		return mcppkg.RuntimeHealthKey{}
	}
	profileID, workspaceID, err := mcpExtensionBindingOwner(ctx, state, record.Scope)
	if err != nil {
		return mcppkg.RuntimeHealthKey{}
	}
	key := extensionpkg.InstanceKey{Name: owner.ID, ProfileID: profileID, WorkspaceID: workspaceID}.Normalize()
	var ext *extensionpkg.Extension
	if runtime, ok := state.extensions.(extensionDevRuntime); ok {
		ext, err = runtime.GetForInstance(key)
	} else if key.WorkspaceID == "" {
		ext, err = state.extensions.Get(key.Name)
	}
	if err != nil || ext == nil {
		return mcppkg.RuntimeHealthKey{}
	}
	return mcppkg.RuntimeHealthKey{
		ResourceID:       record.ID,
		InstanceName:     key.Name,
		WorkspaceID:      key.WorkspaceID,
		BundleGeneration: extensionMCPHealthGeneration(ext),
		ServerName:       record.Spec.Name,
	}
}

func extensionMCPHealthKeyForTarget(
	ctx context.Context,
	state *bootState,
	target mcpauth.Target,
) (mcppkg.RuntimeHealthKey, error) {
	if state == nil || state.mcpServerCatalog == nil || target.Owner == "" {
		return mcppkg.RuntimeHealthKey{}, nil
	}
	for _, record := range state.mcpServerCatalog.Snapshot() {
		if record.Owner.Kind != extensionResourceOwnerKind || record.Spec.Name != target.ServerName {
			continue
		}
		candidate, err := mcpAuthTargetForResource(ctx, state, record.Scope, record.Spec.Name, record.Owner)
		if err != nil {
			return mcppkg.RuntimeHealthKey{}, err
		}
		if candidate.Normalize() == target.Normalize() {
			return extensionMCPHealthKey(ctx, state, record), nil
		}
	}
	return mcppkg.RuntimeHealthKey{}, nil
}

func extensionMCPHealthGeneration(ext *extensionpkg.Extension) string {
	if ext == nil {
		return ""
	}
	if generation := strings.TrimSpace(ext.Status.GenerationHash); generation != "" {
		return generation
	}
	return strings.TrimSpace(ext.Info.Checksum)
}

func (s *daemonExtensionService) evictExtensionMCPHealth(name string, workspaceID string) {
	if s == nil || s.mcpRuntimeHealth == nil {
		return
	}
	s.mcpRuntimeHealth.EvictInstance(name, workspaceID)
}

func extensionMCPHealthDiagnostics(
	registry *mcppkg.RuntimeHealthRegistry,
	ext *extensionpkg.Extension,
) []contract.DiagnosticItem {
	if registry == nil || ext == nil {
		return nil
	}
	key := extensionpkg.InstanceKey{Name: ext.Info.Name, WorkspaceID: ext.Status.WorkspaceID}.Normalize()
	entries := registry.Entries(key.Name, key.WorkspaceID, extensionMCPHealthGeneration(ext))
	items := make([]contract.DiagnosticItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, diagnostics.NewItem(diagnostics.ItemSpec{
			ID:            "extension.mcp." + key.Name + "." + entry.Key.ServerName + "." + entry.Key.ResourceID + ".unhealthy",
			Code:          contract.CodeExtensionMCPServerUnhealthy,
			Category:      contract.CategoryExtension,
			Title:         "Extension MCP server is unhealthy",
			Message:       fmt.Sprintf("MCP server %q is unavailable: %s", entry.Key.ServerName, entry.Message),
			Severity:      contract.SeverityError,
			DataFreshness: contract.FreshnessLive,
		}, diagnostics.WithEvidence(map[string]any{
			daemonExtensionNameKey: key.Name,
			daemonWorkspaceIDKey:   key.WorkspaceID,
			"bundle_generation":    entry.Key.BundleGeneration,
			"server_name":          entry.Key.ServerName,
		})))
	}
	return items
}
