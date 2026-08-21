package session

import (
	"errors"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/testutil"
)

func TestPublishOperatorNotification(t *testing.T) {
	t.Parallel()

	t.Run("Should sanitize and deliver to a live catalog subscriber", func(t *testing.T) {
		t.Parallel()
		manager, _ := newNotifyTestManager()
		events, cancel, err := manager.SubscribeSessionCatalogEvents(testutil.Context(t), CatalogScope{AllWorkspaces: true})
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()

		result, err := manager.PublishOperatorNotification(testutil.Context(t), NotifyRequest{
			SessionID: "sess-notify", WorkspaceID: "ws-notify",
			Title: "  Dependency\n audit done  ",
			Body:  "  No\t blockers. token=ghp_abcdefghijklmnopqrstuv ",
		})
		if err != nil {
			t.Fatalf("PublishOperatorNotification() error = %v", err)
		}
		if result.Outcome != NotifyOutcomeDelivered || result.RetryAfterMS != 0 {
			t.Fatalf("PublishOperatorNotification() = %#v, want delivered", result)
		}
		event := <-events
		if event.Name != CatalogEventNameOperatorNotification || event.OperatorNotification == nil {
			t.Fatalf("catalog event = %#v, want operator notification", event)
		}
		if event.OperatorNotification.Title != "Dependency audit done" ||
			event.OperatorNotification.Body != "No blockers. token=[REDACTED]" {
			t.Fatalf("notification = %#v, want sanitized text", event.OperatorNotification)
		}
	})

	t.Run("Should reject a title above the post-sanitize cap without publishing", func(t *testing.T) {
		t.Parallel()
		manager, _ := newNotifyTestManager()
		events, cancel, err := manager.SubscribeSessionCatalogEvents(testutil.Context(t), CatalogScope{AllWorkspaces: true})
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()
		_, err = manager.PublishOperatorNotification(testutil.Context(t), NotifyRequest{
			SessionID: "sess-cap", WorkspaceID: "ws-cap", Title: strings.Repeat("x", 81),
		})
		if !errors.Is(err, ErrInvalidNotification) || !strings.Contains(err.Error(), "80") {
			t.Fatalf("PublishOperatorNotification() error = %v, want title cap", err)
		}
		select {
		case event := <-events:
			t.Fatalf("unexpected catalog event = %#v", event)
		default:
		}
	})

	t.Run("Should rate limit the second send from one session", func(t *testing.T) {
		t.Parallel()
		manager, advance := newNotifyTestManager()
		first, err := manager.PublishOperatorNotification(testutil.Context(t), NotifyRequest{
			SessionID: "sess-rate", WorkspaceID: "ws-rate", Title: "First",
		})
		if err != nil || first.Outcome != NotifyOutcomeNoClient {
			t.Fatalf("first notification = %#v, error = %v", first, err)
		}
		advance(100 * time.Millisecond)
		second, err := manager.PublishOperatorNotification(testutil.Context(t), NotifyRequest{
			SessionID: "sess-rate", WorkspaceID: "ws-rate", Title: "Second",
		})
		if err != nil {
			t.Fatalf("second notification error = %v", err)
		}
		if second.Outcome != NotifyOutcomeRateLimited || second.RetryAfterMS != 900 {
			t.Fatalf("second notification = %#v, want rate-limited/900ms", second)
		}
	})

	t.Run("Should reject an empty sanitized title", func(t *testing.T) {
		t.Parallel()
		manager, _ := newNotifyTestManager()
		_, err := manager.PublishOperatorNotification(testutil.Context(t), NotifyRequest{
			SessionID: "sess-empty", WorkspaceID: "ws-empty", Title: " \n\t ",
		})
		if !errors.Is(err, ErrInvalidNotification) || !strings.Contains(err.Error(), "title is required") {
			t.Fatalf("PublishOperatorNotification() error = %v, want required title", err)
		}
	})

	t.Run("Should suppress muted workspaces while retaining the rate counter", func(t *testing.T) {
		t.Parallel()
		manager, advance := newNotifyTestManager()
		const workspaceID = "ws_0123456789abcdef"
		if err := manager.SetAttentionConfig(compozyconfig.AttentionConfig{
			Toasts: true, Sound: true, MutedWorkspaces: []string{workspaceID},
		}); err != nil {
			t.Fatalf("SetAttentionConfig() error = %v", err)
		}
		events, cancel, err := manager.SubscribeSessionCatalogEvents(testutil.Context(t), CatalogScope{AllWorkspaces: true})
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()
		first, err := manager.PublishOperatorNotification(testutil.Context(t), NotifyRequest{
			SessionID: "sess-muted", WorkspaceID: workspaceID, Title: "Muted",
		})
		if err != nil || first.Outcome != NotifyOutcomeMutedWorkspace {
			t.Fatalf("muted notification = %#v, error = %v", first, err)
		}
		advance(100 * time.Millisecond)
		second, err := manager.PublishOperatorNotification(testutil.Context(t), NotifyRequest{
			SessionID: "sess-muted", WorkspaceID: workspaceID, Title: "Still muted",
		})
		if err != nil || second.Outcome != NotifyOutcomeRateLimited {
			t.Fatalf("second muted notification = %#v, error = %v", second, err)
		}
		select {
		case event := <-events:
			t.Fatalf("unexpected muted catalog event = %#v", event)
		default:
		}
	})
}

func newNotifyTestManager() (*Manager, func(time.Duration)) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	manager := &Manager{
		catalogEvents:       newSessionCatalogBroadcaster(),
		notifyConfig:        compozyconfig.DefaultAttentionConfig(),
		notifyLastBySession: make(map[string]time.Time),
		newNotificationID:   func() (string, error) { return "ntf_test", nil },
		now:                 func() time.Time { return now },
	}
	return manager, func(delta time.Duration) { now = now.Add(delta) }
}
