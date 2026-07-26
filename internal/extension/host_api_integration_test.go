//go:build integration

package extensionpkg

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	apicontract "github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func withHostAPIHooks(hooks *hookspkg.Hooks) hostAPITestEnvOption {
	return func(cfg *hostAPITestEnvConfig) {
		cfg.hooks = hooks
	}
}

func (d *hostAPIFakeDriver) promptCalls() []acp.PromptRequest {
	d.mu.Lock()
	defer d.mu.Unlock()

	return append([]acp.PromptRequest(nil), d.promptLog...)
}

func TestHostAPIIntegrationSessionLifecycleThroughHostAPI(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant(
		"ext-integration",
		[]string{"sessions/create", "sessions/prompt", "sessions/status", "sessions/events"},
		[]string{"session.write", "session.read"},
	)

	createResult, err := env.call(t, "ext-integration", "sessions/create", map[string]string{
		"agent":     "coder",
		"provider":  "fake-alt",
		"workspace": env.workspaceID,
	})
	if err != nil {
		t.Fatalf("Handle(sessions/create) error = %v", err)
	}

	var created hostAPISessionCreateResult
	decodeResult(t, createResult, &created)
	if created.SessionID == "" {
		t.Fatal("sessions/create session_id = empty, want non-empty")
	}
	if created.Provider != "fake-alt" {
		t.Fatalf("sessions/create provider = %q, want %q", created.Provider, "fake-alt")
	}

	prompt, err := env.submitPrompt(t, "ext-integration", created.SessionID, "integration prompt")
	if err != nil {
		t.Fatalf("submitPrompt() error = %v", err)
	}
	if prompt.TurnID == "" {
		t.Fatal("sessions/prompt turn_id = empty, want non-empty")
	}

	statusResult, err := env.call(t, "ext-integration", "sessions/status", map[string]string{
		"workspace_id": env.workspaceID,
		"session_id":   created.SessionID,
	})
	if err != nil {
		t.Fatalf("Handle(sessions/status) error = %v", err)
	}

	var status hostAPISessionStatus
	decodeResult(t, statusResult, &status)
	if status.State == "" {
		t.Fatal("sessions/status state = empty, want non-empty")
	}
	if status.Provider != "fake-alt" {
		t.Fatalf("sessions/status provider = %q, want %q", status.Provider, "fake-alt")
	}

	eventsResult, err := env.call(t, "ext-integration", "sessions/events", map[string]any{
		"workspace_id": env.workspaceID,
		"session_id":   created.SessionID,
		"turn_id":      prompt.TurnID,
		"limit":        10,
	})
	if err != nil {
		t.Fatalf("Handle(sessions/events) error = %v", err)
	}

	var events []hostAPISessionEvent
	decodeResult(t, eventsResult, &events)
	if len(events) == 0 {
		t.Fatal("sessions/events len = 0, want prompt events")
	}
}

func TestHostAPIIntegrationStoresAndRecallsMemory(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant("ext-integration", []string{"memory/store", "memory/recall"}, []string{"memory.write", "memory.read"})

	if _, err := env.call(t, "ext-integration", "memory/store", map[string]any{
		"key":     "deploy-checklist",
		"content": "Run smoke tests before deploy",
		"tags":    []string{"reference", "deploy"},
	}); err != nil {
		t.Fatalf("Handle(memory/store) error = %v", err)
	}

	result, err := env.call(t, "ext-integration", "memory/recall", map[string]any{
		"query": "smoke tests before deploy",
		"limit": 5,
	})
	if err != nil {
		t.Fatalf("Handle(memory/recall) error = %v", err)
	}

	var entries []hostAPIMemoryRecallEntry
	decodeResult(t, result, &entries)
	if len(entries) == 0 {
		t.Fatal("memory/recall len = 0, want stored memory")
	}
}

func TestHostAPIIntegrationResourcesSnapshotPublishesAndReadsBack(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grantWithResources(
		t,
		"ext-resources",
		[]string{"resources/list", "resources/get", "resources/snapshot"},
		[]string{"resources.read", "resources.write"},
		[]string{"tools"},
		resources.ResourceScopeKindWorkspace,
	)

	sessionNonce := "nonce-integration"
	env.activateResourceSession(t, "ext-resources", sessionNonce)

	if _, err := env.callResource(t, "ext-resources", sessionNonce, "resources/snapshot", map[string]any{
		"source_version": 1,
		"records": []map[string]any{
			{
				"kind":  "tool",
				"id":    "grep",
				"scope": map[string]any{"kind": "workspace", "id": env.workspaceID},
				"spec":  hostAPITestToolSpec("grep", "search workspace", "extension"),
			},
		},
	}); err != nil {
		t.Fatalf("Handle(resources/snapshot) error = %v", err)
	}

	listResult, err := env.callResource(t, "ext-resources", sessionNonce, "resources/list", map[string]any{
		"kind": "tool",
	})
	if err != nil {
		t.Fatalf("Handle(resources/list) error = %v", err)
	}

	var listed []hostAPIResourceRecord
	decodeResult(t, listResult, &listed)
	if got, want := len(listed), 1; got != want {
		t.Fatalf("len(resources/list) = %d, want %d", got, want)
	}
	if got, want := listed[0].ID, "grep"; got != want {
		t.Fatalf("resources/list[0].ID = %q, want %q", got, want)
	}

	getResult, err := env.callResource(t, "ext-resources", sessionNonce, "resources/get", map[string]any{
		"kind": "tool",
		"id":   "grep",
	})
	if err != nil {
		t.Fatalf("Handle(resources/get) error = %v", err)
	}

	var record hostAPIResourceRecord
	decodeResult(t, getResult, &record)
	if got, want := record.Version, int64(1); got != want {
		t.Fatalf("resources/get version = %d, want %d", got, want)
	}
}

