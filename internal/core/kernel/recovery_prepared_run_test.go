package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/kernel/commands"
	"github.com/compozy/compozy/internal/core/model"
	corerun "github.com/compozy/compozy/internal/core/run"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/internal/core/run/recovery"
	"github.com/compozy/compozy/internal/core/workspace"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
)

func TestKernelWorkflowPreparedRunRestartFailedFiltersBySafeName(t *testing.T) {
	t.Parallel()

	scope := newKernelRecoveryScope(t, "filter-run")
	prepare := kernelRecoveryPreparation(scope.RunArtifacts(), []model.Job{
		{SafeName: "already-green"},
		{SafeName: "needs-restart"},
		{SafeName: "also-green"},
	})
	fake := &fakeOperations{
		prepareResult: prepare,
		executeHook: func(_ context.Context, prep *model.SolvePreparation, _ *model.RuntimeConfig, _ int) error {
			writeKernelRunResult(t, prep.RunArtifacts, recovery.StatusSucceeded, prep.Jobs, nil)
			return nil
		},
	}
	prepared := newKernelWorkflowPreparedRun(fake, &model.RuntimeConfig{}, scope)

	outcome, err := prepared.RestartFailed(context.Background(), []string{"needs-restart"})
	if err != nil {
		t.Fatalf("RestartFailed() error = %v", err)
	}
	if outcome.Status != recovery.StatusSucceeded {
		t.Fatalf("status = %q, want succeeded", outcome.Status)
	}
	if len(fake.executeCalls) != 1 {
		t.Fatalf("execute calls = %d, want 1", len(fake.executeCalls))
	}
	gotJobs := fake.executeCalls[0].prep.Jobs
	if len(gotJobs) != 1 || gotJobs[0].SafeName != "needs-restart" {
		t.Fatalf("executed jobs = %#v, want only needs-restart", gotJobs)
	}
}

func TestRunStartRecoveryDisabledUsesOriginalPreparedPath(t *testing.T) {
	t.Parallel()

	scope := newKernelRecoveryScope(t, "disabled-run")
	fake := &fakeOperations{
		openResult: scope,
		prepareResult: kernelRecoveryPreparation(scope.RunArtifacts(), []model.Job{{
			SafeName: "task-001",
		}}),
	}
	deps := testKernelDeps(fake)
	deps.RecoveryStrategy = &kernelFakeRecoveryStrategy{
		verdicts: []recovery.TriageVerdict{{Decision: recovery.VerdictFixed}},
	}

	result, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		BuildDefault(deps),
		commands.RunStartCommand{
			Runtime: model.RuntimeConfig{
				WorkspaceRoot: "/workspace",
				Name:          "demo",
				Mode:          model.ExecutionModePRDTasks,
				IDE:           model.IDECodex,
				BatchSize:     1,
			},
			Recovery: workspace.AgentRecoveryConfig{Enabled: boolPtr(false)},
		},
	)
	if err != nil {
		t.Fatalf("dispatch run start: %v", err)
	}
	if result.Status != runStartStatusSucceeded {
		t.Fatalf("status = %q, want %q", result.Status, runStartStatusSucceeded)
	}
	if len(fake.executeCalls) != 1 {
		t.Fatalf("execute calls = %d, want 1", len(fake.executeCalls))
	}
	if got := len(deps.RecoveryStrategy.(*kernelFakeRecoveryStrategy).inputs); got != 0 {
		t.Fatalf("strategy calls = %d, want 0", got)
	}
}

