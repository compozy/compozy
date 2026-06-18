package parallelrun

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/recovery"
	"github.com/compozy/compozy/internal/core/workspace"
)

func TestParallelFSMModelsRequiredHappyPathTransitions(t *testing.T) {
	t.Parallel()

	machine := newParallelFSM()
	if got := machine.Current(); got != string(parallelStatePlanning) {
		t.Fatalf("initial state = %q, want %q", got, parallelStatePlanning)
	}
	transitions := []struct {
		event string
		want  parallelState
	}{
		{event: parallelEventStartWave, want: parallelStateWaveRunning},
		{event: parallelEventMergeWave, want: parallelStateWaveMerging},
		{event: parallelEventFinishWave, want: parallelStateWaveDone},
		{event: parallelEventFinalize, want: parallelStateFinalizing},
		{event: parallelEventComplete, want: parallelStateCompleted},
	}
	for _, transition := range transitions {
		if err := machine.Event(context.Background(), transition.event); err != nil {
			t.Fatalf("Event(%q) error = %v", transition.event, err)
		}
		if got := machine.Current(); got != string(transition.want) {
			t.Fatalf("after %q state = %q, want %q", transition.event, got, transition.want)
		}
	}

	canceled := newParallelFSM()
	if err := canceled.Event(context.Background(), parallelEventStartWave); err != nil {
		t.Fatalf("start wave before cancel: %v", err)
	}
	if err := canceled.Event(context.Background(), parallelEventCancel); err != nil {
		t.Fatalf("cancel event error = %v", err)
	}
	if got := canceled.Current(); got != string(parallelStateCanceled) {
		t.Fatalf("canceled state = %q, want %q", got, parallelStateCanceled)
	}

	recovering := newParallelFSM()
	if err := recovering.Event(context.Background(), parallelEventStartWave); err != nil {
		t.Fatalf("start wave before recover: %v", err)
	}
	if err := recovering.Event(context.Background(), parallelEventRecoverWave); err != nil {
		t.Fatalf("recover event error = %v", err)
	}
	if got := recovering.Current(); got != string(parallelStateWaveRecovering) {
		t.Fatalf("recovering state = %q, want %q", got, parallelStateWaveRecovering)
	}
	if err := recovering.Event(context.Background(), parallelEventMergeWave); err != nil {
		t.Fatalf("merge after recover error = %v", err)
	}
}

func TestParallelExecutionOrchestratorBoundsWaveConcurrency(t *testing.T) {
	t.Parallel()

	plan := testParallelPlan(t, []model.TaskEntry{
		testTaskEntry("task_01"),
		testTaskEntry("task_02"),
		testTaskEntry("task_03"),
	}, 2)
	worktrees := newFakeWorktreeLifecycle()
	launcher := newBlockingLauncher(t, len(plan.Tasks))
	orchestrator := NewParallelExecutionOrchestrator(worktrees, launcher)

	done := make(chan runResult, 1)
	go func() {
		outcome, err := orchestrator.Run(context.Background(), plan)
		done <- runResult{outcome: outcome, err: err}
	}()

	launcher.waitForEntered(t, 2)
	if got := launcher.maxInFlight(); got != 2 {
		close(launcher.release)
		t.Fatalf("max in-flight tasks = %d, want 2", got)
	}
	select {
	case taskID := <-launcher.entered:
		close(launcher.release)
		t.Fatalf("task %s entered before a concurrency slot was released", taskID)
	default:
	}
	close(launcher.release)
	launcher.waitForEntered(t, 1)
	result := waitRunResult(t, done)
	if result.err != nil {
		t.Fatalf("Run() error = %v", result.err)
	}
	if result.outcome.Status != ParallelOutcomeCompleted {
		t.Fatalf("outcome status = %q, want %q", result.outcome.Status, ParallelOutcomeCompleted)
	}
	if got := launcher.maxInFlight(); got != 2 {
		t.Fatalf("max in-flight tasks = %d, want 2", got)
	}
}

