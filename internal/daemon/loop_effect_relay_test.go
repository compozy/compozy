package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/testutil"
	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	loopdsl "github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// Invariant: the daemon relay isolates entries, uses daemon workspace scope, and closes each
// committed delivery independently. This is the canonical relay suite; only tool I/O is faked.
func TestLoopEffectRelayShouldDrainCommittedEntriesInIsolation(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 2, 14, 0, 0, 0, time.UTC)
	store := newLoopEffectStoreFake(
		loopEffectEntryForTest("missing", 0, `{"kind":"tool","tool":"missing__notify","with":{"body":"one"}}`),
		loopEffectEntryForTest("success", 1, `{"kind":"tool","tool":"known__notify","with":{"body":"two"}}`),
		loopEffectEntryForTest("emit", 2, `{"kind":"emit","emit":{"kind":"fetch_failed","payload":{"safe":true}}}`),
	)
	tools := newLoopEffectToolsFake()
	tools.views["known__notify"] = toolspkg.ToolView{
		Descriptor: toolspkg.Descriptor{ID: "known__notify"},
		Decision:   toolspkg.EffectiveToolDecision{Callable: true},
	}
	relay := &loopEffectRelay{store: store, tools: tools, now: func() time.Time { return now }}
	report, err := relay.DrainPendingLoopEffects(t.Context(), looppkg.EffectDrainPage{Limit: 2})
	if err != nil {
		t.Fatalf("DrainPendingLoopEffects() error = %v", err)
	}
	if report.Read != 3 || report.Delivered != 2 || report.Failed != 1 {
		t.Fatalf("drain report = %#v, want 3 read, 2 delivered, 1 failed", report)
	}
	acks := store.acknowledgements()
	if len(acks) != 3 || acks[0].Outcome != looppkg.EffectResultToolMissing ||
		acks[1].Outcome != looppkg.EffectResultOK || acks[2].Outcome != looppkg.EffectResultOK ||
		len(acks[2].CustomEvent) == 0 {
		t.Fatalf("acknowledgements = %#v, want isolated missing, tool success, and custom event", acks)
	}
	calls := tools.callsSnapshot()
	if len(calls) != 1 || calls[0].request.CorrelationID != store.original[1].DeliveryID ||
		calls[0].scope.WorkspaceID != "ws-effect" || calls[0].scope.AgentName != loopEffectActorName ||
		calls[0].scope.ActorKind != string(workspaceaccess.ActorDaemon) {
		t.Fatalf("tool calls = %#v, want daemon:loop-effect workspace correlation", calls)
	}
}

