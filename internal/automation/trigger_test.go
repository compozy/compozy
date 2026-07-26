package automation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/testutil"
)

type testStringer string

func (s testStringer) String() string {
	return string(s)
}

func TestTriggerEngineExactFilterMatchesActivationEnvelope(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)
	engine := newTestTriggerEngine(t, dispatcher)

	trigger := testEventTrigger(AutomationScopeWorkspace, "session-match", "ws_alpha", "session.stopped")
	trigger.Filter = map[string]string{
		"data.agent_name": "researcher",
	}
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	matched, err := engine.Fire(testutil.Context(t), ActivationEnvelope{
		Kind:        "session.stopped",
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: "ws_alpha",
		Source:      ActivationSourceObserver,
		Data: map[string]any{
			"agent_name": "researcher",
			"session_id": "sess-match",
		},
	})
	if err != nil {
		t.Fatalf("Fire(matched) error = %v", err)
	}
	if got, want := matched.Matched, 1; got != want {
		t.Fatalf("matched.Matched = %d, want %d", got, want)
	}
	if got, want := len(creator.createCalls()), 1; got != want {
		t.Fatalf("len(Create calls after matched event) = %d, want %d", got, want)
	}

	notMatched, err := engine.Fire(testutil.Context(t), ActivationEnvelope{
		Kind:        "session.stopped",
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: "ws_alpha",
		Source:      ActivationSourceObserver,
		Data: map[string]any{
			"agent_name": "reviewer",
			"session_id": "sess-miss",
		},
	})
	if err != nil {
		t.Fatalf("Fire(not matched) error = %v", err)
	}
	if got := notMatched.Matched; got != 0 {
		t.Fatalf("notMatched.Matched = %d, want 0", got)
	}
	if got, want := len(creator.createCalls()), 1; got != want {
		t.Fatalf("len(Create calls after non-match) = %d, want %d", got, want)
	}
}

func TestRenderTriggerPromptFailsWhenEnvelopeFieldIsMissing(t *testing.T) {
	t.Parallel()

	_, err := renderTriggerPrompt(`Missing agent {{ .Data.agent_name }}`, &ActivationEnvelope{
		Kind:   "session.stopped",
		Scope:  AutomationScopeGlobal,
		Source: ActivationSourceObserver,
		Data: map[string]any{
			"session_id": "sess-1",
		},
	})
	if err == nil {
		t.Fatal("renderTriggerPrompt() error = nil, want non-nil")
	}
}

func TestRenderTriggerPromptReturnsTrimmedStaticPrompt(t *testing.T) {
	t.Parallel()

	t.Run("ShouldReturnTrimmedStaticPrompt", func(t *testing.T) {
		rendered, err := renderTriggerPrompt("  Static prompt body  ", &ActivationEnvelope{
			Kind:   "session.stopped",
			Scope:  AutomationScopeGlobal,
			Source: ActivationSourceObserver,
			Data: map[string]any{
				"session_id": "sess-1",
			},
		})
		if err != nil {
			t.Fatalf("renderTriggerPrompt() error = %v", err)
		}
		if got, want := rendered, "Static prompt body"; got != want {
			t.Fatalf("renderTriggerPrompt() = %q, want %q", got, want)
		}
	})
}

func TestParseWebhookEndpointResolvesStableWebhookID(t *testing.T) {
	t.Parallel()

	parsed, err := ParseWebhookEndpoint("deploy-review--wbh_123")
	if err != nil {
		t.Fatalf("ParseWebhookEndpoint() error = %v", err)
	}
	if got, want := parsed.EndpointSlug, "deploy-review"; got != want {
		t.Fatalf("parsed.EndpointSlug = %q, want %q", got, want)
	}
	if got, want := parsed.WebhookID, "wbh_123"; got != want {
		t.Fatalf("parsed.WebhookID = %q, want %q", got, want)
	}
}

func TestTriggerEngineRejectsInvalidWebhookSignatureBeforeDispatch(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)
	now := time.Date(2026, 4, 11, 2, 0, 0, 0, time.UTC)
	engine := newTestTriggerEngine(t, dispatcher, WithTriggerEngineNow(func() time.Time { return now }))

	trigger := testWebhookTrigger(AutomationScopeGlobal, "webhook-invalid-signature", "")
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	result, err := engine.HandleWebhook(testutil.Context(t), WebhookRequest{
		Scope:      AutomationScopeGlobal,
		Endpoint:   "deploy-review--" + trigger.WebhookID,
		DeliveryID: "delivery-invalid-signature",
		Timestamp:  now,
		Signature:  "sha256=deadbeef",
		Payload:    []byte(`{"payload":"deploy"}`),
	})
	if !errors.Is(err, ErrWebhookSignatureInvalid) {
		t.Fatalf("HandleWebhook() error = %v, want ErrWebhookSignatureInvalid", err)
	}
	if got := result.Matched; got != 0 {
		t.Fatalf("result.Matched = %d, want 0", got)
	}
	if got := len(creator.createCalls()); got != 0 {
		t.Fatalf("len(Create calls) = %d, want 0", got)
	}
}

