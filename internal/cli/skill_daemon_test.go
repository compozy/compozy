package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	registrypkg "github.com/compozy/agh/internal/registry"
	"github.com/compozy/agh/internal/session"
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
					Missing: []string{"agh__extension_call"},
					Message: "gate requires_tools unmet: agh__extension_call",
				}},
			},
			Dir:      "/agh-home/extensions/review/skills/extension-review",
			Metadata: map[string]any{"area": "qa"},
		}
		deps := newTestDeps(t, &stubClient{
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
		deps := newTestDeps(t, &stubClient{
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
				if scope.Scope != contract.SettingsWorkspaceScopeGlobal || scope.WorkspaceID != "" {
					t.Fatalf("Marketplace read scope = %#v, want global", scope)
				}
				downloads := 42
				return MarketplaceKindRecord{Kind: "skill", Items: []MarketplaceListingRecord{{
					Kind:        "skill",
					EntryID:     "skill_review",
					Name:        "review",
					Description: "Review helper",
					Author:      "agh",
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
		deps := newTestDeps(t, &stubClient{
			marketplaceInfoFn: func(
				_ context.Context,
				kind string,
				entryID string,
				installedName string,
				scope MarketplaceReadScope,
			) (MarketplaceEntryRecord, error) {
				called = true
				if kind != contract.MarketplaceKindSkill || entryID != "skill_review" {
					t.Fatalf("MarketplaceInfo(%q, %q), want skill skill_review", kind, entryID)
				}
				if installedName != "" {
					t.Fatalf("MarketplaceInfo installedName = %q, want empty", installedName)
				}
				if scope.Scope != contract.SettingsWorkspaceScopeGlobal || scope.WorkspaceID != "" {
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
		deps := newTestDeps(t, &stubClient{
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
					Path:     "/agh-home/skills/review",
					Hash:     "sha256:review",
					Status:   "installed",
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
		if payload.Status != "installed" || payload.Name != "review" || payload.Registry != "clawhub" {
			t.Fatalf("skill install payload = %#v, want daemon install result", payload)
		}
	})

	t.Run("Should remove marketplace skill through daemon client", func(t *testing.T) {
		t.Parallel()

		removedName := ""
		deps := newTestDeps(t, &stubClient{
			removeSkillMarketplaceFn: func(_ context.Context, name string) (SkillMarketplaceRemoveRecord, error) {
				removedName = name
				return SkillMarketplaceRemoveRecord{
					Name:   name,
					Slug:   "review",
					Path:   "/agh-home/skills/review",
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
		deps := newTestDeps(t, &stubClient{
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
					Path:           "/agh-home/skills/review",
					Status:         "update available",
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
		if len(payload) != 1 || payload[0].Status != "update available" {
			t.Fatalf("skill update payload = %#v, want daemon update result", payload)
		}
	})
}

func TestSkillCommandsAutoScopeToAgentSession(t *testing.T) {
	t.Parallel()

	t.Run("Should use validated agent session scope for reads when no flags are set", func(t *testing.T) {
		t.Parallel()

		const (
			sessionID   = "sess-1"
			workspaceID = "ws-agent"
			agentName   = "general"
		)

		record := SkillRecord{
			Name:        "layered-skill",
			Description: "Agent-local layered skill",
			Version:     "1.0.0",
			Source:      "agent-local",
			Enabled:     true,
			Dir:         "/agh-home/agents/general/skills/layered-skill",
		}
		deps := newTestDeps(t, &stubClient{
			getSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
				if id != sessionID {
					t.Fatalf("GetSession() id = %q, want %q", id, sessionID)
				}
				return SessionRecord{
					ID:          sessionID,
					AgentName:   agentName,
					WorkspaceID: workspaceID,
					State:       session.StateActive,
					CreatedAt:   time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
					UpdatedAt:   time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
				}, nil
			},
			listSkillsFn: func(_ context.Context, query SkillQuery) ([]SkillRecord, error) {
				if got := query.Workspace; got != workspaceID {
					t.Fatalf("ListSkills() workspace = %q, want %q", got, workspaceID)
				}
				if got := query.ForAgent; got != agentName {
					t.Fatalf("ListSkills() for_agent = %q, want %q", got, agentName)
				}
				return []SkillRecord{record}, nil
			},
			getSkillFn: func(_ context.Context, name string, query SkillQuery) (SkillRecord, error) {
				if name != record.Name {
					t.Fatalf("GetSkill() name = %q, want %q", name, record.Name)
				}
				if got := query.Workspace; got != workspaceID {
					t.Fatalf("GetSkill() workspace = %q, want %q", got, workspaceID)
				}
				if got := query.ForAgent; got != agentName {
					t.Fatalf("GetSkill() for_agent = %q, want %q", got, agentName)
				}
				return record, nil
			},
			getSkillContentFn: func(_ context.Context, name string, query SkillQuery) (string, error) {
				if name != record.Name {
					t.Fatalf("GetSkillContent() name = %q, want %q", name, record.Name)
				}
				if got := query.Workspace; got != workspaceID {
					t.Fatalf("GetSkillContent() workspace = %q, want %q", got, workspaceID)
				}
				if got := query.ForAgent; got != agentName {
					t.Fatalf("GetSkillContent() for_agent = %q, want %q", got, agentName)
				}
				return "Agent layered skill marker AGT-LAYERED-500", nil
			},
		})
		deps.getenv = func(key string) string {
			switch key {
			case agentidentity.EnvSessionID:
				return sessionID
			case agentidentity.EnvAgent:
				return agentName
			default:
				return ""
			}
		}

		stdout, _, err := executeRootCommand(t, deps, "skill", "list", "-o", "json")
		if err != nil {
			t.Fatalf("skill list auto-scope error = %v", err)
		}
		var listed []skillListItem
		if err := json.Unmarshal([]byte(stdout), &listed); err != nil {
			t.Fatalf("json.Unmarshal(skill list) error = %v; stdout=%s", err, stdout)
		}
		if len(listed) != 1 || listed[0].Source != "agent-local" {
			t.Fatalf("listed skills = %#v, want one agent-local record", listed)
		}

		stdout, _, err = executeRootCommand(t, deps, "skill", "view", record.Name)
		if err != nil {
			t.Fatalf("skill view auto-scope error = %v", err)
		}
		if !strings.Contains(stdout, "AGT-LAYERED-500") {
			t.Fatalf("skill view auto-scope output = %q, want agent-local marker", stdout)
		}
	})

	t.Run("Should use validated agent session scope for mutations when no flags are set", func(t *testing.T) {
		t.Parallel()

		const (
			sessionID   = "sess-2"
			workspaceID = "ws-agent"
			agentName   = "general"
			skillName   = "layered-skill"
		)

		deps := newTestDeps(t, &stubClient{
			getSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
				if id != sessionID {
					t.Fatalf("GetSession() id = %q, want %q", id, sessionID)
				}
				return SessionRecord{
					ID:          sessionID,
					AgentName:   agentName,
					WorkspaceID: workspaceID,
					State:       session.StateActive,
					CreatedAt:   time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
					UpdatedAt:   time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
				}, nil
			},
			disableSkillFn: func(_ context.Context, name string, query SkillQuery) (SkillActionRecord, error) {
				if name != skillName {
					t.Fatalf("DisableSkill() name = %q, want %q", name, skillName)
				}
				if got := query.Workspace; got != workspaceID {
					t.Fatalf("DisableSkill() workspace = %q, want %q", got, workspaceID)
				}
				if got := query.ForAgent; got != agentName {
					t.Fatalf("DisableSkill() for_agent = %q, want %q", got, agentName)
				}
				return SkillActionRecord{OK: true}, nil
			},
		})
		deps.getenv = func(key string) string {
			switch key {
			case agentidentity.EnvSessionID:
				return sessionID
			case agentidentity.EnvAgent:
				return agentName
			default:
				return ""
			}
		}

		stdout, _, err := executeRootCommand(t, deps, "skill", "disable", skillName, "-o", "json")
		if err != nil {
			t.Fatalf("skill disable auto-scope error = %v", err)
		}
		if !strings.Contains(stdout, `"ok": true`) {
			t.Fatalf("skill disable auto-scope output = %q, want ok=true payload", stdout)
		}
	})
}

func TestSkillWorkspaceFlagValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject explicitly blank workspace flag", func(t *testing.T) {
		t.Parallel()

		deps := newTestDeps(t, &stubClient{})
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

		deps := newTestDeps(t, &stubClient{})
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
