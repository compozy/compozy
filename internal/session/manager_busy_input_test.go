package session

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	eventspkg "github.com/compozy/agh/internal/events"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/transcript"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestManagerBusyInputQueue(t *testing.T) {
	t.Run("Should queue busy user input and dispatch it after the active turn ends", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		h := newHarness(
			t,
			WithSessionInputQueueStore(queueStore),
			WithSessionBusyInputConfig(aghconfig.SessionBusyInputConfig{
				DefaultMode:  string(BusyInputModeQueue),
				QueueCap:     3,
				MaxTextBytes: 4096,
			}),
		)
		registerManagerInputQueueWorkspace(t, queueStore, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, queueStore, h, sess)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		firstPromptEntered := make(chan struct{})
		releaseFirstPrompt := make(chan struct{})
		secondPromptEntered := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() {
				close(releaseFirstPrompt)
			})
		})
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				switch req.Message {
				case "first prompt":
					close(firstPromptEntered)
					<-releaseFirstPrompt
				case "queued prompt":
					close(secondPromptEntered)
				}
				emitDonePromptEvents(events, sess.ID, req.TurnID)
			}()
			return events, nil
		}

		firstEvents, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "first prompt",
		})
		if err != nil {
			t.Fatalf("SendPrompt(first) error = %v", err)
		}
		if firstEvents.Events == nil {
			t.Fatal("SendPrompt(first).Events = nil, want accepted stream")
		}
		<-firstPromptEntered

		queued, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "queued prompt",
			Mode:    BusyInputModeQueue,
		})
		if err != nil {
			t.Fatalf("SendPrompt(queue) error = %v", err)
		}
		if !queued.Queued || queued.Status != "queued" || queued.QueueEntryID == "" || queued.QueueGeneration != 0 {
			t.Fatalf("queued result = %#v, want queued generation 0", queued)
		}
		if got := len(managerPromptCalls(h)); got != 1 {
			t.Fatalf("len(promptCalls) while first prompt active = %d, want 1", got)
		}
		releaseOnce.Do(func() {
			close(releaseFirstPrompt)
		})
		_ = collectEvents(t, firstEvents.Events)
		waitForCondition(t, "queued prompt dispatch", func() bool {
			return len(managerPromptCalls(h)) == 2
		})
		<-secondPromptEntered
		promptCalls := managerPromptCalls(h)
		if got := promptCalls[1].Message; got != "queued prompt" {
			t.Fatalf("queued dispatch message = %q, want queued prompt", got)
		}
		if got := promptCalls[1].Meta.TurnSource; got != acp.PromptTurnSourceUser {
			t.Fatalf("queued dispatch turn source = %q, want user", got)
		}
	})
}

