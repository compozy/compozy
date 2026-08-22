package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
	registrypkg "github.com/compozy/compozy/internal/registry"
)

func TestSkillWorkspaceCommandsUseDaemon(t *testing.T) {
	t.Parallel()

	t.Run("Should list inspect and view daemon workspace skills", func(t *testing.T) {
		t.Parallel()

		const workspace = "ws-test"
		record := SkillRecord{
			Name:        "extension-review",
			Description: "Extension review helper",
			Version:     "1.0.0",
			Source:      " user ",
			Enabled:     true,
			Activation: contract.SkillActivationPayload{
				Reasons: []contract.SkillActivationReasonPayload{{
					Gate:    "requires_tools",
					Code:    contract.SkillActivationReasonMissingTool,
					Missing: []string{"compozy__extension_call"},
					Message: "gate requires_tools unmet: compozy__extension_call",
				}},
			},
			Dir:      "/compozy-home/extensions/review/skills/extension-review",
			Metadata: map[string]any{"area": "qa"},
		}
		deps := newWorkspaceTestDeps(t, &stubClient{
			listSkillsFn: func(_ context.Context, query SkillQuery) ([]SkillRecord, error) {
				if query.Workspace != workspace {
					t.Fatalf("ListSkills() workspace = %q, want %q", query.Workspace, workspace)
				}
				return []SkillRecord{record}, nil
			},
			getSkillFn: func(_ context.Context, name string, query SkillQuery) (SkillRecord, error) {
				if name != record.Name {
					t.Fatalf("GetSkill() name = %q, want %q", name, record.Name)
				}
				if query.Workspace != workspace {
					t.Fatalf("GetSkill() workspace = %q, want %q", query.Workspace, workspace)
				}
				return record, nil
			},
			getSkillContentFn: func(_ context.Context, name string, query SkillQuery) (string, error) {
				if name != record.Name {
					t.Fatalf("GetSkillContent() name = %q, want %q", name, record.Name)
				}
				if query.Workspace != workspace {
					t.Fatalf("GetSkillContent() workspace = %q, want %q", query.Workspace, workspace)
				}
				return "# Extension Review\n\nUse extension evidence.", nil
			},
			getSkillShadowsFn: func(_ context.Context, name string, query SkillQuery) (SkillShadowsRecord, error) {
				if name != record.Name {
					t.Fatalf("GetSkillShadows() name = %q, want %q", name, record.Name)
				}
				if query.Workspace != workspace {
					t.Fatalf("GetSkillShadows() workspace = %q, want %q", query.Workspace, workspace)
				}
				return SkillShadowsRecord{
					Name: record.Name,
					Winner: contract.SkillShadowEntryPayload{
						Path:             record.Dir + "/SKILL.md",
						Tier:             "user",
						ResolvedToWinner: true,
						DetectedAt:       fixedTestNow,
					},
					Shadows: []contract.SkillShadowEntryPayload{{
						Path:             record.Dir + "/SKILL.md",
						Tier:             "user",
						ResolvedToWinner: true,
						DetectedAt:       fixedTestNow,
					}},
				}, nil
			},
		})

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"skill",
			"list",
			"--workspace",
			workspace,
			"--source",
			"user",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("skill list --workspace error = %v", err)
		}
		var listed []skillListItem
		if err := json.Unmarshal([]byte(stdout), &listed); err != nil {
			t.Fatalf("json.Unmarshal(skill list) error = %v; stdout=%s", err, stdout)
		}
		if len(listed) != 1 || listed[0].Name != record.Name {
			t.Fatalf("listed skills = %#v, want one %q record", listed, record.Name)
		}
		if !listed[0].Enabled || listed[0].Activation.Active || len(listed[0].Activation.Reasons) != 1 {
			t.Fatalf("listed skill = %#v, want enabled and inactive with one reason", listed[0])
		}

		stdout, _, err = executeRootCommand(
			t,
			deps,
			"skill",
			"inspect",
			record.Name,
			"--workspace",
			workspace,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("skill inspect --workspace error = %v", err)
		}
		var info skillInfoItem
		if err := json.Unmarshal([]byte(stdout), &info); err != nil {
			t.Fatalf("json.Unmarshal(skill inspect) error = %v; stdout=%s", err, stdout)
		}
		if !info.Enabled || info.Activation.Active || len(info.Activation.Reasons) != 1 {
			t.Fatalf("inspected skill = %#v, want enabled and inactive with one reason", info)
		}
		if info.Name != record.Name || info.Source != "user" || info.Path != record.Dir {
			t.Fatalf("skill inspect = %#v, want daemon skill record", info)
		}

		stdout, _, err = executeRootCommand(
			t,
			deps,
			"skill",
			"where",
			record.Name,
			"--workspace",
			workspace,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("skill where --workspace error = %v", err)
		}
		var where SkillShadowsRecord
		if err := json.Unmarshal([]byte(stdout), &where); err != nil {
			t.Fatalf("json.Unmarshal(skill where) error = %v; stdout=%s", err, stdout)
		}
		if where.Winner.Tier != "user" || !where.Winner.ResolvedToWinner {
			t.Fatalf("skill where = %#v, want user winner", where)
		}

		stdout, _, err = executeRootCommand(t, deps, "skill", "view", " "+record.Name+" ", "--workspace", workspace)
		if err != nil {
			t.Fatalf("skill view --workspace error = %v", err)
		}
		if !strings.Contains(stdout, `<skill_content name="extension-review">`) ||
			!strings.Contains(stdout, "Use extension evidence.") {
			t.Fatalf("skill view --workspace output = %q, want rendered daemon content", stdout)
		}
	})
}

