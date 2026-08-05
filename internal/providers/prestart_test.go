package providers

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	diagcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/subprocess"
	"github.com/compozy/compozy/internal/testutil"
	shellquote "github.com/kballard/go-shellquote"
)

func TestPreStarter(t *testing.T) {
	t.Parallel()

	t.Run("Should return nil item when auth mode is none", func(t *testing.T) {
		t.Parallel()

		calls := 0
		report := NewPreStarter().PreStart(testutil.Context(t), compozyconfig.ProviderConfig{
			Command:  "missing-provider acp",
			AuthMode: compozyconfig.ProviderAuthModeNone,
		}, &ProbeEnv{
			ProviderName: "none-provider",
			LookPath: func(string) (string, error) {
				calls++
				return "", exec.ErrNotFound
			},
		})
		if report.Item != nil {
			t.Fatalf("PreStart(none).Item = %#v, want nil", report.Item)
		}
		if calls != 0 {
			t.Fatalf("LookPath calls = %d, want 0", calls)
		}
	})

	t.Run("Should return provider CLI missing item when launch command is unavailable", func(t *testing.T) {
		t.Parallel()

		report := NewPreStarter().PreStart(testutil.Context(t), compozyconfig.ProviderConfig{
			Command:  "missing-provider acp",
			AuthMode: compozyconfig.ProviderAuthModeNativeCLI,
		}, &ProbeEnv{
			ProviderName: "missing",
			LookPath: func(string) (string, error) {
				return "", exec.ErrNotFound
			},
		})
		assertMissingCLIReport(t, report)
	})

	t.Run("Should return credential unresolved item when required env ref is absent", func(t *testing.T) {
		t.Parallel()

		report := NewPreStarter().PreStart(testutil.Context(t), compozyconfig.ProviderConfig{
			Command:  "provider acp",
			AuthMode: compozyconfig.ProviderAuthModeBoundSecret,
			CredentialSlots: []compozyconfig.ProviderCredentialSlot{
				{Name: "api_key", TargetEnv: "TEST_API_KEY", SecretRef: "env:TEST_API_KEY", Required: true},
			},
		}, &ProbeEnv{
			ProviderName: "bound",
			LookPath: func(string) (string, error) {
				return "/bin/provider", nil
			},
			LookupEnv: func(string) (string, bool) {
				return "", false
			},
		})
		if report.Item == nil {
			t.Fatal("PreStart(missing credential).Item = nil, want diagnostic")
		}
		if report.Item.Code != diagcontract.CodeProviderCredentialUnresolved {
			t.Fatalf("Code = %q, want %q", report.Item.Code, diagcontract.CodeProviderCredentialUnresolved)
		}
	})

	t.Run("Should run status command without TTY", func(t *testing.T) {
		t.Parallel()

		report := NewPreStarter().PreStart(testutil.Context(t), compozyconfig.ProviderConfig{
			Command:       "provider acp",
			AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
			AuthStatusCmd: "provider auth status",
		}, &ProbeEnv{
			ProviderName: "native",
			LookPath: func(string) (string, error) {
				return "/bin/provider", nil
			},
			RunCommand: func(_ context.Context, spec ProviderAuthCommandSpec) (ProviderAuthCommandResult, error) {
				if !spec.NoTTY {
					t.Fatal("NoTTY = false, want pre-start probe to be non-interactive")
				}
				if spec.Executable != "/bin/provider" {
					t.Fatalf("Executable = %q, want /bin/provider", spec.Executable)
				}
				if !slices.Equal(spec.Args, []string{"auth", "status"}) {
					t.Fatalf("Args = %#v, want auth status", spec.Args)
				}
				return ProviderAuthCommandResult{ExitCode: 1, Stderr: "not logged in"}, nil
			},
		})
		if report.Item == nil {
			t.Fatal("PreStart(needs login).Item = nil, want diagnostic")
		}
		if report.Item.Code != diagcontract.CodeProviderNotAuthenticated {
			t.Fatalf("Code = %q, want %q", report.Item.Code, diagcontract.CodeProviderNotAuthenticated)
		}
	})
}

func TestPreStarterRunsAuthStatusCommandInFinalEnvironment(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve and run the status command with the final PATH and working directory", func(t *testing.T) {
		t.Parallel()
		if runtime.GOOS == "windows" {
			t.Skip("Unix executable fixture; Windows environment semantics are covered separately")
		}

		finalDir := t.TempDir()
		canonicalFinalDir, err := filepath.EvalSymlinks(finalDir)
		if err != nil {
			t.Fatalf("filepath.EvalSymlinks(final directory) error = %v", err)
		}
		finalBin := t.TempDir()
		testBinary, err := os.Executable()
		if err != nil {
			t.Fatalf("os.Executable() error = %v", err)
		}
		statusExecutable := filepath.Join(finalBin, "sh")
		if err := os.Symlink(testBinary, statusExecutable); err != nil {
			t.Fatalf("os.Symlink(status executable) error = %v", err)
		}

		finalEnv := subprocess.SetEnvValue(os.Environ(), "PATH", finalBin)
		finalEnv = subprocess.SetEnvValue(finalEnv, "COMPOZY_PROVIDER_AUTH_STATUS_HELPER", "1")
		finalEnv = subprocess.SetEnvValue(finalEnv, "COMPOZY_PROVIDER_AUTH_EXPECTED_CWD", canonicalFinalDir)
		provider := compozyconfig.ProviderConfig{
			Command:       shellquote.Join(testBinary, "-test.run=^TestProviderAuthStatusHelperProcess$"),
			AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
			AuthStatusCmd: shellquote.Join("sh", "-test.run=^TestProviderAuthStatusHelperProcess$"),
		}
		report := NewPreStarter().PreStart(testutil.Context(t), provider, &ProbeEnv{
			ProviderName:  "provider-a",
			PreStartScope: preStartTestScope("auth-status-final-env"),
			LookPath: func(command string) (string, error) {
				return subprocess.ResolveExecutable(command, finalEnv, canonicalFinalDir)
			},
			CommandEnv: finalEnv,
			CommandDir: canonicalFinalDir,
		})
		assertNoPreStartReport(t, report)
	})
}