func TestManagerGoalCommandDispatchShouldPreserveIngressAndDraftAdmission(t *testing.T) {
	t.Parallel()

	t.Run("Should keep internal prompt text literal when command parsing is not allowed", func(t *testing.T) {
		t.Parallel()

		var handlerCalls int
		h := newHarness(t, WithGoalCommandHandler(GoalCommandHandlerFunc(func(
			context.Context,
			string,
			string,
			PromptCaller,
			GoalCommand,
		) (GoalDispatchDecision, error) {
			handlerCalls++
			return GoalDispatchDecision{}, errors.New("unexpected Goal handler call")
		})))
		sess := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		result, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "/goal status", ClientMessageID: "client-literal-goal",
		})
		if err != nil {
			t.Fatalf("SendPrompt(literal Goal) error = %v", err)
		}
		collectEvents(t, result.Events)
		if handlerCalls != 0 {
			t.Fatalf("Goal handler calls = %d, want 0", handlerCalls)
		}
		calls := managerPromptCalls(h)
		if len(calls) != 1 || calls[0].Message != "/goal status" {
			t.Fatalf("literal prompt calls = %#v", calls)
		}
		persistedInputs := managerUserPromptEvents(t, h, sess.ID)
		if got, want := len(persistedInputs), 1; got != want {
			t.Fatalf("persisted literal inputs = %d, want %d; events=%#v", got, want, persistedInputs)
		}
		if persistedInputs[0].Text != "/goal status" ||
			persistedInputs[0].ClientMessageIDValue() != "client-literal-goal" {
			t.Fatalf("persisted literal input = %#v", persistedInputs[0])
		}
	})

	t.Run("Should return a structured command result without invoking ACP", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t, WithGoalCommandHandler(GoalCommandHandlerFunc(func(
			_ context.Context,
			workspaceID string,
			sessionID string,
			caller PromptCaller,
			command GoalCommand,
		) (GoalDispatchDecision, error) {
			if workspaceID == "" || sessionID == "" || caller.ID != "operator" || command.Verb != "status" {
				t.Fatalf(
					"Goal handler input = workspace:%q session:%q caller:%#v command:%#v",
					workspaceID,
					sessionID,
					caller,
					command,
				)
			}
			return GoalDispatchDecision{
				Kind: GoalDispatchRespond,
				Result: &GoalCommandResult{Outcome: GoalOutcomeStatus, Snapshot: &GoalSnapshot{
					RunID: "run-1", Status: "active",
				}},
			}, nil
		})))
		sess := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		result, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "/goal status", AllowGoalCommands: true,
			ClientMessageID: "client-goal-status",
			Caller:          PromptCaller{Kind: "human", ID: "operator", Source: "http"},
		})
		if err != nil {
			t.Fatalf("SendPrompt(Goal status) error = %v", err)
		}
		if result.Events != nil || result.Goal == nil || result.Goal.Outcome != GoalOutcomeStatus {
			t.Fatalf("structured Goal result = %#v", result)
		}
		if calls := managerPromptCalls(h); len(calls) != 0 {
			t.Fatalf("ACP prompt calls = %d, want 0", len(calls))
		}
		persistedInputs := managerUserPromptEvents(t, h, sess.ID)
		if got, want := len(persistedInputs), 1; got != want {
			t.Fatalf("persisted Goal status inputs = %d, want %d; events=%#v", got, want, persistedInputs)
		}
		if persistedInputs[0].Text != "/goal status" ||
			persistedInputs[0].ClientMessageIDValue() != "client-goal-status" {
			t.Fatalf("persisted Goal status input = %#v", persistedInputs[0])
		}
	})

	t.Run("Should dispatch a durable clear after the session stops without invoking ACP", func(t *testing.T) {
		t.Parallel()

		var handlerCalls int
		var expectedWorkspaceID string
		var expectedSessionID string
		h := newHarness(t, WithGoalCommandHandler(GoalCommandHandlerFunc(func(
			_ context.Context,
			workspaceID string,
			sessionID string,
			caller PromptCaller,
			command GoalCommand,
		) (GoalDispatchDecision, error) {
			handlerCalls++
			if workspaceID != expectedWorkspaceID || sessionID != expectedSessionID || caller.ID != "operator" ||
				command.Verb != GoalCommandVerbClear {
				t.Fatalf(
					"Goal handler input = workspace:%q session:%q caller:%#v command:%#v",
					workspaceID,
					sessionID,
					caller,
					command,
				)
			}
			return GoalDispatchDecision{
				Kind:   GoalDispatchRespond,
				Result: &GoalCommandResult{Outcome: GoalOutcomeCleared},
			}, nil
		})))
		sess := createSession(t, h)
		expectedWorkspaceID = h.workspaceID
		expectedSessionID = sess.ID
		if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		result, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "/goal clear", AllowGoalCommands: true,
			ClientMessageID: "client-goal-clear",
			Caller:          PromptCaller{Kind: "human", ID: "operator", Source: "http"},
		})
		if err != nil {
			t.Fatalf("SendPrompt(Goal clear) error = %v", err)
		}
		if result.Events != nil || result.Goal == nil || result.Goal.Outcome != GoalOutcomeCleared {
			t.Fatalf("structured Goal result = %#v", result)
		}
		if handlerCalls != 1 {
			t.Fatalf("Goal handler calls = %d, want 1", handlerCalls)
		}
		if calls := managerPromptCalls(h); len(calls) != 0 {
			t.Fatalf("ACP prompt calls = %d, want 0", len(calls))
		}
		persistedInputs := managerUserPromptEvents(t, h, sess.ID)
		if got, want := len(persistedInputs), 1; got != want {
			t.Fatalf("persisted stopped-session Goal inputs = %d, want %d; events=%#v", got, want, persistedInputs)
		}
		if persistedInputs[0].Text != "/goal clear" ||
			persistedInputs[0].ClientMessageIDValue() != "client-goal-clear" {
			t.Fatalf("persisted stopped-session Goal input = %#v", persistedInputs[0])
		}
	})

	t.Run("Should stream an admitted draft and reject a busy draft without queueing", func(t *testing.T) {
		t.Parallel()

		handler := GoalCommandHandlerFunc(func(
			_ context.Context,
			_ string,
			_ string,
			_ PromptCaller,
			command GoalCommand,
		) (GoalDispatchDecision, error) {
			if command.Verb != "draft" {
				return GoalDispatchDecision{}, errors.New("unexpected Goal command")
			}
			return GoalDispatchDecision{
				Kind: GoalDispatchPrompt, RewrittenMessage: "Draft proposal: " + command.Objective,
				BypassGoalParse: true, BusyPolicy: goalBusyPolicyRejectIfBusy,
				BusyReason: GoalReasonDraftRequiresIdle,
			}, nil
		})
		h := newHarness(t, WithGoalCommandHandler(handler))
		sess := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})
		caller := PromptCaller{Kind: "human", ID: "operator", Source: "http"}

		admitted, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "/goal draft improve objective", AllowGoalCommands: true, Caller: caller,
			ClientMessageID: "client-goal-draft",
		})
		if err != nil {
			t.Fatalf("SendPrompt(idle draft) error = %v", err)
		}
		collectEvents(t, admitted.Events)
		calls := managerPromptCalls(h)
		if len(calls) != 1 || calls[0].Message != "Draft proposal: improve objective" {
			t.Fatalf("admitted draft prompt calls = %#v", calls)
		}
		persistedInputs := managerUserPromptEvents(t, h, sess.ID)
		if got, want := len(persistedInputs), 1; got != want {
			t.Fatalf("persisted Goal draft inputs = %d, want %d; events=%#v", got, want, persistedInputs)
		}
		if persistedInputs[0].Text != "/goal draft improve objective" ||
			persistedInputs[0].ClientMessageIDValue() != "client-goal-draft" {
			t.Fatalf("persisted Goal draft input = %#v", persistedInputs[0])
		}

		entered := make(chan struct{})
		release := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				close(entered)
				<-release
				emitDonePromptEvents(events, sess.ID, req.TurnID)
			}()
			return events, nil
		}
		active, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{Message: "active prompt"})
		if err != nil {
			t.Fatalf("SendPrompt(active) error = %v", err)
		}
		<-entered
		busy, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "/goal draft another", Mode: BusyInputModeQueue,
			AllowGoalCommands: true, Caller: caller,
		})
		if err != nil {
			t.Fatalf("SendPrompt(busy draft) error = %v", err)
		}
		if busy.Goal == nil || busy.Goal.ReasonCode == nil ||
			*busy.Goal.ReasonCode != GoalReasonDraftRequiresIdle || busy.Queued || busy.Staged || busy.Events != nil {
			t.Fatalf("busy draft result = %#v", busy)
		}
		releaseOnce.Do(func() { close(release) })
		collectEvents(t, active.Events)
	})

	t.Run("Should reject a draft when an ordinary prompt wins the slot after parsing", func(t *testing.T) {
		t.Parallel()

		draftAtPreSubmit := make(chan struct{})
		releaseDraft := make(chan struct{})
		activeEntered := make(chan struct{})
		releaseActive := make(chan struct{})
		var releaseDraftOnce sync.Once
		var releaseActiveOnce sync.Once
		t.Cleanup(func() {
			releaseDraftOnce.Do(func() { close(releaseDraft) })
			releaseActiveOnce.Do(func() { close(releaseActive) })
		})

		dispatcher := &spyHookDispatcher{}
		dispatcher.dispatchInputPreSubmitFn = func(
			ctx context.Context,
			payload hookspkg.InputPreSubmitPayload,
		) (hookspkg.InputPreSubmitPayload, error) {
			if payload.Message != "Draft proposal: race objective" {
				return payload, nil
			}
			close(draftAtPreSubmit)
			select {
			case <-releaseDraft:
				return payload, nil
			case <-ctx.Done():
				return payload, ctx.Err()
			}
		}
		handler := GoalCommandHandlerFunc(func(
			_ context.Context,
			_ string,
			_ string,
			_ PromptCaller,
			command GoalCommand,
		) (GoalDispatchDecision, error) {
			if command.Verb != "draft" {
				return GoalDispatchDecision{}, errors.New("unexpected Goal command")
			}
			return GoalDispatchDecision{
				Kind: GoalDispatchPrompt, RewrittenMessage: "Draft proposal: " + command.Objective,
				BypassGoalParse: true, BusyPolicy: goalBusyPolicyRejectIfBusy,
				BusyReason: GoalReasonDraftRequiresIdle,
			}, nil
		})
		h := newHarness(
			t,
			WithGoalCommandHandler(handler),
			WithHookSet(fullHookSet(dispatcher)),
		)
		sess := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				close(activeEntered)
				<-releaseActive
				emitDonePromptEvents(events, sess.ID, req.TurnID)
			}()
			return events, nil
		}

		type draftResult struct {
			result SendPromptResult
			err    error
		}
		draftResultC := make(chan draftResult, 1)
		go func() {
			result, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
				Message: "/goal draft race objective", AllowGoalCommands: true,
				Caller: PromptCaller{Kind: "human", ID: "operator", Source: "http"},
			})
			draftResultC <- draftResult{result: result, err: err}
		}()

		select {
		case <-draftAtPreSubmit:
		case <-time.After(time.Second):
			t.Fatal("draft did not reach pre-submit after parsing")
		}
		active, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{Message: "active winner"})
		if err != nil {
			t.Fatalf("SendPrompt(active winner) error = %v", err)
		}
		select {
		case <-activeEntered:
		case <-time.After(time.Second):
			t.Fatal("ordinary prompt did not acquire the prompt slot")
		}
		releaseDraftOnce.Do(func() { close(releaseDraft) })

		var draft draftResult
		select {
		case draft = <-draftResultC:
		case <-time.After(time.Second):
			t.Fatal("draft admission did not resolve after losing the slot")
		}
		if draft.err != nil {
			t.Fatalf("SendPrompt(racing draft) error = %v", draft.err)
		}
		if draft.result.Goal == nil || draft.result.Goal.ReasonCode == nil ||
			*draft.result.Goal.ReasonCode != GoalReasonDraftRequiresIdle || draft.result.Events != nil ||
			draft.result.Queued || draft.result.Staged {
			t.Fatalf("racing draft result = %#v", draft.result)
		}
		if calls := managerPromptCalls(h); len(calls) != 1 || calls[0].Message != "active winner" {
			t.Fatalf("prompt calls after draft race = %#v, want only active winner", calls)
		}
		releaseActiveOnce.Do(func() { close(releaseActive) })
		collectEvents(t, active.Events)
	})
}

