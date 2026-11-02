package globaldb

import (
	"errors"
	"path/filepath"
	"reflect"
	"slices"
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

// Invariant: the rebuild that admits pull_request monitors keeps every existing monitor row.
func TestGlobalDBSessionMonitorKindMigration(t *testing.T) {
	t.Parallel()
	t.Run("Should keep existing monitors and admit the new kind", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefixDB, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00175_schema.sql"))
		if err != nil {
			t.Fatalf("open prefix before 00175 error = %v", err)
		}
		ctx := globalMigrationTestContext(t)
		if _, err := prefixDB.ExecContext(ctx, `INSERT INTO session_monitors (
			id, session_id, workspace_id, profile_id, kind, description, terminal_id, until, timeout_ms, status,
			created_at, deadline_at, exit_code, owner_binding, version
		) VALUES ('mon-aaaaaaaaaaaa', 'sess-1', 'ws-1', 'prof-1', 'terminal', 'wait for CI', 'term-1', 'exit', 60000,
			'active', '2026-11-01T09:00:00Z', '2026-11-01T10:00:00Z', 3, 'binding', 4)`); err != nil {
			t.Fatalf("seed monitor before 00175 error = %v", err)
		}
		if err := prefixDB.Close(); err != nil {
			t.Fatalf("close prefix before 00175 error = %v", err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatalf("upgrade through 00175 error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := upgraded.Close(testutil.Context(t)); closeErr != nil {
				t.Errorf("close upgraded database error = %v", closeErr)
			}
		})
		records, err := upgraded.LoadMonitors(ctx)
		if err != nil || len(records) != 1 {
			t.Fatalf("LoadMonitors() = %+v, %v", records, err)
		}
		mon := records[0].Monitor
		if mon.Kind != monitor.KindTerminal || mon.TerminalID != "term-1" || mon.ExitCode == nil || *mon.ExitCode != 3 ||
			records[0].OwnerBinding != "binding" || records[0].Version != 4 ||
			mon.FireCount != 0 ||
			mon.Branch != "" {
			t.Fatalf("migrated monitor = %+v", records[0])
		}
		deadline := time.Date(2026, 11, 1, 10, 0, 0, 0, time.UTC)
		if mon.Until != "exit" || records[0].Timeout != time.Minute || mon.Status != monitor.StatusActive ||
			!mon.DeadlineAt.Equal(deadline) {
			t.Fatalf("migrated wait = %+v", records[0])
		}
		if err := upgraded.SaveMonitor(ctx, monitor.Record{
			Monitor: monitor.Monitor{
				ID: "mon-bbbbbbbbbbbb", SessionID: "sess-1", Kind: monitor.KindPullRequest, Description: "watch my PR",
				Branch: "feat/x", Status: monitor.StatusActive, CreatedAt: time.Now(), DeadlineAt: time.Now(),
			},
			WorkspaceID: "ws-1", ProfileID: "prof-1", Version: 1,
		}); err != nil {
			t.Fatalf("save pull_request monitor error = %v", err)
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
	t.Run("Should store session targets and record their fired states", func(t *testing.T) {
		watch := monitor.Record{
			Monitor: monitor.Monitor{
				ID: "mon-cccccccccccc", SessionID: "sess-2", Kind: monitor.KindSession, Description: "wait for workers",
				Until: monitor.UntilAll, Status: monitor.StatusActive, CreatedAt: at, DeadlineAt: at.Add(time.Hour),
				TargetSessionIDs: []string{"sess-a", "sess-b"}, States: []string{monitor.TargetStateIdle},
			},
			WorkspaceID: "ws-1", ProfileID: "prof-1", Timeout: time.Hour, Version: 1,
		}
		if err := db.SaveMonitor(ctx, watch); err != nil {
			t.Fatal(err)
		}
		watch.Monitor.Status = monitor.StatusDelivering
		watch.Monitor.FiredAt = at.Add(time.Minute)
		watch.Monitor.Reason = monitor.ReasonConditionMet
		watch.Monitor.Targets = []monitor.TargetStatus{
			{SessionID: "sess-a", Name: "Worker A", State: monitor.TargetStateIdle, Matched: true},
			{SessionID: "sess-b", State: monitor.TargetStateGone, Matched: true},
		}
		watch.Version = 2
		if err := db.SaveMonitor(ctx, watch); err != nil {
			t.Fatal(err)
		}
		got, err := db.LoadMonitors(ctx)
		if err != nil {
			t.Fatal(err)
		}
		index := slices.IndexFunc(
			got,
			func(record monitor.Record) bool { return record.Monitor.ID == watch.Monitor.ID },
		)
		if index < 0 || !reflect.DeepEqual(got[index].Monitor, watch.Monitor) {
			t.Fatalf("LoadMonitors() = %+v; want %+v", got, watch.Monitor)
		}
		if _, err := db.DB().
			ExecContext(ctx, `DELETE FROM session_monitors WHERE id = ?`, watch.Monitor.ID); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Should preserve terminal and timer monitors across the session kind migration", func(t *testing.T) {
		upgradePath := filepath.Join(t.TempDir(), GlobalDatabaseName)
		prefix, err := openGlobalMigrationPrefixDatabase(
			t,
			upgradePath,
			globalMigrationPrefixBefore(t, "00174_schema.sql"),
		)
		if err != nil {
			t.Fatal(err)
		}
		const seed = `INSERT INTO session_monitors
   (id,session_id,workspace_id,profile_id,kind,description,terminal_id,until,timeout_ms,status,created_at,deadline_at,owner_binding,version)
   VALUES (?,'sess-1','ws-1','prof-1',?,'wait',?,?,3600000,'active','2026-11-01T09:00:00Z','2026-11-01T10:00:00Z','binding',3)`
		_, terminalErr := prefix.ExecContext(ctx, seed, "mon-terminal", "terminal", "term-1", "exit")
		_, timerErr := prefix.ExecContext(ctx, seed, "mon-timer", "timer", "", "")
		if err := errors.Join(terminalErr, timerErr, prefix.Close()); err != nil {
			t.Fatal(err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, upgradePath)
		if err != nil {
			t.Fatal(err)
		}
		got, err := upgraded.LoadMonitors(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Monitor.ID != "mon-terminal" || got[0].Monitor.TerminalID != "term-1" ||
			got[0].Monitor.Until != "exit" || got[0].OwnerBinding != "binding" || got[0].Version != 3 ||
			got[1].Monitor.ID != "mon-timer" || got[1].Monitor.Kind != monitor.KindTimer ||
			got[0].Monitor.TargetSessionIDs != nil {
			t.Fatalf("migrated monitors = %+v", got)
		}
		if err := upgraded.SaveMonitor(ctx, monitor.Record{
			Monitor: monitor.Monitor{
				ID: "mon-session", SessionID: "sess-1", Kind: monitor.KindSession, Description: "wait",
				Status: monitor.StatusActive, CreatedAt: at, DeadlineAt: at, TargetSessionIDs: []string{"sess-a"},
			},
			WorkspaceID: "ws-1", ProfileID: "prof-1", Version: 1,
		}); err != nil {
			t.Fatalf("SaveMonitor(session) error = %v, want the widened kind accepted", err)
		}
		if err := upgraded.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Should round-trip the definition, owner binding, and version across a reopen", func(t *testing.T) {
		if err := db.SaveMonitor(ctx, active); err != nil {
			t.Fatal(err)
		}
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
		// The reopened handle belongs to the suite, not this subtest, so later subtests keep using it.
		db = openGlobalDBForTest(suite, path)
		got, err := db.LoadMonitors(ctx)
		if err != nil || len(got) != 1 || !reflect.DeepEqual(got[0].Monitor, active.Monitor) ||
			got[0].Timeout != time.Hour || got[0].OwnerBinding != active.OwnerBinding ||
			!got[0].OwnerCheckFailedAt.Equal(active.OwnerCheckFailedAt) ||
			got[0].WorkspaceID != "ws-1" || got[0].ProfileID != "prof-1" ||
			got[0].Version != active.Version {
			t.Fatalf("LoadMonitors() = %+v, %v; want %+v", got, err, active)
		}
	})
	t.Run("Should round-trip a pull_request watch and update its cursor and fires", func(t *testing.T) {
		watch := monitor.Record{
			Monitor: monitor.Monitor{
				ID: "mon-bbbbbbbbbbbb", SessionID: "sess-1", Kind: monitor.KindPullRequest, Description: "watch my PR",
				Branch: "feat/x", Status: monitor.StatusActive, CreatedAt: at, DeadlineAt: at.Add(6 * time.Hour),
			},
			WorkspaceID: "ws-1", ProfileID: "prof-1", Timeout: 6 * time.Hour, Version: 1,
			RepoKey: "forge.test/acme/repo", Cursor: `{"cursor":{"head":"a"}}`,
		}
		if err := db.SaveMonitor(ctx, watch); err != nil {
			t.Fatal(err)
		}
		fired := watch
		fired.Monitor.Status, fired.Monitor.FireCount = monitor.StatusDelivering, 3
		fired.Monitor.LastFiredAt = at.Add(time.Minute)
		fired.Cursor, fired.Version = `{"cursor":{"head":"b"}}`, 2
		if err := db.SaveMonitor(ctx, fired); err != nil {
			t.Fatal(err)
		}
		records, err := db.LoadMonitors(ctx)
		if err != nil {
			t.Fatal(err)
		}
		index := slices.IndexFunc(records, func(r monitor.Record) bool { return r.Monitor.ID == watch.Monitor.ID })
		if index < 0 || !reflect.DeepEqual(records[index].Monitor, fired.Monitor) ||
			records[index].RepoKey != watch.RepoKey ||
			records[index].Cursor != fired.Cursor {
			t.Fatalf("LoadMonitors() = %+v, want %+v", records, fired)
		}
		canceled := fired
		canceled.Monitor.Status, canceled.Monitor.FiredAt, canceled.Version = monitor.StatusCanceled, at, 3
		if err := db.SaveMonitor(ctx, canceled); err != nil {
			t.Fatal(err)
		}
		if err := db.PruneFinishedMonitors(ctx, at.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		pruned, err := db.LoadMonitors(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if slices.ContainsFunc(pruned, func(r monitor.Record) bool { return r.Monitor.ID == watch.Monitor.ID }) {
			t.Fatalf("PruneFinishedMonitors() kept %s", watch.Monitor.ID)
		}
	})
	t.Run("Should apply newer versions and ignore stale and equal-version writes", func(t *testing.T) {
		fired := active
		fired.Monitor.Status, fired.Monitor.Reason, fired.Monitor.ExitCode = monitor.StatusFired, monitor.ReasonExit, &code
		fired.Monitor.FiredAt, fired.Version = at.Add(time.Minute), 3
		// A newer write that clears the owner-check anchor must clear it in the store.
		fired.OwnerCheckFailedAt = time.Time{}
		if err := db.SaveMonitor(ctx, fired); err != nil {
			t.Fatal(err)
		}
		stale := active
		stale.Monitor.Status, stale.Version = monitor.StatusDelivering, 2
		if err := db.SaveMonitor(ctx, stale); err != nil {
			t.Fatal(err)
		}
		equal := fired
		equal.Monitor.Status, equal.Monitor.Reason = monitor.StatusUndelivered, "other"
		if err := db.SaveMonitor(ctx, equal); err != nil {
			t.Fatal(err)
		}
		got, err := db.LoadMonitors(ctx)
		if err != nil || len(got) != 1 || got[0].Version != 3 || got[0].Monitor.Status != monitor.StatusFired ||
			got[0].Monitor.Reason != monitor.ReasonExit ||
			got[0].Monitor.ExitCode == nil || *got[0].Monitor.ExitCode != code ||
			!got[0].Monitor.FiredAt.Equal(fired.Monitor.FiredAt) || !got[0].OwnerCheckFailedAt.IsZero() {
			t.Fatalf("LoadMonitors() = %+v, %v; want the version-3 fired row with the anchor cleared", got, err)
		}
	})
	t.Run("Should prune finished monitors before the cutoff and keep boundary and occupying ones", func(t *testing.T) {
		cutoff := at.Add(time.Hour)
		save := func(id string, status monitor.Status, firedAt time.Time) {
			record := active
			record.Monitor.ID, record.Monitor.Status, record.Monitor.FiredAt = id, status, firedAt
			record.Version = 1
			if err := db.SaveMonitor(ctx, record); err != nil {
				t.Fatal(err)
			}
		}
		save("mon-bbbbbbbbbbbb", monitor.StatusActive, time.Time{})
		save("mon-cccccccccccc", monitor.StatusDelivering, time.Time{})
		save("mon-dddddddddddd", monitor.StatusFired, cutoff)
		save("mon-eeeeeeeeeeee", monitor.StatusCanceled, cutoff.Add(-time.Second))
		if err := db.PruneFinishedMonitors(ctx, cutoff); err != nil {
			t.Fatal(err)
		}
		got, err := db.LoadMonitors(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, record := range got {
			ids = append(ids, record.Monitor.ID)
		}
		want := []string{"mon-bbbbbbbbbbbb", "mon-cccccccccccc", "mon-dddddddddddd"}
		if !slices.Equal(ids, want) {
			t.Fatalf("remaining monitors = %v, want %v: finished rows before the cutoff go, the boundary row and "+
				"occupying rows stay", ids, want)
		}
	})
}
