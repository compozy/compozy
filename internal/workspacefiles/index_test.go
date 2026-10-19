package workspacefiles

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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

	t.Run("Should log the index build with its counts and source but no root path", func(t *testing.T) {
		t.Parallel()
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatalf("resolve temp dir: %v", err)
		}
		logs := &lockedBuffer{}
		logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
		runner := &fakeGitRunner{files: []string{"a.go", "pkg/b.go"}}
		createListedFiles(t, root, runner.files)

		result, err := New(runner, WithLogger(logger)).Search(t.Context(), root, "", 100)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		record := requireLogRecord(t, logs.String(), "workspacefiles.index_built")
		want := map[string]any{
			"level":     "DEBUG",
			"root_hash": rootHash(root),
			"entries":   float64(len(result.Entries)),
			"truncated": false,
			"source":    sourceGit,
		}
		for key, value := range want {
			if record[key] != value {
				t.Fatalf("index_built %s = %#v, want %#v in %v", key, record[key], value, record)
			}
		}
		if _, ok := record["duration_ms"].(float64); !ok || len(result.Entries) != 3 {
			t.Fatalf("index_built = %v for %d entries, want duration_ms over 3 entries", record, len(result.Entries))
		}
		if strings.Contains(logs.String(), root) {
			t.Fatalf("logs = %s, want no root path", logs.String())
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

	t.Run("Should classify listed symlinks like the walk", func(t *testing.T) {
		t.Parallel()
		runner := realGitRunner(t)
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatalf("resolve temp dir: %v", err)
		}
		outside := t.TempDir()
		writeFiles(t, root, map[string]string{"pkg/main.go": "x", "notes.md": "x"})
		writeFiles(t, outside, map[string]string{"secret.txt": "x"})
		linkSymlinks(t, root, map[string]string{
			"pkg-link":    "pkg",
			"notes-link":  "notes.md",
			"escape-dir":  outside,
			"escape-file": filepath.Join(outside, "secret.txt"),
			"broken":      "missing.go",
		})
		for _, args := range [][]string{
			{"init", "-q"},
			{"add", "-A"},
			{"-c", "user.email=test@example.com", "-c", "user.name=Test", "commit", "-q", "-m", "init"},
		} {
			if _, stderr, err := runner.Run(t.Context(), root, args...); err != nil {
				t.Fatalf("git %v: %v: %s", args, err, stderr)
			}
		}

		result, err := New(runner).Search(t.Context(), root, "", 100)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		assertSymlinkEntries(t, result.Entries)
	})
}

// linkSymlinks creates root/name -> target for each entry, skipping where symlinks are unsupported.
func linkSymlinks(t *testing.T, root string, links map[string]string) {
	t.Helper()
	for name, target := range links {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
	}
}

// assertSymlinkEntries checks the fixture of linkSymlinks: an inner directory link is a folder,
// an inner file link is a file, and escaping or broken links are absent.
func assertSymlinkEntries(t *testing.T, entries []Entry) {
	t.Helper()
	kinds := make(map[string]EntryKind, len(entries))
	for _, entry := range entries {
		kinds[entry.Path] = entry.Kind
	}
	if kinds["pkg-link/"] != KindFolder || kinds["notes-link"] != KindFile {
		t.Fatalf("entries = %#v, want pkg-link/ folder and notes-link file", entries)
	}
	for _, absent := range []string{"pkg-link", "escape-dir", "escape-dir/", "escape-file", "broken"} {
		if _, ok := kinds[absent]; ok {
			t.Fatalf("entries = %#v, want %q absent", entries, absent)
		}
	}
}

// requireLogRecord decodes JSON log lines and returns the first record with msg.
func requireLogRecord(t *testing.T, output string, msg string) map[string]any {
	t.Helper()
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("decode log line %q: %v", line, err)
		}
		if record["msg"] == msg {
			return record
		}
	}
	t.Fatalf("logs = %s, want a %s record", output, msg)
	return nil
}

