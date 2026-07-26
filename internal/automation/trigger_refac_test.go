package automation

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/testutil"
)

type mutatingTriggerFilterDispatcher struct{}

func (mutatingTriggerFilterDispatcher) Dispatch(_ context.Context, req DispatchRequest) (*Run, error) {
	if req.Trigger != nil && req.Trigger.Filter != nil {
		req.Trigger.Filter["data.agent_name"] = "mutated"
	}
	return nil, nil
}

type mutatingNestedEnvelopeDispatcher struct {
	mu   sync.Mutex
	seen []string
}

func (d *mutatingNestedEnvelopeDispatcher) Dispatch(_ context.Context, req DispatchRequest) (*Run, error) {
	if req.Envelope == nil {
		return nil, errors.New("test dispatcher: envelope is required")
	}
	metadata, ok := req.Envelope.Data["metadata"].(map[string]any)
	if !ok {
		return nil, errors.New("test dispatcher: metadata map is required")
	}
	repo, ok := metadata["repo"].(string)
	if !ok {
		return nil, errors.New("test dispatcher: metadata.repo is required")
	}

	d.mu.Lock()
	callIndex := len(d.seen)
	d.seen = append(d.seen, repo)
	d.mu.Unlock()

	if callIndex == 0 {
		metadata["repo"] = "mutated"
	}

	completedAt := time.Date(2026, 4, 12, 9, 0, 0, 0, time.UTC)
	return &Run{
		ID:        "run-" + req.Trigger.ID,
		TriggerID: req.Trigger.ID,
		Status:    RunCompleted,
		Attempt:   1,
		StartedAt: &completedAt,
		EndedAt:   &completedAt,
	}, nil
}

func (d *mutatingNestedEnvelopeDispatcher) seenRepos() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]string(nil), d.seen...)
}

