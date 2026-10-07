//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
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

	t.Run("Should leave an inactive source with stale metadata byte-for-byte unchanged", func(t *testing.T) {
		t.Parallel()
		runDaemonE2ESessionContinueStaleSourceCLI(t)
	})
}

// runDaemonE2ESessionContinueStaleSourceCLI continues a stopped source whose metadata still
// claims a live process through the CLI without fences: resolving the source (owner lookup,
// derive preview) and the derive itself never repair it.
func runDaemonE2ESessionContinueStaleSourceCLI(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()

	source := createFixtureBackedSession(t, ctx, harness, "auto-title-agent", "Migration cleanup")
	if _, err := harness.PromptSession(ctx, source.ID, "Start the migration"); err != nil {
		t.Fatalf("PromptSession(source) error = %v", err)
	}
	if err := harness.StopSession(ctx, source.ID); err != nil {
		t.Fatalf("StopSession(source) error = %v", err)
	}
	metaPath := store.SessionMetaFile(filepath.Join(harness.HomePaths.SessionsDir, source.ID))
	meta := mustReadSessionMeta(t, harness, source.ID)
	meta.State = "active"
	meta.Liveness = &store.SessionLivenessMeta{SubprocessPID: 999999}
	if err := store.WriteSessionMeta(metaPath, &meta); err != nil {
		t.Fatalf("WriteSessionMeta(stale source) error = %v", err)
	}
	before, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("ReadFile(source meta) error = %v", err)
	}

	var response compozycontract.SessionDeriveResponse
	if err := harness.CLI.RunJSONInDir(ctx, harness.WorkspaceRoot, &response,
		"session", "continue", source.ID, "--agent", "auto-title-agent",
		"--idempotency-key", "idem_e2e_stale_source", "-o", "json"); err != nil {
		t.Fatalf("session continue (stale source) error = %v", err)
	}
	if response.Derived.SourceSessionID != source.ID || response.Derived.ChildSessionID == "" {
		t.Fatalf("session continue (stale source) = %+v, want a child of %s", response.Derived, source.ID)
	}
	after, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("ReadFile(source meta after) error = %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("source meta changed by session continue:\nbefore=%s\nafter=%s", before, after)
	}
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
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
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

	_, stderr, err = harness.CLI.RunInDir(ctx, harness.WorkspaceRoot,
		"session", "continue", source.ID, "--agent", "nope", "--idempotency-key", "idem_e2e_nope")
	if err == nil || !strings.Contains(stderr, `no agent named "nope"`) {
		t.Fatalf("session continue --agent nope err = %v stderr = %q, want agent not found", err, stderr)
	}
}

// IT-013/IT-014/IT-015: `compozy session fork` against acpmock agents that clone natively,
// whose clone cannot load, and that cannot clone.
func TestDaemonE2ESessionForkCLI(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should fork natively, replay without fork support, and fall back when the clone cannot load",
		func(t *testing.T) {
			t.Parallel()
			runDaemonE2ESessionForkCLI(t)
		})
}

func runDaemonE2ESessionForkCLI(t *testing.T) {
	t.Helper()

	fixture := mockFixturePath(t, "session_fork_fixture.json")
	agents := []string{"fork-native-agent", "fork-load-missing-agent", "fork-replay-agent"}
	specs := make([]e2etest.MockAgentSpec, 0, len(agents))
	for _, agent := range agents {
		specs = append(specs, e2etest.MockAgentSpec{FixturePath: fixture, FixtureAgent: agent, AgentName: agent})
	}
	harness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) {
			cfg.Roles.AutoTitle.Enabled = false
			cfg.Roles.MemoryExtractor.Enabled = false
		}},
		MockAgents: specs,
	})
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()

	fork := func(agent string, key string) compozycontract.SessionDeriveResponse {
		t.Helper()
		source := createFixtureBackedSession(t, ctx, harness, agent, "Migration cleanup")
		if _, err := harness.PromptSession(ctx, source.ID, "Start the migration"); err != nil {
			t.Fatalf("PromptSession(%s source) error = %v", agent, err)
		}
		var response compozycontract.SessionDeriveResponse
		if err := harness.CLI.RunJSONInDir(ctx, harness.WorkspaceRoot, &response,
			"session", "fork", source.ID, "--idempotency-key", key, "-o", "json"); err != nil {
			t.Fatalf("session fork (%s) error = %v", agent, err)
		}
		if response.Session == nil || response.Derived.Kind != "fork" || response.Session.AgentName != agent ||
			response.Derived.SourceSessionID != source.ID {
			t.Fatalf("session fork (%s) = %+v, want a fork on the same agent", agent, response)
		}
		return response
	}
	derivationAfterPrompt := func(childID string, message string) *compozycontract.SessionDerivationPayload {
		t.Helper()
		if _, err := harness.PromptSession(ctx, childID, message); err != nil {
			t.Fatalf("PromptSession(child %q) error = %v", message, err)
		}
		child, err := harness.GetSession(ctx, childID)
		if err != nil || child.Derivation == nil {
			t.Fatalf("GetSession(child) = %+v, %v, want a derivation", child, err)
		}
		return child.Derivation
	}

	native := fork("fork-native-agent", "idem_e2e_fork_native")
	if native.Derived.Seed != "native_fork" || native.Derived.NativeState != "pending" ||
		native.Derived.ACPSessionID == "" || native.Session.Derivation == nil ||
		native.Session.Derivation.NativeState != "pending" {
		t.Fatalf("native fork = %+v, want seed native_fork pending with the clone id", native.Derived)
	}
	sourceEvents, _, err := harness.CLI.RunInDir(ctx, harness.WorkspaceRoot,
		"session", "events", native.Derived.SourceSessionID, "-o", "json")
	if err != nil || !strings.Contains(sourceEvents, "Started the migration.") ||
		strings.Contains(
			sourceEvents,
			"replayed history for",
		) || strings.Contains(sourceEvents, native.Derived.ACPSessionID) {
		t.Fatalf("source events = %q (%v), want no clone-id traffic in the source log", sourceEvents, err)
	}
	if derivation := derivationAfterPrompt(native.Session.ID, "Child ask"); derivation.NativeState != "loaded" {
		t.Fatalf("native child derivation = %+v, want loaded after the first prompt", derivation)
	}
	status, _, err := harness.CLI.RunInDir(ctx, harness.WorkspaceRoot, "session", "status", native.Session.ID)
	if err != nil || !strings.Contains(status, "seed native_fork · loaded") {
		t.Fatalf("session status = %q (%v), want the loaded Derivation line", status, err)
	}

	missing := fork("fork-load-missing-agent", "idem_e2e_fork_missing")
	if missing.Derived.Seed != "native_fork" || missing.Derived.NativeState != "pending" {
		t.Fatalf("load-missing fork = %+v, want a pending native fork", missing.Derived)
	}
	derivation := derivationAfterPrompt(missing.Session.ID, "Child fallback ask")
	if derivation.NativeState != "failed" || derivation.NativeForkError != "session/load: resource not found" {
		t.Fatalf("load-missing child derivation = %+v, want failed with the load error", derivation)
	}

	replay := fork("fork-replay-agent", "idem_e2e_fork_replay")
	if replay.Derived.Seed != "replay" || replay.Derived.NativeState != "" || replay.Derived.NativeForkError != "" {
		t.Fatalf("replay-only fork = %+v, want seed replay without a native attempt", replay.Derived)
	}
}
