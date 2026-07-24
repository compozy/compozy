package worktree

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReset(t *testing.T) {
	t.Parallel()

	t.Run("Should discard commits, staged edits, and untracked files from a clean baseline", func(t *testing.T) {
		t.Parallel()
		requireScopeGit(t)

		root := initScopeGitRepo(t)
		baseline, err := Capture(context.Background(), root)
		if err != nil {
			t.Fatalf("Capture baseline: %v", err)
		}

		// Simulate everything a stalled agent could leave behind.
		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# rewritten\n"), 0o600); err != nil {
			t.Fatalf("rewrite README: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "committed.txt"), []byte("side effect"), 0o600); err != nil {
			t.Fatalf("write committed: %v", err)
		}
		mustScopeGit(t, root, "add", ".")
		mustScopeGit(t, root, "commit", "-q", "-m", "half-applied work")
		if err := os.WriteFile(filepath.Join(root, "staged.txt"), []byte("staged"), 0o600); err != nil {
			t.Fatalf("write staged: %v", err)
		}
		mustScopeGit(t, root, "add", "staged.txt")
		if err := os.MkdirAll(filepath.Join(root, "scratch"), 0o755); err != nil {
			t.Fatalf("mkdir scratch: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "scratch", "untracked.txt"), []byte("junk"), 0o600); err != nil {
			t.Fatalf("write untracked: %v", err)
		}

		if err := Reset(context.Background(), root, baseline); err != nil {
			t.Fatalf("Reset: %v", err)
		}

		final, err := Capture(context.Background(), root)
		if err != nil {
			t.Fatalf("Capture final: %v", err)
		}
		if !final.Equal(baseline) {
			t.Fatalf("final snapshot does not match baseline: head=%s want=%s", final.Head(), baseline.Head())
		}
		for _, rel := range []string{"committed.txt", "staged.txt", filepath.Join("scratch", "untracked.txt")} {
			if _, err := os.Stat(filepath.Join(root, rel)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("expected %s to be removed, stat err = %v", rel, err)
			}
		}
		content, err := os.ReadFile(filepath.Join(root, "README.md"))
		if err != nil {
			t.Fatalf("read README: %v", err)
		}
		if string(content) != "# initial\n" {
			t.Fatalf("README = %q, want restored baseline content", string(content))
		}
	})

	t.Run("Should restore a clean baseline captured with exclusions", func(t *testing.T) {
		t.Parallel()
		requireScopeGit(t)

		root := initScopeGitRepo(t)
		tasksDir := filepath.Join(root, ".compozy", "tasks", "demo")
		baseline, err := CaptureExcluding(context.Background(), root, tasksDir)
		if err != nil {
			t.Fatalf("CaptureExcluding baseline: %v", err)
		}

		if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# rewritten\n"), 0o600); err != nil {
			t.Fatalf("rewrite README: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "scratch.txt"), []byte("junk"), 0o600); err != nil {
			t.Fatalf("write scratch: %v", err)
		}

		if err := Reset(context.Background(), root, baseline); err != nil {
			t.Fatalf("Reset: %v", err)
		}

		final, err := CaptureExcluding(context.Background(), root, tasksDir)
		if err != nil {
			t.Fatalf("CaptureExcluding final: %v", err)
		}
		if !final.Equal(baseline) {
			t.Fatalf("final snapshot does not match excluded baseline: head=%s want=%s", final.Head(), baseline.Head())
		}
		if _, err := os.Stat(filepath.Join(root, "scratch.txt")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected scratch.txt to be removed, stat err = %v", err)
		}
	})

	t.Run("Should leave ignored files alone", func(t *testing.T) {
		t.Parallel()
		requireScopeGit(t)

		root := initScopeGitRepo(t)
		if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("build/\n"), 0o600); err != nil {
			t.Fatalf("write gitignore: %v", err)
		}
		mustScopeGit(t, root, "add", ".gitignore")
		mustScopeGit(t, root, "commit", "-q", "-m", "ignore build")
		if err := os.MkdirAll(filepath.Join(root, "build"), 0o755); err != nil {
			t.Fatalf("mkdir build: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "build", "cache.bin"), []byte("cache"), 0o600); err != nil {
			t.Fatalf("write cache: %v", err)
		}
		baseline, err := Capture(context.Background(), root)
		if err != nil {
			t.Fatalf("Capture baseline: %v", err)
		}
		if err := os.WriteFile(filepath.Join(root, "dirty.txt"), []byte("x"), 0o600); err != nil {
			t.Fatalf("write dirty: %v", err)
		}

		if err := Reset(context.Background(), root, baseline); err != nil {
			t.Fatalf("Reset: %v", err)
		}
		if _, err := os.Stat(filepath.Join(root, "build", "cache.bin")); err != nil {
			t.Fatalf("ignored file was removed: %v", err)
		}
	})

	t.Run("Should refuse a dirty baseline it cannot reconstruct", func(t *testing.T) {
		t.Parallel()
		requireScopeGit(t)

		root := initScopeGitRepo(t)
		if err := os.WriteFile(filepath.Join(root, "pre-existing.txt"), []byte("user work"), 0o600); err != nil {
			t.Fatalf("write pre-existing: %v", err)
		}
		baseline, err := CaptureExcluding(
			context.Background(),
			root,
			filepath.Join(root, ".compozy", "tasks", "demo"),
		)
		if err != nil {
			t.Fatalf("CaptureExcluding baseline: %v", err)
		}

		err = Reset(context.Background(), root, baseline)
		if !errors.Is(err, ErrResetDirtyBaseline) {
			t.Fatalf("Reset error = %v, want ErrResetDirtyBaseline", err)
		}
		if _, statErr := os.Stat(filepath.Join(root, "pre-existing.txt")); statErr != nil {
			t.Fatalf("refused reset destroyed pre-existing work: %v", statErr)
		}
	})

	t.Run("Should reject an unsupported baseline", func(t *testing.T) {
		t.Parallel()

		baseline, err := Capture(context.Background(), t.TempDir())
		if err != nil {
			t.Fatalf("Capture baseline: %v", err)
		}
		err = Reset(context.Background(), t.TempDir(), baseline)
		if !errors.Is(err, ErrResetUnsupported) {
			t.Fatalf("Reset error = %v, want ErrResetUnsupported", err)
		}
	})

	t.Run("Should reject a blank root", func(t *testing.T) {
		t.Parallel()
		requireScopeGit(t)

		root := initScopeGitRepo(t)
		baseline, err := Capture(context.Background(), root)
		if err != nil {
			t.Fatalf("Capture baseline: %v", err)
		}
		if err := Reset(context.Background(), "  ", baseline); !errors.Is(err, ErrResetUnsupported) {
			t.Fatalf("Reset error = %v, want ErrResetUnsupported", err)
		}
	})

	t.Run("Should surface a git failure rather than reporting a clean reset", func(t *testing.T) {
		t.Parallel()
		requireScopeGit(t)

		root := initScopeGitRepo(t)
		baseline, err := Capture(context.Background(), root)
		if err != nil {
			t.Fatalf("Capture baseline: %v", err)
		}
		if err := os.RemoveAll(filepath.Join(root, ".git")); err != nil {
			t.Fatalf("remove .git: %v", err)
		}

		if err := Reset(context.Background(), root, baseline); err == nil {
			t.Fatal("Reset over a destroyed repository must fail")
		}
	})

	t.Run("Should reject a baseline captured from a different root", func(t *testing.T) {
		t.Parallel()
		requireScopeGit(t)

		baseline, err := Capture(context.Background(), initScopeGitRepo(t))
		if err != nil {
			t.Fatalf("Capture baseline: %v", err)
		}
		other := initScopeGitRepo(t)
		err = Reset(context.Background(), other, baseline)
		if !errors.Is(err, ErrResetRootMismatch) {
			t.Fatalf("Reset error = %v, want ErrResetRootMismatch", err)
		}
	})
}
