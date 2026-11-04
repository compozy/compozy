package globaldb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/workspaceaccess"

	automation "github.com/compozy/compozy/internal/automation/model"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
	"github.com/compozy/compozy/internal/store/telemetrydb"
	"github.com/compozy/compozy/internal/testutil"
	compozyworkspace "github.com/compozy/compozy/internal/workspace"
)

func nilGlobalContext() context.Context {
	return nil
}

func TestGlobalDBPathAndCloseVariants(t *testing.T) {
	t.Parallel()

	var nilDB *GlobalDB
	if got := nilDB.Path(); got != "" {
		t.Fatalf("nil Path() = %q, want empty", got)
	}
	if err := nilDB.Close(testutil.Context(t)); err != nil {
		t.Fatalf("nil Close() error = %v", err)
	}

	globalDB := openTestGlobalDB(t)
	if got, want := globalDB.Path(), globalDB.path; got != want {
		t.Fatalf("Path() = %q, want %q", got, want)
	}
	if err := globalDB.Close(nilGlobalContext()); err == nil {
		t.Fatal("Close(nil ctx) error = nil, want non-nil")
	}
}

func TestGlobalDBTransactionCleanupHelpers(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	db, err := store.OpenSQLiteDatabase(ctx, filepath.Join(t.TempDir(), "cleanup.db"), nil)
	if err != nil {
		t.Fatalf("OpenSQLiteDatabase() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Errorf("db.Close() error = %v", closeErr)
		}
	})

	if err := rollbackTx(nil, "nil"); err != nil {
		t.Fatalf("rollbackTx(nil) error = %v", err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if err := rollbackTx(tx, "committed"); err != nil {
		t.Fatalf("rollbackTx(committed) error = %v", err)
	}

	if err := restoreForeignKeys(ctx, nil); err != nil {
		t.Fatalf("restoreForeignKeys(nil) error = %v", err)
	}
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("Conn() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := conn.Close(); closeErr != nil {
			t.Errorf("conn.Close() error = %v", closeErr)
		}
	})
	if err := restoreForeignKeys(ctx, conn); err != nil {
		t.Fatalf("restoreForeignKeys(conn) error = %v", err)
	}

	primaryErr := errors.New("primary")
	cleanupErr := errors.New("cleanup")
	joinCleanupError(nil, cleanupErr)
	var target error
	joinCleanupError(&target, nil)
	if target != nil {
		t.Fatalf("joinCleanupError(nil cleanup) = %v, want nil", target)
	}
	joinCleanupError(&target, cleanupErr)
	if !errors.Is(target, cleanupErr) {
		t.Fatalf("joinCleanupError(cleanup only) = %v, want cleanup", target)
	}
	target = primaryErr
	joinCleanupError(&target, cleanupErr)
	if !errors.Is(target, primaryErr) || !errors.Is(target, cleanupErr) {
		t.Fatalf("joinCleanupError(joined) = %v, want primary and cleanup", target)
	}
}

func TestGlobalDBGuardClauses(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	if err := globalDB.RegisterSession(
		nilGlobalContext(),
		new(SessionInfo{ProfileID: store.DefaultProfileID}),
	); err == nil {
		t.Fatal("RegisterSession(nil ctx) error = nil, want non-nil")
	}
	if _, err := globalDB.ListSessions(nilGlobalContext(), SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
	}); err == nil {
		t.Fatal("ListSessions(nil ctx) error = nil, want non-nil")
	}
	if err := globalDB.WriteEventSummary(nilGlobalContext(), EventSummary{}); err == nil {
		t.Fatal("WriteEventSummary(nil ctx) error = nil, want non-nil")
	}
	if _, err := globalDB.ListEventSummaries(
		nilGlobalContext(),
		EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}},
	); err == nil {
		t.Fatal("ListEventSummaries(nil ctx) error = nil, want non-nil")
	}
	if err := globalDB.UpdateTokenStats(nilGlobalContext(), TokenStatsUpdate{}); err == nil {
		t.Fatal("UpdateTokenStats(nil ctx) error = nil, want non-nil")
	}
	if _, err := globalDB.ListTokenStats(nilGlobalContext(), TokenStatsQuery{}); err == nil {
		t.Fatal("ListTokenStats(nil ctx) error = nil, want non-nil")
	}
	if err := globalDB.WritePermissionLog(nilGlobalContext(), PermissionLogEntry{}); err == nil {
		t.Fatal("WritePermissionLog(nil ctx) error = nil, want non-nil")
	}
	if _, err := globalDB.ListPermissionLog(nilGlobalContext(), PermissionLogQuery{}); err == nil {
		t.Fatal("ListPermissionLog(nil ctx) error = nil, want non-nil")
	}
}

func TestGlobalDBWorkspaceAndAutomationGuardClauses(t *testing.T) {
	t.Parallel()

	assertErr := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s error = nil, want non-nil", name)
		}
	}

	globalDB := openTestGlobalDB(t)
	assertErr("InsertWorkspace(nil ctx)", globalDB.InsertWorkspace(nilGlobalContext(), compozyworkspace.Workspace{}))
	assertErr("UpdateWorkspace(nil ctx)", globalDB.UpdateWorkspace(nilGlobalContext(), compozyworkspace.Workspace{}))
	assertErr("DeleteWorkspace(nil ctx)", globalDB.DeleteWorkspace(nilGlobalContext(), "ws-1"))
	_, err := globalDB.GetWorkspace(nilGlobalContext(), "ws-1")
	assertErr("GetWorkspace(nil ctx)", err)
	_, err = globalDB.ListWorkspaces(nilGlobalContext())
	assertErr("ListWorkspaces(nil ctx)", err)
	_, err = globalDB.CreateJob(nilGlobalContext(), Job{})
	assertErr("CreateJob(nil ctx)", err)
	_, err = globalDB.CreateTrigger(nilGlobalContext(), Trigger{})
	assertErr("CreateTrigger(nil ctx)", err)
	_, err = globalDB.CreateRun(nilGlobalContext(), Run{ProfileID: store.DefaultProfileID})
	assertErr("CreateRun(nil ctx)", err)
	_, err = globalDB.ReserveRun(nilGlobalContext(), automation.RunReservation{})
	assertErr("ReserveRun(nil ctx)", err)
	_, err = globalDB.CountRuns(nilGlobalContext(), RunQuery{ReadScope: automationAllProfiles})
	assertErr("CountRuns(nil ctx)", err)
}

