package globaldb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBLoopCatalogRunsShouldReturnTruthfulBatchSummaries(t *testing.T) {
	t.Parallel()

	t.Run("Should return exact lean summaries without decoding rich run fields", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)

		oldOnly := testLoopRun("looprun-old-only", now.Add(-40*24*time.Hour), looppkg.StatusFailed)
		oldOnly.WorkspaceID = "ws-a"
		oldOnly.LoopName = "old-only"
		insertLoopCatalogRunForTest(t, globalDB, oldOnly)
		oversizedInvalidJSON := strings.Repeat("{", 1<<20)
		result, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs
			 SET inputs_json = ?, start_metadata_json = ?, active_human_criteria_json = ?
			 WHERE id = ?`,
			oversizedInvalidJSON,
			"{",
			"[",
			string(oldOnly.ID),
		)
		if err != nil {
			t.Fatalf("poison rich loop run fields error = %v", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			t.Fatalf("poison rich loop run fields rows affected error = %v", err)
		}
		if affected != 1 {
			t.Fatalf("poison rich loop run fields rows affected = %d, want 1", affected)
		}

		foreign := testLoopRun("looprun-old-foreign", now.Add(time.Hour), looppkg.StatusDone)
		foreign.WorkspaceID = "ws-b"
		foreign.LoopName = "old-only"
		insertLoopCatalogRunForTest(t, globalDB, foreign)

		for index := range 501 {
			run := testLoopRun(
				"looprun-bulk-"+leftPadCatalogIndex(index),
				now.Add(-time.Duration(501-index)*time.Minute),
				looppkg.StatusDone,
			)
			run.WorkspaceID = "ws-a"
			run.LoopName = "bulk"
			insertLoopCatalogRunForTest(t, globalDB, run)
		}

		query := looppkg.CatalogRunQuery{
			WorkspaceID:    "ws-a",
			LoopNames:      []string{"old-only", "bulk", "never"},
			AggregateAfter: now.Add(-30 * 24 * time.Hour),
		}
		summaries, err := globalDB.ListLoopCatalogRunSummaries(ctx, query)
		if err != nil {
			t.Fatalf("ListLoopCatalogRunSummaries() error = %v", err)
		}
		if got := summaries["bulk"].Aggregate30d; got.Runs != 501 || got.Succeeded != 501 || got.Failed != 0 {
			t.Fatalf("bulk aggregate = %#v, want 501/501/0", got)
		}
		if latest := summaries["bulk"].LastRun; latest == nil || latest.ID != "looprun-bulk-500" {
			t.Fatalf("bulk latest = %#v, want looprun-bulk-500", latest)
		}
		old := summaries["old-only"]
		if old.LastRun == nil || old.LastRun.ID != oldOnly.ID || old.LastRun.Status != oldOnly.Status ||
			!old.LastRun.CreatedAt.Equal(oldOnly.CreatedAt) {
			t.Fatalf("old-only latest = %#v, want ws-a all-time run head", old.LastRun)
		}
		if old.Aggregate30d != (looppkg.CatalogRunAggregate{}) {
			t.Fatalf("old-only aggregate = %#v, want zero 30-day aggregate", old.Aggregate30d)
		}
		if never := summaries["never"]; never.LastRun != nil || never.Aggregate30d.Runs != 0 {
			t.Fatalf("never summary = %#v, want explicit empty summary", never)
		}

		normalized, err := normalizeLoopCatalogRunQuery(query)
		if err != nil {
			t.Fatalf("normalizeLoopCatalogRunQuery() error = %v", err)
		}
		namesJSON, err := json.Marshal(normalized.LoopNames)
		if err != nil {
			t.Fatalf("json.Marshal(normalized loop names) error = %v", err)
		}
		counting := &countingLoopCatalogQueryExecutor{loopCatalogQueryExecutor: globalDB.db}
		countedSummaries := make(map[string]looppkg.CatalogRunSummary, len(normalized.LoopNames))
		for _, name := range normalized.LoopNames {
			countedSummaries[name] = looppkg.CatalogRunSummary{LoopName: name}
		}
		if err := readLoopCatalogRunSummaries(
			ctx,
			counting,
			string(namesJSON),
			normalized,
			countedSummaries,
		); err != nil {
			t.Fatalf("readLoopCatalogRunSummaries() error = %v", err)
		}
		if counting.reads != 2 {
			t.Fatalf("loop catalog read statements = %d, want constant 2", counting.reads)
		}
	})

	t.Run("Should reject missing scope malformed names and missing aggregate window", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-a")
		queries := []looppkg.CatalogRunQuery{
			{LoopNames: []string{"alpha"}, AggregateAfter: time.Now().UTC()},
			{WorkspaceID: "ws-a", LoopNames: []string{"Not Valid"}, AggregateAfter: time.Now().UTC()},
			{WorkspaceID: "ws-a", LoopNames: []string{"alpha"}},
		}
		for _, query := range queries {
			if _, err := globalDB.ListLoopCatalogRunSummaries(testutil.Context(t), query); err == nil {
				t.Fatalf("ListLoopCatalogRunSummaries(%#v) error = nil, want validation error", query)
			}
		}
	})
}

func TestGlobalDBLoopCatalogRunsShouldUseCatalogIndex(t *testing.T) {
	t.Parallel()

	t.Run("Should seek newest all-status ids without a temporary order sort", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-a")
		names, err := json.Marshal([]string{"alpha", "beta"})
		if err != nil {
			t.Fatalf("json.Marshal(loop names) error = %v", err)
		}
		rows, err := globalDB.db.QueryContext(
			testutil.Context(t),
			"EXPLAIN QUERY PLAN "+loopCatalogLatestRunsSQL,
			string(names),
			"ws-a",
		)
		if err != nil {
			t.Fatalf("EXPLAIN QUERY PLAN loop catalog latest error = %v", err)
		}
		defer func() {
			if closeErr := rows.Close(); closeErr != nil {
				t.Errorf("close loop catalog query plan rows error = %v", closeErr)
			}
		}()

		var details []string
		for rows.Next() {
			var id int
			var parent int
			var unused int
			var detail string
			if scanErr := rows.Scan(&id, &parent, &unused, &detail); scanErr != nil {
				t.Fatalf("scan loop catalog query plan error = %v", scanErr)
			}
			details = append(details, detail)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate loop catalog query plan error = %v", err)
		}
		plan := strings.Join(details, "\n")
		if !strings.Contains(plan, "idx_loop_runs_catalog") {
			t.Fatalf("loop catalog query plan = %q, want idx_loop_runs_catalog", plan)
		}
		if strings.Contains(plan, "USE TEMP B-TREE FOR ORDER BY") {
			t.Fatalf("loop catalog query plan = %q, want indexed newest seek", plan)
		}
	})
}

func insertLoopCatalogRunForTest(t *testing.T, globalDB *GlobalDB, run looppkg.Run) {
	t.Helper()
	inputs, err := json.Marshal(run.Inputs)
	if err != nil {
		t.Fatalf("json.Marshal(loop run inputs) error = %v", err)
	}
	metadata, err := json.Marshal(run.StartMetadata)
	if err != nil {
		t.Fatalf("json.Marshal(loop run start metadata) error = %v", err)
	}
	if err := insertLoopRun(testutil.Context(t), globalDB.db, run, inputs, metadata); err != nil {
		t.Fatalf("insertLoopRun(%s) error = %v", run.ID, err)
	}
}

func leftPadCatalogIndex(index int) string {
	return fmt.Sprintf("%03d", index)
}

type countingLoopCatalogQueryExecutor struct {
	loopCatalogQueryExecutor
	reads int
}

func (e *countingLoopCatalogQueryExecutor) QueryContext(
	ctx context.Context,
	query string,
	args ...any,
) (*sql.Rows, error) {
	e.reads++
	return e.loopCatalogQueryExecutor.QueryContext(ctx, query, args...)
}

func TestGlobalDBLoopAPIRunsShouldRemainWorkspaceScoped(t *testing.T) {
	t.Parallel()

	t.Run("Should join loop-run scan and rows-close errors", func(t *testing.T) {
		t.Parallel()

		globalDB := openQueryRowsCloseErrorGlobalDB(t)
		_, err := globalDB.ListLoopRuns(
			testutil.Context(t),
			looppkg.RunListQuery{WorkspaceID: "ws-close-error"},
		)
		if !errors.Is(err, errQueryRowsClose) || !strings.Contains(err.Error(), "scan loop run") {
			t.Fatalf("ListLoopRuns() error = %v, want joined scan and rows-close errors", err)
		}
	})

	t.Run("Should list only the requested workspace runs with filters and aggregates inputs", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 5, 10, 0, 0, 0, time.UTC)

		alpha := testLoopRun("looprun-api-alpha", now, looppkg.StatusRunning)
		alpha.WorkspaceID = "ws-a"
		alpha.LoopName = "delivery"
		alpha.Inputs = map[string]any{"ticket": "A"}
		if _, err := globalDB.CreateLoopRunForStart(ctx, alpha, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart(alpha) error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(
			ctx,
			`UPDATE loop_runs SET
				origin_kind = 'session',
				origin_session_id = ?,
				origin_creation_profile_ref = ?,
				origin_policy_spec_digest = ?,
				origin_creation_digest = ?
			 WHERE id = ?`,
			"session-a",
			"profile:session-a",
			"policy:session-a",
			"creation:session-a",
			string(alpha.ID),
		); err != nil {
			t.Fatalf("set alpha session origin error = %v", err)
		}
		alphaDone := testLoopRun("looprun-api-alpha-done", now.Add(-time.Minute), looppkg.StatusRunning)
		alphaDone.WorkspaceID = "ws-a"
		alphaDone.LoopName = "delivery"
		if _, err := globalDB.CreateLoopRunForStart(ctx, alphaDone, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart(alpha done) error = %v", err)
		}
		if err := globalDB.CompareAndSwapLoopRunStatus(
			ctx,
			alphaDone.ID,
			looppkg.StatusRunning,
			looppkg.StatusDone,
			looppkg.TransitionCauseContract,
			now,
		); err != nil {
			t.Fatalf("CompareAndSwapLoopRunStatus(alpha done) error = %v", err)
		}
		beta := testLoopRun("looprun-api-beta", now.Add(time.Minute), looppkg.StatusRunning)
		beta.WorkspaceID = "ws-b"
		beta.LoopName = "delivery"
		if _, err := globalDB.CreateLoopRunForStart(ctx, beta, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart(beta) error = %v", err)
		}

		runs, err := globalDB.ListLoopRuns(ctx, looppkg.RunListQuery{
			WorkspaceID: "ws-a",
			LoopName:    "delivery",
			Status:      looppkg.StatusRunning,
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ListLoopRuns() error = %v", err)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("len(runs) = %d, want %d: %#v", got, want, runs)
		}
		if runs[0].ID != alpha.ID || runs[0].WorkspaceID != "ws-a" || runs[0].Inputs["ticket"] != "A" {
			t.Fatalf("ListLoopRuns() run = %#v", runs[0])
		}
		if got, want := runs[0].NetworkSpec, participation.LocalSpec(); got != want {
			t.Fatalf("ListLoopRuns() NetworkSpec = %#v, want %#v", got, want)
		}

		foreign, err := globalDB.ListLoopRuns(ctx, looppkg.RunListQuery{WorkspaceID: "ws-b", Limit: 10})
		if err != nil {
			t.Fatalf("ListLoopRuns(foreign) error = %v", err)
		}
		if got, want := len(foreign), 1; got != want {
			t.Fatalf("len(foreign runs) = %d, want %d: %#v", got, want, foreign)
		}
		if foreign[0].ID != beta.ID || foreign[0].WorkspaceID != "ws-b" {
			t.Fatalf("ListLoopRuns(foreign) run = %#v", foreign[0])
		}

		live := true
		sessionRuns, err := globalDB.ListLoopRuns(ctx, looppkg.RunListQuery{
			WorkspaceID:     "ws-a",
			OriginKind:      "session",
			OriginSessionID: "session-a",
			Live:            &live,
			Limit:           10,
		})
		if err != nil {
			t.Fatalf("ListLoopRuns(session live) error = %v", err)
		}
		if got, want := len(sessionRuns), 1; got != want || sessionRuns[0].ID != alpha.ID {
			t.Fatalf("ListLoopRuns(session live) = %#v, want only %q", sessionRuns, alpha.ID)
		}

		foreignSessionRuns, err := globalDB.ListLoopRuns(ctx, looppkg.RunListQuery{
			WorkspaceID:     "ws-b",
			OriginSessionID: "session-a",
			Limit:           10,
		})
		if err != nil {
			t.Fatalf("ListLoopRuns(foreign session) error = %v", err)
		}
		if len(foreignSessionRuns) != 0 {
			t.Fatalf("ListLoopRuns(foreign session) = %#v, want none", foreignSessionRuns)
		}

		terminal := false
		catalogRuns, err := globalDB.ListLoopRuns(ctx, looppkg.RunListQuery{
			WorkspaceID: "ws-a",
			OriginKind:  "catalog",
			Live:        &terminal,
			Limit:       10,
		})
		if err != nil {
			t.Fatalf("ListLoopRuns(catalog terminal) error = %v", err)
		}
		if got, want := len(catalogRuns), 1; got != want || catalogRuns[0].ID != alphaDone.ID {
			t.Fatalf("ListLoopRuns(catalog terminal) = %#v, want only %q", catalogRuns, alphaDone.ID)
		}
	})
}

func TestGlobalDBLoopAPIEventsShouldResumeBySequenceAndWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Should return status_changed events after seq without leaking another workspace", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 5, 11, 0, 0, 0, time.UTC)
		run := testLoopRun("looprun-api-events", now, looppkg.StatusRunning)
		run.WorkspaceID = "ws-a"
		if _, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart(run) error = %v", err)
		}
		if err := globalDB.CompareAndSwapLoopRunStatus(
			ctx,
			run.ID,
			looppkg.StatusRunning,
			looppkg.StatusPaused,
			looppkg.TransitionCausePauseBoundary,
			now.Add(time.Second),
		); err != nil {
			t.Fatalf("CompareAndSwapLoopRunStatus(run) error = %v", err)
		}

		events, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			WorkspaceID: "ws-a",
			RunID:       run.ID,
			AfterSeq:    1,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents() error = %v", err)
		}
		if got, want := len(events), 1; got != want {
			t.Fatalf("len(events) = %d, want %d: %#v", got, want, events)
		}
		if events[0].Seq != 2 || events[0].Kind != "status_changed" || events[0].WorkspaceID != "ws-a" {
			t.Fatalf("ListLoopRunEvents() event = %#v", events[0])
		}

		foreign, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			WorkspaceID: "ws-b",
			RunID:       run.ID,
			AfterSeq:    0,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents(foreign) error = %v", err)
		}
		if len(foreign) != 0 {
			t.Fatalf("foreign events = %#v, want none", foreign)
		}
	})
}

func TestGlobalDBLoopAPIAnnotationsShouldRemainWorkspaceScoped(t *testing.T) {
	t.Parallel()

	t.Run("Should round trip positions for same loop name independently per workspace", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		ctx := testutil.Context(t)

		if err := globalDB.ReplaceLoopUIAnnotations(ctx, "ws-a", "delivery", []looppkg.UIAnnotation{
			{NodeID: "draft", X: 10, Y: 20},
		}); err != nil {
			t.Fatalf("ReplaceLoopUIAnnotations(ws-a) error = %v", err)
		}
		if err := globalDB.ReplaceLoopUIAnnotations(ctx, "ws-b", "delivery", []looppkg.UIAnnotation{
			{NodeID: "draft", X: 30, Y: 40},
		}); err != nil {
			t.Fatalf("ReplaceLoopUIAnnotations(ws-b) error = %v", err)
		}

		alpha, err := globalDB.ListLoopUIAnnotations(ctx, "ws-a", "delivery")
		if err != nil {
			t.Fatalf("ListLoopUIAnnotations(ws-a) error = %v", err)
		}
		if got, want := len(alpha), 1; got != want {
			t.Fatalf("len(alpha) = %d, want %d: %#v", got, want, alpha)
		}
		if alpha[0].NodeID != "draft" || alpha[0].X != 10 || alpha[0].Y != 20 {
			t.Fatalf("alpha annotation = %#v", alpha[0])
		}

		beta, err := globalDB.ListLoopUIAnnotations(ctx, "ws-b", "delivery")
		if err != nil {
			t.Fatalf("ListLoopUIAnnotations(ws-b) error = %v", err)
		}
		if got, want := len(beta), 1; got != want {
			t.Fatalf("len(beta) = %d, want %d: %#v", got, want, beta)
		}
		if beta[0].NodeID != "draft" || beta[0].X != 30 || beta[0].Y != 40 {
			t.Fatalf("beta annotation = %#v", beta[0])
		}

		if err := globalDB.ReplaceLoopUIAnnotations(ctx, "ws-a", "delivery", []looppkg.UIAnnotation{
			{NodeID: "review", X: 50, Y: 60},
		}); err != nil {
			t.Fatalf("ReplaceLoopUIAnnotations(ws-a replace) error = %v", err)
		}
		alpha, err = globalDB.ListLoopUIAnnotations(ctx, "ws-a", "delivery")
		if err != nil {
			t.Fatalf("ListLoopUIAnnotations(ws-a replace) error = %v", err)
		}
		if got, want := len(alpha), 1; got != want {
			t.Fatalf("len(alpha replaced) = %d, want %d: %#v", got, want, alpha)
		}
		if alpha[0].NodeID != "review" || alpha[0].X != 50 || alpha[0].Y != 60 {
			t.Fatalf("alpha replaced annotation = %#v", alpha[0])
		}

		beta, err = globalDB.ListLoopUIAnnotations(ctx, "ws-b", "delivery")
		if err != nil {
			t.Fatalf("ListLoopUIAnnotations(ws-b after replace) error = %v", err)
		}
		if beta[0].NodeID != "draft" || beta[0].X != 30 || beta[0].Y != 40 {
			t.Fatalf("beta annotation after alpha replace = %#v", beta[0])
		}
	})
}