func TestPreStarterPinsAuthStatusExecutableIdentity(t *testing.T) {
	t.Parallel()

	const finalDir = "/final/workspace"
	const finalLaunchExecutable = "/final/bin/provider-launch"
	const finalStatusExecutable = "/final/bin/provider-status"
	provider := compozyconfig.ProviderConfig{
		Command:       finalLaunchExecutable + " acp",
		AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
		AuthStatusCmd: "provider-status auth status",
	}
	finalEnv := []string{"PATH=/final/bin", "PROVIDER_TOKEN=final-secret"}

	t.Run("Should report a resolver error without a fallback lookup", func(t *testing.T) {
		t.Parallel()

		resolverErr := errors.New("final status executable resolver sentinel")
		var calls atomic.Int32
		probe := probeEnvWithResolvedLaunch(ProbeEnv{
			ProviderName: "provider-a",
			CommandEnv:   append([]string(nil), finalEnv...),
			CommandDir:   finalDir,
			LookPath: func(command string) (string, error) {
				if command != "provider-status" {
					t.Fatalf("LookPath command = %q, want provider-status", command)
				}
				if calls.Add(1) == 1 {
					return "", fmt.Errorf("resolver failed token=prestart-resolution-secret: %w", resolverErr)
				}
				return "/later/bin/provider-status", nil
			},
			RunCommand: func(_ context.Context, spec ProviderAuthCommandSpec) (ProviderAuthCommandResult, error) {
				t.Fatalf("RunCommand(%q) called after resolver failure", spec.Command)
				return ProviderAuthCommandResult{}, nil
			},
		}, provider.Command, finalLaunchExecutable)
		report := NewPreStarter().PreStart(testutil.Context(t), provider, &probe)

		if got := calls.Load(); got != 1 {
			t.Fatalf("status executable resolver calls = %d, want 1", got)
		}
		if report.Item == nil {
			t.Fatal("PreStart().Item = nil, want resolution diagnostic")
		}
		if report.Item.Code != diagcontract.CodeProviderClassificationUnknown {
			t.Fatalf(
				"PreStart().Code = %q, want %q",
				report.Item.Code,
				diagcontract.CodeProviderClassificationUnknown,
			)
		}
		if !errors.Is(report.Cause, resolverErr) {
			t.Fatalf("PreStart().Cause = %v, want resolver error", report.Cause)
		}
		if strings.Contains(report.Item.Message, "prestart-resolution-secret") {
			t.Fatalf("PreStart().Message = %q, leaked resolver secret", report.Item.Message)
		}
		if !strings.Contains(report.Item.Message, "[REDACTED]") {
			t.Fatalf("PreStart().Message = %q, want redaction marker", report.Item.Message)
		}
		if got, want := report.Item.Evidence["action"], string(ProviderFailureActionInspect); got != want {
			t.Fatalf("PreStart().Evidence[action] = %#v, want %q", got, want)
		}
	})

	t.Run("Should report a missing status executable from the single final lookup", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32
		probe := probeEnvWithResolvedLaunch(ProbeEnv{
			ProviderName: "provider-a",
			CommandEnv:   append([]string(nil), finalEnv...),
			CommandDir:   finalDir,
			LookPath: func(command string) (string, error) {
				if command != "provider-status" {
					t.Fatalf("LookPath command = %q, want provider-status", command)
				}
				if calls.Add(1) == 1 {
					return "", exec.ErrNotFound
				}
				return "/later/bin/provider-status", nil
			},
			RunCommand: func(_ context.Context, spec ProviderAuthCommandSpec) (ProviderAuthCommandResult, error) {
				t.Fatalf("RunCommand(%q) called after missing resolution", spec.Command)
				return ProviderAuthCommandResult{}, nil
			},
		}, provider.Command, finalLaunchExecutable)
		report := NewPreStarter().PreStart(testutil.Context(t), provider, &probe)

		if got := calls.Load(); got != 1 {
			t.Fatalf("status executable resolver calls = %d, want 1", got)
		}
		assertMissingCLIReport(t, report)
		if got, want := report.Item.Evidence["action"], string(ProviderFailureActionInstallCLI); got != want {
			t.Fatalf("PreStart().Evidence[action] = %#v, want %q", got, want)
		}
	})

	t.Run("Should classify and execute with the first final resolver identity", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32
		probe := probeEnvWithResolvedLaunch(ProbeEnv{
			ProviderName: "provider-a",
			CommandEnv:   append([]string(nil), finalEnv...),
			CommandDir:   finalDir,
			LookPath: func(command string) (string, error) {
				if command != "provider-status" {
					t.Fatalf("LookPath command = %q, want provider-status", command)
				}
				if calls.Add(1) == 1 {
					return finalStatusExecutable, nil
				}
				return "", errors.New("resolver identity changed after the first lookup")
			},
			RunCommand: func(_ context.Context, spec ProviderAuthCommandSpec) (ProviderAuthCommandResult, error) {
				if spec.Executable != finalStatusExecutable {
					t.Fatalf("status executable = %q, want %q", spec.Executable, finalStatusExecutable)
				}
				if !slices.Equal(spec.Args, []string{"auth", "status"}) {
					t.Fatalf("status args = %#v, want auth status", spec.Args)
				}
				if spec.Dir != finalDir {
					t.Fatalf("status directory = %q, want %q", spec.Dir, finalDir)
				}
				if !slices.Equal(spec.Env, finalEnv) {
					t.Fatalf("status environment = %#v, want %#v", spec.Env, finalEnv)
				}
				return ProviderAuthCommandResult{ExitCode: 0, Stdout: "authenticated"}, nil
			},
		}, provider.Command, finalLaunchExecutable)
		report := NewPreStarter().PreStart(testutil.Context(t), provider, &probe)

		if got := calls.Load(); got != 1 {
			t.Fatalf("status executable resolver calls = %d, want 1", got)
		}
		assertNoPreStartReport(t, report)
	})
}

