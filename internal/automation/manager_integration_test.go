//go:build integration

package automation

import (
	"context"
	"errors"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/network/participation"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	"github.com/jonboulle/clockwork"
)

func TestManagerIntegrationAcceptedSuggestionDispatchesThroughScheduler(t *testing.T) {
	t.Parallel()

	h := newManagerResourceHarness(t)
	fakeClock := clockwork.NewFakeClockAt(time.Date(2026, 7, 20, 7, 59, 0, 0, time.UTC))
	manager := h.newManager(
		t,
		integrationAutomationConfig(),
		WithResourceDefinitions(h.jobStore, h.triggerStore, h.actor, nil),
		WithManagerNow(fakeClock.Now),
		WithDispatcherOptions(WithDispatcherNow(fakeClock.Now)),
		WithSchedulerOptions(WithSchedulerClock(fakeClock)),
	)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	suggestions, err := manager.ListSuggestions(
		h.ctx,
		h.workspace.ID,
		SuggestionStatusPending,
	)
	if err != nil {
		t.Fatalf("manager.ListSuggestions() error = %v", err)
	}
	var daily Suggestion
	for _, suggestion := range suggestions {
		if suggestion.Payload.Name == "Daily workspace briefing" {
			daily = suggestion
			break
		}
	}
	if daily.ID == "" {
		t.Fatalf("manager.ListSuggestions() = %#v, want daily workspace briefing", suggestions)
	}
	accepted, err := manager.AcceptSuggestion(h.ctx, h.workspace.ID, daily.ID)
	if err != nil {
		t.Fatalf("manager.AcceptSuggestion() error = %v", err)
	}
	if got, want := accepted.Job.ID, suggestionJobID(daily.WorkspaceID, daily.DedupKey); got != want {
		t.Fatalf("accepted Job ID = %q, want %q", got, want)
	}
	state, err := manager.schedulerSnapshot().State(accepted.Job.ID)
	if err != nil {
		t.Fatalf("scheduler.State() error = %v", err)
	}
	wantNextRun := time.Date(2026, 7, 20, 8, 0, 0, 0, time.UTC)
	if state.NextRun == nil || !state.NextRun.Equal(wantNextRun) {
		t.Fatalf("scheduler.State().NextRun = %v, want %v", state.NextRun, wantNextRun)
	}

	waitForTimers(t, fakeClock, 1)
	fakeClock.Advance(time.Minute)
	waitUntil(t, 2*time.Second, 25*time.Millisecond, func() bool {
		runs, listErr := h.db.ListRuns(h.ctx, RunQuery{JobID: accepted.Job.ID})
		if listErr != nil {
			t.Fatalf("ListRuns() error = %v", listErr)
		}
		return len(runs) == 1
	})
	runs, err := h.db.ListRuns(h.ctx, RunQuery{JobID: accepted.Job.ID})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if got, want := runs[0].Status, RunCompleted; got != want {
		t.Fatalf("ListRuns()[0].Status = %q, want %q; run = %#v", got, want, runs[0])
	}
	if got := len(h.sessions.creator.promptCalls()); got != 1 {
		t.Fatalf("len(Prompt calls) = %d, want 1", got)
	}
}