func TestHostAPIIntegrationBridgeProviderKeepsOperationalMethodsAlongsideGenericResourceReads(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grantWithResources(
		t,
		"telegram-adapter",
		[]string{"resources/list", "resources/get", "bridges/instances/list", "bridges/instances/get"},
		[]string{"resources.read", "bridge.read"},
		[]string{"tools"},
		resources.ResourceScopeKindWorkspace,
	)

	sessionNonce := "nonce-bridge-provider"
	env.activateResourceSession(t, "telegram-adapter", sessionNonce)

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ID:            "brg-resource-coexist",
		ExtensionName: "telegram-adapter",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, instance)

	listResult, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/list", nil)
	if err != nil {
		t.Fatalf("Handle(bridges/instances/list) error = %v", err)
	}

	var listed []bridgecontract.BridgeInstance
	decodeResult(t, listResult, &listed)
	if got, want := len(listed), 1; got != want {
		t.Fatalf("len(bridges/instances/list) = %d, want %d", got, want)
	}
	if got, want := listed[0].ID, instance.ID; got != want {
		t.Fatalf("bridges/instances/list[0].ID = %q, want %q", got, want)
	}

	getResult, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/get", map[string]any{
		"bridge_instance_id": instance.ID,
	})
	if err != nil {
		t.Fatalf("Handle(bridges/instances/get) error = %v", err)
	}

	var loaded bridgecontract.BridgeInstance
	decodeResult(t, getResult, &loaded)
	if got, want := loaded.ID, instance.ID; got != want {
		t.Fatalf("bridges/instances/get id = %q, want %q", got, want)
	}

	resourceListResult, err := env.callResource(
		t,
		"telegram-adapter",
		sessionNonce,
		"resources/list",
		map[string]any{"kind": "tool"},
	)
	if err != nil {
		t.Fatalf("Handle(resources/list) error = %v", err)
	}

	var resourcesListed []hostAPIResourceRecord
	decodeResult(t, resourceListResult, &resourcesListed)
	if got := len(resourcesListed); got != 0 {
		t.Fatalf("len(resources/list tool) = %d, want 0", got)
	}

	_, err = env.callResource(t, "telegram-adapter", sessionNonce, "resources/get", map[string]any{
		"kind": "bridge.instance",
		"id":   instance.ID,
	})
	assertRPCErrorCode(t, err, 403)
}

func TestHostAPIIntegrationSecondResourceSessionInvalidatesOlderNonce(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grantWithResources(
		t,
		"ext-resources",
		[]string{"resources/snapshot"},
		[]string{"resources.write"},
		[]string{"tools"},
		resources.ResourceScopeKindWorkspace,
	)

	firstNonce := "nonce-first"
	env.activateResourceSession(t, "ext-resources", firstNonce)
	if _, err := env.callResource(t, "ext-resources", firstNonce, "resources/snapshot", map[string]any{
		"source_version": 1,
		"records": []map[string]any{
			{
				"kind":  "tool",
				"id":    "grep",
				"scope": map[string]any{"kind": "workspace", "id": env.workspaceID},
				"spec":  hostAPITestToolSpec("grep", "search workspace", "extension"),
			},
		},
	}); err != nil {
		t.Fatalf("first Handle(resources/snapshot) error = %v", err)
	}

	secondNonce := "nonce-second"
	env.activateResourceSession(t, "ext-resources", secondNonce)

	if _, err := env.callResource(t, "ext-resources", secondNonce, "resources/snapshot", map[string]any{
		"source_version": 1,
		"records": []map[string]any{
			{
				"kind":  "tool",
				"id":    "grep",
				"scope": map[string]any{"kind": "workspace", "id": env.workspaceID},
				"spec":  hostAPITestToolSpec("grep", "search workspace v2", "extension"),
			},
		},
	}); err != nil {
		t.Fatalf("second Handle(resources/snapshot) error = %v", err)
	}

	_, err := env.callResource(t, "ext-resources", firstNonce, "resources/snapshot", map[string]any{
		"source_version": 2,
		"records": []map[string]any{
			{
				"kind":  "tool",
				"id":    "grep",
				"scope": map[string]any{"kind": "workspace", "id": env.workspaceID},
				"spec":  hostAPITestToolSpec("grep", "stale workspace search", "extension"),
			},
		},
	})
	assertRPCErrorCode(t, err, 409)
}

func TestHostAPIIntegrationResourceSnapshotReplacesToolSetAndRemovesStaleTools(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grantWithResources(
		t,
		"ext-resources",
		[]string{"resources/list", "resources/snapshot"},
		[]string{"resources.read", "resources.write"},
		[]string{"tools"},
		resources.ResourceScopeKindWorkspace,
	)

	sessionNonce := "nonce-replace"
	env.activateResourceSession(t, "ext-resources", sessionNonce)

	if _, err := env.callResource(t, "ext-resources", sessionNonce, "resources/snapshot", map[string]any{
		"source_version": 1,
		"records": []map[string]any{
			{
				"kind":  "tool",
				"id":    "grep",
				"scope": map[string]any{"kind": "workspace", "id": env.workspaceID},
				"spec":  hostAPITestToolSpec("grep", "search workspace", "extension"),
			},
		},
	}); err != nil {
		t.Fatalf("first Handle(resources/snapshot) error = %v", err)
	}

	if _, err := env.callResource(t, "ext-resources", sessionNonce, "resources/snapshot", map[string]any{
		"source_version": 2,
		"records": []map[string]any{
			{
				"kind":  "tool",
				"id":    "lookup",
				"scope": map[string]any{"kind": "workspace", "id": env.workspaceID},
				"spec":  hostAPITestToolSpec("lookup", "lookup workspace", "extension"),
			},
		},
	}); err != nil {
		t.Fatalf("second Handle(resources/snapshot) error = %v", err)
	}

	listResult, err := env.callResource(t, "ext-resources", sessionNonce, "resources/list", map[string]any{
		"kind": "tool",
	})
	if err != nil {
		t.Fatalf("Handle(resources/list) error = %v", err)
	}

	var listed []hostAPIResourceRecord
	decodeResult(t, listResult, &listed)
	if got, want := len(listed), 1; got != want {
		t.Fatalf("len(resources/list) = %d, want %d", got, want)
	}
	if got, want := listed[0].ID, "lookup"; got != want {
		t.Fatalf("resources/list[0].ID = %q, want %q", got, want)
	}
}