func TestGlobalDBDefaultsAndFilteredListings(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	base := time.Date(2026, 4, 4, 13, 0, 0, 0, time.UTC)
	callCount := 0
	globalDB.now = func() time.Time {
		callCount++
		return base.Add(time.Duration(callCount) * time.Minute)
	}

	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"filtered-workspace",
		filepath.Join(t.TempDir(), "filtered-workspace"),
	)
	if err := globalDB.RegisterSession(testutil.Context(t), new(SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            "sess-defaults",
		AgentName:     "coder",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "active",
	})); err != nil {
		t.Fatalf("RegisterSession(defaults) error = %v", err)
	}
	if err := globalDB.RegisterSession(testutil.Context(t), new(SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            "sess-reviewer",
		AgentName:     "reviewer",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "active",
		CreatedAt:     base.Add(-time.Hour),
		UpdatedAt:     base.Add(-time.Hour),
	})); err != nil {
		t.Fatalf("RegisterSession(reviewer) error = %v", err)
	}

	sessions, err := globalDB.ListSessions(testutil.Context(t), SessionListQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
		AgentName: "coder",
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("ListSessions(filtered) error = %v", err)
	}
	if got, want := len(sessions), 1; got != want {
		t.Fatalf("len(sessions) = %d, want %d", got, want)
	}
	if got, want := sessions[0].SessionType, defaultSessionType; got != want {
		t.Fatalf("sessions[0].SessionType = %q, want %q", got, want)
	}

	if err := globalDB.WriteEventSummary(testutil.Context(t), EventSummary{
		ProfileID: store.DefaultProfileID,
		SessionID: "sess-defaults",
		Type:      "agent_message",
		AgentName: "coder",
	}); err != nil {
		t.Fatalf("WriteEventSummary(default timestamp) error = %v", err)
	}
	if err := globalDB.WriteEventSummary(testutil.Context(t), EventSummary{
		ProfileID: store.DefaultProfileID,
		SessionID: "sess-reviewer",
		Type:      "tool_call",
		AgentName: "reviewer",
		Timestamp: base.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("WriteEventSummary(explicit timestamp) error = %v", err)
	}

	summaries, err := globalDB.ListEventSummaries(
		testutil.Context(t),
		EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true},
			AgentName: "coder",
			Type:      "agent_message",
			Since:     base,
			Limit:     1,
		},
	)
	if err != nil {
		t.Fatalf("ListEventSummaries(filtered) error = %v", err)
	}
	if got, want := len(summaries), 1; got != want {
		t.Fatalf("len(summaries) = %d, want %d", got, want)
	}
	if got, want := summaries[0].AgentName, "coder"; got != want {
		t.Fatalf("summaries[0].AgentName = %q, want %q", got, want)
	}

	if err := globalDB.UpdateTokenStats(testutil.Context(t), TokenStatsUpdate{
		SessionID:  "sess-defaults",
		AgentName:  "coder",
		CostStatus: "unknown",
		CostSource: "none",
	}); err != nil {
		t.Fatalf("UpdateTokenStats(default turns) error = %v", err)
	}
	stats, err := globalDB.ListTokenStats(testutil.Context(t), TokenStatsQuery{
		SessionID: "sess-defaults",
		AgentName: "coder",
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("ListTokenStats(filtered) error = %v", err)
	}
	if got, want := len(stats), 1; got != want {
		t.Fatalf("len(stats) = %d, want %d", got, want)
	}
	if got, want := stats[0].TurnCount, int64(1); got != want {
		t.Fatalf("stats[0].TurnCount = %d, want %d", got, want)
	}

	if err := globalDB.WritePermissionLog(testutil.Context(t), PermissionLogEntry{
		SessionID:  "sess-defaults",
		AgentName:  "coder",
		Action:     "bash",
		Resource:   "/tmp/a",
		Decision:   "allow",
		PolicyUsed: "approve-reads",
	}); err != nil {
		t.Fatalf("WritePermissionLog(default timestamp) error = %v", err)
	}
	if err := globalDB.WritePermissionLog(testutil.Context(t), PermissionLogEntry{
		SessionID:  "sess-reviewer",
		AgentName:  "reviewer",
		Action:     "bash",
		Resource:   "/tmp/b",
		Decision:   "deny",
		PolicyUsed: "permissions",
		Timestamp:  base.Add(-time.Hour),
	}); err != nil {
		t.Fatalf("WritePermissionLog(explicit timestamp) error = %v", err)
	}

	entries, err := globalDB.ListPermissionLog(testutil.Context(t), PermissionLogQuery{
		AgentName: "coder",
		Decision:  "allow",
		Since:     base,
		Limit:     1,
	})
	if err != nil {
		t.Fatalf("ListPermissionLog(filtered) error = %v", err)
	}
	if got, want := len(entries), 1; got != want {
		t.Fatalf("len(entries) = %d, want %d", got, want)
	}
	if got, want := entries[0].Decision, "allow"; got != want {
		t.Fatalf("entries[0].Decision = %q, want %q", got, want)
	}
}

