package workspacefiles

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/worktree"
)

func TestSnapshotRank(t *testing.T) {
	t.Parallel()

	t.Run("Should rank a basename match above a path-only match", func(t *testing.T) {
		t.Parallel()
		snap := newSnapshot([]string{"docs/client_prompt_notes/x.md", "internal/acp/client_prompt.go"}, false)
		got := entryPaths(snap.rank("client_prompt", 10))
		fileIndex := slices.Index(got, "internal/acp/client_prompt.go")
		noteIndex := slices.Index(got, "docs/client_prompt_notes/x.md")
		if fileIndex < 0 || noteIndex < 0 || fileIndex > noteIndex {
			t.Fatalf("rank(client_prompt) = %v, want the basename match before the path match", got)
		}
	})

	t.Run("Should order matches in one tier by depth then length then path", func(t *testing.T) {
		t.Parallel()
		snap := newSnapshot([]string{"a/b/main.go", "z/main.go", "main.go", "x/mainly.go", "y/main.go"}, false)
		got := entryPaths(snap.rank("main.go", 10))
		want := []string{"main.go", "y/main.go", "z/main.go", "a/b/main.go"}
		if !slices.Equal(got[:4], want) {
			t.Fatalf("rank(main.go) = %v, want prefix %v", got, want)
		}
	})

	t.Run("Should apply the tier order exact prefix substring subsequence path", func(t *testing.T) {
		t.Parallel()
		snap := newSnapshot([]string{
			"deep/nested/dir/abc",
			"q/abcd",
			"q/xabcx",
			"q/axbxc",
			"abc_dir/zzz",
			"a_dir/b_dir/c_dir/zzz",
		}, false)
		got := entryPaths(snap.rank("abc", 20))
		want := []string{
			"deep/nested/dir/abc",
			"abc_dir/",
			"q/abcd",
			"q/xabcx",
			"q/axbxc",
			"abc_dir/zzz",
			"a_dir/b_dir/c_dir/",
			"a_dir/b_dir/c_dir/zzz",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("rank(abc) = %v, want %v", got, want)
		}
	})

	t.Run("Should list the shallowest entries lexically for an empty query", func(t *testing.T) {
		t.Parallel()
		snap := newSnapshot([]string{"web/src/app.ts", "b.go", "a.go"}, false)
		got := entryPaths(snap.rank("", 10))
		want := []string{"a.go", "b.go", "web/", "web/src/", "web/src/app.ts"}
		if !slices.Equal(got, want) {
			t.Fatalf("rank(\"\") = %v, want %v", got, want)
		}
	})

	t.Run("Should return derived folders with the folder kind", func(t *testing.T) {
		t.Parallel()
		snap := newSnapshot([]string{"web/src/app.ts"}, false)
		entries := snap.rank("src", 10)
		if len(entries) == 0 || entries[0] != (Entry{Path: "web/src/", Name: "src", Kind: KindFolder}) {
			t.Fatalf("rank(src) = %#v, want web/src/ folder first", entries)
		}
	})

	t.Run("Should return exactly limit entries", func(t *testing.T) {
		t.Parallel()
		snap := newSnapshot([]string{"a.go", "b.go", "c.go", "d.go", "e.go"}, false)
		if got := snap.rank("", 3); len(got) != 3 {
			t.Fatalf("rank limit 3 returned %d entries", len(got))
		}
	})

	t.Run("Should match case-insensitively with backslash separators", func(t *testing.T) {
		t.Parallel()
		snap := newSnapshot([]string{"Internal/ACP/Client.go"}, false)
		got := entryPaths(snap.rank(`acp\client`, 10))
		if !slices.Contains(got, "Internal/ACP/Client.go") {
			t.Fatalf("rank(acp\\client) = %v, want the file", got)
		}
	})
}

func TestIndexSearchGitRepository(t *testing.T) {
	t.Parallel()

	t.Run("Should list tracked and untracked files without ignored or deleted ones", func(t *testing.T) {
		t.Parallel()
		runner := realGitRunner(t)
		root := initGitRepo(t, runner, map[string]string{
			".gitignore":     "ignored.log\n",
			"kept.go":        "package kept\n",
			"removed.go":     "package removed\n",
			"pkg/nested.go":  "package pkg\n",
			".github/ci.yml": "on: push\n",
		})
		writeFiles(t, root, map[string]string{"untracked.go": "package u\n", "ignored.log": "noise\n"})
		if err := os.Remove(filepath.Join(root, "removed.go")); err != nil {
			t.Fatalf("remove tracked file: %v", err)
		}

		result, err := New(runner).Search(t.Context(), root, "", 100)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		got := entryPaths(result.Entries)
		for _, want := range []string{"kept.go", "untracked.go", "pkg/", "pkg/nested.go", ".github/ci.yml"} {
			if !slices.Contains(got, want) {
				t.Fatalf("Search() = %v, missing %q", got, want)
			}
		}
		for _, absent := range []string{"ignored.log", "removed.go"} {
			if slices.Contains(got, absent) {
				t.Fatalf("Search() = %v, want %q absent", got, absent)
			}
		}
		if result.Truncated {
			t.Fatal("Search() Truncated = true, want false")
		}
		if _, err := os.Stat(filepath.Join(root, ".git", "index.lock")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("index.lock stat error = %v, want not exist", err)
		}
	})

	t.Run("Should list paths relative to a repository subdirectory", func(t *testing.T) {
		t.Parallel()
		runner := realGitRunner(t)
		root := initGitRepo(t, runner, map[string]string{"top.go": "x\n", "pkg/main.go": "x\n"})

		result, err := New(runner).Search(t.Context(), filepath.Join(root, "pkg"), "", 10)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if got := entryPaths(result.Entries); !slices.Equal(got, []string{"main.go"}) {
			t.Fatalf("Search(pkg) = %v, want [main.go]", got)
		}
	})
}

