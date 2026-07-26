package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/api/testutil"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestExpandedTaskPayloadBuildersPreserveLiveAndAggregateFields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 17, 12, 0, 0, 0, time.UTC)
	lastActivity := now.Add(-time.Minute)
	lastSeen := now.Add(-2 * time.Minute)
	toolCalls := int64(3)
	totalCost := 1.75
	currency := "USD"
	liveSpec := testLiveParticipation("ws-alpha", "builders")

	summary := taskpkg.Summary{
		ID:              "task-1",
		Identifier:      "TASK-1",
		Scope:           taskpkg.ScopeWorkspace,
		WorkspaceID:     "ws-alpha",
		ParentTaskID:    "task-root",
		Title:           "Review handlers",
		Priority:        taskpkg.PriorityHigh,
		MaxAttempts:     4,
		Status:          taskpkg.TaskStatusBlocked,
		ApprovalPolicy:  taskpkg.ApprovalPolicyManual,
		ApprovalState:   taskpkg.ApprovalStatePending,
		Draft:           false,
		Owner:           &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "reviewers"},
		LatestEventSeq:  17,
		EffectivePaused: true,
		PausedByTaskID:  "task-root",
		CreatedBy:       taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user-1"},
		Origin:          taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.create"},
		CreatedAt:       now.Add(-2 * time.Hour),
		UpdatedAt:       now,
		ChildCount:      2,
		DependencyCount: 1,
		Dependencies: []taskpkg.DependencyReference{{
			TaskID:          "task-1",
			DependsOnTaskID: "task-blocker",
			Kind:            taskpkg.DependencyKindBlocks,
			CreatedAt:       now.Add(-time.Hour),
			DependsOn: taskpkg.Reference{
				ID:             "task-blocker",
				Identifier:     "TASK-2",
				Title:          "Blocked task",
				Status:         taskpkg.TaskStatusReady,
				Priority:       taskpkg.PriorityUrgent,
				Owner:          &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "alice"},
				Scope:          taskpkg.ScopeWorkspace,
				WorkspaceID:    "ws-alpha",
				LatestEventSeq: 11,
			},
		}},
		ActiveRun: &taskpkg.RunSummary{
			ID:                           "run-1",
			TaskID:                       "task-1",
			Status:                       taskpkg.TaskRunStatusRunning,
			Attempt:                      2,
			RecoveryCount:                1,
			MaxAttempts:                  4,
			SessionID:                    "sess-1",
			ResolvedNetworkParticipation: &liveSpec,
			ClaimTokenHash:               "sha256:catalog-verifier",
			DesignationGroupID:           "designation-group",
			Designation: &taskpkg.RunDesignationSummary{
				Index: 2,
				Brief: "private fan-out brief",
			},
			ClaimedBy: &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user-1"},
			QueuedAt:  now.Add(-10 * time.Minute),
			StartedAt: now.Add(-5 * time.Minute),
		},
		LastActivityAt: lastActivity,
	}

	summaryPayload := core.TaskSummaryPayloadFromSummary(&summary)
	if summaryPayload.Priority != taskpkg.PriorityHigh ||
		summaryPayload.MaxAttempts != 4 ||
		summaryPayload.ApprovalState != taskpkg.ApprovalStatePending ||
		summaryPayload.DependencyCount != 1 ||
		summaryPayload.LatestEventSeq != 17 ||
		!summaryPayload.EffectivePaused ||
		summaryPayload.PausedByTaskID != "task-root" ||
		summaryPayload.ActiveRun == nil ||
		summaryPayload.LastActivityAt == nil ||
		summaryPayload.Dependencies[0].DependsOn.Identifier != "TASK-2" ||
		summaryPayload.Dependencies[0].DependsOn.LatestEventSeq != 11 {
		t.Fatalf("TaskSummaryPayloadFromSummary() = %#v", summaryPayload)
	}
	if summaryPayload.ActiveRun.ClaimTokenHash != summary.ActiveRun.ClaimTokenHash ||
		summaryPayload.ActiveRun.RecoveryCount != summary.ActiveRun.RecoveryCount ||
		summaryPayload.ActiveRun.DesignationGroupID != summary.ActiveRun.DesignationGroupID ||
		summaryPayload.ActiveRun.Designation == nil {
		t.Fatalf("TaskSummaryPayloadFromSummary() rich active run = %#v", summaryPayload.ActiveRun)
	}
	catalogPayload := contract.TaskCatalogItemPayloadFromSummary(&summary)
	if catalogPayload.ActiveRun == nil ||
		catalogPayload.ActiveRun.RecoveryCount != summary.ActiveRun.RecoveryCount {
		t.Fatalf("TaskCatalogItemPayloadFromSummary() active run = %#v", catalogPayload.ActiveRun)
	}

	detailPayload := core.TaskDetailPayloadFromView(&taskpkg.View{
		Summary: summary,
		Task: taskpkg.Task{
			ID:             "task-1",
			Identifier:     "TASK-1",
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    "ws-alpha",
			ParentTaskID:   "task-root",
			Title:          "Review handlers",
			Description:    "Deep detail",
			Priority:       taskpkg.PriorityHigh,
			MaxAttempts:    4,
			Status:         taskpkg.TaskStatusBlocked,
			ApprovalPolicy: taskpkg.ApprovalPolicyManual,
			ApprovalState:  taskpkg.ApprovalStatePending,
			Owner:          &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "reviewers"},
			LatestEventSeq: 17,
			CreatedBy:      taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user-1"},
			Origin:         taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.create"},
			CreatedAt:      now.Add(-2 * time.Hour),
			UpdatedAt:      now,
			Metadata:       json.RawMessage(`{"priority":"high"}`),
		},
		DependencyReferences: summary.Dependencies,
	})
	if detailPayload.Summary.Priority != taskpkg.PriorityHigh ||
		detailPayload.Task.MaxAttempts != 4 ||
		detailPayload.Task.ApprovalPolicy != taskpkg.ApprovalPolicyManual ||
		detailPayload.Task.LatestEventSeq != 17 ||
		!detailPayload.Task.EffectivePaused ||
		detailPayload.Task.PausedByTaskID != "task-root" ||
		len(detailPayload.DependencyReferences) != 1 {
		t.Fatalf("TaskDetailPayloadFromView() = %#v", detailPayload)
	}

	runDetailPayload := core.TaskRunDetailPayloadFromView(&taskpkg.RunDetailView{
		Run: taskpkg.Run{
			ID:                 "run-1",
			TaskID:             "task-1",
			Status:             taskpkg.TaskRunStatusRunning,
			Attempt:            2,
			Origin:             taskpkg.Origin{Kind: taskpkg.OriginKindHTTP, Ref: "tasks.start_run"},
			DesignationGroupID: "designation-group",
			Metadata:           json.RawMessage(`{"designation":{"index":0,"brief":"Coordinate the group"}}`),
			QueuedAt:           now.Add(-10 * time.Minute),
		},
		Task: &taskpkg.Reference{
			ID:             "task-1",
			Identifier:     "TASK-1",
			Title:          "Review handlers",
			Status:         taskpkg.TaskStatusInProgress,
			Priority:       taskpkg.PriorityHigh,
			Owner:          &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "reviewers"},
			Scope:          taskpkg.ScopeWorkspace,
			WorkspaceID:    "ws-alpha",
			LatestEventSeq: 19,
		},
		Session: &taskpkg.RunSessionRef{
			SessionID:   "sess-1",
			WorkspaceID: "ws-alpha",
			AgentName:   "coder",
			Name:        "Task Runner",
			State:       "active",
			CreatedAt:   now.Add(-9 * time.Minute),
			UpdatedAt:   now,
		},
		Summary: taskpkg.RunOperationalSummary{
			LastActivityAt: lastActivity,
			LastEventType:  "task.run.started",
			ToolCallCount:  &toolCalls,
			TotalCost:      &totalCost,
			CostCurrency:   &currency,
			CostStatus:     "estimated",
			CostSource:     "catalog_config",
		},
	})
	if runDetailPayload.Session == nil ||
		runDetailPayload.Session.AgentName != "coder" ||
		runDetailPayload.Summary.ToolCallCount == nil ||
		*runDetailPayload.Summary.ToolCallCount != 3 ||
		runDetailPayload.Summary.CostStatus != "estimated" ||
		runDetailPayload.Summary.CostSource != "catalog_config" ||
		runDetailPayload.Task.Priority != taskpkg.PriorityHigh ||
		runDetailPayload.Task.LatestEventSeq != 19 ||
		runDetailPayload.Run.DesignationGroupID != "designation-group" ||
		runDetailPayload.Run.Designation == nil ||
		runDetailPayload.Run.Designation.Index != 0 ||
		runDetailPayload.Run.Designation.Brief != "Coordinate the group" {
		t.Fatalf("TaskRunDetailPayloadFromView() = %#v", runDetailPayload)
	}
	if nilSession := core.TaskRunSessionPayloadFromSession(nil); nilSession != nil {
		t.Fatalf("TaskRunSessionPayloadFromSession(nil) = %#v, want nil", nilSession)
	}
	if nilRunDetail := core.TaskRunDetailPayloadFromView(
		nil,
	); !reflect.DeepEqual(
		nilRunDetail,
		contract.TaskRunDetailPayload{},
	) {
		t.Fatalf("TaskRunDetailPayloadFromView(nil) = %#v, want zero value", nilRunDetail)
	}

	treePayload := core.TaskTreePayloadFromView(&taskpkg.TreeView{
		Root: taskpkg.TreeNode{
			Task: taskpkg.Reference{
				ID:          "task-root",
				Identifier:  "TASK-ROOT",
				Title:       "Root task",
				Status:      taskpkg.TaskStatusInProgress,
				Priority:    taskpkg.PriorityUrgent,
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-alpha",
			},
			Depth:          0,
			ChildCount:     1,
			ActiveRun:      summary.ActiveRun,
			LastActivityAt: now,
		},
		Descendants: []taskpkg.TreeNode{{
			Task: taskpkg.Reference{
				ID:          "task-1",
				Identifier:  "TASK-1",
				Title:       "Review handlers",
				Status:      taskpkg.TaskStatusBlocked,
				Priority:    taskpkg.PriorityHigh,
				Scope:       taskpkg.ScopeWorkspace,
				WorkspaceID: "ws-alpha",
			},
			ParentTaskID:   "task-root",
			Depth:          1,
			ChildCount:     0,
			LastActivityAt: lastActivity,
		}},
	})
	if treePayload.Root.ActiveRun == nil || len(treePayload.Descendants) != 1 ||
		treePayload.Descendants[0].ParentTaskID != "task-root" {
		t.Fatalf("TaskTreePayloadFromView() = %#v", treePayload)
	}

	dashboardPayload := core.TaskDashboardPayloadFromView(&observe.TaskDashboardView{
		Totals: observe.TaskDashboardTotals{TasksTotal: 3, RunsTotal: 2, ActiveRuns: 1, AwaitingApprovalTasks: 1},
		Cards: observe.TaskDashboardCards{
			InProgress: observe.TaskDashboardInProgressCard{Tasks: 1, ActiveRuns: 1, HealthStatus: "healthy"},
			Blocked:    observe.TaskDashboardBlockedCard{Tasks: 1, AwaitingApproval: 1, HealthStatus: "warning"},
			Failed:     observe.TaskDashboardFailedCard{Tasks: 1, FailedRuns: 1, HealthStatus: "warning"},
			Latency: observe.TaskDashboardLatencyCard{
				ClaimLatencyMillis: observe.LatencyMetric{Samples: 2, AverageMillis: 50, MaximumMillis: 75},
				StartLatencyMillis: observe.LatencyMetric{Samples: 2, AverageMillis: 60, MaximumMillis: 90},
			},
		},
		Queue: observe.TaskDashboardQueue{
			Total: 1,
			Depth: []observe.TaskQueueDepth{
				{
					ChannelID:           "builders",
					Count:               1,
					OldestQueuedAt:      now.Add(-5 * time.Minute),
					OldestQueueAgeMilli: 300000,
				},
			},
			OldestQueuedAt:        now.Add(-5 * time.Minute),
			OldestQueueAgeMilli:   300000,
			BacklogWarning:        true,
			BacklogStatus:         "warning",
			BacklogThresholdMilli: 120000,
		},
		Health: observe.TaskDashboardHealth{Status: "warning", StuckRuns: 1, QueueBacklog: true},
		StatusBreakdown: []observe.TaskDashboardStatusBreakdown{{
			Status:       taskpkg.TaskStatusInProgress,
			Count:        1,
			SharePercent: 33,
		}},
		ActiveRuns: observe.TaskDashboardActiveRuns{
			Total: 1, Running: 1,
			Items: []observe.TaskDashboardActiveRun{{
				TaskID:                       "task-1",
				TaskIdentifier:               "TASK-1",
				TaskTitle:                    "Review handlers",
				TaskStatus:                   taskpkg.TaskStatusInProgress,
				TaskPriority:                 taskpkg.PriorityHigh,
				TaskOwner:                    &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: "reviewers"},
				Scope:                        taskpkg.ScopeWorkspace,
				WorkspaceID:                  "ws-alpha",
				LatestEventSeq:               21,
				RunID:                        "run-1",
				RunStatus:                    taskpkg.TaskRunStatusRunning,
				Attempt:                      2,
				MaxAttempts:                  4,
				SessionID:                    "sess-1",
				ResolvedNetworkParticipation: &liveSpec,
				LastActivityAt:               lastActivity,
				AgeMilli:                     60000,
				HealthStatus:                 "healthy",
			}},
		},
		Freshness: observe.TaskDashboardFreshness{
			ObservedAt:       now,
			LatestActivityAt: lastActivity,
			AgeMilli:         60000,
			StaleAfterMilli:  120000,
			HasLiveWork:      true,
			Status:           "live",
		},
	})
	if dashboardPayload.Queue.Depth[0].ChannelID != "builders" ||
		dashboardPayload.ActiveRuns.Items[0].ResolvedNetworkParticipation == nil ||
		dashboardPayload.ActiveRuns.Items[0].ResolvedNetworkParticipation.ChannelID != "builders" ||
		dashboardPayload.ActiveRuns.Items[0].RunID != "run-1" ||
		dashboardPayload.ActiveRuns.Items[0].LatestEventSeq != 21 ||
		len(dashboardPayload.StatusBreakdown) != 1 ||
		dashboardPayload.StatusBreakdown[0].Status != taskpkg.TaskStatusInProgress ||
		dashboardPayload.Freshness.Status != "live" {
		t.Fatalf("TaskDashboardPayloadFromView() = %#v", dashboardPayload)
	}

	inboxPayload := core.TaskInboxPayloadFromView(observe.TaskInboxView{
		Total:         1,
		UnreadTotal:   1,
		ArchivedTotal: 0,
		Groups: []observe.TaskInboxLaneGroup{{
			Lane:        observe.TaskInboxLaneApprovals,
			Count:       1,
			UnreadCount: 1,
			Items: []observe.TaskInboxItem{{
				Task: taskpkg.Reference{
					ID:             "task-1",
					Identifier:     "TASK-1",
					Title:          "Review handlers",
					Status:         taskpkg.TaskStatusBlocked,
					Priority:       taskpkg.PriorityHigh,
					Scope:          taskpkg.ScopeWorkspace,
					WorkspaceID:    "ws-alpha",
					LatestEventSeq: 23,
				},
				Lane:             observe.TaskInboxLaneApprovals,
				ApprovalPolicy:   taskpkg.ApprovalPolicyManual,
				ApprovalState:    taskpkg.ApprovalStatePending,
				BlockingReason:   "awaiting_approval",
				LatestActivityAt: lastActivity,
				Run:              summary.ActiveRun,
				Triage: taskpkg.TriageState{
					TaskID:             "task-1",
					Actor:              taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "user-1"},
					LastSeenActivityAt: lastSeen,
					UpdatedAt:          now,
				},
			}},
		}},
	})
	if inboxPayload.Groups[0].Items[0].Run == nil ||
		inboxPayload.Groups[0].Items[0].Task.LatestEventSeq != 23 ||
		inboxPayload.Groups[0].Items[0].Triage.LastSeenActivityAt == nil ||
		inboxPayload.Groups[0].Items[0].BlockingReason != "awaiting_approval" {
		t.Fatalf("TaskInboxPayloadFromView() = %#v", inboxPayload)
	}
	encodedInbox, err := json.Marshal(inboxPayload)
	if err != nil {
		t.Fatalf("json.Marshal(TaskInboxPayloadFromView()) error = %v", err)
	}
	for _, forbidden := range []string{
		`"paused"`,
		`"effective_paused"`,
		`"paused_by_task_id"`,
		`"claim_token_hash"`,
		`"coordination_channel"`,
		`"designation_group_id"`,
		`"designation"`,
	} {
		if strings.Contains(string(encodedInbox), forbidden) {
			t.Fatalf("TaskInboxPayloadFromView() JSON contains %s: %s", forbidden, encodedInbox)
		}
	}
}

