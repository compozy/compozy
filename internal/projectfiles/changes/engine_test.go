package changes

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/projectfiles"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/worktree"
)

func repo(t *testing.T) (projectfiles.Root, *GitEngine) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := projectfiles.Root{AbsPath: dir}
	git(t, root, "init", "-b", "main")
	write(t, root, "a.go", strings.Repeat("package hello\n", 20))
	write(t, root, "c.go", "old\n")
	git(t, root, "add", ".")
	git(t, root, "commit", "-m", "initial")
	return root, NewEngine()
}
func git(t *testing.T, root projectfiles.Root, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(
		t.Context(),
		"git",
		append([]string{"-c", "user.name=Test", "-c", "user.email=test@example.com"}, args...)...)
	cmd.Dir = root.AbsPath
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
}
func write(t *testing.T, root projectfiles.Root, path, text string) {
	t.Helper()
	full := filepath.Join(root.AbsPath, path)
	if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}
func summary(t *testing.T, e Engine, root projectfiles.Root, req Request) Summary {
	t.Helper()
	s, err := e.Summary(t.Context(), root, req)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestEngine(t *testing.T) {
	t.Run("Should preserve Git failure causes and redact request diagnostics", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		cause := fmt.Errorf("git at %s: %w", root.AbsPath, context.DeadlineExceeded)
		e.runner = failingChangesRunner{StreamingGitRunner: e.runner, cause: cause}
		_, err := e.Summary(t.Context(), root, Request{Scope: Uncommitted})
		if !errors.Is(err, ErrUnavailable) || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("cause lost: %v", err)
		}
		var logs bytes.Buffer
		LogFailure(t.Context(), slog.New(slog.NewTextHandler(&logs, nil)), root.AbsPath, err)
		if strings.Contains(logs.String(), root.AbsPath) || !strings.Contains(logs.String(), "deadline exceeded") ||
			!strings.Contains(logs.String(), "diff") {
			t.Fatalf("logs: %s", &logs)
		}
	})

	t.Parallel()
	t.Run("Should describe branch renames working edits and untracked files", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		clean := summary(t, e, root, Request{Scope: Branch, BaseRef: "main"})
		if len(clean.Files) != 0 || clean.Totals.Files != 0 {
			t.Fatalf("clean: %+v", clean)
		}
		git(t, root, "switch", "-c", "feature")
		git(t, root, "mv", "a.go", "b.go")
		write(t, root, "b.go", strings.Repeat("package hello\n", 20)+"// changed\n")
		git(t, root, "add", ".")
		git(t, root, "commit", "-m", "rename")
		write(t, root, "c.go", "new\n")
		write(t, root, "d.go", "new file\n")
		s := summary(t, e, root, Request{Scope: Branch, BaseRef: "main"})
		if len(s.Files) != 3 || s.Files[0].Path != "b.go" || s.Files[0].OldPath != "a.go" ||
			s.Files[0].Status != "renamed" ||
			s.Totals.Additions != 3 ||
			s.Totals.Deletions != 1 {
			t.Fatalf("summary: %+v", s)
		}
		for _, f := range s.Files {
			target := PatchTarget{f.Path, f.OldPath}
			req := Request{Scope: Branch, BaseRef: "main", Revision: s.Revision}
			patches, err := e.Patches(t.Context(), root, req, []PatchTarget{target})
			if err != nil {
				t.Fatal(err)
			}
			if len(patches) != 1 || !strings.Contains(patches[0].Patch, "diff --git") ||
				patches[0].Fingerprint != f.Fingerprint {
				t.Fatalf("patch: %+v", patches)
			}
			if f.Status == "renamed" && !strings.Contains(patches[0].Patch, "rename from a.go") {
				t.Fatal(patches[0].Patch)
			}
			contents, err := e.Contents(t.Context(), root, req, target)
			if err != nil {
				t.Fatal(err)
			}
			if contents.New == nil || contents.New.Text == "" || (f.Status == "untracked" && contents.Old != nil) {
				t.Fatalf("contents: %+v", contents)
			}
		}
		u := summary(t, e, root, Request{Scope: Uncommitted})
		if len(u.Files) != 2 {
			t.Fatalf("uncommitted: %+v", u)
		}
		git(t, root, "config", "diff.mnemonicPrefix", "true")
		git(t, root, "config", "diff.noprefix", "true")
		git(t, root, "config", "diff.external", "false-tool")
		patches, err := e.Patches(
			t.Context(),
			root,
			Request{Scope: Uncommitted, Revision: u.Revision},
			[]PatchTarget{{Path: "c.go"}},
		)
		if err != nil || !strings.HasPrefix(patches[0].Patch, "diff --git a/c.go b/c.go") {
			t.Fatalf("patches %v: %+v", err, patches)
		}
	})
	t.Run("Should fence individual edits and moved HEAD revisions", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		write(t, root, "c.go", "one\n")
		s := summary(t, e, root, Request{Scope: Uncommitted})
		again := summary(t, e, root, Request{Scope: Uncommitted})
		if fmt.Sprint(s) != fmt.Sprint(again) {
			t.Fatal("unstable summary")
		}
		info, err := os.Stat(filepath.Join(root.AbsPath, "c.go"))
		if err != nil {
			t.Fatal(err)
		}
		write(t, root, "c.go", "two\n")
		if err := os.Chtimes(filepath.Join(root.AbsPath, "c.go"), info.ModTime(), info.ModTime()); err != nil {
			t.Fatal(err)
		}
		patches, err := e.Patches(
			t.Context(),
			root,
			Request{Scope: Uncommitted, Revision: s.Revision},
			[]PatchTarget{{Path: "c.go"}},
		)
		if err != nil {
			t.Fatal(err)
		}
		if patches[0].Fingerprint == s.Files[0].Fingerprint {
			t.Fatal("edit missed")
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "-m", "move head")
		if _, err := e.Patches(
			t.Context(),
			root,
			Request{Scope: Uncommitted, Revision: s.Revision},
			[]PatchTarget{{Path: "c.go"}},
		); !errors.Is(
			err,
			ErrStale,
		) {
			t.Fatalf("stale: %v", err)
		}
	})
	t.Run("Should cap each patch independently without the runner buffer limit", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		targets := []PatchTarget{{Path: "one"}, {Path: "two"}, {Path: "three"}}
		for _, target := range targets {
			write(t, root, target.Path, "before\n")
		}
		git(t, root, "add", ".")
		git(t, root, "commit", "-m", "large fixture")
		for _, target := range targets {
			write(t, root, target.Path, strings.Repeat("line\n", 110000))
		}
		s := summary(t, e, root, Request{Scope: Uncommitted})
		req := Request{Scope: Uncommitted, Revision: s.Revision}
		patches, err := e.Patches(t.Context(), root, req, targets)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range patches {
			if p.TooLarge || len(p.Patch) < 600000 {
				t.Fatalf("truncated: %+v", p)
			}
		}
		write(t, root, "two", strings.Repeat("line\n", 300000))
		patches, err = e.Patches(t.Context(), root, req, targets)
		if err != nil {
			t.Fatal(err)
		}
		if patches[0].TooLarge || !patches[1].TooLarge || patches[1].Patch != "" || patches[2].TooLarge {
			t.Fatal("per-file cap failed")
		}
		if _, err := e.Contents(t.Context(), root, req, targets[1]); !errors.Is(err, ErrTooLargeToExpand) {
			t.Fatalf("contents cap: %v", err)
		}
	})
	t.Run("Should gate sensitive sides and classify binary generated and deleted files", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		write(t, root, ".env", "old secret")
		write(t, root, "binary", "a\x00b")
		write(t, root, "custom", "old")
		git(t, root, "add", ".")
		git(t, root, "commit", "-m", "sensitive fixture")
		write(t, root, ".env", "new secret")
		write(t, root, "binary", "c\x00d")
		write(t, root, "custom", "new")
		write(t, root, ".gitattributes", "custom linguist-generated\n")
		write(t, root, "bun.lock", "lock")
		write(t, root, "web/src/routeTree.gen.ts", "generated")
		if err := os.Remove(filepath.Join(root.AbsPath, "c.go")); err != nil {
			t.Fatal(err)
		}
		s := summary(t, e, root, Request{Scope: Uncommitted})
		req := Request{Scope: Uncommitted, Revision: s.Revision}
		for _, f := range s.Files {
			switch f.Path {
			case "binary":
				if !f.Binary {
					t.Fatal("binary missed")
				}
			case "custom", "bun.lock", "web/src/routeTree.gen.ts":
				if !f.Generated {
					t.Fatalf("generated missed: %s", f.Path)
				}
			case "c.go":
				if f.Status != "deleted" {
					t.Fatal("delete missed")
				}
			}
		}
		patches, err := e.Patches(t.Context(), root, req, []PatchTarget{{Path: ".env"}})
		if err != nil || !patches[0].Sensitive || patches[0].Patch != "" {
			t.Fatalf("sensitive: %v %+v", err, patches)
		}
		contents, err := e.Contents(t.Context(), root, req, PatchTarget{Path: ".env"})
		if err != nil || !contents.Old.Sensitive || !contents.New.Sensitive {
			t.Fatalf("gate: %v %+v", err, contents)
		}
		req.Reveal = true
		contents, err = e.Contents(t.Context(), root, req, PatchTarget{Path: ".env"})
		if err != nil || contents.Old.Text != "old secret" || contents.New.Text != "new secret" {
			t.Fatalf("reveal: %v %+v", err, contents)
		}
		contents, err = e.Contents(t.Context(), root, req, PatchTarget{Path: "c.go"})
		if err != nil || contents.New != nil || contents.Old.Text != "old\n" {
			t.Fatalf("delete: %v %+v", err, contents)
		}
	})
	t.Run("Should reject unsafe inputs and enumerate bases", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		for _, ref := range []string{"--output=/tmp/x", "a..b", "HEAD@{1}", "origin/nope"} {
			if _, err := e.Summary(
				t.Context(),
				root,
				Request{Scope: Branch, BaseRef: ref},
			); !errors.Is(
				err,
				ErrBaseNotFound,
			) {
				t.Fatalf("base %s: %v", ref, err)
			}
		}
		for _, path := range []string{"../x", "/etc/passwd", ".git/config"} {
			if _, err := e.Patches(
				t.Context(),
				root,
				Request{},
				[]PatchTarget{{Path: path}},
			); !errors.Is(
				err,
				projectfiles.ErrInvalidPath,
			) {
				t.Fatalf("path %s: %v", path, err)
			}
		}
		if _, err := e.Patches(
			t.Context(),
			root,
			Request{},
			make([]PatchTarget, 21),
		); !errors.Is(
			err,
			ErrTooManyPaths,
		) {
			t.Fatal(err)
		}
		git(t, root, "branch", "release-1")
		git(t, root, "update-ref", "refs/remotes/origin/release-1", "HEAD")
		bases, err := e.Bases(t.Context(), root, "rel", 200)
		if err != nil || len(bases.Refs) != 3 || !bases.Resolvable {
			t.Fatalf("bases: %v %+v", err, bases)
		}
		git(t, root, "switch", "--detach")
		s := summary(t, e, root, Request{})
		if s.HeadBranch != "" || s.HeadSHA == "" {
			t.Fatal("detached head")
		}
		root.AbsPath = t.TempDir()
		if _, err := e.Summary(t.Context(), root, Request{}); !errors.Is(err, ErrNotARepository) {
			t.Fatal(err)
		}
	})
	t.Run("Should confine changes to a subdirectory root", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		write(t, root, "sub/x.go", "old\n")
		write(t, root, "other/y.go", "moved\n")
		git(t, root, "add", ".")
		git(t, root, "commit", "-m", "subtree fixture")
		git(t, root, "branch", "base")
		git(t, root, "mv", "other/y.go", "sub/y.go")
		git(t, root, "commit", "-m", "move into subtree")
		write(t, root, "sub/x.go", "new\n")
		write(t, root, "top.txt", "outside\n")
		root.AbsPath = filepath.Join(root.AbsPath, "sub")
		s := summary(t, e, root, Request{Scope: Branch, BaseRef: "base"})
		if len(s.Files) != 2 || s.Files[0].Path != "x.go" || s.Files[1].Path != "y.go" || s.Files[1].Status != "added" {
			t.Fatalf("subtree: %+v", s)
		}
		c, err := e.Contents(
			t.Context(),
			root,
			Request{Scope: Branch, BaseRef: "base", Revision: s.Revision},
			PatchTarget{Path: "y.go"},
		)
		if err != nil || c.Old != nil || c.New.Text != "moved\n" {
			t.Fatalf("subtree contents: %v %+v", err, c)
		}
	})
	t.Run("Should never execute configured git programs or mutate repository state", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		markers := testutil.ConfigureGitReadMarkers(t, root.AbsPath)
		write(t, root, "c.go", "new\n")
		s := summary(t, e, root, Request{Scope: Uncommitted})
		req := Request{Scope: Uncommitted, Revision: s.Revision}
		if _, err := e.Patches(t.Context(), root, req, []PatchTarget{{Path: "c.go"}}); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Contents(t.Context(), root, req, PatchTarget{Path: "c.go"}); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Bases(t.Context(), root, "", 200); err != nil {
			t.Fatal(err)
		}
		for _, marker := range []string{markers} {
			if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("marker executed: %s %v", marker, err)
			}
		}
	})
}

