package procutil

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestFilteredDaemonEnvRemovesCredentialShapedVariables(t *testing.T) {
	t.Run("Should keep operational variables and drop secrets", func(t *testing.T) {
		env := FilteredDaemonEnv([]string{
			"PATH=/usr/bin",
			"HOME=/home/compozy",
			"COMPOZY_HOME=/tmp/compozy",
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
			"HOME=/home/compozy",
			"COMPOZY_HOME=/tmp/compozy",
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
	t.Setenv("COMPOZY_HOME", "/tmp/compozy")

	t.Run("Should filter inherited daemon secrets", func(t *testing.T) {
		env := launchSandbox(nil)
		if hasEnvPrefix(env, "OPENAI_API_KEY=") {
			t.Fatalf("launchSandbox(nil) leaked OPENAI_API_KEY in %#v", env)
		}
		if !hasEnvPrefix(env, "COMPOZY_HOME=/tmp/compozy") {
			t.Fatalf("launchSandbox(nil) missing COMPOZY_HOME in %#v", env)
		}
	})
}

func TestResolveLoginShellPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("login shell PATH resolution is not used on Windows")
	}

	t.Run("Should return only the PATH emitted by the operator login shell", func(t *testing.T) {
		home := t.TempDir()
		shell := writeExecutableShell(t, `#!/bin/sh
if [ "$DISABLE_AUTO_UPDATE" != "true" ]; then exit 31; fi
: > login-shell-cwd-marker
printf 'startup noise\000/login/bin:/usr/bin\000trailing noise'
`)
		got, err := ResolveLoginShellPath(t.Context(), []string{
			"HOME=" + home,
			"PATH=/gui/bin",
			"SHELL=" + shell,
		}, LoginShellPathOptions{Timeout: time.Second, MaxOutputBytes: 1024})
		if err != nil {
			t.Fatalf("ResolveLoginShellPath() error = %v", err)
		}
		if got != "/login/bin:/usr/bin" {
			t.Fatalf("ResolveLoginShellPath() = %q, want login shell PATH", got)
		}
		if _, err := os.Stat(filepath.Join(home, "login-shell-cwd-marker")); err != nil {
			t.Fatalf("login shell working directory marker: %v", err)
		}
	})

	t.Run("Should reject an empty login shell PATH", func(t *testing.T) {
		shell := writeExecutableShell(t, "#!/bin/sh\nprintf '\\000\\000'\n")
		got, err := ResolveLoginShellPath(t.Context(), []string{
			"HOME=" + t.TempDir(),
			"PATH=/gui/bin",
			"SHELL=" + shell,
		}, LoginShellPathOptions{Timeout: time.Second, MaxOutputBytes: 1024})
		if err == nil {
			t.Fatal("ResolveLoginShellPath() error = nil, want empty PATH error")
		}
		if got != "" {
			t.Fatalf("ResolveLoginShellPath() = %q, want no PATH on error", got)
		}
	})

	t.Run("Should reject shell output beyond the configured bound", func(t *testing.T) {
		shell := writeExecutableShell(
			t,
			"#!/bin/sh\nprintf '\\000%s\\000' '"+strings.Repeat("x", 64)+"'\n",
		)
		got, err := ResolveLoginShellPath(t.Context(), []string{
			"HOME=" + t.TempDir(),
			"PATH=/gui/bin",
			"SHELL=" + shell,
		}, LoginShellPathOptions{Timeout: time.Second, MaxOutputBytes: 32})
		if !errors.Is(err, errLoginShellPathOutputTooLarge) {
			t.Fatalf("ResolveLoginShellPath() error = %v, want output limit error", err)
		}
		if got != "" {
			t.Fatalf("ResolveLoginShellPath() = %q, want no PATH on error", got)
		}
	})

	t.Run("Should stop the login shell when the deadline expires", func(t *testing.T) {
		shell := writeExecutableShell(t, "#!/bin/sh\nwhile :; do :; done\n")
		got, err := ResolveLoginShellPath(t.Context(), []string{
			"HOME=" + t.TempDir(),
			"PATH=/gui/bin",
			"SHELL=" + shell,
		}, LoginShellPathOptions{Timeout: 20 * time.Millisecond, MaxOutputBytes: 1024})
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("ResolveLoginShellPath() error = %v, want deadline exceeded", err)
		}
		if got != "" {
			t.Fatalf("ResolveLoginShellPath() = %q, want no PATH on error", got)
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

func writeExecutableShell(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "login-shell")
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	return path
}