func TestTaskRunConversationStreamSurface(t *testing.T) {
	t.Parallel()

	t.Run("Should stream the run-fenced conversation and usage projection", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC)
		tasks := &testutil.StubTaskManager{
			RunDetailFn: func(
				_ context.Context,
				runID string,
				_ taskpkg.ActorContext,
			) (*taskpkg.RunDetailView, error) {
				return &taskpkg.RunDetailView{Run: taskpkg.Run{
					ID:     runID,
					TaskID: "task-1",
					Status: taskpkg.TaskRunStatusRunning,
					RunNetworkState: &taskpkg.RunNetworkState{
						NetworkSpec: testLiveParticipation("ws-alpha", "coord-shared"),
					},
					QueuedAt: now.Add(-time.Minute),
				}}, nil
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
		var gotRef store.NetworkConversationRef
		var gotQuery store.NetworkConversationMessageQuery
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListConversationMessagesFn: func(
				_ context.Context,
				ref store.NetworkConversationRef,
				query store.NetworkConversationMessageQuery,
			) ([]store.NetworkConversationMessage, error) {
				gotRef = ref
				gotQuery = query
				return []store.NetworkConversationMessage{{
					MessageID:   "msg-1",
					WorkspaceID: "ws-alpha",
					Channel:     "coord-shared",
					Surface:     store.NetworkSurfaceThread,
					ThreadID:    "thread_agent_channel",
					Direction:   "received",
					PeerFrom:    "coordinator.sess-1",
					Kind:        "say",
					Text:        "Coordinator posted durable evidence",
					Timestamp:   now,
				}}, nil
			},
		}
		fixture.Handlers.NetworkUsage = &stubNetworkUsageStore{report: store.NetworkUsageReport{
			Total: store.NetworkUsageSummary{
				WakeCount:       1,
				ActualWakeCount: 1,
				InputTokens:     13,
				OutputTokens:    5,
			},
			Budget: &store.NetworkBudgetUsage{
				OwnerKey:     "task_run:run-1",
				WakesUsed:    1,
				WallTimeUsed: time.Second,
				UpdatedAt:    now,
			},
		}}
		done := make(chan struct{})
		close(done)
		fixture.Handlers.SetStreamDone(done)

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/task-runs/run-1/conversation/stream?after=msg-0",
			nil,
		)
		if resp.Code != http.StatusOK {
			t.Fatalf("stream status = %d, want %d; body=%s", resp.Code, http.StatusOK, resp.Body.String())
		}
		if got := resp.Header().Get("Content-Type"); got != "text/event-stream" {
			t.Fatalf("stream content type = %q, want text/event-stream", got)
		}
		if gotRef.WorkspaceID != "ws-alpha" || gotRef.Channel != "coord-shared" ||
			gotRef.Surface != store.NetworkSurfaceThread || gotRef.ThreadID != "thread_agent_channel" {
			t.Fatalf("conversation ref = %#v, want run-fenced thread", gotRef)
		}
		if gotQuery.AfterMessageID != "msg-0" || gotQuery.Limit != 200 {
			t.Fatalf("conversation query = %#v, want reconnect cursor + bounded page", gotQuery)
		}
		body := resp.Body.String()
		for _, want := range []string{
			"id: msg-1",
			"event: network.message",
			`"message_id":"msg-1"`,
			"event: network.usage",
			`"participation_status":{"owner":{"workspace_id":"ws-alpha","kind":"task_run","id":"run-1"},"available":true,"participating":true}`,
			`"actual_wake_count":1`,
		} {
			if !strings.Contains(body, want) {
				t.Fatalf("stream body missing %q: %s", want, body)
			}
		}
	})

	t.Run("Should reject a Local run without inventing a conversation", func(t *testing.T) {
		t.Parallel()

		tasks := &testutil.StubTaskManager{
			RunDetailFn: func(
				_ context.Context,
				runID string,
				_ taskpkg.ActorContext,
			) (*taskpkg.RunDetailView, error) {
				return &taskpkg.RunDetailView{Run: taskpkg.Run{
					ID:       runID,
					Status:   taskpkg.TaskRunStatusRunning,
					QueuedAt: time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC),
				}}, nil
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
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{}
		fixture.Handlers.NetworkUsage = &stubNetworkUsageStore{}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/task-runs/run-local/conversation/stream",
			nil,
		)
		if resp.Code != http.StatusConflict || !strings.Contains(resp.Body.String(), "local task run") {
			t.Fatalf("local stream status/body = %d/%s, want 409 diagnostic", resp.Code, resp.Body.String())
		}
	})

	t.Run("Should deny before SSE headers and Network reads", func(t *testing.T) {
		t.Parallel()

		tasks := &testutil.StubTaskManager{
			RunDetailFn: func(
				_ context.Context,
				_ string,
				_ taskpkg.ActorContext,
			) (*taskpkg.RunDetailView, error) {
				return nil, taskpkg.ErrTaskRunNotFound
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
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListConversationMessagesFn: func(
				context.Context,
				store.NetworkConversationRef,
				store.NetworkConversationMessageQuery,
			) ([]store.NetworkConversationMessage, error) {
				t.Fatal("ListConversationMessages() called for denied stream")
				return nil, nil
			},
		}
		fixture.Handlers.NetworkUsage = &stubNetworkUsageStore{}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/task-runs/run-foreign/conversation/stream",
			nil,
		)
		if resp.Code != http.StatusNotFound {
			t.Fatalf("stream status = %d, want %d; body=%s", resp.Code, http.StatusNotFound, resp.Body.String())
		}
		if got := resp.Header().Get("Content-Type"); got == "text/event-stream" {
			t.Fatalf("stream content type = %q, want denial before SSE headers", got)
		}
	})

	t.Run("Should reject an invalid conversation cursor before SSE headers", func(t *testing.T) {
		t.Parallel()

		tasks := &testutil.StubTaskManager{
			RunDetailFn: func(
				_ context.Context,
				runID string,
				_ taskpkg.ActorContext,
			) (*taskpkg.RunDetailView, error) {
				return &taskpkg.RunDetailView{Run: taskpkg.Run{
					ID:     runID,
					TaskID: "task-1",
					Status: taskpkg.TaskRunStatusRunning,
					RunNetworkState: &taskpkg.RunNetworkState{
						NetworkSpec: testLiveParticipation("ws-alpha", "coord-shared"),
					},
					QueuedAt: time.Date(2026, 7, 15, 9, 0, 0, 0, time.UTC),
				}}, nil
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
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{
			ListConversationMessagesFn: func(
				context.Context,
				store.NetworkConversationRef,
				store.NetworkConversationMessageQuery,
			) ([]store.NetworkConversationMessage, error) {
				return nil, fmt.Errorf("%w: missing cursor", store.ErrNetworkCursorInvalid)
			},
		}
		fixture.Handlers.NetworkUsage = &stubNetworkUsageStore{}

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/task-runs/run-1/conversation/stream?after=msg-missing",
			nil,
		)
		if resp.Code != http.StatusUnprocessableEntity {
			t.Fatalf(
				"stream status = %d, want %d; body=%s",
				resp.Code,
				http.StatusUnprocessableEntity,
				resp.Body.String(),
			)
		}
		if got := resp.Header().Get("Content-Type"); got == "text/event-stream" {
			t.Fatalf("stream content type = %q, want cursor rejection before SSE headers", got)
		}
	})

	t.Run("Should reject a missing usage service before loading the run or SSE headers", func(t *testing.T) {
		t.Parallel()

		runDetailCalls := 0
		fixture := newHandlerFixtureWithTasks(
			t,
			testutil.StubSessionManager{},
			testutil.StubObserver{},
			&testutil.StubTaskManager{
				RunDetailFn: func(
					context.Context,
					string,
					taskpkg.ActorContext,
				) (*taskpkg.RunDetailView, error) {
					runDetailCalls++
					return nil, errors.New("run detail should not be loaded without Network usage")
				},
			},
			testutil.StubWorkspaceService{},
			nil,
			nil,
		)
		fixture.Handlers.NetworkStore = testutil.StubNetworkStore{}
		fixture.Handlers.NetworkUsage = nil

		resp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/task-runs/run-1/conversation/stream",
			nil,
		)
		if resp.Code != http.StatusServiceUnavailable {
			t.Fatalf(
				"stream status = %d, want %d; body=%s",
				resp.Code,
				http.StatusServiceUnavailable,
				resp.Body.String(),
			)
		}
		if runDetailCalls != 0 {
			t.Fatalf("RunDetail() calls = %d, want 0", runDetailCalls)
		}
		if got := resp.Header().Get("Content-Type"); got == "text/event-stream" {
			t.Fatalf("stream content type = %q, want service rejection before SSE headers", got)
		}
	})
}

