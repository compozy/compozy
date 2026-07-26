package automation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
)

func TestDispatchWorkspaceAutomationUsesWorkspaceCreateOpts(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)

	job := testJob(AutomationScopeWorkspace, "job-workspace", "ws_alpha")
	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindSchedule,
		Job:  &job,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if got, want := len(creator.createCalls()), 1; got != want {
		t.Fatalf("len(Create calls) = %d, want %d", got, want)
	}
	createCall := creator.createCalls()[0]
	if got, want := createCall.AgentName, job.AgentName; got != want {
		t.Fatalf("Create().AgentName = %q, want %q", got, want)
	}
	if got, want := createCall.Name, job.Name; got != want {
		t.Fatalf("Create().Name = %q, want %q", got, want)
	}
	if got, want := createCall.Workspace, job.WorkspaceID; got != want {
		t.Fatalf("Create().Workspace = %q, want %q", got, want)
	}
	if got := createCall.Provider; got != "" {
		t.Fatalf("Create().Provider = %q, want explicit empty provider", got)
	}
	if got := createCall.WorkspacePath; got != "" {
		t.Fatalf("Create().WorkspacePath = %q, want empty", got)
	}
	if got, want := createCall.Type, session.SessionTypeSystem; got != want {
		t.Fatalf("Create().Type = %q, want %q", got, want)
	}

	if got, want := len(creator.promptCalls()), 1; got != want {
		t.Fatalf("len(Prompt calls) = %d, want %d", got, want)
	}
	if got, want := creator.promptCalls()[0].message, job.Prompt; got != want {
		t.Fatalf("Prompt().message = %q, want %q", got, want)
	}

	if got, want := run.Status, RunCompleted; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
	if got, want := run.JobID, job.ID; got != want {
		t.Fatalf("run.JobID = %q, want %q", got, want)
	}
}

func TestDispatchGlobalAutomationUsesGlobalWorkspacePath(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	globalPath := filepath.Join(t.TempDir(), "agh-home")
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherGlobalWorkspacePath(globalPath))

	job := testJob(AutomationScopeGlobal, "job-global", "")
	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindManual,
		Job:  &job,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	createCall := creator.createCalls()[0]
	if got := createCall.Workspace; got != "" {
		t.Fatalf("Create().Workspace = %q, want empty", got)
	}
	if got := createCall.Provider; got != "" {
		t.Fatalf("Create().Provider = %q, want explicit empty provider", got)
	}
	if got, want := createCall.WorkspacePath, globalPath; got != want {
		t.Fatalf("Create().WorkspacePath = %q, want %q", got, want)
	}
	if got, want := run.Status, RunCompleted; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
	if got, want := run.JobID, job.ID; got != want {
		t.Fatalf("run.JobID = %q, want %q", got, want)
	}
}

func TestDispatchTaskBackedJobDelegatesToTaskServiceWithoutSessionRuntime(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	tasks := newRecordingTaskService()
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherTasks(tasks))

	job := testJob(AutomationScopeWorkspace, "job-task-backed", "ws_alpha")
	job.Task = &JobTaskConfig{
		Title:                "Review automation findings",
		Description:          "Create a durable review task.",
		NetworkParticipation: testNamedParticipation("ops-automation"),
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindAutomation,
			Ref:  "job-task-backed",
		},
	}

	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindSchedule,
		Job:  &job,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if got := len(creator.createCalls()); got != 0 {
		t.Fatalf("len(Create calls) = %d, want 0", got)
	}
	if got := len(creator.promptCalls()); got != 0 {
		t.Fatalf("len(Prompt calls) = %d, want 0", got)
	}
	if got := len(creator.stopCalls()); got != 0 {
		t.Fatalf("len(StopWithCause calls) = %d, want 0", got)
	}
	if got, want := run.Status, RunDelegated; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
	if got := run.SessionID; got != "" {
		t.Fatalf("run.SessionID = %q, want empty", got)
	}
	if got, want := run.TaskID, "task-1"; got != want {
		t.Fatalf("run.TaskID = %q, want %q", got, want)
	}
	if got, want := run.TaskRunID, "task-run-1"; got != want {
		t.Fatalf("run.TaskRunID = %q, want %q", got, want)
	}

	if got, want := len(tasks.createCalls), 1; got != want {
		t.Fatalf("len(CreateTask calls) = %d, want %d", got, want)
	}
	createCall := tasks.createCalls[0]
	if got, want := createCall.actor.Actor.Kind, taskpkg.ActorKindAutomation; got != want {
		t.Fatalf("CreateTask().actor.kind = %q, want %q", got, want)
	}
	if got, want := createCall.actor.Actor.Ref, job.ID; got != want {
		t.Fatalf("CreateTask().actor.ref = %q, want %q", got, want)
	}
	if got, want := createCall.actor.Origin.Kind, taskpkg.OriginKindAutomation; got != want {
		t.Fatalf("CreateTask().origin.kind = %q, want %q", got, want)
	}
	if got, want := createCall.actor.Origin.Ref, "run:"+run.ID; got != want {
		t.Fatalf("CreateTask().origin.ref = %q, want %q", got, want)
	}
	if got, want := createCall.spec.Scope, taskpkg.ScopeWorkspace; got != want {
		t.Fatalf("CreateTask().scope = %q, want %q", got, want)
	}
	if got, want := createCall.spec.WorkspaceID, "ws_alpha"; got != want {
		t.Fatalf("CreateTask().workspace_id = %q, want %q", got, want)
	}
	assertNamedParticipation(t, createCall.spec.NetworkParticipation, "ops-automation")
	if got, want := len(tasks.enqueueCalls), 1; got != want {
		t.Fatalf("len(EnqueueRun calls) = %d, want %d", got, want)
	}
	enqueueCall := tasks.enqueueCalls[0]
	if got, want := enqueueCall.spec.TaskID, "task-1"; got != want {
		t.Fatalf("EnqueueRun().task_id = %q, want %q", got, want)
	}
	if got, want := enqueueCall.spec.IdempotencyKey, "automation-run:"+run.ID; got != want {
		t.Fatalf("EnqueueRun().idempotency_key = %q, want %q", got, want)
	}
	assertNamedParticipation(t, enqueueCall.spec.NetworkParticipation, "ops-automation")
	if got, want := enqueueCall.spec.NetworkParticipationSource, participation.SourceAutomationJob; got != want {
		t.Fatalf("EnqueueRun().network_participation_source = %q, want %q", got, want)
	}
	assertNamedParticipation(t, run.NetworkParticipation, "ops-automation")
}

func TestDispatchLoopTargetJobDelegatesToLoopStarterWithoutSessionRuntime(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	starter := &recordingLoopStarter{runID: "looprun-schedule"}
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherLoopStarter(starter))

	job := testJob(AutomationScopeWorkspace, "job-loop-backed", "ws_alpha")
	job.AgentName = ""
	job.Prompt = ""
	job.TargetKind = TargetKindLoop
	job.LoopTarget = &LoopTarget{
		WorkspaceID: "ws_alpha",
		LoopName:    "triage",
		Inputs: map[string]any{
			"tasks": "task-ref",
		},
	}

	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindSchedule,
		Job:  &job,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if got := len(creator.createCalls()); got != 0 {
		t.Fatalf("len(Create calls) = %d, want 0", got)
	}
	if got := len(creator.promptCalls()); got != 0 {
		t.Fatalf("len(Prompt calls) = %d, want 0", got)
	}
	if got, want := run.Status, RunDelegated; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
	if got, want := run.LoopRunID, "looprun-schedule"; got != want {
		t.Fatalf("run.LoopRunID = %q, want %q", got, want)
	}
	if got := run.TaskID; got != "" {
		t.Fatalf("run.TaskID = %q, want empty", got)
	}
	if got := run.TaskRunID; got != "" {
		t.Fatalf("run.TaskRunID = %q, want empty", got)
	}

	startCalls := starter.startCallSnapshot()
	if got, want := len(startCalls), 1; got != want {
		t.Fatalf("len(StartLoop calls) = %d, want %d", got, want)
	}
	call := startCalls[0]
	if got, want := call.Kind, LoopStartKindSchedule; got != want {
		t.Fatalf("StartLoop().Kind = %q, want %q", got, want)
	}
	if got, want := call.WorkspaceID, "ws_alpha"; got != want {
		t.Fatalf("StartLoop().WorkspaceID = %q, want %q", got, want)
	}
	if got, want := call.LoopName, "triage"; got != want {
		t.Fatalf("StartLoop().LoopName = %q, want %q", got, want)
	}
	if got, want := call.AutomationRunID, run.ID; got != want {
		t.Fatalf("StartLoop().AutomationRunID = %q, want %q", got, want)
	}
	if got, want := call.Actor.Actor.Kind, taskpkg.ActorKindAutomation; got != want {
		t.Fatalf("StartLoop().Actor.Actor.Kind = %q, want %q", got, want)
	}
	if got, want := call.Actor.Actor.Ref, job.ID; got != want {
		t.Fatalf("StartLoop().Actor.Actor.Ref = %q, want %q", got, want)
	}
	if got, want := call.Actor.Origin.Ref, "run:"+run.ID; got != want {
		t.Fatalf("StartLoop().Actor.Origin.Ref = %q, want %q", got, want)
	}
	if got, want := call.Inputs["tasks"], "task-ref"; got != want {
		t.Fatalf("StartLoop().Inputs[tasks] = %v, want %v", got, want)
	}
	if call.NetworkParticipation != nil || run.NetworkParticipation != nil {
		t.Fatalf(
			"participation = call:%#v run:%#v, want no declaration and no implicit enrollment",
			call.NetworkParticipation,
			run.NetworkParticipation,
		)
	}
}

