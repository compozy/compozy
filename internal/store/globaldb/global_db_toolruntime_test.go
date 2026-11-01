package globaldb

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/monitor"
	"github.com/compozy/compozy/internal/sessionprocess"
	"github.com/compozy/compozy/internal/testutil"
	"github.com/compozy/compozy/internal/toolruntime"
)

// Invariant: an observed identity keeps its latest owner across durable reads.
// Owning layer/canonical suite: globaldb process-record persistence.
func TestGlobalDBSessionProcessObservations(t *testing.T) {
	t.Parallel()
	t.Run("Should replace ownership without replacing the process identity or first observation", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		db := openTestGlobalDB(t)
		start := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
		observation := sessionprocess.Observation{
			PID: 1234, StartedAt: start, FirstSeen: start.Add(time.Second),
			Owner:       sessionprocess.Owner{SessionID: "sess-old", WorkspaceID: "ws-old", ProfileID: "profile-old"},
			Attribution: "lineage",
			KernelToken: "boot:123",
		}
		if err := db.SaveProcessObservations(ctx, []sessionprocess.Observation{observation}, nil); err != nil {
			t.Fatal(err)
		}
		observation.Owner = sessionprocess.Owner{
			SessionID: "sess-new", WorkspaceID: "ws-new", ProfileID: "profile-new", Name: "Moved session",
		}
		observation.Attribution = "env"
		if err := db.SaveProcessObservations(ctx, []sessionprocess.Observation{observation}, nil); err != nil {
			t.Fatal(err)
		}
		rows, err := db.LoadProcessObservations(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || rows[0] != observation {
			t.Fatalf("reloaded observations = %+v, want %+v", rows, observation)
		}
	})
	// Invariant: the additive token migration preserves legacy observations across reopen.
	t.Run("Should preserve observations while adding kernel identity", func(t *testing.T) {
		t.Parallel()
		ctx := globalMigrationTestContext(t)
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prior, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00158_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		_, err = prior.ExecContext(ctx, `INSERT INTO session_process_observations
   (pid,started_at,session_id,workspace_id,profile_id,session_name,pgid,first_seen,group_observed_at,attribution)
   VALUES (1234,'2026-10-29T10:00:00Z','retained','ws','profile','Kept',1234,'2026-10-29T10:00:01Z','2026-10-29T10:00:00Z','lineage')`)
		if err != nil {
			t.Fatal(err)
		}
		if err := prior.Close(); err != nil {
			t.Fatal(err)
		}
		for range 2 {
			upgraded, err := openGlobalMigrationUpgrade(t, path)
			if err != nil {
				t.Fatal(err)
			}
			rows, err := upgraded.LoadProcessObservations(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(rows) != 1 || rows[0].PID != 1234 || rows[0].Owner.Name != "Kept" || rows[0].KernelToken != "" ||
				rows[0].Attribution != "lineage" {
				t.Fatalf("preserved rows=%+v", rows)
			}
			if err := upgraded.Close(ctx); err != nil {
				t.Fatal(err)
			}
		}
	})
}

func TestGlobalDBToolProcessRecords(t *testing.T) {
	t.Parallel()

	t.Run("Should persist and update tool process records", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		startedAt := time.Date(2026, 4, 24, 11, 0, 0, 0, time.UTC)
		record := toolruntime.ProcessRecord{
			ID:             "proc-globaldb",
			Source:         toolruntime.ProcessSourceACPTerminal,
			Owner:          toolruntime.ProcessOwner{SessionID: "sess-1", TurnID: "turn-1", TerminalID: "term-1"},
			PID:            1234,
			ProcessGroupID: 1234,
			Command:        "sleep",
			Args:           []string{"60"},
			Cwd:            "/workspace",
			StartedAt:      startedAt,
			StartedByPID:   4321,
			State:          toolruntime.ProcessStateRunning,
			CreatedAt:      startedAt,
			UpdatedAt:      startedAt,
		}

		if err := globalDB.UpsertProcessRecord(ctx, record); err != nil {
			t.Fatalf("UpsertProcessRecord() error = %v", err)
		}
		records, err := globalDB.ListProcessRecords(ctx, toolruntime.ProcessQuery{
			Scope: toolruntime.InterruptScope{SessionID: "sess-1", TurnID: "turn-1"},
		})
		if err != nil {
			t.Fatalf("ListProcessRecords() error = %v", err)
		}
		if got, want := len(records), 1; got != want {
			t.Fatalf("records = %d, want %d", got, want)
		}
		if records[0].Owner.TerminalID != "term-1" || records[0].Args[0] != "60" {
			t.Fatalf("record = %#v, want persisted owner and args", records[0])
		}

		exitCode := 130
		completedAt := startedAt.Add(time.Minute)
		if err := globalDB.UpdateProcessRecordState(ctx, toolruntime.ProcessStateUpdate{
			ID:          record.ID,
			State:       toolruntime.ProcessStateInterrupted,
			ExitCode:    &exitCode,
			Error:       "canceled",
			UpdatedAt:   completedAt,
			CompletedAt: &completedAt,
		}); err != nil {
			t.Fatalf("UpdateProcessRecordState() error = %v", err)
		}
		records, err = globalDB.ListProcessRecords(ctx, toolruntime.ProcessQuery{
			IDs:    []string{record.ID},
			States: []toolruntime.ProcessState{toolruntime.ProcessStateInterrupted},
		})
		if err != nil {
			t.Fatalf("ListProcessRecords(updated) error = %v", err)
		}
		if got, want := len(records), 1; got != want {
			t.Fatalf("updated records = %d, want %d", got, want)
		}
		updated := records[0]
		if updated.ExitCode == nil || *updated.ExitCode != exitCode {
			t.Fatalf("updated.ExitCode = %v, want %d", updated.ExitCode, exitCode)
		}
		if updated.CompletedAt == nil || !updated.CompletedAt.Equal(completedAt) {
			t.Fatalf("updated.CompletedAt = %v, want %s", updated.CompletedAt, completedAt)
		}
	})

	t.Run("Should reject state updates for missing records", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		err := globalDB.UpdateProcessRecordState(ctx, toolruntime.ProcessStateUpdate{
			ID:        "proc-missing",
			State:     toolruntime.ProcessStateCompleted,
			UpdatedAt: time.Date(2026, 4, 24, 11, 0, 0, 0, time.UTC),
		})
		if !errors.Is(err, toolruntime.ErrProcessNotFound) {
			t.Fatalf("UpdateProcessRecordState(missing) error = %v, want ErrProcessNotFound", err)
		}
	})
}