func TestTaskCatalogResponseUsesLeanPageItems(t *testing.T) {
	t.Parallel()

	t.Run("Should omit enrichment that requires per-task graph reads", func(t *testing.T) {
		t.Parallel()

		response := core.TaskCatalogResponseFromPage(taskpkg.CatalogPage{
			Tasks: []taskpkg.Summary{{
				ID:              "task-lean",
				Title:           "Lean catalog item",
				Paused:          true,
				PausedBy:        "operator",
				PausedAt:        time.Now(),
				PausedReason:    "pause",
				EffectivePaused: true,
				PausedByTaskID:  "task-parent",
				BlockedReasons:  &[]taskpkg.BlockedReason{{Source: taskpkg.BlockedSourceDependency}},
				Dependencies:    []taskpkg.DependencyReference{{TaskID: "task-lean"}},
				ActiveRun: &taskpkg.RunSummary{
					ID:                 "run-lean",
					ClaimTokenHash:     "sha256:catalog-verifier",
					DesignationGroupID: "designation-group",
					Designation:        &taskpkg.RunDesignationSummary{Index: 2, Brief: "private fan-out brief"},
				},
			}},
			Total: 1,
			Limit: 10,
		})
		if len(response.Tasks) != 1 {
			t.Fatalf("catalog tasks = %d, want 1", len(response.Tasks))
		}
		item := response.Tasks[0]
		if item.ID != "task-lean" || item.ActiveRun == nil || item.ActiveRun.ID != "run-lean" {
			t.Fatalf("catalog item = %#v, want bounded task and run summaries", item)
		}
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Fatalf("json.Marshal(TaskCatalogResponseFromPage()) error = %v", err)
		}
		for _, forbidden := range []string{
			`"paused"`,
			`"paused_by"`,
			`"paused_at"`,
			`"paused_reason"`,
			`"effective_paused"`,
			`"paused_by_task_id"`,
			`"blocked_reasons"`,
			`"dependencies"`,
			`"claim_token_hash"`,
			`"coordination_channel"`,
			`"designation_group_id"`,
			`"designation"`,
		} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf("TaskCatalogResponseFromPage() JSON contains %s: %s", forbidden, encoded)
			}
		}
	})
}

