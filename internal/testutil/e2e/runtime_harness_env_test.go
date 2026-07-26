package e2e

import "testing"

func TestRuntimeHarnessEnvContract(t *testing.T) {
	t.Parallel()

	t.Run("Should keep isolated home env when options provide reserved keys", func(t *testing.T) {
		t.Parallel()

		layout := prepareRuntimeLayout(t, &RuntimeHarnessOptions{
			Env: map[string]string{
				"AGH_HOME": "/tmp/outside-agh-home",
				"HOME":     "/tmp/outside-home",
			},
		})

		if got, want := lookupEnvValue(layout.Env, "HOME"), layout.HomePaths.HomeDir; got != want {
			t.Fatalf("lookupEnvValue(HOME) = %q, want %q", got, want)
		}
		if got, want := lookupEnvValue(layout.Env, "AGH_HOME"), layout.HomePaths.HomeDir; got != want {
			t.Fatalf("lookupEnvValue(AGH_HOME) = %q, want %q", got, want)
		}
		if got, want := countEnvEntries(layout.Env, "HOME"), 1; got != want {
			t.Fatalf("countEnvEntries(HOME) = %d, want %d", got, want)
		}
		if got, want := countEnvEntries(layout.Env, "AGH_HOME"), 1; got != want {
			t.Fatalf("countEnvEntries(AGH_HOME) = %d, want %d", got, want)
		}
	})
}