func TestHostAPIIntegrationExtensionCanCreateTaskAndEnqueueRun(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant(
		"ext-tasks",
		[]string{"tasks/create", "tasks/get", "tasks/runs/enqueue"},
		[]string{"task.write", "task.read"},
	)

	createResult, err := env.call(t, "ext-tasks", "tasks/create", map[string]any{
		"scope":     taskpkg.ScopeWorkspace,
		"workspace": env.workspaceID,
		"title":     "Extension-created task",
	})
	if err != nil {
		t.Fatalf("Handle(tasks/create) error = %v", err)
	}

	var created apicontract.TaskPayload
	decodeResult(t, createResult, &created)
	if created.ID == "" {
		t.Fatal("tasks/create id = empty, want non-empty")
	}

	enqueueResult, err := env.call(t, "ext-tasks", "tasks/runs/enqueue", map[string]any{
		"task_id":         created.ID,
		"idempotency_key": "enqueue-int",
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs/enqueue) error = %v", err)
	}

	var run apicontract.TaskRunPayload
	decodeResult(t, enqueueResult, &run)
	if run.ID == "" {
		t.Fatal("tasks/runs/enqueue id = empty, want non-empty")
	}
	if got, want := run.Status, taskpkg.TaskRunStatusQueued; got != want {
		t.Fatalf("tasks/runs/enqueue status = %q, want %q", got, want)
	}

	storedTask, err := env.registry.GetTask(testutil.Context(t), created.ID)
	if err != nil {
		t.Fatalf("registry.GetTask(%q) error = %v", created.ID, err)
	}
	if got, want := storedTask.CreatedBy.Kind, taskpkg.ActorKindExtension; got != want {
		t.Fatalf("storedTask.CreatedBy.Kind = %q, want %q", got, want)
	}
	if got, want := storedTask.CreatedBy.Ref, "ext-tasks"; got != want {
		t.Fatalf("storedTask.CreatedBy.Ref = %q, want %q", got, want)
	}

	storedRun, err := env.registry.GetTaskRun(testutil.Context(t), run.ID)
	if err != nil {
		t.Fatalf("registry.GetTaskRun(%q) error = %v", run.ID, err)
	}
	if got, want := storedRun.Origin.Kind, taskpkg.OriginKindExtension; got != want {
		t.Fatalf("storedRun.Origin.Kind = %q, want %q", got, want)
	}
	if got, want := storedRun.Origin.Ref, "ext-tasks"; got != want {
		t.Fatalf("storedRun.Origin.Ref = %q, want %q", got, want)
	}

	getResult, err := env.call(t, "ext-tasks", "tasks/get", map[string]any{"id": created.ID})
	if err != nil {
		t.Fatalf("Handle(tasks/get) error = %v", err)
	}

	var detail apicontract.TaskDetailPayload
	decodeResult(t, getResult, &detail)
	if got, want := len(detail.Runs), 1; got != want {
		t.Fatalf("len(tasks/get.runs) = %d, want %d", got, want)
	}
	if got, want := detail.Runs[0].ID, run.ID; got != want {
		t.Fatalf("tasks/get.runs[0].ID = %q, want %q", got, want)
	}
}

func TestHostAPIIntegrationStartRunAllocatesDedicatedSession(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant(
		"ext-tasks",
		[]string{"tasks/create", "tasks/runs/enqueue", "tasks/runs/start"},
		[]string{"task.write"},
	)

	createResult, err := env.call(t, "ext-tasks", "tasks/create", map[string]any{
		"scope":     taskpkg.ScopeWorkspace,
		"workspace": env.workspaceID,
		"title":     "Executable extension task",
	})
	if err != nil {
		t.Fatalf("Handle(tasks/create) error = %v", err)
	}

	var created apicontract.TaskPayload
	decodeResult(t, createResult, &created)

	enqueueResult, err := env.call(t, "ext-tasks", "tasks/runs/enqueue", map[string]any{
		"task_id":         created.ID,
		"idempotency_key": "enqueue-start-int",
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs/enqueue) error = %v", err)
	}

	var queued apicontract.TaskRunPayload
	decodeResult(t, enqueueResult, &queued)

	seedHostAPIRunClaimed(t, env, queued.ID, "ext-tasks")

	startResult, err := env.call(t, "ext-tasks", "tasks/runs/start", map[string]any{
		"id":              queued.ID,
		"idempotency_key": "start-start-int",
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs/start) error = %v", err)
	}

	var started apicontract.TaskRunPayload
	decodeResult(t, startResult, &started)
	if got, want := started.Status, taskpkg.TaskRunStatusRunning; got != want {
		t.Fatalf("tasks/runs/start status = %q, want %q", got, want)
	}
	if started.SessionID == "" {
		t.Fatal("tasks/runs/start session_id = empty, want non-empty")
	}

	info, err := env.sessions.Status(testutil.Context(t), started.SessionID)
	if err != nil {
		t.Fatalf("sessions.Status(%q) error = %v", started.SessionID, err)
	}
	if got, want := info.WorkspaceID, env.workspaceID; got != want {
		t.Fatalf("session.WorkspaceID = %q, want %q", got, want)
	}
	if got, want := info.Type, session.SessionTypeSystem; got != want {
		t.Fatalf("session.Type = %q, want %q", got, want)
	}
	if got, want := info.State, session.StateActive; got != want {
		t.Fatalf("session.State = %q, want %q", got, want)
	}
}

