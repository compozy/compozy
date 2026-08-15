package session

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	attachmentspkg "github.com/compozy/compozy/internal/attachments"
	commandpkg "github.com/compozy/compozy/internal/command"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/session/inputqueue"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func TestManagerBusyInputQueue(t *testing.T) {
	t.Run("Should surface queue entry entropy failure before persistence", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			owner     string
			operation string
			mode      string
			call      func(context.Context, *inputqueue.Service, string, store.SessionPromptAdmissionRequest) error
		}{
			{
				name:      "Should reject Enqueue before persistence",
				owner:     "Enqueue",
				operation: store.SessionPromptOperationPrompt,
				mode:      store.SessionInputQueueModeQueue,
				call: func(ctx context.Context, queue *inputqueue.Service, sessionID string, _ store.SessionPromptAdmissionRequest) error {
					_, _, err := queue.Enqueue(ctx, sessionID, "queued prompt", 0, store.SessionInputRuntime{}, nil, nil)
					return err
				},
			},
			{
				name:      "Should reject StageSteer before persistence",
				owner:     "StageSteer",
				operation: store.SessionPromptOperationSteer,
				mode:      store.SessionInputQueueModeSteer,
				call: func(ctx context.Context, queue *inputqueue.Service, sessionID string, _ store.SessionPromptAdmissionRequest) error {
					_, err := queue.StageSteer(
						ctx,
						sessionID,
						"steering prompt",
						"turn-active",
						0,
						store.SessionInputRuntime{},
						nil,
						nil,
					)
					return err
				},
			},
			{
				name:      "Should reject EnqueueAdmitted before receipt persistence",
				owner:     "EnqueueAdmitted",
				operation: store.SessionPromptOperationPrompt,
				mode:      store.SessionInputQueueModeQueue,
				call: func(ctx context.Context, queue *inputqueue.Service, _ string, admission store.SessionPromptAdmissionRequest) error {
					_, _, _, _, err := queue.EnqueueAdmitted(ctx, admission, 0)
					return err
				},
			},
			{
				name:      "Should reject StageAdmittedSteer before receipt persistence",
				owner:     "StageAdmittedSteer",
				operation: store.SessionPromptOperationSteer,
				mode:      store.SessionInputQueueModeSteer,
				call: func(ctx context.Context, queue *inputqueue.Service, _ string, admission store.SessionPromptAdmissionRequest) error {
					_, _, _, err := queue.StageAdmittedSteer(ctx, admission, "turn-active", 0)
					return err
				},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				queueStore := openManagerInputQueueStore(t)
				h := newHarness(
					t,
					WithSessionInputQueueStore(queueStore),
					WithSessionBusyInputConfig(compozyconfig.SessionBusyInputConfig{
						DefaultMode:  string(BusyInputModeQueue),
						QueueCap:     3,
						MaxTextBytes: 4096,
					}),
				)
				registerManagerInputQueueWorkspace(t, queueStore, h)
				session := createSession(t, h)
				registerManagerInputQueueSession(t, queueStore, h, session)
				t.Cleanup(func() {
					if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
						t.Errorf("Stop() error = %v", err)
					}
				})

				entropyErr := errors.New("entropy unavailable")
				queue, err := inputqueue.New(
					queueStore,
					inputqueue.Config{QueueCap: 3, MaxTextBytes: 4096},
					inputqueue.WithIDGenerator(func() (string, error) {
						return "", entropyErr
					}),
				)
				if err != nil {
					t.Fatalf("inputqueue.New() error = %v", err)
				}
				admission := store.SessionPromptAdmissionRequest{
					ID:                 "admission-entropy",
					WorkspaceID:        h.workspaceID,
					SessionID:          session.ID,
					MessageID:          "message-entropy",
					IdempotencyKey:     "idempotency-entropy",
					Operation:          test.operation,
					FingerprintVersion: "v1",
					RequestFingerprint: "fingerprint-entropy",
					Mode:               test.mode,
					AuthoredText:       "admitted prompt",
					TurnID:             "turn-entropy",
					EventID:            "event-entropy",
					Now:                time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC),
				}

				if err := test.call(testutil.Context(t), queue, session.ID, admission); !errors.Is(err, entropyErr) {
					t.Fatalf("%s() error = %v, want entropy failure", test.owner, err)
				}
				assertManagerInputQueueEntropyFailurePersistedNothing(t, queueStore, session.ID)
			})
		}
	})

	t.Run("Should reject explicit busy modes for stopped sessions without resuming or persisting", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name string
			mode BusyInputMode
		}{
			{name: "Should reject queue mode", mode: BusyInputModeQueue},
			{name: "Should reject interrupt mode", mode: BusyInputModeInterrupt},
			{name: "Should reject steer mode", mode: BusyInputModeSteer},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				queueStore := openManagerInputQueueStore(t)
				h := newHarness(t, WithSessionInputQueueStore(queueStore))
				registerManagerInputQueueWorkspace(t, queueStore, h)
				sess := createSession(t, h)
				registerManagerInputQueueSession(t, queueStore, h, sess)
				if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
					t.Fatalf("Stop() error = %v", err)
				}

				_, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
					Message:        "must remain stopped",
					MessageID:      "message-stopped-" + string(test.mode),
					IdempotencyKey: "idempotency-stopped-" + string(test.mode),
					Mode:           test.mode,
					ExpectedTurnID: "turn-before-stop",
				})
				if !errors.Is(err, ErrSessionNotActive) {
					t.Fatalf("SendPrompt(%s) error = %v, want ErrSessionNotActive", test.mode, err)
				}
				if _, active := h.manager.Get(sess.ID); active {
					t.Fatalf("Get(%q) active = true after rejected %s", sess.ID, test.mode)
				}
				h.driver.mu.Lock()
				startCalls := len(h.driver.startCalls)
				h.driver.mu.Unlock()
				if startCalls != 1 {
					t.Fatalf("driver Start() calls = %d, want only initial create", startCalls)
				}

				var admissions int
				if err := queueStore.DB().QueryRowContext(
					testutil.Context(t),
					"SELECT COUNT(*) FROM session_prompt_admissions WHERE session_id = ?",
					sess.ID,
				).Scan(&admissions); err != nil {
					t.Fatalf("count session prompt admissions error = %v", err)
				}
				if admissions != 0 {
					t.Fatalf("session prompt admissions = %d, want 0", admissions)
				}
			})
		}
	})

	t.Run("Should queue busy user input and dispatch it after the active turn ends", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		queuedAttachmentData := []byte("queued attachment")
		opener := &promptAttachmentOpenerStub{data: map[string][]byte{
			"att-queued": queuedAttachmentData,
		}}
		h := newHarness(
			t,
			WithSessionInputQueueStore(queueStore),
			WithAttachmentOpener(opener),
			WithSessionBusyInputConfig(compozyconfig.SessionBusyInputConfig{
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
			Attachments: []AttachmentMeta{promptAttachmentMeta(
				"att-queued", "queued.txt", "text/plain", queuedAttachmentData,
			)},
		})
		if err != nil {
			t.Fatalf("SendPrompt(queue) error = %v", err)
		}
		if queued.Status != "queued" || queued.Delivery != store.SessionInputDeliveryAfterTurn ||
			queued.QueueEntryID == "" || queued.QueueGeneration != 0 {
			t.Fatalf("queued result = %#v, want queued generation 0", queued)
		}
		if got := len(managerPromptCalls(h)); got != 1 {
			t.Fatalf("len(promptCalls) while first prompt active = %d, want 1", got)
		}
		releaseOnce.Do(func() {
			close(releaseFirstPrompt)
		})
		collectEvents(t, firstEvents.Events)
		<-secondPromptEntered
		promptCalls := managerPromptCalls(h)
		if got := len(promptCalls); got != 2 {
			t.Fatalf("len(promptCalls) after queued prompt entered = %d, want 2", got)
		}
		if got := promptCalls[1].Message; got != "queued prompt" {
			t.Fatalf("queued dispatch message = %q, want queued prompt", got)
		}
		if got := promptCalls[1].Meta.TurnSource; got != acp.PromptTurnSourceUser {
			t.Fatalf("queued dispatch turn source = %q, want user", got)
		}
		if len(promptCalls[1].Attachments) != 1 ||
			!bytes.Equal(promptCalls[1].Attachments[0].Data, queuedAttachmentData) {
			t.Fatalf("queued dispatch attachments = %#v, want resolved attachment", promptCalls[1].Attachments)
		}
	})

	t.Run("Should preserve admitted skill invocations through busy modes until dispatch", func(t *testing.T) {
		t.Parallel()

		modes := []struct {
			name string
			mode BusyInputMode
		}{
			{name: "queue", mode: BusyInputModeQueue},
			{name: "steer", mode: BusyInputModeSteer},
			{name: "interrupt", mode: BusyInputModeInterrupt},
		}
		for _, test := range modes {
			t.Run("Should preserve "+test.name+" skill invocation", func(t *testing.T) {
				t.Parallel()

				queueStore := openManagerInputQueueStore(t)
				catalog, err := commandpkg.BuildCatalog(
					commandpkg.DefaultBuiltins(),
					nil,
					[]commandpkg.SkillSpec{{
						Name: "review", Description: "Review carefully", Available: true,
						Source: commandpkg.Source{Kind: "workspace", Scope: "workspace"},
					}},
				)
				if err != nil {
					t.Fatalf("BuildCatalog() error = %v", err)
				}
				service := &busyInputCommandService{
					catalog:  catalog,
					expanded: make(chan []commandpkg.Invocation, 1),
				}
				h := newHarness(
					t,
					WithSessionInputQueueStore(queueStore),
					WithSessionBusyInputConfig(compozyconfig.SessionBusyInputConfig{
						DefaultMode:  string(BusyInputModeQueue),
						QueueCap:     3,
						MaxTextBytes: 4096,
					}),
					WithCommandService(service),
				)
				registerManagerInputQueueWorkspace(t, queueStore, h)
				sess := createSession(t, h)
				registerManagerInputQueueSession(t, queueStore, h, sess)

				activeEntered := make(chan struct{})
				releaseActive := make(chan struct{})
				dispatched := make(chan struct{})
				var releaseActiveOnce sync.Once
				var dispatchedOnce sync.Once
				t.Cleanup(func() {
					releaseActiveOnce.Do(func() { close(releaseActive) })
					if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
						t.Errorf("Stop() error = %v", err)
					}
				})

				h.driver.cancelHook = func(*fakeProcess) error {
					releaseActiveOnce.Do(func() { close(releaseActive) })
					return nil
				}
				h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
					events := make(chan acp.AgentEvent, 2)
					go func() {
						defer close(events)
						switch {
						case req.Message == "active prompt":
							close(activeEntered)
							<-releaseActive
						case strings.HasSuffix(req.Message, "queue /review"),
							strings.HasSuffix(req.Message, "steer /review"),
							strings.HasSuffix(req.Message, "interrupt /review"):
							dispatchedOnce.Do(func() { close(dispatched) })
						}
						emitDonePromptEvents(events, sess.ID, req.TurnID)
					}()
					return events, nil
				}

				authored := test.name + " /review"
				wantInvocations, err := commandpkg.ParseSkillInvocations(authored, catalog)
				if err != nil {
					t.Fatalf("ParseSkillInvocations() error = %v", err)
				}
				if len(wantInvocations) != 1 {
					t.Fatalf("ParseSkillInvocations() = %#v, want one invocation", wantInvocations)
				}

				active, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
					Message: "active prompt",
				})
				if err != nil {
					t.Fatalf("SendPrompt(active) error = %v", err)
				}
				<-activeEntered

				caller := PromptCaller{Kind: "human", ID: "operator", Source: "http"}
				var result SendPromptResult
				switch test.mode {
				case BusyInputModeSteer:
					result, err = h.manager.SteerPrompt(testutil.Context(t), sess.ID, SteerPromptOpts{
						Message: authored, ExpectedTurnID: sess.CurrentTurnID(),
						AllowCommands: true, Caller: caller,
					})
				default:
					result, err = h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
						Message: authored, Mode: test.mode,
						ExpectedTurnID: sess.CurrentTurnID(),
						AllowCommands:  true, Caller: caller,
					})
				}
				if err != nil {
					t.Fatalf("busy %s submission error = %v", test.name, err)
				}
				if result.QueueEntryID == "" {
					t.Fatalf("busy %s result = %#v, want durable queue entry", test.name, result)
				}

				entry, err := h.manager.inputQueue.Get(testutil.Context(t), sess.ID, result.QueueEntryID)
				if err != nil {
					t.Fatalf("inputQueue.Get(%q) error = %v", result.QueueEntryID, err)
				}
				if !slices.Equal(entry.SkillInvocations, wantInvocations) {
					t.Fatalf(
						"%s queue skill invocations = %#v, want %#v",
						test.name,
						entry.SkillInvocations,
						wantInvocations,
					)
				}

				releaseActiveOnce.Do(func() { close(releaseActive) })
				collectEvents(t, active.Events)
				select {
				case got := <-service.expanded:
					if !slices.Equal(got, wantInvocations) {
						t.Fatalf("%s dispatched skill invocations = %#v, want %#v", test.name, got, wantInvocations)
					}
				case <-time.After(time.Second):
					t.Fatalf("%s skill invocation did not reach command expansion", test.name)
				}
				select {
				case <-dispatched:
				case <-time.After(time.Second):
					t.Fatalf("%s prompt did not dispatch", test.name)
				}
			})
		}
	})
}

