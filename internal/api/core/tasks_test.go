package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

func taskRunValueWithResult(run taskpkg.Run, result json.RawMessage) taskpkg.Run {
	run.SetResult(result)
	return run
}

func TestTaskPayloadBuildersPreserveIdentityOwnershipAndRunBindings(t *testing.T) {
	t.Parallel()

	taskMetadata := json.RawMessage(`{"priority":"high"}`)
	runResult := json.RawMessage(`{"ok":true}`)
	eventPayload := json.RawMessage(`{"action":"claim"}`)

	view := &taskpkg.View{
		Task: taskpkg.Task{
			ID:           "task-1",
			ProfileID:    store.DefaultProfileID,
			Identifier:   "TASK-1",
			Scope:        taskpkg.ScopeWorkspace,
			WorkspaceID:  "ws-alpha",
			ParentTaskID: "task-root",
			Title:        "Review task API",
			Description:  "Check handler wiring",
			Status:       taskpkg.TaskStatusInProgress,
			Owner:        &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "reviewers"},
			CreatedBy:    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
			Origin:       taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.create"},
			CreatedAt:    time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
			UpdatedAt:    time.Date(2026, 4, 14, 10, 1, 0, 0, time.UTC),
			Metadata:     taskMetadata,
		},
		Children: []taskpkg.Summary{{
			ID:        "task-child",
			Title:     "Follow up",
			Scope:     taskpkg.ScopeWorkspace,
			Status:    taskpkg.TaskStatusReady,
			CreatedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.create_child"},
			CreatedAt: time.Date(2026, 4, 14, 10, 2, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 14, 10, 2, 0, 0, time.UTC),
		}},
		Dependencies: []taskpkg.Dependency{{
			TaskID:          "task-1",
			DependsOnTaskID: "task-blocker",
			Kind:            taskpkg.DependencyKindBlocks,
			CreatedAt:       time.Date(2026, 4, 14, 10, 3, 0, 0, time.UTC),
		}},
		Runs: []taskpkg.Run{taskRunValueWithResult(taskpkg.Run{
			ID:             "run-1",
			TaskID:         "task-1",
			Status:         taskpkg.TaskRunStatusRunning,
			Attempt:        2,
			ClaimedBy:      &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
			SessionID:      "sess-1",
			Origin:         taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.start_run"},
			IdempotencyKey: "key-1",

			QueuedAt:  time.Date(2026, 4, 14, 10, 0, 0, 0, time.UTC),
			StartedAt: time.Date(2026, 4, 14, 10, 4, 0, 0, time.UTC),
		}, runResult)},
		Events: []taskpkg.Event{{
			ID:        "evt-1",
			TaskID:    "task-1",
			RunID:     "run-1",
			EventType: "task.run.started",
			Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.start_run"},
			Payload:   eventPayload,
			Timestamp: time.Date(2026, 4, 14, 10, 4, 0, 0, time.UTC),
		}},
	}

	payload := core.TaskDetailPayloadFromView(view)
	if payload.Task.CreatedBy.Ref != "local-user" || payload.Task.Origin.Ref != "tasks.create" {
		t.Fatalf("task payload identity = %#v", payload.Task)
	}
	if payload.Task.Owner == nil || payload.Task.Owner.Ref != "reviewers" {
		t.Fatalf("task payload owner = %#v", payload.Task.Owner)
	}
	if got := payload.Runs[0].SessionID; got != "sess-1" {
		t.Fatalf("run payload session_id = %q, want %q", got, "sess-1")
	}
	if got := payload.Runs[0].ClaimedBy; got == nil || got.Ref != "local-user" {
		t.Fatalf("run payload claimed_by = %#v", got)
	}
	if string(payload.Task.Metadata) != `{"priority":"high"}` {
		t.Fatalf("task payload metadata = %s", string(payload.Task.Metadata))
	}
	if string(payload.Runs[0].Result) != `{"ok":true}` {
		t.Fatalf("run payload result = %s", string(payload.Runs[0].Result))
	}

	taskMetadata[2] = 'X'
	runResult[2] = 'Y'
	eventPayload[2] = 'Z'
	if string(payload.Task.Metadata) != `{"priority":"high"}` {
		t.Fatalf("task payload metadata mutated = %s", string(payload.Task.Metadata))
	}
	if string(payload.Runs[0].Result) != `{"ok":true}` {
		t.Fatalf("run payload result mutated = %s", string(payload.Runs[0].Result))
	}
	if string(payload.Events[0].Payload) != `{"action":"claim"}` {
		t.Fatalf("event payload mutated = %s", string(payload.Events[0].Payload))
	}
}