func TestManagerBusyInputManagedLifecycle(t *testing.T) {
	t.Run("Should persist exact Goal prompt metadata for every managed prompt kind", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name       string
			queueKind  string
			publicKind string
			turn       *int
		}{
			{
				name:       "Should persist Goal work metadata",
				queueKind:  "work",
				publicKind: "goal-work",
				turn:       new(1),
			},
			{
				name:       "Should persist Goal continuation metadata",
				queueKind:  "continuation",
				publicKind: "goal-continuation",
				turn:       new(2),
			},
			{name: "Should persist Goal compaction metadata", queueKind: "compact", publicKind: "goal-compaction"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				h := newHarness(t)
				sess := createSession(t, h)
				t.Cleanup(func() {
					if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
						t.Errorf("Stop() error = %v", err)
					}
				})
				lifecycle := newRecordingManagedInputLifecycle()
				lifecycle.promptMeta = func(claim ManagedInputClaim) ManagedInputPromptMeta {
					return ManagedInputPromptMeta{
						LoopRunID: claim.Owner.LoopRunID, NodeID: "converge", PromptID: claim.Owner.PromptID,
						Kind: claim.Owner.PromptKind, Generation: claim.Owner.RunGeneration, ItemIndex: 3,
						PromptAttempt: claim.Owner.PromptAttempt, Turn: tc.turn,
					}
				}
				h.manager.SetManagedInputLifecycle(lifecycle)
				h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
					events := make(chan acp.AgentEvent)
					go func() {
						defer close(events)
						emitDonePromptEvents(events, sess.ID, req.TurnID)
					}()
					return events, nil
				}

				entry := managedInputQueueEntry(sess.ID, "goalq-meta-"+tc.queueKind)
				entry.PromptKind = tc.queueKind
				entry.PromptAttempt = 4
				h.manager.startManagedInputPrompt(sess, entry)
				lifecycle.waitTerminal(t)

				calls := managerPromptCalls(h)
				if len(calls) != 1 || calls[0].Meta.Synthetic == nil || calls[0].Meta.Synthetic.Goal == nil {
					t.Fatalf("managed driver prompt metadata = %#v", calls)
				}
				assertGoalPromptMeta(t, calls[0].Meta.Synthetic.Goal, tc.publicKind, entry, tc.turn)

				stored, err := h.manager.Events(testutil.Context(t), sess.ID, store.EventQuery{})
				if err != nil {
					t.Fatalf("Events() error = %v", err)
				}
				matched := 0
				for _, storedEvent := range stored {
					if storedEvent.TurnID != entry.PromptID {
						continue
					}
					event, unmarshalErr := transcript.UnmarshalAgentEvent(storedEvent.Content)
					if unmarshalErr != nil {
						t.Fatalf("UnmarshalAgentEvent() error = %v", unmarshalErr)
					}
					assertGoalPromptMeta(t, event.Goal, tc.publicKind, entry, tc.turn)
					matched++
				}
				if matched < 3 {
					t.Fatalf("persisted Goal-tagged events = %d, want input, output, and terminal", matched)
				}

				entries, err := transcript.ToUIEntries(stored)
				if err != nil {
					t.Fatalf("ToUIEntries() error = %v", err)
				}
				var transcriptMeta struct {
					Goal *acp.GoalPromptMeta `json:"goal"`
				}
				for _, transcriptEntry := range entries {
					if transcriptEntry.Message.Role != transcript.UIRoleSystem ||
						len(transcriptEntry.Message.Metadata) == 0 {
						continue
					}
					if err := json.Unmarshal(transcriptEntry.Message.Metadata, &transcriptMeta); err != nil {
						t.Fatalf("unmarshal transcript metadata error = %v", err)
					}
					if transcriptMeta.Goal != nil {
						break
					}
				}
				assertGoalPromptMeta(t, transcriptMeta.Goal, tc.publicKind, entry, tc.turn)
			})
		}
	})

	t.Run("Should persist one exact managed terminal after claiming before driver effects", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})
		lifecycle := newRecordingManagedInputLifecycle()
		h.manager.SetManagedInputLifecycle(lifecycle)
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			lifecycle.record("driver")
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				emitDonePromptEvents(events, sess.ID, req.TurnID)
			}()
			return events, nil
		}

		runGeneration := int64(1)
		controlEpoch := int64(1)
		bindingEpoch := int64(1)
		entry := store.SessionInputQueueEntry{
			ID: "goalq-managed", SessionID: sess.ID, Status: store.SessionInputQueueStatusQueued,
			Mode: store.SessionInputQueueModeQueue, Text: "managed objective", TaskRunID: "taskrun-goal",
			RunGeneration: &runGeneration, LoopRunID: "looprun-goal", OwnerKind: managedInputOwnerGoal,
			OwnerEpoch: &controlEpoch, BindingEpoch: &bindingEpoch, PromptID: "goal-prompt-1",
			PromptKind: "work", PromptAttempt: 0,
		}

		h.manager.startManagedInputPrompt(sess, entry)
		terminal := lifecycle.waitTerminal(t)

		got := lifecycle.callsSnapshot()
		want := []string{"begin", "driver", "attached", "terminal"}
		if !slices.Equal(got, want) {
			t.Fatalf("managed lifecycle calls = %#v, want %#v", got, want)
		}
		if terminal.Owner.QueueEntryID != entry.ID || terminal.DriverTurnID != entry.PromptID ||
			terminal.Outcome != managedInputOutcomeCompleted || terminal.StopReason != "end_turn" ||
			terminal.EventStartSeq < 1 || terminal.EventEndSeq < terminal.EventStartSeq || terminal.Text != "reply" {
			t.Fatalf("managed terminal = %#v", terminal)
		}
		if got, want := lifecycle.usageSnapshot(), []int64{3}; !slices.Equal(got, want) {
			t.Fatalf("managed usage = %#v, want %#v", got, want)
		}
		calls := managerPromptCalls(h)
		if len(calls) != 1 || calls[0].TurnID != entry.PromptID ||
			calls[0].Meta.TurnSource != acp.PromptTurnSourceSynthetic {
			t.Fatalf("managed driver calls = %#v", calls)
		}
	})

	t.Run("Should accept unrelated session events between exact managed prompt frames", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})
		ctx := testutil.Context(t)
		promptID := "goal-prompt-interleaved"
		events := []acp.AgentEvent{
			{Type: acp.EventTypeSyntheticReentry, TurnID: promptID, Text: "managed objective"},
			{Type: acp.EventTypeAgentMessage, TurnID: "goal-snapshot:session-1", Text: "unrelated"},
			{Type: acp.EventTypeAgentMessage, TurnID: promptID, Text: "reply"},
			{
				Type:             acp.EventTypeDone,
				TurnID:           promptID,
				StopReason:       string(acp.PromptStopReasonEndTurn),
				PromptStopReason: acp.PromptStopReasonEndTurn,
			},
			{Type: eventspkg.TranscriptMarkerCreated, TurnID: promptID, Text: "terminal metadata"},
		}
		for _, event := range events {
			if err := h.manager.recordEvent(ctx, sess, event); err != nil {
				t.Fatalf("recordEvent(%s) error = %v", event.TurnID, err)
			}
		}

		terminal, err := h.manager.managedInputTerminal(ctx, sess, ManagedInputSubmission{
			Owner:         ManagedInputOwner{PromptID: promptID},
			DispatchToken: "dispatch-interleaved",
		})
		if err != nil {
			t.Fatalf("managedInputTerminal() error = %v", err)
		}
		if terminal.EventEndSeq != terminal.EventStartSeq+3 || terminal.Text != "reply" ||
			terminal.Outcome != managedInputOutcomeCompleted || terminal.StopReason != acp.PromptStopReasonEndTurn {
			t.Fatalf("managed terminal = %#v", terminal)
		}
	})

	t.Run("Should reconcile revocation before canceling only the exact managed lease", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		lifecycle := newRecordingManagedInputLifecycle()
		h.manager.SetManagedInputLifecycle(lifecycle)
		owner := ManagedInputOwner{
			QueueEntryID: "goalq-revoke", SessionID: "session-revoke", OwnerKind: managedInputOwnerGoal,
			LoopRunID: "looprun-revoke", TaskRunID: "taskrun-revoke", RunGeneration: 1,
			ControlEpoch: 2, BindingEpoch: 3, PromptID: "goal-prompt-revoke", PromptKind: "work",
		}
		leaseCtx, cancelLease := context.WithCancel(context.Background())
		t.Cleanup(cancelLease)
		if err := h.manager.registerManagedInputLease(owner, cancelLease); err != nil {
			t.Fatalf("registerManagedInputLease() error = %v", err)
		}

		h.manager.RevokeManagedInput(owner, "operator_stop")
		select {
		case <-leaseCtx.Done():
		case <-time.After(time.Second):
			t.Fatal("managed input lease was not canceled")
		}
		if got, want := lifecycle.callsSnapshot(), []string{"revoke"}; !slices.Equal(got, want) {
			t.Fatalf("managed lifecycle calls = %#v, want %#v", got, want)
		}
		waitCtx, cancelWait := context.WithTimeout(testutil.Context(t), 10*time.Millisecond)
		defer cancelWait()
		if err := h.manager.WaitManagedInputLease(waitCtx, owner); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("WaitManagedInputLease(before drain) error = %v, want deadline", err)
		}
		h.manager.releaseManagedInputLease(owner)
		if err := h.manager.WaitManagedInputLease(testutil.Context(t), owner); err != nil {
			t.Fatalf("WaitManagedInputLease(after drain) error = %v", err)
		}
		if h.manager.CancelManagedInputLease(owner) {
			t.Fatal("CancelManagedInputLease(after drain) = true, want released lease")
		}
	})

	t.Run("Should mark invalid cumulative usage ambiguous without persisting a terminal", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name  string
			usage func() *acp.TokenUsage
		}{
			{
				name: "Should reject negative total usage",
				usage: func() *acp.TokenUsage {
					total := int64(-1)
					return &acp.TokenUsage{TotalTokens: &total}
				},
			},
			{
				name: "Should reject overflowing component usage",
				usage: func() *acp.TokenUsage {
					input := int64(math.MaxInt64)
					output := int64(1)
					return &acp.TokenUsage{InputTokens: &input, OutputTokens: &output}
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				h := newHarness(t)
				sess := createSession(t, h)
				t.Cleanup(func() {
					if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
						t.Errorf("Stop() error = %v", err)
					}
				})
				lifecycle := newRecordingManagedInputLifecycle()
				h.manager.SetManagedInputLifecycle(lifecycle)
				h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
					lifecycle.record("driver")
					events := make(chan acp.AgentEvent)
					go func() {
						defer close(events)
						emitDonePromptEventsWithUsage(events, sess.ID, req.TurnID, tc.usage())
					}()
					return events, nil
				}

				entry := managedInputQueueEntry(sess.ID, "goalq-invalid-usage")
				h.manager.startManagedInputPrompt(sess, entry)
				receipt := lifecycle.waitAmbiguous(t)

				if receipt.Owner.QueueEntryID != entry.ID || receipt.ReasonCode != managedInputReasonRecoveryAmbiguous {
					t.Fatalf("ambiguous receipt = %#v", receipt)
				}
				if got, want := lifecycle.callsSnapshot(), []string{
					"begin",
					"driver",
					"attached",
					"ambiguous",
				}; !slices.Equal(
					got,
					want,
				) {
					t.Fatalf("managed lifecycle calls = %#v, want %#v", got, want)
				}
				if got := lifecycle.usageSnapshot(); len(got) != 0 {
					t.Fatalf("managed usage = %#v, want no invalid reports", got)
				}
				select {
				case terminal := <-lifecycle.terminal:
					t.Fatalf("managed terminal = %#v, want none", terminal)
				default:
				}
			})
		}
	})

	t.Run("Should mark a terminal persistence failure ambiguous before releasing the lease", func(t *testing.T) {
		t.Parallel()

		h := newHarness(t)
		sess := createSession(t, h)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})
		lifecycle := newRecordingManagedInputLifecycle()
		lifecycle.terminalErr = errors.New("terminal persistence failed")
		h.manager.SetManagedInputLifecycle(lifecycle)
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			lifecycle.record("driver")
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				emitDonePromptEvents(events, sess.ID, req.TurnID)
			}()
			return events, nil
		}

		entry := managedInputQueueEntry(sess.ID, "goalq-terminal-failure")
		h.manager.startManagedInputPrompt(sess, entry)
		terminal := lifecycle.waitTerminal(t)
		receipt := lifecycle.waitAmbiguous(t)

		if terminal.Owner.QueueEntryID != entry.ID || receipt.Owner.QueueEntryID != entry.ID ||
			receipt.ReasonCode != managedInputReasonRecoveryAmbiguous {
			t.Fatalf("terminal = %#v; ambiguous receipt = %#v", terminal, receipt)
		}
		if got, want := lifecycle.callsSnapshot(), []string{
			"begin",
			"driver",
			"attached",
			"terminal",
			"ambiguous",
		}; !slices.Equal(
			got,
			want,
		) {
			t.Fatalf("managed lifecycle calls = %#v, want %#v", got, want)
		}
	})
}

