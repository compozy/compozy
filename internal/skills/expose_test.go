package skills

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/store"
)

// Invariant: the exposure manager is the sole owner of provider links and
// reconciles durable ownership against a real filesystem before every mutation.
// Owning layer: internal/skills. Canonical suite: TestExposeManager in this file.
func TestExposeManager(t *testing.T) {
	t.Parallel()

	t.Run("Should expose idempotently and unexpose without touching the canonical skill", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t, "agents")

		results, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents"})
		if err != nil {
			t.Fatalf("Expose() error = %v", err)
		}
		assertHealthyResult(t, results, "agents")
		record := fixture.store.onlyRecord(t)
		if record.LinkTarget == "" {
			t.Fatal("record LinkTarget is empty")
		}
		before, err := os.Lstat(record.LinkPath)
		if err != nil {
			t.Fatalf("Lstat(link) error = %v", err)
		}
		createdAt := record.UpdatedAt

		time.Sleep(2 * time.Millisecond)
		repeated, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents"})
		if err != nil {
			t.Fatalf("repeat Expose() error = %v", err)
		}
		assertHealthyResult(t, repeated, "agents")
		after, err := os.Lstat(record.LinkPath)
		if err != nil {
			t.Fatalf("Lstat(repeated link) error = %v", err)
		}
		if !after.ModTime().Equal(before.ModTime()) {
			t.Fatalf("repeat expose changed link mtime: before=%v after=%v", before.ModTime(), after.ModTime())
		}
		if got := fixture.store.onlyRecord(t).UpdatedAt; !got.Equal(createdAt) {
			t.Fatalf("repeat expose changed record updated_at: before=%v after=%v", createdAt, got)
		}

		removed, err := fixture.manager.Unexpose(context.Background(), fixture.skill, []string{"agents"})
		if err != nil {
			t.Fatalf("Unexpose() error = %v", err)
		}
		if len(removed) != 1 || !removed[0].OK {
			t.Fatalf("Unexpose() results = %#v", removed)
		}
		if _, err := os.Stat(fixture.skill.Dir); err != nil {
			t.Fatalf("canonical skill removed: %v", err)
		}
		if _, err := os.Lstat(record.LinkPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("link still exists, error = %v", err)
		}
		if got := fixture.store.recordCount(); got != 0 {
			t.Fatalf("record count = %d, want 0", got)
		}
	})

	t.Run("Should refuse ineligible and disabled targets before mutation", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t, "agents")
		cases := []struct {
			name   string
			skill  *Skill
			target string
			code   string
		}{
			{name: "bundled", skill: &Skill{Meta: SkillMeta{Name: "bundled"}, Source: SourceBundled}, target: "agents", code: ExposureCodeSkillNotExposable},
			{name: "disabled", skill: fixture.skill, target: "claude", code: ExposureCodeTargetDisabled},
			{name: "custom", skill: fixture.skill, target: "team", code: ExposureCodeTargetInvalid},
		}
		fixture.manager.roots = append(fixture.manager.roots, compozyconfig.SkillRootSpec{
			Dir: filepath.Join(fixture.base, "team"), SourceSlug: "team", Kind: compozyconfig.RootKindCustom,
			ResourceScope: userExposureScope(),
		})
		for _, tc := range cases {
			t.Run("Should refuse "+tc.name, func(t *testing.T) {
				results, err := fixture.manager.Expose(context.Background(), tc.skill, []string{tc.target})
				assertExposureCode(t, err, tc.code)
				if len(results) != 1 {
					t.Fatalf("results length = %d", len(results))
				}
			})
		}
		if got := fixture.store.recordCount(); got != 0 {
			t.Fatalf("record count = %d after refusals", got)
		}
		if _, err := os.Lstat(fixture.root("agents")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("provider root mutated, error = %v", err)
		}
	})

	t.Run("Should refuse profile ownership before root store or link activity", func(t *testing.T) {
		t.Parallel()
		for _, kind := range []resources.ResourceScopeKind{
			resources.ResourceScopeKindProfile,
			resources.ResourceScopeKindWorkspaceProfile,
		} {
			fixture := newExposureFixture(t, "agents")
			fixture.skill.ResourceScope = resources.ResourceScope{Kind: kind, ID: "profile-a"}
			_, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents"})
			assertExposureCode(t, err, ExposureCodeProfileSkillNotExposable)
			if got := fixture.store.operationCount(); got != 0 {
				t.Fatalf("store operations = %d, want 0", got)
			}
			if _, err := os.Lstat(fixture.root("agents")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("provider root mutated, error = %v", err)
			}
		}
	})

	t.Run("Should report and preserve foreign entries", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t, "agents")
		if err := os.MkdirAll(fixture.root("agents"), 0o755); err != nil {
			t.Fatalf("MkdirAll(root) error = %v", err)
		}
		foreignPath := filepath.Join(fixture.root("agents"), fixture.skill.Meta.Name)
		if err := os.WriteFile(foreignPath, []byte("foreign"), 0o600); err != nil {
			t.Fatalf("WriteFile(foreign) error = %v", err)
		}
		before, err := os.ReadFile(foreignPath)
		if err != nil {
			t.Fatalf("ReadFile(foreign) error = %v", err)
		}
		_, err = fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents"})
		assertExposureCode(t, err, ExposureCodeNameConflict)
		states, err := fixture.manager.Exposures(context.Background(), fixture.skill)
		if err != nil {
			t.Fatalf("Exposures() error = %v", err)
		}
		assertExposureStatus(t, states, "agents", ExposureForeignConflict)
		_, err = fixture.manager.Unexpose(context.Background(), fixture.skill, []string{"agents"})
		assertExposureCode(t, err, ExposureCodeForeignLink)
		after, err := os.ReadFile(foreignPath)
		if err != nil {
			t.Fatalf("ReadFile(foreign after) error = %v", err)
		}
		if string(after) != string(before) {
			t.Fatalf("foreign entry changed: before=%q after=%q", before, after)
		}
	})

	t.Run("Should reconcile healthy missing broken and foreign records", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t, "agents", "claude")
		_, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents", "claude"})
		if err != nil {
			t.Fatalf("Expose() error = %v", err)
		}
		for _, target := range []string{"codex", "gemini"} {
			root := filepath.Join(fixture.base, "providers", target, "skills")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatalf("MkdirAll(%s) error = %v", target, err)
			}
			linkPath, err := resolveExposeDest(root, fixture.skill.Meta.Name)
			if err != nil {
				t.Fatalf("resolveExposeDest(%s) error = %v", target, err)
			}
			canonical, err := filepath.EvalSymlinks(fixture.skill.Dir)
			if err != nil {
				t.Fatalf("EvalSymlinks(canonical) error = %v", err)
			}
			linkTarget := relativeExposureTarget(linkPath, canonical)
			record, err := fixture.store.CreateSkillExposure(context.Background(), ExposureRecord{
				SkillName: fixture.skill.Meta.Name, CanonicalDir: canonical, TargetSlug: target,
				LinkPath: linkPath, LinkTarget: linkTarget, OwnerScope: store.SkillExposureOwnerUser,
			})
			if err != nil {
				t.Fatalf("CreateSkillExposure(%s) error = %v", target, err)
			}
			if err := os.Symlink(record.LinkTarget, record.LinkPath); err != nil {
				t.Fatalf("Symlink(%s) error = %v", target, err)
			}
		}
		records := fixture.store.recordsSnapshot()
		byTarget := recordsByTarget(records)
		if err := os.Remove(byTarget["claude"].LinkPath); err != nil {
			t.Fatalf("remove missing fixture link: %v", err)
		}
		if err := os.RemoveAll(fixture.skill.Dir); err != nil {
			t.Fatalf("remove canonical fixture: %v", err)
		}
		foreignCanonical := filepath.Join(fixture.base, "foreign")
		if err := os.MkdirAll(foreignCanonical, 0o755); err != nil {
			t.Fatalf("MkdirAll(foreign canonical) error = %v", err)
		}
		if err := os.Remove(byTarget["gemini"].LinkPath); err != nil {
			t.Fatalf("remove retarget fixture link: %v", err)
		}
		if err := os.Symlink(foreignCanonical, byTarget["gemini"].LinkPath); err != nil {
			t.Fatalf("Symlink(foreign) error = %v", err)
		}

		states, err := fixture.manager.Exposures(context.Background(), fixture.skill)
		if err != nil {
			t.Fatalf("Exposures() error = %v", err)
		}
		assertExposureStatus(t, states, "agents", ExposureBroken)
		assertExposureStatus(t, states, "claude", ExposureMissing)
		assertExposureStatus(t, states, "codex", ExposureBroken)
		assertExposureStatus(t, states, "gemini", ExposureForeignConflict)
	})

	t.Run("Should preflight every target and roll back completed commits", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t, "agents", "claude")
		if err := os.MkdirAll(fixture.root("claude"), 0o755); err != nil {
			t.Fatalf("MkdirAll(claude) error = %v", err)
		}
		conflict := filepath.Join(fixture.root("claude"), fixture.skill.Meta.Name)
		if err := os.WriteFile(conflict, []byte("foreign"), 0o600); err != nil {
			t.Fatalf("WriteFile(conflict) error = %v", err)
		}
		_, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents", "claude"})
		assertExposureCode(t, err, ExposureCodeNameConflict)
		if fixture.store.recordCount() != 0 {
			t.Fatal("preflight failure inserted a record")
		}
		if _, err := os.Lstat(filepath.Join(fixture.root("agents"), fixture.skill.Meta.Name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("preflight failure created first link, error = %v", err)
		}

		if err := os.Remove(conflict); err != nil {
			t.Fatalf("Remove(conflict) error = %v", err)
		}
		failedPath, resolveErr := resolveExposeDest(fixture.root("claude"), fixture.skill.Meta.Name)
		if resolveErr != nil {
			t.Fatalf("resolveExposeDest(failure path) error = %v", resolveErr)
		}
		faults := &faultExposureFS{exposureFS: osExposureFS{}, failSymlinkPath: failedPath}
		fixture.manager.fs = faults
		results, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents", "claude"})
		assertExposureCode(t, err, ExposureCodeLinkUnsupported)
		if len(results) != 2 || !results[0].RolledBack || exposureErrorCode(results[0].Err) != ExposureCodeRolledBack {
			t.Fatalf("rollback results = %#v", results)
		}
		if fixture.store.recordCount() != 0 {
			t.Fatal("commit failure left records")
		}
		if _, err := os.Lstat(filepath.Join(fixture.root("agents"), fixture.skill.Meta.Name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("commit failure left completed link, error = %v", err)
		}
		if _, err := os.Stat(filepath.Join(fixture.root("claude"), fixture.skill.Meta.Name, "SKILL.md")); err == nil {
			t.Fatal("symlink failure fell back to copying")
		}
	})

	t.Run("Should create absent roots and remove only self-created empty directories on rollback", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t, "agents", "claude")
		failedPath, resolveErr := resolveExposeDest(fixture.root("claude"), fixture.skill.Meta.Name)
		if resolveErr != nil {
			t.Fatalf("resolveExposeDest(failure path) error = %v", resolveErr)
		}
		fixture.manager.fs = &faultExposureFS{exposureFS: osExposureFS{}, failSymlinkPath: failedPath}
		_, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents", "claude"})
		assertExposureCode(t, err, ExposureCodeLinkUnsupported)
		if _, err := os.Stat(fixture.root("agents")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("empty self-created agents root remains, error = %v", err)
		}
		if _, err := os.Stat(fixture.root("claude")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("empty self-created claude root remains, error = %v", err)
		}

		success := newExposureFixture(t, "agents")
		results, err := success.manager.Expose(context.Background(), success.skill, []string{"agents"})
		if err != nil {
			t.Fatalf("Expose(absent root) error = %v", err)
		}
		assertHealthyResult(t, results, "agents")
		if info, err := os.Stat(success.root("agents")); err != nil || !info.IsDir() {
			t.Fatalf("created root info=%v error=%v", info, err)
		}

		foreign := newExposureFixture(t, "agents", "claude")
		foreignClaudePath, err := resolveExposeDest(foreign.root("claude"), foreign.skill.Meta.Name)
		if err != nil {
			t.Fatalf("resolveExposeDest(foreign failure path) error = %v", err)
		}
		foreignFile := filepath.Join(foreign.root("agents"), "foreign.txt")
		foreign.manager.fs = &faultExposureFS{
			exposureFS: osExposureFS{}, failSymlinkPath: foreignClaudePath,
			beforeSymlinkFailure: func() {
				if err := os.WriteFile(foreignFile, []byte("foreign"), 0o600); err != nil {
					t.Errorf("WriteFile(foreign rollback fixture) error = %v", err)
				}
			},
		}
		_, err = foreign.manager.Expose(context.Background(), foreign.skill, []string{"agents", "claude"})
		assertExposureCode(t, err, ExposureCodeLinkUnsupported)
		if content, err := os.ReadFile(foreignFile); err != nil || string(content) != "foreign" {
			t.Fatalf("foreign rollback content=%q error=%v", content, err)
		}
	})

	t.Run("Should remove link before record and converge after record deletion failure", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t, "agents")
		_, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents"})
		if err != nil {
			t.Fatalf("Expose() error = %v", err)
		}
		record := fixture.store.onlyRecord(t)
		fixture.store.failDeleteOnce = errors.New("simulated crash")
		_, err = fixture.manager.Unexpose(context.Background(), fixture.skill, []string{"agents"})
		if err == nil {
			t.Fatal("Unexpose() error = nil, want simulated crash")
		}
		if _, err := os.Lstat(record.LinkPath); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("link exists after delete-record failure, error = %v", err)
		}
		states, err := fixture.manager.Exposures(context.Background(), fixture.skill)
		if err != nil {
			t.Fatalf("Exposures() error = %v", err)
		}
		assertExposureStatus(t, states, "agents", ExposureMissing)
		results, err := fixture.manager.Unexpose(context.Background(), fixture.skill, []string{"agents"})
		if err != nil || len(results) != 1 || !results[0].OK {
			t.Fatalf("retry Unexpose() results=%#v error=%v", results, err)
		}
		if fixture.store.recordCount() != 0 {
			t.Fatal("retry unexpose did not delete missing-link record")
		}
	})

	t.Run("Should repair a missing owned link and preserve one record", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t, "agents")
		_, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents"})
		if err != nil {
			t.Fatalf("Expose() error = %v", err)
		}
		record := fixture.store.onlyRecord(t)
		if err := os.Remove(record.LinkPath); err != nil {
			t.Fatalf("Remove(link) error = %v", err)
		}
		states, err := fixture.manager.Exposures(context.Background(), fixture.skill)
		if err != nil {
			t.Fatalf("Exposures() error = %v", err)
		}
		assertExposureStatus(t, states, "agents", ExposureMissing)
		results, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents"})
		if err != nil {
			t.Fatalf("repair Expose() error = %v", err)
		}
		assertHealthyResult(t, results, "agents")
		if got := fixture.store.recordCount(); got != 1 {
			t.Fatalf("record count after repair = %d, want 1", got)
		}
	})

	t.Run("Should clean every recorded target including a disabled preset before canonical removal", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t, "agents", "claude")
		_, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents", "claude"})
		if err != nil {
			t.Fatalf("Expose() error = %v", err)
		}
		records := fixture.store.recordsSnapshot()
		fixture.manager.roots = fixture.manager.roots[:1]
		if err := fixture.manager.CleanupCanonicalDir(context.Background(), fixture.skill.Dir); err != nil {
			t.Fatalf("CleanupCanonicalDir() error = %v", err)
		}
		for _, record := range records {
			if _, err := os.Lstat(record.LinkPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("link %q remains, error = %v", record.LinkPath, err)
			}
		}
		if fixture.store.recordCount() != 0 {
			t.Fatal("cleanup left exposure records")
		}
		if _, err := os.Stat(fixture.skill.Dir); err != nil {
			t.Fatalf("cleanup removed canonical dir: %v", err)
		}
	})

	t.Run("Should block removal on an uncleanable owned link and complete on retry", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t, "agents")
		_, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents"})
		if err != nil {
			t.Fatalf("Expose() error = %v", err)
		}
		record := fixture.store.onlyRecord(t)
		fixture.manager.fs = &faultExposureFS{exposureFS: osExposureFS{}, failRemovePath: record.LinkPath}
		err = fixture.manager.CleanupCanonicalDir(context.Background(), fixture.skill.Dir)
		assertExposureCode(t, err, ExposureCodeSkillRemoveBlocked)
		var exposureErr *ExposureError
		if !errors.As(err, &exposureErr) || !exposureErr.Retryable || exposureErr.Path != record.LinkPath {
			t.Fatalf("cleanup error = %#v", exposureErr)
		}
		if fixture.store.recordCount() != 1 {
			t.Fatal("blocked cleanup deleted ownership record")
		}
		if _, err := os.Stat(fixture.skill.Dir); err != nil {
			t.Fatalf("blocked cleanup removed canonical dir: %v", err)
		}
		fixture.manager.fs = osExposureFS{}
		if err := fixture.manager.CleanupCanonicalDir(context.Background(), fixture.skill.Dir); err != nil {
			t.Fatalf("retry CleanupCanonicalDir() error = %v", err)
		}
		if fixture.store.recordCount() != 0 {
			t.Fatal("retry cleanup left record")
		}
	})

	t.Run("Should retain ownership and emit cleanup failure when rollback cannot remove a link", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t, "agents", "claude")
		agentsPath, err := resolveExposeDest(fixture.root("agents"), fixture.skill.Meta.Name)
		if err != nil {
			t.Fatalf("resolveExposeDest(agents) error = %v", err)
		}
		claudePath, err := resolveExposeDest(fixture.root("claude"), fixture.skill.Meta.Name)
		if err != nil {
			t.Fatalf("resolveExposeDest(claude) error = %v", err)
		}
		events := &recordingExposureEvents{}
		fixture.manager.events = events
		fixture.manager.fs = &faultExposureFS{
			exposureFS: osExposureFS{}, failSymlinkPath: claudePath, failRemovePath: agentsPath,
		}
		results, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents", "claude"})
		assertExposureCode(t, err, ExposureCodeLinkUnsupported)
		if len(results) != 2 || results[0].CleanupErr == nil || results[0].RolledBack {
			t.Fatalf("rollback cleanup results = %#v", results)
		}
		if got := fixture.store.recordCount(); got != 1 {
			t.Fatalf("record count = %d, want ownership proof retained", got)
		}
		if _, err := os.Lstat(agentsPath); err != nil {
			t.Fatalf("unremoved link missing after cleanup failure: %v", err)
		}
		if !events.hasType(exposureEventCleanupFailure) {
			t.Fatalf("events = %#v, want %q", events.summaries, exposureEventCleanupFailure)
		}
	})

	t.Run("Should correlate exposure events to the acting profile without changing user ownership", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t, "agents")
		events := &recordingExposureEvents{}
		fixture.manager.events = events
		ctx := WithSourceEventCorrelation(context.Background(), SourceEventCorrelation{
			ProfileID: "profile-acting", ActorKind: "agent", ActorID: "agent-7",
		})
		ctx = WithConfigGeneration(ctx, 42)
		_, err := fixture.manager.Expose(ctx, fixture.skill, []string{"agents"})
		if err != nil {
			t.Fatalf("Expose() error = %v", err)
		}
		summary := events.onlyType(t, exposureEventCreated)
		if summary.ProfileID != "profile-acting" || summary.WorkspaceID != "" {
			t.Fatalf("event correlation profile=%q workspace=%q", summary.ProfileID, summary.WorkspaceID)
		}
		var content exposureEventContent
		if err := json.Unmarshal(summary.Content, &content); err != nil {
			t.Fatalf("Unmarshal(event content) error = %v", err)
		}
		if content.ProfileID != "profile-acting" || content.ActorKind != "agent" || content.ActorID != "agent-7" ||
			content.ConfigGeneration != 42 || summary.EventCorrelation.ActorKind != "agent" ||
			summary.EventCorrelation.ActorID != "agent-7" ||
			content.OwnerScope != string(store.SkillExposureOwnerUser) || content.WorkspaceID != "" {
			t.Fatalf("event content = %#v", content)
		}
	})

	t.Run("Should expose workspace skills only into the same workspace preset root", func(t *testing.T) {
		t.Parallel()
		fixture := newExposureFixture(t)
		workspaceA := filepath.Join(fixture.base, "workspace-a", ".agents", "skills")
		workspaceB := filepath.Join(fixture.base, "workspace-b", ".agents", "skills")
		fixture.skill.Source = SourceWorkspace
		fixture.skill.ResourceScope = resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-a"}
		fixture.manager.roots = []compozyconfig.SkillRootSpec{
			{Dir: workspaceB, SourceSlug: "agents", Kind: compozyconfig.RootKindPreset, ResourceScope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-b"}},
			{Dir: workspaceA, SourceSlug: "agents", Kind: compozyconfig.RootKindPreset, ResourceScope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-a"}},
		}
		results, err := fixture.manager.Expose(context.Background(), fixture.skill, []string{"agents"})
		if err != nil {
			t.Fatalf("Expose(workspace) error = %v", err)
		}
		assertHealthyResult(t, results, "agents")
		record := fixture.store.onlyRecord(t)
		if record.OwnerScope != store.SkillExposureOwnerWorkspace || record.WorkspaceID != "workspace-a" ||
			filepath.Dir(record.LinkPath) != mustCanonicalPath(t, workspaceA) {
			t.Fatalf("workspace exposure record = %#v", record)
		}
		if _, err := os.Lstat(workspaceB); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("other workspace root mutated, error = %v", err)
		}
	})
}

