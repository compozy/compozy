package globaldb

import (
	"context"
	"testing"
	"time"

	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil"
)

func seedOverviewTask(
	t *testing.T,
	globalDB *GlobalDB,
	taskID string,
	workspaceID any,
	status string,
	closedAt any,
) {
	t.Helper()
	createdAt := store.FormatTimestamp(time.Now().Add(-48 * time.Hour))
	scope := "global"
	if workspaceID != nil {
		scope = "workspace"
	}
	if _, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO tasks (
			id, scope, workspace_id, title, status, created_by_kind, created_by_ref,
			origin_kind, origin_ref, created_at, updated_at, closed_at
		) VALUES (?, ?, ?, ?, ?, 'human', 'tester', 'cli', 'test', ?, ?, ?)`,
		taskID, scope, workspaceID, "Task "+taskID, status, createdAt, createdAt, closedAt,
	); err != nil {
		t.Fatalf("seed task %q error = %v", taskID, err)
	}
}

func seedOverviewRun(
	t *testing.T,
	globalDB *GlobalDB,
	runID string,
	taskID string,
	workspaceID any,
	status string,
	runKind string,
	endedAt any,
) {
	t.Helper()
	queuedAt := store.FormatTimestamp(time.Now().Add(-47 * time.Hour))
	if _, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO task_runs (
			id, task_id, workspace_id, status, attempt, origin_kind, origin_ref,
			run_kind, queued_at, ended_at
		) VALUES (?, ?, ?, ?, 1, 'cli', 'test', ?, ?, ?)`,
		runID, taskID, workspaceID, status, runKind, queuedAt, endedAt,
	); err != nil {
		t.Fatalf("seed run %q error = %v", runID, err)
	}
}

func seedOverviewSession(
	t *testing.T,
	globalDB *GlobalDB,
	sessionID string,
	workspaceID string,
	agentName string,
	sessionType string,
	createdAt time.Time,
	updatedAt time.Time,
) {
	t.Helper()
	seedOverviewSessionWithState(
		t, globalDB, sessionID, workspaceID, agentName, sessionType, "stopped", createdAt, updatedAt,
	)
}

func seedOverviewSessionWithState(
	t *testing.T,
	globalDB *GlobalDB,
	sessionID string,
	workspaceID string,
	agentName string,
	sessionType string,
	state string,
	createdAt time.Time,
	updatedAt time.Time,
) {
	t.Helper()
	if _, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO sessions (
			id, agent_name, workspace_id, session_type, state, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sessionID, agentName, workspaceID, sessionType, state,
		store.FormatTimestamp(createdAt), store.FormatTimestamp(updatedAt),
	); err != nil {
		t.Fatalf("seed session %q error = %v", sessionID, err)
	}
}

func seedOverviewEvent(
	t *testing.T,
	globalDB *GlobalDB,
	eventID string,
	eventType string,
	sessionID string,
	workspaceID string,
	outcome string,
	timestamp time.Time,
) {
	t.Helper()
	summary := store.EventSummary{
		ID:          eventID,
		SessionID:   sessionID,
		Type:        eventType,
		AgentName:   "coder",
		WorkspaceID: workspaceID,
		Outcome:     outcome,
		Timestamp:   timestamp,
	}
	if err := globalDB.WriteEventSummary(testutil.Context(t), summary); err != nil {
		t.Fatalf("WriteEventSummary(%q) error = %v", eventID, err)
	}
}