type busyInputCommandService struct {
	catalog  commandpkg.Catalog
	expanded chan []commandpkg.Invocation
}

func (s *busyInputCommandService) Catalog(
	context.Context,
	*Info,
	compozyconfig.AgentDef,
) (commandpkg.Catalog, error) {
	return s.catalog, nil
}

func (s *busyInputCommandService) Expand(
	_ context.Context,
	_ *Info,
	_ compozyconfig.AgentDef,
	invocations []commandpkg.Invocation,
	message string,
) (string, error) {
	s.expanded <- append([]commandpkg.Invocation(nil), invocations...)
	return "EXPANDED\n\n" + message, nil
}

func TestManagerBusyInputExpectedTurnFence(t *testing.T) {
	testCases := []struct {
		name   string
		submit func(context.Context, *Manager, string) error
	}{
		{
			name: "Should reject steering without an active turn fence",
			submit: func(ctx context.Context, manager *Manager, sessionID string) error {
				_, err := manager.SteerPrompt(ctx, sessionID, SteerPromptOpts{
					Message: "unfenced steer", MessageID: "message-unfenced-steer",
					IdempotencyKey: "idem-unfenced-steer",
				})
				return err
			},
		},
		{
			name: "Should reject interrupt without an active turn fence",
			submit: func(ctx context.Context, manager *Manager, sessionID string) error {
				_, err := manager.SendPrompt(ctx, sessionID, SendPromptOpts{
					Message: "unfenced interrupt", MessageID: "message-unfenced-interrupt",
					IdempotencyKey: "idem-unfenced-interrupt", Mode: BusyInputModeInterrupt,
				})
				return err
			},
		},
		{
			name: "Should reject stale steering before persistence or cancellation",
			submit: func(ctx context.Context, manager *Manager, sessionID string) error {
				_, err := manager.SteerPrompt(ctx, sessionID, SteerPromptOpts{
					Message: "stale steer", MessageID: "message-stale-steer",
					IdempotencyKey: "idem-stale-steer", ExpectedTurnID: "turn-stale",
				})
				return err
			},
		},
		{
			name: "Should reject stale interrupt before persistence or cancellation",
			submit: func(ctx context.Context, manager *Manager, sessionID string) error {
				_, err := manager.SendPrompt(ctx, sessionID, SendPromptOpts{
					Message: "stale interrupt", MessageID: "message-stale-interrupt",
					IdempotencyKey: "idem-stale-interrupt", Mode: BusyInputModeInterrupt,
					ExpectedTurnID: "turn-stale",
				})
				return err
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queueStore := openManagerInputQueueStore(t)
			h := newHarness(t, WithSessionInputQueueStore(queueStore))
			registerManagerInputQueueWorkspace(t, queueStore, h)
			sess := createSession(t, h)
			registerManagerInputQueueSession(t, queueStore, h, sess)
			entered := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			t.Cleanup(func() {
				releaseOnce.Do(func() { close(release) })
				if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
					t.Errorf("Stop() error = %v", err)
				}
			})
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

			active, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
				Message: "active prompt",
			})
			if err != nil {
				t.Fatalf("SendPrompt(active) error = %v", err)
			}
			<-entered
			err = testCase.submit(testutil.Context(t), h.manager, sess.ID)
			if !errors.Is(err, ErrActiveTurnMismatch) {
				t.Fatalf("busy input error = %v, want ErrActiveTurnMismatch", err)
			}
			if got := managerCancelCalls(h); got != 0 {
				t.Fatalf("driver cancel calls = %d, want 0", got)
			}
			pending, listErr := h.manager.ListPendingInputs(testutil.Context(t), sess.ID)
			if listErr != nil {
				t.Fatalf("ListPendingInputs() error = %v", listErr)
			}
			if len(pending) != 0 || len(managerPromptCalls(h)) != 1 {
				t.Fatalf("pending=%#v prompt calls=%d, want no mutation", pending, len(managerPromptCalls(h)))
			}
			releaseOnce.Do(func() { close(release) })
			collectEvents(t, active.Events)
		})
	}
}