func TestDispatchLoopTargetWebhookTriggerPassesPayloadToLoopStarter(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	starter := &recordingLoopStarter{runID: "looprun-webhook"}
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherLoopStarter(starter))

	trigger := testTrigger(AutomationScopeWorkspace, "trigger-loop-backed", "ws_alpha")
	trigger.AgentName = ""
	trigger.Prompt = ""
	trigger.TargetKind = TargetKindLoop
	trigger.LoopTarget = &LoopTarget{
		WorkspaceID:          "ws_alpha",
		LoopName:             "deploy",
		NetworkParticipation: testNamedParticipation("definition-channel"),
		InputMapping: map[string]string{
			"title": "{{ .trigger.payload.title }}",
		},
	}
	envelope := testEnvelope(AutomationScopeWorkspace, "ws_alpha")
	envelope.Data = map[string]any{
		"title":                 "Deploy release",
		"network_participation": testNamedParticipation("payload-channel"),
	}

	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind:     DispatchKindTrigger,
		Trigger:  &trigger,
		Envelope: &envelope,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if got := len(creator.createCalls()); got != 0 {
		t.Fatalf("len(Create calls) = %d, want 0", got)
	}
	if got, want := run.Status, RunDelegated; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
	if got, want := run.LoopRunID, "looprun-webhook"; got != want {
		t.Fatalf("run.LoopRunID = %q, want %q", got, want)
	}

	startCalls := starter.startCallSnapshot()
	if got, want := len(startCalls), 1; got != want {
		t.Fatalf("len(StartLoop calls) = %d, want %d", got, want)
	}
	call := startCalls[0]
	if got, want := call.Kind, LoopStartKindWebhook; got != want {
		t.Fatalf("StartLoop().Kind = %q, want %q", got, want)
	}
	if got, want := call.TriggerPayload["title"], "Deploy release"; got != want {
		t.Fatalf("StartLoop().TriggerPayload[title] = %v, want %v", got, want)
	}
	if got, want := call.InputMapping["title"], "{{ .trigger.payload.title }}"; got != want {
		t.Fatalf("StartLoop().InputMapping[title] = %q, want %q", got, want)
	}
	if got, want := call.Actor.Actor.Ref, trigger.ID; got != want {
		t.Fatalf("StartLoop().Actor.Actor.Ref = %q, want %q", got, want)
	}
	if got, want := call.Actor.Origin.Ref, "run:"+run.ID; got != want {
		t.Fatalf("StartLoop().Actor.Origin.Ref = %q, want %q", got, want)
	}
	assertNamedParticipation(t, call.NetworkParticipation, "definition-channel")
	assertNamedParticipation(t, run.NetworkParticipation, "definition-channel")
}

func TestDispatchLoopTargetFailsRunWhenLoopStarterRejects(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	starter := &recordingLoopStarter{startErr: errors.New("start_kind_not_allowed: schedule not declared")}
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherLoopStarter(starter))

	job := testJob(AutomationScopeWorkspace, "job-loop-rejected", "ws_alpha")
	job.AgentName = ""
	job.Prompt = ""
	job.TargetKind = TargetKindLoop
	job.LoopTarget = &LoopTarget{
		WorkspaceID: "ws_alpha",
		LoopName:    "triage",
	}

	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindSchedule,
		Job:  &job,
	})
	if err == nil {
		t.Fatal("Dispatch() error = nil, want loop starter rejection")
	}
	if run == nil {
		t.Fatal("Dispatch() run = nil, want failed automation run")
	}
	if got, want := run.Status, RunFailed; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
	if got := run.LoopRunID; got != "" {
		t.Fatalf("run.LoopRunID = %q, want empty", got)
	}
	if !strings.Contains(run.Error, "start_kind_not_allowed") {
		t.Fatalf("run.Error = %q, want start_kind_not_allowed reason", run.Error)
	}
	if got := len(creator.createCalls()); got != 0 {
		t.Fatalf("len(Create calls) = %d, want 0", got)
	}
}

func TestDispatchLoopTargetCatchUpConflictRecordsMetadata(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	starter := &recordingLoopStarter{
		startErr: fmt.Errorf("%w: active_loop_run_exists", ErrLoopConcurrencyConflict),
	}
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherLoopStarter(starter))
	scheduledAt := time.Date(2026, 7, 7, 9, 30, 0, 0, time.UTC)
	job := testJob(AutomationScopeWorkspace, "job-loop-conflict", "ws_alpha")
	job.AgentName = ""
	job.Prompt = ""
	job.TargetKind = TargetKindLoop
	job.LoopTarget = &LoopTarget{
		WorkspaceID: "ws_alpha",
		LoopName:    "triage",
	}

	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind:          DispatchKindSchedule,
		Job:           &job,
		ScheduledAt:   &scheduledAt,
		CatchUp:       true,
		CatchUpPolicy: SchedulerCatchUpPolicyCoalesce,
	})
	if err == nil {
		t.Fatal("Dispatch() error = nil, want loop concurrency conflict")
	}
	if run == nil {
		t.Fatal("Dispatch() run = nil, want failed automation run")
	}
	if got, want := run.Status, RunFailed; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
	if got, want := run.Metadata["reason"], "loop_concurrency_conflict"; got != want {
		t.Fatalf("run.Metadata[reason] = %#v, want %q", got, want)
	}
	if got, want := run.Metadata["catch_up_policy"], "coalesce"; got != want {
		t.Fatalf("run.Metadata[catch_up_policy] = %#v, want %q", got, want)
	}
	if got, want := run.Metadata["scheduled_at"], scheduledAt.Format(time.RFC3339Nano); got != want {
		t.Fatalf("run.Metadata[scheduled_at] = %#v, want %q", got, want)
	}
	startCalls := starter.startCallSnapshot()
	if got, want := len(startCalls), 1; got != want {
		t.Fatalf("len(StartLoop calls) = %d, want %d", got, want)
	}
	if startCalls[0].ScheduledAt == nil || !startCalls[0].ScheduledAt.Equal(scheduledAt) {
		t.Fatalf("StartLoop().ScheduledAt = %v, want %s", startCalls[0].ScheduledAt, scheduledAt.Format(time.RFC3339))
	}
	if !startCalls[0].CatchUp || startCalls[0].CatchUpPolicy != SchedulerCatchUpPolicyCoalesce {
		t.Fatalf(
			"StartLoop() catch-up = %v/%q, want true/coalesce",
			startCalls[0].CatchUp,
			startCalls[0].CatchUpPolicy,
		)
	}
}

