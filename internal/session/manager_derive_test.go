package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	acpsdk "github.com/coder/acp-go-sdk"

	"github.com/compozy/compozy/internal/acp"
	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
)

const (
	deriveRouteOneCommand = "SEAT=1 codex --acp"
	deriveRouteTwoCommand = "SEAT=2 codex --acp"
)

type deriveHarness struct {
	*harness
	db *globaldb.GlobalDB
}

// newDeriveHarness wires a real global DB as catalog, creation/derivation store,
// prompt admission store, and ledger, plus a second agent "b" (codex) that declares two
// routes with distinct commands.
// Extra options receive the harness DB so fault-injecting wrappers can embed it; they
// override the default wiring.
func newDeriveHarness(t *testing.T, extraOpts ...func(*globaldb.GlobalDB) Option) *deriveHarness {
	t.Helper()
	h := newHarness(t)
	db := openManagerInputQueueStore(t)
	registerManagerInputQueueWorkspace(t, db, h)
	setDeriveAgentB(t, h, deriveRoutes(h, deriveRouteOneCommand, deriveRouteTwoCommand))
	dh := &deriveHarness{harness: h, db: db}
	dh.restartManager(t, extraOpts...)
	return dh
}

// restartManager replaces the harness manager with a fresh one over the same home and DB.
func (h *deriveHarness) restartManager(t *testing.T, extraOpts ...func(*globaldb.GlobalDB) Option) {
	t.Helper()
	opts := []Option{
		WithSessionCatalog(h.db), WithEventLedger(h.db), WithSessionPromptAdmissionStore(h.db),
	}
	for _, extra := range extraOpts {
		opts = append(opts, extra(h.db))
	}
	h.manager = newManagerWithHarness(t, h.harness, opts...)
	cleanupTestManager(t, h.manager)
}

func deriveRoutes(h *harness, commands ...string) []compozyconfig.RoleFallback {
	routes := make([]compozyconfig.RoleFallback, 0, len(commands))
	for _, command := range commands {
		routes = append(routes, compozyconfig.RoleFallback{
			Provider: "codex", Model: h.cfg.Providers["codex"].Models.Default, Command: command,
		})
	}
	return routes
}

func setDeriveAgentB(t *testing.T, h *harness, chain []compozyconfig.RoleFallback) {
	t.Helper()
	workspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	agents := make([]compozyconfig.AgentDef, 0, len(workspace.Agents)+1)
	for _, agent := range workspace.Agents {
		if agent.Name != "b" {
			agents = append(agents, agent)
		}
	}
	workspace.Agents = append(agents, compozyconfig.AgentDef{
		Name: "b", Provider: "codex", Prompt: "You are agent b.", FallbackChain: chain,
	})
	h.resolver.upsert(&workspace)
}

// newDeriveSource creates a logical user session on "coder" and runs two settled turns.
func (h *deriveHarness) newDeriveSource(t *testing.T) *Session {
	t.Helper()
	created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
		Session: CreateOpts{AgentName: "coder", Name: "Migration cleanup", Workspace: h.workspaceID},
	})
	if err != nil {
		t.Fatalf("CreateAccepted(source) error = %v", err)
	}
	source, ok := h.manager.Get(created.ID)
	if !ok {
		t.Fatalf("Get(%q) did not find the source", created.ID)
	}
	for _, text := range []string{"Start the migration", "Now run the tests"} {
		events, err := h.manager.Prompt(testutil.Context(t), source.ID, text)
		if err != nil {
			t.Fatalf("Prompt(%q) error = %v", text, err)
		}
		collectEvents(t, events)
	}
	return source
}

func (h *deriveHarness) continueOpts(source *Session, key string) ContinueSessionOpts {
	return ContinueSessionOpts{
		SourceSessionID: source.ID, WorkspaceID: h.workspaceID, AgentName: "b", IdempotencyKey: key,
	}
}

func (h *deriveHarness) promptMessages() []string {
	h.driver.mu.Lock()
	defer h.driver.mu.Unlock()
	messages := make([]string, 0, len(h.driver.promptCalls))
	for _, call := range h.driver.promptCalls {
		messages = append(messages, call.Message)
	}
	return messages
}

