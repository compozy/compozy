package globaldb

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestGlobalDBTaskRunReviewStore(t *testing.T) {
	t.Parallel()

	t.Run("Should request reviews idempotently and link the task run", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		globalDB.now = fixedTaskReviewStoreTime
		taskRecord, runRecord := createReviewStoreTaskRun(t, globalDB, taskpkg.TaskRunStatusCompleted)

		review := taskReviewForGlobalDBTest("review-1", taskRecord.ID, runRecord.ID)
		stored, created, err := globalDB.RequestRunReview(ctx, &review)
		if err != nil {
			t.Fatalf("RequestRunReview(create) error = %v", err)
		}
		if !created {
			t.Fatal("RequestRunReview(create) created = false, want true")
		}
		assertTaskRunReviewShape(t, stored, review.ReviewID, taskpkg.RunReviewStatusRequested)
		assertTaskRunReviewRequestLink(t, globalDB, runRecord.ID, review.ReviewID)

		duplicate := review
		duplicate.ReviewID = "review-duplicate"
		stored, created, err = globalDB.RequestRunReview(ctx, &duplicate)
		if err != nil {
			t.Fatalf("RequestRunReview(duplicate) error = %v", err)
		}
		if created {
			t.Fatal("RequestRunReview(duplicate) created = true, want false")
		}
		if got, want := stored.ReviewID, review.ReviewID; got != want {
			t.Fatalf("duplicate ReviewID = %q, want %q", got, want)
		}
	})

	t.Run("Should bind lookup and list active reviewer sessions", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		globalDB.now = fixedTaskReviewStoreTime
		taskRecord, runRecord := createReviewStoreTaskRun(t, globalDB, taskpkg.TaskRunStatusFailed)

		review := taskReviewForGlobalDBTest("review-bind", taskRecord.ID, runRecord.ID)
		review.Policy = taskpkg.ReviewPolicyOnFailure
		stored, _, err := globalDB.RequestRunReview(ctx, &review)
		if err != nil {
			t.Fatalf("RequestRunReview() error = %v", err)
		}
		bound, err := globalDB.BindRunReviewSession(ctx, taskpkg.BindRunReviewSessionRequest{
			ReviewID:          stored.ReviewID,
			SessionID:         "sess-reviewer",
			ReviewerAgentName: "reviewer",
			ReviewerPeerID:    "peer-reviewer",
			ReviewerChannelID: "channel-review",
		}, fixedTaskReviewStoreTime().Add(time.Minute))
		if err != nil {
			t.Fatalf("BindRunReviewSession() error = %v", err)
		}
		assertTaskRunReviewShape(t, bound, stored.ReviewID, taskpkg.RunReviewStatusInReview)
		if got, want := bound.ReviewerSessionID, "sess-reviewer"; got != want {
			t.Fatalf("ReviewerSessionID = %q, want %q", got, want)
		}

		lookup, err := globalDB.LookupRunReviewBySession(ctx, "sess-reviewer")
		if err != nil {
			t.Fatalf("LookupRunReviewBySession() error = %v", err)
		}
		if got, want := lookup.ReviewID, stored.ReviewID; got != want {
			t.Fatalf("lookup ReviewID = %q, want %q", got, want)
		}
		listed, err := globalDB.ListRunReviews(ctx, taskpkg.RunReviewQuery{
			Status:            taskpkg.RunReviewStatusInReview,
			ReviewerSessionID: "sess-reviewer",
			Limit:             1,
		})
		if err != nil {
			t.Fatalf("ListRunReviews() error = %v", err)
		}
		if len(listed) != 1 || listed[0].ReviewID != stored.ReviewID {
			t.Fatalf("ListRunReviews() = %#v, want bound review", listed)
		}
	})

	t.Run("Should rebind in review rows when reviewer sessions stop", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		globalDB.now = fixedTaskReviewStoreTime
		taskRecord, runRecord := createReviewStoreTaskRun(t, globalDB, taskpkg.TaskRunStatusFailed)

		review := taskReviewForGlobalDBTest("review-rebind", taskRecord.ID, runRecord.ID)
		review.Policy = taskpkg.ReviewPolicyOnFailure
		stored, _, err := globalDB.RequestRunReview(ctx, &review)
		if err != nil {
			t.Fatalf("RequestRunReview() error = %v", err)
		}
		_, err = globalDB.BindRunReviewSession(ctx, taskpkg.BindRunReviewSessionRequest{
			ReviewID:          stored.ReviewID,
			SessionID:         "sess-stopped-reviewer",
			ReviewerAgentName: "reviewer",
			ReviewerPeerID:    "peer-stopped-reviewer",
			ReviewerChannelID: "channel-review",
		}, fixedTaskReviewStoreTime().Add(time.Minute))
		if err != nil {
			t.Fatalf("BindRunReviewSession(initial) error = %v", err)
		}

		bound, err := globalDB.BindRunReviewSession(ctx, taskpkg.BindRunReviewSessionRequest{
			ReviewID:          stored.ReviewID,
			SessionID:         "sess-active-reviewer",
			ReviewerAgentName: "reviewer",
			ReviewerPeerID:    "peer-active-reviewer",
			ReviewerChannelID: "channel-review",
		}, fixedTaskReviewStoreTime().Add(2*time.Minute))
		if err != nil {
			t.Fatalf("BindRunReviewSession(rebind) error = %v", err)
		}
		assertTaskRunReviewShape(t, bound, stored.ReviewID, taskpkg.RunReviewStatusInReview)
		if got, want := bound.ReviewerSessionID, "sess-active-reviewer"; got != want {
			t.Fatalf("ReviewerSessionID = %q, want %q", got, want)
		}
		if got, want := bound.ReviewerPeerID, "peer-active-reviewer"; got != want {
			t.Fatalf("ReviewerPeerID = %q, want %q", got, want)
		}

		if _, err := globalDB.LookupRunReviewBySession(ctx, "sess-stopped-reviewer"); !errors.Is(
			err,
			taskpkg.ErrRunReviewNotFound,
		) {
			t.Fatalf("LookupRunReviewBySession(stopped) error = %v, want ErrRunReviewNotFound", err)
		}
		lookup, err := globalDB.LookupRunReviewBySession(ctx, "sess-active-reviewer")
		if err != nil {
			t.Fatalf("LookupRunReviewBySession(active) error = %v", err)
		}
		if got, want := lookup.ReviewID, stored.ReviewID; got != want {
			t.Fatalf("lookup ReviewID = %q, want %q", got, want)
		}
	})

	t.Run("Should classify active reviewer session unique conflicts as invalid transitions", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		globalDB.now = fixedTaskReviewStoreTime
		taskRecord, runRecord := createReviewStoreTaskRun(t, globalDB, taskpkg.TaskRunStatusCompleted)
		first := taskReviewForGlobalDBTest("review-active-first", taskRecord.ID, runRecord.ID)
		storedFirst, _, err := globalDB.RequestRunReview(ctx, &first)
		if err != nil {
			t.Fatalf("RequestRunReview(first) error = %v", err)
		}
		if _, err := globalDB.BindRunReviewSession(ctx, taskpkg.BindRunReviewSessionRequest{
			ReviewID:  storedFirst.ReviewID,
			SessionID: "sess-active-reviewer",
		}, fixedTaskReviewStoreTime().Add(time.Minute)); err != nil {
			t.Fatalf("BindRunReviewSession(first) error = %v", err)
		}

		secondRun := taskRunForTest("run-review-store-second", taskRecord.ID)
		secondRun.Attempt = 2
		secondRun.Status = taskpkg.TaskRunStatusCompleted
		secondRun.EndedAt = fixedTaskReviewStoreTime().Add(2 * time.Minute)
		if err := globalDB.CreateTaskRun(ctx, secondRun); err != nil {
			t.Fatalf("CreateTaskRun(second) error = %v", err)
		}
		second := taskReviewForGlobalDBTest("review-active-second", taskRecord.ID, secondRun.ID)
		storedSecond, _, err := globalDB.RequestRunReview(ctx, &second)
		if err != nil {
			t.Fatalf("RequestRunReview(second) error = %v", err)
		}

		_, err = globalDB.BindRunReviewSession(ctx, taskpkg.BindRunReviewSessionRequest{
			ReviewID:  storedSecond.ReviewID,
			SessionID: "sess-active-reviewer",
		}, fixedTaskReviewStoreTime().Add(3*time.Minute))
		if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			t.Fatalf(
				"BindRunReviewSession(active conflict) error = %v, want %v",
				err,
				taskpkg.ErrInvalidStatusTransition,
			)
		}
	})

	t.Run("Should reject reviews before run terminal state", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		taskRecord, runRecord := createReviewStoreTaskRun(t, globalDB, taskpkg.TaskRunStatusQueued)

		review := taskReviewForGlobalDBTest("review-non-terminal", taskRecord.ID, runRecord.ID)
		_, _, err := globalDB.RequestRunReview(ctx, &review)
		if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			t.Fatalf("RequestRunReview(non-terminal) error = %v, want %v", err, taskpkg.ErrInvalidStatusTransition)
		}
	})

	t.Run("Should report review not found for missing rows", func(t *testing.T) {
		t.Parallel()

		globalDB := openTestGlobalDB(t)
		if _, err := globalDB.GetRunReview(testutil.Context(t), "missing-review"); !errors.Is(
			err,
			taskpkg.ErrRunReviewNotFound,
		) {
			t.Fatalf("GetRunReview(missing) error = %v, want ErrRunReviewNotFound", err)
		}
	})

	t.Run("Should record approved review verdict without continuation", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		globalDB.now = fixedTaskReviewStoreTime
		taskRecord, runRecord := createReviewStoreTaskRun(t, globalDB, taskpkg.TaskRunStatusCompleted)
		review := taskReviewForGlobalDBTest("review-approved", taskRecord.ID, runRecord.ID)
		stored, _, err := globalDB.RequestRunReview(ctx, &review)
		if err != nil {
			t.Fatalf("RequestRunReview() error = %v", err)
		}

		confidence := 0.92
		result, err := globalDB.RecordRunReview(
			ctx,
			taskpkg.RecordRunReviewRequest{
				ReviewID: stored.ReviewID,
				RunID:    stored.RunID,
				Verdict: taskpkg.RunReviewVerdict{
					Outcome:     taskpkg.RunReviewOutcomeApproved,
					Confidence:  &confidence,
					Reason:      "the run is complete",
					DeliveryID:  "delivery-approved",
					MissingWork: []byte(`[]`),
				},
			},
			reviewStoreActorContext(),
			fixedTaskReviewStoreTime().Add(time.Minute),
			"unused-run-id",
		)
		if err != nil {
			t.Fatalf("RecordRunReview(approved) error = %v", err)
		}
		if got, want := result.Review.Status, taskpkg.RunReviewStatusRecorded; got != want {
			t.Fatalf("Review.Status = %q, want %q", got, want)
		}
		if got, want := result.Review.Outcome, taskpkg.RunReviewOutcomeApproved; got != want {
			t.Fatalf("Review.Outcome = %q, want %q", got, want)
		}
		if result.ContinuationRun != nil {
			t.Fatalf("ContinuationRun = %#v, want nil", result.ContinuationRun)
		}
	})

	t.Run(
		"Should record rejected review and replay same delivery without duplicating continuation",
		func(t *testing.T) {
			t.Parallel()

			ctx := testutil.Context(t)
			globalDB := openTestGlobalDB(t)
			globalDB.now = fixedTaskReviewStoreTime
			seedLoopTestWorkspaces(t, globalDB, "ws-review-store")
			taskRecord := taskRecordForTest("task-review-store")
			if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
				t.Fatalf("CreateTask() error = %v", err)
			}
			wantParticipation := participation.Spec{
				Version:         participation.SpecVersion,
				Mode:            participation.ModeLive,
				WorkspaceID:     "ws-review-store",
				ChannelStrategy: participation.StrategyNamed,
				ChannelID:       "review-builders",
				Source:          participation.SourceExplicitRequest,
				Bounds: participation.Bounds{
					MaxWakes:         4,
					MaxWakeWallTime:  "30s",
					MaxTotalWallTime: "2m",
					MaxInputTokens:   4096,
					MaxOutputTokens:  4096,
					MaxWakeDepth:     4,
					CoalesceWindow:   "250ms",
				},
			}
			runRecord := taskRunForTest("run-review-store", taskRecord.ID)
			runRecord.Status = taskpkg.TaskRunStatusCompleted
			runRecord.EndedAt = fixedTaskReviewStoreTime()
			runRecord.WorkspaceID = wantParticipation.WorkspaceID
			runRecord.SetNetworkState(wantParticipation, "", "", "")
			if err := globalDB.CreateTaskRun(ctx, runRecord); err != nil {
				t.Fatalf("CreateTaskRun(Live parent) error = %v", err)
			}
			review := taskReviewForGlobalDBTest("review-rejected", taskRecord.ID, runRecord.ID)
			stored, _, err := globalDB.RequestRunReview(ctx, &review)
			if err != nil {
				t.Fatalf("RequestRunReview() error = %v", err)
			}

			confidence := 0.67
			req := taskpkg.RecordRunReviewRequest{
				ReviewID: stored.ReviewID,
				RunID:    stored.RunID,
				Verdict: taskpkg.RunReviewVerdict{
					Outcome:           taskpkg.RunReviewOutcomeRejected,
					Confidence:        &confidence,
					Reason:            "tests are missing",
					DeliveryID:        "delivery-rejected",
					MissingWork:       []byte(`["add regression tests"]`),
					NextRoundGuidance: "Add tests before resubmitting.",
				},
			}
			result, err := globalDB.RecordRunReview(
				ctx,
				req,
				reviewStoreActorContext(),
				fixedTaskReviewStoreTime().Add(time.Minute),
				"run-continuation",
			)
			if err != nil {
				t.Fatalf("RecordRunReview(rejected) error = %v", err)
			}
			assertRejectedContinuationRun(t, result, runRecord.ID, stored.ReviewID)
			if got := result.ContinuationRun.NetworkSpecSnapshot(); got != wantParticipation {
				t.Fatalf("continuation participation = %#v, want %#v", got, wantParticipation)
			}
			if got, want := result.ContinuationRun.WorkspaceID, wantParticipation.WorkspaceID; got != want {
				t.Fatalf("continuation workspace_id = %q, want %q", got, want)
			}

			replayed, err := globalDB.RecordRunReview(
				ctx,
				req,
				reviewStoreActorContext(),
				fixedTaskReviewStoreTime().Add(2*time.Minute),
				"run-duplicate",
			)
			if err != nil {
				t.Fatalf("RecordRunReview(replay) error = %v", err)
			}
			assertRejectedContinuationRun(t, replayed, runRecord.ID, stored.ReviewID)
			if got := replayed.ContinuationRun.NetworkSpecSnapshot(); got != wantParticipation {
				t.Fatalf("replayed continuation participation = %#v, want %#v", got, wantParticipation)
			}
			if got, want := replayed.ContinuationRun.ID, result.ContinuationRun.ID; got != want {
				t.Fatalf("replay continuation id = %q, want %q", got, want)
			}
		},
	)

	t.Run("Should reject rejected review when continuation would exceed max attempts", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		globalDB := openTestGlobalDB(t)
		globalDB.now = fixedTaskReviewStoreTime
		taskRecord, runRecord := createReviewStoreTaskRun(t, globalDB, taskpkg.TaskRunStatusCompleted)
		taskRecord.MaxAttempts = 1
		if err := globalDB.UpdateTask(ctx, taskRecord, coordinatorActorContextForTest()); err != nil {
			t.Fatalf("UpdateTask() error = %v", err)
		}
		review := taskReviewForGlobalDBTest("review-exhausted", taskRecord.ID, runRecord.ID)
		stored, _, err := globalDB.RequestRunReview(ctx, &review)
		if err != nil {
			t.Fatalf("RequestRunReview() error = %v", err)
		}

		confidence := 0.55
		_, err = globalDB.RecordRunReview(
			ctx,
			taskpkg.RecordRunReviewRequest{
				ReviewID: stored.ReviewID,
				RunID:    stored.RunID,
				Verdict: taskpkg.RunReviewVerdict{
					Outcome:           taskpkg.RunReviewOutcomeRejected,
					Confidence:        &confidence,
					Reason:            "needs another pass",
					DeliveryID:        "delivery-exhausted",
					MissingWork:       []byte(`["retry"]`),
					NextRoundGuidance: "Try again.",
				},
			},
			reviewStoreActorContext(),
			fixedTaskReviewStoreTime().Add(time.Minute),
			"run-exhausted",
		)
		if !errors.Is(err, taskpkg.ErrInvalidStatusTransition) {
			t.Fatalf("RecordRunReview(exhausted) error = %v, want %v", err, taskpkg.ErrInvalidStatusTransition)
		}

		recordedReview, err := globalDB.GetRunReview(ctx, stored.ReviewID)
		if err != nil {
			t.Fatalf("GetRunReview() error = %v", err)
		}
		if got, want := recordedReview.Status, taskpkg.RunReviewStatusRequested; got != want {
			t.Fatalf("review status = %q, want %q", got, want)
		}
		runs, err := globalDB.ListTaskRuns(ctx, taskpkg.RunQuery{TaskID: taskRecord.ID})
		if err != nil {
			t.Fatalf("ListTaskRuns() error = %v", err)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("len(runs) = %d, want %d", got, want)
		}
	})
}

