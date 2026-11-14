package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/extensionmcp"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/resources"
	settingspkg "github.com/compozy/compozy/internal/settings"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/vault"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestDaemonMCPServerResolverPreservesWorkspaceResourceIdentity(t *testing.T) {
	t.Parallel()

	// Invariant: deleting a published declaration cannot resurrect its stale boot snapshot.
	// Owner: daemon MCP identity resolver; canonical suite: resource identity.
	t.Run("Should reject a removed published server despite stale boot config", func(t *testing.T) {
		t.Parallel()
		state := &bootState{
			cfg: compozyconfig.Config{
				MCPServers: []compozyconfig.MCPServer{{Name: "removed", Command: "old"}},
			},
			mcpServerCatalog: newResourceCatalog(cloneDaemonMCPServer),
			toolMCPResources: toolMCPPublisherFunc(func(context.Context) error { return nil }),
		}
		_, err := resolveDaemonMCPServer(
			t.Context(),
			state,
			toolspkg.SourceRef{Kind: toolspkg.SourceMCP, RawServerName: "removed"},
			"",
		)
		if err == nil || !strings.Contains(err.Error(), "unavailable") {
			t.Fatalf("removed source resolved: %v", err)
		}
	})

	t.Run("Should apply the active profile before workspace overrides without moving credentials", func(t *testing.T) {
		t.Parallel()
		db := openDaemonTestGlobalDB(t)
		home := testHomePaths(t)
		profiles, err := profilepkg.NewManager(
			profilepkg.WithStore(db),
			profilepkg.WithHomePaths(home),
			profilepkg.WithLogger(discardLogger()),
		)
		if err != nil {
			t.Fatal(err)
		}
		root := t.TempDir()
		writeMCPDeclarationFile(t, home.ConfigFile, `[[mcp_servers]]
name = "docs"
command = "user-server"
`)
		writeMCPDeclarationFile(
			t,
			filepath.Join(root, compozyconfig.DirName, compozyconfig.ConfigName),
			`[[mcp_servers]]
name = "docs"
args = ["workspace"]
`,
		)
		resolver, err := workspacepkg.NewResolver(db, workspacepkg.WithHomePaths(home),
			workspacepkg.WithProfileAvailabilityChecker(profiles),
			workspacepkg.WithConfigLoader(func(root string) (compozyconfig.Config, error) {
				return compozyconfig.LoadForHome(home, compozyconfig.WithWorkspaceRoot(root))
			}),
			workspacepkg.WithProfileConfigLoader(func(root, profile string) (compozyconfig.Config, error) {
				return compozyconfig.LoadForHome(
					home,
					compozyconfig.WithWorkspaceRoot(root),
					compozyconfig.WithProfile(profile),
				)
			}),
		)
		if err != nil {
			t.Fatal(err)
		}
		workspace, err := resolver.ResolveOrRegister(t.Context(), root)
		if err != nil {
			t.Fatal(err)
		}
		record := resources.Record[compozyconfig.MCPServer]{ID: "workspace-docs", Source: toolMCPSyncActor().Source,
			Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: workspace.ID},
			Spec:  compozyconfig.MCPServer{Name: "docs", Command: "user-server", Args: []string{"workspace"}},
		}
		catalog := newResourceCatalog(cloneDaemonMCPServer)
		catalog.Replace(1, []resources.Record[compozyconfig.MCPServer]{record})
		state := &bootState{profiles: profiles, workspaceResolver: resolver, mcpServerCatalog: catalog}
		for _, name := range []string{"marketing", "sales"} {
			profile, err := profiles.Create(t.Context(), profilepkg.CreateInput{Name: name})
			if err != nil {
				t.Fatal(err)
			}
			want := "user-server"
			if name == "marketing" {
				want = "profile-server"
				writeMCPDeclarationFile(
					t,
					filepath.Join(home.ProfilesDir, name, compozyconfig.ConfigName),
					`[[mcp_servers]]
name = "docs"
command = "profile-server"
`,
				)
			}
			resolved, err := resolveDaemonMCPServer(t.Context(), state, toolspkg.SourceRef{
				Kind: toolspkg.SourceMCP, ResourceID: record.ID, RawServerName: "docs", ProfileID: profile.ID,
			}, "")
			if err != nil || resolved.Server.Command != want ||
				!slices.Equal(resolved.Server.Args, []string{"workspace"}) {
				t.Fatalf("%s server = %+v, %v", name, resolved.Server, err)
			}
			if resolved.Target.Scope != mcpauth.ScopeWorkspace || resolved.Target.WorkspaceID != workspace.ID {
				t.Fatalf("credential owner moved for %s: %+v", name, resolved.Target)
			}
		}
	})

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
			{"global", "workspace-a", "", settingspkg.ScopeWorkspace, true},
			{"local", "workspace-b", "", settingspkg.ScopeWorkspace, false},
			{"named", "workspace-a", "marketing", settingspkg.ScopeProfile, true},
			{"bundle.global", "workspace-a", "", settingspkg.ScopeWorkspace, false},
			{"bundle.named", "workspace-a", "marketing", settingspkg.ScopeProfile, false},
		} {
			target, _, found, err := adapter.ResolveMCPExtensionDefinition(
				t.Context(),
				settingspkg.MCPAuthTargetRequest{
					Name:        tc.name,
					Owner:       "extension:bundle",
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
			{"global", store.DefaultProfileID, "workspace-a", "user"},
			{"local", store.DefaultProfileID, "workspace-b", ""},
			{"named", profile.ID, "workspace-a", "profile"},
			{"global", profile.ID, "workspace-a", ""},
			{"named", store.DefaultProfileID, "workspace-a", ""},
			{"bundle.global", store.DefaultProfileID, "workspace-a", ""},
			{"bundle.named", profile.ID, "workspace-a", ""},
		} {
			resolved, err := resolveDaemonMCPServer(t.Context(), adapter.state, toolspkg.SourceRef{
				RawServerName: tc.name,
				ProfileID:     tc.profileID,
				WorkspaceID:   tc.workspaceID,
			}, "extension:bundle",
			)
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
			{settingspkg.ScopeWorkspace, "workspace-a", "", "bundle.named", false},
			{settingspkg.ScopeUser, "", "", "bundle.named", true},
			{settingspkg.ScopeProfile, "", "marketing", "bundle.named", true},
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
		resolved, err := resolveDaemonMCPServer(t.Context(), state, source, "")
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
		if _, err := resolveDaemonMCPServer(t.Context(), state, source, ""); err == nil {
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
			}, "")
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
			{"github.github", "", ""}, {"github.github", "extension:github", ""},
			{"github", "extension:other", ""},
			{"github.github", "manual", ""},
		} {
			resolved, err := resolveDaemonMCPServer(t.Context(), state, toolspkg.SourceRef{
				Kind: toolspkg.SourceMCP, RawServerName: tc.name,
			}, tc.owner,
			)
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
		// Invariant: public Settings requires the explicit extension owner and logical manifest name.
		// Owner: daemon Settings catalog adapter; canonical suite: mcp_server_resolver_test.go.
		for _, tc := range []struct {
			name, owner string
			found       bool
		}{
			{"github", "extension:github", true}, {"github.github", "extension:github", false}, {"github.github", "", false},
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
			resolved, err := resolveDaemonMCPServer(t.Context(), state, source, "")
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
			}, "")
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
		}, "")
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
		}, "")
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
			}, "")
			if err != nil {
				t.Fatalf("resolveDaemonMCPServer(stdio) error = %v", err)
			}
			if len(stdio.Server.SecretHeaders) != 0 || stdio.Server.Command != "deployment-mcp" {
				t.Fatalf("resolved stdio server = %#v, want no projected remote headers", stdio.Server)
			}
		})
	})
}