func TestParallelExecutionOrchestratorMergesWaveSeriallyInTaskOrder(t *testing.T) {
	t.Parallel()

	plan := testParallelPlan(t, []model.TaskEntry{
		testTaskEntry("task_03"),
		testTaskEntry("task_01"),
		testTaskEntry("task_02"),
	}, 3)
	worktrees := newFakeWorktreeLifecycle()
	launcher := fakeTaskLauncherFunc(func(_ context.Context, spec TaskLaunchSpec) (PreparedTaskRun, error) {
		return successfulPreparedTaskRun(spec), nil
	})

	outcome, err := NewParallelExecutionOrchestrator(worktrees, launcher).Run(context.Background(), plan)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(outcome.Tasks) != 3 {
		t.Fatalf("merged tasks = %d, want 3", len(outcome.Tasks))
	}
	want := []int{1, 2, 3}
	if got := worktrees.commitOrder(); !reflect.DeepEqual(got, want) {
		t.Fatalf("commit order = %#v, want %#v", got, want)
	}
	if got := worktrees.mergeOrder(); !reflect.DeepEqual(
		got,
		[]string{"commit-task-01", "commit-task-02", "commit-task-03"},
	) {
		t.Fatalf("merge order = %#v, want task-number commit order", got)
	}
}

func TestParallelExecutionOrchestratorAllocatesNextWaveFromPostMergeHead(t *testing.T) {
	t.Parallel()

	plan := testParallelPlan(t, []model.TaskEntry{
		testTaskEntry("task_01"),
		testTaskEntry("task_02", "task_01"),
	}, 1)
	worktrees := newFakeWorktreeLifecycle()
	worktrees.heads = []string{"head-after-wave-1", "head-after-wave-2"}
	launcher := &recordingLauncher{}

	outcome, err := NewParallelExecutionOrchestrator(worktrees, launcher).Run(context.Background(), plan)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if outcome.Status != ParallelOutcomeCompleted {
		t.Fatalf("status = %q, want %q", outcome.Status, ParallelOutcomeCompleted)
	}
	got := launcher.baseByTask()
	want := map[TaskID]string{
		"task_01": "base-sha",
		"task_02": "head-after-wave-1",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("launch bases = %#v, want %#v", got, want)
	}
}

func TestParallelExecutionOrchestratorCancellationTransitionsAndJoinsWorkers(t *testing.T) {
	t.Parallel()

	plan := testParallelPlan(t, []model.TaskEntry{
		testTaskEntry("task_01"),
		testTaskEntry("task_02"),
		testTaskEntry("task_03"),
	}, 2)
	worktrees := newFakeWorktreeLifecycle()
	launcher := newBlockingLauncher(t, len(plan.Tasks))
	orchestrator := NewParallelExecutionOrchestrator(worktrees, launcher)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan runResult, 1)
	go func() {
		outcome, err := orchestrator.Run(ctx, plan)
		done <- runResult{outcome: outcome, err: err}
	}()

	launcher.waitForEntered(t, 2)
	cancel()
	result := waitRunResult(t, done)
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("Run() error = %v, want context canceled", result.err)
	}
	if result.outcome.Status != ParallelOutcomeCanceled {
		t.Fatalf("outcome status = %q, want %q", result.outcome.Status, ParallelOutcomeCanceled)
	}
	launcher.assertNoActiveWorkers(t)
}

func TestParallelExecutionOrchestratorRecoversFailedTaskThenMergesRecoveredStatus(t *testing.T) {
	t.Parallel()

	plan := testParallelPlan(t, []model.TaskEntry{
		testTaskEntry("task_01"),
	}, 1)
	plan.Recovery = enabledParallelRecoveryConfig(1)
	failedTask := failedPreparedTaskRun(taskLaunchSpecForTask(plan, "task_01"), "tests failed")
	failedTask.restartOutcomes = []recovery.RunOutcome{
		succeededTaskRunOutcome(taskLaunchSpecForTask(plan, "task_01")),
	}
	launcher := &scriptedTaskLauncher{
		prepared: map[TaskID]*fakePreparedTaskRun{"task_01": failedTask},
	}
	strategy := &fakeParallelRecoveryStrategy{
		verdicts: []recovery.TriageVerdict{{Decision: recovery.VerdictFixed, Reason: "fixed"}},
	}
	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	worktrees := newFakeWorktreeLifecycle()

	outcome, err := NewParallelExecutionOrchestrator(
		worktrees,
		launcher,
		WithRecoveryStrategy(strategy),
		WithLogger(logger),
	).Run(context.Background(), plan)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got := len(strategy.inputs); got != 1 {
		t.Fatalf("recovery strategy calls = %d, want 1", got)
	}
	if !reflect.DeepEqual(failedTask.restartCalls, [][]string{{"task-01"}}) {
		t.Fatalf("RestartFailed calls = %#v, want failed job restart", failedTask.restartCalls)
	}
	if len(outcome.Tasks) != 1 {
		t.Fatalf("tasks = %d, want 1", len(outcome.Tasks))
	}
	if got := outcome.Tasks[0].Status; got != TaskOutcomeRecovered {
		t.Fatalf("task status = %q, want %q", got, TaskOutcomeRecovered)
	}
	if got := worktrees.mergeOrder(); !reflect.DeepEqual(got, []string{"commit-task-01"}) {
		t.Fatalf("merge order = %#v, want recovered task merge", got)
	}
	if !strings.Contains(logs.String(), string(parallelStateWaveRecovering)) {
		t.Fatalf("fsm logs %q do not include %q", logs.String(), parallelStateWaveRecovering)
	}
}