func TestManagerBusyInputActivationCleanup(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		admitted bool
		submit   func(context.Context, *Manager, *Session) error
	}{
		{
			name: "Should cancel staged steer input when active-turn cancellation fails",
			submit: func(ctx context.Context, manager *Manager, sess *Session) error {
				_, err := manager.SteerPrompt(ctx, sess.ID, SteerPromptOpts{
					Message: "steer after cancellation", ExpectedTurnID: sess.CurrentTurnID(),
				})
				return err
			},
		},
		{
			name:     "Should cancel and fence admitted steer when active-turn cancellation fails",
			admitted: true,
			submit: func(ctx context.Context, manager *Manager, sess *Session) error {
				_, err := manager.SteerPrompt(ctx, sess.ID, SteerPromptOpts{
					Message: "admitted steer after cancellation", MessageID: "message-admitted-steer-failure",
					IdempotencyKey: "idem-admitted-steer-failure", ExpectedTurnID: sess.CurrentTurnID(),
				})
				return err
			},
		},
		{
			name: "Should cancel interrupt input when active-turn cancellation fails",
			submit: func(ctx context.Context, manager *Manager, sess *Session) error {
				_, err := manager.SendPrompt(ctx, sess.ID, SendPromptOpts{
					Message: "replace after cancellation", Mode: BusyInputModeInterrupt,
					ExpectedTurnID: sess.CurrentTurnID(),
				})
				return err
			},
		},
		{
			name:     "Should cancel and fence admitted interrupt when active-turn cancellation fails",
			admitted: true,
			submit: func(ctx context.Context, manager *Manager, sess *Session) error {
				_, err := manager.SendPrompt(ctx, sess.ID, SendPromptOpts{
					Message: "admitted replacement after cancellation", Mode: BusyInputModeInterrupt,
					MessageID:      "message-admitted-interrupt-failure",
					IdempotencyKey: "idem-admitted-interrupt-failure", ExpectedTurnID: sess.CurrentTurnID(),
				})
				return err
			},
		},
		{
			name: "Should cancel promoted input when active-turn cancellation fails",
			submit: func(ctx context.Context, manager *Manager, sess *Session) error {
				queued, err := manager.SendPrompt(ctx, sess.ID, SendPromptOpts{
					Message: "queued before promotion", Mode: BusyInputModeQueue,
				})
				if err != nil {
					return err
				}
				_, err = manager.PromotePendingInputToSteer(ctx, sess.ID, queued.QueueEntryID, PromotePendingInputOpts{
					Text: "promoted after cancellation", ExpectedTurnID: sess.CurrentTurnID(),
				})
				return err
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			queueStore := openManagerInputQueueStore(t)
			h := newHarness(t, WithSessionInputQueueStore(queueStore))
			registerManagerInputQueueWorkspace(t, queueStore, h)
			sess := createSession(t, h)
			registerManagerInputQueueSession(t, queueStore, h, sess)
			activeEntered := make(chan struct{})
			releaseActive := make(chan struct{})
			var releaseOnce sync.Once
			t.Cleanup(func() {
				releaseOnce.Do(func() { close(releaseActive) })
				if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
					t.Errorf("Stop() error = %v", err)
				}
			})
			h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				events := make(chan acp.AgentEvent)
				go func() {
					defer close(events)
					if req.Message == "active prompt" {
						close(activeEntered)
						<-releaseActive
					}
					emitDonePromptEvents(events, sess.ID, req.TurnID)
				}()
				return events, nil
			}

			active, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
				Message: "active prompt",
			})
			if err != nil {
				t.Fatalf("SendPrompt(active) error = %v", err)
			}
			<-activeEntered
			cancelErr := errors.New("driver cancellation failed")
			h.driver.cancelHook = func(*fakeProcess) error { return cancelErr }

			err = testCase.submit(testutil.Context(t), h.manager, sess)
			if testCase.admitted {
				if !errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) {
					t.Fatalf("busy input activation error = %v, want dispatch indeterminate", err)
				}
			} else if !errors.Is(err, cancelErr) {
				t.Fatalf("busy input activation error = %v, want cancellation failure", err)
			}
			pending, listErr := h.manager.ListPendingInputs(testutil.Context(t), sess.ID)
			if listErr != nil {
				t.Fatalf("ListPendingInputs() error = %v", listErr)
			}
			if len(pending) != 0 {
				t.Fatalf("pending inputs after activation failure = %#v, want none", pending)
			}
			if testCase.admitted {
				retryErr := testCase.submit(testutil.Context(t), h.manager, sess)
				if !errors.Is(retryErr, store.ErrSessionPromptDispatchIndeterminate) {
					t.Fatalf("busy input retry error = %v, want dispatch indeterminate", retryErr)
				}
			}

			releaseOnce.Do(func() { close(releaseActive) })
			collectEvents(t, active.Events)
		})
	}

	t.Run("Should fence canceled promotion replay after active-turn cancellation fails", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(queueStore))
		registerManagerInputQueueWorkspace(t, queueStore, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, queueStore, h, sess)
		activeEntered := make(chan struct{})
		releaseActive := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() { close(releaseActive) })
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})
		h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			go func() {
				defer close(events)
				if req.Message == "active prompt" {
					close(activeEntered)
					<-releaseActive
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
		targetTurnID := sess.CurrentTurnID()
		queued, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "queued before promotion replay", Mode: BusyInputModeQueue,
		})
		if err != nil {
			t.Fatalf("SendPrompt(queue) error = %v", err)
		}
		cancelErr := errors.New("driver cancellation failed")
		h.driver.cancelHook = func(*fakeProcess) error { return cancelErr }
		opts := PromotePendingInputOpts{
			Text: "promoted after cancellation", ExpectedTurnID: targetTurnID,
			MessageID: "message-promote-canceled-replay", IdempotencyKey: "idem-promote-canceled-replay",
		}
		_, err = h.manager.PromotePendingInputToSteer(
			testutil.Context(t), sess.ID, queued.QueueEntryID, opts,
		)
		if !errors.Is(err, cancelErr) {
			t.Fatalf("PromotePendingInputToSteer(first) error = %v, want cancellation failure", err)
		}

		_, err = h.manager.PromotePendingInputToSteer(
			testutil.Context(t), sess.ID, queued.QueueEntryID, opts,
		)
		if !errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) {
			t.Fatalf("PromotePendingInputToSteer(replay) error = %v, want dispatch indeterminate", err)
		}
		pending, listErr := h.manager.ListPendingInputs(testutil.Context(t), sess.ID)
		if listErr != nil {
			t.Fatalf("ListPendingInputs() error = %v", listErr)
		}
		if len(pending) != 0 {
			t.Fatalf("pending inputs after promotion replay = %#v, want none", pending)
		}
		if got := managerCancelCalls(h); got != 1 {
			t.Fatalf("driver cancel calls = %d, want one", got)
		}

		releaseOnce.Do(func() { close(releaseActive) })
		collectEvents(t, active.Events)
	})
}

