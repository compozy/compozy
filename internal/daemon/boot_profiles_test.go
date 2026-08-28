package daemon

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func TestDaemonProfileEventRecorder(t *testing.T) {
	t.Parallel()

	t.Run("Should persist deleted profile events under the permanent operator owner", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
		writer := &profileEventSummaryWriterStub{}
		recorder := &daemonProfileEventRecorder{writer: writer, now: func() time.Time { return now }}
		event := profile.Event{
			Name: "profile.deleted", ProfileID: "deleted-profile", ProfileName: "growth", OperationID: "op-delete",
		}

		recorder.RecordProfileEvent(event)

		if writer.summary.ProfileID != store.DefaultProfileID {
			t.Fatalf(
				"stored event owner = %q, want permanent operator profile %q",
				writer.summary.ProfileID,
				store.DefaultProfileID,
			)
		}
		var payload profile.Event
		if err := json.Unmarshal(writer.summary.Content, &payload); err != nil {
			t.Fatalf("decode stored profile event payload error = %v", err)
		}
		if payload.ProfileID != event.ProfileID || payload.ProfileName != event.ProfileName {
			t.Fatalf("stored profile event subject = %#v, want %#v", payload, event)
		}
	})
	t.Run("Should republish profile-owned skills after the lifecycle event is durable", func(t *testing.T) {
		t.Parallel()

		writer := &profileEventSummaryWriterStub{}
		syncCalls := 0
		state := &bootState{agentSkillResources: agentSkillPublisherFunc(func(context.Context) error {
			syncCalls++
			if writer.summary.Type == "" {
				t.Fatal("skill republication ran before the profile event was durable")
			}
			return nil
		})}
		recorder := &daemonProfileEventRecorder{writer: writer, state: state}

		recorder.RecordProfileEvent(profile.Event{
			Name: "profile.renamed", ProfileID: "profile-growth", ProfileName: "growth",
			PreviousProfileName: "marketing", OperationID: "op-rename",
		})

		if syncCalls != 1 {
			t.Fatalf("profile skill republication calls = %d, want 1", syncCalls)
		}
	})

	t.Run("Should sweep terminal runtime state when a profile is archived", func(t *testing.T) {
		t.Parallel()
		manager, err := terminalpkg.NewManager(terminalpkg.WithJournal(nativeTerminalJournalStub{}))
		if err != nil {
			t.Fatalf("terminal.NewManager() error = %v", err)
		}
		if err := manager.Start(t.Context()); err != nil {
			t.Fatalf("terminal manager Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(context.Background()); err != nil {
				t.Errorf("terminal manager Shutdown() error = %v", err)
			}
		})
		handle, err := manager.OpenPipe(t.Context(), terminalpkg.PipeRequest{
			WS: "workspace-a", Cwd: t.TempDir(), Argv: []string{"sh", "-c", "sleep 300"},
			Actor: terminalpkg.Actor{
				Kind: terminalpkg.ActorKindAgent, ID: "agent", ProfileID: "profile-a",
				SessionID: "session-a", RunID: "run-a", Generation: 1,
			},
		})
		if err != nil {
			t.Fatalf("OpenPipe() error = %v", err)
		}
		recorder := &daemonProfileEventRecorder{
			writer: &profileEventSummaryWriterStub{}, state: &bootState{terminals: manager},
		}
		recorder.RecordProfileEvent(profile.Event{
			Name:        eventspkg.ProfileArchived,
			ProfileID:   "profile-a",
			ProfileName: "marketing",
			OperationID: "op-archive",
		})
		items, err := manager.List(t.Context(), "workspace-a", store.ReadScope{ProfileID: "profile-a"})
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}
		if len(items) != 0 || handle.Info().Exit == nil {
			t.Fatalf("after archive items=%d exit=%#v, want empty and drained", len(items), handle.Info().Exit)
		}
	})
}

type profileEventSummaryWriterStub struct {
	summary store.EventSummary
}

func (s *profileEventSummaryWriterStub) WriteEventSummary(
	_ context.Context,
	summary store.EventSummary,
) error {
	s.summary = summary
	return nil
}

func (s *profileEventSummaryWriterStub) ListEventSummaries(
	context.Context,
	store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	return nil, nil
}