func (h *deriveHarness) waitForPromptCount(t *testing.T, want int) []string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		messages := h.promptMessages()
		if len(messages) >= want {
			return messages
		}
		if time.Now().After(deadline) {
			t.Fatalf("prompt calls = %d, want %d", len(messages), want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (h *deriveHarness) derivedLedgerRows(t *testing.T, sessionID string) []store.EventSummary {
	t.Helper()
	rows, err := h.db.ListEventSummaries(testutil.Context(t), store.EventSummaryQuery{
		ReadScope: store.ReadScope{AllProfiles: true}, Type: eventspkg.SessionDerived,
		SessionID: sessionID, Limit: 10,
	})
	if err != nil {
		t.Fatalf("ListEventSummaries() error = %v", err)
	}
	return rows
}

func (h *deriveHarness) childMeta(t *testing.T, childID string) store.SessionMeta {
	t.Helper()
	child, ok := h.manager.Get(childID)
	if !ok {
		t.Fatalf("Get(%q) did not find the child", childID)
	}
	return readMeta(t, child.MetaPath())
}

func TestContinueSession(t *testing.T) {
	t.Parallel()

	t.Run("Should commit the child, carry the context in its first prompt, and leave the source untouched",
		func(t *testing.T) {
			t.Parallel()
			h := newDeriveHarness(t)
			source := h.newDeriveSource(t)
			sourceMetaBefore, err := os.ReadFile(source.MetaPath())
			if err != nil {
				t.Fatalf("ReadFile(source meta) error = %v", err)
			}
			sourceEventsBefore := len(readStoredEvents(t, source))
			sourceInfoBefore := source.Info()
			promptsBefore := len(h.promptMessages())

			opts := h.continueOpts(source, "idem_continue_1")
			opts.Message = "go"
			result, err := h.manager.ContinueSession(testutil.Context(t), opts)
			if err != nil {
				t.Fatalf("ContinueSession() error = %v", err)
			}
			child := result.Child
			lineage := store.NormalizeSessionLineage(child.ID, child.Lineage)
			if child.AgentName != "b" || lineage.ParentSessionID != source.ID || lineage.SpawnDepth != 1 ||
				lineage.Kind != store.LineageKindContinue || lineage.OriginAgentName != "coder" {
				t.Fatalf("child = %s lineage %+v, want agent b continued from %s", child.AgentName, lineage, source.ID)
			}
			if result.Seed != DeriveSeedReplay || result.FirstPrompt != store.SessionDerivationFirstPromptAdmitted ||
				result.Replayed || result.ReplayMessageCount != 4 || result.ThroughTurnID == "" {
				t.Fatalf("result = %+v, want replay seed, admitted, 4 messages, a through turn", result)
			}
			meta := h.childMeta(t, child.ID)
			if meta.Derivation == nil || meta.Derivation.IdempotencyKey != "idem_continue_1" ||
				meta.Derivation.FirstPrompt.AdmissionKey != "idem_continue_1:first" {
				t.Fatalf("child derivation = %+v, want key and first admission key", meta.Derivation)
			}
			imported := meta.ImportedContext
			if imported == nil || imported.MessageCount != 4 || imported.Bytes == 0 ||
				imported.ThroughTurnID != result.ThroughTurnID || imported.SourceMaxSequence == 0 {
				t.Fatalf("imported context = %+v, want 4 messages through %s", imported, result.ThroughTurnID)
			}

			prompts := h.waitForPromptCount(t, promptsBefore+1)
			first := prompts[promptsBefore]
			replayed, err := h.manager.ContinueSession(testutil.Context(t), opts)
			if err != nil || !replayed.Replayed || replayed.ChildSessionID != child.ID {
				t.Fatalf("ContinueSession(retry) = %+v, %v, want the recorded child", replayed, err)
			}
			if got := len(h.promptMessages()); got != promptsBefore+1 {
				t.Fatalf("prompt calls after retry = %d, want the first message sent once", got-promptsBefore)
			}
			for _, want := range []string{
				"Context rebuilt from log.\nThis session continues " + source.ID + " (agent: coder).",
				"Now run the tests",
				resumeReplayCloseTag + "\n\nUser request:\n\ngo",
			} {
				if !strings.Contains(first, want) {
					t.Fatalf("child first prompt = %q, want it to contain %q", first, want)
				}
			}
			waitForDeriveConsumption(t, h, child.ID)

			receipt, found, err := h.db.SessionDerivationReceipt(testutil.Context(t), h.workspaceID, "idem_continue_1")
			if err != nil || !found || receipt.ChildSessionID != child.ID ||
				receipt.Outcome.ReplayBytes != result.ReplayBytes {
				t.Fatalf("receipt = %+v found=%t err=%v, want the child's outcome", receipt, found, err)
			}
			rows := h.derivedLedgerRows(t, child.ID)
			if len(rows) != 1 {
				t.Fatalf("session.derived rows = %d, want 1", len(rows))
			}
			var payload map[string]any
			if err := json.Unmarshal(rows[0].ContentValue(), &payload); err != nil {
				t.Fatalf("decode session.derived payload: %v", err)
			}
			for _, key := range []string{
				"kind", "source_session_id", "origin_agent_name", "through_turn_id", "seed",
				"replay_message_count", "replay_bytes", "truncated", "omitted_count", "first_prompt",
				"idempotency_key",
			} {
				if _, ok := payload[key]; !ok {
					t.Fatalf("session.derived payload = %v, missing %q", payload, key)
				}
			}
			if _, err := store.LookupSessionDBOwner(testutil.Context(t), h.db, child.ID); err != nil {
				t.Fatalf("catalog row for child: %v", err)
			}

			sourceMetaAfter, err := os.ReadFile(source.MetaPath())
			if err != nil {
				t.Fatalf("ReadFile(source meta) error = %v", err)
			}
			sourceInfoAfter := source.Info()
			if !bytes.Equal(sourceMetaBefore, sourceMetaAfter) ||
				len(readStoredEvents(t, source)) != sourceEventsBefore ||
				sourceInfoAfter.TranscriptEpoch != sourceInfoBefore.TranscriptEpoch ||
				sourceInfoAfter.ACPSessionID != sourceInfoBefore.ACPSessionID ||
				sourceInfoAfter.RuntimeStatus != sourceInfoBefore.RuntimeStatus {
				t.Fatal("ContinueSession() changed the source session")
			}
		})

	t.Run("Should stage the context until the first prompt and carry it only once", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_staged"))
		if err != nil {
			t.Fatalf("ContinueSession() error = %v", err)
		}
		if result.FirstPrompt != store.SessionDerivationFirstPromptStaged {
			t.Fatalf("FirstPrompt = %q, want staged", result.FirstPrompt)
		}
		before := len(h.promptMessages())
		sendDeriveChildPrompt(t, h, result.Child.ID, "hi", "msg_hi", "idem_hi")
		sendDeriveChildPrompt(t, h, result.Child.ID, "again", "msg_again", "idem_again")
		prompts := h.promptMessages()
		if len(prompts) != before+2 {
			t.Fatalf("prompt calls = %d, want %d", len(prompts), before+2)
		}
		if !strings.Contains(prompts[before], resumeReplayOpenTag) || !strings.HasSuffix(prompts[before], "hi") {
			t.Fatalf("first child prompt = %q, want the carried context before hi", prompts[before])
		}
		if strings.Contains(prompts[before+1], resumeReplayOpenTag) {
			t.Fatalf("second child prompt = %q, want no carried context", prompts[before+1])
		}
		consumed := h.childMeta(t, result.Child.ID).ImportedContext.Consumed
		if consumed == nil || consumed.AdmissionKey != "idem_hi" || consumed.MessageID != "msg_hi" {
			t.Fatalf("consumed = %+v, want the hi admission", consumed)
		}
	})

	t.Run("Should return the recorded outcome on retry, conflict on a different request", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		first, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_retry"))
		if err != nil {
			t.Fatalf("ContinueSession() error = %v", err)
		}
		events, err := h.manager.Prompt(testutil.Context(t), source.ID, "source moved on")
		if err != nil {
			t.Fatalf("Prompt(source) error = %v", err)
		}
		collectEvents(t, events)
		stale := int64(0)
		retry := h.continueOpts(source, "idem_retry")
		retry.Fences = DeriveFences{ExpectedEpoch: &stale, ExpectedGeneration: &stale, ExpectedMaxSequence: &stale}
		second, err := h.manager.ContinueSession(testutil.Context(t), retry)
		if err != nil {
			t.Fatalf("ContinueSession(retry) error = %v", err)
		}
		if !second.Replayed || second.Child == nil || second.Child.ID != first.Child.ID ||
			second.ReplayBytes != first.ReplayBytes {
			t.Fatalf("retry = %+v, want the recorded outcome for %s", second, first.Child.ID)
		}
		conflict := h.continueOpts(source, "idem_retry")
		conflict.AgentName = "coder"
		if _, err := h.manager.ContinueSession(
			testutil.Context(t),
			conflict,
		); !errors.Is(
			err,
			ErrDeriveIdempotencyConflict,
		) {
			t.Fatalf("ContinueSession(conflict) error = %v, want ErrDeriveIdempotencyConflict", err)
		}
		if err := h.db.MarkDerivationChildDeleted(testutil.Context(t), first.Child.ID, time.Now()); err != nil {
			t.Fatalf("MarkDerivationChildDeleted() error = %v", err)
		}
		deleted, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_retry"))
		if err != nil || !deleted.Replayed || !deleted.ChildDeleted || deleted.Child != nil {
			t.Fatalf("retry after deletion = %+v, %v, want replayed child_deleted", deleted, err)
		}
	})

	t.Run("Should create exactly one child for racing identical requests", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		const callers = 8
		ids := make([]string, callers)
		var wg sync.WaitGroup
		for index := range callers {
			wg.Go(func() {
				result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_race"))
				if err != nil {
					t.Errorf("ContinueSession(%d) error = %v", index, err)
					return
				}
				ids[index] = result.ChildSessionID
			})
		}
		wg.Wait()
		for _, id := range ids[1:] {
			if id != ids[0] {
				t.Fatalf("child ids = %v, want one child", ids)
			}
		}
		if rows := h.derivedLedgerRows(t, ids[0]); len(rows) != 1 {
			t.Fatalf("session.derived rows = %d, want 1", len(rows))
		}
		entries, err := os.ReadDir(h.homePaths.SessionsDir)
		if err != nil {
			t.Fatalf("ReadDir(sessions) error = %v", err)
		}
		dirs := make([]string, 0, len(entries))
		for _, entry := range entries {
			if entry.IsDir() {
				dirs = append(dirs, entry.Name())
			}
		}
		if len(dirs) != 2 {
			t.Fatalf("session directories = %v, want only the source and the one child (losers swept)", dirs)
		}
	})

	t.Run("Should refuse unknown agents, archived sources, and stale fences without a receipt", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		unknown := h.continueOpts(source, "idem_unknown")
		unknown.AgentName = "nope"
		if _, err := h.manager.ContinueSession(testutil.Context(t), unknown); !errors.Is(err, ErrDeriveAgentNotFound) ||
			err.Error() != `no agent named "nope"` {
			t.Fatalf("ContinueSession(nope) error = %v, want agent not found", err)
		}
		stale := int64(99)
		fenced := h.continueOpts(source, "idem_fenced")
		fenced.Fences = DeriveFences{ExpectedEpoch: &stale, ExpectedGeneration: &stale, ExpectedMaxSequence: &stale}
		if _, err := h.manager.ContinueSession(testutil.Context(t), fenced); !errors.Is(err, ErrDeriveFenceConflict) {
			t.Fatalf("ContinueSession(stale fences) error = %v, want ErrDeriveFenceConflict", err)
		}
		for _, key := range []string{"idem_unknown", "idem_fenced"} {
			if _, found, err := h.db.SessionDerivationReceipt(
				testutil.Context(t),
				h.workspaceID,
				key,
			); err != nil ||
				found {
				t.Fatalf("receipt %s found=%t err=%v, want none", key, found, err)
			}
		}
		if err := h.manager.Stop(testutil.Context(t), source.ID); err != nil {
			t.Fatalf("Stop(source) error = %v", err)
		}
		if _, err := h.manager.Archive(testutil.Context(t), h.workspaceID, source.ID); err != nil {
			t.Fatalf("Archive(source) error = %v", err)
		}
		_, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_archived"))
		if !errors.Is(err, ErrDeriveSourceArchived) || !errors.Is(err, ErrSessionArchived) {
			t.Fatalf("ContinueSession(archived) error = %v, want ErrDeriveSourceArchived", err)
		}
	})

	t.Run("Should record the declared route and bind the first prompt on its command", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		outOfRange := h.continueOpts(source, "idem_route_3")
		outOfRange.Route = 3
		if _, err := h.manager.ContinueSession(
			testutil.Context(t),
			outOfRange,
		); !errors.Is(
			err,
			ErrDeriveRouteNotFound,
		) {
			t.Fatalf("ContinueSession(route 3) error = %v, want ErrDeriveRouteNotFound", err)
		}
		routed := h.continueOpts(source, "idem_route_2")
		routed.Route = 2
		result, err := h.manager.ContinueSession(testutil.Context(t), routed)
		if err != nil {
			t.Fatalf("ContinueSession(route 2) error = %v", err)
		}
		pending := h.childMeta(t, result.Child.ID).Derivation.PendingRoute
		if pending == nil || pending.Index != 2 ||
			pending.CommandFingerprint != compozyconfig.CommandFingerprint(deriveRouteTwoCommand) {
			t.Fatalf("pending route = %+v, want route 2 with its fingerprint", pending)
		}
		startsBefore := len(h.driver.startCalls)
		sendDeriveChildPrompt(t, h, result.Child.ID, "go", "msg_route_go", "idem_route_go")
		h.driver.mu.Lock()
		command := h.driver.startCalls[startsBefore].Command
		h.driver.mu.Unlock()
		if command != deriveRouteTwoCommand {
			t.Fatalf("first bind command = %q, want route 2's command", command)
		}
		if h.childMeta(t, result.Child.ID).Derivation.PendingRoute != nil {
			t.Fatal("pending route kept after it bound")
		}
	})

	t.Run("Should fail the first bind with route_not_found when the route was removed", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		routed := h.continueOpts(source, "idem_route_removed")
		routed.Route = 2
		result, err := h.manager.ContinueSession(testutil.Context(t), routed)
		if err != nil {
			t.Fatalf("ContinueSession(route 2) error = %v", err)
		}
		setDeriveAgentB(t, h.harness, deriveRoutes(h.harness, deriveRouteOneCommand))
		startsBefore := len(h.driver.startCalls)
		_, err = h.manager.SendPrompt(testutil.Context(t), result.Child.ID, SendPromptOpts{Message: "go"})
		if !errors.Is(err, ErrDeriveRouteNotFound) {
			t.Fatalf("SendPrompt() error = %v, want ErrDeriveRouteNotFound", err)
		}
		child, _ := h.manager.Get(result.Child.ID)
		info := child.Info()
		if info.ACPSessionID != "" || len(h.driver.startCalls) != startsBefore ||
			!strings.HasPrefix(info.RuntimeFailure, "route_not_found: agent \"b\" declares 1 route(s); route 2") {
			t.Fatalf("child after failed bind = acp %q failure %q, want unbound with route_not_found",
				info.ACPSessionID, info.RuntimeFailure)
		}
	})
}

func TestDeriveCarriedContextLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should keep a staged context across a manager restart", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_restart"))
		if err != nil {
			t.Fatalf("ContinueSession() error = %v", err)
		}
		if err := h.manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		h.restartManager(t)
		if _, err := h.manager.Resume(testutil.Context(t), result.Child.ID); err != nil {
			t.Fatalf("Resume(child) error = %v", err)
		}
		before := len(h.promptMessages())
		sendDeriveChildPrompt(t, h, result.Child.ID, "hi after restart", "msg_restart", "idem_restart_hi")
		prompts := h.promptMessages()
		if len(prompts) != before+1 || !strings.Contains(prompts[before], "Start the migration") ||
			!strings.HasSuffix(prompts[before], "hi after restart") {
			t.Fatalf("prompt after restart = %q, want the carried context before hi", prompts[len(prompts)-1])
		}
	})

	t.Run("Should flatten a continue of a continued child exactly once", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		b, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_ab"))
		if err != nil {
			t.Fatalf("ContinueSession(A→B) error = %v", err)
		}
		sendDeriveChildPrompt(t, h, b.Child.ID, "B own turn", "msg_b", "idem_b")
		childB, _ := h.manager.Get(b.Child.ID)
		c, err := h.manager.ContinueSession(testutil.Context(t), ContinueSessionOpts{
			SourceSessionID: childB.ID, WorkspaceID: h.workspaceID, AgentName: "coder", IdempotencyKey: "idem_bc",
		})
		if err != nil {
			t.Fatalf("ContinueSession(B→C) error = %v", err)
		}
		imported := h.childMeta(t, c.Child.ID).ImportedContext
		var messages []transcript.Message
		if err := json.Unmarshal([]byte(imported.MessagesJSON), &messages); err != nil {
			t.Fatalf("decode C imported context: %v", err)
		}
		contents := make([]string, 0, len(messages))
		for _, message := range messages {
			contents = append(contents, message.Content)
		}
		joined := strings.Join(contents, "|")
		if !strings.Contains(joined, "Start the migration") || !strings.Contains(joined, "B own turn") ||
			strings.Contains(joined, resumeReplayOpenTag) || strings.Contains(joined, "This session continues") {
			t.Fatalf("C carried = %q, want A's messages then B's own, never nested framing", joined)
		}
		if strings.Index(joined, "Start the migration") > strings.Index(joined, "B own turn") {
			t.Fatalf("C carried = %q, want A's messages before B's", joined)
		}
	})

	t.Run("Should prepend the imported context when the child's history is rebuilt", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_rebuild"))
		if err != nil {
			t.Fatalf("ContinueSession() error = %v", err)
		}
		sendDeriveChildPrompt(t, h, result.Child.ID, "child turn", "msg_child", "idem_child")
		child, _ := h.manager.Get(result.Child.ID)
		block, count, err := h.manager.buildResumeReplay(testutil.Context(t), child)
		if err != nil {
			t.Fatalf("buildResumeReplay() error = %v", err)
		}
		if strings.Count(block, resumeReplayOpenTag) != 1 || !strings.HasPrefix(block, contextRebuiltMarkerSummary) ||
			strings.Index(block, "Start the migration") > strings.Index(block, "child turn") || count < 5 {
			t.Fatalf("rebuild block = %q (%d messages), want imported context before the child's own", block, count)
		}
	})

	t.Run("Should discard the imported context when the child conversation is cleared", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_clear"))
		if err != nil {
			t.Fatalf("ContinueSession() error = %v", err)
		}
		cleared, err := h.manager.ClearConversation(testutil.Context(t), result.Child.ID)
		if err != nil {
			t.Fatalf("ClearConversation() error = %v", err)
		}
		meta := readMeta(t, cleared.MetaPath())
		lineage := store.NormalizeSessionLineage(meta.ID, meta.Lineage)
		if meta.ImportedContext != nil || meta.Derivation != nil || lineage.Kind != store.LineageKindContinue {
			t.Fatalf("cleared meta derivation=%v imported=%v lineage=%+v, want both gone and lineage kept",
				meta.Derivation, meta.ImportedContext, lineage)
		}
	})
}

func TestDerivePreview(t *testing.T) {
	t.Parallel()

	t.Run("Should report the numbers the continue records without writing", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		sourceMetaBefore, err := os.ReadFile(source.MetaPath())
		if err != nil {
			t.Fatalf("ReadFile() error = %v", err)
		}
		preview, err := h.manager.DerivePreview(testutil.Context(t), h.workspaceID, source.ID, "")
		if err != nil {
			t.Fatalf("DerivePreview() error = %v", err)
		}
		if preview.Cut != nil || preview.MaxSequence == 0 || preview.MessageCount != 4 ||
			preview.SourceMessageCount != 4 {
			t.Fatalf("preview = %+v, want whole-session numbers", preview)
		}
		sourceMetaAfter, _ := os.ReadFile(source.MetaPath())
		if !bytes.Equal(sourceMetaBefore, sourceMetaAfter) {
			t.Fatal("DerivePreview() changed the source meta")
		}
		opts := h.continueOpts(source, "idem_preview")
		opts.Fences = DeriveFences{
			ExpectedEpoch: &preview.Epoch, ExpectedGeneration: &preview.Generation,
			ExpectedMaxSequence: &preview.MaxSequence,
		}
		result, err := h.manager.ContinueSession(testutil.Context(t), opts)
		if err != nil {
			t.Fatalf("ContinueSession(preview fences) error = %v", err)
		}
		if result.ReplayMessageCount != preview.MessageCount || result.ReplayBytes != preview.ReplayBytes {
			t.Fatalf("result %d/%d, preview %d/%d, want equal", result.ReplayMessageCount, result.ReplayBytes,
				preview.MessageCount, preview.ReplayBytes)
		}
	})

	t.Run("Should continue an empty source with an empty carried context", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
		})
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		source, _ := h.manager.Get(created.ID)
		result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_empty"))
		if err != nil || result.ReplayMessageCount != 0 || result.Truncated {
			t.Fatalf("ContinueSession(empty) = %+v, %v, want zero carried messages", result, err)
		}
		if imported := h.childMeta(
			t,
			result.Child.ID,
		).ImportedContext; imported == nil ||
			imported.MessagesJSON != "[]" {
			t.Fatalf("imported context = %+v, want an empty array", imported)
		}
	})

	t.Run("Should note an open source turn and exclude it", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		entered := make(chan struct{})
		release := make(chan struct{})
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				close(entered)
				<-release
				emitDonePromptEvents(events, proc.handle.SessionID, req.TurnID)
			}()
			return events, nil
		}
		running, err := h.manager.Prompt(testutil.Context(t), source.ID, "still running")
		if err != nil {
			t.Fatalf("Prompt(running) error = %v", err)
		}
		<-entered
		result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_running"))
		close(release)
		collectEvents(t, running)
		if err != nil {
			t.Fatalf("ContinueSession(running source) error = %v", err)
		}
		imported := h.childMeta(t, result.Child.ID).ImportedContext
		if !result.SourceTurnInProgress || !strings.Contains(imported.MessagesJSON, "compozy_turn_aborted") ||
			strings.Contains(imported.MessagesJSON, "still running") {
			t.Fatalf("imported = %s, want the aborted note and no open-turn content", imported.MessagesJSON)
		}
	})

	t.Run("Should read a crash-classifiable stopped source without repairing it", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		metaPath := source.MetaPath()
		if err := h.manager.Stop(testutil.Context(t), source.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		meta := readMeta(t, metaPath)
		meta.State = string(StateActive)
		meta.Liveness = &store.SessionLivenessMeta{SubprocessPID: 999999}
		if err := store.WriteSessionMeta(metaPath, meta); err != nil {
			t.Fatalf("WriteSessionMeta() error = %v", err)
		}
		before, _ := os.ReadFile(metaPath)
		read, err := h.manager.readSessionMetaReadOnly(testutil.Context(t), source.ID)
		if err != nil || read.State != string(StateActive) {
			t.Fatalf("readSessionMetaReadOnly() = %s, %v, want the stored active document", read.State, err)
		}
		if _, err := h.manager.DerivePreview(testutil.Context(t), h.workspaceID, source.ID, ""); err != nil {
			t.Fatalf("DerivePreview(stopped) error = %v", err)
		}
		after, _ := os.ReadFile(metaPath)
		if !bytes.Equal(before, after) {
			t.Fatal("derive read path repaired the stopped source meta")
		}
	})
}

func sendDeriveChildPrompt(t *testing.T, h *deriveHarness, childID string, text string, messageID string, key string) {
	t.Helper()
	result, err := h.manager.SendPrompt(testutil.Context(t), childID, SendPromptOpts{
		Message: text, MessageID: messageID, IdempotencyKey: key,
	})
	if err != nil {
		t.Fatalf("SendPrompt(%q) error = %v", text, err)
	}
	if result.Events != nil {
		collectEvents(t, result.Events)
	}
}

func waitForDeriveConsumption(t *testing.T, h *deriveHarness, childID string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		imported := h.childMeta(t, childID).ImportedContext
		if imported != nil && imported.Consumed != nil {
			if imported.Consumed.AdmissionKey == "" {
				t.Fatalf("consumed = %+v, want the first admission key", imported.Consumed)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("imported context was never marked consumed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestPromptProviderErrorHandoff(t *testing.T) {
	t.Parallel()

	t.Run("Should persist next_action handoff on a rate-limited user session turn", func(t *testing.T) {
		t.Parallel()
		h := newHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() { cleanupSessionStop(t, h, sess.ID) })
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent, 1)
			events <- acp.AgentEvent{
				Type: acp.EventTypeError, SessionID: proc.handle.SessionID, TurnID: req.TurnID,
				Timestamp: time.Now().UTC(), Error: "HTTP 429 rate limit exceeded",
				ProviderError: &acp.ProviderErrorDiagnostic{
					Code: acp.ProviderErrorRateLimited, Provider: "claude",
					NextAction: acp.ProviderFailureActionRetry, Guidance: "retry later",
				},
			}
			close(events)
			return events, nil
		}
		events, err := h.manager.Prompt(testutil.Context(t), sess.ID, "hello")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		collectEvents(t, events)
		stored := storedEventByType(t, readStoredEvents(t, sess), acp.EventTypeError)
		payload := decodeStoredEventPayload(t, stored)
		providerError, _ := payload["provider_error"].(map[string]any)
		if providerError["next_action"] != string(acp.ProviderFailureActionHandoff) ||
			!strings.Contains(providerError["guidance"].(string), "compozy session continue "+sess.ID) {
			t.Fatalf("persisted provider_error = %v, want handoff naming the continue command", providerError)
		}
		if got := sess.Info().State; got != StateActive {
			t.Fatalf("session state = %s, want active after the failed turn", got)
		}
	})
}

var errDeriveInjectedCrash = errors.New("injected crash")

// deriveFaultStore is the creation/derivation store over the harness DB with a hook that
// runs right before the registration transaction.
type deriveFaultStore struct {
	*globaldb.GlobalDB
	mu     sync.Mutex
	calls  int
	before func(store.SessionInfo, store.SessionCreationIdentity, store.SessionDerivationReceipt) error
}

func (s *deriveFaultStore) option(db *globaldb.GlobalDB) Option {
	s.GlobalDB = db
	return WithSessionCreationStore(s)
}

func (s *deriveFaultStore) RegisterDerivedSession(
	ctx context.Context,
	info store.SessionInfo,
	identity store.SessionCreationIdentity,
	receipt store.SessionDerivationReceipt,
) error {
	s.mu.Lock()
	s.calls++
	hook := s.before
	s.mu.Unlock()
	if hook != nil {
		if err := hook(info, identity, receipt); err != nil {
			return err
		}
	}
	return s.GlobalDB.RegisterDerivedSession(ctx, info, identity, receipt)
}