func TestValidateExposeName(t *testing.T) {
	t.Parallel()
	invalid := []string{"", "/absolute", "a/b", `a\b`, "..", "%2e%2e", "nul\x00name", "e\u0301", strings.Repeat("a", 256)}
	for _, name := range invalid {
		name := name
		t.Run("Should reject "+strings.ReplaceAll(name, "/", "_"), func(t *testing.T) {
			t.Parallel()
			assertExposureCode(t, validateExposeName(name), ExposureCodeUnsafeSkillName)
		})
	}
	t.Run("Should accept the canonical skill-name grammar", func(t *testing.T) {
		t.Parallel()
		if err := validateExposeName("review-checklist_1.0"); err != nil {
			t.Fatalf("validateExposeName(valid) error = %v", err)
		}
	})
}

func TestResolveExposeDest(t *testing.T) {
	t.Parallel()
	t.Run("Should keep an absent destination inside the canonical root", func(t *testing.T) {
		t.Parallel()
		root := filepath.Join(t.TempDir(), "absent", "skills")
		got, err := resolveExposeDest(root, "review")
		if err != nil {
			t.Fatalf("resolveExposeDest() error = %v", err)
		}
		parent, err := filepath.EvalSymlinks(filepath.Dir(filepath.Dir(root)))
		if err != nil {
			t.Fatalf("EvalSymlinks(existing prefix) error = %v", err)
		}
		want := filepath.Join(parent, "absent", "skills", "review")
		if got != want {
			t.Fatalf("resolveExposeDest() = %q, want %q", got, want)
		}
	})

	t.Run("Should reject a root that traverses a symlinked parent", func(t *testing.T) {
		t.Parallel()
		base := t.TempDir()
		outside := t.TempDir()
		alias := filepath.Join(base, "alias")
		if err := os.Symlink(outside, alias); err != nil {
			t.Fatalf("Symlink(alias) error = %v", err)
		}
		_, err := resolveExposeDest(filepath.Join(alias, "skills"), "review")
		assertExposureCode(t, err, ExposureCodeNameConflict)
	})
}