func TestMCPSourceResourceFindsExtensionServersByName(t *testing.T) {
	t.Parallel()

	server := compozyconfig.MCPServer{Name: "linear", Transport: compozyconfig.MCPServerTransportHTTP,
		URL: "https://example.com/mcp"}
	extensionRecord := func(id, extension string) resources.Record[compozyconfig.MCPServer] {
		return resources.Record[compozyconfig.MCPServer]{
			ID: id, Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			Spec: server, Owner: *extensionOwner(extension),
		}
	}
	lookup := func(t *testing.T, state *bootState) (mcpauth.Target, error) {
		t.Helper()
		resolved, err := resolveDaemonMCPServer(t.Context(), state, toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, RawServerName: "linear",
		}, "")
		return resolved.Target, err
	}
	catalogOf := func(records ...resources.Record[compozyconfig.MCPServer]) *resourceCatalog[compozyconfig.MCPServer] {
		catalog := newResourceCatalog(cloneDaemonMCPServer)
		catalog.Replace(1, records)
		return catalog
	}

	// Invariant: native diagnostics find an extension server by name, and never shadow the operator's own.
	// Owner: daemon MCP resolver; canonical suite: mcp_server_resolver_test.go.
	t.Run("Should resolve the only extension definition without an owner", func(t *testing.T) {
		t.Parallel()
		target, err := lookup(t, &bootState{mcpServerCatalog: catalogOf(extensionRecord("ext", "linear"))})
		if err != nil || target.Owner != "extension:linear" {
			t.Fatalf("lookup = %#v, %v", target, err)
		}
	})
	t.Run("Should prefer a server declared in provider config over an extension", func(t *testing.T) {
		t.Parallel()
		state := &bootState{mcpServerCatalog: catalogOf(extensionRecord("ext", "linear"))}
		state.cfg.Providers = map[string]compozyconfig.ProviderConfig{
			"claude": {MCPServers: []compozyconfig.MCPServer{server}},
		}
		target, err := lookup(t, state)
		if err != nil || target.Owner != mcpDefinitionOwnerManual {
			t.Fatalf("lookup = %#v, %v", target, err)
		}
	})
	t.Run("Should require an owner when two extensions declare the name", func(t *testing.T) {
		t.Parallel()
		_, err := lookup(t, &bootState{mcpServerCatalog: catalogOf(
			extensionRecord("ext-a", "linear-a"), extensionRecord("ext-b", "linear-b"),
		)})
		if err == nil || !strings.Contains(err.Error(), "explicit owner") {
			t.Fatalf("ambiguous lookup error = %v", err)
		}
	})
	t.Run("Should keep answering with the manual server when extensions tie", func(t *testing.T) {
		t.Parallel()
		manual := resources.Record[compozyconfig.MCPServer]{
			ID: "manual", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser}, Spec: server,
		}
		target, err := lookup(t, &bootState{mcpServerCatalog: catalogOf(
			extensionRecord("ext-a", "linear-a"), extensionRecord("ext-b", "linear-b"), manual,
		)})
		if err != nil || target.Owner != mcpDefinitionOwnerManual {
			t.Fatalf("lookup = %#v, %v", target, err)
		}
		state := &bootState{mcpServerCatalog: catalogOf(
			extensionRecord("ext-a", "linear-a"), extensionRecord("ext-b", "linear-b"),
		)}
		state.cfg.MCPServers = []compozyconfig.MCPServer{server}
		target, err = lookup(t, state)
		if err != nil || target.Owner != mcpDefinitionOwnerManual {
			t.Fatalf("config lookup = %#v, %v", target, err)
		}
	})
	t.Run("Should not let an unreadable extension record block the manual server", func(t *testing.T) {
		t.Parallel()
		broken := extensionRecord("ext", "linear")
		broken.Scope = resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspaceProfile, ID: "invalid"}
		manual := resources.Record[compozyconfig.MCPServer]{
			ID: "manual", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser}, Spec: server,
		}
		target, err := lookup(t, &bootState{mcpServerCatalog: catalogOf(broken, manual)})
		if err != nil || target.Owner != mcpDefinitionOwnerManual {
			t.Fatalf("lookup = %#v, %v", target, err)
		}
		if _, err = lookup(t, &bootState{mcpServerCatalog: catalogOf(broken)}); err == nil ||
			!strings.Contains(err.Error(), "workspace-profile") {
			t.Fatalf("lookup without a manual server error = %v", err)
		}
	})
	t.Run("Should not resolve an extension enabled for another profile", func(t *testing.T) {
		t.Parallel()
		record := extensionRecord("ext", "linear")
		record.Scope = resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: "profile-other"}
		_, err := resolveDaemonMCPServer(t.Context(), &bootState{mcpServerCatalog: catalogOf(record)},
			toolspkg.SourceRef{Kind: toolspkg.SourceMCP, RawServerName: "linear", ProfileID: "profile-mine"}, "")
		if err == nil || !strings.Contains(err.Error(), "is unavailable") {
			t.Fatalf("name lookup crossed the profile boundary: %v", err)
		}
	})
}