func TestStatusForTaskError(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  error
		want int
	}{
		{name: "validation", err: core.NewTaskValidationError(context.Canceled), want: http.StatusBadRequest},
		{name: "payload too large", err: taskpkg.ErrPayloadTooLarge, want: http.StatusRequestEntityTooLarge},
		{name: "permission denied", err: taskpkg.ErrPermissionDenied, want: http.StatusForbidden},
		{name: "task not found", err: taskpkg.ErrTaskNotFound, want: http.StatusNotFound},
		{name: "workspace missing", err: workspacepkg.ErrWorkspaceNotFound, want: http.StatusNotFound},
		{name: "invalid transition", err: taskpkg.ErrInvalidStatusTransition, want: http.StatusConflict},
		{name: "daemon draining", err: admission.ErrDraining, want: http.StatusServiceUnavailable},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := core.StatusForTaskError(tc.err); got != tc.want {
				t.Fatalf("StatusForTaskError(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestBaseHandlersTaskExecutionProfileEndpoints(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 5, 10, 0, 0, 0, time.UTC)
	storedProfile := taskpkg.ExecutionProfile{
		TaskID: "task-1",
		Coordinator: taskpkg.CoordinatorProfile{
			Mode: taskpkg.CoordinatorModeGuided,
		},
		Worker: taskpkg.WorkerProfile{
			Mode:      taskpkg.WorkerModeSelect,
			AgentName: "worker-a",
			Provider:  "openai",
			Model:     "gpt-5.4",
		},

		CreatedAt: now,
		UpdatedAt: now,
	}

	var (
		gotSetProfile taskpkg.ExecutionProfile
		gotWorktree   taskpkg.WorktreePolicy
		deletedTaskID string
	)
	tasks := &testutil.StubTaskManager{
		GetExecutionProfileFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (taskpkg.ExecutionProfile, error) {
			if id != "task-1" || actor.Origin.Ref != "tasks.get_profile" {
				t.Fatalf("GetExecutionProfile(id, actor) = %q, %#v", id, actor)
			}
			return storedProfile, nil
		},
		SetExecutionProfileFn: func(
			_ context.Context,
			id string,
			profile *taskpkg.ExecutionProfile,
			actor taskpkg.ActorContext,
		) (taskpkg.ExecutionProfile, error) {
			if id != "task-1" || actor.Origin.Ref != "tasks.set_profile" {
				t.Fatalf("SetExecutionProfile(id, actor) = %q, %#v", id, actor)
			}
			if profile == nil {
				t.Fatal("SetExecutionProfile profile = nil")
				return taskpkg.ExecutionProfile{}, nil
			}
			gotSetProfile = *profile
			stored := *profile
			stored.UpdatedAt = now
			return stored, nil
		},
		SetWorktreePolicyFn: func(
			_ context.Context,
			id string,
			policy taskpkg.WorktreePolicy,
			actor taskpkg.ActorContext,
		) (taskpkg.ExecutionProfile, error) {
			if id != "task-1" || actor.Origin.Ref != "tasks.set_worktree_policy" {
				t.Fatalf("SetWorktreePolicy(id, actor) = %q, %#v", id, actor)
			}
			gotWorktree = policy
			stored := storedProfile
			stored.Worktree = policy
			return stored, nil
		},
		DeleteExecutionProfileFn: func(_ context.Context, id string, actor taskpkg.ActorContext) error {
			if actor.Origin.Ref != "tasks.delete_profile" {
				t.Fatalf("DeleteExecutionProfile actor = %#v", actor)
			}
			deletedTaskID = id
			return nil
		},
	}
	fixture := newHandlerFixtureWithTasks(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		tasks,
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)

	resp := performRequest(t, fixture.Engine, http.MethodGet, "/tasks/task-1/execution-profile", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("get profile status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	var getPayload contract.TaskExecutionProfileResponse
	testutil.DecodeJSONResponse(t, resp, &getPayload)

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPut,
		"/tasks/task-1/execution-profile",
		[]byte(
			`{"worker":{"mode":"select","agent_name":"worker-b"},"created_at":"2026-05-01T10:00:00Z","updated_at":"2026-05-02T10:00:00Z"}`,
		),
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("set profile status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	if gotSetProfile.TaskID != "task-1" ||
		gotSetProfile.Worker.Mode != taskpkg.WorkerModeSelect ||
		gotSetProfile.Worker.AgentName != "worker-b" ||
		!gotSetProfile.CreatedAt.IsZero() ||
		!gotSetProfile.UpdatedAt.IsZero() {
		t.Fatalf("set profile request = %#v", gotSetProfile)
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPatch,
		"/tasks/task-1/execution-profile/worktree",
		[]byte(`{"mode":"ref","worktree_ref":"feature-a"}`),
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("set worktree status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if gotWorktree.Mode != taskpkg.WorktreeModeRef || gotWorktree.WorktreeRef != "feature-a" {
		t.Fatalf("set worktree request = %#v", gotWorktree)
	}
	var worktreePayload contract.TaskExecutionProfileResponse
	testutil.DecodeJSONResponse(t, resp, &worktreePayload)
	if worktreePayload.Profile.Worktree != gotWorktree {
		t.Fatalf("set worktree payload = %#v, want %#v", worktreePayload.Profile.Worktree, gotWorktree)
	}

	resp = performRequest(t, fixture.Engine, http.MethodDelete, "/tasks/task-1/execution-profile", nil)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("delete profile status = %d, want %d; body=%s", resp.Code, http.StatusNoContent, resp.Body.String())
	}
	if resp.Body.Len() != 0 {
		t.Fatalf("delete profile body = %q, want empty body", resp.Body.String())
	}
	if deletedTaskID != "task-1" {
		t.Fatalf("deleted task id = %q, want task-1", deletedTaskID)
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPut,
		"/tasks/task-1/execution-profile",
		[]byte(`{"task_id":"other"}`),
	)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf(
			"set mismatched profile status = %d, want %d; body=%s",
			resp.Code,
			http.StatusBadRequest,
			resp.Body.String(),
		)
	}
	var mismatchPayload contract.ErrorPayload
	testutil.DecodeJSONResponse(t, resp, &mismatchPayload)
	if !strings.Contains(mismatchPayload.Error, `task_execution_profile.task_id must match task id "task-1"`) {
		t.Fatalf("set mismatched profile payload = %#v", mismatchPayload)
	}
}

func TestBaseHandlersTaskRunReviewEndpoints(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 5, 10, 30, 0, 0, time.UTC)
	review := taskpkg.RunReview{
		ReviewID:          "review-1",
		TaskID:            "task-1",
		RunID:             "run-1",
		Policy:            taskpkg.ReviewPolicyAlways,
		ReviewRound:       1,
		Attempt:           1,
		Status:            taskpkg.RunReviewStatusRequested,
		Reason:            "ready for review",
		MissingWork:       json.RawMessage(`[]`),
		ReviewerSessionID: "sess-review",
		RequestedAt:       now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	var (
		requestCalls int
		listQueries  []taskpkg.RunReviewQuery
		gotReviewID  string
		gotVerdict   taskpkg.RecordRunReviewRequest
	)
	tasks := &testutil.StubTaskManager{
		GetTaskFn: func(_ context.Context, taskID string, _ taskpkg.ActorContext) (*taskpkg.View, error) {
			if taskID != "task-1" {
				t.Fatalf("GetTask taskID = %q, want task-1", taskID)
			}
			return &taskpkg.View{Task: taskpkg.Task{
				ID: taskID, ProfileID: store.DefaultProfileID,
			}}, nil
		},
		RunDetailFn: func(_ context.Context, runID string, actor taskpkg.ActorContext) (*taskpkg.RunDetailView, error) {
			if runID != "run-1" || actor.Origin.Ref != "tasks.request_review" {
				t.Fatalf("RunDetail(runID, actor) = %q, %#v", runID, actor)
			}
			return &taskpkg.RunDetailView{
				Run: taskpkg.Run{
					ID:      "run-1",
					TaskID:  "task-1",
					Status:  taskpkg.TaskRunStatusCompleted,
					Attempt: 1,
					Origin:  taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.run.start"},
				},
			}, nil
		},
		RequestRunReviewFn: func(
			_ context.Context,
			req taskpkg.RunReviewRequest,
			actor taskpkg.ActorContext,
		) (taskpkg.RunReview, bool, error) {
			requestCalls++
			if actor.Origin.Ref != "tasks.request_review" ||
				req.TaskID != "task-1" ||
				req.RunID != "run-1" ||
				req.Policy != taskpkg.ReviewPolicyAlways ||
				req.Reason != "ready for review" {
				t.Fatalf("RequestRunReview(req, actor) = %#v, %#v", req, actor)
			}
			return review, true, nil
		},
		ListRunReviewsFn: func(
			_ context.Context,
			query taskpkg.RunReviewQuery,
			actor taskpkg.ActorContext,
		) ([]taskpkg.RunReview, error) {
			if actor.Origin.Ref != "tasks.list_reviews" {
				t.Fatalf("ListRunReviews actor = %#v", actor)
			}
			listQueries = append(listQueries, query)
			return []taskpkg.RunReview{review}, nil
		},
		GetRunReviewFn: func(_ context.Context, reviewID string, actor taskpkg.ActorContext) (taskpkg.RunReview, error) {
			if actor.Origin.Ref != "tasks.get_review" {
				t.Fatalf("GetRunReview actor = %#v", actor)
			}
			gotReviewID = reviewID
			return review, nil
		},
		RecordRunReviewFn: func(
			_ context.Context,
			req taskpkg.RecordRunReviewRequest,
			actor taskpkg.ActorContext,
		) (taskpkg.RunReviewResult, error) {
			if actor.Origin.Ref != "tasks.submit_review" {
				t.Fatalf("RecordRunReview actor = %#v", actor)
			}
			gotVerdict = req
			recorded := review
			recorded.Status = taskpkg.RunReviewStatusRecorded
			recorded.Outcome = taskpkg.RunReviewOutcomeRejected
			continuation := &taskpkg.Run{
				ID:      "run-2",
				TaskID:  "task-1",
				Status:  taskpkg.TaskRunStatusQueued,
				Attempt: 2,
				Origin:  taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.run.review_retry"},
			}
			return taskpkg.RunReviewResult{Review: recorded, ContinuationRun: continuation}, nil
		},
	}
	fixture := newHandlerFixtureWithTasks(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		tasks,
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)

	resp := performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/task-runs/run-1/reviews",
		[]byte(`{"reason":"ready for review"}`),
	)
	if resp.Code != http.StatusCreated {
		t.Fatalf("request review status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
	}
	var requestPayload contract.TaskRunReviewRequestResponse
	testutil.DecodeJSONResponse(t, resp, &requestPayload)
	if !requestPayload.Created || requestPayload.Review.ReviewID != "review-1" ||
		requestPayload.Review.ProfileID != store.DefaultProfileID || requestPayload.Review.ProfileName != "default" {
		t.Fatalf("request review payload = %#v", requestPayload)
	}
	if requestCalls != 1 {
		t.Fatalf("request calls = %d, want 1", requestCalls)
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/task-runs/run-1/reviews?status=requested&reviewer_session_id=sess-review&limit=2",
		nil,
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("list run reviews status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	resp = performRequest(t, fixture.Engine, http.MethodGet, "/tasks/task-1/reviews?status=requested", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("list task reviews status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if len(listQueries) != 2 ||
		listQueries[0].RunID != "run-1" ||
		listQueries[0].ReviewerSessionID != "sess-review" ||
		listQueries[0].Limit != 2 ||
		listQueries[1].TaskID != "task-1" {
		t.Fatalf("review list queries = %#v", listQueries)
	}

	resp = performRequest(t, fixture.Engine, http.MethodGet, "/task-reviews/review-1", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("get review status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if gotReviewID != "review-1" {
		t.Fatalf("review id = %q, want review-1", gotReviewID)
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/task-reviews/review-1/verdict",
		[]byte(
			`{"run_id":"run-1","verdict":{"outcome":"rejected","confidence":0.8,"reason":"missing tests","delivery_id":"delivery-1","missing_work":["tests"],"next_round_guidance":"add tests"}}`,
		),
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("submit review status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if gotVerdict.ReviewID != "review-1" ||
		gotVerdict.RunID != "run-1" ||
		gotVerdict.Verdict.Outcome != taskpkg.RunReviewOutcomeRejected ||
		gotVerdict.Verdict.Confidence == nil ||
		*gotVerdict.Verdict.Confidence != 0.8 ||
		string(gotVerdict.Verdict.MissingWork) != `["tests"]` {
		t.Fatalf("record review request = %#v", gotVerdict)
	}
	var verdictPayload contract.TaskRunReviewVerdictResponse
	testutil.DecodeJSONResponse(t, resp, &verdictPayload)
	if verdictPayload.Review.Status != taskpkg.RunReviewStatusRecorded ||
		verdictPayload.Review.ProfileName != "default" ||
		verdictPayload.ContinuationRun == nil ||
		verdictPayload.ContinuationRun.ID != "run-2" ||
		verdictPayload.ContinuationRun.ProfileName != "default" {
		t.Fatalf("verdict payload = %#v", verdictPayload)
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/task-runs/run-1/reviews",
		[]byte(`{"run_id":"other"}`),
	)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf(
			"mismatched review status = %d, want %d; body=%s",
			resp.Code,
			http.StatusBadRequest,
			resp.Body.String(),
		)
	}
	var mismatchReviewPayload contract.ErrorPayload
	testutil.DecodeJSONResponse(t, resp, &mismatchReviewPayload)
	if !strings.Contains(mismatchReviewPayload.Error, `task_run_review.run_id must match run id "run-1"`) {
		t.Fatalf("mismatched review payload = %#v", mismatchReviewPayload)
	}
	if requestCalls != 1 {
		t.Fatalf("request calls after mismatch = %d, want 1", requestCalls)
	}
}

func TestBaseHandlersTaskSchedulerControlEndpoints(t *testing.T) {
	t.Parallel()

	t.Run("Should map task pause and resume requests", func(t *testing.T) {
		t.Parallel()

		var pauseRequest taskpkg.PauseTaskRequest
		var resumeRequest taskpkg.ResumeTaskRequest
		tasks := &testutil.StubTaskManager{
			PauseTaskFn: func(
				_ context.Context,
				id string,
				req taskpkg.PauseTaskRequest,
				_ taskpkg.ActorContext,
			) (*taskpkg.Task, error) {
				if id != "task-1" {
					t.Fatalf("PauseTask id = %q, want task-1", id)
				}
				pauseRequest = req
				return &taskpkg.Task{
					ID:           id,
					ProfileID:    store.DefaultProfileID,
					Scope:        taskpkg.ScopeGlobal,
					Title:        "Paused task",
					Status:       taskpkg.TaskStatusReady,
					Paused:       true,
					PausedBy:     "human:operator",
					PausedReason: req.Reason,
					CreatedBy:    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "operator"},
					Origin:       taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.pause"},
					CreatedAt:    time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
					UpdatedAt:    time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC),
				}, nil
			},
			ResumeTaskFn: func(
				_ context.Context,
				id string,
				req taskpkg.ResumeTaskRequest,
				_ taskpkg.ActorContext,
			) (*taskpkg.Task, error) {
				if id != "task-1" {
					t.Fatalf("ResumeTask id = %q, want task-1", id)
				}
				resumeRequest = req
				return &taskpkg.Task{
					ID:        id,
					ProfileID: store.DefaultProfileID,
					Scope:     taskpkg.ScopeGlobal,
					Title:     "Resumed task",
					Status:    taskpkg.TaskStatusReady,
					CreatedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "operator"},
					Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.resume"},
					CreatedAt: time.Date(2026, 5, 21, 10, 1, 0, 0, time.UTC),
					UpdatedAt: time.Date(2026, 5, 21, 10, 1, 0, 0, time.UTC),
				}, nil
			},
		}
		fixture := newHandlerFixtureWithTasks(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			tasks,
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		pauseResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/tasks/task-1/pause",
			[]byte("{\"reason\":\"provider incident\",\"metadata\":{\"source\":\"api\"}}"),
		)
		if pauseResp.Code != http.StatusOK {
			t.Fatalf("pause response status = %d body = %s", pauseResp.Code, pauseResp.Body.String())
		}
		if pauseRequest.Reason != "provider incident" || string(pauseRequest.Metadata) != "{\"source\":\"api\"}" {
			t.Fatalf("PauseTask request = %#v, want reason and metadata", pauseRequest)
		}
		var paused contract.TaskResponse
		if err := json.Unmarshal(pauseResp.Body.Bytes(), &paused); err != nil {
			t.Fatalf("decode pause response: %v", err)
		}
		if !paused.Task.Paused || paused.Task.PausedReason != "provider incident" {
			t.Fatalf("pause response task = %#v, want paused reason", paused.Task)
		}

		resumeResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/tasks/task-1/resume",
			[]byte("{\"metadata\":{\"source\":\"api\"}}"),
		)
		if resumeResp.Code != http.StatusOK {
			t.Fatalf("resume response status = %d body = %s", resumeResp.Code, resumeResp.Body.String())
		}
		if string(resumeRequest.Metadata) != "{\"source\":\"api\"}" {
			t.Fatalf("ResumeTask request = %#v, want metadata", resumeRequest)
		}
	})

	t.Run("Should map scheduler status pause drain and backlog requests", func(t *testing.T) {
		t.Parallel()

		var pauseReason string
		var resumeReason string
		var drainRequest taskpkg.SchedulerDrainRequest
		var backlogQuery taskpkg.SchedulerBacklogQuery
		now := time.Date(2026, 5, 21, 10, 15, 0, 0, time.UTC)
		tasks := &testutil.StubTaskManager{
			SchedulerStatusFn: func(_ context.Context, _ taskpkg.ActorContext) (taskpkg.SchedulerStatus, error) {
				return taskpkg.SchedulerStatus{Paused: false, ActiveClaimCount: 2, QueuedRunCount: 3, AsOf: now}, nil
			},
			PauseSchedulerFn: func(
				_ context.Context,
				req taskpkg.SchedulerPauseRequest,
				_ taskpkg.ActorContext,
			) (taskpkg.SchedulerStatus, error) {
				pauseReason = req.Reason
				return taskpkg.SchedulerStatus{
					Paused:           true,
					PausedBy:         "human:operator",
					PausedReason:     req.Reason,
					ActiveClaimCount: 2,
					QueuedRunCount:   3,
					PausedTaskCount:  1,
					AsOf:             now,
				}, nil
			},
			ResumeSchedulerFn: func(
				_ context.Context,
				req taskpkg.SchedulerResumeRequest,
				_ taskpkg.ActorContext,
			) (taskpkg.SchedulerStatus, error) {
				resumeReason = req.Reason
				return taskpkg.SchedulerStatus{Paused: false, QueuedRunCount: 3, AsOf: now.Add(time.Minute)}, nil
			},
			DrainSchedulerFn: func(
				_ context.Context,
				req taskpkg.SchedulerDrainRequest,
				_ taskpkg.ActorContext,
			) (taskpkg.SchedulerDrainResult, error) {
				drainRequest = req
				return taskpkg.SchedulerDrainResult{
					Status: taskpkg.SchedulerStatus{
						Paused:           true,
						ActiveClaimCount: 0,
						AsOf:             now.Add(2 * time.Minute),
					},
					Completed:   true,
					StartedAt:   now,
					CompletedAt: now.Add(time.Second),
				}, nil
			},
			SchedulerBacklogFn: func(
				_ context.Context,
				query taskpkg.SchedulerBacklogQuery,
				_ taskpkg.ActorContext,
			) (taskpkg.SchedulerBacklog, error) {
				backlogQuery = query
				return taskpkg.SchedulerBacklog{
					Total: 1,
					Runs: []taskpkg.SchedulerBacklogRun{{
						Task: taskpkg.Task{
							ID:        "task-paused",
							ProfileID: "profile-marketing",
							Scope:     taskpkg.ScopeWorkspace,
							Title:     "Paused task",
							Status:    taskpkg.TaskStatusReady,
							CreatedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "operator"},
							Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.create"},
							CreatedAt: now,
							UpdatedAt: now,
						},
						Run: taskpkg.Run{
							ProfileID: "profile-marketing",
							ID:        "run-paused",
							TaskID:    "task-paused",
							Status:    taskpkg.TaskRunStatusQueued,
							Attempt:   1,
							Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.start"},
							QueuedAt:  now,
						},
						EffectivePaused: true,
						PausedByTaskID:  "task-root",
					}},
				}, nil
			},
		}
		fixture := newHandlerFixtureWithTasks(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			tasks,
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		fixture.Handlers.Profiles = sessionProfileServiceStub{}
		statusResp := performRequest(t, fixture.Engine, http.MethodGet, "/scheduler", nil)
		if statusResp.Code != http.StatusOK {
			t.Fatalf("scheduler status response status = %d body = %s", statusResp.Code, statusResp.Body.String())
		}
		var status contract.SchedulerStatusResponse
		if err := json.Unmarshal(statusResp.Body.Bytes(), &status); err != nil {
			t.Fatalf("decode scheduler status response: %v", err)
		}
		if status.Scheduler.QueuedRunCount != 3 || status.Scheduler.ActiveClaimCount != 2 {
			t.Fatalf("scheduler status = %#v, want queue pressure", status.Scheduler)
		}

		pauseResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/scheduler/pause",
			[]byte("{\"reason\":\"deploy freeze\"}"),
		)
		if pauseResp.Code != http.StatusOK {
			t.Fatalf("scheduler pause response status = %d body = %s", pauseResp.Code, pauseResp.Body.String())
		}
		if pauseReason != "deploy freeze" {
			t.Fatalf("PauseScheduler reason = %q, want deploy freeze", pauseReason)
		}
		var pausedScheduler contract.SchedulerStatusResponse
		if err := json.Unmarshal(pauseResp.Body.Bytes(), &pausedScheduler); err != nil {
			t.Fatalf("decode scheduler pause response: %v", err)
		}
		if !pausedScheduler.Scheduler.Paused || pausedScheduler.Scheduler.PausedTaskCount != 1 {
			t.Fatalf("pause response = %#v, want paused with task count", pausedScheduler.Scheduler)
		}

		resumeResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/scheduler/resume",
			[]byte("{\"reason\":\"deploy complete\"}"),
		)
		if resumeResp.Code != http.StatusOK {
			t.Fatalf("scheduler resume response status = %d body = %s", resumeResp.Code, resumeResp.Body.String())
		}
		if resumeReason != "deploy complete" {
			t.Fatalf("ResumeScheduler reason = %q, want deploy complete", resumeReason)
		}
		var resumedScheduler contract.SchedulerStatusResponse
		if err := json.Unmarshal(resumeResp.Body.Bytes(), &resumedScheduler); err != nil {
			t.Fatalf("decode scheduler resume response: %v", err)
		}
		if resumedScheduler.Scheduler.Paused || resumedScheduler.Scheduler.QueuedRunCount != 3 {
			t.Fatalf("resume response = %#v, want unpaused with queued count", resumedScheduler.Scheduler)
		}

		drainResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/scheduler/drain",
			[]byte("{\"reason\":\"deploy freeze\",\"timeout_seconds\":2}"),
		)
		if drainResp.Code != http.StatusOK {
			t.Fatalf("scheduler drain response status = %d body = %s", drainResp.Code, drainResp.Body.String())
		}
		if drainRequest.Reason != "deploy freeze" || drainRequest.Timeout != 2*time.Second {
			t.Fatalf("DrainScheduler request = %#v, want reason and 2s timeout", drainRequest)
		}

		backlogResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/scheduler/backlog?limit=7&scope=workspace&workspace=ws-alpha&include_paused=true",
			nil,
		)
		if backlogResp.Code != http.StatusOK {
			t.Fatalf("scheduler backlog response status = %d body = %s", backlogResp.Code, backlogResp.Body.String())
		}
		if backlogQuery.Limit != 7 ||
			backlogQuery.Scope != taskpkg.ScopeWorkspace ||
			backlogQuery.WorkspaceID != "ws-alpha" ||
			!backlogQuery.IncludePaused {
			t.Fatalf("SchedulerBacklog query = %#v, want parsed filters", backlogQuery)
		}
		var backlog contract.SchedulerBacklogResponse
		if err := json.Unmarshal(backlogResp.Body.Bytes(), &backlog); err != nil {
			t.Fatalf("decode scheduler backlog response: %v", err)
		}
		if backlog.Backlog.Total != 1 || len(backlog.Backlog.Runs) != 1 ||
			!backlog.Backlog.Runs[0].Task.EffectivePaused ||
			backlog.Backlog.Runs[0].Task.PausedByTaskID != "task-root" {
			t.Fatalf("backlog response = %#v, want inherited pause metadata", backlog.Backlog)
		}
		item := backlog.Backlog.Runs[0]
		if item.Task.ProfileID != "profile-marketing" || item.Task.ProfileName != "marketing" ||
			item.Task.ProfileColor != "#E8572A" || item.Task.ProfileIcon != "megaphone" ||
			item.Run.ProfileID != "profile-marketing" || item.Run.ProfileName != "marketing" ||
			item.Run.ProfileColor != "#E8572A" || item.Run.ProfileIcon != "megaphone" {
			t.Fatalf("backlog owners = %#v, want archived marketing profile", item)
		}
	})

	t.Run("Should reject scheduler drain timeouts above the duration ceiling", func(t *testing.T) {
		t.Parallel()

		calls := 0
		var captured taskpkg.SchedulerDrainRequest
		now := time.Date(2026, 5, 21, 10, 15, 0, 0, time.UTC)
		tasks := &testutil.StubTaskManager{
			DrainSchedulerFn: func(
				_ context.Context,
				req taskpkg.SchedulerDrainRequest,
				_ taskpkg.ActorContext,
			) (taskpkg.SchedulerDrainResult, error) {
				calls++
				captured = req
				return taskpkg.SchedulerDrainResult{
					Status:      taskpkg.SchedulerStatus{Paused: true, AsOf: now},
					Completed:   true,
					StartedAt:   now,
					CompletedAt: now,
				}, nil
			},
		}
		fixture := newHandlerFixtureWithTasks(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			tasks,
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		maximum := contract.SchedulerDrainTimeoutMaxSeconds
		accepted := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/scheduler/drain",
			fmt.Appendf(nil, "{\"timeout_seconds\":%d}", maximum),
		)
		if accepted.Code != http.StatusOK {
			t.Fatalf("maximum drain status = %d, want 200; body=%s", accepted.Code, accepted.Body.String())
		}
		if calls != 1 || captured.Timeout != time.Duration(maximum)*time.Second {
			t.Fatalf("maximum drain calls/request = %d/%#v, want one exact-duration call", calls, captured)
		}

		for _, invalidBody := range [][]byte{
			[]byte(`{"timeout_seconds":-1}`),
			fmt.Appendf(nil, "{\"timeout_seconds\":%d}", maximum+1),
		} {
			rejected := performRequest(t, fixture.Engine, http.MethodPost, "/scheduler/drain", invalidBody)
			if rejected.Code != http.StatusUnprocessableEntity {
				t.Fatalf(
					"invalid drain %s status = %d, want 422; body=%s",
					invalidBody,
					rejected.Code,
					rejected.Body.String(),
				)
			}
			var payload contract.ErrorPayload
			if err := json.Unmarshal(rejected.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode invalid drain response: %v", err)
			}
			if strings.TrimSpace(payload.Error) == "" {
				t.Fatalf("invalid drain error payload = %#v, want non-empty error", payload)
			}
		}
		if calls != 1 {
			t.Fatalf("DrainScheduler calls after invalid requests = %d, want 1", calls)
		}

		malformed := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/scheduler/drain",
			[]byte(`{"timeout_seconds":`),
		)
		if malformed.Code != http.StatusBadRequest {
			t.Fatalf("malformed drain status = %d, want 400; body=%s", malformed.Code, malformed.Body.String())
		}
		if calls != 1 {
			t.Fatalf("DrainScheduler calls after malformed request = %d, want 1", calls)
		}
	})
}