// deriveLedgerFault drops session.derived writes while dropDerived is set, the durable
// state a crash between the child's commit and its event write leaves behind.
type deriveLedgerFault struct {
	*globaldb.GlobalDB
	dropDerived atomic.Bool
}

func (l *deriveLedgerFault) WriteEventSummary(ctx context.Context, summary store.EventSummary) error {
	if summary.Type == eventspkg.SessionDerived && l.dropDerived.Load() {
		return errDeriveInjectedCrash
	}
	return l.GlobalDB.WriteEventSummary(ctx, summary)
}

// deriveAdmissionFault fails the next claim of failKey, a crash before the admission.
type deriveAdmissionFault struct {
	*globaldb.GlobalDB
	mu      sync.Mutex
	failKey string
}

func (a *deriveAdmissionFault) ClaimSessionPromptAdmission(
	ctx context.Context,
	request store.SessionPromptAdmissionRequest,
) (store.SessionPromptAdmission, bool, error) {
	a.mu.Lock()
	fail := a.failKey != "" && request.IdempotencyKey == a.failKey
	if fail {
		a.failKey = ""
	}
	a.mu.Unlock()
	if fail {
		return store.SessionPromptAdmission{}, false, errDeriveInjectedCrash
	}
	return a.GlobalDB.ClaimSessionPromptAdmission(ctx, request)
}

func (h *deriveHarness) importedMessages(t *testing.T, childID string) []transcript.Message {
	t.Helper()
	imported := h.childMeta(t, childID).ImportedContext
	if imported == nil {
		t.Fatalf("child %s has no imported context", childID)
	}
	var messages []transcript.Message
	if err := json.Unmarshal([]byte(imported.MessagesJSON), &messages); err != nil {
		t.Fatalf("decode imported context: %v", err)
	}
	return messages
}

func importedContains(messages []transcript.Message, text string) bool {
	for _, message := range messages {
		if strings.Contains(message.Content, text) {
			return true
		}
	}
	return false
}

// rewindOpts resolves the durable user message with text and the current fences.
func (h *deriveHarness) rewindOpts(t *testing.T, sessionID string, text string, key string) ConversationRewindOptions {
	t.Helper()
	page, err := h.manager.TranscriptPage(testutil.Context(t), sessionID, transcript.PageQuery{Limit: 50})
	if err != nil {
		t.Fatalf("TranscriptPage() error = %v", err)
	}
	info, err := h.manager.Status(testutil.Context(t), sessionID)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	for _, entry := range page.Entries {
		if transcript.UIMessageText(entry.Message) == text {
			return ConversationRewindOptions{
				MessageID: entry.Message.ID, IdempotencyKey: key, ExpectedEpoch: info.TranscriptEpoch,
				ExpectedGeneration: page.Generation, ExpectedMaxSequence: page.MaxSequence,
			}
		}
	}
	t.Fatalf("user message %q not found in %s", text, sessionID)
	return ConversationRewindOptions{}
}

func TestDeriveCommitBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("Should issue one registration transaction after the meta write with nothing observable yet",
		func(t *testing.T) {
			t.Parallel()
			fault := &deriveFaultStore{}
			h := newDeriveHarness(t, fault.option)
			source := h.newDeriveSource(t)
			var (
				gotInfo     store.SessionInfo
				gotIdentity store.SessionCreationIdentity
				gotReceipt  store.SessionDerivationReceipt
				metaExisted bool
				catalogRow  bool
				ledgerRows  int
				notified    bool
			)
			fault.before = func(
				info store.SessionInfo, identity store.SessionCreationIdentity, receipt store.SessionDerivationReceipt,
			) error {
				gotInfo, gotIdentity, gotReceipt = info, identity, receipt
				_, statErr := os.Stat(filepath.Join(h.homePaths.SessionsDir, info.ID, store.SessionMetaName))
				metaExisted = statErr == nil
				_, lookupErr := store.LookupSessionDBOwner(testutil.Context(t), h.db, info.ID)
				catalogRow = lookupErr == nil
				ledgerRows = len(h.derivedLedgerRows(t, info.ID))
				h.notifier.mu.Lock()
				for _, created := range h.notifier.created {
					notified = notified || created.ID == info.ID
				}
				h.notifier.mu.Unlock()
				return nil
			}
			result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_order"))
			if err != nil {
				t.Fatalf("ContinueSession() error = %v", err)
			}
			if fault.calls != 1 || gotInfo.ID != result.Child.ID || gotIdentity.CreationProfileRef == "" ||
				gotReceipt.ChildSessionID != result.Child.ID || gotReceipt.IdempotencyKey != "idem_order" {
				t.Fatalf("registration calls=%d info=%s identity=%+v receipt=%+v, want one transaction for %s",
					fault.calls, gotInfo.ID, gotIdentity, gotReceipt, result.Child.ID)
			}
			if !metaExisted || catalogRow || ledgerRows != 0 || notified {
				t.Fatalf("at registration: meta=%t catalog=%t session.derived=%d notified=%t, "+
					"want only the meta written", metaExisted, catalogRow, ledgerRows, notified)
			}
			err = h.db.RegisterDerivedSession(testutil.Context(t), gotInfo, gotIdentity, gotReceipt)
			if !errors.Is(err, store.ErrSessionDerivationExists) {
				t.Fatalf("RegisterDerivedSession(duplicate) error = %v, want ErrSessionDerivationExists", err)
			}
		})

	t.Run("Should sweep a child whose registration failed and create a fresh one on retry", func(t *testing.T) {
		t.Parallel()
		fault := &deriveFaultStore{}
		h := newDeriveHarness(t, fault.option)
		source := h.newDeriveSource(t)
		var failedID string
		fault.before = func(info store.SessionInfo, _ store.SessionCreationIdentity, _ store.SessionDerivationReceipt) error {
			if failedID == "" {
				failedID = info.ID
				return errDeriveInjectedCrash
			}
			return nil
		}
		if _, err := h.manager.ContinueSession(
			testutil.Context(t), h.continueOpts(source, "idem_sweep"),
		); !errors.Is(err, errDeriveInjectedCrash) {
			t.Fatalf("ContinueSession(failing registration) error = %v, want the injected crash", err)
		}
		if _, err := os.Stat(filepath.Join(h.homePaths.SessionsDir, failedID)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("failed child directory stat error = %v, want swept", err)
		}
		if _, err := store.LookupSessionDBOwner(testutil.Context(t), h.db, failedID); err == nil {
			t.Fatal("failed child left a catalog row")
		}
		if _, found, err := h.db.SessionDerivationReceipt(
			testutil.Context(t), h.workspaceID, "idem_sweep",
		); err != nil || found {
			t.Fatalf("receipt after failed registration found=%t err=%v, want none", found, err)
		}
		retry, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_sweep"))
		if err != nil || retry.Replayed || retry.Child == nil || retry.Child.ID == failedID {
			t.Fatalf("ContinueSession(retry) = %+v, %v, want a fresh child", retry, err)
		}
		if rows := h.derivedLedgerRows(t, retry.Child.ID); len(rows) != 1 {
			t.Fatalf("session.derived rows = %d, want 1", len(rows))
		}
	})

	t.Run("Should re-emit a session.derived lost after the commit exactly once after restart", func(t *testing.T) {
		t.Parallel()
		ledger := &deriveLedgerFault{}
		ledger.dropDerived.Store(true)
		h := newDeriveHarness(t, func(db *globaldb.GlobalDB) Option {
			ledger.GlobalDB = db
			return WithEventLedger(ledger)
		})
		source := h.newDeriveSource(t)
		result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_lost_event"))
		if err != nil {
			t.Fatalf("ContinueSession() error = %v", err)
		}
		childID := result.Child.ID
		if rows := h.derivedLedgerRows(t, childID); len(rows) != 0 {
			t.Fatalf("session.derived rows before restart = %d, want the event lost", len(rows))
		}
		if err := h.manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		h.restartManager(t)
		infos, err := h.manager.ListAll(testutil.Context(t))
		if err != nil {
			t.Fatalf("ListAll() error = %v", err)
		}
		listed := false
		for _, info := range infos {
			listed = listed || info.ID == childID
		}
		if !listed {
			t.Fatalf("child %s not listed after restart", childID)
		}
		for attempt, want := range []int{1, 0} {
			emitted, err := h.manager.ReconcileDerivedSessionEvents(testutil.Context(t), infos)
			if err != nil || emitted != want {
				t.Fatalf("ReconcileDerivedSessionEvents(#%d) = %d, %v, want %d", attempt+1, emitted, err, want)
			}
		}
		rows := h.derivedLedgerRows(t, childID)
		if len(rows) != 1 {
			t.Fatalf("session.derived rows after reconciliation = %d, want exactly 1", len(rows))
		}
		var payload map[string]any
		if err := json.Unmarshal(rows[0].ContentValue(), &payload); err != nil {
			t.Fatalf("decode session.derived payload: %v", err)
		}
		if payload["source_session_id"] != source.ID || payload["idempotency_key"] != "idem_lost_event" ||
			payload["replay_bytes"] != float64(result.ReplayBytes) {
			t.Fatalf("re-emitted payload = %v, want the receipt outcome", payload)
		}
		if rows := h.derivedLedgerRows(t, source.ID); len(rows) != 0 {
			t.Fatalf("source session.derived rows = %d, want none", len(rows))
		}
	})

	t.Run("Should leave the first message staged after a crash before admission and send it once on retry",
		func(t *testing.T) {
			t.Parallel()
			admission := &deriveAdmissionFault{failKey: "idem_first_crash:first"}
			h := newDeriveHarness(t, func(db *globaldb.GlobalDB) Option {
				admission.GlobalDB = db
				return WithSessionPromptAdmissionStore(admission)
			})
			source := h.newDeriveSource(t)
			opts := h.continueOpts(source, "idem_first_crash")
			opts.Message = "go"
			promptsBefore := len(h.promptMessages())
			result, err := h.manager.ContinueSession(testutil.Context(t), opts)
			if !errors.Is(err, errDeriveInjectedCrash) || result.Child == nil {
				t.Fatalf("ContinueSession(crash before admission) = %+v, %v, want the child and the crash", result, err)
			}
			childID := result.Child.ID
			if state := h.childMeta(t, childID).Derivation.FirstPrompt.State; state !=
				store.SessionDerivationFirstPromptStaged {
				t.Fatalf("child first_prompt = %q, want staged before admission", state)
			}
			retry, err := h.manager.ContinueSession(testutil.Context(t), opts)
			if err != nil || !retry.Replayed || retry.ChildSessionID != childID {
				t.Fatalf("ContinueSession(retry) = %+v, %v, want the recorded child", retry, err)
			}
			prompts := h.waitForPromptCount(t, promptsBefore+1)
			if !strings.HasSuffix(prompts[promptsBefore], "User request:\n\ngo") {
				t.Fatalf("child first prompt = %q, want the carried context then go", prompts[promptsBefore])
			}
			waitForDeriveConsumption(t, h, childID)
			if _, err := h.manager.ContinueSession(testutil.Context(t), opts); err != nil {
				t.Fatalf("ContinueSession(second retry) error = %v", err)
			}
			if got := len(h.promptMessages()) - promptsBefore; got != 1 {
				t.Fatalf("first message sent %d times, want once", got)
			}
			if state := h.childMeta(t, childID).Derivation.FirstPrompt.State; state !=
				store.SessionDerivationFirstPromptAdmitted {
				t.Fatalf("child first_prompt = %q, want admitted after the retry", state)
			}
		})
}

