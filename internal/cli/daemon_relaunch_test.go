package cli

import (
	"context"
	"strings"
	"testing"

	aghdaemon "github.com/compozy/agh/internal/daemon"
)

func TestDaemonRelaunchCommandInvokesHelper(t *testing.T) {
	// not parallel: relaunch reads process environment through os.Getenv.
	t.Run("Should invoke relaunch helper with restart operation environment", func(t *testing.T) {
		deps := newTestDeps(t, &stubClient{})
		deps.executable = func() (string, error) { return "/usr/bin/agh", nil }

		var captured aghdaemon.RelaunchHelperConfig
		deps.runRelaunchHelper = func(_ context.Context, cfg aghdaemon.RelaunchHelperConfig) error {
			captured = cfg
			return nil
		}

		t.Setenv(aghdaemon.RestartOperationEnvKey, "restart-op-123")

		if _, _, err := executeRootCommand(t, deps, "daemon", "relaunch"); err != nil {
			t.Fatalf("executeRootCommand() error = %v", err)
		}
		if got, want := strings.TrimSpace(captured.OperationID), "restart-op-123"; got != want {
			t.Fatalf("captured.OperationID = %q, want %q", got, want)
		}
		homePaths, err := deps.resolveHome()
		if err != nil {
			t.Fatalf("deps.resolveHome() error = %v", err)
		}
		if got, want := captured.HomePaths.HomeDir, homePaths.HomeDir; got != want {
			t.Fatalf("captured.HomePaths.HomeDir = %q, want %q", got, want)
		}
		if captured.Executable == nil {
			t.Fatal("captured.Executable = nil, want forwarded executable resolver")
		}
	})
}