func TestSkillMarketplaceCommandsUseDaemonWhenRunning(t *testing.T) {
	t.Parallel()

	t.Run("Should search marketplace through daemon client", func(t *testing.T) {
		t.Parallel()

		called := false
		deps := newWorkspaceTestDeps(t, &stubClient{
			browseMarketplaceFn: func(
				_ context.Context,
				kind string,
				query string,
				limit int,
				cursor string,
				scope MarketplaceReadScope,
			) (MarketplaceKindRecord, error) {
				called = true
				if kind != "skill" || query != "review" || limit != 7 {
					t.Fatalf("BrowseMarketplace(%q, %q, %d), want skill review 7", kind, query, limit)
				}
				if cursor != "" {
					t.Fatalf("BrowseMarketplace cursor = %q, want empty", cursor)
				}
				if scope.Scope != contract.SettingsLayeredScopeUser || scope.WorkspaceID != "" {
					t.Fatalf("Marketplace read scope = %#v, want global", scope)
				}
				downloads := 42
				return MarketplaceKindRecord{Kind: "skill", Items: []MarketplaceListingRecord{{
					Kind:        "skill",
					EntryID:     "skill_review",
					Name:        "review",
					Description: "Review helper",
					Author:      "compozy",
					Version:     "1.2.0",
					Downloads:   &downloads,
					InstallSlug: "review",
					Source:      "clawhub",
				}}}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(t, deps, "skill", "search", "review", "--limit", "7", "-o", "json")
		if err != nil {
			t.Fatalf("skill search error = %v", err)
		}
		if !called {
			t.Fatal("BrowseMarketplace was not called")
		}
		var payload []registrypkg.Listing
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(skill search) error = %v; stdout=%s", err, stdout)
		}
		if len(payload) != 1 || payload[0].Slug != "review" || payload[0].Source != "clawhub" {
			t.Fatalf("skill search payload = %#v, want daemon marketplace review", payload)
		}
	})

	t.Run("Should read marketplace skill detail through daemon client", func(t *testing.T) {
		t.Parallel()

		called := false
		want := MarketplaceEntryRecord{
			Entry: MarketplaceListingRecord{
				Kind:        contract.MarketplaceKindSkill,
				EntryID:     "skill_review",
				Name:        "review",
				Description: "Review helper",
				Version:     "1.2.0",
				Source:      "clawhub",
			},
			Skill: &contract.MarketplaceSkillDetailPayload{
				InstallSlug: "review",
				Tags:        []string{"review", "quality"},
			},
		}
		deps := newWorkspaceTestDeps(t, &stubClient{
			marketplaceInfoFn: func(
				_ context.Context,
				kind string,
				entryID string,
				installedName string,
				scope MarketplaceReadScope,
			) (MarketplaceEntryRecord, error) {
				called = true
				if kind != string(contract.MarketplaceKindSkill) || entryID != "skill_review" {
					t.Fatalf("MarketplaceInfo(%q, %q), want skill skill_review", kind, entryID)
				}
				if installedName != "" {
					t.Fatalf("MarketplaceInfo installedName = %q, want empty", installedName)
				}
				if scope.Scope != contract.SettingsLayeredScopeUser || scope.WorkspaceID != "" {
					t.Fatalf("Marketplace read scope = %#v, want global", scope)
				}
				return want, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(t, deps, "skill", "info", "skill_review", "-o", "json")
		if err != nil {
			t.Fatalf("skill info error = %v", err)
		}
		if !called {
			t.Fatal("MarketplaceInfo was not called")
		}
		var got MarketplaceEntryRecord
		if err := json.Unmarshal([]byte(stdout), &got); err != nil {
			t.Fatalf("json.Unmarshal(skill info) error = %v; stdout=%s", err, stdout)
		}
		if got.Entry.EntryID != want.Entry.EntryID || got.Skill == nil || got.Skill.InstallSlug != "review" {
			t.Fatalf("skill info = %#v, want marketplace detail %#v", got, want)
		}
	})

	t.Run("Should install marketplace skill through daemon client", func(t *testing.T) {
		t.Parallel()

		var captured SkillMarketplaceInstallRequest
		deps := newWorkspaceTestDeps(t, &stubClient{
			installSkillMarketplaceFn: func(
				_ context.Context,
				request SkillMarketplaceInstallRequest,
			) (SkillMarketplaceInstallRecord, error) {
				captured = request
				return SkillMarketplaceInstallRecord{
					Name:     "review",
					Slug:     "review",
					Version:  "1.2.0",
					Registry: "clawhub",
					Path:     "/compozy-home/skills/review",
					Hash:     "sha256:review",
					Status:   "installed",
					CleanupDiagnostics: []contract.SkillMarketplaceCleanupDiagnosticPayload{{
						Operation: "remove_marketplace_staging_root",
					}},
				}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(
			t,
			deps,
			"skill",
			"install",
			"review",
			"--version",
			"1.2.0",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("skill install error = %v", err)
		}
		want := SkillMarketplaceInstallRequest{Slug: "review", Version: "1.2.0"}
		if captured != want {
			t.Fatalf("InstallSkillMarketplace request = %#v, want %#v", captured, want)
		}
		var payload skillInstallItem
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(skill install) error = %v; stdout=%s", err, stdout)
		}
		if payload.Status != "installed" || payload.Name != "review" || payload.Registry != "clawhub" ||
			len(payload.CleanupDiagnostics) != 1 ||
			payload.CleanupDiagnostics[0].Operation != "remove_marketplace_staging_root" {
			t.Fatalf("skill install payload = %#v, want daemon install result", payload)
		}
	})

	t.Run("Should remove marketplace skill through daemon client", func(t *testing.T) {
		t.Parallel()

		removedName := ""
		deps := newWorkspaceTestDeps(t, &stubClient{
			removeSkillMarketplaceFn: func(_ context.Context, name string) (SkillMarketplaceRemoveRecord, error) {
				removedName = name
				return SkillMarketplaceRemoveRecord{
					Name:   name,
					Slug:   "review",
					Path:   "/compozy-home/skills/review",
					Status: "removed",
				}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(t, deps, "skill", "remove", "review", "-o", "json")
		if err != nil {
			t.Fatalf("skill remove error = %v", err)
		}
		if removedName != "review" {
			t.Fatalf("RemoveSkillMarketplace name = %q, want review", removedName)
		}
		var payload skillRemoveItem
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(skill remove) error = %v; stdout=%s", err, stdout)
		}
		if payload.Status != "removed" || payload.Name != "review" {
			t.Fatalf("skill remove payload = %#v, want daemon remove result", payload)
		}
	})

	t.Run("Should update marketplace skill through daemon client", func(t *testing.T) {
		t.Parallel()

		var captured SkillMarketplaceUpdateRequest
		deps := newWorkspaceTestDeps(t, &stubClient{
			updateSkillMarketplaceFn: func(
				_ context.Context,
				request SkillMarketplaceUpdateRequest,
			) ([]SkillMarketplaceUpdateRecord, error) {
				captured = request
				return []SkillMarketplaceUpdateRecord{{
					Name:           "review",
					Slug:           "review",
					CurrentVersion: "1.0.0",
					LatestVersion:  "1.2.0",
					Path:           "/compozy-home/skills/review",
					Status:         "update available",
					CleanupDiagnostics: []contract.SkillMarketplaceCleanupDiagnosticPayload{{
						Operation: "close_marketplace_registry",
					}},
				}}, nil
			},
		})
		markExtensionDaemonRunning(&deps)

		stdout, _, err := executeRootCommand(t, deps, "skill", "update", "review", "--check", "-o", "json")
		if err != nil {
			t.Fatalf("skill update error = %v", err)
		}
		want := SkillMarketplaceUpdateRequest{Name: "review", CheckOnly: true}
		if captured != want {
			t.Fatalf("UpdateSkillMarketplace request = %#v, want %#v", captured, want)
		}
		var payload []skillUpdateItem
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("json.Unmarshal(skill update) error = %v; stdout=%s", err, stdout)
		}
		if len(payload) != 1 || payload[0].Status != "update available" ||
			len(payload[0].CleanupDiagnostics) != 1 ||
			payload[0].CleanupDiagnostics[0].Operation != "close_marketplace_registry" {
			t.Fatalf("skill update payload = %#v, want daemon update result", payload)
		}
	})
}

func TestSkillCommandsRejectManagedSessionCLI(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "list", args: []string{"skill", "list"}},
		{name: "view", args: []string{"skill", "view", "compozy"}},
		{name: "info", args: []string{"skill", "info", "skill_compozy"}},
		{name: "inspect", args: []string{"skill", "inspect", "compozy"}},
		{name: "where", args: []string{"skill", "where", "compozy"}},
		{name: "search", args: []string{"skill", "search", "review"}},
		{name: "install", args: []string{"skill", "install", "review"}},
		{name: "remove", args: []string{"skill", "remove", "review"}},
		{name: "update", args: []string{"skill", "update", "review", "--check"}},
		{name: "create", args: []string{"skill", "create", "review"}},
		{name: "enable", args: []string{"skill", "enable", "review"}},
		{name: "disable", args: []string{"skill", "disable", "review"}},
	}
	for _, test := range tests {
		t.Run("Should reject managed "+test.name+" before client or filesystem access", func(t *testing.T) {
			t.Parallel()

			clientCalls := 0
			workspace := t.TempDir()
			deps := newWorkspaceTestDeps(t, &stubClient{})
			deps.getwd = func() (string, error) { return workspace, nil }
			deps.newClient = func(ClientTarget) (DaemonClient, error) {
				clientCalls++
				return &stubClient{}, nil
			}
			deps.getenv = func(key string) string {
				switch key {
				case agentidentity.EnvSessionID:
					return "sess-managed"
				case agentidentity.EnvAgent:
					return "general"
				default:
					return ""
				}
			}

			_, _, err := executeRootCommand(t, deps, test.args...)
			if !errors.Is(err, errManagedSessionSkillCLIUnsupported) {
				t.Fatalf("skill %s error = %v, want managed-session supported-path guard", test.name, err)
			}
			if clientCalls != 0 {
				t.Fatalf("skill %s client calls = %d, want zero", test.name, clientCalls)
			}
			entries, readErr := os.ReadDir(workspace)
			if readErr != nil {
				t.Fatalf("ReadDir(%q) error = %v", workspace, readErr)
			}
			if len(entries) != 0 {
				t.Fatalf("skill %s workspace entries = %#v, want no filesystem changes", test.name, entries)
			}
		})
	}

	markers := []string{agentidentity.EnvSessionID, agentidentity.EnvAgent}
	for _, marker := range markers {
		t.Run("Should reject list when only "+marker+" is present", func(t *testing.T) {
			t.Parallel()

			deps := newWorkspaceTestDeps(t, &stubClient{})
			deps.getenv = func(key string) string {
				if key == marker {
					return "managed-marker"
				}
				return ""
			}
			_, _, err := executeRootCommand(t, deps, "skill", "list")
			if !errors.Is(err, errManagedSessionSkillCLIUnsupported) {
				t.Fatalf("skill list error = %v, want managed-session supported-path guard", err)
			}
		})
	}

	t.Run("Should allow operator skill list through the daemon client", func(t *testing.T) {
		t.Parallel()

		clientCalls := 0
		client := &stubClient{
			listSkillsFn: func(context.Context, SkillQuery) ([]SkillRecord, error) { return nil, nil },
		}
		deps := newWorkspaceTestDeps(t, client)
		markExtensionDaemonRunning(&deps)
		deps.getenv = func(string) string { return "" }
		deps.newClient = func(ClientTarget) (DaemonClient, error) {
			clientCalls++
			return client, nil
		}

		if _, _, err := executeRootCommand(t, deps, "skill", "list"); err != nil {
			t.Fatalf("skill list error = %v", err)
		}
		if clientCalls == 0 {
			t.Fatal("skill list daemon client calls = 0, want at least one")
		}
	})
}

func TestSkillWorkspaceFlagValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject explicitly blank workspace flag", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{})
		_, _, err := executeRootCommand(t, deps, "skill", "list", "--workspace", " ")
		if err == nil {
			t.Fatal("skill list --workspace blank error = nil, want error")
		}
		if !strings.Contains(err.Error(), "workspace flag cannot be empty") {
			t.Fatalf("skill list --workspace blank error = %v, want workspace flag message", err)
		}
	})

	t.Run("Should reject resource file reads through daemon workspace mode", func(t *testing.T) {
		t.Parallel()

		deps := newWorkspaceTestDeps(t, &stubClient{})
		_, _, err := executeRootCommand(
			t,
			deps,
			"skill",
			"view",
			"review",
			"--workspace",
			"ws-test",
			"--file",
			"refs/a.md",
		)
		if err == nil {
			t.Fatal("skill view --workspace --file error = nil, want error")
		}
		if !strings.Contains(err.Error(), "skill view --workspace does not support --file") {
			t.Fatalf("skill view --workspace --file error = %v, want unsupported file message", err)
		}
	})
}
