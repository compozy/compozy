package providers

import (
	"context"
	"errors"
	"os/exec"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

func TestRunAuthStatusProbe(t *testing.T) {
	t.Parallel()

	provider := compozyconfig.ProviderConfig{
		Command:       "acme acp",
		AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
		AuthStatusCmd: "acme auth status",
	}
	probeEnv := func(run ProviderAuthCommandRunner) *ProbeEnv {
		return &ProbeEnv{
			ProviderName: "acme",
			LookPath:     func(string) (string, error) { return "/bin/acme", nil },
			RunCommand:   run,
		}
	}

	t.Run("Should classify a completed status command and stamp its start", func(t *testing.T) {
		t.Parallel()
		startedAt := time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)
		probe, err := RunAuthStatusProbe(
			t.Context(),
			provider,
			probeEnv(func(context.Context, ProviderAuthCommandSpec) (ProviderAuthCommandResult, error) {
				return ProviderAuthCommandResult{ExitCode: 0, Stdout: "Logged in", DurationMs: 12}, nil
			}),
			func() time.Time { return startedAt },
		)
		if err != nil {
			t.Fatal(err)
		}
		if !probe.Ran || probe.Classification.State != ProviderAuthStateAuthenticated {
			t.Fatalf("probe = %#v, want an authenticated run", probe)
		}
		if probe.Result.ExitCode != 0 || probe.Result.DurationMs != 12 || !probe.StartedAt.Equal(startedAt) {
			t.Fatalf("probe result = %#v at %v", probe.Result, probe.StartedAt)
		}
	})

	t.Run("Should report an unprepared command without running it", func(t *testing.T) {
		t.Parallel()
		env := probeEnv(func(context.Context, ProviderAuthCommandSpec) (ProviderAuthCommandResult, error) {
			t.Fatal("runner called for a command that could not be prepared")
			return ProviderAuthCommandResult{}, nil
		})
		env.LookPath = func(string) (string, error) { return "", exec.ErrNotFound }
		probe, err := RunAuthStatusProbe(t.Context(), provider, env, time.Now)
		if err != nil {
			t.Fatal(err)
		}
		if probe.Ran || probe.Classification.State == ProviderAuthStateAuthenticated {
			t.Fatalf("probe = %#v, want an unprepared, non-authenticated result", probe)
		}
	})

	t.Run("Should return a runner failure as an error", func(t *testing.T) {
		t.Parallel()
		failure := errors.New("spawn failed")
		_, err := RunAuthStatusProbe(
			t.Context(),
			provider,
			probeEnv(func(context.Context, ProviderAuthCommandSpec) (ProviderAuthCommandResult, error) {
				return ProviderAuthCommandResult{}, failure
			}),
			time.Now,
		)
		if !errors.Is(err, failure) {
			t.Fatalf("err = %v, want %v", err, failure)
		}
	})
}
