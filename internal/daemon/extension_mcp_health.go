package daemon

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/resources"
)

func extensionMCPHealthKey(
	state *bootState,
	record resources.Record[compozyconfig.MCPServer],
) mcppkg.RuntimeHealthKey {
	owner := record.Owner.Normalize()
	if state == nil || state.extensions == nil || owner.Kind != extensionResourceOwnerKind {
		return mcppkg.RuntimeHealthKey{}
	}
	key := extensionpkg.InstanceKey{Name: owner.ID, WorkspaceID: record.Scope.ID}.Normalize()
	var ext *extensionpkg.Extension
	var err error
	if key.WorkspaceID != "" {
		if runtime, ok := state.extensions.(extensionDevRuntime); ok {
			ext, err = runtime.GetForInstance(key)
		}
	} else {
		ext, err = state.extensions.Get(key.Name)
	}
	if err != nil || ext == nil {
		return mcppkg.RuntimeHealthKey{}
	}
	return mcppkg.RuntimeHealthKey{
		InstanceName:     key.Name,
		WorkspaceID:      key.WorkspaceID,
		BundleGeneration: extensionMCPHealthGeneration(ext),
		ServerName:       record.Spec.Name,
	}
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
			ID:            "extension.mcp." + key.Name + "." + entry.Key.ServerName + ".unhealthy",
			Code:          contract.CodeExtensionMCPServerUnhealthy,
			Category:      contract.CategoryExtension,
			Title:         "Extension MCP server is unhealthy",
			Message:       fmt.Sprintf("MCP server %q is unavailable: %s", entry.Key.ServerName, entry.Message),
			Severity:      contract.SeverityError,
			DataFreshness: contract.FreshnessLive,
		}, diagnostics.WithEvidence(map[string]any{
			"extension_name":    key.Name,
			"workspace_id":      key.WorkspaceID,
			"bundle_generation": entry.Key.BundleGeneration,
			"server_name":       entry.Key.ServerName,
		})))
	}
	return items
}
