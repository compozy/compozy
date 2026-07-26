package observe

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/modelcatalog"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
)

type overviewFixture struct {
	observer *Observer
	registry *globaldb.GlobalDB
	now      time.Time
	actor    taskpkg.ActorIdentity
}

func newOverviewFixture(t *testing.T) *overviewFixture {
	t.Helper()
	ctx := observeTestContext(t)

	home, err := aghconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(home); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}
	if err := observeTestGlobalSeed.Clone(home.DatabaseFile); err != nil {
		t.Fatalf("global store seed Clone() error = %v", err)
	}
	registry, err := globaldb.OpenGlobalDB(ctx, home.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := registry.Close(observeTestContext(t)); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})

	now := time.Now().UTC()
	observer, err := New(
		ctx,
		WithHomePaths(home),
		WithRegistry(registry),
		WithNow(func() time.Time { return now }),
		WithSessionSource(&stubSessionSource{}),
		WithRetentionConfig(RetentionConfig{Enabled: true, RetentionDays: 7, SweepInterval: time.Hour}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	return &overviewFixture{
		observer: observer,
		registry: registry,
		now:      now,
		actor:    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "tester"},
	}
}

func (f *overviewFixture) query() OverviewQuery {
	return OverviewQuery{TaskScope: taskpkg.CatalogScopeGlobal, Actor: f.actor}
}

func (f *overviewFixture) seedTask(t *testing.T, record taskpkg.Task) {
	t.Helper()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = f.now.Add(-24 * time.Hour)
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	if record.Scope == "" {
		record.Scope = taskpkg.ScopeGlobal
	}
	if record.CreatedBy.Kind == "" {
		record.CreatedBy = taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "tester"}
	}
	if record.Origin.Kind == "" {
		record.Origin = taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "test"}
	}
	if record.MaxAttempts == 0 {
		record.MaxAttempts = 3
	}
	if err := f.registry.CreateTask(observeTestContext(t), record); err != nil {
		t.Fatalf("CreateTask(%q) error = %v", record.ID, err)
	}
}

func (f *overviewFixture) seedRun(t *testing.T, run taskpkg.Run) {
	t.Helper()
	if run.Attempt == 0 {
		run.Attempt = 1
	}
	if run.Origin.Kind == "" {
		run.Origin = taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "test"}
	}
	if run.QueuedAt.IsZero() {
		run.QueuedAt = f.now.Add(-2 * time.Hour)
	}
	if err := f.registry.CreateTaskRun(observeTestContext(t), run); err != nil {
		t.Fatalf("CreateTaskRun(%q) error = %v", run.ID, err)
	}
}

func (f *overviewFixture) seedUsage(
	t *testing.T,
	day string,
	agent string,
	tokens int64,
	status string,
	source string,
) {
	t.Helper()
	update := store.TokenUsageDailyUpdate{
		Day:         day,
		AgentName:   agent,
		TotalTokens: new(tokens),
		CostStatus:  status,
		CostSource:  source,
		UpdatedAt:   f.now,
	}
	if status == "estimated" {
		update.CostAmount = new(float64(float64(tokens) / 1000))
		update.CostCurrency = new(string("USD"))
	}
	if err := f.registry.UpsertTokenUsageDaily(observeTestContext(t), update); err != nil {
		t.Fatalf("UpsertTokenUsageDaily(%s/%s) error = %v", day, agent, err)
	}
}

func (f *overviewFixture) seedEvent(t *testing.T, id string, timestamp time.Time) {
	t.Helper()
	if err := f.registry.WriteEventSummary(observeTestContext(t), store.EventSummary{
		ID:          id,
		SessionID:   "sess-overview",
		AgentName:   "coder",
		WorkspaceID: "ws-overview",
		Type:        "message",
		Outcome:     "info",
		Timestamp:   timestamp,
	}); err != nil {
		t.Fatalf("WriteEventSummary(%q) error = %v", id, err)
	}
}

func TestQueryObserveOverviewValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should reject an unsupported usage window", func(t *testing.T) {
		t.Parallel()
		fixture := newOverviewFixture(t)

		query := fixture.query()
		query.UsageWindowDays = 13
		_, err := fixture.observer.QueryObserveOverview(observeTestContext(t), query)
		if err == nil || !strings.Contains(err.Error(), "unsupported usage window 13") {
			t.Fatalf("QueryObserveOverview(window 13) error = %v, want unsupported usage window 13", err)
		}
	})
}