func TestPreStarterPinsLaunchExecutableIdentity(t *testing.T) {
	t.Parallel()

	provider := compozyconfig.ProviderConfig{
		Command:  "provider-launch acp",
		AuthMode: compozyconfig.ProviderAuthModeNativeCLI,
	}
	finalEnv := []string{"PATH=/final/bin"}
	const finalDir = "/final/workspace"

	t.Run("Should keep a normalized missing executable terminal when a later lookup would succeed", func(t *testing.T) {
		t.Parallel()

		var calls atomic.Int32
		probe := ProbeEnv{
			ProviderName:  "provider-a",
			PreStartScope: preStartTestScope("launch-not-found"),
			CommandEnv:    append([]string(nil), finalEnv...),
			CommandDir:    finalDir,
			ResolveCommand: func(context.Context, string, []string, string) (string, error) {
				calls.Add(1)
				return "/later/bin/provider-launch", nil
			},
		}
		probe = probe.WithLaunchExecutableResolution(subprocess.NewExecutableResolution(
			provider.Command,
			probe.CommandEnv,
			probe.CommandDir,
			"",
			fmt.Errorf("normalized launch lookup: %w", exec.ErrNotFound),
		))

		report := NewPreStarter().PreStart(testutil.Context(t), provider, &probe)
		assertMissingCLIReport(t, report)
		if report.Cause != nil {
			t.Fatalf("PreStart().Cause = %v, want cacheable not-found diagnostic", report.Cause)
		}
		if got := calls.Load(); got != 0 {
			t.Fatalf("fallback resolver calls = %d, want 0 after terminal not-found", got)
		}
	})

	causes := []error{
		fmt.Errorf("inspect cwd token=launch-operational-secret: %w", os.ErrNotExist),
		context.Canceled,
		context.DeadlineExceeded,
		errors.New("launch resolver sentinel"),
	}
	for _, cause := range causes {
		t.Run("Should preserve failed launch cause "+cause.Error(), func(t *testing.T) {
			t.Parallel()

			var calls atomic.Int32
			probe := ProbeEnv{
				ProviderName:  "provider-a",
				PreStartScope: preStartTestScope("launch-failed"),
				CommandEnv:    append([]string(nil), finalEnv...),
				CommandDir:    finalDir,
				ResolveCommand: func(context.Context, string, []string, string) (string, error) {
					calls.Add(1)
					return "/later/bin/provider-launch", nil
				},
			}
			probe = probe.WithLaunchExecutableResolution(subprocess.NewExecutableResolution(
				provider.Command,
				probe.CommandEnv,
				probe.CommandDir,
				"",
				cause,
			))

			starter := NewPreStarter()
			for attempt := 1; attempt <= 2; attempt++ {
				report := starter.PreStart(testutil.Context(t), provider, &probe)
				if !errors.Is(report.Cause, cause) {
					t.Fatalf("PreStart() attempt %d Cause = %v, want %v", attempt, report.Cause, cause)
				}
				if report.Item == nil || report.Item.Code != diagcontract.CodeProviderClassificationUnknown {
					t.Fatalf("PreStart() attempt %d Item = %#v, want unknown diagnostic", attempt, report.Item)
				}
				if strings.Contains(report.Item.Message, "launch-operational-secret") {
					t.Fatalf("PreStart() attempt %d Message = %q, leaked cause text", attempt, report.Item.Message)
				}
			}
			if got := calls.Load(); got != 0 {
				t.Fatalf("fallback resolver calls = %d, want 0 after terminal failure", got)
			}
		})
	}
}