type exposureFixture struct {
	base    string
	skill   *Skill
	store   *memoryExposureStore
	manager *ExposeManager
	roots   map[string]string
}

func newExposureFixture(t *testing.T, targets ...string) *exposureFixture {
	t.Helper()
	base := t.TempDir()
	canonical := filepath.Join(base, "compozy", "skills", "review")
	if err := os.MkdirAll(canonical, 0o755); err != nil {
		t.Fatalf("MkdirAll(canonical) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(canonical, "SKILL.md"), []byte("---\nname: review\n---\n"), 0o600); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}
	repository := &memoryExposureStore{nextID: 1}
	roots := make(map[string]string, len(targets))
	specs := make([]compozyconfig.SkillRootSpec, 0, len(targets))
	for _, target := range targets {
		root := filepath.Join(base, "providers", target, "skills")
		roots[target] = root
		specs = append(specs, compozyconfig.SkillRootSpec{
			Dir: root, SourceSlug: target, Kind: compozyconfig.RootKindPreset, ResourceScope: userExposureScope(),
		})
	}
	return &exposureFixture{
		base: base,
		skill: &Skill{
			Meta: SkillMeta{Name: "review"}, Source: SourceUser, Dir: canonical, ResourceScope: userExposureScope(),
		},
		store: repository, manager: NewExposeManager(repository, specs), roots: roots,
	}
}

func (f *exposureFixture) root(target string) string { return f.roots[target] }

func userExposureScope() resources.ResourceScope {
	return resources.ResourceScope{Kind: resources.ResourceScopeKindUser}
}

type memoryExposureStore struct {
	mu             sync.Mutex
	nextID         int64
	records        []store.SkillExposureRecord
	operations     []string
	failCreateAt   string
	failDeleteOnce error
}

func (s *memoryExposureStore) CreateSkillExposure(
	_ context.Context,
	record store.SkillExposureRecord,
) (store.SkillExposureRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations = append(s.operations, "create:"+record.TargetSlug)
	if record.TargetSlug == s.failCreateAt {
		return store.SkillExposureRecord{}, errors.New("injected create failure")
	}
	for _, existing := range s.records {
		if existing.LinkPath == record.LinkPath ||
			(existing.SkillName == record.SkillName && existing.OwnerScope == record.OwnerScope &&
				existing.WorkspaceID == record.WorkspaceID && existing.TargetSlug == record.TargetSlug) {
			return store.SkillExposureRecord{}, errors.New("duplicate exposure")
		}
	}
	record.ID = s.nextID
	s.nextID++
	record.CreatedAt = time.Now().UTC()
	record.UpdatedAt = record.CreatedAt
	s.records = append(s.records, record)
	return record, nil
}

func (s *memoryExposureStore) GetSkillExposureByOwnerTarget(
	_ context.Context,
	skillName string,
	ownerScope store.SkillExposureOwnerScope,
	workspaceID string,
	target string,
) (store.SkillExposureRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations = append(s.operations, "get:"+target)
	for _, record := range s.records {
		if record.SkillName == skillName && record.OwnerScope == ownerScope &&
			record.WorkspaceID == workspaceID && record.TargetSlug == target {
			return record, nil
		}
	}
	return store.SkillExposureRecord{}, sql.ErrNoRows
}

func (s *memoryExposureStore) ListSkillExposuresByOwner(
	_ context.Context,
	skillName string,
	ownerScope store.SkillExposureOwnerScope,
	workspaceID string,
) ([]store.SkillExposureRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations = append(s.operations, "list-owner")
	result := make([]store.SkillExposureRecord, 0)
	for _, record := range s.records {
		if record.SkillName == skillName && record.OwnerScope == ownerScope && record.WorkspaceID == workspaceID {
			result = append(result, record)
		}
	}
	return result, nil
}

func (s *memoryExposureStore) ListSkillExposuresByCanonicalDir(
	_ context.Context,
	canonicalDir string,
) ([]store.SkillExposureRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations = append(s.operations, "list-canonical")
	result := make([]store.SkillExposureRecord, 0)
	for _, record := range s.records {
		if record.CanonicalDir == canonicalDir {
			result = append(result, record)
		}
	}
	return result, nil
}

func (s *memoryExposureStore) DeleteSkillExposure(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations = append(s.operations, "delete")
	if s.failDeleteOnce != nil {
		err := s.failDeleteOnce
		s.failDeleteOnce = nil
		return err
	}
	index := slices.IndexFunc(s.records, func(record store.SkillExposureRecord) bool { return record.ID == id })
	if index < 0 {
		return sql.ErrNoRows
	}
	s.records = slices.Delete(s.records, index, index+1)
	return nil
}

func (s *memoryExposureStore) recordCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.records)
}

