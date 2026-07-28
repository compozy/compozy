package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}

func TestRun(t *testing.T) {
	t.Parallel()

	t.Run("Should propagate digest output failures", func(t *testing.T) {
		t.Parallel()

		artifactPath := filepath.Join(t.TempDir(), "artifact")
		if err := os.WriteFile(artifactPath, []byte("catalog artifact"), 0o600); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
		writeErr := errors.New("stdout unavailable")
		err := run([]string{"digest", artifactPath}, failingWriter{err: writeErr})
		if !errors.Is(err, writeErr) {
			t.Fatalf("run(digest) error = %v, want output failure", err)
		}
	})
}