func TestManagerBusyInputInterrupt(t *testing.T) {
	t.Run("Should advance generation cancel stale queue and send replacement prompt", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		h := newHarness(
			t,
			WithSessionInputQueueStore(queueStore),
			WithSessionBusyInputConfig(aghconfig.SessionBusyInputConfig{
				DefaultMode:  string(BusyInputModeQueue),
				QueueCap:     3,
				MaxTextBytes: 4096,
			}),
		)
		registerManagerInputQueueWorkspace(t, queueStore, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, queueStore, h, sess)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
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
		h.driver.cancelHook = func(*fakeProcess) error {
			releaseOnce.Do(func() {
				close(releaseFirstPrompt)
			})
			return nil
		}
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				if req.Message == "first prompt" {
					close(firstPromptEntered)
					<-releaseFirstPrompt
				}
				emitDonePromptEvents(events, sess.ID, req.TurnID)
			}()
			return events, nil
		}

		firstEvents, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "first prompt",
		})
		if err != nil {
			t.Fatalf("SendPrompt(first) error = %v", err)
		}
		<-firstPromptEntered
		queued, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "stale queued prompt",
			Mode:    BusyInputModeQueue,
		})
		if err != nil {
			t.Fatalf("SendPrompt(stale queue) error = %v", err)
		}
		if !queued.Queued {
			t.Fatalf("queued result = %#v, want queued", queued)
		}

		interrupted, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "replacement prompt",
			Mode:    BusyInputModeInterrupt,
		})
		if err != nil {
			t.Fatalf("SendPrompt(interrupt) error = %v", err)
		}
		if !interrupted.Interrupted || interrupted.QueueGeneration != 1 || interrupted.CanceledQueuedEntries != 1 {
			t.Fatalf("interrupted result = %#v, want generation 1 with one canceled queue entry", interrupted)
		}
		if interrupted.Events == nil {
			t.Fatal("SendPrompt(interrupt).Events = nil, want replacement stream")
		}
		_ = collectEvents(t, firstEvents.Events)
		_ = collectEvents(t, interrupted.Events)
		promptCalls := managerPromptCalls(h)
		messages := make([]string, 0, len(promptCalls))
		for _, call := range promptCalls {
			messages = append(messages, call.Message)
		}
		if !slices.Equal(messages, []string{"first prompt", "replacement prompt"}) {
			t.Fatalf("prompt messages = %#v, want first then replacement without stale queue", messages)
		}
	})

	t.Run(
		"Should compose one generation-fenced salvage prompt after explicit interrupt then steer",
		func(t *testing.T) {
			t.Parallel()

			queueStore := openManagerInputQueueStore(t)
			h := newHarness(t, WithSessionInputQueueStore(queueStore))
			registerManagerInputQueueWorkspace(t, queueStore, h)
			sess := createSession(t, h)
			registerManagerInputQueueSession(t, queueStore, h, sess)
			t.Cleanup(func() {
				if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
					t.Errorf("Stop() error = %v", err)
				}
			})

			entered := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
			h.driver.cancelHook = func(*fakeProcess) error {
				releaseOnce.Do(func() { close(release) })
				return nil
			}
			h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				events := make(chan acp.AgentEvent)
				go func() {
					defer close(events)
					if req.Message == "Implement checkout retry fencing" {
						close(entered)
						<-release
					}
					emitDonePromptEvents(events, sess.ID, req.TurnID)
				}()
				return events, nil
			}

			first, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
				Message: "Implement checkout retry fencing",
			})
			if err != nil {
				t.Fatalf("SendPrompt(first) error = %v", err)
			}
			<-entered
			interrupted, err := h.manager.InterruptPrompt(testutil.Context(t), sess.ID)
			if err != nil {
				t.Fatalf("InterruptPrompt() error = %v", err)
			}
			if interrupted.QueueGeneration != 1 || !interrupted.Interrupted {
				t.Fatalf("InterruptPrompt() result = %#v", interrupted)
			}
			salvaged, err := h.manager.SteerPrompt(testutil.Context(t), sess.ID, "Preserve the retry budget")
			if err != nil {
				t.Fatalf("SteerPrompt() error = %v", err)
			}
			if salvaged.Mode != BusyInputModeSteer || salvaged.Status != promptStatusAccepted ||
				salvaged.QueueGeneration != interrupted.QueueGeneration || salvaged.Events == nil {
				t.Fatalf("SteerPrompt() result = %#v", salvaged)
			}
			collectEvents(t, first.Events)
			collectEvents(t, salvaged.Events)

			wantSalvage := composeInterruptedPromptSalvage(
				"Implement checkout retry fencing",
				"Preserve the retry budget",
			)
			calls := managerPromptCalls(h)
			if len(calls) != 2 || calls[1].Message != wantSalvage {
				t.Fatalf("prompt calls = %#v, want one composed salvage", calls)
			}
			inputs := managerUserPromptEvents(t, h, sess.ID)
			if len(inputs) != 2 || inputs[1].Text != wantSalvage {
				t.Fatalf("persisted user inputs = %#v, want one composed salvage", inputs)
			}
			_, err = h.manager.SteerPrompt(
				testutil.Context(t),
				sess.ID,
				"duplicate correction",
			)
			if !errors.Is(err, ErrPromptNotInProgress) {
				t.Fatalf("SteerPrompt(duplicate) error = %v, want ErrPromptNotInProgress", err)
			}
		},
	)

	t.Run("Should discard salvage when ordinary replacement input follows explicit interrupt", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(queueStore))
		registerManagerInputQueueWorkspace(t, queueStore, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, queueStore, h, sess)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		entered := make(chan struct{})
		release := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
		h.driver.cancelHook = func(*fakeProcess) error {
			releaseOnce.Do(func() { close(release) })
			return nil
		}
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				if req.Message == "Original interrupted task" {
					close(entered)
					<-release
				}
				emitDonePromptEvents(events, sess.ID, req.TurnID)
			}()
			return events, nil
		}

		first, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "Original interrupted task",
		})
		if err != nil {
			t.Fatalf("SendPrompt(first) error = %v", err)
		}
		<-entered
		if _, err := h.manager.InterruptPrompt(testutil.Context(t), sess.ID); err != nil {
			t.Fatalf("InterruptPrompt() error = %v", err)
		}
		collectEvents(t, first.Events)
		replacement, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "Plain replacement task",
		})
		if err != nil {
			t.Fatalf("SendPrompt(replacement) error = %v", err)
		}
		collectEvents(t, replacement.Events)
		calls := managerPromptCalls(h)
		if len(calls) != 2 || calls[1].Message != "Plain replacement task" {
			t.Fatalf("prompt calls = %#v, want unsalvaged replacement", calls)
		}
		_, err = h.manager.SteerPrompt(
			testutil.Context(t),
			sess.ID,
			"late correction",
		)
		if !errors.Is(err, ErrPromptNotInProgress) {
			t.Fatalf("SteerPrompt(after replacement) error = %v, want ErrPromptNotInProgress", err)
		}
	})
}