type mcpGrantAuthStatus struct {
	status map[string]toolspkg.MCPAuthStatus
	err    error
}

func (s mcpGrantAuthStatus) Status(
	_ context.Context, source toolspkg.SourceRef, _ string,
) (toolspkg.MCPAuthStatus, error) {
	return s.status[source.ResourceID], s.err
}

func TestOperatorConsentedMCPSourceGrants(t *testing.T) {
	t.Parallel()

	db := openDaemonTestGlobalDB(t)
	profiles, err := profilepkg.NewManager(profilepkg.WithStore(db),
		profilepkg.WithHomePaths(testHomePaths(t)), profilepkg.WithLogger(discardLogger()))
	if err != nil {
		t.Fatal(err)
	}
	open, err := profiles.Create(t.Context(), profilepkg.CreateInput{Name: "open"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := profiles.Create(t.Context(), profilepkg.CreateInput{Name: "other"})
	if err != nil {
		t.Fatal(err)
	}
	openScope := resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: open.ID}
	otherScope := resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: other.ID}
	named := func(name string, oauth bool) compozyconfig.MCPServer {
		server := compozyconfig.MCPServer{
			Name: name, Transport: compozyconfig.MCPServerTransportHTTP, URL: "https://example.com/mcp",
		}
		if oauth {
			server.Auth = compozyconfig.MCPAuthConfig{Registration: compozyconfig.MCPAuthRegistrationAuto}
		}
		return server
	}
	catalog := newResourceCatalog(cloneDaemonMCPServer)
	catalog.Replace(1, []resources.Record[compozyconfig.MCPServer]{
		{ID: "ext", Scope: openScope, Owner: *extensionOwner("lab"), Spec: named("lab-mcp", true)},
		{ID: "plain", Scope: openScope, Owner: *extensionOwner("plain"), Spec: named("plain", false)},
		{
			ID:    "header",
			Scope: openScope,
			Owner: *extensionOwner("header"),
			Spec: compozyconfig.MCPServer{
				Name:      "header",
				Transport: "http",
				URL:       "https://example.com/mcp",
				Headers:   map[string]string{"X-Key": "value"},
			},
		},
		{ID: "manual", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser}, Spec: compozyconfig.MCPServer{Name: "manual", Command: "fixture", DefinitionSource: filepath.Join(testHomePaths(t).HomeDir, compozyconfig.MCPJSONName)}},
		{ID: "foreign", Scope: otherScope, Owner: *extensionOwner("foreign"), Spec: named("foreign", true)},
		{ID: "logged-out", Scope: openScope, Owner: *extensionOwner("logged"), Spec: named("logged-out", true)},
	})
	present := toolspkg.MCPAuthStatus{Status: "expired", TokenPresent: true}
	authed := mcpGrantAuthStatus{status: map[string]toolspkg.MCPAuthStatus{
		"ext": present, "plain": present, "header": present, "manual": present, "foreign": present,
		"logged-out": {Status: "needs_login"},
	}}
	state := func(status toolspkg.MCPAuthStatusProvider) *bootState {
		return &bootState{mcpServerCatalog: catalog, mcpAuthStatus: status, profiles: profiles}
	}
	scope := toolspkg.Scope{ProfileID: open.ID}
	descriptor := toolspkg.Descriptor{
		ID: "mcp__lab_mcp__echo", Visibility: toolspkg.VisibilityModel, Risk: toolspkg.RiskOpenWorld,
		Backend:     toolspkg.BackendRef{Kind: toolspkg.BackendMCP, MCPServer: "lab_mcp", MCPTool: "echo"},
		InputSchema: json.RawMessage(`{"type":"object"}`),
		Source: toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: "lab_mcp", RawServerName: "lab-mcp", RawToolName: "echo",
			ResourceID: "ext",
		},
	}

	// Invariant (UT-001/002/003): published personal and enabled extension sources grant consent
	// independently of token availability, without reaching another profile.
	// Owner: daemon MCP source grants; canonical suite: mcp_server_resolver_test.go.
	t.Run("Should grant personal and enabled extension sources independently of OAuth", func(t *testing.T) {
		t.Parallel()
		grants, err := operatorConsentedSourceGrants(t.Context(), state(authed), scope)
		if err != nil {
			t.Fatal(err)
		}
		want := []toolspkg.SourceGrant{
			{Kind: toolspkg.SourceMCP, Owner: "lab-mcp", ResourceID: "ext"},
			{Kind: toolspkg.SourceMCP, Owner: "plain", ResourceID: "plain"},
			{Kind: toolspkg.SourceMCP, Owner: "header", ResourceID: "header"},
			{Kind: toolspkg.SourceMCP, Owner: "manual", ResourceID: "manual"},
			{Kind: toolspkg.SourceMCP, Owner: "logged-out", ResourceID: "logged-out"},
		}
		if !slices.Equal(grants, want) {
			t.Fatalf("grants = %#v, want %#v", grants, want)
		}
	})
	t.Run("Should not fail policy when one catalog record cannot be enumerated", func(t *testing.T) {
		t.Parallel()
		broken := newResourceCatalog(cloneDaemonMCPServer)
		broken.Replace(1, []resources.Record[compozyconfig.MCPServer]{{
			ID:    "broken",
			Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspaceProfile, ID: "invalid"},
			Owner: *extensionOwner("lab"), Spec: named("broken", true),
		}})
		grants, err := operatorConsentedSourceGrants(t.Context(),
			&bootState{mcpServerCatalog: broken, mcpAuthStatus: authed}, scope)
		if err != nil || len(grants) != 0 {
			t.Fatalf("grants = %#v, %v", grants, err)
		}
	})
	t.Run("Should preserve source consent when credential status cannot be read", func(t *testing.T) {
		t.Parallel()
		grants, err := operatorConsentedSourceGrants(t.Context(),
			state(mcpGrantAuthStatus{err: errors.New("store unavailable")}), scope)
		if err != nil || len(grants) != 5 {
			t.Fatalf("grants = %#v, %v", grants, err)
		}
	})
	t.Run("Should make source consent lift the disabled external default", func(t *testing.T) {
		t.Parallel()
		cfg := compozyconfig.Config{
			Tools:       compozyconfig.ToolsConfig{Enabled: true},
			Permissions: compozyconfig.PermissionsConfig{Mode: compozyconfig.PermissionModeApproveAll},
		}
		for _, tc := range []struct {
			name     string
			grants   nativeToolPolicyMCPSourceGrants
			callable bool
		}{
			{name: "without login", grants: func(context.Context, toolspkg.Scope) ([]toolspkg.SourceGrant, error) {
				return nil, nil
			}},
			{name: "with login", callable: true, grants: func(
				ctx context.Context, scope toolspkg.Scope,
			) ([]toolspkg.SourceGrant, error) {
				return operatorConsentedSourceGrants(ctx, state(authed), scope)
			}},
		} {
			resolver, err := newNativeToolPolicyResolver(nativeToolPolicyResolverDeps{
				Config: &cfg, MCPSourceGrants: tc.grants,
			})
			if err != nil {
				t.Fatal(err)
			}
			inputs, err := resolver.Resolve(t.Context(), scope)
			if err != nil {
				t.Fatal(err)
			}
			evaluator, err := toolspkg.NewEffectivePolicyEvaluator(
				inputs, toolspkg.ToolsetCatalog{}, []toolspkg.ToolID{descriptor.ID},
			)
			if err != nil {
				t.Fatal(err)
			}
			decision, err := evaluator.Evaluate(t.Context(), scope, descriptor)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Callable != tc.callable {
				t.Fatalf("%s: decision = %#v, want callable %t", tc.name, decision, tc.callable)
			}
			personal := descriptor
			personal.ID = "mcp__manual__echo"
			personal.Source = toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "manual", RawServerName: "manual", RawToolName: "echo", ResourceID: "manual"}
			personalDecision, err := evaluator.Evaluate(t.Context(), scope, personal)
			if err != nil || personalDecision.Callable != tc.callable {
				t.Fatalf("home source decision=%+v %v", personalDecision, err)
			}
			twin := descriptor
			twin.Source.RawServerName, twin.Source.ResourceID = "lab_mcp", "ext-twin"
			decision, err = evaluator.Evaluate(t.Context(), scope, twin)
			if err != nil {
				t.Fatal(err)
			}
			if decision.Callable {
				t.Fatalf("%s: a login reached another server sharing the owner: %#v", tc.name, decision)
			}
		}
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
		_, err = provider.List(t.Context(), toolspkg.Scope{Operator: true})
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

// Invariant (UT-005/006/007/008/009/019/031/032/033/034/036): consent binds the
// exact profile/workspace/declaring-file/definition, never a token or allocated name.
// Owner: daemon consent repository and write boundary; canonical suite: MCP source resolver.
func TestWorkspaceMCPConsent(t *testing.T) {
	t.Parallel()
	t.Run("Should approve only the observed definition and invalidate cached decisions", func(t *testing.T) {
		t.Parallel()
		state, req := newMCPConsentFixture(t)
		consent := settingsMCPConsent{state: state}
		waiting, err := consent.MCPConsent(t.Context(), req)
		if err != nil || !waiting.Waiting || waiting.Admitted {
			t.Fatalf("waiting=%+v %v", waiting, err)
		}
		epoch := state.toolProjectionEpoch.Current()
		if _, err = consent.ApproveMCP(t.Context(), req, "stale", false); err == nil {
			t.Fatal("stale digest approved")
		} else if failure, ok := errors.AsType[*settingspkg.MCPApprovalError](err); !ok || failure.Code != "mcp_approval_definition_changed" {
			t.Fatal(err)
		}
		records, err := state.mcpApprovalRecords(t.Context(), store.DefaultProfileID, req.WorkspaceID)
		if err != nil || len(records) != 0 {
			t.Fatalf("stale approval wrote %+v %v", records, err)
		}
		if _, err = consent.ApproveMCP(t.Context(), req, waiting.DefinitionDigest, false); err != nil {
			t.Fatal(err)
		}
		allowed, err := consent.MCPConsent(t.Context(), req)
		if err != nil || !allowed.Admitted || allowed.Waiting || state.toolProjectionEpoch.Current() <= epoch {
			t.Fatalf("allowed=%+v %v", allowed, err)
		}
		for _, change := range []func(*settingspkg.MCPConsentRequest){
			func(r *settingspkg.MCPConsentRequest) { r.Server.Args = []string{"changed"} },
			func(r *settingspkg.MCPConsentRequest) { r.Server.Env = map[string]string{"MODE": "changed"} },
			func(r *settingspkg.MCPConsentRequest) { r.Server.Headers = map[string]string{"X-Tenant": "changed"} },
			func(r *settingspkg.MCPConsentRequest) {
				r.Server.SecretEnv = map[string]string{"TOKEN": "vault:mcp/shared/changed"}
			},
			func(r *settingspkg.MCPConsentRequest) {
				r.Server.SecretHeaders = map[string]string{"Authorization": "vault:mcp/shared/header"}
			},
			func(r *settingspkg.MCPConsentRequest) { r.Server.ReadOnlyTools = []string{"echo"} },
			func(r *settingspkg.MCPConsentRequest) {
				r.Server.DefinitionSource = filepath.Join(filepath.Dir(r.Server.DefinitionSource), "agent", compozyconfig.MCPJSONName)
			},
		} {
			changed := req
			change(&changed)
			decision, err := consent.MCPConsent(t.Context(), changed)
			if err != nil || decision.Admitted || !decision.Waiting {
				t.Fatalf("changed=%+v %v", decision, err)
			}
		}
		// Removing a record does not remove its approval. Restoring identical bytes remains admitted.
		state.mcpServerCatalog.Replace(2, nil)
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: req.Server.Name, RawServerName: req.Server.Name, WorkspaceID: req.WorkspaceID, ProfileID: store.DefaultProfileID, ResourceID: "workspace-server"}
		if err := requireMCPSourceConsent(t.Context(), state, source); err == nil || !strings.Contains(err.Error(), "was removed from CompozyOS") {
			t.Fatalf("removed error=%v", err)
		}
		restored, err := consent.MCPConsent(t.Context(), req)
		if err != nil || !restored.Admitted {
			t.Fatalf("restored=%+v %v", restored, err)
		}
		if _, err = consent.ApproveMCP(t.Context(), req, "", true); err != nil {
			t.Fatal(err)
		}
		revoked, err := consent.MCPConsent(t.Context(), req)
		if err != nil || revoked.Admitted || !revoked.Waiting {
			t.Fatalf("revoked=%+v %v", revoked, err)
		}
	})
	t.Run("Should retain consent when a stable Vault reference rotates", func(t *testing.T) {
		t.Parallel()
		state, req := newMCPConsentFixture(t)
		db := state.registry.(*globaldb.GlobalDB)
		secretVault, err := vault.NewService(db.VaultRepo, vault.NewFileKeyProvider(t.TempDir(), nil))
		if err != nil {
			t.Fatal(err)
		}
		const ref = "vault:mcp/shared/stable"
		req.Server.SecretEnv = map[string]string{"TOKEN": ref}
		consent := settingsMCPConsent{state: state}
		digest, err := compozyconfig.MCPDefinitionDigest(req.Server)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := consent.ApproveMCP(t.Context(), req, digest, false); err != nil {
			t.Fatal(err)
		}
		for _, value := range []string{"first-private-value", "rotated-private-value"} {
			if _, err := secretVault.PutSecret(t.Context(), ref, "mcp", value); err != nil {
				t.Fatal(err)
			}
			decision, err := consent.MCPConsent(t.Context(), req)
			if err != nil || !decision.Admitted || decision.DefinitionDigest != digest {
				t.Fatalf("Vault rotation changed consent=%+v %v", decision, err)
			}
		}
	})
	t.Run("Should isolate profiles workspaces and default-profile project layers", func(t *testing.T) {
		t.Parallel()
		state, req := newMCPConsentFixture(t)
		consent := settingsMCPConsent{state: state}
		decision, err := consent.MCPConsent(t.Context(), req)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = consent.ApproveMCP(t.Context(), req, decision.DefinitionDigest, false); err != nil {
			t.Fatal(err)
		}
		profile, err := state.profiles.Create(t.Context(), profilepkg.CreateInput{Name: "work"})
		if err != nil {
			t.Fatal(err)
		}
		otherProfile := req
		otherProfile.ProfileName = profile.Name
		other, err := consent.MCPConsent(t.Context(), otherProfile)
		if err != nil || other.Admitted {
			t.Fatalf("profile leaked=%+v %v", other, err)
		}
		second, err := state.workspaceResolver.ResolveOrRegister(t.Context(), t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		otherWorkspace := req
		otherWorkspace.WorkspaceID = second.ID
		otherWorkspace.Server.DefinitionSource = filepath.Join(second.RootDir, compozyconfig.DirName, compozyconfig.MCPJSONName)
		other, err = consent.MCPConsent(t.Context(), otherWorkspace)
		if err != nil || other.Admitted {
			t.Fatalf("workspace leaked=%+v %v", other, err)
		}
		root, err := state.workspaceResolver.Resolve(t.Context(), req.WorkspaceID)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range []string{compozyconfig.MCPJSONName, compozyconfig.ConfigName} {
			projected := req
			projected.Scope = "workspace_profile"
			projected.ProfileName = "default"
			projected.Server.DefinitionSource = filepath.Join(root.RootDir, compozyconfig.DirName, compozyconfig.ProfilesDirName, "default", file)
			other, err = consent.MCPConsent(t.Context(), projected)
			if err != nil || other.Admitted || !other.Waiting {
				t.Fatalf("project default layer admitted=%+v %v", other, err)
			}
		}
	})
	t.Run("Should honor enabled external default without an approval", func(t *testing.T) {
		t.Parallel()
		state, req := newMCPConsentFixture(t)
		root, err := state.workspaceResolver.Resolve(t.Context(), req.WorkspaceID)
		if err != nil {
			t.Fatal(err)
		}
		writeMCPDeclarationFile(t, filepath.Join(root.RootDir, compozyconfig.DirName, compozyconfig.ConfigName), "[tools.policy]\nexternal_default = \"enabled\"\n")
		decision, err := (settingsMCPConsent{state: state}).MCPConsent(t.Context(), req)
		if err != nil || !decision.Admitted || decision.Waiting {
			t.Fatalf("enabled=%+v %v", decision, err)
		}
	})
	t.Run("Should commit the file before approval and preserve it on approval failure", func(t *testing.T) {
		t.Parallel()
		for _, fail := range []bool{false, true} {
			t.Run(fmt.Sprint(fail), func(t *testing.T) {
				state, req := newMCPConsentFixture(t)
				recorder := &recordingMCPApprovalRegistry{Registry: state.registry, MCPApprovalStore: state.registry.(MCPApprovalStore), path: req.Server.DefinitionSource, fail: fail}
				state.registry = recorder
				warning, err := (settingsMCPConsent{state: state}).RecordWorkspaceMCPWrite(t.Context(), settingspkg.WorkspaceMCPWrite{WorkspaceID: req.WorkspaceID, SourceID: req.Server.DefinitionSource, Server: req.Server, Write: func() error {
					return os.WriteFile(req.Server.DefinitionSource, []byte(`{"mcpServers":{"sentry":{"command":"fixture"}}}`), 0o600)
				}})
				if err != nil || recorder.puts != 1 || !recorder.filePresent || (warning != "") != fail {
					t.Fatalf("write result=%q %v recorder=%+v", warning, err, recorder)
				}
				decision, err := (settingsMCPConsent{state: state}).MCPConsent(t.Context(), req)
				if err != nil || decision.Admitted == fail {
					t.Fatalf("post-write=%+v %v", decision, err)
				}
			})
		}
	})
}

func newMCPConsentFixture(t *testing.T) (*bootState, settingspkg.MCPConsentRequest) {
	t.Helper()
	db := openDaemonTestGlobalDB(t)
	home := testHomePaths(t)
	profiles, err := profilepkg.NewManager(profilepkg.WithStore(db), profilepkg.WithHomePaths(home), profilepkg.WithLogger(discardLogger()))
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := workspacepkg.NewResolver(db, workspacepkg.WithHomePaths(home), workspacepkg.WithProfileAvailabilityChecker(profiles), workspacepkg.WithConfigLoader(func(root string) (compozyconfig.Config, error) {
		return compozyconfig.LoadForHome(home, compozyconfig.WithWorkspaceRoot(root))
	}), workspacepkg.WithProfileConfigLoader(func(root, profile string) (compozyconfig.Config, error) {
		return compozyconfig.LoadForHome(home, compozyconfig.WithWorkspaceRoot(root), compozyconfig.WithProfile(profile))
	}))
	if err != nil {
		t.Fatal(err)
	}
	workspace, err := resolver.ResolveOrRegister(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workspace.RootDir, compozyconfig.DirName, compozyconfig.MCPJSONName)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	server := compozyconfig.MCPServer{Name: "sentry", Command: "fixture", DefinitionSource: path}
	catalog := newResourceCatalog(cloneDaemonMCPServer)
	catalog.Replace(1, []resources.Record[compozyconfig.MCPServer]{{ID: "workspace-server", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: workspace.ID}, Spec: server}})
	return &bootState{registry: db, cfg: compozyconfig.DefaultWithHome(home), profiles: profiles, workspaceResolver: resolver, mcpServerCatalog: catalog, toolProjectionEpoch: mcppkg.NewProjectionEpoch()}, settingspkg.MCPConsentRequest{Scope: "workspace", WorkspaceID: workspace.ID, Server: server}
}