func TestSchedulerControlHandlersDelegateToTaskService(t *testing.T) {
	t.Run("Should expose scheduler and per-task pause operations through shared handlers", func(t *testing.T) {
		now := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
		var pauseTask taskpkg.PauseTaskRequest
		var drainRequest taskpkg.SchedulerDrainRequest
		var backlogQuery taskpkg.SchedulerBacklogQuery
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
				pauseTask = req
				return &taskpkg.Task{
					ID:           id,
					Title:        "Paused task",
					Status:       taskpkg.TaskStatusReady,
					Paused:       true,
					PausedBy:     "human:operator",
					PausedAt:     now,
					PausedReason: req.Reason,
					CreatedAt:    now,
					UpdatedAt:    now,
				}, nil
			},
			SchedulerStatusFn: func(_ context.Context, _ taskpkg.ActorContext) (taskpkg.SchedulerStatus, error) {
				return taskpkg.SchedulerStatus{
					Paused:           true,
					PausedBy:         "human:operator",
					PausedAt:         now,
					PausedReason:     "maintenance",
					ActiveClaimCount: 2,
					QueuedRunCount:   3,
					PausedTaskCount:  1,
					AsOf:             now,
				}, nil
			},
			DrainSchedulerFn: func(
				_ context.Context,
				req taskpkg.SchedulerDrainRequest,
				_ taskpkg.ActorContext,
			) (taskpkg.SchedulerDrainResult, error) {
				drainRequest = req
				return taskpkg.SchedulerDrainResult{
					Status:      taskpkg.SchedulerStatus{Paused: true, AsOf: now},
					Completed:   false,
					TimedOut:    true,
					StartedAt:   now,
					CompletedAt: now,
				}, nil
			},
			SchedulerBacklogFn: func(
				_ context.Context,
				query taskpkg.SchedulerBacklogQuery,
				_ taskpkg.ActorContext,
			) (taskpkg.SchedulerBacklog, error) {
				backlogQuery = query
				return taskpkg.SchedulerBacklog{
					Runs: []taskpkg.SchedulerBacklogRun{{
						Task: taskpkg.Task{
							ID:          "task-queued",
							Title:       "Queued task",
							Status:      taskpkg.TaskStatusReady,
							WorkspaceID: "ws-alpha",
							CreatedAt:   now,
							UpdatedAt:   now,
						},
						Run: taskpkg.Run{
							ID:       "run-queued",
							TaskID:   "task-queued",
							Status:   taskpkg.TaskRunStatusQueued,
							QueuedAt: now,
						},
					}},
					Total: 1,
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
			[]byte("{\"reason\":\"maintenance\"}"),
		)
		if pauseResp.Code != http.StatusOK {
			t.Fatalf("pause status = %d, want 200; body=%s", pauseResp.Code, pauseResp.Body.String())
		}
		if pauseTask.Reason != "maintenance" {
			t.Fatalf("pause request = %#v, want reason", pauseTask)
		}
		var pausePayload contract.TaskResponse
		if err := json.Unmarshal(pauseResp.Body.Bytes(), &pausePayload); err != nil {
			t.Fatalf("decode pause response: %v", err)
		}
		if !pausePayload.Task.Paused || pausePayload.Task.PausedReason != "maintenance" {
			t.Fatalf("pause payload task = %#v", pausePayload.Task)
		}

		statusResp := performRequest(t, fixture.Engine, http.MethodGet, "/scheduler", nil)
		if statusResp.Code != http.StatusOK {
			t.Fatalf("status code = %d, want 200; body=%s", statusResp.Code, statusResp.Body.String())
		}
		var statusPayload contract.SchedulerStatusResponse
		if err := json.Unmarshal(statusResp.Body.Bytes(), &statusPayload); err != nil {
			t.Fatalf("decode scheduler status: %v", err)
		}
		if !statusPayload.Scheduler.Paused || statusPayload.Scheduler.QueuedRunCount != 3 {
			t.Fatalf("scheduler status = %#v", statusPayload.Scheduler)
		}

		drainResp := performRequest(
			t,
			fixture.Engine,
			http.MethodPost,
			"/scheduler/drain",
			[]byte("{\"timeout_seconds\":0}"),
		)
		if drainResp.Code != http.StatusOK {
			t.Fatalf("drain status = %d, want 200; body=%s", drainResp.Code, drainResp.Body.String())
		}
		if drainRequest.Timeout != 0 {
			t.Fatalf("drain timeout = %s, want 0", drainRequest.Timeout)
		}

		backlogResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/scheduler/backlog?limit=2&scope=workspace&workspace=ws-alpha&include_paused=true",
			nil,
		)
		if backlogResp.Code != http.StatusOK {
			t.Fatalf("backlog status = %d, want 200; body=%s", backlogResp.Code, backlogResp.Body.String())
		}
		if backlogQuery.Limit != 2 ||
			backlogQuery.Scope != taskpkg.ScopeWorkspace ||
			backlogQuery.WorkspaceID != "ws-alpha" ||
			!backlogQuery.IncludePaused {
			t.Fatalf("backlog query = %#v", backlogQuery)
		}
	})
}