func managerPromptCalls(h *harness) []acp.PromptRequest {
	h.driver.mu.Lock()
	defer h.driver.mu.Unlock()
	return append([]acp.PromptRequest(nil), h.driver.promptCalls...)
}

func emitDonePromptEvents(events chan<- acp.AgentEvent, sessionID string, turnID string) {
	totalTokens := int64(3)
	emitDonePromptEventsWithUsage(
		events,
		sessionID,
		turnID,
		&acp.TokenUsage{TurnID: turnID, TotalTokens: &totalTokens},
	)
}

func emitDonePromptEventsWithUsage(
	events chan<- acp.AgentEvent,
	sessionID string,
	turnID string,
	usage *acp.TokenUsage,
) {
	ts := time.Now().UTC()
	events <- acp.AgentEvent{
		Type:      acp.EventTypeAgentMessage,
		SessionID: sessionID,
		TurnID:    turnID,
		Timestamp: ts,
		Text:      "reply",
	}
	events <- acp.AgentEvent{
		Type:             acp.EventTypeDone,
		SessionID:        sessionID,
		TurnID:           turnID,
		Timestamp:        ts,
		StopReason:       string(acp.PromptStopReasonEndTurn),
		PromptStopReason: acp.PromptStopReasonEndTurn,
		Usage:            usage,
	}
}