func createReviewStoreTaskRun(
	t *testing.T,
	globalDB *GlobalDB,
	status taskpkg.RunStatus,
) (taskpkg.Task, taskpkg.Run) {
	t.Helper()

	ctx := testutil.Context(t)
	taskRecord := taskRecordForTest("task-review-store")
	if err := globalDB.CreateTask(ctx, taskRecord); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	runRecord := taskRunForTest("run-review-store", taskRecord.ID)
	runRecord.Status = status
	if taskpkg.IsTerminalRunStatus(status) {
		runRecord.EndedAt = fixedTaskReviewStoreTime()
	}
	if err := globalDB.CreateTaskRun(ctx, runRecord); err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	return taskRecord, runRecord
}

func taskReviewForGlobalDBTest(reviewID string, taskID string, runID string) taskpkg.RunReview {
	now := fixedTaskReviewStoreTime()
	return taskpkg.RunReview{
		ReviewID:    reviewID,
		TaskID:      taskID,
		RunID:       runID,
		Policy:      taskpkg.ReviewPolicyAlways,
		ReviewRound: 1,
		Attempt:     1,
		Status:      taskpkg.RunReviewStatusRequested,
		Reason:      "final run requires review",
		MissingWork: []byte(`[]`),
		RequestedAt: now,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func fixedTaskReviewStoreTime() time.Time {
	return time.Date(2026, 4, 14, 15, 30, 0, 0, time.UTC)
}

func reviewStoreActorContext() taskpkg.ActorContext {
	return taskpkg.ActorContext{
		Actor: taskpkg.ActorIdentity{
			Kind: taskpkg.ActorKindHuman,
			Ref:  "operator",
		},
		Origin: taskpkg.Origin{
			Kind: taskpkg.OriginKindCLI,
			Ref:  "agh",
		},
		Scope:     taskpkg.CallerScope{Operator: true},
		Authority: taskpkg.Authority{Read: true, Write: true},
	}
}

func assertRejectedContinuationRun(
	t *testing.T,
	result taskpkg.RunReviewResult,
	parentRunID string,
	reviewID string,
) {
	t.Helper()

	if result.ContinuationRun == nil {
		t.Fatal("ContinuationRun = nil, want queued continuation")
	}
	if got, want := result.ContinuationRun.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("ContinuationRun.Status = %q, want %q", got, want)
	}
	if result.ContinuationRun.Review == nil {
		t.Fatal("ContinuationRun.Review = nil, want review lineage")
	}
	if got, want := result.ContinuationRun.Review.ParentRunID, parentRunID; got != want {
		t.Fatalf("ContinuationRun.ParentRunID = %q, want %q", got, want)
	}
	if got, want := result.ContinuationRun.Review.ReviewID, reviewID; got != want {
		t.Fatalf("ContinuationRun.ReviewID = %q, want %q", got, want)
	}
	if got, want := string(result.ContinuationRun.Review.MissingWork), `["add regression tests"]`; got != want {
		t.Fatalf("ContinuationRun.MissingWork = %s, want %s", got, want)
	}
}

func assertTaskRunReviewShape(
	t *testing.T,
	review taskpkg.RunReview,
	reviewID string,
	status taskpkg.RunReviewStatus,
) {
	t.Helper()

	if got, want := review.ReviewID, reviewID; got != want {
		t.Fatalf("ReviewID = %q, want %q", got, want)
	}
	if got, want := review.Status, status; got != want {
		t.Fatalf("Status = %q, want %q", got, want)
	}
	if got, want := string(review.MissingWork), "[]"; got != want {
		t.Fatalf("MissingWork = %q, want %q", got, want)
	}
	if review.CreatedAt.IsZero() || review.UpdatedAt.IsZero() {
		t.Fatalf("timestamps were not persisted: %#v", review)
	}
}

func assertTaskRunReviewRequestLink(t *testing.T, globalDB *GlobalDB, runID string, reviewID string) {
	t.Helper()

	var linkedReviewID sql.NullString
	var required bool
	if err := globalDB.db.QueryRowContext(
		testutil.Context(t),
		`SELECT review_request_id, review_required FROM task_runs WHERE id = ?`,
		runID,
	).Scan(&linkedReviewID, &required); err != nil {
		t.Fatalf("QueryRowContext(task_runs review link) error = %v", err)
	}
	if !linkedReviewID.Valid || linkedReviewID.String != reviewID {
		t.Fatalf("review_request_id = %#v, want %q", linkedReviewID, reviewID)
	}
	if required {
		t.Fatal("review_required = true, want false after request link")
	}
}
