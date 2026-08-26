package session

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestPromptSyntheticPersistsDedicatedEventAndMetadata(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session, err := h.manager.Create(testutil.Context(t), CreateOpts{
		AgentName: "coder",
		Name:      "system-session",
		Workspace: h.workspaceID,
		Type:      SessionTypeSystem,
	})
	if err != nil {
		t.Fatalf("Create(system) error = %v", err)
	}
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	eventsCh, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "synthetic wake-up",
		Metadata: acp.PromptSyntheticMeta{
			TaskID:               "task-1",
			TaskRunID:            "run-1",
			ClaimTokenHash:       "sha256:claim-1",
			CoordinatorSessionID: "sess-coordinator-1",
			Reason:               "task_run_completed",
			Summary:              "background work finished",
		},
	})
	if err != nil {
		t.Fatalf("PromptSynthetic() error = %v", err)
	}
	_ = collectEvents(t, eventsCh)

	if got := len(h.driver.promptCalls); got != 1 {
		t.Fatalf("len(promptCalls) = %d, want 1", got)
	}
	if got := h.driver.promptCalls[0].Meta.TurnSource; got != acp.PromptTurnSourceSynthetic {
		t.Fatalf("promptCalls[0].Meta.TurnSource = %q, want %q", got, acp.PromptTurnSourceSynthetic)
	}
	if h.driver.promptCalls[0].Meta.Synthetic == nil {
		t.Fatal("promptCalls[0].Meta.Synthetic = nil, want metadata")
	}
	if got, want := h.driver.promptCalls[0].Meta.Synthetic.TaskRunID, "run-1"; got != want {
		t.Fatalf("promptCalls[0].Meta.Synthetic.TaskRunID = %q, want %q", got, want)
	}
	if got, want := h.driver.promptCalls[0].Meta.Synthetic.ClaimTokenHash, "sha256:claim-1"; got != want {
		t.Fatalf("promptCalls[0].Meta.Synthetic.ClaimTokenHash = %q, want %q", got, want)
	}

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(stored) == 0 {
		t.Fatal("stored events = 0, want persisted synthetic event")
	}
	if got := stored[0].Type; got != acp.EventTypeSyntheticReentry {
		t.Fatalf("stored[0].Type = %q, want %q", got, acp.EventTypeSyntheticReentry)
	}
	if got := countEventType(stored, acp.EventTypeUserMessage); got != 0 {
		t.Fatalf("countEventType(user_message) = %d, want 0 for synthetic-only prompt", got)
	}

	payload := decodeStoredEventPayload(t, stored[0])
	if got, want := payload["type"], acp.EventTypeSyntheticReentry; got != want {
		t.Fatalf("stored synthetic payload type = %v, want %q", got, want)
	}
	if got, want := payload["text"], "synthetic wake-up"; got != want {
		t.Fatalf("stored synthetic payload text = %v, want %q", got, want)
	}
	syntheticPayload, ok := payload["synthetic"].(map[string]any)
	if !ok {
		t.Fatalf("stored synthetic payload metadata = %#v, want object", payload["synthetic"])
	}
	if got, want := syntheticPayload["task_run_id"], "run-1"; got != want {
		t.Fatalf("stored synthetic task_run_id = %v, want %q", got, want)
	}
	if got, want := syntheticPayload["reason"], "task_run_completed"; got != want {
		t.Fatalf("stored synthetic reason = %v, want %q", got, want)
	}
	if got, want := syntheticPayload["summary"], "background work finished"; got != want {
		t.Fatalf("stored synthetic summary = %v, want %q", got, want)
	}
	notified := h.notifier.eventsForSession(session.ID)
	if len(notified) == 0 {
		t.Fatal("notified events = 0, want synthetic observe notification")
	}
	if got, want := notified[0].SchedulerReason, "task_run_completed"; got != want {
		t.Fatalf("notified[0].SchedulerReason = %q, want %q", got, want)
	}
	if got, want := notified[0].ClaimTokenHash, "sha256:claim-1"; got != want {
		t.Fatalf("notified[0].ClaimTokenHash = %q, want %q", got, want)
	}
	if got, want := notified[0].CoordinatorSessionID, "sess-coordinator-1"; got != want {
		t.Fatalf("notified[0].CoordinatorSessionID = %q, want %q", got, want)
	}
}