func TestRunStartRecoveryRecoversAndRestartsOnlyFailedJobs(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("unit tests failed")
	scope := newKernelRecoveryScope(t, "recovered-run")
	jobs := []model.Job{{SafeName: "unit-tests"}, {SafeName: "lint"}}
	fake := &fakeOperations{
		openResult:    scope,
		prepareResult: kernelRecoveryPreparation(scope.RunArtifacts(), jobs),
		executeHook: func(_ context.Context, prep *model.SolvePreparation, _ *model.RuntimeConfig, call int) error {
			switch call {
			case 1:
				writeKernelRunResult(
					t,
					prep.RunArtifacts,
					recovery.StatusFailed,
					prep.Jobs,
					map[string]recovery.RunStatus{
						"unit-tests": recovery.StatusFailed,
						"lint":       recovery.StatusSucceeded,
					},
				)
				return originalErr
			case 2:
				if got := safeNames(prep.Jobs); !reflect.DeepEqual(got, []string{"unit-tests"}) {
					t.Fatalf("restart jobs = %#v, want [unit-tests]", got)
				}
				writeKernelRunResult(t, prep.RunArtifacts, recovery.StatusSucceeded, prep.Jobs, nil)
				return nil
			default:
				t.Fatalf("unexpected execute call %d", call)
				return nil
			}
		},
	}
	strategy := &kernelFakeRecoveryStrategy{
		verdicts: []recovery.TriageVerdict{{Decision: recovery.VerdictFixed, Reason: "fixed test"}},
	}
	deps := testKernelDeps(fake)
	deps.RecoveryStrategy = strategy

	result, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		BuildDefault(deps),
		commands.RunStartCommand{
			Runtime: model.RuntimeConfig{
				WorkspaceRoot: "/workspace",
				Name:          "demo",
				Mode:          model.ExecutionModePRDTasks,
				IDE:           model.IDECodex,
				BatchSize:     1,
			},
			Recovery: enabledKernelRecoveryConfig(),
		},
	)
	if err != nil {
		t.Fatalf("dispatch run start: %v", err)
	}
	if result.Status != string(recovery.StatusSucceeded) {
		t.Fatalf("status = %q, want succeeded", result.Status)
	}
	if len(fake.executeCalls) != 2 {
		t.Fatalf("execute calls = %d, want 2", len(fake.executeCalls))
	}
	if len(strategy.inputs) != 1 {
		t.Fatalf("strategy calls = %d, want 1", len(strategy.inputs))
	}
}

func TestRunStartRecoveryIntegrationUsesDeterministicACPAndRealRestart(t *testing.T) {
	workspaceRoot := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	installKernelRuntimeProbeStub(t, "codex-acp")
	initKernelGitRepo(t, workspaceRoot)
	tasksDir := filepath.Join(workspaceRoot, model.TasksBaseDir(), "demo")
	if err := os.MkdirAll(tasksDir, 0o755); err != nil {
		t.Fatalf("mkdir tasks dir: %v", err)
	}
	writeKernelTaskFile(
		t,
		tasksDir,
		"task_01.md",
		"Needs recovery",
		"Create task_01.done, but this task requires fixed.txt before it can pass.",
	)
	writeKernelTaskFile(
		t,
		tasksDir,
		"task_02.md",
		"Already green",
		"Create task_02.done. This task should not be restarted after recovery.",
	)
	fakeACP := newKernelBoundaryACP(t, workspaceRoot)
	restore := corerun.SwapNewAgentClientForTest(
		func(context.Context, agent.ClientConfig) (agent.Client, error) {
			return fakeACP, nil
		},
	)
	t.Cleanup(restore)

	dispatcher := BuildDefault(KernelDeps{AgentRegistry: agent.DefaultRegistry()})
	result, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		dispatcher,
		commands.RunStartCommand{
			Runtime: model.RuntimeConfig{
				WorkspaceRoot:          workspaceRoot,
				Name:                   "demo",
				TasksDir:               tasksDir,
				Mode:                   model.ExecutionModePRDTasks,
				IDE:                    model.IDECodex,
				Model:                  workspace.DefaultRecoveryModel,
				ReasoningEffort:        workspace.DefaultRecoveryReasoningEffort,
				AccessMode:             model.AccessModeDefault,
				BatchSize:              1,
				Concurrent:             1,
				MaxRetries:             0,
				RetryBackoffMultiplier: 1.5,
				Timeout:                time.Minute,
			},
			Recovery: workspace.AgentRecoveryConfig{
				Enabled:         boolPtr(true),
				IDE:             strPtr(model.IDECodex),
				Model:           strPtr(workspace.DefaultRecoveryModel),
				ReasoningEffort: strPtr(workspace.DefaultRecoveryReasoningEffort),
				MaxAttempts:     intPtr(1),
			},
		},
	)
	if err != nil {
		t.Fatalf("dispatch run start with real recovery: %v", err)
	}
	if result.Status != string(recovery.StatusSucceeded) {
		t.Fatalf("result status = %q, want succeeded", result.Status)
	}
	if result.RunID == "" {
		t.Fatal("expected run ID")
	}
	fakeACP.assertCallCounts(t, map[string]int{
		"task_01": 2,
		"task_02": 1,
	}, 1)
	if _, err := os.Stat(filepath.Join(workspaceRoot, "fixed.txt")); err != nil {
		t.Fatalf("expected recovery agent to create fixed.txt: %v", err)
	}
	if got := kernelGitCommitCount(t, workspaceRoot); got != "1" {
		t.Fatalf("git commit count = %s, want 1 (no recovery commit)", got)
	}
	artifacts, err := model.ResolveHomeRunArtifacts(result.RunID)
	if err != nil {
		t.Fatalf("resolve run artifacts: %v", err)
	}
	assertKernelRecoveryArtifacts(t, artifacts.RecoveryDir)
	eventsPayload, err := os.ReadFile(artifacts.EventsPath)
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	for _, kind := range []eventspkg.EventKind{
		eventspkg.EventKindRunRecoveryStarted,
		eventspkg.EventKindRunRecoveryRestarting,
		eventspkg.EventKindRunRecovered,
	} {
		if got := strings.Count(string(eventsPayload), string(kind)); got != 1 {
			t.Fatalf("event %s count = %d, want 1\n%s", kind, got, string(eventsPayload))
		}
	}
	outcome, err := recovery.ReadRunOutcome(artifacts)
	if err != nil {
		t.Fatalf("read final run outcome: %v", err)
	}
	if outcome == nil || outcome.Status != recovery.StatusSucceeded {
		t.Fatalf("final outcome = %#v, want succeeded", outcome)
	}
}

