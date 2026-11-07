package projectfiles

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"reflect"
	"slices"

	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/worktree"
)

// The reader suite owns confinement, classification, caps, ordering and read-only Git policy.
func TestReader(t *testing.T) {
	t.Parallel()
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
