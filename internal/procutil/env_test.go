package procutil

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestFilteredDaemonEnvRemovesCredentialShapedVariables(t *testing.T) {
	t.Run("Should keep operational variables and drop secrets", func(t *testing.T) {
		env := FilteredDaemonEnv([]string{
			"PATH=/usr/bin",
			"HOME=/home/agh",
			"AGH_HOME=/tmp/agh",
			"PROVIDER_CODEX_HOME=/tmp/provider",
			"OPENAI_API_KEY=sk-secret",
			"GITHUB_TOKEN=ghp-secret",
			"SESSION_MANAGER=local/session",
			"CLIENT_SECRET=client-secret",
			"MALFORMED",
		})

		for _, leaked := range []string{
			"OPENAI_API_KEY=sk-secret",
			"GITHUB_TOKEN=ghp-secret",
			"SESSION_MANAGER=local/session",
			"CLIENT_SECRET=client-secret",
			"MALFORMED",
		} {
			if containsEnvEntry(env, leaked) {
				t.Fatalf("FilteredDaemonEnv() leaked %q in %#v", leaked, env)
			}
		}
		for _, kept := range []string{
			"PATH=/usr/bin",
			"HOME=/home/agh",
			"AGH_HOME=/tmp/agh",
			"PROVIDER_CODEX_HOME=/tmp/provider",
		} {
			if !containsEnvEntry(env, kept) {
				t.Fatalf("FilteredDaemonEnv() missing %q in %#v", kept, env)
			}
		}
	})

	t.Run("Should drop credential shaped operational prefix variables", func(t *testing.T) {
		env := []string{
			"PATH=/usr/bin",
			"XDG_RUNTIME_DIR=/run/user/1000",
			"LC_CTYPE=UTF-8",
			"XDG_TOKEN=secret",
			"XDG_CLIENT_SECRET=secret",
			"XDG_SESSION_COOKIE=secret",
			"LC_API_KEY=secret",
		}

		filtered := FilteredDaemonEnv(env)
		isolated := IsolatedDaemonEnv(env)
		for _, leaked := range []string{
			"XDG_TOKEN=secret",
			"XDG_CLIENT_SECRET=secret",
			"XDG_SESSION_COOKIE=secret",
			"LC_API_KEY=secret",
		} {
			if containsEnvEntry(filtered, leaked) {
				t.Fatalf("FilteredDaemonEnv() leaked %q in %#v", leaked, filtered)
			}
			if containsEnvEntry(isolated, leaked) {
				t.Fatalf("IsolatedDaemonEnv() leaked %q in %#v", leaked, isolated)
			}
		}
		for _, kept := range []string{
			"PATH=/usr/bin",
			"XDG_RUNTIME_DIR=/run/user/1000",
			"LC_CTYPE=UTF-8",
		} {
			if !containsEnvEntry(filtered, kept) {
				t.Fatalf("FilteredDaemonEnv() missing %q in %#v", kept, filtered)
			}
			if !containsEnvEntry(isolated, kept) {
				t.Fatalf("IsolatedDaemonEnv() missing %q in %#v", kept, isolated)
			}
		}
	})
}

// not parallel: mutates process environment with t.Setenv.
func TestLaunchSandboxFiltersFallbackEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-launch-secret")
	t.Setenv("AGH_HOME", "/tmp/agh")

	t.Run("Should filter inherited daemon secrets", func(t *testing.T) {
		env := launchSandbox(nil)
		if hasEnvPrefix(env, "OPENAI_API_KEY=") {
			t.Fatalf("launchSandbox(nil) leaked OPENAI_API_KEY in %#v", env)
		}
		if !hasEnvPrefix(env, "AGH_HOME=/tmp/agh") {
			t.Fatalf("launchSandbox(nil) missing AGH_HOME in %#v", env)
		}
	})
}

func TestAttachCommandLogRedactsRecentError(t *testing.T) {
	t.Run("Should redact token shaped stderr before wrapping", func(t *testing.T) {
		logPath := filepath.Join(t.TempDir(), "command.log")
		if err := os.WriteFile(logPath, []byte("info\nerror: token=super-secret\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", logPath, err)
		}

		err := attachCommandLog(errors.New("launch failed"), logPath, 0)
		if err == nil {
			t.Fatal("attachCommandLog() error = nil, want wrapped error")
		}
		if strings.Contains(err.Error(), "super-secret") {
			t.Fatalf("attachCommandLog() = %v, want redacted secret", err)
		}
		if !strings.Contains(err.Error(), "token=[REDACTED]") {
			t.Fatalf("attachCommandLog() = %v, want redacted token marker", err)
		}
	})

	t.Run("Should read only a bounded tail from large logs", func(t *testing.T) {
		logPath := filepath.Join(t.TempDir(), "command.log")
		oldPrefix := "old prefix that must not be read\n"
		recentError := "error: recent failure\n"
		body := oldPrefix + strings.Repeat("noisy log line\n", maxDetachedCommandErrorBytes*4) + recentError
		if err := os.WriteFile(logPath, []byte(body), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", logPath, err)
		}

		text, err := readCommandLog(logPath, 0)
		if err != nil {
			t.Fatalf("readCommandLog() error = %v", err)
		}
		if len(text) > maxDetachedCommandErrorBytes*8 {
			t.Fatalf("len(readCommandLog()) = %d, want bounded tail", len(text))
		}
		if strings.Contains(text, oldPrefix) {
			t.Fatalf("readCommandLog() included old prefix in bounded tail")
		}
		if !strings.Contains(text, strings.TrimSpace(recentError)) {
			t.Fatalf("readCommandLog() = %q, want recent error", text)
		}
	})
}

func containsEnvEntry(env []string, target string) bool {
	return slices.Contains(env, target)
}

func hasEnvPrefix(env []string, prefix string) bool {
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			return true
		}
	}
	return false
}
