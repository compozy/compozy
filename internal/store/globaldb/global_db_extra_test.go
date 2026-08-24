package globaldb

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	automation "github.com/compozy/compozy/internal/automation/model"
	"github.com/compozy/compozy/internal/store"
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
	if err := globalDB.RegisterSession(nilGlobalContext(), SessionInfo{ProfileID: store.DefaultProfileID}); err == nil {
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
	if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            "sess-defaults",
		AgentName:     "coder",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "active",
	}); err != nil {
		t.Fatalf("RegisterSession(defaults) error = %v", err)
	}
	if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            "sess-reviewer",
		AgentName:     "reviewer",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "active",
		CreatedAt:     base.Add(-time.Hour),
		UpdatedAt:     base.Add(-time.Hour),
	}); err != nil {
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
		PolicyUsed: "sandbox",
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
	if err := globalDB.RegisterSession(testutil.Context(t), SessionInfo{
		ProfileID:     store.DefaultProfileID,
		ID:            "sess-harness",
		AgentName:     "coder",
		RuntimeStatus: store.SessionRuntimeUnbound,
		WorkspaceID:   workspaceID,
		State:         "active",
		CreatedAt:     base,
		UpdatedAt:     base,
	}); err != nil {
		t.Fatalf("RegisterSession() error = %v", err)
	}

	summaries := []EventSummary{
		{
			ProfileID: store.DefaultProfileID,
			SessionID: "sess-harness",
			Type:      "harness.context_resolved",
			AgentName: "coder",
			Summary:   "surface=startup sections=memory|skills|network",
			Timestamp: base,
		},
		{
			ProfileID: store.DefaultProfileID,
			SessionID: "sess-harness",
			Type:      "harness.section_selected",
			AgentName: "coder",
			Summary:   "selected=memory|skills|network count=3",
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
