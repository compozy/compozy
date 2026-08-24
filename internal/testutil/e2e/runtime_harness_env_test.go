package e2e

import "testing"

func TestRuntimeHarnessEnvContract(t *testing.T) {
	t.Run("Should keep isolated home env when options provide reserved keys", func(t *testing.T) {
		layout := prepareRuntimeLayout(t, &RuntimeHarnessOptions{
			Env: map[string]string{
				"COMPOZY_HOME": "/tmp/outside-compozy-home",
				"HOME":         "/tmp/outside-home",
			},
		})

		if got, want := lookupEnvValue(layout.Env, "HOME"), layout.OperatorHomeDir; got != want {
			t.Fatalf("lookupEnvValue(HOME) = %q, want %q", got, want)
		}
		if layout.OperatorHomeDir == layout.HomePaths.HomeDir {
			t.Fatalf("OperatorHomeDir = %q, want distinct COMPOZY_HOME", layout.OperatorHomeDir)
		}
		if got, want := lookupEnvValue(layout.Env, "COMPOZY_HOME"), layout.HomePaths.HomeDir; got != want {
			t.Fatalf("lookupEnvValue(COMPOZY_HOME) = %q, want %q", got, want)
		}
		if got, want := countEnvEntries(layout.Env, "HOME"), 1; got != want {
			t.Fatalf("countEnvEntries(HOME) = %d, want %d", got, want)
		}
		if got, want := countEnvEntries(layout.Env, "COMPOZY_HOME"), 1; got != want {
			t.Fatalf("countEnvEntries(COMPOZY_HOME) = %d, want %d", got, want)
		}
	})

	t.Run("Should override caller home state with isolated paths", func(t *testing.T) {
		// not parallel: this case verifies process environment isolation.
		t.Setenv("HOME", "/tmp/caller-home")
		t.Setenv("COMPOZY_HOME", "/tmp/caller-compozy-home")

		layout := prepareRuntimeLayout(t, &RuntimeHarnessOptions{})

		if got, want := lookupEnvValue(layout.Env, "HOME"), layout.OperatorHomeDir; got != want {
			t.Fatalf("lookupEnvValue(HOME) = %q, want %q", got, want)
		}
		if layout.OperatorHomeDir == layout.HomePaths.HomeDir {
			t.Fatalf("OperatorHomeDir = %q, want distinct COMPOZY_HOME", layout.OperatorHomeDir)
		}
		if got, want := lookupEnvValue(layout.Env, "COMPOZY_HOME"), layout.HomePaths.HomeDir; got != want {
			t.Fatalf("lookupEnvValue(COMPOZY_HOME) = %q, want %q", got, want)
		}
		if got, want := countEnvEntries(layout.Env, "HOME"), 1; got != want {
			t.Fatalf("countEnvEntries(HOME) = %d, want %d", got, want)
		}
		if got, want := countEnvEntries(layout.Env, "COMPOZY_HOME"), 1; got != want {
			t.Fatalf("countEnvEntries(COMPOZY_HOME) = %d, want %d", got, want)
		}
	})
}