func TestQueryObserveOverviewComposition(t *testing.T) {
	t.Parallel()

	t.Run("Should compose an honest empty overview", func(t *testing.T) {
		t.Parallel()
		fixture := newOverviewFixture(t)

		view, err := fixture.observer.QueryObserveOverview(observeTestContext(t), fixture.query())
		if err != nil {
			t.Fatalf("QueryObserveOverview() error = %v", err)
		}

		if view.Attention.Total != 0 || len(view.Attention.Items) != 0 {
			t.Fatalf("Attention = %+v, want empty", view.Attention)
		}
		if view.Today != (OverviewToday{}) {
			t.Fatalf("Today = %+v, want zeros", view.Today)
		}
		if len(view.Outcomes.Days) != overviewOutcomeWindowDays ||
			view.Outcomes.WindowDays != overviewOutcomeWindowDays {
			t.Fatalf(
				"Outcomes.Days = %d, want %d zero-filled days",
				len(view.Outcomes.Days),
				overviewOutcomeWindowDays,
			)
		}
		if view.Outcomes.Completed != 0 || view.Outcomes.Failed != 0 ||
			view.Outcomes.Canceled != 0 || view.Outcomes.SuccessPct != 0 {
			t.Fatalf("Outcomes = %+v, want empty totals", view.Outcomes)
		}
		if view.Usage.EstimatedCost != nil || view.Usage.CostStatus != string(modelcatalog.CostStatusUnknown) {
			t.Fatalf("Usage cost = %+v/%q, want nil/unknown", view.Usage.EstimatedCost, view.Usage.CostStatus)
		}
		if view.Usage.WindowDays != 30 || !view.Usage.Truncated || view.Usage.RetentionDays != 7 {
			t.Fatalf("Usage window = %+v, want default 30 truncated by retention 7", view.Usage)
		}
		if len(view.Pulse.Buckets) != 168 {
			t.Fatalf("Pulse buckets = %d, want 168 zero-filled", len(view.Pulse.Buckets))
		}
		if view.Pulse.Busiest != nil || view.Pulse.LongestSession != nil {
			t.Fatalf("Pulse insights = %+v, want omitted", view.Pulse)
		}
		if view.Freshness.Status != freshnessStatusEmpty {
			t.Fatalf("Freshness.Status = %q, want empty", view.Freshness.Status)
		}
		if view.System.RetentionDays != 7 {
			t.Fatalf("System.RetentionDays = %d, want 7", view.System.RetentionDays)
		}
	})

	t.Run("Should compose today counters and outcomes from terminal runs", func(t *testing.T) {
		t.Parallel()
		fixture := newOverviewFixture(t)
		ctx := observeTestContext(t)

		today := store.LocalDayStart(fixture.now, 0).Add(2 * time.Hour)
		yesterday := store.LocalDayStart(fixture.now, 1).Add(2 * time.Hour)

		fixture.seedTask(t, taskpkg.Task{ID: "task-a", Title: "Task A", Status: taskpkg.TaskStatusCompleted})
		fixture.seedRun(t, taskpkg.Run{
			ID: "run-a1", TaskID: "task-a",
			Status: taskpkg.TaskRunStatusCompleted, EndedAt: today,
		})
		fixture.seedRun(t, taskpkg.Run{
			ID: "run-a2", TaskID: "task-a",
			Status: taskpkg.TaskRunStatusCompleted, EndedAt: today.Add(time.Minute),
		})
		fixture.seedRun(t, taskpkg.Run{
			ID: "run-a3", TaskID: "task-a",
			Status: taskpkg.TaskRunStatusFailed, EndedAt: today.Add(2 * time.Minute),
		})
		fixture.seedRun(t, taskpkg.Run{
			ID: "run-a4", TaskID: "task-a",
			Status: taskpkg.TaskRunStatusCompleted, EndedAt: yesterday,
		})

		closed := taskpkg.Task{ID: "task-closed", Title: "Closed", Status: taskpkg.TaskStatusCompleted}
		closed.ClosedAt = today.Add(3 * time.Minute)
		fixture.seedTask(t, closed)

		view, err := fixture.observer.QueryObserveOverview(ctx, fixture.query())
		if err != nil {
			t.Fatalf("QueryObserveOverview() error = %v", err)
		}

		want := OverviewToday{RunsCompleted: 2, RunsFailed: 1, TasksClosed: 1}
		if view.Today != want {
			t.Fatalf("Today = %+v, want %+v", view.Today, want)
		}
		if view.Outcomes.Completed != 3 || view.Outcomes.Failed != 1 || view.Outcomes.Canceled != 0 {
			t.Fatalf("Outcomes totals = %+v, want 3 completed, 1 failed", view.Outcomes)
		}
		if view.Outcomes.SuccessPct != 75 {
			t.Fatalf("Outcomes.SuccessPct = %v, want 75", view.Outcomes.SuccessPct)
		}
		if len(view.Outcomes.Days) != overviewOutcomeWindowDays {
			t.Fatalf(
				"Outcomes.Days = %d, want %d zero-filled buckets",
				len(view.Outcomes.Days),
				overviewOutcomeWindowDays,
			)
		}
	})

	t.Run("Should aggregate usage totals and roll the agent share tail into other", func(t *testing.T) {
		t.Parallel()
		fixture := newOverviewFixture(t)

		day := store.LocalDay(fixture.now)
		agents := []struct {
			name   string
			tokens int64
		}{
			{"writer", 500}, {"researcher", 300}, {"ops", 120}, {"publisher", 50}, {"scout", 30},
		}
		for _, agent := range agents {
			fixture.seedUsage(t, day, agent.name, agent.tokens, "estimated", "models_dev")
		}

		query := fixture.query()
		query.UsageWindowDays = 7
		view, err := fixture.observer.QueryObserveOverview(observeTestContext(t), query)
		if err != nil {
			t.Fatalf("QueryObserveOverview() error = %v", err)
		}

		if view.Usage.TotalTokens != 1000 {
			t.Fatalf("Usage.TotalTokens = %d, want 1000", view.Usage.TotalTokens)
		}
		if view.Usage.Truncated {
			t.Fatal("Usage.Truncated = true, want false for window 7 within retention 7")
		}
		if view.Usage.EstimatedCost == nil || view.Usage.CostStatus != "estimated" || view.Usage.CostCurrency != "USD" {
			t.Fatalf(
				"Usage cost = %+v %q %q, want estimated USD total",
				view.Usage.EstimatedCost, view.Usage.CostStatus, view.Usage.CostCurrency,
			)
		}
		if len(view.Usage.AgentShare) != 4 {
			t.Fatalf("AgentShare = %+v, want top 3 plus other", view.Usage.AgentShare)
		}
		last := view.Usage.AgentShare[3]
		if last.AgentName != "other" || last.Tokens != 80 {
			t.Fatalf("AgentShare tail = %+v, want other with 80 tokens", last)
		}
	})

	t.Run("Should degrade usage cost to unknown across mixed provenance groups", func(t *testing.T) {
		t.Parallel()
		fixture := newOverviewFixture(t)

		day := store.LocalDay(fixture.now)
		fixture.seedUsage(t, day, "writer", 100, "estimated", "models_dev")
		fixture.seedUsage(t, day, "native", 50, "included", "none")

		view, err := fixture.observer.QueryObserveOverview(observeTestContext(t), fixture.query())
		if err != nil {
			t.Fatalf("QueryObserveOverview() error = %v", err)
		}
		if view.Usage.EstimatedCost != nil || view.Usage.CostStatus != "unknown" {
			t.Fatalf(
				"Usage cost = %+v %q, want nil/unknown for mixed provenance",
				view.Usage.EstimatedCost, view.Usage.CostStatus,
			)
		}
		if view.Usage.TotalTokens != 150 {
			t.Fatalf("Usage.TotalTokens = %d, want tokens preserved at 150", view.Usage.TotalTokens)
		}
	})

	t.Run("Should surface the busiest pulse bucket and current freshness", func(t *testing.T) {
		t.Parallel()
		fixture := newOverviewFixture(t)

		slot := store.LocalDayStart(fixture.now, 1).Add(14 * time.Hour)
		local := slot.In(time.Local)
		fixture.seedEvent(t, "evt-1", slot)
		fixture.seedEvent(t, "evt-2", slot.Add(5*time.Minute))
		fixture.seedEvent(t, "evt-3", slot.Add(26*time.Hour))

		view, err := fixture.observer.QueryObserveOverview(observeTestContext(t), fixture.query())
		if err != nil {
			t.Fatalf("QueryObserveOverview() error = %v", err)
		}
		if view.Pulse.Busiest == nil {
			t.Fatal("Pulse.Busiest = nil, want the seeded slot")
		}
		if view.Pulse.Busiest.Weekday != int(local.Weekday()) || view.Pulse.Busiest.Hour != local.Hour() {
			t.Fatalf(
				"Pulse.Busiest = %+v, want weekday %d hour %d",
				view.Pulse.Busiest, int(local.Weekday()), local.Hour(),
			)
		}
		if view.Pulse.Busiest.Events != 2 {
			t.Fatalf("Pulse.Busiest.Events = %d, want 2", view.Pulse.Busiest.Events)
		}
		if view.Freshness.Status == "empty" {
			t.Fatal("Freshness.Status = empty, want a non-empty status with seeded events")
		}
	})

	t.Run("Should map attention items with truthful actions", func(t *testing.T) {
		t.Parallel()
		fixture := newOverviewFixture(t)
		ctx := observeTestContext(t)

		fixture.seedTask(t, taskpkg.Task{
			ID:             "task-approval",
			Title:          "Deploy docs site",
			Status:         taskpkg.TaskStatusBlocked,
			ApprovalPolicy: taskpkg.ApprovalPolicyManual,
			ApprovalState:  taskpkg.ApprovalStatePending,
		})
		fixture.seedTask(t, taskpkg.Task{
			ID:     "task-failed",
			Title:  "Overnight writer session",
			Status: taskpkg.TaskStatusFailed,
		})
		fixture.seedRun(t, taskpkg.Run{
			ID:     "run-failed",
			TaskID: "task-failed",
			Status: taskpkg.TaskRunStatusFailed,
			Error:  "provider timed out",
			EndedAt: store.
				LocalDayStart(fixture.now, 0).
				Add(time.Hour),
		})
		fixture.seedTask(t, taskpkg.Task{
			ID:     "task-input",
			Title:  "Competitor research",
			Status: taskpkg.TaskStatusNeedsAttention,
			Owner:  &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "tester"},
			NeedsAttention: &taskpkg.NeedsAttention{
				Reason: "question from peer",
				At:     fixture.now.Add(-time.Hour),
				By:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "tester"},
			},
		})

		view, err := fixture.observer.QueryObserveOverview(ctx, fixture.query())
		if err != nil {
			t.Fatalf("QueryObserveOverview() error = %v", err)
		}

		if view.Attention.Total != 3 {
			t.Fatalf("Attention.Total = %d (%+v), want 3", view.Attention.Total, view.Attention.ByKind)
		}
		byKind := view.Attention.ByKind
		if byKind[OverviewAttentionKindApproval] != 1 ||
			byKind[OverviewAttentionKindFailure] != 1 ||
			byKind[OverviewAttentionKindNeedsInput] != 1 {
			t.Fatalf("Attention.ByKind = %+v, want one of each kind", byKind)
		}

		itemsByKind := map[string]OverviewAttentionItem{}
		for _, item := range view.Attention.Items {
			itemsByKind[item.Kind] = item
		}
		approval := itemsByKind[OverviewAttentionKindApproval]
		if approval.TaskID != "task-approval" || strings.Join(approval.Actions, ",") != "approve,reject,open" {
			t.Fatalf("approval item = %+v, want approve/reject/open actions", approval)
		}
		failure := itemsByKind[OverviewAttentionKindFailure]
		if failure.RunID != "run-failed" || strings.Join(failure.Actions, ",") != "retry,open" {
			t.Fatalf("failure item = %+v, want retry/open actions with run id", failure)
		}
		if failure.Detail != "provider timed out" {
			t.Fatalf("failure detail = %q, want run error", failure.Detail)
		}
		needsInput := itemsByKind[OverviewAttentionKindNeedsInput]
		if needsInput.TaskID != "task-input" || strings.Join(needsInput.Actions, ",") != "open" {
			t.Fatalf("needs-input item = %+v, want open action", needsInput)
		}
	})

	t.Run("Should withhold needs_input tasks owned by another actor", func(t *testing.T) {
		t.Parallel()
		fixture := newOverviewFixture(t)
		ctx := observeTestContext(t)

		fixture.seedTask(t, taskpkg.Task{
			ID:     "task-foreign",
			Title:  "Someone else's escalation",
			Status: taskpkg.TaskStatusNeedsAttention,
			Owner:  &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "intruder"},
			NeedsAttention: &taskpkg.NeedsAttention{
				Reason: "not yours",
				At:     fixture.now.Add(-time.Hour),
				By:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "intruder"},
			},
		})
		fixture.seedTask(t, taskpkg.Task{
			ID:     "task-mine",
			Title:  "My escalation",
			Status: taskpkg.TaskStatusNeedsAttention,
			Owner:  &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "tester"},
			NeedsAttention: &taskpkg.NeedsAttention{
				Reason: "yours",
				At:     fixture.now.Add(-time.Hour),
				By:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "tester"},
			},
		})

		view, err := fixture.observer.QueryObserveOverview(ctx, fixture.query())
		if err != nil {
			t.Fatalf("QueryObserveOverview() error = %v", err)
		}
		if got := view.Attention.ByKind[OverviewAttentionKindNeedsInput]; got != 1 {
			t.Fatalf("needs_input = %d, want 1 (only the caller-owned task)", got)
		}
		for _, item := range view.Attention.Items {
			if item.TaskID == "task-foreign" {
				t.Fatalf("attention leaked a task owned by another actor: %+v", item)
			}
		}
	})
}