func TestEngineLimitsAndRaces(t *testing.T) {
	t.Parallel()
	t.Run("Should cap summary rows while preserving totals", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		for i := range 3001 {
			write(t, root, fmt.Sprintf("file-%04d", i), "line\n")
		}
		s := summary(t, e, root, Request{Scope: Uncommitted})
		if len(s.Files) != 3000 || !s.Truncated || s.Totals.Files != 3001 || s.Totals.Additions != 3001 {
			t.Fatalf("rows=%d totals=%+v truncated=%v", len(s.Files), s.Totals, s.Truncated)
		}
	})
	t.Run("Should detect edits during patch generation", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		write(t, root, "c.go", "one\n")
		s := summary(t, e, root, Request{Scope: Uncommitted})
		originalRunner := e.runner
		e.runner = &mutatingRunner{
			StreamingGitRunner: originalRunner,
			mutate:             func() { write(t, root, "c.go", "two\n") },
		}
		if _, err := e.Patches(
			t.Context(),
			root,
			Request{Scope: Uncommitted, Revision: s.Revision},
			[]PatchTarget{{Path: "c.go"}},
		); !errors.Is(
			err,
			ErrStale,
		) {
			t.Fatalf("race: %v", err)
		}
	})
	t.Run("Should follow target identity and sensitivity for symlinks", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		write(t, root, "target", "one\n")
		write(t, root, ".env", "secret")
		if err := os.Symlink("target", filepath.Join(root.AbsPath, "link")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(".env", filepath.Join(root.AbsPath, "alias")); err != nil {
			t.Fatal(err)
		}
		s := summary(t, e, root, Request{Scope: Uncommitted})
		req := Request{Scope: Uncommitted, Revision: s.Revision}
		before, err := e.fingerprint(t.Context(), root, s.MergeBaseSHA, PatchTarget{Path: "link"})
		if err != nil {
			t.Fatal(err)
		}
		write(t, root, "target", "two\n")
		after, err := e.fingerprint(t.Context(), root, s.MergeBaseSHA, PatchTarget{Path: "link"})
		if err != nil || before == after {
			t.Fatalf("symlink fingerprint %v", err)
		}
		patches, err := e.Patches(t.Context(), root, req, []PatchTarget{{Path: "alias"}})
		if err != nil || !patches[0].Sensitive {
			t.Fatalf("resolved sensitivity: %v %+v", err, patches)
		}
	})
	t.Run("Should classify conflicts within a subtree", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		write(t, root, "sub/c.go", "base\n")
		git(t, root, "add", ".")
		git(t, root, "commit", "-m", "conflict base")
		git(t, root, "switch", "-c", "other")
		write(t, root, "sub/c.go", "other\n")
		git(t, root, "commit", "-am", "other")
		git(t, root, "switch", "main")
		write(t, root, "sub/c.go", "main\n")
		git(t, root, "commit", "-am", "main")
		cmd := exec.CommandContext(t.Context(), "git", "merge", "other")
		cmd.Dir = root.AbsPath
		if _, err := cmd.CombinedOutput(); err == nil {
			t.Fatal("expected merge conflict")
		}
		root.AbsPath = filepath.Join(root.AbsPath, "sub")
		s := summary(t, e, root, Request{Scope: Uncommitted})
		if len(s.Files) != 1 || s.Files[0].Path != "c.go" || s.Files[0].Status != "conflicted" {
			t.Fatalf("conflict: %+v", s)
		}
		p, err := e.Patches(
			t.Context(),
			root,
			Request{Scope: Uncommitted, Revision: s.Revision},
			[]PatchTarget{{Path: "c.go"}},
		)
		if err != nil || len(p) != 1 || p[0].Fingerprint != s.Files[0].Fingerprint {
			t.Fatalf("conflicted patch: %v %+v", err, p)
		}
	})

	t.Run("Should compute fingerprints only for requested targets", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		for _, path := range []string{"c.go", "one", "two"} {
			write(t, root, path, "changed\n")
		}
		s := summary(t, e, root, Request{Scope: Uncommitted})
		reader := &recordingReader{Reader: e.reader}
		e.reader = reader
		if _, err := e.Patches(
			t.Context(),
			root,
			Request{Scope: Uncommitted, Revision: s.Revision},
			[]PatchTarget{{Path: "c.go"}},
		); err != nil {
			t.Fatal(err)
		}
		if len(reader.paths) == 0 {
			t.Fatal("no file identity reads")
		}
		for _, path := range reader.paths {
			if path != "c.go" {
				t.Fatalf("unrequested identity read: %s", path)
			}
		}
	})

	t.Run("Should resolve worktree remote and fallback bases", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		git(t, root, "update-ref", "refs/remotes/origin/develop", "HEAD")
		git(t, root, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/develop")
		s := summary(t, e, root, Request{Scope: Branch})
		if s.BaseRef != "origin/develop" || s.BaseSource != "default_branch" {
			t.Fatalf("default: %+v", s)
		}
		root.BaseRef = "main"
		s = summary(t, e, root, Request{Scope: Branch})
		if s.BaseSource != "worktree" || s.BaseRef != "main" {
			t.Fatalf("worktree: %+v", s)
		}
	})
	t.Run("Should discover drivers added between reads and preserve git state", func(t *testing.T) {
		t.Parallel()
		root, e := repo(t)
		marker := testutil.ConfigureGitReadMarkers(t, root.AbsPath)
		indexPath := filepath.Join(root.AbsPath, ".git", "index")
		configPath := filepath.Join(root.AbsPath, ".git", "config")
		beforeIndex, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		indexInfo, err := os.Stat(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		summary(t, e, root, Request{Scope: Uncommitted})
		git(t, root, "config", "filter.late.clean", "touch "+marker)
		write(t, root, ".gitattributes", "c.go filter=late\n")
		write(t, root, "c.go", "changed\n")
		beforeConfig, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		s := summary(t, e, root, Request{Scope: Uncommitted})
		req := Request{Scope: Uncommitted, Revision: s.Revision}
		if _, err := e.Patches(t.Context(), root, req, []PatchTarget{{Path: "c.go"}}); err != nil {
			t.Fatal(err)
		}
		if _, err := e.Contents(t.Context(), root, req, PatchTarget{Path: "c.go"}); err != nil {
			t.Fatal(err)
		}
		afterIndex, err := os.ReadFile(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		afterInfo, err := os.Stat(indexPath)
		if err != nil {
			t.Fatal(err)
		}
		afterConfig, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(beforeIndex, afterIndex) || !indexInfo.ModTime().Equal(afterInfo.ModTime()) ||
			!bytes.Equal(beforeConfig, afterConfig) {
			t.Fatal("git state mutated")
		}
		if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
			t.Fatal("late filter ran")
		}
	})
}