func TestDispatchLoopTargetRetryShouldReuseDefinitionParticipation(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	starter := &recordingLoopStarter{
		runID:       "looprun-retried",
		startErrors: []error{errors.New("transient loop start failure"), nil},
	}
	dispatcher := newTestDispatcher(
		t,
		newRecordingSessionCreator(),
		store,
		WithDispatcherLoopStarter(starter),
		WithDispatcherSleep(func(context.Context, time.Duration) error { return nil }),
	)
	job := testJob(AutomationScopeWorkspace, "job-loop-retry", "ws_alpha")
	job.AgentName = ""
	job.Prompt = ""
	job.TargetKind = TargetKindLoop
	job.LoopTarget = &LoopTarget{
		WorkspaceID:          "ws_alpha",
		LoopName:             "triage",
		NetworkParticipation: testNamedParticipation("retry-channel"),
	}
	job.Retry = RetryConfig{Strategy: RetryStrategyBackoff, MaxRetries: 1, BaseDelay: "1ms"}

	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindSchedule,
		Job:  &job,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if run == nil || run.Status != RunDelegated {
		t.Fatalf("Dispatch() run = %#v, want delegated retry", run)
	}
	calls := starter.startCallSnapshot()
	if got, want := len(calls), 2; got != want {
		t.Fatalf("len(StartLoop calls) = %d, want %d", got, want)
	}
	for index, call := range calls {
		assertNamedParticipation(t, call.NetworkParticipation, "retry-channel")
		if strings.TrimSpace(call.AutomationRunID) == "" {
			t.Fatalf("StartLoop call %d AutomationRunID = empty", index)
		}
	}
	for _, stored := range store.listRuns() {
		assertNamedParticipation(t, stored.NetworkParticipation, "retry-channel")
	}
}

func TestDispatchNonTaskJobStillUsesSessionRuntimeAndRecordsAutomationSessionActor(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator(sessionAttemptPlan{
		sessionID:   "sess-automation-1",
		workspaceID: "ws-automation-global",
	})
	recorder := &recordingTaskActorRecorder{}
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherTaskActorRecorder(recorder))

	job := testJob(AutomationScopeGlobal, "job-runtime", "")
	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindManual,
		Job:  &job,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	if got, want := len(creator.createCalls()), 1; got != want {
		t.Fatalf("len(Create calls) = %d, want %d", got, want)
	}
	if got, want := len(creator.promptCalls()), 1; got != want {
		t.Fatalf("len(Prompt calls) = %d, want %d", got, want)
	}
	if got, want := len(recorder.recorded), 1; got != want {
		t.Fatalf("len(recorded actors) = %d, want %d", got, want)
	}
	recordedActor, ok := recorder.recorded["sess-automation-1"]
	if !ok {
		t.Fatalf("recorded actors = %#v, want session key", recorder.recorded)
	}
	if got, want := recordedActor.Actor.Kind, taskpkg.ActorKindAgentSession; got != want {
		t.Fatalf("recorded actor.kind = %q, want %q", got, want)
	}
	if got, want := recordedActor.Actor.Ref, "sess-automation-1"; got != want {
		t.Fatalf("recorded actor.ref = %q, want %q", got, want)
	}
	if got, want := recordedActor.Origin.Kind, taskpkg.OriginKindAutomation; got != want {
		t.Fatalf("recorded origin.kind = %q, want %q", got, want)
	}
	if got, want := recordedActor.Origin.Ref, "run:"+run.ID; got != want {
		t.Fatalf("recorded origin.ref = %q, want %q", got, want)
	}
	if got, want := recordedActor.Scope, (taskpkg.CallerScope{
		SessionID:   "sess-automation-1",
		WorkspaceID: "ws-automation-global",
	}); got != want {
		t.Fatalf("recorded scope = %#v, want %#v", got, want)
	}
	if got, want := len(recorder.deleted), 1; got != want {
		t.Fatalf("len(deleted actors) = %d, want %d", got, want)
	}
	if got, want := recorder.deleted[0], "sess-automation-1"; got != want {
		t.Fatalf("deleted session id = %q, want %q", got, want)
	}
	if got, want := run.Status, RunCompleted; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
}

func TestDispatchTaskBackedJobFailsWhenTaskServiceIsUnavailable(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)

	job := testJob(AutomationScopeGlobal, "job-task-missing-service", "")
	job.Task = &JobTaskConfig{Title: "Create durable task"}

	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindManual,
		Job:  &job,
	})
	if err == nil || !strings.Contains(err.Error(), "task-backed job requires task service") {
		t.Fatalf("Dispatch() error = %v, want missing task service failure", err)
	}
	if got, want := run.Status, RunFailed; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
	if got := len(creator.createCalls()); got != 0 {
		t.Fatalf("len(Create calls) = %d, want 0", got)
	}
}

func TestDispatchTaskBackedJobMarksRunCancelledForTaskServiceCancellation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		createErr  error
		enqueueErr error
	}{
		{name: "ShouldClassifyCreateTaskCancellation", createErr: context.Canceled},
		{name: "ShouldClassifyEnqueueRunDeadlineExceeded", enqueueErr: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := newMemoryRunStore()
			creator := newRecordingSessionCreator()
			tasks := &recordingTaskService{
				createErr:  tt.createErr,
				enqueueErr: tt.enqueueErr,
			}
			dispatcher := newTestDispatcher(t, creator, store, WithDispatcherTasks(tasks))

			job := testJob(AutomationScopeGlobal, "job-task-canceled", "")
			job.Task = &JobTaskConfig{Title: "Create durable task"}

			run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
				Kind: DispatchKindManual,
				Job:  &job,
			})
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Dispatch() error = %v, want cancellation-classified error", err)
			}
			if got, want := run.Status, RunCancelled; got != want {
				t.Fatalf("run.Status = %q, want %q", got, want)
			}
			if got := len(creator.createCalls()); got != 0 {
				t.Fatalf("len(Create calls) = %d, want 0", got)
			}
		})
	}
}

func TestDirectTaskSpecFallsBackToJobNameAndPrompt(t *testing.T) {
	t.Parallel()

	job := testJob(AutomationScopeGlobal, "job-fallbacks", "")
	job.Prompt = "  Review the latest automation output.  "
	job.Task = &JobTaskConfig{}

	spec := directTaskSpec(&job, job.Prompt)
	if got, want := spec.Scope, taskpkg.ScopeGlobal; got != want {
		t.Fatalf("spec.Scope = %q, want %q", got, want)
	}
	if got, want := spec.Title, "job-fallbacks"; got != want {
		t.Fatalf("spec.Title = %q, want %q", got, want)
	}
	if got, want := spec.Description, "Review the latest automation output."; got != want {
		t.Fatalf("spec.Description = %q, want %q", got, want)
	}
}

func TestDispatchTaskBackedJobUsesRewrittenPreFirePromptForTaskDescription(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	tasks := newRecordingTaskService()
	hooks := &recordingAutomationHooks{
		onJobPreFire: func(_ context.Context, payload hookspkg.AutomationJobPreFirePayload) (hookspkg.AutomationJobPreFirePayload, error) {
			payload.Prompt = "Hook-rewritten durable task prompt."
			return payload, nil
		},
	}
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherTasks(tasks), WithDispatcherHooks(hooks))

	job := testJob(AutomationScopeGlobal, "job-hook-task", "")
	job.Prompt = "Original automation prompt."
	job.Task = &JobTaskConfig{Title: "Create durable task"}

	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindManual,
		Job:  &job,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if got, want := run.Status, RunDelegated; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
	if got, want := len(tasks.createCalls), 1; got != want {
		t.Fatalf("len(CreateTask calls) = %d, want %d", got, want)
	}
	if got, want := tasks.createCalls[0].spec.Description, "Hook-rewritten durable task prompt."; got != want {
		t.Fatalf("CreateTask().description = %q, want %q", got, want)
	}
}

func TestDispatchStopsCreatedSessionWhenRunCompletes(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)

	job := testJob(AutomationScopeGlobal, "job-stop-complete", "")
	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindManual,
		Job:  &job,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	stopCalls := creator.stopCalls()
	if got, want := len(stopCalls), 1; got != want {
		t.Fatalf("len(StopWithCause calls) = %d, want %d", got, want)
	}
	if got, want := stopCalls[0].sessionID, run.SessionID; got != want {
		t.Fatalf("StopWithCause().sessionID = %q, want %q", got, want)
	}
	if got, want := stopCalls[0].cause, session.CauseCompleted; got != want {
		t.Fatalf("StopWithCause().cause = %v, want %v", got, want)
	}
	if got := stopCalls[0].detail; got != "" {
		t.Fatalf("StopWithCause().detail = %q, want empty", got)
	}
}