func TestManagerIntegrationDirectTaskBackedJobDelegatesIntoTaskDomain(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	taskManager, err := taskpkg.NewManager(
		taskpkg.WithStore(h.db),
		taskpkg.WithParticipationResolver(automationIntegrationParticipationResolver(t)),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}

	manager := h.newManager(t, integrationAutomationConfig(), WithTasks(taskManager))
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	job, err := manager.CreateJob(h.ctx, Job{
		Scope:       AutomationScopeWorkspace,
		Name:        "direct-task-backed",
		WorkspaceID: h.workspace.ID,
		Schedule: &ScheduleSpec{
			Mode:     ScheduleModeEvery,
			Interval: "1h",
		},
		Task: &JobTaskConfig{
			Title:                "Direct automation review",
			Description:          "Persist a durable review task.",
			NetworkParticipation: automationRunParticipation(),
			Owner: &taskpkg.Ownership{
				Kind: taskpkg.OwnerKindAutomation,
				Ref:  "job:direct-task-backed",
			},
		},
		Enabled:   true,
		Retry:     DefaultRetryConfig(),
		FireLimit: DefaultFireLimitConfig(),
	})
	if err != nil {
		t.Fatalf("manager.CreateJob() error = %v", err)
	}

	run, err := manager.TriggerJob(h.ctx, job.ID)
	if err != nil {
		t.Fatalf("manager.TriggerJob() error = %v", err)
	}
	if got, want := run.Status, RunDelegated; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
	if got := run.SessionID; got != "" {
		t.Fatalf("run.SessionID = %q, want empty", got)
	}
	if got, want := len(h.sessions.creator.createCalls()), 0; got != want {
		t.Fatalf("len(Create calls) = %d, want %d", got, want)
	}
	if got, want := len(h.sessions.creator.promptCalls()), 0; got != want {
		t.Fatalf("len(Prompt calls) = %d, want %d", got, want)
	}

	storedRun, err := h.db.GetRun(h.ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if got, want := storedRun.TaskID, run.TaskID; got != want {
		t.Fatalf("storedRun.TaskID = %q, want %q", got, want)
	}
	if got, want := storedRun.TaskRunID, run.TaskRunID; got != want {
		t.Fatalf("storedRun.TaskRunID = %q, want %q", got, want)
	}
	if storedRun.NetworkParticipation == nil || storedRun.NetworkParticipation.ChannelStrategy == nil ||
		*storedRun.NetworkParticipation.ChannelStrategy != participation.StrategyRun {
		t.Fatalf("storedRun.NetworkParticipation = %#v, want run strategy", storedRun.NetworkParticipation)
	}

	taskRecord, err := h.db.GetTask(h.ctx, run.TaskID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got, want := taskRecord.Scope, taskpkg.ScopeWorkspace; got != want {
		t.Fatalf("task.Scope = %q, want %q", got, want)
	}
	if got, want := taskRecord.WorkspaceID, h.workspace.ID; got != want {
		t.Fatalf("task.WorkspaceID = %q, want %q", got, want)
	}
	if taskRecord.Owner == nil || taskRecord.Owner.Kind != taskpkg.OwnerKindAutomation ||
		taskRecord.Owner.Ref != "job:direct-task-backed" {
		t.Fatalf("task.Owner = %#v, want automation owner", taskRecord.Owner)
	}
	if got, want := taskRecord.CreatedBy.Kind, taskpkg.ActorKindAutomation; got != want {
		t.Fatalf("task.CreatedBy.Kind = %q, want %q", got, want)
	}
	if got, want := taskRecord.CreatedBy.Ref, job.ID; got != want {
		t.Fatalf("task.CreatedBy.Ref = %q, want %q", got, want)
	}
	if got, want := taskRecord.Origin.Kind, taskpkg.OriginKindAutomation; got != want {
		t.Fatalf("task.Origin.Kind = %q, want %q", got, want)
	}
	if got, want := taskRecord.Origin.Ref, "run:"+run.ID; got != want {
		t.Fatalf("task.Origin.Ref = %q, want %q", got, want)
	}

	taskRun, err := h.db.GetTaskRun(h.ctx, run.TaskRunID)
	if err != nil {
		t.Fatalf("GetTaskRun() error = %v", err)
	}
	if got, want := taskRun.Origin.Kind, taskpkg.OriginKindAutomation; got != want {
		t.Fatalf("taskRun.Origin.Kind = %q, want %q", got, want)
	}
	if got, want := taskRun.Origin.Ref, "run:"+run.ID; got != want {
		t.Fatalf("taskRun.Origin.Ref = %q, want %q", got, want)
	}
	if got, want := taskRun.IdempotencyKey, "automation-run:"+run.ID; got != want {
		t.Fatalf("taskRun.IdempotencyKey = %q, want %q", got, want)
	}
	snapshot := taskRun.NetworkSpecSnapshot()
	if snapshot.Mode != participation.ModeLive || snapshot.Source != participation.SourceAutomationJob {
		t.Fatalf("taskRun.NetworkSpec = %#v, want live automation-job resolution", snapshot)
	}
	if snapshot.ChannelStrategy != participation.StrategyRun || snapshot.ChannelID == "" {
		t.Fatalf("taskRun.NetworkSpec = %#v, want derived run channel", snapshot)
	}
}

func TestManagerIntegrationTaskTargetDefaultsLocalWithoutDeclaration(t *testing.T) {
	t.Parallel()

	t.Run("Should materialize a Local task run", func(t *testing.T) {
		t.Parallel()

		h := newManagerHarness(t)
		taskManager, err := taskpkg.NewManager(taskpkg.WithStore(h.db))
		if err != nil {
			t.Fatalf("task.NewManager() error = %v", err)
		}
		manager := h.newManager(t, integrationAutomationConfig(), WithTasks(taskManager))
		if err := manager.Start(h.ctx); err != nil {
			t.Fatalf("manager.Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("manager.Shutdown() error = %v", err)
			}
		})

		job, err := manager.CreateJob(h.ctx, Job{
			Scope:       AutomationScopeWorkspace,
			Name:        "local-task-target",
			WorkspaceID: h.workspace.ID,
			Schedule: &ScheduleSpec{
				Mode:     ScheduleModeEvery,
				Interval: "1h",
			},
			Task:      &JobTaskConfig{Title: "Local automation task"},
			Enabled:   true,
			Retry:     DefaultRetryConfig(),
			FireLimit: DefaultFireLimitConfig(),
		})
		if err != nil {
			t.Fatalf("CreateJob() error = %v", err)
		}
		run, err := manager.TriggerJob(h.ctx, job.ID)
		if err != nil {
			t.Fatalf("TriggerJob() error = %v", err)
		}
		if run.NetworkParticipation != nil {
			t.Fatalf("automation run participation = %#v, want absent request", run.NetworkParticipation)
		}
		taskRun, err := h.db.GetTaskRun(h.ctx, run.TaskRunID)
		if err != nil {
			t.Fatalf("GetTaskRun() error = %v", err)
		}
		if got := taskRun.NetworkSpecSnapshot(); got != participation.LocalSpec() {
			t.Fatalf("task run participation = %#v, want canonical Local", got)
		}
	})
}

func automationRunParticipation() *participation.Request {
	mode := participation.ModeLive
	strategy := participation.StrategyRun
	return &participation.Request{Mode: &mode, ChannelStrategy: &strategy}
}

func automationIntegrationParticipationResolver(t *testing.T) participation.Resolver {
	t.Helper()
	resolver, err := participation.NewResolver(participation.ResolverOptions{
		Defaults: participation.Bounds{
			MaxWakes:         4,
			MaxWakeWallTime:  "30s",
			MaxTotalWallTime: "2m",
			MaxInputTokens:   4096,
			MaxOutputTokens:  4096,
			MaxWakeDepth:     4,
			CoalesceWindow:   "250ms",
		},
		Limits: participation.Limits{
			MaxWakes:          16,
			MaxWakeWallTime:   "2m",
			MaxTotalWallTime:  "10m",
			MaxInputTokens:    65536,
			MaxOutputTokens:   65536,
			MaxWakeDepth:      16,
			MinCoalesceWindow: "100ms",
			MaxCoalesceWindow: "5s",
		},
	})
	if err != nil {
		t.Fatalf("participation.NewResolver() error = %v", err)
	}
	return resolver
}

func TestManagerIntegrationAutomationSessionCanCreateTaskWithAutomationOrigin(t *testing.T) {
	t.Parallel()

	promptStarted := make(chan struct{}, 1)
	promptRelease := make(chan struct{})

	h := newManagerHarness(t)
	h.sessions = newManagerSessionStub(sessionAttemptPlan{
		sessionID:     "sess-automation-agent",
		workspaceID:   h.workspace.ID,
		promptStarted: promptStarted,
		promptRelease: promptRelease,
	})

	taskManager, err := taskpkg.NewManager(taskpkg.WithStore(h.db))
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}

	manager := h.newManager(t, integrationAutomationConfig(), WithTasks(taskManager))
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	job, err := manager.CreateJob(h.ctx, Job{
		Scope:       AutomationScopeWorkspace,
		Name:        "agent-mediated-task-create",
		AgentName:   "researcher",
		WorkspaceID: h.workspace.ID,
		Prompt:      "Inspect the repo and decide whether to create a task.",
		Schedule: &ScheduleSpec{
			Mode:     ScheduleModeEvery,
			Interval: "1h",
		},
		Enabled:   true,
		Retry:     DefaultRetryConfig(),
		FireLimit: DefaultFireLimitConfig(),
	})
	if err != nil {
		t.Fatalf("manager.CreateJob() error = %v", err)
	}

	runCh := make(chan Run, 1)
	errCh := make(chan error, 1)
	go func() {
		run, err := manager.TriggerJob(h.ctx, job.ID)
		runCh <- run
		errCh <- err
	}()

	select {
	case <-promptStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("automation session did not reach Prompt() in time")
	}

	runs, err := h.db.ListRuns(h.ctx, RunQuery{JobID: job.ID})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
	if got, want := runs[0].Status, RunRunning; got != want {
		t.Fatalf("runs[0].Status = %q, want %q", got, want)
	}
	if got, want := runs[0].SessionID, "sess-automation-agent"; got != want {
		t.Fatalf("runs[0].SessionID = %q, want %q", got, want)
	}

	actor, err := manager.TaskActorContextForSession("sess-automation-agent")
	if err != nil {
		t.Fatalf("TaskActorContextForSession() error = %v", err)
	}
	if got, want := actor.Actor.Kind, taskpkg.ActorKindAgentSession; got != want {
		t.Fatalf("actor.Actor.Kind = %q, want %q", got, want)
	}
	if got, want := actor.Origin.Kind, taskpkg.OriginKindAutomation; got != want {
		t.Fatalf("actor.Origin.Kind = %q, want %q", got, want)
	}
	if got, want := actor.Origin.Ref, "run:"+runs[0].ID; got != want {
		t.Fatalf("actor.Origin.Ref = %q, want %q", got, want)
	}

	createdTask, err := taskManager.CreateTask(h.ctx, taskpkg.CreateTask{
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: h.workspace.ID,
		Title:       "Agent-created follow-up",
	}, actor)
	if err != nil {
		t.Fatalf("taskManager.CreateTask() error = %v", err)
	}

	storedTask, err := h.db.GetTask(h.ctx, createdTask.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got, want := storedTask.CreatedBy.Kind, taskpkg.ActorKindAgentSession; got != want {
		t.Fatalf("storedTask.CreatedBy.Kind = %q, want %q", got, want)
	}
	if got, want := storedTask.CreatedBy.Ref, "sess-automation-agent"; got != want {
		t.Fatalf("storedTask.CreatedBy.Ref = %q, want %q", got, want)
	}
	if got, want := storedTask.Origin.Kind, taskpkg.OriginKindAutomation; got != want {
		t.Fatalf("storedTask.Origin.Kind = %q, want %q", got, want)
	}
	if got, want := storedTask.Origin.Ref, "run:"+runs[0].ID; got != want {
		t.Fatalf("storedTask.Origin.Ref = %q, want %q", got, want)
	}

	close(promptRelease)

	var completedRun Run
	select {
	case completedRun = <-runCh:
	case <-time.After(2 * time.Second):
		t.Fatal("manager.TriggerJob() did not return after prompt release")
	}
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("manager.TriggerJob() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("manager.TriggerJob() error channel did not return")
	}
	if got, want := completedRun.Status, RunCompleted; got != want {
		t.Fatalf("completedRun.Status = %q, want %q", got, want)
	}

	if _, err := manager.TaskActorContextForSession(
		"sess-automation-agent",
	); !errors.Is(
		err,
		ErrSessionTaskActorNotFound,
	) {
		t.Fatalf("TaskActorContextForSession(after completion) error = %v, want ErrSessionTaskActorNotFound", err)
	}
}

func TestManagerIntegrationManualTriggerSurvivesCallerCancellation(t *testing.T) {
	t.Parallel()

	t.Run("Should survive caller cancellation after prompt starts", func(t *testing.T) {
		t.Parallel()

		promptStarted := make(chan struct{}, 1)
		promptRelease := make(chan struct{})

		h := newManagerHarness(t)
		h.sessions = newManagerSessionStub(sessionAttemptPlan{
			sessionID:     "sess-trigger-cancel",
			promptStarted: promptStarted,
			promptRelease: promptRelease,
		})

		manager := h.newManager(t, integrationAutomationConfig())
		if err := manager.Start(h.ctx); err != nil {
			t.Fatalf("manager.Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("manager.Shutdown() error = %v", err)
			}
		})

		job, err := manager.CreateJob(h.ctx, Job{
			Scope:       AutomationScopeWorkspace,
			Name:        "manual-trigger-cancel",
			AgentName:   "researcher",
			WorkspaceID: h.workspace.ID,
			Prompt:      "Run the accepted automation.",
			Schedule: &ScheduleSpec{
				Mode:     ScheduleModeEvery,
				Interval: "1h",
			},
			Enabled:   true,
			Retry:     DefaultRetryConfig(),
			FireLimit: DefaultFireLimitConfig(),
		})
		if err != nil {
			t.Fatalf("manager.CreateJob() error = %v", err)
		}

		triggerCtx, cancel := context.WithCancel(h.ctx)
		runCh := make(chan Run, 1)
		errCh := make(chan error, 1)
		go func() {
			run, triggerErr := manager.TriggerJob(triggerCtx, job.ID)
			runCh <- run
			errCh <- triggerErr
		}()

		select {
		case <-promptStarted:
		case <-time.After(2 * time.Second):
			t.Fatal("automation session did not reach Prompt() in time")
		}

		runs, err := h.db.ListRuns(h.ctx, RunQuery{JobID: job.ID})
		if err != nil {
			t.Fatalf("ListRuns() error = %v", err)
		}
		if got, want := len(runs), 1; got != want {
			t.Fatalf("len(runs) = %d, want %d", got, want)
		}
		if got, want := runs[0].Status, RunRunning; got != want {
			t.Fatalf("runs[0].Status = %q, want %q", got, want)
		}

		cancel()
		close(promptRelease)

		var completedRun Run
		select {
		case completedRun = <-runCh:
		case <-time.After(2 * time.Second):
			t.Fatal("manager.TriggerJob() did not return after prompt release")
		}
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("manager.TriggerJob() error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("manager.TriggerJob() error channel did not return")
		}
		if got, want := completedRun.Status, RunCompleted; got != want {
			t.Fatalf("completedRun.Status = %q, want %q", got, want)
		}
	})
}