func TestTriggerEngineRejectsStaleWebhookTimestampBeforeDispatch(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)
	now := time.Date(2026, 4, 11, 2, 30, 0, 0, time.UTC)
	engine := newTestTriggerEngine(
		t,
		dispatcher,
		WithTriggerEngineNow(func() time.Time { return now }),
		WithTriggerEngineWebhookFreshnessWindow(5*time.Minute),
	)

	trigger := testWebhookTrigger(AutomationScopeGlobal, "webhook-stale", "")
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	staleTimestamp := now.Add(-10 * time.Minute)
	signature, err := SignWebhookPayload(
		testWebhookSecretValue(trigger.WebhookSecretRef),
		staleTimestamp,
		[]byte(`{"payload":"deploy"}`),
	)
	if err != nil {
		t.Fatalf("SignWebhookPayload() error = %v", err)
	}

	result, err := engine.HandleWebhook(testutil.Context(t), WebhookRequest{
		Scope:      AutomationScopeGlobal,
		Endpoint:   "deploy-review--" + trigger.WebhookID,
		DeliveryID: "delivery-stale",
		Timestamp:  staleTimestamp,
		Signature:  signature,
		Payload:    []byte(`{"payload":"deploy"}`),
	})
	if !errors.Is(err, ErrWebhookTimestampInvalid) {
		t.Fatalf("HandleWebhook() error = %v, want ErrWebhookTimestampInvalid", err)
	}
	if got := result.Matched; got != 0 {
		t.Fatalf("result.Matched = %d, want 0", got)
	}
	if got := len(creator.createCalls()); got != 0 {
		t.Fatalf("len(Create calls) = %d, want 0", got)
	}
}

func TestTriggerEngineHookTelemetrySinkNormalizesCompletion(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)
	engine := newTestTriggerEngine(
		t,
		dispatcher,
		WithTriggerEngineHookSessionResolver(stubHookSessionResolver{
			info: &session.Info{
				ID:          "sess-hook",
				Name:        "hook-session",
				AgentName:   "researcher",
				WorkspaceID: "ws_alpha",
				Type:        session.SessionTypeUser,
				State:       session.StateActive,
			},
		}),
	)

	trigger := testEventTrigger(AutomationScopeWorkspace, "hook-complete", "ws_alpha", "hook.review.completed")
	trigger.Filter = map[string]string{
		"data.hook_outcome": "applied",
	}
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	err := engine.HookTelemetrySink().WriteHookRecord(testutil.Context(t), "sess-hook", hookspkg.HookRunRecord{
		HookName:      "review",
		Event:         hookspkg.HookPromptPostAssemble,
		Source:        hookspkg.HookSourceConfig,
		Mode:          hookspkg.HookModeSync,
		Outcome:       hookspkg.HookRunOutcomeApplied,
		DispatchDepth: 1,
		RecordedAt:    time.Date(2026, 4, 11, 3, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("WriteHookRecord() error = %v", err)
	}
	if got, want := len(creator.createCalls()), 1; got != want {
		t.Fatalf("len(Create calls) = %d, want %d", got, want)
	}
}

func TestTriggerEngineFireHookCompletionRejectsNilContextBeforeResolvingSession(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)
	resolver := &recordingHookSessionResolver{
		info: &session.Info{
			ID:          "sess-hook",
			Name:        "hook-session",
			WorkspaceID: "ws_alpha",
			Type:        session.SessionTypeUser,
		},
	}
	engine := newTestTriggerEngine(
		t,
		dispatcher,
		WithTriggerEngineHookSessionResolver(resolver),
	)

	_, err := engine.FireHookCompletion(nilContextForTests(), "sess-hook", hookspkg.HookRunRecord{HookName: "review"})
	if err == nil {
		t.Fatal("FireHookCompletion(nil) error = nil, want non-nil")
	}
	if got, want := err.Error(), "automation: trigger fire context is required"; got != want {
		t.Fatalf("FireHookCompletion(nil) error = %q, want %q", got, want)
	}
	if got, want := resolver.calls, 0; got != want {
		t.Fatalf("hook session resolver calls = %d, want %d", got, want)
	}
}

