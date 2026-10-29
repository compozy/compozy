package procutil

import (
	"context"
	"errors"
	"os"
	"os/exec"
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

func TestFilteredDaemonEnvAppImage(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		base []string
		want []string
	}{
		{"Should remove runtime variables and current and stale mount libraries", []string{
			"PATH=/usr/bin", "ARGV0=/apps/Compozy.AppImage", "OWD=/home/user",
			"LD_LIBRARY_PATH=/tmp/.mount_current/usr/lib:/opt/lib:/tmp/.mount_stale/usr/lib",
			"APPDIR=/tmp/.mount_current", "APPIMAGE=/apps/Compozy.AppImage",
		}, []string{"PATH=/usr/bin", "LD_LIBRARY_PATH=/opt/lib"}},
		{"Should remove an extracted AppDir without matching sibling paths", []string{
			"APPIMAGE=/apps/Compozy.AppImage", "APPDIR=/opt/Compozy/",
			"LD_LIBRARY_PATH=/opt/Compozy:/opt/Compozy/usr/lib;/opt/Compozy-other/lib:/usr/lib",
		}, []string{"LD_LIBRARY_PATH=/opt/Compozy-other/lib:/usr/lib"}},
		{"Should unset an exhausted library path", []string{
			"APPIMAGE=/apps/Compozy.AppImage", "LD_LIBRARY_PATH=/tmp/.mount_old/usr/lib",
		}, []string{}},
		{"Should preserve empty search components alongside host libraries", []string{
			"APPIMAGE=/apps/Compozy.AppImage", "LD_LIBRARY_PATH=:/usr/lib:/tmp/.mount_old/lib:",
		}, []string{"LD_LIBRARY_PATH=:/usr/lib:"}},
		{"Should leave non AppImage environments unchanged", []string{
			"ARGV0=custom", "APPDIR=/opt/custom", "OWD=/work", "LD_LIBRARY_PATH=/tmp/.mount_old/lib:/usr/lib",
		}, []string{"ARGV0=custom", "APPDIR=/opt/custom", "OWD=/work", "LD_LIBRARY_PATH=/tmp/.mount_old/lib:/usr/lib"}},
		{"Should leave an empty AppImage marker unchanged", []string{
			"APPIMAGE=", "ARGV0=custom", "LD_LIBRARY_PATH=/tmp/.mount_old/lib",
		}, []string{"APPIMAGE=", "ARGV0=custom", "LD_LIBRARY_PATH=/tmp/.mount_old/lib"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			original := slices.Clone(tc.base)
			got := FilteredDaemonEnv(tc.base)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("FilteredDaemonEnv() = %q, want %q", got, tc.want)
			}
			if !slices.Equal(tc.base, original) {
				t.Fatal("FilteredDaemonEnv mutated its input")
			}
		})
	}
}

func TestFilteredDaemonEnvAppImageChild(t *testing.T) {
	t.Run("Should launch a real child without AppImage runtime state", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("Unix shell process check")
		}
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "/bin/sh", "-c", `
for name in APPIMAGE APPDIR ARGV0 OWD; do
 eval 'present=${'"$name"'+yes}'
 if [ "$present" = yes ]; then exit 1; fi
done
[ "${LD_LIBRARY_PATH+x}" != x ] || exit 2
printf clean
`)
		cmd.Env = FilteredDaemonEnv([]string{
			"PATH=/usr/bin:/bin", "APPIMAGE=/apps/Compozy.AppImage",
			"APPDIR=/tmp/.mount_current", "ARGV0=/apps/Compozy.AppImage", "OWD=/work",
			"LD_LIBRARY_PATH=/tmp/.mount_current/usr/lib:/tmp/.mount_stale/usr/lib",
		})
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("child failed: %v; output: %s", err, output)
		}
		if string(output) != "clean" {
			t.Fatalf("child output = %q, want clean", output)
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
		}, LoginShellPathOptions{Timeout: time.Second, MaxOutputBytes: 1024, AccountShell: noAccountShell})
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

	t.Run("Should use the account login shell when SHELL cannot run", func(t *testing.T) {
		account := writeExecutableShell(t, "#!/bin/sh\nprintf '\\000/account/bin\\000'\n")
		notExecutable := filepath.Join(t.TempDir(), "bash")
		if err := os.WriteFile(notExecutable, []byte("#!/bin/sh\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		for name, inherited := range map[string]string{
			"Should skip a relative SHELL":       "bash",
			"Should skip a non-executable SHELL": notExecutable,
		} {
			t.Run(name, func(t *testing.T) {
				got, err := ResolveLoginShellPath(t.Context(), []string{
					"HOME=" + t.TempDir(),
					"SHELL=" + inherited,
				}, LoginShellPathOptions{
					Timeout: time.Second, MaxOutputBytes: 1024,
					AccountShell: func(context.Context) (string, error) { return account, nil },
				})
				if err != nil {
					t.Fatalf("ResolveLoginShellPath() error = %v", err)
				}
				if got != "/account/bin" {
					t.Fatalf("ResolveLoginShellPath() = %q, want the account shell PATH", got)
				}
			})
		}
	})

	t.Run("Should reject an empty login shell PATH", func(t *testing.T) {
		shell := writeExecutableShell(t, "#!/bin/sh\nprintf '\\000\\000'\n")
		got, err := ResolveLoginShellPath(t.Context(), []string{
			"HOME=" + t.TempDir(),
			"PATH=/gui/bin",
			"SHELL=" + shell,
		}, LoginShellPathOptions{Timeout: time.Second, MaxOutputBytes: 1024, AccountShell: noAccountShell})
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
		}, LoginShellPathOptions{Timeout: time.Second, MaxOutputBytes: 32, AccountShell: noAccountShell})
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
		}, LoginShellPathOptions{
			Timeout: 20 * time.Millisecond, MaxOutputBytes: 1024, AccountShell: noAccountShell,
		})
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