func TestIndexSearchWalkFallback(t *testing.T) {
	t.Parallel()

	t.Run("Should walk a non-git directory skipping git and node_modules", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFiles(t, root, map[string]string{
			"main.go":                  "x",
			".git/config":              "x",
			"node_modules/pkg/i.js":    "x",
			"web/node_modules/b/i.js":  "x",
			".github/workflows/ci.yml": "x",
			"bad\"name.txt":            "x",
		})

		result, err := New(realGitRunner(t)).Search(t.Context(), root, "", 100)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		got := entryPaths(result.Entries)
		want := []string{"main.go", ".github/", ".github/workflows/", ".github/workflows/ci.yml"}
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Fatalf("Search() = %v, want %v", got, want)
		}
	})

	t.Run("Should cap the index and report truncation", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFiles(t, root, map[string]string{"a.go": "x", "b.go": "x", "c.go": "x", "d.go": "x"})

		result, err := New(nil, WithMaxFiles(3)).Search(t.Context(), root, "", 100)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(result.Entries) != 3 || !result.Truncated {
			t.Fatalf("Search() = %d entries truncated=%v, want 3 truncated", len(result.Entries), result.Truncated)
		}
	})

	t.Run("Should fall back to the walk when git fails", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFiles(t, root, map[string]string{"walked.go": "x"})
		runner := &fakeGitRunner{err: errors.New("git exploded")}

		result, err := New(runner).Search(t.Context(), root, "", 10)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if got := entryPaths(result.Entries); !slices.Equal(got, []string{"walked.go"}) {
			t.Fatalf("Search() = %v, want [walked.go]", got)
		}
	})

	t.Run("Should fall back to the walk when git exceeds its deadline", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFiles(t, root, map[string]string{"walked.go": "x"})
		runner := &fakeGitRunner{blockUntilDone: true}

		result, err := New(runner, WithGitTimeout(20*time.Millisecond)).Search(t.Context(), root, "", 10)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if got := entryPaths(result.Entries); !slices.Equal(got, []string{"walked.go"}) {
			t.Fatalf("Search() = %v, want [walked.go]", got)
		}
	})

	t.Run("Should fail when the root does not exist", func(t *testing.T) {
		t.Parallel()
		missing := filepath.Join(t.TempDir(), "gone")
		if _, err := New(nil).Search(t.Context(), missing, "", 10); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Search(missing) error = %v, want os.ErrNotExist", err)
		}
	})
}

func TestIndexSearchCache(t *testing.T) {
	t.Parallel()

	t.Run("Should share one build across concurrent searches on a cold root", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		release := make(chan struct{})
		runner := &fakeGitRunner{files: []string{"a.go"}, release: release}
		idx := New(runner)

		var wg sync.WaitGroup
		errs := make(chan error, 10)
		for range 10 {
			wg.Go(func() {
				_, err := idx.Search(t.Context(), root, "a", 10)
				errs <- err
			})
		}
		waitFor(t, func() bool { return runner.listCalls.Load() == 1 })
		close(release)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
		}
		if calls := runner.listCalls.Load(); calls != 1 {
			t.Fatalf("git ls-files builds = %d, want 1", calls)
		}
	})

	t.Run("Should reuse the index inside the TTL and rebuild after it", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		runner := &fakeGitRunner{files: []string{"a.go"}}
		clock := &fakeClock{now: time.Unix(1_700_000_000, 0)}
		idx := New(runner, WithClock(clock.Now))

		search := func() {
			t.Helper()
			if _, err := idx.Search(t.Context(), root, "", 10); err != nil {
				t.Fatalf("Search() error = %v", err)
			}
		}
		search()
		clock.Advance(14 * time.Second)
		search()
		if calls := runner.listCalls.Load(); calls != 1 {
			t.Fatalf("builds before TTL = %d, want 1", calls)
		}
		clock.Advance(2 * time.Second)
		search()
		if calls := runner.listCalls.Load(); calls != 2 {
			t.Fatalf("builds after TTL = %d, want 2", calls)
		}
	})
}

type fakeGitRunner struct {
	files          []string
	err            error
	blockUntilDone bool
	release        <-chan struct{}
	listCalls      atomic.Int32
}

func (f *fakeGitRunner) Run(ctx context.Context, _ string, args ...string) ([]byte, []byte, error) {
	if slices.Contains(args, "--deleted") {
		return nil, nil, nil
	}
	f.listCalls.Add(1)
	if f.release != nil {
		<-f.release
	}
	if f.blockUntilDone {
		<-ctx.Done()
		return nil, nil, ctx.Err()
	}
	if f.err != nil {
		return nil, []byte("fatal: boom"), f.err
	}
	var out strings.Builder
	for _, file := range f.files {
		out.WriteString(file)
		out.WriteByte(0)
	}
	return []byte(out.String()), nil, nil
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func entryPaths(entries []Entry) []string {
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	return paths
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached before deadline")
		}
		time.Sleep(time.Millisecond)
	}
}

func realGitRunner(t *testing.T) *worktree.RealGitRunner {
	t.Helper()
	runner, err := worktree.NewRealGitRunner(DefaultGitTimeout)
	if err != nil {
		t.Skipf("git unavailable: %v", err)
	}
	return runner
}

func initGitRepo(t *testing.T, runner GitRunner, files map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("resolve temp dir: %v", err)
	}
	writeFiles(t, root, files)
	for _, args := range [][]string{
		{"init", "-q"},
		{"add", "-A"},
		{"-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-q", "-m", "init"},
	} {
		if _, stderr, err := runner.Run(t.Context(), root, args...); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, stderr)
		}
	}
	return root
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
}