func TestTriggerEngineSessionAndMemoryObserversDispatchThroughSharedPath(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)
	engine := newTestTriggerEngine(t, dispatcher)

	createdTrigger := testEventTrigger(AutomationScopeGlobal, "session-created", "", "session.created")
	createdTrigger.Prompt = `Created {{ .Data.session_id }}`
	stoppedTrigger := testEventTrigger(AutomationScopeGlobal, "session-stopped", "", "session.stopped")
	stoppedTrigger.Prompt = `Stopped {{ .Data.session_id }}`
	memoryTrigger := testEventTrigger(AutomationScopeGlobal, "memory-observer", "", "memory.consolidated")
	memoryTrigger.Prompt = `Memory {{ .Data.summary }}`

	for _, trigger := range []Trigger{createdTrigger, stoppedTrigger, memoryTrigger} {
		if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
			t.Fatalf("Register(%s) error = %v", trigger.Event, err)
		}
	}

	sess := &session.Session{
		ID:        "sess-observer",
		Name:      "observer-session",
		AgentName: "researcher",
		Type:      session.SessionTypeUser,
		State:     session.StateStopped,
		CreatedAt: time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 11, 5, 5, 0, 0, time.UTC),
	}

	engine.SessionObserver().OnSessionCreated(testutil.Context(t), sess)
	engine.SessionObserver().OnSessionStopped(testutil.Context(t), sess)
	if err := engine.MemoryObserver().OnMemoryConsolidated(testutil.Context(t), MemoryConsolidatedEvent{
		Timestamp: time.Date(2026, 4, 11, 5, 10, 0, 0, time.UTC),
		Data: map[string]any{
			"summary": "fresh context",
		},
	}); err != nil {
		t.Fatalf("OnMemoryConsolidated() error = %v", err)
	}

	if got, want := len(creator.createCalls()), 3; got != want {
		t.Fatalf("len(Create calls) = %d, want %d", got, want)
	}
}

func TestTriggerEngineHandleWebhookDispatchesValidRequest(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	now := time.Date(2026, 4, 11, 5, 30, 0, 0, time.UTC)
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherNow(func() time.Time { return now }))
	engine := newTestTriggerEngine(t, dispatcher, WithTriggerEngineNow(func() time.Time { return now }))

	trigger := testWebhookTrigger(AutomationScopeGlobal, "webhook-valid", "")
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	payload := []byte(`{"payload":"deploy"}`)
	signature, err := SignWebhookPayload(testWebhookSecretValue(trigger.WebhookSecretRef), now, payload)
	if err != nil {
		t.Fatalf("SignWebhookPayload() error = %v", err)
	}

	result, err := engine.HandleWebhook(testutil.Context(t), WebhookRequest{
		Scope:      AutomationScopeGlobal,
		Endpoint:   "deploy-review--" + trigger.WebhookID,
		DeliveryID: "delivery-valid",
		Timestamp:  now,
		Signature:  signature,
		Payload:    payload,
		Data: map[string]any{
			"payload": "deploy",
		},
	})
	if err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}
	if got, want := result.Matched, 1; got != want {
		t.Fatalf("result.Matched = %d, want %d", got, want)
	}
	if got, want := len(creator.promptCalls()), 1; got != want {
		t.Fatalf("len(Prompt calls) = %d, want %d", got, want)
	}
	if got, want := creator.promptCalls()[0].message, "Review payload deploy"; got != want {
		t.Fatalf("Prompt().message = %q, want %q", got, want)
	}
}

func TestTriggerEnginePersistentWebhookReservationCarriesDefinitionParticipation(t *testing.T) {
	t.Parallel()

	t.Run("Should create the durable reservation with canonical authored participation", func(t *testing.T) {
		t.Parallel()

		store := newMemoryRunStore()
		starter := &recordingLoopStarter{runID: "looprun-webhook-participation"}
		now := time.Date(2026, 4, 11, 5, 45, 0, 0, time.UTC)
		dispatcher := newTestDispatcher(
			t,
			newRecordingSessionCreator(),
			store,
			WithDispatcherNow(func() time.Time { return now }),
			WithDispatcherLoopStarter(starter),
		)
		engine := newTestTriggerEngine(
			t,
			dispatcher,
			WithTriggerEngineNow(func() time.Time { return now }),
			WithTriggerEngineWebhookDeliveryStore(store),
		)

		trigger := testWebhookTrigger(AutomationScopeGlobal, "webhook-participation", "")
		trigger.TargetKind = TargetKindLoop
		trigger.AgentName = ""
		trigger.Prompt = ""
		trigger.LoopTarget = &LoopTarget{
			WorkspaceID:          "ws_alpha",
			LoopName:             "release-loop",
			NetworkParticipation: testNamedParticipation("webhook-loop"),
		}
		if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		payload := []byte(`{"payload":"deploy"}`)
		signature, err := SignWebhookPayload(testWebhookSecretValue(trigger.WebhookSecretRef), now, payload)
		if err != nil {
			t.Fatalf("SignWebhookPayload() error = %v", err)
		}
		result, err := engine.HandleWebhook(testutil.Context(t), WebhookRequest{
			Scope:      AutomationScopeGlobal,
			Endpoint:   "deploy-review--" + trigger.WebhookID,
			DeliveryID: "delivery-participation",
			Timestamp:  now,
			Signature:  signature,
			Payload:    payload,
			Data:       map[string]any{"payload": "deploy"},
		})
		if err != nil {
			t.Fatalf("HandleWebhook() error = %v", err)
		}
		if got, want := len(result.Runs), 1; got != want {
			t.Fatalf("len(result.Runs) = %d, want %d", got, want)
		}
		assertNamedParticipation(t, result.Runs[0].NetworkParticipation, "webhook-loop")
		updates := store.updateSnapshot()
		if len(updates) == 0 || updates[0].Status == RunScheduled {
			t.Fatalf("run updates = %#v, want execution transitions without reservation backfill", updates)
		}
	})
}

