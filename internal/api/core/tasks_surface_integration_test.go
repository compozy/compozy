//go:build integration

package core_test

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/observe"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestExpandedTaskReadHandlersDelegateIntegration(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 17, 14, 0, 0, 0, time.UTC)
	lastActivity := now.Add(-time.Minute)
	var listQuery taskpkg.CatalogQuery
	var listActor taskpkg.ActorContext
	var detailActor taskpkg.ActorContext
	var runDetailActor taskpkg.ActorContext
	var timelineQuery taskpkg.TimelineQuery
	var timelineActor taskpkg.ActorContext
	var treeActor taskpkg.ActorContext
	var dashboardQuery observe.TaskDashboardQuery
	var inboxQuery observe.TaskInboxQuery
	var inboxActor taskpkg.ActorIdentity

	tasks := &testutil.StubTaskManager{
		ListTaskCatalogFn: func(
			_ context.Context,
			query taskpkg.CatalogQuery,
			actor taskpkg.ActorContext,
		) (taskpkg.CatalogPage, error) {
			listQuery = query
			listActor = actor
			return taskpkg.CatalogPage{
				Tasks: []taskpkg.Summary{{
					ID:             "task-1",
					Identifier:     "TASK-1",
					Title:          "Review handlers",
					Scope:          taskpkg.ScopeWorkspace,
					WorkspaceID:    "ws-alpha",
					Priority:       taskpkg.PriorityHigh,
					Status:         taskpkg.TaskStatusReady,
					ApprovalState:  taskpkg.ApprovalStatePending,
					CreatedBy:      actor.Actor,
					Origin:         actor.Origin,
					CreatedAt:      now.Add(-2 * time.Hour),
					UpdatedAt:      now,
					LastActivityAt: lastActivity,
				}},
				Total: 1,
				Limit: query.Limit,
			}, nil
		},
		GetTaskFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (*taskpkg.View, error) {
			detailActor = actor
			return &taskpkg.View{
				Summary: taskpkg.Summary{
					ID:          id,
					Identifier:  "TASK-1",
					Title:       "Review handlers",
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-alpha",
					Status:      taskpkg.TaskStatusReady,
					CreatedBy:   actor.Actor,
					Origin:      actor.Origin,
					CreatedAt:   now.Add(-2 * time.Hour),
					UpdatedAt:   now,
				},
				Task: taskpkg.Task{
					ID:          id,
					Identifier:  "TASK-1",
					Title:       "Review handlers",
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-alpha",
					Status:      taskpkg.TaskStatusReady,
					CreatedBy:   actor.Actor,
					Origin:      actor.Origin,
					CreatedAt:   now.Add(-2 * time.Hour),
					UpdatedAt:   now,
				},
			}, nil
		},
		RunDetailFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (*taskpkg.RunDetailView, error) {
			runDetailActor = actor
			return &taskpkg.RunDetailView{
				Run: taskpkg.Run{
					ID:       id,
					TaskID:   "task-1",
					Status:   taskpkg.TaskRunStatusRunning,
					Attempt:  1,
					Origin:   actor.Origin,
					QueuedAt: now.Add(-5 * time.Minute),
				},
				Task: &taskpkg.Reference{
					ID:          "task-1",
					Identifier:  "TASK-1",
					Title:       "Review handlers",
					Status:      taskpkg.TaskStatusInProgress,
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-alpha",
				},
			}, nil
		},
		TimelineFn: func(_ context.Context, taskID string, query taskpkg.TimelineQuery, actor taskpkg.ActorContext) ([]taskpkg.TimelineItem, error) {
			timelineQuery = query
			timelineActor = actor
			return []taskpkg.TimelineItem{{
				Sequence: 7,
				EventID:  "evt-7",
				Task: taskpkg.Reference{
					ID:          taskID,
					Identifier:  "TASK-1",
					Title:       "Review handlers",
					Status:      taskpkg.TaskStatusInProgress,
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-alpha",
				},
				EventType: "task.run.started",
				Actor:     actor.Actor,
				Origin:    actor.Origin,
				Timestamp: now,
			}}, nil
		},
		TreeFn: func(_ context.Context, taskID string, actor taskpkg.ActorContext) (*taskpkg.TreeView, error) {
			treeActor = actor
			return &taskpkg.TreeView{
				Root: taskpkg.TreeNode{
					Task: taskpkg.Reference{
						ID:          taskID,
						Identifier:  "TASK-1",
						Title:       "Review handlers",
						Status:      taskpkg.TaskStatusInProgress,
						Scope:       taskpkg.ScopeWorkspace,
						WorkspaceID: "ws-alpha",
					},
					Depth:          0,
					LastActivityAt: now,
				},
			}, nil
		},
	}
	observer := testutil.StubObserver{
		QueryTaskDashboardFn: func(_ context.Context, query observe.TaskDashboardQuery) (observe.TaskDashboardView, error) {
			dashboardQuery = query
			return observe.TaskDashboardView{
				Totals:     observe.TaskDashboardTotals{TasksTotal: 1, RunsTotal: 1},
				Cards:      observe.TaskDashboardCards{},
				Queue:      observe.TaskDashboardQueue{Total: 1, OldestQueuedAt: now, BacklogStatus: "ok"},
				Health:     observe.TaskDashboardHealth{Status: "ok"},
				ActiveRuns: observe.TaskDashboardActiveRuns{Total: 1, Running: 1},
				Freshness: observe.TaskDashboardFreshness{
					ObservedAt:       now,
					LatestActivityAt: lastActivity,
					Status:           "live",
				},
			}, nil
		},
		QueryTaskInboxFn: func(_ context.Context, query observe.TaskInboxQuery, actor taskpkg.ActorIdentity) (observe.TaskInboxView, error) {
			inboxQuery = query
			inboxActor = actor
			return observe.TaskInboxView{
				Total:       1,
				UnreadTotal: 1,
				Groups: []observe.TaskInboxLaneGroup{{
					Lane:        observe.TaskInboxLaneApprovals,
					Count:       1,
					UnreadCount: 1,
				}},
			}, nil
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

	fixture := newHandlerFixtureWithTasks(t, testutil.StubSessionManager{}, observer, tasks, workspaces, nil, nil)
	fixture.Handlers.TaskActorContextResolver = func(_ *gin.Context, action string) (taskpkg.ActorContext, error) {
		return taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindHTTP, "tasks."+action)
	}

	listResp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/tasks?scope=workspace&workspace=alpha&priority=high&approval_state=pending&query=review&include_drafts=false&limit=2",
		nil,
	)
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listResp.Code, http.StatusOK, listResp.Body.String())
	}
	var listPayload contract.TasksResponse
	testutil.DecodeJSONResponse(t, listResp, &listPayload)
	if len(listPayload.Tasks) != 1 || listPayload.Tasks[0].ID != "task-1" {
		t.Fatalf("list payload = %#v", listPayload)
	}
	if listQuery.WorkspaceID != "ws-alpha" || listQuery.Priority != taskpkg.PriorityHigh ||
		listQuery.ApprovalState != taskpkg.ApprovalStatePending ||
		listQuery.Search != "review" {
		t.Fatalf("list query = %#v", listQuery)
	}
	if listActor.Origin.Ref != "tasks.list" {
		t.Fatalf("list actor = %#v", listActor)
	}

	detailResp := performRequest(t, fixture.Engine, http.MethodGet, "/tasks/task-1", nil)
	if detailResp.Code != http.StatusOK {
		t.Fatalf("detail status = %d, want %d; body=%s", detailResp.Code, http.StatusOK, detailResp.Body.String())
	}
	if detailActor.Origin.Ref != "tasks.get" {
		t.Fatalf("detail actor = %#v", detailActor)
	}

	runResp := performRequest(t, fixture.Engine, http.MethodGet, "/task-runs/run-1", nil)
	if runResp.Code != http.StatusOK {
		t.Fatalf("run detail status = %d, want %d; body=%s", runResp.Code, http.StatusOK, runResp.Body.String())
	}
	if runDetailActor.Origin.Ref != "tasks.get_run" {
		t.Fatalf("run detail actor = %#v", runDetailActor)
	}

	timelineResp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/tasks/task-1/timeline?after_sequence=5&limit=2",
		nil,
	)
	if timelineResp.Code != http.StatusOK {
		t.Fatalf("timeline status = %d, want %d; body=%s", timelineResp.Code, http.StatusOK, timelineResp.Body.String())
	}
	if timelineQuery.AfterSequence != 5 || timelineQuery.Limit != 2 || timelineActor.Origin.Ref != "tasks.timeline" {
		t.Fatalf("timeline query/actor = %#v / %#v", timelineQuery, timelineActor)
	}

	treeResp := performRequest(t, fixture.Engine, http.MethodGet, "/tasks/task-1/tree", nil)
	if treeResp.Code != http.StatusOK {
		t.Fatalf("tree status = %d, want %d; body=%s", treeResp.Code, http.StatusOK, treeResp.Body.String())
	}
	if treeActor.Origin.Ref != "tasks.tree" {
		t.Fatalf("tree actor = %#v", treeActor)
	}

	dashboardResp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/observe/tasks/dashboard?scope=workspace&workspace=alpha&owner_kind=human&owner_ref=alice&participation_channel=builders&origin_kind=http",
		nil,
	)
	if dashboardResp.Code != http.StatusOK {
		t.Fatalf(
			"dashboard status = %d, want %d; body=%s",
			dashboardResp.Code,
			http.StatusOK,
			dashboardResp.Body.String(),
		)
	}
	if dashboardQuery.WorkspaceID != "ws-alpha" || dashboardQuery.ParticipationChannel != "builders" ||
		dashboardQuery.OriginKind != taskpkg.OriginKindHTTP {
		t.Fatalf("dashboard query = %#v", dashboardQuery)
	}

	inboxResp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/observe/tasks/inbox?scope=workspace&workspace=alpha&owner_kind=human&owner_ref=alice&lane=approvals&unread=true&query=review&limit=1",
		nil,
	)
	if inboxResp.Code != http.StatusOK {
		t.Fatalf("inbox status = %d, want %d; body=%s", inboxResp.Code, http.StatusOK, inboxResp.Body.String())
	}
	if inboxQuery.WorkspaceID != "ws-alpha" || inboxQuery.Lane != observe.TaskInboxLaneApprovals ||
		inboxQuery.Unread == nil || !*inboxQuery.Unread ||
		inboxQuery.Search != "review" ||
		inboxActor.Ref != "user-1" {
		t.Fatalf("inbox query/actor = %#v / %#v", inboxQuery, inboxActor)
	}
}

