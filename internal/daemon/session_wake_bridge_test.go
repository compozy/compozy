package daemon

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	monitorpkg "github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

type sessionWakeManagerStub struct {
	mu          sync.Mutex
	status      *session.Info
	statusErr   error
	promptErr   error
	promptID    string
	promptOpts  session.SyntheticPromptOpts
	promptCalls int
	events      chan acp.AgentEvent
	ctxErr      error
}

func (stub *sessionWakeManagerStub) Status(ctx context.Context, _ string) (*session.Info, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.ctxErr = ctx.Err()
	return stub.status, stub.statusErr
}

func (stub *sessionWakeManagerStub) PromptSynthetic(
	ctx context.Context,
	id string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.ctxErr = ctx.Err()
	stub.promptID = id
	stub.promptOpts = opts
	stub.promptCalls++
	return stub.events, stub.promptErr
}

func TestSessionWakeBridge(t *testing.T) {
	t.Parallel()

	t.Run("Should deliver detached bounded metadata and drain through shutdown", func(t *testing.T) {
		t.Parallel()

		events := make(chan acp.AgentEvent)
		manager := &sessionWakeManagerStub{
			status: &session.Info{ID: "parent-1", State: session.StateActive},
			events: events,
		}
		bridge, err := newSessionWakeBridge(
			testutil.Context(t),
			func() sessionWakeSessionManager { return manager },
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
		if err != nil {
			t.Fatalf("newSessionWakeBridge() error = %v", err)
		}
		canceledCtx, cancel := context.WithCancel(testutil.Context(t))
		cancel()
		event := session.SpawnWakeEvent{
			ChildSessionID: "child-1",
			ChildAgentName: "researcher",
			Reason:         session.SpawnWakeReasonNeedsAttention,
			Badge:          session.BadgeWaitingForInput,
			Detail:         "Include vendored packages? compozy_claim_private-token" + strings.Repeat("界", 300),
			WakeEventID:    "wake-1",
		}

		if err := bridge.WakeSpawnCreator(canceledCtx, "parent-1", event); err != nil {
			t.Fatalf("WakeSpawnCreator() error = %v", err)
		}
		manager.mu.Lock()
		promptOpts := manager.promptOpts
		promptID := manager.promptID
		promptCalls := manager.promptCalls
		ctxErr := manager.ctxErr
		manager.mu.Unlock()
		if ctxErr != nil {
			t.Fatalf("delivery context error = %v, want detached context", ctxErr)
		}
		if promptCalls != 1 || promptID != "parent-1" || promptOpts.SkipIfBusy {
			t.Fatalf(
				"prompt delivery = calls %d id %q opts %#v, want one queued wake",
				promptCalls,
				promptID,
				promptOpts,
			)
		}
		if got := len([]rune(promptOpts.Message)); got > 240 {
			t.Fatalf("wake prompt runes = %d, want <= 240", got)
		}
		if strings.Contains(promptOpts.Message, "compozy_claim_private-token") {
			t.Fatalf("wake prompt leaked claim token: %q", promptOpts.Message)
		}
		if !strings.Contains(promptOpts.Message, "compozy__session_clarify_answer") ||
			!strings.Contains(promptOpts.Message, "compozy__session_events") {
			t.Fatalf("wake prompt = %q, want actionable native tools", promptOpts.Message)
		}
		if promptOpts.Metadata.ChildSessionID != "child-1" ||
			promptOpts.Metadata.ChildAgentName != "researcher" ||
			promptOpts.Metadata.Badge != string(session.BadgeWaitingForInput) ||
			promptOpts.Metadata.WakeEventID != "wake-1" {
			t.Fatalf("synthetic metadata = %#v, want exact child identity", promptOpts.Metadata)
		}
		if err := bridge.shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("shutdown() error = %v", err)
		}
	})

	earlyStopWakePrompt := func(t *testing.T, event session.SpawnWakeEvent) string {
		t.Helper()
		manager := &sessionWakeManagerStub{
			status: &session.Info{ID: "parent-1", State: session.StateActive},
			events: make(chan acp.AgentEvent),
		}
		bridge, err := newSessionWakeBridge(
			testutil.Context(t),
			func() sessionWakeSessionManager { return manager },
			slog.New(slog.NewTextHandler(io.Discard, nil)),
		)
		if err != nil {
			t.Fatalf("newSessionWakeBridge() error = %v", err)
		}
		t.Cleanup(func() {
			if err := bridge.shutdown(testutil.Context(t)); err != nil {
				t.Errorf("shutdown() error = %v", err)
			}
		})
		event.ChildSessionID = cmp.Or(event.ChildSessionID, "child-1")
		event.Reason, event.Badge = session.SpawnWakeReasonNeedsAttention, session.BadgeNeedsAttention
		event.Detail, event.EndedEarly = "Turn ended without a linked pull request.", true
		event.WakeEventID = "wake-early"
		if err := bridge.WakeSpawnCreator(testutil.Context(t), "parent-1", event); err != nil {
			t.Fatalf("WakeSpawnCreator() error = %v", err)
		}
		manager.mu.Lock()
		defer manager.mu.Unlock()
		return manager.promptOpts.Message
	}

	t.Run("Should carry the early-stopped child's final answer in the wake prompt", func(t *testing.T) {
		t.Parallel()

		answer := "Delivery incomplete; next step is to push the branch. " + strings.Repeat("detail ", 60)
		message := earlyStopWakePrompt(t, session.SpawnWakeEvent{FinalAnswer: answer})
		if !strings.Contains(message, "Final answer:\n---\n"+strings.TrimSpace(answer)) ||
			!strings.Contains(message, "compozy__session_stop") ||
			!strings.Contains(message, "Detail: Turn ended without a linked pull request.") {
			t.Fatalf("wake prompt = %q, want the full bounded final answer and next actions", message)
		}
	})

	t.Run("Should truncate an early-stop final answer above the bound", func(t *testing.T) {
		t.Parallel()

		answer := strings.Repeat("reply ", 1000) + "end"
		// The bound is 4000 runes, including the 3-rune "..." suffix.
		want := string([]rune(answer)[:4000-3]) + "..."
		message := earlyStopWakePrompt(t, session.SpawnWakeEvent{FinalAnswer: answer})
		if !strings.HasSuffix(message, "Final answer:\n---\n"+want) || strings.Contains(message, answer) {
			t.Fatalf("wake prompt = %q, want the final answer truncated to the bound", message)
		}
	})

	t.Run("Should keep the early-stop guidance when the final answer is unreadable", func(t *testing.T) {
		t.Parallel()

		message := earlyStopWakePrompt(t, session.SpawnWakeEvent{})
		if !strings.Contains(message, "It stopped before delivering.") ||
			!strings.Contains(message, "compozy__session_stop") || strings.Contains(message, "Final answer:") {
			t.Fatalf("wake prompt = %q, want the continue-or-stop guidance without an answer block", message)
		}
	})

	t.Run("Should redact child identifiers in the early-stop wake prompt", func(t *testing.T) {
		t.Parallel()

		message := earlyStopWakePrompt(t, session.SpawnWakeEvent{
			ChildAgentName: "compozy_claim_private-token", FinalAnswer: "partial",
		})
		if strings.Contains(message, "compozy_claim_private-token") || !strings.Contains(message, "child-1") {
			t.Fatalf("wake prompt = %q, want the child identity with the claim token redacted", message)
		}
	})

	t.Run("Should classify a stopped creator as not live", func(t *testing.T) {
		t.Parallel()

		manager := &sessionWakeManagerStub{status: &session.Info{ID: "parent-1", State: session.StateStopped}}
		bridge, err := newSessionWakeBridge(
			testutil.Context(t),
			func() sessionWakeSessionManager { return manager },
			nil,
		)
		if err != nil {
			t.Fatalf("newSessionWakeBridge() error = %v", err)
		}
		t.Cleanup(func() {
			if shutdownErr := bridge.shutdown(testutil.Context(t)); shutdownErr != nil {
				t.Errorf("shutdown() error = %v", shutdownErr)
			}
		})

		err = bridge.WakeSpawnCreator(testutil.Context(t), "parent-1", validSessionWakeEvent())
		if !errors.Is(err, session.ErrSpawnWakeSessionNotLive) {
			t.Fatalf("WakeSpawnCreator() error = %v, want ErrSpawnWakeSessionNotLive", err)
		}
		if manager.promptCalls != 0 {
			t.Fatalf("PromptSynthetic() calls = %d, want 0", manager.promptCalls)
		}
	})

	t.Run("Should reject wake admission after shutdown starts", func(t *testing.T) {
		t.Parallel()

		manager := &sessionWakeManagerStub{status: &session.Info{ID: "parent-1", State: session.StateActive}}
		bridge, err := newSessionWakeBridge(
			testutil.Context(t),
			func() sessionWakeSessionManager { return manager },
			nil,
		)
		if err != nil {
			t.Fatalf("newSessionWakeBridge() error = %v", err)
		}
		if err := bridge.shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("shutdown() error = %v", err)
		}
		if err := bridge.WakeSpawnCreator(
			testutil.Context(t),
			"parent-1",
			validSessionWakeEvent(),
		); err == nil ||
			!strings.Contains(err.Error(), "stopping") {
			t.Fatalf("WakeSpawnCreator() error = %v, want stopping rejection", err)
		}
	})
}