func TestRunStartRecoverySkipsCanceledAndRecoveryAttemptRuns(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("build failed")
	tests := []struct {
		name            string
		status          recovery.RunStatus
		execErr         error
		recoveryAttempt int
	}{
		{
			name:    "canceled",
			status:  recovery.StatusCanceled,
			execErr: context.Canceled,
		},
		{
			name:            "recovery attempt",
			status:          recovery.StatusFailed,
			execErr:         originalErr,
			recoveryAttempt: 1,
		},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			scope := newKernelRecoveryScope(t, "skip-"+strings.ReplaceAll(tc.name, " ", "-"))
			fake := &fakeOperations{
				openResult: scope,
				prepareResult: kernelRecoveryPreparation(scope.RunArtifacts(), []model.Job{{
					SafeName: "unit-tests",
				}}),
				executeHook: func(_ context.Context, prep *model.SolvePreparation, _ *model.RuntimeConfig, _ int) error {
					writeKernelRunResult(t, prep.RunArtifacts, tc.status, prep.Jobs, map[string]recovery.RunStatus{
						"unit-tests": tc.status,
					})
					return tc.execErr
				},
			}
			strategy := &kernelFakeRecoveryStrategy{
				verdicts: []recovery.TriageVerdict{{Decision: recovery.VerdictFixed}},
			}
			deps := testKernelDeps(fake)
			deps.RecoveryStrategy = strategy

			_, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
				context.Background(),
				BuildDefault(deps),
				commands.RunStartCommand{
					Runtime: model.RuntimeConfig{
						WorkspaceRoot:   "/workspace",
						Name:            "demo",
						Mode:            model.ExecutionModePRDTasks,
						IDE:             model.IDECodex,
						BatchSize:       1,
						RecoveryAttempt: tc.recoveryAttempt,
					},
					Recovery: enabledKernelRecoveryConfig(),
				},
			)
			if !errors.Is(err, tc.execErr) {
				t.Fatalf("dispatch error = %v, want %v", err, tc.execErr)
			}
			if len(strategy.inputs) != 0 {
				t.Fatalf("strategy calls = %d, want 0", len(strategy.inputs))
			}
		})
	}
}