func TestExpandedTaskMutationHandlersDelegateIntegration(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 17, 14, 30, 0, 0, time.UTC)
	calls := make([]string, 0, 7)
	origins := make([]string, 0, 7)
	executionRequests := make(map[string]taskpkg.ExecutionRequest, 3)

	appendCall := func(name string, actor taskpkg.ActorContext) {
		calls = append(calls, name)
		origins = append(origins, actor.Origin.Ref)
	}

	tasks := &testutil.StubTaskManager{
		PublishTaskFn: func(
			_ context.Context,
			id string,
			req taskpkg.ExecutionRequest,
			actor taskpkg.ActorContext,
		) (*taskpkg.Execution, error) {
			appendCall("publish", actor)
			executionRequests["publish"] = req
			taskRecord := taskpkg.Task{
				ID:          id,
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-alpha",
				Title:       "Publish",
				Status:      taskpkg.TaskStatusReady,
				CreatedBy:   actor.Actor,
				Origin:      actor.Origin,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			return &taskpkg.Execution{Task: taskRecord, Run: taskpkg.Run{
				ID:             "run-publish",
				TaskID:         id,
				Status:         taskpkg.TaskRunStatusQueued,
				Attempt:        1,
				Origin:         actor.Origin,
				IdempotencyKey: req.IdempotencyKey,
				Metadata:       req.Metadata,
				QueuedAt:       now,
			}}, nil
		},
		StartTaskFn: func(
			_ context.Context,
			id string,
			req taskpkg.ExecutionRequest,
			actor taskpkg.ActorContext,
		) (*taskpkg.Execution, error) {
			appendCall("start", actor)
			executionRequests["start"] = req
			taskRecord := taskpkg.Task{
				ID:          id,
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-alpha",
				Title:       "Start",
				Status:      taskpkg.TaskStatusReady,
				CreatedBy:   actor.Actor,
				Origin:      actor.Origin,
				CreatedAt:   now,
				UpdatedAt:   now,
			}
			return &taskpkg.Execution{Task: taskRecord, Run: taskpkg.Run{
				ID:             "run-start",
				TaskID:         id,
				Status:         taskpkg.TaskRunStatusQueued,
				Attempt:        1,
				Origin:         actor.Origin,
				IdempotencyKey: req.IdempotencyKey,
				Metadata:       req.Metadata,
				QueuedAt:       now,
			}}, nil
		},
		ApproveTaskFn: func(
			_ context.Context,
			id string,
			req taskpkg.ExecutionRequest,
			actor taskpkg.ActorContext,
		) (*taskpkg.Execution, error) {
			appendCall("approve", actor)
			executionRequests["approve"] = req
			taskRecord := taskpkg.Task{
				ID:             id,
				Scope:          taskpkg.ScopeWorkspace,
				WorkspaceID:    "ws-alpha",
				Title:          "Approve",
				Status:         taskpkg.TaskStatusReady,
				ApprovalPolicy: taskpkg.ApprovalPolicyManual,
				ApprovalState:  taskpkg.ApprovalStateApproved,
				CreatedBy:      actor.Actor,
				Origin:         actor.Origin,
				CreatedAt:      now,
				UpdatedAt:      now,
			}
			return &taskpkg.Execution{Task: taskRecord, Run: taskpkg.Run{
				ID:             "run-approve",
				TaskID:         id,
				Status:         taskpkg.TaskRunStatusQueued,
				Attempt:        1,
				Origin:         actor.Origin,
				IdempotencyKey: req.IdempotencyKey,
				Metadata:       req.Metadata,
				QueuedAt:       now,
			}}, nil
		},
		RejectTaskFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (*taskpkg.Task, error) {
			appendCall("reject", actor)
			return &taskpkg.Task{
				ID:             id,
				Scope:          taskpkg.ScopeWorkspace,
				WorkspaceID:    "ws-alpha",
				Title:          "Reject",
				Status:         taskpkg.TaskStatusBlocked,
				ApprovalPolicy: taskpkg.ApprovalPolicyManual,
				ApprovalState:  taskpkg.ApprovalStateRejected,
				CreatedBy:      actor.Actor,
				Origin:         actor.Origin,
				CreatedAt:      now,
				UpdatedAt:      now,
			}, nil
		},
		MarkTaskReadFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (taskpkg.TriageState, error) {
			appendCall("read", actor)
			return taskpkg.TriageState{TaskID: id, Actor: actor.Actor, Read: true, UpdatedAt: now}, nil
		},
		ArchiveTaskFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (taskpkg.TriageState, error) {
			appendCall("archive", actor)
			return taskpkg.TriageState{TaskID: id, Actor: actor.Actor, Read: true, Archived: true, UpdatedAt: now}, nil
		},
		DismissTaskFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (taskpkg.TriageState, error) {
			appendCall("dismiss", actor)
			return taskpkg.TriageState{TaskID: id, Actor: actor.Actor, Dismissed: true, UpdatedAt: now}, nil
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
	fixture.Handlers.TaskActorContextResolver = func(_ *gin.Context, action string) (taskpkg.ActorContext, error) {
		return taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindHTTP, "tasks."+action)
	}

	for _, tc := range []struct {
		name         string
		call         string
		path         string
		body         []byte
		want         int
		wantKey      string
		wantChannel  string
		wantMetadata string
	}{
		{
			name:         "Should return 200 for task publish and forward execution request",
			call:         "publish",
			path:         "/tasks/task-1/publish",
			body:         []byte(`{"idempotency_key":"publish-1","metadata":{"action":"publish"}}`),
			want:         http.StatusOK,
			wantKey:      "publish-1",
			wantMetadata: `{"action":"publish"}`,
		},
		{
			name:         "Should return 201 for task start and forward execution request",
			call:         "start",
			path:         "/tasks/task-1/start",
			body:         []byte(`{"idempotency_key":"start-1","metadata":{"action":"start"}}`),
			want:         http.StatusCreated,
			wantKey:      "start-1",
			wantMetadata: `{"action":"start"}`,
		},
		{
			name:         "Should return 201 for task approve and forward execution request",
			call:         "approve",
			path:         "/tasks/task-1/approve",
			body:         []byte(`{"idempotency_key":"approve-1","metadata":{"action":"approve"}}`),
			want:         http.StatusCreated,
			wantKey:      "approve-1",
			wantMetadata: `{"action":"approve"}`,
		},
		{name: "Should return 200 for task reject", call: "reject", path: "/tasks/task-1/reject", want: http.StatusOK},
		{name: "Should return 200 for task triage read", call: "read", path: "/tasks/task-1/triage/read", want: http.StatusOK},
		{name: "Should return 200 for task triage archive", call: "archive", path: "/tasks/task-1/triage/archive", want: http.StatusOK},
		{name: "Should return 200 for task triage dismiss", call: "dismiss", path: "/tasks/task-1/triage/dismiss", want: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := performRequest(t, fixture.Engine, http.MethodPost, tc.path, tc.body)
			if resp.Code != tc.want {
				t.Fatalf("%s status = %d, want %d; body=%s", tc.path, resp.Code, tc.want, resp.Body.String())
			}
			switch tc.call {
			case "publish", "start", "approve":
				var response contract.TaskExecutionResponse
				testutil.DecodeJSONResponse(t, resp, &response)
				if response.Task.ID != "task-1" ||
					response.Run.ID != "run-"+tc.call ||
					response.Run.TaskID != "task-1" ||
					response.Run.Status != taskpkg.TaskRunStatusQueued ||
					response.Run.IdempotencyKey != tc.wantKey ||
					response.Run.ResolvedNetworkParticipation == nil ||
					response.Run.ResolvedNetworkParticipation.ChannelID != tc.wantChannel ||
					string(response.Run.Metadata) != tc.wantMetadata {
					t.Fatalf("%s response = %#v, want task/run execution payload", tc.path, response)
				}
			case "reject":
				var response contract.TaskResponse
				testutil.DecodeJSONResponse(t, resp, &response)
				if response.Task.ID != "task-1" ||
					response.Task.ApprovalState != taskpkg.ApprovalStateRejected {
					t.Fatalf("%s response = %#v, want rejected task payload", tc.path, response)
				}
			case "read", "archive", "dismiss":
				var response contract.TaskTriageStateResponse
				testutil.DecodeJSONResponse(t, resp, &response)
				if response.Triage.TaskID != "task-1" ||
					response.Triage.Actor.Ref != "user-1" {
					t.Fatalf("%s response = %#v, want actor triage payload", tc.path, response)
				}
				if (tc.call == "read" && !response.Triage.Read) ||
					(tc.call == "archive" && !response.Triage.Archived) ||
					(tc.call == "dismiss" && !response.Triage.Dismissed) {
					t.Fatalf("%s triage response = %#v, want %s state", tc.path, response.Triage, tc.call)
				}
			default:
				t.Fatalf("unhandled mutation case %q", tc.call)
			}
			if tc.wantKey == "" {
				return
			}
			got, ok := executionRequests[tc.call]
			if !ok {
				t.Fatalf("%s request was not recorded", tc.call)
			}
			if got.IdempotencyKey != tc.wantKey || string(got.Metadata) != tc.wantMetadata {
				t.Fatalf("%s request = %#v, want key=%q channel=%q metadata=%s",
					tc.call,
					got,
					tc.wantKey,
					tc.wantChannel,
					tc.wantMetadata,
				)
			}
		})
	}

	if want := []string{
		"publish",
		"start",
		"approve",
		"reject",
		"read",
		"archive",
		"dismiss",
	}; !reflect.DeepEqual(
		calls,
		want,
	) {
		t.Fatalf("mutation calls = %#v, want %#v", calls, want)
	}
	if want := []string{
		"tasks.publish",
		"tasks.start",
		"tasks.approve",
		"tasks.reject",
		"tasks.triage_read",
		"tasks.triage_archive",
		"tasks.triage_dismiss",
	}; !reflect.DeepEqual(origins, want) {
		t.Fatalf("mutation origins = %#v, want %#v", origins, want)
	}
}

