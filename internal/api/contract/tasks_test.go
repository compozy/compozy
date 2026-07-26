package contract

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
)

func TestTaskContractsMarshalExpandedTaskReadModels(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 17, 12, 0, 0, 0, time.UTC)
	lastActivity := now.Add(15 * time.Minute)
	claimedAt := now.Add(2 * time.Minute)
	startedAt := now.Add(3 * time.Minute)
	liveParticipation := contractTestLiveParticipation("ws-1", "ops")

	summary := TaskSummaryPayload{
		ID:                           "task-1",
		Identifier:                   "TSK-001",
		Scope:                        taskpkg.ScopeWorkspace,
		WorkspaceID:                  "ws-1",
		ParentTaskID:                 "parent-1",
		ResolvedNetworkParticipation: participation.CloneSpec(liveParticipation),
		Title:                        "Review contract coverage",
		Priority:                     taskpkg.PriorityHigh,
		MaxAttempts:                  4,
		Status:                       taskpkg.TaskStatusBlocked,
		ApprovalPolicy:               taskpkg.ApprovalPolicyManual,
		ApprovalState:                taskpkg.ApprovalStatePending,
		Draft:                        true,
		Owner:                        &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "alice"},
		CreatedBy:                    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "alice"},
		Origin:                       taskpkg.Origin{Kind: taskpkg.OriginKindWeb, Ref: "agh-web"},
		CreatedAt:                    now,
		UpdatedAt:                    now,
		ChildCount:                   2,
		DependencyCount:              1,
		Dependencies: []TaskDependencyReferencePayload{
			{
				TaskID:          "task-1",
				DependsOnTaskID: "task-2",
				Kind:            taskpkg.DependencyKindBlocks,
				CreatedAt:       now,
				DependsOn: TaskReferencePayload{
					ID:          "task-2",
					Identifier:  "TSK-002",
					Title:       "Ship inbox model",
					Status:      taskpkg.TaskStatusReady,
					Priority:    taskpkg.PriorityUrgent,
					Owner:       &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "reviewers"},
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-1",
				},
			},
		},
		ActiveRun: &TaskRunSummaryPayload{
			ID:          "run-1",
			TaskID:      "task-1",
			Status:      taskpkg.TaskRunStatusRunning,
			Attempt:     2,
			MaxAttempts: 4,
			SessionID:   "session-1",
			ClaimedBy:   &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-1"},
			QueuedAt:    now,
			ClaimedAt:   &claimedAt,
			StartedAt:   &startedAt,
		},
		LastActivityAt: &lastActivity,
	}

	detail := TaskDetailPayload{
		Summary: summary,
		Task: TaskPayload{
			ID:                           "task-1",
			Identifier:                   "TSK-001",
			Scope:                        taskpkg.ScopeWorkspace,
			WorkspaceID:                  "ws-1",
			ParentTaskID:                 "parent-1",
			ResolvedNetworkParticipation: participation.CloneSpec(liveParticipation),
			Title:                        "Review contract coverage",
			Description:                  "Expand the task contract surface",
			Priority:                     taskpkg.PriorityHigh,
			MaxAttempts:                  4,
			Status:                       taskpkg.TaskStatusBlocked,
			ApprovalPolicy:               taskpkg.ApprovalPolicyManual,
			ApprovalState:                taskpkg.ApprovalStatePending,
			Draft:                        true,
			Owner:                        &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "alice"},
			CreatedBy:                    taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "alice"},
			Origin:                       taskpkg.Origin{Kind: taskpkg.OriginKindWeb, Ref: "agh-web"},
			CreatedAt:                    now,
			UpdatedAt:                    now,
			Metadata:                     json.RawMessage(`{"ticket":"TASK-07"}`),
		},
		Children: []TaskSummaryPayload{summary},
		Dependencies: []TaskDependencyPayload{
			{
				TaskID:          "task-1",
				DependsOnTaskID: "task-2",
				Kind:            taskpkg.DependencyKindBlocks,
				CreatedAt:       now,
			},
		},
		DependencyReferences: summary.Dependencies,
		Runs: []TaskRunPayload{
			{
				ID:      "run-1",
				TaskID:  "task-1",
				Status:  taskpkg.TaskRunStatusRunning,
				Attempt: 2,
				ClaimedBy: &taskpkg.ActorIdentity{
					Kind: taskpkg.ActorKindAgentSession,
					Ref:  "sess-1",
				},
				SessionID:                    "session-1",
				Origin:                       taskpkg.Origin{Kind: taskpkg.OriginKindAgentSession, Ref: "sess-1"},
				IdempotencyKey:               "idem-1",
				ResolvedNetworkParticipation: participation.CloneSpec(liveParticipation),
				QueuedAt:                     now,
				ClaimedAt:                    &claimedAt,
				StartedAt:                    &startedAt,
			},
		},
		Events: []TaskEventPayload{
			{
				ID:        "evt-1",
				TaskID:    "task-1",
				RunID:     "run-1",
				EventType: "task.run_started",
				Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-1"},
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindAgentSession, Ref: "sess-1"},
				Payload:   json.RawMessage(`{"step":1}`),
				Timestamp: now,
			},
		},
	}

	t.Run("Should marshal task detail payload keys", func(t *testing.T) {
		t.Parallel()

		object := marshalObject(t, TaskDetailResponse{Task: detail})
		taskObject := nestedObject(t, object, "task")
		assertObjectKeys(
			t,
			taskObject,
			"summary",
			"task",
			"children",
			"dependencies",
			"dependency_references",
			"runs",
			"events",
		)
		assertParticipationJSON(t, nestedObject(t, taskObject, "summary"), liveParticipation)
		nestedTask := nestedObject(t, taskObject, "task")
		assertObjectKeys(t, nestedTask, "draft")
		assertParticipationJSON(t, nestedTask, liveParticipation)
		firstRun := firstObjectFromArray(t, taskObject, "runs")
		assertParticipationJSON(t, firstRun, liveParticipation)
	})

	t.Run("Should marshal summary dependency reference keys", func(t *testing.T) {
		t.Parallel()

		object := marshalObject(t, TaskDetailResponse{Task: detail})
		taskObject := nestedObject(t, object, "task")
		summaryObject := nestedObject(t, taskObject, "summary")
		assertObjectKeys(
			t,
			summaryObject,
			"priority",
			"max_attempts",
			"approval_policy",
			"approval_state",
			"draft",
			"child_count",
			"dependency_count",
			"dependencies",
			"active_run",
			"last_activity_at",
		)

		dependency := firstObjectFromArray(t, summaryObject, "dependencies")
		assertObjectKeys(t, dependency, "depends_on")

		dependsOn := nestedObject(t, dependency, "depends_on")
		assertObjectKeys(t, dependsOn, "id", "identifier", "title", "status", "priority", "owner", "scope")
	})
}

