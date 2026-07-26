package daemon

import (
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/resources"
	toolspkg "github.com/compozy/agh/internal/tools"
)

func TestDaemonMCPServerResolverPreservesWorkspaceResourceIdentity(t *testing.T) {
	t.Parallel()

	catalog := newResourceCatalog(cloneDaemonMCPServer)
	catalog.Replace(1, []resources.Record[aghconfig.MCPServer]{
		{
			ID: "mcp-workspace-a", Version: 1,
			Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-a"},
			Spec: aghconfig.MCPServer{
				Name: "linear", Transport: aghconfig.MCPServerTransportHTTP,
				URL: "https://workspace-a.linear.example/mcp",
			},
		},
		{
			ID: "mcp-workspace-b", Version: 1,
			Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-b"},
			Spec: aghconfig.MCPServer{
				Name: "linear", Transport: aghconfig.MCPServerTransportHTTP,
				URL: "https://workspace-b.linear.example/mcp",
			},
		},
	})
	state := &bootState{
		cfg: aghconfig.Config{MCPServers: []aghconfig.MCPServer{{
			Name: "linear", Transport: aghconfig.MCPServerTransportHTTP,
			URL: "https://global.linear.example/mcp",
		}}},
		mcpServerCatalog: catalog,
	}

	for _, tc := range []struct {
		resourceID  string
		workspaceID string
		url         string
	}{
		{
			resourceID:  "mcp-workspace-a",
			workspaceID: "workspace-a",
			url:         "https://workspace-a.linear.example/mcp",
		},
		{
			resourceID:  "mcp-workspace-b",
			workspaceID: "workspace-b",
			url:         "https://workspace-b.linear.example/mcp",
		},
	} {
		t.Run("Should resolve "+tc.workspaceID+" scoped server", func(t *testing.T) {
			t.Parallel()

			resolved, err := resolveDaemonMCPServer(state, toolspkg.SourceRef{
				Kind: toolspkg.SourceMCP, Owner: "linear", RawServerName: "linear",
				ResourceID: tc.resourceID, Scope: "workspace", WorkspaceID: tc.workspaceID,
			})
			if err != nil {
				t.Fatalf("resolveDaemonMCPServer(%s) error = %v", tc.workspaceID, err)
			}
			if resolved.Target.WorkspaceID != tc.workspaceID ||
				string(resolved.Target.Scope) != "workspace" || resolved.Server.URL != tc.url {
				t.Fatalf("resolveDaemonMCPServer(%s) = %#v", tc.workspaceID, resolved)
			}
		})
	}

	t.Run("Should fall back to global scope without a resource identity", func(t *testing.T) {
		t.Parallel()

		global, err := resolveDaemonMCPServer(state, toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: "linear", RawServerName: "linear",
		})
		if err != nil {
			t.Fatalf("resolveDaemonMCPServer(global) error = %v", err)
		}
		if string(global.Target.Scope) != "global" || global.Target.WorkspaceID != "" ||
			global.Server.URL != "https://global.linear.example/mcp" {
			t.Fatalf("resolveDaemonMCPServer(global) = %#v", global)
		}
	})

	t.Run("Should preserve workspace identities in daemon MCP sources", func(t *testing.T) {
		t.Parallel()

		sources := daemonMCPSources(state)
		seen := map[string]string{}
		for _, source := range sources {
			if source.ResourceID != "" {
				seen[source.ResourceID] = source.WorkspaceID
			}
		}
		if seen["mcp-workspace-a"] != "workspace-a" || seen["mcp-workspace-b"] != "workspace-b" {
			t.Fatalf("daemonMCPSources() workspace identities = %#v", seen)
		}
	})
}
