package daemon

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/worktree"
)

func TestNewSessionFileIndex(t *testing.T) {
	t.Parallel()

	t.Run("Should index git listings beyond the buffered runner output cap", func(t *testing.T) {
		t.Parallel()
		runner, err := worktree.NewRealGitRunner(0)
		if err != nil {
			t.Skipf("git unavailable: %v", err)
		}
		root, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatalf("resolve temp dir: %v", err)
		}
		listingBytes := 0
		last := ""
		for i := range 5_500 {
			name := fmt.Sprintf("dir%02d/%s_%05d.txt", i%50, strings.Repeat("long_segment_", 15), i)
			path := filepath.Join(root, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", name, err)
			}
			if err := os.WriteFile(path, nil, 0o600); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
			listingBytes += len(name) + 1
			if name > last {
				last = name
			}
		}
		if listingBytes <= 1<<20 {
			t.Fatalf("listing = %d bytes, want more than 1 MiB", listingBytes)
		}
		// The ignored file proves the listing came from git: the walk fallback does not apply .gitignore.
		for name, content := range map[string]string{".gitignore": "ignored_marker.txt\n", "ignored_marker.txt": ""} {
			if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0o600); err != nil {
				t.Fatalf("write %s: %v", name, err)
			}
		}
		if _, stderr, err := runner.Run(t.Context(), root, "init", "-q"); err != nil {
			t.Fatalf("git init: %v: %s", err, stderr)
		}

		index := newSessionFileIndex(slog.New(slog.NewTextHandler(io.Discard, nil)))
		ignored, err := index.Search(t.Context(), root, "ignored_marker", 5)
		if err != nil {
			t.Fatalf("Search(ignored) error = %v", err)
		}
		if len(ignored.Entries) != 0 {
			t.Fatalf("Search(ignored) = %#v, want the git listing without ignored files", ignored.Entries)
		}
		result, err := index.Search(t.Context(), root, filepath.Base(last), 5)
		if err != nil {
			t.Fatalf("Search() error = %v", err)
		}
		if len(result.Entries) == 0 || result.Entries[0].Path != last || result.Truncated {
			t.Fatalf("Search(last listed file) = %#v truncated=%v, want %q", result.Entries, result.Truncated, last)
		}
	})
}
