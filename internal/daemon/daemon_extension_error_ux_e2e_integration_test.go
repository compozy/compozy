//go:build integration && !windows

// Suite: stamped extension error UX journey
// Invariant: common extension failures expose the same actionable diagnostic in human and structured CLI output.
// Boundary IN: real stamped CLI, daemon transport, and extension install policy.
// Boundary OUT: diagnostic authoring details, owned by the originating domain suites.
package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	"github.com/compozy/compozy/internal/testutil"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	"golang.org/x/sys/execabs"
)

func TestDaemonE2EExtensionErrorUXPreservesStructuredRemediationInHumanStderr(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	repoRoot := extensionAuthoringE2ERepoRoot(t)
	binaryPath := buildStampedExtensionAuthoringBinary(t, ctx, repoRoot)

	t.Run("Should preserve policy-blocked install remediation", func(t *testing.T) {
		harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			BinaryPath: binaryPath,
			ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
				cfg.Extensions.Trust.AllowUnverified = false
			}},
			StartTimeout: 30 * time.Second,
		})
		sourceDir := filepath.Join(harness.WorkspaceRoot, "policy-blocked")
		var scaffold extensionpkg.ScaffoldResult
		runExtensionAuthoringCLI(
			t,
			ctx,
			harness,
			&scaffold,
			"extension", "init", "policy-blocked",
			"--template", "tool-provider-go",
			"--dir", sourceDir,
			"-o", "json",
		)
		configureExtensionAuthoringSDKReplace(t, ctx, scaffold.Directory, repoRoot)
		var build extensionpkg.BuildResult
		runExtensionAuthoringCLI(
			t,
			ctx,
			harness,
			&build,
			"extension", "build", scaffold.Directory,
			"-o", "json",
		)

		assertStampedExtensionErrorParity(t, func(args ...string) (string, string, error) {
			return harness.CLI.RunInDir(ctx, harness.WorkspaceRoot, args...)
		}, "extension", "install", build.GenerationDir, "--allow-unverified", "--yes")
	})

	t.Run("Should preserve daemon-down remediation", func(t *testing.T) {
		homePaths := e2etest.NewHomePaths(t)
		e2etest.SeedConfig(t, homePaths, e2etest.ConfigSeedOptions{})
		env := extensionErrorUXEnv(homePaths.HomeDir)
		workdir := t.TempDir()

		assertStampedExtensionErrorParity(t, func(args ...string) (string, string, error) {
			return runStampedExtensionCLI(ctx, binaryPath, workdir, env, args...)
		}, "status")
	})
}

func assertStampedExtensionErrorParity(
	t *testing.T,
	run func(...string) (string, string, error),
	args ...string,
) {
	t.Helper()

	_, humanStderr, humanErr := run(args...)
	if humanErr == nil {
		t.Fatalf("CLI %q human error = nil, want failure", strings.Join(args, " "))
	}
	jsonArgs := append(append([]string(nil), args...), "-o", "json")
	_, jsonStderr, jsonErr := run(jsonArgs...)
	if jsonErr == nil {
		t.Fatalf("CLI %q JSON error = nil, want failure", strings.Join(args, " "))
	}

	var payload compozycontract.ErrorPayload
	if err := json.Unmarshal([]byte(strings.TrimSpace(jsonStderr)), &payload); err != nil {
		t.Fatalf("json.Unmarshal(%q stderr) error = %v; stderr=%q", strings.Join(args, " "), err, jsonStderr)
	}
	if payload.Diagnostic == nil {
		t.Fatalf("CLI %q structured diagnostic = nil; payload=%#v", strings.Join(args, " "), payload)
	}
	for _, want := range []string{
		"error: " + payload.Diagnostic.Title,
		payload.Diagnostic.Message,
		"try: " + payload.Diagnostic.SuggestedCommand,
	} {
		if strings.TrimSpace(want) == "" || !strings.Contains(humanStderr, want) {
			t.Fatalf("CLI %q human stderr = %q, want structured remediation %q", args, humanStderr, want)
		}
	}
}

func runStampedExtensionCLI(
	ctx context.Context,
	binaryPath string,
	workdir string,
	env []string,
	args ...string,
) (string, string, error) {
	// #nosec G204 -- this E2E helper intentionally invokes the stamped test binary with fixed test arguments.
	command := execabs.CommandContext(ctx, binaryPath, args...)
	command.Dir = workdir
	command.Env = append([]string(nil), env...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	return stdout.String(), stderr.String(), err
}

func extensionErrorUXEnv(homeDir string) []string {
	base := testutil.HermeticProcessEnv(os.Environ())
	env := make([]string, 0, len(base)+2)
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok && (key == "HOME" || key == "COMPOZY_HOME") {
			continue
		}
		env = append(env, entry)
	}
	return append(env, "HOME="+homeDir, "COMPOZY_HOME="+homeDir)
}
