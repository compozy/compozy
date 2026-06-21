package worktree

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildScopeIdentifiesProducedPathsWithoutPreExistingNoise(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	root := initScopeGitRepo(t)
	target := filepath.Join(root, ".agents", "skills", "review")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir skill target: %v", err)
	}
	link := filepath.Join(root, ".claude", "skills", "review")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatalf("mkdir skill link dir: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	mustScopeGit(t, root, "add", ".")
	mustScopeGit(t, root, "commit", "-q", "-m", "track skill")

	rewrittenTarget := filepath.Join(root, ".agents", "skills", "review-worktree")
	if err := os.MkdirAll(rewrittenTarget, 0o755); err != nil {
		t.Fatalf("mkdir rewritten target: %v", err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatalf("remove link: %v", err)
	}
	if err := os.Symlink(rewrittenTarget, link); err != nil {
		t.Fatalf("rewrite link: %v", err)
	}
	baseline, err := Capture(context.Background(), root)
	if err != nil {
		t.Fatalf("Capture baseline: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "produced.txt"), []byte("agent output"), 0o600); err != nil {
		t.Fatalf("write produced: %v", err)
	}

	scope, err := BuildScope(context.Background(), root, baseline)
	if err != nil {
		t.Fatalf("BuildScope: %v", err)
	}
	if !scope.Supported {
		t.Fatalf("scope supported = false: %#v", scope)
	}
	if got, want := strings.Join(scope.ProducedPaths, ","), "produced.txt"; got != want {
		t.Fatalf("produced paths = %q, want %q", got, want)
	}
	if got, want := strings.Join(scope.PreExistingPaths, ","), ".claude/skills/review"; got != want {
		t.Fatalf("pre-existing paths = %q, want %q", got, want)
	}
	if len(scope.PreExistingChangedPaths) != 0 {
		t.Fatalf("pre-existing changed paths = %#v, want none", scope.PreExistingChangedPaths)
	}
}

func TestBuildScopeIgnoresTrackedAbsoluteSymlinkRetargetAfterBaseline(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	root := initScopeGitRepo(t)
	originalRoot := t.TempDir()
	originalTarget := filepath.Join(originalRoot, ".agents", "skills", "review")
	if err := os.MkdirAll(originalTarget, 0o755); err != nil {
		t.Fatalf("mkdir original skill target: %v", err)
	}
	link := filepath.Join(root, ".claude", "skills", "review")
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		t.Fatalf("mkdir skill link dir: %v", err)
	}
	if err := os.Symlink(originalTarget, link); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	mustScopeGit(t, root, "add", ".")
	mustScopeGit(t, root, "commit", "-q", "-m", "track skill")

	baseline, err := Capture(context.Background(), root)
	if err != nil {
		t.Fatalf("Capture baseline: %v", err)
	}
	rewrittenTarget := filepath.Join(root, ".agents", "skills", "review")
	if err := os.MkdirAll(rewrittenTarget, 0o755); err != nil {
		t.Fatalf("mkdir rewritten target: %v", err)
	}
	if err := os.Remove(link); err != nil {
		t.Fatalf("remove link: %v", err)
	}
	if err := os.Symlink(rewrittenTarget, link); err != nil {
		t.Fatalf("rewrite link: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "produced.txt"), []byte("agent output"), 0o600); err != nil {
		t.Fatalf("write produced: %v", err)
	}

	scope, err := BuildScope(context.Background(), root, baseline)
	if err != nil {
		t.Fatalf("BuildScope: %v", err)
	}
	if got, want := strings.Join(scope.ProducedPaths, ","), "produced.txt"; got != want {
		t.Fatalf("produced paths = %q, want %q", got, want)
	}
	if got, want := strings.Join(scope.PreExistingPaths, ","), ".claude/skills/review"; got != want {
		t.Fatalf("pre-existing paths = %q, want %q", got, want)
	}
	if len(scope.PreExistingChangedPaths) != 0 {
		t.Fatalf("pre-existing changed paths = %#v, want none", scope.PreExistingChangedPaths)
	}
}

func TestBuildScopeReportsContaminatedPreExistingChanges(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	root := initScopeGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# dirty\n"), 0o600); err != nil {
		t.Fatalf("dirty README: %v", err)
	}
	baseline, err := Capture(context.Background(), root)
	if err != nil {
		t.Fatalf("Capture baseline: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# dirtier\n"), 0o600); err != nil {
		t.Fatalf("change dirty README: %v", err)
	}
	scope, err := BuildScope(context.Background(), root, baseline)
	if err != nil {
		t.Fatalf("BuildScope: %v", err)
	}
	if got, want := strings.Join(scope.PreExistingChangedPaths, ","), "README.md"; got != want {
		t.Fatalf("pre-existing changed paths = %q, want %q", got, want)
	}
}

func initScopeGitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mustScopeGit(t, root, "init", "-q", "-b", "main")
	mustScopeGit(t, root, "config", "user.email", "scope@example.com")
	mustScopeGit(t, root, "config", "user.name", "Scope Tester")
	mustScopeGit(t, root, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# initial\n"), 0o600); err != nil {
		t.Fatalf("seed README: %v", err)
	}
	mustScopeGit(t, root, "add", "README.md")
	mustScopeGit(t, root, "commit", "-q", "-m", "initial")
	return root
}

func mustScopeGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z",
		"GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, string(out))
	}
}
