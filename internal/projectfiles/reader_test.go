package projectfiles

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/workspacefiles"

	"reflect"
	"slices"

	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/worktree"
)

// The reader suite owns confinement, classification, caps, ordering and read-only Git policy.
func TestReader(t *testing.T) {
	t.Parallel()
	for _, list := range []bool{false, true} {
		t.Run(fmt.Sprintf("Should reject FIFOs without blocking list=%v", list), func(t *testing.T) {
			t.Parallel()
			root := readerRoot(t)
			pipe := filepath.Join(root.AbsPath, "pipe")
			if out, err := exec.CommandContext(t.Context(), "mkfifo", pipe).CombinedOutput(); err != nil {
				t.Skipf("mkfifo unavailable: %s %v", out, err)
			}
			done := make(chan error, 1)
			go func() {
				var err error
				if list {
					_, err = NewReader().List(t.Context(), root, "pipe", ListOptions{})
				} else {
					_, err = NewReader().Read(t.Context(), root, "pipe", ReadOptions{})
				}
				done <- err
			}()
			select {
			case err := <-done:
				want := ErrNotAFile
				if list {
					want = ErrNotADirectory
				}
				if !errors.Is(err, want) {
					t.Fatalf("list=%v error=%v", list, err)
				}
			case <-time.After(time.Second):
				// Release the blocked open before failing.
				f, err := os.OpenFile(pipe, os.O_RDWR, 0600)
				if err != nil {
					t.Fatal(err)
				}
				<-done
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
				t.Fatalf("list=%v blocked opening FIFO", list)
			}
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			var err error
			if list {
				_, err = NewReader().List(ctx, root, "pipe", ListOptions{})
			} else {
				_, err = NewReader().Read(ctx, root, "pipe", ReadOptions{})
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel=%v", err)
			}
		})
	}
	t.Run("Should sort directories first in natural order", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		writeReaderFile(t, root, "A.txt", "a")
		writeReaderFile(t, root, "c.md", "c")
		writeReaderFile(t, root, "file10", "10")
		writeReaderFile(t, root, "file2", "2")
		if err := os.Mkdir(filepath.Join(root.AbsPath, "a"), 0700); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := os.Mkdir(filepath.Join(root.AbsPath, "b"), 0700); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err := NewReader().List(t.Context(), root, "", ListOptions{ShowIgnored: true})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var names []string
		for _, entry := range got.Entries {
			names = append(names, entry.Name)
		}
		if !reflect.DeepEqual([]string{"a", "b", "A.txt", "c.md", "file2", "file10"}, names) {
			t.Fatalf("got %v, want %v", names, []string{"a", "b", "A.txt", "c.md", "file2", "file10"})
		}
	})
	t.Run("Should cap listings only beyond five thousand entries", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		r := NewReader()
		for i := range EntryLimit {
			writeReaderFile(t, root, fmt.Sprintf("f%d", i), "")
		}
		got, err := r.List(t.Context(), root, "", ListOptions{})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got.Entries) != EntryLimit {
			t.Fatalf("length = %d, want %d", len(got.Entries), EntryLimit)
		}
		if got.Truncated {
			t.Fatal("expected false")
		}
		writeReaderFile(t, root, "extra", "")
		got, err = r.List(t.Context(), root, "", ListOptions{})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !(got.Truncated) {
			t.Fatal("expected true: got.Truncated")
		}
		if len(got.Entries) != EntryLimit {
			t.Fatalf("length = %d, want %d", len(got.Entries), EntryLimit)
		}
	})
	t.Run("Should classify symlinks and protect resolved targets", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		writeReaderFile(t, root, "pkg/a", "a")
		writeReaderFile(t, root, ".env", "SECRET")
		writeReaderFile(t, root, ".git/config", "metadata")
		for name, target := range map[string]string{"absolute": filepath.Join(root.AbsPath, "pkg", "a"), "inside": "pkg", "outside": "/etc", "dangling": "nope", "notes.txt": ".env", "meta": ".git"} {
			if err := os.Symlink(target, filepath.Join(root.AbsPath, name)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}
		r := NewReader()
		listing, err := r.List(t.Context(), root, "", ListOptions{ShowIgnored: true})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		entries := map[string]Entry{}
		for _, entry := range listing.Entries {
			entries[entry.Name] = entry
		}
		if func() bool { _, ok := entries["outside"]; return ok }() {
			t.Fatal("unexpected membership")
		}
		if func() bool { _, ok := entries["dangling"]; return ok }() {
			t.Fatal("unexpected membership")
		}
		if func() bool { _, ok := entries[".git"]; return ok }() {
			t.Fatal("unexpected membership")
		}
		if func() bool { _, ok := entries["meta"]; return ok }() {
			t.Fatal("unexpected membership")
		}
		if !(entries["inside"].Symlink) {
			t.Fatal("expected true: entries['inside'].Symlink")
		}
		if !reflect.DeepEqual(EntryKindDirectory, entries["inside"].Kind) {
			t.Fatalf("got %v, want %v", entries["inside"].Kind, EntryKindDirectory)
		}
		if !(entries["notes.txt"].Sensitive) {
			t.Fatal("expected true: entries['notes.txt'].Sensitive")
		}
		absolute, err := r.Read(t.Context(), root, "absolute", ReadOptions{})
		if err != nil || absolute.Text != "a" {
			t.Fatalf("absolute in-root alias = %#v, %v", absolute, err)
		}
		got, err := r.Read(t.Context(), root, "notes.txt", ReadOptions{})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(ContentKindSensitive, got.Kind) {
			t.Fatalf("got %v, want %v", got.Kind, ContentKindSensitive)
		}
		if got.Text != "" {
			t.Fatal("expected empty value")
		}
		_, err = r.Read(t.Context(), root, "meta/config", ReadOptions{})
		if !errors.Is(err, ErrPathDenied) {
			t.Fatalf("error = %v, want %v", err, ErrPathDenied)
		}
	})
	t.Run("Should reject malformed paths for list and read", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		r := NewReader()
		for _, path := range []string{"../x", "/etc/passwd", "a/../../x", "%2e%2e/x", "a\x00b", strings.Repeat("x", 1025), ".git/config", ".git"} {
			_, err := r.List(t.Context(), root, path, ListOptions{})
			if !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidPath)
			}
			_, err = r.Read(t.Context(), root, path, ReadOptions{})
			if !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("error = %v, want %v", err, ErrInvalidPath)
			}
		}
	})
	t.Run("Should reject an alias swapped outside the root at open time", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		writeReaderFile(t, root, "safe.txt", "safe")
		link := filepath.Join(root.AbsPath, "inner")
		if err := os.Symlink("safe.txt", link); err != nil {
			t.Fatal(err)
		}
		r := NewReader()
		r.open = func(confined *os.Root, path string) (*os.File, error) {
			if err := os.Remove(link); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("/etc/hosts", link); err != nil {
				t.Fatal(err)
			}
			return confined.Open(path)
		}
		got, err := r.Read(t.Context(), root, "inner", ReadOptions{})
		if !errors.Is(err, ErrPathOutsideRoot) || got.Text != "" {
			t.Fatalf("Read = %#v, %v", got, err)
		}
	})

	t.Run("Should read text with language size and revision", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		writeReaderFile(t, root, "web/main.tsx", "export {}\n")
		var r Reader = NewReader()
		got, err := r.Read(t.Context(), root, "web/main.tsx", ReadOptions{})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(ContentKindText, got.Kind) {
			t.Fatalf("got %v, want %v", got.Kind, ContentKindText)
		}
		if !reflect.DeepEqual("tsx", got.Language) {
			t.Fatalf("got %v, want %v", got.Language, "tsx")
		}
		if !reflect.DeepEqual("export {}\n", got.Text) {
			t.Fatalf("got %v, want %v", got.Text, "export {}\n")
		}
		if got.Size != 10 {
			t.Fatalf("got %v, want %v", got.Size, 10)
		}
		if !(strings.HasPrefix(got.Revision, "sha256:")) {
			t.Fatal("expected true: strings.HasPrefix(got.Revision,'sha256:')")
		}
	})
	t.Run("Should classify missing files directories and unavailable roots", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		r := NewReader()
		_, err := r.Read(t.Context(), root, "missing", ReadOptions{})
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("error = %v, want %v", err, ErrNotFound)
		}
		_, err = r.Read(t.Context(), root, "", ReadOptions{})
		if !errors.Is(err, ErrNotAFile) {
			t.Fatalf("error = %v, want %v", err, ErrNotAFile)
		}
		writeReaderFile(t, root, "file", "x")
		_, err = r.List(t.Context(), root, "file", ListOptions{})
		if !errors.Is(err, ErrNotADirectory) {
			t.Fatalf("error = %v, want %v", err, ErrNotADirectory)
		}
		if err := os.RemoveAll(root.AbsPath); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = r.List(t.Context(), root, "", ListOptions{})
		if !errors.Is(err, ErrRootUnavailable) {
			t.Fatalf("error = %v, want %v", err, ErrRootUnavailable)
		}
	})
	t.Run("Should truncate text at a UTF8 boundary", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		writeReaderFile(t, root, "ascii.txt", strings.Repeat("a", TextLimit+1))
		ascii, err := NewReader().Read(t.Context(), root, "ascii.txt", ReadOptions{})
		if err != nil || len(ascii.Text) != TextLimit || !ascii.Truncated {
			t.Fatalf("ASCII limit = %d, %v", len(ascii.Text), err)
		}
		writeReaderFile(t, root, "long.txt", strings.Repeat("é", ImageLimit))
		long, err := NewReader().Read(t.Context(), root, "long.txt", ReadOptions{})
		if err != nil || long.Kind != ContentKindText || !long.Truncated {
			t.Fatalf("long UTF8 = %s, %v", long.Kind, err)
		}
		body := strings.Repeat("a", TextLimit-1) + "é"
		writeReaderFile(t, root, "large.txt", body)
		got, err := NewReader().Read(t.Context(), root, "large.txt", ReadOptions{})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(ContentKindText, got.Kind) {
			t.Fatalf("got %v, want %v", got.Kind, ContentKindText)
		}
		if !(got.Truncated) {
			t.Fatal("expected true: got.Truncated")
		}
		if len(got.Text) != TextLimit-1 {
			t.Fatalf("length = %d, want %d", len(got.Text), TextLimit-1)
		}
	})
	t.Run("Should classify images binary and sensitive contents", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		r := NewReader()
		for name, body := range map[string]string{".env": "secret", "config/.env.production": "secret", "id_rsa": "secret", "key.pem": "secret", ".env.example": "example", "nul": "a\x00b", "invalid": string([]byte{255, 254})} {
			writeReaderFile(t, root, name, body)
			got, err := r.Read(t.Context(), root, name, ReadOptions{})
			if err := err; err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			switch name {
			case ".env.example":
				if !reflect.DeepEqual(ContentKindText, got.Kind) {
					t.Fatalf("got %v, want %v", got.Kind, ContentKindText)
				}
			case "nul", "invalid":
				if !reflect.DeepEqual(ContentKindBinary, got.Kind) {
					t.Fatalf("got %v, want %v", got.Kind, ContentKindBinary)
				}
				if got.Text != "" {
					t.Fatal("expected empty value")
				}
			default:
				if !reflect.DeepEqual(ContentKindSensitive, got.Kind) {
					t.Fatalf("got %v, want %v", got.Kind, ContentKindSensitive)
				}
				if got.Text != "" {
					t.Fatal("expected empty value")
				}
				revealed, err := r.Read(t.Context(), root, name, ReadOptions{Reveal: true})
				if err := err; err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if !reflect.DeepEqual(body, revealed.Text) {
					t.Fatalf("got %v, want %v", revealed.Text, body)
				}
				if !(revealed.Sensitive) {
					t.Fatal("expected true: revealed.Sensitive")
				}
			}
		}
		png := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{'a'}, ImageLimit-8)...)
		writeReaderFile(t, root, "image.png", string(png))
		got, err := r.Read(t.Context(), root, "image.png", ReadOptions{})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(ContentKindImage, got.Kind) {
			t.Fatalf("got %v, want %v", got.Kind, ContentKindImage)
		}
		if !reflect.DeepEqual("image/png", got.ImageMIME) {
			t.Fatalf("got %v, want %v", got.ImageMIME, "image/png")
		}
		if len(got.ImageData) != ImageLimit {
			t.Fatalf("length = %d, want %d", len(got.ImageData), ImageLimit)
		}
		writeReaderFile(t, root, "image.png", string(append(png, 'a')))
		got, err = r.Read(t.Context(), root, "image.png", ReadOptions{})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !(got.Truncated) {
			t.Fatal("expected true: got.Truncated")
		}
		if len(got.ImageData) != 0 {
			t.Fatal("expected empty value")
		}
	})
	t.Run("Should return entries when git times out", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		gitReader(t, root, "init")
		writeReaderFile(t, root, "a", "a")
		runner, err := worktree.NewRealGitRunner(time.Nanosecond)
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got, err := NewReader(WithGitRunner(runner)).List(t.Context(), root, "", ListOptions{})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !(got.StatusUnavailable) {
			t.Fatal("expected true: got.StatusUnavailable")
		}
		if len(got.Entries) != 1 {
			t.Fatalf("length = %d, want %d", len(got.Entries), 1)
		}
	})
	t.Run("Should decorate status within a subdirectory without running configured programs", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		gitReader(t, root, "init")
		gitReader(t, root, "config", "user.email", "test@example.com")
		gitReader(t, root, "config", "user.name", "Test")
		writeReaderFile(t, root, "sub/src/x.go", "initial\n")
		writeReaderFile(t, root, "sub/y.go", "initial\n")
		writeReaderFile(t, root, "top", "top\n")
		gitReader(t, root, "add", ".")
		gitReader(t, root, "commit", "-m", "initial")
		gitReader(t, root, "branch", "-M", "main")
		gitReader(t, root, "branch", "side")
		writeReaderFile(t, root, "sub/y.go", "main\n")
		gitReader(t, root, "commit", "-am", "main change")
		gitReader(t, root, "switch", "side")
		writeReaderFile(t, root, "sub/y.go", "side\n")
		gitReader(t, root, "commit", "-am", "side change")
		merge := exec.CommandContext(t.Context(), "git", "merge", "main")
		merge.Dir = root.AbsPath
		mergeData, mergeErr := merge.CombinedOutput()
		if mergeErr == nil || !bytes.Contains(mergeData, []byte("CONFLICT")) {
			t.Fatalf("expected conflict: %s, %v", mergeData, mergeErr)
		}
		writeReaderFile(t, root, "sub/src/x.go", "modified\n")
		writeReaderFile(t, root, "sub/new.txt", "new\n")
		writeReaderFile(t, root, "sub/.gitignore", "ignored\n")
		writeReaderFile(t, root, "sub/ignored", "ignored\n")
		marker := testutil.ConfigureGitReadMarkers(t, root.AbsPath)
		sub := root
		sub.AbsPath = filepath.Join(root.AbsPath, "sub")
		r := NewReader()
		got, err := r.List(t.Context(), sub, "", ListOptions{ShowIgnored: true})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got.StatusUnavailable {
			t.Fatal("expected false")
		}
		entries := map[string]Entry{}
		for _, entry := range got.Entries {
			entries[entry.Name] = entry
		}
		if !(entries["src"].ContainsChanges) {
			t.Fatal("expected true: entries['src'].ContainsChanges")
		}
		if entries["y.go"].GitStatus != "conflicted" {
			t.Fatalf("conflict status = %s", entries["y.go"].GitStatus)
		}
		if !reflect.DeepEqual(GitStatus("untracked"), entries["new.txt"].GitStatus) {
			t.Fatalf("got %v, want %v", entries["new.txt"].GitStatus, GitStatus("untracked"))
		}
		if !(entries["ignored"].Ignored) {
			t.Fatal("expected true: entries['ignored'].Ignored")
		}
		if func() bool { _, ok := entries["top"]; return ok }() {
			t.Fatal("unexpected membership")
		}
		got, err = r.List(t.Context(), sub, "src", ListOptions{})
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(GitStatus("modified"), got.Entries[0].GitStatus) {
			t.Fatalf("got %v, want %v", got.Entries[0].GitStatus, GitStatus("modified"))
		}
		_, err = os.Stat(marker)
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("error = %v, want %v", err, os.ErrNotExist)
		}
		runner, err := worktree.NewRealGitRunner(time.Second)
		if err := err; err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, command := range []string{"status", "diff", "grep", "ls-files", "cat-file"} {
			args, err := GitReadArgs(t.Context(), runner, root.AbsPath, command)
			if err := err; err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Contains(args, "--no-optional-locks") {
				t.Fatal("unexpected membership")
			}
			if !slices.Contains(args, "core.fsmonitor=false") {
				t.Fatal("unexpected membership")
			}
			if command == "grep" {
				if slices.Contains(args, "--no-ext-diff") {
					t.Fatal("unexpected membership")
				}
			}
			if command == "status" {
				if !slices.Contains(args, "filter.evil.clean=") {
					t.Fatal("unexpected membership")
				}
			}
		}
	})
}
func readerRoot(t *testing.T) Root {
	t.Helper()
	root, err := CanonicalRoot(Root{Kind: RootKindWorkspace, WorkspaceID: "workspace", AbsPath: t.TempDir()})
	if err := err; err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return root
}
func writeReaderFile(t *testing.T, root Root, path, body string) {
	t.Helper()
	target := filepath.Join(root.AbsPath, path)
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := os.WriteFile(target, []byte(body), 0600); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
func gitReader(t *testing.T, root Root, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = root.AbsPath
	data, err := cmd.CombinedOutput()
	if err := err; err != nil {
		t.Fatalf("git error: %v: %s", err, data)
	}
}

func TestFileSearch(t *testing.T) {
	t.Parallel()
	t.Run("Should rank fuzzy paths and refresh an isolated index", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		searchGit(t, root, "init")
		path := "web/src/systems/session/components/session-review-panel.tsx"
		writeReaderFile(t, root, path, "useSessionChanges\n")
		for i := range 500 {
			writeReaderFile(t, root, fmt.Sprintf("decoy%03d.go", i), "")
		}
		writeReaderFile(t, root, ".gitignore", "node_modules/\n")
		writeReaderFile(t, root, "node_modules/hidden", "hidden")
		writeReaderFile(t, root, ".env", "SECRET")
		if err := os.Symlink(".env", filepath.Join(root.AbsPath, "notes.txt")); err != nil {
			t.Fatal(err)
		}
		now := time.Unix(100, 0)
		search := NewSearch(nil, workspacefiles.WithClock(func() time.Time { return now }))
		got, err := search.Find(t.Context(), root, "sesrevpan", 50)
		if err != nil || len(got.Results) == 0 || got.Results[0].Path != path ||
			!reflect.DeepEqual(got.Results[0].MatchRanges, [][2]int{{35, 38}, {43, 46}, {50, 53}}) {
			t.Fatalf("find=%+v err=%v", got, err)
		}
		for _, limit := range []int{0, 100} {
			matches, err := search.Find(t.Context(), root, "decoy", limit)
			if err != nil || len(matches.Results) != 50 {
				t.Fatalf("limit %d results %d err %v", limit, len(matches.Results), err)
			}
		}
		got, err = search.Find(t.Context(), root, "notes", 0)
		if err != nil || len(got.Results) == 0 || got.Results[0].Path != "notes.txt" || !got.Results[0].Sensitive {
			t.Fatalf("sensitive=%+v err=%v", got, err)
		}
		got, err = search.Find(t.Context(), root, "hidden", 100)
		if err != nil || len(got.Results) != 0 {
			t.Fatalf("ignored=%+v err=%v", got, err)
		}
		writeReaderFile(t, root, "added.go", "")
		now = now.Add(16 * time.Second)
		got, err = search.Find(t.Context(), root, "added", 50)
		if err != nil || len(got.Results) != 1 {
			t.Fatalf("refresh=%+v err=%v", got, err)
		}
		if _, err = search.Find(t.Context(), root, "", 50); !errors.Is(err, ErrInvalidQuery) {
			t.Fatalf("empty: %v", err)
		}
	})
	t.Run("Should search confined content with engine parity and safe Git configuration", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		searchGit(t, root, "init")
		writeReaderFile(t, root, "top.txt", "useSessionChanges\n")
		sub := root
		sub.AbsPath = filepath.Join(root.AbsPath, "project")
		writeReaderFile(t, sub, "web/a.ts", "first useSessionChanges\nuseOtherChanges\n")
		writeReaderFile(t, sub, "web/a.test.ts", "useSessionChanges\n")
		writeReaderFile(t, sub, "outside.ts", "useSessionChanges\n")
		writeReaderFile(t, sub, ".env", "useSessionChanges\n")
		writeReaderFile(t, sub, ".env.example", "useSessionChanges\n")
		writeReaderFile(t, sub, "binary", "useSessionChanges\x00")
		writeReaderFile(t, sub, "large", strings.Repeat("useSessionChanges", 130000))
		if err := os.Symlink(".env", filepath.Join(sub.AbsPath, "notes.txt")); err != nil {
			t.Fatal(err)
		}
		postPattern, err := compileSearch(SearchQuery{Pattern: "useSessionChanges"})
		if err != nil {
			t.Fatal(err)
		}
		post := searchState{root: sub, search: NewSearch(nil), pattern: postPattern, seen: map[string]bool{}}
		for _, path := range []string{".env", "notes.txt", "binary", "large"} {
			post.addFile(t.Context(), path)
		}
		if post.result.TotalMatches != 0 {
			t.Fatal("post-filter exposed protected content")
		}
		marker := testutil.ConfigureGitReadMarkers(t, root.AbsPath)
		q := SearchQuery{
			Pattern:       "use[A-Z][a-z]+Changes",
			Regex:         true,
			CaseSensitive: true,
			WholeWord:     true,
			Include:       []string{"web/**"},
			Exclude:       []string{"**/*.test.ts"},
		}
		got, err := NewSearch(nil).Search(t.Context(), sub, q)
		if err != nil || got.Engine != "git_grep" || got.TotalMatches != 2 || got.TotalFiles != 1 ||
			got.Files[0].Path != "web/a.ts" {
			t.Fatalf("git=%+v err=%v", got, err)
		}
		plain := readerRoot(t)
		writeReaderFile(t, plain, "web/a.ts", "first useSessionChanges\nuseOtherChanges\n")
		walk, err := NewSearch(nil).Search(t.Context(), plain, q)
		if err != nil || walk.Engine != "walk" || !reflect.DeepEqual(walk.Files, got.Files) {
			t.Fatalf("walk=%+v git=%+v err=%v", walk, got, err)
		}
		all, err := NewSearch(nil).Search(t.Context(), sub, SearchQuery{Pattern: "useSessionChanges"})
		if err != nil || all.TotalFiles != 4 || all.SensitiveExcluded != 2 {
			t.Fatalf("filtered=%+v err=%v", all, err)
		}
		if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Git ran configured program: %v", err)
		}
	})
	t.Run("Should enforce total per-file and excerpt caps", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		for i := range 25 {
			writeReaderFile(t, root, fmt.Sprintf("file%02d", i), strings.Repeat("needle\n", 150))
		}
		got, err := NewSearch(nil).Search(t.Context(), root, SearchQuery{Pattern: "needle"})
		if err != nil || got.TotalMatches != 2000 || !got.Truncated {
			t.Fatalf("caps=%+v err=%v", got, err)
		}
		for _, file := range got.Files {
			if len(file.Matches) > 100 {
				t.Fatal("per file cap")
			}
		}
		long := readerRoot(t)
		writeReaderFile(t, long, "line", strings.Repeat("x", 4000)+"needle"+strings.Repeat("x", 1000))
		got, err = NewSearch(nil).Search(t.Context(), long, SearchQuery{Pattern: "needle"})
		if err != nil || got.TotalMatches != 1 {
			t.Fatalf("excerpt=%+v err=%v", got, err)
		}
		match := got.Files[0].Matches[0]
		if len([]rune(match.Text)) > 300 || match.Column != 4001 || match.TextOffset == 0 ||
			match.Text[match.Ranges[0][0]:match.Ranges[0][1]] != "needle" {
			t.Fatalf("excerpt=%+v", match)
		}
	})
	t.Run("Should reject invalid queries before invoking Git", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		queries := []SearchQuery{
			{Pattern: "a"},
			{Pattern: strings.Repeat("a", 513)},
			{Pattern: "foo(", Regex: true},
			{Pattern: "good", Include: make([]string, 11)},
		}
		for _, glob := range []string{"../go.mod", "/etc/x", ":(top)x", "a\\b", "[", "a["} {
			queries = append(queries, SearchQuery{Pattern: "good", Include: []string{glob}})
		}
		for _, q := range queries {
			if _, err := NewSearch(nil).Search(t.Context(), root, q); !errors.Is(err, ErrInvalidQuery) {
				t.Fatalf("q=%+v err=%v", q, err)
			}
		}
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		if _, err := NewSearch(nil).Search(ctx, root, SearchQuery{Pattern: "good"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("cancel=%v", err)
		}
	})
}
func searchGit(t *testing.T, root Root, args ...string) {
	t.Helper()
	command := exec.CommandContext(t.Context(), "git", args...)
	command.Dir = root.AbsPath
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s %v", args, output, err)
	}
}