func TestRunStartRecoveryRejectFailsLoudlyAndRecordsOneAttempt(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("missing credentials")
	scope := newKernelRecoveryScope(t, "reject-run")
	fake := &fakeOperations{
		openResult: scope,
		prepareResult: kernelRecoveryPreparation(scope.RunArtifacts(), []model.Job{{
			SafeName: "integration",
		}}),
		executeHook: func(_ context.Context, prep *model.SolvePreparation, _ *model.RuntimeConfig, _ int) error {
			writeKernelRunResult(t, prep.RunArtifacts, recovery.StatusFailed, prep.Jobs, map[string]recovery.RunStatus{
				"integration": recovery.StatusFailed,
			})
			return originalErr
		},
	}
	strategy := &kernelFakeRecoveryStrategy{
		verdicts: []recovery.TriageVerdict{{Decision: recovery.VerdictReject, Reason: "infra failure"}},
	}
	deps := testKernelDeps(fake)
	deps.RecoveryStrategy = strategy

	_, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		BuildDefault(deps),
		commands.RunStartCommand{
			Runtime: model.RuntimeConfig{
				WorkspaceRoot: "/workspace",
				Name:          "demo",
				Mode:          model.ExecutionModePRDTasks,
				IDE:           model.IDECodex,
				BatchSize:     1,
			},
			Recovery: enabledKernelRecoveryConfig(),
		},
	)
	if !errors.Is(err, originalErr) {
		t.Fatalf("dispatch error = %v, want original cause", err)
	}
	if len(strategy.inputs) != 1 {
		t.Fatalf("strategy calls = %d, want 1", len(strategy.inputs))
	}
	payload, readErr := os.ReadFile(scope.RunArtifacts().EventsPath)
	if readErr != nil {
		t.Fatalf("read recovery events: %v", readErr)
	}
	if got := strings.Count(string(payload), string(eventspkg.EventKindRunRecoveryStarted)); got != 1 {
		t.Fatalf("recovery started events = %d, want 1\n%s", got, string(payload))
	}
}

func TestRunStartExecRecoveryRerunsSingleJob(t *testing.T) {
	t.Parallel()

	originalErr := errors.New("exec failed")
	workspaceRoot := t.TempDir()
	scope := newKernelRecoveryScopeInRoot(t, workspaceRoot, "exec-recovery")
	fake := &fakeOperations{
		openResult: scope,
		execHook: func(_ context.Context, cfg *model.RuntimeConfig, scope model.RunScope, call int) error {
			switch call {
			case 1:
				writeKernelExecOutcome(t, cfg, scope.RunArtifacts(), recovery.StatusFailed, "exec failed")
				return originalErr
			case 2:
				writeKernelExecOutcome(t, cfg, scope.RunArtifacts(), recovery.StatusSucceeded, "")
				return nil
			default:
				t.Fatalf("unexpected exec call %d", call)
				return nil
			}
		},
	}
	strategy := &kernelFakeRecoveryStrategy{
		verdicts: []recovery.TriageVerdict{{Decision: recovery.VerdictFixed, Reason: "fixed exec"}},
	}
	deps := testKernelDeps(fake)
	deps.RecoveryStrategy = strategy

	result, err := Dispatch[commands.RunStartCommand, commands.RunStartResult](
		context.Background(),
		BuildDefault(deps),
		commands.RunStartCommand{
			Runtime: model.RuntimeConfig{
				WorkspaceRoot: workspaceRoot,
				RunID:         "exec-recovery",
				Mode:          model.ExecutionModeExec,
				IDE:           model.IDECodex,
				PromptText:    "fix this",
			},
			Recovery: enabledKernelRecoveryConfig(),
		},
	)
	if err != nil {
		t.Fatalf("dispatch exec run start: %v", err)
	}
	if result.Status != string(recovery.StatusSucceeded) {
		t.Fatalf("status = %q, want succeeded", result.Status)
	}
	if len(fake.execCalls) != 2 {
		t.Fatalf("exec calls = %d, want 2", len(fake.execCalls))
	}
	if len(strategy.inputs) != 1 || len(strategy.inputs[0].Outcome.FailedJobIDs()) != 1 {
		t.Fatalf("unexpected strategy inputs: %#v", strategy.inputs)
	}
}

type kernelBoundaryACP struct {
	t    *testing.T
	root string

	mu            sync.Mutex
	jobCalls      map[string]int
	recoveryCalls int
	sessionSeq    int
}

func newKernelBoundaryACP(t *testing.T, root string) *kernelBoundaryACP {
	t.Helper()
	return &kernelBoundaryACP{
		t:        t,
		root:     root,
		jobCalls: make(map[string]int),
	}
}