func TestBaseHandlersExpandedTaskEndpoints(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 17, 13, 0, 0, 0, time.UTC)
	var publishActor taskpkg.ActorContext
	var inspectTaskActor taskpkg.ActorContext
	var inspectRunActor taskpkg.ActorContext
	var runDetailActor taskpkg.ActorContext
	var timelineActor taskpkg.ActorContext
	var timelineQuery taskpkg.TimelineQuery
	var treeActor taskpkg.ActorContext
	var dashboardQuery observe.TaskDashboardQuery
	var inboxQuery observe.TaskInboxQuery
	var inboxActor taskpkg.ActorIdentity
	var approveActor taskpkg.ActorContext
	var startActor taskpkg.ActorContext
	var readActor taskpkg.ActorContext
	var archiveActor taskpkg.ActorContext
	var dismissActor taskpkg.ActorContext
	var streamActor taskpkg.ActorContext
	var streamQuery taskpkg.StreamQuery

	tasks := &testutil.StubTaskManager{
		InspectTaskFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (*taskpkg.InspectView, error) {
			inspectTaskActor = actor
			return &taskpkg.InspectView{
				Target: taskpkg.InspectTargetTask,
				Task: taskpkg.Summary{
					ID:          id,
					Title:       "Published task",
					Status:      taskpkg.TaskStatusReady,
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-alpha",
				},
				CurrentRun: &taskpkg.InspectRunSummary{
					RunID:                   "run-1",
					TaskID:                  id,
					Status:                  taskpkg.TaskRunStatusQueued,
					ClaimTokenHashTruncated: "abcdef12",
					QueuedAt:                now.Add(-10 * time.Minute),
					Attempt:                 1,
				},
				Diagnostics: []contract.DiagnosticItem{{
					ID:            "task.inspect.task_run_stranded.run-1",
					Code:          contract.CodeTaskRunStranded,
					Severity:      contract.SeverityWarn,
					Category:      contract.CategoryTask,
					Title:         "Queued task run has no eligible session",
					Message:       "No eligible session is visible.",
					DataFreshness: contract.FreshnessLive,
				}},
				NextAction: taskpkg.InspectNextActionStranded,
				AsOf:       now,
			}, nil
		},
		InspectRunFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (*taskpkg.InspectView, error) {
			inspectRunActor = actor
			return &taskpkg.InspectView{
				Target: taskpkg.InspectTargetRun,
				Task: taskpkg.Summary{
					ID:     "task-1",
					Title:  "Published task",
					Status: taskpkg.TaskStatusInProgress,
				},
				CurrentRun: &taskpkg.InspectRunSummary{
					RunID:    id,
					TaskID:   "task-1",
					Status:   taskpkg.TaskRunStatusRunning,
					QueuedAt: now.Add(-10 * time.Minute),
					Attempt:  2,
				},
				NextAction: taskpkg.InspectNextActionRunning,
				AsOf:       now,
			}, nil
		},
		PublishTaskFn: func(
			_ context.Context,
			id string,
			_ taskpkg.ExecutionRequest,
			actor taskpkg.ActorContext,
		) (*taskpkg.Execution, error) {
			publishActor = actor
			taskRecord := taskpkg.Task{
				ID:             id,
				Identifier:     "TASK-1",
				Scope:          taskpkg.ScopeWorkspace,
				WorkspaceID:    "ws-alpha",
				Title:          "Published task",
				Priority:       taskpkg.PriorityHigh,
				MaxAttempts:    3,
				Status:         taskpkg.TaskStatusReady,
				ApprovalPolicy: taskpkg.ApprovalPolicyNone,
				ApprovalState:  taskpkg.ApprovalStateNotRequired,
				CreatedBy:      actor.Actor,
				Origin:         actor.Origin,
				CreatedAt:      now.Add(-time.Hour),
				UpdatedAt:      now,
			}
			return &taskpkg.Execution{
				Task: taskRecord,
				Run: taskpkg.Run{
					ID:      "run-publish",
					TaskID:  id,
					Status:  taskpkg.TaskRunStatusQueued,
					Attempt: 1,
					Origin:  actor.Origin,
					RunNetworkState: &taskpkg.RunNetworkState{
						NetworkSpec: testLiveParticipation("ws-alpha", "coord-run-publish"),
					},
					QueuedAt: now,
				},
			}, nil
		},
		StartTaskFn: func(
			_ context.Context,
			id string,
			_ taskpkg.ExecutionRequest,
			actor taskpkg.ActorContext,
		) (*taskpkg.Execution, error) {
			startActor = actor
			taskRecord := taskpkg.Task{
				ID:             id,
				Scope:          taskpkg.ScopeWorkspace,
				WorkspaceID:    "ws-alpha",
				Title:          "Started task",
				Status:         taskpkg.TaskStatusReady,
				ApprovalPolicy: taskpkg.ApprovalPolicyNone,
				ApprovalState:  taskpkg.ApprovalStateNotRequired,
				CreatedBy:      actor.Actor,
				Origin:         actor.Origin,
				CreatedAt:      now.Add(-time.Hour),
				UpdatedAt:      now,
			}
			return &taskpkg.Execution{
				Task: taskRecord,
				Run: taskpkg.Run{
					ID:      "run-start",
					TaskID:  id,
					Status:  taskpkg.TaskRunStatusQueued,
					Attempt: 1,
					Origin:  actor.Origin,
					RunNetworkState: &taskpkg.RunNetworkState{
						NetworkSpec: testLiveParticipation("ws-alpha", "coord-run-start"),
					},
					QueuedAt: now,
				},
			}, nil
		},
		RunDetailFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (*taskpkg.RunDetailView, error) {
			runDetailActor = actor
			return &taskpkg.RunDetailView{
				Run: taskpkg.Run{
					ID:      id,
					TaskID:  "task-1",
					Status:  taskpkg.TaskRunStatusRunning,
					Attempt: 2,
					Origin:  actor.Origin,
					RunNetworkState: &taskpkg.RunNetworkState{
						NetworkSpec: testLiveParticipation("ws-alpha", "coord-run-detail"),
					},
					QueuedAt: now.Add(-10 * time.Minute),
				},
				Task: &taskpkg.Reference{
					ID:          "task-1",
					Identifier:  "TASK-1",
					Title:       "Published task",
					Status:      taskpkg.TaskStatusInProgress,
					Priority:    taskpkg.PriorityHigh,
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-alpha",
				},
				Session: &taskpkg.RunSessionRef{
					SessionID:   "sess-1",
					WorkspaceID: "ws-alpha",
					AgentName:   "coder",
					Name:        "Runner",
					State:       "active",
					CreatedAt:   now.Add(-9 * time.Minute),
					UpdatedAt:   now,
				},
			}, nil
		},
		TimelineFn: func(_ context.Context, id string, query taskpkg.TimelineQuery, actor taskpkg.ActorContext) ([]taskpkg.TimelineItem, error) {
			timelineActor = actor
			timelineQuery = query
			return []taskpkg.TimelineItem{{
				Sequence: 11,
				EventID:  "evt-11",
				Task: taskpkg.Reference{
					ID:          id,
					Identifier:  "TASK-1",
					Title:       "Published task",
					Status:      taskpkg.TaskStatusInProgress,
					Priority:    taskpkg.PriorityHigh,
					Scope:       taskpkg.ScopeWorkspace,
					WorkspaceID: "ws-alpha",
				},
				Run: &taskpkg.RunSummary{
					ID:          "run-1",
					TaskID:      id,
					Status:      taskpkg.TaskRunStatusRunning,
					Attempt:     2,
					MaxAttempts: 3,
					SessionID:   "sess-1",
					QueuedAt:    now.Add(-10 * time.Minute),
				},
				EventType: "task.run.started",
				Actor:     actor.Actor,
				Origin:    actor.Origin,
				Payload:   json.RawMessage(`{"status":"running"}`),
				Timestamp: now,
			}}, nil
		},
		StreamFn: func(_ context.Context, _ string, query taskpkg.StreamQuery, actor taskpkg.ActorContext) (<-chan taskpkg.StreamEvent, error) {
			streamActor = actor
			streamQuery = query
			ch := make(chan taskpkg.StreamEvent, 1)
			ch <- taskpkg.StreamEvent{
				Sequence: 13,
				Type:     "task.run.started",
				Timeline: taskpkg.TimelineItem{
					Sequence: 13,
					EventID:  "evt-13",
					Task: taskpkg.Reference{
						ID:          "task-1",
						Identifier:  "TASK-1",
						Title:       "Published task",
						Status:      taskpkg.TaskStatusInProgress,
						Priority:    taskpkg.PriorityHigh,
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
		TreeFn: func(_ context.Context, _ string, actor taskpkg.ActorContext) (*taskpkg.TreeView, error) {
			treeActor = actor
			return &taskpkg.TreeView{
				Root: taskpkg.TreeNode{
					Task: taskpkg.Reference{
						ID:          "task-1",
						Identifier:  "TASK-1",
						Title:       "Published task",
						Status:      taskpkg.TaskStatusInProgress,
						Priority:    taskpkg.PriorityHigh,
						Scope:       taskpkg.ScopeWorkspace,
						WorkspaceID: "ws-alpha",
					},
					Depth:          0,
					ChildCount:     1,
					LastActivityAt: now,
				},
			}, nil
		},
		ApproveTaskFn: func(
			_ context.Context,
			id string,
			_ taskpkg.ExecutionRequest,
			actor taskpkg.ActorContext,
		) (*taskpkg.Execution, error) {
			approveActor = actor
			taskRecord := taskpkg.Task{
				ID:             id,
				Scope:          taskpkg.ScopeWorkspace,
				WorkspaceID:    "ws-alpha",
				Title:          "Approval task",
				Status:         taskpkg.TaskStatusReady,
				ApprovalPolicy: taskpkg.ApprovalPolicyManual,
				ApprovalState:  taskpkg.ApprovalStateApproved,
				CreatedBy:      actor.Actor,
				Origin:         actor.Origin,
				CreatedAt:      now.Add(-time.Hour),
				UpdatedAt:      now,
			}
			return &taskpkg.Execution{
				Task: taskRecord,
				Run: taskpkg.Run{
					ID:      "run-approve",
					TaskID:  id,
					Status:  taskpkg.TaskRunStatusQueued,
					Attempt: 1,
					Origin:  actor.Origin,
					RunNetworkState: &taskpkg.RunNetworkState{
						NetworkSpec: testLiveParticipation("ws-alpha", "coord-run-approve"),
					},
					QueuedAt: now,
				},
			}, nil
		},
		MarkTaskReadFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (taskpkg.TriageState, error) {
			readActor = actor
			return taskpkg.TriageState{TaskID: id, Actor: actor.Actor, Read: true, UpdatedAt: now}, nil
		},
		ArchiveTaskFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (taskpkg.TriageState, error) {
			archiveActor = actor
			return taskpkg.TriageState{TaskID: id, Actor: actor.Actor, Read: true, Archived: true, UpdatedAt: now}, nil
		},
		DismissTaskFn: func(_ context.Context, id string, actor taskpkg.ActorContext) (taskpkg.TriageState, error) {
			dismissActor = actor
			return taskpkg.TriageState{TaskID: id, Actor: actor.Actor, Dismissed: true, UpdatedAt: now}, nil
		},
	}
	observer := testutil.StubObserver{
		QueryTaskDashboardFn: func(_ context.Context, query observe.TaskDashboardQuery) (observe.TaskDashboardView, error) {
			dashboardQuery = query
			return observe.TaskDashboardView{
				Totals:     observe.TaskDashboardTotals{TasksTotal: 3, RunsTotal: 1, ActiveRuns: 1},
				Cards:      observe.TaskDashboardCards{},
				Queue:      observe.TaskDashboardQueue{Total: 1, OldestQueuedAt: now, BacklogStatus: "ok"},
				Health:     observe.TaskDashboardHealth{Status: "ok"},
				ActiveRuns: observe.TaskDashboardActiveRuns{Total: 1, Running: 1},
				Freshness:  observe.TaskDashboardFreshness{ObservedAt: now, LatestActivityAt: now, Status: "live"},
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
					Items: []observe.TaskInboxItem{{
						Task: taskpkg.Reference{
							ID:          "task-1",
							Identifier:  "TASK-1",
							Title:       "Approval task",
							Status:      taskpkg.TaskStatusBlocked,
							Priority:    taskpkg.PriorityHigh,
							Scope:       taskpkg.ScopeWorkspace,
							WorkspaceID: "ws-alpha",
						},
						Lane:             observe.TaskInboxLaneApprovals,
						ApprovalPolicy:   taskpkg.ApprovalPolicyManual,
						ApprovalState:    taskpkg.ApprovalStatePending,
						BlockingReason:   "awaiting_approval",
						LatestActivityAt: now,
						Triage: taskpkg.TriageState{
							TaskID:    "task-1",
							Actor:     actor,
							UpdatedAt: now,
						},
					}},
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
	usage := &stubNetworkUsageStore{report: store.NetworkUsageReport{
		Total: store.NetworkUsageSummary{
			WakeCount:       1,
			ActualWakeCount: 1,
			InputTokens:     11,
			OutputTokens:    7,
		},
		Budget: &store.NetworkBudgetUsage{
			OwnerKey:         "task_run:run-1",
			WakesUsed:        1,
			WallTimeUsed:     time.Second,
			InputTokensUsed:  11,
			OutputTokensUsed: 7,
			UpdatedAt:        now,
		},
	}}
	fixture.Handlers.NetworkUsage = usage
	fixture.Handlers.TaskActorContextResolver = func(_ *gin.Context, action string) (taskpkg.ActorContext, error) {
		return taskpkg.DeriveHumanActorContext("user-1", taskpkg.OriginKindHTTP, "tasks."+action)
	}

	t.Run("Should happy path json endpoints", func(t *testing.T) {
		t.Parallel()

		publishResp := performRequest(t, fixture.Engine, http.MethodPost, "/tasks/task-1/publish", nil)
		if publishResp.Code != http.StatusOK {
			t.Fatalf(
				"publish status = %d, want %d; body=%s",
				publishResp.Code,
				http.StatusOK,
				publishResp.Body.String(),
			)
		}
		var publishPayload contract.TaskExecutionResponse
		testutil.DecodeJSONResponse(t, publishResp, &publishPayload)
		if publishPayload.Task.Status != taskpkg.TaskStatusReady ||
			publishPayload.Run.Status != taskpkg.TaskRunStatusQueued ||
			publishActor.Origin.Ref != "tasks.publish" {
			t.Fatalf("publish payload/actor = %#v / %#v", publishPayload, publishActor)
		}

		startResp := performRequest(t, fixture.Engine, http.MethodPost, "/tasks/task-1/start", nil)
		if startResp.Code != http.StatusCreated {
			t.Fatalf(
				"start status = %d, want %d; body=%s",
				startResp.Code,
				http.StatusCreated,
				startResp.Body.String(),
			)
		}
		var startPayload contract.TaskExecutionResponse
		testutil.DecodeJSONResponse(t, startResp, &startPayload)
		if startPayload.Run.ID != "run-start" || startActor.Origin.Ref != "tasks.start" {
			t.Fatalf("start payload/actor = %#v / %#v", startPayload, startActor)
		}

		runResp := performRequest(t, fixture.Engine, http.MethodGet, "/task-runs/run-1", nil)
		if runResp.Code != http.StatusOK {
			t.Fatalf("run detail status = %d, want %d; body=%s", runResp.Code, http.StatusOK, runResp.Body.String())
		}
		var runPayload contract.TaskRunDetailResponse
		testutil.DecodeJSONResponse(t, runResp, &runPayload)
		if runPayload.Run.Run.ID != "run-1" || runPayload.Run.Session == nil ||
			runDetailActor.Origin.Ref != "tasks.get_run" {
			t.Fatalf("run detail payload/actor = %#v / %#v", runPayload, runDetailActor)
		}
		if runPayload.Run.Network == nil ||
			runPayload.Run.Network.Conversation.WorkspaceID != "ws-alpha" ||
			runPayload.Run.Network.Conversation.Channel != "coord-run-detail" ||
			runPayload.Run.Network.Conversation.Surface != store.NetworkSurfaceThread ||
			runPayload.Run.Network.Conversation.ThreadID != "thread_agent_channel" ||
			runPayload.Run.Network.Conversation.StreamURL != "/api/task-runs/run-1/conversation/stream" ||
			runPayload.Run.Network.Usage.Budget == nil ||
			runPayload.Run.Network.Usage.Budget.WakesUsed != 1 ||
			runPayload.Run.Network.Usage.Total.ActualWakeCount != 1 {
			t.Fatalf("run detail network projection = %#v", runPayload.Run.Network)
		}
		if usage.lastQuery.WorkspaceID != "ws-alpha" || usage.lastQuery.RunID != "run-1" {
			t.Fatalf("run detail usage query = %#v, want workspace/run fence", usage.lastQuery)
		}

		inspectTaskResp := performRequest(t, fixture.Engine, http.MethodGet, "/tasks/task-1/inspect", nil)
		if inspectTaskResp.Code != http.StatusOK {
			t.Fatalf(
				"task inspect status = %d, want %d; body=%s",
				inspectTaskResp.Code,
				http.StatusOK,
				inspectTaskResp.Body.String(),
			)
		}
		var inspectTaskPayload contract.TaskInspectResponse
		testutil.DecodeJSONResponse(t, inspectTaskResp, &inspectTaskPayload)
		if inspectTaskPayload.Inspect.NextAction != string(taskpkg.InspectNextActionStranded) ||
			inspectTaskPayload.Inspect.CurrentRun == nil ||
			inspectTaskPayload.Inspect.CurrentRun.ClaimTokenHashTruncated != "abcdef12" ||
			inspectTaskActor.Origin.Ref != "tasks.inspect" {
			t.Fatalf("task inspect payload/actor = %#v / %#v", inspectTaskPayload, inspectTaskActor)
		}

		inspectRunResp := performRequest(t, fixture.Engine, http.MethodGet, "/runs/run-1/inspect", nil)
		if inspectRunResp.Code != http.StatusOK {
			t.Fatalf(
				"run inspect status = %d, want %d; body=%s",
				inspectRunResp.Code,
				http.StatusOK,
				inspectRunResp.Body.String(),
			)
		}
		var inspectRunPayload contract.TaskInspectResponse
		testutil.DecodeJSONResponse(t, inspectRunResp, &inspectRunPayload)
		if inspectRunPayload.Inspect.Target != string(taskpkg.InspectTargetRun) ||
			inspectRunPayload.Inspect.NextAction != string(taskpkg.InspectNextActionRunning) ||
			inspectRunActor.Origin.Ref != "tasks.inspect" {
			t.Fatalf("run inspect payload/actor = %#v / %#v", inspectRunPayload, inspectRunActor)
		}

		timelineResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/tasks/task-1/timeline?after_sequence=4&limit=3",
			nil,
		)
		if timelineResp.Code != http.StatusOK {
			t.Fatalf(
				"timeline status = %d, want %d; body=%s",
				timelineResp.Code,
				http.StatusOK,
				timelineResp.Body.String(),
			)
		}
		var timelinePayload contract.TaskTimelineResponse
		testutil.DecodeJSONResponse(t, timelineResp, &timelinePayload)
		if len(timelinePayload.Timeline) != 1 || timelinePayload.Timeline[0].Sequence != 11 {
			t.Fatalf("timeline payload = %#v", timelinePayload)
		}
		if timelineActor.Origin.Ref != "tasks.timeline" || timelineQuery.AfterSequence != 4 ||
			timelineQuery.Limit != 3 {
			t.Fatalf("timeline actor/query = %#v / %#v", timelineActor, timelineQuery)
		}

		treeResp := performRequest(t, fixture.Engine, http.MethodGet, "/tasks/task-1/tree", nil)
		if treeResp.Code != http.StatusOK {
			t.Fatalf("tree status = %d, want %d; body=%s", treeResp.Code, http.StatusOK, treeResp.Body.String())
		}
		var treePayload contract.TaskTreeResponse
		testutil.DecodeJSONResponse(t, treeResp, &treePayload)
		if treePayload.Tree.Root.Task.ID != "task-1" || treeActor.Origin.Ref != "tasks.tree" {
			t.Fatalf("tree payload/actor = %#v / %#v", treePayload, treeActor)
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
		var dashboardPayload contract.TaskDashboardResponse
		testutil.DecodeJSONResponse(t, dashboardResp, &dashboardPayload)
		if dashboardPayload.Dashboard.Totals.TasksTotal != 3 || dashboardQuery.WorkspaceID != "ws-alpha" ||
			dashboardQuery.OriginKind != taskpkg.OriginKindHTTP {
			t.Fatalf("dashboard payload/query = %#v / %#v", dashboardPayload, dashboardQuery)
		}

		inboxResp := performRequest(
			t,
			fixture.Engine,
			http.MethodGet,
			"/observe/tasks/inbox?scope=workspace&workspace=alpha&owner_kind=human&owner_ref=alice&lane=approvals&unread=true&query=approve&limit=2",
			nil,
		)
		if inboxResp.Code != http.StatusOK {
			t.Fatalf("inbox status = %d, want %d; body=%s", inboxResp.Code, http.StatusOK, inboxResp.Body.String())
		}
		var inboxPayload contract.TaskInboxResponse
		testutil.DecodeJSONResponse(t, inboxResp, &inboxPayload)
		if inboxPayload.Inbox.Page.Total != 1 || len(inboxPayload.Inbox.Groups) != 1 {
			t.Fatalf("inbox payload = %#v", inboxPayload)
		}
		if inboxActor.Ref != "user-1" || inboxActor.Kind != taskpkg.ActorKindHuman ||
			inboxQuery.Lane != observe.TaskInboxLaneApprovals ||
			inboxQuery.Unread == nil || !*inboxQuery.Unread {
			t.Fatalf("inbox actor/query = %#v / %#v", inboxActor, inboxQuery)
		}

		approveResp := performRequest(t, fixture.Engine, http.MethodPost, "/tasks/task-1/approve", nil)
		if approveResp.Code != http.StatusCreated {
			t.Fatalf(
				"approve status = %d, want %d; body=%s",
				approveResp.Code,
				http.StatusCreated,
				approveResp.Body.String(),
			)
		}
		var approvePayload contract.TaskExecutionResponse
		testutil.DecodeJSONResponse(t, approveResp, &approvePayload)
		if approvePayload.Run.ID != "run-approve" || approveActor.Origin.Ref != "tasks.approve" {
			t.Fatalf("approve payload/actor = %#v / %#v", approvePayload, approveActor)
		}

		readResp := performRequest(t, fixture.Engine, http.MethodPost, "/tasks/task-1/triage/read", nil)
		if readResp.Code != http.StatusOK {
			t.Fatalf("triage read status = %d, want %d; body=%s", readResp.Code, http.StatusOK, readResp.Body.String())
		}
		archiveResp := performRequest(t, fixture.Engine, http.MethodPost, "/tasks/task-1/triage/archive", nil)
		if archiveResp.Code != http.StatusOK {
			t.Fatalf(
				"triage archive status = %d, want %d; body=%s",
				archiveResp.Code,
				http.StatusOK,
				archiveResp.Body.String(),
			)
		}
		dismissResp := performRequest(t, fixture.Engine, http.MethodPost, "/tasks/task-1/triage/dismiss", nil)
		if dismissResp.Code != http.StatusOK {
			t.Fatalf(
				"triage dismiss status = %d, want %d; body=%s",
				dismissResp.Code,
				http.StatusOK,
				dismissResp.Body.String(),
			)
		}
		if readActor.Origin.Ref != "tasks.triage_read" ||
			archiveActor.Origin.Ref != "tasks.triage_archive" ||
			dismissActor.Origin.Ref != "tasks.triage_dismiss" {
			t.Fatalf("triage actors = %#v / %#v / %#v", readActor, archiveActor, dismissActor)
		}
	})

	t.Run("Should task stream sse", func(t *testing.T) {
		t.Parallel()

		streamResp := testutil.PerformRequestWithHeaders(
			t,
			fixture.Engine,
			http.MethodGet,
			"/tasks/task-1/stream?after_sequence=2",
			nil,
			map[string]string{"Last-Event-ID": "9"},
		)
		if streamResp.Code != http.StatusOK {
			t.Fatalf("stream status = %d, want %d; body=%s", streamResp.Code, http.StatusOK, streamResp.Body.String())
		}
		if got := streamResp.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
			t.Fatalf("stream content-type = %q, want prefix %q", got, "text/event-stream")
		}
		if !streamResp.Flushed {
			t.Fatal("stream recorder was not flushed")
		}
		records := testutil.ParseSSE(t, streamResp.Body.String())
		if len(records) != 1 {
			t.Fatalf("stream records = %d, want 1; body=%s", len(records), streamResp.Body.String())
		}
		if records[0].ID != "13" || records[0].Event != "" {
			t.Fatalf("stream record = %#v", records[0])
		}
		var payload contract.TaskStreamEventPayload
		testutil.DecodeSSEData(t, records[0], &payload)
		if payload.Sequence != 13 || payload.Type != "task.run.started" || payload.Timeline.EventID != "evt-13" {
			t.Fatalf("stream payload = %#v", payload)
		}
		if streamActor.Origin.Ref != "tasks.stream" || streamQuery.AfterSequence != 9 {
			t.Fatalf("stream actor/query = %#v / %#v", streamActor, streamQuery)
		}
	})
}

func TestBaseHandlersExpandedTaskEndpointErrorPaths(t *testing.T) {
	t.Parallel()

	workspaces := testutil.StubWorkspaceService{
		GetFn: func(_ context.Context, ref string) (workspacepkg.Workspace, error) {
			if ref == "missing" {
				return workspacepkg.Workspace{}, workspacepkg.ErrWorkspaceNotFound
			}
			return workspacepkg.Workspace{ID: "ws-alpha", Name: ref}, nil
		},
	}

	fixture := newHandlerFixtureWithTasks(
		t,
		testutil.StubSessionManager{},
		testutil.StubObserver{},
		&testutil.StubTaskManager{
			PublishTaskFn: func(
				context.Context,
				string,
				taskpkg.ExecutionRequest,
				taskpkg.ActorContext,
			) (*taskpkg.Execution, error) {
				return nil, taskpkg.ErrInvalidStatusTransition
			},
			StartTaskFn: func(
				context.Context,
				string,
				taskpkg.ExecutionRequest,
				taskpkg.ActorContext,
			) (*taskpkg.Execution, error) {
				return nil, taskpkg.ErrInvalidStatusTransition
			},
			ApproveTaskFn: func(
				context.Context,
				string,
				taskpkg.ExecutionRequest,
				taskpkg.ActorContext,
			) (*taskpkg.Execution, error) {
				return nil, taskpkg.ErrInvalidStatusTransition
			},
			RejectTaskFn: func(context.Context, string, taskpkg.ActorContext) (*taskpkg.Task, error) {
				return nil, taskpkg.ErrTaskNotFound
			},
			MarkTaskReadFn: func(context.Context, string, taskpkg.ActorContext) (taskpkg.TriageState, error) {
				return taskpkg.TriageState{}, taskpkg.ErrTaskNotFound
			},
			RunDetailFn: func(context.Context, string, taskpkg.ActorContext) (*taskpkg.RunDetailView, error) {
				return nil, taskpkg.ErrTaskRunNotFound
			},
			TimelineFn: func(context.Context, string, taskpkg.TimelineQuery, taskpkg.ActorContext) ([]taskpkg.TimelineItem, error) {
				return nil, taskpkg.ErrTaskNotFound
			},
			StreamFn: func(context.Context, string, taskpkg.StreamQuery, taskpkg.ActorContext) (<-chan taskpkg.StreamEvent, error) {
				return nil, taskpkg.ErrTaskNotFound
			},
			TreeFn: func(context.Context, string, taskpkg.ActorContext) (*taskpkg.TreeView, error) {
				return nil, taskpkg.ErrTaskNotFound
			},
		},
		workspaces,
		nil,
		nil,
	)

	if resp := performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks/task-1/publish",
		nil,
	); resp.Code != http.StatusConflict {
		t.Fatalf("publish conflict status = %d, want %d; body=%s", resp.Code, http.StatusConflict, resp.Body.String())
	}
	if resp := performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks/task-1/start",
		nil,
	); resp.Code != http.StatusConflict {
		t.Fatalf("start conflict status = %d, want %d; body=%s", resp.Code, http.StatusConflict, resp.Body.String())
	}
	if resp := performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks/task-1/approve",
		nil,
	); resp.Code != http.StatusConflict {
		t.Fatalf("approve conflict status = %d, want %d; body=%s", resp.Code, http.StatusConflict, resp.Body.String())
	}
	if resp := performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks/task-1/reject",
		nil,
	); resp.Code != http.StatusNotFound {
		t.Fatalf("reject not found status = %d, want %d; body=%s", resp.Code, http.StatusNotFound, resp.Body.String())
	}
	if resp := performRequest(
		t,
		fixture.Engine,
		http.MethodPost,
		"/tasks/task-1/triage/read",
		nil,
	); resp.Code != http.StatusNotFound {
		t.Fatalf(
			"triage read not found status = %d, want %d; body=%s",
			resp.Code,
			http.StatusNotFound,
			resp.Body.String(),
		)
	}
	if resp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/task-runs/run-1",
		nil,
	); resp.Code != http.StatusNotFound {
		t.Fatalf(
			"run detail not found status = %d, want %d; body=%s",
			resp.Code,
			http.StatusNotFound,
			resp.Body.String(),
		)
	}
	if resp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/tasks/task-1/timeline?after_sequence=1",
		nil,
	); resp.Code != http.StatusNotFound {
		t.Fatalf("timeline not found status = %d, want %d; body=%s", resp.Code, http.StatusNotFound, resp.Body.String())
	}
	if resp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/tasks/task-1/stream",
		nil,
	); resp.Code != http.StatusNotFound {
		t.Fatalf("stream not found status = %d, want %d; body=%s", resp.Code, http.StatusNotFound, resp.Body.String())
	}
	if resp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/tasks/task-1/tree",
		nil,
	); resp.Code != http.StatusNotFound {
		t.Fatalf("tree not found status = %d, want %d; body=%s", resp.Code, http.StatusNotFound, resp.Body.String())
	}
	if resp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/observe/tasks/dashboard?scope=workspace&workspace=missing",
		nil,
	); resp.Code != http.StatusNotFound {
		t.Fatalf(
			"dashboard missing workspace status = %d, want %d; body=%s",
			resp.Code,
			http.StatusNotFound,
			resp.Body.String(),
		)
	}
	if resp := performRequest(
		t,
		fixture.Engine,
		http.MethodGet,
		"/observe/tasks/inbox?lane=bogus",
		nil,
	); resp.Code != http.StatusBadRequest {
		t.Fatalf(
			"inbox invalid lane status = %d, want %d; body=%s",
			resp.Code,
			http.StatusBadRequest,
			resp.Body.String(),
		)
	}
	if resp := testutil.PerformRequestWithHeaders(
		t,
		fixture.Engine,
		http.MethodGet,
		"/tasks/task-1/stream",
		nil,
		map[string]string{"Last-Event-ID": "bogus"},
	); resp.Code != http.StatusBadRequest {
		t.Fatalf(
			"stream invalid Last-Event-ID status = %d, want %d; body=%s",
			resp.Code,
			http.StatusBadRequest,
			resp.Body.String(),
		)
	}

	taskless := newHandlerFixture(t, testutil.StubSessionManager{}, testutil.StubObserver{}, workspaces, nil, nil)
	taskless.Handlers.Tasks = nil
	if resp := performRequest(
		t,
		taskless.Engine,
		http.MethodPost,
		"/tasks/task-1/publish",
		nil,
	); resp.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"publish without task service status = %d, want %d; body=%s",
			resp.Code,
			http.StatusServiceUnavailable,
			resp.Body.String(),
		)
	}

	taskless.Handlers.Observer = nil
	if resp := performRequest(
		t,
		taskless.Engine,
		http.MethodGet,
		"/observe/tasks/dashboard",
		nil,
	); resp.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"dashboard without observer status = %d, want %d; body=%s",
			resp.Code,
			http.StatusServiceUnavailable,
			resp.Body.String(),
		)
	}
}
