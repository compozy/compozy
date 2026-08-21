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

	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/network/participation"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/testutil"
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

	t.Run("Should satisfy IT-021 with SQL-backed snapshot-fenced backward pages", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-a")
		ctx := testutil.Context(t)
		now := time.Date(2026, 8, 20, 14, 0, 0, 0, time.UTC)
		run := testLoopRun("looprun-api-long-timeline", now, looppkg.StatusRunning)
		run.WorkspaceID = "ws-a"
		if _, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		for sequence := int64(2); sequence <= 621; sequence++ {
			insertLoopRunEventForTimelineTest(
				t,
				globalDB,
				run,
				sequence,
				now.Add(time.Duration(sequence)*time.Millisecond),
			)
		}

		reads := looppkg.NewRunReadService(globalDB, nil)
		query := looppkg.TimelineQuery{View: looppkg.TimelineViewNotable, Limit: 73}
		seen := make(map[int64]bool, 621)
		firstPage := true
		for {
			page, err := reads.Timeline(ctx, "ws-a", run.ID, query)
			if err != nil {
				t.Fatalf("Timeline() error = %v", err)
			}
			if page.HeadSeq != 621 {
				t.Fatalf("Timeline() head = %d, want frozen head 621", page.HeadSeq)
			}
			for _, entry := range page.Entries {
				if seen[entry.Seq] {
					t.Fatalf("Timeline() duplicated sequence %d", entry.Seq)
				}
				seen[entry.Seq] = true
			}
			if firstPage {
				insertLoopRunEventForTimelineTest(t, globalDB, run, 622, now.Add(622*time.Millisecond))
				firstPage = false
			}
			if page.NextCursor == "" {
				break
			}
			query.Cursor = page.NextCursor
		}
		if len(seen) != 621 || seen[622] {
			t.Fatalf("Timeline() snapshot sequences = %d, appended event included = %t", len(seen), seen[622])
		}

		resumed, err := reads.Timeline(ctx, "ws-a", run.ID, looppkg.TimelineQuery{
			View: looppkg.TimelineViewNotable, AfterSeq: 600, Limit: 50,
		})
		if err != nil {
			t.Fatalf("Timeline(after 600) error = %v", err)
		}
		if len(resumed.Entries) != 22 || resumed.Entries[0].Seq != 622 || resumed.Entries[21].Seq != 601 {
			t.Fatalf("Timeline(after 600) entries = %#v", resumed.Entries)
		}
	})

	t.Run("Should persist applied runtime and its event across reopen without workspace leaks", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		ctx := testutil.Context(t)
		now := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC)
		run := testLoopRun("looprun-runtime-applied", now, looppkg.StatusRunning)
		run.WorkspaceID = "ws-a"
		if _, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		insertLoopRuntimeOutputForTest(t, globalDB, run.ID, 1, "work", 2)
		resolved := looppkg.ResolvedRuntime{
			Runtime: looppkg.RuntimeSpec{
				Provider: "codex", Model: "gpt-5.6-terra", Reasoning: "high", Speed: speedpkg.SpeedFast,
			},
			Source: looppkg.RuntimeProvenance{
				Provider: looppkg.RuntimeSourceRun,
				Model:    looppkg.RuntimeSourceFrontmatter, Reasoning: looppkg.RuntimeSourceDefault,
				Speed: looppkg.RuntimeSourceInput,
			},
			SpeedResolution: &speedpkg.Resolution{
				Requested: speedpkg.SpeedFast, Status: speedpkg.ResolutionUnsupported,
				Reason: speedpkg.ReasonCapabilityAbsent,
			},
		}
		if err := globalDB.RecordAppliedRuntime(ctx, "ws-a", run.ID, 1, "work", 2, resolved); err != nil {
			t.Fatalf("RecordAppliedRuntime() error = %v", err)
		}

		foreignOutputs, err := globalDB.ListGenerationOutputs(ctx, "ws-b", run.ID, 1)
		if err != nil {
			t.Fatalf("ListGenerationOutputs(foreign) error = %v", err)
		}
		if len(foreignOutputs) != 0 {
			t.Fatalf("foreign outputs = %#v, want none", foreignOutputs)
		}
		foreignEvents, err := globalDB.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			WorkspaceID: "ws-b", RunID: run.ID,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents(foreign) error = %v", err)
		}
		if len(foreignEvents) != 0 {
			t.Fatalf("foreign events = %#v, want none", foreignEvents)
		}

		path := globalDB.path
		if err := globalDB.Close(ctx); err != nil {
			t.Fatalf("Close(before reopen) error = %v", err)
		}
		reopened, err := OpenGlobalDB(ctx, path)
		if err != nil {
			t.Fatalf("OpenGlobalDB(reopen) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := reopened.Close(context.Background()); closeErr != nil {
				t.Errorf("Close(reopened) error = %v", closeErr)
			}
		})
		outputs, err := reopened.ListGenerationOutputs(ctx, "ws-a", run.ID, 1)
		if err != nil {
			t.Fatalf("ListGenerationOutputs(reopen) error = %v", err)
		}
		if len(outputs) != 1 || outputs[0].ResolvedRuntime == nil {
			t.Fatalf("outputs after reopen = %#v, want durable runtime", outputs)
		}
		got := outputs[0].ResolvedRuntime
		if got.Runtime.Provider != resolved.Runtime.Provider || got.Runtime.Model != resolved.Runtime.Model ||
			got.Runtime.Reasoning != resolved.Runtime.Reasoning || got.Runtime.Speed != resolved.Runtime.Speed ||
			got.Source != resolved.Source || got.SpeedResolution == nil ||
			*got.SpeedResolution != *resolved.SpeedResolution {
			t.Fatalf("resolved runtime after reopen = %#v, want %#v", got, resolved)
		}
		events, err := reopened.ListLoopRunEvents(ctx, looppkg.RunEventQuery{
			WorkspaceID: "ws-a", RunID: run.ID,
		})
		if err != nil {
			t.Fatalf("ListLoopRunEvents(reopen) error = %v", err)
		}
		var runtimeEvent *looppkg.RunEvent
		for index := range events {
			if events[index].Kind == loopRunEventRuntimeApplied {
				runtimeEvent = &events[index]
				break
			}
		}
		if runtimeEvent == nil {
			t.Fatalf("events after reopen = %#v, want runtime_applied", events)
		}
		var eventPayload struct {
			Generation      int    `json:"generation"`
			NodeID          string `json:"node_id"`
			ItemIndex       int    `json:"item_index"`
			ResolvedRuntime struct {
				Provider        string              `json:"provider"`
				Model           string              `json:"model"`
				Speed           speedpkg.Speed      `json:"speed"`
				SpeedResolution speedpkg.Resolution `json:"speed_resolution"`
				Source          struct {
					Model string                `json:"model"`
					Speed looppkg.RuntimeSource `json:"speed"`
				} `json:"source"`
			} `json:"resolved_runtime"`
		}
		if err := json.Unmarshal(runtimeEvent.Payload, &eventPayload); err != nil {
			t.Fatalf("json.Unmarshal(runtime event) error = %v", err)
		}
		if eventPayload.Generation != 1 || eventPayload.NodeID != "work" || eventPayload.ItemIndex != 2 ||
			eventPayload.ResolvedRuntime.Provider != "codex" ||
			eventPayload.ResolvedRuntime.Model != "gpt-5.6-terra" ||
			eventPayload.ResolvedRuntime.Speed != speedpkg.SpeedFast ||
			eventPayload.ResolvedRuntime.SpeedResolution.Status != speedpkg.ResolutionUnsupported ||
			eventPayload.ResolvedRuntime.SpeedResolution.Reason != speedpkg.ReasonCapabilityAbsent ||
			eventPayload.ResolvedRuntime.Source.Model != "frontmatter" ||
			eventPayload.ResolvedRuntime.Source.Speed != looppkg.RuntimeSourceInput {
			t.Fatalf("runtime event payload = %#v, want durable public runtime shape", eventPayload)
		}
	})

	t.Run("Should roll back output runtime when event append fails", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-a")
		ctx := testutil.Context(t)
		run := testLoopRun(
			"looprun-runtime-rollback",
			time.Date(2026, 7, 26, 13, 0, 0, 0, time.UTC),
			looppkg.StatusRunning,
		)
		run.WorkspaceID = "ws-a"
		if _, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow); err != nil {
			t.Fatalf("CreateLoopRunForStart() error = %v", err)
		}
		insertLoopRuntimeOutputForTest(t, globalDB, run.ID, 1, "work", 0)
		if _, err := globalDB.db.ExecContext(ctx, `
			CREATE TRIGGER fail_runtime_applied_event
			BEFORE INSERT ON loop_run_events
			WHEN NEW.kind = 'runtime_applied'
			BEGIN
				SELECT RAISE(ABORT, 'runtime event failure');
			END
		`); err != nil {
			t.Fatalf("create runtime event failure trigger: %v", err)
		}

		err := globalDB.RecordAppliedRuntime(ctx, "ws-a", run.ID, 1, "work", 0, looppkg.ResolvedRuntime{
			Runtime: looppkg.RuntimeSpec{Provider: "codex", Model: "gpt-5.6-terra"},
		})
		if err == nil || !strings.Contains(err.Error(), "runtime event failure") {
			t.Fatalf("RecordAppliedRuntime() error = %v, want injected event failure", err)
		}
		var runtimeJSON sql.NullString
		if err := globalDB.db.QueryRowContext(ctx, `
			SELECT resolved_runtime_json
			FROM loop_generation_outputs
			WHERE loop_run_id = ? AND generation = 1 AND node_id = 'work' AND item_index = 0
		`, string(run.ID)).Scan(&runtimeJSON); err != nil {
			t.Fatalf("query rolled back runtime: %v", err)
		}
		if runtimeJSON.Valid {
			t.Fatalf("resolved_runtime_json = %q, want transaction rollback", runtimeJSON.String)
		}
	})
}

