package fileutil

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExistingPathWithinRoot(t *testing.T) {
	t.Parallel()

	t.Run("Should accept direct and internally linked existing paths", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		target := filepath.Join(root, "target.txt")
		if err := os.WriteFile(target, []byte("ok"), 0o600); err != nil {
			t.Fatalf("WriteFile(target) error = %v", err)
		}
		link := filepath.Join(root, "link.txt")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("Symlink(internal) unavailable: %v", err)
		}

		resolvedRoot, resolvedCandidate, err := ResolveExistingPathWithinRoot(root, link)
		if err != nil {
			t.Fatalf("ResolveExistingPathWithinRoot() error = %v", err)
		}
		wantRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			t.Fatalf("EvalSymlinks(root) error = %v", err)
		}
		wantCandidate, err := filepath.EvalSymlinks(target)
		if err != nil {
			t.Fatalf("EvalSymlinks(target) error = %v", err)
		}
		if resolvedRoot != wantRoot || resolvedCandidate != wantCandidate {
			t.Fatalf(
				"resolved paths = %q/%q, want %q/%q",
				resolvedRoot,
				resolvedCandidate,
				wantRoot,
				wantCandidate,
			)
		}
	})

	t.Run("Should reject a symlink whose target escapes the root", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		outside := filepath.Join(t.TempDir(), "outside.txt")
		if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
			t.Fatalf("WriteFile(outside) error = %v", err)
		}
		link := filepath.Join(root, "escape.txt")
		if err := os.Symlink(outside, link); err != nil {
			t.Skipf("Symlink(escape) unavailable: %v", err)
		}

		_, resolvedCandidate, err := ResolveExistingPathWithinRoot(root, link)
		if !errors.Is(err, ErrPathOutsideRoot) {
			t.Fatalf("ResolveExistingPathWithinRoot() error = %v, want ErrPathOutsideRoot", err)
		}
		wantCandidate, evalErr := filepath.EvalSymlinks(outside)
		if evalErr != nil {
			t.Fatalf("EvalSymlinks(outside) error = %v", evalErr)
		}
		if resolvedCandidate != wantCandidate {
			t.Fatalf("resolved candidate = %q, want %q", resolvedCandidate, wantCandidate)
		}
	})

	t.Run("Should preserve a missing-path cause", func(t *testing.T) {
		t.Parallel()

		_, _, err := ResolveExistingPathWithinRoot(t.TempDir(), filepath.Join(t.TempDir(), "missing"))
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("ResolveExistingPathWithinRoot() error = %v, want os.ErrNotExist", err)
		}
	})
}

func TestCanonicalExistingDirectory(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve a linked existing directory", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		target := filepath.Join(root, "target")
		if err := os.Mkdir(target, 0o700); err != nil {
			t.Fatalf("Mkdir(target) error = %v", err)
		}
		link := filepath.Join(root, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Skipf("Symlink(directory) unavailable: %v", err)
		}

		got, err := CanonicalExistingDirectory(link)
		if err != nil {
			t.Fatalf("CanonicalExistingDirectory() error = %v", err)
		}
		want, err := filepath.EvalSymlinks(target)
		if err != nil {
			t.Fatalf("EvalSymlinks(target) error = %v", err)
		}
		if got != want {
			t.Fatalf("CanonicalExistingDirectory() = %q, want %q", got, want)
		}
	})

	t.Run("Should reject an existing regular file", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(path, []byte("value"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		if _, err := CanonicalExistingDirectory(path); err == nil {
			t.Fatal("CanonicalExistingDirectory() error = nil, want non-directory failure")
		}
	})
}

func TestPathWithinRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	inside := filepath.Join(root, "nested", "value")
	outside := filepath.Join(filepath.Dir(root), filepath.Base(root)+"-outside")
	for _, testCase := range []struct {
		name      string
		candidate string
		want      bool
	}{
		{name: "Should accept the root", candidate: root, want: true},
		{name: "Should accept a descendant", candidate: inside, want: true},
		{name: "Should reject a sibling prefix", candidate: outside, want: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := PathWithinRoot(root, testCase.candidate)
			if err != nil {
				t.Fatalf("PathWithinRoot() error = %v", err)
			}
			if got != testCase.want {
				t.Fatalf("PathWithinRoot() = %t, want %t", got, testCase.want)
			}
		})
	}
}