func TestSendPromptDurablyAdmitsSyntheticDeliveryMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve synthetic metadata through durable busy-input dispatch", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(queueStore))
		registerManagerInputQueueWorkspace(t, queueStore, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, queueStore, h, sess)
		activeEntered := make(chan struct{})
		releaseActive := make(chan struct{})
		syntheticDispatched := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() { close(releaseActive) })
			reportSessionStop(t, h, sess.ID)
		})
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				if req.Message == "active prompt" {
					close(activeEntered)
					<-releaseActive
				} else {
					close(syntheticDispatched)
				}
				emitDonePromptEvents(events, sess.ID, req.TurnID)
			}()
			return events, nil
		}
		active, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{Message: "active prompt"})
		if err != nil {
			t.Fatalf("SendPrompt(active) error = %v", err)
		}
		<-activeEntered
		opts := SendPromptOpts{
			Message:   "Call completed: reviewer (call_1) → completed.",
			MessageID: "msg_wake_1", IdempotencyKey: "call:wake_1", Mode: BusyInputModeQueue,
			Synthetic: &acp.PromptSyntheticMeta{
				CallID: "call_1", CallState: "completed", ResultRef: "sha256:result",
				DeliveryKind: "completion", Reason: "call_completion", WakeEventID: "wake_1",
			},
		}

		queued, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, opts)
		if err != nil {
			t.Fatalf("SendPrompt(synthetic) error = %v", err)
		}
		if queued.Status != "queued" || queued.Events != nil {
			t.Fatalf("SendPrompt(synthetic) = %#v, want durable queue admission", queued)
		}
		replayed, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, opts)
		if err != nil {
			t.Fatalf("SendPrompt(synthetic replay) error = %v", err)
		}
		if !replayed.Replayed || len(h.driver.promptCalls) != 1 {
			t.Fatalf("SendPrompt(synthetic replay) = %#v calls=%d, want stable replay", replayed, len(h.driver.promptCalls))
		}

		releaseOnce.Do(func() { close(releaseActive) })
		collectEvents(t, active.Events)
		select {
		case <-syntheticDispatched:
		case <-time.After(5 * time.Second):
			t.Fatal("queued synthetic prompt was not dispatched")
		}
		if got := len(h.driver.promptCalls); got != 2 {
			t.Fatalf("len(promptCalls) = %d, want active plus synthetic dispatch", got)
		}
		meta := h.driver.promptCalls[1].Meta.Normalize()
		if meta.TurnSource != acp.PromptTurnSourceSynthetic || meta.Synthetic == nil ||
			meta.Synthetic.CallID != "call_1" || meta.Synthetic.WakeEventID != "wake_1" ||
			meta.Synthetic.ResultRef != "sha256:result" {
			t.Fatalf("durable queued synthetic prompt metadata = %#v", meta)
		}
	})
}

func TestPromptSyntheticRejectsMissingWakeupMetadata(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	_, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "synthetic wake-up",
		Metadata: acp.PromptSyntheticMeta{
			TaskRunID: "run-1",
		},
	})
	if err == nil {
		t.Fatal("PromptSynthetic() error = nil, want validation failure")
	}
	if !strings.Contains(err.Error(), "requires a reason") {
		t.Fatalf("PromptSynthetic() error = %v, want missing-reason validation", err)
	}
	if got := len(h.driver.promptCalls); got != 0 {
		t.Fatalf("len(promptCalls) = %d, want 0 after validation failure", got)
	}
}

func TestPromptSyntheticHeartbeatWakeOptions(t *testing.T) {
	t.Parallel()

	t.Run("Should reject busy synthetic prompt without queueing when requested", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		firstPromptEntered := make(chan struct{})
		releaseFirstPrompt := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() {
				close(releaseFirstPrompt)
			})
		})
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			if req.TurnID == "turn-1" {
				close(firstPromptEntered)
				events := make(chan acp.AgentEvent, 1)
				go func() {
					<-releaseFirstPrompt
					events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
					close(events)
				}()
				return events, nil
			}
			return completedSyntheticPromptEvents(req.TurnID), nil
		}

		userEvents, err := h.manager.Prompt(testutil.Context(t), session.ID, "user prompt")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		<-firstPromptEntered

		_, err = h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
			Message: "heartbeat wake",
			Metadata: acp.PromptSyntheticMeta{
				Reason:      "agent_heartbeat_wake",
				WakeEventID: "hwe-busy",
			},
			SkipIfBusy: true,
		})
		if !errors.Is(err, ErrPromptInProgress) {
			t.Fatalf("PromptSynthetic(skip busy) error = %v, want ErrPromptInProgress", err)
		}
		if got, want := len(h.driver.promptCalls), 1; got != want {
			t.Fatalf("len(promptCalls) = %d, want %d", got, want)
		}

		releaseOnce.Do(func() {
			close(releaseFirstPrompt)
		})
		_ = collectEvents(t, userEvents)
	})

	t.Run("Should preserve heartbeat metadata and supplied prompt turn id", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		eventsCh, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
			Message: "heartbeat wake",
			TurnID:  "turn-heartbeat",
			Metadata: acp.PromptSyntheticMeta{
				Reason:           "agent_heartbeat_wake",
				Summary:          "policy summary",
				WakeEventID:      "hwe-heartbeat",
				PolicySnapshotID: "hb-heartbeat",
				PolicyDigest:     "sha256:policy",
				ConfigDigest:     "sha256:config",
			},
			SkipIfBusy: true,
		})
		if err != nil {
			t.Fatalf("PromptSynthetic(heartbeat) error = %v", err)
		}
		_ = collectEvents(t, eventsCh)

		if got, want := h.driver.promptCalls[0].TurnID, "turn-heartbeat"; got != want {
			t.Fatalf("synthetic turn id = %q, want %q", got, want)
		}
		if got, want := h.driver.promptCalls[0].Meta.Synthetic.WakeEventID, "hwe-heartbeat"; got != want {
			t.Fatalf("synthetic wake_event_id = %q, want %q", got, want)
		}

		stored := readStoredEvents(t, session)
		payload := decodeStoredEventPayload(t, stored[0])
		syntheticPayload, ok := payload["synthetic"].(map[string]any)
		if !ok {
			t.Fatalf("stored synthetic metadata = %#v, want object", payload["synthetic"])
		}
		if got, want := syntheticPayload["wake_event_id"], "hwe-heartbeat"; got != want {
			t.Fatalf("stored wake_event_id = %v, want %q", got, want)
		}
		if got, want := syntheticPayload["policy_snapshot_id"], "hb-heartbeat"; got != want {
			t.Fatalf("stored policy_snapshot_id = %v, want %q", got, want)
		}
		if got, want := syntheticPayload["policy_digest"], "sha256:policy"; got != want {
			t.Fatalf("stored policy_digest = %v, want %q", got, want)
		}
	})
}