func TestDeriveSnapshotConcurrency(t *testing.T) {
	t.Parallel()

	t.Run("Should make a concurrent rewind wait for the snapshot and fence later derives", func(t *testing.T) {
		t.Parallel()
		fault := &deriveFaultStore{}
		h := newDeriveHarness(t, fault.option)
		source := h.newDeriveSource(t)
		preview, err := h.manager.DerivePreview(testutil.Context(t), h.workspaceID, source.ID, "")
		if err != nil {
			t.Fatalf("DerivePreview() error = %v", err)
		}
		rewind := h.rewindOpts(t, source.ID, "Now run the tests", "idem_rewind_during_derive")
		entered := make(chan struct{})
		release := make(chan struct{})
		var once sync.Once
		fault.before = func(_ store.SessionInfo, _ store.SessionCreationIdentity, receipt store.SessionDerivationReceipt) error {
			if receipt.IdempotencyKey == "idem_held" {
				once.Do(func() { close(entered) })
				<-release
			}
			return nil
		}
		type deriveOutcome struct {
			result DeriveResult
			err    error
		}
		derived := make(chan deriveOutcome, 1)
		go func() {
			result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_held"))
			derived <- deriveOutcome{result: result, err: err}
		}()
		<-entered
		rewound := make(chan error, 1)
		go func() {
			_, err := h.manager.RewindConversation(testutil.Context(t), source.ID, rewind)
			rewound <- err
		}()
		select {
		case err := <-rewound:
			close(release)
			t.Fatalf("RewindConversation() finished while the derive held the snapshot: %v", err)
		case <-time.After(200 * time.Millisecond):
		}
		close(release)
		held := <-derived
		if held.err != nil {
			t.Fatalf("ContinueSession(held) error = %v", held.err)
		}
		if err := <-rewound; err != nil {
			t.Fatalf("RewindConversation() error = %v", err)
		}
		if !importedContains(h.importedMessages(t, held.result.Child.ID), "Now run the tests") {
			t.Fatal("held derive child does not reflect the pre-rewind transcript")
		}
		stale := h.continueOpts(source, "idem_stale_fences")
		stale.Fences = DeriveFences{
			ExpectedEpoch: &preview.Epoch, ExpectedGeneration: &preview.Generation,
			ExpectedMaxSequence: &preview.MaxSequence,
		}
		if _, err := h.manager.ContinueSession(testutil.Context(t), stale); !errors.Is(err, ErrDeriveFenceConflict) {
			t.Fatalf("ContinueSession(pre-rewind fences) error = %v, want ErrDeriveFenceConflict", err)
		}
		fresh, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_fresh"))
		if err != nil {
			t.Fatalf("ContinueSession(no fences) error = %v", err)
		}
		messages := h.importedMessages(t, fresh.Child.ID)
		if importedContains(messages, "Now run the tests") || !importedContains(messages, "Start the migration") {
			t.Fatal("derive after the rewind did not take a new snapshot of the rewound transcript")
		}
	})
}

func TestDeriveCarriedContextSurvivesRewind(t *testing.T) {
	t.Parallel()

	t.Run("Should begin the rewind restart's replay with the imported context", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_rewind_child"))
		if err != nil {
			t.Fatalf("ContinueSession() error = %v", err)
		}
		childID := result.Child.ID
		sendDeriveChildPrompt(t, h, childID, "child first ask", "msg_child_first", "idem_child_first")
		rewind := h.rewindOpts(t, childID, "child first ask", "idem_child_rewind")
		if _, err := h.manager.RewindConversation(testutil.Context(t), childID, rewind); err != nil {
			t.Fatalf("RewindConversation(child) error = %v", err)
		}
		before := len(h.promptMessages())
		sendDeriveChildPrompt(t, h, childID, "child second ask", "msg_child_second", "idem_child_second")
		prompts := h.promptMessages()
		if len(prompts) != before+1 {
			t.Fatalf("prompt calls = %d, want %d", len(prompts), before+1)
		}
		replay := prompts[before]
		if !strings.HasPrefix(replay, contextRebuiltMarkerSummary) ||
			strings.Count(replay, resumeReplayOpenTag) != 1 ||
			!strings.Contains(replay, "Start the migration") || strings.Contains(replay, "child first ask") ||
			!strings.HasSuffix(replay, "child second ask") {
			t.Fatalf("post-rewind prompt = %q, want the imported context replayed before the new ask", replay)
		}
	})
}

func TestDeriveReceiptScope(t *testing.T) {
	t.Parallel()

	t.Run("Should derive independently when another workspace reuses the key", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		primary := h.newDeriveSource(t)
		const secondaryID = "ws-secondary"
		resolved, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		secondaryRoot := filepath.Join(h.homePaths.HomeDir, "workspace-secondary")
		if err := os.MkdirAll(secondaryRoot, 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		resolved.Workspace.ID, resolved.Workspace.RootDir, resolved.Workspace.Name =
			secondaryID, secondaryRoot, "workspace-secondary"
		h.resolver.upsert(&resolved)
		if err := h.db.InsertWorkspace(testutil.Context(t), resolved.Workspace); err != nil {
			t.Fatalf("InsertWorkspace(secondary) error = %v", err)
		}
		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Name: "Other workspace", Workspace: secondaryID},
		})
		if err != nil {
			t.Fatalf("CreateAccepted(secondary source) error = %v", err)
		}
		events, err := h.manager.Prompt(testutil.Context(t), created.ID, "Secondary work")
		if err != nil {
			t.Fatalf("Prompt(secondary source) error = %v", err)
		}
		collectEvents(t, events)

		first, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(primary, "idem_shared"))
		if err != nil {
			t.Fatalf("ContinueSession(primary) error = %v", err)
		}
		second, err := h.manager.ContinueSession(testutil.Context(t), ContinueSessionOpts{
			SourceSessionID: created.ID, WorkspaceID: secondaryID, AgentName: "b", IdempotencyKey: "idem_shared",
		})
		if err != nil {
			t.Fatalf("ContinueSession(secondary, same key) error = %v", err)
		}
		if second.Replayed || second.Child == nil || second.Child.ID == first.Child.ID ||
			second.Child.WorkspaceID != secondaryID {
			t.Fatalf("secondary derive = %+v, want an independent child in %s", second, secondaryID)
		}
		for workspaceID, childID := range map[string]string{h.workspaceID: first.Child.ID, secondaryID: second.Child.ID} {
			receipt, found, err := h.db.SessionDerivationReceipt(testutil.Context(t), workspaceID, "idem_shared")
			if err != nil || !found || receipt.ChildSessionID != childID {
				t.Fatalf("receipt(%s) = %+v found=%t err=%v, want child %s", workspaceID, receipt, found, err, childID)
			}
		}
	})
}

func TestDeriveLineageReadModel(t *testing.T) {
	t.Parallel()

	t.Run("Should expose lineage on catalog list rows and derivation on the child read", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_list"))
		if err != nil {
			t.Fatalf("ContinueSession() error = %v", err)
		}
		if err := h.manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		h.restartManager(t)
		page, err := h.manager.ListPage(testutil.Context(t), ListQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: h.workspaceID, Limit: 20,
		})
		if err != nil {
			t.Fatalf("ListPage() error = %v", err)
		}
		kinds := map[string]store.LineageKind{}
		for _, info := range page.Sessions {
			if lineage := store.NormalizeSessionLineage(info.ID, info.Lineage); lineage != nil {
				kinds[info.ID] = lineage.Kind
			} else {
				kinds[info.ID] = ""
			}
		}
		if kind, ok := kinds[result.Child.ID]; !ok || kind != store.LineageKindContinue {
			t.Fatalf("list page kinds = %v, want child %s with kind continue", kinds, result.Child.ID)
		}
		if kind, ok := kinds[source.ID]; !ok || kind != "" {
			t.Fatalf("list page kinds = %v, want root source %s without a kind", kinds, source.ID)
		}
		child, err := h.manager.Status(testutil.Context(t), result.Child.ID)
		if err != nil {
			t.Fatalf("Status(child) error = %v", err)
		}
		lineage := store.NormalizeSessionLineage(child.ID, child.Lineage)
		if lineage == nil || lineage.OriginAgentName != "coder" || child.Derivation == nil ||
			child.Derivation.Kind != store.LineageKindContinue || child.Derivation.SourceSessionID != source.ID {
			t.Fatalf("child read lineage=%+v derivation=%+v, want origin coder and the continue derivation",
				lineage, child.Derivation)
		}
	})
}

func TestDeriveCarriesAttachmentMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should carry attachment metadata without copying attachment bytes", func(t *testing.T) {
		t.Parallel()
		data := map[string][]byte{
			"att_" + strings.Repeat("a", 64): []byte("first attachment"),
			"att_" + strings.Repeat("b", 64): []byte("second attachment"),
		}
		opener := &promptAttachmentOpenerStub{
			data: map[string][]byte{}, refs: map[string]attachmentspkg.AttachmentRef{},
		}
		metas := make([]AttachmentMeta, 0, len(data))
		for id, content := range data {
			name := id[:8] + ".txt"
			opener.data[id] = content
			opener.refs[id] = storedPromptAttachmentRef(t, id, name, "text/plain", content)
			metas = append(metas, promptAttachmentMeta(t, id, name, "text/plain", content))
		}
		h := newDeriveHarness(t, func(*globaldb.GlobalDB) Option { return WithAttachmentOpener(opener) })
		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Name: "With files", Workspace: h.workspaceID},
		})
		if err != nil {
			t.Fatalf("CreateAccepted() error = %v", err)
		}
		sent, err := h.manager.SendPrompt(testutil.Context(t), created.ID, SendPromptOpts{
			Message: "Review these files", MessageID: "msg_files", IdempotencyKey: "idem_files", Attachments: metas,
		})
		if err != nil {
			t.Fatalf("SendPrompt(attachments) error = %v", err)
		}
		collectEvents(t, sent.Events)
		source, _ := h.manager.Get(created.ID)
		openerCalls := opener.calls
		result, err := h.manager.ContinueSession(testutil.Context(t), h.continueOpts(source, "idem_attachments"))
		if err != nil {
			t.Fatalf("ContinueSession() error = %v", err)
		}
		if opener.calls != openerCalls {
			t.Fatalf("attachment reads during derive = %d, want none", opener.calls-openerCalls)
		}
		var carried *transcript.Message
		for _, message := range h.importedMessages(t, result.Child.ID) {
			if strings.Contains(message.Content, "Review these files") {
				carried = &message
			}
		}
		if carried == nil || len(carried.Attachments) != 2 {
			t.Fatalf("carried user message = %+v, want two attachment metadata entries", carried)
		}
		for _, attachment := range carried.Attachments {
			if _, ok := data[attachment.ID]; !ok || attachment.Name == "" {
				t.Fatalf("carried attachment = %+v, want the source attachment metadata", attachment)
			}
		}
		child, _ := h.manager.Get(result.Child.ID)
		for _, event := range readStoredEvents(t, child) {
			if strings.Contains(event.Content, "att_") {
				t.Fatalf("child event %s references an attachment, want none copied", event.Type)
			}
		}
	})
}

func (h *deriveHarness) forkOpts(source *Session, key string) ForkSessionOpts {
	return ForkSessionOpts{SourceSessionID: source.ID, WorkspaceID: h.workspaceID, IdempotencyKey: key}
}

func (h *deriveHarness) promptSource(t *testing.T, sessionID string, texts ...string) {
	t.Helper()
	for _, text := range texts {
		events, err := h.manager.Prompt(testutil.Context(t), sessionID, text)
		if err != nil {
			t.Fatalf("Prompt(%q) error = %v", text, err)
		}
		collectEvents(t, events)
	}
}

func (h *deriveHarness) startCallsSnapshot() []acp.StartOpts {
	h.driver.mu.Lock()
	defer h.driver.mu.Unlock()
	return append([]acp.StartOpts(nil), h.driver.startCalls...)
}

func TestForkSession(t *testing.T) {
	t.Parallel()

	t.Run("Should fork the whole session on the source agent and runtime and leave the source untouched",
		func(t *testing.T) {
			t.Parallel()
			h := newDeriveHarness(t)
			source := h.newDeriveSource(t)
			sourceMetaBefore, err := os.ReadFile(source.MetaPath())
			if err != nil {
				t.Fatalf("ReadFile(source meta) error = %v", err)
			}
			sourceEventsBefore := len(readStoredEvents(t, source))
			result, err := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_fork_whole"))
			if err != nil {
				t.Fatalf("ForkSession() error = %v", err)
			}
			child := result.Child
			lineage := store.NormalizeSessionLineage(child.ID, child.Lineage)
			sourceInfo := source.Info()
			if child.AgentName != "coder" || lineage.Kind != store.LineageKindFork || lineage.OriginMessageID != "" ||
				lineage.OriginAgentName != "coder" || result.OriginMessageID != "" || result.ThroughTurnID == "" {
				t.Fatalf("child = %s lineage %+v result %+v, want a whole-session fork on coder",
					child.AgentName, lineage, result)
			}
			meta := h.childMeta(t, child.ID)
			if meta.Provider != sourceInfo.Provider || meta.Model != sourceInfo.Model {
				t.Fatalf("child runtime = %s/%s, want the source runtime %s/%s",
					meta.Provider, meta.Model, sourceInfo.Provider, sourceInfo.Model)
			}
			if result.Seed != DeriveSeedReplay || result.ReplayMessageCount != 4 || h.driver.forkCallCount() != 0 {
				t.Fatalf("result = %+v fork calls = %d, want replay of 4 messages without session/fork",
					result, h.driver.forkCallCount())
			}
			sourceMetaAfter, err := os.ReadFile(source.MetaPath())
			if err != nil {
				t.Fatalf("ReadFile(source meta) error = %v", err)
			}
			if !bytes.Equal(sourceMetaBefore, sourceMetaAfter) ||
				len(readStoredEvents(t, source)) != sourceEventsBefore {
				t.Fatal("ForkSession() changed the source meta or events")
			}
		})

	t.Run("Should fork a stopped source through the read-only path", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		if err := h.manager.Stop(testutil.Context(t), source.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		before, err := os.ReadFile(source.MetaPath())
		if err != nil {
			t.Fatalf("ReadFile(source meta) error = %v", err)
		}
		result, err := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_fork_stopped"))
		if err != nil || result.Seed != DeriveSeedReplay || result.Child.AgentName != "coder" {
			t.Fatalf("ForkSession(stopped) = %+v, %v, want a replay fork on coder", result, err)
		}
		after, err := os.ReadFile(source.MetaPath())
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("stopped source meta changed (err=%v)", err)
		}
	})

	t.Run("Should cut through the chosen message's turn inclusively", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		h.promptSource(t, source.ID, "Third step", "Fourth step", "Fifth step")
		third := h.rewindOpts(t, source.ID, "Third step", "unused").MessageID
		cutPreview, err := h.manager.DerivePreview(testutil.Context(t), h.workspaceID, source.ID, third)
		if err != nil || cutPreview.MessageCount != 6 || cutPreview.SourceMessageCount != 10 {
			t.Fatalf("DerivePreview(third) = %+v, %v, want 6 of 10 source messages", cutPreview, err)
		}
		opts := h.forkOpts(source, "idem_fork_third")
		opts.MessageID = third
		result, err := h.manager.ForkSession(testutil.Context(t), opts)
		if err != nil {
			t.Fatalf("ForkSession(msg) error = %v", err)
		}
		messages := h.importedMessages(t, result.Child.ID)
		if len(messages) != 6 || !importedContains(messages, "Third step") ||
			importedContains(messages, "Fourth step") || importedContains(messages, "Fifth step") ||
			messages[len(messages)-1].Role != "assistant" {
			t.Fatalf("carried messages = %+v, want t1..t3 ending with t3's reply", messages)
		}
		lineage := store.NormalizeSessionLineage(result.Child.ID, result.Child.Lineage)
		if lineage.OriginMessageID != third || result.OriginMessageID != third || result.ThroughTurnID == "" {
			t.Fatalf("lineage = %+v result = %+v, want origin %s", lineage, result, third)
		}

		last := h.forkOpts(source, "idem_fork_last")
		last.MessageID = h.rewindOpts(t, source.ID, "Fifth step", "unused").MessageID
		lastResult, err := h.manager.ForkSession(testutil.Context(t), last)
		if err != nil {
			t.Fatalf("ForkSession(last message) error = %v", err)
		}
		whole, err := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_fork_all"))
		if err != nil {
			t.Fatalf("ForkSession(whole) error = %v", err)
		}
		if lastResult.ReplayMessageCount != whole.ReplayMessageCount ||
			lastResult.ThroughTurnID != whole.ThroughTurnID {
			t.Fatalf("last-message fork %d/%s, whole fork %d/%s, want equal", lastResult.ReplayMessageCount,
				lastResult.ThroughTurnID, whole.ReplayMessageCount, whole.ThroughTurnID)
		}
	})

	t.Run("Should refuse a cut whose turn has not settled and report it in the preview", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		entered := make(chan struct{})
		release := make(chan struct{})
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				close(entered)
				<-release
				emitDonePromptEvents(events, proc.handle.SessionID, req.TurnID)
			}()
			return events, nil
		}
		running, err := h.manager.Prompt(testutil.Context(t), source.ID, "still running")
		if err != nil {
			t.Fatalf("Prompt(running) error = %v", err)
		}
		<-entered
		defer func() {
			close(release)
			collectEvents(t, running)
		}()
		messageID := h.rewindOpts(t, source.ID, "still running", "unused").MessageID
		preview, err := h.manager.DerivePreview(testutil.Context(t), h.workspaceID, source.ID, messageID)
		if err != nil || preview.Cut == nil || preview.Cut.TurnSettled || preview.NativeForkPossible {
			t.Fatalf("DerivePreview(open turn) = %+v, %v, want an unsettled cut", preview, err)
		}
		opts := h.forkOpts(source, "idem_fork_running")
		opts.MessageID = messageID
		_, err = h.manager.ForkSession(testutil.Context(t), opts)
		if !errors.Is(err, ErrDeriveTurnInProgress) || !strings.Contains(err.Error(), "has not settled") {
			t.Fatalf("ForkSession(open turn) error = %v, want ErrDeriveTurnInProgress", err)
		}
	})

	t.Run("Should refuse stale fences and unknown messages and record the snapshot fences", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		preview, err := h.manager.DerivePreview(testutil.Context(t), h.workspaceID, source.ID, "")
		if err != nil {
			t.Fatalf("DerivePreview() error = %v", err)
		}
		stale := preview.MaxSequence - 1
		opts := h.forkOpts(source, "idem_fork_stale")
		opts.Fences = DeriveFences{
			ExpectedEpoch: &preview.Epoch, ExpectedGeneration: &preview.Generation, ExpectedMaxSequence: &stale,
		}
		if _, err := h.manager.ForkSession(testutil.Context(t), opts); !errors.Is(err, ErrDeriveFenceConflict) {
			t.Fatalf("ForkSession(stale fences) error = %v, want ErrDeriveFenceConflict", err)
		}
		if _, found, err := h.db.SessionDerivationReceipt(
			testutil.Context(t), h.workspaceID, "idem_fork_stale",
		); err != nil || found {
			t.Fatalf("receipt after fence conflict found=%t err=%v, want none", found, err)
		}
		unknown := h.forkOpts(source, "idem_fork_unknown")
		unknown.MessageID = "msg_missing"
		_, err = h.manager.ForkSession(testutil.Context(t), unknown)
		if !errors.Is(err, ErrDeriveMessageNotFound) ||
			err.Error() != "message msg_missing not found in session "+source.ID {
			t.Fatalf("ForkSession(unknown message) error = %v, want ErrDeriveMessageNotFound", err)
		}
		if _, err := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_fork_nofence")); err != nil {
			t.Fatalf("ForkSession(no fences) error = %v", err)
		}
		receipt, found, err := h.db.SessionDerivationReceipt(testutil.Context(t), h.workspaceID, "idem_fork_nofence")
		if err != nil || !found || receipt.Outcome.SourceMaxSequence != preview.MaxSequence ||
			receipt.Outcome.SourceGeneration != preview.Generation || receipt.Kind != store.LineageKindFork {
			t.Fatalf("receipt = %+v found=%t err=%v, want the snapshot fences", receipt, found, err)
		}
	})

	t.Run("Should accept anchors after a compacted prefix and after a rewind with new turns", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := h.newDeriveSource(t)
		h.promptSource(t, source.ID, "Third step")
		events := readStoredEvents(t, source)
		firstTurn := events[0].TurnID
		var through int64
		for _, event := range events {
			if event.TurnID == firstTurn {
				through = max(through, event.Sequence)
			}
		}
		archiver, ok := source.recorderHandle().(store.EventArchiver)
		if !ok {
			t.Fatal("source recorder cannot archive events")
		}
		if _, err := archiver.ArchiveEvents(testutil.Context(t), store.EventArchiveRequest{
			FromSequence: events[0].Sequence, ToSequence: through,
		}); err != nil {
			t.Fatalf("ArchiveEvents(prefix) error = %v", err)
		}
		third := h.rewindOpts(t, source.ID, "Third step", "idem_rewind_compacted")
		if _, err := h.manager.RewindConversation(testutil.Context(t), source.ID, third); !errors.Is(
			err, store.ErrConversationRewindTargetInvalid,
		) {
			t.Fatalf("RewindConversation(after compaction) error = %v, want ErrConversationRewindTargetInvalid", err)
		}
		opts := h.forkOpts(source, "idem_fork_compacted")
		opts.MessageID = third.MessageID
		result, err := h.manager.ForkSession(testutil.Context(t), opts)
		if err != nil || !importedContains(h.importedMessages(t, result.Child.ID), "Third step") {
			t.Fatalf("ForkSession(after compaction) = %+v, %v, want the anchor accepted", result, err)
		}

		h2 := newDeriveHarness(t)
		rewound := h2.newDeriveSource(t)
		rewind := h2.rewindOpts(t, rewound.ID, "Now run the tests", "idem_rewind_then_turns")
		if _, err := h2.manager.RewindConversation(testutil.Context(t), rewound.ID, rewind); err != nil {
			t.Fatalf("RewindConversation() error = %v", err)
		}
		h2.promptSource(t, rewound.ID, "New direction", "Keep going")
		newTurn := h2.forkOpts(rewound, "idem_fork_after_rewind")
		newTurn.MessageID = h2.rewindOpts(t, rewound.ID, "New direction", "unused").MessageID
		forked, err := h2.manager.ForkSession(testutil.Context(t), newTurn)
		if err != nil {
			t.Fatalf("ForkSession(after rewind) error = %v", err)
		}
		messages := h2.importedMessages(t, forked.Child.ID)
		if !importedContains(messages, "Start the migration") || !importedContains(messages, "New direction") ||
			importedContains(messages, "Keep going") || importedContains(messages, "Now run the tests") {
			t.Fatalf("carried = %+v, want the rewind baseline plus the new turn through the cut", messages)
		}
	})
}

