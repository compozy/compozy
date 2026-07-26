package automation

import (
	"errors"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/testutil"
)

func TestManagerContractWebhookReplay(t *testing.T) {
	t.Run("Should reject replayed webhook deliveries after trigger engine restart", func(t *testing.T) {
		// not parallel: t.Setenv updates process-wide webhook secret lookup state.
		h := newManagerHarness(t)
		t.Setenv("AGH_TEST_WEBHOOK_SECRET", "super-secret")
		current := time.Date(2026, 4, 12, 11, 0, 0, 0, time.UTC)
		cfg := aghconfig.AutomationConfig{
			Enabled:           true,
			Timezone:          DefaultTimezone,
			MaxConcurrentJobs: DefaultMaxConcurrentJobs,
			DefaultFireLimit:  DefaultFireLimitConfig(),
			Triggers: []aghconfig.AutomationTrigger{
				func() aghconfig.AutomationTrigger {
					trigger := managerConfigTrigger(
						AutomationScopeWorkspace,
						"webhook-replay-restart",
						h.workspaceRoot,
						"webhook",
					)
					trigger.EndpointSlug = "deploy-review"
					trigger.WebhookSecretRef = "env:AGH_TEST_WEBHOOK_SECRET"
					trigger.Prompt = `Review payload {{ index .Data "payload" }}`
					trigger.Filter = map[string]string{"data.payload": "deploy"}
					return trigger
				}(),
			},
		}
		firstManager := h.newManager(
			t,
			cfg,
			WithTriggerEngineOptions(
				WithTriggerEngineNow(func() time.Time { return current }),
				WithTriggerEngineWebhookFreshnessWindow(5*time.Minute),
			),
			WithDispatcherOptions(WithDispatcherNow(func() time.Time { return current })),
		)
		if err := firstManager.Start(h.ctx); err != nil {
			t.Fatalf("first manager Start() error = %v", err)
		}

		trigger, err := firstManager.resolveConfigTrigger(h.ctx, cfg.Triggers[0])
		if err != nil {
			t.Fatalf("first manager resolveConfigTrigger() error = %v", err)
		}
		endpoint, err := FormatWebhookEndpoint(trigger.EndpointSlug, trigger.WebhookID)
		if err != nil {
			t.Fatalf("FormatWebhookEndpoint() error = %v", err)
		}
		payload := []byte(`{"payload":"deploy"}`)
		signature, err := SignWebhookPayload("super-secret", current, payload)
		if err != nil {
			t.Fatalf("SignWebhookPayload() error = %v", err)
		}

		firstResult, err := firstManager.HandleWebhook(h.ctx, WebhookRequest{
			Scope:       AutomationScopeWorkspace,
			WorkspaceID: h.workspace.ID,
			Endpoint:    endpoint,
			DeliveryID:  "delivery-restart-replay",
			Timestamp:   current,
			Signature:   signature,
			Payload:     payload,
			Data: map[string]any{
				"payload": "deploy",
			},
		})
		if err != nil {
			t.Fatalf("first manager HandleWebhook() error = %v", err)
		}
		if got, want := firstResult.Matched, 1; got != want {
			t.Fatalf("first result Matched = %d, want %d", got, want)
		}
		if err := firstManager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("first manager Shutdown() error = %v", err)
		}

		secondManager := h.newManager(
			t,
			cfg,
			WithTriggerEngineOptions(
				WithTriggerEngineNow(func() time.Time { return current }),
				WithTriggerEngineWebhookFreshnessWindow(5*time.Minute),
			),
			WithDispatcherOptions(WithDispatcherNow(func() time.Time { return current })),
		)
		if err := secondManager.Start(h.ctx); err != nil {
			t.Fatalf("second manager Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := secondManager.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("second manager Shutdown() error = %v", err)
			}
		})

		secondResult, err := secondManager.HandleWebhook(h.ctx, WebhookRequest{
			Scope:       AutomationScopeWorkspace,
			WorkspaceID: h.workspace.ID,
			Endpoint:    endpoint,
			DeliveryID:  "delivery-restart-replay",
			Timestamp:   current,
			Signature:   signature,
			Payload:     payload,
			Data: map[string]any{
				"payload": "deploy",
			},
		})
		if !errors.Is(err, ErrWebhookReplayDetected) {
			t.Fatalf("second manager HandleWebhook() error = %v, want ErrWebhookReplayDetected", err)
		}
		if got := secondResult.Matched; got != 0 {
			t.Fatalf("second result Matched = %d, want 0", got)
		}
		if got, want := h.sessions.promptCount(), 1; got != want {
			t.Fatalf("Prompt() call count = %d, want %d", got, want)
		}
	})
}