func validSessionWakeEvent() session.SpawnWakeEvent {
	return session.SpawnWakeEvent{
		ChildSessionID: "child-1",
		Reason:         session.SpawnWakeReasonStopped,
		Badge:          session.BadgeStopped,
		WakeEventID:    "wake-1",
	}
}

type monitorWakeManagerStub struct {
	status     *session.Info
	statusErr  error
	enqueueErr error
	admitted   bool
	calls      int
	id         string
	opts       session.SyntheticPromptOpts
}

func (stub *monitorWakeManagerStub) Status(context.Context, string) (*session.Info, error) {
	return stub.status, stub.statusErr
}

func (stub *monitorWakeManagerStub) SyntheticWakeAdmitted(context.Context, string, string) (bool, error) {
	return stub.admitted, nil
}

func (stub *monitorWakeManagerStub) EnqueueSyntheticWake(
	_ context.Context,
	id string,
	opts session.SyntheticPromptOpts,
) (store.SessionInputQueueEntry, error) {
	stub.calls++
	stub.id, stub.opts = id, opts
	return store.SessionInputQueueEntry{ID: "inq-1"}, stub.enqueueErr
}

func TestMonitorSessionWaker(t *testing.T) {
	t.Parallel()

	fired := monitorpkg.Notification{
		Monitor: monitorpkg.Monitor{
			ID: "mon-0123456789ab", SessionID: "sess-1", Kind: monitorpkg.KindTerminal,
			Description: "release workflow", TerminalID: "term-0123456789ab",
		},
		Outcome: monitorpkg.Outcome{Reason: monitorpkg.ReasonExit, Output: "done"},
	}
	wakerFor := func(manager *monitorWakeManagerStub) *monitorSessionWaker {
		return &monitorSessionWaker{sessions: func() monitorSessionManager { return manager }}
	}

	t.Run("Should admit one attributed, idempotent monitor wake for an active session", func(t *testing.T) {
		t.Parallel()

		manager := &monitorWakeManagerStub{status: &session.Info{ID: "sess-1", State: session.StateActive}}
		if err := wakerFor(manager).WakeSession(testutil.Context(t), fired); err != nil {
			t.Fatalf("WakeSession() error = %v", err)
		}
		if manager.calls != 1 || manager.id != "sess-1" || manager.opts.MessageID != "monitor:mon-0123456789ab" {
			t.Fatalf("admission = calls %d id %q opts %#v, want one idempotent wake", manager.calls,
				manager.id, manager.opts)
		}
		meta := manager.opts.Metadata
		if meta.Kind != acp.PromptSyntheticKindMonitor || meta.Reason != "monitor_exit" ||
			meta.WakeEventID != "mon-0123456789ab" || meta.Summary != "release workflow" {
			t.Fatalf("synthetic metadata = %#v, want monitor attribution", meta)
		}
		if !strings.Contains(manager.opts.Message, "NOT USER INPUT") {
			t.Fatalf("wake message = %q, want automated-wake header", manager.opts.Message)
		}
	})

	t.Run("Should not admit a wake again when its queue entry was already admitted", func(t *testing.T) {
		t.Parallel()

		manager := &monitorWakeManagerStub{
			status:   &session.Info{ID: "sess-1", State: session.StateActive},
			admitted: true,
		}
		if err := wakerFor(manager).WakeSession(testutil.Context(t), fired); err != nil {
			t.Fatalf("WakeSession() error = %v", err)
		}
		if manager.calls != 0 {
			t.Fatalf("enqueue calls = %d, want none for an already admitted wake", manager.calls)
		}
	})

	t.Run("Should refuse permanently for a stopped or missing session without admitting", func(t *testing.T) {
		t.Parallel()

		for name, manager := range map[string]*monitorWakeManagerStub{
			"stopped": {status: &session.Info{ID: "sess-1", State: session.StateStopped}},
			"missing": {statusErr: session.ErrSessionNotFound},
			"removed": {statusErr: fmt.Errorf("read session meta: %w", store.ErrSessionNotFound)},
		} {
			err := wakerFor(manager).WakeSession(testutil.Context(t), fired)
			if !errors.Is(err, monitorpkg.ErrSessionGone) || manager.calls != 0 {
				t.Fatalf("%s: WakeSession() error = %v calls = %d, want ErrSessionGone and no admission",
					name, err, manager.calls)
			}
		}
	})

	t.Run("Should report a full input queue as transient", func(t *testing.T) {
		t.Parallel()

		manager := &monitorWakeManagerStub{
			status:     &session.Info{ID: "sess-1", State: session.StateActive},
			enqueueErr: store.ErrSessionInputQueueFull,
		}
		err := wakerFor(manager).WakeSession(testutil.Context(t), fired)
		if !errors.Is(err, store.ErrSessionInputQueueFull) || errors.Is(err, monitorpkg.ErrSessionGone) {
			t.Fatalf("WakeSession() error = %v, want transient ErrSessionInputQueueFull", err)
		}
	})

	t.Run("Should cancel a session's monitors when it stops", func(t *testing.T) {
		t.Parallel()

		monitors, err := monitorpkg.NewService(testutil.Context(t), nil, wakerFor(&monitorWakeManagerStub{}))
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		t.Cleanup(func() {
			if err := monitors.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})
		if _, err := monitors.Create(testutil.Context(t), monitorpkg.Spec{
			WorkspaceID: "ws-1", SessionID: "sess-1", Kind: monitorpkg.KindTimer,
			Description: "check CI", Delay: time.Hour,
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		monitorLifecycleObserver{
			monitors: monitors,
		}.OnSessionStopped(
			testutil.Context(t),
			&session.Session{ID: "sess-1"},
		)
		if listed := monitors.List("sess-1"); len(listed) != 0 {
			t.Fatalf("List() after stop = %+v, want empty", listed)
		}
	})

	t.Run("Should keep a monitor durable when a daemon restart interrupts its owner", func(t *testing.T) {
		t.Parallel()

		durable := &monitorStoreStub{records: map[string]monitorpkg.Record{}}
		monitors, err := monitorpkg.NewService(
			testutil.Context(t), nil, wakerFor(&monitorWakeManagerStub{}), monitorpkg.WithStore(durable),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		t.Cleanup(func() {
			if err := monitors.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("Shutdown() error = %v", err)
			}
		})
		if _, err := monitors.Create(testutil.Context(t), monitorpkg.Spec{
			WorkspaceID: "ws-1", SessionID: "sess-1", Kind: monitorpkg.KindTimer,
			Description: "check CI", Delay: time.Hour,
		}); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		observer := monitorLifecycleObserver{monitors: monitors}
		observer.release("sess-1", true)
		if counts := monitors.ActiveCounts([]string{"sess-1"}); len(counts) != 0 {
			t.Fatalf("ActiveCounts() after interruption = %+v, want the watch stopped", counts)
		}
		for _, record := range durable.records {
			if record.Monitor.Status != monitorpkg.StatusActive {
				t.Fatalf("durable record = %+v, want it left active for Recover", record)
			}
		}
		if len(durable.records) != 1 {
			t.Fatalf("durable records = %d, want 1", len(durable.records))
		}
	})
}

type monitorStoreStub struct {
	mu      sync.Mutex
	records map[string]monitorpkg.Record
}

func (m *monitorStoreStub) SaveMonitor(_ context.Context, record monitorpkg.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.records[record.Monitor.ID]; !ok || existing.Version < record.Version {
		m.records[record.Monitor.ID] = record
	}
	return nil
}

func (m *monitorStoreStub) LoadMonitors(context.Context) ([]monitorpkg.Record, error) {
	return nil, nil
}

func (m *monitorStoreStub) PruneFinishedMonitors(context.Context, time.Time) error { return nil }