func (s *memoryExposureStore) operationCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.operations)
}

func (s *memoryExposureStore) recordsSnapshot() []store.SkillExposureRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.SkillExposureRecord(nil), s.records...)
}

func (s *memoryExposureStore) onlyRecord(t *testing.T) store.SkillExposureRecord {
	t.Helper()
	records := s.recordsSnapshot()
	if len(records) != 1 {
		t.Fatalf("record count = %d, want 1", len(records))
	}
	return records[0]
}

type faultExposureFS struct {
	exposureFS
	failSymlinkPath      string
	failRemovePath       string
	beforeSymlinkFailure func()
}

type recordingExposureEvents struct {
	mu        sync.Mutex
	summaries []store.EventSummary
}

func (e *recordingExposureEvents) WriteEventSummary(_ context.Context, summary store.EventSummary) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.summaries = append(e.summaries, summary)
	return nil
}

func (e *recordingExposureEvents) ListEventSummaries(
	context.Context,
	store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]store.EventSummary(nil), e.summaries...), nil
}

func (e *recordingExposureEvents) hasType(eventType string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.ContainsFunc(e.summaries, func(summary store.EventSummary) bool { return summary.Type == eventType })
}

func (e *recordingExposureEvents) onlyType(t *testing.T, eventType string) store.EventSummary {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	matches := make([]store.EventSummary, 0, 1)
	for _, summary := range e.summaries {
		if summary.Type == eventType {
			matches = append(matches, summary)
		}
	}
	if len(matches) != 1 {
		t.Fatalf("event %q matches = %d in %#v", eventType, len(matches), e.summaries)
	}
	return matches[0]
}

