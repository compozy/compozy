//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestConfigSidecarReadsRejectFIFOWithoutBlocking(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a FIFO sidecar without blocking", func(t *testing.T) {
		t.Parallel()

		path := filepath.Join(t.TempDir(), MCPJSONName)
		if err := unix.Mkfifo(path, 0o600); err != nil {
			t.Fatalf("unix.Mkfifo(%q) error = %v", path, err)
		}

		result := make(chan error, 1)
		go func() {
			_, err := LoadMCPServersJSONFile(path)
			result <- err
		}()

		select {
		case err := <-result:
			if err == nil {
				t.Fatal("LoadMCPServersJSONFile(FIFO) error = nil, want regular-file rejection")
			}
			if !strings.Contains(err.Error(), "regular file") {
				t.Fatalf("LoadMCPServersJSONFile(FIFO) error = %v, want regular-file context", err)
			}
		case <-time.After(time.Second):
			t.Fatal("LoadMCPServersJSONFile(FIFO) blocked instead of rejecting the special file")
		}
	})
}