func (c *kernelBoundaryACP) CreateSession(_ context.Context, req agent.SessionRequest) (agent.Session, error) {
	promptText := string(req.Prompt)
	if strings.Contains(promptText, "Failure context:") {
		return c.createRecoverySession()
	}
	jobID := strings.TrimSpace(req.JobID)
	taskID := kernelBoundaryTaskID(jobID)
	switch taskID {
	case "task_01":
		return c.createTaskSession(taskID, "task_01.done", !kernelFileExists(filepath.Join(c.root, "fixed.txt")))
	case "task_02":
		return c.createTaskSession(taskID, "task_02.done", false)
	default:
		return nil, fmt.Errorf("unexpected kernel boundary ACP job %q", jobID)
	}
}

func kernelBoundaryTaskID(jobID string) string {
	for _, taskID := range []string{"task_01", "task_02"} {
		if strings.HasPrefix(jobID, taskID) {
			return taskID
		}
	}
	return jobID
}

func (c *kernelBoundaryACP) createRecoverySession() (agent.Session, error) {
	c.mu.Lock()
	c.recoveryCalls++
	sessionID := c.nextSessionIDLocked("recovery")
	c.mu.Unlock()
	if err := os.WriteFile(filepath.Join(c.root, "fixed.txt"), []byte("fixed\n"), 0o600); err != nil {
		return nil, fmt.Errorf("write recovery fixture: %w", err)
	}
	return newKernelBoundarySession(
		c.t,
		sessionID,
		`{"decision":"fixed","reason":"created fixed.txt","changed_files":["fixed.txt"]}`,
		nil,
	), nil
}

func (c *kernelBoundaryACP) createTaskSession(jobID string, marker string, fail bool) (agent.Session, error) {
	c.mu.Lock()
	c.jobCalls[jobID]++
	sessionID := c.nextSessionIDLocked(jobID)
	c.mu.Unlock()
	if fail {
		return newKernelBoundarySession(c.t, sessionID, "missing fixed.txt", errors.New("missing fixed.txt")), nil
	}
	if err := os.WriteFile(filepath.Join(c.root, marker), []byte("done\n"), 0o600); err != nil {
		return nil, fmt.Errorf("write task marker: %w", err)
	}
	return newKernelBoundarySession(c.t, sessionID, "task completed", nil), nil
}

func (c *kernelBoundaryACP) nextSessionIDLocked(prefix string) string {
	c.sessionSeq++
	return fmt.Sprintf("sess-%s-%d", prefix, c.sessionSeq)
}

func (c *kernelBoundaryACP) assertCallCounts(t *testing.T, wantJobs map[string]int, wantRecovery int) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	for jobID, want := range wantJobs {
		if got := c.jobCalls[jobID]; got != want {
			t.Fatalf("ACP calls for %s = %d, want %d; all calls=%#v", jobID, got, want, c.jobCalls)
		}
	}
	if got := c.recoveryCalls; got != wantRecovery {
		t.Fatalf("recovery ACP calls = %d, want %d", got, wantRecovery)
	}
}

func (*kernelBoundaryACP) ResumeSession(context.Context, agent.ResumeSessionRequest) (agent.Session, error) {
	return nil, errors.New("resume not supported in kernel boundary fake")
}

func (*kernelBoundaryACP) CancelSession(context.Context, string) error {
	return nil
}

func (*kernelBoundaryACP) PromptSession(context.Context, agent.PromptSessionRequest) (agent.Session, error) {
	return nil, errors.New("prompt not supported in kernel boundary fake")
}

func (*kernelBoundaryACP) SupportsLoadSession() bool {
	return false
}

func (*kernelBoundaryACP) Close() error {
	return nil
}

func (*kernelBoundaryACP) Kill() error {
	return nil
}

type kernelBoundarySession struct {
	id      string
	updates chan model.SessionUpdate
	done    chan struct{}
	err     error
}

func newKernelBoundarySession(
	t *testing.T,
	id string,
	text string,
	sessionErr error,
) *kernelBoundarySession {
	t.Helper()
	session := &kernelBoundarySession{
		id:      id,
		updates: make(chan model.SessionUpdate, 1),
		done:    make(chan struct{}),
		err:     sessionErr,
	}
	if strings.TrimSpace(text) != "" {
		block, err := model.NewContentBlock(model.TextBlock{Text: text})
		if err != nil {
			t.Fatalf("build text content block: %v", err)
		}
		session.updates <- model.SessionUpdate{
			Kind:   model.UpdateKindAgentMessageChunk,
			Status: model.StatusRunning,
			Blocks: []model.ContentBlock{block},
		}
	}
	close(session.updates)
	close(session.done)
	return session
}