func TestDispatchStopsCreatedSessionWhenRunFails(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator(sessionAttemptPlan{
		events: []acp.AgentEvent{{Error: "prompt failed"}},
	})
	dispatcher := newTestDispatcher(t, creator, store)

	job := testJob(AutomationScopeGlobal, "job-stop-failed", "")
	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindManual,
		Job:  &job,
	})
	if err == nil {
		t.Fatal("Dispatch() error = nil, want non-nil")
	}
	if got, want := run.Status, RunFailed; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}

	stopCalls := creator.stopCalls()
	if got, want := len(stopCalls), 1; got != want {
		t.Fatalf("len(StopWithCause calls) = %d, want %d", got, want)
	}
	if got, want := stopCalls[0].sessionID, run.SessionID; got != want {
		t.Fatalf("StopWithCause().sessionID = %q, want %q", got, want)
	}
	if got, want := stopCalls[0].cause, session.CauseFailed; got != want {
		t.Fatalf("StopWithCause().cause = %v, want %v", got, want)
	}
	if got, want := stopCalls[0].detail, "prompt failed"; got != want {
		t.Fatalf("StopWithCause().detail = %q, want %q", got, want)
	}
}

func TestDispatchRejectsWhenGlobalConcurrencyLimitIsReached(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	blockPrompt := make(chan struct{})
	promptStarted := make(chan struct{}, 1)
	creator := newRecordingSessionCreator(
		sessionAttemptPlan{
			promptRelease: blockPrompt,
			promptStarted: promptStarted,
		},
	)
	dispatcher := newTestDispatcher(t, creator, store, WithDispatcherMaxConcurrent(1))

	job := testJob(AutomationScopeGlobal, "job-concurrency", "")
	firstErrCh := make(chan error, 1)
	go func() {
		_, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
			Kind: DispatchKindSchedule,
			Job:  &job,
		})
		firstErrCh <- err
	}()

	select {
	case <-promptStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("first dispatch did not reach Prompt() in time")
	}

	_, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindManual,
		Job:  &job,
	})
	if !errors.Is(err, ErrConcurrencyLimitReached) {
		t.Fatalf("Dispatch(second) error = %v, want ErrConcurrencyLimitReached", err)
	}
	if got, want := len(creator.createCalls()), 1; got != want {
		t.Fatalf("len(Create calls) = %d, want %d", got, want)
	}

	close(blockPrompt)
	select {
	case err := <-firstErrCh:
		if err != nil {
			t.Fatalf("Dispatch(first) error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("first dispatch did not finish after prompt release")
	}
}

func TestDispatchFireLimitPersistsAcrossDispatcherRecreation(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	dbPath := filepath.Join(t.TempDir(), store.GlobalDatabaseName)
	db, err := globaldb.OpenGlobalDB(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	job, err := db.CreateJob(ctx, testJob(AutomationScopeGlobal, "job-fire-limit", ""))
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}
	job.FireLimit = FireLimitConfig{Max: 1, Window: "1h"}

	now := time.Date(2026, 4, 10, 23, 0, 0, 0, time.UTC)
	firstCreator := newRecordingSessionCreator()
	firstDispatcher := newTestDispatcher(
		t,
		firstCreator,
		db,
		WithDispatcherNow(func() time.Time { return now }),
	)
	if _, err := firstDispatcher.Dispatch(ctx, DispatchRequest{
		Kind: DispatchKindSchedule,
		Job:  &job,
	}); err != nil {
		t.Fatalf("Dispatch(first) error = %v", err)
	}

	secondCreator := newRecordingSessionCreator()
	secondDispatcher := newTestDispatcher(
		t,
		secondCreator,
		db,
		WithDispatcherNow(func() time.Time { return now }),
	)
	_, err = secondDispatcher.Dispatch(ctx, DispatchRequest{
		Kind: DispatchKindManual,
		Job:  &job,
	})
	if !errors.Is(err, ErrFireLimitReached) {
		t.Fatalf("Dispatch(second) error = %v, want ErrFireLimitReached", err)
	}
	var fireLimitErr *FireLimitError
	if !errors.As(err, &fireLimitErr) {
		t.Fatalf("Dispatch(second) error = %v, want FireLimitError details", err)
	}
	wantRetryAt := now.Add(time.Hour)
	if got := fireLimitErr.RetryAt; !got.Equal(wantRetryAt) {
		t.Fatalf("fireLimitErr.RetryAt = %s, want %s", got.Format(time.RFC3339), wantRetryAt.Format(time.RFC3339))
	}
	if got := len(secondCreator.createCalls()); got != 0 {
		t.Fatalf("len(Create calls after fire-limit rejection) = %d, want 0", got)
	}
}

func TestDispatchScheduledReservedRunCancelsOnFireLimit(t *testing.T) {
	t.Run(
		"Should cancel the reserved run and preserve retry timing when the fire limit is reached",
		func(t *testing.T) {
			t.Parallel()

			now := time.Date(2026, 4, 10, 23, 0, 0, 0, time.UTC)
			store := newMemoryRunStore()
			dispatcher := newTestDispatcher(
				t,
				newRecordingSessionCreator(),
				store,
				WithDispatcherNow(func() time.Time { return now }),
			)

			job := testJob(AutomationScopeGlobal, "job-scheduled-fire-limit", "")
			job.FireLimit = FireLimitConfig{Max: 1, Window: "1h"}

			earlierStartedAt := now.Add(-15 * time.Minute)
			if _, err := store.CreateRun(testutil.Context(t), Run{
				ID:        "run-existing",
				JobID:     job.ID,
				Status:    RunCompleted,
				Attempt:   1,
				StartedAt: timePointer(earlierStartedAt),
				EndedAt:   timePointer(earlierStartedAt.Add(time.Minute)),
			}); err != nil {
				t.Fatalf("CreateRun(existing) error = %v", err)
			}

			reservedRun, err := store.CreateRun(testutil.Context(t), Run{
				ID:          "run-reserved",
				JobID:       job.ID,
				Status:      RunScheduled,
				Attempt:     1,
				ScheduledAt: timePointer(now),
				StartedAt:   timePointer(now),
			})
			if err != nil {
				t.Fatalf("CreateRun(reserved) error = %v", err)
			}

			run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
				Kind:        DispatchKindSchedule,
				Job:         &job,
				ReservedRun: &reservedRun,
			})
			if !errors.Is(err, ErrFireLimitReached) {
				t.Fatalf("Dispatch() error = %v, want ErrFireLimitReached", err)
			}
			var fireLimitErr *FireLimitError
			if !errors.As(err, &fireLimitErr) {
				t.Fatalf("Dispatch() error = %v, want FireLimitError details", err)
			}
			wantRetryAt := earlierStartedAt.Add(time.Hour)
			if got := fireLimitErr.RetryAt; !got.Equal(wantRetryAt) {
				t.Fatalf(
					"fireLimitErr.RetryAt = %s, want %s",
					got.Format(time.RFC3339),
					wantRetryAt.Format(time.RFC3339),
				)
			}
			if run == nil {
				t.Fatal("Dispatch() run = nil, want canceled reserved run")
			}
			if got, want := run.Status, RunCancelled; got != want {
				t.Fatalf("run.Status = %q, want %q", got, want)
			}
			if got := run.Error; got == "" || !strings.Contains(got, ErrFireLimitReached.Error()) {
				t.Fatalf("run.Error = %q, want fire-limit message", got)
			}
		},
	)
}