func TestProviderAuthStatusHelperProcess(t *testing.T) {
	if os.Getenv("COMPOZY_PROVIDER_AUTH_STATUS_HELPER") != "1" {
		return
	}
	t.Run("Should observe the final provider working directory", func(t *testing.T) {
		want := os.Getenv("COMPOZY_PROVIDER_AUTH_EXPECTED_CWD")
		got, err := os.Getwd()
		if err != nil {
			t.Fatalf("os.Getwd() error = %v", err)
		}
		if got != want {
			t.Fatalf("provider auth status cwd = %q, want %q", got, want)
		}
		if _, err := fmt.Fprintln(os.Stdout, "authenticated"); err != nil {
			t.Fatalf("write provider auth status output: %v", err)
		}
	})
}

func TestPreStarterCachesOnlyMatchingFinalScope(t *testing.T) {
	t.Parallel()
	t.Run("Should cache only the matching final workspace home and runtime scope", func(t *testing.T) {
		t.Parallel()

		starter := NewPreStarter()
		provider := preStartTestProvider()
		calls := 0
		probe := func(scope PreStartScope) PreStartReport {
			return starter.PreStart(testutil.Context(t), provider, &ProbeEnv{
				ProviderName:  "provider-a",
				PreStartScope: scope,
				LookPath: func(string) (string, error) {
					calls++
					return "", exec.ErrNotFound
				},
			})
		}

		scope := preStartTestScope("one")
		assertMissingCLIReport(t, probe(scope))
		assertMissingCLIReport(t, probe(scope))
		if calls != 1 {
			t.Fatalf("same scope LookPath calls = %d, want 1", calls)
		}

		workspaceScope := scope
		workspaceScope.WorkspaceID = "workspace-two"
		assertMissingCLIReport(t, probe(workspaceScope))

		homeScope := scope
		homeScope.HomeIdentity = "/compozy/home-two"
		assertMissingCLIReport(t, probe(homeScope))

		runtimeScope := scope
		runtimeScope.SandboxID = "sandbox-two"
		assertMissingCLIReport(t, probe(runtimeScope))

		if calls != 4 {
			t.Fatalf("scope-separated LookPath calls = %d, want 4", calls)
		}
	})
}

func TestPreStarterCachesOnlyMatchingFinalProbeIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should distinguish final command environment and auth configuration", func(t *testing.T) {
		t.Parallel()

		starter := NewPreStarter()
		provider := compozyconfig.ProviderConfig{
			Command:       "provider acp",
			AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
			AuthStatusCmd: "provider auth status",
			AuthLoginCmd:  "provider auth login",
		}
		commandEnv := []string{
			"PATH=/provider/bin",
			"PATHEXT=.EXE;.CMD",
			"PROVIDER_TOKEN=first-secret",
		}
		var calls atomic.Int32
		probe := func(t *testing.T, config compozyconfig.ProviderConfig, env []string) PreStartReport {
			t.Helper()

			return starter.PreStart(testutil.Context(t), config, &ProbeEnv{
				ProviderName:  "provider-a",
				PreStartScope: preStartTestScope("one"),
				CommandEnv:    append([]string(nil), env...),
				LookPath: func(string) (string, error) {
					return "/provider/bin/provider", nil
				},
				RunCommand: func(context.Context, ProviderAuthCommandSpec) (ProviderAuthCommandResult, error) {
					calls.Add(1)
					return ProviderAuthCommandResult{ExitCode: 0, Stdout: "authenticated"}, nil
				},
			})
		}

		assertNoPreStartReport(t, probe(t, provider, commandEnv))
		assertNoPreStartReport(t, probe(t, provider, commandEnv))
		if got := calls.Load(); got != 1 {
			t.Fatalf("identical final probe calls = %d, want 1 cache hit", got)
		}

		changedCommand := provider
		changedCommand.Command = "provider-next acp"
		assertNoPreStartReport(t, probe(t, changedCommand, commandEnv))

		changedPath := append([]string(nil), commandEnv...)
		changedPath[0] = "PATH=/provider/next-bin"
		assertNoPreStartReport(t, probe(t, provider, changedPath))

		changedPathExt := append([]string(nil), commandEnv...)
		changedPathExt[1] = "PATHEXT=.CMD;.BAT"
		assertNoPreStartReport(t, probe(t, provider, changedPathExt))

		changedConfig := provider
		changedConfig.AuthStatusCmd = "provider auth inspect"
		assertNoPreStartReport(t, probe(t, changedConfig, commandEnv))

		changedLogin := provider
		changedLogin.AuthLoginCmd = "provider auth login-next"
		assertNoPreStartReport(t, probe(t, changedLogin, commandEnv))

		changedSecretEnv := append([]string(nil), commandEnv...)
		changedSecretEnv[2] = "PROVIDER_TOKEN=second-secret"
		assertNoPreStartReport(t, probe(t, provider, changedSecretEnv))

		if got := calls.Load(); got != 7 {
			t.Fatalf("distinct final probe calls = %d, want 7 cache misses", got)
		}

		var fingerprintKey preStartCacheHMACKey
		fingerprintKey[0] = 1
		baseFingerprint, ok := preStartSemanticFingerprint(
			fingerprintKey,
			provider,
			ProbeEnv{CommandEnv: commandEnv},
		)
		if !ok {
			t.Fatal("base pre-start fingerprint failed")
		}
		resolvedEnv := probeEnvWithResolvedLaunch(
			ProbeEnv{CommandEnv: commandEnv},
			provider.Command,
			"/provider/bin/provider-resolved",
		)
		resolvedFingerprint, ok := preStartSemanticFingerprint(
			fingerprintKey,
			provider,
			resolvedEnv,
		)
		if !ok {
			t.Fatal("resolved pre-start fingerprint failed")
		}
		if baseFingerprint == resolvedFingerprint {
			t.Fatal("resolved launch executable did not change the opaque fingerprint")
		}
		workingDirFingerprint, ok := preStartSemanticFingerprint(
			fingerprintKey,
			provider,
			ProbeEnv{CommandEnv: commandEnv, CommandDir: "/provider/workspace-next"},
		)
		if !ok {
			t.Fatal("working-directory pre-start fingerprint failed")
		}
		if baseFingerprint == workingDirFingerprint {
			t.Fatal("final provider working directory did not change the opaque fingerprint")
		}
	})
}