func (s *kernelBoundarySession) ID() string {
	return s.id
}

func (s *kernelBoundarySession) Identity() agent.SessionIdentity {
	return agent.SessionIdentity{ACPSessionID: s.id}
}

func (s *kernelBoundarySession) Updates() <-chan model.SessionUpdate {
	return s.updates
}

func (s *kernelBoundarySession) Done() <-chan struct{} {
	return s.done
}

func (s *kernelBoundarySession) Err() error {
	return s.err
}

func (*kernelBoundarySession) SlowPublishes() uint64 {
	return 0
}

func (*kernelBoundarySession) DroppedUpdates() uint64 {
	return 0
}

type kernelFakeRecoveryStrategy struct {
	verdicts []recovery.TriageVerdict
	inputs   []recovery.RemediationInput
}

func (*kernelFakeRecoveryStrategy) Name() string {
	return "kernel-fake"
}

func (s *kernelFakeRecoveryStrategy) Remediate(
	_ context.Context,
	in recovery.RemediationInput,
) (recovery.TriageVerdict, error) {
	s.inputs = append(s.inputs, in)
	if len(s.verdicts) == 0 {
		return recovery.TriageVerdict{Decision: recovery.VerdictReject, Reason: "unexpected remediation"}, nil
	}
	verdict := s.verdicts[0]
	s.verdicts = s.verdicts[1:]
	return verdict, nil
}

func newKernelRecoveryScope(t *testing.T, runID string) *model.BaseRunScope {
	t.Helper()
	return newKernelRecoveryScopeInRoot(t, t.TempDir(), runID)
}

func newKernelRecoveryScopeInRoot(t *testing.T, root string, runID string) *model.BaseRunScope {
	t.Helper()
	artifacts := model.NewRunArtifacts(root, runID)
	if err := os.MkdirAll(artifacts.JobsDir, 0o755); err != nil {
		t.Fatalf("mkdir jobs dir: %v", err)
	}
	bus := eventspkg.New[eventspkg.Event](16)
	runJournal, err := journal.Open(artifacts.EventsPath, bus, 0)
	if err != nil {
		t.Fatalf("open run journal: %v", err)
	}
	scope := &model.BaseRunScope{Artifacts: artifacts, Journal: runJournal, EventBus: bus}
	t.Cleanup(func() {
		if err := scope.Close(context.Background()); err != nil {
			t.Fatalf("close run scope: %v", err)
		}
	})
	return scope
}

func kernelRecoveryPreparation(artifacts model.RunArtifacts, jobs []model.Job) *model.SolvePreparation {
	return &model.SolvePreparation{
		Jobs:         append([]model.Job(nil), jobs...),
		RunArtifacts: artifacts,
	}
}

func writeKernelRunResult(
	t *testing.T,
	artifacts model.RunArtifacts,
	status recovery.RunStatus,
	jobs []model.Job,
	statusBySafeName map[string]recovery.RunStatus,
) {
	t.Helper()
	jobOutcomes := make([]recovery.JobOutcome, 0, len(jobs))
	for i := range jobs {
		job := jobs[i]
		jobStatus := status
		if statusBySafeName != nil {
			if configured, ok := statusBySafeName[job.SafeName]; ok {
				jobStatus = configured
			}
		}
		exitCode := 0
		errText := ""
		if jobStatus == recovery.StatusFailed {
			exitCode = 1
			errText = "failed " + job.SafeName
		}
		jobOutcomes = append(jobOutcomes, recovery.JobOutcome{
			SafeName: job.SafeName,
			Status:   jobStatus,
			ExitCode: exitCode,
			Error:    errText,
		})
	}
	payload := struct {
		SchemaVersion int                   `json:"schema_version"`
		RunID         string                `json:"run_id"`
		Status        recovery.RunStatus    `json:"status"`
		ArtifactsDir  string                `json:"artifacts_dir"`
		ResultPath    string                `json:"result_path"`
		Jobs          []recovery.JobOutcome `json:"jobs"`
	}{
		SchemaVersion: recovery.ResultSchemaVersion,
		RunID:         artifacts.RunID,
		Status:        status,
		ArtifactsDir:  artifacts.RunDir,
		ResultPath:    artifacts.ResultPath,
		Jobs:          jobOutcomes,
	}
	writeKernelJSON(t, artifacts.ResultPath, payload)
}