func TestFileSearchCancellation(t *testing.T) {
	t.Parallel()
	t.Run("Should join the streamed Git call on request cancellation", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		searchGit(t, root, "init")
		writeReaderFile(t, root, "a.go", "needle")
		search := NewSearch(nil)
		if _, err := search.Find(t.Context(), root, "a.go", 50); err != nil {
			t.Fatal(err)
		}
		runner := &blockingSearchGit{delegate: search.runner, started: make(chan struct{}), joined: make(chan struct{})}
		search.runner = runner
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		result := make(chan error, 1)
		go func() { _, err := search.Search(ctx, root, SearchQuery{Pattern: "needle"}); result <- err }()
		select {
		case <-runner.started:
		case <-time.After(5 * time.Second):
			t.Fatal("grep did not start")
		}
		cancel()
		select {
		case err := <-result:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("cancel=%v", err)
			}
		case <-time.After(200 * time.Millisecond):
			t.Fatal("cancellation exceeded 200ms")
		}
		select {
		case <-runner.joined:
		default:
			t.Fatal("runner has not joined")
		}
	})
}

type blockingSearchGit struct {
	delegate        worktree.StreamingGitRunner
	started, joined chan struct{}
	emit            bool
}

var _ worktree.StreamingGitRunner = (*blockingSearchGit)(nil)