func TestManagerPromptAdmissionReplay(t *testing.T) {
	t.Parallel()

	t.Run("Should replay one direct prompt without repeated effects", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		inputPreSubmitCalls := 0
		turnStartCalls := 0
		dispatcher := &spyHookDispatcher{}
		dispatcher.dispatchInputPreSubmitFn = func(
			_ context.Context,
			payload hookspkg.InputPreSubmitPayload,
		) (hookspkg.InputPreSubmitPayload, error) {
			inputPreSubmitCalls++
			return payload, nil
		}
		dispatcher.dispatchTurnStartFn = func(
			_ context.Context,
			payload hookspkg.TurnStartPayload,
		) (hookspkg.TurnStartPayload, error) {
			turnStartCalls++
			return payload, nil
		}
		h := newHarness(
			t,
			WithSessionInputQueueStore(queueStore),
			WithHookSet(fullHookSet(dispatcher)),
		)
		registerManagerInputQueueWorkspace(t, queueStore, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, queueStore, h, sess)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		first, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "one durable prompt", MessageID: "message-direct-replay",
			IdempotencyKey: "idem-direct-replay",
		})
		if err != nil {
			t.Fatalf("SendPrompt(first) error = %v", err)
		}
		collectEvents(t, first.Events)

		replayed, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "one durable prompt", MessageID: "message-direct-replay",
			IdempotencyKey: "idem-direct-replay",
		})
		if err != nil {
			t.Fatalf("SendPrompt(replay) error = %v", err)
		}
		if !replayed.Replayed || replayed.Events != nil ||
			replayed.Status != store.SessionPromptResultStatusAccepted ||
			replayed.MessageID != first.MessageID || replayed.IdempotencyKey != first.IdempotencyKey ||
			replayed.NewTurnID != first.NewTurnID {
			t.Fatalf("SendPrompt(replay) = %#v, want stable non-streaming replay of %#v", replayed, first)
		}
		if got := len(managerPromptCalls(h)); got != 1 {
			t.Fatalf("len(promptCalls) = %d, want 1", got)
		}
		if inputPreSubmitCalls != 1 || turnStartCalls != 1 {
			t.Fatalf(
				"hook calls = input_pre_submit:%d turn_start:%d, want 1 each",
				inputPreSubmitCalls,
				turnStartCalls,
			)
		}

		matchingAuthoredEvents := 0
		for _, storedEvent := range readStoredEvents(t, sess) {
			if storedEvent.Type != acp.EventTypeUserMessage {
				continue
			}
			decoded, decodeErr := transcript.UnmarshalAgentEvent(storedEvent.Content)
			if decodeErr != nil {
				t.Fatalf("UnmarshalAgentEvent() error = %v", decodeErr)
			}
			if decoded.MessageIDValue() != first.MessageID {
				continue
			}
			matchingAuthoredEvents++
			if strings.TrimSpace(storedEvent.ID) == "" || decoded.TurnID != first.NewTurnID {
				t.Fatalf("stored authored event = %#v / %#v, want stable ids", storedEvent, decoded)
			}
		}
		if matchingAuthoredEvents != 1 {
			t.Fatalf("matching authored events = %d, want 1", matchingAuthoredEvents)
		}

		_, err = h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "changed payload", MessageID: "message-direct-replay",
			IdempotencyKey: "idem-direct-replay",
		})
		if !errors.Is(err, store.ErrSessionPromptIdempotencyConflict) {
			t.Fatalf("SendPrompt(changed payload) error = %v, want idempotency conflict", err)
		}
		_, err = h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "one durable prompt", MessageID: "message-direct-replay",
			IdempotencyKey: "idem-direct-replay-other",
		})
		if !errors.Is(err, store.ErrSessionPromptMessageConflict) {
			t.Fatalf("SendPrompt(reused message id) error = %v, want message identity conflict", err)
		}
	})

	for _, test := range []struct {
		name                 string
		makeModelUnavailable func(t *testing.T, h *harness, session *Session)
	}{
		{
			name: "Should replay a completed active Cursor prompt after its model becomes unavailable",
			makeModelUnavailable: func(t *testing.T, _ *harness, session *Session) {
				t.Helper()
				session.mu.Lock()
				defer session.mu.Unlock()
				session.ACPCaps = acp.Caps{ConfigOptions: []acp.SessionConfigOption{{
					ID:       "model",
					Category: "model",
					Kind:     acp.SessionConfigOptionKindSelect,
					Current:  "other-model",
					Values:   []acp.SessionConfigOptionValue{{Value: "other-model"}},
				}}}
			},
		},
		{
			name: "Should replay a completed stopped Cursor prompt after its model becomes unavailable",
			makeModelUnavailable: func(t *testing.T, h *harness, session *Session) {
				t.Helper()
				if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
					t.Fatalf("Stop() error = %v", err)
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			queueStore := openManagerInputQueueStore(t)
			h := newHarness(
				t,
				WithSessionInputQueueStore(queueStore),
				WithModelCatalog(cursorModelCatalogStub{}),
			)
			registerManagerInputQueueWorkspace(t, queueStore, h)
			session := createCursorPromptAdmissionSession(t, h)
			registerManagerInputQueueSession(t, queueStore, h, session)
			t.Cleanup(func() {
				if _, active := h.manager.Get(session.ID); !active {
					return
				}
				if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
					t.Errorf("Stop() error = %v", err)
				}
			})

			opts := SendPromptOpts{
				Message: "one durable Cursor prompt", MessageID: "message-cursor-replay",
				IdempotencyKey: "idem-cursor-replay",
				Runtime: &RuntimeSelection{
					Provider: "cursor", Model: cursorPromptAdmissionModel,
				},
			}
			first, err := h.manager.SendPrompt(testutil.Context(t), session.ID, opts)
			if err != nil {
				t.Fatalf("SendPrompt(first) error = %v", err)
			}
			collectEvents(t, first.Events)

			test.makeModelUnavailable(t, h, session)

			replayed, err := h.manager.SendPrompt(testutil.Context(t), session.ID, opts)
			if err != nil {
				t.Fatalf("SendPrompt(replay) error = %v", err)
			}
			if !replayed.Replayed || replayed.Events != nil ||
				replayed.Status != first.Status || replayed.MessageID != first.MessageID ||
				replayed.IdempotencyKey != first.IdempotencyKey || replayed.NewTurnID != first.NewTurnID {
				t.Fatalf("SendPrompt(replay) = %#v, want stable non-streaming replay of %#v", replayed, first)
			}
			if got := len(managerPromptCalls(h)); got != 1 {
				t.Fatalf("len(promptCalls) = %d, want 1", got)
			}
			h.driver.mu.Lock()
			startCalls := len(h.driver.startCalls)
			h.driver.mu.Unlock()
			if startCalls != 1 {
				t.Fatalf("driver Start() calls = %d, want only initial create", startCalls)
			}
		})
	}

	t.Run("Should reject a Cursor alias before a retryable direct dispatch commit", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(queueStore))
		registerManagerInputQueueWorkspace(t, queueStore, h)
		sess := createCursorPromptAdmissionSession(t, h)
		registerManagerInputQueueSession(t, queueStore, h, sess)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		_, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "use Cursor Grok", MessageID: "cursor-alias-direct",
			IdempotencyKey: "cursor-alias-direct",
			Runtime: &RuntimeSelection{
				Provider: "cursor", Model: "cursor-grok-4.5-high",
			},
		})
		assertCursorAliasRejectedBeforeDispatch(t, err)
		assertManagerPromptAdmissionCount(t, queueStore, sess.ID, 0)
		if calls := managerPromptCalls(h); len(calls) != 0 {
			t.Fatalf("ACP prompt calls after alias rejection = %d, want 0", len(calls))
		}

		accepted, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "use Cursor Grok", MessageID: "cursor-alias-direct",
			IdempotencyKey: "cursor-alias-direct",
			Runtime: &RuntimeSelection{
				Provider: "cursor", Model: cursorPromptAdmissionModel,
			},
		})
		if err != nil {
			t.Fatalf("SendPrompt(retry with advertised Cursor model) error = %v", err)
		}
		collectEvents(t, accepted.Events)
		assertManagerPromptAdmissionCount(t, queueStore, sess.ID, 1)
		if calls := managerPromptCalls(h); len(calls) != 1 {
			t.Fatalf("ACP prompt calls after advertised retry = %d, want 1", len(calls))
		}
	})
}

func TestManagerBusyInputPromptAdmissionReplay(t *testing.T) {
	t.Parallel()

	t.Run("Should reject admitted interrupt when the durable input queue is unavailable", func(t *testing.T) {
		t.Parallel()

		admissionStore := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionPromptAdmissionStore(admissionStore))
		registerManagerInputQueueWorkspace(t, admissionStore, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, admissionStore, h, sess)
		activeEntered := make(chan struct{})
		releaseActive := make(chan struct{})
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() { close(releaseActive) })
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

		active, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{Message: "active prompt"})
		if err != nil {
			t.Fatalf("SendPrompt(active) error = %v", err)
		}
		<-activeEntered
		_, err = h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "admitted replacement", MessageID: "message-admitted-interrupt",
			IdempotencyKey: "idem-admitted-interrupt", Mode: BusyInputModeInterrupt,
			ExpectedTurnID: sess.CurrentTurnID(),
		})
		if !errors.Is(err, ErrPromptInProgress) {
			t.Fatalf("SendPrompt(admitted interrupt) error = %v, want ErrPromptInProgress", err)
		}

		releaseOnce.Do(func() { close(releaseActive) })
		collectEvents(t, active.Events)
	})

	cases := []struct {
		name   string
		mode   BusyInputMode
		submit func(context.Context, *Manager, string, string, string) (SendPromptResult, error)
	}{
		{
			name: "Should dispatch one identity-bearing queued prompt after an exact retry",
			mode: BusyInputModeQueue,
			submit: func(
				ctx context.Context,
				manager *Manager,
				sessionID string,
				messageID string,
				idempotencyKey string,
			) (SendPromptResult, error) {
				return manager.SendPrompt(ctx, sessionID, SendPromptOpts{
					Message: "durable queued retry", MessageID: messageID, IdempotencyKey: idempotencyKey,
					Mode: BusyInputModeQueue,
				})
			},
		},
		{
			name: "Should dispatch one identity-bearing staged steer after an exact retry",
			mode: BusyInputModeSteer,
			submit: func(
				ctx context.Context,
				manager *Manager,
				sessionID string,
				messageID string,
				idempotencyKey string,
			) (SendPromptResult, error) {
				active, ok := manager.Get(sessionID)
				if !ok {
					return SendPromptResult{}, ErrSessionNotFound
				}
				return manager.SteerPrompt(ctx, sessionID, SteerPromptOpts{
					Message: "durable queued retry", MessageID: messageID, IdempotencyKey: idempotencyKey,
					ExpectedTurnID: active.CurrentTurnID(),
				})
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			queueStore := openManagerInputQueueStore(t)
			h := newHarness(
				t,
				WithSessionInputQueueStore(queueStore),
				WithSessionBusyInputConfig(compozyconfig.SessionBusyInputConfig{
					DefaultMode:  string(BusyInputModeQueue),
					QueueCap:     3,
					MaxTextBytes: 4096,
				}),
			)
			registerManagerInputQueueWorkspace(t, queueStore, h)
			sess := createSession(t, h)
			registerManagerInputQueueSession(t, queueStore, h, sess)

			activeEntered := make(chan struct{})
			releaseActive := make(chan struct{})
			dispatched := make(chan struct{})
			var releaseActiveOnce sync.Once
			t.Cleanup(func() {
				if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
					t.Errorf("Stop() error = %v", err)
				}
			})
			t.Cleanup(func() {
				releaseActiveOnce.Do(func() { close(releaseActive) })
			})
			h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				events := make(chan acp.AgentEvent)
				go func() {
					defer close(events)
					switch req.Message {
					case "active prompt":
						close(activeEntered)
						<-releaseActive
					case "durable queued retry":
						close(dispatched)
					}
					emitDonePromptEvents(events, sess.ID, req.TurnID)
				}()
				return events, nil
			}

			active, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
				Message: "active prompt",
			})
			if err != nil {
				t.Fatalf("SendPrompt(active) error = %v", err)
			}
			<-activeEntered

			first, err := tc.submit(
				testutil.Context(t), h.manager, sess.ID, "message-busy-retry", "idem-busy-retry",
			)
			if err != nil {
				t.Fatalf("submit(first) error = %v", err)
			}
			if first.Events != nil || first.MessageID != "message-busy-retry" ||
				first.IdempotencyKey != "idem-busy-retry" || first.QueueEntryID == "" ||
				first.Mode != tc.mode {
				t.Fatalf("submit(first) = %#v, want durable %s admission", first, tc.mode)
			}
			if tc.mode == BusyInputModeQueue && first.Status != "queued" {
				t.Fatalf("submit(first) = %#v, want queued result", first)
			}
			if tc.mode == BusyInputModeSteer && first.Status != "steering" {
				t.Fatalf("submit(first) = %#v, want staged result", first)
			}
			entry, err := h.manager.inputQueue.Get(testutil.Context(t), sess.ID, first.QueueEntryID)
			if err != nil {
				t.Fatalf("inputQueue.Get(first) error = %v", err)
			}
			if entry.MessageID != first.MessageID || entry.IdempotencyKey != first.IdempotencyKey ||
				entry.TurnID == "" || entry.EventID == "" {
				t.Fatalf("first durable queue entry = %#v, want complete prompt identities", entry)
			}

			replayed, err := tc.submit(
				testutil.Context(t), h.manager, sess.ID, "message-busy-retry", "idem-busy-retry",
			)
			if err != nil {
				t.Fatalf("submit(replay) error = %v", err)
			}
			if !replayed.Replayed || replayed.Events != nil || replayed.QueueEntryID != first.QueueEntryID ||
				replayed.MessageID != first.MessageID || replayed.IdempotencyKey != first.IdempotencyKey ||
				replayed.Mode != first.Mode {
				t.Fatalf("submit(replay) = %#v, want stable non-streaming replay of %#v", replayed, first)
			}
			if got := len(managerPromptCalls(h)); got != 1 {
				t.Fatalf("len(promptCalls) before busy retry dispatch = %d, want active prompt only", got)
			}

			releaseActiveOnce.Do(func() { close(releaseActive) })
			collectEvents(t, active.Events)
			select {
			case <-dispatched:
			case <-time.After(time.Second):
				t.Fatal("durable busy-input retry did not dispatch")
			}
			promptCalls := managerPromptCalls(h)
			if len(promptCalls) != 2 {
				t.Fatalf(
					"prompt calls after busy retry dispatch = %#v, want active and one retried command",
					promptCalls,
				)
			}
			if got := promptCalls[1].TurnID; got != entry.TurnID {
				t.Fatalf("dispatched retry turn id = %q, want %q", got, entry.TurnID)
			}
			assertManagerBusyInputDispatchedIdentity(t, sess, &entry)
		})
	}
}

