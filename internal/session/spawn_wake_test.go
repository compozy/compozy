package session

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil"
)

type recordingSpawnWakeNotifier struct {
	mu      sync.Mutex
	events  []SpawnWakeEvent
	parents []string
	err     error
}

func (notifier *recordingSpawnWakeNotifier) WakeSpawnCreator(
	_ context.Context,
	creatorSessionID string,
	event SpawnWakeEvent,
) error {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	notifier.parents = append(notifier.parents, creatorSessionID)
	notifier.events = append(notifier.events, event)
	return notifier.err
}

func (notifier *recordingSpawnWakeNotifier) calls() ([]string, []SpawnWakeEvent) {
	notifier.mu.Lock()
	defer notifier.mu.Unlock()
	return append([]string(nil), notifier.parents...), append([]SpawnWakeEvent(nil), notifier.events...)
}

func TestSpawnWakeEventContract(t *testing.T) {
	t.Parallel()

	t.Run("Should redact and bound wake detail before validation", func(t *testing.T) {
		t.Parallel()

		rawToken := "compozy_claim_secret-value"
		event := SpawnWakeEvent{
			ChildSessionID: " child-1 ",
			ChildAgentName: " researcher ",
			Reason:         " NEEDS_ATTENTION ",
			Badge:          " WAITING-FOR-INPUT ",
			Detail:         rawToken + strings.Repeat("界", 300),
			WakeEventID:    " wake-1 ",
		}.Normalize()

		if err := event.Validate(); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if strings.Contains(event.Detail, rawToken) || !strings.Contains(event.Detail, "compozy_claim_[REDACTED]") {
			t.Fatalf("Normalize().Detail = %q, want claim token redacted", event.Detail)
		}
		if got := len([]rune(event.Detail)); got > spawnWakeTextMaxRunes {
			t.Fatalf("wake detail runes = %d, want <= %d", got, spawnWakeTextMaxRunes)
		}
	})

	t.Run("Should reject a badge and reason mismatch", func(t *testing.T) {
		t.Parallel()

		err := (SpawnWakeEvent{
			ChildSessionID: "child-1",
			Reason:         SpawnWakeReasonStopped,
			Badge:          BadgeFailed,
			WakeEventID:    "wake-1",
		}).Validate()
		if !errors.Is(err, ErrSpawnWakeValidation) {
			t.Fatalf("Validate() error = %v, want ErrSpawnWakeValidation", err)
		}
	})
}

func TestManagerDispatchSpawnWake(t *testing.T) {
	t.Parallel()

	t.Run("Should deliver one wake per cause episode", func(t *testing.T) {
		t.Parallel()

		notifier := &recordingSpawnWakeNotifier{}
		manager := spawnWakeTestManager(notifier, nil)
		info := spawnWakeTestInfo(true)

		manager.dispatchSpawnWake(testutil.Context(t), info, BadgeWaitingForInput)
		manager.dispatchSpawnWake(testutil.Context(t), info, BadgeWaitingForInput)

		parents, events := notifier.calls()
		if got, want := len(events), 1; got != want {
			t.Fatalf("wake calls = %d, want %d", got, want)
		}
		if parents[0] != "parent-1" || events[0].ChildSessionID != "child-1" ||
			events[0].Reason != SpawnWakeReasonNeedsAttention || events[0].Badge != BadgeWaitingForInput {
			t.Fatalf("wake delivery = parents %#v events %#v, want canonical child wake", parents, events)
		}
		if !strings.Contains(events[0].Detail, "Include vendored packages?") {
			t.Fatalf("wake detail = %q, want pending interaction title", events[0].Detail)
		}
	})

	tests := []struct {
		name        string
		notify      bool
		parentID    string
		notifierErr error
		wantReason  string
	}{
		{name: "Should audit creator opt out", notify: false, parentID: "parent-1", wantReason: "wake_creator_disabled"},
		{name: "Should audit self wake", notify: true, parentID: "child-1", wantReason: "self_wake"},
		{
			name: "Should audit a creator that is not live", notify: true, parentID: "parent-1",
			notifierErr: ErrSpawnWakeSessionNotLive, wantReason: "session_not_live",
		},
		{
			name: "Should audit delivery failure", notify: true, parentID: "parent-1",
			notifierErr: errors.New("provider rejected compozy_claim_private-token"), wantReason: "delivery_failed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var logs bytes.Buffer
			notifier := &recordingSpawnWakeNotifier{err: test.notifierErr}
			manager := spawnWakeTestManager(notifier, &logs)
			info := spawnWakeTestInfo(test.notify)
			info.Lineage.ParentSessionID = test.parentID

			manager.dispatchSpawnWake(testutil.Context(t), info, BadgeWaitingForInput)

			if got := logs.String(); !strings.Contains(got, "session.spawn_wake.suppressed") ||
				!strings.Contains(got, "suppression_reason="+test.wantReason) {
				t.Fatalf("wake audit log = %q, want suppression %q", got, test.wantReason)
			} else if strings.Contains(got, "compozy_claim_private-token") {
				t.Fatalf("wake audit log leaked claim token: %q", got)
			}
		})
	}
}

func spawnWakeTestManager(notifier SpawnWakeNotifier, logs *bytes.Buffer) *Manager {
	if logs == nil {
		logs = &bytes.Buffer{}
	}
	return &Manager{
		logger:              slog.New(slog.NewTextHandler(logs, nil)),
		spawnWakeNotifier:   notifier,
		spawnWakeEventIDs:   make(map[string]struct{}),
		spawnWakeEventOrder: make([]string, 0),
	}
}

func spawnWakeTestInfo(notify bool) *Info {
	return &Info{
		ID:                "child-1",
		AgentName:         "researcher",
		WorkspaceID:       "workspace-1",
		TranscriptEpoch:   3,
		AttentionRevision: 7,
		Lineage: &store.SessionLineage{
			ParentSessionID: "parent-1",
			RootSessionID:   "parent-1",
			SpawnDepth:      1,
			NotifyCreator:   notify,
		},
		PendingInteractions: []store.PendingInteraction{{
			InteractionID: "interaction-1",
			Title:         "Include vendored packages?",
		}},
	}
}
