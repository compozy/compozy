//go:build integration && !windows

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/testutil"
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

// Side-chat journey invariants belong here: real daemon transports, provider dispatch, and restart reconciliation.
func TestDaemonE2ESessionSide(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()
	t.Run("Should open a side during main work without writing to main", func(t *testing.T) {
		t.Parallel()
		h := startSideDaemonHarness(t)
		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
		defer cancel()
		source := createFixtureBackedSession(t, ctx, h, "side-web-agent", "Main work")
		if _, err := h.PromptSession(ctx, source.ID, "hello side"); err != nil {
			t.Fatal(err)
		}
		entered := make(chan struct{})
		var once sync.Once
		mainDone := make(chan error, 1)
		go func() {
			_, err := h.PromptSessionWithEvents(ctx, source.ID, "take a while", func(event e2etest.SSEEvent) error {
				if bytes.Contains(event.Data, []byte("main started the work")) {
					once.Do(func() { close(entered) })
				}
				return nil
			})
			mainDone <- err
		}()
		select {
		case <-entered:
		case err := <-mainDone:
			t.Fatalf("main ended before busy cut: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		var response struct {
			Prompt compozycontract.SendPromptResultPayload `json:"prompt"`
		}
		path := sideDaemonPath(h, source.ID)
		// IT-001
		if err := h.HTTPJSON(ctx, http.MethodPost, strings.TrimSuffix(path, "/sides")+"/prompt", compozycontract.SendPromptRequest{
			Message: "/side what now?", MessageID: "side-busy-message", IdempotencyKey: "side-busy-key",
		}, &response); err != nil {
			t.Fatal(err)
		}
		if response.Prompt.Status != "side_opened" || response.Prompt.Delivery != "none" || response.Prompt.Side == nil || response.Prompt.Side.Seed != "replay" || !response.Prompt.Side.SourceTurnInProgress {
			t.Fatalf("prompt response=%+v", response)
		}
		prompt := waitSideDaemonPrompt(t, h, "side-web-agent", response.Prompt.Side.SessionID)
		if !strings.Contains(prompt, "main started the work") || !strings.Contains(prompt, "compozy_turn_in_progress") || !strings.Contains(prompt, "<compozy_side_chat>") || !strings.Contains(prompt, "what now?") {
			t.Fatalf("side provider prompt=%s", prompt)
		}
		select {
		case err := <-mainDone:
			if err != nil {
				t.Fatal(err)
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
		transcript, err := h.SessionTranscript(ctx, source.ID)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(transcript)
		if bytes.Contains(encoded, []byte("/side")) || !bytes.Contains(encoded, []byte("main finished the work")) {
			t.Fatalf("source transcript=%s", encoded)
		}
		events, err := h.SessionEvents(ctx, source.ID)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ = json.Marshal(events)
		if bytes.Contains(encoded, []byte("what now?")) || bytes.Contains(encoded, []byte("side-busy-message")) {
			t.Fatalf("side input leaked to source: %s", encoded)
		}
	})
	t.Run("Should share native seed, transport replay, and multi-subscriber invalidation", func(t *testing.T) {
		t.Parallel()
		h := startSideDaemonHarness(t)
		ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
		defer cancel()
		source := createFixtureBackedSession(t, ctx, h, "side-native-agent", "Native main")
		if _, err := h.PromptSession(ctx, source.ID, "hello side"); err != nil {
			t.Fatal(err)
		}
		predicate := func(event e2etest.SSEEvent) bool {
			return event.Event == "session_side_changed" && bytes.Contains(event.Data, []byte(source.ID))
		}
		first, err := h.StartSessionCatalogHTTPStream(ctx, predicate)
		if err != nil {
			t.Fatal(err)
		}
		second, err := h.StartSessionCatalogHTTPStream(ctx, predicate)
		if err != nil {
			t.Fatal(err)
		}
		req := compozycontract.OpenSessionSideRequest{Message: "quick side question", IdempotencyKey: "native-side"}
		var opened compozycontract.SessionDeriveResponse
		// IT-002
		if err := h.HTTPJSON(ctx, http.MethodPost, sideDaemonPath(h, source.ID), req, &opened); err != nil {
			t.Fatal(err)
		}
		if opened.Derived.Seed != "native_fork" {
			t.Fatalf("native side=%+v", opened.Derived)
		}
		prompt := waitSideDaemonPrompt(t, h, "side-native-agent", opened.Derived.ChildSessionID)
		if !strings.Contains(prompt, "<compozy_side_chat>") || strings.Contains(prompt, "<compozy_context_replay>") {
			t.Fatalf("native prompt=%s", prompt)
		}
		// IT-006
		for _, stream := range []<-chan e2etest.SessionCatalogStreamResult{first, second} {
			select {
			case event := <-stream:
				if event.Err != nil {
					t.Fatal(event.Err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			var list compozycontract.SessionSidesResponse
			if err := h.HTTPJSON(ctx, http.MethodGet, sideDaemonPath(h, source.ID), nil, &list); err != nil || len(list.Sides) != 1 || list.Sides[0].ID != opened.Derived.ChildSessionID {
				t.Fatalf("subscriber reread=%+v %v", list, err)
			}
		}
		// IT-004: compare both transports once the provider turn has settled, so state is identical.
		testutil.Eventually(t, "side settled", 10*time.Second, func() bool {
			var list compozycontract.SessionSidesResponse
			return h.HTTPJSON(ctx, http.MethodGet, sideDaemonPath(h, source.ID), nil, &list) == nil && len(list.Sides) == 1 && !list.Sides[0].Prompting
		})
		var httpList, udsList compozycontract.SessionSidesResponse
		if err := h.HTTPJSON(ctx, http.MethodGet, sideDaemonPath(h, source.ID), nil, &httpList); err != nil {
			t.Fatal(err)
		}
		if err := h.UDSJSON(ctx, http.MethodGet, sideDaemonPath(h, source.ID), nil, &udsList); err != nil {
			t.Fatal(err)
		}
		left, _ := json.Marshal(httpList)
		right, _ := json.Marshal(udsList)
		if !bytes.Equal(left, right) {
			t.Fatalf("transport list mismatch: %s / %s", left, right)
		}
		var httpReplay, udsReplay compozycontract.SessionDeriveResponse
		if err := h.HTTPJSON(ctx, http.MethodPost, sideDaemonPath(h, source.ID), req, &httpReplay); err != nil {
			t.Fatal(err)
		}
		if err := h.UDSJSON(ctx, http.MethodPost, sideDaemonPath(h, source.ID), req, &udsReplay); err != nil {
			t.Fatal(err)
		}
		left, _ = json.Marshal(httpReplay)
		right, _ = json.Marshal(udsReplay)
		if !bytes.Equal(left, right) || !udsReplay.Derived.Replayed {
			t.Fatalf("transport POST mismatch: %s / %s", left, right)
		}
	})
	t.Run("Should reconcile a committed staged side and restore its source family after restart", func(t *testing.T) {
		t.Parallel()
		h := startSideDaemonHarness(t)
		ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
		defer cancel()
		source := createFixtureBackedSession(t, ctx, h, "side-web-agent", "Recovery main")
		if _, err := h.PromptSession(ctx, source.ID, "hello side"); err != nil {
			t.Fatal(err)
		}
		var side compozycontract.SessionDeriveResponse
		if err := h.HTTPJSON(ctx, http.MethodPost, sideDaemonPath(h, source.ID), compozycontract.OpenSessionSideRequest{Message: "quick side question", IdempotencyKey: "recovery-template"}, &side); err != nil {
			t.Fatal(err)
		}
		_ = waitSideDaemonPrompt(t, h, "side-web-agent", side.Derived.ChildSessionID)
		if err := h.Stop(ctx); err != nil {
			t.Fatal(err)
		}
		// IT-003: arrange the exact durable pre-admission crash window, using store writers rather than SQL edits.
		stagedID := seedStagedSideDaemonFixture(t, ctx, h, side.Derived.ChildSessionID)
		h.Restart(t)
		var list compozycontract.SessionSidesResponse
		if err := h.HTTPJSON(ctx, http.MethodGet, sideDaemonPath(h, source.ID), nil, &list); err != nil || len(list.Sides) != 2 {
			t.Fatalf("reconciled sides=%+v %v", list, err)
		}
		var derived map[string]any
		if err := h.HTTPJSON(ctx, http.MethodGet, "/api/events?session_id="+stagedID+"&type=session.derived", nil, &derived); err != nil {
			t.Fatal(err)
		}
		encoded, _ := json.Marshal(derived)
		if !bytes.Contains(encoded, []byte(stagedID)) {
			t.Fatalf("missing reconciled derivation: %s", encoded)
		}
		if _, err := h.PromptSession(ctx, stagedID, "what is this repo?"); err != nil {
			t.Fatal(err)
		}
		for _, suffix := range []string{"/archive?stop=true", "/unarchive"} {
			if err := h.HTTPJSON(ctx, http.MethodPost, strings.TrimSuffix(sideDaemonPath(h, source.ID), "/sides")+suffix, nil, nil); err != nil {
				t.Fatal(err)
			}
		}
		if err := h.HTTPJSON(ctx, http.MethodGet, sideDaemonPath(h, source.ID), nil, &list); err != nil || len(list.Sides) != 2 {
			t.Fatalf("restored sides=%+v %v", list, err)
		}
	})
}

func startSideDaemonHarness(t *testing.T) *e2etest.RuntimeHarness {
	t.Helper()
	fixture := mockFixturePath(t, "browser_side_chat_fixture.json")
	return e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{Mutate: func(cfg *compozyconfig.Config) { cfg.Roles.AutoTitle.Enabled = false }},
		MockAgents: []e2etest.MockAgentSpec{{FixturePath: fixture, FixtureAgent: "side-web-agent", AgentName: "side-web-agent"}, {FixturePath: fixture, FixtureAgent: "side-native-agent", AgentName: "side-native-agent"}},
	})
}
func sideDaemonPath(h *e2etest.RuntimeHarness, id string) string {
	return fmt.Sprintf("/api/workspaces/%s/sessions/%s/sides", h.WorkspaceID, id)
}
func waitSideDaemonPrompt(t *testing.T, h *e2etest.RuntimeHarness, agent, id string) string {
	t.Helper()
	var prompt string
	testutil.Eventually(t, "side provider dispatch", 10*time.Second, func() bool {
		records, err := acpmock.ReadDiagnostics(h.MockAgents[agent].DiagnosticsPath)
		if err != nil {
			return false
		}
		for _, record := range acpmock.PromptDiagnostics(acpmock.DiagnosticsForCompozySession(records, id)) {
			prompt = record.Prompt
			return true
		}
		return false
	})
	return prompt
}

func seedStagedSideDaemonFixture(t *testing.T, ctx context.Context, h *e2etest.RuntimeHarness, templateID string) string {
	t.Helper()
	db, err := globaldb.OpenGlobalDB(ctx, h.HomePaths.DatabaseFile)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(ctx); err != nil {
			t.Error(err)
		}
	}()
	rows, err := db.ListSessions(ctx, store.SessionListQuery{ReadScope: store.ReadScope{AllProfiles: true}, ID: templateID, Archive: store.SessionArchiveInclude, Limit: 1})
	if err != nil || len(rows) != 1 {
		t.Fatalf("template=%+v %v", rows, err)
	}
	identity, err := db.GetSessionCreationIdentity(ctx, templateID)
	if err != nil {
		t.Fatal(err)
	}
	meta := mustReadSessionMeta(t, h, templateID)
	meta.ID = "sess-side-staged-recovery"
	meta.State = "stopped"
	meta.ACPSessionID = ""
	meta.Liveness = nil
	meta.RuntimeStatus = store.SessionRuntimeUnbound
	meta.ImportedContext.Consumed = nil
	meta.Derivation.IdempotencyKey = "staged-side-recovery"
	meta.Derivation.FirstPrompt = store.SessionFirstPrompt{State: store.SessionDerivationFirstPromptStaged}
	dir := filepath.Join(h.HomePaths.SessionsDir, meta.ID)
	if err := store.WriteSessionMeta(store.SessionMetaFile(dir), &meta); err != nil {
		t.Fatal(err)
	}
	recorder, err := sessiondb.OpenSessionDB(ctx, store.SessionDBOwner{SessionID: meta.ID, WorkspaceID: meta.WorkspaceID}, store.SessionDBFile(dir))
	if err != nil {
		t.Fatal(err)
	}
	if err := recorder.Close(ctx); err != nil {
		t.Fatal(err)
	}
	row := rows[0]
	row.ID = meta.ID
	row.ACPSessionID = nil
	row.State = "stopped"
	row.RuntimeStatus = store.SessionRuntimeUnbound
	receipt := store.SessionDerivationReceipt{WorkspaceID: row.WorkspaceID, ProfileID: row.ProfileID, SourceSessionID: meta.Lineage.ParentSessionID, ChildSessionID: meta.ID, Kind: store.LineageKindSide, IdempotencyKey: meta.Derivation.IdempotencyKey, RequestFingerprint: "staged-crash-fixture", CreatedAt: time.Now().UTC(), Outcome: store.SessionDerivationOutcome{Seed: "replay", FirstPrompt: "staged"}}
	if err := db.RegisterDerivedSession(ctx, &row, identity, receipt); err != nil {
		t.Fatal(err)
	}
	return meta.ID
}