func TestManagerBusyInputPromptAdmissionIdentityConflicts(t *testing.T) {
	t.Parallel()

	baseline := RuntimeSelection{
		Provider: "claude", Model: "shared-runtime-model", ReasoningEffort: "medium", Speed: speedpkg.SpeedNormal,
	}
	cases := []struct {
		name   string
		mode   BusyInputMode
		mutate func(*RuntimeSelection)
	}{
		{
			name:   "Should reject a changed busy-input mode with the same identity",
			mode:   BusyInputModeSteer,
			mutate: func(*RuntimeSelection) {},
		},
		{
			name: "Should reject a changed resolved runtime provider with the same identity",
			mode: BusyInputModeQueue,
			mutate: func(runtime *RuntimeSelection) {
				runtime.Provider = "codex"
			},
		},
		{
			name: "Should reject a changed resolved runtime model with the same identity",
			mode: BusyInputModeQueue,
			mutate: func(runtime *RuntimeSelection) {
				runtime.Model = "alternate-runtime-model"
			},
		},
		{
			name: "Should reject a changed resolved runtime reasoning effort with the same identity",
			mode: BusyInputModeQueue,
			mutate: func(runtime *RuntimeSelection) {
				runtime.ReasoningEffort = "high"
			},
		},
		{
			name: "Should reject a changed resolved runtime speed with the same identity",
			mode: BusyInputModeQueue,
			mutate: func(runtime *RuntimeSelection) {
				runtime.Speed = speedpkg.SpeedFast
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			queueStore := openManagerInputQueueStore(t)
			h := newHarness(
				t,
				WithSessionInputQueueStore(queueStore),
				WithSessionBusyInputConfig(compozyconfig.SessionBusyInputConfig{
					DefaultMode:  string(BusyInputModeQueue),
					QueueCap:     3,
					MaxTextBytes: 4096,
				}),
			)
			registerManagerInputQueueWorkspace(t, queueStore, h)
			sess := createSession(t, h)
			registerManagerInputQueueSession(t, queueStore, h, sess)
			setManagerBusyInputProviderDefault(t, h, "claude", "shared-runtime-model")
			setManagerBusyInputProviderDefault(t, h, "codex", "shared-runtime-model")

			activeEntered := make(chan struct{})
			releaseActive := make(chan struct{})
			var releaseActiveOnce sync.Once
			t.Cleanup(func() {
				if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
					t.Errorf("Stop() error = %v", err)
				}
			})
			t.Cleanup(func() {
				releaseActiveOnce.Do(func() { close(releaseActive) })
			})
			h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				events := make(chan acp.AgentEvent)
				go func() {
					defer close(events)
					if req.Message == "active prompt" {
						close(activeEntered)
						<-releaseActive
					}
					emitDonePromptEvents(events, sess.ID, req.TurnID)
				}()
				return events, nil
			}

			active, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
				Message: "active prompt",
			})
			if err != nil {
				t.Fatalf("SendPrompt(active) error = %v", err)
			}
			<-activeEntered

			first, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
				Message: "runtime-bound prompt", MessageID: "message-runtime-conflict",
				IdempotencyKey: "idem-runtime-conflict", Mode: BusyInputModeQueue, Runtime: &baseline,
			})
			if err != nil {
				t.Fatalf("SendPrompt(first) error = %v", err)
			}
			if first.Status != "queued" || first.QueueEntryID == "" {
				t.Fatalf("SendPrompt(first) = %#v, want queued admission", first)
			}
			promptCallsBefore := len(managerPromptCalls(h))
			eventsBefore := len(readStoredEvents(t, sess))

			conflictingRuntime := baseline
			tc.mutate(&conflictingRuntime)
			_, err = h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
				Message: "runtime-bound prompt", MessageID: "message-runtime-conflict",
				IdempotencyKey: "idem-runtime-conflict", Mode: tc.mode, Runtime: &conflictingRuntime,
				ExpectedTurnID: sess.CurrentTurnID(),
			})
			if !errors.Is(err, store.ErrSessionPromptIdempotencyConflict) {
				t.Fatalf("SendPrompt(conflicting identity) error = %v, want idempotency conflict", err)
			}
			if got := len(managerPromptCalls(h)); got != promptCallsBefore {
				t.Fatalf("prompt calls after identity conflict = %d, want %d", got, promptCallsBefore)
			}
			if got := len(readStoredEvents(t, sess)); got != eventsBefore {
				t.Fatalf("stored events after identity conflict = %d, want %d", got, eventsBefore)
			}

			releaseActiveOnce.Do(func() { close(releaseActive) })
			collectEvents(t, active.Events)
		})
	}
}

func TestManagerBusyInputRuntimeSnapshots(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should dispatch steering before queued input with each runtime selected at admission",
		func(t *testing.T) {
			t.Parallel()

			queueStore := openManagerInputQueueStore(t)
			h := newHarness(
				t,
				WithSessionInputQueueStore(queueStore),
				WithSessionBusyInputConfig(compozyconfig.SessionBusyInputConfig{
					DefaultMode:  string(BusyInputModeQueue),
					QueueCap:     3,
					MaxTextBytes: 4096,
				}),
			)
			registerManagerInputQueueWorkspace(t, queueStore, h)
			session := createSession(t, h)
			registerManagerInputQueueSession(t, queueStore, h, session)
			setManagerBusyInputProviderDefault(t, h, "codex", "codex-admission-model")
			setManagerBusyInputProviderDefault(t, h, "claude", "claude-admission-model")

			activeEntered := make(chan struct{})
			queuedEntered := make(chan struct{})
			steerEntered := make(chan struct{})
			releaseActive := make(chan struct{})
			releaseQueued := make(chan struct{})
			releaseSteer := make(chan struct{})
			var releaseActiveOnce sync.Once
			var releaseQueuedOnce sync.Once
			var releaseSteerOnce sync.Once
			t.Cleanup(func() {
				releaseActiveOnce.Do(func() { close(releaseActive) })
				releaseQueuedOnce.Do(func() { close(releaseQueued) })
				releaseSteerOnce.Do(func() { close(releaseSteer) })
			})
			t.Cleanup(func() {
				if err := h.manager.Stop(testutil.Context(t), session.ID); err != nil {
					t.Errorf("Stop(%q) error = %v", session.ID, err)
				}
			})

			h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				events := make(chan acp.AgentEvent)
				go func() {
					defer close(events)
					switch {
					case req.Message == "active turn":
						close(activeEntered)
						<-releaseActive
					case promptRequestEndsWith(req.Message, "queued codex runtime"):
						close(queuedEntered)
						<-releaseQueued
					case promptRequestEndsWith(req.Message, "steered claude runtime"):
						close(steerEntered)
						<-releaseSteer
					}
					emitDonePromptEvents(events, session.ID, req.TurnID)
				}()
				return events, nil
			}

			active, err := h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{Message: "active turn"})
			if err != nil {
				t.Fatalf("SendPrompt(active) error = %v", err)
			}
			<-activeEntered

			queued, err := h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{
				Message: "queued codex runtime",
				Mode:    BusyInputModeQueue,
				Runtime: &RuntimeSelection{Provider: "codex"},
			})
			if err != nil {
				t.Fatalf("SendPrompt(queued) error = %v", err)
			}
			steered, err := h.manager.SendPrompt(testutil.Context(t), session.ID, SendPromptOpts{
				Message:        "steered claude runtime",
				Mode:           BusyInputModeSteer,
				ExpectedTurnID: session.CurrentTurnID(),
				Runtime:        &RuntimeSelection{Provider: "claude"},
			})
			if err != nil {
				t.Fatalf("SendPrompt(steer) error = %v", err)
			}
			assertManagerBusyInputRuntime(t, h, session.ID, queued.QueueEntryID, RuntimeSelection{
				Provider: "codex",
				Model:    "codex-admission-model",
			})
			assertManagerBusyInputRuntime(t, h, session.ID, steered.QueueEntryID, RuntimeSelection{
				Provider: "claude",
				Model:    "claude-admission-model",
			})

			setManagerBusyInputProviderDefault(t, h, "codex", "codex-updated-model")
			setManagerBusyInputProviderDefault(t, h, "claude", "claude-updated-model")

			releaseActiveOnce.Do(func() { close(releaseActive) })
			collectEvents(t, active.Events)
			<-steerEntered
			if got := session.Info().Provider; got != "claude" {
				t.Fatalf("runtime after steering dispatch = %q, want claude", got)
			}
			if got := session.Info().Model; got != "claude-admission-model" {
				t.Fatalf("runtime model after steering dispatch = %q, want claude admission model", got)
			}
			assertManagerBusyInputPersistedRuntime(
				t,
				session,
				acp.EventTypeUserMessage,
				"steered claude runtime",
				RuntimeSelection{
					Provider: "claude",
					Model:    "claude-admission-model",
					Speed:    speedpkg.SpeedNormal,
				},
			)

			releaseSteerOnce.Do(func() { close(releaseSteer) })
			<-queuedEntered
			if got := session.Info().Provider; got != "codex" {
				t.Fatalf("runtime after queued dispatch = %q, want codex", got)
			}
			if got := session.Info().Model; got != "codex-admission-model" {
				t.Fatalf("runtime model after queued dispatch = %q, want codex admission model", got)
			}
			assertManagerBusyInputPersistedRuntime(
				t,
				session,
				acp.EventTypeUserMessage,
				"queued codex runtime",
				RuntimeSelection{
					Provider: "codex",
					Model:    "codex-admission-model",
					Speed:    speedpkg.SpeedNormal,
				},
			)
			releaseQueuedOnce.Do(func() { close(releaseQueued) })
			waitForCondition(t, "all busy-input prompts dispatched", func() bool {
				return len(managerPromptCalls(h)) == 3
			})
		},
	)
}