func TestParallelExecutionOrchestratorExhaustionSkipsDependentsAndPartiallyFinalizes(t *testing.T) {
	t.Parallel()

	plan := testParallelPlan(t, []model.TaskEntry{
		testTaskEntry("task_01"),
		testTaskEntry("task_02", "task_01"),
		testTaskEntry("task_03"),
	}, 2)
	plan.Recovery = enabledParallelRecoveryConfig(1)
	task1 := failedPreparedTaskRun(taskLaunchSpecForTask(plan, "task_01"), "unrecoverable")
	task3 := successfulPreparedTaskRun(taskLaunchSpecForTask(plan, "task_03")).(*fakePreparedTaskRun)
	launcher := &scriptedTaskLauncher{
		prepared: map[TaskID]*fakePreparedTaskRun{
			"task_01": task1,
			"task_03": task3,
		},
	}
	strategy := &fakeParallelRecoveryStrategy{
		verdicts: []recovery.TriageVerdict{{Decision: recovery.VerdictReject, Reason: "cannot fix"}},
	}
	worktrees := newFakeWorktreeLifecycle()

	outcome, err := NewParallelExecutionOrchestrator(
		worktrees,
		launcher,
		WithRecoveryStrategy(strategy),
	).Run(context.Background(), plan)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	statusByTask := taskStatusesByID(outcome.Tasks)
	if got := statusByTask["task_01"].Status; got != TaskOutcomeFailed {
		t.Fatalf("task_01 status = %q, want %q", got, TaskOutcomeFailed)
	}
	if got := statusByTask["task_02"].Status; got != TaskOutcomeSkipped {
		t.Fatalf("task_02 status = %q, want %q", got, TaskOutcomeSkipped)
	}
	if got := statusByTask["task_02"].StatusReport(); got != "skipped (blocked by task_01)" {
		t.Fatalf("task_02 status report = %q", got)
	}
	if got := statusByTask["task_03"].Status; got != TaskOutcomeMerged {
		t.Fatalf("task_03 status = %q, want %q", got, TaskOutcomeMerged)
	}
	if launcher.launched("task_02") {
		t.Fatalf("task_02 prepared unexpectedly; blocked dependents must not execute")
	}
	if task3.executeCalls != 1 {
		t.Fatalf("independent task execute calls = %d, want 1", task3.executeCalls)
	}
	if got := worktrees.mergeOrder(); !reflect.DeepEqual(got, []string{"commit-task-03"}) {
		t.Fatalf("merge order = %#v, want only independent task merge", got)
	}
	if !worktrees.wasFastForwarded() {
		t.Fatal("expected partial success to fast-forward")
	}
	if got := len(strategy.inputs); got != 1 {
		t.Fatalf("recovery strategy calls = %d, want 1", got)
	}
}

type runResult struct {
	outcome ParallelOutcome
	err     error
}

type fakeTaskLauncherFunc func(context.Context, TaskLaunchSpec) (PreparedTaskRun, error)

func (f fakeTaskLauncherFunc) PrepareTask(ctx context.Context, spec TaskLaunchSpec) (PreparedTaskRun, error) {
	return f(ctx, spec)
}

type fakeWorktreeLifecycle struct {
	mu              sync.Mutex
	heads           []string
	headCalls       int
	committed       []int
	merged          []string
	removed         []string
	fastForwarded   bool
	discardedBranch bool
	pruned          bool
}

func newFakeWorktreeLifecycle() *fakeWorktreeLifecycle {
	return &fakeWorktreeLifecycle{heads: []string{"head-after-wave"}}
}

func (f *fakeWorktreeLifecycle) CreateIntegrationBranch(context.Context, IntegrationSpec) error {
	return nil
}