func TestGlobalDBLoopRunSummariesShouldBatchProgressAttentionAndForks(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t, "ws-a")
	ctx := testutil.Context(t)
	now := time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC)
	parent := testLoopRun("looprun-summary-parent", now, looppkg.StatusRunning)
	parent.WorkspaceID = "ws-a"
	if _, err := globalDB.CreateLoopRunForStart(ctx, parent, dsl.ConcurrencyAllow); err != nil {
		t.Fatalf("CreateLoopRunForStart(parent) error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`UPDATE loop_runs SET status = 'needs-approval', generation = 1 WHERE id = ?`,
		parent.ID,
	); err != nil {
		t.Fatalf("prepare parent summary state error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`INSERT INTO loop_generation_outputs
			(loop_run_id, generation, node_id, item_index, status, attempt)
		 VALUES (?, 1, 'finish', 0, 'succeeded', 1)`,
		parent.ID,
	); err != nil {
		t.Fatalf("insert parent summary output error = %v", err)
	}
	child := testLoopRun("looprun-summary-child", now.Add(time.Second), looppkg.StatusRunning)
	child.WorkspaceID = "ws-a"
	child.SetForkedFrom(&looppkg.ForkRef{RunID: parent.ID, Generation: 1})
	if _, err := globalDB.CreateLoopRunForStart(ctx, child, dsl.ConcurrencyAllow); err != nil {
		t.Fatalf("CreateLoopRunForStart(child) error = %v", err)
	}

	summaries, err := globalDB.ListLoopRunSummaries(
		ctx,
		"ws-a",
		[]looppkg.RunID{parent.ID, child.ID},
	)
	if err != nil {
		t.Fatalf("ListLoopRunSummaries() error = %v", err)
	}
	parentSummary := summaries[parent.ID]
	if parentSummary.Progress.Round != 1 || parentSummary.Progress.StepsDone != 1 ||
		parentSummary.Progress.StepsTotal != 1 {
		t.Fatalf("parent progress = %#v", parentSummary.Progress)
	}
	if parentSummary.Attention == nil || parentSummary.Attention.Kind != "approval" ||
		parentSummary.Attention.Count != 1 {
		t.Fatalf("parent attention = %#v", parentSummary.Attention)
	}
	if len(parentSummary.Forks) != 1 || parentSummary.Forks[0].RunID != child.ID {
		t.Fatalf("parent forks = %#v", parentSummary.Forks)
	}
}

func TestGlobalDBLoopBriefingShouldDeriveArtifactAvailabilityFromRetainedBlobs(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t, "ws-a")
	ctx := testutil.Context(t)
	now := time.Date(2026, 8, 20, 15, 30, 0, 0, time.UTC)
	run := testLoopRun("looprun-artifact-availability", now, looppkg.StatusRunning)
	run.WorkspaceID = "ws-a"
	created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`UPDATE loop_runs SET status = 'done', generation = 1 WHERE id = ?`,
		created.ID,
	); err != nil {
		t.Fatalf("terminalize artifact run error = %v", err)
	}
	outputs := []struct {
		itemIndex int
		status    string
		ref       string
		retained  bool
	}{
		{itemIndex: 0, status: "succeeded", ref: "blob-available", retained: true},
		{itemIndex: 1, status: "partial", ref: "blob-partial", retained: true},
		{itemIndex: 2, status: "succeeded", ref: "blob-pruned", retained: false},
	}
	for _, output := range outputs {
		if output.retained {
			payload := `{"result":"retained"}`
			if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_output_blobs
				(output_ref, payload_json, byte_size, created_at, last_used_at)
				VALUES (?, ?, ?, ?, ?)`, output.ref, payload, len(payload), now, now); err != nil {
				t.Fatalf("insert output blob %q error = %v", output.ref, err)
			}
		}
		_, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs
			(loop_run_id, generation, node_id, item_index, status, output_ref, task_run_id, attempt)
			VALUES (?, 1, 'finish', ?, ?, ?, ?, 1)`,
			created.ID, output.itemIndex, output.status, output.ref, fmt.Sprintf("task-%d", output.itemIndex))
		if err != nil {
			t.Fatalf("insert generation output %d error = %v", output.itemIndex, err)
		}
	}

	briefing, err := looppkg.NewRunReadService(globalDB, func() time.Time { return now }).Briefing(
		ctx,
		"ws-a",
		created.ID,
	)
	if err != nil {
		t.Fatalf("Briefing() error = %v", err)
	}
	want := []looppkg.ArtifactAvailability{
		looppkg.ArtifactAvailable,
		looppkg.ArtifactPartial,
		looppkg.ArtifactPruned,
	}
	if len(briefing.Artifacts) != len(want) {
		t.Fatalf("artifacts = %#v, want three rows", briefing.Artifacts)
	}
	for index := range want {
		if briefing.Artifacts[index].Availability != want[index] {
			t.Fatalf("artifact %d = %#v, want availability %q", index, briefing.Artifacts[index], want[index])
		}
	}
}

func TestGlobalDBLoopRunSummaryShouldMatchBriefingProgressWithUntakenAndFailedActions(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t, "ws-a")
	ctx := testutil.Context(t)
	now := time.Date(2026, 8, 20, 15, 45, 0, 0, time.UTC)
	run := loopRunWithRouteDefinitionForReadTest(t, "looprun-progress-parity", "ws-a", now)
	created, err := globalDB.CreateLoopRunForStart(ctx, run, dsl.ConcurrencyAllow)
	if err != nil {
		t.Fatalf("CreateLoopRunForStart() error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(
		ctx,
		`UPDATE loop_runs SET generation = 1 WHERE id = ?`,
		created.ID,
	); err != nil {
		t.Fatalf("advance run generation error = %v", err)
	}
	if _, err := globalDB.db.ExecContext(ctx, `INSERT INTO loop_generation_outputs
		(loop_run_id, generation, node_id, item_index, status, attempt)
		VALUES (?, 1, 'selected', 0, 'failed', 1)`, created.ID); err != nil {
		t.Fatalf("insert failed selected output error = %v", err)
	}
	if err := appendLoopRunEventWithExecutor(ctx, globalDB.db, created.ID, created.WorkspaceID,
		loopRunEventRouteTaken, map[string]any{
			"generation": 1,
			"node_id":    "router",
			"item_index": 0,
			"route":      "selected",
			"cause":      "matched_when",
		}, now.Add(time.Second)); err != nil {
		t.Fatalf("append route_taken error = %v", err)
	}

	briefing, err := looppkg.NewRunReadService(globalDB, func() time.Time { return now }).Briefing(
		ctx,
		"ws-a",
		created.ID,
	)
	if err != nil {
		t.Fatalf("Briefing() error = %v", err)
	}
	summaries, err := globalDB.ListLoopRunSummaries(ctx, "ws-a", []looppkg.RunID{created.ID})
	if err != nil {
		t.Fatalf("ListLoopRunSummaries() error = %v", err)
	}
	if briefing.Progress != (looppkg.StepProgress{Round: 1, StepsDone: 1, StepsTotal: 1}) {
		t.Fatalf("briefing progress = %#v, want failed selected settled and skipped excluded", briefing.Progress)
	}
	if summaries[created.ID].Progress != briefing.Progress {
		t.Fatalf("summary/briefing progress = %#v/%#v", summaries[created.ID].Progress, briefing.Progress)
	}
}

func loopRunWithRouteDefinitionForReadTest(
	t *testing.T,
	runID string,
	workspaceID string,
	at time.Time,
) looppkg.Run {
	t.Helper()
	definition, err := dsl.Parse([]byte(`{
		"apiVersion":"compozy.loop/v1",
		"kind":"Loop",
		"meta":{"name":"route-read","version":1},
		"contract":{"goal":"test","definition_of_done":"done","iteration_cap":3,
			"no_progress":{"window":1},"budget":{"tokens":100,"wall_clock_sec":60}},
		"graph":{"nodes":[
			{"id":"router","class":"control","kind":"route",
			 "routes":[{"when":"false","to":"skipped"}],"default":"selected"},
			{"id":"selected","class":"action","kind":"transform","params":{"map":{"ok":{"value":true}}}},
			{"id":"skipped","class":"action","kind":"transform","params":{"map":{"ok":{"value":false}}}}
		],"edges":[{"from":"router","to":"selected"},{"from":"router","to":"skipped"}]}
	}`))
	if err != nil {
		t.Fatalf("dsl.Parse(route read definition) error = %v", err)
	}
	resolved, err := looppkg.NewCompiler().Compile(definition)
	if err != nil {
		t.Fatalf("Compile(route read definition) error = %v", err)
	}
	effective, err := looppkg.ResolveEffectiveConfig(resolved, looppkg.DefaultLoopDefaults(), nil, looppkg.LoopConfig{})
	if err != nil {
		t.Fatalf("ResolveEffectiveConfig() error = %v", err)
	}
	snapshot, digest, err := looppkg.BuildExecutedDefinitionSnapshot(resolved, effective)
	if err != nil {
		t.Fatalf("BuildExecutedDefinitionSnapshot() error = %v", err)
	}
	run := testLoopRun(runID, at, looppkg.StatusRunning)
	run.WorkspaceID = looppkg.WorkspaceID(workspaceID)
	run.LoopName = "route-read"
	run.DefinitionVersion = resolved.DefinitionVersion
	run.DefinitionSnapshot = snapshot
	run.DefinitionDigest = digest
	return run
}

func TestGlobalDBLoopRunsShouldOrderOperationalPagesBeforeLimiting(t *testing.T) {
	t.Parallel()

	globalDB := openLoopTestGlobalDB(t, "ws-a")
	ctx := testutil.Context(t)
	now := time.Date(2026, 8, 20, 17, 0, 0, 0, time.UTC)
	fixtures := []looppkg.Run{
		testLoopRun("terminal-new", now.Add(4*time.Minute), looppkg.StatusDone),
		testLoopRun("active-new", now.Add(3*time.Minute), looppkg.StatusRunning),
		testLoopRun("needs-you", now.Add(2*time.Minute), looppkg.StatusNeedsApproval),
		testLoopRun("active-old", now.Add(time.Minute), looppkg.StatusQueued),
		testLoopRun("terminal-old", now, looppkg.StatusFailed),
	}
	for index := range fixtures {
		fixtures[index].WorkspaceID = "ws-a"
		insertLoopCatalogRunForTest(t, globalDB, fixtures[index])
	}

	firstPage, err := globalDB.ListLoopRuns(ctx, looppkg.RunListQuery{
		WorkspaceID: "ws-a", OperationalOrder: true, Limit: 2,
	})
	if err != nil {
		t.Fatalf("ListLoopRuns(first page) error = %v", err)
	}
	if len(firstPage) != 2 || firstPage[0].ID != "needs-you" || firstPage[1].ID != "active-new" {
		t.Fatalf("first page = %#v", firstPage)
	}
	secondPage, err := globalDB.ListLoopRuns(ctx, looppkg.RunListQuery{
		WorkspaceID: "ws-a", OperationalOrder: true, Limit: 3,
		After: &looppkg.RunListPosition{
			Rank: 1, CreatedAt: firstPage[1].CreatedAt, ID: firstPage[1].ID,
		},
	})
	if err != nil {
		t.Fatalf("ListLoopRuns(second page) error = %v", err)
	}
	want := []looppkg.RunID{"active-old", "terminal-new", "terminal-old"}
	if len(secondPage) != len(want) {
		got := make([]looppkg.RunID, len(secondPage))
		for index := range secondPage {
			got[index] = secondPage[index].ID
		}
		t.Fatalf("second page ids = %#v, want %#v", got, want)
	}
	for index := range want {
		if secondPage[index].ID != want[index] {
			t.Fatalf("second page run %d = %q, want %q", index, secondPage[index].ID, want[index])
		}
	}
	_, err = globalDB.ListLoopRuns(ctx, looppkg.RunListQuery{
		WorkspaceID:      "ws-a",
		OperationalOrder: true,
		Limit:            2,
		After: &looppkg.RunListPosition{
			Rank:      1,
			CreatedAt: firstPage[1].CreatedAt,
			ID:        "missing-cursor-run",
		},
	})
	if !errors.Is(err, looppkg.ErrInvalidRunListCursor) {
		t.Fatalf("ListLoopRuns(missing cursor row) error = %v, want invalid cursor", err)
	}
}

func insertLoopRunEventForTimelineTest(
	t *testing.T,
	globalDB *GlobalDB,
	run looppkg.Run,
	sequence int64,
	at time.Time,
) {
	t.Helper()
	_, err := globalDB.db.ExecContext(
		testutil.Context(t),
		`INSERT INTO loop_run_events
			(id, loop_run_id, workspace_id, seq, kind, payload_json, at, delivery_key)
		 VALUES (?, ?, ?, ?, 'node_succeeded', '{}', ?, NULL)`,
		fmt.Sprintf("event-long-timeline-%d", sequence),
		run.ID,
		run.WorkspaceID,
		sequence,
		at,
	)
	if err != nil {
		t.Fatalf("insert timeline event %d error = %v", sequence, err)
	}
}

func insertLoopRuntimeOutputForTest(
	t *testing.T,
	globalDB *GlobalDB,
	runID looppkg.RunID,
	generation int,
	nodeID string,
	itemIndex int,
) {
	t.Helper()
	if _, err := globalDB.db.ExecContext(testutil.Context(t), `
		INSERT INTO loop_generation_outputs (
			loop_run_id, generation, node_id, item_index, status
		) VALUES (?, ?, ?, ?, 'pending')
	`, string(runID), generation, nodeID, itemIndex); err != nil {
		t.Fatalf("insert loop runtime output: %v", err)
	}
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

	t.Run("Should delete definition sidecars without touching another workspace", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-a", "ws-b")
		ctx := testutil.Context(t)
		for _, workspaceID := range []looppkg.WorkspaceID{"ws-a", "ws-b"} {
			if err := globalDB.UpsertLoopConfig(ctx, workspaceID, "delivery", looppkg.LoopConfig{
				IterationCap: new(7),
			}); err != nil {
				t.Fatalf("UpsertLoopConfig(%s) error = %v", workspaceID, err)
			}
			if err := globalDB.ReplaceLoopUIAnnotations(ctx, workspaceID, "delivery", []looppkg.UIAnnotation{{
				NodeID: "draft",
				X:      10,
				Y:      20,
			}}); err != nil {
				t.Fatalf("ReplaceLoopUIAnnotations(%s) error = %v", workspaceID, err)
			}
		}

		if _, err := globalDB.DeleteLoopDefinitionState(ctx, "ws-a", "delivery"); err != nil {
			t.Fatalf("DeleteLoopDefinitionState(ws-a) error = %v", err)
		}
		if _, err := globalDB.GetLoopConfig(ctx, "ws-a", "delivery"); !errors.Is(err, looppkg.ErrConfigNotFound) {
			t.Fatalf("GetLoopConfig(ws-a after delete) error = %v, want ErrConfigNotFound", err)
		}
		alpha, err := globalDB.ListLoopUIAnnotations(ctx, "ws-a", "delivery")
		if err != nil {
			t.Fatalf("ListLoopUIAnnotations(ws-a after delete) error = %v", err)
		}
		if len(alpha) != 0 {
			t.Fatalf("ws-a annotations after delete = %#v, want empty", alpha)
		}
		if _, err := globalDB.GetLoopConfig(ctx, "ws-b", "delivery"); err != nil {
			t.Fatalf("GetLoopConfig(ws-b after ws-a delete) error = %v", err)
		}
		beta, err := globalDB.ListLoopUIAnnotations(ctx, "ws-b", "delivery")
		if err != nil {
			t.Fatalf("ListLoopUIAnnotations(ws-b after ws-a delete) error = %v", err)
		}
		if len(beta) != 1 || beta[0].NodeID != "draft" {
			t.Fatalf("ws-b annotations after ws-a delete = %#v, want preserved draft", beta)
		}
	})

	t.Run("Should roll back both sidecars when either delete fails", func(t *testing.T) {
		t.Parallel()

		globalDB := openLoopTestGlobalDB(t, "ws-a")
		ctx := testutil.Context(t)
		if err := globalDB.UpsertLoopConfig(ctx, "ws-a", "delivery", looppkg.LoopConfig{
			IterationCap: new(7),
		}); err != nil {
			t.Fatalf("UpsertLoopConfig() error = %v", err)
		}
		if err := globalDB.ReplaceLoopUIAnnotations(ctx, "ws-a", "delivery", []looppkg.UIAnnotation{{
			NodeID: "draft",
			X:      10,
			Y:      20,
		}}); err != nil {
			t.Fatalf("ReplaceLoopUIAnnotations() error = %v", err)
		}
		if _, err := globalDB.db.ExecContext(ctx, `
			CREATE TRIGGER fail_loop_config_delete
			BEFORE DELETE ON loop_config
			BEGIN
				SELECT RAISE(ABORT, 'injected loop config delete failure');
			END;
		`); err != nil {
			t.Fatalf("create delete failure trigger: %v", err)
		}

		_, err := globalDB.DeleteLoopDefinitionState(ctx, "ws-a", "delivery")
		if err == nil || !strings.Contains(err.Error(), "injected loop config delete failure") {
			t.Fatalf("DeleteLoopDefinitionState() error = %v, want injected failure", err)
		}
		if _, err := globalDB.GetLoopConfig(ctx, "ws-a", "delivery"); err != nil {
			t.Fatalf("GetLoopConfig(after rollback) error = %v", err)
		}
		annotations, err := globalDB.ListLoopUIAnnotations(ctx, "ws-a", "delivery")
		if err != nil {
			t.Fatalf("ListLoopUIAnnotations(after rollback) error = %v", err)
		}
		if len(annotations) != 1 || annotations[0].NodeID != "draft" {
			t.Fatalf("annotations after rollback = %#v, want preserved draft", annotations)
		}
	})
}
