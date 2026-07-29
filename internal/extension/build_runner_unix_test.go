//go:build !windows

package extensionpkg

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/procutil"
	"github.com/compozy/compozy/internal/testutil"
)

func TestOSBuildCommandRunnerCancellation(t *testing.T) {
	t.Parallel()

	t.Run("Should stop the complete process group when canceled", func(t *testing.T) {
		t.Parallel()

		directory := t.TempDir()
		pidPath := filepath.Join(directory, "child.pid")
		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			_, err := (osBuildCommandRunner{}).Run(ctx, directory, []string{
				"sh",
				"-c",
				"sleep 30 & child=$!; printf '%s' \"$child\" > child.pid; wait",
			})
			errCh <- err
		}()

		pid := waitForBuildRunnerChildPID(t, pidPath)
		cancel()
		select {
		case err := <-errCh:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("Run() error = %v, want context.Canceled", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Run() did not return after cancellation")
		}
		if procutil.Alive(pid) {
			t.Fatalf("child process %d is still alive after cancellation", pid)
		}
	})
}

func waitForBuildRunnerChildPID(t *testing.T, path string) int {
	t.Helper()

	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data)))
			if parseErr != nil {
				t.Fatalf("parse child pid %q: %v", data, parseErr)
			}
			return pid
		}
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("read child pid: %v", err)
		}
		select {
		case <-deadline.C:
			t.Fatal("child process did not publish its pid")
		case <-ticker.C:
		}
	}
}
