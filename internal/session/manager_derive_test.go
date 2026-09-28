package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
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
func newDeriveHarness(t *testing.T, extraOpts ...Option) *deriveHarness {
	t.Helper()
	h := newHarness(t)
	db := openManagerInputQueueStore(t)
	registerManagerInputQueueWorkspace(t, db, h)
	setDeriveAgentB(t, h, deriveRoutes(h, deriveRouteOneCommand, deriveRouteTwoCommand))
	opts := append([]Option{
		WithSessionCatalog(db), WithEventLedger(db), WithSessionPromptAdmissionStore(db),
	}, extraOpts...)
	h.manager = newManagerWithHarness(t, h, opts...)
	cleanupTestManager(t, h.manager)
	return &deriveHarness{harness: h, db: db}
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
		h.manager = newManagerWithHarness(
			t, h.harness, WithSessionCatalog(h.db), WithEventLedger(h.db), WithSessionPromptAdmissionStore(h.db),
		)
		cleanupTestManager(t, h.manager)
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
		if preview.Cut != nil || preview.MaxSequence == 0 || preview.MessageCount != 4 {
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
		if imported := h.childMeta(t, result.Child.ID).ImportedContext; imported == nil || imported.MessagesJSON != "[]" {
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
