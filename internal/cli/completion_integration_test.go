package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func assertCompletionStderrClean(t *testing.T, stderr string) {
	t.Helper()

	var unexpected []string
	for line := range strings.SplitSeq(stderr, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "Completion ended with directive:") {
			continue
		}
		unexpected = append(unexpected, line)
	}
	if len(unexpected) > 0 {
		t.Fatalf("unexpected completion stderr:\n%s", strings.Join(unexpected, "\n"))
	}
}

func TestCompletionIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should complete session inspect IDs with NoFileComp directive and exit 0", func(t *testing.T) {
		t.Parallel()

		stub := &stubClient{
			listSessionsFn: func(context.Context, SessionListQuery) ([]SessionRecord, error) {
				return []SessionRecord{
					{ID: "sess-123", Name: "Inspect task"},
					{ID: "other-999", Name: "Ignored"},
				}, nil
			},
		}
		code, stdout, stderr := executeRootCommandWithExit(
			t,
			newTestDeps(t, stub),
			"__complete", "session", "inspect", "sess-",
		)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr=%q)", code, stderr)
		}
		assertCompletionStderrClean(t, stderr)
		if !strings.Contains(stdout, "sess-123\tInspect task\n:4\n") {
			t.Fatalf("stdout = %q, want sess-123 candidate and :4 directive", stdout)
		}
		if strings.Contains(stdout, "other-999") {
			t.Fatalf("stdout = %q, should filter out non-matching prefix", stdout)
		}
	})

	t.Run("Should return no candidates, exit 0, and silence retired config warning on client failure", func(t *testing.T) {
		t.Parallel()

		homeDir := t.TempDir()
		if err := os.WriteFile(
			filepath.Join(homeDir, "config.toml"),
			[]byte("window_manager = { enabled = true }\n"),
			0o600,
		); err != nil {
			t.Fatalf("WriteFile(config.toml) error = %v", err)
		}
		deps := commandDeps{
			getenv: func(key string) string {
				if key == "COMPOZY_HOME" {
					return homeDir
				}
				return ""
			},
			newClient: func(ClientTarget) (DaemonClient, error) {
				return nil, errors.New("daemon offline")
			},
		}
		code, stdout, stderr := executeRootCommandWithExit(
			t,
			deps,
			"__complete", "session", "inspect", "sess-",
		)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		assertCompletionStderrClean(t, stderr)
		if stdout != ":4\n" {
			t.Fatalf("stdout = %q, want only :4", stdout)
		}
	})

	t.Run("Should complete --workspace flag with workspace candidates", func(t *testing.T) {
		t.Parallel()

		stub := &stubClient{
			listWorkspacesFn: func(context.Context) ([]WorkspaceRecord, error) {
				return []WorkspaceRecord{
					{ID: "ws_1", Name: "compozy", RootDir: "/repo/compozy"},
				}, nil
			},
		}
		code, stdout, stderr := executeRootCommandWithExit(
			t,
			newTestDeps(t, stub),
			"__complete", "session", "list", "--workspace", "",
		)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		assertCompletionStderrClean(t, stderr)
		if !strings.Contains(stdout, "compozy\t/repo/compozy\n:4\n") {
			t.Fatalf("stdout = %q, want workspace candidate + :4", stdout)
		}
	})

	t.Run("Should keep default file directive for workspace add", func(t *testing.T) {
		t.Parallel()

		code, stdout, stderr := executeRootCommandWithExit(
			t,
			newTestDeps(t, &stubClient{}),
			"__complete", "workspace", "add", "",
		)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		assertCompletionStderrClean(t, stderr)
		if stdout != ":0\n" {
			t.Fatalf("stdout = %q, want :0 default file directive", stdout)
		}
	})

	t.Run("Should preserve static ValidArgs for daemon profile", func(t *testing.T) {
		t.Parallel()

		code, stdout, stderr := executeRootCommandWithExit(
			t,
			newTestDeps(t, &stubClient{}),
			"__complete", "daemon", "profile", "",
		)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0", code)
		}
		assertCompletionStderrClean(t, stderr)
		for _, want := range []string{"cpu\n", "heap\n", ":4\n"} {
			if !strings.Contains(stdout, want) {
				t.Fatalf("stdout = %q, missing %q", stdout, want)
			}
		}
	})
}