type mutatingRunner struct {
	worktree.StreamingGitRunner
	mutate func()
}

var _ worktree.StreamingGitRunner = (*mutatingRunner)(nil)

func (r *mutatingRunner) RunStreaming(
	ctx context.Context,
	dir string,
	fn worktree.GitOutputHandler,
	args ...string,
) ([]byte, []byte, error) {
	out, stderr, err := r.StreamingGitRunner.RunStreaming(ctx, dir, fn, args...)
	for _, arg := range args {
		if arg == "-U3" && r.mutate != nil {
			r.mutate()
			r.mutate = nil
			break
		}
	}
	return out, stderr, err
}

type recordingReader struct {
	projectfiles.Reader
	paths []string
}

var _ projectfiles.Reader = (*recordingReader)(nil)

func (r *recordingReader) Read(
	ctx context.Context,
	root projectfiles.Root,
	path string,
	opts projectfiles.ReadOptions,
) (projectfiles.Content, error) {
	r.paths = append(r.paths, path)
	return r.Reader.Read(ctx, root, path, opts)
}

type failingChangesRunner struct {
	worktree.StreamingGitRunner
	cause error
}

func (r failingChangesRunner) RunStreaming(
	ctx context.Context,
	root string,
	visit worktree.GitOutputHandler,
	args ...string,
) ([]byte, []byte, error) {
	if slices.Contains(args, "diff") {
		return nil, nil, r.cause
	}
	return r.StreamingGitRunner.RunStreaming(ctx, root, visit, args...)
}
