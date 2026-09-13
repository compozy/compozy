package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
)

func TestSkillWorkspaceCommandsUseDaemon(t *testing.T) {
	t.Parallel()

	t.Run("Should filter daemon skills by both profile tiers", func(t *testing.T) {
		t.Parallel()

		const workspace = "ws-profile-filters"
		records := []SkillRecord{
			{Name: "personal", Source: profileSkillSource, Enabled: true},
			{Name: "project-personal", Source: workspaceProfileSkillSource, Enabled: true},
		}
		deps := newWorkspaceTestDeps(t, &stubClient{
			listSkillsFn: func(_ context.Context, query SkillQuery) ([]SkillRecord, error) {
				if query.Workspace != workspace {
					t.Fatalf("ListSkills() workspace = %q, want %q", query.Workspace, workspace)
				}
				return records, nil
			},
		})

		for _, source := range []string{profileSkillSource, workspaceProfileSkillSource} {
			stdout, _, err := executeRootCommand(
				t, deps, "skill", "list", "--workspace", workspace, "--source", source, "-o", "json",
			)
			if err != nil {
				t.Fatalf("skill list --source %s error = %v", source, err)
			}
			var listed []skillListItem
			if err := json.Unmarshal([]byte(stdout), &listed); err != nil {
				t.Fatalf("json.Unmarshal(skill list --source %s) error = %v", source, err)
			}
			if len(listed) != 1 || listed[0].Source != source {
				t.Fatalf("skill list --source %s = %#v, want one matching record", source, listed)
			}
		}
	})

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
			"info",
			record.Name,
			"--workspace",
			workspace,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("skill info --workspace error = %v", err)
		}
		var info skillInfoItem
		if err := json.Unmarshal([]byte(stdout), &info); err != nil {
			t.Fatalf("json.Unmarshal(skill info) error = %v; stdout=%s", err, stdout)
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
		var where skillWhereItem
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

func TestSkillExposureCommandsUseCanonicalDaemonEnvelope(t *testing.T) {
	t.Parallel()

	t.Run("Should render expose idempotency and unexpose paths", func(t *testing.T) {
		t.Parallel()
		workspaceID := "ws-exposure"
		exposurePath := "/repo/.agents/skills/review-checklist"
		getCalls := 0
		client := &stubClient{
			getSkillFn: func(_ context.Context, name string, query SkillQuery) (SkillRecord, error) {
				getCalls++
				if name != "review-checklist" || query.Workspace != workspaceID {
					t.Fatalf("GetSkill(%q, %#v)", name, query)
				}
				exposures := []contract.SkillExposurePayload{}
				if getCalls == 2 {
					exposures = append(exposures, contract.SkillExposurePayload{
						Target: "agents", Path: exposurePath, Status: "healthy",
					})
				}
				return SkillRecord{Name: name, Exposures: &exposures}, nil
			},
			exposeSkillFn: func(
				_ context.Context,
				name string,
				request contract.SkillExposureRequest,
				query SkillQuery,
			) (contract.SkillExposeResponse, error) {
				if name != "review-checklist" || request.WorkspaceID != workspaceID ||
					query.Workspace != workspaceID || !slices.Equal(request.Targets, []string{"agents"}) {
					t.Fatalf("ExposeSkill(%q, %#v, %#v)", name, request, query)
				}
				return contract.SkillExposeResponse{Name: name, WorkspaceID: workspaceID,
					Results: []contract.SkillExposureTargetResultPayload{
						{
							Target: "agents",
							OK:     true,
							Exposure: &contract.SkillExposurePayload{
								Target: "agents",
								Path:   exposurePath,
								Status: "healthy",
							},
						},
					},
				}, nil
			},
			unexposeSkillFn: func(
				_ context.Context,
				name string,
				request contract.SkillExposureRequest,
				_ SkillQuery,
			) (contract.SkillUnexposeResponse, error) {
				return contract.SkillUnexposeResponse{Name: name, WorkspaceID: request.WorkspaceID,
					Results: []contract.SkillExposureTargetResultPayload{
						{
							Target: "agents",
							OK:     true,
							Exposure: &contract.SkillExposurePayload{
								Target: "agents",
								Path:   exposurePath,
								Status: "healthy",
							},
						},
					},
				}, nil
			},
		}
		deps := newWorkspaceTestDeps(t, client)

		stdout, _, err := executeRootCommand(
			t, deps, "skill", "expose", "review-checklist", "--to", "agents", "--workspace", workspaceID,
		)
		if err != nil {
			t.Fatalf("skill expose error = %v", err)
		}
		if strings.TrimSpace(stdout) != "exposed review-checklist → "+exposurePath {
			t.Fatalf("first expose stdout = %q", stdout)
		}

		stdout, _, err = executeRootCommand(
			t, deps, "skill", "expose", "review-checklist", "--to", "agents", "--workspace", workspaceID,
		)
		if err != nil {
			t.Fatalf("repeat skill expose error = %v", err)
		}
		if strings.TrimSpace(stdout) != "already exposed: review-checklist → "+exposurePath+" (no change)" {
			t.Fatalf("repeat expose stdout = %q", stdout)
		}

		stdout, _, err = executeRootCommand(
			t, deps, "skill", "unexpose", "review-checklist", "--to", "agents", "--workspace", workspaceID,
		)
		if err != nil {
			t.Fatalf("skill unexpose error = %v", err)
		}
		if strings.TrimSpace(stdout) != "unexposed review-checklist ← "+exposurePath {
			t.Fatalf("unexpose stdout = %q", stdout)
		}
	})

	t.Run("Should render and marshal the one expose failure envelope", func(t *testing.T) {
		t.Parallel()
		rolledBack := true
		failure := contract.SkillExposureFailureResponse{
			Error: contract.SkillExposureFailureErrorPayload{Code: "expose_failed", Message: "1 of 2 targets failed"},
			Name:  "review-checklist",
			Results: []contract.SkillExposureTargetResultPayload{
				{Target: "agents", Error: &contract.SkillExposureErrorPayload{Code: "rolled_back"}},
				{Target: "claude", Error: &contract.SkillExposureErrorPayload{
					Code: "expose_name_conflict", OccupiedBy: "/repo/.claude/skills/review-checklist",
				}},
			},
			RolledBack: &rolledBack,
		}
		client := &stubClient{
			getSkillFn: func(_ context.Context, name string, _ SkillQuery) (SkillRecord, error) {
				exposures := []contract.SkillExposurePayload{}
				return SkillRecord{Name: name, Exposures: &exposures}, nil
			},
			exposeSkillFn: func(
				context.Context,
				string,
				contract.SkillExposureRequest,
				SkillQuery,
			) (contract.SkillExposeResponse, error) {
				return contract.SkillExposeResponse{}, &skillExposureAPIError{payload: failure}
			},
		}
		deps := newWorkspaceTestDeps(t, client)
		exitCode, _, stderr := executeRootCommandWithExit(
			t, deps, "skill", "expose", "review-checklist", "--to", "agents,claude", "--workspace", "ws",
		)
		if exitCode == 0 {
			t.Fatal("skill expose exit code = 0, want failure")
		}
		for _, want := range []string{
			"Error: skill exposure failed (1 of 2 targets; completed targets rolled back)",
			"agents  rolled_back",
			"claude  expose_name_conflict — occupied by /repo/.claude/skills/review-checklist",
		} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
		}

		exitCode, _, stderr = executeRootCommandWithExit(
			t, deps, "skill", "expose", "review-checklist", "--to", "agents,claude", "--workspace", "ws",
			"-o", "json",
		)
		if exitCode == 0 {
			t.Fatal("skill expose -o json exit code = 0, want failure")
		}
		var got contract.SkillExposureFailureResponse
		if err := json.Unmarshal([]byte(stderr), &got); err != nil {
			t.Fatalf("json.Unmarshal(skill expose failure) error = %v; stderr=%s", err, stderr)
		}
		if got.Error.Code != failure.Error.Code || got.Name != failure.Name ||
			len(got.Results) != len(failure.Results) ||
			got.RolledBack == nil ||
			!*got.RolledBack {
			t.Fatalf("skill expose JSON failure = %#v, want canonical API envelope %#v", got, failure)
		}
	})

	t.Run("Should distinguish a preflight-aborted target from a rolled-back target", func(t *testing.T) {
		t.Parallel()
		rolledBack := false
		failure := contract.SkillExposureFailureResponse{
			Error: contract.SkillExposureFailureErrorPayload{Code: "expose_failed", Message: "1 of 2 targets failed"},
			Name:  "review-checklist",
			Results: []contract.SkillExposureTargetResultPayload{
				{Target: "agents", Error: &contract.SkillExposureErrorPayload{
					Code:    "expose_not_applied",
					Message: "exposure was not applied because target \"claude\" failed preflight",
				}},
				{Target: "claude", Error: &contract.SkillExposureErrorPayload{
					Code: "expose_name_conflict", OccupiedBy: "/repo/.claude/skills/review-checklist",
				}},
			},
			RolledBack: &rolledBack,
		}
		client := &stubClient{
			getSkillFn: func(_ context.Context, name string, _ SkillQuery) (SkillRecord, error) {
				exposures := []contract.SkillExposurePayload{}
				return SkillRecord{Name: name, Exposures: &exposures}, nil
			},
			exposeSkillFn: func(
				context.Context,
				string,
				contract.SkillExposureRequest,
				SkillQuery,
			) (contract.SkillExposeResponse, error) {
				return contract.SkillExposeResponse{}, &skillExposureAPIError{payload: failure}
			},
		}
		deps := newWorkspaceTestDeps(t, client)
		exitCode, _, stderr := executeRootCommandWithExit(
			t, deps, "skill", "expose", "review-checklist", "--to", "agents,claude", "--workspace", "ws",
		)
		if exitCode == 0 {
			t.Fatal("skill expose exit code = 0, want failure")
		}
		for _, want := range []string{
			"Error: skill exposure failed (1 of 2 targets)",
			"agents  expose_not_applied — exposure was not applied because target \"claude\" failed preflight",
			"claude  expose_name_conflict — occupied by /repo/.claude/skills/review-checklist",
		} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("stderr = %q, want %q", stderr, want)
			}
		}
	})

	t.Run("Should keep a created skill when its requested exposure fails", func(t *testing.T) {
		t.Parallel()
		workspace := t.TempDir()
		workspaceID := "ws-create-exposure"
		client := &stubClient{
			getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
				if ref != workspace {
					t.Fatalf("GetWorkspace(%q), want %q", ref, workspace)
				}
				return WorkspaceDetailRecord{Workspace: WorkspaceRecord{ID: workspaceID, RootDir: workspace}}, nil
			},
			exposeSkillFn: func(
				_ context.Context,
				name string,
				request contract.SkillExposureRequest,
				query SkillQuery,
			) (contract.SkillExposeResponse, error) {
				if name != "review-checklist" || request.WorkspaceID != workspaceID || query.Workspace != workspaceID {
					t.Fatalf("ExposeSkill(%q, %#v, %#v)", name, request, query)
				}
				return contract.SkillExposeResponse{}, &skillExposureAPIError{
					payload: contract.SkillExposureFailureResponse{
						Error: contract.SkillExposureFailureErrorPayload{
							Code:    "expose_failed",
							Message: "1 of 1 targets failed",
						},
						Name:        name,
						WorkspaceID: workspaceID,
						Results: []contract.SkillExposureTargetResultPayload{{
							Target: "claude", Error: &contract.SkillExposureErrorPayload{
								Code: "expose_target_disabled", Message: "source not enabled (enabled targets: agents)",
							},
						}},
					},
				}
			},
		}
		deps := newWorkspaceTestDeps(t, client)
		deps.getwd = func() (string, error) { return workspace, nil }

		exitCode, stdout, stderr := executeRootCommandWithExit(
			t, deps, "skill", "create", "review-checklist", "--expose", "claude",
		)
		if exitCode == 0 {
			t.Fatal("skill create --expose exit code = 0, want partial failure")
		}
		if strings.TrimSpace(stdout) != "created .compozy/skills/review-checklist/SKILL.md" {
			t.Fatalf("skill create --expose stdout = %q", stdout)
		}
		for _, want := range []string{
			"Error: skill exposure failed (1 target) — the skill was created; fix the cause and run `compozy skill expose`",
			"claude  expose_target_disabled — source not enabled (enabled targets: agents)",
		} {
			if !strings.Contains(stderr, want) {
				t.Fatalf("skill create --expose stderr = %q, want %q", stderr, want)
			}
		}
		createdFile := filepath.Join(workspace, ".compozy", "skills", "review-checklist", "SKILL.md")
		if _, err := os.Stat(createdFile); err != nil {
			t.Fatalf("created skill %q is unavailable after exposure failure: %v", createdFile, err)
		}
	})

	t.Run("Should render created and exposed lines after the daemon succeeds", func(t *testing.T) {
		t.Parallel()
		workspace := t.TempDir()
		workspaceID := "ws-create-success"
		exposurePath := filepath.Join(workspace, ".agents", "skills", "review-checklist")
		client := &stubClient{
			getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
				return WorkspaceDetailRecord{Workspace: WorkspaceRecord{ID: workspaceID, RootDir: ref}}, nil
			},
			exposeSkillFn: func(
				_ context.Context,
				name string,
				request contract.SkillExposureRequest,
				_ SkillQuery,
			) (contract.SkillExposeResponse, error) {
				return contract.SkillExposeResponse{
					Name: name, WorkspaceID: request.WorkspaceID,
					Results: []contract.SkillExposureTargetResultPayload{
						{
							Target: "agents",
							OK:     true,
							Exposure: &contract.SkillExposurePayload{
								Target: "agents",
								Path:   exposurePath,
								Status: "healthy",
							},
						},
					},
				}, nil
			},
		}
		deps := newWorkspaceTestDeps(t, client)
		deps.getwd = func() (string, error) { return workspace, nil }

		stdout, _, err := executeRootCommand(
			t, deps, "skill", "create", "review-checklist", "--expose", "agents",
		)
		if err != nil {
			t.Fatalf("skill create --expose error = %v", err)
		}
		want := "created .compozy/skills/review-checklist/SKILL.md\nexposed review-checklist → " + exposurePath
		if strings.TrimSpace(stdout) != want {
			t.Fatalf("skill create --expose stdout = %q, want %q", stdout, want)
		}
	})
}