func (f *fakeWorktreeLifecycle) Commit(_ context.Context, path string, _ string) (string, error) {
	number := taskNumberFromPath(path)
	f.mu.Lock()
	f.committed = append(f.committed, number)
	f.mu.Unlock()
	return fmt.Sprintf("commit-task-%02d", number), nil
}

func (f *fakeWorktreeLifecycle) SquashMerge(
	_ context.Context,
	_ string,
	worktreeRef string,
	_ string,
) (ConflictSet, error) {
	f.mu.Lock()
	f.merged = append(f.merged, worktreeRef)
	f.mu.Unlock()
	return ConflictSet{Clean: true}, nil
}

func (f *fakeWorktreeLifecycle) Head(context.Context, string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.headCalls < len(f.heads) {
		head := f.heads[f.headCalls]
		f.headCalls++
		return head, nil
	}
	f.headCalls++
	return fmt.Sprintf("head-after-wave-%d", f.headCalls), nil
}

func (f *fakeWorktreeLifecycle) FastForward(context.Context, string, string, string) error {
	f.mu.Lock()
	f.fastForwarded = true
	f.mu.Unlock()
	return nil
}

func (f *fakeWorktreeLifecycle) DiscardIntegrationBranch(context.Context, string, string, string) error {
	f.mu.Lock()
	f.discardedBranch = true
	f.mu.Unlock()
	return nil
}

func (f *fakeWorktreeLifecycle) Remove(_ context.Context, _ string, path string) error {
	f.mu.Lock()
	f.removed = append(f.removed, path)
	f.mu.Unlock()
	return nil
}

func (f *fakeWorktreeLifecycle) Prune(context.Context, string) error {
	f.mu.Lock()
	f.pruned = true
	f.mu.Unlock()
	return nil
}

func (f *fakeWorktreeLifecycle) commitOrder() []int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]int(nil), f.committed...)
}

func (f *fakeWorktreeLifecycle) mergeOrder() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.merged...)
}

func (f *fakeWorktreeLifecycle) wasFastForwarded() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.fastForwarded
}

type scriptedTaskLauncher struct {
	mu       sync.Mutex
	prepared map[TaskID]*fakePreparedTaskRun
	calls    []TaskID
}

func (l *scriptedTaskLauncher) PrepareTask(_ context.Context, spec TaskLaunchSpec) (PreparedTaskRun, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, spec.Task.ID)
	prepared, ok := l.prepared[spec.Task.ID]
	if !ok {
		return nil, fmt.Errorf("unexpected task launch %s", spec.Task.ID)
	}
	return prepared, nil
}

func (l *scriptedTaskLauncher) launched(taskID TaskID) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, call := range l.calls {
		if call == taskID {
			return true
		}
	}
	return false
}

type recordingLauncher struct {
	mu    sync.Mutex
	bases map[TaskID]string
}

func (l *recordingLauncher) PrepareTask(_ context.Context, spec TaskLaunchSpec) (PreparedTaskRun, error) {
	l.mu.Lock()
	if l.bases == nil {
		l.bases = map[TaskID]string{}
	}
	l.bases[spec.Task.ID] = spec.Base.Commit
	l.mu.Unlock()
	return successfulPreparedTaskRun(spec), nil
}

func (l *recordingLauncher) baseByTask() map[TaskID]string {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make(map[TaskID]string, len(l.bases))
	for taskID, base := range l.bases {
		result[taskID] = base
	}
	return result
}

type blockingLauncher struct {
	t       *testing.T
	entered chan TaskID
	release chan struct{}

	mu       sync.Mutex
	inFlight int
	maxSeen  int
}

func newBlockingLauncher(t *testing.T, taskCount int) *blockingLauncher {
	t.Helper()
	return &blockingLauncher{
		t:       t,
		entered: make(chan TaskID, taskCount),
		release: make(chan struct{}),
	}
}

func (l *blockingLauncher) PrepareTask(_ context.Context, spec TaskLaunchSpec) (PreparedTaskRun, error) {
	return &blockingPreparedTaskRun{
		launcher: l,
		taskID:   spec.Task.ID,
		result:   taskRunResultForSpec(spec),
		outcome:  succeededTaskRunOutcome(spec),
	}, nil
}

type blockingPreparedTaskRun struct {
	launcher *blockingLauncher
	taskID   TaskID
	result   TaskRunResult
	outcome  recovery.RunOutcome
}

