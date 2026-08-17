package daemon

import (
	"context"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/resources"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestDaemonMCPServerResolverPreservesWorkspaceResourceIdentity(t *testing.T) {
	t.Parallel()

	catalog := newResourceCatalog(cloneDaemonMCPServer)
	catalog.Replace(1, []resources.Record[compozyconfig.MCPServer]{
		{
			ID: "mcp-workspace-a", Version: 1,
			Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-a"},
			Spec: compozyconfig.MCPServer{
				Name: "linear", Transport: compozyconfig.MCPServerTransportHTTP,
				URL: "https://workspace-a.linear.example/mcp",
			},
		},
		{
			ID: "mcp-workspace-b", Version: 1,
			Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-b"},
			Spec: compozyconfig.MCPServer{
				Name: "linear", Transport: compozyconfig.MCPServerTransportHTTP,
				URL: "https://workspace-b.linear.example/mcp",
			},
		},
	})
	state := &bootState{
		cfg: compozyconfig.Config{MCPServers: []compozyconfig.MCPServer{{
			Name: "linear", Transport: compozyconfig.MCPServerTransportHTTP,
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

			resolved, err := resolveDaemonMCPServer(t.Context(), state, toolspkg.SourceRef{
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

		global, err := resolveDaemonMCPServer(t.Context(), state, toolspkg.SourceRef{
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

func TestDaemonMCPServerResolverProjectsRemoteHeaderBindings(t *testing.T) {
	t.Parallel()

	t.Run("Should project only the owning workspace and server bindings", func(t *testing.T) {
		t.Parallel()

		db := openDaemonTestGlobalDB(t)
		for _, binding := range []extensionpkg.EnvBinding{
			{
				ExtensionName: "kit", WorkspaceID: "workspace-a", EnvName: "DEPLOYMENT_KEY",
				SecretRef: "vault:extensions/ws/workspace-a/kit/env/DEPLOYMENT_KEY",
				MCPServer: "deployment-api", HeaderName: "X-Deployment-Key",
				Kind: extensionpkg.ExtensionEnvBindingKind,
			},
			{
				ExtensionName: "kit", WorkspaceID: "workspace-a", EnvName: "OTHER_KEY",
				SecretRef: "vault:extensions/ws/workspace-a/kit/env/OTHER_KEY",
				MCPServer: "other-api", HeaderName: "X-Other-Key",
				Kind: extensionpkg.ExtensionEnvBindingKind,
			},
			{
				ExtensionName: "kit", WorkspaceID: "workspace-b", EnvName: "DEPLOYMENT_KEY",
				SecretRef: "vault:extensions/ws/workspace-b/kit/env/DEPLOYMENT_KEY",
				MCPServer: "deployment-api", HeaderName: "X-Deployment-Key",
				Kind: extensionpkg.ExtensionEnvBindingKind,
			},
		} {
			if err := db.PutEnvBinding(t.Context(), binding); err != nil {
				t.Fatalf("PutEnvBinding(%#v) error = %v", binding, err)
			}
		}

		catalog := newResourceCatalog(cloneDaemonMCPServer)
		catalog.Replace(1, []resources.Record[compozyconfig.MCPServer]{
			{
				ID: "mcp-deployment-workspace-a", Version: 1,
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-a"},
				Owner: resources.ResourceOwner{Kind: extensionResourceOwnerKind, ID: "kit"},
				Spec: compozyconfig.MCPServer{
					Name: "deployment-api", Transport: compozyconfig.MCPServerTransportHTTP,
					URL: "https://deployment.example.test/mcp",
				},
			},
		})
		state := &bootState{mcpServerCatalog: catalog, extensionEnvBindings: db.ExtensionEnvRepo}
		resolved, err := resolveDaemonMCPServer(t.Context(), state, toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: "deployment-api", RawServerName: "deployment-api",
			ResourceID: "mcp-deployment-workspace-a", Scope: "workspace", WorkspaceID: "workspace-a",
		})
		if err != nil {
			t.Fatalf("resolveDaemonMCPServer() error = %v", err)
		}
		wantRef := "vault:extensions/ws/workspace-a/kit/env/DEPLOYMENT_KEY"
		if len(resolved.Server.SecretHeaders) != 1 || resolved.Server.SecretHeaders["X-Deployment-Key"] != wantRef {
			t.Fatalf("SecretHeaders = %#v, want only workspace-a deployment-api ref", resolved.Server.SecretHeaders)
		}
		if resolved.Server.SecretHeaders["X-Other-Key"] != "" ||
			resolved.Server.SecretHeaders["X-Deployment-Key"] ==
				"vault:extensions/ws/workspace-b/kit/env/DEPLOYMENT_KEY" {
			t.Fatalf("SecretHeaders leaked sibling server or workspace: %#v", resolved.Server.SecretHeaders)
		}

		record := catalog.Snapshot()[0]
		if len(record.Spec.SecretHeaders) != 0 {
			t.Fatalf("catalog declaration mutated with projected refs: %#v", record.Spec.SecretHeaders)
		}

		t.Run("Should ignore stale remote bindings after the server becomes stdio", func(t *testing.T) {
			t.Parallel()

			stdioCatalog := newResourceCatalog(cloneDaemonMCPServer)
			stdioCatalog.Replace(2, []resources.Record[compozyconfig.MCPServer]{
				{
					ID: "mcp-deployment-workspace-a", Version: 2,
					Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-a"},
					Owner: resources.ResourceOwner{Kind: extensionResourceOwnerKind, ID: "kit"},
					Spec: compozyconfig.MCPServer{
						Name: "deployment-api", Transport: compozyconfig.MCPServerTransportStdio,
						Command: "deployment-mcp",
					},
				},
			})
			stdioState := &bootState{mcpServerCatalog: stdioCatalog, extensionEnvBindings: db.ExtensionEnvRepo}
			stdio, err := resolveDaemonMCPServer(t.Context(), stdioState, toolspkg.SourceRef{
				Kind: toolspkg.SourceMCP, Owner: "deployment-api", RawServerName: "deployment-api",
				ResourceID: "mcp-deployment-workspace-a", Scope: "workspace", WorkspaceID: "workspace-a",
			})
			if err != nil {
				t.Fatalf("resolveDaemonMCPServer(stdio) error = %v", err)
			}
			if len(stdio.Server.SecretHeaders) != 0 || stdio.Server.Command != "deployment-mcp" {
				t.Fatalf("resolved stdio server = %#v, want no projected remote headers", stdio.Server)
			}
		})
	})
}

func TestDaemonMCPProviderRecordsExtensionLaunchFailures(t *testing.T) {
	t.Parallel()

	t.Run("Should publish a failing provider exchange to the shared health registry", func(t *testing.T) {
		t.Parallel()

		catalog := newResourceCatalog(cloneDaemonMCPServer)
		catalog.Replace(1, []resources.Record[compozyconfig.MCPServer]{
			{
				ID: "mcp-kit-broken", Version: 1,
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				Owner: resources.ResourceOwner{Kind: extensionResourceOwnerKind, ID: "kit"},
				Spec: compozyconfig.MCPServer{
					Name: "broken", Transport: compozyconfig.MCPServerTransportStdio,
					Command: "compozy-mcp-command-that-does-not-exist",
				},
			},
		})
		state := &bootState{
			mcpServerCatalog: catalog,
			extensions: &fakeExtensionRuntime{getExt: &extensionpkg.Extension{
				Info: extensionpkg.ExtensionInfo{Name: "kit", Checksum: "generation-a"},
			}},
		}
		provider, _, err := (&Daemon{}).newDaemonMCPToolProvider(state)
		if err != nil {
			t.Fatalf("newDaemonMCPToolProvider() error = %v", err)
		}
		_, err = provider.List(context.Background(), toolspkg.Scope{Operator: true})
		if err != nil {
			t.Fatalf("provider.List() error = %v; discovery failures should degrade the source", err)
		}
		entries := state.mcpRuntimeHealth.Entries("kit", "", "generation-a")
		if len(entries) != 1 || entries[0].Key.ServerName != "broken" ||
			!strings.Contains(entries[0].Message, "does-not-exist") {
			t.Fatalf("runtime health entries = %#v, want broken server failure", entries)
		}
	})
}
