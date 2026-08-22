package cmdpalette

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// Invariant: personalization accepts only a real profile ULID or the reserved aggregate lens.
// The existing personalization suite owns the lens contract because every ranking operation consumes it.
func TestProfileLensIDValidation(t *testing.T) {
	t.Parallel()

	for _, lens := range []ProfileLensID{DefaultProfileLensID, AggregateProfileLensID, "01ARZ3NDEKTSV4RRFFQ69G5FAV"} {
		if err := lens.Validate(); err != nil {
			t.Fatalf("ProfileLensID(%q).Validate() error = %v", lens, err)
		}
	}
	for _, lens := range []ProfileLensID{"", "default", "@unknown"} {
		if err := lens.Validate(); err == nil {
			t.Fatalf("ProfileLensID(%q).Validate() error = nil, want non-nil", lens)
		}
	}
	for name, lens := range map[string]ProfileLens{
		"scoped":    ScopedProfileLens(DefaultProfileLensID, "default"),
		"aggregate": AggregateProfileLens(),
	} {
		if err := lens.Validate(); err != nil {
			t.Fatalf("%s ProfileLens.Validate() error = %v", name, err)
		}
	}
	for name, lens := range map[string]ProfileLens{
		"missing":               {},
		"unlabeled aggregate":   {ID: AggregateProfileLensID},
		"mislabelled aggregate": {ID: AggregateProfileLensID, Name: "default"},
	} {
		if err := lens.Validate(); err == nil {
			t.Fatalf("%s ProfileLens.Validate() error = nil, want non-nil", name)
		}
	}
}