func (r *blockingPreparedTaskRun) Execute(ctx context.Context) (recovery.RunOutcome, error) {
	l := r.launcher
	l.mu.Lock()
	l.inFlight++
	if l.inFlight > l.maxSeen {
		l.maxSeen = l.inFlight
	}
	l.mu.Unlock()
	l.entered <- r.taskID
	defer func() {
		l.mu.Lock()
		l.inFlight--
		l.mu.Unlock()
	}()

	select {
	case <-l.release:
	case <-ctx.Done():
		return recovery.RunOutcome{}, ctx.Err()
	}
	return r.outcome, nil
}

func (r *blockingPreparedTaskRun) RestartFailed(context.Context, []string) (recovery.RunOutcome, error) {
	return recovery.RunOutcome{}, errors.New("blocking prepared task run should not restart")
}

func (r *blockingPreparedTaskRun) Result() TaskRunResult {
	return r.result
}

func (r *blockingPreparedTaskRun) FailedConfig() *model.RuntimeConfig {
	return &model.RuntimeConfig{}
}

func (l *blockingLauncher) waitForEntered(t *testing.T, count int) {
	t.Helper()
	for range count {
		select {
		case <-l.entered:
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for task %d to enter launcher", count)
		}
	}
}

func (l *blockingLauncher) maxInFlight() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.maxSeen
}

func (l *blockingLauncher) assertNoActiveWorkers(t *testing.T) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		l.mu.Lock()
		inFlight := l.inFlight
		l.mu.Unlock()
		if inFlight == 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("launcher still has %d active workers", inFlight)
		case <-ticker.C:
		}
	}
}

func testParallelPlan(t *testing.T, entries []model.TaskEntry, maxConcurrency int) ParallelPlan {
	t.Helper()
	waves, err := BuildWaves(entries)
	if err != nil {
		t.Fatalf("BuildWaves(): %v", err)
	}
	tasks := make([]TaskSpec, 0, len(entries))
	for _, entry := range entries {
		number := taskNumberFromID(TaskID(entry.ID))
		tasks = append(tasks, TaskSpec{
			ID:     TaskID(entry.ID),
			Number: number,
			Title:  entry.Title,
			Slug:   "demo",
		})
	}
	enabled := true
	return ParallelPlan{
		RunID:             "parallel-run",
		WorkspaceRoot:     "/repo",
		BaseBranch:        "main",
		BaseCommit:        "base-sha",
		IntegrationBranch: "compozy/parallel-run",
		IntegrationPath:   "/repo-integration",
		Waves:             waves,
		Tasks:             tasks,
		Config: workspace.ParallelTasksConfig{
			Enabled:        &enabled,
			MaxConcurrency: &maxConcurrency,
		},
	}
}

func taskLaunchSpecForTask(plan ParallelPlan, taskID TaskID) TaskLaunchSpec {
	tasksByID := taskSpecsByID(plan.Tasks)
	return TaskLaunchSpec{
		RunID:     plan.RunID,
		WaveIndex: 0,
		Task:      tasksByID[taskID],
		Base: WorktreeBase{
			Branch: plan.IntegrationBranch,
			Commit: plan.BaseCommit,
		},
	}
}

func taskStatusesByID(outcomes []TaskOutcome) map[TaskID]TaskOutcome {
	result := make(map[TaskID]TaskOutcome, len(outcomes))
	for index := range outcomes {
		outcome := &outcomes[index]
		result[outcome.Task.ID] = *outcome
	}
	return result
}

func enabledParallelRecoveryConfig(maxAttempts int) workspace.AgentRecoveryConfig {
	enabled := true
	return workspace.AgentRecoveryConfig{
		Enabled:     &enabled,
		MaxAttempts: &maxAttempts,
	}
}

func testTaskEntry(id string, dependencies ...string) model.TaskEntry {
	return model.TaskEntry{
		ID:           id,
		Status:       "pending",
		Title:        id,
		TaskType:     "backend",
		Dependencies: dependencies,
	}
}

func taskRunResultForSpec(spec TaskLaunchSpec) TaskRunResult {
	return TaskRunResult{
		Task:         spec.Task,
		RunID:        fmt.Sprintf("run-task-%02d", spec.Task.Number),
		WorktreePath: fmt.Sprintf("/worktree/task_%02d", spec.Task.Number),
		BaseBranch:   spec.Base.Branch,
		BaseCommit:   spec.Base.Commit,
	}
}