func TestBaseHandlersTaskValidationAndErrorMapping(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRejectInvalidScopeWorkspaceAndChannelInputs", func(t *testing.T) {
		t.Parallel()

		tasks := &testutil.StubTaskManager{
			CreateTaskFn: func(context.Context, taskpkg.CreateTask, taskpkg.ActorContext) (*taskpkg.Task, error) {
				t.Fatal("CreateTask should not be called for invalid task input")
				return nil, nil
			},
		}
		fixture := newHandlerFixtureWithTasks(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			tasks,
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/tasks",
			[]byte(`{"scope":"workspace","title":"Broken"}`),
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf(
				"workspace create status = %d, want %d; body=%s",
				resp.Code,
				http.StatusBadRequest,
				resp.Body.String(),
			)
		}

		resp = performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/tasks",
			[]byte(`{"scope":"global","title":"Broken","unsupported_field":"bad.channel"}`),
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf(
				"unknown-field create status = %d, want %d; body=%s",
				resp.Code,
				http.StatusBadRequest,
				resp.Body.String(),
			)
		}
		body := resp.Body.String()
		if !strings.Contains(body, "unknown_field") || !strings.Contains(body, "unsupported_field") {
			t.Fatalf("unknown-field create body = %s, want unknown_field with unsupported_field named", body)
		}
	})

	t.Run("ShouldRejectUnknownWorkspaceAndInvalidOwnerInput", func(t *testing.T) {
		t.Parallel()

		tasks := &testutil.StubTaskManager{
			CreateTaskFn: func(context.Context, taskpkg.CreateTask, taskpkg.ActorContext) (*taskpkg.Task, error) {
				t.Fatal("CreateTask should not be called when workspace lookup fails")
				return nil, nil
			},
			UpdateTaskFn: func(context.Context, string, taskpkg.Patch, taskpkg.ActorContext) (*taskpkg.Task, error) {
				t.Fatal("UpdateTask should not be called for invalid owner input")
				return nil, nil
			},
		}
		workspaces := testutil.StubWorkspaceService{
			GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
				return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
			},
		}
		fixture := newHandlerFixtureWithTasks(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			tasks,
			workspaces,
			nil,
			nil,
		)

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/tasks",
			[]byte(`{"scope":"workspace","workspace":"missing","title":"Broken"}`),
		)
		if resp.Code != http.StatusNotFound {
			t.Fatalf(
				"workspace lookup status = %d, want %d; body=%s",
				resp.Code,
				http.StatusNotFound,
				resp.Body.String(),
			)
		}

		resp = performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/tasks/task-1",
			[]byte(`{"owner":{"kind":"bogus","ref":"ops"}}`),
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf(
				"invalid owner status = %d, want %d; body=%s",
				resp.Code,
				http.StatusBadRequest,
				resp.Body.String(),
			)
		}
	})

	t.Run("ShouldRejectGlobalWorkspaceBindingsWithoutWorkspaceLookup", func(t *testing.T) {
		t.Parallel()

		workspaceLookups := 0
		tasks := &testutil.StubTaskManager{
			ListTasksFn: func(context.Context, taskpkg.Query, taskpkg.ActorContext) ([]taskpkg.Summary, error) {
				t.Fatal("ListTasks should not be called when global scope includes workspace filter")
				return nil, nil
			},
			CreateTaskFn: func(context.Context, taskpkg.CreateTask, taskpkg.ActorContext) (*taskpkg.Task, error) {
				t.Fatal("CreateTask should not be called when global scope includes workspace binding")
				return nil, nil
			},
			CreateChildTaskFn: func(context.Context, string, taskpkg.CreateTask, taskpkg.ActorContext) (*taskpkg.Task, error) {
				t.Fatal("CreateChildTask should not be called when global scope includes workspace binding")
				return nil, nil
			},
		}
		workspaces := testutil.StubWorkspaceService{
			GetFn: func(context.Context, string) (workspacepkg.Workspace, error) {
				workspaceLookups++
				return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
			},
		}
		fixture := newHandlerFixtureWithTasks(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			tasks,
			workspaces,
			nil,
			nil,
		)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/tasks?scope=global&workspace=missing", nil)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("global list status = %d, want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
		}

		resp = performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/tasks",
			[]byte(`{"scope":"global","workspace":"missing","title":"Broken"}`),
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf(
				"global create status = %d, want %d; body=%s",
				resp.Code,
				http.StatusBadRequest,
				resp.Body.String(),
			)
		}

		resp = performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/tasks/task-root/children",
			[]byte(`{"scope":"global","workspace":"missing","title":"Broken child"}`),
		)
		if resp.Code != http.StatusBadRequest {
			t.Fatalf(
				"global child create status = %d, want %d; body=%s",
				resp.Code,
				http.StatusBadRequest,
				resp.Body.String(),
			)
		}

		if workspaceLookups != 0 {
			t.Fatalf("workspace lookup count = %d, want 0", workspaceLookups)
		}
	})

	t.Run("ShouldMapTaskDomainErrorsToStableStatuses", func(t *testing.T) {
		t.Parallel()

		tasks := &testutil.StubTaskManager{
			GetTaskFn: func(context.Context, string, taskpkg.ActorContext) (*taskpkg.View, error) {
				return nil, taskpkg.ErrTaskNotFound
			},
			UpdateTaskFn: func(context.Context, string, taskpkg.Patch, taskpkg.ActorContext) (*taskpkg.Task, error) {
				return nil, taskpkg.ErrPermissionDenied
			},
			StartRunFn: func(context.Context, string, taskpkg.StartRun, taskpkg.ActorContext) (*taskpkg.Run, error) {
				return nil, taskpkg.ErrInvalidStatusTransition
			},
		}
		fixture := newHandlerFixtureWithTasks(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			tasks,
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)

		resp := performRequest(t, fixture.Engine, http.MethodGet, "/tasks/missing", nil)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("get missing status = %d, want %d; body=%s", resp.Code, http.StatusNotFound, resp.Body.String())
		}

		resp = performRequest(t, fixture.Engine, http.MethodPatch, "/tasks/task-1", []byte(`{"title":"rename"}`))
		if resp.Code != http.StatusForbidden {
			t.Fatalf(
				"update forbidden status = %d, want %d; body=%s",
				resp.Code,
				http.StatusForbidden,
				resp.Body.String(),
			)
		}

		resp = performRequest(t, fixture.Engine, http.MethodPost, "/task-runs/run-1/start", []byte(`{}`))
		if resp.Code != http.StatusConflict {
			t.Fatalf("start conflict status = %d, want %d; body=%s", resp.Code, http.StatusConflict, resp.Body.String())
		}
	})
}