func TestDispatchReservedRunAdvancesAttemptAcrossRetry(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	store := newMemoryRunStore()
	starter := &recordingLoopStarter{
		runID:       "looprun-reserved-retry",
		startErrors: []error{errors.New("first reserved failure"), nil},
	}
	dispatcher := newTestDispatcher(
		t,
		newRecordingSessionCreator(),
		store,
		WithDispatcherLoopStarter(starter),
		WithDispatcherSleep(func(context.Context, time.Duration) error { return nil }),
	)

	job := testJob(AutomationScopeWorkspace, "job-reserved-retry", "ws_alpha")
	job.AgentName = ""
	job.Prompt = ""
	job.TargetKind = TargetKindLoop
	job.LoopTarget = &LoopTarget{
		WorkspaceID:          "ws_alpha",
		LoopName:             "triage",
		NetworkParticipation: testNamedParticipation("reserved-retry"),
	}
	job.Retry = RetryConfig{
		Strategy:   RetryStrategyBackoff,
		MaxRetries: 1,
		BaseDelay:  "1s",
	}
	reserved, err := store.CreateRun(ctx, Run{
		ID:                   "run-reserved-retry",
		JobID:                job.ID,
		Status:               RunScheduled,
		Attempt:              1,
		StartedAt:            timePointer(time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)),
		NetworkParticipation: cloneParticipationRequest(job.LoopTarget.NetworkParticipation),
	})
	if err != nil {
		t.Fatalf("CreateRun(reserved) error = %v", err)
	}

	run, err := dispatcher.Dispatch(ctx, DispatchRequest{
		Kind:        DispatchKindSchedule,
		Job:         &job,
		ReservedRun: &reserved,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if run == nil {
		t.Fatal("Dispatch() run = nil, want completed reserved run")
	}
	if got, want := run.Attempt, 2; got != want {
		t.Fatalf("run.Attempt = %d, want %d after retry", got, want)
	}
	var stored Run
	for _, candidate := range store.listRuns() {
		if candidate.ID == reserved.ID {
			stored = candidate
			break
		}
	}
	if stored.ID == "" {
		t.Fatalf("stored reserved run %q not found", reserved.ID)
	}
	if got, want := stored.Attempt, 2; got != want {
		t.Fatalf("stored.Attempt = %d, want %d after retry", got, want)
	}
	updates := store.updateSnapshot()
	statuses := make([]RunStatus, 0, len(updates))
	for _, update := range updates {
		statuses = append(statuses, update.Status)
	}
	if got, want := statuses, []RunStatus{RunFailed, RunDelegated}; !slices.Equal(got, want) {
		t.Fatalf("reserved run update statuses = %#v, want %#v without stale rewrite", got, want)
	}
}

func TestDispatchFireLimitIgnoresCancelledRuns(t *testing.T) {
	t.Run("Should ignore canceled runs when counting the fire-limit window", func(t *testing.T) {
		t.Parallel()

		now := time.Date(2026, 4, 10, 23, 0, 0, 0, time.UTC)
		store := newMemoryRunStore()
		creator := newRecordingSessionCreator()
		dispatcher := newTestDispatcher(
			t,
			creator,
			store,
			WithDispatcherNow(func() time.Time { return now }),
		)

		job := testJob(AutomationScopeGlobal, "job-canceled-fire-limit", "")
		job.FireLimit = FireLimitConfig{Max: 1, Window: "1h"}

		cancelledAt := now.Add(-10 * time.Minute)
		if _, err := store.CreateRun(testutil.Context(t), Run{
			ID:        "run-canceled",
			JobID:     job.ID,
			Status:    RunCancelled,
			Attempt:   1,
			StartedAt: timePointer(cancelledAt),
			EndedAt:   timePointer(cancelledAt.Add(time.Minute)),
		}); err != nil {
			t.Fatalf("CreateRun(canceled) error = %v", err)
		}

		run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
			Kind: DispatchKindManual,
			Job:  &job,
		})
		if err != nil {
			t.Fatalf("Dispatch() error = %v", err)
		}
		if run == nil {
			t.Fatal("Dispatch() run = nil, want created run")
		}
		if got, want := run.Status, RunCompleted; got != want {
			t.Fatalf("run.Status = %q, want %q", got, want)
		}
		if got, want := len(creator.createCalls()), 1; got != want {
			t.Fatalf("len(Create calls) = %d, want %d", got, want)
		}
	})
}

func TestDispatchBackoffRetryRecordsAttemptMetadata(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator(
		sessionAttemptPlan{
			events: []acp.AgentEvent{{Error: "first failure"}},
		},
		sessionAttemptPlan{},
	)

	var (
		sleepMu sync.Mutex
		delays  []time.Duration
	)
	dispatcher := newTestDispatcher(
		t,
		creator,
		store,
		WithDispatcherSleep(func(_ context.Context, delay time.Duration) error {
			sleepMu.Lock()
			delays = append(delays, delay)
			sleepMu.Unlock()
			return nil
		}),
	)

	job := testJob(AutomationScopeGlobal, "job-retry", "")
	job.Retry = RetryConfig{
		Strategy:   RetryStrategyBackoff,
		MaxRetries: 1,
		BaseDelay:  "2s",
	}

	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindSchedule,
		Job:  &job,
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if got, want := run.Attempt, 2; got != want {
		t.Fatalf("run.Attempt = %d, want %d", got, want)
	}
	if got, want := run.Status, RunCompleted; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}

	sleepMu.Lock()
	if got, want := len(delays), 1; got != want {
		sleepMu.Unlock()
		t.Fatalf("len(delays) = %d, want %d", got, want)
	}
	if got, want := delays[0], 2*time.Second; got != want {
		sleepMu.Unlock()
		t.Fatalf("delays[0] = %s, want %s", got, want)
	}
	sleepMu.Unlock()

	runs := store.listRuns()
	if got, want := len(runs), 2; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].Attempt < runs[j].Attempt
	})
	if got, want := runs[0].Status, RunFailed; got != want {
		t.Fatalf("runs[0].Status = %q, want %q", got, want)
	}
	if got, want := runs[0].Attempt, 1; got != want {
		t.Fatalf("runs[0].Attempt = %d, want %d", got, want)
	}
	if got := runs[0].Error; got == "" {
		t.Fatal("runs[0].Error = empty, want failure recorded")
	}
	if got, want := runs[1].Status, RunCompleted; got != want {
		t.Fatalf("runs[1].Status = %q, want %q", got, want)
	}
	if got, want := runs[1].Attempt, 2; got != want {
		t.Fatalf("runs[1].Attempt = %d, want %d", got, want)
	}
	if got := runs[1].Error; got != "" {
		t.Fatalf("runs[1].Error = %q, want empty", got)
	}
}

func TestNewDispatcherRejectsMissingDependenciesAndGlobalWorkspacePath(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()

	if _, err := NewDispatcher(nil, store, WithDispatcherGlobalWorkspacePath(t.TempDir())); err == nil {
		t.Fatal("NewDispatcher(nil sessions) error = nil, want non-nil")
	}
	if _, err := NewDispatcher(creator, nil, WithDispatcherGlobalWorkspacePath(t.TempDir())); err == nil {
		t.Fatal("NewDispatcher(nil store) error = nil, want non-nil")
	}
	if _, err := NewDispatcher(creator, store); err == nil {
		t.Fatal("NewDispatcher(missing global workspace path) error = nil, want non-nil")
	}
	if _, err := NewDispatcher(
		creator,
		store,
		WithDispatcherGlobalWorkspacePath(t.TempDir()),
		WithDispatcherLogger(nil),
		WithDispatcherSleep(nil),
		WithDispatcherNow(nil),
	); err != nil {
		t.Fatalf("NewDispatcher(with nil optional dependencies) error = %v", err)
	}
}

func TestNewDispatcherSessionStopTimeoutOption(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		opt  DispatcherOption
		want time.Duration
	}{
		{
			name: "Should use the default session stop timeout",
			want: defaultDispatcherSessionStopTimeout,
		},
		{
			name: "Should use the configured session stop timeout",
			opt:  WithDispatcherSessionStopTimeout(30 * time.Second),
			want: 30 * time.Second,
		},
		{
			name: "Should fall back to default for invalid session stop timeout",
			opt:  WithDispatcherSessionStopTimeout(0),
			want: defaultDispatcherSessionStopTimeout,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			options := []DispatcherOption{WithDispatcherGlobalWorkspacePath(t.TempDir())}
			if tc.opt != nil {
				options = append(options, tc.opt)
			}
			dispatcher, err := NewDispatcher(newRecordingSessionCreator(), newMemoryRunStore(), options...)
			if err != nil {
				t.Fatalf("NewDispatcher() error = %v", err)
			}
			if dispatcher.sessionStopTimeout != tc.want {
				t.Fatalf("sessionStopTimeout = %s, want %s", dispatcher.sessionStopTimeout, tc.want)
			}
		})
	}
}

func TestDispatchRequestValidateRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	job := testJob(AutomationScopeWorkspace, "job-validate", "ws_alpha")
	trigger := testTrigger(AutomationScopeGlobal, "trigger-validate", "")
	envelope := testEnvelope(AutomationScopeGlobal, "")

	tests := []struct {
		name string
		req  DispatchRequest
	}{
		{
			name: "invalid kind",
			req: DispatchRequest{
				Kind: DispatchKind("bad"),
				Job:  &job,
			},
		},
		{
			name: "missing subject",
			req: DispatchRequest{
				Kind: DispatchKindManual,
			},
		},
		{
			name: "both job and trigger",
			req: DispatchRequest{
				Kind:    DispatchKindManual,
				Job:     &job,
				Trigger: &trigger,
			},
		},
		{
			name: "trigger missing envelope",
			req: DispatchRequest{
				Kind:    DispatchKindTrigger,
				Trigger: &trigger,
			},
		},
		{
			name: "trigger event mismatch",
			req: DispatchRequest{
				Kind:    DispatchKindTrigger,
				Trigger: &trigger,
				Envelope: pointerToEnvelope(ActivationEnvelope{
					Kind:   "session.stopped",
					Scope:  AutomationScopeGlobal,
					Source: ActivationSourceWebhook,
				}),
			},
		},
		{
			name: "trigger workspace mismatch",
			req: DispatchRequest{
				Kind:    DispatchKindTrigger,
				Trigger: pointerToTrigger(testTrigger(AutomationScopeWorkspace, "trigger-workspace", "ws_alpha")),
				Envelope: pointerToEnvelope(ActivationEnvelope{
					Kind:        "webhook",
					Scope:       AutomationScopeWorkspace,
					WorkspaceID: "ws_bravo",
					Source:      ActivationSourceWebhook,
				}),
			},
		},
		{
			name: "trigger scope mismatch",
			req: DispatchRequest{
				Kind:    DispatchKindTrigger,
				Trigger: &trigger,
				Envelope: pointerToEnvelope(ActivationEnvelope{
					Kind:   envelope.Kind,
					Scope:  AutomationScopeWorkspace,
					Source: ActivationSourceWebhook,
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := tt.req.Validate("dispatch"); err == nil {
				t.Fatal("Validate() error = nil, want non-nil")
			}
		})
	}
}

func TestDispatchTriggerRendersPromptTemplateAndUsesTriggerMetadata(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator()
	dispatcher := newTestDispatcher(t, creator, store)

	trigger := testTrigger(AutomationScopeGlobal, "trigger-render", "")
	envelope := testEnvelope(AutomationScopeGlobal, "")

	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind:     DispatchKindTrigger,
		Trigger:  &trigger,
		Envelope: pointerToEnvelope(envelope),
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}

	createCall := creator.createCalls()[0]
	if got, want := createCall.AgentName, trigger.AgentName; got != want {
		t.Fatalf("Create().AgentName = %q, want %q", got, want)
	}
	if got, want := createCall.Name, trigger.Name; got != want {
		t.Fatalf("Create().Name = %q, want %q", got, want)
	}
	if got := createCall.Workspace; got != "" {
		t.Fatalf("Create().Workspace = %q, want empty", got)
	}
	if got := createCall.Provider; got != "" {
		t.Fatalf("Create().Provider = %q, want explicit empty provider", got)
	}

	promptCall := creator.promptCalls()[0]
	if got, want := promptCall.message, "Review payload deploy"; got != want {
		t.Fatalf("Prompt().message = %q, want %q", got, want)
	}
	if got, want := run.TriggerID, trigger.ID; got != want {
		t.Fatalf("run.TriggerID = %q, want %q", got, want)
	}
}

