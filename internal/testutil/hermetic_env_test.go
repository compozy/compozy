package testutil

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestApplyHermeticEnv(t *testing.T) {
	t.Run("Should scrub credentials and pin deterministic runtime variables", func(t *testing.T) {
		setTestEnv(t, "OPENAI_API_KEY", "sk-ambient")
		setTestEnv(t, "AGH_LOG_LEVEL", "debug")
		setTestEnv(t, "PROVIDER_HOME", filepath.Join(t.TempDir(), "operator-provider-home"))
		setTestEnv(t, "AGH_TEST_ACPMOCK_DRIVER_BIN", "/tmp/agh-test-driver")
		originalHome, hadHome := os.LookupEnv("HOME")

		state := ApplyHermeticEnv(t)

		for _, key := range []string{"OPENAI_API_KEY", "AGH_LOG_LEVEL"} {
			if value, ok := os.LookupEnv(key); ok {
				t.Fatalf("%s = %q, want unset by hermetic test environment", key, value)
			}
		}
		if got, want := os.Getenv("AGH_HOME"), state.HomeDir; got != want {
			t.Fatalf("AGH_HOME = %q, want %q", got, want)
		}
		if got, want := os.Getenv("PROVIDER_HOME"), state.ProviderHomeDir; got != want {
			t.Fatalf("PROVIDER_HOME = %q, want %q", got, want)
		}
		if got, want := os.Getenv("TZ"), hermeticTimezone; got != want {
			t.Fatalf("TZ = %q, want %q", got, want)
		}
		if got, want := os.Getenv("LANG"), hermeticLocale; got != want {
			t.Fatalf("LANG = %q, want %q", got, want)
		}
		if got, want := os.Getenv("AGH_TEST_ACPMOCK_DRIVER_BIN"), "/tmp/agh-test-driver"; got != want {
			t.Fatalf("AGH_TEST_ACPMOCK_DRIVER_BIN = %q, want %q", got, want)
		}
		if hadHome {
			if got := os.Getenv("HOME"); got != originalHome {
				t.Fatalf("HOME = %q, want preserved operator home %q", got, originalHome)
			}
		}
	})
}

func TestHermeticProcessEnv(t *testing.T) {
	t.Parallel()

	t.Run("Should filter credentials and preserve operational test variables", func(t *testing.T) {
		t.Parallel()

		env := HermeticProcessEnv([]string{
			"PATH=/usr/bin",
			"HOME=/Users/operator",
			"OPENAI_API_KEY=sk-ambient",
			"AGH_HOME=/Users/operator/.agh",
			"AGH_TEST_DAEMON_BIN=/tmp/agh",
			"PROVIDER_CODEX_HOME=/Users/operator/.codex",
			"TZ=America/Sao_Paulo",
			"LANG=pt_BR.UTF-8",
			"LC_ALL=pt_BR.UTF-8",
		})

		for _, key := range []string{"OPENAI_API_KEY", "AGH_HOME", "PROVIDER_CODEX_HOME"} {
			if value, ok := lookupEnvEntry(env, key); ok {
				t.Fatalf("%s = %q, want filtered out of hermetic process env", key, value)
			}
		}
		for _, entry := range []string{
			"PATH=/usr/bin",
			"HOME=/Users/operator",
			"AGH_TEST_DAEMON_BIN=/tmp/agh",
			"TZ=UTC",
			"LANG=C.UTF-8",
			"LC_ALL=C.UTF-8",
			"LC_CTYPE=C.UTF-8",
		} {
			if !slices.Contains(env, entry) {
				t.Fatalf("HermeticProcessEnv() missing %q in %#v", entry, env)
			}
		}
	})
}

func TestApplyHermeticEnvRestoresOriginalValues(t *testing.T) {
	const key = "AGH_HERMETIC_TEST_TOKEN"
	setTestEnv(t, key, "original")

	t.Run("Should restore the caller environment after cleanup", func(t *testing.T) {
		ApplyHermeticEnv(t)
		if value, ok := os.LookupEnv(key); ok {
			t.Fatalf("%s = %q, want unset inside hermetic environment", key, value)
		}
	})

	if got, want := os.Getenv(key), "original"; got != want {
		t.Fatalf("%s after cleanup = %q, want %q", key, got, want)
	}
}

func setTestEnv(t *testing.T, key string, value string) {
	t.Helper()

	original, hadOriginal := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Setenv(%q) error = %v", key, err)
	}
	t.Cleanup(func() {
		var err error
		if hadOriginal {
			err = os.Setenv(key, original)
		} else {
			err = os.Unsetenv(key)
		}
		if err != nil {
			t.Fatalf("restore env %q error = %v", key, err)
		}
	})
}

func lookupEnvEntry(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, entry := range env {
		if value, ok := strings.CutPrefix(entry, prefix); ok {
			return value, true
		}
	}
	return "", false
}