func TestManagerIntegrationManualTriggerHonorsCallerDeadline(t *testing.T) {
	t.Parallel()

	t.Run("Should stop manual trigger when caller deadline expires", func(t *testing.T) {
		t.Parallel()

		promptStarted := make(chan struct{}, 1)
		promptRelease := make(chan struct{})

		h := newManagerHarness(t)
		h.sessions = newManagerSessionStub(sessionAttemptPlan{
			sessionID:     "sess-trigger-deadline",
			promptStarted: promptStarted,
			promptRelease: promptRelease,
		})

		manager := h.newManager(t, integrationAutomationConfig())
		if err := manager.Start(h.ctx); err != nil {
			t.Fatalf("manager.Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(testutil.Context(t)); err != nil {
				t.Fatalf("manager.Shutdown() error = %v", err)
			}
		})

		job, err := manager.CreateJob(h.ctx, Job{
			Scope:       AutomationScopeWorkspace,
			Name:        "manual-trigger-deadline",
			AgentName:   "researcher",
			WorkspaceID: h.workspace.ID,
			Prompt:      "Run until the deadline expires.",
			Schedule: &ScheduleSpec{
				Mode:     ScheduleModeEvery,
				Interval: "1h",
			},
			Enabled:   true,
			Retry:     DefaultRetryConfig(),
			FireLimit: DefaultFireLimitConfig(),
		})
		if err != nil {
			t.Fatalf("manager.CreateJob() error = %v", err)
		}

		triggerCtx, cancel := context.WithTimeout(h.ctx, 100*time.Millisecond)
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			_, triggerErr := manager.TriggerJob(triggerCtx, job.ID)
			errCh <- triggerErr
		}()

		select {
		case <-promptStarted:
		case <-time.After(2 * time.Second):
			t.Fatal("automation session did not reach Prompt() in time")
		}

		select {
		case err := <-errCh:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("manager.TriggerJob() error = %v, want context deadline exceeded", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("manager.TriggerJob() did not return after caller deadline")
		}

		close(promptRelease)
	})
}

func integrationAutomationConfig() aghconfig.AutomationConfig {
	return aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
		Suggestions: aghconfig.AutomationSuggestionsConfig{
			PendingCap: DefaultSuggestionPendingCap,
		},
	}
}