func TestSkillPublicTranscriptsMatchDXContract(t *testing.T) {
	t.Parallel()

	t.Run("Should render the list origin columns without legacy status columns", func(t *testing.T) {
		t.Parallel()
		got := renderSkillListTranscript([]skillListItem{
			{Name: "compozy", Source: "bundled", Description: "Operate CompozyOS sessions, tasks, and memory"},
			{
				Name:        "frontend-qa",
				Source:      "user",
				Origin:      "agents",
				Description: "Audit web UIs against the team checklist",
			},
			{
				Name:        "git-hygiene",
				Source:      "user",
				Origin:      "agents",
				Description: "Keep branches, commits, and PRs clean",
			},
		})
		want := strings.Join([]string{
			"NAME         SOURCE   ORIGIN  DESCRIPTION",
			"compozy      bundled  —       Operate CompozyOS sessions, tasks, and memory",
			"frontend-qa  user     agents  Audit web UIs against the team checklist",
			"git-hygiene  user     agents  Keep branches, commits, and PRs clean",
		}, "\n")
		if got != want {
			t.Fatalf("skill list transcript =\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("Should render every exposure health state with its safe action", func(t *testing.T) {
		t.Parallel()
		statuses := []contract.SkillExposurePayload{
			{Target: "agents", Path: "/repo/.agents/skills/review-checklist", Status: "healthy"},
			{Target: "claude", Path: "/repo/.claude/skills/review-checklist", Status: "missing"},
			{Target: "codex", Path: "/repo/.codex/skills/review-checklist", Status: "broken"},
			{Target: "cursor", Path: "/repo/.cursor/skills/review-checklist", Status: "foreign_conflict"},
		}
		got := renderSkillInfoTranscript(skillInfoItem{
			Name: "review-checklist", Source: "workspace", Path: "/repo/.compozy/skills/review-checklist",
			Exposures: statuses,
		})
		want := strings.Join([]string{
			"NAME         review-checklist",
			"SOURCE       workspace",
			"PATH         /repo/.compozy/skills/review-checklist",
			"EXPOSED TO   agents → /repo/.agents/skills/review-checklist (healthy)",
			"             claude → /repo/.claude/skills/review-checklist (missing — re-expose repairs)",
			"             codex → /repo/.codex/skills/review-checklist (broken — unexpose or re-expose repairs)",
			"             cursor → /repo/.cursor/skills/review-checklist (foreign conflict — not our link; no action)",
		}, "\n")
		if got != want {
			t.Fatalf("skill info transcript =\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("Should render winner shadows qualified hints and exposure links", func(t *testing.T) {
		t.Parallel()
		got := renderSkillWhereTranscript(skillWhereItem{
			Name: "frontend-qa", Source: "workspace", Dir: "/repo/.compozy/skills/frontend-qa",
			Winner: contract.SkillShadowEntryPayload{
				Path: "/repo/.compozy/skills/frontend-qa/SKILL.md", Tier: "workspace", ResolvedToWinner: true,
			},
			Shadows: []contract.SkillShadowEntryPayload{
				{Path: "/repo/.compozy/skills/frontend-qa/SKILL.md", Tier: "workspace", ResolvedToWinner: true},
				{Path: "/repo/.agents/skills/frontend-qa/SKILL.md", Tier: "workspace", Origin: "agents"},
				{Path: "~/.agents/skills/frontend-qa/SKILL.md", Tier: "user", Origin: "agents"},
			},
			Exposures: []contract.SkillExposurePayload{{
				Target: "claude", Path: "/repo/.claude/skills/frontend-qa", Status: "healthy",
			}},
		})
		want := strings.Join([]string{
			"WINNER   /repo/.compozy/skills/frontend-qa (workspace · compozy)",
			"ALSO     /repo/.agents/skills/frontend-qa (workspace · agents · shadowed — invoke as agents:frontend-qa)",
			"         ~/.agents/skills/frontend-qa (user · agents · shadowed)",
			"LINKS    /repo/.claude/skills/frontend-qa → /repo/.compozy/skills/frontend-qa (exposure · healthy)",
		}, "\n")
		if got != want {
			t.Fatalf("skill where transcript =\n%s\nwant:\n%s", got, want)
		}

		empty := renderSkillWhereTranscript(skillWhereItem{
			Name: "pdf-tools", Source: "user", Origin: "claude", Dir: "~/.claude/skills/pdf-tools",
			Winner: contract.SkillShadowEntryPayload{
				Path: "~/.claude/skills/pdf-tools/SKILL.md", Tier: "user", Origin: "claude", ResolvedToWinner: true,
			},
		})
		if empty != "WINNER   ~/.claude/skills/pdf-tools (user · claude)\nALSO     — none —" {
			t.Fatalf("empty skill where transcript = %q", empty)
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
		{name: "info", args: []string{"skill", "info", "compozy"}},
		{name: "where", args: []string{"skill", "where", "compozy"}},
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

	t.Run("Should allow managed agents to expose and unexpose through the daemon", func(t *testing.T) {
		t.Parallel()

		exposurePath := "/repo/.agents/skills/review"
		client := &stubClient{
			getSessionFn: func(_ context.Context, id string) (SessionRecord, error) {
				return SessionRecord{
					ID: id, ProfileID: "default", AgentName: "general", WorkspaceID: "ws-managed", State: "active",
				}, nil
			},
			getSkillFn: func(_ context.Context, name string, _ SkillQuery) (SkillRecord, error) {
				exposures := []contract.SkillExposurePayload{}
				return SkillRecord{Name: name, Exposures: &exposures}, nil
			},
			exposeSkillFn: func(
				_ context.Context,
				name string,
				request contract.SkillExposureRequest,
				_ SkillQuery,
			) (contract.SkillExposeResponse, error) {
				return contract.SkillExposeResponse{Name: name, WorkspaceID: request.WorkspaceID,
					Results: []contract.SkillExposureTargetResultPayload{
						{
							Target: "agents",
							OK:     true,
							Exposure: &contract.SkillExposurePayload{
								Target: "agents",
								Path:   exposurePath,
								Status: "healthy",
							},
						},
					},
				}, nil
			},
			unexposeSkillFn: func(
				_ context.Context,
				name string,
				request contract.SkillExposureRequest,
				_ SkillQuery,
			) (contract.SkillUnexposeResponse, error) {
				return contract.SkillUnexposeResponse{Name: name, WorkspaceID: request.WorkspaceID,
					Results: []contract.SkillExposureTargetResultPayload{
						{
							Target: "agents",
							OK:     true,
							Exposure: &contract.SkillExposurePayload{
								Target: "agents",
								Path:   exposurePath,
								Status: "healthy",
							},
						},
					},
				}, nil
			},
		}
		deps := newWorkspaceTestDeps(t, client)
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
		if _, _, err := executeRootCommand(
			t, deps, "skill", "expose", "review", "--to", "agents", "--workspace", "ws-managed",
		); err != nil {
			t.Fatalf("managed skill expose error = %v", err)
		}
		if _, _, err := executeRootCommand(
			t, deps, "skill", "unexpose", "review", "--to", "agents", "--workspace", "ws-managed",
		); err != nil {
			t.Fatalf("managed skill unexpose error = %v", err)
		}
	})

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
