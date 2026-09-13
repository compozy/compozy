package daemon

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensionmcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/resources"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestDaemonMCPServerResolverPreservesWorkspaceResourceIdentity(t *testing.T) {
	t.Parallel()

	// Invariant: failed publication/auth cleanup restores the persisted override even after caller cancellation, and auth calls keep the exact owner.
	// Owner: daemon override commit boundary; canonical suite: mcp_server_resolver_test.go.
	t.Run("Should compensate failed MCP override publication and credential cleanup", func(t *testing.T) {
		t.Parallel()
		for _, phase := range []string{"publication", "auth", "cancellation", "success"} {
			t.Run(phase, func(t *testing.T) {
				t.Parallel()
				db := openDaemonTestGlobalDB(t)
				target := extensionmcp.Target{
					Extension:  "bundle",
					ProfileID:  store.DefaultProfileID,
					ServerName: "remote",
				}
				before, err := db.ExtensionMCP.Reserve(t.Context(), target, "bundle.remote", nil)
				if err != nil {
					t.Fatal(err)
				}
				authTarget := mcpauth.Target{Scope: mcpauth.ScopeUser, Owner: "extension:bundle", ServerName: "remote"}
				failure := errors.New("injected mutation boundary failure")
				auth := &mcpOverrideAuthRecorder{}
				if phase == "auth" {
					auth.deleteError = failure
				}
				ctx, cancel := context.WithCancel(t.Context())
				defer cancel()
				calls := 0
				publisher := toolMCPPublisherFunc(func(callCtx context.Context) error {
					calls++
					if calls > 1 && callCtx.Err() != nil {
						t.Fatal("rollback inherited canceled caller")
					}
					if calls == 1 && phase == "publication" {
						return failure
					}
					if calls == 1 && phase == "cancellation" {
						cancel()
						return ctx.Err()
					}
					return nil
				})
				adapter := settingsMCPExtensionDefinitions{
					state: &bootState{extensionMCP: db.ExtensionMCP, toolMCPResources: publisher},
					auth:  auth,
				}
				err = adapter.commitMCPExtensionOverride(
					ctx,
					authTarget,
					before,
					extensionmcp.Override{URL: "https://example.com/changed"},
				)
				if (err == nil) != (phase == "success") {
					t.Fatalf("wrong mutation outcome for %s: %v", phase, err)
				}
				records, readErr := db.ExtensionMCP.List(t.Context(), store.DefaultProfileID, "")
				if readErr != nil || len(records) != 1 {
					t.Fatalf("missing restored allocation: %v", readErr)
				}
				wantURL, wantCalls := "", 2
				if phase == "success" {
					wantURL, wantCalls = "https://example.com/changed", 1
				}
				if records[0].URL != wantURL || records[0].RuntimeName != before.RuntimeName || calls != wantCalls {
					t.Fatalf("wrong compensation: %#v, calls=%d", records[0], calls)
				}
				for _, seen := range auth.targets {
					if seen != authTarget {
						t.Fatalf("credential mutation crossed owner: %#v", seen)
					}
				}
			})
		}
	})

	// Invariant: Settings shows only effective owner-qualified extension rows, while every retained allocation protects its visible name.
	// Owner: daemon Settings adapter; canonical suite: mcp_server_resolver_test.go with real SQLite.
	t.Run("Should expose scoped extensions and protect disabled allocations from manual writes", func(t *testing.T) {
		t.Parallel()
		db := openDaemonTestGlobalDB(t)
		profiles, err := profilepkg.NewManager(
			profilepkg.WithStore(db),
			profilepkg.WithHomePaths(testHomePaths(t)),
			profilepkg.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatal(err)
		}
		profile, err := profiles.Create(t.Context(), profilepkg.CreateInput{Name: "marketing"})
		if err != nil {
			t.Fatal(err)
		}
		catalog := newResourceCatalog(cloneDaemonMCPServer)
		var published []resources.Record[compozyconfig.MCPServer]
		for _, tc := range []struct {
			name, profileID, workspaceID string
			scope                        resources.ResourceScope
		}{
			{"global", store.DefaultProfileID, "", resources.ResourceScope{Kind: resources.ResourceScopeKindUser}},
			{"local", store.DefaultProfileID, "workspace-a", resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-a"}},
			{"other-workspace", store.DefaultProfileID, "workspace-b", resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-b"}},
			{"named", profile.ID, "", resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: profile.ID}},
			{"disabled", profile.ID, "workspace-a", resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspaceProfile, ID: "workspace-a@pf:marketing"}},
		} {
			target := extensionmcp.Target{
				Extension:   "bundle",
				ProfileID:   tc.profileID,
				WorkspaceID: tc.workspaceID,
				ServerName:  tc.name,
			}
			allocation, err := db.ExtensionMCP.Reserve(t.Context(), target, "bundle."+tc.name, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err := db.ExtensionMCP.Update(
				t.Context(),
				target,
				extensionmcp.Override{URL: "https://example.com/override"},
			); err != nil {
				t.Fatal(err)
			}
			if tc.name == "disabled" {
				continue
			}
			published = append(
				published,
				resources.Record[compozyconfig.MCPServer]{
					ID:    tc.name,
					Owner: *extensionOwner("bundle"),
					Scope: tc.scope,
					Spec: compozyconfig.MCPServer{
						Name:        tc.name,
						RuntimeName: allocation.RuntimeName,
						Transport:   compozyconfig.MCPServerTransportHTTP,
						URL:         "https://example.com/override",
					},
				},
			)
		}
		catalog.Replace(1, published)
		adapter := settingsMCPExtensionDefinitions{
			state: &bootState{profiles: profiles, extensionMCP: db.ExtensionMCP, mcpServerCatalog: catalog},
		}
		// Invariant: effective-registry addressing inherits parent definitions but never sibling workspaces.
		for _, tc := range []struct {
			name, workspace, profile string
			scope                    settingspkg.ScopeKind
			found                    bool
		}{
			{"bundle.global", "workspace-a", "", settingspkg.ScopeWorkspace, true},
			{"bundle.local", "workspace-b", "", settingspkg.ScopeWorkspace, false},
			{"bundle.named", "workspace-a", "marketing", settingspkg.ScopeProfile, true},
		} {
			target, _, found, err := adapter.ResolveMCPExtensionDefinition(
				t.Context(),
				settingspkg.MCPAuthTargetRequest{
					Name:        tc.name,
					Scope:       tc.scope,
					WorkspaceID: tc.workspace,
					ProfileName: tc.profile,
				},
			)
			if err != nil || found != tc.found {
				t.Fatalf("wrong effective registry lookup for %#v: %v", tc, err)
			}
			if found &&
				((tc.profile == "" && target.Scope != mcpauth.ScopeUser) || (tc.profile != "" && target.WorkspaceID != tc.profile)) {
				t.Fatalf("inherited definition changed credential scope: %#v", target)
			}
		}
		// Invariant: native source selectors cannot borrow a sibling workspace or foreign profile definition.
		for _, tc := range []struct{ name, profileID, workspaceID, wantScope string }{
			{"bundle.global", store.DefaultProfileID, "workspace-a", "user"},
			{"bundle.local", store.DefaultProfileID, "workspace-b", ""},
			{"bundle.named", profile.ID, "workspace-a", "profile"},
			{"bundle.global", profile.ID, "workspace-a", ""},
			{"bundle.named", store.DefaultProfileID, "workspace-a", ""},
		} {
			resolved, err := resolveDaemonMCPServer(t.Context(), adapter.state, toolspkg.SourceRef{
				RawServerName: tc.name, ProfileID: tc.profileID, WorkspaceID: tc.workspaceID,
			})
			if tc.wantScope == "" {
				if err == nil {
					t.Fatalf("native scope isolation failed for %#v", tc)
				}
				continue
			}
			if err != nil || string(resolved.Target.Scope) != tc.wantScope ||
				resolved.Target.Owner != "extension:bundle" {
				t.Fatalf("native inherited credential target for %#v: %#v, %v", tc, resolved.Target, err)
			}
		}
		for _, tc := range []struct {
			scope              settingspkg.ScopeKind
			workspace, profile string
			names              []string
		}{
			{settingspkg.ScopeUser, "", "", []string{"global"}},
			{settingspkg.ScopeWorkspace, "workspace-a", "", []string{"global", "local"}},
			{settingspkg.ScopeProfile, "workspace-a", "marketing", []string{"named"}},
		} {
			definitions, err := adapter.ListMCPExtensionDefinitions(
				t.Context(),
				settingspkg.MCPAuthTargetRequest{Scope: tc.scope, WorkspaceID: tc.workspace, ProfileName: tc.profile},
			)
			if err != nil || len(definitions) != len(tc.names) {
				t.Fatalf("wrong visible definitions: %#v %v", definitions, err)
			}
			for _, definition := range definitions {
				if !slices.Contains(tc.names, definition.Server.Name) ||
					definition.Target.Owner != "extension:bundle" ||
					definition.Server.RuntimeName != "bundle."+definition.Server.Name ||
					definition.Override.URL != "https://example.com/override" {
					t.Fatalf("wrong Settings identity/override: %#v", definition)
				}
			}
		}
		for _, tc := range []struct {
			scope                    settingspkg.ScopeKind
			workspace, profile, name string
			collision                bool
		}{
			{settingspkg.ScopeUser, "", "", "bundle.disabled", true},
			{settingspkg.ScopeWorkspace, "workspace-a", "", "bundle.disabled", true},
			{settingspkg.ScopeWorkspace, "workspace-b", "", "bundle.disabled", false},
			{settingspkg.ScopeProfile, "", "marketing", "bundle.disabled", true},
			{settingspkg.ScopeProfile, "workspace-a", "marketing", "bundle.disabled", true},
			{settingspkg.ScopeProfile, "workspace-b", "marketing", "bundle.disabled", false},
			{settingspkg.ScopeProfile, "workspace-a", "marketing", "bundle.global", false},
			{settingspkg.ScopeWorkspace, "workspace-b", "", "bundle.global", true},
		} {
			err := adapter.ValidateManualMCPName(
				t.Context(),
				settingspkg.MCPAuthTargetRequest{
					Scope:       tc.scope,
					WorkspaceID: tc.workspace,
					ProfileName: tc.profile,
					Name:        tc.name,
				},
			)
			if errors.Is(err, extensionmcp.ErrNameTaken) != tc.collision || (err != nil && !tc.collision) {
				t.Fatalf("wrong manual collision for %#v: %v", tc, err)
			}
		}
	})
	// Invariant: profile resource IDs resolve to the same named OAuth target used by Settings.
	// Owner: daemon MCP resolver; canonical suite: mcp_server_resolver_test.go.
	t.Run("Should use the authoritative profile name for OAuth credentials", func(t *testing.T) {
		t.Parallel()
		db := openDaemonTestGlobalDB(t)
		profiles, err := profilepkg.NewManager(profilepkg.WithStore(db),
			profilepkg.WithHomePaths(testHomePaths(t)), profilepkg.WithLogger(discardLogger()))
		if err != nil {
			t.Fatal(err)
		}
		profile, err := profiles.Create(t.Context(), profilepkg.CreateInput{Name: "marketing"})
		if err != nil {
			t.Fatal(err)
		}
		catalog := newResourceCatalog(cloneDaemonMCPServer)
		catalog.Replace(1, []resources.Record[compozyconfig.MCPServer]{{
			ID: "profile-server", Owner: *extensionOwner("github"),
			Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: profile.ID},
			Spec: compozyconfig.MCPServer{Name: "github", Transport: compozyconfig.MCPServerTransportHTTP,
				URL: "https://example.com/mcp"},
		}})
		state := &bootState{profiles: profiles, mcpServerCatalog: catalog}
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, RawServerName: "github", ResourceID: "profile-server"}
		resolved, err := resolveDaemonMCPServer(t.Context(), state, source)
		if err != nil {
			t.Fatal(err)
		}
		if string(resolved.Target.Scope) != "profile" || resolved.Target.WorkspaceID != "marketing" ||
			resolved.Target.Owner != "extension:github" || resolved.Target.ServerName != "github" {
			t.Fatalf("runtime target differs from the named Settings identity: %#v", resolved.Target)
		}
		settingsTarget, _, found, err := (settingsMCPExtensionDefinitions{state: state}).ResolveMCPExtensionDefinition(
			t.Context(), settingspkg.MCPAuthTargetRequest{
				Scope: settingspkg.ScopeProfile, ProfileName: "marketing", Name: "github", Owner: "extension:github",
			},
		)
		if err != nil || !found || settingsTarget != resolved.Target {
			t.Fatalf("Settings and runtime disagree on the profile credential identity: %v", err)
		}
		state.profiles = nil
		if _, err := resolveDaemonMCPServer(t.Context(), state, source); err == nil {
			t.Fatal("missing profile catalog silently became an ID-based OAuth credential target")
		}
	})
	// Invariant: runtime auth identity comes from the resource owner, independently of its display name.
	// Owner: daemon MCP resolver; canonical suite: mcp_server_resolver_test.go.
	t.Run("Should isolate extension credentials from a manual server with the same name", func(t *testing.T) {
		t.Parallel()
		catalog := newResourceCatalog(cloneDaemonMCPServer)
		server := compozyconfig.MCPServer{Name: "github", Transport: compozyconfig.MCPServerTransportHTTP,
			URL: "https://example.com/mcp"}
		extensionServer := server
		extensionServer.RuntimeName = "github.github"
		catalog.Replace(1, []resources.Record[compozyconfig.MCPServer]{
			{ID: "manual", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser}, Spec: server},
			{
				ID:    "extension",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				Spec:  extensionServer,
				Owner: *extensionOwner("github"),
			},
		})
		state := &bootState{mcpServerCatalog: catalog}
		keys := make(map[string]bool)
		for _, entry := range []struct{ id, owner string }{{"manual", "manual"}, {"extension", "extension:github"}} {
			resolved, err := resolveDaemonMCPServer(t.Context(), state, toolspkg.SourceRef{
				Kind: toolspkg.SourceMCP, RawServerName: "github", ResourceID: entry.id,
			})
			if err != nil || resolved.Target.Owner != entry.owner {
				t.Fatalf("resource %s lost its credential owner: %v", entry.id, err)
			}
			key, err := resolved.Target.Key()
			if err != nil || keys[key] {
				t.Fatalf("manual and extension auth targets collided: %v", err)
			}
			keys[key] = true
		}
		// Invariant: native diagnostics without resource IDs retain owner-qualified credentials.
		// Owner: daemon MCP resolver; canonical suite: this file.
		for _, tc := range []struct{ name, owner, want string }{
			{"github", "", "manual"}, {"github", "manual", "manual"},
			{"github", "extension:github", "extension:github"},
			{"github.github", "", "extension:github"},
			{"github", "extension:other", ""},
			{"github.github", "manual", ""},
		} {
			resolved, err := resolveDaemonMCPServer(t.Context(), state, toolspkg.SourceRef{
				Kind: toolspkg.SourceMCP, RawServerName: tc.name, MCPDefinitionOwner: tc.owner,
			})
			if tc.want == "" {
				if err == nil {
					t.Fatalf("unexpected definition for %#v", tc)
				}
				continue
			}
			if err != nil || resolved.Target.Owner != tc.want || resolved.Target.ServerName != "github" {
				t.Fatalf("native lookup %#v = %#v, %v", tc, resolved.Target, err)
			}
		}
		// Invariant: Settings addresses extension definitions by owner or unique runtime name, within scope.
		// Owner: daemon Settings catalog adapter; canonical suite: mcp_server_resolver_test.go.
		for _, tc := range []struct {
			name, owner string
			found       bool
		}{
			{"github", "extension:github", true}, {"github.github", "", true},
			{"github", "", false}, {"github.github", "manual", false},
			{"github", "extension:other", false},
		} {
			target, _, found, err := (settingsMCPExtensionDefinitions{state: state}).ResolveMCPExtensionDefinition(
				t.Context(),
				settingspkg.MCPAuthTargetRequest{Scope: settingspkg.ScopeUser, Name: tc.name, Owner: tc.owner},
			)
			if err != nil || found != tc.found ||
				(found && (target.Owner != "extension:github" || target.ServerName != "github")) {
				t.Fatalf(
					"Settings target %q owner %q resolved incorrectly: found=%t, %v",
					tc.name,
					tc.owner,
					found,
					err,
				)
			}
		}
		// Invariant: source enumeration preserves colliding logical names via their allocations.
		// Owner: daemon source routing; canonical suite: mcp_server_resolver_test.go.
		sources, err := daemonMCPSources(t.Context(), state)
		if err != nil || len(sources) != 2 {
			t.Fatalf("same-name sources collapsed: %#v %v", sources, err)
		}
		for _, source := range sources {
			if source.ResourceID != "extension" {
				continue
			}
			resolved, err := resolveDaemonMCPServer(t.Context(), state, source)
			if err != nil || source.RawServerName != "github.github" || resolved.Target.ServerName != "github" ||
				resolved.Target.Owner != "extension:github" {
				t.Fatalf("runtime name entered credential identity: %#v %v", resolved.Target, err)
			}
		}
		catalog.Replace(2, []resources.Record[compozyconfig.MCPServer]{
			{
				ID:    "extension-default",
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: store.DefaultProfileID},
				Spec:  extensionServer,
				Owner: *extensionOwner("github"),
			},
		})
		target, _, found, err := (settingsMCPExtensionDefinitions{state: state}).ResolveMCPExtensionDefinition(
			t.Context(),
			settingspkg.MCPAuthTargetRequest{Scope: settingspkg.ScopeUser, Name: "github", Owner: "extension:github"},
		)
		if err != nil || !found || string(target.Scope) != "user" || target.WorkspaceID != "" {
			t.Fatalf("default profile projection did not preserve the public user-scope credential identity: %v", err)
		}
	})

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

	t.Run("Should fall back to user scope without a resource identity", func(t *testing.T) {
		t.Parallel()

		global, err := resolveDaemonMCPServer(t.Context(), state, toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: "linear", RawServerName: "linear",
		})
		if err != nil {
			t.Fatalf("resolveDaemonMCPServer(global) error = %v", err)
		}
		if string(global.Target.Scope) != "user" || global.Target.WorkspaceID != "" ||
			global.Server.URL != "https://global.linear.example/mcp" {
			t.Fatalf("resolveDaemonMCPServer(global) = %#v", global)
		}
	})

	t.Run("Should preserve workspace identities in daemon MCP sources", func(t *testing.T) {
		t.Parallel()

		sources, err := daemonMCPSources(t.Context(), state)
		if err != nil {
			t.Fatalf("daemonMCPSources() error = %v", err)
		}
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
				Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
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

type mcpOverrideAuthRecorder struct {
	targets     []mcpauth.Target
	deleteError error
}

func (r *mcpOverrideAuthRecorder) MCPAuthInvalidate(target mcpauth.Target) error {
	r.targets = append(r.targets, target)
	return nil
}
func (r *mcpOverrideAuthRecorder) MCPAuthDeleteState(_ context.Context, target mcpauth.Target) error {
	r.targets = append(r.targets, target)
	return r.deleteError
}