func TestPreStarterCachesOnlyMatchingCredentialSlotShape(t *testing.T) {
	t.Parallel()

	t.Run("Should distinguish final bound-secret credential shape", func(t *testing.T) {
		t.Parallel()

		starter := NewPreStarter()
		provider := compozyconfig.ProviderConfig{
			Command:  "provider acp",
			AuthMode: compozyconfig.ProviderAuthModeBoundSecret,
			CredentialSlots: []compozyconfig.ProviderCredentialSlot{{
				Name:      "token",
				TargetEnv: "PROVIDER_TOKEN",
				SecretRef: "env:PROVIDER_TOKEN",
				Kind:      "api_key",
				Required:  true,
			}},
		}
		commandEnv := []string{
			"PATH=/provider/bin",
			"PROVIDER_TOKEN=first-secret",
			"PROVIDER_TOKEN_NEXT=second-secret",
		}
		var lookupCalls atomic.Int32
		probe := func(t *testing.T, config compozyconfig.ProviderConfig) PreStartReport {
			t.Helper()

			return starter.PreStart(testutil.Context(t), config, &ProbeEnv{
				ProviderName:  "provider-a",
				PreStartScope: preStartTestScope("one"),
				CommandEnv:    append([]string(nil), commandEnv...),
				LookPath: func(string) (string, error) {
					return "/provider/bin/provider", nil
				},
				LookupEnv: func(key string) (string, bool) {
					lookupCalls.Add(1)
					for _, entry := range slices.Backward(commandEnv) {
						candidate, value, ok := strings.Cut(entry, "=")
						if ok && candidate == key {
							return value, true
						}
					}
					return "", false
				},
			})
		}

		if report := probe(t, provider); report.Item != nil {
			t.Fatalf("first bound-secret report = %#v, want nil", report.Item)
		}
		if report := probe(t, provider); report.Item != nil {
			t.Fatalf("identical bound-secret report = %#v, want nil", report.Item)
		}
		if got := lookupCalls.Load(); got != 1 {
			t.Fatalf("identical credential lookup calls = %d, want 1 cache hit", got)
		}

		changes := []struct {
			name   string
			mutate func(*compozyconfig.ProviderCredentialSlot)
		}{
			{
				name: "Should distinguish credential names",
				mutate: func(slot *compozyconfig.ProviderCredentialSlot) {
					slot.Name = "next-token"
				},
			},
			{
				name: "Should distinguish credential target environments",
				mutate: func(slot *compozyconfig.ProviderCredentialSlot) {
					slot.TargetEnv = "PROVIDER_TOKEN_NEXT"
				},
			},
			{
				name: "Should distinguish credential secret references",
				mutate: func(slot *compozyconfig.ProviderCredentialSlot) {
					slot.SecretRef = "env:PROVIDER_TOKEN_NEXT"
				},
			},
			{
				name: "Should distinguish credential kinds",
				mutate: func(slot *compozyconfig.ProviderCredentialSlot) {
					slot.Kind = "bearer"
				},
			},
			{
				name: "Should distinguish required credential slots",
				mutate: func(slot *compozyconfig.ProviderCredentialSlot) {
					slot.Required = false
				},
			},
		}
		for _, change := range changes {
			t.Run(change.name, func(t *testing.T) {
				changedSlot := provider
				changedSlot.CredentialSlots = append(
					[]compozyconfig.ProviderCredentialSlot(nil),
					provider.CredentialSlots...,
				)
				change.mutate(&changedSlot.CredentialSlots[0])
				if report := probe(t, changedSlot); report.Item != nil {
					t.Fatalf("changed bound-secret report = %#v, want nil", report.Item)
				}
			})
		}
		if got, want := lookupCalls.Load(), int32(1+len(changes)); got != want {
			t.Fatalf("distinct credential lookup calls = %d, want %d cache misses", got, want)
		}
	})
}

