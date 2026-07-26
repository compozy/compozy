package config

import (
	"os"
	"testing"

	"github.com/compozy/agh/internal/testutil"
)

func TestHermeticEnvShieldsConfigAndHomeLoads(t *testing.T) {
	t.Run("Should scrub operator environment before config and home resolution", func(t *testing.T) {
		setConfigTestEnv(t, "OPENAI_API_KEY", "sk-operator")
		setConfigTestEnv(t, "AGH_LOG_LEVEL", "debug")

		hermetic := testutil.ApplyHermeticEnv(t)
		for _, key := range []string{"OPENAI_API_KEY", "AGH_LOG_LEVEL"} {
			if value, ok := os.LookupEnv(key); ok {
				t.Fatalf("%s = %q, want scrubbed before config load", key, value)
			}
		}

		homePaths, err := ResolveHomePaths()
		if err != nil {
			t.Fatalf("ResolveHomePaths() error = %v", err)
		}
		if got, want := homePaths.HomeDir, hermetic.HomeDir; got != want {
			t.Fatalf("ResolveHomePaths() HomeDir = %q, want hermetic AGH_HOME %q", got, want)
		}
		if err := EnsureHomeLayout(homePaths); err != nil {
			t.Fatalf("EnsureHomeLayout() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, "\n[defaults]\nagent = \"hermetic-agent\"\nprovider = \"claude\"\n")

		cfg, err := Load()
		if err != nil {
			t.Fatalf("Load() error = %v", err)
		}
		if got, want := cfg.Defaults.Agent, "hermetic-agent"; got != want {
			t.Fatalf("Load() Defaults.Agent = %q, want %q", got, want)
		}
		if got, want := os.Getenv("TZ"), "UTC"; got != want {
			t.Fatalf("TZ = %q, want %q", got, want)
		}
		if got, want := os.Getenv("LANG"), "C.UTF-8"; got != want {
			t.Fatalf("LANG = %q, want %q", got, want)
		}
	})
}

func setConfigTestEnv(t *testing.T, key string, value string) {
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