func TestLoopEffectRelayShouldDeliverNativePromptThroughDaemonWorkspaceScope(t *testing.T) {
	t.Parallel()

	entry := loopEffectEntryForTest(
		"native-session-prompt",
		0,
		`{"kind":"tool","tool":"compozy__session_prompt","with":{"workspace":"ws-effect","session_id":"sess-target","message":"Continue the verified flow.","message_id":"msg-effect","idempotency_key":"idem-effect"}}`,
	)
	effectStore := newLoopEffectStoreFake(entry)
	var (
		promptCalls int
		promptID    string
		promptOpts  session.SendPromptOpts
	)
	sessions := testutil.StubSessionManager{
		StatusFn: func(ctx context.Context, id string) (*session.Info, error) {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			if id != "sess-target" {
				return nil, session.ErrSessionNotFound
			}
			return &session.Info{
				ID:          "sess-target",
				AgentName:   "worker",
				WorkspaceID: "ws-effect",
				State:       session.StateActive,
			}, nil
		},
		SendPromptFn: func(
			ctx context.Context,
			id string,
			opts session.SendPromptOpts,
		) (session.SendPromptResult, error) {
			if err := ctx.Err(); err != nil {
				return session.SendPromptResult{}, err
			}
			promptCalls++
			promptID = id
			promptOpts = opts
			return session.SendPromptResult{
				Status:    "accepted",
				Delivery:  store.SessionInputDeliveryDirect,
				NewTurnID: "turn-effect",
			}, nil
		},
	}
	cfg := testConfig(t, testHomePaths(t))
	cfg.Permissions.Mode = compozyconfig.PermissionModeApproveAll
	agents := &nativeToolPolicyAgentResolverStub{
		agent: compozyconfig.AgentDef{Name: "authored-agent", Provider: "opencode", Prompt: "Work."},
	}
	policyResolver, err := newNativeToolPolicyResolver(nativeToolPolicyResolverDeps{
		Config:            &cfg,
		Sessions:          sessions,
		AgentResolver:     agents,
		ApprovalAvailable: true,
	})
	if err != nil {
		t.Fatalf("newNativeToolPolicyResolver() error = %v", err)
	}
	registry := newDaemonNativeRegistryWithPolicyResolverAndWorkspaceAccess(
		t,
		&daemonNativeToolsDeps{
			Sessions:   sessions,
			Workspaces: nativeNetworkTestWorkspaceService(t),
		},
		policyResolver,
		nil,
	)
	tools := &loopEffectRegistryRecorder{registry: registry}
	relay := &loopEffectRelay{store: effectStore, tools: tools}

	report, err := relay.DrainPendingLoopEffects(t.Context(), looppkg.EffectDrainPage{})
	if err != nil {
		t.Fatalf("DrainPendingLoopEffects() error = %v", err)
	}
	acks := effectStore.acknowledgements()
	if report.Read != 1 || report.Delivered != 1 || report.Failed != 0 ||
		len(acks) != 1 || acks[0].Outcome != looppkg.EffectResultOK {
		t.Fatalf("drain report/acknowledgements = %#v/%#v, want one successful delivery", report, acks)
	}
	if promptCalls != 1 || promptID != "sess-target" || promptOpts.Message != "Continue the verified flow." {
		t.Fatalf(
			"prompt calls/id/options = %d/%q/%#v, want one prompt to sess-target",
			promptCalls,
			promptID,
			promptOpts,
		)
	}
	calls := tools.callsSnapshot()
	if len(calls) != 1 || calls[0].scope.ActorKind != string(workspaceaccess.ActorDaemon) ||
		calls[0].scope.WorkspaceID != "ws-effect" ||
		calls[0].request.ActorKind != string(workspaceaccess.ActorDaemon) ||
		calls[0].request.CorrelationID != entry.DeliveryID {
		t.Fatalf("native registry calls = %#v, want daemon actor and stable delivery correlation", calls)
	}
	if len(agents.calls) != 0 {
		t.Fatalf("ResolveAgent calls = %#v, want no authored-agent lookup for daemon effect", agents.calls)
	}
}

func TestLoopEffectRelayShouldFailApprovalAndRenderErrorsWithoutCallingTools(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 2, 14, 5, 0, 0, time.UTC)
	store := newLoopEffectStoreFake(
		loopEffectEntryForTest("approval", 0, `{"kind":"tool","tool":"known__approval","with":{}}`),
		loopEffectEntryForTest(
			"render",
			1,
			`{"kind":"render_error","render_error":true,"diagnostic":"missing effect value"}`,
		),
	)
	tools := newLoopEffectToolsFake()
	tools.views["known__approval"] = toolspkg.ToolView{
		Descriptor: toolspkg.Descriptor{ID: "known__approval"},
		Decision:   toolspkg.EffectiveToolDecision{Callable: true, ApprovalRequired: true},
	}
	relay := &loopEffectRelay{store: store, tools: tools, now: func() time.Time { return now }}
	report, err := relay.DrainPendingLoopEffects(t.Context(), looppkg.EffectDrainPage{})
	if err != nil {
		t.Fatalf("DrainPendingLoopEffects() error = %v", err)
	}
	acks := store.acknowledgements()
	if report.Failed != 2 || len(acks) != 2 ||
		acks[0].Outcome != looppkg.EffectResultApprovalRequired ||
		acks[0].Code != "effect_tool_approval_required" ||
		acks[1].Code != "effect_render_failed" {
		t.Fatalf("report/acks = %#v / %#v, want deterministic approval and render failures", report, acks)
	}
	if calls := tools.callsSnapshot(); len(calls) != 0 {
		t.Fatalf("tool calls = %#v, want no execution for approval or render failure", calls)
	}
}

