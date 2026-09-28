//go:build integration && !windows

package daemon

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
)

// E2E-003: `compozy session continue` through the CLI against a daemon with acpmock agents.
func TestDaemonE2ESessionContinueCLI(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should continue a session through the CLI and replay the recorded outcome", func(t *testing.T) {
		t.Parallel()
		runDaemonE2ESessionContinueCLI(t)
	})
}

func runDaemonE2ESessionContinueCLI(t *testing.T) {
	t.Helper()

	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
			cfg.Roles.AutoTitle.Enabled = false
			cfg.Roles.MemoryExtractor.Enabled = false
		}},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  mockFixturePath(t, "auto_title_fixture.json"),
			FixtureAgent: "auto-title-agent",
			AgentName:    "auto-title-agent",
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	source := createFixtureBackedSession(t, ctx, harness, "auto-title-agent", "Migration cleanup")
	if _, err := harness.PromptSession(ctx, source.ID, "Start the migration"); err != nil {
		t.Fatalf("PromptSession(source) error = %v", err)
	}

	args := []string{
		"session", "continue", source.ID, "--agent", "auto-title-agent",
		"--message", "Carry on with the migration; run the tests first.",
		"--idempotency-key", "idem_e2e_continue",
	}
	stdout, stderr, err := harness.CLI.RunInDir(ctx, harness.WorkspaceRoot, args...)
	if err != nil {
		t.Fatalf("session continue error = %v stderr=%s", err, stderr)
	}
	for _, want := range []string{
		"Continued " + source.ID + " (auto-title-agent) into ",
		"Origin", "continue · from " + source.ID,
		"Context", "Seed", "replay", "First prompt", "admitted",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("session continue stdout = %q, want %q", stdout, want)
		}
	}

	var replayed compozycontract.SessionDeriveResponse
	if err := harness.CLI.RunJSONInDir(
		ctx,
		harness.WorkspaceRoot,
		&replayed,
		append(args, "-o", "json")...); err != nil {
		t.Fatalf("session continue -o json (retry) error = %v", err)
	}
	if !replayed.Derived.Replayed || replayed.Session == nil ||
		replayed.Session.Lineage == nil || string(replayed.Session.Lineage.Kind) != "continue" ||
		replayed.Session.Lineage.ParentSessionID != source.ID || replayed.Derived.FirstPrompt != "admitted" {
		t.Fatalf("retry response = %+v, want the recorded continue outcome", replayed)
	}
	humanRetry, _, err := harness.CLI.RunInDir(ctx, harness.WorkspaceRoot, args...)
	if err != nil || !strings.Contains(humanRetry, "Replayed") {
		t.Fatalf("human retry stdout = %q err = %v, want Replayed yes", humanRetry, err)
	}

	assertCLIExitCode(t, ctx, harness, 2, "--agent is required", "session", "continue", source.ID)
	assertCLIExitCode(t, ctx, harness, 2, "--route cannot be combined with runtime flags",
		"session", "continue", source.ID, "--agent", "auto-title-agent", "--route", "2", "--speed", "fast")
	_, stderr, err = harness.CLI.RunInDir(ctx, harness.WorkspaceRoot,
		"session", "continue", source.ID, "--agent", "nope", "--idempotency-key", "idem_e2e_nope")
	if err == nil || !strings.Contains(stderr, `no agent named "nope"`) {
		t.Fatalf("session continue --agent nope err = %v stderr = %q, want agent not found", err, stderr)
	}
}

func assertCLIExitCode(
	t *testing.T,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	want int,
	message string,
	args ...string,
) {
	t.Helper()
	_, stderr, err := harness.CLI.RunInDir(ctx, harness.WorkspaceRoot, args...)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != want || !strings.Contains(stderr, message) {
		t.Fatalf("%v: err = %v stderr = %q, want exit %d with %q", args, err, stderr, want, message)
	}
}
