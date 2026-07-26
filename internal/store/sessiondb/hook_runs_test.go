package sessiondb

import (
	"encoding/json"
	"testing"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func TestSessionDBRecordHookRunPersistsSecurityPatchFields(t *testing.T) {
	t.Parallel()

	sessionDB := openTestSessionDB(t, "sess-hook-run")
	recordedAt := time.Date(2026, 4, 9, 18, 0, 0, 0, time.UTC)
	patch := json.RawMessage(`{"decision":"deny","reason":"policy"}`)

	if err := sessionDB.RecordHookRun(testutil.Context(t), hookspkg.HookRunRecord{
		HookName:      "permission-audit",
		Event:         hookspkg.HookPermissionRequest,
		Source:        hookspkg.HookSourceConfig,
		Mode:          hookspkg.HookModeSync,
		Duration:      25 * time.Millisecond,
		Outcome:       hookspkg.HookRunOutcomeDenied,
		DispatchDepth: 2,
		PatchApplied:  patch,
		Error:         "denied by policy",
		Required:      true,
		RecordedAt:    recordedAt,
	}); err != nil {
		t.Fatalf("RecordHookRun() error = %v", err)
	}

	records, err := sessionDB.QueryHookRuns(testutil.Context(t), store.HookRunQuery{})
	if err != nil {
		t.Fatalf("QueryHookRuns() error = %v", err)
	}
	if got, want := len(records), 1; got != want {
		t.Fatalf("len(records) = %d, want %d", got, want)
	}

	record := records[0]
	if record.HookName != "permission-audit" {
		t.Fatalf("record.HookName = %q, want permission-audit", record.HookName)
	}
	if record.Event != hookspkg.HookPermissionRequest {
		t.Fatalf("record.Event = %q, want %q", record.Event, hookspkg.HookPermissionRequest)
	}
	if record.Source != hookspkg.HookSourceConfig {
		t.Fatalf("record.Source = %q, want %q", record.Source, hookspkg.HookSourceConfig)
	}
	if record.Mode != hookspkg.HookModeSync {
		t.Fatalf("record.Mode = %q, want %q", record.Mode, hookspkg.HookModeSync)
	}
	if record.Duration != 25*time.Millisecond {
		t.Fatalf("record.Duration = %s, want 25ms", record.Duration)
	}
	if record.Outcome != hookspkg.HookRunOutcomeDenied {
		t.Fatalf("record.Outcome = %q, want %q", record.Outcome, hookspkg.HookRunOutcomeDenied)
	}
	if record.DispatchDepth != 2 {
		t.Fatalf("record.DispatchDepth = %d, want 2", record.DispatchDepth)
	}
	if string(record.PatchApplied) != string(patch) {
		t.Fatalf("record.PatchApplied = %s, want %s", record.PatchApplied, patch)
	}
	if record.Error != "denied by policy" {
		t.Fatalf("record.Error = %q, want denied by policy", record.Error)
	}
	if !record.Required {
		t.Fatal("record.Required = false, want true")
	}
	if !record.RecordedAt.Equal(recordedAt) {
		t.Fatalf("record.RecordedAt = %s, want %s", record.RecordedAt, recordedAt)
	}
}

func TestSessionDBQueryHookRunsFiltersByEvent(t *testing.T) {
	t.Parallel()

	sessionDB := openTestSessionDB(t, "sess-hook-filter")
	records := []hookspkg.HookRunRecord{
		{
			HookName:      "prompt-hook",
			Event:         hookspkg.HookPromptPostAssemble,
			Source:        hookspkg.HookSourceConfig,
			Mode:          hookspkg.HookModeSync,
			Outcome:       hookspkg.HookRunOutcomeApplied,
			DispatchDepth: 1,
			RecordedAt:    time.Date(2026, 4, 9, 18, 1, 0, 0, time.UTC),
		},
		{
			HookName:      "permission-hook",
			Event:         hookspkg.HookPermissionRequest,
			Source:        hookspkg.HookSourceConfig,
			Mode:          hookspkg.HookModeSync,
			Outcome:       hookspkg.HookRunOutcomeDenied,
			DispatchDepth: 1,
			RecordedAt:    time.Date(2026, 4, 9, 18, 2, 0, 0, time.UTC),
		},
	}

	for _, record := range records {
		if err := sessionDB.RecordHookRun(testutil.Context(t), record); err != nil {
			t.Fatalf("RecordHookRun(%q) error = %v", record.HookName, err)
		}
	}

	filtered, err := sessionDB.QueryHookRuns(
		testutil.Context(t),
		store.HookRunQuery{Event: hookspkg.HookPermissionRequest.String()},
	)
	if err != nil {
		t.Fatalf("QueryHookRuns(filtered) error = %v", err)
	}
	if got, want := len(filtered), 1; got != want {
		t.Fatalf("len(filtered) = %d, want %d", got, want)
	}
	if filtered[0].HookName != "permission-hook" {
		t.Fatalf("filtered[0].HookName = %q, want permission-hook", filtered[0].HookName)
	}
}

func TestSessionDBQueryHookRunsFiltersByOutcome(t *testing.T) {
	t.Parallel()

	sessionDB := openTestSessionDB(t, "sess-hook-outcome")
	records := []hookspkg.HookRunRecord{
		{
			HookName:   "applied-hook",
			Event:      hookspkg.HookPermissionRequest,
			Source:     hookspkg.HookSourceConfig,
			Mode:       hookspkg.HookModeSync,
			Outcome:    hookspkg.HookRunOutcomeApplied,
			RecordedAt: time.Date(2026, 4, 9, 18, 3, 0, 0, time.UTC),
		},
		{
			HookName:   "failed-hook",
			Event:      hookspkg.HookPermissionRequest,
			Source:     hookspkg.HookSourceConfig,
			Mode:       hookspkg.HookModeSync,
			Outcome:    hookspkg.HookRunOutcomeFailed,
			RecordedAt: time.Date(2026, 4, 9, 18, 4, 0, 0, time.UTC),
		},
	}

	for _, record := range records {
		if err := sessionDB.RecordHookRun(testutil.Context(t), record); err != nil {
			t.Fatalf("RecordHookRun(%q) error = %v", record.HookName, err)
		}
	}

	filtered, err := sessionDB.QueryHookRuns(
		testutil.Context(t),
		store.HookRunQuery{Outcome: hookspkg.HookRunOutcomeFailed},
	)
	if err != nil {
		t.Fatalf("QueryHookRuns(filtered) error = %v", err)
	}
	if got, want := len(filtered), 1; got != want {
		t.Fatalf("len(filtered) = %d, want %d", got, want)
	}
	if filtered[0].HookName != "failed-hook" {
		t.Fatalf("filtered[0].HookName = %q, want failed-hook", filtered[0].HookName)
	}
}

func TestSessionDBQueryHookRunsAppliesEventOutcomeSinceAndLimitInAscendingOrder(t *testing.T) {
	t.Parallel()

	sessionDB := openTestSessionDB(t, "sess-hook-combined")
	records := []hookspkg.HookRunRecord{
		{
			HookName:   "ignore-other-event",
			Event:      hookspkg.HookPromptPostAssemble,
			Source:     hookspkg.HookSourceConfig,
			Mode:       hookspkg.HookModeSync,
			Outcome:    hookspkg.HookRunOutcomeApplied,
			RecordedAt: time.Date(2026, 4, 9, 18, 0, 0, 0, time.UTC),
		},
		{
			HookName:   "permission-old",
			Event:      hookspkg.HookPermissionRequest,
			Source:     hookspkg.HookSourceConfig,
			Mode:       hookspkg.HookModeSync,
			Outcome:    hookspkg.HookRunOutcomeApplied,
			RecordedAt: time.Date(2026, 4, 9, 18, 1, 0, 0, time.UTC),
		},
		{
			HookName:   "permission-denied",
			Event:      hookspkg.HookPermissionRequest,
			Source:     hookspkg.HookSourceConfig,
			Mode:       hookspkg.HookModeSync,
			Outcome:    hookspkg.HookRunOutcomeDenied,
			RecordedAt: time.Date(2026, 4, 9, 18, 2, 0, 0, time.UTC),
		},
		{
			HookName:   "permission-recent-a",
			Event:      hookspkg.HookPermissionRequest,
			Source:     hookspkg.HookSourceConfig,
			Mode:       hookspkg.HookModeSync,
			Outcome:    hookspkg.HookRunOutcomeApplied,
			RecordedAt: time.Date(2026, 4, 9, 18, 3, 0, 0, time.UTC),
		},
		{
			HookName:   "permission-recent-b",
			Event:      hookspkg.HookPermissionRequest,
			Source:     hookspkg.HookSourceConfig,
			Mode:       hookspkg.HookModeSync,
			Outcome:    hookspkg.HookRunOutcomeApplied,
			RecordedAt: time.Date(2026, 4, 9, 18, 4, 0, 0, time.UTC),
		},
	}

	for _, record := range records {
		if err := sessionDB.RecordHookRun(testutil.Context(t), record); err != nil {
			t.Fatalf("RecordHookRun(%q) error = %v", record.HookName, err)
		}
	}

	filtered, err := sessionDB.QueryHookRuns(testutil.Context(t), store.HookRunQuery{
		Event:   hookspkg.HookPermissionRequest.String(),
		Outcome: hookspkg.HookRunOutcomeApplied,
		Since:   time.Date(2026, 4, 9, 18, 0, 30, 0, time.UTC),
		Limit:   2,
	})
	if err != nil {
		t.Fatalf("QueryHookRuns(filtered) error = %v", err)
	}
	if got, want := len(filtered), 2; got != want {
		t.Fatalf("len(filtered) = %d, want %d", got, want)
	}
	if filtered[0].HookName != "permission-recent-a" || filtered[1].HookName != "permission-recent-b" {
		t.Fatalf("filtered = %#v, want ascending last-two applied permission hooks", filtered)
	}
	if !filtered[0].RecordedAt.Before(filtered[1].RecordedAt) {
		t.Fatalf(
			"filtered order = %s then %s, want ascending chronology",
			filtered[0].RecordedAt,
			filtered[1].RecordedAt,
		)
	}
}