func TestPersonalization(t *testing.T) {
	t.Parallel()

	t.Run("Should decay exactly one half-life and prune only old weak signals", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
		if got := DecayFrecency(10, now.Add(-30*24*time.Hour), now, 30*24*time.Hour); got != 5 {
			t.Fatalf("DecayFrecency(one half-life) = %v, want 5", got)
		}
		if shouldPruneSignal(WeightsV1.PruneThreshold/2, now.Add(-120*24*time.Hour), now) != true {
			t.Fatal("old weak signal should be prunable")
		}
		if shouldPruneSignal(WeightsV1.PruneThreshold, now.Add(-120*24*time.Hour), now) {
			t.Fatal("signal at the prune threshold should survive")
		}
	})

	t.Run("Should normalize and record only the pre-selection query", func(t *testing.T) {
		t.Parallel()
		store := &personalizationStoreStub{}
		service := personalizationTestRegistry(t, store, nil)
		if err := service.RecordUsage(t.Context(), Usage{ProfileLensID: testProfileLens.ID,
			WorkspaceID: "workspace-a", CommandID: "session.new", Query: "  Séssão   NOVA  ",
		}); err != nil {
			t.Fatalf("RecordUsage() error = %v", err)
		}
		if recorded := store.lastUsage(); recorded.Query != "sessao nova" ||
			recorded.WorkspaceID != "workspace-a" || recorded.CommandID != "session.new" {
			t.Fatalf("recorded usage = %#v, want normalized query and identifiers only", recorded)
		}
	})

	t.Run("Should stop recording while personalization is disabled", func(t *testing.T) {
		t.Parallel()
		store := &personalizationStoreStub{}
		service := testRegistryWithOptions(
			staticTestProvider{commands: []Descriptor{testDescriptor("session.new")}},
			nil,
			nil,
			&testExecutor{},
			WithPersonalizationStore(store),
			WithPersonalizationPolicy(personalizationPolicyFunc(func(
				context.Context,
				WorkspaceID,
			) (bool, error) {
				return false, nil
			})),
		)

		if err := service.RecordUsage(t.Context(), Usage{ProfileLensID: testProfileLens.ID,
			WorkspaceID: "workspace-a", CommandID: "session.new", Query: "new",
		}); err != nil {
			t.Fatalf("RecordUsage() error = %v", err)
		}
		if recorded := store.lastUsage(); recorded.CommandID != "" {
			t.Fatalf("recorded usage = %#v, want no write while disabled", recorded)
		}
	})

	t.Run("Should prune stale catalog IDs and low decayed rows without failing the snapshot", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
		store := &personalizationStoreStub{rows: PersonalizationRows{
			Usage: []UsageSignal{
				{CommandID: "session.new", Weight: 2, LastUsedAt: now.Add(-time.Hour).UnixMilli()},
				{CommandID: "removed.command", Weight: 2, LastUsedAt: now.UnixMilli()},
				{CommandID: "session.new", Weight: 0.01, LastUsedAt: now.Add(-121 * 24 * time.Hour).UnixMilli()},
			},
			QueryHits: []QueryHit{
				{Query: "ns", CommandID: "session.new", Weight: 1, LastUsedAt: now.UnixMilli()},
				{
					Query:      "old",
					CommandID:  "session.new",
					Weight:     0.01,
					LastUsedAt: now.Add(-121 * 24 * time.Hour).UnixMilli(),
				},
			},
			Pins: []Pin{
				{CommandID: "session.new", PinnedAt: 1},
				{CommandID: "removed.command", PinnedAt: 2},
			},
		}}
		service := personalizationTestRegistry(t, store, func() time.Time { return now })
		snapshot, err := service.Personalization(t.Context(), testProfileLens, "workspace-a")
		if err != nil {
			t.Fatalf("Personalization() error = %v", err)
		}
		if len(snapshot.Usage) != 1 || len(snapshot.QueryHits) != 1 || len(snapshot.Pins) != 1 {
			t.Fatalf("snapshot = %#v, want one maintained row per signal", snapshot)
		}
		if len(store.prunedCommands) != 1 || store.prunedCommands[0] != "removed.command" ||
			len(store.prunedUsage) != 1 || store.prunedUsage[0] != "session.new" ||
			len(store.prunedHits) != 1 || store.prunedHits[0] != "old\x00session.new" {
			t.Fatalf(
				"pruned commands/usage/hits = %#v/%#v/%#v",
				store.prunedCommands,
				store.prunedUsage,
				store.prunedHits,
			)
		}
	})

	t.Run("Should degrade corrupt reads to stable empty signals and log once", func(t *testing.T) {
		t.Parallel()
		store := &personalizationStoreStub{readErr: errors.New("corrupt row")}
		var logs bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&logs, nil))
		service := personalizationTestRegistryWithLogger(t, store, nil, logger)
		first, err := service.Personalization(t.Context(), testProfileLens, "workspace-a")
		if err != nil {
			t.Fatalf("Personalization(first) error = %v", err)
		}
		second, err := service.Personalization(t.Context(), testProfileLens, "workspace-a")
		if err != nil {
			t.Fatalf("Personalization(second) error = %v", err)
		}
		if len(first.Usage) != 0 || len(first.QueryHits) != 0 || len(first.Pins) != 0 ||
			first.Revision == "" || first.Revision != second.Revision {
			t.Fatalf("degraded snapshots = %#v / %#v", first, second)
		}
		if count := strings.Count(logs.String(), "personalization degraded to empty signals"); count != 1 {
			t.Fatalf("degradation log count = %d, want 1; logs=%s", count, logs.String())
		}
	})

	t.Run("Should keep the scorer fixture byte-for-field aligned with WeightsV1", func(t *testing.T) {
		t.Parallel()
		contents, err := os.ReadFile(filepath.Join("testdata", "ranking_weights_v1.json"))
		if err != nil {
			t.Fatalf("ReadFile(ranking weights fixture) error = %v", err)
		}
		var fixture Weights
		if err := json.Unmarshal(contents, &fixture); err != nil {
			t.Fatalf("json.Unmarshal(ranking weights fixture) error = %v", err)
		}
		if !reflect.DeepEqual(fixture, WeightsV1) {
			t.Fatalf("ranking weights fixture = %#v, want WeightsV1 %#v", fixture, WeightsV1)
		}
	})

	t.Run("Should publish workspace pin and reset events after persistence", func(t *testing.T) {
		t.Parallel()
		store := &personalizationStoreStub{}
		recorder := &recordingEventRecorder{}
		now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
		service := testRegistryWithOptions(
			staticTestProvider{commands: []Descriptor{testDescriptor("session.new")}},
			nil,
			nil,
			&testExecutor{},
			WithPersonalizationStore(store),
			WithEventRecorder(recorder),
			WithClock(func() time.Time { return now }),
		)

		if err := service.Pin(t.Context(), testProfileLens, "workspace-a", "session.new"); err != nil {
			t.Fatalf("Pin() error = %v", err)
		}
		if err := service.ResetPersonalization(t.Context(), testProfileLens, "workspace-a"); err != nil {
			t.Fatalf("ResetPersonalization() error = %v", err)
		}
		if !store.pinned || store.resetWorkspace != "workspace-a" {
			t.Fatalf("persisted pin/reset = %v/%q", store.pinned, store.resetWorkspace)
		}
		events := recorder.recorded()
		if len(events) != 4 || events[0].Name != EventPinChanged || events[0].Pinned == nil ||
			!*events[0].Pinned || events[0].CommandID != "session.new" ||
			events[1].Name != EventCatalogChanged ||
			events[2].Name != EventPersonalizationReset ||
			events[3].Name != EventCatalogChanged {
			t.Fatalf("personalization events = %#v", events)
		}
		for _, event := range events {
			if event.WorkspaceID != "workspace-a" || !event.OccurredAt.Equal(now) {
				t.Fatalf("event scope/time = %#v", event)
			}
		}
	})
}