func TestTaskContractsMarshalLiveDashboardAndInboxPayloads(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 17, 12, 0, 0, 0, time.UTC)
	lastActivity := now.Add(10 * time.Minute)
	claimedAt := now.Add(2 * time.Minute)

	runDetail := TaskRunDetailResponse{
		Run: TaskRunDetailPayload{
			Run: TaskRunPayload{
				ID:        "run-1",
				TaskID:    "task-1",
				Status:    taskpkg.TaskRunStatusRunning,
				Attempt:   1,
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindAgentSession, Ref: "sess-1"},
				QueuedAt:  now,
				ClaimedAt: &claimedAt,
			},
			Task: &TaskReferencePayload{
				ID:          "task-1",
				Identifier:  "TSK-001",
				Title:       "Review contract coverage",
				Status:      taskpkg.TaskStatusInProgress,
				Priority:    taskpkg.PriorityHigh,
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-1",
			},
			Session: &TaskRunSessionPayload{
				SessionID:   "session-1",
				WorkspaceID: "ws-1",
				AgentName:   "codex",
				Name:        "task-run",
				State:       "active",
				CreatedAt:   now,
				UpdatedAt:   now,
			},
			Summary: TaskRunOperationalSummaryPayload{
				LastActivityAt: lastActivity,
				LastEventType:  "task.run_started",
			},
		},
	}

	tree := TaskTreeResponse{
		Tree: TaskTreePayload{
			Root: TaskTreeNodePayload{
				Task: TaskReferencePayload{
					ID:          "task-1",
					Identifier:  "TSK-001",
					Title:       "Root task",
					Status:      taskpkg.TaskStatusInProgress,
					Priority:    taskpkg.PriorityHigh,
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-1",
				},
				Depth:          0,
				ChildCount:     1,
				LastActivityAt: lastActivity,
			},
			Descendants: []TaskTreeNodePayload{
				{
					Task: TaskReferencePayload{
						ID:          "task-2",
						Identifier:  "TSK-002",
						Title:       "Child task",
						Status:      taskpkg.TaskStatusReady,
						Priority:    taskpkg.PriorityMedium,
						Scope:       taskpkg.ScopeWorkspace,
						WorkspaceID: "ws-1",
					},
					ParentTaskID:   "task-1",
					Depth:          1,
					LastActivityAt: lastActivity,
				},
			},
		},
	}

	timeline := TaskTimelineResponse{
		Timeline: []TaskTimelineItemPayload{
			{
				Sequence:  7,
				EventID:   "evt-7",
				EventType: "task.run_started",
				Task: TaskReferencePayload{
					ID:          "task-1",
					Identifier:  "TSK-001",
					Title:       "Review contract coverage",
					Status:      taskpkg.TaskStatusInProgress,
					Priority:    taskpkg.PriorityHigh,
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-1",
				},
				Run: &TaskRunSummaryPayload{
					ID:          "run-1",
					TaskID:      "task-1",
					Status:      taskpkg.TaskRunStatusRunning,
					Attempt:     1,
					MaxAttempts: 3,
					QueuedAt:    now,
				},
				Actor:     taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-1"},
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindAgentSession, Ref: "sess-1"},
				Payload:   json.RawMessage(`{"step":2}`),
				Timestamp: lastActivity,
			},
		},
	}

	dashboard := TaskDashboardResponse{
		Dashboard: TaskDashboardPayload{
			Totals: TaskDashboardTotalsPayload{TasksTotal: 3, RunsTotal: 2, ActiveRuns: 1},
			Cards: TaskDashboardCardsPayload{
				InProgress: TaskDashboardInProgressCardPayload{Tasks: 1, ActiveRuns: 1, HealthStatus: "healthy"},
				Blocked:    TaskDashboardBlockedCardPayload{Tasks: 1, AwaitingApproval: 1, HealthStatus: "warning"},
				Failed:     TaskDashboardFailedCardPayload{Tasks: 1, FailedRuns: 1, HealthStatus: "warning"},
				Latency: TaskDashboardLatencyCardPayload{
					ClaimLatencyMillis: TaskLatencyMetricPayload{Samples: 1, AverageMillis: 10, MaximumMillis: 10},
					StartLatencyMillis: TaskLatencyMetricPayload{Samples: 1, AverageMillis: 20, MaximumMillis: 20},
				},
			},
			StatusBreakdown: []TaskDashboardStatusBreakdownPayload{
				{Status: taskpkg.TaskStatusInProgress, Count: 1, SharePercent: 33},
			},
			Queue:      TaskDashboardQueuePayload{Total: 1, BacklogStatus: "ok", OldestQueuedAt: now},
			Health:     TaskDashboardHealthPayload{Status: "healthy", StuckRuns: 0, ActiveOrphanRuns: 0},
			ActiveRuns: TaskDashboardActiveRunsPayload{Total: 1, Running: 1},
			Freshness: TaskDashboardFreshnessPayload{
				ObservedAt:       now,
				LatestActivityAt: lastActivity,
				Status:           "fresh",
			},
		},
	}

	lastSeen := lastActivity.Add(-time.Minute)
	inbox := TaskInboxResponse{
		Inbox: TaskInboxPayload{
			UnreadTotal:   1,
			ArchivedTotal: 0,
			Page:          CountedCursorPagePayload{Total: 2, Limit: 10},
			Groups: []TaskInboxLaneGroupPayload{
				{
					Lane:        TaskInboxLaneApprovals,
					Count:       1,
					UnreadCount: 1,
					Items: []TaskInboxItemPayload{
						{
							Task: TaskInboxTaskPayload{
								ID:          "task-1",
								Identifier:  "TSK-001",
								Title:       "Approve task",
								Status:      taskpkg.TaskStatusBlocked,
								Priority:    taskpkg.PriorityUrgent,
								Scope:       taskpkg.ScopeWorkspace,
								WorkspaceID: "ws-1",
							},
							Lane:             TaskInboxLaneApprovals,
							ApprovalPolicy:   taskpkg.ApprovalPolicyManual,
							ApprovalState:    taskpkg.ApprovalStatePending,
							BlockingReason:   "awaiting_approval",
							LatestActivityAt: lastActivity,
							Run: &TaskCatalogRunPayload{
								ID:          "run-1",
								TaskID:      "task-1",
								Status:      taskpkg.TaskRunStatusClaimed,
								Attempt:     1,
								MaxAttempts: 3,
								QueuedAt:    now,
							},
							Triage: TaskTriageStatePayload{
								TaskID:             "task-1",
								Actor:              taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "alice"},
								Read:               false,
								Archived:           false,
								Dismissed:          false,
								LastSeenActivityAt: &lastSeen,
								UpdatedAt:          lastActivity,
							},
						},
					},
				},
			},
		},
	}

	t.Run("Should marshal run detail payload keys", func(t *testing.T) {
		t.Parallel()

		runDetailObject := marshalObject(t, runDetail)
		assertObjectKeys(t, nestedObject(t, runDetailObject, "run"), "run", "task", "session", "summary")
	})

	t.Run("Should omit the optional task member for a taskless run detail", func(t *testing.T) {
		t.Parallel()

		taskless := runDetail
		taskless.Run.Task = nil
		runDetailObject := marshalObject(t, taskless)
		runObject := nestedObject(t, runDetailObject, "run")
		assertObjectKeys(t, runObject, "run", "session", "summary")
		if _, ok := runObject["task"]; ok {
			t.Fatalf("taskless run detail contains optional task member: %v", runObject)
		}
	})

	t.Run("Should marshal task tree payload keys", func(t *testing.T) {
		t.Parallel()

		treeObject := marshalObject(t, tree)
		assertObjectKeys(t, nestedObject(t, treeObject, "tree"), "root", "descendants")
	})

	t.Run("Should marshal timeline item payload keys", func(t *testing.T) {
		t.Parallel()

		timelineObject := marshalObject(t, timeline)
		firstTimelineItem := firstObjectFromArray(t, timelineObject, "timeline")
		assertObjectKeys(
			t,
			firstTimelineItem,
			"sequence",
			"event_id",
			"task",
			"run",
			"event_type",
			"actor",
			"origin",
			"payload",
			"timestamp",
		)
	})

	t.Run("Should marshal dashboard payload keys", func(t *testing.T) {
		t.Parallel()

		dashboardObject := marshalObject(t, dashboard)
		assertObjectKeys(
			t,
			nestedObject(t, dashboardObject, "dashboard"),
			"totals",
			"cards",
			"status_breakdown",
			"queue",
			"health",
			"active_runs",
			"freshness",
		)
	})

	t.Run("Should marshal inbox payload keys", func(t *testing.T) {
		t.Parallel()

		inboxObject := marshalObject(t, inbox)
		inboxRoot := nestedObject(t, inboxObject, "inbox")
		assertObjectKeys(t, inboxRoot, "unread_total", "archived_total", "groups", "page", "facets")
		assertObjectKeys(t, nestedObject(t, inboxRoot, "page"), "has_more", "total", "limit")
		assertObjectKeys(t, nestedObject(t, inboxRoot, "facets"), "statuses", "priorities")
		firstGroup := firstObjectFromArray(t, inboxRoot, "groups")
		assertObjectKeys(t, firstGroup, "lane", "count", "unread_count", "items")
		firstItem := firstObjectFromArray(t, firstGroup, "items")
		assertObjectKeys(
			t,
			firstItem,
			"task",
			"lane",
			"approval_policy",
			"approval_state",
			"blocking_reason",
			"latest_activity_at",
			"run",
			"triage",
		)
	})
}