func TestListEventSummariesPreservesHarnessFiltersAndRecentOrdering(t *testing.T) {
	t.Parallel()

	globalDB := openTestGlobalDB(t)
	base := time.Date(2026, 4, 18, 13, 0, 0, 0, time.UTC)

	workspaceID := registerWorkspaceForGlobalTests(
		t,
		globalDB,
		"harness-observe-workspace",
		filepath.Join(t.TempDir(), "harness-observe-workspace"),
	)
	if err := globalDB.RegisterSession(testutil.Context(t), new(SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            "sess-harness",
		AgentName:     "coder",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "active",
		CreatedAt:     base,
		UpdatedAt:     base,
	})); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}

	summaries := []EventSummary{
		{
			ProfileID: store.DefaultProfileID,
			SessionID: "sess-harness",
			Type:      "harness.context_resolved",
			AgentName: "coder",
			Summary:   "surface=startup sections=memory|skills|tools",
			Timestamp: base,
		},
		{
			ProfileID: store.DefaultProfileID,
			SessionID: "sess-harness",
			Type:      "harness.section_selected",
			AgentName: "coder",
			Summary:   "selected=memory|skills|tools count=3",
			Timestamp: base.Add(time.Second),
		},
		{
			ProfileID: store.DefaultProfileID,
			SessionID: "sess-harness",
			Type:      "harness.augmenter_applied",
			AgentName: "coder",
			Summary:   "augmenter=durable_memory outcome=blank",
			Timestamp: base.Add(2 * time.Second),
		},
	}
	for _, summary := range summaries {
		if err := globalDB.WriteEventSummary(testutil.Context(t), summary); err != nil {
			t.Fatalf("WriteEventSummary(%q) error = %v", summary.Type, err)
		}
	}

	filtered, err := globalDB.ListEventSummaries(
		testutil.Context(t),
		EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true},
			SessionID: "sess-harness",
			Type:      "harness.context_resolved",
		},
	)
	if err != nil {
		t.Fatalf("ListEventSummaries(filtered) error = %v", err)
	}
	if got, want := len(filtered), 1; got != want {
		t.Fatalf("len(filtered) = %d, want %d", got, want)
	}
	if got, want := filtered[0].Summary, summaries[0].Summary; got != want {
		t.Fatalf("filtered[0].Summary = %q, want %q", got, want)
	}

	recent, err := globalDB.ListEventSummaries(
		testutil.Context(t),
		EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true},
			SessionID: "sess-harness",
			Limit:     2,
		},
	)
	if err != nil {
		t.Fatalf("ListEventSummaries(recent) error = %v", err)
	}
	if got, want := len(recent), 2; got != want {
		t.Fatalf("len(recent) = %d, want %d", got, want)
	}
	if got, want := recent[0].Type, "harness.section_selected"; got != want {
		t.Fatalf("recent[0].Type = %q, want %q", got, want)
	}
	if got, want := recent[1].Type, "harness.augmenter_applied"; got != want {
		t.Fatalf("recent[1].Type = %q, want %q", got, want)
	}
	if !recent[0].Timestamp.Before(recent[1].Timestamp) {
		t.Fatalf("recent timestamps = [%v %v], want ascending recent order", recent[0].Timestamp, recent[1].Timestamp)
	}

	foreignProfileID := "eeeeeeeeeeeeeeeeeeeeeeeeee"
	if _, err := globalDB.DB().ExecContext(testutil.Context(t), `
		INSERT INTO profiles (id, name, color, icon, state, created_at)
		VALUES (?, 'events-foreign', '#5FBF85', 'circle', 'active', ?)`,
		foreignProfileID,
		store.FormatTimestamp(base),
	); err != nil {
		t.Fatalf("insert foreign profile error = %v", err)
	}
	if err := globalDB.WriteEventSummary(testutil.Context(t), EventSummary{
		ProfileID: foreignProfileID,
		ID:        "sum-foreign-profile",
		Type:      "settings.changed",
		Timestamp: base.Add(3 * time.Second),
	}); err != nil {
		t.Fatalf("WriteEventSummary(foreign profile) error = %v", err)
	}
	scoped, err := globalDB.ListEventSummaries(testutil.Context(t), EventSummaryQuery{
		ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
	})
	if err != nil {
		t.Fatalf("ListEventSummaries(scoped) error = %v", err)
	}
	for _, summary := range scoped {
		if summary.ProfileID == foreignProfileID {
			t.Fatalf("ListEventSummaries(scoped) leaked foreign event: %+v", summary)
		}
	}
	aggregate, err := globalDB.ListEventSummaries(testutil.Context(t), EventSummaryQuery{
		ReadScope: store.ReadScope{AllProfiles: true},
		Type:      "settings.changed",
	})
	if err != nil {
		t.Fatalf("ListEventSummaries(aggregate) error = %v", err)
	}
	if len(aggregate) != 1 || aggregate[0].ProfileName != "events-foreign" ||
		aggregate[0].ProfileColor != "#5FBF85" {
		t.Fatalf("ListEventSummaries(aggregate) = %+v, want owner-labeled foreign event", aggregate)
	}

	// Invariant: recent profile/error scans satisfy timestamp and sequence ordering
	// without sorting retained history or scanning unrelated outcomes.
	// Owner: telemetry indexes. Canonical suite: recent Observe reads above.
	for _, tc := range []struct {
		name  string
		where string
		index string
		args  []any
	}{
		{
			name: "profile", where: "profile_id = ?", index: "idx_summaries_profile_timestamp",
			args: []any{store.DefaultProfileID},
		},
		{
			name: "errors", where: "outcome IN ('failure', 'warning')", index: "idx_summaries_failures_timestamp",
		},
	} {
		t.Run("Should read recent "+tc.name+" through an ordered index", func(t *testing.T) {
			plan := explainQueryPlan(t, globalDB.telemetry.DB(),
				"SELECT seq, id FROM event_summaries WHERE "+tc.where+" ORDER BY timestamp DESC, seq DESC LIMIT 100",
				tc.args...)
			if !strings.Contains(plan, tc.index) || strings.Contains(plan, "TEMP B-TREE") {
				t.Fatalf("recent %s plan = %s, want ordered filtered scan", tc.name, plan)
			}
		})
	}
}

func TestGlobalDBWorkspaceHelperUtilities(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize and validate encoded workspace directories", func(t *testing.T) {
		t.Parallel()

		dirs, err := decodeWorkspaceDirs(`[" /tmp/a ","","/tmp/b"]`)
		if err != nil {
			t.Fatalf("decodeWorkspaceDirs(valid) error = %v", err)
		}
		if !testutil.EqualStringSlices(dirs, []string{"/tmp/a", "/tmp/b"}) {
			t.Fatalf("decodeWorkspaceDirs(valid) = %#v", dirs)
		}
		if _, err := decodeWorkspaceDirs(`{`); err == nil {
			t.Fatal("decodeWorkspaceDirs(invalid) error = nil, want non-nil")
		}
	})
}

