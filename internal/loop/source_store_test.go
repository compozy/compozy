package loop_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/compozy/agh/internal/loop"
)

func TestSourceStoreShouldValidateWriteForkAndDelete(t *testing.T) {
	t.Parallel()

	t.Run("Should write and delete a valid writable definition", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		spec, path, err := loop.WriteDefinition(
			root,
			[]byte(loopDefinitionYAML("writable-loop")),
			loop.WriteDefinitionOptions{Source: loop.SourceUser},
		)
		if err != nil {
			t.Fatalf("WriteDefinition() error = %v", err)
		}
		if got, want := path, filepath.Join(root, "writable-loop", loop.DefinitionFileName); got != want {
			t.Fatalf("WriteDefinition() path = %q, want %q", got, want)
		}
		if got, want := spec.FilePath, path; got != want {
			t.Fatalf("WriteDefinition() spec.FilePath = %q, want %q", got, want)
		}
		if err := loop.DeleteWritableDefinition(root, "writable-loop"); err != nil {
			t.Fatalf("DeleteWritableDefinition() error = %v", err)
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("os.Stat(deleted path) error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("Should reject invalid YAML before writing", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		_, _, err := loop.WriteDefinition(
			root,
			[]byte("apiVersion: agh.loop/v1\nkind: ["),
			loop.WriteDefinitionOptions{Source: loop.SourceUser},
		)
		if err == nil {
			t.Fatal("WriteDefinition(invalid) error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "parse loop definition") {
			t.Fatalf("WriteDefinition(invalid) error = %v, want parse loop definition", err)
		}
		if entries, readErr := os.ReadDir(root); readErr != nil || len(entries) != 0 {
			t.Fatalf("os.ReadDir(root) = %#v, %v, want empty directory", entries, readErr)
		}
	})

	t.Run("Should reject duplicate writable definitions without overwrite", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		body := []byte(loopDefinitionYAML("duplicate-loop"))
		if _, _, err := loop.WriteDefinition(
			root,
			body,
			loop.WriteDefinitionOptions{Source: loop.SourceUser},
		); err != nil {
			t.Fatalf("WriteDefinition(first) error = %v", err)
		}
		_, _, err := loop.WriteDefinition(
			root,
			body,
			loop.WriteDefinitionOptions{Source: loop.SourceUser},
		)
		if !errors.Is(err, loop.ErrDefinitionExists) {
			t.Fatalf("WriteDefinition(second) error = %v, want ErrDefinitionExists", err)
		}
	})

	t.Run("Should write a tool action definition without schema source", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		body := strings.Replace(
			loopDefinitionYAML("tool-action-write"),
			"      kind: transform",
			"      kind: agh__network_send",
			1,
		)
		spec, path, err := loop.WriteDefinition(
			root,
			[]byte(body),
			loop.WriteDefinitionOptions{Source: loop.SourceUser},
		)
		if err != nil {
			t.Fatalf("WriteDefinition(tool action) error = %v", err)
		}
		if got, want := spec.FilePath, path; got != want {
			t.Fatalf("WriteDefinition() spec.FilePath = %q, want %q", got, want)
		}
		if got, want := path, filepath.Join(root, "tool-action-write", loop.DefinitionFileName); got != want {
			t.Fatalf("WriteDefinition() path = %q, want %q", got, want)
		}
	})

	t.Run("Should fork a file-backed definition into a writable root", func(t *testing.T) {
		t.Parallel()

		sourceRoot := t.TempDir()
		sourceSpec, sourcePath, err := loop.WriteDefinition(
			sourceRoot,
			[]byte(loopDefinitionYAML("file-backed-loop")),
			loop.WriteDefinitionOptions{Source: loop.SourceUser},
		)
		if err != nil {
			t.Fatalf("WriteDefinition(source) error = %v", err)
		}
		sidecar := filepath.Join(sourceSpec.Dir, "README.md")
		if err := os.WriteFile(sidecar, []byte("sidecar"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(sidecar) error = %v", err)
		}

		targetRoot := t.TempDir()
		path, err := loop.ForkDefinitionFile(sourcePath, targetRoot)
		if err != nil {
			t.Fatalf("ForkDefinitionFile() error = %v", err)
		}
		if got, want := path, filepath.Join(targetRoot, "file-backed-loop", loop.DefinitionFileName); got != want {
			t.Fatalf("ForkDefinitionFile() path = %q, want %q", got, want)
		}
		if _, err := os.Stat(filepath.Join(targetRoot, "file-backed-loop", "README.md")); err != nil {
			t.Fatalf("forked sidecar stat error = %v", err)
		}
		_, err = loop.ForkDefinitionFile(sourcePath, targetRoot)
		if !errors.Is(err, loop.ErrDefinitionExists) {
			t.Fatalf("second ForkDefinitionFile() error = %v, want ErrDefinitionExists", err)
		}
	})
}