func TestObserveOverviewTokenUsageDaily(t *testing.T) {
	t.Parallel()

	t.Run("Should add token usage into one daily bucket across upserts", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)

		day := "2026-07-20"
		for range 2 {
			if err := globalDB.UpsertTokenUsageDaily(ctx, store.TokenUsageDailyUpdate{
				Day:          day,
				WorkspaceID:  "ws-usage",
				AgentName:    "writer",
				InputTokens:  new(int64(100)),
				OutputTokens: new(int64(40)),
				TotalTokens:  new(int64(140)),
				CostAmount:   new(float64(0.5)),
				CostCurrency: new(string("USD")),
				CostStatus:   "estimated",
				CostSource:   "models_dev",
				Turns:        1,
				UpdatedAt:    time.Now(),
			}); err != nil {
				t.Fatalf("UpsertTokenUsageDaily() error = %v", err)
			}
		}

		days, err := globalDB.ListTokenUsageByDay(ctx, store.OverviewDayQuery{SinceDay: day})
		if err != nil {
			t.Fatalf("ListTokenUsageByDay() error = %v", err)
		}
		if len(days) != 1 {
			t.Fatalf("ListTokenUsageByDay() len = %d, want 1", len(days))
		}
		if days[0].Day != day || days[0].TotalTokens != 280 || days[0].InputTokens != 200 {
			t.Fatalf("ListTokenUsageByDay() = %+v, want additive bucket for %s", days[0], day)
		}

		groups, err := globalDB.SumTokenUsageCost(ctx, store.OverviewDayQuery{SinceDay: day})
		if err != nil {
			t.Fatalf("SumTokenUsageCost() error = %v", err)
		}
		if len(groups) != 1 || groups[0].CostStatus != "estimated" || groups[0].TotalCost != 1.0 {
			t.Fatalf("SumTokenUsageCost() = %+v, want one estimated group totalling 1.0", groups)
		}
	})

	t.Run("Should roll back token_stats when the daily rollup write fails", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)

		// Force the second statement of RecordTokenUsage (the daily rollup insert) to
		// abort, so the whole transaction — including the token_stats upsert — must roll back.
		// A non-TEMP trigger lives in this unique DB's schema, so it applies to whichever
		// pooled connection RecordTokenUsage acquires (a TEMP trigger would be connection-local).
		if _, err := globalDB.db.ExecContext(ctx,
			`CREATE TRIGGER fail_token_usage_daily BEFORE INSERT ON token_usage_daily `+
				`BEGIN SELECT RAISE(ABORT, 'forced daily rollup failure'); END;`,
		); err != nil {
			t.Fatalf("create abort trigger error = %v", err)
		}
		t.Cleanup(func() {
			if _, err := globalDB.db.ExecContext(
				testutil.Context(t), `DROP TRIGGER IF EXISTS fail_token_usage_daily`,
			); err != nil {
				t.Errorf("drop abort trigger error = %v", err)
			}
		})

		err := globalDB.RecordTokenUsage(ctx,
			store.TokenStatsUpdate{
				SessionID: "sess-atomic", AgentName: "writer",
				TotalTokens: new(int64(140)), CostStatus: "unknown", CostSource: "none",
				UpdatedAt: time.Now(),
			},
			store.TokenUsageDailyUpdate{
				Day: "2026-07-24", AgentName: "writer",
				TotalTokens: new(int64(140)), CostStatus: "unknown", CostSource: "none",
				UpdatedAt: time.Now(),
			},
		)
		if err == nil {
			t.Fatal("RecordTokenUsage() error = nil, want the forced daily rollup failure")
		}

		stats, err := globalDB.ListTokenStats(ctx, store.TokenStatsQuery{SessionID: "sess-atomic", Limit: 10})
		if err != nil {
			t.Fatalf("ListTokenStats() error = %v", err)
		}
		if len(stats) != 0 {
			t.Fatalf("token_stats rows = %d, want 0 (RecordTokenUsage must roll back both writes)", len(stats))
		}

		daily, err := globalDB.ListTokenUsageByDay(ctx, store.OverviewDayQuery{SinceDay: "2026-07-24"})
		if err != nil {
			t.Fatalf("ListTokenUsageByDay() error = %v", err)
		}
		if len(daily) != 0 {
			t.Fatalf("token_usage_daily rows = %d, want 0 (neither write may persist)", len(daily))
		}
	})

	t.Run("Should degrade cost provenance to unknown on mixed provenance upserts", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)

		day := "2026-07-21"
		if err := globalDB.UpsertTokenUsageDaily(ctx, store.TokenUsageDailyUpdate{
			Day: day, AgentName: "writer",
			TotalTokens:  new(int64(100)),
			CostAmount:   new(float64(0.5)),
			CostCurrency: new(string("USD")),
			CostStatus:   "estimated", CostSource: "models_dev",
			UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("UpsertTokenUsageDaily(estimated) error = %v", err)
		}
		if err := globalDB.UpsertTokenUsageDaily(ctx, store.TokenUsageDailyUpdate{
			Day: day, AgentName: "writer",
			TotalTokens: new(int64(50)),
			CostStatus:  "included", CostSource: "none",
			UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("UpsertTokenUsageDaily(included) error = %v", err)
		}

		groups, err := globalDB.SumTokenUsageCost(ctx, store.OverviewDayQuery{SinceDay: day})
		if err != nil {
			t.Fatalf("SumTokenUsageCost() error = %v", err)
		}
		if len(groups) != 1 || groups[0].CostStatus != "unknown" || groups[0].RowsWithoutCost != 1 {
			t.Fatalf("SumTokenUsageCost() = %+v, want one unknown group without cost", groups)
		}
		days, err := globalDB.ListTokenUsageByDay(ctx, store.OverviewDayQuery{SinceDay: day})
		if err != nil {
			t.Fatalf("ListTokenUsageByDay() error = %v", err)
		}
		if len(days) != 1 || days[0].TotalTokens != 150 {
			t.Fatalf("ListTokenUsageByDay() = %+v, want tokens preserved at 150", days)
		}
	})

	t.Run("Should bound usage listings by since day and workspace", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)

		seedRow := func(day string, workspaceID string, agent string, tokens int64) {
			t.Helper()
			if err := globalDB.UpsertTokenUsageDaily(ctx, store.TokenUsageDailyUpdate{
				Day: day, WorkspaceID: workspaceID, AgentName: agent,
				TotalTokens: new(tokens),
				CostStatus:  "unknown", CostSource: "none",
				UpdatedAt: time.Now(),
			}); err != nil {
				t.Fatalf("UpsertTokenUsageDaily(%s) error = %v", day, err)
			}
		}
		seedRow("2026-07-01", "ws-a", "writer", 10)
		seedRow("2026-07-10", "ws-a", "writer", 20)
		seedRow("2026-07-10", "ws-b", "researcher", 40)

		days, err := globalDB.ListTokenUsageByDay(ctx, store.OverviewDayQuery{SinceDay: "2026-07-05"})
		if err != nil {
			t.Fatalf("ListTokenUsageByDay() error = %v", err)
		}
		if len(days) != 1 || days[0].TotalTokens != 60 {
			t.Fatalf("ListTokenUsageByDay(all workspaces) = %+v, want one day summing 60", days)
		}

		scoped, err := globalDB.ListTokenUsageByAgent(ctx, store.OverviewDayQuery{
			SinceDay:    "2026-07-05",
			WorkspaceID: "ws-b",
		})
		if err != nil {
			t.Fatalf("ListTokenUsageByAgent() error = %v", err)
		}
		if len(scoped) != 1 || scoped[0].AgentName != "researcher" || scoped[0].TotalTokens != 40 {
			t.Fatalf("ListTokenUsageByAgent(ws-b) = %+v, want researcher 40", scoped)
		}
	})

	t.Run("Should sweep token usage daily rows older than the cutoff day", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)

		cutoff := time.Now()
		oldDay := store.LocalDay(store.LocalDayStart(cutoff, 3))
		currentDay := store.LocalDay(cutoff)
		for _, day := range []string{oldDay, currentDay} {
			if err := globalDB.UpsertTokenUsageDaily(ctx, store.TokenUsageDailyUpdate{
				Day: day, AgentName: "writer",
				TotalTokens: new(int64(10)),
				CostStatus:  "unknown", CostSource: "none",
				UpdatedAt: cutoff,
			}); err != nil {
				t.Fatalf("UpsertTokenUsageDaily(%s) error = %v", day, err)
			}
		}

		result, err := globalDB.SweepObservability(ctx, cutoff)
		if err != nil {
			t.Fatalf("SweepObservability() error = %v", err)
		}
		if result.DeletedTokenUsageDaily != 1 {
			t.Fatalf("SweepObservability() DeletedTokenUsageDaily = %d, want 1", result.DeletedTokenUsageDaily)
		}
		days, err := globalDB.ListTokenUsageByDay(ctx, store.OverviewDayQuery{SinceDay: oldDay})
		if err != nil {
			t.Fatalf("ListTokenUsageByDay() error = %v", err)
		}
		if len(days) != 1 || days[0].Day != currentDay {
			t.Fatalf("ListTokenUsageByDay() = %+v, want only %s to survive", days, currentDay)
		}
	})

	t.Run("Should preserve rollup rows across reopen", func(t *testing.T) {
		t.Parallel()
		ctx := testutil.Context(t)
		path := t.TempDir() + "/" + GlobalDatabaseName
		copyCurrentSchemaGlobalDBSeed(t, path)

		first, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(first) error = %v", err)
		}
		if err := first.UpsertTokenUsageDaily(ctx, store.TokenUsageDailyUpdate{
			Day: "2026-07-22", AgentName: "writer",
			TotalTokens: new(int64(77)),
			CostStatus:  "unknown", CostSource: "none",
			UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("UpsertTokenUsageDaily() error = %v", err)
		}
		if err := first.Close(ctx); err != nil {
			t.Fatalf("Close(first) error = %v", err)
		}

		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if err := reopened.Close(context.Background()); err != nil {
				t.Errorf("Close(reopen) error = %v", err)
			}
		})
		days, err := reopened.ListTokenUsageByDay(ctx, store.OverviewDayQuery{SinceDay: "2026-07-22"})
		if err != nil {
			t.Fatalf("ListTokenUsageByDay(reopen) error = %v", err)
		}
		if len(days) != 1 || days[0].TotalTokens != 77 {
			t.Fatalf("ListTokenUsageByDay(reopen) = %+v, want preserved 77 tokens", days)
		}
	})
}