func TestUpdateTaskRequestHasChangesIncludesExpandedFields(t *testing.T) {
	t.Parallel()

	t.Run("Should report no changes for an empty request", func(t *testing.T) {
		t.Parallel()

		if (UpdateTaskRequest{}).HasChanges() {
			t.Fatal("HasChanges() = true, want false for empty request")
		}
	})

	t.Run("Should report changes for a priority patch", func(t *testing.T) {
		t.Parallel()

		priority := taskpkg.PriorityUrgent
		if !(UpdateTaskRequest{Priority: &priority}).HasChanges() {
			t.Fatal("HasChanges() = false, want true when priority changes")
		}
	})

	t.Run("Should report changes for a max attempts patch", func(t *testing.T) {
		t.Parallel()

		maxAttempts := 5
		if !(UpdateTaskRequest{MaxAttempts: &maxAttempts}).HasChanges() {
			t.Fatal("HasChanges() = false, want true when max_attempts changes")
		}
	})

	t.Run("Should report changes for an approval policy patch", func(t *testing.T) {
		t.Parallel()

		approvalPolicy := taskpkg.ApprovalPolicyManual
		if !(UpdateTaskRequest{ApprovalPolicy: &approvalPolicy}).HasChanges() {
			t.Fatal("HasChanges() = false, want true when approval_policy changes")
		}
	})
}