func TestLoopEffectRelayShouldReexecuteAfterPostToolAckCrash(t *testing.T) {
	t.Run("Should report the delivery before replaying the unacknowledged effect", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, time.August, 2, 14, 10, 0, 0, time.UTC)
		store := newLoopEffectStoreFake(
			loopEffectEntryForTest("replay", 0, `{"kind":"tool","tool":"known__notify","with":{"body":"retry"}}`),
		)
		store.failAcks = 1
		tools := newLoopEffectToolsFake()
		tools.views["known__notify"] = toolspkg.ToolView{
			Descriptor: toolspkg.Descriptor{ID: "known__notify"},
			Decision:   toolspkg.EffectiveToolDecision{Callable: true},
		}
		relay := &loopEffectRelay{store: store, tools: tools, now: func() time.Time { return now }}
		if _, err := relay.DrainPendingLoopEffects(t.Context(), looppkg.EffectDrainPage{}); err == nil {
			t.Fatal("DrainPendingLoopEffects(first) error = nil, want simulated post-tool ack crash")
		} else if !strings.Contains(err.Error(), "simulated ack crash") ||
			!strings.Contains(err.Error(), store.original[0].DeliveryID) {
			t.Fatalf("DrainPendingLoopEffects(first) error = %v, want delivery id and ack crash", err)
		}
		if _, err := relay.DrainPendingLoopEffects(t.Context(), looppkg.EffectDrainPage{}); err != nil {
			t.Fatalf("DrainPendingLoopEffects(replay) error = %v", err)
		}
		if calls := tools.callsSnapshot(); len(calls) != 2 ||
			calls[0].request.CorrelationID != calls[1].request.CorrelationID {
			t.Fatalf("tool calls = %#v, want at-least-once replay with stable delivery correlation", calls)
		}
		if acks := store.acknowledgements(); len(acks) != 1 {
			t.Fatalf("acknowledgements = %#v, want one durable winner", acks)
		}
	})
}

func TestLoopEffectRelayShouldStopAfterAFullPageWithoutAcknowledgement(t *testing.T) {
	t.Run("Should end the pass when every acknowledgement loses its race", func(t *testing.T) {
		t.Parallel()

		store := newLoopEffectStoreFake(
			loopEffectEntryForTest("stale-ack", 0, `{"kind":"emit","emit":{"kind":"stale"}}`),
		)
		store.rejectAcks = true
		relay := &loopEffectRelay{store: store, tools: newLoopEffectToolsFake()}
		report, err := relay.DrainPendingLoopEffects(t.Context(), looppkg.EffectDrainPage{Limit: 1})
		if err != nil {
			t.Fatalf("DrainPendingLoopEffects() error = %v", err)
		}
		if report.Read != 1 || report.Delivered != 0 || report.Failed != 0 {
			t.Fatalf("drain report = %#v, want one read and no acknowledged outcomes", report)
		}
	})
}

func TestLoopEffectRelayShouldDrainPersistedWorkOnRunStart(t *testing.T) {
	t.Parallel()

	store := newLoopEffectStoreFake(
		loopEffectEntryForTest("boot-drain", 0, `{"kind":"emit","emit":{"kind":"boot_recovered"}}`),
	)
	relay := &loopEffectRelay{
		store: store, tools: newLoopEffectToolsFake(), pollInterval: time.Hour,
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		relay.Run(ctx)
	}()
	select {
	case <-store.acknowledged:
	case <-time.After(time.Second):
		cancel()
		<-done
		t.Fatal("relay did not drain committed work before its first poll or nudge")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("relay did not stop after cancellation")
	}
	if acknowledgements := store.acknowledgements(); len(acknowledgements) != 1 ||
		acknowledgements[0].Outcome != looppkg.EffectResultOK {
		t.Fatalf("acknowledgements = %#v, want one recovered delivery", acknowledgements)
	}
}