// Invariant UT-115: observation cursors preserve old rowids and never reuse a
// sequence after retention, VACUUM or reopen. Owner: global observation store.
func TestGlobalDBEventSummarySequenceMigration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	path := filepath.Join(t.TempDir(), GlobalDatabaseName)
	prefix, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00107_schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := prefix.ExecContext(ctx, `INSERT INTO event_summaries(rowid,id,profile_id,type,summary,timestamp)
 VALUES (7,'old-7',?,'settings.changed','retained seven','2026-07-01T12:00:00Z'),
 (101,'old-101',?,'settings.changed','retained last','2026-06-01T12:00:00Z')`, store.DefaultProfileID, store.DefaultProfileID); err != nil {
		t.Fatal(err)
	}
	if err := prefix.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := openGlobalMigrationUpgrade(t, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	rows, err := db.ListEventSummaries(
		ctx,
		EventSummaryQuery{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, AfterSequence: 7, Limit: 1},
	)
	if err != nil || len(rows) != 1 || rows[0].Sequence != 101 || rows[0].Summary != "retained last" {
		t.Fatalf("resumed migration rows = %+v, %v", rows, err)
	}
	if _, err := db.telemetry.DB().ExecContext(ctx, `DELETE FROM event_summaries`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.telemetry.DB().ExecContext(ctx, `VACUUM`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(ctx); err != nil {
		t.Fatal(err)
	}
	db, err = OpenGlobalDB(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.WriteEventSummaries(
		ctx,
		[]store.EventSummary{
			{
				ProfileID: store.DefaultProfileID,
				ID:        "new",
				Type:      "settings.changed",
				Summary:   "new",
				Timestamp: time.Now(),
			},
		},
	); err != nil {
		t.Fatal(err)
	}
	rows, err = db.ListEventSummaries(
		ctx,
		EventSummaryQuery{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, AfterSequence: 101, Limit: 1},
	)
	if err != nil || len(rows) != 1 || rows[0].Sequence <= 101 {
		t.Fatalf("cursor reused after sweep/reopen: %+v, %v", rows, err)
	}
}

func TestUserEventSummarySelection(t *testing.T) {
	t.Parallel()
	t.Run("Should select only User events within the requested profile", func(t *testing.T) {
		t.Parallel()
		db := openTestGlobalDB(t)
		ctx := t.Context()
		at := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
		foreign := "eeeeeeeeeeeeeeeeeeeeeeeeee"
		if _, err := db.DB().
			ExecContext(ctx, `INSERT INTO profiles (id,name,color,icon,state,created_at) VALUES (?, 'user-events-foreign','#5FBF85','circle','active',?)`, foreign, store.FormatTimestamp(at)); err != nil {
			t.Fatal(err)
		}
		workspace := registerWorkspaceForGlobalTests(t, db, "event-project", t.TempDir())
		for _, event := range []EventSummary{
			{ID: "user-default", ProfileID: store.DefaultProfileID, Type: "workspace.access_granted", Timestamp: at},
			{ID: "user-foreign", ProfileID: foreign, Type: "workspace.access_granted", Timestamp: at},
			{ID: "project-default", ProfileID: store.DefaultProfileID, WorkspaceID: workspace, Type: "workspace.access_granted", Timestamp: at},
		} {
			if err := db.WriteEventSummary(ctx, event); err != nil {
				t.Fatal(err)
			}
		}
		rows, err := db.ListEventSummaries(
			ctx,
			EventSummaryQuery{
				ReadScope:   store.ReadScope{ProfileID: store.DefaultProfileID},
				WorkspaceID: workspaceaccess.UserSelector,
			},
		)
		if err != nil || len(rows) != 1 || rows[0].ID != "user-default" {
			t.Fatalf("User events = %#v, %v", rows, err)
		}
		rows, err = db.ListEventSummaries(
			ctx,
			EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: workspaceaccess.UserSelector},
		)
		if err != nil || len(rows) != 2 {
			t.Fatalf("all-profile User events = %#v, %v", rows, err)
		}
		rows, err = db.ListEventSummaries(
			ctx,
			EventSummaryQuery{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}},
		)
		if err != nil || len(rows) != 2 {
			t.Fatalf("unfiltered events = %#v, %v", rows, err)
		}
	})
}

// Invariant: relocation preserves every persisted value and the cursor high-water
// mark across a failed acknowledgement. Owner: global migration/open suite.
func TestGlobalDBTelemetryRelocation(t *testing.T) {
	ctx := t.Context()
	// Fresh bootstrap and cross-file recovery share this migration/open owner.
	for _, stage := range []string{"before sibling creation", "after sibling binding", "with a foreign sibling"} {
		t.Run("Should resume fresh telemetry initialization "+stage, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), GlobalDatabaseName)
			source, err := store.OpenSQLiteDatabase(ctx, path, nil)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := source.Close(); err != nil {
					t.Error(err)
				}
			})
			bootstrapped, err := bootstrapGlobalTelemetry(ctx, source)
			if err != nil || !bootstrapped {
				t.Fatalf("fresh bootstrap = %v, %v", bootstrapped, err)
			}
			if pending, err := pendingTelemetryInitialization(ctx, source); err != nil || !pending {
				t.Fatalf("pending fresh initialization = %v, %v", pending, err)
			}
			if stage != "before sibling creation" {
				target, err := telemetrydb.Open(ctx, telemetrydb.Path(path))
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := target.Close(context.WithoutCancel(ctx)); err != nil {
						t.Error(err)
					}
				})
				if stage == "after sibling binding" {
					err = bindTelemetryDestination(ctx, source, target.DB())
				} else {
					_, err = target.DB().
						ExecContext(ctx, "INSERT INTO telemetry_identity(singleton, identity) VALUES (1, 'foreign-family')")
				}
				if err != nil {
					t.Fatal(err)
				}
				if err := target.Close(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if err := source.Close(); err != nil {
				t.Fatal(err)
			}
			opened, err := OpenGlobalDB(ctx, path)
			if stage == "with a foreign sibling" {
				if opened != nil {
					if closeErr := opened.Close(ctx); closeErr != nil {
						t.Error(closeErr)
					}
				}
				if err == nil || !strings.Contains(err.Error(), "does not match the core") {
					t.Fatalf("foreign fresh destination = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := opened.Close(context.WithoutCancel(ctx)); err != nil {
					t.Error(err)
				}
			})
			if pending, err := pendingTelemetryInitialization(ctx, opened.DB()); err != nil || pending {
				t.Fatalf("completed fresh initialization = %v, %v", pending, err)
			}
			if err := verifyTelemetryIdentity(ctx, opened.DB(), opened.telemetry.DB()); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("Should resume committed copies without loss or duplicate cursors", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		source, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00191_telemetry_export_complete.sql"),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := source.Close(); err != nil {
				t.Error(err)
			}
		})
		_, err = source.ExecContext(ctx, `INSERT INTO event_summary_export
		(seq,id,profile_id,session_id,workspace_id,worktree_id,type,agent_name,content_json,task_id,run_id,
		workflow_id,claim_token_hash,lease_until,coordinator_session_id,scheduler_reason,hook_event,hook_name,
		actor_kind,actor_id,release_reason,parent_session_id,root_session_id,spawn_depth,provider,outcome,summary,timestamp)
		VALUES(7,'relocated',?,'session','workspace','worktree','settings.changed','coder','{"value":42}',
		'task','run','workflow','hash','2026-11-03T20:00:00Z','coordinator','scheduled','before','hook',
		'agent','actor','released','parent','root',2,'codex','success','preserved','2026-11-03T19:00:00Z')`, store.DefaultProfileID)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := source.ExecContext(
			ctx,
			`UPDATE sqlite_sequence SET seq = 900 WHERE name = 'event_summary_export'`,
		); err != nil {
			t.Fatal(err)
		}
		columns, expected, err := readTelemetryExportBatch(ctx, source)
		if err != nil {
			t.Fatal(err)
		}
		// Production binds the destination before committing the source freeze.
		target, err := telemetrydb.Open(ctx, telemetrydb.Path(path))
		if err != nil {
			t.Fatal(err)
		}
		if err := bindTelemetryDestination(ctx, source, target.DB()); err != nil {
			t.Fatal(err)
		}
		if err := target.Close(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := source.ExecContext(ctx, `CREATE TRIGGER fail_export_ack BEFORE DELETE ON event_summary_export
		BEGIN SELECT RAISE(ABORT, 'injected export acknowledgement failure'); END`); err != nil {
			t.Fatal(err)
		}
		if err := applyTelemetryRelocation(
			ctx,
			source,
			path,
			telemetryExportVersion,
		); err == nil ||
			!strings.Contains(err.Error(), "injected export acknowledgement failure") {
			t.Fatalf("interrupted export = %v, want retained source after committed target", err)
		}
		var remaining int
		if err := source.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_summary_export").
			Scan(&remaining); err != nil ||
			remaining != 1 {
			t.Fatalf("source after interrupted acknowledgement = %d, %v", remaining, err)
		}
		if _, err := source.ExecContext(ctx, "DROP TRIGGER fail_export_ack"); err != nil {
			t.Fatal(err)
		}
		if err := source.Close(); err != nil {
			t.Fatal(err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := upgraded.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		actual := make([]any, len(columns))
		dest := make([]any, len(columns))
		for i := range actual {
			dest[i] = &actual[i]
		}
		if err := upgraded.telemetry.DB().
			QueryRowContext(ctx, "SELECT * FROM event_summaries WHERE id = 'relocated'").
			Scan(dest...); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, expected[0]) {
			t.Fatalf("relocated payload changed: got %#v, want %#v", actual, expected[0])
		}
		if err := upgraded.WriteEventSummary(
			ctx,
			store.EventSummary{ID: "after-relocation", ProfileID: store.DefaultProfileID, Type: "settings.changed"},
		); err != nil {
			t.Fatal(err)
		}
		rows, err := upgraded.ListEventSummaries(
			ctx,
			store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}, AfterSequence: 900},
		)
		if err != nil || len(rows) != 1 || rows[0].ID != "after-relocation" || rows[0].Sequence != 901 {
			t.Fatalf("resumed cursor = %#v, %v", rows, err)
		}
	})
	t.Run("Should refuse a lost destination after acknowledging an export batch", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		source, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00189_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := source.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := source.ExecContext(ctx, `WITH RECURSIVE rows(n) AS (
			SELECT 1 UNION ALL SELECT n+1 FROM rows WHERE n<129
		) INSERT INTO event_summaries(seq,id,profile_id,type,timestamp)
		SELECT n,'export-'||n,?,'settings.changed','2026-11-03T19:00:00Z' FROM rows`, store.DefaultProfileID); err != nil {
			t.Fatal(err)
		}
		if _, err := source.ExecContext(ctx, `CREATE TRIGGER fail_export_tail BEFORE DELETE ON event_summaries
			WHEN OLD.seq>128 BEGIN SELECT RAISE(ABORT, 'export tail interrupted'); END`); err != nil {
			t.Fatal(err)
		}
		if err := applyTelemetryRelocation(ctx, source, path, 185); err == nil ||
			!strings.Contains(err.Error(), "export tail interrupted") {
			t.Fatalf("export interruption = %v", err)
		}
		var remaining int
		if err := source.QueryRowContext(ctx, "SELECT count(*) FROM event_summary_export").
			Scan(&remaining); err != nil ||
			remaining != 1 {
			t.Fatalf("unacknowledged rows = %d, %v; want 1", remaining, err)
		}
		if _, err := source.ExecContext(ctx, "DROP TRIGGER fail_export_tail"); err != nil {
			t.Fatal(err)
		}
		if err := source.Close(); err != nil {
			t.Fatal(err)
		}
		target := telemetrydb.Path(path)
		if err := os.Rename(target, target+".preserved"); err != nil {
			t.Fatal(err)
		}
		opened, err := openGlobalMigrationUpgrade(t, path)
		if err == nil {
			if closeErr := opened.Close(ctx); closeErr != nil {
				t.Error(closeErr)
			}
			t.Fatal("interrupted export recreated a missing destination")
		}
		if !strings.Contains(err.Error(), "telemetry database is required") {
			t.Fatalf("missing destination diagnostic = %v", err)
		}
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing destination was changed: %v", err)
		}
		if err := os.WriteFile(target, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		opened, err = openGlobalMigrationUpgrade(t, path)
		if err == nil {
			if closeErr := opened.Close(ctx); closeErr != nil {
				t.Error(closeErr)
			}
			t.Fatal("interrupted export accepted an empty replacement")
		}
		if !strings.Contains(err.Error(), "telemetry database is uninitialized") {
			t.Fatalf("empty destination diagnostic = %v", err)
		}
		replacement, err := telemetrydb.Open(ctx, target)
		if err != nil {
			t.Fatal(err)
		}
		if err := replacement.Close(ctx); err != nil {
			t.Fatal(err)
		}
		opened, err = openGlobalMigrationUpgrade(t, path)
		if err == nil {
			if closeErr := opened.Close(ctx); closeErr != nil {
				t.Error(closeErr)
			}
			t.Fatal("interrupted export accepted an initialized replacement")
		}
		if !strings.Contains(err.Error(), "telemetry identity is required") {
			t.Fatalf("initialized replacement diagnostic = %v", err)
		}
		if err := os.Remove(target); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(target+".preserved", target); err != nil {
			t.Fatal(err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := upgraded.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		var count, highWater int
		if err := upgraded.readDB.QueryRowContext(ctx, "SELECT count(*),max(seq) FROM event_summaries").
			Scan(&count, &highWater); err != nil || count != 129 ||
			highWater != 129 {
			t.Fatalf("restored export = %d rows through %d, %v", count, highWater, err)
		}
	})
	t.Run("Should preserve a retired cursor when every source row was deleted", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		source, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00189_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := source.ExecContext(
			ctx,
			`INSERT INTO event_summaries(seq,id,profile_id,type,timestamp) VALUES(420,'retired',?,'settings.changed','2026-11-03T19:00:00Z')`,
			store.DefaultProfileID,
		); err != nil {
			t.Fatal(err)
		}
		if _, err := source.ExecContext(ctx, "DELETE FROM event_summaries"); err != nil {
			t.Fatal(err)
		}
		if err := source.Close(); err != nil {
			t.Fatal(err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := upgraded.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if err := upgraded.WriteEventSummary(
			ctx,
			store.EventSummary{ID: "new", ProfileID: store.DefaultProfileID, Type: "settings.changed"},
		); err != nil {
			t.Fatal(err)
		}
		rows, err := upgraded.ListEventSummaries(
			ctx,
			store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}, AfterSequence: 420},
		)
		if err != nil || len(rows) != 1 || rows[0].Sequence != 421 {
			t.Fatalf("retired cursor = %#v, %v", rows, err)
		}
	})
	t.Run("Should refuse a missing relocated file without replacing retained telemetry", func(t *testing.T) {
		db := openTestGlobalDB(t)
		path := db.Path()
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
		target := telemetrydb.Path(path)
		if err := os.Rename(target, target+".preserved"); err != nil {
			t.Fatal(err)
		}
		opened, err := OpenGlobalDB(ctx, path)
		if err == nil {
			if closeErr := opened.Close(ctx); closeErr != nil {
				t.Error(closeErr)
			}
			t.Fatal("missing telemetry silently recreated")
		}
		if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("missing file was changed: %v", err)
		}
	})
	t.Run("Should refuse a conflicting destination without deleting the source", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		source, err := openGlobalMigrationPrefixDatabase(
			t,
			path,
			globalMigrationPrefixBefore(t, "00191_telemetry_export_complete.sql"),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := source.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := source.ExecContext(
			ctx,
			`INSERT INTO event_summary_export(seq,id,profile_id,type,summary,timestamp) VALUES(1,'collision',?,'settings.changed','source','2026-11-03T19:00:00Z')`,
			store.DefaultProfileID,
		); err != nil {
			t.Fatal(err)
		}
		target, err := telemetrydb.Open(ctx, telemetrydb.Path(path))
		if err != nil {
			t.Fatal(err)
		}
		if err := bindTelemetryDestination(ctx, source, target.DB()); err != nil {
			t.Fatal(err)
		}
		if _, err := target.DB().
			ExecContext(ctx, `INSERT INTO event_summaries(seq,id,profile_id,type,summary,timestamp) VALUES(1,'collision',?,'settings.changed','other','2026-11-03T19:00:00Z')`, store.DefaultProfileID); err != nil {
			t.Fatal(err)
		}
		if err := target.Close(ctx); err != nil {
			t.Fatal(err)
		}
		if err := applyTelemetryRelocation(ctx, source, path, telemetryExportVersion); err == nil ||
			!strings.Contains(err.Error(), "collides with different stored values") {
			t.Fatalf("conflicting payload diagnostic = %v", err)
		}
		var summary string
		if err := source.QueryRowContext(ctx, "SELECT summary FROM event_summary_export WHERE id = 'collision'").
			Scan(&summary); err != nil ||
			summary != "source" {
			t.Fatalf("preserved source = %q, %v", summary, err)
		}
	})
	t.Run("Should refuse telemetry belonging to another core after a completed upgrade", func(t *testing.T) {
		db := openTestGlobalDB(t)
		if err := db.WriteEventSummary(ctx, store.EventSummary{
			ID: "retained", ProfileID: store.DefaultProfileID, Type: "settings.changed", Summary: "original",
		}); err != nil {
			t.Fatal(err)
		}
		path := db.Path()
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
		other, err := OpenGlobalDB(ctx, filepath.Join(t.TempDir(), GlobalDatabaseName))
		if err != nil {
			t.Fatal(err)
		}
		otherPath := telemetrydb.Path(other.Path())
		if err := other.Close(ctx); err != nil {
			t.Fatal(err)
		}
		target := telemetrydb.Path(path)
		if err := os.Rename(target, target+".preserved"); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(otherPath, target); err != nil {
			t.Fatal(err)
		}
		opened, err := OpenGlobalDB(ctx, path)
		if err == nil {
			if closeErr := opened.Close(ctx); closeErr != nil {
				t.Error(closeErr)
			}
			t.Fatal("completed upgrade accepted another core's telemetry")
		}
		if !strings.Contains(err.Error(), "telemetry identity does not match") {
			t.Fatalf("replacement diagnostic = %v", err)
		}
		if err := os.Remove(target); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(target+".preserved", target); err != nil {
			t.Fatal(err)
		}
		opened, err = OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := opened.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		rows, err := opened.ListEventSummaries(ctx, store.EventSummaryQuery{
			ReadScope: store.ReadScope{AllProfiles: true},
		})
		if err != nil || len(rows) != 1 || rows[0].ID != "retained" || rows[0].Summary != "original" {
			t.Fatalf("restored original = %#v, %v", rows, err)
		}
	})
	t.Run("Should resume binding before freezing or deleting the source", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), GlobalDatabaseName)
		source, err := openGlobalMigrationPrefixDatabase(t, path, globalMigrationPrefixBefore(t, "00189_schema.sql"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := source.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := source.ExecContext(
			ctx,
			`INSERT INTO event_summaries(id,profile_id,type,timestamp) VALUES('binding',?,'settings.changed','2026-11-03T19:00:00Z')`,
			store.DefaultProfileID,
		); err != nil {
			t.Fatal(err)
		}
		target, err := telemetrydb.Open(ctx, telemetrydb.Path(path))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := target.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		if _, err := target.DB().ExecContext(ctx, `CREATE TRIGGER interrupt_binding BEFORE INSERT ON telemetry_identity
			BEGIN SELECT RAISE(ABORT, 'binding interrupted'); END`); err != nil {
			t.Fatal(err)
		}
		if err := applyTelemetryRelocation(ctx, source, path, 185); err == nil ||
			!strings.Contains(err.Error(), "binding interrupted") {
			t.Fatalf("interrupted binding = %v", err)
		}
		var count int
		if err := source.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_summaries").
			Scan(&count); err != nil ||
			count != 1 {
			t.Fatalf("source before binding = %d, %v", count, err)
		}
		if _, err := target.DB().ExecContext(ctx, "DROP TRIGGER interrupt_binding"); err != nil {
			t.Fatal(err)
		}
		if err := target.Close(ctx); err != nil {
			t.Fatal(err)
		}
		if err := source.Close(); err != nil {
			t.Fatal(err)
		}
		upgraded, err := openGlobalMigrationUpgrade(t, path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := upgraded.Close(context.Background()); err != nil {
				t.Error(err)
			}
		})
		rows, err := upgraded.ListEventSummaries(ctx, store.EventSummaryQuery{
			ReadScope: store.ReadScope{AllProfiles: true},
		})
		if err != nil || len(rows) != 1 || rows[0].ID != "binding" {
			t.Fatalf("resumed binding = %#v, %v", rows, err)
		}
	})
}