func TestDispatchMarksRunCancelledWhenPromptIsCancelled(t *testing.T) {
	t.Parallel()

	store := newMemoryRunStore()
	creator := newRecordingSessionCreator(sessionAttemptPlan{
		promptErr: context.Canceled,
	})
	dispatcher := newTestDispatcher(t, creator, store)

	job := testJob(AutomationScopeGlobal, "job-canceled", "")
	run, err := dispatcher.Dispatch(testutil.Context(t), DispatchRequest{
		Kind: DispatchKindSchedule,
		Job:  &job,
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Dispatch() error = %v, want context.Canceled", err)
	}
	if got, want := run.Status, RunCancelled; got != want {
		t.Fatalf("run.Status = %q, want %q", got, want)
	}
}

func TestRetryDelayHelpersAndContextAwareSleep(t *testing.T) {
	t.Parallel()

	if _, err := retryDelay(RetryConfig{
		Strategy:   RetryStrategyBackoff,
		MaxRetries: 1,
		BaseDelay:  "bad",
	}, 1); err == nil {
		t.Fatal("retryDelay(invalid duration) error = nil, want non-nil")
	}

	if delay, err := retryDelay(RetryConfig{
		Strategy:   RetryStrategyBackoff,
		MaxRetries: 1,
		BaseDelay:  "500ms",
	}, 2); err != nil || delay != time.Second {
		t.Fatalf("retryDelay(valid) = (%s, %v), want (1s, nil)", delay, err)
	}

	if err := sleepWithContext(testutil.Context(t), 0); err != nil {
		t.Fatalf("sleepWithContext(zero) error = %v", err)
	}

	cancelledCtx, cancel := context.WithCancel(testutil.Context(t))
	cancel()
	if err := sleepWithContext(cancelledCtx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("sleepWithContext(canceled) error = %v, want context.Canceled", err)
	}
}

func TestCollectPromptErrorRejectsNilStream(t *testing.T) {
	t.Parallel()

	if err := collectPromptError(testutil.Context(t), nil); err == nil {
		t.Fatal("collectPromptError(nil) error = nil, want non-nil")
	}
}

func TestCollectPromptErrorReturnsContextCancellationWhenStreamDoesNotClose(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(testutil.Context(t))
	events := make(chan acp.AgentEvent)
	cancel()

	if err := collectPromptError(ctx, events); !errors.Is(err, context.Canceled) {
		t.Fatalf("collectPromptError(canceled) error = %v, want context.Canceled", err)
	}
}

func TestDispatchLogsHookDispatchFailures(t *testing.T) {
	t.Parallel()

	var logs bytes.Buffer
	dispatcher := &Dispatcher{
		logger: slog.New(slog.NewTextHandler(&logs, nil)),
		hooks:  failingAutomationHookDispatcher{err: errors.New("hook failed")},
	}

	dispatcher.dispatchPostFireHook(testutil.Context(t), DispatchRequest{
		Job: &Job{
			ID:          "job-1",
			Name:        "daily-review",
			AgentName:   "coder",
			WorkspaceID: "ws-alpha",
		},
	}, Run{ID: "run-1", SessionID: "sess-1"})
	dispatcher.dispatchPostFireHook(testutil.Context(t), DispatchRequest{
		Trigger: &Trigger{
			ID:          "trg-1",
			Name:        "deploy-review",
			Event:       "webhook",
			AgentName:   "coder",
			WorkspaceID: "ws-alpha",
		},
	}, Run{ID: "run-2", SessionID: "sess-2"})
	dispatcher.emitRunLifecycleHooks(testutil.Context(t), DispatchRequest{
		Job: &Job{Name: "daily-review", AgentName: "coder"},
	}, Run{ID: "run-3", Status: RunCompleted}, nil, false)
	dispatcher.emitRunLifecycleHooks(testutil.Context(t), DispatchRequest{
		Trigger: &Trigger{Name: "deploy-review", AgentName: "coder"},
	}, Run{ID: "run-4", Status: RunFailed, Error: "boom"}, errors.New("boom"), false)

	output := logs.String()
	for _, message := range []string{
		"automation.dispatch.job_post_fire_hook_failed",
		"automation.dispatch.trigger_post_fire_hook_failed",
		"automation.dispatch.run_completed_hook_failed",
		"automation.dispatch.run_failed_hook_failed",
	} {
		if !strings.Contains(output, message) {
			t.Fatalf("logged output missing %q: %s", message, output)
		}
	}
}

type failingAutomationHookDispatcher struct {
	err error
}

func (f failingAutomationHookDispatcher) DispatchAutomationJobPreFire(
	context.Context,
	hookspkg.AutomationJobPreFirePayload,
) (hookspkg.AutomationJobPreFirePayload, error) {
	return hookspkg.AutomationJobPreFirePayload{}, f.err
}

func (f failingAutomationHookDispatcher) DispatchAutomationJobPostFire(
	context.Context,
	hookspkg.AutomationJobPostFirePayload,
) (hookspkg.AutomationJobPostFirePayload, error) {
	return hookspkg.AutomationJobPostFirePayload{}, f.err
}

func (f failingAutomationHookDispatcher) DispatchAutomationTriggerPreFire(
	context.Context,
	hookspkg.AutomationTriggerPreFirePayload,
) (hookspkg.AutomationTriggerPreFirePayload, error) {
	return hookspkg.AutomationTriggerPreFirePayload{}, f.err
}

func (f failingAutomationHookDispatcher) DispatchAutomationTriggerPostFire(
	context.Context,
	hookspkg.AutomationTriggerPostFirePayload,
) (hookspkg.AutomationTriggerPostFirePayload, error) {
	return hookspkg.AutomationTriggerPostFirePayload{}, f.err
}

func (f failingAutomationHookDispatcher) DispatchAutomationRunCompleted(
	context.Context,
	hookspkg.AutomationRunCompletedPayload,
) (hookspkg.AutomationRunCompletedPayload, error) {
	return hookspkg.AutomationRunCompletedPayload{}, f.err
}

func (f failingAutomationHookDispatcher) DispatchAutomationRunFailed(
	context.Context,
	hookspkg.AutomationRunFailedPayload,
) (hookspkg.AutomationRunFailedPayload, error) {
	return hookspkg.AutomationRunFailedPayload{}, f.err
}

type memoryRunStore struct {
	mu      sync.Mutex
	seq     int
	runs    map[string]Run
	updates []Run
}

func newMemoryRunStore() *memoryRunStore {
	return &memoryRunStore{
		runs: make(map[string]Run),
	}
}

func (s *memoryRunStore) CreateRun(ctx context.Context, run Run) (Run, error) {
	if err := ctx.Err(); err != nil {
		return Run{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.seq++
	created := cloneRun(&run)
	if created.ID == "" {
		created.ID = fmt.Sprintf("run-%d", s.seq)
	}
	if _, exists := s.runs[created.ID]; exists {
		return Run{}, ErrRunAlreadyExists
	}
	s.runs[created.ID] = *cloneRun(created)
	return *cloneRun(created), nil
}

func (s *memoryRunStore) GetRun(ctx context.Context, id string) (Run, error) {
	if err := ctx.Err(); err != nil {
		return Run{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[strings.TrimSpace(id)]
	if !ok {
		return Run{}, ErrRunNotFound
	}
	return *cloneRun(&run), nil
}

func (s *memoryRunStore) DeleteRun(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	trimmedID := strings.TrimSpace(id)
	if _, ok := s.runs[trimmedID]; !ok {
		return ErrRunNotFound
	}
	delete(s.runs, trimmedID)
	return nil
}

func (s *memoryRunStore) UpdateRun(ctx context.Context, run Run) (Run, error) {
	if err := ctx.Err(); err != nil {
		return Run{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.runs[run.ID]; !ok {
		return Run{}, ErrRunNotFound
	}
	s.runs[run.ID] = *cloneRun(&run)
	s.updates = append(s.updates, *cloneRun(&run))
	return *cloneRun(&run), nil
}

func (s *memoryRunStore) CountRuns(ctx context.Context, query RunQuery) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var count int64
	for _, run := range s.runs {
		if !matchesRunQuery(run, query) {
			continue
		}
		count++
	}
	return count, nil
}

func (s *memoryRunStore) ListRuns(ctx context.Context, query RunQuery) ([]Run, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	runs := make([]Run, 0, len(s.runs))
	for _, run := range s.runs {
		if !matchesRunQuery(run, query) {
			continue
		}
		runs = append(runs, *cloneRun(&run))
	}
	return runs, nil
}

func (s *memoryRunStore) listRuns() []Run {
	s.mu.Lock()
	defer s.mu.Unlock()

	runs := make([]Run, 0, len(s.runs))
	for _, run := range s.runs {
		runs = append(runs, *cloneRun(&run))
	}
	return runs
}

func (s *memoryRunStore) updateSnapshot() []Run {
	s.mu.Lock()
	defer s.mu.Unlock()

	updates := make([]Run, 0, len(s.updates))
	for idx := range s.updates {
		updates = append(updates, *cloneRun(&s.updates[idx]))
	}
	return updates
}

func matchesRunQuery(run Run, query RunQuery) bool {
	if query.JobID != "" && run.JobID != query.JobID {
		return false
	}
	if query.TriggerID != "" && run.TriggerID != query.TriggerID {
		return false
	}
	if query.Status != "" && run.Status != query.Status {
		return false
	}
	if query.ExcludeID != "" && run.ID == query.ExcludeID {
		return false
	}
	if query.Since.IsZero() && query.Until.IsZero() {
		return true
	}
	if run.StartedAt == nil {
		return false
	}
	if !query.Since.IsZero() && run.StartedAt.Before(query.Since) {
		return false
	}
	if !query.Until.IsZero() && run.StartedAt.After(query.Until) {
		return false
	}
	return true
}

type sessionAttemptPlan struct {
	sessionID     string
	workspaceID   string
	createErr     error
	createStarted chan struct{}
	createRelease chan struct{}
	promptErr     error
	promptStarted chan struct{}
	promptRelease chan struct{}
	stopErr       error
	stopStarted   chan struct{}
	stopRelease   chan struct{}
	events        []acp.AgentEvent
}

type promptCall struct {
	sessionID string
	message   string
}

type stopCall struct {
	sessionID string
	cause     session.StopCause
	detail    string
}

type taskCreateCall struct {
	spec  taskpkg.CreateTask
	actor taskpkg.ActorContext
}

type taskEnqueueCall struct {
	spec  taskpkg.EnqueueRun
	actor taskpkg.ActorContext
}

type recordingTaskService struct {
	createCalls  []taskCreateCall
	enqueueCalls []taskEnqueueCall
	createErr    error
	enqueueErr   error
}

func newRecordingTaskService() *recordingTaskService {
	return &recordingTaskService{}
}

func (s *recordingTaskService) CreateTask(
	_ context.Context,
	spec taskpkg.CreateTask,
	actor taskpkg.ActorContext,
) (*taskpkg.Task, error) {
	s.createCalls = append(s.createCalls, taskCreateCall{spec: spec, actor: actor})
	if s.createErr != nil {
		return nil, s.createErr
	}
	return &taskpkg.Task{
		ID:          "task-1",
		Scope:       spec.Scope,
		WorkspaceID: spec.WorkspaceID,
		Owner:       cloneTaskOwnership(spec.Owner),
	}, nil
}

func (s *recordingTaskService) EnqueueRun(
	_ context.Context,
	spec taskpkg.EnqueueRun,
	actor taskpkg.ActorContext,
) (*taskpkg.Run, error) {
	s.enqueueCalls = append(s.enqueueCalls, taskEnqueueCall{spec: spec, actor: actor})
	if s.enqueueErr != nil {
		return nil, s.enqueueErr
	}
	return &taskpkg.Run{
		ID:             "task-run-1",
		TaskID:         spec.TaskID,
		Origin:         actor.Origin,
		IdempotencyKey: spec.IdempotencyKey,
	}, nil
}

func testNamedParticipation(channelID string) *participation.Request {
	mode := participation.ModeLive
	strategy := participation.StrategyNamed
	return &participation.Request{
		Mode:            &mode,
		ChannelStrategy: &strategy,
		ChannelID:       &channelID,
	}
}

func assertNamedParticipation(t *testing.T, request *participation.Request, channelID string) {
	t.Helper()
	if request == nil || request.Mode == nil || *request.Mode != participation.ModeLive {
		t.Fatalf("network participation = %#v, want live", request)
	}
	if request.ChannelStrategy == nil || *request.ChannelStrategy != participation.StrategyNamed {
		t.Fatalf("network participation strategy = %#v, want named", request.ChannelStrategy)
	}
	if request.ChannelID == nil || *request.ChannelID != channelID {
		t.Fatalf("network participation channel_id = %#v, want %q", request.ChannelID, channelID)
	}
}

type recordingLoopStarter struct {
	mu            sync.Mutex
	validateCalls []LoopTargetValidationRequest
	startCalls    []LoopStartRequest
	validateErr   error
	startErr      error
	startErrors   []error
	runID         string
}

func (s *recordingLoopStarter) ValidateLoopTarget(
	ctx context.Context,
	req LoopTargetValidationRequest,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.validateCalls = append(s.validateCalls, cloneLoopTargetValidationRequest(req))
	return s.validateErr
}

func (s *recordingLoopStarter) StartLoop(ctx context.Context, req LoopStartRequest) (LoopStartResult, error) {
	if err := ctx.Err(); err != nil {
		return LoopStartResult{}, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	callIndex := len(s.startCalls)
	s.startCalls = append(s.startCalls, cloneLoopStartRequest(req))
	if callIndex < len(s.startErrors) && s.startErrors[callIndex] != nil {
		return LoopStartResult{}, s.startErrors[callIndex]
	}
	if s.startErr != nil {
		return LoopStartResult{}, s.startErr
	}
	runID := strings.TrimSpace(s.runID)
	if runID == "" {
		runID = "looprun-1"
	}
	return LoopStartResult{RunID: runID}, nil
}

func (s *recordingLoopStarter) validateCallSnapshot() []LoopTargetValidationRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]LoopTargetValidationRequest, 0, len(s.validateCalls))
	for _, call := range s.validateCalls {
		out = append(out, cloneLoopTargetValidationRequest(call))
	}
	return out
}

func (s *recordingLoopStarter) startCallSnapshot() []LoopStartRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]LoopStartRequest, 0, len(s.startCalls))
	for _, call := range s.startCalls {
		out = append(out, cloneLoopStartRequest(call))
	}
	return out
}

func cloneLoopTargetValidationRequest(req LoopTargetValidationRequest) LoopTargetValidationRequest {
	return LoopTargetValidationRequest{
		WorkspaceID:  strings.TrimSpace(req.WorkspaceID),
		LoopName:     strings.TrimSpace(req.LoopName),
		Kind:         req.Kind,
		Inputs:       cloneJSONMap(req.Inputs),
		InputMapping: cloneStringMap(req.InputMapping),
	}
}

func cloneLoopStartRequest(req LoopStartRequest) LoopStartRequest {
	return LoopStartRequest{
		WorkspaceID:          strings.TrimSpace(req.WorkspaceID),
		LoopName:             strings.TrimSpace(req.LoopName),
		Kind:                 req.Kind,
		Inputs:               cloneJSONMap(req.Inputs),
		InputMapping:         cloneStringMap(req.InputMapping),
		TriggerPayload:       cloneJSONMap(req.TriggerPayload),
		NetworkParticipation: cloneParticipationRequest(req.NetworkParticipation),
		Actor:                req.Actor,
		AutomationRunID:      strings.TrimSpace(req.AutomationRunID),
		ScheduledAt:          cloneTimePointer(req.ScheduledAt),
		CatchUp:              req.CatchUp,
		CatchUpPolicy:        req.CatchUpPolicy,
	}
}

type recordingTaskActorRecorder struct {
	recorded map[string]taskpkg.ActorContext
	deleted  []string
}

func (r *recordingTaskActorRecorder) RecordAutomationSessionTaskActor(
	sessionID string,
	actor taskpkg.ActorContext,
) error {
	if r.recorded == nil {
		r.recorded = make(map[string]taskpkg.ActorContext)
	}
	r.recorded[sessionID] = actor
	return nil
}

func (r *recordingTaskActorRecorder) DeleteAutomationSessionTaskActor(sessionID string) {
	r.deleted = append(r.deleted, sessionID)
}

type recordingSessionCreator struct {
	mu          sync.Mutex
	plans       []sessionAttemptPlan
	nextSession int
	createLog   []session.CreateOpts
	promptLog   []promptCall
	stopLog     []stopCall
	bySessionID map[string]sessionAttemptPlan
}

func newRecordingSessionCreator(plans ...sessionAttemptPlan) *recordingSessionCreator {
	return &recordingSessionCreator{
		plans:       append([]sessionAttemptPlan(nil), plans...),
		bySessionID: make(map[string]sessionAttemptPlan),
	}
}

func (c *recordingSessionCreator) Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.createLog = append(c.createLog, opts)
	plan := sessionAttemptPlan{}
	if len(c.plans) > 0 {
		plan = c.plans[0]
		c.plans = c.plans[1:]
	}
	c.nextSession++
	sessionID := plan.sessionID
	if sessionID == "" {
		sessionID = fmt.Sprintf("sess-%d", c.nextSession)
	}
	c.bySessionID[sessionID] = plan
	c.mu.Unlock()

	notify(plan.createStarted)
	if err := waitForRelease(ctx, plan.createRelease); err != nil {
		return nil, err
	}
	if plan.createErr != nil {
		return nil, plan.createErr
	}

	return &session.Session{ID: sessionID, WorkspaceID: plan.workspaceID}, nil
}