// Invariant: the relay reads committed SQLite rows, persists result events, and closes the
// outbox row through the real store implementation. The global database is the owning layer.
func TestLoopEffectRelayShouldDrainRealSQLiteOutboxEndToEnd(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	now := time.Date(2026, time.August, 2, 14, 20, 0, 0, time.UTC)
	db := openDaemonTestGlobalDB(t)
	if err := db.InsertWorkspace(ctx, workspacepkg.Workspace{
		ID: "ws-scheduler", RootDir: t.TempDir(), Name: "Effects", CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
	run := looppkg.Run{
		ID: "looprun-effect-integration", WorkspaceID: "ws-scheduler",
		LoopName: "effect-integration", Status: looppkg.StatusRunning,
		ReattemptStrategy: looppkg.ReattemptFailedOnly, IterationCap: 3,
		BudgetOnExceeded: loopdsl.BudgetExceededHalt, CreatedAt: now, LastProgressAt: now,
	}
	applyLoopRunPinningForTest(t, &run, now)
	created, err := db.CreateLoopRunForStart(ctx, run, loopdsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	const sourceEventID = "loopevt-effect-integration-source"
	deliveryID := looppkg.EffectDeliveryID(
		created.ID,
		sourceEventID,
		looppkg.EffectTriggerOnError,
		0,
	)
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx(seed effect outbox) error = %v", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			t.Errorf("Rollback(seed effect outbox) error = %v", rollbackErr)
		}
	}()
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO loop_run_events (
			id, loop_run_id, workspace_id, seq, kind, payload_json, at
		) VALUES (?, ?, ?, (SELECT COALESCE(MAX(seq), 0) + 1 FROM loop_run_events WHERE loop_run_id = ?),
			'node_failed', '{"node_id":"fetch","generation":1,"item_index":0}', ?)`,
		sourceEventID,
		string(created.ID),
		string(created.WorkspaceID),
		string(created.ID),
		now,
	); err != nil {
		t.Fatalf("insert source event error = %v", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO loop_effect_outbox (
			loop_run_id, delivery_id, source_event_id, trigger, generation,
			node_id, item_index, entry_index, entry_json, created_at
		) VALUES (?, ?, ?, 'on_error', 1, 'fetch', 0, 0, ?, ?)`,
		string(created.ID),
		deliveryID,
		sourceEventID,
		`{"kind":"tool","tool":"known__notify","with":{"body":"failed"}}`,
		now,
	); err != nil {
		t.Fatalf("insert effect outbox error = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit(seed effect outbox) error = %v", err)
	}
	committed = true
	tools := newLoopEffectToolsFake()
	tools.views["known__notify"] = toolspkg.ToolView{
		Descriptor: toolspkg.Descriptor{ID: "known__notify"},
		Decision:   toolspkg.EffectiveToolDecision{Callable: true},
	}
	relay := &loopEffectRelay{store: db, tools: tools, now: func() time.Time { return now.Add(time.Second) }}
	report, err := relay.DrainPendingLoopEffects(ctx, looppkg.EffectDrainPage{})
	if err != nil {
		t.Fatalf("DrainPendingLoopEffects() error = %v", err)
	}
	if report.Read != 1 || report.Delivered != 1 || report.Failed != 0 {
		t.Fatalf("drain report = %#v, want one delivered SQLite row", report)
	}
	outbox, err := db.ListEffectOutbox(ctx, created.WorkspaceID, created.ID)
	if err != nil {
		t.Fatalf("ListEffectOutbox() error = %v", err)
	}
	if len(outbox) != 1 || outbox[0].State != looppkg.EffectDelivered || outbox[0].Attempts != 1 {
		t.Fatalf("effect outbox = %#v, want one delivered attempt", outbox)
	}
	events, err := db.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
		ReadScope:   store.ReadScope{AllProfiles: true},
		WorkspaceID: created.WorkspaceID,
		RunID:       created.ID,
	})
	if err != nil {
		t.Fatalf("ListLoopRunEvents() error = %v", err)
	}
	resultEvents := 0
	for _, event := range events {
		if event.DeliveryKey == deliveryID+":effect_results" {
			resultEvents++
		}
	}
	if resultEvents != 1 {
		t.Fatalf("effect result events = %d, want exactly one idempotent append", resultEvents)
	}
}

type loopEffectStoreFake struct {
	mu           sync.Mutex
	pending      []looppkg.EffectOutboxEntry
	original     []looppkg.EffectOutboxEntry
	acks         []looppkg.EffectAcknowledgement
	failAcks     int
	rejectAcks   bool
	acknowledged chan struct{}
}

func newLoopEffectStoreFake(entries ...looppkg.EffectOutboxEntry) *loopEffectStoreFake {
	return &loopEffectStoreFake{
		pending:      append([]looppkg.EffectOutboxEntry(nil), entries...),
		original:     append([]looppkg.EffectOutboxEntry(nil), entries...),
		acknowledged: make(chan struct{}, 1),
	}
}