func TestPreStarterDoesNotCacheCanceledProbe(t *testing.T) {
	t.Parallel()

	t.Run("Should not publish a report after its probe context is canceled", func(t *testing.T) {
		t.Parallel()

		starter := NewPreStarter()
		provider := compozyconfig.ProviderConfig{
			Command:       "provider acp",
			AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
			AuthStatusCmd: "provider auth status",
		}
		var calls atomic.Int32
		probeEnv := func(cancelFirst bool, cancel context.CancelFunc) *ProbeEnv {
			return &ProbeEnv{
				ProviderName:  "provider-a",
				PreStartScope: preStartTestScope("one"),
				CommandEnv:    []string{"PATH=/provider/bin"},
				LookPath: func(string) (string, error) {
					return "/provider/bin/provider", nil
				},
				RunCommand: func(_ context.Context, _ ProviderAuthCommandSpec) (ProviderAuthCommandResult, error) {
					calls.Add(1)
					if cancelFirst {
						cancel()
					}
					return ProviderAuthCommandResult{ExitCode: 0, Stdout: "authenticated"}, nil
				},
			}
		}

		ctx, cancel := context.WithCancel(testutil.Context(t))
		defer cancel()
		first := starter.PreStart(ctx, provider, probeEnv(true, cancel))
		if !errors.Is(first.Cause, context.Canceled) {
			t.Fatalf("PreStart(canceled).Cause = %v, want context.Canceled", first.Cause)
		}
		second := starter.PreStart(testutil.Context(t), provider, probeEnv(false, func() {}))
		assertNoPreStartReport(t, second)
		if got := calls.Load(); got != 2 {
			t.Fatalf("probe calls after cancellation = %d, want 2 without cached canceled result", got)
		}
	})

	t.Run("Should not publish an auth status resolution failure with a cause", func(t *testing.T) {
		t.Parallel()

		starter := NewPreStarter()
		provider := compozyconfig.ProviderConfig{
			Command:       "/provider/bin/provider acp",
			AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
			AuthStatusCmd: "provider auth status",
		}
		resolverErr := errors.New("status resolution sentinel")
		var calls atomic.Int32
		probeEnv := func() *ProbeEnv {
			probe := probeEnvWithResolvedLaunch(ProbeEnv{
				ProviderName:  "provider-a",
				PreStartScope: preStartTestScope("resolution-failure"),
				CommandEnv:    []string{"PATH=/provider/bin"},
				LookPath: func(string) (string, error) {
					calls.Add(1)
					return "", fmt.Errorf("resolver failed token=cache-resolution-secret: %w", resolverErr)
				},
				RunCommand: func(
					_ context.Context,
					spec ProviderAuthCommandSpec,
				) (ProviderAuthCommandResult, error) {
					t.Fatalf("RunCommand(%q) called after resolver failure", spec.Command)
					return ProviderAuthCommandResult{}, nil
				},
			}, provider.Command, "/provider/bin/provider")
			return &probe
		}

		for attempt := 1; attempt <= 2; attempt++ {
			report := starter.PreStart(testutil.Context(t), provider, probeEnv())
			if !errors.Is(report.Cause, resolverErr) {
				t.Fatalf("PreStart() attempt %d Cause = %v, want resolver sentinel", attempt, report.Cause)
			}
			if report.Item == nil || report.Item.Code != diagcontract.CodeProviderClassificationUnknown {
				t.Fatalf("PreStart() attempt %d Item = %#v, want unknown diagnostic", attempt, report.Item)
			}
			if strings.Contains(report.Item.Message, "cache-resolution-secret") {
				t.Fatalf("PreStart() attempt %d Message = %q, leaked resolver secret", attempt, report.Item.Message)
			}
		}
		if got, want := calls.Load(), int32(2); got != want {
			t.Fatalf("resolver calls = %d, want %d without cached cause", got, want)
		}
	})
}

func TestPreStarterClearPreventsInFlightProbeRepopulation(t *testing.T) {
	t.Parallel()

	t.Run("Should not repopulate after clear races a blocked probe", func(t *testing.T) {
		t.Parallel()

		starter := NewPreStarter()
		provider := compozyconfig.ProviderConfig{
			Command:       "provider acp",
			AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
			AuthStatusCmd: "provider auth status",
		}
		entered := make(chan struct{})
		release := make(chan struct{})
		reports := make(chan PreStartReport, 1)
		var calls atomic.Int32
		probeEnv := &ProbeEnv{
			ProviderName:  "provider-a",
			PreStartScope: preStartTestScope("one"),
			CommandEnv:    []string{"PATH=/provider/bin"},
			LookPath: func(string) (string, error) {
				return "/provider/bin/provider", nil
			},
			RunCommand: func(ctx context.Context, _ ProviderAuthCommandSpec) (ProviderAuthCommandResult, error) {
				if calls.Add(1) == 1 {
					close(entered)
					select {
					case <-release:
					case <-ctx.Done():
						return ProviderAuthCommandResult{}, ctx.Err()
					}
				}
				return ProviderAuthCommandResult{ExitCode: 1, Stderr: "not logged in"}, nil
			},
		}

		go func() {
			reports <- starter.PreStart(testutil.Context(t), provider, probeEnv)
		}()

		select {
		case <-entered:
		case <-testutil.Context(t).Done():
			t.Fatal("blocked probe did not begin before test context expired")
		}
		starter.Clear()
		close(release)

		select {
		case report := <-reports:
			assertProviderNeedsLoginReport(t, report)
		case <-testutil.Context(t).Done():
			t.Fatal("blocked probe did not finish before test context expired")
		}

		assertProviderNeedsLoginReport(t, starter.PreStart(testutil.Context(t), provider, probeEnv))
		if got := calls.Load(); got != 2 {
			t.Fatalf("probe calls after clear = %d, want 2 without in-flight repopulation", got)
		}
	})
}