func (r *blockingSearchGit) RunStreaming(
	ctx context.Context,
	root string,
	visit worktree.GitOutputHandler,
	args ...string,
) ([]byte, []byte, error) {
	if slices.Contains(args, "grep") {
		close(r.started)
		if r.emit {
			visit(worktree.GitOutput{Stream: worktree.GitOutputStdout, Chunk: []byte("a.go\x001\x001\x00needle\n")})
		}
		<-ctx.Done()
		close(r.joined)
		return nil, nil, ctx.Err()
	}
	return r.delegate.RunStreaming(ctx, root, visit, args...)
}

func TestFileSearchTimeout(t *testing.T) {
	t.Parallel()
	t.Run("Should return partial results at the fixed search deadline", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		searchGit(t, root, "init")
		writeReaderFile(t, root, "a.go", "needle")
		search := NewSearch(nil)
		if _, err := search.Find(t.Context(), root, "a.go", 50); err != nil {
			t.Fatal(err)
		}
		runner := &blockingSearchGit{
			delegate: search.runner,
			started:  make(chan struct{}),
			joined:   make(chan struct{}),
			emit:     true,
		}
		search.runner = runner
		result, err := search.Search(t.Context(), root, SearchQuery{Pattern: "needle"})
		if err != nil || !result.TimedOut || result.TotalMatches != 1 {
			t.Fatalf("timeout=%+v err=%v", result, err)
		}
	})
}

