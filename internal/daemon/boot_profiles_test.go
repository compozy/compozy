package daemon

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	"github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/store"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

func TestDaemonProfileEventRecorder(t *testing.T) {
	t.Parallel()

	t.Run("Should retain an archive event after its subject becomes unavailable", func(t *testing.T) {
		t.Parallel()
		database := openDaemonTestGlobalDB(t)
		home, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		now := time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)
		manager, err := profile.NewManager(
			profile.WithStore(database), profile.WithHomePaths(home),
			profile.WithEventRecorder(&daemonProfileEventRecorder{
				writer: database, now: func() time.Time { return now },
			}),
		)
		if err != nil {
			t.Fatalf("profile.NewManager() error = %v", err)
		}
		created, err := manager.Create(t.Context(), profile.CreateInput{Name: "delivery-review"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		plan, err := manager.PrepareArchive(t.Context(), created.Name)
		if err != nil {
			t.Fatalf("PrepareArchive() error = %v", err)
		}
		if _, err := manager.Archive(t.Context(), created.Name, plan.Revision); err != nil {
			t.Fatalf("Archive() error = %v", err)
		}
		entries, err := database.ListEventSummaries(t.Context(), store.EventSummaryQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, Type: eventspkg.ProfileArchived,
		})
		if err != nil {
			t.Fatalf("ListEventSummaries() error = %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("archive events = %d, want one durable event", len(entries))
		}
		var event profile.Event
		if err := json.Unmarshal(entries[0].ContentValue(), &event); err != nil {
			t.Fatalf("decode archive event: %v", err)
		}
		if event.ProfileID != created.ID || event.ProfileName != created.Name || event.OperationID == "" {
			t.Fatalf("archive event = %#v, want the archived subject and operation", event)
		}
		if err := database.WriteEventSummary(t.Context(), store.EventSummary{
			ProfileID: created.ID, Type: eventspkg.ProfileIdentityUpdated, Timestamp: now,
		}); err == nil || !strings.Contains(err.Error(), "profile_archived") {
			t.Fatalf("ordinary event write error = %v, want profile_archived", err)
		}
		if _, err := manager.UpdateIdentity(t.Context(), created.Name, profile.IdentityPatch{
			Color: new("#b27051"),
		}); err != nil {
			t.Fatalf("UpdateIdentity(archived) error = %v", err)
		}
		entries, err = database.ListEventSummaries(t.Context(), store.EventSummaryQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, Type: eventspkg.ProfileIdentityUpdated,
		})
		if err != nil {
			t.Fatalf("ListEventSummaries(identity update) error = %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("archived identity events = %d, want one durable event", len(entries))
		}
		if err := json.Unmarshal(entries[0].ContentValue(), &event); err != nil {
			t.Fatalf("decode archived identity event: %v", err)
		}
		if event.ProfileID != created.ID || event.ProfileName != created.Name {
			t.Fatalf("archived identity event = %#v, want the original subject", event)
		}
	})

	t.Run("Should retain a failed lifecycle event while its owner is reserved", func(t *testing.T) {
		t.Parallel()
		database := openDaemonTestGlobalDB(t)
		home, err := compozyconfig.ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		now := time.Date(2026, 10, 3, 14, 0, 0, 0, time.UTC)
		manager, err := profile.NewManager(
			profile.WithStore(database), profile.WithHomePaths(home),
			profile.WithClock(func() time.Time { return now }),
			profile.WithEventRecorder(&daemonProfileEventRecorder{
				writer: database, now: func() time.Time { return now },
			}),
		)
		if err != nil {
			t.Fatalf("profile.NewManager() error = %v", err)
		}
		created, err := manager.Create(t.Context(), profile.CreateInput{Name: "recovery-notes"})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}
		if err := os.Mkdir(filepath.Join(home.ProfilesDir, "recovery-guides"), 0o700); err != nil {
			t.Fatalf("create conflicting destination: %v", err)
		}
		plan, err := manager.PrepareRename(t.Context(), created.Name, "recovery-guides")
		if err != nil {
			t.Fatalf("PrepareRename() error = %v", err)
		}
		if _, err := manager.Rename(t.Context(), created.Name, profile.RenameOptions{
			NewName: "recovery-guides", PlanRevision: plan.Revision, Repos: profile.RepoChoice{None: true},
		}); err == nil {
			t.Fatal("Rename() succeeded despite the conflicting destination")
		}
		ops, err := manager.ListOps(t.Context())
		if err != nil {
			t.Fatalf("ListOps() error = %v", err)
		}
		var failed profile.LifecycleOp
		for _, op := range ops {
			if op.Kind == "rename" {
				failed = op
			}
		}
		if failed.Status != "failed" || failed.Step != "rename_profile" {
			t.Fatalf("rename operation = %#v, want failed filesystem step", failed)
		}
		entries, err := database.ListEventSummaries(t.Context(), store.EventSummaryQuery{
			ReadScope: store.ReadScope{AllProfiles: true}, Type: eventspkg.ProfileLifecycleOpFailed,
		})
		if err != nil {
			t.Fatalf("ListEventSummaries() error = %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("failure events = %d, want one durable event", len(entries))
		}
		var event profile.Event
		if err := json.Unmarshal(entries[0].ContentValue(), &event); err != nil {
			t.Fatalf("decode failure event: %v", err)
		}
		if event.ProfileID != created.ID || event.ProfileName != failed.Profile ||
			event.OperationID != failed.ID || event.Error != failed.Error {
			t.Fatalf("failure event = %#v, want the reserved owner and failed operation %#v", event, failed)
		}
		if err := database.WriteEventSummary(t.Context(), store.EventSummary{
			ProfileID: created.ID, Type: eventspkg.ProfileIdentityUpdated, Timestamp: now,
		}); err == nil || !strings.Contains(err.Error(), "profile_unavailable") {
			t.Fatalf("ordinary event write error = %v, want profile_unavailable", err)
		}
	})

	for _, testCase := range []struct {
		name      string
		eventName string
	}{
		{name: "Should persist deleted profile events under the permanent operator owner", eventName: eventspkg.ProfileDeleted},
		{name: "Should persist lifecycle recovery events under the permanent operator owner", eventName: eventspkg.ProfileLifecycleOpRecovered},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
			writer := &profileEventSummaryWriterStub{}
			recorder := &daemonProfileEventRecorder{writer: writer, now: func() time.Time { return now }}
			event := profile.Event{
				Name: testCase.eventName, ProfileID: "deleted-profile", ProfileName: "growth", OperationID: "op-delete",
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
	}
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