func TestTaskStreamHandlerUsesSharedSSEPathIntegration(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 17, 15, 0, 0, 0, time.UTC)
	var streamQuery taskpkg.StreamQuery
	var streamActor taskpkg.ActorContext

	tasks := &testutil.StubTaskManager{
		StreamFn: func(_ context.Context, taskID string, query taskpkg.StreamQuery, actor taskpkg.ActorContext) (<-chan taskpkg.StreamEvent, error) {
			streamQuery = query
			streamActor = actor
			ch := make(chan taskpkg.StreamEvent, 1)
			ch <- taskpkg.StreamEvent{
				Sequence: 21,
				Type:     "task.run.started",
				Timeline: taskpkg.TimelineItem{
					Sequence: 21,
					EventID:  "evt-21",
					Task: taskpkg.Reference{
						ID:          taskID,
						Identifier:  "TASK-1",
						Title:       "Stream task",
						Status:      taskpkg.TaskStatusInProgress,
						Scope:       taskpkg.ScopeWorkspace,
						WorkspaceID: "ws-alpha",
					},
					EventType: "task.run.started",
					Actor:     actor.Actor,
					Origin:    actor.Origin,
					Timestamp: now,
				},
			}
			close(ch)
			return ch, nil
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
	fixture.Handlers.TaskActorContextResolver = func(_ *gin.Context, action string) (taskpkg.ActorContext, error) {
		return taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindHTTP, "tasks."+action)
	}

	resp := testutil.PerformRequestWithHeaders(
		t,
		fixture.Engine,
		http.MethodGet,
		"/tasks/task-1/stream?after_sequence=3",
		nil,
		map[string]string{"Last-Event-ID": "7"},
	)
	if resp.Code != http.StatusOK {
		t.Fatalf("stream status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
	}
	if got := resp.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("stream content-type = %q, want prefix %q", got, "text/event-stream")
	}
	if !resp.Flushed {
		t.Fatal("stream response was not flushed")
	}
	records := testutil.ParseSSE(t, resp.Body.String())
	if len(records) != 1 || records[0].ID != "21" || records[0].Event != "" {
		t.Fatalf("stream records = %#v", records)
	}
	var payload contract.TaskStreamEventPayload
	testutil.DecodeSSEData(t, records[0], &payload)
	if payload.Type != "task.run.started" {
		t.Fatalf("stream payload type = %q, want %q", payload.Type, "task.run.started")
	}
	if streamQuery.AfterSequence != 7 || streamActor.Origin.Ref != "tasks.stream" {
		t.Fatalf("stream query/actor = %#v / %#v", streamQuery, streamActor)
	}
}