func TestFileSearchIndexCap(t *testing.T) {
	t.Parallel()
	t.Run("Should index at most two hundred thousand files", func(t *testing.T) {
		t.Parallel()
		root := readerRoot(t)
		for i := range FindIndexLimit + 1 {
			writeReaderFile(t, root, fmt.Sprintf("f%06d.go", i), "")
		}
		result, err := NewSearch(nil).Find(t.Context(), root, "no-match", 50)
		if err != nil || !result.IndexTruncated || result.IndexedFiles != 200000 || len(result.Results) != 0 {
			t.Fatalf(
				"cap files=%d truncated=%v results=%d err=%v",
				result.IndexedFiles,
				result.IndexTruncated,
				len(result.Results),
				err,
			)
		}
	})
}

type searchPathsFixture struct{ entries []workspacefiles.Entry }

var _ workspacefiles.Index = searchPathsFixture{}

func (f searchPathsFixture) Paths(context.Context, string) ([]workspacefiles.Entry, bool, error) {
	return f.entries, false, nil
}
func (f searchPathsFixture) Search(context.Context, string, string, int) (workspacefiles.Result, error) {
	return workspacefiles.Result{}, nil
}
func BenchmarkFileFinder100k(b *testing.B) {
	root := Root{AbsPath: b.TempDir()}
	target := "session-review-panel.tsx"
	if err := os.WriteFile(filepath.Join(root.AbsPath, target), nil, 0600); err != nil {
		b.Fatal(err)
	}
	entries := make([]workspacefiles.Entry, 100000)
	for i := range entries {
		entries[i] = workspacefiles.Entry{Path: fmt.Sprintf("src/module%06d/item.go", i), Kind: workspacefiles.KindFile}
	}
	entries[0].Path = target
	search := NewSearch(nil)
	search.index = searchPathsFixture{entries: entries}
	for b.Loop() {
		result, err := search.Find(b.Context(), root, "sesrevpan", 50)
		if err != nil || len(result.Results) != 1 {
			b.Fatalf("find results=%d err=%v", len(result.Results), err)
		}
	}
}