type personalizationPolicyFunc func(context.Context, WorkspaceID) (bool, error)

func (f personalizationPolicyFunc) PersonalizationEnabled(
	ctx context.Context,
	_ ProfileLens,
	workspaceID WorkspaceID,
) (bool, error) {
	return f(ctx, workspaceID)
}

func personalizationTestRegistry(
	t *testing.T,
	store *personalizationStoreStub,
	clock func() time.Time,
) *Service {
	t.Helper()
	return personalizationTestRegistryWithLogger(t, store, clock, slog.Default())
}

func personalizationTestRegistryWithLogger(
	t *testing.T,
	store *personalizationStoreStub,
	clock func() time.Time,
	logger *slog.Logger,
) *Service {
	t.Helper()
	options := []Option{WithPersonalizationStore(store), WithLogger(logger)}
	if clock != nil {
		options = append(options, WithClock(clock))
	}
	return testRegistryWithOptions(
		staticTestProvider{commands: []Descriptor{testDescriptor("session.new")}},
		nil,
		nil,
		&testExecutor{},
		options...,
	)
}

type personalizationStoreStub struct {
	mu             sync.Mutex
	recorded       Usage
	recordErr      error
	rows           PersonalizationRows
	readErr        error
	prunedCommands []CommandID
	prunedUsage    []CommandID
	prunedHits     []string
	pinned         bool
	resetWorkspace WorkspaceID
}

func (s *personalizationStoreStub) RecordCmdPaletteUsage(
	_ context.Context,
	usage Usage,
	_ Weights,
) error {
	s.mu.Lock()
	s.recorded = usage
	s.mu.Unlock()
	return s.recordErr
}

func (s *personalizationStoreStub) lastUsage() Usage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recorded
}

func (s *personalizationStoreStub) CmdPalettePersonalization(
	context.Context,
	ProfileLensID,
	WorkspaceID,
) (PersonalizationRows, error) {
	return s.rows, s.readErr
}

func (s *personalizationStoreStub) PutCmdPalettePin(
	_ context.Context,
	_ ProfileLensID,
	_ WorkspaceID,
	_ CommandID,
	_ time.Time,
) error {
	s.pinned = true
	return nil
}

func (s *personalizationStoreStub) DeleteCmdPalettePin(
	context.Context,
	ProfileLensID,
	WorkspaceID,
	CommandID,
) error {
	return nil
}

func (s *personalizationStoreStub) PruneCmdPaletteCommand(
	_ context.Context,
	_ ProfileLensID,
	_ WorkspaceID,
	commandID CommandID,
) error {
	s.prunedCommands = append(s.prunedCommands, commandID)
	return nil
}

func (s *personalizationStoreStub) PruneCmdPaletteUsage(
	_ context.Context,
	_ ProfileLensID,
	_ WorkspaceID,
	commandID CommandID,
) error {
	s.prunedUsage = append(s.prunedUsage, commandID)
	return nil
}

func (s *personalizationStoreStub) PruneCmdPaletteQueryHit(
	_ context.Context,
	_ ProfileLensID,
	_ WorkspaceID,
	query string,
	commandID CommandID,
) error {
	s.prunedHits = append(s.prunedHits, query+"\x00"+string(commandID))
	return nil
}

func (s *personalizationStoreStub) ResetCmdPalettePersonalization(
	_ context.Context,
	_ ProfileLensID,
	workspaceID WorkspaceID,
) error {
	s.resetWorkspace = workspaceID
	return nil
}
