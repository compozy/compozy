package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

func TestPublishOperatorNotification(t *testing.T) {
	t.Parallel()

	t.Run("Should sanitize and deliver to a live catalog subscriber", func(t *testing.T) {
		t.Parallel()
		manager, _ := newNotifyTestManager()
		events, cancel, err := manager.SubscribeSessionCatalogEvents(
			testutil.Context(t),
			CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true},
		)
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()

		result, err := manager.PublishOperatorNotification(
			testutil.Context(t),
			NotifyRequest{ProfileID: store.DefaultProfileID,
				SessionID: "sess-notify", WorkspaceID: "ws-notify",
				Title: "  Dependency\n audit done  ",
				Body:  "  No\t blockers. token=ghp_abcdefghijklmnopqrstuv ",
			},
		)
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
		events, cancel, err := manager.SubscribeSessionCatalogEvents(
			testutil.Context(t),
			CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true},
		)
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()
		_, err = manager.PublishOperatorNotification(
			testutil.Context(t),
			NotifyRequest{ProfileID: store.DefaultProfileID,
				SessionID: "sess-cap", WorkspaceID: "ws-cap", Title: strings.Repeat("x", 81),
			},
		)
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
		first, err := manager.PublishOperatorNotification(
			testutil.Context(t),
			NotifyRequest{ProfileID: store.DefaultProfileID,
				SessionID: "sess-rate", WorkspaceID: "ws-rate", Title: "First",
			},
		)
		if err != nil || first.Outcome != NotifyOutcomeNoClient {
			t.Fatalf("first notification = %#v, error = %v", first, err)
		}
		advance(100 * time.Millisecond)
		second, err := manager.PublishOperatorNotification(
			testutil.Context(t),
			NotifyRequest{ProfileID: store.DefaultProfileID,
				SessionID: "sess-rate", WorkspaceID: "ws-rate", Title: "Second",
			},
		)
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
		_, err := manager.PublishOperatorNotification(
			testutil.Context(t),
			NotifyRequest{ProfileID: store.DefaultProfileID,
				SessionID: "sess-empty", WorkspaceID: "ws-empty", Title: " \n\t ",
			},
		)
		if !errors.Is(err, ErrInvalidNotification) || !strings.Contains(err.Error(), "title is required") {
			t.Fatalf("PublishOperatorNotification() error = %v, want required title", err)
		}
	})

	t.Run("Should suppress muted workspaces while retaining the rate counter", func(t *testing.T) {
		t.Parallel()
		manager, advance := newNotifyTestManager()
		const workspaceID = "ws_0123456789abcdef"
		manager.attentionWorkspaceMutes = notifyMuteReader{
			profileID:   store.DefaultProfileID,
			workspaceID: workspaceID,
			muted:       true,
		}
		events, cancel, err := manager.SubscribeSessionCatalogEvents(
			testutil.Context(t),
			CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true},
		)
		if err != nil {
			t.Fatalf("SubscribeSessionCatalogEvents() error = %v", err)
		}
		defer cancel()
		first, err := manager.PublishOperatorNotification(
			testutil.Context(t),
			NotifyRequest{ProfileID: store.DefaultProfileID,
				SessionID: "sess-muted", WorkspaceID: workspaceID, Title: "Muted",
			},
		)
		if err != nil || first.Outcome != NotifyOutcomeMutedWorkspace {
			t.Fatalf("muted notification = %#v, error = %v", first, err)
		}
		advance(100 * time.Millisecond)
		second, err := manager.PublishOperatorNotification(
			testutil.Context(t),
			NotifyRequest{ProfileID: store.DefaultProfileID,
				SessionID: "sess-muted", WorkspaceID: workspaceID, Title: "Still muted",
			},
		)
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
		catalogEvents:           newSessionCatalogBroadcaster(),
		notifyConfig:            compozyconfig.DefaultAttentionConfig(),
		attentionWorkspaceMutes: notifyMuteReader{},
		notifyLastBySession:     make(map[string]time.Time),
		newNotificationID:       func() (string, error) { return "ntf_test", nil },
		now:                     func() time.Time { return now },
	}
	return manager, func(delta time.Duration) { now = now.Add(delta) }
}

type notifyMuteReader struct {
	profileID   string
	workspaceID string
	muted       bool
}

func (r notifyMuteReader) IsAttentionWorkspaceMuted(
	_ context.Context,
	profileID string,
	workspaceID string,
) (bool, error) {
	return r.muted && profileID == r.profileID && workspaceID == r.workspaceID, nil
}

func TestApprovalExpiryNotification(t *testing.T) {
	t.Parallel()
	for _, muted := range []bool{false, true} {
		name := "Should publish every expiry with action and resource"
		if muted {
			name = "Should respect workspace mutes on expiry"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			manager, _ := newNotifyTestManager()
			manager.attentionWorkspaceMutes = notifyMuteReader{
				profileID:   store.DefaultProfileID,
				workspaceID: "ws-expiry",
				muted:       muted,
			}
			events, cancel, err := manager.SubscribeSessionCatalogEvents(t.Context(),
				CatalogScope{ReadScope: store.ReadScope{AllProfiles: true}, AllWorkspaces: true})
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			current := &Session{ID: "expiry", ProfileID: store.DefaultProfileID, WorkspaceID: "ws-expiry"}
			event := (acp.AgentEvent{Type: acp.EventTypePermission, Action: "edit", Resource: "docs/guide.md",
				Decision: "reject_once"}).WithResolvedBy("timeout")
			for range 2 {
				manager.notifyManagedPromptEvent(t.Context(), current, nil, event)
				select {
				case received := <-events:
					if muted {
						t.Fatalf("muted expiry published: %+v", received)
					}
					notification := received.OperatorNotification
					if notification == nil || notification.Kind != "approval_expired" ||
						notification.Title != "Approval expired" || notification.Action != "edit" ||
						notification.Resource != "docs/guide.md" {
						t.Fatalf("expiry notification=%+v", notification)
					}
				default:
					if !muted {
						t.Fatal("expiry notification missing")
					}
				}
			}
		})
	}
}

func TestUserScopeSourceNotification(t *testing.T) {
	t.Run("Should allow user scoped source approval notices", func(t *testing.T) {
		t.Parallel()
		manager, _ := newNotifyTestManager()
		notice, err := manager.sanitizedOperatorNotification(
			NotifyRequest{
				Kind:      "heartbeat_source_unapproved",
				SessionID: "sess-source",
				ProfileID: "default",
				Title:     "Check-in script needs approval",
			},
		)
		if err != nil || notice.WorkspaceID != "" || notice.Kind != "heartbeat_source_unapproved" {
			t.Fatalf("notice=%+v err=%v", notice, err)
		}
	})
}
