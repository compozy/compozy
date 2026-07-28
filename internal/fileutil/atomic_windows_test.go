//go:build windows

package fileutil

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFileWindowsOverwrite(t *testing.T) {
	t.Run("ShouldReplaceExistingTargetWithoutMissingOrPartialReads", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), "target.txt")
		versions := [][]byte{
			bytes.Repeat([]byte("A"), 1024),
			bytes.Repeat([]byte("B"), 4096),
			bytes.Repeat([]byte("C"), 8192),
		}
		if err := os.WriteFile(path, versions[0], 0o644); err != nil {
			t.Fatalf("WriteFile(seed) error = %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		readErr := make(chan error, 1)
		go func() {
			for {
				select {
				case <-ctx.Done():
					readErr <- nil
					return
				default:
				}

				content, err := os.ReadFile(path)
				if err != nil {
					readErr <- fmt.Errorf("ReadFile(during overwrite): %w", err)
					return
				}
				if !matchesAtomicContent(content, versions) {
					readErr <- fmt.Errorf("ReadFile(during overwrite) returned unexpected content length %d", len(content))
					return
				}
			}
		}()

		last := versions[0]
		for i := 1; i <= 150; i++ {
			last = versions[i%len(versions)]
			if err := AtomicWriteFile(path, last, 0o644); err != nil {
				t.Fatalf("AtomicWriteFile() error = %v", err)
			}
		}

		cancel()
		if err := <-readErr; err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("ReadFile(final) error = %v", err)
		}
		if !bytes.Equal(got, last) {
			t.Fatalf("ReadFile(final) = %d bytes, want %d bytes", len(got), len(last))
		}
	})
}

func matchesAtomicContent(got []byte, versions [][]byte) bool {
	for _, version := range versions {
		if bytes.Equal(got, version) {
			return true
		}
	}
	return false
}