type recordingMCPApprovalRegistry struct {
	Registry
	MCPApprovalStore
	path        string
	fail        bool
	puts        int
	filePresent bool
}

func (r *recordingMCPApprovalRegistry) PutMCPServerApproval(ctx context.Context, a MCPApproval) error {
	r.puts++
	_, err := os.Stat(r.path)
	r.filePresent = err == nil
	if r.fail {
		return errors.New("injected approval failure")
	}
	return r.MCPApprovalStore.PutMCPServerApproval(ctx, a)
}

// Invariant (UT-004): enabled non-bundled extension tools carry consent; withdrawal
// removes it. Owner: daemon published tool catalog; canonical source-policy suite.
func TestOperatorConsentedExtensionTools(t *testing.T) {
	t.Parallel()
	catalog := newResourceCatalog(cloneToolSpec)
	catalog.Replace(1, []resources.Record[toolspkg.Tool]{{ID: "extension-ping", Owner: *extensionOwner("non-bundled"), Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser}}})
	state := &bootState{toolCatalog: catalog}
	grants, err := operatorConsentedSourceGrants(t.Context(), state, toolspkg.Scope{ProfileID: store.DefaultProfileID})
	if err != nil || len(grants) != 1 || grants[0].Kind != toolspkg.SourceExtension || grants[0].Owner != "non-bundled" {
		t.Fatalf("enabled grants=%+v %v", grants, err)
	}
	catalog.Replace(2, nil)
	grants, err = operatorConsentedSourceGrants(t.Context(), state, toolspkg.Scope{ProfileID: store.DefaultProfileID})
	if err != nil || len(grants) != 0 {
		t.Fatalf("disabled grants=%+v %v", grants, err)
	}
}