func TestTriggerFilterPathMatching(t *testing.T) {
	t.Parallel()

	envelope := ActivationEnvelope{
		Kind:        "session.stopped",
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: "ws_alpha",
		Source:      ActivationSourceObserver,
		Data: map[string]any{
			"agent_name": "researcher",
			"metadata": map[string]any{
				"step": "complete",
			},
			"labels": map[string]string{
				"priority": "high",
			},
		},
	}

	tests := []struct {
		name   string
		filter map[string]string
		want   bool
	}{
		{
			name: "Should match nested data while trimming path segments",
			filter: map[string]string{
				" data.metadata. step ": "complete",
			},
			want: true,
		},
		{
			name: "Should match nested string maps",
			filter: map[string]string{
				"data.labels.priority": "high",
			},
			want: true,
		},
		{
			name: "Should reject empty nested path segments",
			filter: map[string]string{
				"data.metadata..step": "complete",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := exactFilterMatch(tt.filter, envelope); got != tt.want {
				t.Fatalf("exactFilterMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTriggerDispatchSnapshotIsolation(t *testing.T) {
	t.Parallel()

	t.Run("Should keep registered filter isolated from dispatcher mutations", func(t *testing.T) {
		t.Parallel()

		engine, err := NewTriggerEngine(mutatingTriggerFilterDispatcher{})
		if err != nil {
			t.Fatalf("NewTriggerEngine() error = %v", err)
		}
		trigger := testEventTrigger(AutomationScopeWorkspace, "snapshot-isolation", "ws_alpha", "session.stopped")
		trigger.Filter = map[string]string{
			"data.agent_name": "researcher",
		}
		if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		envelope := ActivationEnvelope{
			Kind:        "session.stopped",
			Scope:       AutomationScopeWorkspace,
			WorkspaceID: "ws_alpha",
			Source:      ActivationSourceObserver,
			Data: map[string]any{
				"agent_name": "researcher",
			},
		}
		for i := range 2 {
			result, err := engine.Fire(testutil.Context(t), envelope)
			if err != nil {
				t.Fatalf("Fire(%d) error = %v", i, err)
			}
			if got, want := result.Matched, 1; got != want {
				t.Fatalf("Fire(%d).Matched = %d, want %d", i, got, want)
			}
		}
	})

	t.Run("Should keep nested activation data isolated across sibling trigger dispatches", func(t *testing.T) {
		t.Parallel()

		dispatcher := &mutatingNestedEnvelopeDispatcher{}
		engine, err := NewTriggerEngine(dispatcher)
		if err != nil {
			t.Fatalf("NewTriggerEngine() error = %v", err)
		}

		first := testEventTrigger(AutomationScopeWorkspace, "nested-first", "ws_alpha", "ext.github.push")
		second := testEventTrigger(AutomationScopeWorkspace, "nested-second", "ws_alpha", "ext.github.push")
		if err := engine.Register(TriggerRegistration{Trigger: first}); err != nil {
			t.Fatalf("Register(first) error = %v", err)
		}
		if err := engine.Register(TriggerRegistration{Trigger: second}); err != nil {
			t.Fatalf("Register(second) error = %v", err)
		}

		envelope := ActivationEnvelope{
			Kind:        "ext.github.push",
			Scope:       AutomationScopeWorkspace,
			WorkspaceID: "ws_alpha",
			Source:      ActivationSourceExtension,
			Data: map[string]any{
				"metadata": map[string]any{
					"repo": "acme/api",
				},
			},
		}
		result, err := engine.Fire(testutil.Context(t), envelope)
		if err != nil {
			t.Fatalf("Fire() error = %v", err)
		}
		if got, want := result.Matched, 2; got != want {
			t.Fatalf("result.Matched = %d, want %d", got, want)
		}
		repos := dispatcher.seenRepos()
		if len(repos) != 2 {
			t.Fatalf("len(seen repos) = %d, want 2", len(repos))
		}
		for idx, got := range repos {
			if want := "acme/api"; got != want {
				t.Fatalf("seen repo[%d] = %q, want %q", idx, got, want)
			}
		}
	})

	t.Run("Should keep trigger pre-fire hook payload mutations out of dispatch envelope", func(t *testing.T) {
		t.Parallel()

		store := newMemoryRunStore()
		creator := newRecordingSessionCreator()
		hooks := &recordingAutomationHooks{
			onTriggerPreFire: func(
				_ context.Context,
				payload hookspkg.AutomationTriggerPreFirePayload,
			) (hookspkg.AutomationTriggerPreFirePayload, error) {
				metadata, ok := payload.Payload["metadata"].(map[string]any)
				if !ok {
					t.Fatal("trigger pre-fire payload metadata = missing, want map")
				}
				metadata["repo"] = "mutated"
				return payload, nil
			},
		}
		dispatcher := newTestDispatcher(t, creator, store, WithDispatcherHooks(hooks))

		trigger := testEventTrigger(AutomationScopeWorkspace, "hook-payload", "ws_alpha", "ext.github.push")
		trigger.Prompt = `Review repo {{ index .Data "metadata" "repo" }}`
		envelope := ActivationEnvelope{
			Kind:        "ext.github.push",
			Scope:       AutomationScopeWorkspace,
			WorkspaceID: "ws_alpha",
			Source:      ActivationSourceExtension,
			Data: map[string]any{
				"metadata": map[string]any{
					"repo": "acme/api",
				},
			},
		}
		run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
			Kind:     DispatchKindExtension,
			Trigger:  &trigger,
			Envelope: &envelope,
		})
		if err != nil {
			t.Fatalf("Dispatch() error = %v", err)
		}
		if run.Status != RunCompleted {
			t.Fatalf("run.Status = %q, want %q", run.Status, RunCompleted)
		}
		metadata, ok := envelope.Data["metadata"].(map[string]any)
		if !ok {
			t.Fatal("dispatch envelope metadata = missing, want map")
		}
		if got, want := metadata["repo"], "acme/api"; got != want {
			t.Fatalf("dispatch envelope metadata.repo = %#v, want %q", got, want)
		}
	})
}

func TestTriggerWebhookRouting(t *testing.T) {
	t.Parallel()

	t.Run("Should reject endpoints whose slug does not match the registered webhook trigger", func(t *testing.T) {
		t.Parallel()

		store := newMemoryRunStore()
		creator := newRecordingSessionCreator()
		current := time.Date(2026, 4, 12, 10, 0, 0, 0, time.UTC)
		dispatcher := newTestDispatcher(t, creator, store, WithDispatcherNow(func() time.Time { return current }))
		engine := newTestTriggerEngine(t, dispatcher, WithTriggerEngineNow(func() time.Time { return current }))
		trigger := testWebhookTrigger(AutomationScopeGlobal, "slug-mismatch", "")
		if err := engine.Register(TriggerRegistration{Trigger: trigger}); err != nil {
			t.Fatalf("Register() error = %v", err)
		}

		payload := []byte(`{"payload":"deploy"}`)
		signature, err := SignWebhookPayload(testWebhookSecretValue(trigger.WebhookSecretRef), current, payload)
		if err != nil {
			t.Fatalf("SignWebhookPayload() error = %v", err)
		}

		result, err := engine.HandleWebhook(testutil.Context(t), WebhookRequest{
			Scope:      AutomationScopeGlobal,
			Endpoint:   "bogus--" + trigger.WebhookID,
			DeliveryID: "delivery-slug-mismatch",
			Timestamp:  current,
			Signature:  signature,
			Payload:    payload,
			Data: map[string]any{
				"payload": "deploy",
			},
		})
		if !errors.Is(err, ErrWebhookTriggerNotRegistered) {
			t.Fatalf("HandleWebhook(slug mismatch) error = %v, want ErrWebhookTriggerNotRegistered", err)
		}
		if got := result.Matched; got != 0 {
			t.Fatalf("result.Matched = %d, want 0", got)
		}
		if got := len(creator.promptCalls()); got != 0 {
			t.Fatalf("len(prompt calls) = %d, want 0", got)
		}
	})
}
