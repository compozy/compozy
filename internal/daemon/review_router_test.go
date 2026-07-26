package daemon

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestReviewRouterRoutesRunReviewRequests(t *testing.T) {
	t.Parallel()

	t.Run("Should bind an active eligible reviewer and exclude the original worker", func(t *testing.T) {
		t.Parallel()

		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{
					AgentName:            "reviewer",
					AllowedChannelIDs:    []string{"reviews"},
					RequiredCapabilities: []string{"review-pr"},
				},
			},
		}
		sessions := &coordinatorRuntimeSessions{
			infos: []*session.Info{
				reviewRouterSessionInfo("sess-worker", "worker", "reviews"),
				reviewRouterSessionInfo("sess-reviewer", "reviewer", "reviews"),
			},
		}
		router := newReviewRouterForTest(
			t,
			tasks,
			reviewRouterStoreForTest(),
			sessions,
			reviewRouterAgentResolverStub{
				"reviewer": reviewRouterAgentDef("reviewer", "review-pr"),
				"worker":   reviewRouterAgentDef("worker", "build"),
			},
		)

		notification := reviewRouterNotificationForTest()
		router.OnRunReviewRequested(context.Background(), &notification)

		if got, want := len(tasks.binds), 1; got != want {
			t.Fatalf("bind calls = %d, want %d", got, want)
		}
		bind := tasks.binds[0]
		if got, want := bind.SessionID, "sess-reviewer"; got != want {
			t.Fatalf("BindRunReviewSession.SessionID = %q, want %q", got, want)
		}
		if got, want := bind.ReviewerPeerID, "reviewer.sess-reviewer"; got != want {
			t.Fatalf("BindRunReviewSession.ReviewerPeerID = %q, want %q", got, want)
		}
		if len(tasks.records) != 0 {
			t.Fatalf("RecordRunReview calls = %#v, want none", tasks.records)
		}
	})

	t.Run("Should exclude the requesting agent session from active reviewer candidates", func(t *testing.T) {
		t.Parallel()

		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{},
			},
		}
		sessions := &coordinatorRuntimeSessions{
			infos: []*session.Info{
				reviewRouterSessionInfo("sess-worker", "worker", "reviews"),
				reviewRouterSessionInfo("sess-requester", "requester", "reviews"),
				reviewRouterSessionInfo("sess-reviewer", "reviewer", "reviews"),
			},
		}
		router := newReviewRouterForTest(
			t,
			tasks,
			reviewRouterStoreForTest(),
			sessions,
			reviewRouterAgentResolverStub{
				"requester": reviewRouterAgentDef("requester"),
				"reviewer":  reviewRouterAgentDef("reviewer"),
				"worker":    reviewRouterAgentDef("worker"),
			},
		)

		notification := reviewRouterNotificationForTest()
		notification.Actor = taskpkg.ActorContext{
			Actor: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-requester"},
		}
		router.OnRunReviewRequested(context.Background(), &notification)

		if got, want := len(tasks.binds), 1; got != want {
			t.Fatalf("bind calls = %d, want %d", got, want)
		}
		if got, want := tasks.binds[0].SessionID, "sess-reviewer"; got != want {
			t.Fatalf("BindRunReviewSession.SessionID = %q, want %q", got, want)
		}
	})

	t.Run("Should not create a reviewer session using the requesting agent", func(t *testing.T) {
		t.Parallel()

		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{AgentName: "requester"},
			},
		}
		sessions := &coordinatorRuntimeSessions{
			infos: []*session.Info{
				reviewRouterSessionInfo("sess-requester", "requester", "reviews"),
			},
		}
		router := newReviewRouterForTest(
			t,
			tasks,
			reviewRouterStoreForTest(),
			sessions,
			reviewRouterAgentResolverStub{"requester": reviewRouterAgentDef("requester")},
		)

		notification := reviewRouterNotificationForTest()
		notification.Actor = taskpkg.ActorContext{
			Actor: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAgentSession, Ref: "sess-requester"},
		}
		router.OnRunReviewRequested(context.Background(), &notification)

		if got := sessions.createCount(); got != 0 {
			t.Fatalf("session create calls = %d, want 0", got)
		}
		if len(tasks.binds) != 0 {
			t.Fatalf("BindRunReviewSession calls = %#v, want none", tasks.binds)
		}
		if got, want := len(tasks.records), 1; got != want {
			t.Fatalf("RecordRunReview calls = %d, want %d", got, want)
		}
		if !strings.Contains(tasks.records[0].Verdict.Reason, "exclude all eligible reviewer agents") {
			t.Fatalf("RecordRunReview reason = %q, want selector diagnostic", tasks.records[0].Verdict.Reason)
		}
	})

	t.Run("Should create and bind a reviewer session when no active reviewer exists", func(t *testing.T) {
		t.Parallel()

		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{
					AgentName:           "reviewer",
					PreferredChannelIDs: []string{"reviews"},
				},
			},
		}
		sessions := &coordinatorRuntimeSessions{}
		router := newReviewRouterForTest(
			t,
			tasks,
			reviewRouterStoreForTest(),
			sessions,
			reviewRouterAgentResolverStub{"reviewer": reviewRouterAgentDef("reviewer")},
		)

		notification := reviewRouterNotificationForTest()
		router.OnRunReviewRequested(context.Background(), &notification)

		if got, want := sessions.createCount(), 1; got != want {
			t.Fatalf("session create calls = %d, want %d", got, want)
		}
		create := sessions.createCall(0)
		if create.Type != session.SessionTypeSystem {
			t.Fatalf("CreateOpts.Type = %q, want system", create.Type)
		}
		if create.AgentName != "reviewer" || create.NetworkParticipation != nil ||
			create.ResolvedNetworkParticipation != nil {
			t.Fatalf(
				"CreateOpts agent/participation = %q/%#v/%#v, want reviewer with independent Local default",
				create.AgentName,
				create.NetworkParticipation,
				create.ResolvedNetworkParticipation,
			)
		}
		if !strings.Contains(create.PromptOverlay, "references/tasks-and-orchestration.md") ||
			!strings.Contains(create.PromptOverlay, "submit_run_review") {
			t.Fatalf("CreateOpts.PromptOverlay = %q, want reviewer instructions", create.PromptOverlay)
		}
		if got, want := len(tasks.binds), 1; got != want {
			t.Fatalf("bind calls = %d, want %d", got, want)
		}
		if got, want := tasks.binds[0].ReviewerAgentName, "reviewer"; got != want {
			t.Fatalf("bind reviewer agent = %q, want %q", got, want)
		}
	})

	t.Run("Should include task context bundle overlay when creating a reviewer session", func(t *testing.T) {
		t.Parallel()

		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{
					AgentName:           "reviewer",
					PreferredChannelIDs: []string{"reviews"},
				},
			},
		}
		sessions := &coordinatorRuntimeSessions{}
		router := newReviewRouterForTest(
			t,
			tasks,
			reviewRouterStoreForTest(),
			sessions,
			reviewRouterAgentResolverStub{"reviewer": reviewRouterAgentDef("reviewer")},
		)
		overlay := &taskContextOverlayStub{overlay: "review task context bundle"}
		router.contextOverlay = overlay

		notification := reviewRouterNotificationForTest()
		router.OnRunReviewRequested(context.Background(), &notification)

		create := sessions.createCall(0)
		if !strings.Contains(create.PromptOverlay, "review task context bundle") ||
			!strings.Contains(create.PromptOverlay, "submit_run_review") {
			t.Fatalf("CreateOpts.PromptOverlay = %q, want task bundle plus reviewer instructions", create.PromptOverlay)
		}
		if len(overlay.calls) != 1 ||
			overlay.calls[0].taskID != "task-1" ||
			overlay.calls[0].runID != "run-1" {
			t.Fatalf("overlay calls = %#v, want reviewed task/run", overlay.calls)
		}
	})

	t.Run("Should detach routing work from a canceled caller context", func(t *testing.T) {
		t.Parallel()

		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{
					AgentName:            "reviewer",
					AllowedChannelIDs:    []string{"reviews"},
					RequiredCapabilities: []string{"review-pr"},
				},
			},
			rejectCanceledContext: true,
		}
		store := reviewRouterStoreForTest()
		store.rejectCanceledContext = true
		sessions := &coordinatorRuntimeSessions{
			infos: []*session.Info{
				reviewRouterSessionInfo("sess-worker", "worker", "reviews"),
				reviewRouterSessionInfo("sess-reviewer", "reviewer", "reviews"),
			},
		}
		router := newReviewRouterForTest(
			t,
			tasks,
			store,
			sessions,
			reviewRouterAgentResolverStub{
				"reviewer": reviewRouterAgentDef("reviewer", "review-pr"),
				"worker":   reviewRouterAgentDef("worker", "build"),
			},
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		notification := reviewRouterNotificationForTest()
		router.OnRunReviewRequested(ctx, &notification)

		if got, want := len(tasks.binds), 1; got != want {
			t.Fatalf("bind calls = %d, want %d", got, want)
		}
	})

	t.Run("Should attach deadlines to detached routing and no-route diagnostic work", func(t *testing.T) {
		t.Parallel()

		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{
					AllowedPeerIDs: []string{"peer-missing"},
				},
			},
			requireDeadline: true,
		}
		store := reviewRouterStoreForTest()
		store.requireDeadline = true
		sessions := &coordinatorRuntimeSessions{
			infos: []*session.Info{
				reviewRouterSessionInfo("sess-worker", "worker", "reviews"),
			},
		}
		router := newReviewRouterForTest(
			t,
			tasks,
			store,
			sessions,
			reviewRouterAgentResolverStub{"worker": reviewRouterAgentDef("worker")},
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		notification := reviewRouterNotificationForTest()
		router.OnRunReviewRequested(ctx, &notification)

		if len(tasks.binds) != 0 {
			t.Fatalf("BindRunReviewSession calls = %#v, want none", tasks.binds)
		}
		if got, want := len(tasks.records), 1; got != want {
			t.Fatalf("RecordRunReview calls = %d, want %d", got, want)
		}
	})

	t.Run("Should reject Local reviewer creation when explicit channels allow no active candidate", func(t *testing.T) {
		t.Parallel()

		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{
					AgentName:         "reviewer",
					AllowedChannelIDs: []string{"reviews"},
				},
			},
		}
		sessions := &coordinatorRuntimeSessions{
			infos: []*session.Info{reviewRouterSessionInfo("sess-worker", "worker", "reviews")},
		}
		router := newReviewRouterForTest(
			t,
			tasks,
			reviewRouterStoreForTest(),
			sessions,
			reviewRouterAgentResolverStub{
				"reviewer": reviewRouterAgentDef("reviewer"),
				"worker":   reviewRouterAgentDef("worker"),
			},
		)

		notification := reviewRouterNotificationForTest()
		router.OnRunReviewRequested(context.Background(), &notification)

		if got := sessions.createCount(); got != 0 {
			t.Fatalf("session create calls = %d, want 0", got)
		}
		if len(tasks.binds) != 0 {
			t.Fatalf("BindRunReviewSession calls = %#v, want none", tasks.binds)
		}
		if got, want := len(tasks.records), 1; got != want {
			t.Fatalf("RecordRunReview calls = %d, want %d", got, want)
		}
		if !strings.Contains(tasks.records[0].Verdict.Reason, "only explicit channels") {
			t.Fatalf("RecordRunReview reason = %q, want channel allowlist diagnostic", tasks.records[0].Verdict.Reason)
		}
	})

	t.Run("Should rebind in review requests when reviewer session stops", func(t *testing.T) {
		t.Parallel()

		stoppedReview := reviewRouterNotificationForTest().Review
		stoppedReview.Status = taskpkg.RunReviewStatusInReview
		stoppedReview.ReviewerSessionID = "sess-stopped-reviewer"
		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{},
			},
			listReviews: []taskpkg.RunReview{stoppedReview},
		}
		sessions := &coordinatorRuntimeSessions{
			infos: []*session.Info{
				reviewRouterSessionInfo("sess-worker", "worker", "reviews"),
				stoppedReviewRouterSessionInfo("sess-stopped-reviewer", "reviewer", "reviews"),
				reviewRouterSessionInfo("sess-active-reviewer", "reviewer", "reviews"),
			},
		}
		router := newReviewRouterForTest(
			t,
			tasks,
			reviewRouterStoreForTest(),
			sessions,
			reviewRouterAgentResolverStub{
				"worker":   reviewRouterAgentDef("worker"),
				"reviewer": reviewRouterAgentDef("reviewer"),
			},
		)

		router.OnSessionStopped(context.Background(), &session.Session{
			ID:                   "sess-stopped-reviewer",
			AgentName:            "reviewer",
			WorkspaceID:          "ws-1",
			Workspace:            "ws-1",
			NetworkParticipation: daemonTestLiveParticipation("ws-1", "reviews"),
			Type:                 session.SessionTypeSystem,
			State:                session.StateStopped,
		})

		if got, want := len(tasks.listQueries), 1; got != want {
			t.Fatalf("ListRunReviews calls = %d, want %d", got, want)
		}
		query := tasks.listQueries[0]
		if query.Status != taskpkg.RunReviewStatusInReview ||
			query.ReviewerSessionID != "sess-stopped-reviewer" {
			t.Fatalf("ListRunReviews query = %#v, want stopped in-review binding", query)
		}
		if got, want := len(tasks.binds), 1; got != want {
			t.Fatalf("BindRunReviewSession calls = %d, want %d", got, want)
		}
		if got, want := tasks.binds[0].SessionID, "sess-active-reviewer"; got != want {
			t.Fatalf("BindRunReviewSession.SessionID = %q, want %q", got, want)
		}
		if got := len(tasks.records); got != 0 {
			t.Fatalf("RecordRunReview calls = %d, want 0 after rebind", got)
		}
	})

	t.Run("Should record blocked diagnostic when stopped reviewer has no recovery route", func(t *testing.T) {
		t.Parallel()

		stoppedReview := reviewRouterNotificationForTest().Review
		stoppedReview.Status = taskpkg.RunReviewStatusInReview
		stoppedReview.ReviewerSessionID = "sess-stopped-reviewer"
		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{
					AllowedPeerIDs: []string{"missing-peer"},
				},
			},
			listReviews: []taskpkg.RunReview{stoppedReview},
		}
		sessions := &coordinatorRuntimeSessions{
			infos: []*session.Info{
				reviewRouterSessionInfo("sess-worker", "worker", "reviews"),
				stoppedReviewRouterSessionInfo("sess-stopped-reviewer", "reviewer", "reviews"),
			},
		}
		router := newReviewRouterForTest(
			t,
			tasks,
			reviewRouterStoreForTest(),
			sessions,
			reviewRouterAgentResolverStub{
				"worker":   reviewRouterAgentDef("worker"),
				"reviewer": reviewRouterAgentDef("reviewer"),
			},
		)

		router.OnSessionStopped(context.Background(), &session.Session{
			ID:                   "sess-stopped-reviewer",
			AgentName:            "reviewer",
			WorkspaceID:          "ws-1",
			Workspace:            "ws-1",
			NetworkParticipation: daemonTestLiveParticipation("ws-1", "reviews"),
			Type:                 session.SessionTypeSystem,
			State:                session.StateStopped,
		})

		if len(tasks.binds) != 0 {
			t.Fatalf("BindRunReviewSession calls = %#v, want none", tasks.binds)
		}
		if got, want := len(tasks.records), 1; got != want {
			t.Fatalf("RecordRunReview calls = %d, want %d", got, want)
		}
		record := tasks.records[0]
		if got, want := record.Verdict.Outcome, taskpkg.RunReviewOutcomeBlocked; got != want {
			t.Fatalf("RecordRunReview outcome = %q, want %q", got, want)
		}
		if !strings.Contains(record.Verdict.Reason, "sess-stopped-reviewer") ||
			!strings.Contains(record.Verdict.Reason, "allows only explicit peers") {
			t.Fatalf("RecordRunReview reason = %q, want stopped-session no-route diagnostic", record.Verdict.Reason)
		}
	})

	t.Run("Should stop a created reviewer session when binding fails", func(t *testing.T) {
		t.Parallel()

		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{
					AgentName: "reviewer",
				},
			},
			bindErr: context.DeadlineExceeded,
		}
		sessions := &reviewRouterDeadlineSessions{base: &coordinatorRuntimeSessions{}}
		router := newReviewRouterForTest(
			t,
			tasks,
			reviewRouterStoreForTest(),
			sessions,
			reviewRouterAgentResolverStub{"reviewer": reviewRouterAgentDef("reviewer")},
		)

		notification := reviewRouterNotificationForTest()
		router.OnRunReviewRequested(context.Background(), &notification)

		if got, want := sessions.createCount(), 1; got != want {
			t.Fatalf("session create calls = %d, want %d", got, want)
		}
		if got, want := sessions.stopCount(), 1; got != want {
			t.Fatalf("session stop calls = %d, want %d", got, want)
		}
		stop := sessions.stopCall(0)
		if stop.id == "" || stop.cause != session.CauseFailed || !strings.Contains(stop.detail, "bind failed") {
			t.Fatalf("stop call = %#v, want failed cleanup for created reviewer", stop)
		}
		if !sessions.stopHasDeadline() {
			t.Fatal("StopWithCause() context had no deadline, want cleanup timeout")
		}
		if err := sessions.stopContextErr(); err != nil {
			t.Fatalf("StopWithCause() context err = %v, want detached timeout context", err)
		}
		if got := len(tasks.records); got != 0 {
			t.Fatalf("RecordRunReview calls = %d, want 0 after transient bind failure", got)
		}
	})

	t.Run("Should reject reviewer creation when the only candidate is the original worker agent", func(t *testing.T) {
		t.Parallel()

		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{
					AgentName: "worker",
				},
			},
		}
		sessions := &coordinatorRuntimeSessions{
			infos: []*session.Info{
				reviewRouterSessionInfo("sess-worker", "worker", "reviews"),
			},
		}
		router := newReviewRouterForTest(
			t,
			tasks,
			reviewRouterStoreForTest(),
			sessions,
			reviewRouterAgentResolverStub{"worker": reviewRouterAgentDef("worker")},
		)

		notification := reviewRouterNotificationForTest()
		router.OnRunReviewRequested(context.Background(), &notification)

		if got := sessions.createCount(); got != 0 {
			t.Fatalf("session create calls = %d, want 0 when only original worker agent is available", got)
		}
		if len(tasks.binds) != 0 {
			t.Fatalf("BindRunReviewSession calls = %#v, want none", tasks.binds)
		}
		if got, want := len(tasks.records), 1; got != want {
			t.Fatalf("RecordRunReview calls = %d, want %d", got, want)
		}
		if !strings.Contains(tasks.records[0].Verdict.Reason, "exclude all eligible reviewer agents") {
			t.Fatalf(
				"RecordRunReview reason = %q, want original-worker exclusion diagnostic",
				tasks.records[0].Verdict.Reason,
			)
		}
	})

	t.Run(
		"Should record a deterministic no-route diagnostic instead of binding the original worker",
		func(t *testing.T) {
			t.Parallel()

			tasks := &reviewRouterTasksStub{
				profile: taskpkg.ExecutionProfile{
					TaskID: "task-1",
					Review: taskpkg.ReviewProfile{
						AllowedPeerIDs: []string{"peer-missing"},
					},
				},
			}
			sessions := &coordinatorRuntimeSessions{
				infos: []*session.Info{
					reviewRouterSessionInfo("sess-worker", "worker", "reviews"),
				},
			}
			router := newReviewRouterForTest(
				t,
				tasks,
				reviewRouterStoreForTest(),
				sessions,
				reviewRouterAgentResolverStub{"worker": reviewRouterAgentDef("worker")},
			)

			notification := reviewRouterNotificationForTest()
			router.OnRunReviewRequested(context.Background(), &notification)

			if len(tasks.binds) != 0 {
				t.Fatalf("BindRunReviewSession calls = %#v, want none", tasks.binds)
			}
			if got, want := len(tasks.records), 1; got != want {
				t.Fatalf("RecordRunReview calls = %d, want %d", got, want)
			}
			record := tasks.records[0]
			if got, want := record.Verdict.Outcome, taskpkg.RunReviewOutcomeBlocked; got != want {
				t.Fatalf("RecordRunReview outcome = %q, want %q", got, want)
			}
			if !strings.HasPrefix(record.Verdict.DeliveryID, reviewRouterNoRouteDeliveryPrefix) {
				t.Fatalf(
					"RecordRunReview delivery_id = %q, want review-router no-route prefix",
					record.Verdict.DeliveryID,
				)
			}
			if !strings.Contains(record.Verdict.Reason, "allows only explicit peers") {
				t.Fatalf("RecordRunReview reason = %q, want deterministic selector diagnostic", record.Verdict.Reason)
			}
		},
	)

	t.Run("Should not record a no-route diagnostic when routing fails transiently", func(t *testing.T) {
		t.Parallel()

		tasks := &reviewRouterTasksStub{
			profile: taskpkg.ExecutionProfile{
				TaskID: "task-1",
				Review: taskpkg.ReviewProfile{
					AllowedPeerIDs: []string{"peer-missing"},
				},
			},
		}
		store := reviewRouterStoreForTest()
		store.runErr = context.DeadlineExceeded
		sessions := &coordinatorRuntimeSessions{
			infos: []*session.Info{
				reviewRouterSessionInfo("sess-worker", "worker", "reviews"),
			},
		}
		router := newReviewRouterForTest(
			t,
			tasks,
			store,
			sessions,
			reviewRouterAgentResolverStub{"worker": reviewRouterAgentDef("worker")},
		)

		notification := reviewRouterNotificationForTest()
		router.OnRunReviewRequested(context.Background(), &notification)

		if len(tasks.binds) != 0 {
			t.Fatalf("BindRunReviewSession calls = %#v, want none", tasks.binds)
		}
		if got := len(tasks.records); got != 0 {
			t.Fatalf("RecordRunReview calls = %d, want 0 after transient routing failure", got)
		}
	})
}