func TestTriggerEngineRejectsReplayedWebhookDeliveriesWithinFreshnessWindow(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	current := time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC)
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherNow(func() time.Time { return current }))
	engine := newTestTriggerEngine(
		t,
		dispatcher,
		WithTriggerEngineNow(func() time.Time { return current }),
		WithTriggerEngineWebhookFreshnessWindow(5*time.Minute),
	)

	trigger := testWebhookTrigger(AutomationScopeGlobal, "webhook-replay", "")
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	payload := []byte(`{"payload":"deploy"}`)
	signature, err := SignWebhookPayload(testWebhookSecretValue(trigger.WebhookSecretRef), current, payload)
	if err != nil {
		t.Fatalf("SignWebhookPayload() error = %v", err)
	}

	firstResult, err := engine.HandleWebhook(testutil.Context(t), WebhookRequest{
		Scope:      AutomationScopeGlobal,
		Endpoint:   "deploy-review--" + trigger.WebhookID,
		DeliveryID: "delivery-replay",
		Timestamp:  current,
		Signature:  signature,
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("HandleWebhook(first) error = %v", err)
	}
	if got, want := firstResult.Matched, 1; got != want {
		t.Fatalf("firstResult.Matched = %d, want %d", got, want)
	}

	secondResult, err := engine.HandleWebhook(testutil.Context(t), WebhookRequest{
		Scope:      AutomationScopeGlobal,
		Endpoint:   "deploy-review--" + trigger.WebhookID,
		DeliveryID: "delivery-replay",
		Timestamp:  current,
		Signature:  signature,
		Payload:    payload,
	})
	if !errors.Is(err, ErrWebhookReplayDetected) {
		t.Fatalf("HandleWebhook(replay) error = %v, want ErrWebhookReplayDetected", err)
	}
	if got := secondResult.Matched; got != 0 {
		t.Fatalf("secondResult.Matched = %d, want 0", got)
	}
	if got, want := len(creator.promptCalls()), 1; got != want {
		t.Fatalf("len(Prompt calls after replay) = %d, want %d", got, want)
	}

	current = current.Add(6 * time.Minute)
	signature, err = SignWebhookPayload(testWebhookSecretValue(trigger.WebhookSecretRef), current, payload)
	if err != nil {
		t.Fatalf("SignWebhookPayload(after expiry) error = %v", err)
	}
	thirdResult, err := engine.HandleWebhook(testutil.Context(t), WebhookRequest{
		Scope:      AutomationScopeGlobal,
		Endpoint:   "deploy-review--" + trigger.WebhookID,
		DeliveryID: "delivery-replay",
		Timestamp:  current,
		Signature:  signature,
		Payload:    payload,
	})
	if err != nil {
		t.Fatalf("HandleWebhook(after expiry) error = %v", err)
	}
	if got, want := thirdResult.Matched, 1; got != want {
		t.Fatalf("thirdResult.Matched = %d, want %d", got, want)
	}
}