func TestHostAPIIntegrationTaskReadAndAggregateSurfaces(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant(
		"ext-reader",
		[]string{
			"tasks",
			"tasks/get",
			"tasks/runs/get",
			"tasks/timeline",
			"tasks/tree",
			"tasks/dashboard",
			"tasks/inbox",
		},
		[]string{"task.read"},
	)

	actor := mustExtensionTaskActorContext(t, "seed-writer")
	root, err := env.tasks.CreateTask(testutil.Context(t), taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: env.workspaceID,
		Title:       "Integration root",
	}, actor)
	if err != nil {
		t.Fatalf("tasks.CreateTask(root) error = %v", err)
	}

	child, err := env.tasks.CreateChildTask(testutil.Context(t), root.ID, taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: env.workspaceID,
		Title:       "Integration child",
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindExtension,
			Ref:  "ext-reader",
		},
	}, actor)
	if err != nil {
		t.Fatalf("tasks.CreateChildTask(child) error = %v", err)
	}

	approvalTask, err := env.tasks.CreateTask(testutil.Context(t), taskpkg.CreateTask{
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    env.workspaceID,
		Title:          "Integration approval",
		ApprovalPolicy: taskpkg.ApprovalPolicyManual,
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindExtension,
			Ref:  "ext-reader",
		},
	}, actor)
	if err != nil {
		t.Fatalf("tasks.CreateTask(approval) error = %v", err)
	}

	queued, err := env.tasks.EnqueueRun(testutil.Context(t), taskpkg.EnqueueRun{
		TaskID:         child.ID,
		IdempotencyKey: "integration-read-run",
	}, actor)
	if err != nil {
		t.Fatalf("tasks.EnqueueRun() error = %v", err)
	}
	seedHostAPIRunClaimed(t, env, queued.ID, "ext-reader")
	started, err := env.tasks.StartRun(testutil.Context(t), queued.ID, taskpkg.StartRun{
		IdempotencyKey: "integration-read-start",
	}, actor)
	if err != nil {
		t.Fatalf("tasks.StartRun() error = %v", err)
	}

	getResult, err := env.call(t, "ext-reader", "tasks/get", map[string]any{"id": child.ID})
	if err != nil {
		t.Fatalf("Handle(tasks/get) error = %v", err)
	}
	var detail apicontract.TaskDetailPayload
	decodeResult(t, getResult, &detail)
	if got, want := detail.Task.ID, child.ID; got != want {
		t.Fatalf("tasks/get.task.id = %q, want %q", got, want)
	}

	runDetailResult, err := env.call(t, "ext-reader", "tasks/runs/get", map[string]any{"id": started.ID})
	if err != nil {
		t.Fatalf("Handle(tasks/runs/get) error = %v", err)
	}
	var runDetail apicontract.TaskRunDetailPayload
	decodeResult(t, runDetailResult, &runDetail)
	if got, want := runDetail.Run.ID, started.ID; got != want {
		t.Fatalf("tasks/runs/get.run.id = %q, want %q", got, want)
	}
	if runDetail.Session == nil || runDetail.Session.SessionID == "" {
		t.Fatal("tasks/runs/get.session = nil/empty, want session link")
	}

	timelineResult, err := env.call(t, "ext-reader", "tasks/timeline", map[string]any{"id": child.ID, "limit": 10})
	if err != nil {
		t.Fatalf("Handle(tasks/timeline) error = %v", err)
	}
	var timeline []apicontract.TaskTimelineItemPayload
	decodeResult(t, timelineResult, &timeline)
	if len(timeline) == 0 {
		t.Fatal("tasks/timeline len = 0, want events")
	}

	treeResult, err := env.call(t, "ext-reader", "tasks/tree", map[string]any{"id": root.ID})
	if err != nil {
		t.Fatalf("Handle(tasks/tree) error = %v", err)
	}
	var tree apicontract.TaskTreePayload
	decodeResult(t, treeResult, &tree)
	if got, want := tree.Root.Task.ID, root.ID; got != want {
		t.Fatalf("tasks/tree.root.task.id = %q, want %q", got, want)
	}

	dashboardResult, err := env.call(t, "ext-reader", "tasks/dashboard", map[string]any{
		"scope":     taskpkg.ScopeWorkspace,
		"workspace": env.workspaceID,
	})
	if err != nil {
		t.Fatalf("Handle(tasks/dashboard) error = %v", err)
	}
	var dashboard apicontract.TaskDashboardPayload
	decodeResult(t, dashboardResult, &dashboard)
	if dashboard.Totals.ActiveRuns < 1 {
		t.Fatalf("tasks/dashboard active_runs = %d, want >= 1", dashboard.Totals.ActiveRuns)
	}

	dashboardWorkspaceOnlyResult, err := env.call(t, "ext-reader", "tasks/dashboard", map[string]any{
		"workspace": env.workspaceID,
	})
	if err != nil {
		t.Fatalf("Handle(tasks/dashboard workspace-only) error = %v", err)
	}
	var dashboardWorkspaceOnly apicontract.TaskDashboardPayload
	decodeResult(t, dashboardWorkspaceOnlyResult, &dashboardWorkspaceOnly)
	if dashboardWorkspaceOnly.Totals.TasksTotal < 1 {
		t.Fatalf("tasks/dashboard workspace-only tasks_total = %d, want >= 1", dashboardWorkspaceOnly.Totals.TasksTotal)
	}

	inboxResult, err := env.call(t, "ext-reader", "tasks/inbox", map[string]any{
		"scope":     taskpkg.ScopeWorkspace,
		"workspace": env.workspaceID,
		"lane":      apicontract.TaskInboxLaneApprovals,
		"limit":     10,
	})
	if err != nil {
		t.Fatalf("Handle(tasks/inbox) error = %v", err)
	}
	var inbox apicontract.TaskInboxPayload
	decodeResult(t, inboxResult, &inbox)
	if inbox.Page.Total < 1 || len(inbox.Groups) == 0 {
		t.Fatalf("tasks/inbox = %#v, want approval item", inbox)
	}
	if !taskInboxContainsTask(inbox.Groups, approvalTask.ID) {
		t.Fatalf("tasks/inbox groups = %#v, want task %q", inbox.Groups, approvalTask.ID)
	}

	inboxWorkspaceOnlyResult, err := env.call(t, "ext-reader", "tasks/inbox", map[string]any{
		"workspace": env.workspaceID,
		"lane":      apicontract.TaskInboxLaneApprovals,
		"limit":     10,
	})
	if err != nil {
		t.Fatalf("Handle(tasks/inbox workspace-only) error = %v", err)
	}
	var inboxWorkspaceOnly apicontract.TaskInboxPayload
	decodeResult(t, inboxWorkspaceOnlyResult, &inboxWorkspaceOnly)
	if !taskInboxContainsTask(inboxWorkspaceOnly.Groups, approvalTask.ID) {
		t.Fatalf("tasks/inbox workspace-only groups = %#v, want task %q", inboxWorkspaceOnly.Groups, approvalTask.ID)
	}

	if _, err := env.tasks.CreateTask(testutil.Context(t), taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: env.workspaceID,
		Title:       "Newest hidden draft",
		Draft:       true,
	}, actor); err != nil {
		t.Fatalf("tasks.CreateTask(draft) error = %v", err)
	}

	listResult, err := env.call(t, "ext-reader", "tasks", map[string]any{
		"workspace": env.workspaceID,
		"limit":     1,
	})
	if err != nil {
		t.Fatalf("Handle(tasks) error = %v", err)
	}
	var listedPage apicontract.TasksResponse
	decodeResult(t, listResult, &listedPage)
	listed := listedPage.Tasks
	if got, want := len(listed), 1; got != want {
		t.Fatalf("len(tasks workspace-only) = %d, want %d", got, want)
	}
	if got, want := listedPage.Page.Total, 3; got != want {
		t.Fatalf("tasks workspace-only page.total = %d, want %d", got, want)
	}
	if got, want := listedPage.Page.Limit, 1; got != want {
		t.Fatalf("tasks workspace-only page.limit = %d, want %d", got, want)
	}
	if !listedPage.Page.HasMore || listedPage.Page.NextCursor == "" {
		t.Fatalf("tasks workspace-only page = %#v, want bounded continuation", listedPage.Page)
	}
	if listed[0].Draft {
		t.Fatalf("tasks workspace-only[0] = %#v, want non-draft item", listed[0])
	}
}

