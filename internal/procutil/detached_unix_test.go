//go:build !windows

package procutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// not parallel: mutates the detached launch hook.
func TestSpawnDetachedLoggedProcessCreatesIndependentSession(t *testing.T) {
	t.Run("Should leave the parent session before running the daemon", func(t *testing.T) {
		startErr := errors.New("captured launch")
		originalStart := startDetachedProcess
		startDetachedProcess = func(_ string, _ []string, attr *os.ProcAttr) (*os.Process, error) {
			if attr == nil || attr.Sys == nil || !attr.Sys.Setsid {
				t.Fatalf("detached SysProcAttr = %#v, want Setsid", attr)
			}
			if attr.Sys.Setpgid {
				t.Fatalf("detached SysProcAttr = %#v, want Setsid without Setpgid", attr.Sys)
			}
			return nil, startErr
		}
		t.Cleanup(func() { startDetachedProcess = originalStart })

		process, err := SpawnDetachedLoggedProcess(context.Background(), DetachedLaunchRequest{
			Binary:  os.Args[0],
			LogPath: filepath.Join(t.TempDir(), "detached.log"),
		})
		if process != nil || !errors.Is(err, startErr) {
			t.Fatalf("SpawnDetachedLoggedProcess() = %#v, %v, want captured launch error", process, err)
		}
	})
}
