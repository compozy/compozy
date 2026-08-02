package providers

import (
	"os"
	"slices"
	"testing"

	"github.com/compozy/compozy/internal/subprocess"
	"github.com/kballard/go-shellquote"
)

func TestProviderAuthCommandEnvironmentPrefix(t *testing.T) {
	t.Parallel()

	t.Run("Should apply private leading assignments before resolving the executable", func(t *testing.T) {
		t.Parallel()

		testBinary, err := os.Executable()
		if err != nil {
			t.Fatalf("os.Executable() error = %v", err)
		}
		command := "QA_LOGIN_TOKEN=distinctive-secret " + shellquote.Join(
			testBinary,
			"login",
			"--account",
			"hidden-account",
		)
		baseEnv := subprocess.SetEnvValue(os.Environ(), "QA_LOGIN_TOKEN", "stale-value")
		cmd, err := commandContext(t.Context(), ProviderAuthCommandSpec{
			Command: command,
			Env:     baseEnv,
			NoTTY:   true,
		})
		if err != nil {
			t.Fatalf("commandContext() error = %v", err)
		}
		if got, want := cmd.Path, testBinary; got != want {
			t.Fatalf("Path = %q, want %q", got, want)
		}
		if got, want := cmd.Args[1:], []string{"login", "--account", "hidden-account"}; !slices.Equal(got, want) {
			t.Fatalf("Args = %#v, want %#v", got, want)
		}
		if got, ok := subprocess.LookupEnv(cmd.Env, "QA_LOGIN_TOKEN"); !ok || got != "distinctive-secret" {
			t.Fatalf("QA_LOGIN_TOKEN = %q/%v, want distinctive-secret/true", got, ok)
		}
	})
}