func taskInboxContainsTask(groups []apicontract.TaskInboxLaneGroupPayload, taskID string) bool {
	for _, group := range groups {
		for _, item := range group.Items {
			if item.Task.ID == taskID {
				return true
			}
		}
	}
	return false
}

func TestHostAPIIntegrationTaskReadSurfaceErrorsStayHostFacing(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant("ext-reader", []string{"tasks/get", "tasks/runs/get"}, []string{"task.read"})

	_, err := env.call(t, "ext-reader", "tasks/get", map[string]any{"id": "task-missing"})
	assertRPCErrorCode(t, err, HostAPINotFoundCode)

	_, err = env.call(t, "ext-reader", "tasks/unknown", nil)
	assertRPCErrorCode(t, err, HostAPIMethodNotFoundCode)
}

func TestHostAPIIntegrationBridgesMessagesIngestCreatesRouteAndSession(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ID:            "brg-integration-ingest",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true, IncludeThread: true},
	})
	ctx := env.bridgeContext(t, instance)

	result, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", map[string]any{
		"bridge_instance_id":  instance.ID,
		"scope":               instance.Scope,
		"workspace_id":        instance.WorkspaceID,
		"peer_id":             "peer-1",
		"thread_id":           "thread-1",
		"platform_message_id": "msg-1",
		"received_at":         env.currentTime().Format(time.RFC3339Nano),
		"idempotency_key":     "idem-1",
		"content":             map[string]any{"text": "hello from telegram"},
	})
	if err != nil {
		t.Fatalf("Handle(bridges/messages/ingest) error = %v", err)
	}

	var ingest bridgecontract.BridgesMessagesIngestResult
	decodeResult(t, result, &ingest)
	if ingest.SessionID == "" {
		t.Fatal("bridges/messages/ingest session_id = empty, want non-empty")
	}
	if !ingest.RouteCreated {
		t.Fatal("bridges/messages/ingest route_created = false, want true")
	}

	route, err := env.bridges.ResolveRoute(testutil.Context(t), bridgeRoutingKeyDomain(ingest.RoutingKey))
	if err != nil {
		t.Fatalf("bridges.ResolveRoute() error = %v", err)
	}
	if route.SessionID != ingest.SessionID {
		t.Fatalf("resolved route session_id = %q, want %q", route.SessionID, ingest.SessionID)
	}
}

func TestHostAPIIntegrationBridgesMessagesIngestSupportsSiblingInstancesInOneRuntime(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

	first := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ID:            "brg-integration-multi-a",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	second := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ID:            "brg-integration-multi-b",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContextForInstances(t, first, second)

	result, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", map[string]any{
		"bridge_instance_id":  second.ID,
		"scope":               second.Scope,
		"workspace_id":        second.WorkspaceID,
		"peer_id":             "peer-multi",
		"platform_message_id": "msg-multi",
		"received_at":         env.currentTime().Format(time.RFC3339Nano),
		"idempotency_key":     "idem-multi",
		"content":             map[string]any{"text": "hello from sibling runtime"},
	})
	if err != nil {
		t.Fatalf("Handle(bridges/messages/ingest multi) error = %v", err)
	}

	var ingest bridgecontract.BridgesMessagesIngestResult
	decodeResult(t, result, &ingest)
	if !ingest.RouteCreated {
		t.Fatal("bridges/messages/ingest multi route_created = false, want true")
	}

	route, err := env.bridges.ResolveRoute(testutil.Context(t), bridgeRoutingKeyDomain(ingest.RoutingKey))
	if err != nil {
		t.Fatalf("bridges.ResolveRoute(multi) error = %v", err)
	}
	if got, want := route.BridgeInstanceID, second.ID; got != want {
		t.Fatalf("route.BridgeInstanceID = %q, want %q", got, want)
	}

	firstRoutes, err := env.bridges.ListRoutes(testutil.Context(t), first.ID)
	if err != nil {
		t.Fatalf("bridges.ListRoutes(first) error = %v", err)
	}
	if got := len(firstRoutes); got != 0 {
		t.Fatalf("len(first routes) = %d, want 0", got)
	}
}