func TestPreStarterExpiresAndEvictsLeastRecentlyUsedReports(t *testing.T) {
	t.Parallel()
	t.Run("Should expire reports and evict the least recently used scope", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)
		starter := newPreStarter(
			withPreStartClock(func() time.Time { return now }),
			withPreStartCacheTTL(time.Minute),
			withPreStartCacheCapacity(2),
		)
		provider := preStartTestProvider()
		calls := 0
		probe := func(scope PreStartScope) {
			assertMissingCLIReport(t, starter.PreStart(testutil.Context(t), provider, &ProbeEnv{
				ProviderName:  "provider-a",
				PreStartScope: scope,
				LookPath: func(string) (string, error) {
					calls++
					return "", exec.ErrNotFound
				},
			}))
		}

		firstScope := preStartTestScope("one")
		secondScope := preStartTestScope("two")
		thirdScope := preStartTestScope("three")
		probe(firstScope)
		probe(secondScope)
		probe(firstScope)
		probe(thirdScope)
		probe(firstScope)
		probe(secondScope)
		if calls != 4 {
			t.Fatalf("LRU LookPath calls = %d, want 4", calls)
		}

		now = now.Add(time.Minute)
		probe(firstScope)
		if calls != 5 {
			t.Fatalf("expired cache LookPath calls = %d, want 5", calls)
		}
	})
}

func TestPreStarterDoesNotRetainSecretsOrMutableReports(t *testing.T) {
	t.Parallel()
	t.Run("Should avoid retaining secret inputs and caller-owned diagnostics", func(t *testing.T) {
		t.Parallel()

		starter := NewPreStarter()
		secret := "unretainable-provider-secret"
		provider := preStartTestProvider()
		provider.AuthStatusCmd = "missing-provider auth status"
		provider.CredentialSlots = []compozyconfig.ProviderCredentialSlot{{
			Name:      "token",
			TargetEnv: "PROVIDER_TOKEN",
			SecretRef: "secret:provider-token",
			Required:  true,
		}}
		probe := probeEnvWithResolvedLaunch(ProbeEnv{
			ProviderName:  "provider-a",
			PreStartScope: preStartTestScope("one"),
			CommandEnv:    []string{"PROVIDER_TOKEN=" + secret},
			LookPath: func(string) (string, error) {
				return "", exec.ErrNotFound
			},
		}, provider.Command, "/sensitive/provider/resolved-path")
		env := &probe

		first := starter.PreStart(testutil.Context(t), provider, env)
		assertMissingCLIReport(t, first)
		if first.Item == nil {
			t.Fatal("first report item = nil, want diagnostic")
		}
		first.Item.Message = "mutated"
		first.Item.Evidence["state"] = "mutated"
		env.CommandEnv[0] = "PROVIDER_TOKEN=changed"

		starter.mu.Lock()
		entryCount := len(starter.entries)
		rawSecretDigest := preStartCacheFingerprint(sha256.Sum256([]byte(secret)))
		if entryCount != 1 {
			starter.mu.Unlock()
			t.Fatalf("cache entries = %d, want 1", entryCount)
		}
		for key, entry := range starter.entries {
			retained := fmt.Sprintf("%#v %#v", key, entry.report)
			if strings.Contains(retained, secret) ||
				strings.Contains(retained, "secret:provider-token") ||
				strings.Contains(retained, "PROVIDER_TOKEN=") ||
				strings.Contains(retained, "/sensitive/provider/resolved-path") {
				starter.mu.Unlock()
				t.Fatal("cache retained sensitive probe input")
			}
			if key.Fingerprint == rawSecretDigest {
				starter.mu.Unlock()
				t.Fatal("cache retained a raw secret-derived SHA fingerprint")
			}
		}
		starter.mu.Unlock()

		env.CommandEnv[0] = "PROVIDER_TOKEN=" + secret
		second := starter.PreStart(testutil.Context(t), provider, env)
		assertMissingCLIReport(t, second)
		if second.Item == first.Item {
			t.Fatal("cached report item shares the caller pointer")
		}
		if second.Item.Evidence["state"] == "mutated" || second.Item.Message == "mutated" {
			t.Fatalf("cached report retained caller mutation: %#v", second.Item)
		}
	})
}