func TestBaseHandlersTaskMutationOwners(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve the selected task owner in an update response", func(t *testing.T) {
		t.Parallel()
		tasks := &testutil.StubTaskManager{
			UpdateTaskFn: func(_ context.Context, id string, patch taskpkg.Patch, actor taskpkg.ActorContext) (*taskpkg.Task, error) {
				if actor.ReadScope.ProfileID != "profile-marketing" {
					t.Fatalf("update profile = %q, want marketing", actor.ReadScope.ProfileID)
				}
				return &taskpkg.Task{ID: id, ProfileID: "profile-marketing", Title: *patch.Title}, nil
			},
		}
		fixture := newHandlerFixtureWithTasks(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			tasks,
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.Profiles = sessionProfileServiceStub{}
		response := performRequest(
			t,
			fixture.Engine,
			http.MethodPatch,
			"/tasks/task-marketing?profile=marketing",
			[]byte(`{"title":"Publication plan"}`),
		)
		if response.Code != http.StatusOK {
			t.Fatalf("update status = %d, body = %s, want success", response.Code, response.Body)
		}
		var payload contract.TaskResponse
		testutil.DecodeJSONResponse(t, response, &payload)
		if payload.Task.Title != "Publication plan" || payload.Task.ProfileID != "profile-marketing" ||
			payload.Task.ProfileName != "marketing" || payload.Task.ProfileColor != "#E8572A" || payload.Task.ProfileIcon != "megaphone" {
			t.Fatalf("updated task = %#v, want requested title and marketing owner", payload.Task)
		}
	})
	for _, tc := range []struct {
		name       string
		path       string
		wantStatus int
		withTask   bool
	}{
		{"Should preserve owners when starting a task", "/tasks/task-marketing/start", http.StatusCreated, true},
		{"Should preserve owners when publishing a task", "/tasks/task-marketing/publish", http.StatusOK, true},
		{"Should preserve owners when approving a task", "/tasks/task-marketing/approve", http.StatusCreated, true},
		{"Should preserve the owner when starting a run", "/task-runs/run-marketing/start", http.StatusOK, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			run := taskpkg.Run{
				ID: "run-marketing", TaskID: "task-marketing", ProfileID: "profile-marketing",
				Status: taskpkg.TaskRunStatusQueued,
			}
			if !tc.withTask {
				run.Status = taskpkg.TaskRunStatusRunning
			}
			startTask := func(_ context.Context, id string, _ taskpkg.ExecutionRequest, actor taskpkg.ActorContext) (*taskpkg.Execution, error) {
				if actor.ReadScope.ProfileID != run.ProfileID {
					t.Fatalf("execution profile = %q, want marketing", actor.ReadScope.ProfileID)
				}
				return &taskpkg.Execution{Task: taskpkg.Task{ID: id, ProfileID: run.ProfileID}, Run: run}, nil
			}
			tasks := &testutil.StubTaskManager{
				StartTaskFn:   startTask,
				PublishTaskFn: startTask,
				ApproveTaskFn: startTask,
				StartRunFn: func(_ context.Context, id string, _ taskpkg.StartRun, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
					if id != run.ID || actor.ReadScope.ProfileID != run.ProfileID {
						t.Fatalf("start run = %q, profile = %q, want marketing run", id, actor.ReadScope.ProfileID)
					}
					return &run, nil
				},
			}
			fixture := newHandlerFixtureWithTasks(t, testutil.StubSessionManager{}, testutil.StubObserver{},
				tasks, testutil.StubWorkspaceService{}, nil, nil)
			fixture.Handlers.Profiles = sessionProfileServiceStub{}
			response := performRequest(t, fixture.Engine, http.MethodPost, tc.path+"?profile=marketing", []byte(`{}`))
			if response.Code != tc.wantStatus {
				t.Fatalf("status = %d, body = %s, want %d", response.Code, response.Body, tc.wantStatus)
			}
			var payload contract.TaskExecutionResponse
			testutil.DecodeJSONResponse(t, response, &payload)
			if payload.Run.ProfileID != run.ProfileID || payload.Run.ProfileName != "marketing" ||
				payload.Run.ProfileColor != "#E8572A" || payload.Run.ProfileIcon != "megaphone" {
				t.Fatalf("run owner = %#v, want marketing identity", payload.Run)
			}
			if tc.withTask && (payload.Task.ProfileID != run.ProfileID || payload.Task.ProfileName != "marketing" ||
				payload.Task.ProfileColor != "#E8572A" || payload.Task.ProfileIcon != "megaphone") {
				t.Fatalf("task owner = %#v, want marketing identity", payload.Task)
			}
		})
	}
}

func TestBaseHandlersTaskInspectOwners(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		path string
	}{
		{"Should label the owner in task inspection", "/tasks/task-marketing/inspect"},
		{"Should label the owner in run inspection", "/runs/run-marketing/inspect"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			inspect := func(_ context.Context, _ string, actor taskpkg.ActorContext) (*taskpkg.InspectView, error) {
				if actor.ReadScope.ProfileID != "profile-marketing" {
					t.Fatalf("inspect profile = %q, want marketing", actor.ReadScope.ProfileID)
				}
				return &taskpkg.InspectView{
					Target: taskpkg.InspectTargetTask,
					Task: taskpkg.Summary{
						ID: "task-marketing", ProfileID: "profile-marketing", Title: "Publication plan",
						Status: taskpkg.TaskStatusReady,
					},
				}, nil
			}
			fixture := newHandlerFixtureWithTasks(t, testutil.StubSessionManager{}, testutil.StubObserver{},
				&testutil.StubTaskManager{InspectTaskFn: inspect, InspectRunFn: inspect},
				testutil.StubWorkspaceService{}, nil, nil)
			fixture.Handlers.Profiles = sessionProfileServiceStub{}
			response := performRequest(t, fixture.Engine, http.MethodGet, tc.path+"?profile=marketing", nil)
			if response.Code != http.StatusOK {
				t.Fatalf("inspect status = %d, body = %s, want success", response.Code, response.Body)
			}
			var payload contract.TaskInspectResponse
			testutil.DecodeJSONResponse(t, response, &payload)
			owner := payload.Inspect.Task
			if owner.ProfileID != "profile-marketing" || owner.ProfileName != "marketing" ||
				owner.ProfileColor != "#E8572A" || owner.ProfileIcon != "megaphone" {
				t.Fatalf("inspected owner = %#v, want marketing identity", owner)
			}
		})
	}
}