func TestHostAPIIntegrationBridgesMessagesIngestDuplicateRetryIsSuppressed(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ID:            "brg-integration-dedup",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, instance)
	params := map[string]any{
		"bridge_instance_id":  instance.ID,
		"scope":               instance.Scope,
		"workspace_id":        instance.WorkspaceID,
		"peer_id":             "peer-1",
		"platform_message_id": "msg-1",
		"received_at":         env.currentTime().Format(time.RFC3339Nano),
		"idempotency_key":     "idem-1",
		"content":             map[string]any{"text": "retry me"},
	}

	first, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", params)
	if err != nil {
		t.Fatalf("first Handle(bridges/messages/ingest) error = %v", err)
	}
	var firstResult bridgecontract.BridgesMessagesIngestResult
	decodeResult(t, first, &firstResult)

	env.advanceTime(2 * time.Minute)

	second, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", params)
	if err != nil {
		t.Fatalf("retry Handle(bridges/messages/ingest) error = %v", err)
	}
	var secondResult bridgecontract.BridgesMessagesIngestResult
	decodeResult(t, second, &secondResult)

	routes, err := env.bridges.ListRoutes(testutil.Context(t), instance.ID)
	if err != nil {
		t.Fatalf("bridges.ListRoutes() error = %v", err)
	}
	if got := len(routes); got != 1 {
		t.Fatalf("len(routes) = %d, want 1", got)
	}
	if got := env.driver.promptCount(); got != 1 {
		t.Fatalf("driver.promptCount() = %d, want 1", got)
	}
	if secondResult.SessionID != firstResult.SessionID {
		t.Fatalf("retry session_id = %q, want %q", secondResult.SessionID, firstResult.SessionID)
	}
}

func TestHostAPIIntegrationBridgesMessagesIngestRejectsNonOwnedInstance(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

	owned := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ID:            "brg-integration-owned",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	foreign := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ID:            "brg-integration-non-owned",
		ExtensionName: "discord-adapter",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, owned)

	_, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", map[string]any{
		"bridge_instance_id":  foreign.ID,
		"scope":               foreign.Scope,
		"workspace_id":        foreign.WorkspaceID,
		"peer_id":             "peer-foreign",
		"platform_message_id": "msg-foreign",
		"received_at":         env.currentTime().Format(time.RFC3339Nano),
		"idempotency_key":     "idem-foreign",
		"content":             map[string]any{"text": "hello"},
	})
	assertRPCErrorCode(t, err, HostAPINotFoundCode)
	assertErrorContains(t, err, foreign.ID)
}

func TestHostAPIIntegrationBridgesInstancesReportStatePublishesAuthRequired(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant(
		"telegram-adapter",
		[]string{"bridges/instances/report_state", "bridges/instances/get"},
		[]string{"bridge.write", "bridge.read"},
	)

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ID:            "brg-integration-state",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, instance)

	result, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/report_state", map[string]any{
		"bridge_instance_id": instance.ID,
		"status":             "auth_required",
		"degradation": map[string]any{
			"reason":  "auth_failed",
			"message": "token expired",
		},
	})
	if err != nil {
		t.Fatalf("Handle(bridges/instances/report_state) error = %v", err)
	}

	var updated bridgecontract.BridgeInstance
	decodeResult(t, result, &updated)
	if updated.Status != bridgecontract.BridgeStatusAuthRequired {
		t.Fatalf(
			"bridges/instances/report_state status = %q, want %q",
			updated.Status,
			bridgecontract.BridgeStatusAuthRequired,
		)
	}
	if updated.Degradation == nil || updated.Degradation.Reason != bridgecontract.BridgeDegradationReasonAuthFailed {
		t.Fatalf("bridges/instances/report_state degradation = %#v, want auth_failed", updated.Degradation)
	}

	fetched, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/get", map[string]any{
		"bridge_instance_id": instance.ID,
	})
	if err != nil {
		t.Fatalf("Handle(bridges/instances/get) error = %v", err)
	}
	var loaded bridgecontract.BridgeInstance
	decodeResult(t, fetched, &loaded)
	if loaded.Status != bridgecontract.BridgeStatusAuthRequired {
		t.Fatalf("bridges/instances/get status = %q, want %q", loaded.Status, bridgecontract.BridgeStatusAuthRequired)
	}
	if loaded.Degradation == nil || loaded.Degradation.Reason != bridgecontract.BridgeDegradationReasonAuthFailed {
		t.Fatalf("bridges/instances/get degradation = %#v, want auth_failed", loaded.Degradation)
	}
}

func TestHostAPIIntegrationBridgesInstancesListAndGetReturnOwnedInstances(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/instances/list", "bridges/instances/get"}, []string{"bridge.read"})

	first := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ID:            "brg-integration-owned-a",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	second := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ID:            "brg-integration-owned-b",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	_ = env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ID:            "brg-integration-foreign",
		ExtensionName: "discord-adapter",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})

	ctx := env.bridgeContextForInstances(t, first, second)

	listedResult, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/list", nil)
	if err != nil {
		t.Fatalf("Handle(bridges/instances/list) error = %v", err)
	}

	var listed []bridgecontract.BridgeInstance
	decodeResult(t, listedResult, &listed)
	if got := len(listed); got != 2 {
		t.Fatalf("len(bridges/instances/list) = %d, want 2", got)
	}
	if got, want := []string{
		listed[0].ID,
		listed[1].ID,
	}, []string{
		first.ID,
		second.ID,
	}; got[0] != want[0] ||
		got[1] != want[1] {
		t.Fatalf("bridges/instances/list ids = %#v, want %#v", got, want)
	}

	fetchedResult, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/get", map[string]any{
		"bridge_instance_id": second.ID,
	})
	if err != nil {
		t.Fatalf("Handle(bridges/instances/get) error = %v", err)
	}

	var fetched bridgecontract.BridgeInstance
	decodeResult(t, fetchedResult, &fetched)
	if got, want := fetched.ID, second.ID; got != want {
		t.Fatalf("bridges/instances/get id = %q, want %q", got, want)
	}
}