func TestObserveOverviewRunAndTaskBuckets(t *testing.T) {
	t.Parallel()

	t.Run("Should bucket terminal worker run outcomes by daemon-local day", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)

		now := time.Now()
		today := store.LocalDayStart(now, 0).Add(2 * time.Hour)
		yesterday := store.LocalDayStart(now, 1).Add(3 * time.Hour)
		outside := store.LocalDayStart(now, 20)

		seedOverviewTask(t, globalDB, "task-out-1", nil, "completed", nil)
		seedOverviewRun(t, globalDB, "run-1", "task-out-1", nil, "completed", "worker",
			store.FormatTimestamp(today))
		seedOverviewRun(t, globalDB, "run-2", "task-out-1", nil, "failed", "worker", store.FormatTimestamp(today))
		seedOverviewRun(
			t, globalDB, "run-3", "task-out-1", nil, "canceled", "worker", store.FormatTimestamp(yesterday),
		)
		seedOverviewRun(
			t, globalDB, "run-4", "task-out-1", nil, "completed", "coordinator", store.FormatTimestamp(today),
		)
		seedOverviewRun(t, globalDB, "run-5", "task-out-1", nil, "completed", "worker",
			store.FormatTimestamp(outside))
		seedOverviewRun(t, globalDB, "run-6", "task-out-1", nil, "running", "worker", nil)

		days, err := globalDB.CountTaskRunOutcomesByDay(ctx, store.OverviewSinceQuery{
			Since: store.LocalDayStart(now, 13),
		})
		if err != nil {
			t.Fatalf("CountTaskRunOutcomesByDay() error = %v", err)
		}
		if len(days) != 2 {
			t.Fatalf("CountTaskRunOutcomesByDay() days = %+v, want 2 buckets", days)
		}
		if days[0].Day != store.LocalDay(yesterday) || days[0].Canceled != 1 {
			t.Fatalf("CountTaskRunOutcomesByDay()[0] = %+v, want yesterday canceled bucket", days[0])
		}
		if days[1].Day != store.LocalDay(today) || days[1].Completed != 1 || days[1].Failed != 1 {
			t.Fatalf("CountTaskRunOutcomesByDay()[1] = %+v, want today completed+failed", days[1])
		}
	})

	t.Run("Should scope run outcomes by workspace", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		workspaceID := registerSessionForGlobalTests(t, globalDB, "sess-overview-scope")

		now := time.Now()
		endedAt := store.FormatTimestamp(store.LocalDayStart(now, 0).Add(time.Hour))
		seedOverviewTask(t, globalDB, "task-global", nil, "completed", nil)
		seedOverviewTask(t, globalDB, "task-scoped", workspaceID, "completed", nil)
		seedOverviewRun(t, globalDB, "run-global", "task-global", nil, "completed", "worker", endedAt)
		seedOverviewRun(t, globalDB, "run-scoped", "task-scoped", workspaceID, "completed", "worker", endedAt)

		all, err := globalDB.CountTaskRunOutcomesByDay(ctx, store.OverviewSinceQuery{
			Since: store.LocalDayStart(now, 1),
		})
		if err != nil {
			t.Fatalf("CountTaskRunOutcomesByDay(all) error = %v", err)
		}
		if len(all) != 1 || all[0].Completed != 2 {
			t.Fatalf("CountTaskRunOutcomesByDay(all) = %+v, want 2 completed", all)
		}

		scoped, err := globalDB.CountTaskRunOutcomesByDay(ctx, store.OverviewSinceQuery{
			WorkspaceID: workspaceID,
			Since:       store.LocalDayStart(now, 1),
		})
		if err != nil {
			t.Fatalf("CountTaskRunOutcomesByDay(scoped) error = %v", err)
		}
		if len(scoped) != 1 || scoped[0].Completed != 1 {
			t.Fatalf("CountTaskRunOutcomesByDay(scoped) = %+v, want 1 completed", scoped)
		}
	})

	t.Run("Should count only completed tasks closed inside the window", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)

		now := time.Now()
		closedToday := store.FormatTimestamp(store.LocalDayStart(now, 0).Add(4 * time.Hour))
		seedOverviewTask(t, globalDB, "task-closed", nil, "completed", closedToday)
		seedOverviewTask(t, globalDB, "task-canceled", nil, "canceled", closedToday)
		seedOverviewTask(t, globalDB, "task-open", nil, "in_progress", nil)

		days, err := globalDB.CountTasksClosedByDay(ctx, store.OverviewSinceQuery{
			Since: store.LocalDayStart(now, 0),
		})
		if err != nil {
			t.Fatalf("CountTasksClosedByDay() error = %v", err)
		}
		if len(days) != 1 || days[0].Closed != 1 || days[0].Day != store.LocalDay(now) {
			t.Fatalf("CountTasksClosedByDay() = %+v, want one completed closure today", days)
		}
	})
}