// lockedBuffer is a concurrency-safe log sink.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
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

	t.Run("Should stop the walk at the visited-entry cap counting directories", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		for i := range 30 {
			if err := os.Mkdir(filepath.Join(root, fmt.Sprintf("d%02d", i)), 0o755); err != nil {
				t.Fatalf("mkdir: %v", err)
			}
		}
		writeFiles(t, root, map[string]string{"z.go": "x"})

		result, err := New(nil, WithMaxWalkEntries(10)).Search(t.Context(), root, "", 100)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(result.Entries) != 0 || !result.Truncated {
			t.Fatalf("Search() = %v truncated=%v, want no entries and truncated after 10 directories",
				entryPaths(result.Entries), result.Truncated)
		}
	})

	t.Run("Should end the shared build at its deadline with a truncated index", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeFiles(t, root, map[string]string{"a.go": "x"})
		runner := &fakeGitRunner{blockUntilDone: true}
		idx := New(runner, WithGitTimeout(time.Hour), WithBuildTimeout(50*time.Millisecond))

		started := time.Now()
		result, err := idx.Search(t.Context(), root, "", 10)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if !result.Truncated || len(result.Entries) != 0 {
			t.Fatalf("Search() = %v truncated=%v, want an empty truncated index", entryPaths(result.Entries),
				result.Truncated)
		}
		if elapsed := time.Since(started); elapsed > 5*time.Second {
			t.Fatalf("Search() took %v, want it bounded by the build deadline", elapsed)
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

	t.Run("Should cap the index at the default 25,000 files", func(t *testing.T) {
		t.Parallel()
		files := make([]string, 0, DefaultMaxFiles+1)
		for i := range DefaultMaxFiles + 1 {
			files = append(files, fmt.Sprintf("f%05d.go", i))
		}
		runner := &fakeGitRunner{files: files}

		idx := New(runner)
		root := t.TempDir()
		createListedFiles(t, root, files)
		result, err := idx.Search(t.Context(), root, "", DefaultMaxFiles+10)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(result.Entries) != DefaultMaxFiles || !result.Truncated {
			t.Fatalf(
				"Search() = %d entries truncated=%v, want %d truncated",
				len(result.Entries), result.Truncated, DefaultMaxFiles,
			)
		}
		last := files[DefaultMaxFiles]
		if slices.Contains(entryPaths(result.Entries), last) {
			t.Fatalf("Search() indexed %q past the cap", last)
		}
	})

	for _, test := range []struct {
		name   string
		runner *fakeGitRunner
		opts   []Option
	}{
		{name: "fails", runner: &fakeGitRunner{err: errors.New("git exploded")}},
		{
			name:   "exceeds its deadline",
			runner: &fakeGitRunner{blockUntilDone: true},
			opts:   []Option{WithGitTimeout(20 * time.Millisecond)},
		},
	} {
		t.Run("Should fall back to the walk inside a repository when git "+test.name, func(t *testing.T) {
			t.Parallel()
			root := initGitRepo(t, realGitRunner(t), map[string]string{"walked.go": "x"})
			logs := &lockedBuffer{}
			logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

			opts := append([]Option{WithLogger(logger)}, test.opts...)
			result, err := New(test.runner, opts...).Search(t.Context(), root, "", 10)
			if err != nil {
				t.Fatalf("Search() error = %v", err)
			}
			if got := entryPaths(result.Entries); !slices.Equal(got, []string{"walked.go"}) {
				t.Fatalf("Search() = %v, want [walked.go] without .git", got)
			}
			output := logs.String()
			if !strings.Contains(output, `"msg":"workspacefiles.git_fallback"`) ||
				!strings.Contains(output, `"level":"WARN"`) {
				t.Fatalf("logs = %s, want a workspacefiles.git_fallback warning", output)
			}
			if !strings.Contains(output, `"source":"walk"`) {
				t.Fatalf("logs = %s, want the index built from the walk", output)
			}
			if strings.Contains(output, root) {
				t.Fatalf("logs = %s, want no root path", output)
			}
		})
	}

	t.Run("Should classify walked symlinks by their target inside the root", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		outside := t.TempDir()
		writeFiles(t, root, map[string]string{"pkg/main.go": "x", "notes.md": "x"})
		writeFiles(t, outside, map[string]string{"secret.txt": "x"})
		linkSymlinks(t, root, map[string]string{
			"pkg-link":    "pkg",
			"notes-link":  "notes.md",
			"escape-dir":  outside,
			"escape-file": filepath.Join(outside, "secret.txt"),
			"broken":      "missing.go",
		})

		result, err := New(nil).Search(t.Context(), root, "", 100)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		assertSymlinkEntries(t, result.Entries)
	})

	t.Run("Should fail when the root does not exist", func(t *testing.T) {
		t.Parallel()
		missing := filepath.Join(t.TempDir(), "gone")
		if _, err := New(nil).Search(t.Context(), missing, "", 10); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Search(missing) error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("Should log index_failed when both git and the walk fail", func(t *testing.T) {
		t.Parallel()
		if os.Geteuid() == 0 {
			t.Skip("root reads unreadable directories")
		}
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatalf("resolve temp dir: %v", err)
		}
		if err := os.Chmod(root, 0o000); err != nil {
			t.Fatalf("chmod root: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(root, 0o700) })
		logs := &lockedBuffer{}
		logger := slog.New(slog.NewJSONHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))

		runner := &fakeGitRunner{err: errors.New("git exploded")}
		if _, err := New(runner, WithLogger(logger)).Search(t.Context(), root, "", 10); err == nil {
			t.Fatal("Search(unreadable root) error = nil, want the index failure")
		}
		record := requireLogRecord(t, logs.String(), "workspacefiles.index_failed")
		if record["level"] != "ERROR" || record["root_hash"] != rootHash(root) {
			t.Fatalf("index_failed = %v, want an ERROR record with root_hash %s", record, rootHash(root))
		}
		if message, ok := record["error"].(string); !ok || !strings.Contains(message, "git exploded") {
			t.Fatalf("index_failed error = %#v, want both source failures", record["error"])
		}
		if message, _ := record["error"].(string); strings.Contains(logs.String(), root) ||
			!strings.Contains(message, "<root>") {
			t.Fatalf("logs = %s, want the walk error with the root path redacted", logs.String())
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

	t.Run("Should let a canceled first caller stop waiting while the build serves others", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		createListedFiles(t, root, []string{"a.go"})
		release := make(chan struct{})
		runner := &fakeGitRunner{files: []string{"a.go"}, release: release}
		idx := New(runner)

		firstCtx, cancelFirst := context.WithCancel(t.Context())
		firstErr := make(chan error, 1)
		go func() {
			_, err := idx.Search(firstCtx, root, "a", 10)
			firstErr <- err
		}()
		waitFor(t, func() bool { return runner.listCalls.Load() == 1 })
		second := make(chan Result, 1)
		go func() {
			result, err := idx.Search(t.Context(), root, "a", 10)
			if err != nil {
				t.Errorf("second Search() error = %v", err)
			}
			second <- result
		}()
		cancelFirst()
		select {
		case err := <-firstErr:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("first Search() error = %v, want context.Canceled", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("first Search() kept waiting for the build after its context was canceled")
		}
		close(release)
		if got := entryPaths((<-second).Entries); !slices.Equal(got, []string{"a.go"}) {
			t.Fatalf("second Search() = %v, want [a.go] from the shared build", got)
		}
		if calls := runner.listCalls.Load(); calls != 1 {
			t.Fatalf("git ls-files builds = %d, want 1 shared build", calls)
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

func TestIndexSearchStreamingGit(t *testing.T) {
	t.Parallel()

	t.Run("Should index streamed entries past one mebibyte of listing", func(t *testing.T) {
		t.Parallel()
		runner := newFakeStreamingRunner(6_000)
		if runner.listingBytes <= 1<<20 {
			t.Fatalf("listing = %d bytes, want more than 1 MiB", runner.listingBytes)
		}

		root := t.TempDir()
		createListedFiles(t, root, runner.files)
		result, err := New(runner).Search(t.Context(), root, runner.files[len(runner.files)-1], 5)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(result.Entries) == 0 || result.Entries[0].Path != runner.files[len(runner.files)-1] || result.Truncated {
			t.Fatalf(
				"Search(last file) = %#v truncated=%v, want the last streamed file",
				result.Entries,
				result.Truncated,
			)
		}
		if runner.bufferedCalls.Load() != 0 {
			t.Fatalf("buffered Run calls = %d, want streaming only", runner.bufferedCalls.Load())
		}
	})

	t.Run("Should stop the git listing once the cap is exceeded", func(t *testing.T) {
		t.Parallel()
		runner := newFakeStreamingRunner(6_000)

		root := t.TempDir()
		createListedFiles(t, root, runner.files)
		result, err := New(runner, WithMaxFiles(100)).Search(t.Context(), root, "", 200)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if !result.Truncated {
			t.Fatal("Search() Truncated = false, want true")
		}
		if !runner.canceled.Load() {
			t.Fatal("streaming git context was not canceled at the cap")
		}
		if sent, total := runner.sentBytes.Load(), int64(runner.listingBytes); sent >= total {
			t.Fatalf("streamed %d of %d bytes, want the listing cut short", sent, total)
		}
	})
}

// fakeStreamingRunner streams a NUL-separated listing in small chunks that split records.
type fakeStreamingRunner struct {
	files         []string
	listingBytes  int
	sentBytes     atomic.Int64
	canceled      atomic.Bool
	bufferedCalls atomic.Int32
}

func newFakeStreamingRunner(count int) *fakeStreamingRunner {
	runner := &fakeStreamingRunner{files: make([]string, 0, count)}
	for i := range count {
		name := fmt.Sprintf("dir%02d/%s_%05d.go", i%50, strings.Repeat("long_segment_", 15), i)
		runner.files = append(runner.files, name)
		runner.listingBytes += len(name) + 1
	}
	return runner
}

func (f *fakeStreamingRunner) Run(context.Context, string, ...string) ([]byte, []byte, error) {
	f.bufferedCalls.Add(1)
	return nil, nil, errors.New("buffered run must not be used")
}

func (f *fakeStreamingRunner) RunStreaming(
	ctx context.Context,
	_ string,
	onStdout func([]byte),
	args ...string,
) ([]byte, error) {
	var listing strings.Builder
	for _, file := range f.files {
		listing.WriteString(file)
		listing.WriteByte(0)
	}
	data := []byte(listing.String())
	const chunkSize = 4093
	for start := 0; start < len(data); start += chunkSize {
		if err := ctx.Err(); err != nil {
			f.canceled.Store(true)
			return nil, err
		}
		chunk := data[start:min(start+chunkSize, len(data))]
		f.sentBytes.Add(int64(len(chunk)))
		onStdout(chunk)
	}
	return nil, nil
}

type fakeGitRunner struct {
	files          []string
	err            error
	blockUntilDone bool
	release        <-chan struct{}
	listCalls      atomic.Int32
}

func (f *fakeGitRunner) Run(ctx context.Context, _ string, args ...string) ([]byte, []byte, error) {
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

// createListedFiles creates the empty files a fake git listing names, because listed
// paths that do not exist are dropped as deleted.
func createListedFiles(t *testing.T, root string, files []string) {
	t.Helper()
	for _, name := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
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