func (s *loopEffectStoreFake) ListPendingLoopEffects(
	_ context.Context,
	limit int,
) ([]looppkg.EffectOutboxEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	limit = min(max(limit, 1), len(s.pending))
	return append([]looppkg.EffectOutboxEntry(nil), s.pending[:limit]...), nil
}

func (s *loopEffectStoreFake) AcknowledgeLoopEffect(
	_ context.Context,
	ack looppkg.EffectAcknowledgement,
) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failAcks > 0 {
		s.failAcks--
		return false, errors.New("simulated ack crash")
	}
	if s.rejectAcks {
		return false, nil
	}
	for index, entry := range s.pending {
		if entry.DeliveryID != ack.Entry.DeliveryID {
			continue
		}
		s.pending = append(s.pending[:index], s.pending[index+1:]...)
		s.acks = append(s.acks, ack)
		select {
		case s.acknowledged <- struct{}{}:
		default:
		}
		return true, nil
	}
	return false, nil
}

func (s *loopEffectStoreFake) acknowledgements() []looppkg.EffectAcknowledgement {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]looppkg.EffectAcknowledgement(nil), s.acks...)
}

type loopEffectToolCall struct {
	scope   toolspkg.Scope
	request toolspkg.CallRequest
}

type loopEffectRegistryRecorder struct {
	mu       sync.Mutex
	registry *toolspkg.RuntimeRegistry
	calls    []loopEffectToolCall
}

func (r *loopEffectRegistryRecorder) Get(
	ctx context.Context,
	scope toolspkg.Scope,
	id toolspkg.ToolID,
) (toolspkg.ToolView, error) {
	return r.registry.Get(ctx, scope, id)
}

func (r *loopEffectRegistryRecorder) Call(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	r.mu.Lock()
	r.calls = append(r.calls, loopEffectToolCall{scope: scope, request: req})
	r.mu.Unlock()
	return r.registry.Call(ctx, scope, req)
}

func (r *loopEffectRegistryRecorder) callsSnapshot() []loopEffectToolCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]loopEffectToolCall(nil), r.calls...)
}

type loopEffectToolsFake struct {
	mu       sync.Mutex
	views    map[toolspkg.ToolID]toolspkg.ToolView
	callErrs map[toolspkg.ToolID]error
	calls    []loopEffectToolCall
}

func newLoopEffectToolsFake() *loopEffectToolsFake {
	return &loopEffectToolsFake{
		views:    make(map[toolspkg.ToolID]toolspkg.ToolView),
		callErrs: make(map[toolspkg.ToolID]error),
	}
}

func (f *loopEffectToolsFake) Get(
	_ context.Context,
	_ toolspkg.Scope,
	id toolspkg.ToolID,
) (toolspkg.ToolView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	view, ok := f.views[id]
	if !ok {
		return toolspkg.ToolView{}, toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			id,
			"tool not found",
			toolspkg.ErrToolNotFound,
		)
	}
	return view, nil
}

func (f *loopEffectToolsFake) Call(
	_ context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, loopEffectToolCall{scope: scope, request: req})
	return toolspkg.ToolResult{Structured: json.RawMessage(`{"ok":true}`)}, f.callErrs[req.ToolID]
}

func (f *loopEffectToolsFake) callsSnapshot() []loopEffectToolCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]loopEffectToolCall(nil), f.calls...)
}

func loopEffectEntryForTest(label string, index int, raw string) looppkg.EffectOutboxEntry {
	sourceEventID := "event-" + label
	return looppkg.EffectOutboxEntry{
		LoopRunID: "run-effect", WorkspaceID: "ws-effect",
		DeliveryID:    looppkg.EffectDeliveryID("run-effect", sourceEventID, looppkg.EffectTriggerOnError, index),
		SourceEventID: sourceEventID, Trigger: string(looppkg.EffectTriggerOnError),
		Generation: 1, NodeID: "fetch", EntryIndex: index, Entry: json.RawMessage(raw),
		State: looppkg.EffectPending, CreatedAt: time.Date(2026, time.August, 2, 13, 0, 0, 0, time.UTC),
	}
}
