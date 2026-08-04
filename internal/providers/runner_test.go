package providers

import (
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

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

func TestDefaultProviderAuthCommandRunner(t *testing.T) {
	t.Parallel()

	t.Run("Should return when a lingering child keeps the output pipe open", func(t *testing.T) {
		t.Parallel()

		shell, err := exec.LookPath("sh")
		if err != nil {
			t.Skipf("POSIX shell unavailable: %v", err)
		}
		type runOutcome struct {
			result ProviderAuthCommandResult
			err    error
		}
		outcome := make(chan runOutcome, 1)
		ctx := t.Context()
		go func() {
			result, runErr := DefaultProviderAuthCommandRunner(ctx, ProviderAuthCommandSpec{
				Command:    "sh",
				Executable: shell,
				Args:       []string{"-c", "sleep 120 & exit 0"},
				Timeout:    DefaultProviderAuthCommandTimeout,
				NoTTY:      true,
			})
			outcome <- runOutcome{result: result, err: runErr}
		}()

		budget := time.NewTimer(20 * time.Second)
		defer budget.Stop()
		var observed runOutcome
		select {
		case observed = <-outcome:
		case <-budget.C:
			t.Fatal("DefaultProviderAuthCommandRunner() never returned, want the inherited pipe bounded")
		}
		if observed.err != nil {
			t.Fatalf("DefaultProviderAuthCommandRunner() error = %v", observed.err)
		}
		if observed.result.ExitCode != 0 {
			t.Fatalf("ExitCode = %d, want 0 from the completed status command", observed.result.ExitCode)
		}
	})
}