func assertManagerBusyInputRuntime(
	t *testing.T,
	h *harness,
	sessionID string,
	entryID string,
	want RuntimeSelection,
) {
	t.Helper()

	entry, err := h.manager.inputQueue.Get(testutil.Context(t), sessionID, entryID)
	if err != nil {
		t.Fatalf("inputQueue.Get(%q) error = %v", entryID, err)
	}
	if got := entry.Runtime.Provider; got != want.Provider {
		t.Fatalf("queue entry %q runtime provider = %q, want %q", entryID, got, want.Provider)
	}
	if got := entry.Runtime.Model; got != want.Model {
		t.Fatalf("queue entry %q runtime model = %q, want %q", entryID, got, want.Model)
	}
}

func setManagerBusyInputProviderDefault(t *testing.T, h *harness, provider string, model string) {
	t.Helper()

	resolved, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve(workspace) error = %v", err)
	}
	resolved.Config.Providers[provider] = compozyconfig.ProviderConfig{
		Models: compozyconfig.ProviderModelsConfig{Default: model},
	}
	h.resolver.mu.Lock()
	h.resolver.upsert(&resolved)
	h.resolver.mu.Unlock()
}

func assertManagerBusyInputPersistedRuntime(
	t *testing.T,
	session *Session,
	eventType string,
	wantText string,
	want RuntimeSelection,
) {
	t.Helper()

	for _, event := range readStoredEvents(t, session) {
		if event.Type != eventType {
			continue
		}
		payload := decodeStoredEventPayload(t, event)
		if got, _ := payload["text"].(string); got != wantText {
			continue
		}
		runtime, ok := payload["prompt_runtime"].(map[string]any)
		if !ok {
			t.Fatalf("persisted runtime for %q = %#v, want object", wantText, payload["prompt_runtime"])
		}
		if got, _ := runtime["provider"].(string); got != want.Provider {
			t.Fatalf("persisted runtime provider for %q = %q, want %q", wantText, got, want.Provider)
		}
		if got, _ := runtime["model"].(string); got != want.Model {
			t.Fatalf("persisted runtime model for %q = %q, want %q", wantText, got, want.Model)
		}
		if got, _ := runtime["reasoning_effort"].(string); got != want.ReasoningEffort {
			t.Fatalf(
				"persisted runtime reasoning effort for %q = %q, want %q",
				wantText,
				got,
				want.ReasoningEffort,
			)
		}
		if got, _ := runtime["speed"].(string); got != string(want.Speed) {
			t.Fatalf("persisted runtime speed for %q = %q, want %q", wantText, got, want.Speed)
		}
		return
	}
	t.Fatalf("persisted %s event %q not found", eventType, wantText)
}

func promptRequestEndsWith(message string, request string) bool {
	return message == request || strings.HasSuffix(message, "User request:\n\n"+request)
}