func (f *faultExposureFS) Symlink(target string, path string) error {
	if filepath.Clean(path) == filepath.Clean(f.failSymlinkPath) {
		if f.beforeSymlinkFailure != nil {
			f.beforeSymlinkFailure()
		}
		return &fs.PathError{Op: "symlink", Path: path, Err: fs.ErrPermission}
	}
	return f.exposureFS.Symlink(target, path)
}

func (f *faultExposureFS) Remove(path string) error {
	if filepath.Clean(path) == filepath.Clean(f.failRemovePath) {
		return &fs.PathError{Op: "remove", Path: path, Err: fs.ErrPermission}
	}
	return f.exposureFS.Remove(path)
}

func assertHealthyResult(t *testing.T, results []TargetResult, target string) {
	t.Helper()
	if len(results) != 1 || results[0].Target != target || !results[0].OK ||
		results[0].Exposure == nil || results[0].Exposure.Status != ExposureHealthy {
		t.Fatalf("results = %#v", results)
	}
}

func assertExposureCode(t *testing.T, err error, want string) {
	t.Helper()
	if got := exposureErrorCode(err); got != want {
		t.Fatalf("error code = %q, want %q; error = %v", got, want, err)
	}
}

func assertExposureStatus(t *testing.T, states []ExposureState, target string, want ExposureStatus) {
	t.Helper()
	for _, state := range states {
		if state.Record.TargetSlug == target {
			if state.Status != want {
				t.Fatalf("target %q status = %q, want %q", target, state.Status, want)
			}
			return
		}
	}
	t.Fatalf("target %q missing from states %#v", target, states)
}

func recordsByTarget(records []store.SkillExposureRecord) map[string]store.SkillExposureRecord {
	result := make(map[string]store.SkillExposureRecord, len(records))
	for _, record := range records {
		result[record.TargetSlug] = record
	}
	return result
}

func mustCanonicalPath(t *testing.T, path string) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("EvalSymlinks(%q) error = %v", path, err)
	}
	return canonical
}