func TestPreStarterDoesNotRetainSensitiveProviderCommand(t *testing.T) {
	t.Parallel()
	t.Run("Should cache only a redacted report when provider command text is sensitive", func(t *testing.T) {
		t.Parallel()

		starter := NewPreStarter()
		calls := 0
		provider := compozyconfig.ProviderConfig{
			Command:       "provider acp",
			AuthMode:      compozyconfig.ProviderAuthModeNativeCLI,
			AuthStatusCmd: "provider auth status",
			AuthLoginCmd:  "provider login --operator-token=do-not-cache",
		}
		env := &ProbeEnv{
			ProviderName:  "provider-a",
			PreStartScope: preStartTestScope("one"),
			LookPath: func(string) (string, error) {
				return "/bin/provider", nil
			},
			RunCommand: func(context.Context, ProviderAuthCommandSpec) (ProviderAuthCommandResult, error) {
				calls++
				return ProviderAuthCommandResult{ExitCode: 1, Stderr: "not logged in"}, nil
			},
		}

		first := starter.PreStart(testutil.Context(t), provider, env)
		second := starter.PreStart(testutil.Context(t), provider, env)
		if first.Item == nil || second.Item == nil ||
			first.Item.Code != diagcontract.CodeProviderNotAuthenticated ||
			second.Item.Code != diagcontract.CodeProviderNotAuthenticated {
			t.Fatal("PreStart() reports = missing provider authentication diagnostic")
		}
		if strings.Contains(first.Item.Message, provider.AuthLoginCmd) ||
			strings.Contains(second.Item.Message, provider.AuthLoginCmd) {
			t.Fatal("provider login command leaked into a pre-start report")
		}
		if calls != 1 {
			t.Fatalf("status probe calls = %d, want 1 cached redacted report", calls)
		}
		starter.mu.Lock()
		entryCount := len(starter.entries)
		for _, entry := range starter.entries {
			if strings.Contains(fmt.Sprintf("%#v", entry.report), provider.AuthLoginCmd) {
				starter.mu.Unlock()
				t.Fatal("cache retained a provider login command")
			}
		}
		starter.mu.Unlock()
		if entryCount != 1 {
			t.Fatalf("cache entries = %d, want 1 redacted report", entryCount)
		}
	})
}

func TestPreStarterDoesNotCacheWithoutFinalScope(t *testing.T) {
	t.Parallel()
	t.Run("Should skip cache entries without a final scope", func(t *testing.T) {
		t.Parallel()

		starter := NewPreStarter()
		calls := 0
		env := &ProbeEnv{
			ProviderName: "provider-a",
			LookPath: func(string) (string, error) {
				calls++
				return "", exec.ErrNotFound
			},
		}
		provider := preStartTestProvider()
		assertMissingCLIReport(t, starter.PreStart(testutil.Context(t), provider, env))
		assertMissingCLIReport(t, starter.PreStart(testutil.Context(t), provider, env))
		if calls != 2 {
			t.Fatalf("unscoped LookPath calls = %d, want 2", calls)
		}
	})
}

func preStartTestProvider() compozyconfig.ProviderConfig {
	return compozyconfig.ProviderConfig{
		Command:  "missing-provider acp",
		AuthMode: compozyconfig.ProviderAuthModeNativeCLI,
	}
}

func probeEnvWithResolvedLaunch(
	env ProbeEnv,
	command string,
	executable string,
) ProbeEnv {
	return env.WithLaunchExecutableResolution(subprocess.NewExecutableResolution(
		command,
		env.CommandEnv,
		env.CommandDir,
		executable,
		nil,
	))
}

func preStartTestScope(sandboxID string) PreStartScope {
	return PreStartScope{
		WorkspaceID:    "workspace-one",
		HomeIdentity:   "/compozy/home-one",
		SandboxID:      "sandbox-" + sandboxID,
		SandboxBackend: "local",
		SandboxProfile: "local",
	}
}

func assertMissingCLIReport(t *testing.T, report PreStartReport) {
	t.Helper()

	if report.Item == nil {
		t.Fatal("PreStart().Item = nil, want diagnostic")
	}
	if report.Item.Code != diagcontract.CodeProviderCLIMissing {
		t.Fatalf(
			"PreStart().Code = %q, want %q (message %q, cause %v)",
			report.Item.Code,
			diagcontract.CodeProviderCLIMissing,
			report.Item.Message,
			report.Cause,
		)
	}
}

func assertProviderNeedsLoginReport(t *testing.T, report PreStartReport) {
	t.Helper()

	if report.Item == nil {
		t.Fatal("PreStart().Item = nil, want provider authentication diagnostic")
	}
	if report.Item.Code != diagcontract.CodeProviderNotAuthenticated {
		t.Fatalf(
			"PreStart().Code = %q, want %q",
			report.Item.Code,
			diagcontract.CodeProviderNotAuthenticated,
		)
	}
}

func assertNoPreStartReport(t *testing.T, report PreStartReport) {
	t.Helper()

	if report.Item != nil {
		t.Fatalf("PreStart().Item = %#v, want nil", report.Item)
	}
}