func TestManagerGoalCommandDispatchShouldPreserveIngressAndDraftAdmission(t *testing.T) {
	t.Parallel()

	t.Run("Should keep internal prompt text literal when command parsing is not allowed", func(t *testing.T) {
		t.Parallel()

		var handlerCalls int
		queueStore := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(queueStore), WithGoalCommandHandler(GoalCommandHandlerFunc(func(
			context.Context,
			string,
			string,
			PromptCaller,
			GoalCommand,
		) (GoalDispatchDecision, error) {
			handlerCalls++
			return GoalDispatchDecision{}, errors.New("unexpected Goal handler call")
		})))
		registerManagerInputQueueWorkspace(t, queueStore, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, queueStore, h, sess)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		result, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "/goal status", MessageID: "client-literal-goal", IdempotencyKey: "idem-literal-goal",
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
			persistedInputs[0].MessageIDValue() != "client-literal-goal" {
			t.Fatalf("persisted literal input = %#v", persistedInputs[0])
		}
	})

	t.Run("Should reject a Cursor alias before a retryable Goal dispatch commit", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		handlerCalls := 0
		h := newHarness(t,
			WithSessionInputQueueStore(queueStore),
			WithGoalCommandHandler(GoalCommandHandlerFunc(func(
				context.Context,
				string,
				string,
				PromptCaller,
				GoalCommand,
			) (GoalDispatchDecision, error) {
				handlerCalls++
				return GoalDispatchDecision{
					Kind:   GoalDispatchRespond,
					Result: &GoalCommandResult{Outcome: GoalOutcomeStatus},
				}, nil
			})),
		)
		registerManagerInputQueueWorkspace(t, queueStore, h)
		sess := createCursorPromptAdmissionSession(t, h)
		registerManagerInputQueueSession(t, queueStore, h, sess)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})
		caller := PromptCaller{Kind: "human", ID: "operator", Source: "http"}

		_, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "/goal status", AllowCommands: true, Caller: caller,
			MessageID: "cursor-alias-goal", IdempotencyKey: "cursor-alias-goal",
			Runtime: &RuntimeSelection{Provider: "cursor", Model: "cursor-grok-4.5-high"},
		})
		assertCursorAliasRejectedBeforeDispatch(t, err)
		assertManagerPromptAdmissionCount(t, queueStore, sess.ID, 0)
		if handlerCalls != 0 {
			t.Fatalf("Goal handler calls after alias rejection = %d, want 0", handlerCalls)
		}

		accepted, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "/goal status", AllowCommands: true, Caller: caller,
			MessageID: "cursor-alias-goal", IdempotencyKey: "cursor-alias-goal",
			Runtime: &RuntimeSelection{Provider: "cursor", Model: cursorPromptAdmissionModel},
		})
		if err != nil {
			t.Fatalf("SendPrompt(retry advertised Goal model) error = %v", err)
		}
		if accepted.Goal == nil || accepted.Goal.Outcome != GoalOutcomeStatus {
			t.Fatalf("Goal retry result = %#v, want status result", accepted)
		}
		assertManagerPromptAdmissionCount(t, queueStore, sess.ID, 1)
		if handlerCalls != 1 {
			t.Fatalf("Goal handler calls after advertised retry = %d, want 1", handlerCalls)
		}
	})

	t.Run("Should return a structured command result without invoking ACP", func(t *testing.T) {
		t.Parallel()

		queueStore := openManagerInputQueueStore(t)
		attachmentID := "att_" + strings.Repeat("c", 64)
		attachmentData := []byte("Goal context")
		opener := &promptAttachmentOpenerStub{
			data: map[string][]byte{attachmentID: attachmentData},
			refs: map[string]attachmentspkg.AttachmentRef{
				attachmentID: storedPromptAttachmentRef(
					attachmentID, "goal-context.txt", "text/plain", attachmentData,
				),
			},
		}
		handlerCalls := 0
		h := newHarness(t, WithSessionInputQueueStore(queueStore), WithAttachmentOpener(opener), WithGoalCommandHandler(GoalCommandHandlerFunc(func(
			_ context.Context,
			workspaceID string,
			sessionID string,
			caller PromptCaller,
			command GoalCommand,
		) (GoalDispatchDecision, error) {
			handlerCalls++
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
		registerManagerInputQueueWorkspace(t, queueStore, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, queueStore, h, sess)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})

		opts := SendPromptOpts{
			Message: "/goal status", AllowCommands: true,
			MessageID:      "client-goal-status",
			IdempotencyKey: "idem-goal-status",
			Caller:         PromptCaller{Kind: "human", ID: "operator", Source: "http"},
			Attachments: []AttachmentMeta{
				promptAttachmentMeta(attachmentID, "goal-context.txt", "text/plain", attachmentData),
			},
		}
		result, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, opts)
		if err != nil {
			t.Fatalf("SendPrompt(Goal status) error = %v", err)
		}
		if result.Events != nil || result.Goal == nil || result.Goal.Outcome != GoalOutcomeStatus {
			t.Fatalf("structured Goal result = %#v", result)
		}
		if calls := managerPromptCalls(h); len(calls) != 0 {
			t.Fatalf("ACP prompt calls = %d, want 0", len(calls))
		}
		callsBeforeReplay := opener.calls
		delete(opener.data, attachmentID)
		delete(opener.refs, attachmentID)
		replayed, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, opts)
		if err != nil {
			t.Fatalf("SendPrompt(replay Goal status) error = %v", err)
		}
		if !replayed.Replayed || replayed.Goal == nil || replayed.Goal.Outcome != GoalOutcomeStatus {
			t.Fatalf("replayed Goal result = %#v, want replayed status result", replayed)
		}
		if opener.calls != callsBeforeReplay {
			t.Fatalf("attachment opener calls = %d after replay, want %d", opener.calls, callsBeforeReplay)
		}
		if handlerCalls != 1 {
			t.Fatalf("Goal handler calls = %d, want 1", handlerCalls)
		}
		persistedInputs := managerUserPromptEvents(t, h, sess.ID)
		if got, want := len(persistedInputs), 1; got != want {
			t.Fatalf("persisted Goal status inputs = %d, want %d; events=%#v", got, want, persistedInputs)
		}
		if persistedInputs[0].Text != "/goal status" ||
			persistedInputs[0].MessageIDValue() != "client-goal-status" {
			t.Fatalf("persisted Goal status input = %#v", persistedInputs[0])
		}
	})

	t.Run("Should dispatch a durable clear after the session stops without invoking ACP", func(t *testing.T) {
		t.Parallel()

		var handlerCalls int
		var expectedWorkspaceID string
		var expectedSessionID string
		queueStore := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(queueStore), WithGoalCommandHandler(GoalCommandHandlerFunc(func(
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
		registerManagerInputQueueWorkspace(t, queueStore, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, queueStore, h, sess)
		expectedWorkspaceID = h.workspaceID
		expectedSessionID = sess.ID
		if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
			t.Fatalf("Stop() error = %v", err)
		}

		result, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "/goal clear", AllowCommands: true,
			MessageID:      "client-goal-clear",
			IdempotencyKey: "idem-goal-clear",
			Caller:         PromptCaller{Kind: "human", ID: "operator", Source: "http"},
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
			persistedInputs[0].MessageIDValue() != "client-goal-clear" {
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
		queueStore := openManagerInputQueueStore(t)
		h := newHarness(t, WithSessionInputQueueStore(queueStore), WithGoalCommandHandler(handler))
		registerManagerInputQueueWorkspace(t, queueStore, h)
		sess := createSession(t, h)
		registerManagerInputQueueSession(t, queueStore, h, sess)
		t.Cleanup(func() {
			if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
				t.Errorf("Stop() error = %v", err)
			}
		})
		caller := PromptCaller{Kind: "human", ID: "operator", Source: "http"}

		admitted, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message: "/goal draft improve objective", AllowCommands: true, Caller: caller,
			MessageID: "client-goal-draft", IdempotencyKey: "idem-goal-draft",
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
			persistedInputs[0].MessageIDValue() != "client-goal-draft" {
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
			AllowCommands: true, Caller: caller,
		})
		if err != nil {
			t.Fatalf("SendPrompt(busy draft) error = %v", err)
		}
		if busy.Goal == nil || busy.Goal.ReasonCode == nil ||
			*busy.Goal.ReasonCode != GoalReasonDraftRequiresIdle || busy.Events != nil {
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
		turnStarted := make(chan struct{}, 2)
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
		dispatcher.dispatchTurnStartFn = func(
			_ context.Context,
			payload hookspkg.TurnStartPayload,
		) (hookspkg.TurnStartPayload, error) {
			turnStarted <- struct{}{}
			return payload, nil
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
				Message: "/goal draft race objective", AllowCommands: true,
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
			*draft.result.Goal.ReasonCode != GoalReasonDraftRequiresIdle || draft.result.Events != nil {
			t.Fatalf("racing draft result = %#v", draft.result)
		}
		if calls := managerPromptCalls(h); len(calls) != 1 || calls[0].Message != "active winner" {
			t.Fatalf("prompt calls after draft race = %#v, want only active winner", calls)
		}
		if got := len(turnStarted); got != 1 {
			t.Fatalf("turn.start dispatches after rejected draft = %d, want active turn only", got)
		}
		releaseActiveOnce.Do(func() { close(releaseActive) })
		collectEvents(t, active.Events)
	})
}

const cursorPromptAdmissionModel = "grok-4.5[effort=high,fast=true]"

func createCursorPromptAdmissionSession(t *testing.T, h *harness) *Session {
	t.Helper()

	resolvedWorkspace, err := h.resolver.Resolve(testutil.Context(t), h.workspaceID)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	for index := range resolvedWorkspace.Agents {
		if resolvedWorkspace.Agents[index].Name == "coder" {
			resolvedWorkspace.Agents[index].Provider = "cursor"
			resolvedWorkspace.Agents[index].Model = ""
		}
	}
	h.resolver.upsert(&resolvedWorkspace)
	h.driver.startHook = func(opts acp.StartOpts, _ int) (*fakeProcess, error) {
		proc := newFakeProcess(opts.AgentName, opts.Command, opts.Cwd, "acp-cursor-prompt-admission")
		proc.handle.setCaps(acp.Caps{ConfigOptions: []acp.SessionConfigOption{{
			ID:       "model",
			Category: "model",
			Kind:     acp.SessionConfigOptionKindSelect,
			Current:  cursorPromptAdmissionModel,
			Values:   []acp.SessionConfigOptionValue{{Value: cursorPromptAdmissionModel}},
		}}})
		return proc, nil
	}
	return createSession(t, h)
}

func assertCursorAliasRejectedBeforeDispatch(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Cursor alias submission error = nil, want exact-membership rejection")
	}
	negotiationErr, ok := errors.AsType[*acp.NegotiationError](err)
	if !ok || negotiationErr.Code != acp.NegotiationCodeModelUnavailable {
		t.Fatalf("Cursor alias submission error = %v, want model_unavailable NegotiationError", err)
	}
	if errors.Is(err, store.ErrSessionPromptDispatchIndeterminate) {
		t.Fatalf("Cursor alias submission error = %v, must remain retryable", err)
	}
}

func assertManagerPromptAdmissionCount(
	t *testing.T,
	queueStore *globaldb.GlobalDB,
	sessionID string,
	want int,
) {
	t.Helper()

	var count int
	if err := queueStore.DB().QueryRowContext(
		testutil.Context(t),
		"SELECT COUNT(*) FROM session_prompt_admissions WHERE session_id = ?",
		sessionID,
	).Scan(&count); err != nil {
		t.Fatalf("count session_prompt_admissions error = %v", err)
	}
	if count != want {
		t.Fatalf("session_prompt_admissions = %d, want %d", count, want)
	}
}

func TestManagerBusyInputManagedLifecycle(t *testing.T) {
	t.Run("Should persist exact Goal prompt metadata for every managed prompt kind", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name       string
			queueKind  string
			publicKind string
			generation int64
			turn       *int
		}{
			{
				name:       "Should persist Goal work metadata",
				queueKind:  "work",
				publicKind: "goal-work",
				generation: 1,
				turn:       new(1),
			},
			{
				name:       "Should persist Goal continuation metadata",
				queueKind:  "continuation",
				publicKind: "goal-continuation",
				generation: int64(math.MaxInt32) + 1,
				turn:       new(2),
			},
			{
				name:       "Should persist Goal compaction metadata",
				queueKind:  "compact",
				publicKind: "goal-compaction",
				generation: math.MaxInt64,
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
				entry.RunGeneration = &tc.generation
				entry.PromptKind = tc.queueKind
				entry.PromptAttempt = 4
				entry.Runtime = store.SessionInputRuntime{
					Provider: "codex", Model: "gpt-5.6", ReasoningEffort: "medium", Speed: "normal",
				}
				h.manager.startManagedInputPrompt(sess, managedInputFromQueueEntry(&entry))
				lifecycle.waitTerminal(t)

				calls := managerPromptCalls(h)
				if len(calls) != 1 || calls[0].Meta.Synthetic == nil || calls[0].Meta.Synthetic.Goal == nil {
					t.Fatalf("managed driver prompt metadata = %#v", calls)
				}
				assertGoalPromptMeta(t, calls[0].Meta.Synthetic.Goal, tc.publicKind, &entry, tc.turn)
				if got := sess.Info(); got.Provider != entry.Runtime.Provider || got.Model != entry.Runtime.Model ||
					got.ReasoningEffort != entry.Runtime.ReasoningEffort || got.Speed != speedpkg.Speed(entry.Runtime.Speed) {
					t.Fatalf("managed runtime after dispatch = %#v, want queue snapshot %#v", got, entry.Runtime)
				}
				assertManagerBusyInputPersistedRuntime(
					t,
					sess,
					acp.EventTypeSyntheticReentry,
					entry.Text,
					RuntimeSelection{
						Provider: entry.Runtime.Provider, Model: entry.Runtime.Model,
						ReasoningEffort: entry.Runtime.ReasoningEffort, Speed: speedpkg.Speed(entry.Runtime.Speed),
					},
				)

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
					assertGoalPromptMeta(t, event.Goal, tc.publicKind, &entry, tc.turn)
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
				assertGoalPromptMeta(t, transcriptMeta.Goal, tc.publicKind, &entry, tc.turn)
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

		h.manager.startManagedInputPrompt(sess, managedInputFromQueueEntry(&entry))
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
				h.manager.startManagedInputPrompt(sess, managedInputFromQueueEntry(&entry))
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
		h.manager.startManagedInputPrompt(sess, managedInputFromQueueEntry(&entry))
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
			WithSessionBusyInputConfig(compozyconfig.SessionBusyInputConfig{
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
		if queued.Status != "queued" {
			t.Fatalf("queued result = %#v, want queued", queued)
		}

		interrupted, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message:        "replacement prompt",
			Mode:           BusyInputModeInterrupt,
			ExpectedTurnID: sess.CurrentTurnID(),
		})
		if err != nil {
			t.Fatalf("SendPrompt(interrupt) error = %v", err)
		}
		if interrupted.Status != "interrupting" ||
			interrupted.Delivery != store.SessionInputDeliveryInterruptThenPrompt ||
			interrupted.QueueGeneration != 1 || interrupted.CanceledQueuedEntries != 1 {
			t.Fatalf("interrupted result = %#v, want generation 1 with one canceled queue entry", interrupted)
		}
		if interrupted.Events != nil {
			t.Fatal("SendPrompt(interrupt).Events is non-nil, want durable acknowledgement")
		}
		collectEvents(t, firstEvents.Events)
		waitForCondition(t, "replacement prompt dispatch", func() bool {
			return len(managerPromptCalls(h)) == 2
		})
		promptCalls := managerPromptCalls(h)
		messages := make([]string, 0, len(promptCalls))
		for _, call := range promptCalls {
			messages = append(messages, call.Message)
		}
		if !slices.Equal(messages, []string{"first prompt", "replacement prompt"}) {
			t.Fatalf("prompt messages = %#v, want first then replacement without stale queue", messages)
		}
	})

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
		replacement, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
			Message:        "Plain replacement task",
			Mode:           BusyInputModeInterrupt,
			ExpectedTurnID: sess.CurrentTurnID(),
		})
		if err != nil {
			t.Fatalf("SendPrompt(replacement) error = %v", err)
		}
		collectEvents(t, first.Events)
		if replacement.Events != nil {
			t.Fatal("SendPrompt(replacement).Events is non-nil, want durable acknowledgement")
		}
		waitForCondition(t, "plain replacement dispatch", func() bool {
			return len(managerPromptCalls(h)) == 2
		})
		waitForCondition(t, "plain replacement completion", func() bool {
			return !sess.IsPrompting()
		})
		calls := managerPromptCalls(h)
		if len(calls) != 2 || calls[1].Message != "Plain replacement task" {
			t.Fatalf("prompt calls = %#v, want unsalvaged replacement", calls)
		}
		_, err = h.manager.SteerPrompt(
			testutil.Context(t),
			sess.ID,
			SteerPromptOpts{Message: "late correction"},
		)
		if !errors.Is(err, ErrPromptNotInProgress) {
			t.Fatalf("SteerPrompt(after replacement) error = %v, want ErrPromptNotInProgress", err)
		}
	})
}