func TestPromptSyntheticQueuesBehindActiveTurnAndPreservesStoredOrder(t *testing.T) {
	t.Parallel()

	h := newHarness(t)
	session := createSession(t, h)
	t.Cleanup(func() {
		reportSessionStop(t, h, session.ID)
	})

	firstPromptEntered := make(chan struct{})
	releaseFirstPrompt := make(chan struct{})
	h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		if req.TurnID == "turn-1" {
			close(firstPromptEntered)
			events := make(chan acp.AgentEvent, 1)
			go func() {
				<-releaseFirstPrompt
				events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
				close(events)
			}()
			return events, nil
		}
		return completedSyntheticPromptEvents(req.TurnID), nil
	}

	userEventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "user prompt")
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	<-firstPromptEntered

	syntheticEventsCh, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
		Message: "synthetic prompt",
		Metadata: acp.PromptSyntheticMeta{
			TaskRunID: "run-2",
			Reason:    "task_run_completed",
			Summary:   "queued behind user turn",
		},
	})
	if err != nil {
		t.Fatalf("PromptSynthetic() error = %v", err)
	}

	if got := len(h.driver.promptCalls); got != 1 {
		t.Fatalf("len(promptCalls) before releasing active turn = %d, want 1", got)
	}

	close(releaseFirstPrompt)
	_ = collectEvents(t, userEventsCh)
	_ = collectEvents(t, syntheticEventsCh)

	if got := len(h.driver.promptCalls); got != 2 {
		t.Fatalf("len(promptCalls) after draining synthetic queue = %d, want 2", got)
	}
	if got := h.driver.promptCalls[1].Meta.TurnSource; got != acp.PromptTurnSourceSynthetic {
		t.Fatalf("promptCalls[1].Meta.TurnSource = %q, want %q", got, acp.PromptTurnSourceSynthetic)
	}

	stored, err := session.recorderHandle().Query(testutil.Context(t), store.EventQuery{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(stored) < 3 {
		t.Fatalf("stored events = %d, want user, terminal, and synthetic input events", len(stored))
	}
	if got := stored[0].Type; got != acp.EventTypeUserMessage {
		t.Fatalf("stored[0].Type = %q, want %q", got, acp.EventTypeUserMessage)
	}
	if got := stored[1].Type; got != acp.EventTypeDone {
		t.Fatalf("stored[1].Type = %q, want %q", got, acp.EventTypeDone)
	}
	if got := stored[2].Type; got != acp.EventTypeSyntheticReentry {
		t.Fatalf("stored[2].Type = %q, want %q", got, acp.EventTypeSyntheticReentry)
	}
}

func TestPromptSyntheticInterruptsAgentWaitingTurnWhenRequested(t *testing.T) {
	t.Run("Should interrupt agent waiting when synthetic prompt requests it", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("failed to stop session %s: %v", session.ID, err)
			}
		})

		firstPromptEntered := make(chan struct{})
		releaseFirstPrompt := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() {
				close(releaseFirstPrompt)
			})
		})
		h.driver.cancelHook = func(*fakeProcess) error {
			releaseOnce.Do(func() {
				close(releaseFirstPrompt)
			})
			return nil
		}
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			if req.Message == "user prompt" {
				req.ActivityReporter(acp.PromptActivityReport{
					Kind:   runtimeActivityKindAgentWaiting,
					Detail: "waiting for detached work",
				})
				close(firstPromptEntered)
				events := make(chan acp.AgentEvent, 1)
				go func() {
					<-releaseFirstPrompt
					events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
					close(events)
				}()
				return events, nil
			}
			return completedSyntheticPromptEvents(req.TurnID), nil
		}

		userEventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "user prompt")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		<-firstPromptEntered
		info := session.Info()
		if info.Liveness == nil || info.Liveness.Activity == nil ||
			info.Liveness.Activity.LastActivityKind != runtimeActivityKindAgentWaiting {
			t.Fatalf("session activity = %#v, want %q", info.Liveness, runtimeActivityKindAgentWaiting)
		}

		syntheticEventsCh, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
			Message: "detached completion",
			Metadata: acp.PromptSyntheticMeta{
				TaskRunID: "run-detached",
				Reason:    "task_run_completed",
				Summary:   "detached work completed",
			},
			InterruptIfAgentWaiting: true,
		})
		if err != nil {
			t.Fatalf("PromptSynthetic(interrupt waiting) error = %v", err)
		}
		_ = collectEvents(t, userEventsCh)
		_ = collectEvents(t, syntheticEventsCh)

		promptCalls := managerPromptCalls(h)
		if got, want := len(promptCalls), 2; got != want {
			t.Fatalf("len(promptCalls) = %d, want %d", got, want)
		}
		if got, want := promptCalls[1].Message, "detached completion"; got != want {
			t.Fatalf("synthetic prompt message = %q, want %q", got, want)
		}
		if got := promptCalls[1].Meta.TurnSource; got != acp.PromptTurnSourceSynthetic {
			t.Fatalf("synthetic turn source = %q, want %q", got, acp.PromptTurnSourceSynthetic)
		}
		if got := h.driver.cancelCalls; got != 1 {
			t.Fatalf("cancel calls = %d, want 1", got)
		}
	})

	t.Run("Should bound cancel when interrupting agent waiting for synthetic prompt", func(t *testing.T) {
		t.Parallel()

		supervision := compozyconfig.DefaultSessionSupervisionConfig()
		supervision.TimeoutCancelGrace = 20 * time.Millisecond
		h := newHarness(t, WithSessionSupervision(supervision))
		session := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
				t.Fatalf("failed to stop session %s: %v", session.ID, err)
			}
		})

		firstPromptEntered := make(chan struct{})
		releaseFirstPrompt := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() {
				close(releaseFirstPrompt)
			})
		})
		h.driver.cancelWithContextHook = func(ctx context.Context, _ *fakeProcess) error {
			timer := time.NewTimer(5 * supervision.TimeoutCancelGrace)
			defer timer.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				return errors.New("cancel prompt did not receive bounded context")
			}
		}
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			if req.Message == "user prompt" {
				req.ActivityReporter(acp.PromptActivityReport{
					Kind:   runtimeActivityKindAgentWaiting,
					Detail: "waiting for detached work",
				})
				close(firstPromptEntered)
				events := make(chan acp.AgentEvent, 1)
				go func() {
					<-releaseFirstPrompt
					events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: req.TurnID}
					close(events)
				}()
				return events, nil
			}
			return completedSyntheticPromptEvents(req.TurnID), nil
		}

		userEventsCh, err := h.manager.Prompt(testutil.Context(t), session.ID, "user prompt")
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}
		<-firstPromptEntered
		info := session.Info()
		if info.Liveness == nil || info.Liveness.Activity == nil ||
			info.Liveness.Activity.LastActivityKind != runtimeActivityKindAgentWaiting {
			t.Fatalf("session activity = %#v, want %q", info.Liveness, runtimeActivityKindAgentWaiting)
		}

		syntheticEventsCh, err := h.manager.PromptSynthetic(testutil.Context(t), session.ID, SyntheticPromptOpts{
			Message: "detached completion",
			Metadata: acp.PromptSyntheticMeta{
				TaskRunID: "run-detached",
				Reason:    "task_run_completed",
				Summary:   "detached work completed",
			},
			InterruptIfAgentWaiting: true,
		})
		if err != nil {
			t.Fatalf("PromptSynthetic(interrupt waiting) error = %v", err)
		}
		releaseOnce.Do(func() {
			close(releaseFirstPrompt)
		})
		_ = collectEvents(t, userEventsCh)
		_ = collectEvents(t, syntheticEventsCh)
	})
}

func completedSyntheticPromptEvents(turnID string) <-chan acp.AgentEvent {
	events := make(chan acp.AgentEvent, 1)
	events <- acp.AgentEvent{Type: acp.EventTypeDone, TurnID: turnID}
	close(events)
	return events
}