func managerUserPromptEvents(t *testing.T, h *harness, sessionID string) []acp.AgentEvent {
	t.Helper()
	stored, err := h.manager.Events(testutil.Context(t), sessionID, store.EventQuery{})
	if err != nil {
		t.Fatalf("Events(%s) error = %v", sessionID, err)
	}
	events := make([]acp.AgentEvent, 0, len(stored))
	for _, storedEvent := range stored {
		if storedEvent.Type != acp.EventTypeUserMessage {
			continue
		}
		event, unmarshalErr := transcript.UnmarshalAgentEvent(storedEvent.Content)
		if unmarshalErr != nil {
			t.Fatalf("UnmarshalAgentEvent(%s) error = %v", storedEvent.ID, unmarshalErr)
		}
		events = append(events, event)
	}
	return events
}

func managedInputQueueEntry(sessionID string, queueEntryID string) store.SessionInputQueueEntry {
	runGeneration := int64(1)
	controlEpoch := int64(1)
	bindingEpoch := int64(1)
	return store.SessionInputQueueEntry{
		ID: queueEntryID, SessionID: sessionID, Status: store.SessionInputQueueStatusQueued,
		Mode: store.SessionInputQueueModeQueue, Text: "managed objective", TaskRunID: "taskrun-goal",
		RunGeneration: &runGeneration, LoopRunID: "looprun-goal", OwnerKind: managedInputOwnerGoal,
		OwnerEpoch: &controlEpoch, BindingEpoch: &bindingEpoch, PromptID: "goal-prompt-1",
		PromptKind: "work", PromptAttempt: 0,
	}
}

