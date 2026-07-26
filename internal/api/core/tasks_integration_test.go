//go:build integration

package core_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/testutil"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestTaskHandlersCreateTaskAndListFiltersReachManagerIntegration(t *testing.T) {
	t.Parallel()

	var capturedCreate taskpkg.CreateTask
	var capturedCreateActor taskpkg.ActorContext
	var capturedList taskpkg.Query
	var capturedListActor taskpkg.ActorContext

	tasks := &testutil.StubTaskManager{
		CreateTaskFn: func(_ context.Context, spec taskpkg.CreateTask, actor taskpkg.ActorContext) (*taskpkg.Task, error) {
			capturedCreate = spec
			capturedCreateActor = actor
			return &taskpkg.Task{
				ID:          "task-1",
				Identifier:  spec.Identifier,
				Scope:       spec.Scope,
				WorkspaceID: spec.WorkspaceID,
				Title:       spec.Title,
				Description: spec.Description,
				Status:      taskpkg.TaskStatusPending,
				Owner:       spec.Owner,
				CreatedBy:   actor.Actor,
				Origin:      actor.Origin,
				CreatedAt:   time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
				UpdatedAt:   time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
				Metadata:    spec.Metadata,
			}, nil
		},
		ListTasksFn: func(_ context.Context, query taskpkg.Query, actor taskpkg.ActorContext) ([]taskpkg.Summary, error) {
			capturedList = query
			capturedListActor = actor
			return []taskpkg.Summary{{
				ID:        "task-1",
				Scope:     query.Scope,
				Title:     "Review task API",
				Status:    query.Status,
				CreatedBy: actor.Actor,
				Origin:    actor.Origin,
				CreatedAt: time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
				UpdatedAt: time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC),
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

	createResp := performRequest(
		t,
		fixture.Engine,
		"POST",
		"/tasks",
		[]byte(
			`{"scope":"workspace","workspace":"alpha","identifier":"TASK-1","title":"Review task API","description":"Check handler wiring","auto_enqueue_on_ready":true,"owner":{"kind":"pool","ref":"reviewers"},"metadata":{"priority":"high"}}`,
		),
	)
	if createResp.Code != 201 {
		t.Fatalf("create status = %d, want %d; body=%s", createResp.Code, 201, createResp.Body.String())
	}

	if capturedCreate.Scope != taskpkg.ScopeWorkspace || capturedCreate.WorkspaceID != "ws-alpha" {
		t.Fatalf("create spec = %#v", capturedCreate)
	}
	if capturedCreate.Owner == nil || capturedCreate.Owner.Ref != "reviewers" {
		t.Fatalf("create spec = %#v", capturedCreate)
	}
	if !capturedCreate.AutoEnqueueOnReady {
		t.Fatalf("create spec auto_enqueue_on_ready = false, want true; spec=%#v", capturedCreate)
	}
	if capturedCreateActor.Actor.Ref != "user-1" || capturedCreateActor.Origin.Ref != "tasks.create" {
		t.Fatalf("create actor = %#v", capturedCreateActor)
	}

	listResp := performRequest(
		t,
		fixture.Engine,
		"GET",
		"/tasks?scope=workspace&workspace=alpha&status=ready&owner_kind=pool&owner_ref=reviewers&parent_task_id=task-root&limit=5",
		nil,
	)
	if listResp.Code != 200 {
		t.Fatalf("list status = %d, want %d; body=%s", listResp.Code, 200, listResp.Body.String())
	}

	if capturedList.Scope != taskpkg.ScopeWorkspace || capturedList.WorkspaceID != "ws-alpha" {
		t.Fatalf("list query = %#v", capturedList)
	}
	if capturedList.Status != taskpkg.TaskStatusReady || capturedList.OwnerKind != taskpkg.OwnerKindPool ||
		capturedList.OwnerRef != "reviewers" {
		t.Fatalf("list query = %#v", capturedList)
	}
	if capturedList.ParentTaskID != "task-root" || capturedList.Limit != 5 {
		t.Fatalf("list query = %#v", capturedList)
	}
	if capturedListActor.Actor.Ref != "user-1" || capturedListActor.Origin.Ref != "tasks.list" {
		t.Fatalf("list actor = %#v", capturedListActor)
	}
}

func TestTaskRunHandlersDelegateLifecycleSequenceIntegration(t *testing.T) {
	t.Parallel()

	calls := make([]string, 0, 3)
	now := time.Date(2026, 4, 14, 12, 0, 0, 0, time.UTC)

	tasks := &testutil.StubTaskManager{
		EnqueueRunFn: func(_ context.Context, spec taskpkg.EnqueueRun, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
			calls = append(calls, "enqueue")
			return &taskpkg.Run{
				ID:             "run-1",
				TaskID:         spec.TaskID,
				Status:         taskpkg.TaskRunStatusQueued,
				Attempt:        1,
				Origin:         actor.Origin,
				IdempotencyKey: spec.IdempotencyKey,
				QueuedAt:       now,
			}, nil
		},
		StartRunFn: func(_ context.Context, runID string, _ taskpkg.StartRun, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
			calls = append(calls, "start")
			return &taskpkg.Run{
				ID:        runID,
				TaskID:    "task-1",
				Status:    taskpkg.TaskRunStatusRunning,
				Attempt:   1,
				SessionID: "sess-1",
				Origin:    actor.Origin,
				QueuedAt:  now,
				StartedAt: now.Add(2 * time.Minute),
			}, nil
		},
		CompleteRunFn: func(_ context.Context, runID string, result taskpkg.RunResult, actor taskpkg.ActorContext) (*taskpkg.Run, error) {
			calls = append(calls, "complete")
			return &taskpkg.Run{
				ID:       runID,
				TaskID:   "task-1",
				Status:   taskpkg.TaskRunStatusCompleted,
				Attempt:  1,
				Origin:   actor.Origin,
				QueuedAt: now,
				EndedAt:  now.Add(3 * time.Minute),
				Result:   result.Value,
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
	fixture.Handlers.TaskActorContextResolver = func(_ *gin.Context, action string) (taskpkg.ActorContext, error) {
		return taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindHTTP, "tasks."+action)
	}

	resp := performRequest(
		t,
		fixture.Engine,
		"POST",
		"/tasks/task-1/runs",
		[]byte(`{"idempotency_key":"key-1"}`),
	)
	if resp.Code != 201 {
		t.Fatalf("enqueue status = %d, want %d; body=%s", resp.Code, 201, resp.Body.String())
	}

	resp = performRequest(t, fixture.Engine, "POST", "/task-runs/run-1/start", []byte(`{}`))
	if resp.Code != 200 {
		t.Fatalf("start status = %d, want %d; body=%s", resp.Code, 200, resp.Body.String())
	}

	resp = performRequest(t, fixture.Engine, "POST", "/task-runs/run-1/complete", []byte(`{"result":{"ok":true}}`))
	if resp.Code != 200 {
		t.Fatalf("complete status = %d, want %d; body=%s", resp.Code, 200, resp.Body.String())
	}

	if want := []string{"enqueue", "start", "complete"}; !reflect.DeepEqual(calls, want) {
		t.Fatalf("call order = %#v, want %#v", calls, want)
	}
}