// Invariant (UT-003): profile token state affects availability, never source consent.
// Owner: daemon credential resolver; canonical MCP source resolver suite.
func TestMCPConsentDoesNotShareProfileTokens(t *testing.T) {
	t.Parallel()
	state, _ := newMCPConsentFixture(t)
	work, err := state.profiles.Create(t.Context(), profilepkg.CreateInput{Name: "work"})
	if err != nil {
		t.Fatal(err)
	}
	server := compozyconfig.MCPServer{Name: "linear", Transport: "http", URL: "https://example.com/mcp", Auth: compozyconfig.MCPAuthConfig{Registration: compozyconfig.MCPAuthRegistrationPreRegistered, ClientID: "client", IssuerURL: "https://example.com"}}
	state.mcpServerCatalog.Replace(2, []resources.Record[compozyconfig.MCPServer]{
		{ID: "default-linear", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: store.DefaultProfileID}, Spec: server},
		{ID: "work-linear", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindProfile, ID: work.ID}, Spec: server},
	})
	target := mcpauth.Target{Scope: mcpauth.ScopeProfile, WorkspaceID: "work", ServerName: "linear"}.Normalize()
	cfg, err := mcpauth.ServerConfigFromMCP(t.Context(), target, server, nil)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := mcpauth.ServerDefinitionFingerprint(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.registry.(mcpauth.TokenStore).SaveMCPAuthToken(t.Context(), mcpauth.TokenRecord{Target: target, DefinitionFingerprint: fingerprint, Issuer: server.Auth.IssuerURL, ClientID: "client", AccessToken: "profile-private", TokenType: "Bearer", ObtainedAt: approvalTimestamp(), UpdatedAt: approvalTimestamp()}); err != nil {
		t.Fatal(err)
	}
	grants, err := operatorConsentedSourceGrants(t.Context(), state, toolspkg.Scope{ProfileID: store.DefaultProfileID})
	if err != nil || len(grants) != 1 || grants[0].ResourceID != "default-linear" {
		t.Fatalf("grants=%+v %v", grants, err)
	}
	executor, err := mcppkg.NewMCPCallExecutor(newDaemonMCPServerResolver(state), mcppkg.WithTokenStore(state.registry.(mcpauth.TokenStore)))
	if err != nil {
		t.Fatal(err)
	}
	status, err := executor.Status(t.Context(), toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "linear", RawServerName: "linear", ResourceID: "default-linear", ProfileID: store.DefaultProfileID}, "manual")
	if err != nil || status.Status != "needs_login" || status.TokenPresent {
		t.Fatalf("default auth=%+v %v", status, err)
	}
}