// newNativeForkHarness is a derive harness whose fake processes advertise session/fork
// and session/load, with a bound, idle source.
func newNativeForkHarness(t *testing.T) (*deriveHarness, *Session) {
	t.Helper()
	h := newDeriveHarness(t)
	h.driver.mu.Lock()
	h.driver.advertiseFork = true
	h.driver.mu.Unlock()
	source := h.newDeriveSource(t)
	return h, source
}

func (h *deriveHarness) childNative(t *testing.T, childID string) store.SessionNativeBootstrap {
	t.Helper()
	derivation := h.childMeta(t, childID).Derivation
	if derivation == nil || derivation.Native == nil {
		t.Fatalf("child %s derivation = %+v, want a native bootstrap", childID, derivation)
	}
	return *derivation.Native
}

func TestForkNativeSeed(t *testing.T) {
	t.Parallel()

	t.Run("Should clone natively, keep the carried context, and load the clone at the first bind",
		func(t *testing.T) {
			t.Parallel()
			h, source := newNativeForkHarness(t)
			preview, err := h.manager.DerivePreview(testutil.Context(t), h.workspaceID, source.ID, "")
			if err != nil || !preview.NativeForkPossible {
				t.Fatalf("DerivePreview() = %+v, %v, want native_fork_possible", preview, err)
			}
			sourceEvents := len(readStoredEvents(t, source))
			result, err := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_native"))
			if err != nil {
				t.Fatalf("ForkSession() error = %v", err)
			}
			clone := source.Info().ACPSessionID + "-fork"
			if result.Seed != DeriveSeedNativeFork || result.NativeState != store.SessionNativeStatePending ||
				result.ACPSessionID != clone || result.ReplayBytes == 0 || h.driver.forkCallCount() != 1 {
				t.Fatalf("result = %+v fork calls = %d, want a pending native fork of %s",
					result, h.driver.forkCallCount(), clone)
			}
			native := h.childNative(t, result.Child.ID)
			if native.ACPSessionID != clone || native.State != store.SessionNativeStatePending ||
				native.Provider == "" || native.CommandFingerprint == "" {
				t.Fatalf("child native = %+v, want the pinned pending clone", native)
			}
			if imported := h.childMeta(t, result.Child.ID).ImportedContext; imported == nil || imported.Bytes == 0 {
				t.Fatalf("imported context = %+v, want it persisted for fallback", imported)
			}
			if got := len(readStoredEvents(t, source)); got != sourceEvents {
				t.Fatalf("source events = %d, want %d unchanged", got, sourceEvents)
			}

			before := len(h.promptMessages())
			sendDeriveChildPrompt(t, h, result.Child.ID, "child ask", "msg_native_child", "idem_native_child")
			starts := h.startCallsSnapshot()
			if last := starts[len(starts)-1]; last.ResumeSessionID != clone {
				t.Fatalf("child bind resume id = %q, want the clone %q", last.ResumeSessionID, clone)
			}
			if prompts := h.promptMessages(); len(prompts) != before+1 || prompts[before] != "child ask" {
				t.Fatalf("child prompts = %q, want the bare message without a replay block", prompts[before:])
			}
			loaded := h.childNative(t, result.Child.ID)
			if loaded.State != store.SessionNativeStateLoaded || loaded.SettledAt == nil || loaded.Error != "" {
				t.Fatalf("child native after bind = %+v, want loaded", loaded)
			}
			child, _ := h.manager.Get(result.Child.ID)
			replay, _, err := h.manager.buildResumeReplay(testutil.Context(t), child)
			if err != nil || !strings.Contains(replay, "This session was forked from "+source.ID) {
				t.Fatalf("rebuild of a loaded child = %q, %v, want the carried context prepended", replay, err)
			}
		})

	t.Run("Should replay without calling session/fork when the clone is not possible", func(t *testing.T) {
		t.Parallel()
		plain := newDeriveHarness(t)
		plainSource := plain.newDeriveSource(t)
		preview, err := plain.manager.DerivePreview(testutil.Context(t), plain.workspaceID, plainSource.ID, "")
		if err != nil || preview.NativeForkPossible {
			t.Fatalf("DerivePreview(no fork caps) = %+v, %v, want not possible", preview, err)
		}
		result, err := plain.manager.ForkSession(testutil.Context(t), plain.forkOpts(plainSource, "idem_plain"))
		if err != nil || result.Seed != DeriveSeedReplay || plain.driver.forkCallCount() != 0 {
			t.Fatalf("ForkSession(no fork caps) = %+v, %v, want replay without session/fork", result, err)
		}

		h, source := newNativeForkHarness(t)
		cut := h.forkOpts(source, "idem_native_cut")
		cut.MessageID = h.rewindOpts(t, source.ID, "Start the migration", "unused").MessageID
		if result, err := h.manager.ForkSession(
			testutil.Context(t),
			cut,
		); err != nil ||
			result.Seed != DeriveSeedReplay {
			t.Fatalf("ForkSession(message cut) = %+v, %v, want replay", result, err)
		}
		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "coder", Workspace: h.workspaceID},
		})
		if err != nil {
			t.Fatalf("CreateAccepted(unbound) error = %v", err)
		}
		if preview, err := h.manager.DerivePreview(
			testutil.Context(t), h.workspaceID, created.ID, "",
		); err != nil || preview.NativeForkPossible {
			t.Fatalf("DerivePreview(unbound) = %+v, %v, want not possible", preview, err)
		}
		if err := h.manager.Stop(testutil.Context(t), source.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}
		stopped, err := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_native_stopped"))
		if err != nil || stopped.Seed != DeriveSeedReplay || h.driver.forkCallCount() != 0 {
			t.Fatalf("ForkSession(stopped) = %+v, %v fork calls %d, want replay without session/fork",
				stopped, err, h.driver.forkCallCount())
		}
	})

	t.Run("Should record a redacted, bounded error and replay when session/fork fails", func(t *testing.T) {
		t.Parallel()
		h, source := newNativeForkHarness(t)
		const token = "sk-live-0123456789abcdefghijklmnopqrstuv"
		var command string
		h.driver.forkHook = func(proc *AgentProcess, _ string) (acp.ForkSessionResult, error) {
			command = proc.Command
			return acp.ForkSessionResult{}, fmt.Errorf(
				"acp: session/fork %q: agent %s failed: Authorization: Bearer %s %s",
				proc.SessionID, proc.Command, token, strings.Repeat("x", 10*1024),
			)
		}
		result, err := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_native_error"))
		if err != nil || result.Child == nil || result.Seed != DeriveSeedReplay {
			t.Fatalf("ForkSession(fork error) = %+v, %v, want a replay child", result, err)
		}
		text := result.NativeForkError
		if !strings.HasPrefix(text, "session/fork: ") || len(text) > maxSessionFailureSummaryBytes ||
			strings.Contains(text, token) || command == "" || strings.Contains(text, command) {
			t.Fatalf("native_fork_error = %q (%d bytes), want redacted, bounded, without the command", text, len(text))
		}
		if derivation := h.childMeta(t, result.Child.ID).Derivation; derivation.Native != nil ||
			derivation.Seed != store.SessionDerivationSeedReplay {
			t.Fatalf("child derivation = %+v, want a replay seed without a native bootstrap", derivation)
		}
	})

	t.Run("Should settle failed and replay the carried context when the clone cannot load", func(t *testing.T) {
		t.Parallel()
		h, source := newNativeForkHarness(t)
		result, err := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_native_missing"))
		if err != nil || result.Seed != DeriveSeedNativeFork {
			t.Fatalf("ForkSession() = %+v, %v, want a native fork", result, err)
		}
		h.driver.mu.Lock()
		h.driver.startHook = func(opts acp.StartOpts, sequence int) (*fakeProcess, error) {
			if opts.ResumeSessionID != "" {
				return nil, fmt.Errorf("%w: load session %q: %w", acp.ErrLoadSessionFailed, opts.ResumeSessionID,
					&acpsdk.RequestError{Code: -32002, Message: "Resource not found"})
			}
			return newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, fmt.Sprintf("acp-new-%d", sequence)), nil
		}
		h.driver.mu.Unlock()
		before := len(h.promptMessages())
		sendDeriveChildPrompt(t, h, result.Child.ID, "child ask", "msg_missing_child", "idem_missing_child")
		native := h.childNative(t, result.Child.ID)
		if native.State != store.SessionNativeStateFailed || native.Error != "session/load: resource not found" {
			t.Fatalf("child native = %+v, want failed with the load error", native)
		}
		info, err := h.manager.Status(testutil.Context(t), result.Child.ID)
		if err != nil || info.Derivation == nil || info.Derivation.Native.State != store.SessionNativeStateFailed {
			t.Fatalf("Status(child) = %+v, %v, want the failed native state on the read model", info, err)
		}
		prompt := h.promptMessages()[before]
		if !strings.Contains(prompt, resumeReplayOpenTag) || !strings.Contains(prompt, "Start the migration") ||
			!strings.HasSuffix(prompt, "child ask") {
			t.Fatalf("child first prompt = %q, want the carried context before the ask", prompt)
		}
	})

	t.Run("Should never load the clone under a route that differs from the pinned identity", func(t *testing.T) {
		t.Parallel()
		h, source := newNativeForkHarness(t)
		result, err := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_native_mismatch"))
		if err != nil || result.Seed != DeriveSeedNativeFork {
			t.Fatalf("ForkSession() = %+v, %v, want a native fork", result, err)
		}
		child, _ := h.manager.Get(result.Child.ID)
		child.mu.Lock()
		child.derivation.Native.CommandFingerprint = "another-account"
		child.mu.Unlock()
		startsBefore := len(h.startCallsSnapshot())
		sendDeriveChildPrompt(t, h, result.Child.ID, "child ask", "msg_mismatch_child", "idem_mismatch_child")
		for _, start := range h.startCallsSnapshot()[startsBefore:] {
			if start.ResumeSessionID != "" {
				t.Fatalf("child bind loaded %q under a mismatched route", start.ResumeSessionID)
			}
		}
		if native := h.childNative(t, result.Child.ID); native.State != store.SessionNativeStateFailed ||
			!strings.HasPrefix(native.Error, "route_mismatch") {
			t.Fatalf("child native = %+v, want failed route_mismatch", native)
		}
	})
}