func (c *recordingSessionCreator) Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.promptLog = append(c.promptLog, promptCall{sessionID: id, message: msg})
	plan, ok := c.bySessionID[id]
	c.mu.Unlock()
	if !ok {
		plan = sessionAttemptPlan{}
	}

	notify(plan.promptStarted)
	if plan.promptErr != nil {
		return nil, plan.promptErr
	}

	out := make(chan acp.AgentEvent, len(plan.events))
	go func() {
		defer close(out)
		if err := waitForRelease(ctx, plan.promptRelease); err != nil {
			return
		}
		for _, event := range plan.events {
			select {
			case <-ctx.Done():
				return
			case out <- event:
			}
		}
	}()

	return out, nil
}

func (c *recordingSessionCreator) StopWithCause(
	ctx context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	plan, ok := c.bySessionID[id]
	c.mu.Unlock()
	if !ok {
		plan = sessionAttemptPlan{}
	}

	notify(plan.stopStarted)
	if err := waitForRelease(ctx, plan.stopRelease); err != nil {
		return err
	}
	if plan.stopErr != nil {
		return plan.stopErr
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.stopLog = append(c.stopLog, stopCall{
		sessionID: strings.TrimSpace(id),
		cause:     cause,
		detail:    strings.TrimSpace(detail),
	})
	return nil
}

func (c *recordingSessionCreator) createCalls() []session.CreateOpts {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]session.CreateOpts(nil), c.createLog...)
}

func (c *recordingSessionCreator) promptCalls() []promptCall {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]promptCall(nil), c.promptLog...)
}

func (c *recordingSessionCreator) stopCalls() []stopCall {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]stopCall(nil), c.stopLog...)
}

func waitForRelease(ctx context.Context, release <-chan struct{}) error {
	if release == nil {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-release:
		return nil
	}
}

func notify(ch chan struct{}) {
	if ch == nil {
		return
	}

	select {
	case ch <- struct{}{}:
	default:
	}
}

func newTestDispatcher(t *testing.T, creator SessionCreator, store RunStore, opts ...DispatcherOption) *Dispatcher {
	t.Helper()

	allOpts := append([]DispatcherOption{
		WithDispatcherGlobalWorkspacePath(t.TempDir()),
		WithDispatcherNow(func() time.Time {
			return time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
		}),
	}, opts...)

	dispatcher, err := NewDispatcher(creator, store, allOpts...)
	if err != nil {
		t.Fatalf("NewDispatcher() error = %v", err)
	}
	return dispatcher
}

func testJob(scope Scope, name string, workspaceID string) Job {
	return Job{
		ID:          "job-" + name,
		Scope:       scope,
		Name:        name,
		AgentName:   "researcher",
		WorkspaceID: workspaceID,
		Prompt:      "Summarize the latest state.",
		Schedule: &ScheduleSpec{
			Mode:     ScheduleModeEvery,
			Interval: "30m",
		},
		Enabled:   true,
		Retry:     DefaultRetryConfig(),
		FireLimit: DefaultFireLimitConfig(),
		Source:    JobSourceDynamic,
	}
}

func testTrigger(scope Scope, name string, workspaceID string) Trigger {
	return Trigger{
		ID:               "trigger-" + name,
		Scope:            scope,
		Name:             name,
		AgentName:        "reviewer",
		WorkspaceID:      workspaceID,
		Prompt:           `Review payload {{ index .Data "payload" }}`,
		Event:            "webhook",
		Enabled:          true,
		Retry:            DefaultRetryConfig(),
		FireLimit:        DefaultFireLimitConfig(),
		Source:           JobSourceDynamic,
		WebhookID:        "wbh_" + name,
		WebhookSecretRef: "env:AGH_TEST_WEBHOOK_SECRET",
	}
}

func testEnvelope(scope Scope, workspaceID string) ActivationEnvelope {
	return ActivationEnvelope{
		Kind:        "webhook",
		Scope:       scope,
		WorkspaceID: workspaceID,
		Source:      ActivationSourceWebhook,
		Data: map[string]any{
			"payload": "deploy",
		},
	}
}

func pointerToTrigger(trigger Trigger) *Trigger {
	clone := trigger
	return &clone
}

func pointerToEnvelope(envelope ActivationEnvelope) *ActivationEnvelope {
	clone := envelope
	return &clone
}