func noAccountShell(context.Context) (string, error) { return "", ErrAccountShellUnavailable }

func TestAccountLoginShell(t *testing.T) {
	t.Run("Should read the shell of the matching directory services record", func(t *testing.T) {
		output := []byte("name: other\nuid: 500\nshell: /bin/zsh\n\nname: operator\nuid: 501\n" +
			"dir: /Users/operator\nshell: /opt/homebrew/bin/fish\n\n")
		got, err := parseDirectoryServicesShell(output, 501)
		if err != nil || got != "/opt/homebrew/bin/fish" {
			t.Fatalf("parseDirectoryServicesShell() = %q, %v; want fish", got, err)
		}
		if _, err := parseDirectoryServicesShell(output, 502); !errors.Is(err, ErrAccountShellUnavailable) {
			t.Fatalf("parseDirectoryServicesShell(unknown uid) error = %v, want unavailable", err)
		}
	})

	t.Run("Should read the shell field of the matching passwd entry", func(t *testing.T) {
		passwd := []byte("# comment\nroot:x:0:0:root:/root:/bin/bash\n" +
			"operator:x:1000:1000:Operator:/home/operator:/usr/bin/fish\n")
		got, err := parsePasswdShell(passwd, 1000)
		if err != nil || got != "/usr/bin/fish" {
			t.Fatalf("parsePasswdShell() = %q, %v; want fish", got, err)
		}
		if _, err := parsePasswdShell(passwd, 1001); !errors.Is(err, ErrAccountShellUnavailable) {
			t.Fatalf("parsePasswdShell(unknown uid) error = %v, want unavailable", err)
		}
	})

	t.Run("Should reject recorded shells that cannot run", func(t *testing.T) {
		notExecutable := filepath.Join(t.TempDir(), "fish")
		if err := os.WriteFile(notExecutable, []byte("#!/bin/sh\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		for name, shell := range map[string]string{
			"Should reject an empty shell":        "",
			"Should reject a relative shell":      "fish",
			"Should reject a missing shell":       "/definitely/not/fish",
			"Should reject a non-executable file": notExecutable,
			"Should reject a directory":           t.TempDir(),
		} {
			t.Run(name, func(t *testing.T) {
				if _, err := validateAccountShell(t.Context(), shell); !errors.Is(err, ErrAccountShellUnavailable) {
					t.Fatalf("validateAccountShell(%q) error = %v, want unavailable", shell, err)
				}
			})
		}
	})

	t.Run("Should resolve the current account shell from the host database", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			if _, err := AccountLoginShell(t.Context()); !errors.Is(err, ErrAccountShellUnavailable) {
				t.Fatalf("AccountLoginShell() error = %v, want unavailable on Windows", err)
			}
			return
		}
		shell, err := AccountLoginShell(t.Context())
		if errors.Is(err, ErrAccountShellUnavailable) {
			t.Skipf("host account records no usable shell: %v", err)
		}
		if err != nil || !filepath.IsAbs(shell) {
			t.Fatalf("AccountLoginShell() = %q, %v; want an absolute executable", shell, err)
		}
	})
}

func writeExecutableShell(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "login-shell")
	if err := os.WriteFile(path, []byte(contents), 0o700); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
	return path
}
