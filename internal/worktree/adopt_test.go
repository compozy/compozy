package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/config"
)

// Canonical suite: strict linked-worktree adoption identity and idempotency.
func TestServiceAdopt(t *testing.T) {
	t.Parallel()

	t.Run("Should adopt an exact linked worktree without running bootstrap", func(t *testing.T) {
		t.Parallel()
		fixture := newAdoptionTestFixture(t)
		item, err := fixture.service.Adopt(context.Background(), fixture.workspace.ID, fixture.candidate)
		if err != nil {
			t.Fatalf("Adopt() error = %v", err)
		}
		if item.State != StateReady || item.Origin != OriginAdopted || item.Branch != "feature/adopt" ||
			item.GitDir != fixture.adminGitDir || item.SetupState != SetupNone {
			t.Fatalf("Adopt() = %#v, want ready adopted fingerprint", item)
		}
		for _, command := range invocationCommands(fixture.runner.invocations()) {
			if strings.HasPrefix(command, "ls-files ") || strings.Contains(command, "worktree add") {
				t.Fatalf("adoption ran bootstrap or mutated Git: %q", command)
			}
		}
	})

	t.Run("Should return the existing row on repeated adoption", func(t *testing.T) {
		t.Parallel()
		fixture := newAdoptionTestFixture(t)
		events := &recordingEventSink{}
		fixture.service.events = events
		first, err := fixture.service.Adopt(context.Background(), fixture.workspace.ID, fixture.candidate)
		if err != nil {
			t.Fatalf("Adopt(first) error = %v", err)
		}
		callsBefore := len(fixture.runner.invocations())
		second, err := fixture.service.Adopt(context.Background(), fixture.workspace.ID, fixture.candidate)
		if err != nil || second.ID != first.ID {
			t.Fatalf("Adopt(second) = %#v, %v, want existing %q", second, err, first.ID)
		}
		if rows, listErr := fixture.store.List(context.Background(), fixture.workspace.ID); listErr != nil || len(rows) != 1 {
			t.Fatalf("List() = %#v, %v, want one row", rows, listErr)
		}
		if got := len(fixture.runner.invocations()); got <= callsBefore+1 {
			t.Fatalf("second adoption Git calls = %d, want full identity revalidation", got-callsBefore)
		}
		if got := events.count(EventAdopted); got != 1 {
			t.Fatalf("adopted events = %d, want one across retry", got)
		}
	})

	t.Run("Should refuse idempotent adoption after the checkout identity is replaced", func(t *testing.T) {
		t.Parallel()
		fixture := newAdoptionTestFixture(t)
		if _, err := fixture.service.Adopt(context.Background(), fixture.workspace.ID, fixture.candidate); err != nil {
			t.Fatalf("Adopt(first) error = %v", err)
		}
		replacementAdmin := filepath.Join(fixture.commonDir, "worktrees", "replacement")
		if err := os.MkdirAll(replacementAdmin, 0o700); err != nil {
			t.Fatalf("MkdirAll(replacement admin) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(replacementAdmin, "commondir"), []byte("../..\n"), 0o600); err != nil {
			t.Fatalf("write replacement commondir: %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(replacementAdmin, "gitdir"), []byte(filepath.Join(fixture.candidate, ".git")+"\n"), 0o600,
		); err != nil {
			t.Fatalf("write replacement backlink: %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(fixture.candidate, ".git"), []byte("gitdir: "+replacementAdmin+"\n"), 0o600,
		); err != nil {
			t.Fatalf("replace candidate identity: %v", err)
		}
		if _, err := fixture.service.Adopt(
			context.Background(), fixture.workspace.ID, fixture.candidate,
		); !errors.Is(err, ErrAdoptionUnreadable) || !strings.Contains(err.Error(), "identity changed") {
			t.Fatalf("Adopt(replaced identity) error = %v, want classified refusal", err)
		}
	})

	t.Run("Should classify main checkout and vanished candidates", func(t *testing.T) {
		t.Parallel()
		fixture := newAdoptionTestFixture(t)
		if _, err := fixture.service.Adopt(context.Background(), fixture.workspace.ID, fixture.workspace.Root); !errors.Is(err, ErrAdoptionMainCheckout) {
			t.Fatalf("Adopt(main) error = %v, want ErrAdoptionMainCheckout", err)
		}
		if _, err := fixture.service.Adopt(context.Background(), fixture.workspace.ID, filepath.Join(t.TempDir(), "gone")); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Adopt(vanished) error = %v, want ErrNotFound", err)
		}
	})

	t.Run("Should reject foreign and broken bidirectional administration links", func(t *testing.T) {
		t.Parallel()
		foreign := newAdoptionTestFixture(t)
		foreignAdmin := filepath.Join(t.TempDir(), "foreign-admin")
		if err := os.MkdirAll(foreignAdmin, 0o700); err != nil {
			t.Fatalf("create foreign admin dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(foreign.candidate, ".git"), []byte("gitdir: "+foreignAdmin+"\n"), 0o600); err != nil {
			t.Fatalf("write foreign .git pointer: %v", err)
		}
		if _, err := foreign.service.Adopt(context.Background(), foreign.workspace.ID, foreign.candidate); !errors.Is(err, ErrAdoptionForeignRepo) {
			t.Fatalf("Adopt(foreign) error = %v, want ErrAdoptionForeignRepo", err)
		}

		broken := newAdoptionTestFixture(t)
		if err := os.WriteFile(filepath.Join(broken.adminGitDir, "gitdir"), []byte(filepath.Join(t.TempDir(), ".git")), 0o600); err != nil {
			t.Fatalf("write broken backlink: %v", err)
		}
		if _, err := broken.service.Adopt(context.Background(), broken.workspace.ID, broken.candidate); !errors.Is(err, ErrAdoptionUnreadable) {
			t.Fatalf("Adopt(broken backlink) error = %v, want ErrAdoptionUnreadable", err)
		}
	})

	t.Run("Should recover a missing commondir file without weakening identity checks", func(t *testing.T) {
		t.Parallel()
		fixture := newAdoptionTestFixture(t)
		if err := os.Remove(filepath.Join(fixture.adminGitDir, "commondir")); err != nil {
			t.Fatalf("Remove(commondir) error = %v", err)
		}
		item, err := fixture.service.Adopt(context.Background(), fixture.workspace.ID, fixture.candidate)
		if err != nil || item == nil || item.GitDir != fixture.adminGitDir {
			t.Fatalf("Adopt(missing commondir) = (%#v, %v), want strict fallback success", item, err)
		}
	})

	t.Run("Should reject core worktree redirection and submodule administration", func(t *testing.T) {
		t.Parallel()
		redirected := newAdoptionTestFixture(t)
		baseRun := redirected.runner.run
		redirected.runner.run = func(call gitInvocation) gitResponse {
			if strings.Join(call.args, " ") == "config --get core.worktree" {
				return gitResponse{stdout: []byte("/redirected\n")}
			}
			return baseRun(call)
		}
		if _, err := redirected.service.Adopt(
			context.Background(), redirected.workspace.ID, redirected.candidate,
		); !errors.Is(err, ErrAdoptionUnreadable) {
			t.Fatalf("Adopt(core.worktree) error = %v, want ErrAdoptionUnreadable", err)
		}

		submodule := newAdoptionTestFixture(t)
		submoduleAdmin := filepath.Join(submodule.commonDir, "modules", "nested")
		if err := os.MkdirAll(submoduleAdmin, 0o700); err != nil {
			t.Fatalf("MkdirAll(submodule admin) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(submodule.candidate, ".git"), []byte("gitdir: "+submoduleAdmin+"\n"), 0o600,
		); err != nil {
			t.Fatalf("WriteFile(submodule pointer) error = %v", err)
		}
		if _, err := submodule.service.Adopt(
			context.Background(), submodule.workspace.ID, submodule.candidate,
		); !errors.Is(err, ErrAdoptionUnreadable) || !strings.Contains(err.Error(), "submodule") {
			t.Fatalf("Adopt(submodule) error = %v, want classified unreadable submodule", err)
		}
	})

	t.Run("Should reject candidates absent from porcelain and bare repositories", func(t *testing.T) {
		t.Parallel()
		absent := newAdoptionTestFixture(t)
		baseRun := absent.runner.run
		absent.runner.run = func(call gitInvocation) gitResponse {
			if strings.Contains(strings.Join(call.args, " "), "worktree list --porcelain -z") {
				return gitResponse{stdout: worktreeListFixture(absent.workspace.Root, "main")}
			}
			return baseRun(call)
		}
		if _, err := absent.service.Adopt(
			context.Background(), absent.workspace.ID, absent.candidate,
		); !errors.Is(err, ErrAdoptionUnreadable) || !strings.Contains(err.Error(), "not an attached") {
			t.Fatalf("Adopt(absent from porcelain) error = %v, want classified unreadable", err)
		}

		bare := newAdoptionTestFixture(t)
		barePath := t.TempDir()
		if err := os.WriteFile(filepath.Join(barePath, "HEAD"), []byte("ref: refs/heads/main\n"), 0o600); err != nil {
			t.Fatalf("write bare HEAD: %v", err)
		}
		if _, err := bare.service.Adopt(
			context.Background(), bare.workspace.ID, barePath,
		); !errors.Is(err, ErrAdoptionUnreadable) {
			t.Fatalf("Adopt(bare) error = %v, want ErrAdoptionUnreadable", err)
		}
	})

	t.Run("Should reject an admin-directory symlink that escapes the repository", func(t *testing.T) {
		t.Parallel()
		fixture := newAdoptionTestFixture(t)
		outsideAdmin := t.TempDir()
		linkPath := filepath.Join(fixture.commonDir, "worktrees", "escape")
		if err := os.Symlink(outsideAdmin, linkPath); err != nil {
			t.Fatalf("Symlink(admin escape) error = %v", err)
		}
		if err := os.WriteFile(
			filepath.Join(fixture.candidate, ".git"), []byte("gitdir: "+linkPath+"\n"), 0o600,
		); err != nil {
			t.Fatalf("write escaping .git pointer: %v", err)
		}
		if _, err := fixture.service.Adopt(
			context.Background(), fixture.workspace.ID, fixture.candidate,
		); !errors.Is(err, ErrAdoptionUnreadable) || !strings.Contains(err.Error(), "escapes") {
			t.Fatalf("Adopt(admin symlink escape) error = %v, want classified unreadable", err)
		}
	})
}

type adoptionTestFixture struct {
	service     *Service
	store       *memoryWorktreeStore
	runner      *recordingGitRunner
	workspace   Workspace
	candidate   string
	commonDir   string
	adminGitDir string
}

func newAdoptionTestFixture(t *testing.T) adoptionTestFixture {
	t.Helper()
	workspaceRoot := t.TempDir()
	commonDir, err := filepath.EvalSymlinks(workspaceRoot)
	if err != nil {
		t.Fatalf("canonicalize workspace root: %v", err)
	}
	commonDir = filepath.Join(commonDir, ".git")
	if err := os.MkdirAll(filepath.Join(commonDir, "worktrees"), 0o700); err != nil {
		t.Fatalf("create common directory: %v", err)
	}
	candidateRaw := t.TempDir()
	candidate, err := filepath.EvalSymlinks(candidateRaw)
	if err != nil {
		t.Fatalf("canonicalize candidate: %v", err)
	}
	adminGitDir := filepath.Join(commonDir, "worktrees", "adopted")
	if err := os.MkdirAll(adminGitDir, 0o700); err != nil {
		t.Fatalf("create admin directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(candidate, ".git"), []byte("gitdir: "+adminGitDir+"\n"), 0o600); err != nil {
		t.Fatalf("write candidate .git pointer: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adminGitDir, "commondir"), []byte("../..\n"), 0o600); err != nil {
		t.Fatalf("write commondir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(adminGitDir, "gitdir"), []byte(filepath.Join(candidate, ".git")+"\n"), 0o600); err != nil {
		t.Fatalf("write backlink: %v", err)
	}
	workspace := Workspace{ID: "ws-adopt", Name: "Adopt", Root: workspaceRoot}
	runner := &recordingGitRunner{run: func(call gitInvocation) gitResponse {
		switch strings.Join(call.args, " ") {
		case "rev-parse --path-format=absolute --git-common-dir":
			return gitResponse{stdout: []byte(commonDir + "\n")}
		case "--git-dir " + commonDir + " worktree list --porcelain -z":
			return gitResponse{stdout: []byte(
				"worktree " + workspaceRoot + "\x00HEAD root\x00branch refs/heads/main\x00\x00" +
					"worktree " + candidate + "\x00HEAD feature\x00branch refs/heads/feature/adopt\x00\x00",
			)}
		case "config --get core.worktree":
			return gitResponse{err: errors.New("unset")}
		default:
			return gitResponse{err: errors.New("unexpected adoption call")}
		}
	}}
	store := newMemoryWorktreeStore()
	service := NewService(
		store, runner, WithCapabilityGate(readyCapabilityGate()),
		WithWorkspaceResolver(staticWorkspaceResolver{workspaces: map[string]Workspace{workspace.ID: workspace}}),
		WithConfig(config.DefaultWorktreesConfig(), t.TempDir()),
		WithIDGenerator(func(string) (string, error) { return "wt-adopt", nil }),
		WithClock(func() time.Time { return time.Date(2026, 8, 12, 14, 0, 0, 0, time.UTC) }),
	)
	return adoptionTestFixture{
		service: service, store: store, runner: runner, workspace: workspace,
		candidate: candidate, commonDir: commonDir, adminGitDir: adminGitDir,
	}
}
