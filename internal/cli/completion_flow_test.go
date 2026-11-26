package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	compozyconfig "github.com/compozy/compozy/internal/config"
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

	t.Run(
		"Should return no candidates, exit 0, and keep stderr clean on client failure",
		func(t *testing.T) {
			t.Parallel()

			deps := commandDeps{
				getenv: func(string) string { return "" },
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
		},
	)

	t.Run(
		"Should suppress retired config warnings through the completion loader",
		func(t *testing.T) {
			t.Parallel()

			homeDir := t.TempDir()
			if err := os.WriteFile(
				filepath.Join(homeDir, "config.toml"),
				[]byte("[window_manager]\nnew_window_policy = \"floating\"\n"),
				0o600,
			); err != nil {
				t.Fatalf("WriteFile(config.toml) error = %v", err)
			}
			homePaths, err := compozyconfig.ResolveHomePathsFrom(homeDir)
			if err != nil {
				t.Fatalf("ResolveHomePathsFrom() error = %v", err)
			}

			warnings := 0
			warning := compozyconfig.WithWindowManagerWarning(func(string, bool) { warnings++ })
			if _, err := compozyconfig.LoadForHome(homePaths, warning); err != nil {
				t.Fatalf("baseline LoadForHome() error = %v", err)
			}
			if warnings == 0 {
				t.Fatal("baseline config load did not emit the retired warning")
			}

			deps := commandDeps{
				defaultLoadConfig:    true,
				windowManagerWarning: warning,
				resolveHome: func() (compozyconfig.HomePaths, error) {
					return homePaths, nil
				},
				ensureHome: func(compozyconfig.HomePaths) error { return nil },
				getwd:      func() (string, error) { return homeDir, nil },
				getenv:     func(string) string { return "" },
				newClient: func(ClientTarget) (DaemonClient, error) {
					return nil, errors.New("daemon offline")
				},
			}
			completionDeps := completionCommandDeps(t.Context(), deps, nil)
			before := warnings
			if _, err := completionDeps.loadConfig(); err != nil {
				t.Fatalf("completion loadConfig() error = %v", err)
			}
			if warnings != before {
				t.Fatalf("completion loader emitted %d retired warnings", warnings-before)
			}
		},
	)

	t.Run("Should apply --profile to completion requests", func(t *testing.T) {
		t.Parallel()

		var gotProfile string
		stub := &stubClient{
			listSessionsFn: func(ctx context.Context, _ SessionListQuery) ([]SessionRecord, error) {
				if selection, ok := ctx.Value(profileReadSelectionContextKey{}).(profileReadSelection); ok {
					gotProfile = selection.Profile
				}
				return []SessionRecord{{ID: "sess-work", Name: "Work session"}}, nil
			},
		}
		deps := newTestDeps(t, stub)
		deps.newClient = func(ClientTarget) (DaemonClient, error) { return stub, nil }
		code, stdout, stderr := executeRootCommandWithExit(
			t,
			deps,
			"__complete", "session", "inspect", "--profile", "work", "",
		)
		if code != 0 {
			t.Fatalf("exit code = %d, want 0 (stderr=%q)", code, stderr)
		}
		if gotProfile != "work" {
			t.Fatalf("completion profile = %q, want %q", gotProfile, "work")
		}
		if !strings.Contains(stdout, "sess-work") {
			t.Fatalf("stdout = %q, want sess-work candidate", stdout)
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