func writeKernelExecOutcome(
	t *testing.T,
	cfg *model.RuntimeConfig,
	artifacts model.RunArtifacts,
	status recovery.RunStatus,
	errText string,
) {
	t.Helper()
	record := struct {
		Version         int       `json:"version"`
		Mode            string    `json:"mode"`
		RunID           string    `json:"run_id"`
		Status          string    `json:"status"`
		WorkspaceRoot   string    `json:"workspace_root"`
		IDE             string    `json:"ide"`
		Model           string    `json:"model"`
		ReasoningEffort string    `json:"reasoning_effort"`
		AccessMode      string    `json:"access_mode"`
		CreatedAt       time.Time `json:"created_at"`
		UpdatedAt       time.Time `json:"updated_at"`
		TurnCount       int       `json:"turn_count"`
		LastError       string    `json:"last_error,omitempty"`
		EventsPath      string    `json:"events_path,omitempty"`
		TurnsDir        string    `json:"turns_dir,omitempty"`
	}{
		Version:       1,
		Mode:          model.ModeExec,
		RunID:         artifacts.RunID,
		Status:        string(status),
		WorkspaceRoot: cfg.WorkspaceRoot,
		IDE:           cfg.IDE,
		Model:         cfg.Model,
		AccessMode:    cfg.AccessMode,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		TurnCount:     1,
		LastError:     errText,
		EventsPath:    artifacts.EventsPath,
		TurnsDir:      artifacts.TurnsDir,
	}
	writeKernelJSON(t, artifacts.RunMetaPath, record)
	turnPath := filepath.Join(artifacts.TurnsDir, "0001", "result.json")
	turn := struct {
		Turn          int                `json:"turn"`
		Status        recovery.RunStatus `json:"status"`
		ResultPath    string             `json:"result_path"`
		StdoutLogPath string             `json:"stdout_log_path,omitempty"`
		StderrLogPath string             `json:"stderr_log_path,omitempty"`
		Error         string             `json:"error,omitempty"`
	}{
		Turn:       1,
		Status:     status,
		ResultPath: turnPath,
		Error:      errText,
	}
	writeKernelJSON(t, turnPath, turn)
}

func writeKernelJSON(t *testing.T, path string, value any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func enabledKernelRecoveryConfig() workspace.AgentRecoveryConfig {
	return workspace.AgentRecoveryConfig{Enabled: boolPtr(true), MaxAttempts: intPtr(1)}
}

func writeKernelTaskFile(t *testing.T, tasksDir string, name string, title string, body string) {
	t.Helper()
	content := fmt.Sprintf(`---
status: pending
title: %s
type: backend
complexity: low
---

# %s

%s
`, title, title, body)
	if err := os.WriteFile(filepath.Join(tasksDir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write task file %s: %v", name, err)
	}
}

func installKernelRuntimeProbeStub(t *testing.T, command string) {
	t.Helper()
	binDir := t.TempDir()
	scriptPath := filepath.Join(binDir, command)
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write runtime probe stub: %v", err)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func initKernelGitRepo(t *testing.T, root string) {
	t.Helper()
	runKernelGit(t, root, "init")
	runKernelGit(t, root, "config", "user.email", "compozy@example.test")
	runKernelGit(t, root, "config", "user.name", "Compozy Test")
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# fixture\n"), 0o600); err != nil {
		t.Fatalf("write fixture readme: %v", err)
	}
	runKernelGit(t, root, "add", "README.md")
	runKernelGit(t, root, "commit", "-m", "initial")
}

func kernelGitCommitCount(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(runKernelGit(t, root, "rev-list", "--count", "HEAD"))
}

func runKernelGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, string(out))
	}
	return string(out)
}

func kernelFileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func assertKernelRecoveryArtifacts(t *testing.T, recoveryDir string) {
	t.Helper()
	for _, name := range []string{"baseline.json", "final.json", "changed_files.json", "metadata.json"} {
		if _, err := os.Stat(filepath.Join(recoveryDir, name)); err != nil {
			t.Fatalf("expected recovery artifact %s: %v", name, err)
		}
	}
}

func boolPtr(value bool) *bool {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func strPtr(value string) *string {
	return &value
}

func safeNames(jobs []model.Job) []string {
	names := make([]string, 0, len(jobs))
	for i := range jobs {
		names = append(names, jobs[i].SafeName)
	}
	return names
}