func TestForkNativeGate(t *testing.T) {
	t.Parallel()

	t.Run("Should gate the clone on idleness, agent, and route compatibility", func(t *testing.T) {
		t.Parallel()
		_, source := newNativeForkHarness(t)
		route := source.providerRoutingSnapshot()
		otherHome := route
		otherHome.HomePolicy = "isolated-elsewhere"
		otherCommand := route
		otherCommand.Command = route.Command + " --account 2"
		for _, tc := range []struct {
			name   string
			agent  string
			route  compozyconfig.ResolvedAgent
			queued int
			want   string
		}{
			{name: "eligible", agent: "coder", route: route},
			{name: "queued input", agent: "coder", route: route, queued: 1, want: nativeForkReasonBusy},
			{name: "other agent", agent: "b", route: route, want: nativeForkReasonAgent},
			{name: "other home policy", agent: "coder", route: otherHome, want: nativeForkReasonRoute},
			{name: "other command", agent: "coder", route: otherCommand, want: nativeForkReasonRoute},
		} {
			decision := nativeForkEligible(source, "", tc.agent, tc.route, tc.queued)
			if decision.Eligible != (tc.want == "") || decision.Reason != tc.want {
				t.Fatalf("%s: decision = %+v, want reason %q", tc.name, decision, tc.want)
			}
		}
		if decision := nativeForkEligible(
			source,
			"msg_1",
			"coder",
			route,
			0,
		); decision.Reason != nativeForkReasonMessageCut {
			t.Fatalf("message cut decision = %+v, want %s", decision, nativeForkReasonMessageCut)
		}
		if decision := nativeForkEligible(nil, "", "coder", route, 0); decision.Reason != nativeForkReasonUnbound {
			t.Fatalf("unbound decision = %+v, want %s", decision, nativeForkReasonUnbound)
		}
	})

	t.Run("Should fall back to replay while the source is running a prompt", func(t *testing.T) {
		t.Parallel()
		h, source := newNativeForkHarness(t)
		entered := make(chan struct{})
		release := make(chan struct{})
		h.driver.promptHook = func(proc *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				close(entered)
				<-release
				emitDonePromptEvents(events, proc.handle.SessionID, req.TurnID)
			}()
			return events, nil
		}
		running, err := h.manager.Prompt(testutil.Context(t), source.ID, "busy turn")
		if err != nil {
			t.Fatalf("Prompt(busy) error = %v", err)
		}
		<-entered
		preview, previewErr := h.manager.DerivePreview(testutil.Context(t), h.workspaceID, source.ID, "")
		result, forkErr := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_native_busy"))
		close(release)
		collectEvents(t, running)
		if previewErr != nil || preview.NativeForkPossible {
			t.Fatalf("DerivePreview(busy) = %+v, %v, want not possible", preview, previewErr)
		}
		if forkErr != nil || result.Seed != DeriveSeedReplay || !result.SourceTurnInProgress ||
			h.driver.forkCallCount() != 0 {
			t.Fatalf("ForkSession(busy) = %+v, %v, want a replay fork without session/fork", result, forkErr)
		}
	})

	t.Run("Should hold the source's prompt slot for the whole session/fork call", func(t *testing.T) {
		t.Parallel()
		h, source := newNativeForkHarness(t)
		promptsBefore := len(h.promptMessages())
		inFork := make(chan struct{})
		releaseFork := make(chan struct{})
		h.driver.forkHook = func(proc *AgentProcess, _ string) (acp.ForkSessionResult, error) {
			close(inFork)
			<-releaseFork
			return acp.ForkSessionResult{SessionID: acpsdk.SessionId(proc.SessionID + "-clone")}, nil
		}
		forked := make(chan DeriveResult, 1)
		go func() {
			result, err := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_native_slot"))
			if err != nil {
				t.Errorf("ForkSession() error = %v", err)
			}
			forked <- result
		}()
		<-inFork
		_, promptErr := h.manager.Prompt(testutil.Context(t), source.ID, "concurrent")
		reachedAgent := len(h.promptMessages()) != promptsBefore
		close(releaseFork)
		result := <-forked
		if reachedAgent || !errors.Is(promptErr, ErrPromptInProgress) {
			t.Fatalf("concurrent source prompt reached agent=%t err=%v, want it refused while forking",
				reachedAgent, promptErr)
		}
		if result.Seed != DeriveSeedNativeFork || result.ACPSessionID != source.Info().ACPSessionID+"-clone" {
			t.Fatalf("ForkSession() = %+v, want the native clone", result)
		}
		h.promptSource(t, source.ID, "after fork")
	})
}

func TestForkAccountInheritance(t *testing.T) {
	t.Parallel()

	commitSourceRoute := func(t *testing.T, h *deriveHarness, source *Session, mutate func(*compozyconfig.ResolvedAgent)) {
		t.Helper()
		workspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		var agentB compozyconfig.AgentDef
		for _, agent := range workspace.Agents {
			if agent.Name == "b" {
				agentB = agent
			}
		}
		resolved, err := workspace.Config.ResolveSessionAgentWithRuntime(agentB, compozyconfig.RuntimeOverrides{
			Provider: "codex", Model: h.cfg.Providers["codex"].Models.Default, Command: deriveRouteTwoCommand,
		})
		if err != nil {
			t.Fatalf("ResolveSessionAgentWithRuntime(route 2) error = %v", err)
		}
		if mutate != nil {
			mutate(&resolved)
		}
		source.commitAcceptedRoute(acceptedRouteRecord(2, resolved, ""), deriveRouteTwoCommand)
		if err := h.manager.persistSessionMetadataOnly(source); err != nil {
			t.Fatalf("persistSessionMetadataOnly(source) error = %v", err)
		}
	}
	newRouteSource := func(t *testing.T, h *deriveHarness) *Session {
		t.Helper()
		created, err := h.manager.CreateAccepted(testutil.Context(t), CreateAcceptedOpts{
			Session: CreateOpts{AgentName: "b", Workspace: h.workspaceID},
		})
		if err != nil {
			t.Fatalf("CreateAccepted(b) error = %v", err)
		}
		source, _ := h.manager.Get(created.ID)
		h.promptSource(t, source.ID, "Work on route two")
		return source
	}

	t.Run("Should inherit the source's accepted route as the child's pending route", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := newRouteSource(t, h)
		commitSourceRoute(t, h, source, nil)
		result, err := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_fork_route"))
		if err != nil {
			t.Fatalf("ForkSession() error = %v", err)
		}
		pending := h.childMeta(t, result.Child.ID).Derivation.PendingRoute
		if pending == nil || pending.Index != 2 ||
			pending.CommandFingerprint != compozyconfig.CommandFingerprint(deriveRouteTwoCommand) {
			t.Fatalf("child pending route = %+v, want route 2 with its fingerprint", pending)
		}
	})

	t.Run("Should not inherit an accepted route the child cannot bind compatibly", func(t *testing.T) {
		t.Parallel()
		h := newDeriveHarness(t)
		source := newRouteSource(t, h)
		commitSourceRoute(t, h, source, func(resolved *compozyconfig.ResolvedAgent) {
			resolved.HomePolicy = "isolated-elsewhere"
		})
		result, err := h.manager.ForkSession(testutil.Context(t), h.forkOpts(source, "idem_fork_no_route"))
		if err != nil {
			t.Fatalf("ForkSession() error = %v", err)
		}
		if pending := h.childMeta(t, result.Child.ID).Derivation.PendingRoute; pending != nil {
			t.Fatalf("child pending route = %+v, want none for an incompatible accepted route", pending)
		}
	})
}