func TestTriggerEngineAllowsWebhookRetryAfterDispatchFailsWithoutPersistingARun(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	createStarted := make(chan struct{}, 1)
	createRelease := make(chan struct{})
	creator := newRecordingSessionCreator(sessionAttemptPlan{
		createStarted: createStarted,
		createRelease: createRelease,
	})
	current := time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC)
	dispatcher := newTestDispatcher(
		t,
		creator,
		store,
		WithDispatcherNow(func() time.Time { return current }),
		WithDispatcherMaxConcurrent(1),
	)
	engine := newTestTriggerEngine(
		t,
		dispatcher,
		WithTriggerEngineNow(func() time.Time { return current }),
		WithTriggerEngineWebhookFreshnessWindow(5*time.Minute),
	)

	trigger := testWebhookTrigger(AutomationScopeGlobal, "webhook-retry-after-failure", "")
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	blockingJob := testJob(AutomationScopeGlobal, "blocking-webhook-dispatch", "")
	blockingDispatchErr := make(chan error, 1)
	go func() {
		_, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
			Kind: DispatchKindManual,
			Job:  &blockingJob,
		})
		blockingDispatchErr <- err
	}()

	<-createStarted

	payload := []byte(`{"payload":"deploy"}`)
	signature, err := SignWebhookPayload(testWebhookSecretValue(trigger.WebhookSecretRef), current, payload)
	if err != nil {
		t.Fatalf("SignWebhookPayload() error = %v", err)
	}

	firstResult, err := engine.HandleWebhook(testutil.Context(t), WebhookRequest{
		Scope:      AutomationScopeGlobal,
		Endpoint:   "deploy-review--" + trigger.WebhookID,
		DeliveryID: "delivery-transient-failure",
		Timestamp:  current,
		Signature:  signature,
		Payload:    payload,
		Data: map[string]any{
			"payload": "deploy",
		},
	})
	if !errors.Is(err, ErrConcurrencyLimitReached) {
		t.Fatalf("HandleWebhook(first) error = %v, want ErrConcurrencyLimitReached", err)
	}
	if got := len(firstResult.Runs); got != 0 {
		t.Fatalf("len(firstResult.Runs) = %d, want 0", got)
	}

	close(createRelease)
	if err := <-blockingDispatchErr; err != nil {
		t.Fatalf("blocking dispatcher.Dispatch() error = %v", err)
	}

	secondResult, err := engine.HandleWebhook(testutil.Context(t), WebhookRequest{
		Scope:      AutomationScopeGlobal,
		Endpoint:   "deploy-review--" + trigger.WebhookID,
		DeliveryID: "delivery-transient-failure",
		Timestamp:  current,
		Signature:  signature,
		Payload:    payload,
		Data: map[string]any{
			"payload": "deploy",
		},
	})
	if err != nil {
		t.Fatalf("HandleWebhook(second) error = %v", err)
	}
	if got, want := secondResult.Matched, 1; got != want {
		t.Fatalf("secondResult.Matched = %d, want %d", got, want)
	}
	if got, want := len(secondResult.Runs), 1; got != want {
		t.Fatalf("len(secondResult.Runs) = %d, want %d", got, want)
	}
}

func TestTriggerEngineAllowsWebhookRetryWhenFilteringProducesNoRuns(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	current := time.Date(2026, 4, 11, 5, 0, 0, 0, time.UTC)
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherNow(func() time.Time { return current }))
	engine := newTestTriggerEngine(
		t,
		dispatcher,
		WithTriggerEngineNow(func() time.Time { return current }),
		WithTriggerEngineWebhookFreshnessWindow(5*time.Minute),
	)

	trigger := testWebhookTrigger(AutomationScopeGlobal, "webhook-filter-no-runs", "")
	trigger.Filter = map[string]string{"data.payload": "approved"}
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	payload := []byte(`{"payload":"deploy"}`)
	signature, err := SignWebhookPayload(testWebhookSecretValue(trigger.WebhookSecretRef), current, payload)
	if err != nil {
		t.Fatalf("SignWebhookPayload() error = %v", err)
	}

	firstResult, err := engine.HandleWebhook(testutil.Context(t), WebhookRequest{
		Scope:      AutomationScopeGlobal,
		Endpoint:   "deploy-review--" + trigger.WebhookID,
		DeliveryID: "delivery-filtered",
		Timestamp:  current,
		Signature:  signature,
		Payload:    payload,
		Data: map[string]any{
			"payload": "deploy",
		},
	})
	if err != nil {
		t.Fatalf("HandleWebhook(first) error = %v", err)
	}
	if got := len(firstResult.Runs); got != 0 {
		t.Fatalf("len(firstResult.Runs) = %d, want 0", got)
	}

	secondResult, err := engine.HandleWebhook(testutil.Context(t), WebhookRequest{
		Scope:      AutomationScopeGlobal,
		Endpoint:   "deploy-review--" + trigger.WebhookID,
		DeliveryID: "delivery-filtered",
		Timestamp:  current,
		Signature:  signature,
		Payload:    payload,
		Data: map[string]any{
			"payload": "deploy",
		},
	})
	if err != nil {
		t.Fatalf("HandleWebhook(second) error = %v, want nil retry after filter", err)
	}
	if got := len(secondResult.Runs); got != 0 {
		t.Fatalf("len(secondResult.Runs) = %d, want 0", got)
	}
	if got := len(creator.promptCalls()); got != 0 {
		t.Fatalf("len(Prompt calls) = %d, want 0", got)
	}
}