func successfulPreparedTaskRun(spec TaskLaunchSpec) PreparedTaskRun {
	return &fakePreparedTaskRun{
		result:         taskRunResultForSpec(spec),
		executeOutcome: succeededTaskRunOutcome(spec),
		failedConfig:   &model.RuntimeConfig{WorkspaceRoot: fmt.Sprintf("/worktree/task_%02d", spec.Task.Number)},
	}
}

func failedPreparedTaskRun(spec TaskLaunchSpec, message string) *fakePreparedTaskRun {
	return &fakePreparedTaskRun{
		result:         taskRunResultForSpec(spec),
		executeOutcome: failedTaskRunOutcome(spec, message),
		executeErr:     errors.New(message),
		failedConfig:   &model.RuntimeConfig{WorkspaceRoot: fmt.Sprintf("/worktree/task_%02d", spec.Task.Number)},
	}
}

type fakePreparedTaskRun struct {
	result          TaskRunResult
	failedConfig    *model.RuntimeConfig
	executeOutcome  recovery.RunOutcome
	executeErr      error
	restartOutcomes []recovery.RunOutcome
	restartErrs     []error
	restartCalls    [][]string
	executeCalls    int
}

func (f *fakePreparedTaskRun) Execute(context.Context) (recovery.RunOutcome, error) {
	f.executeCalls++
	return f.executeOutcome, f.executeErr
}

func (f *fakePreparedTaskRun) RestartFailed(_ context.Context, failedJobIDs []string) (recovery.RunOutcome, error) {
	f.restartCalls = append(f.restartCalls, append([]string(nil), failedJobIDs...))
	idx := len(f.restartCalls) - 1
	if idx < len(f.restartOutcomes) {
		err := errorAt(f.restartErrs, idx)
		return f.restartOutcomes[idx], err
	}
	return f.executeOutcome, f.executeErr
}

func (f *fakePreparedTaskRun) Result() TaskRunResult {
	return f.result
}

func (f *fakePreparedTaskRun) FailedConfig() *model.RuntimeConfig {
	return f.failedConfig
}

func errorAt(errs []error, idx int) error {
	if idx < len(errs) {
		return errs[idx]
	}
	return nil
}

func succeededTaskRunOutcome(spec TaskLaunchSpec) recovery.RunOutcome {
	return recovery.RunOutcome{
		RunID:  fmt.Sprintf("run-task-%02d", spec.Task.Number),
		Status: recovery.StatusSucceeded,
		Jobs: []recovery.JobOutcome{{
			SafeName: fmt.Sprintf("task-%02d", spec.Task.Number),
			Status:   recovery.StatusSucceeded,
		}},
	}
}

func failedTaskRunOutcome(spec TaskLaunchSpec, message string) recovery.RunOutcome {
	return recovery.RunOutcome{
		RunID:  fmt.Sprintf("run-task-%02d", spec.Task.Number),
		Status: recovery.StatusFailed,
		Jobs: []recovery.JobOutcome{{
			SafeName: fmt.Sprintf("task-%02d", spec.Task.Number),
			Status:   recovery.StatusFailed,
			ExitCode: 1,
			Error:    message,
		}},
	}
}

type fakeParallelRecoveryStrategy struct {
	verdicts []recovery.TriageVerdict
	inputs   []recovery.RemediationInput
}

func (s *fakeParallelRecoveryStrategy) Name() string {
	return "fake-parallel-recovery"
}

func (s *fakeParallelRecoveryStrategy) Remediate(
	_ context.Context,
	in recovery.RemediationInput,
) (recovery.TriageVerdict, error) {
	s.inputs = append(s.inputs, in)
	idx := len(s.inputs) - 1
	if idx < len(s.verdicts) {
		return s.verdicts[idx], nil
	}
	return recovery.TriageVerdict{Decision: recovery.VerdictReject, Reason: "unrecoverable"}, nil
}

func taskNumberFromPath(path string) int {
	_, suffix, ok := strings.Cut(strings.TrimSpace(path), "task_")
	if !ok {
		return 0
	}
	number, err := strconv.Atoi(strings.TrimLeft(suffix, "0"))
	if err != nil {
		return 0
	}
	return number
}

func taskNumberFromID(id TaskID) int {
	_, suffix, ok := strings.Cut(string(id), "task_")
	if !ok {
		return 0
	}
	number, err := strconv.Atoi(strings.TrimLeft(suffix, "0"))
	if err != nil {
		return 0
	}
	return number
}

func waitRunResult(t *testing.T, ch <-chan runResult) runResult {
	t.Helper()
	select {
	case result := <-ch:
		return result
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for orchestrator")
	}
	return runResult{}
}