func marshalObject(t *testing.T, value any) map[string]any {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return object
}

func nestedObject(t *testing.T, object map[string]any, key string) map[string]any {
	t.Helper()

	value, ok := object[key]
	if !ok {
		t.Fatalf("missing key %q in object %v", key, object)
	}
	nested, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("key %q = %T, want map[string]any", key, value)
	}
	return nested
}

func firstObjectFromArray(t *testing.T, object map[string]any, key string) map[string]any {
	t.Helper()

	value, ok := object[key]
	if !ok {
		t.Fatalf("missing key %q in object %v", key, object)
	}
	items, ok := value.([]any)
	if !ok {
		t.Fatalf("key %q = %T, want []any", key, value)
	}
	if len(items) == 0 {
		t.Fatalf("key %q array is empty", key)
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("key %q first item = %T, want map[string]any", key, items[0])
	}
	return item
}

func assertObjectKeys(t *testing.T, object map[string]any, keys ...string) {
	t.Helper()

	for _, key := range keys {
		if _, ok := object[key]; !ok {
			t.Fatalf("missing key %q in object %v", key, object)
		}
	}
}

func contractTestLiveParticipation(workspaceID, channelID string) participation.Spec {
	return participation.Spec{
		Version:         participation.SpecVersion,
		Mode:            participation.ModeLive,
		WorkspaceID:     workspaceID,
		ChannelStrategy: participation.StrategyNamed,
		ChannelID:       channelID,
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
}

func assertParticipationJSON(t *testing.T, object map[string]any, want participation.Spec) {
	t.Helper()

	resolved := nestedObject(t, object, "resolved_network_participation")
	if got := resolved["channel_id"]; got != want.ChannelID {
		t.Fatalf("resolved participation channel_id = %#v, want %q", got, want.ChannelID)
	}
	if got := resolved["source"]; got != string(want.Source) {
		t.Fatalf("resolved participation source = %#v, want %q", got, want.Source)
	}
	bounds := nestedObject(t, resolved, "bounds")
	if got := bounds["max_wakes"]; got != float64(want.Bounds.MaxWakes) {
		t.Fatalf("resolved participation max_wakes = %#v, want %d", got, want.Bounds.MaxWakes)
	}
}
