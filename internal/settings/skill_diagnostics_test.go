package settings

import (
	"context"
	"maps"
	"path/filepath"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	skillspkg "github.com/compozy/compozy/internal/skills"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestSkillsSectionDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should expose skill resolution diagnostics from runtime", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		runtime := &diagnosticSkillsRuntime{
			fakeSkillsRuntime: newFakeSkillsRuntime(testSkill("review", true)),
			diagnostics: []skillspkg.SkillDiagnostic{
				{
					Name:               "review",
					State:              skillspkg.SkillDiagnosticStateValid,
					Source:             "workspace",
					Path:               "/workspace/.compozy/skills/review/SKILL.md",
					WinningSource:      "workspace",
					WinningPath:        "/workspace/.compozy/skills/review/SKILL.md",
					VerificationStatus: skillspkg.SkillVerificationStatusPassed,
				},
				{
					Name:               "review",
					State:              skillspkg.SkillDiagnosticStateShadowed,
					Source:             "user",
					Path:               "/user/skills/review/SKILL.md",
					WinningSource:      "workspace",
					WinningPath:        "/workspace/.compozy/skills/review/SKILL.md",
					VerificationStatus: skillspkg.SkillVerificationStatusPassed,
				},
				{
					Name:               "blocked",
					State:              skillspkg.SkillDiagnosticStateVerificationFailed,
					Source:             "marketplace",
					Path:               "/user/skills/blocked/SKILL.md",
					VerificationStatus: skillspkg.SkillVerificationStatusFailed,
					Failure: &skillspkg.SkillVerificationFailure{
						Code:    "hash_mismatch",
						Message: "marketplace skill hash mismatch",
					},
				},
			},
		}
		service := testService(t, homePaths, Dependencies{SkillsRuntime: runtime})

		envelope, err := service.GetSection(ctx, SectionRequest{Section: SectionSkills})
		if err != nil {
			t.Fatalf("GetSection(skills) error = %v", err)
		}
		if envelope.Skills == nil {
			t.Fatal("Skills section = nil, want diagnostics section")
		}
		if got, want := len(envelope.Skills.Diagnostics), 3; got != want {
			t.Fatalf("Skills.Diagnostics len = %d, want %d", got, want)
		}
		if got, want := envelope.Skills.Diagnostics[1].State, skillspkg.SkillDiagnosticStateShadowed; got != want {
			t.Fatalf("shadowed diagnostic state = %q, want %q", got, want)
		}
		if got, want := envelope.Skills.Diagnostics[1].WinningPath, "/workspace/.compozy/skills/review/SKILL.md"; got != want {
			t.Fatalf("shadowed winning path = %q, want %q", got, want)
		}
		if envelope.Skills.Diagnostics[2].Failure == nil {
			t.Fatal("failed diagnostic failure = nil, want verification failure")
		}
		if got, want := envelope.Skills.Diagnostics[2].Failure.Code, "hash_mismatch"; got != want {
			t.Fatalf("failed diagnostic code = %q, want %q", got, want)
		}
	})

	t.Run("Should expose complete per-root source measurements", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		agentsRoot := compozyconfig.SkillRootSpec{
			Dir: "/missing/.agents/skills", SourceSlug: "agents", Kind: compozyconfig.RootKindPreset,
			ResourceScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
		}
		customRoot := compozyconfig.SkillRootSpec{
			Dir: "/private/team-skills", SourceSlug: "team-skills", Kind: compozyconfig.RootKindCustom,
			ResourceScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
		}
		runtime := &sourceStatusSkillsRuntime{
			fakeSkillsRuntime: newFakeSkillsRuntime(testSkill("review", true)),
			statuses: []skillspkg.SkillSourceRootStatus{
				{
					Spec: agentsRoot, Exists: false, Readable: true,
				},
				{
					Spec: customRoot, Exists: true, Readable: false, ScannedCount: 73, SkillCount: 12,
					Truncated: true,
					SkippedLinks: []skillspkg.SkillSourceSkippedLink{{
						Path: "/private/team-skills/escape", Reason: "escape",
					}},
					Collisions: []skillspkg.SkillSourceCollision{{
						Name: "review", WinnerRootID: agentsRoot.RootID(), QualifiedForm: "team-skills:review",
					}},
					Verification: skillspkg.SkillSourceVerification{Blocked: 2, Warned: 1},
				},
			},
		}
		service := testService(t, homePaths, Dependencies{SkillsRuntime: runtime})

		envelope, err := service.GetSection(t.Context(), SectionRequest{Section: SectionSkills})
		if err != nil {
			t.Fatalf("GetSection(skills) error = %v", err)
		}
		agents := findSkillSourceItem(t, envelope.Skills.Sources, "agents")
		if got, want := len(agents.Roots), 1; got != want {
			t.Fatalf("agents roots = %d, want %d", got, want)
		}
		if agents.Roots[0].Exists || agents.Roots[0].ScannedCount == nil || *agents.Roots[0].ScannedCount != 0 ||
			agents.Roots[0].SkillCount == nil || *agents.Roots[0].SkillCount != 0 {
			t.Fatalf("absent agents root = %#v, want explicit zero counts", agents.Roots[0])
		}

		custom := findSkillSourceItem(t, envelope.Skills.Sources, "team-skills")
		if got, want := custom.Label, "team-skills"; got != want {
			t.Fatalf("custom label = %q, want %q", got, want)
		}
		if got, want := len(custom.Roots), 1; got != want {
			t.Fatalf("custom roots = %d, want %d", got, want)
		}
		root := custom.Roots[0]
		if root.Readable || root.ScannedCount != nil || root.SkillCount != nil {
			t.Fatalf("unreadable root counts = %#v, want omitted", root)
		}
		if !root.Truncated || len(root.SkippedLinks) != 1 || len(root.Collisions) != 1 ||
			root.Verification.Blocked != 2 || root.Verification.Warned != 1 {
			t.Fatalf("custom diagnostics = %#v, want complete scan diagnostics", root)
		}
	})

	t.Run("Should preserve configured custom paths beside canonical root measurements", func(t *testing.T) {
		t.Parallel()

		configuredPath := "/var/team-skills"
		canonicalPath := "/private/var/team-skills"
		items, _, err := (&service{}).buildSkillSourceReadModel(
			compozyconfig.SkillsConfig{CustomSources: []string{configuredPath}},
			nil,
			ScopeUser,
			[]skillspkg.SkillSourceRootStatus{{
				Spec: compozyconfig.SkillRootSpec{
					Dir: canonicalPath, SourceSlug: "team-skills", Kind: compozyconfig.RootKindCustom,
					ResourceScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				},
				Exists: true, Readable: true, ScannedCount: 1, SkillCount: 1,
			}},
			true,
		)
		if err != nil {
			t.Fatalf("buildSkillSourceReadModel() error = %v", err)
		}
		custom := findSkillSourceItem(t, items, "team-skills")
		if got, want := custom.Path, configuredPath; got != want {
			t.Fatalf("custom path = %q, want configured path %q", got, want)
		}
		if got, want := custom.Roots[0].Path, canonicalPath; got != want {
			t.Fatalf("custom root path = %q, want canonical measurement %q", got, want)
		}
		if custom.Roots[0].SkillCount == nil || *custom.Roots[0].SkillCount != 1 {
			t.Fatalf("custom root skill count = %#v, want 1", custom.Roots[0].SkillCount)
		}
	})

	t.Run("Should report workspace inheritance independently per source key", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		workspaceRoot := t.TempDir()
		writeFile(
			t,
			filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.ConfigName),
			"[skills]\nsources = [\"claude\"]\n",
		)
		resolved := workspacepkg.ResolvedWorkspace{
			Workspace:   workspacepkg.Workspace{ID: "ws-sources", RootDir: workspaceRoot},
			WorkspaceID: "ws-sources",
		}
		service := testService(t, homePaths, Dependencies{
			SkillsRuntime: newSourceStatusSkillsRuntime(),
			WorkspaceResolver: fakeWorkspaceResolver{resolved: map[string]workspacepkg.ResolvedWorkspace{
				"ws-sources": resolved,
			}},
		})

		envelope, err := service.GetSection(t.Context(), SectionRequest{
			Section: SectionSkills, Scope: ScopeWorkspace, WorkspaceID: "ws-sources",
		})
		if err != nil {
			t.Fatalf("GetSection(workspace skills) error = %v", err)
		}
		if envelope.Skills == nil || envelope.Skills.Inherits == nil {
			t.Fatalf("workspace skills = %#v, want inheritance", envelope.Skills)
		}
		if envelope.Skills.Inherits.Sources || !envelope.Skills.Inherits.CustomSources {
			t.Fatalf("inherits = %#v, want sources=false custom_sources=true", envelope.Skills.Inherits)
		}
	})

	t.Run("Should correlate source applies with every effective root", func(t *testing.T) {
		t.Parallel()

		homePaths := testHomePaths(t)
		writeFile(t, homePaths.ConfigFile, baseSettingsConfig())
		runtime := &sourceStatusSkillsRuntime{
			fakeSkillsRuntime: newFakeSkillsRuntime(),
			statuses: []skillspkg.SkillSourceRootStatus{
				{Spec: compozyconfig.SkillRootSpec{SourceSlug: "compozy"}},
				{Spec: compozyconfig.SkillRootSpec{SourceSlug: "compozy"}},
				{Spec: compozyconfig.SkillRootSpec{SourceSlug: "agents"}},
				{Spec: compozyconfig.SkillRootSpec{SourceSlug: "agents"}},
				{Spec: compozyconfig.SkillRootSpec{SourceSlug: "team-skills"}},
			},
		}
		settingsService, ok := testService(t, homePaths, Dependencies{SkillsRuntime: runtime}).(*service)
		if !ok {
			t.Fatal("testService() did not return the concrete settings service")
		}
		ctx, err := settingsService.withSkillSourceEventCorrelation(t.Context(), MutationResult{
			Section:     SectionSkills,
			Scope:       ScopeProfile,
			ProfileName: "marketing",
		})
		if err != nil {
			t.Fatalf("withSkillSourceEventCorrelation() error = %v", err)
		}
		correlation := skillspkg.SourceEventCorrelationFromContext(ctx)
		want := map[string]int{"compozy": 2, "agents": 2, "team-skills": 1}
		if !maps.Equal(correlation.RootCounts, want) {
			t.Fatalf("RootCounts = %#v, want %#v", correlation.RootCounts, want)
		}
	})
}