func TestTriggerEngineRegisterUpdateUnregisterAndLifecycle(t *testing.T) {
	t.Parallel()

	if _, err := NewTriggerEngine(nil); err == nil {
		t.Fatal("NewTriggerEngine(nil) error = nil, want non-nil")
	}

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)
	engine := newTestTriggerEngine(t, dispatcher, WithTriggerEngineLogger(nil))

	if err := engine.Start(testutil.Context(t)); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	trigger := testWebhookTrigger(AutomationScopeGlobal, "register-update", "")
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); !errors.Is(err, ErrTriggerAlreadyRegistered) {
		t.Fatalf("Register(duplicate) error = %v, want ErrTriggerAlreadyRegistered", err)
	}

	updated := trigger
	updated.EndpointSlug = "deploy-updated"
	if err := engine.Update(TriggerRegistration{Trigger: updated}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if err := engine.Unregister(trigger.ID); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}
	if err := engine.Unregister(trigger.ID); !errors.Is(err, ErrTriggerNotFound) {
		t.Fatalf("Unregister(missing) error = %v, want ErrTriggerNotFound", err)
	}

	if err := engine.Shutdown(testutil.Context(t)); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
	if err := engine.Start(testutil.Context(t)); !errors.Is(err, ErrTriggerEngineStopped) {
		t.Fatalf("Start(after shutdown) error = %v, want ErrTriggerEngineStopped", err)
	}
	if err := engine.Register(
		TriggerRegistration{Trigger: trigger},
	); !errors.Is(
		err,
		ErrTriggerEngineStopped,
	) {
		t.Fatalf("Register(after shutdown) error = %v, want ErrTriggerEngineStopped", err)
	}
}

func TestTriggerRegistrationAndWebhookRequestValidation(t *testing.T) {
	t.Parallel()

	eventTrigger := testEventTrigger(AutomationScopeGlobal, "bad-secret", "", "session.stopped")
	eventTrigger.WebhookSecretRef = "env:UNEXPECTED_WEBHOOK_SECRET"
	err := (TriggerRegistration{Trigger: eventTrigger}).Validate("registration")
	if err == nil {
		t.Fatal("TriggerRegistration.Validate(non-webhook secret) error = nil, want non-nil")
	}

	webhookTrigger := testWebhookTrigger(AutomationScopeGlobal, "missing-secret", "")
	webhookTrigger.WebhookSecretRef = ""
	err = (TriggerRegistration{Trigger: webhookTrigger}).Validate("registration")
	if err == nil {
		t.Fatal("TriggerRegistration.Validate(missing secret) error = nil, want non-nil")
	}

	err = (WebhookRequest{Scope: AutomationScopeGlobal}).Validate("webhook")
	if err == nil {
		t.Fatal("WebhookRequest.Validate() error = nil, want non-nil")
	}
}

func TestTriggerEngineRegisterRejectsInvalidWebhookSecretConfiguration(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)

	t.Run("Should reject webhook registrations without a resolver", func(t *testing.T) {
		t.Parallel()

		engine, err := NewTriggerEngine(dispatcher)
		if err != nil {
			t.Fatalf("NewTriggerEngine() error = %v", err)
		}

		err = engine.Register(
			TriggerRegistration{Trigger: testWebhookTrigger(AutomationScopeGlobal, "missing-resolver", "")},
		)
		if !errors.Is(err, ErrWebhookSecretRequired) {
			t.Fatalf("Register(missing resolver) error = %v, want ErrWebhookSecretRequired", err)
		}
	})

	t.Run("Should reject webhook registrations outside the automation vault namespace", func(t *testing.T) {
		t.Parallel()

		engine := newTestTriggerEngine(t, dispatcher)
		trigger := testWebhookTrigger(AutomationScopeGlobal, "invalid-namespace", "")
		trigger.WebhookSecretRef = "vault:bridges/brg-core/bot_token"

		err := engine.Register(TriggerRegistration{Trigger: trigger})
		if err == nil || !strings.Contains(err.Error(), "must use vault:automation/<path>") {
			t.Fatalf("Register(invalid namespace) error = %v, want automation namespace validation", err)
		}
	})
}

func TestWebhookHelpersRejectInvalidInputs(t *testing.T) {
	t.Parallel()

	if _, err := ParseWebhookEndpoint("invalid-endpoint"); !errors.Is(err, ErrWebhookEndpointInvalid) {
		t.Fatalf("ParseWebhookEndpoint(invalid) error = %v, want ErrWebhookEndpointInvalid", err)
	}
	if _, err := FormatWebhookEndpoint("", "wbh_123"); !errors.Is(err, ErrWebhookEndpointInvalid) {
		t.Fatalf("FormatWebhookEndpoint(empty slug) error = %v, want ErrWebhookEndpointInvalid", err)
	}
	if err := ValidateWebhookSignature(
		"secret",
		time.Date(2026, 4, 11, 6, 0, 0, 0, time.UTC),
		[]byte("body"),
		"invalid",
	); !errors.Is(
		err,
		ErrWebhookSignatureInvalid,
	) {
		t.Fatalf("ValidateWebhookSignature(invalid) error = %v, want ErrWebhookSignatureInvalid", err)
	}
	if err := ValidateWebhookTimestamp(time.Time{}, time.Now().UTC(), time.Minute); err == nil {
		t.Fatal("ValidateWebhookTimestamp(zero timestamp) error = nil, want non-nil")
	}
}