// Invariant: a monitor definition and lifecycle survive a reopen, a stale write never
// overwrites a newer one, and retention prunes only finished monitors.
// Owning layer/canonical suite: globaldb daemon-owned wait persistence.
func TestGlobalDBSessionMonitors(t *testing.T) {
	t.Parallel()
	ctx := testutil.Context(t)
	suite := t
	path := filepath.Join(t.TempDir(), GlobalDatabaseName)
	copyCurrentSchemaGlobalDBSeed(t, path)
	db := openGlobalDBForTest(t, path)
	at := time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC)
	code := 2
	active := monitor.Record{
		Monitor: monitor.Monitor{
			ID: "mon-aaaaaaaaaaaa", SessionID: "sess-1", Kind: monitor.KindTerminal, Description: "wait for CI",
			TerminalID: "term-1", Until: monitor.UntilMatch, Pattern: "done", Status: monitor.StatusActive,
			CreatedAt: at, DeadlineAt: at.Add(time.Hour),
		},
		WorkspaceID: "ws-1", ProfileID: "prof-1", Timeout: time.Hour, OwnerBinding: `{"turn_id":""}`, Version: 1,
		OwnerCheckFailedAt: at.Add(time.Minute),
	}
	t.Run("Should round-trip the definition and owner binding across a reopen", func(t *testing.T) {
		if err := db.SaveMonitor(ctx, active); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
		// The reopened handle belongs to the suite, not this subtest, so later subtests keep using it.
		db = openGlobalDBForTest(suite, path)
		got, err := db.LoadMonitors(ctx)
		if err != nil || len(got) != 1 || got[0].Monitor != active.Monitor || got[0].Timeout != time.Hour ||
			got[0].OwnerBinding != active.OwnerBinding || !got[0].OwnerCheckFailedAt.Equal(active.OwnerCheckFailedAt) || got[0].WorkspaceID != "ws-1" || got[0].ProfileID != "prof-1" {
			t.Fatalf("LoadMonitors() = %+v, %v; want %+v", got, err, active)
		}
	})
	t.Run("Should apply newer versions and ignore stale ones", func(t *testing.T) {
		fired := active
		fired.Monitor.Status, fired.Monitor.Reason, fired.Monitor.ExitCode = monitor.StatusFired, monitor.ReasonExit, &code
		fired.Monitor.FiredAt, fired.Version = at.Add(time.Minute), 3
		if err := db.SaveMonitor(ctx, fired); err != nil {
			t.Fatal(err)
		}
		stale := active
		stale.Monitor.Status, stale.Version = monitor.StatusDelivering, 2
		if err := db.SaveMonitor(ctx, stale); err != nil {
			t.Fatal(err)
		}
		got, err := db.LoadMonitors(ctx)
		if err != nil || len(got) != 1 || got[0].Monitor.Status != monitor.StatusFired ||
			got[0].Monitor.ExitCode == nil || *got[0].Monitor.ExitCode != code || !got[0].Monitor.FiredAt.Equal(fired.Monitor.FiredAt) {
			t.Fatalf("LoadMonitors() = %+v, %v; want the version-3 fired row", got, err)
		}
	})
	t.Run("Should prune finished monitors past the cutoff and keep occupying ones", func(t *testing.T) {
		live := active
		live.Monitor.ID, live.Version = "mon-bbbbbbbbbbbb", 1
		if err := db.SaveMonitor(ctx, live); err != nil {
			t.Fatal(err)
		}
		if err := db.PruneFinishedMonitors(ctx, at.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		got, err := db.LoadMonitors(ctx)
		if err != nil || len(got) != 1 || got[0].Monitor.ID != live.Monitor.ID {
			t.Fatalf("LoadMonitors() = %+v, %v; want only the active monitor", got, err)
		}
	})
}