func TestObserveOverviewEventAndSessionAggregates(t *testing.T) {
	t.Parallel()

	t.Run("Should count events by hour and weekday with workspace filter", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		workspaceID := registerSessionForGlobalTests(t, globalDB, "sess-overview-pulse")

		now := time.Now()
		slot := store.LocalDayStart(now, 1).Add(14 * time.Hour)
		local := slot.In(time.Local)
		otherWorkspaceID := registerSessionForGlobalTests(t, globalDB, "sess-overview-pulse-other")
		seedOverviewEvent(t, globalDB, "evt-pulse-1", "message", "sess-overview-pulse", workspaceID, "info", slot)
		seedOverviewEvent(
			t, globalDB,
			"evt-pulse-2", "message", "sess-overview-pulse", workspaceID, "info", slot.Add(10*time.Minute),
		)
		seedOverviewEvent(
			t, globalDB,
			"evt-pulse-3", "message", "sess-overview-pulse-other", otherWorkspaceID, "info", slot,
		)

		buckets, err := globalDB.CountEventsByHourWeekday(ctx, store.OverviewSinceQuery{
			WorkspaceID: workspaceID,
			Since:       store.LocalDayStart(now, 13),
		})
		if err != nil {
			t.Fatalf("CountEventsByHourWeekday() error = %v", err)
		}
		if len(buckets) != 1 {
			t.Fatalf("CountEventsByHourWeekday() = %+v, want a single bucket", buckets)
		}
		if buckets[0].Weekday != int(local.Weekday()) || buckets[0].Hour != local.Hour() || buckets[0].Events != 2 {
			t.Fatalf(
				"CountEventsByHourWeekday() = %+v, want weekday %d hour %d events 2",
				buckets[0], int(local.Weekday()), local.Hour(),
			)
		}
	})

	t.Run("Should count hook dispatch completions and failures", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		hookWorkspaceID := registerSessionForGlobalTests(t, globalDB, "sess-overview-hooks")

		now := time.Now()
		at := store.LocalDayStart(now, 0).Add(time.Hour)
		seedOverviewEvent(
			t, globalDB,
			"evt-hook-1", "hook.dispatch.complete", "sess-overview-hooks", hookWorkspaceID, "success", at,
		)
		seedOverviewEvent(
			t, globalDB,
			"evt-hook-2", "hook.dispatch.complete", "sess-overview-hooks", hookWorkspaceID,
			"failure", at.Add(time.Minute),
		)
		seedOverviewEvent(
			t, globalDB,
			"evt-hook-3", "hook.dispatch.start", "sess-overview-hooks", hookWorkspaceID, "info", at,
		)

		counts, err := globalDB.CountHookDispatchesSince(ctx, store.OverviewSinceQuery{
			Since: store.LocalDayStart(now, 0),
		})
		if err != nil {
			t.Fatalf("CountHookDispatchesSince() error = %v", err)
		}
		if counts.Runs != 2 || counts.Failures != 1 {
			t.Fatalf("CountHookDispatchesSince() = %+v, want 2 runs and 1 failure", counts)
		}
	})

	t.Run("Should report the latest event summary timestamp", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		latestWorkspaceID := registerSessionForGlobalTests(t, globalDB, "sess-overview-latest")

		empty, err := globalDB.LatestEventSummaryAt(ctx, "")
		if err != nil {
			t.Fatalf("LatestEventSummaryAt(empty) error = %v", err)
		}
		if !empty.IsZero() {
			t.Fatalf("LatestEventSummaryAt(empty) = %v, want zero", empty)
		}

		latest := time.Now().Add(-5 * time.Minute).UTC().Truncate(time.Second)
		seedOverviewEvent(
			t, globalDB,
			"evt-latest-1", "message", "sess-overview-latest", latestWorkspaceID, "info", latest.Add(-time.Hour),
		)
		seedOverviewEvent(t, globalDB, "evt-latest-2", "message", "sess-overview-latest",
			latestWorkspaceID, "info", latest)

		observed, err := globalDB.LatestEventSummaryAt(ctx, "")
		if err != nil {
			t.Fatalf("LatestEventSummaryAt() error = %v", err)
		}
		if !observed.Equal(latest) {
			t.Fatalf("LatestEventSummaryAt() = %v, want %v", observed, latest)
		}
	})

	t.Run("Should count network audit envelopes since the window start", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		workspaceID := registerSessionForGlobalTests(t, globalDB, "sess-overview-net")

		now := time.Now()
		write := func(id string, at time.Time) {
			t.Helper()
			if err := globalDB.WriteNetworkAudit(ctx, store.NetworkAuditEntry{
				ID:          id,
				SessionID:   "sess-overview-net",
				WorkspaceID: workspaceID,
				Direction:   "sent",
				Kind:        "say",
				Channel:     "builders",
				Surface:     store.NetworkSurfaceThread,
				ThreadID:    "thread_overview",
				MessageID:   "msg-" + id,
				PeerFrom:    "coder.sess-overview-net",
				Timestamp:   at,
			}); err != nil {
				t.Fatalf("WriteNetworkAudit(%q) error = %v", id, err)
			}
		}
		write("naud-ov-1", store.LocalDayStart(now, 0).Add(time.Hour))
		write("naud-ov-2", store.LocalDayStart(now, 2))

		messages, err := globalDB.CountNetworkMessagesSince(ctx, store.OverviewSinceQuery{
			Since: store.LocalDayStart(now, 0),
		})
		if err != nil {
			t.Fatalf("CountNetworkMessagesSince() error = %v", err)
		}
		if messages != 1 {
			t.Fatalf("CountNetworkMessagesSince() = %d, want 1", messages)
		}
	})

	t.Run("Should return the longest user session inside the window", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		workspaceID := registerSessionForGlobalTests(t, globalDB, "sess-overview-longest")

		now := time.Now()
		start := store.LocalDayStart(now, 2).Add(9 * time.Hour)
		seedOverviewSession(t, globalDB, "sess-long", workspaceID, "writer", "user", start, start.Add(2*time.Hour))
		seedOverviewSession(t, globalDB, "sess-short", workspaceID, "researcher", "user", start,
			start.Add(10*time.Minute))
		seedOverviewSession(t, globalDB, "sess-spawned", workspaceID, "spawned", "agent", start,
			start.Add(6*time.Hour))

		longest, err := globalDB.LongestUserSessionSince(ctx, store.OverviewSinceQuery{
			Since: store.LocalDayStart(now, 13),
		})
		if err != nil {
			t.Fatalf("LongestUserSessionSince() error = %v", err)
		}
		if longest == nil {
			t.Fatal("LongestUserSessionSince() = nil, want the writer session")
		}
		if longest.SessionID != "sess-long" || longest.AgentName != "writer" {
			t.Fatalf("LongestUserSessionSince() = %+v, want sess-long by writer", longest)
		}
		if longest.RuntimeSeconds != int64((2 * time.Hour).Seconds()) {
			t.Fatalf("LongestUserSessionSince() runtime = %d, want 7200", longest.RuntimeSeconds)
		}

		none, err := globalDB.LongestUserSessionSince(ctx, store.OverviewSinceQuery{
			WorkspaceID: "ws-elsewhere",
			Since:       store.LocalDayStart(now, 13),
		})
		if err != nil {
			t.Fatalf("LongestUserSessionSince(scoped) error = %v", err)
		}
		if none != nil {
			t.Fatalf("LongestUserSessionSince(scoped) = %+v, want nil", none)
		}
	})

	t.Run("Should measure active user session runtime against now", func(t *testing.T) {
		t.Parallel()
		globalDB := openTestGlobalDB(t)
		ctx := testutil.Context(t)
		// The pre-registered session is dated 2026-04-03; inject a now far enough
		// ahead that it falls outside the 13-day window and cannot compete.
		now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
		globalDB.now = func() time.Time { return now }
		workspaceID := registerSessionForGlobalTests(t, globalDB, "sess-overview-active")

		start := now.Add(-90 * time.Minute)
		// Active session: updated_at is stale, so runtime must measure against now, not updated_at.
		seedOverviewSessionWithState(t, globalDB, "sess-active", workspaceID, "writer", "user", "active",
			start, start.Add(5*time.Minute))

		longest, err := globalDB.LongestUserSessionSince(ctx, store.OverviewSinceQuery{
			Since: store.LocalDayStart(now, 13),
		})
		if err != nil {
			t.Fatalf("LongestUserSessionSince() error = %v", err)
		}
		if longest == nil || longest.SessionID != "sess-active" {
			t.Fatalf("LongestUserSessionSince() = %+v, want sess-active", longest)
		}
		if longest.RuntimeSeconds != int64((90 * time.Minute).Seconds()) {
			t.Fatalf("RuntimeSeconds = %d, want 5400 (now - start, not updated_at)", longest.RuntimeSeconds)
		}
	})
}