func TestEnvelopeFilterValueHandlesNestedMapsAndScalarKinds(t *testing.T) {
	t.Parallel()

	recordedAt := time.Date(2026, 4, 11, 6, 5, 0, 0, time.UTC)
	envelope := ActivationEnvelope{
		Kind:   "hook.review.completed",
		Scope:  AutomationScopeWorkspace,
		Source: ActivationSourceHook,
		Data: map[string]any{
			"bool":    true,
			"int":     7,
			"float":   1.5,
			"time":    recordedAt,
			"bytes":   []byte("payload"),
			"nested":  map[string]any{"leaf": "value"},
			"strings": map[string]string{"leaf": "text"},
		},
	}

	testCases := []struct {
		path   string
		want   string
		wantOK bool
	}{
		{path: "kind", want: "hook.review.completed", wantOK: true},
		{path: "scope", want: "workspace", wantOK: true},
		{path: "source", want: "hook", wantOK: true},
		{path: "data.bool", want: "true", wantOK: true},
		{path: "data.int", want: "7", wantOK: true},
		{path: "data.float", want: "1.5", wantOK: true},
		{path: "data.time", want: recordedAt.Format(time.RFC3339Nano), wantOK: true},
		{path: "data.bytes", want: "payload", wantOK: true},
		{path: "data.nested.leaf", want: "value", wantOK: true},
		{path: "data.strings.leaf", want: "text", wantOK: true},
		{path: "data.missing", wantOK: false},
		{path: "data.nested", wantOK: false},
	}

	for _, tc := range testCases {
		t.Run("Should resolve "+tc.path, func(t *testing.T) {
			t.Parallel()

			got, ok := envelopeFilterValue(envelope, tc.path)
			if ok != tc.wantOK {
				t.Fatalf("envelopeFilterValue(%q) ok = %v, want %v", tc.path, ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Fatalf("envelopeFilterValue(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestStringifyEnvelopeValueHandlesScalarKindsAndFallbacks(t *testing.T) {
	t.Parallel()

	recordedAt := time.Date(2026, 4, 11, 6, 45, 0, 123, time.UTC)
	testCases := []struct {
		name   string
		value  any
		want   string
		wantOK bool
	}{
		{name: "nil", value: nil, wantOK: false},
		{name: "string", value: "value", want: "value", wantOK: true},
		{name: "int8", value: int8(-8), want: "-8", wantOK: true},
		{name: "int16", value: int16(-16), want: "-16", wantOK: true},
		{name: "int32", value: int32(-32), want: "-32", wantOK: true},
		{name: "int64", value: int64(-64), want: "-64", wantOK: true},
		{name: "uint", value: uint(8), want: "8", wantOK: true},
		{name: "uint8", value: uint8(16), want: "16", wantOK: true},
		{name: "uint16", value: uint16(32), want: "32", wantOK: true},
		{name: "uint32", value: uint32(64), want: "64", wantOK: true},
		{name: "uint64", value: uint64(128), want: "128", wantOK: true},
		{name: "float32", value: float32(2.5), want: "2.5", wantOK: true},
		{name: "time", value: recordedAt, want: recordedAt.Format(time.RFC3339Nano), wantOK: true},
		{name: "stringer", value: testStringer("stringer-value"), want: "stringer-value", wantOK: true},
		{name: "unsupported", value: struct{}{}, wantOK: false},
	}

	for _, tc := range testCases {
		t.Run("Should stringify "+tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := stringifyEnvelopeValue(tc.value)
			if ok != tc.wantOK {
				t.Fatalf("stringifyEnvelopeValue(%s) ok = %v, want %v", tc.name, ok, tc.wantOK)
			}
			if got != tc.want {
				t.Fatalf("stringifyEnvelopeValue(%s) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}

func TestTriggerObserversHandleNilReceiversAndAgentEvents(t *testing.T) {
	t.Parallel()

	var sessionObserver *triggerSessionObserver
	sessionObserver.OnSessionCreated(testutil.Context(t), nil)
	sessionObserver.OnSessionStopped(testutil.Context(t), nil)
	sessionObserver.OnAgentEvent(testutil.Context(t), "agent.event", map[string]any{"k": "v"})

	var hookSink *triggerHookTelemetrySink
	if err := hookSink.WriteHookRecord(testutil.Context(t), "sess", hookspkg.HookRunRecord{}); err != nil {
		t.Fatalf("WriteHookRecord(nil engine) error = %v", err)
	}

	var memoryObserver *triggerMemoryObserver
	if err := memoryObserver.OnMemoryConsolidated(testutil.Context(t), MemoryConsolidatedEvent{}); err != nil {
		t.Fatalf("OnMemoryConsolidated(nil engine) error = %v", err)
	}
}

func TestTriggerEngineRejectsWebhookScopeMismatchAndDuplicateWebhookID(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)
	engine := newTestTriggerEngine(t, dispatcher)

	first := testWebhookTrigger(AutomationScopeGlobal, "first-webhook", "")
	first.WebhookID = "wbh_duplicate"
	second := testWebhookTrigger(AutomationScopeGlobal, "second-webhook", "")
	second.WebhookID = "wbh_duplicate"

	if err := engine.Register(TriggerRegistration{Trigger: first}); err != nil {
		t.Fatalf("Register(first) error = %v", err)
	}
	if err := engine.Register(
		TriggerRegistration{Trigger: second},
	); !errors.Is(
		err,
		ErrTriggerWebhookIDTaken,
	) {
		t.Fatalf("Register(duplicate webhook id) error = %v, want ErrTriggerWebhookIDTaken", err)
	}

	now := time.Date(2026, 4, 11, 6, 30, 0, 0, time.UTC)
	signature, err := SignWebhookPayload(
		testWebhookSecretValue(first.WebhookSecretRef),
		now,
		[]byte(`{"payload":"deploy"}`),
	)
	if err != nil {
		t.Fatalf("SignWebhookPayload() error = %v", err)
	}
	_, err = engine.HandleWebhook(testutil.Context(t), WebhookRequest{
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: "ws_alpha",
		Endpoint:    "deploy-review--wbh_duplicate",
		DeliveryID:  "delivery-scope-mismatch",
		Timestamp:   now,
		Signature:   signature,
		Payload:     []byte(`{"payload":"deploy"}`),
	})
	if !errors.Is(err, ErrWebhookTriggerNotRegistered) {
		t.Fatalf("HandleWebhook(scope mismatch) error = %v, want ErrWebhookTriggerNotRegistered", err)
	}
}

func newTestTriggerEngine(t *testing.T, dispatcher TriggerDispatcher, opts ...TriggerEngineOption) *TriggerEngine {
	t.Helper()

	opts = append(
		[]TriggerEngineOption{
			WithTriggerEngineWebhookSecretResolver(staticTriggerWebhookSecretResolver{}),
		},
		opts...,
	)
	engine, err := NewTriggerEngine(dispatcher, opts...)
	if err != nil {
		t.Fatalf("NewTriggerEngine() error = %v", err)
	}
	return engine
}

func testEventTrigger(scope Scope, name string, workspaceID string, event string) Trigger {
	trigger := testTrigger(scope, name, workspaceID)
	trigger.Event = event
	trigger.WebhookID = ""
	trigger.EndpointSlug = ""
	trigger.WebhookSecretRef = ""
	trigger.Prompt = `Handle {{ .Data.session_id }}`
	return trigger
}

func testWebhookTrigger(scope Scope, name string, workspaceID string) Trigger {
	trigger := testTrigger(scope, name, workspaceID)
	trigger.WebhookID = "wbh_" + name
	trigger.EndpointSlug = "deploy-review"
	trigger.WebhookSecretRef = "vault:automation/triggers/" + name + "/webhook-secret"
	return trigger
}

func testWebhookSecretValue(ref string) string {
	return "secret-for:" + strings.TrimSpace(ref)
}

type staticTriggerWebhookSecretResolver struct {
	refs map[string]string
}

func (r staticTriggerWebhookSecretResolver) ResolveRef(_ context.Context, ref string) (string, error) {
	if len(r.refs) == 0 {
		return testWebhookSecretValue(ref), nil
	}
	value, ok := r.refs[ref]
	if !ok {
		return "", fmt.Errorf("unexpected webhook secret ref %q", ref)
	}
	return value, nil
}

type stubHookSessionResolver struct {
	info *session.Info
	err  error
}

func (r stubHookSessionResolver) Status(context.Context, string) (*session.Info, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.info, nil
}

type recordingHookSessionResolver struct {
	calls int
	info  *session.Info
	err   error
}

func (r *recordingHookSessionResolver) Status(ctx context.Context, _ string) (*session.Info, error) {
	r.calls++
	if ctx == nil {
		return nil, errors.New("nil context reached hook session resolver")
	}
	if r.err != nil {
		return nil, r.err
	}
	return r.info, nil
}