func TestHostAPIIntegrationBridgesMessagesIngestConcurrentSameRoutingKeyUsesOneRouteAndSession(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.useSessionsWithoutObserver(t)
	env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ID:            "brg-integration-concurrent",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, instance)

	type ingestResult struct {
		result bridgecontract.BridgesMessagesIngestResult
		err    error
	}

	results := make([]ingestResult, 2)
	done := make(chan struct{}, len(results))
	for idx := range results {
		idx := idx
		go func() {
			defer func() { done <- struct{}{} }()
			result, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", map[string]any{
				"bridge_instance_id":  instance.ID,
				"scope":               instance.Scope,
				"workspace_id":        instance.WorkspaceID,
				"peer_id":             "peer-1",
				"platform_message_id": fmt.Sprintf("msg-%d", idx),
				"received_at":         env.currentTime().Format(time.RFC3339Nano),
				"idempotency_key":     fmt.Sprintf("idem-%d", idx),
				"content":             map[string]any{"text": fmt.Sprintf("hello-%d", idx)},
			})
			if err != nil {
				results[idx].err = err
				return
			}
			if err := decodeIntegrationResult(result, &results[idx].result); err != nil {
				results[idx].err = err
			}
		}()
	}
	for range results {
		<-done
	}

	for idx, result := range results {
		if result.err != nil {
			t.Fatalf("ingest[%d] error = %v", idx, result.err)
		}
	}

	routes, err := env.bridges.ListRoutes(testutil.Context(t), instance.ID)
	if err != nil {
		t.Fatalf("bridges.ListRoutes() error = %v", err)
	}
	if got := len(routes); got != 1 {
		t.Fatalf("len(routes) = %d, want 1", got)
	}

	sessions, err := env.sessions.ListAll(testutil.Context(t))
	if err != nil {
		t.Fatalf("sessions.ListAll() error = %v", err)
	}
	if got := len(sessions); got != 1 {
		t.Fatalf("len(sessions) = %d, want 1", got)
	}
	if results[0].result.SessionID != results[1].result.SessionID {
		t.Fatalf("session IDs = %q and %q, want same session", results[0].result.SessionID, results[1].result.SessionID)
	}
}

func decodeIntegrationResult(result any, target any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("json.Marshal(result): %w", err)
	}
	if err := json.Unmarshal(encoded, target); err != nil {
		return fmt.Errorf("json.Unmarshal(result): %w", err)
	}
	return nil
}

