package udsapi

import (
	"path/filepath"
	"testing"
)

func TestNewWithHomePathsRealignsDefaultConfig(t *testing.T) {
	// not parallel: t.Setenv mutates process environment for the test process.
	t.Run("Should use overridden home paths for the default daemon socket", func(t *testing.T) {
		processHome := filepath.Join(t.TempDir(), "process-home")
		t.Setenv("AGH_HOME", processHome)
		homePaths := newTestHomePaths(t)
		socketPath := shortSocketPath(t)

		server, err := New(
			WithHomePaths(homePaths),
			WithSocketPath(socketPath),
			WithSessionManager(stubSessionManager{}),
			WithTaskService(&stubTaskManager{}),
			WithObserver(stubObserver{}),
			WithWorkspaceResolver(stubWorkspaceService{}),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if got, want := server.config.Daemon.Socket, homePaths.DaemonSocket; got != want {
			t.Fatalf("config daemon socket = %q, want %q", got, want)
		}
		if got, want := server.Path(), socketPath; got != want {
			t.Fatalf("Path() = %q, want %q", got, want)
		}
	})
}