// Invariant: manually edited agent sidecars remain visible and can be approved by
// their observed digest. Owner: daemon Settings published-definition boundary.
func TestSettingsPublishedAgentMCPConsent(t *testing.T) {
	t.Parallel()
	state, req := newMCPConsentFixture(t)
	workspace, err := state.workspaceResolver.Resolve(t.Context(), req.WorkspaceID)
	if err != nil {
		t.Fatal(err)
	}
	req.Server.DefinitionSource = filepath.Join(workspace.RootDir, compozyconfig.DirName, "agents", "worker", compozyconfig.MCPJSONName)
	state.mcpServerCatalog.Replace(2, []resources.Record[compozyconfig.MCPServer]{{ID: "agent-sentry", Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: req.WorkspaceID}, Spec: req.Server}})
	service, err := settingspkg.NewService(testHomePaths(t), settingspkg.Dependencies{WorkspaceResolver: state.workspaceResolver, ProfileResolver: state.profiles, MCPConsent: settingsMCPConsent{state: state}})
	if err != nil {
		t.Fatal(err)
	}
	collection, err := service.ListCollection(t.Context(), settingspkg.CollectionRequest{Collection: settingspkg.CollectionMCPServers, Scope: settingspkg.ScopeWorkspace, WorkspaceID: req.WorkspaceID})
	if err != nil || len(collection.MCPServers) != 1 || collection.MCPServers[0].Approval.State != "waiting" || collection.MCPServers[0].CallableInSessions {
		t.Fatalf("agent listing=%+v %v", collection.MCPServers, err)
	}
	_, err = service.SetMCPApproval(t.Context(), settingspkg.MCPAuthTargetRequest{Scope: settingspkg.ScopeWorkspace, WorkspaceID: req.WorkspaceID, Name: "sentry"}, collection.MCPServers[0].Approval.DefinitionDigest, false)
	if err != nil {
		t.Fatal(err)
	}
	decision, err := (settingsMCPConsent{state: state}).MCPConsent(t.Context(), req)
	if err != nil || !decision.Admitted {
		t.Fatalf("agent consent=%+v %v", decision, err)
	}
	// A same-name project declaration is a separate record selected by its own digest.
	configPath := filepath.Join(workspace.RootDir, compozyconfig.DirName, compozyconfig.MCPJSONName)
	writeMCPDeclarationFile(t, configPath, `{"mcpServers":{"sentry":{"command":"different-fixture"}}}`)
	collection, err = service.ListCollection(t.Context(), settingspkg.CollectionRequest{Collection: settingspkg.CollectionMCPServers, Scope: settingspkg.ScopeWorkspace, WorkspaceID: req.WorkspaceID})
	if err != nil || len(collection.MCPServers) != 2 {
		t.Fatalf("same-name listing=%+v %v", collection.MCPServers, err)
	}
	target := settingspkg.MCPAuthTargetRequest{Scope: settingspkg.ScopeWorkspace, WorkspaceID: req.WorkspaceID, Name: "sentry"}
	for _, item := range collection.MCPServers {
		if _, err := service.SetMCPApproval(t.Context(), target, item.Approval.DefinitionDigest, false); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.SetMCPApproval(t.Context(), target, "", true); err != nil {
		t.Fatal(err)
	}
	collection, err = service.ListCollection(t.Context(), settingspkg.CollectionRequest{Collection: settingspkg.CollectionMCPServers, Scope: settingspkg.ScopeWorkspace, WorkspaceID: req.WorkspaceID})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range collection.MCPServers {
		if item.Approval.State != "waiting" || item.CallableInSessions {
			t.Fatalf("same-name revoke left consent=%+v", item)
		}
	}
}