func findSkillSourceItem(t *testing.T, items []SkillSourceItem, slug string) SkillSourceItem {
	t.Helper()
	for _, item := range items {
		if item.Slug == slug {
			return item
		}
	}
	t.Fatalf("skill source %q not found in %#v", slug, items)
	return SkillSourceItem{}
}

type diagnosticSkillsRuntime struct {
	*fakeSkillsRuntime
	diagnostics []skillspkg.SkillDiagnostic
}

type sourceStatusSkillsRuntime struct {
	*fakeSkillsRuntime
	statuses []skillspkg.SkillSourceRootStatus
}

func newSourceStatusSkillsRuntime() *sourceStatusSkillsRuntime {
	return &sourceStatusSkillsRuntime{fakeSkillsRuntime: newFakeSkillsRuntime()}
}

func (s *sourceStatusSkillsRuntime) SkillSourceRoots(
	_ context.Context,
	_ *workspacepkg.ResolvedWorkspace,
) ([]skillspkg.SkillSourceRootStatus, error) {
	return append([]skillspkg.SkillSourceRootStatus(nil), s.statuses...), nil
}

func (s *sourceStatusSkillsRuntime) ForWorkspace(
	_ context.Context,
	_ *workspacepkg.ResolvedWorkspace,
) ([]*skillspkg.Skill, error) {
	return s.List(), nil
}

func (d *diagnosticSkillsRuntime) SkillDiagnostics(
	_ context.Context,
	_ *workspacepkg.ResolvedWorkspace,
	_ string,
) ([]skillspkg.SkillDiagnostic, error) {
	return append([]skillspkg.SkillDiagnostic(nil), d.diagnostics...), nil
}

var _ SkillsRuntime = (*diagnosticSkillsRuntime)(nil)
var _ SkillsDiagnosticsRuntime = (*diagnosticSkillsRuntime)(nil)
var _ SkillsSourcesRuntime = (*sourceStatusSkillsRuntime)(nil)
var _ SkillsWorkspaceRuntime = (*sourceStatusSkillsRuntime)(nil)