func newReviewRouterForTest(
	t *testing.T,
	tasks *reviewRouterTasksStub,
	store *reviewRouterStoreStub,
	sessions reviewRouterSessionManager,
	agents reviewRouterAgentResolverStub,
) *reviewRouter {
	t.Helper()
	router, err := newReviewRouter(
		tasks,
		store,
		sessions,
		nil,
		agents,
		discardLogger(),
		func() time.Time { return time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatalf("newReviewRouter() error = %v", err)
	}
	return router
}

func reviewRouterStoreForTest() *reviewRouterStoreStub {
	return &reviewRouterStoreStub{
		taskRecord: taskpkg.Task{
			ID:          "task-1",
			Scope:       taskpkg.ScopeWorkspace,
			WorkspaceID: "ws-1",
			Status:      taskpkg.TaskStatusReady,
			Title:       "Review routed task",
		},
		run: taskpkg.Run{
			ID:              "run-1",
			TaskID:          "task-1",
			Status:          taskpkg.TaskRunStatusCompleted,
			SessionID:       "sess-worker",
			RunNetworkState: &taskpkg.RunNetworkState{NetworkSpec: daemonTestLiveParticipation("ws-1", "reviews")},
		},
	}
}

func reviewRouterNotificationForTest() taskpkg.RunReviewRequestedNotification {
	return taskpkg.RunReviewRequestedNotification{
		Review: taskpkg.RunReview{
			ReviewID:    "review-1",
			TaskID:      "task-1",
			RunID:       "run-1",
			Policy:      taskpkg.ReviewPolicyAlways,
			ReviewRound: 1,
			Attempt:     1,
			Status:      taskpkg.RunReviewStatusRequested,
		},
	}
}

func reviewRouterSessionInfo(id string, agent string, channel string) *session.Info {
	return &session.Info{
		ID:                   id,
		AgentName:            agent,
		WorkspaceID:          "ws-1",
		Workspace:            "ws-1",
		NetworkParticipation: daemonTestLiveParticipation("ws-1", channel),
		Type:                 session.SessionTypeSystem,
		State:                session.StateActive,
	}
}

func stoppedReviewRouterSessionInfo(id string, agent string, channel string) *session.Info {
	info := reviewRouterSessionInfo(id, agent, channel)
	info.State = session.StateStopped
	return info
}

func reviewRouterAgentDef(name string, capabilities ...string) aghconfig.AgentDef {
	agent := aghconfig.AgentDef{Name: name, Provider: "codex", Model: "gpt-5"}
	if len(capabilities) == 0 {
		return agent
	}
	agent.Capabilities = &aghconfig.CapabilityCatalog{
		Capabilities: make([]aghconfig.CapabilityDef, 0, len(capabilities)),
	}
	for _, capability := range capabilities {
		agent.Capabilities.Capabilities = append(agent.Capabilities.Capabilities, aghconfig.CapabilityDef{
			ID:      capability,
			Summary: capability,
			Outcome: capability,
		})
	}
	return agent
}

type reviewRouterStoreStub struct {
	taskRecord            taskpkg.Task
	run                   taskpkg.Run
	taskErr               error
	runErr                error
	rejectCanceledContext bool
	requireDeadline       bool
}

func (s *reviewRouterStoreStub) GetTask(ctx context.Context, id string) (taskpkg.Task, error) {
	if s.taskErr != nil {
		return taskpkg.Task{}, s.taskErr
	}
	if s.rejectCanceledContext && ctx != nil && ctx.Err() != nil {
		return taskpkg.Task{}, ctx.Err()
	}
	if s.requireDeadline {
		if _, ok := ctx.Deadline(); !ok {
			return taskpkg.Task{}, context.DeadlineExceeded
		}
	}
	if id != s.taskRecord.ID {
		return taskpkg.Task{}, taskpkg.ErrTaskNotFound
	}
	return s.taskRecord, nil
}

func (s *reviewRouterStoreStub) GetTaskRun(ctx context.Context, id string) (taskpkg.Run, error) {
	if s.runErr != nil {
		return taskpkg.Run{}, s.runErr
	}
	if s.rejectCanceledContext && ctx != nil && ctx.Err() != nil {
		return taskpkg.Run{}, ctx.Err()
	}
	if s.requireDeadline {
		if _, ok := ctx.Deadline(); !ok {
			return taskpkg.Run{}, context.DeadlineExceeded
		}
	}
	if id != s.run.ID {
		return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
	}
	return s.run, nil
}

type reviewRouterTasksStub struct {
	mu                    sync.Mutex
	profile               taskpkg.ExecutionProfile
	listReviews           []taskpkg.RunReview
	listQueries           []taskpkg.RunReviewQuery
	binds                 []taskpkg.BindRunReviewSessionRequest
	records               []taskpkg.RecordRunReviewRequest
	listErr               error
	bindErr               error
	rejectCanceledContext bool
	requireDeadline       bool
}

func (s *reviewRouterTasksStub) GetExecutionProfile(
	ctx context.Context,
	_ string,
	_ taskpkg.ActorContext,
) (taskpkg.ExecutionProfile, error) {
	if s.rejectCanceledContext && ctx != nil && ctx.Err() != nil {
		return taskpkg.ExecutionProfile{}, ctx.Err()
	}
	if s.requireDeadline {
		if _, ok := ctx.Deadline(); !ok {
			return taskpkg.ExecutionProfile{}, context.DeadlineExceeded
		}
	}
	return s.profile, nil
}

func (s *reviewRouterTasksStub) ListRunReviews(
	ctx context.Context,
	query taskpkg.RunReviewQuery,
	_ taskpkg.ActorContext,
) ([]taskpkg.RunReview, error) {
	if s.rejectCanceledContext && ctx != nil && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if s.requireDeadline {
		if _, ok := ctx.Deadline(); !ok {
			return nil, context.DeadlineExceeded
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listQueries = append(s.listQueries, query)
	if s.listErr != nil {
		return nil, s.listErr
	}
	reviews := make([]taskpkg.RunReview, len(s.listReviews))
	copy(reviews, s.listReviews)
	return reviews, nil
}

func (s *reviewRouterTasksStub) BindRunReviewSession(
	ctx context.Context,
	req taskpkg.BindRunReviewSessionRequest,
	_ taskpkg.ActorContext,
) (taskpkg.RunReviewBinding, error) {
	if s.rejectCanceledContext && ctx != nil && ctx.Err() != nil {
		return taskpkg.RunReviewBinding{}, ctx.Err()
	}
	if s.requireDeadline {
		if _, ok := ctx.Deadline(); !ok {
			return taskpkg.RunReviewBinding{}, context.DeadlineExceeded
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.bindErr != nil {
		return taskpkg.RunReviewBinding{}, s.bindErr
	}
	s.binds = append(s.binds, req)
	return taskpkg.RunReviewBinding{SessionID: req.SessionID}, nil
}

func (s *reviewRouterTasksStub) RecordRunReview(
	ctx context.Context,
	req taskpkg.RecordRunReviewRequest,
	_ taskpkg.ActorContext,
) (taskpkg.RunReviewResult, error) {
	if s.rejectCanceledContext && ctx != nil && ctx.Err() != nil {
		return taskpkg.RunReviewResult{}, ctx.Err()
	}
	if s.requireDeadline {
		if _, ok := ctx.Deadline(); !ok {
			return taskpkg.RunReviewResult{}, context.DeadlineExceeded
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, req)
	return taskpkg.RunReviewResult{
		Review: taskpkg.RunReview{
			ReviewID: req.ReviewID,
			RunID:    req.RunID,
			Status:   taskpkg.RunReviewStatusRecorded,
			Outcome:  req.Verdict.Outcome,
		},
	}, nil
}

type reviewRouterAgentResolverStub map[string]aghconfig.AgentDef

func (s reviewRouterAgentResolverStub) ResolveAgent(
	name string,
	_ *workspacepkg.ResolvedWorkspace,
) (aghconfig.AgentDef, error) {
	agent, ok := s[strings.TrimSpace(name)]
	if !ok {
		return aghconfig.AgentDef{}, workspacepkg.ErrAgentNotAvailable
	}
	return agent, nil
}

type reviewRouterDeadlineSessions struct {
	base        *coordinatorRuntimeSessions
	mu          sync.Mutex
	hasDeadline bool
	ctxErr      error
}

func (s *reviewRouterDeadlineSessions) Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error) {
	return s.base.Create(ctx, opts)
}

func (s *reviewRouterDeadlineSessions) ListAll(ctx context.Context) ([]*session.Info, error) {
	return s.base.ListAll(ctx)
}

func (s *reviewRouterDeadlineSessions) StopWithCause(
	ctx context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	s.mu.Lock()
	s.hasDeadline = false
	if _, ok := ctx.Deadline(); ok {
		s.hasDeadline = true
	}
	s.ctxErr = ctx.Err()
	s.mu.Unlock()
	return s.base.StopWithCause(ctx, id, cause, detail)
}

func (s *reviewRouterDeadlineSessions) createCount() int {
	return s.base.createCount()
}

func (s *reviewRouterDeadlineSessions) stopCount() int {
	return s.base.stopCount()
}

func (s *reviewRouterDeadlineSessions) stopCall(index int) coordinatorRuntimeStopCall {
	return s.base.stopCall(index)
}

func (s *reviewRouterDeadlineSessions) stopHasDeadline() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hasDeadline
}

func (s *reviewRouterDeadlineSessions) stopContextErr() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctxErr
}