func TestHostAPIIntegrationUnauthorizedExtensionIsDeniedForEveryMethod(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant("ext-denied", nil, nil)

	session := env.createSession(t)
	tests := []struct {
		method string
		params any
	}{
		{method: "sessions/list", params: map[string]any{"workspace": env.workspaceID}},
		{method: "sessions/create", params: map[string]any{"agent": "coder", "workspace": env.workspaceID}},
		{
			method: "sessions/prompt",
			params: map[string]any{"workspace_id": env.workspaceID, "session_id": session.ID, "message": "hello"},
		},
		{method: "sessions/stop", params: map[string]any{"workspace_id": env.workspaceID, "session_id": session.ID}},
		{method: "sessions/status", params: map[string]any{"workspace_id": env.workspaceID, "session_id": session.ID}},
		{
			method: "sessions/events",
			params: map[string]any{"workspace_id": env.workspaceID, "session_id": session.ID, "limit": 1},
		},
		{method: "memory/recall", params: map[string]any{"query": "needle"}},
		{method: "memory/store", params: map[string]any{"key": "note", "content": "body"}},
		{method: "memory/forget", params: map[string]any{"key": "note"}},
		{method: "observe/health", params: nil},
		{method: "logs/list", params: map[string]any{"session_id": session.ID, "limit": 1}},
		{method: "skills/list", params: map[string]any{"workspace": env.workspaceID}},
		{method: "automation/jobs", params: map[string]any{"scope": "workspace", "workspace_id": env.workspaceID}},
		{method: "automation/jobs/create", params: map[string]any{
			"name":         "integration-job",
			"scope":        "workspace",
			"workspace_id": env.workspaceID,
			"agent_name":   "coder",
			"prompt":       "run integration job",
			"schedule": map[string]any{
				"mode":     "every",
				"interval": "5m",
			},
		}},
		{method: "automation/triggers/fire", params: map[string]any{
			"event":        "ext.github.push",
			"scope":        "workspace",
			"workspace_id": env.workspaceID,
		}},
		{method: "bridges/messages/ingest", params: map[string]any{
			"bridge_instance_id":  "brg-1",
			"scope":               "workspace",
			"workspace_id":        env.workspaceID,
			"peer_id":             "peer-1",
			"platform_message_id": "msg-1",
			"received_at":         env.currentTime().Format(time.RFC3339Nano),
			"idempotency_key":     "idem-1",
		}},
		{method: "bridges/instances/list", params: nil},
		{method: "bridges/instances/get", params: map[string]any{"bridge_instance_id": "brg-1"}},
		{
			method: "bridges/instances/report_state",
			params: map[string]any{"bridge_instance_id": "brg-1", "status": "ready"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.method, func(t *testing.T) {
			_, err := env.call(t, "ext-denied", tt.method, tt.params)
			assertCapabilityDenied(t, err, tt.method)
		})
	}
}

func TestHostAPIIntegrationAutomationJobCreateReturnsCreatedJobPayload(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant("ext-automation", []string{"automation/jobs/create"}, []string{"automation.write"})

	result, err := env.call(t, "ext-automation", "automation/jobs/create", map[string]any{
		"name":         "nightly-report",
		"scope":        "workspace",
		"workspace_id": env.workspaceID,
		"agent_name":   "coder",
		"prompt":       "Generate nightly report",
		"schedule": map[string]any{
			"mode":     "every",
			"interval": "5m",
		},
	})
	if err != nil {
		t.Fatalf("Handle(automation/jobs/create) error = %v", err)
	}

	var created automationpkg.Job
	decodeResult(t, result, &created)
	if created.ID == "" {
		t.Fatal("automation/jobs/create id = empty, want non-empty")
	}
	if created.Name != "nightly-report" {
		t.Fatalf("automation/jobs/create name = %q, want nightly-report", created.Name)
	}
	if created.Source != automationpkg.JobSourceDynamic {
		t.Fatalf("automation/jobs/create source = %q, want %q", created.Source, automationpkg.JobSourceDynamic)
	}
}

func TestHostAPIIntegrationAutomationTriggerFireDispatchesThroughTriggerEngine(t *testing.T) {
	env := newHostAPITestEnv(t)
	env.grant(
		"ext-automation",
		[]string{"automation/triggers/create", "automation/triggers/fire"},
		[]string{"automation.write"},
	)

	createResult, err := env.call(t, "ext-automation", "automation/triggers/create", map[string]any{
		"name":         "review-push",
		"scope":        "workspace",
		"workspace_id": env.workspaceID,
		"agent_name":   "coder",
		"event":        "ext.github.push",
		"prompt":       `Review push to {{ index .Data "repo" }} by {{ index .Data "author" }}`,
	})
	if err != nil {
		t.Fatalf("Handle(automation/triggers/create) error = %v", err)
	}

	var trigger automationpkg.Trigger
	decodeResult(t, createResult, &trigger)
	if trigger.ID == "" {
		t.Fatal("automation/triggers/create id = empty, want non-empty")
	}

	fireResult, err := env.call(t, "ext-automation", "automation/triggers/fire", map[string]any{
		"event":        "ext.github.push",
		"scope":        "workspace",
		"workspace_id": env.workspaceID,
		"payload": map[string]any{
			"repo":   "acme/api",
			"author": "dev@acme.com",
		},
	})
	if err != nil {
		t.Fatalf("Handle(automation/triggers/fire) error = %v", err)
	}

	var result automationpkg.TriggerResult
	decodeResult(t, fireResult, &result)
	if result.Matched != 1 {
		t.Fatalf("automation/triggers/fire matched = %d, want 1", result.Matched)
	}
	if len(result.Runs) != 1 {
		t.Fatalf("automation/triggers/fire runs = %d, want 1", len(result.Runs))
	}

	prompts := env.driver.promptCalls()
	if len(prompts) == 0 {
		t.Fatal("driver prompt calls = 0, want trigger dispatch prompt")
	}
	if got, want := prompts[len(prompts)-1].Message, "Review push to acme/api by dev@acme.com"; got != want {
		t.Fatalf("last prompt message = %q, want %q", got, want)
	}
}

func TestHostAPIIntegrationAutomationPreFireHookMutatesPrompt(t *testing.T) {
	hooks := hookspkg.NewHooks(
		hookspkg.WithNativeDeclarations([]hookspkg.HookDecl{{
			Name:         "mutate-automation-prompt",
			Event:        hookspkg.HookAutomationJobPreFire,
			Mode:         hookspkg.HookModeSync,
			ExecutorKind: hookspkg.HookExecutorNative,
		}}),
		hookspkg.WithExecutorResolver(func(decl hookspkg.HookDecl) (hookspkg.Executor, error) {
			return hookspkg.NewTypedNativeExecutor(
				func(_ context.Context, _ hookspkg.RegisteredHook, payload hookspkg.AutomationJobPreFirePayload) (hookspkg.AutomationFirePatch, error) {
					prompt := payload.Prompt + " with hook mutation"
					return hookspkg.AutomationFirePatch{Prompt: &prompt}, nil
				},
			), nil
		}),
	)
	if err := hooks.Rebuild(t.Context()); err != nil {
		t.Fatalf("hooks.Rebuild() error = %v", err)
	}
	t.Cleanup(hooks.Close)

	env := newHostAPITestEnv(t, withHostAPIHooks(hooks))
	env.grant(
		"ext-automation",
		[]string{"automation/jobs/create", "automation/jobs/trigger"},
		[]string{"automation.write"},
	)

	createResult, err := env.call(t, "ext-automation", "automation/jobs/create", map[string]any{
		"name":         "hooked-job",
		"scope":        "workspace",
		"workspace_id": env.workspaceID,
		"agent_name":   "coder",
		"prompt":       "Original prompt",
		"schedule": map[string]any{
			"mode":     "every",
			"interval": "5m",
		},
	})
	if err != nil {
		t.Fatalf("Handle(automation/jobs/create) error = %v", err)
	}

	var created automationpkg.Job
	decodeResult(t, createResult, &created)

	if _, err := env.call(
		t,
		"ext-automation",
		"automation/jobs/trigger",
		map[string]any{"id": created.ID},
	); err != nil {
		t.Fatalf("Handle(automation/jobs/trigger) error = %v", err)
	}

	prompts := env.driver.promptCalls()
	if len(prompts) == 0 {
		t.Fatal("driver prompt calls = 0, want job dispatch prompt")
	}
	if got, want := prompts[len(prompts)-1].Message, "Original prompt with hook mutation"; got != want {
		t.Fatalf("last prompt message = %q, want %q", got, want)
	}
}