func TestBaseHandlersTaskHappyPathEndpoints(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)

	var listedQuery taskpkg.CatalogQuery
	var createdSpec taskpkg.CreateTask
	var childSpec taskpkg.CreateTask
	var deletedTaskID string
	var updatedPatch taskpkg.Patch
	var cancelledTask taskpkg.CancelTask
	var addedDependency taskpkg.AddDependency
	var removedTaskID string
	var removedDependsOnID string
	var enqueuedRun taskpkg.EnqueueRun
	enqueuedRuns := make([]taskpkg.EnqueueRun, 0)
	var listedRunTaskID string
	var listedRunQuery taskpkg.RunQuery
	var startedRun taskpkg.StartRun
	var attachedRunID string
	var attachedSessionID string
	var completedRun taskpkg.RunResult
	var failedRun taskpkg.RunFailure
	var cancelledRun taskpkg.CancelRun
	var forceReleasedRun taskpkg.ForceReleaseRun
	var forceFailedRun taskpkg.ForceFailRun
	var retryRunRequest taskpkg.RetryRunRequest
	var bulkForceRelease taskpkg.BulkForceRunRequest
	var bulkForceFail taskpkg.BulkForceRunRequest
	var fanoutRollup store.TaskDesignationRollup

	taskView := &taskpkg.View{
		Task: taskpkg.Task{
			ID:          "task-1",
			ProfileID:   store.DefaultProfileID,
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: "ws-alpha",
			Title:       "Review task API",
			Description: "Check handler wiring",
			Status:      taskpkg.TaskStatusInProgress,
			Owner:       &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "reviewers"},
			CreatedBy:   taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
			Origin:      taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.create"},
			CreatedAt:   now,
			UpdatedAt:   now,
			Metadata:    json.RawMessage(`{"priority":"high"}`),
		},
		Dependencies: []taskpkg.Dependency{{
			TaskID:          "task-1",
			DependsOnTaskID: "task-blocker",
			Kind:            taskpkg.DependencyKindBlocks,
			CreatedAt:       now,
		}},
		Runs: []taskpkg.Run{
			{
				ID:        "run-1",
				ProfileID: store.DefaultProfileID,
				TaskID:    "task-1",
				Status:    taskpkg.TaskRunStatusRunning,
				Attempt:   1,
				SessionID: "sess-1",
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.start_run"},
				QueuedAt:  now,
				StartedAt: now,
			},
			{
				ID:        "run-2",
				ProfileID: store.DefaultProfileID,
				TaskID:    "task-1",
				Status:    taskpkg.TaskRunStatusQueued,
				Attempt:   2,
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.enqueue_run"},
				QueuedAt:  now,
			},
		},
		Events: []taskpkg.Event{{
			ID:        "evt-1",
			TaskID:    "task-1",
			EventType: "task.created",
			Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
			Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.create"},
			Timestamp: now,
		}},
	}

	getTaskCalls := 0

	tasks := &testutil.StubTaskManager{
		ListTaskCatalogFn: func(
			_ context.Context,
			query taskpkg.CatalogQuery,
			_ taskpkg.ActorContext,
		) (taskpkg.CatalogPage, error) {
			listedQuery = query
			return taskpkg.CatalogPage{Tasks: []taskpkg.Summary{{
				ID:           "task-1",
				ProfileID:    store.DefaultProfileID,
				Scope:        taskpkg.Scope(query.Scope),
				WorkspaceID:  query.WorkspaceID,
				ParentTaskID: query.ParentTaskID,
				Title:        "Review task API",
				Status:       query.Status,
				Owner:        &taskpkg.Ownership{Kind: query.OwnerKind, Ref: query.OwnerRef},
				CreatedBy:    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
				Origin:       taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.list"},
				CreatedAt:    now,
				UpdatedAt:    now,
				ActiveRun: &taskpkg.RunSummary{
					ID:     "run-1",
					TaskID: "task-1",
					Status: taskpkg.TaskRunStatusRunning,
				},
			}}, Total: 1, Limit: query.Limit}, nil
		},
		CreateTaskFn: func(_ context.Context, spec taskpkg.CreateTask, actor taskpkg.ActorContext) (*taskpkg.Task, error) {
			createdSpec = spec
			record := taskView.Task
			record.CreatedBy = actor.Actor
			record.Origin = actor.Origin
			record.Title = spec.Title
			record.Description = spec.Description
			record.Metadata = spec.Metadata
			return &record, nil
		},
		GetTaskFn: func(_ context.Context, id string, _ taskpkg.ActorContext) (*taskpkg.View, error) {
			getTaskCalls++
			if id != "task-1" {
				t.Fatalf("GetTask id = %q, want %q", id, "task-1")
			}
			return taskView, nil
		},
		ListTaskRunsFn: func(_ context.Context, taskID string, query taskpkg.RunQuery, _ taskpkg.ActorContext) ([]taskpkg.Run, error) {
			listedRunTaskID = taskID
			listedRunQuery = query
			return []taskpkg.Run{taskView.Runs[0]}, nil
		},
		UpdateTaskFn: func(_ context.Context, _ string, patch taskpkg.Patch, _ taskpkg.ActorContext) (*taskpkg.Task, error) {
			updatedPatch = patch
			record := taskView.Task
			if patch.Title != nil {
				record.Title = *patch.Title
			}
			return &record, nil
		},
		CancelTaskFn: func(_ context.Context, _ string, req taskpkg.CancelTask, _ taskpkg.ActorContext) (*taskpkg.Task, error) {
			cancelledTask = req
			record := taskView.Task
			record.Status = taskpkg.TaskStatusCanceled
			return &record, nil
		},
		CreateChildTaskFn: func(_ context.Context, parentTaskID string, spec taskpkg.CreateTask, actor taskpkg.ActorContext) (*taskpkg.Task, error) {
			if parentTaskID != "task-root" {
				t.Fatalf("CreateChildTask parentTaskID = %q, want %q", parentTaskID, "task-root")
			}
			childSpec = spec
			return &taskpkg.Task{
				ID:           "task-child",
				ProfileID:    store.DefaultProfileID,
				Scope:        spec.Scope,
				WorkspaceID:  spec.WorkspaceID,
				Title:        spec.Title,
				Description:  spec.Description,
				Status:       taskpkg.TaskStatusReady,
				CreatedBy:    actor.Actor,
				Origin:       actor.Origin,
				CreatedAt:    now,
				UpdatedAt:    now,
				ParentTaskID: parentTaskID,
			}, nil
		},
		DeleteTaskFn: func(_ context.Context, id string, _ taskpkg.ActorContext) error {
			deletedTaskID = id
			return nil
		},
		AddDependencyFn: func(_ context.Context, spec taskpkg.AddDependency, _ taskpkg.ActorContext) error {
			addedDependency = spec
			return nil
		},
		RemoveDependencyFn: func(_ context.Context, taskID string, dependsOnID string, _ taskpkg.ActorContext) error {
			removedTaskID = taskID
			removedDependsOnID = dependsOnID
			return nil
		},
		EnqueueRunFn: func(_ context.Context, spec taskpkg.EnqueueRun, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
			if spec.IdempotencyKey == "key-3" {
				enqueuedRun = spec
			}
			enqueuedRuns = append(enqueuedRuns, spec)
			runID := fmt.Sprintf("run-%d", len(enqueuedRuns)+2)
			return &taskpkg.Run{
				ID:                 runID,
				TaskID:             spec.TaskID,
				Status:             taskpkg.TaskRunStatusQueued,
				Attempt:            int32(len(enqueuedRuns) + 2),
				Origin:             actor.Origin,
				IdempotencyKey:     spec.IdempotencyKey,
				DesignationGroupID: spec.DesignationGroupID,
				Metadata:           spec.Metadata,
				QueuedAt:           now,
			}, nil
		},
		StartRunFn: func(_ context.Context, _ string, req taskpkg.StartRun, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
			startedRun = req
			return &taskpkg.Run{
				ID:        "run-1",
				TaskID:    "task-1",
				Status:    taskpkg.TaskRunStatusRunning,
				Attempt:   1,
				SessionID: "sess-1",
				Origin:    actor.Origin,
				QueuedAt:  now,
				StartedAt: now,
			}, nil
		},
		AttachRunSessionFn: func(_ context.Context, runID string, sessionID string, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
			attachedRunID = runID
			attachedSessionID = sessionID
			return &taskpkg.Run{
				ID:        runID,
				TaskID:    "task-1",
				Status:    taskpkg.TaskRunStatusStarting,
				Attempt:   1,
				SessionID: sessionID,
				Origin:    actor.Origin,
				QueuedAt:  now,
			}, nil
		},
		CompleteRunFn: func(_ context.Context, runID string, result taskpkg.RunResult, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
			if runID != "run-1" || actor.Actor.Ref != "user-1" || actor.Origin.Ref != "tasks.complete_run" {
				t.Fatalf("complete run identity = %q, actor = %#v", runID, actor)
			}
			completedRun = result
			run := &taskpkg.Run{
				ID:      "run-1",
				TaskID:  "task-1",
				Status:  taskpkg.TaskRunStatusCompleted,
				Attempt: 1,
				Origin:  actor.Origin,

				QueuedAt: now,
				EndedAt:  now,
			}
			run.SetResult(result.Value)
			return run, nil
		},
		FailRunFn: func(_ context.Context, runID string, failure taskpkg.RunFailure, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
			if runID != "run-2" || actor.Actor.Ref != "user-1" || actor.Origin.Ref != "tasks.fail_run" {
				t.Fatalf("fail run identity = %q, actor = %#v", runID, actor)
			}
			failedRun = failure
			return &taskpkg.Run{
				ID:      "run-2",
				TaskID:  "task-1",
				Status:  taskpkg.TaskRunStatusFailed,
				Attempt: 2,
				Origin:  actor.Origin,

				QueuedAt: now,
				EndedAt:  now,
				Error:    failure.Error,
				Metadata: failure.Metadata,
			}, nil
		},
		CancelRunFn: func(_ context.Context, runID string, req taskpkg.CancelRun, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
			if runID != "run-2" || actor.Actor.Ref != "user-1" || actor.Origin.Ref != "tasks.cancel_run" {
				t.Fatalf("cancel run identity = %q, actor = %#v", runID, actor)
			}
			cancelledRun = req
			return &taskpkg.Run{
				ID:        "run-2",
				TaskID:    "task-1",
				Status:    taskpkg.TaskRunStatusCanceled,
				Attempt:   2,
				ClaimedBy: &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-history"},
				SessionID: "sess-history",
				Origin:    actor.Origin,

				QueuedAt: now,
				EndedAt:  now,
				Metadata: req.Metadata,
			}, nil
		},
		ForceReleaseRunFn: func(
			_ context.Context,
			runID string,
			req taskpkg.ForceReleaseRun,
			actor taskpkg.ActorContext,
		) (*taskpkg.Run, error) {
			forceReleasedRun = req
			return &taskpkg.Run{
				ID:       runID,
				TaskID:   "task-1",
				Status:   taskpkg.TaskRunStatusQueued,
				Attempt:  1,
				Origin:   actor.Origin,
				QueuedAt: now,
			}, nil
		},
		ForceFailRunFn: func(
			_ context.Context,
			runID string,
			req taskpkg.ForceFailRun,
			actor taskpkg.ActorContext,
		) (*taskpkg.Run, error) {
			forceFailedRun = req
			return &taskpkg.Run{
				ID:          runID,
				TaskID:      "task-1",
				Status:      taskpkg.TaskRunStatusFailed,
				Attempt:     2,
				Origin:      actor.Origin,
				QueuedAt:    now,
				EndedAt:     now,
				Error:       req.Reason,
				FailureKind: taskpkg.FailureKindOperatorForced,
			}, nil
		},
		RetryRunFn: func(
			_ context.Context,
			runID string,
			req taskpkg.RetryRunRequest,
			actor taskpkg.ActorContext,
		) (*taskpkg.RetryRunResult, error) {
			retryRunRequest = req
			return &taskpkg.RetryRunResult{
				PreviousRun: taskpkg.Run{
					ID:       runID,
					TaskID:   "task-1",
					Status:   taskpkg.TaskRunStatusFailed,
					Attempt:  2,
					Origin:   actor.Origin,
					QueuedAt: now,
					EndedAt:  now,
				},
				Run: taskpkg.Run{
					ID:            "run-retry",
					TaskID:        "task-1",
					Status:        taskpkg.TaskRunStatusQueued,
					Attempt:       3,
					PreviousRunID: runID,
					Origin:        actor.Origin,
					QueuedAt:      now,
				},
			}, nil
		},
		BulkForceReleaseRunsFn: func(
			_ context.Context,
			req taskpkg.BulkForceRunRequest,
			_ taskpkg.ActorContext,
		) (taskpkg.BulkForceRunResult, error) {
			bulkForceRelease = req
			return taskpkg.BulkForceRunResult{Items: []taskpkg.BulkForceRunItem{
				{
					RunID: "run-1",
					OK:    true,
					Run:   &taskpkg.Run{ID: "run-1", TaskID: "task-1", Status: taskpkg.TaskRunStatusQueued},
				},
				{
					RunID: "run-2",
					OK:    true,
					Run:   &taskpkg.Run{ID: "run-2", TaskID: "task-1", Status: taskpkg.TaskRunStatusQueued},
				},
			}}, nil
		},
		BulkForceFailRunsFn: func(
			_ context.Context,
			req taskpkg.BulkForceRunRequest,
			_ taskpkg.ActorContext,
		) (taskpkg.BulkForceRunResult, error) {
			bulkForceFail = req
			return taskpkg.BulkForceRunResult{Items: []taskpkg.BulkForceRunItem{
				{
					RunID: "run-2",
					OK:    false,
					Err:   errors.New("database internal detail exploded"),
				},
			}}, nil
		},
	}

	workspaces := testutil.StubWorkspaceService{
		GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
			if ref != "alpha" {
				t.Fatalf("workspace ref = %q, want %q", ref, "alpha")
			}
			return workspacepkg.Workspace{ID: "ws-alpha", Name: "alpha"}, nil
		},
	}

	fixture := newHandlerFixtureWithTasks(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		tasks,
		workspaces,
		nil,
		nil,
	)

	fixture.Handlers.TaskActorContextResolver = func(_ *gin.Context, action string) (taskpkg.ActorContext, error) {
		return taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindHTTP, "tasks."+action)
	}
	fixture.Handlers.TaskDesignations = testutil.StubTaskDesignationStore{
		PutTaskDesignationRollupFn: func(_ context.Context, rollup store.TaskDesignationRollup) error {
			fanoutRollup = rollup
			return nil
		},
		ListTaskDesignationRollupsFn: func(_ context.Context, query store.TaskDesignationRollupQuery) ([]store.TaskDesignationRollup, error) {
			if query.TaskID != "task-1" {
				t.Fatalf("designation rollup query task_id = %q, want task-1", query.TaskID)
			}
			if query.Limit != 20 {
				t.Fatalf("designation rollup query limit = %d, want 20", query.Limit)
			}
			if fanoutRollup.DesignationGroupID == "" {
				return nil, nil
			}
			return []store.TaskDesignationRollup{fanoutRollup}, nil
		},
	}

	resp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/tasks?scope=workspace&workspace=alpha&status=ready&owner_kind=pool&owner_ref=reviewers&parent_task_id=task-root&limit=2",
		nil,
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks",
		[]byte(
			`{"scope":"workspace","workspace":"alpha","title":"Review task API","description":"Check handler wiring","owner":{"kind":"pool","ref":"reviewers"},"metadata":{"priority":"high"}}`,
		),
	)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
	}

	resp = performRequest(t, fixture.Engine, http.MethodGet, "/tasks/task-1", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("get status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	resp = performRequest(t, fixture.Engine, http.MethodDelete, "/tasks/task-1", nil)
	if resp.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d; body=%s", resp.Code, http.StatusNoContent, resp.Body.String())
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPatch,
		"/tasks/task-1",
		[]byte(`{"title":"Renamed task","metadata":{"priority":"medium"}}`),
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("update status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks/task-1/cancel",
		[]byte(`{"reason":"no longer needed","metadata":{"source":"test"}}`),
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("cancel task status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks/task-root/children",
		[]byte(`{"scope":"workspace","workspace":"alpha","title":"Child task","description":"Follow-up work"}`),
	)
	if resp.Code != http.StatusCreated {
		t.Fatalf("create child status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks/task-1/dependencies",
		[]byte(`{"depends_on_task_id":"task-blocker"}`),
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("add dependency status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	resp = performRequest(t, fixture.Engine, http.MethodDelete, "/tasks/task-1/dependencies/task-blocker", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("remove dependency status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/tasks/task-1/runs?status=running&session_id=sess-1&limit=1",
		nil,
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("list runs status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	var runsResponse contract.TaskRunsResponse
	testutil.DecodeJSONResponse(t, resp, &runsResponse)
	if len(runsResponse.Runs) != 1 ||
		runsResponse.Runs[0].ProfileID != store.DefaultProfileID ||
		runsResponse.Runs[0].ProfileName != "default" {
		t.Fatalf("list runs owner payload = %#v, want default profile identity", runsResponse.Runs)
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks/task-1/runs",
		[]byte(
			`{"idempotency_key":"key-3","metadata":{"schema":"compozy.harness.detached.v1","kind":"harness_detached_run"}}`,
		),
	)
	if resp.Code != http.StatusCreated {
		t.Fatalf("enqueue status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks/task-1/runs/fan-out",
		[]byte(
			`{"idempotency_key":"fanout-key","worktree_per_run":true,"designations":[{"brief":"Review API handlers","metadata":{"lane":"api"}},{"brief":"Review web wiring","metadata":{"lane":"web"}}]}`,
		),
	)
	if resp.Code != http.StatusCreated {
		t.Fatalf("fan-out status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
	}
	var fanoutResp contract.FanOutTaskRunsResponse
	testutil.DecodeJSONResponse(t, resp, &fanoutResp)
	if fanoutResp.DesignationGroupID == "" || len(fanoutResp.Runs) != 2 {
		t.Fatalf("fan-out response = %#v", fanoutResp)
	}
	if fanoutResp.Runs[0].DesignationGroupID != fanoutResp.DesignationGroupID ||
		fanoutResp.Runs[1].DesignationGroupID != fanoutResp.DesignationGroupID {
		t.Fatalf("fan-out runs missing group id: %#v", fanoutResp.Runs)
	}
	for index, spec := range enqueuedRuns[len(enqueuedRuns)-2:] {
		if !spec.WorktreePerRun {
			t.Fatalf("fan-out enqueue %d WorktreePerRun = false, want true", index)
		}
	}

	enqueuedBeforeInvalidFanout := len(enqueuedRuns)
	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks/task-1/runs/fan-out",
		[]byte(
			`{"idempotency_key":"invalid-fanout","designations":[{"brief":"Valid lane"},{"brief":"   "}]}`,
		),
	)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("invalid fan-out status = %d, want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
	}
	if got := len(enqueuedRuns); got != enqueuedBeforeInvalidFanout {
		t.Fatalf("enqueued runs after invalid fan-out = %d, want %d", got, enqueuedBeforeInvalidFanout)
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks/task-1/runs/fan-out",
		[]byte(
			`{"designations":[{"brief":"Missing idempotency"}]}`,
		),
	)
	if resp.Code != http.StatusBadRequest {
		t.Fatalf(
			"missing idempotency fan-out status = %d, want %d; body=%s",
			resp.Code,
			http.StatusBadRequest,
			resp.Body.String(),
		)
	}
	if got := len(enqueuedRuns); got != enqueuedBeforeInvalidFanout {
		t.Fatalf("enqueued runs after missing idempotency = %d, want %d", got, enqueuedBeforeInvalidFanout)
	}

	resp = performRequest(t, fixture.Engine, http.MethodGet, "/tasks/task-1", nil)
	if resp.Code != http.StatusOK {
		t.Fatalf("get after fan-out status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	var detailResp contract.TaskDetailResponse
	testutil.DecodeJSONResponse(t, resp, &detailResp)
	if len(detailResp.Task.DesignationRollups) != 1 {
		t.Fatalf("designation rollups = %#v, want one rollup", detailResp.Task.DesignationRollups)
	}
	detailRollup := detailResp.Task.DesignationRollups[0]
	if detailRollup.DesignationGroupID != fanoutResp.DesignationGroupID ||
		detailRollup.TaskID != "task-1" ||
		string(detailRollup.Summary) != string(fanoutRollup.SummaryJSON) {
		t.Fatalf("designation rollup payload = %#v, want stored rollup %#v", detailRollup, fanoutRollup)
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/task-runs/run-1/start",
		[]byte(`{"idempotency_key":"start-1"}`),
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("start status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/task-runs/run-1/attach-session",
		[]byte(`{"session_id":"sess-1"}`),
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("attach status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/task-runs/run-1/complete",
		[]byte(`{"result":{"ok":true}}`),
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("complete status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	var completedResp contract.TaskRunResponse
	testutil.DecodeJSONResponse(t, resp, &completedResp)
	if completedResp.Run.Status != taskpkg.TaskRunStatusCompleted ||
		completedResp.Run.Origin.Ref != "tasks.complete_run" || string(completedResp.Run.Result) != `{"ok":true}` ||
		completedResp.Run.Error != "" || len(completedResp.Run.Metadata) != 0 {
		t.Fatalf("completed run response = %#v", completedResp.Run)
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/task-runs/run-2/fail",
		[]byte(`{"error":"boom","metadata":{"step":"claim"}}`),
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("fail status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	var failedResp contract.TaskRunResponse
	testutil.DecodeJSONResponse(t, resp, &failedResp)
	if failedResp.Run.Status != taskpkg.TaskRunStatusFailed || failedResp.Run.Origin.Ref != "tasks.fail_run" ||
		failedResp.Run.Error != "boom" || string(failedResp.Run.Metadata) != `{"step":"claim"}` ||
		len(failedResp.Run.Result) != 0 {
		t.Fatalf("failed run response = %#v", failedResp.Run)
	}

	resp = performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/task-runs/run-2/cancel",
		[]byte(`{"reason":"operator canceled","metadata":{"step":"cancel"}}`),
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("cancel run status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	var cancelledResp contract.TaskRunResponse
	testutil.DecodeJSONResponse(t, resp, &cancelledResp)
	if cancelledResp.Run.Status != taskpkg.TaskRunStatusCanceled ||
		cancelledResp.Run.Origin.Ref != "tasks.cancel_run" || string(cancelledResp.Run.Metadata) != `{"step":"cancel"}` ||
		cancelledResp.Run.Error != "" || len(cancelledResp.Run.Result) != 0 {
		t.Fatalf("canceled run response = %#v", cancelledResp.Run)
	}
	if cancelledResp.Run.ClaimedBy == nil || cancelledResp.Run.ClaimedBy.Ref != "sess-history" ||
		cancelledResp.Run.SessionID != "sess-history" {
		t.Fatalf("canceled run ownership = %#v", cancelledResp.Run)
	}

	// not parallel: these cases share one fixture and captured request variables for the end-of-flow assertions.
	t.Run("Should force release run", func(t *testing.T) {
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/runs/run-1/release",
			[]byte("{\"reason\":\"handoff\",\"metadata\":{\"source\":\"operator\"}}"),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("force release status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		if forceReleasedRun.Reason != "handoff" || string(forceReleasedRun.Metadata) != "{\"source\":\"operator\"}" {
			t.Fatalf("force release run = %#v", forceReleasedRun)
		}
	})

	t.Run("Should force fail run", func(t *testing.T) {
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/runs/run-2/fail",
			[]byte("{\"reason\":\"operator recovery\",\"metadata\":{\"incident\":\"INC-42\"}}"),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("force fail status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		if forceFailedRun.Reason != "operator recovery" ||
			string(forceFailedRun.Metadata) != "{\"incident\":\"INC-42\"}" {
			t.Fatalf("force fail run = %#v", forceFailedRun)
		}
	})

	t.Run("Should retry run", func(t *testing.T) {
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/runs/run-2/retry",
			[]byte("{\"metadata\":{\"source\":\"operator\"}}"),
		)
		if resp.Code != http.StatusCreated {
			t.Fatalf("retry status = %d, want %d; body=%s", resp.Code, http.StatusCreated, resp.Body.String())
		}
		if string(retryRunRequest.Metadata) != "{\"source\":\"operator\"}" {
			t.Fatalf("retry run request = %#v", retryRunRequest)
		}
	})

	t.Run("Should bulk release runs", func(t *testing.T) {
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/runs/bulk/release",
			[]byte("{\"run_ids\":[\"run-1\",\"run-2\"],\"reason\":\"handoff\"}"),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("bulk release status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		if strings.Join(bulkForceRelease.RunIDs, ",") != "run-1,run-2" || bulkForceRelease.Reason != "handoff" {
			t.Fatalf("bulk force release = %#v", bulkForceRelease)
		}
	})

	t.Run("Should bulk fail runs with unmasked handler errors", func(t *testing.T) {
		fixture.Handlers.MaskInternalErrors = false
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/runs/bulk/fail",
			[]byte("{\"run_ids\":[\"run-2\"],\"reason\":\"operator recovery\"}"),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("bulk fail status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		var bulkFailPayload contract.BulkForceTaskRunResponse
		if err := json.Unmarshal(resp.Body.Bytes(), &bulkFailPayload); err != nil {
			t.Fatalf("decode bulk fail response: %v", err)
		}
		if len(bulkFailPayload.Results) != 1 || bulkFailPayload.Results[0].Error == nil {
			t.Fatalf("bulk fail payload = %#v, want item error", bulkFailPayload.Results)
		}
		if got, want := bulkFailPayload.Results[0].Error.Error,
			"database internal detail exploded"; got != want {
			t.Fatalf("bulk fail item error = %q, want %q", got, want)
		}
		if strings.Join(bulkForceFail.RunIDs, ",") != "run-2" || bulkForceFail.Reason != "operator recovery" {
			t.Fatalf("bulk force fail = %#v", bulkForceFail)
		}
	})

	t.Run("Should bulk fail runs with masked handler errors", func(t *testing.T) {
		fixture.Handlers.MaskInternalErrors = true
		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/runs/bulk/fail",
			[]byte("{\"run_ids\":[\"run-2\"],\"reason\":\"operator recovery\"}"),
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("bulk fail status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		var bulkFailPayload contract.BulkForceTaskRunResponse
		if err := json.Unmarshal(resp.Body.Bytes(), &bulkFailPayload); err != nil {
			t.Fatalf("decode bulk fail response: %v", err)
		}
		if len(bulkFailPayload.Results) != 1 || bulkFailPayload.Results[0].Error == nil {
			t.Fatalf("bulk fail payload = %#v, want item error", bulkFailPayload.Results)
		}
		if got, want := bulkFailPayload.Results[0].Error.Error,
			http.StatusText(http.StatusInternalServerError); got != want {
			t.Fatalf("bulk fail item error = %q, want %q", got, want)
		}
		if strings.Contains(bulkFailPayload.Results[0].Error.Error, "internal detail") {
			t.Fatalf("bulk fail item error leaked internal detail: %#v", bulkFailPayload.Results[0].Error)
		}
	})

	if listedQuery.WorkspaceID != "ws-alpha" || listedQuery.Scope != taskpkg.CatalogScopeWorkspace {
		t.Fatalf("listed query = %#v", listedQuery)
	}
	if listedQuery.Status != taskpkg.TaskStatusReady || listedQuery.OwnerKind != taskpkg.OwnerKindPool ||
		listedQuery.OwnerRef != "reviewers" || listedQuery.Limit != 2 {
		t.Fatalf("listed query = %#v", listedQuery)
	}

	if listedRunTaskID != "task-1" {
		t.Fatalf("listed run task id = %q, want %q", listedRunTaskID, "task-1")
	}
	if listedRunQuery.Status != taskpkg.TaskRunStatusRunning || listedRunQuery.SessionID != "sess-1" ||
		listedRunQuery.Limit != 1 {
		t.Fatalf("listed run query = %#v", listedRunQuery)
	}
	if getTaskCalls != 4 {
		t.Fatalf("GetTask() calls = %d, want 4 detail reads without extra run-list fetch", getTaskCalls)
	}
	if createdSpec.WorkspaceID != "ws-alpha" || createdSpec.Owner == nil ||
		createdSpec.Owner.Ref != "reviewers" {
		t.Fatalf("created spec = %#v", createdSpec)
	}

	if childSpec.WorkspaceID != "ws-alpha" || childSpec.Title != "Child task" {
		t.Fatalf("child spec = %#v", childSpec)
	}
	if deletedTaskID != "task-1" {
		t.Fatalf("deleted task id = %q, want %q", deletedTaskID, "task-1")
	}
	if updatedPatch.Title == nil || *updatedPatch.Title != "Renamed task" {
		t.Fatalf("updated patch = %#v", updatedPatch)
	}
	if cancelledTask.Reason != "no longer needed" {
		t.Fatalf("canceled task = %#v", cancelledTask)
	}
	if addedDependency.Kind != taskpkg.DependencyKindBlocks || addedDependency.DependsOnTaskID != "task-blocker" {
		t.Fatalf("added dependency = %#v", addedDependency)
	}
	if removedTaskID != "task-1" || removedDependsOnID != "task-blocker" {
		t.Fatalf("removed dependency = task=%q dependsOn=%q", removedTaskID, removedDependsOnID)
	}
	if enqueuedRun.IdempotencyKey != "key-3" {
		t.Fatalf("enqueued run = %#v", enqueuedRun)
	}

	if got, want := string(
		enqueuedRun.Metadata,
	), `{"schema":"compozy.harness.detached.v1","kind":"harness_detached_run"}`; got != want {
		t.Fatalf("enqueued run metadata = %q, want %q", got, want)
	}
	if len(enqueuedRuns) < 3 {
		t.Fatalf("len(enqueuedRuns) = %d, want at least normal enqueue plus fan-out runs", len(enqueuedRuns))
	}
	fanoutEnqueues := enqueuedRuns[1:3]
	if fanoutRollup.DesignationGroupID == "" || fanoutRollup.TaskID != "task-1" {
		t.Fatalf("fanout rollup = %#v", fanoutRollup)
	}
	for idx, run := range fanoutEnqueues {
		if run.TaskID != "task-1" ||
			run.DesignationGroupID != fanoutRollup.DesignationGroupID {
			t.Fatalf("fanout enqueue %d = %#v", idx, run)
		}

		if !strings.Contains(string(run.Metadata), `"designation"`) {
			t.Fatalf("fanout enqueue %d metadata = %s, want designation payload", idx, run.Metadata)
		}
	}
	if startedRun.IdempotencyKey != "start-1" {
		t.Fatalf("started run = %#v", startedRun)
	}
	if attachedRunID != "run-1" || attachedSessionID != "sess-1" {
		t.Fatalf("attached run = %q session = %q", attachedRunID, attachedSessionID)
	}
	if string(completedRun.Value) != `{"ok":true}` {
		t.Fatalf("completed run = %#v", completedRun)
	}
	if failedRun.Error != "boom" || string(failedRun.Metadata) != `{"step":"claim"}` {
		t.Fatalf("failed run = %#v", failedRun)
	}
	if cancelledRun.Reason != "operator canceled" || string(cancelledRun.Metadata) != `{"step":"cancel"}` {
		t.Fatalf("canceled run = %#v", cancelledRun)
	}
}

func TestBaseHandlersTaskActorResolverErrors(t *testing.T) {
	t.Parallel()

	fixture := newHandlerFixtureWithTasks(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		&testutil.StubTaskManager{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)
	fixture.Handlers.TaskActorContextResolver = func(*gin.Context, string) (taskpkg.ActorContext, error) {
		return taskpkg.ActorContext{}, errors.New("resolver failed")
	}

	requests := []struct {
		method string
		path   string
		body   []byte
	}{
		{method: http.MethodGet, path: "/tasks"},
		{method: http.MethodPost, path: "/tasks", body: []byte(`{"scope":"global","title":"Review task API"}`)},
		{method: http.MethodGet, path: "/tasks/task-1"},
		{method: http.MethodDelete, path: "/tasks/task-1"},
		{method: http.MethodPatch, path: "/tasks/task-1", body: []byte(`{"title":"Renamed task"}`)},
		{method: http.MethodPost, path: "/tasks/task-1/cancel", body: []byte(`{}`)},
		{
			method: http.MethodPost,
			path:   "/tasks/task-root/children",
			body:   []byte(`{"scope":"global","title":"Child task"}`),
		},
		{
			method: http.MethodPost,
			path:   "/tasks/task-1/dependencies",
			body:   []byte(`{"depends_on_task_id":"task-blocker"}`),
		},
		{method: http.MethodDelete, path: "/tasks/task-1/dependencies/task-blocker"},
		{method: http.MethodGet, path: "/tasks/task-1/runs"},
		{method: http.MethodPost, path: "/tasks/task-1/runs", body: []byte(`{}`)},
		{method: http.MethodPost, path: "/task-runs/run-1/start", body: []byte(`{}`)},
		{method: http.MethodPost, path: "/task-runs/run-1/attach-session", body: []byte(`{"session_id":"sess-1"}`)},
		{method: http.MethodPost, path: "/task-runs/run-1/complete", body: []byte(`{}`)},
		{method: http.MethodPost, path: "/task-runs/run-1/fail", body: []byte(`{"error":"boom"}`)},
		{method: http.MethodPost, path: "/task-runs/run-1/cancel", body: []byte(`{}`)},
	}

	for _, request := range requests {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			resp := performRequest(t, fixture.Engine, request.method, request.path, request.body)
			if resp.Code != http.StatusInternalServerError {
				t.Fatalf(
					"%s %s status = %d, want %d; body=%s",
					request.method,
					request.path,
					resp.Code,
					http.StatusInternalServerError,
					resp.Body.String(),
				)
			}
		})
	}
}

func TestBaseHandlersTaskServiceUnavailable(t *testing.T) {
	t.Parallel()

	fixture := newHandlerFixtureWithTasks(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		&testutil.StubTaskManager{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)
	fixture.Handlers.Tasks = nil

	requests := []struct {
		method string
		path   string
		body   []byte
	}{
		{method: http.MethodGet, path: "/tasks"},
		{method: http.MethodPost, path: "/tasks", body: []byte(`{"scope":"global","title":"Review task API"}`)},
		{method: http.MethodGet, path: "/tasks/task-1"},
		{method: http.MethodDelete, path: "/tasks/task-1"},
		{method: http.MethodPatch, path: "/tasks/task-1", body: []byte(`{"title":"Renamed task"}`)},
		{method: http.MethodPost, path: "/tasks/task-1/cancel", body: []byte(`{}`)},
		{
			method: http.MethodPost,
			path:   "/tasks/task-root/children",
			body:   []byte(`{"scope":"global","title":"Child task"}`),
		},
		{
			method: http.MethodPost,
			path:   "/tasks/task-1/dependencies",
			body:   []byte(`{"depends_on_task_id":"task-blocker"}`),
		},
		{method: http.MethodDelete, path: "/tasks/task-1/dependencies/task-blocker"},
		{method: http.MethodGet, path: "/tasks/task-1/runs"},
		{method: http.MethodPost, path: "/tasks/task-1/runs", body: []byte(`{}`)},
		{method: http.MethodPost, path: "/task-runs/run-1/start", body: []byte(`{}`)},
		{method: http.MethodPost, path: "/task-runs/run-1/attach-session", body: []byte(`{"session_id":"sess-1"}`)},
		{method: http.MethodPost, path: "/task-runs/run-1/complete", body: []byte(`{}`)},
		{method: http.MethodPost, path: "/task-runs/run-1/fail", body: []byte(`{"error":"boom"}`)},
		{method: http.MethodPost, path: "/task-runs/run-1/cancel", body: []byte(`{}`)},
	}

	for _, request := range requests {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			resp := performRequest(t, fixture.Engine, request.method, request.path, request.body)
			if resp.Code != http.StatusServiceUnavailable {
				t.Fatalf(
					"%s %s status = %d, want %d; body=%s",
					request.method,
					request.path,
					resp.Code,
					http.StatusServiceUnavailable,
					resp.Body.String(),
				)
			}
		})
	}
}

func TestBaseHandlersTaskManagerErrors(t *testing.T) {
	t.Parallel()

	fixture := newHandlerFixtureWithTasks(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		&testutil.StubTaskManager{
			ListTasksFn: func(context.Context, taskpkg.Query, taskpkg.ActorContext) ([]taskpkg.Summary, error) {
				return nil, taskpkg.ErrPermissionDenied
			},
			CreateTaskFn: func(context.Context, taskpkg.CreateTask, taskpkg.ActorContext) (*taskpkg.Task, error) {
				return nil, taskpkg.ErrPermissionDenied
			},
			GetTaskFn: func(context.Context, string, taskpkg.ActorContext) (*taskpkg.View, error) {
				return nil, taskpkg.ErrTaskNotFound
			},
			DeleteTaskFn: func(context.Context, string, taskpkg.ActorContext) error {
				return taskpkg.ErrValidation
			},
			ListTaskRunsFn: func(context.Context, string, taskpkg.RunQuery, taskpkg.ActorContext) ([]taskpkg.Run, error) {
				return nil, taskpkg.ErrTaskNotFound
			},
			UpdateTaskFn: func(context.Context, string, taskpkg.Patch, taskpkg.ActorContext) (*taskpkg.Task, error) {
				return nil, taskpkg.ErrPermissionDenied
			},
			CancelTaskFn: func(context.Context, string, taskpkg.CancelTask, taskpkg.ActorContext) (*taskpkg.Task, error) {
				return nil, taskpkg.ErrInvalidStatusTransition
			},
			CreateChildTaskFn: func(context.Context, string, taskpkg.CreateTask, taskpkg.ActorContext) (*taskpkg.Task, error) {
				return nil, taskpkg.ErrPermissionDenied
			},
			AddDependencyFn: func(context.Context, taskpkg.AddDependency, taskpkg.ActorContext) error {
				return taskpkg.ErrCycleDetected
			},
			RemoveDependencyFn: func(context.Context, string, string, taskpkg.ActorContext) error {
				return taskpkg.ErrTaskDependencyNotFound
			},
			EnqueueRunFn: func(context.Context, taskpkg.EnqueueRun, taskpkg.ActorContext) (*taskpkg.Run, error) {
				return nil, taskpkg.ErrInvalidStatusTransition
			},
			StartRunFn: func(context.Context, string, taskpkg.StartRun, taskpkg.ActorContext) (*taskpkg.Run, error) {
				return nil, taskpkg.ErrInvalidStatusTransition
			},
			AttachRunSessionFn: func(context.Context, string, string, taskpkg.ActorContext) (*taskpkg.Run, error) {
				return nil, taskpkg.ErrSessionAlreadyBound
			},
			CompleteRunFn: func(context.Context, string, taskpkg.RunResult, taskpkg.ActorContext) (*taskpkg.Run, error) {
				return nil, taskpkg.ErrTaskRunNotFound
			},
			FailRunFn: func(context.Context, string, taskpkg.RunFailure, taskpkg.ActorContext) (*taskpkg.Run, error) {
				return nil, taskpkg.ErrTaskRunNotFound
			},
			CancelRunFn: func(context.Context, string, taskpkg.CancelRun, taskpkg.ActorContext) (*taskpkg.Run, error) {
				return nil, taskpkg.ErrTaskRunNotFound
			},
		},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)

	requests := []struct {
		method string
		path   string
		body   []byte
		want   int
	}{
		{method: http.MethodGet, path: "/tasks", want: http.StatusForbidden},
		{
			method: http.MethodPost,
			path:   "/tasks",
			body:   []byte(`{"scope":"global","title":"Review task API"}`),
			want:   http.StatusForbidden,
		},
		{method: http.MethodGet, path: "/tasks/task-1", want: http.StatusNotFound},
		{method: http.MethodDelete, path: "/tasks/task-1", want: http.StatusBadRequest},
		{
			method: http.MethodPatch,
			path:   "/tasks/task-1",
			body:   []byte(`{"title":"Renamed task"}`),
			want:   http.StatusForbidden,
		},
		{method: http.MethodPost, path: "/tasks/task-1/cancel", body: []byte(`{}`), want: http.StatusConflict},
		{
			method: http.MethodPost,
			path:   "/tasks/task-root/children",
			body:   []byte(`{"scope":"global","title":"Child task"}`),
			want:   http.StatusForbidden,
		},
		{
			method: http.MethodPost,
			path:   "/tasks/task-1/dependencies",
			body:   []byte(`{"depends_on_task_id":"task-blocker"}`),
			want:   http.StatusConflict,
		},
		{method: http.MethodDelete, path: "/tasks/task-1/dependencies/task-blocker", want: http.StatusNotFound},
		{method: http.MethodGet, path: "/tasks/task-1/runs", want: http.StatusNotFound},
		{method: http.MethodPost, path: "/tasks/task-1/runs", body: []byte(`{}`), want: http.StatusConflict},
		{method: http.MethodPost, path: "/task-runs/run-1/start", body: []byte(`{}`), want: http.StatusConflict},
		{
			method: http.MethodPost,
			path:   "/task-runs/run-1/attach-session",
			body:   []byte(`{"session_id":"sess-1"}`),
			want:   http.StatusConflict,
		},
		{method: http.MethodPost, path: "/task-runs/run-1/complete", body: []byte(`{}`), want: http.StatusNotFound},
		{
			method: http.MethodPost,
			path:   "/task-runs/run-1/fail",
			body:   []byte(`{"error":"boom"}`),
			want:   http.StatusNotFound,
		},
		{method: http.MethodPost, path: "/task-runs/run-1/cancel", body: []byte(`{}`), want: http.StatusNotFound},
	}

	for _, request := range requests {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			resp := performRequest(t, fixture.Engine, request.method, request.path, request.body)
			if resp.Code != request.want {
				t.Fatalf(
					"%s %s status = %d, want %d; body=%s",
					request.method,
					request.path,
					resp.Code,
					request.want,
					resp.Body.String(),
				)
			}
		})
	}

	for _, tc := range []struct {
		name   string
		path   string
		body   string
		action string
	}{
		{
			name:   "Should reject human completion with a redacted token-fence conflict",
			path:   "/task-runs/run-2/complete",
			body:   `{"result":{"ok":true}}`,
			action: "tasks.complete_run",
		},
		{
			name:   "Should reject human failure with a redacted token-fence conflict",
			path:   "/task-runs/run-2/fail",
			body:   `{"error":"boom"}`,
			action: "tasks.fail_run",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			const rawToken = "compozy_claim_terminal-secret-123"
			var capturedRunID string
			var capturedActor taskpkg.ActorContext
			conflict := func(runID string, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
				capturedRunID, capturedActor = runID, actor
				return nil, fmt.Errorf(
					"%w: task run %q requires token-fenced terminal write with %s",
					taskpkg.ErrInvalidClaimToken,
					runID,
					rawToken,
				)
			}
			fixture := newHandlerFixtureWithTasks(
				t, testutil.StubSessionManager{}, testutil.StubObserver{},
				&testutil.StubTaskManager{
					CompleteRunFn: func(_ context.Context, runID string, _ taskpkg.RunResult, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
						return conflict(runID, actor)
					},
					FailRunFn: func(_ context.Context, runID string, _ taskpkg.RunFailure, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
						return conflict(runID, actor)
					},
				},
				testutil.StubWorkspaceService{}, nil, nil,
			)
			fixture.Handlers.TaskActorContextResolver = func(_ *gin.Context, action string) (taskpkg.ActorContext, error) {
				return taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindHTTP, "tasks."+action)
			}

			resp := performRequest(t, fixture.Engine, http.MethodPost, tc.path, []byte(tc.body))
			if resp.Code != http.StatusConflict {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, http.StatusConflict, resp.Body.String())
			}
			var payload contract.ErrorPayload
			testutil.DecodeJSONResponse(t, resp, &payload)
			if strings.Contains(payload.Error, rawToken) ||
				!strings.Contains(payload.Error, "requires token-fenced terminal write with compozy_claim_[REDACTED]") {
				t.Fatalf("token-fence error = %q, want a redacted claim token", payload.Error)
			}
			if capturedRunID != "run-2" || capturedActor.Actor.Ref != "user-1" ||
				capturedActor.Origin.Ref != tc.action {
				t.Fatalf("terminal write identity = %q, actor = %#v", capturedRunID, capturedActor)
			}
		})
	}
}

func TestBaseHandlersTaskDecodeErrors(t *testing.T) {
	t.Parallel()

	fixture := newHandlerFixtureWithTasks(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		&testutil.StubTaskManager{},
		testutil.StubWorkspaceService{},
		nil,
		nil,
	)

	requests := []struct {
		method string
		path   string
		body   []byte
	}{
		{method: http.MethodPost, path: "/tasks", body: []byte(`{"scope":`)},
		{method: http.MethodPatch, path: "/tasks/task-1", body: []byte(`{"title":`)},
		{method: http.MethodPost, path: "/tasks/task-1/cancel", body: []byte(`{"reason":`)},
		{method: http.MethodPost, path: "/tasks/task-root/children", body: []byte(`{"scope":`)},
		{method: http.MethodPost, path: "/tasks/task-1/dependencies", body: []byte(`{"depends_on_task_id":`)},
		{method: http.MethodPost, path: "/tasks/task-1/runs", body: []byte(`{"idempotency_key":`)},
		{method: http.MethodPost, path: "/task-runs/run-1/start", body: []byte(`{"idempotency_key":`)},
		{method: http.MethodPost, path: "/task-runs/run-1/attach-session", body: []byte(`{"session_id":`)},
		{method: http.MethodPost, path: "/task-runs/run-1/complete", body: []byte(`{"result":`)},
		{method: http.MethodPost, path: "/task-runs/run-1/fail", body: []byte(`{"error":`)},
		{method: http.MethodPost, path: "/task-runs/run-1/cancel", body: []byte(`{"reason":`)},
	}

	for _, request := range requests {
		t.Run(request.method+" "+request.path, func(t *testing.T) {
			resp := performRequest(t, fixture.Engine, request.method, request.path, request.body)
			if resp.Code != http.StatusBadRequest {
				t.Fatalf(
					"%s %s status = %d, want %d; body=%s",
					request.method,
					request.path,
					resp.Code,
					http.StatusBadRequest,
					resp.Body.String(),
				)
			}
		})
	}
}