func TestManagerPendingInputPromotionReplay(t *testing.T) {
	t.Parallel()

	t.Run(
		"Should replay promotion after the replacement turn starts without interrupting it again",
		func(t *testing.T) {
			t.Parallel()

			queueStore := openManagerInputQueueStore(t)
			h := newHarness(t, WithSessionInputQueueStore(queueStore))
			registerManagerInputQueueWorkspace(t, queueStore, h)
			sess := createSession(t, h)
			registerManagerInputQueueSession(t, queueStore, h, sess)
			activeEntered := make(chan struct{})
			replacementEntered := make(chan struct{})
			releaseActive := make(chan struct{})
			releaseReplacement := make(chan struct{})
			var releaseActiveOnce sync.Once
			var releaseReplacementOnce sync.Once
			t.Cleanup(func() {
				releaseActiveOnce.Do(func() { close(releaseActive) })
				releaseReplacementOnce.Do(func() { close(releaseReplacement) })
				if err := h.manager.Stop(testutil.Context(t), sess.ID); err != nil {
					t.Errorf("Stop() error = %v", err)
				}
			})
			h.driver.cancelHook = func(*fakeProcess) error {
				releaseActiveOnce.Do(func() { close(releaseActive) })
				return nil
			}
			h.driver.promptHook = func(_ *fakeProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
				events := make(chan acp.AgentEvent)
				go func() {
					defer close(events)
					switch req.Message {
					case "active prompt":
						close(activeEntered)
						<-releaseActive
					case "promote this input":
						close(replacementEntered)
						<-releaseReplacement
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
			targetTurnID := sess.CurrentTurnID()
			queued, err := h.manager.SendPrompt(testutil.Context(t), sess.ID, SendPromptOpts{
				Message: "promote this input", Mode: BusyInputModeQueue,
			})
			if err != nil {
				t.Fatalf("SendPrompt(queue) error = %v", err)
			}
			opts := PromotePendingInputOpts{
				Text: "promote this input", ExpectedTurnID: targetTurnID,
				MessageID: "message-promote-replay", IdempotencyKey: "idem-promote-replay",
			}
			first, err := h.manager.PromotePendingInputToSteer(
				testutil.Context(t), sess.ID, queued.QueueEntryID, opts,
			)
			if err != nil {
				t.Fatalf("PromotePendingInputToSteer(first) error = %v", err)
			}
			collectEvents(t, active.Events)
			<-replacementEntered
			replayed, err := h.manager.PromotePendingInputToSteer(
				testutil.Context(t), sess.ID, queued.QueueEntryID, opts,
			)
			if err != nil {
				t.Fatalf("PromotePendingInputToSteer(replay) error = %v", err)
			}
			if replayed.QueueEntryID != first.QueueEntryID {
				t.Fatalf("promotion replay queue id = %q, want %q", replayed.QueueEntryID, first.QueueEntryID)
			}
			if got := managerCancelCalls(h); got != 1 {
				t.Fatalf("driver cancel calls = %d, want one", got)
			}
			releaseReplacementOnce.Do(func() { close(releaseReplacement) })
		},
	)
}

func managerPromptCalls(h *harness) []acp.PromptRequest {
	h.driver.mu.Lock()
	defer h.driver.mu.Unlock()
	return append([]acp.PromptRequest(nil), h.driver.promptCalls...)
}

func managerCancelCalls(h *harness) int {
	h.driver.mu.Lock()
	defer h.driver.mu.Unlock()
	return h.driver.cancelCalls
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

func assertManagerBusyInputDispatchedIdentity(
	t *testing.T,
	session *Session,
	entry *store.SessionInputQueueEntry,
) {
	t.Helper()

	matched := 0
	for _, storedEvent := range readStoredEvents(t, session) {
		if storedEvent.Type != acp.EventTypeUserMessage {
			continue
		}
		event, err := transcript.UnmarshalAgentEvent(storedEvent.Content)
		if err != nil {
			t.Fatalf("UnmarshalAgentEvent(%s) error = %v", storedEvent.ID, err)
		}
		if event.MessageIDValue() != entry.MessageID {
			continue
		}
		matched++
		if storedEvent.ID != entry.EventID || storedEvent.TurnID != entry.TurnID ||
			event.TurnID != entry.TurnID || event.Text != entry.Text {
			t.Fatalf(
				"dispatched busy-input event = stored:%#v event:%#v, want entry:%#v",
				storedEvent,
				event,
				entry,
			)
		}
	}
	if matched != 1 {
		t.Fatalf("dispatched busy-input events for message %q = %d, want 1", entry.MessageID, matched)
	}
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
	entry *store.SessionInputQueueEntry,
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
		ID:            sess.ID,
		Name:          "Input Queue",
		AgentName:     "coder",
		Provider:      "claude",
		WorkspaceID:   h.workspaceID,
		SessionType:   string(SessionTypeUser),
		State:         string(StateActive),
		RuntimeStatus: store.SessionRuntimeReady,
		CreatedAt:     time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
		UpdatedAt:     time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}
}

func assertManagerInputQueueEntropyFailurePersistedNothing(
	t *testing.T,
	queueStore *globaldb.GlobalDB,
	sessionID string,
) {
	t.Helper()

	var queueEntries int
	if err := queueStore.DB().QueryRowContext(
		testutil.Context(t),
		"SELECT COUNT(*) FROM session_input_queue WHERE session_id = ?",
		sessionID,
	).Scan(&queueEntries); err != nil {
		t.Fatalf("count session_input_queue entries error = %v", err)
	}
	if queueEntries != 0 {
		t.Fatalf("session_input_queue entries = %d, want 0", queueEntries)
	}

	var admissions int
	if err := queueStore.DB().QueryRowContext(
		testutil.Context(t),
		"SELECT COUNT(*) FROM session_prompt_admissions WHERE session_id = ?",
		sessionID,
	).Scan(&admissions); err != nil {
		t.Fatalf("count session_prompt_admissions error = %v", err)
	}
	if admissions != 0 {
		t.Fatalf("session_prompt_admissions = %d, want 0", admissions)
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