// Invariant: core contention cannot stall telemetry; committed lifecycle events
// survive failed delivery and replay once. Owner: global observation store.
func TestGlobalDBTelemetryDelivery(t *testing.T) {
	t.Run("Should wait only for publication attempts visible to the reader scope", func(t *testing.T) {
		db := openTestGlobalDB(t)
		workspaceA := registerWorkspaceForGlobalTests(t, db, "publication-a", t.TempDir())
		workspaceB := registerWorkspaceForGlobalTests(t, db, "publication-b", t.TempDir())
		summary := store.EventSummary{ID: "unrelated-publication", ProfileID: store.DefaultProfileID,
			WorkspaceID: workspaceB, Type: "settings.changed"}
		summary.SetContent([]byte(`{}`))
		if err := db.WriteEventSummary(t.Context(), summary); err != nil {
			t.Fatal(err)
		}
		// Hold the real publication barrier at the core-commit/auxiliary-commit
		// boundary. Public readers must not inherit another scope's pending work.
		finish := db.ObserveRepo.summaryPublications.begin(store.DefaultProfileID, workspaceA)
		defer finish()
		for _, query := range []store.EventSummaryQuery{
			{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, WorkspaceID: workspaceB},
			{ReadScope: store.ReadScope{AllProfiles: true}, WorkspaceID: workspaceB},
			{ReadScope: store.ReadScope{ProfileID: "unrelated-profile"}},
		} {
			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			rows, err := db.ListEventSummaries(ctx, query)
			cancel()
			if err != nil {
				t.Fatalf("unrelated scope blocked: %#v: %v", query, err)
			}
			if query.WorkspaceID == workspaceB && (len(rows) != 1 || rows[0].ID != summary.ID) {
				t.Fatalf("unrelated scope lost committed rows: %#v", rows)
			}
		}
		watch := looppkg.WatchEventsQuery{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
			WorkspaceID: workspaceB, Streams: map[string]int64{looppkg.WatchEventsObserveStream: 0},
			Kinds: []string{summary.Type}, Limit: 10}
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		cursors, err := db.ReadCursors(ctx, watch)
		if err != nil || cursors[looppkg.WatchEventsObserveStream] == 0 {
			t.Fatalf("unrelated watch cursor = %#v, %v", cursors, err)
		}
		matches, err := db.ReadMatches(ctx, watch)
		if err != nil || len(matches) != 1 {
			t.Fatalf("unrelated watch events = %#v, %v", matches, err)
		}
		for _, query := range []store.EventSummaryQuery{
			{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, WorkspaceID: workspaceA},
			{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}},
			{ReadScope: store.ReadScope{AllProfiles: true}},
		} {
			waiting, stop := context.WithTimeout(t.Context(), 20*time.Millisecond)
			_, err := db.ListEventSummaries(waiting, query)
			stop()
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("matching scope bypassed publication or cancellation: %v", err)
			}
		}
	})
	t.Run("Should read and append events while the entire core writer pool is reserved", func(t *testing.T) {
		db := openTestGlobalDB(t)
		db.db.SetMaxOpenConns(1)
		conn, err := db.db.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		if _, err := conn.ExecContext(t.Context(), "BEGIN IMMEDIATE"); err != nil {
			t.Fatal(err)
		}
		defer conn.ExecContext(context.Background(), "ROLLBACK")
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		if err := db.WriteEventSummary(
			ctx,
			store.EventSummary{ID: "independent", ProfileID: store.DefaultProfileID, Type: "settings.changed"},
		); err != nil {
			t.Fatal(err)
		}
		rows, err := db.ListEventSummaries(ctx, store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}})
		if err != nil || len(rows) != 1 || rows[0].ID != "independent" {
			t.Fatalf("independent Observe read = %#v, %v", rows, err)
		}
		// Force another physical read connection to initialize its attachment and guards.
		reader, err := db.readDB.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		var count int
		if err := db.readDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_summaries").
			Scan(&count); err != nil ||
			count != 1 {
			t.Fatalf("recycled view = %d, %v", count, err)
		}
		if _, err := db.readDB.ExecContext(ctx, "DELETE FROM telemetry.event_summaries"); err == nil {
			t.Fatal("read pool permitted telemetry mutation")
		}
	})
	t.Run("Should hide a deleted workspace before physical telemetry cleanup", func(t *testing.T) {
		db := openTestGlobalDB(t)
		ctx := t.Context()
		workspaceID := registerWorkspaceForGlobalTests(t, db, "telemetry-deletion", t.TempDir())
		summary := store.EventSummary{
			ID:          "deleted-workspace-event",
			ProfileID:   store.DefaultProfileID,
			WorkspaceID: workspaceID,
			Type:        "settings.changed",
		}
		if err := db.WriteEventSummary(ctx, summary); err != nil {
			t.Fatal(err)
		}
		if err := db.StageWorkspaceDeletion(ctx, workspaceID); err != nil {
			t.Fatal(err)
		}
		var physical int
		if err := db.telemetry.DB().
			QueryRowContext(ctx, "SELECT COUNT(*) FROM event_summaries").
			Scan(&physical); err != nil ||
			physical != 1 {
			t.Fatalf("pending cleanup = %d, %v", physical, err)
		}
		rows, err := db.ListEventSummaries(ctx, store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}})
		if err != nil || len(rows) != 0 {
			t.Fatalf("deleted workspace leaked events: %#v, %v", rows, err)
		}
		summary.ID = "late"
		if err := db.WriteEventSummary(ctx, summary); err == nil {
			t.Fatal("late observation accepted for deleted workspace")
		}
		if err := db.CompleteWorkspaceDeletion(ctx, workspaceID); err != nil {
			t.Fatal(err)
		}
		if err := db.telemetry.DB().
			QueryRowContext(ctx, "SELECT COUNT(*) FROM event_summaries").
			Scan(&physical); err != nil ||
			physical != 0 {
			t.Fatalf("final cleanup = %d, %v", physical, err)
		}
	})
	t.Run("Should retain and replay a committed outbox after delivery fails", func(t *testing.T) {
		db := openTestGlobalDB(t)
		ctx := t.Context()
		summary := store.EventSummary{
			ID:        "outbox-delivery",
			ProfileID: store.DefaultProfileID,
			Type:      "settings.changed",
		}
		if err := db.prepareEventSummary(ctx, &summary); err != nil {
			t.Fatal(err)
		}
		if _, err := db.telemetry.DB().
			ExecContext(ctx, `CREATE TRIGGER fail_delivery BEFORE INSERT ON event_summaries BEGIN SELECT RAISE(ABORT,'injected telemetry delivery failure'); END`); err != nil {
			t.Fatal(err)
		}
		if err := store.ExecuteWriteOperation(
			ctx,
			db.db,
			"commit lifecycle outbox",
			func(ctx context.Context, tx *store.WriteTx) error {
				return insertEventSummary(ctx, sqlcgen.New(tx), summary)
			},
		); err != nil {
			t.Fatal(err)
		}
		if err := db.ObserveRepo.flushEventSummaryOutbox(ctx); err == nil {
			t.Fatal("delivery failure ignored")
		}
		var pending int
		if err := db.readDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_summary_outbox").
			Scan(&pending); err != nil ||
			pending != 1 {
			t.Fatalf("durable outbox = %d, %v", pending, err)
		}
		if _, err := db.telemetry.DB().ExecContext(ctx, "DROP TRIGGER fail_delivery"); err != nil {
			t.Fatal(err)
		}
		// Simulate target commit followed by interruption before source acknowledgement.
		if err := errors.Join(db.telemetry.WriteOutbox(ctx, []store.EventSummary{summary})...); err != nil {
			t.Fatal(err)
		}
		path := db.Path()
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
		reopened := openGlobalDBForTest(t, path)
		rows, err := reopened.ListEventSummaries(
			ctx,
			store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}},
		)
		if err != nil || len(rows) != 1 || rows[0].ID != summary.ID {
			t.Fatalf("replayed events = %#v, %v", rows, err)
		}
		if err := reopened.readDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_summary_outbox").
			Scan(&pending); err != nil ||
			pending != 0 {
			t.Fatalf("acknowledged outbox = %d, %v", pending, err)
		}
	})
	t.Run("Should bound each outbox pass and drain the remaining backlog on clean close", func(t *testing.T) {
		db := openTestGlobalDB(t)
		ctx := t.Context()
		db.summaryRetry.cancel()
		<-db.summaryRetry.done
		if _, err := db.db.ExecContext(ctx, `WITH RECURSIVE rows(n) AS (
			SELECT 1 UNION ALL SELECT n+1 FROM rows WHERE n<129
		) INSERT INTO event_summary_outbox(id,profile_id,type,timestamp)
		SELECT 'batch-'||n,?,'settings.changed','2026-11-03T19:00:00Z' FROM rows`, store.DefaultProfileID); err != nil {
			t.Fatal(err)
		}
		if err := db.ObserveRepo.flushEventSummaryOutbox(ctx); err != nil {
			t.Fatal(err)
		}
		var pending int
		if err := db.readDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_summary_outbox").
			Scan(&pending); err != nil ||
			pending != 1 {
			t.Fatalf("remaining bounded backlog = %d, %v; want one row", pending, err)
		}
		path := db.Path()
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
		reopened := openGlobalDBForTest(t, path)
		rows, err := reopened.ListEventSummaries(
			ctx,
			store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}},
		)
		if err != nil || len(rows) != 129 {
			t.Fatalf("events after clean close = %d, %v; want 129", len(rows), err)
		}
	})
	t.Run("Should acknowledge successful outbox items when another item fails", func(t *testing.T) {
		db := openTestGlobalDB(t)
		ctx := t.Context()
		db.summaryRetry.cancel()
		<-db.summaryRetry.done
		if _, err := db.db.ExecContext(ctx, `INSERT INTO event_summary_outbox(id,profile_id,type,timestamp)
			VALUES('failing',?,'settings.changed','2026-11-03T19:00:00Z'),
			('successful',?,'settings.changed','2026-11-03T19:00:00Z')`, store.DefaultProfileID, store.DefaultProfileID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.telemetry.DB().
			ExecContext(ctx, `CREATE TRIGGER fail_one_delivery BEFORE INSERT ON event_summaries
			WHEN NEW.id='failing' BEGIN SELECT RAISE(ABORT,'one delivery failed'); END`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := db.telemetry.DB().
				ExecContext(context.Background(), "DROP TRIGGER IF EXISTS fail_one_delivery"); err != nil {
				t.Error(err)
			}
		})
		if err := db.ObserveRepo.flushEventSummaryOutbox(
			ctx,
		); err == nil ||
			!strings.Contains(err.Error(), "one delivery failed") {
			t.Fatalf("failed item outcome = %v", err)
		}
		var pending string
		if err := db.readDB.QueryRowContext(ctx, "SELECT id FROM event_summary_outbox").
			Scan(&pending); err != nil ||
			pending != "failing" {
			t.Fatalf("retained item = %q, %v", pending, err)
		}
		rows, err := db.ListEventSummaries(ctx, store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}})
		if err != nil || len(rows) != 1 || rows[0].ID != "successful" {
			t.Fatalf("independent delivery = %#v, %v", rows, err)
		}
	})
	t.Run("Should open with a failed durable delivery and retry it without a restart", func(t *testing.T) {
		db := openTestGlobalDB(t)
		ctx := t.Context()
		db.summaryRetry.cancel()
		<-db.summaryRetry.done
		if _, err := db.db.ExecContext(ctx, `INSERT INTO event_summary_outbox(id,profile_id,type,timestamp)
			VALUES('retry-after-open',?,'settings.changed','2026-11-03T19:00:00Z')`, store.DefaultProfileID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.telemetry.DB().ExecContext(ctx, `CREATE TRIGGER fail_retry BEFORE INSERT ON event_summaries
			BEGIN SELECT RAISE(ABORT,'retry unavailable'); END`); err != nil {
			t.Fatal(err)
		}
		path := db.Path()
		if err := db.Close(ctx); err == nil || !strings.Contains(err.Error(), "retry unavailable") {
			t.Fatalf("clean-close failure = %v, want durable delivery error", err)
		}
		reopened := openGlobalDBForTest(t, path)
		if err := reopened.SetAppMetadata(ctx, "delivery.core-available", "yes"); err != nil {
			t.Fatal(err)
		}
		if _, err := reopened.telemetry.DB().ExecContext(ctx, "DROP TRIGGER fail_retry"); err != nil {
			t.Fatal(err)
		}
		deadline := time.NewTimer(5 * time.Second)
		defer deadline.Stop()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			var pending int
			if err := reopened.readDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM event_summary_outbox").
				Scan(&pending); err != nil {
				t.Fatal(err)
			}
			if pending == 0 {
				break
			}
			select {
			case <-deadline.C:
				t.Fatal("owned retry did not acknowledge the retained event")
			case <-ticker.C:
			}
		}
		rows, err := reopened.ListEventSummaries(
			ctx,
			store.EventSummaryQuery{ReadScope: store.ReadScope{AllProfiles: true}},
		)
		if err != nil || len(rows) != 1 || rows[0].ID != "retry-after-open" {
			t.Fatalf("retry publication = %#v, %v", rows, err)
		}
	})
	t.Run("Should finalize one workspace independently of another failed telemetry purge", func(t *testing.T) {
		db := openTestGlobalDB(t)
		ctx := t.Context()
		workspaceA := registerWorkspaceForGlobalTests(t, db, "purge-a", t.TempDir())
		workspaceB := registerWorkspaceForGlobalTests(t, db, "purge-b", t.TempDir())
		for i, workspaceID := range []string{workspaceA, workspaceB} {
			if err := db.WriteEventSummary(ctx, store.EventSummary{
				ID: fmt.Sprintf(
					"purge-%d",
					i,
				),
				ProfileID:   store.DefaultProfileID,
				WorkspaceID: workspaceID,
				Type:        "settings.changed",
			}); err != nil {
				t.Fatal(err)
			}
			if err := db.StageWorkspaceDeletion(ctx, workspaceID); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.telemetry.DB().
			ExecContext(ctx, `CREATE TRIGGER fail_unrelated_purge BEFORE DELETE ON event_summaries
			WHEN OLD.id='purge-1' BEGIN SELECT RAISE(ABORT,'unrelated purge failed'); END`); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if _, err := db.telemetry.DB().
				ExecContext(context.Background(), "DROP TRIGGER IF EXISTS fail_unrelated_purge"); err != nil {
				t.Error(err)
			}
		})
		if err := db.CompleteWorkspaceDeletion(ctx, workspaceA); err != nil {
			t.Fatal(err)
		}
		if _, err := db.GetWorkspaceDeletionIntent(
			ctx,
			workspaceA,
		); !errors.Is(
			err,
			compozyworkspace.ErrWorkspaceDeletionIntentNotFound,
		) {
			t.Fatalf("completed workspace intent = %v", err)
		}
		if err := db.CompleteWorkspaceDeletion(
			ctx,
			workspaceB,
		); err == nil ||
			!strings.Contains(err.Error(), "unrelated purge failed") {
			t.Fatalf("failed scope cleanup = %v", err)
		}
		if _, err := db.GetWorkspaceDeletionIntent(ctx, workspaceB); err != nil {
			t.Fatal(err)
		}
	})
}
