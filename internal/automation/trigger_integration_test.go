//go:build integration

package automation

import (
	"testing"
	"time"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/testutil"
)

func TestTriggerEngineIntegrationSessionStoppedViaObserverBoundaryDispatchesOneRun(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openAutomationIntegrationDB(t, ctx)
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, db)
	engine := newTestTriggerEngine(t, dispatcher)

	triggerDef := testEventTrigger(AutomationScopeGlobal, "session-stopped-integration", "", "session.stopped")
	triggerDef.Filter = map[string]string{
		"data.agent_name": "researcher",
	}
	trigger, err := db.CreateTrigger(ctx, triggerDef)
	if err != nil {
		t.Fatalf("CreateTrigger() error = %v", err)
	}
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	engine.SessionObserver().OnSessionStopped(ctx, &session.Session{
		ID:        "sess-stop-source",
		Name:      "source-session",
		AgentName: "researcher",
		Type:      session.SessionTypeUser,
		State:     session.StateStopped,
		CreatedAt: time.Date(2026, 4, 11, 3, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 4, 11, 3, 5, 0, 0, time.UTC),
	})

	if got, want := len(creator.createCalls()), 1; got != want {
		t.Fatalf("len(Create calls) = %d, want %d", got, want)
	}
	if got, want := len(creator.promptCalls()), 1; got != want {
		t.Fatalf("len(Prompt calls) = %d, want %d", got, want)
	}
	promptCalls := creator.promptCalls()
	if got, want := promptCalls[0].message, "Handle sess-stop-source"; got != want {
		t.Fatalf("Prompt().message = %q, want %q", got, want)
	}

	runs, err := db.ListRuns(ctx, RunQuery{TriggerID: trigger.ID})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
	if got, want := runs[0].Status, RunCompleted; got != want {
		t.Fatalf("runs[0].Status = %q, want %q", got, want)
	}
}

func TestTriggerEngineIntegrationMemoryConsolidatedDispatchesOneRun(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openAutomationIntegrationDB(t, ctx)
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, db)
	engine := newTestTriggerEngine(t, dispatcher)

	triggerDef := testEventTrigger(AutomationScopeGlobal, "memory-consolidated", "", "memory.consolidated")
	triggerDef.Prompt = `Digest {{ .Data.summary }}`
	trigger, err := db.CreateTrigger(ctx, triggerDef)
	if err != nil {
		t.Fatalf("CreateTrigger() error = %v", err)
	}
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	if err := engine.MemoryObserver().OnMemoryConsolidated(ctx, MemoryConsolidatedEvent{
		Timestamp: time.Date(2026, 4, 11, 4, 0, 0, 0, time.UTC),
		Data: map[string]any{
			"summary": "fresh context",
		},
	}); err != nil {
		t.Fatalf("OnMemoryConsolidated() error = %v", err)
	}

	if got, want := len(creator.createCalls()), 1; got != want {
		t.Fatalf("len(Create calls) = %d, want %d", got, want)
	}
	promptCalls := creator.promptCalls()
	if got, want := len(promptCalls), 1; got != want {
		t.Fatalf("len(Prompt calls) = %d, want %d", got, want)
	}
	if got, want := promptCalls[0].message, "Digest fresh context"; got != want {
		t.Fatalf("Prompt().message = %q, want %q", got, want)
	}

	runs, err := db.ListRuns(ctx, RunQuery{TriggerID: trigger.ID})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
	if got, want := runs[0].Status, RunCompleted; got != want {
		t.Fatalf("runs[0].Status = %q, want %q", got, want)
	}
}

func TestTriggerEngineIntegrationWebhookDispatchesExactlyOnce(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db := openAutomationIntegrationDB(t, ctx)
	creator := newRecordingSessionCreator()
	now := time.Date(2026, 4, 11, 4, 30, 0, 0, time.UTC)
	dispatcher := newTestDispatcher(t, creator, db, WithDispatcherNow(func() time.Time { return now }))
	engine := newTestTriggerEngine(t, dispatcher, WithTriggerEngineNow(func() time.Time { return now }))

	triggerDef := testWebhookTrigger(AutomationScopeGlobal, "webhook-dispatch", "")
	trigger, err := db.CreateTrigger(ctx, triggerDef)
	if err != nil {
		t.Fatalf("CreateTrigger() error = %v", err)
	}
	if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	payload := []byte(`{"payload":"deploy"}`)
	signature, err := SignWebhookPayload(testWebhookSecretValue(trigger.WebhookSecretRef), now, payload)
	if err != nil {
		t.Fatalf("SignWebhookPayload() error = %v", err)
	}

	result, err := engine.HandleWebhook(ctx, WebhookRequest{
		Scope:      AutomationScopeGlobal,
		Endpoint:   "deploy-review--" + trigger.WebhookID,
		DeliveryID: "delivery-1",
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
	if got, want := len(result.Runs), 1; got != want {
		t.Fatalf("len(result.Runs) = %d, want %d", got, want)
	}
	if got, want := len(creator.createCalls()), 1; got != want {
		t.Fatalf("len(Create calls) = %d, want %d", got, want)
	}
	promptCalls := creator.promptCalls()
	if got, want := len(promptCalls), 1; got != want {
		t.Fatalf("len(Prompt calls) = %d, want %d", got, want)
	}
	if got, want := promptCalls[0].message, "Review payload deploy"; got != want {
		t.Fatalf("Prompt().message = %q, want %q", got, want)
	}

	runs, err := db.ListRuns(ctx, RunQuery{TriggerID: trigger.ID})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
	if got, want := runs[0].Status, RunCompleted; got != want {
		t.Fatalf("runs[0].Status = %q, want %q", got, want)
	}
}