func assertGoalPromptMeta(
	t *testing.T,
	meta *acp.GoalPromptMeta,
	wantKind string,
	entry store.SessionInputQueueEntry,
	wantTurn *int,
) {
	t.Helper()
	if meta == nil {
		t.Fatal("GoalPromptMeta = nil")
	}
	if meta.Kind != wantKind || meta.RunID != entry.LoopRunID || meta.NodeID != "converge" ||
		meta.Generation != *entry.RunGeneration || meta.ItemIndex != 3 || meta.PromptAttempt != entry.PromptAttempt ||
		meta.PromptID != entry.PromptID || !equalIntPointers(meta.Turn, wantTurn) {
		t.Fatalf("GoalPromptMeta = %#v", meta)
	}
}

func equalIntPointers(left *int, right *int) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func openManagerInputQueueStore(t *testing.T) *globaldb.GlobalDB {
	t.Helper()

	ctx := testutil.Context(t)
	queueStore, err := globaldb.OpenGlobalDB(ctx, filepath.Join(t.TempDir(), store.GlobalDatabaseName))
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := queueStore.Close(ctx); err != nil {
			t.Errorf("Close(globalDB) error = %v", err)
		}
	})
	return queueStore
}

func registerManagerInputQueueWorkspace(t *testing.T, queueStore *globaldb.GlobalDB, h *harness) {
	t.Helper()

	if err := queueStore.InsertWorkspace(testutil.Context(t), workspacepkg.Workspace{
		ID:        h.workspaceID,
		RootDir:   h.workspace,
		Name:      h.workspaceName,
		CreatedAt: time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("InsertWorkspace() error = %v", err)
	}
}

func registerManagerInputQueueSession(
	t *testing.T,
	queueStore *globaldb.GlobalDB,
	h *harness,
	sess *Session,
) {
	t.Helper()

	if err := queueStore.RegisterSession(testutil.Context(t), store.SessionInfo{
		ID:          sess.ID,
		Name:        "Input Queue",
		AgentName:   "coder",
		Provider:    "claude",
		WorkspaceID: h.workspaceID,
		SessionType: string(SessionTypeUser),
		State:       string(StateActive),
		CreatedAt:   time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
		UpdatedAt:   time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}
}

type recordingManagedInputLifecycle struct {
	mu          sync.Mutex
	calls       []string
	usage       []int64
	terminal    chan ManagedInputTerminal
	ambiguous   chan ManagedInputReceipt
	terminalErr error
	promptMeta  func(ManagedInputClaim) ManagedInputPromptMeta
}

func newRecordingManagedInputLifecycle() *recordingManagedInputLifecycle {
	return &recordingManagedInputLifecycle{
		terminal:  make(chan ManagedInputTerminal, 1),
		ambiguous: make(chan ManagedInputReceipt, 1),
	}
}

func (l *recordingManagedInputLifecycle) BeginSubmission(
	_ context.Context,
	claim ManagedInputClaim,
) (ManagedInputSubmission, error) {
	l.record("begin")
	turn := 1
	promptMeta := ManagedInputPromptMeta{
		LoopRunID: claim.Owner.LoopRunID, NodeID: "converge", PromptID: claim.Owner.PromptID,
		Kind: claim.Owner.PromptKind, Generation: claim.Owner.RunGeneration, Turn: &turn,
	}
	if l.promptMeta != nil {
		promptMeta = l.promptMeta(claim)
	}
	return ManagedInputSubmission{
		Owner:         claim.Owner,
		PromptMeta:    promptMeta,
		DispatchToken: "dispatch-token",
		BudgetVersion: 1,
		StartedAt:     time.Now().UTC(),
		UsageReporter: l,
	}, nil
}

func (l *recordingManagedInputLifecycle) ReportActionTokensUsed(tokens int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.usage = append(l.usage, tokens)
}

func (l *recordingManagedInputLifecycle) RecordDriverAttached(
	_ context.Context,
	_ ManagedInputReceipt,
) error {
	l.record("attached")
	return nil
}

func (l *recordingManagedInputLifecycle) RecordRejected(
	context.Context,
	ManagedInputRejection,
) error {
	l.record("rejected")
	return nil
}

func (l *recordingManagedInputLifecycle) RecordAmbiguous(
	_ context.Context,
	receipt ManagedInputReceipt,
) error {
	l.record("ambiguous")
	l.ambiguous <- receipt
	return nil
}

func (l *recordingManagedInputLifecycle) RecordTerminal(
	_ context.Context,
	terminal ManagedInputTerminal,
) error {
	l.record("terminal")
	l.terminal <- terminal
	return l.terminalErr
}

func (l *recordingManagedInputLifecycle) Revoke(context.Context, ManagedInputOwner, string) error {
	l.record("revoke")
	return nil
}

func (l *recordingManagedInputLifecycle) record(call string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, call)
}

func (l *recordingManagedInputLifecycle) callsSnapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string(nil), l.calls...)
}

func (l *recordingManagedInputLifecycle) usageSnapshot() []int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]int64(nil), l.usage...)
}

func (l *recordingManagedInputLifecycle) waitTerminal(t *testing.T) ManagedInputTerminal {
	t.Helper()
	select {
	case terminal := <-l.terminal:
		return terminal
	case <-time.After(5 * time.Second):
		t.Fatal("managed input terminal was not recorded")
		return ManagedInputTerminal{}
	}
}

func (l *recordingManagedInputLifecycle) waitAmbiguous(t *testing.T) ManagedInputReceipt {
	t.Helper()
	select {
	case receipt := <-l.ambiguous:
		return receipt
	case <-time.After(5 * time.Second):
		t.Fatal("managed input ambiguity was not recorded")
		return ManagedInputReceipt{}
	}
}
