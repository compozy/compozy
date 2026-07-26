package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/network/participation"
	schedulerpkg "github.com/compozy/agh/internal/scheduler"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	taskpkg "github.com/compozy/agh/internal/task"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestShellQuoteSimpleAlwaysSingleQuotes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "Should quote empty input", value: "", want: "''"},
		{name: "Should quote simple input", value: "frontend", want: "'frontend'"},
		{name: "Should quote metacharacters", value: "frontend; rm -rf /", want: "'frontend; rm -rf /'"},
		{name: "Should quote single-quote input", value: "owner's-tool", want: "'owner'\\''s-tool'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := shellQuoteSimple(tt.value); got != tt.want {
				t.Fatalf("shellQuoteSimple(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

func TestTaskRoleRuntimeActivatesPoolOwnerSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should start the pool owner agent session when a run is enqueued", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-frontend", "frontend-engineer-agent")
		run := taskRoleRuntimeRun("run-frontend", taskRecord.ID, "design-review")
		store := newTaskRoleRuntimeStore(taskRecord, run)
		sessions := &taskRoleRuntimeSessions{}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)

		runtime.OnTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
			TaskRunContext: hookspkg.TaskRunContext{TaskID: taskRecord.ID, RunID: run.ID},
		})
		runtime.wg.Wait()

		if got, want := sessions.createCount(), 1; got != want {
			t.Fatalf("create count = %d, want %d", got, want)
		}
		call := sessions.createCall(0)
		if got, want := call.AgentName, "frontend-engineer-agent"; got != want {
			t.Fatalf("CreateOpts.AgentName = %q, want %q", got, want)
		}
		if got, want := call.Workspace, taskRecord.WorkspaceID; got != want {
			t.Fatalf("CreateOpts.Workspace = %q, want %q", got, want)
		}
		if got, want := participationSnapshotValue(
			call.ResolvedNetworkParticipation,
		).ChannelID, "design-review"; got != want {
			t.Fatalf("CreateOpts resolved participation channel = %q, want %q", got, want)
		}
		if got, want := call.Type, session.SessionTypeSystem; got != want {
			t.Fatalf("CreateOpts.Type = %q, want %q", got, want)
		}
		if got := call.Provider; got != "" {
			t.Fatalf("CreateOpts.Provider = %q, want default provider resolution", got)
		}
		if got := call.Model; got != "" {
			t.Fatalf("CreateOpts.Model = %q, want provider-native default model resolution", got)
		}
		for _, required := range []string{"agh task next --run-id 'run-frontend' --wait -o json", run.ID, "design-review"} {
			if !strings.Contains(call.PromptOverlay, required) {
				t.Fatalf("PromptOverlay missing %q:\n%s", required, call.PromptOverlay)
			}
		}
		if got, want := sessions.promptCount(), 1; got != want {
			t.Fatalf("synthetic prompt count = %d, want %d", got, want)
		}
		prompt := sessions.promptCall(0)
		if got, want := prompt.sessionID, "role-1"; got != want {
			t.Fatalf("PromptSynthetic() session id = %q, want %q", got, want)
		}
		if got, want := prompt.opts.Metadata.TaskID, taskRecord.ID; got != want {
			t.Fatalf("PromptSynthetic() task id = %q, want %q", got, want)
		}
		if got, want := prompt.opts.Metadata.TaskRunID, run.ID; got != want {
			t.Fatalf("PromptSynthetic() run id = %q, want %q", got, want)
		}
		if got, want := prompt.opts.Metadata.Reason, taskRoleActivationReasonRunEnqueued; got != want {
			t.Fatalf("PromptSynthetic() reason = %q, want %q", got, want)
		}
		if got, want := prompt.opts.TurnID, taskRolePromptTurnID("role-1", run.ID); got != want {
			t.Fatalf("PromptSynthetic() turn id = %q, want %q", got, want)
		}
	})

	t.Run("Should return before session provisioning and cancel owned activation on shutdown", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-detached", "frontend-engineer-agent")
		run := taskRoleRuntimeRun("run-detached", taskRecord.ID, "design-review")
		store := newTaskRoleRuntimeStore(taskRecord, run)
		sessions := newBlockingTaskRoleRuntimeSessions()
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)
		var releaseOnce sync.Once
		t.Cleanup(func() {
			releaseOnce.Do(func() { close(sessions.createRelease) })
		})

		returned := make(chan struct{})
		go func() {
			runtime.OnTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
				TaskRunContext: hookspkg.TaskRunContext{TaskID: taskRecord.ID, RunID: run.ID},
			})
			close(returned)
		}()

		select {
		case <-sessions.createStarted:
		case <-time.After(time.Second):
			t.Fatal("session provisioning did not start")
		}
		select {
		case <-returned:
		case <-time.After(time.Second):
			t.Fatal("OnTaskRunEnqueued blocked on session provisioning")
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runtime.shutdown(shutdownCtx); err != nil {
			t.Fatalf("shutdown() error = %v", err)
		}
		select {
		case <-sessions.createCanceled:
		case <-time.After(time.Second):
			t.Fatal("shutdown did not cancel session provisioning")
		}
		if got := sessions.createCount(); got != 0 {
			t.Fatalf("create count = %d, want 0 after canceled provisioning", got)
		}
	})

	t.Run("Should drain task role activation before daemon session shutdown snapshots", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-daemon-shutdown", "frontend-engineer-agent")
		run := taskRoleRuntimeRun("run-daemon-shutdown", taskRecord.ID, "design-review")
		store := newTaskRoleRuntimeStore(taskRecord, run)
		sessions := newShutdownOrderingSessionManager()
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)
		runtime.OnTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
			TaskRunContext: hookspkg.TaskRunContext{TaskID: taskRecord.ID, RunID: run.ID},
		})

		select {
		case <-sessions.createStarted:
		case <-time.After(time.Second):
			t.Fatal("session provisioning did not start")
		}

		tasks := &taskRuntime{}
		tasks.roles.Store(runtime)
		d, err := New(WithLogger(discardLogger()))
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var shutdownErrs []error
		d.shutdownRuntimeWorkers(shutdownCtx, shutdownTargets{tasks: tasks, sessions: sessions}, &shutdownErrs)
		if err := errors.Join(shutdownErrs...); err != nil {
			t.Fatalf("shutdownRuntimeWorkers() error = %v", err)
		}

		select {
		case <-sessions.createCanceled:
		default:
			t.Fatal("task role activation was not canceled before session shutdown")
		}
		select {
		case <-sessions.listCalled:
		default:
			t.Fatal("session shutdown did not take its snapshot")
		}
		if got := sessions.createCount(); got != 0 {
			t.Fatalf("create count = %d, want no session admitted during shutdown", got)
		}
	})

	t.Run("Should include designated fan-out assignment in worker prompt", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-fanout", "frontend-engineer-agent")
		run := taskRoleRuntimeRun("run-fanout", taskRecord.ID, "design-review")
		run.DesignationGroupID = "tdg-fanout"
		run.Metadata = json.RawMessage(`{"designation":{"index":2,"brief":"Audit billing latency"}}`)
		store := newTaskRoleRuntimeStore(taskRecord, run)
		sessions := &taskRoleRuntimeSessions{}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)

		runtime.OnTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
			TaskRunContext: hookspkg.TaskRunContext{TaskID: taskRecord.ID, RunID: run.ID},
		})
		runtime.wg.Wait()

		if got, want := sessions.createCount(), 1; got != want {
			t.Fatalf("create count = %d, want %d", got, want)
		}
		overlay := sessions.createCall(0).PromptOverlay
		for _, required := range []string{
			"Designated fan-out assignment",
			"group tdg-fanout",
			"index 2",
			`brief "Audit billing latency"`,
		} {
			if !strings.Contains(overlay, required) {
				t.Fatalf("PromptOverlay missing %q:\n%s", required, overlay)
			}
		}
		if strings.Contains(overlay, ".\n\nUse `") {
			t.Fatalf("PromptOverlay includes a blank line after designation:\n%s", overlay)
		}
	})

	t.Run("Should bind distinct role sessions to distinct owning runs", func(t *testing.T) {
		t.Parallel()

		firstTask := taskRoleRuntimeTask("task-frontend-a", "frontend-engineer-agent")
		firstRun := taskRoleRuntimeRun("run-frontend-a", firstTask.ID, "design-review")
		secondTask := taskRoleRuntimeTask("task-frontend-b", "frontend-engineer-agent")
		secondRun := taskRoleRuntimeRun("run-frontend-b", secondTask.ID, "design-review")
		store := newTaskRoleRuntimeStore(firstTask, secondTask, firstRun, secondRun)
		sessions := &taskRoleRuntimeSessions{}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)

		runtime.OnTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
			TaskRunContext: hookspkg.TaskRunContext{TaskID: firstTask.ID, RunID: firstRun.ID},
		})
		runtime.OnTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
			TaskRunContext: hookspkg.TaskRunContext{TaskID: secondTask.ID, RunID: secondRun.ID},
		})
		runtime.wg.Wait()

		if got, want := sessions.createCount(), 2; got != want {
			t.Fatalf("create count = %d, want %d", got, want)
		}
		if got, want := sessions.promptCount(), 2; got != want {
			t.Fatalf("synthetic prompt count = %d, want one per distinct run", got)
		}
		if first, second := sessions.promptCall(0), sessions.promptCall(1); first.opts.TurnID == second.opts.TurnID {
			t.Fatalf("distinct runs reused activation turn id %q", first.opts.TurnID)
		}
		if first, second := sessions.createCall(0).Name, sessions.createCall(1).Name; first == second {
			t.Fatalf("role session names = %q, want run-bound identities", first)
		}
	})

	t.Run("Should keep local role identity and prompt free of fictional channels", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-local", "frontend-engineer-agent")
		run := taskRoleRuntimeRun("run-local", taskRecord.ID, "design-review")
		run.SetNetworkState(participation.LocalSpec(), "", "", "")
		store := newTaskRoleRuntimeStore(taskRecord, run)
		sessions := &taskRoleRuntimeSessions{}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)

		runtime.OnTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
			TaskRunContext: hookspkg.TaskRunContext{TaskID: taskRecord.ID, RunID: run.ID},
		})
		runtime.wg.Wait()

		call := sessions.createCall(0)
		if !strings.Contains(call.Name, "run-local") {
			t.Fatalf("CreateOpts.Name = %q, want owning run identity", call.Name)
		}
		for _, forbidden := range []string{"Coordination channel", "default", "design-review"} {
			if strings.Contains(call.PromptOverlay, forbidden) || strings.Contains(call.Name, forbidden) {
				t.Fatalf(
					"local role identity contains fictional channel %q: name=%q prompt=%q",
					forbidden,
					call.Name,
					call.PromptOverlay,
				)
			}
		}
	})

	t.Run("Should not reuse an active role session with different profile startup settings", func(t *testing.T) {
		t.Parallel()

		firstTask := taskRoleRuntimeTask("task-profile-a", "")
		firstTask.Owner = nil
		firstRun := taskRoleRuntimeRun("run-profile-a", firstTask.ID, "design-review")
		firstProfile := taskpkg.ExecutionProfile{
			TaskID: firstTask.ID,
			Worker: taskpkg.WorkerProfile{
				Mode:      taskpkg.WorkerModeSelect,
				AgentName: "frontend-engineer",
				Provider:  "claude",
				Model:     "sonnet",
			},
			Sandbox: taskpkg.SandboxPolicy{Mode: taskpkg.SandboxModeRef, SandboxRef: "evidence-lab-a"},
			Runtime: taskpkg.RuntimePolicy{Mode: taskpkg.RuntimeModeEvidence},
		}
		secondTask := taskRoleRuntimeTask("task-profile-b", "")
		secondTask.Owner = nil
		secondRun := taskRoleRuntimeRun("run-profile-b", secondTask.ID, "design-review")
		secondProfile := taskpkg.ExecutionProfile{
			TaskID: secondTask.ID,
			Worker: taskpkg.WorkerProfile{
				Mode:      taskpkg.WorkerModeSelect,
				AgentName: "frontend-engineer",
				Provider:  "claude",
				Model:     "sonnet",
			},
			Sandbox: taskpkg.SandboxPolicy{Mode: taskpkg.SandboxModeRef, SandboxRef: "evidence-lab-b"},
			Runtime: taskpkg.RuntimePolicy{Mode: taskpkg.RuntimeModeEvidence},
		}
		store := newTaskRoleRuntimeStore(firstTask, firstRun, firstProfile, secondTask, secondRun, secondProfile)
		sessions := &taskRoleRuntimeSessions{}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)

		runtime.OnTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
			TaskRunContext: hookspkg.TaskRunContext{TaskID: firstTask.ID, RunID: firstRun.ID},
		})
		runtime.OnTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
			TaskRunContext: hookspkg.TaskRunContext{TaskID: secondTask.ID, RunID: secondRun.ID},
		})
		runtime.wg.Wait()

		if got, want := sessions.createCount(), 2; got != want {
			t.Fatalf("create count = %d, want %d for different profile startup settings", got, want)
		}
		firstName := sessions.createCall(0).Name
		secondName := sessions.createCall(1).Name
		if firstName == secondName {
			t.Fatalf("CreateOpts.Name = %q for both profile workers, want distinct profile fingerprints", firstName)
		}
	})

	t.Run("Should start the selected execution-profile worker when no owner is set", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-profile-worker", "")
		taskRecord.Owner = nil
		run := taskRoleRuntimeRun("run-profile-worker", taskRecord.ID, "design-review")
		profile := taskpkg.ExecutionProfile{
			TaskID: taskRecord.ID,
			Worker: taskpkg.WorkerProfile{
				Mode:                 taskpkg.WorkerModeSelect,
				AgentName:            "frontend-engineer",
				Provider:             "cursor",
				Model:                "grok-4.5[effort=high,fast=true]",
				RequiredCapabilities: []string{"frontend"},
			},
			Sandbox: taskpkg.SandboxPolicy{
				Mode:       taskpkg.SandboxModeRef,
				SandboxRef: "evidence-lab",
			},
			Runtime: taskpkg.RuntimePolicy{Mode: taskpkg.RuntimeModeEvidence},
		}
		store := newTaskRoleRuntimeStore(taskRecord, run, profile)
		sessions := &taskRoleRuntimeSessions{}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)

		runtime.OnTaskRunEnqueued(context.Background(), hookspkg.TaskRunEnqueuedPayload{
			TaskRunContext: hookspkg.TaskRunContext{TaskID: taskRecord.ID, RunID: run.ID},
		})
		runtime.wg.Wait()

		if got, want := sessions.createCount(), 1; got != want {
			t.Fatalf("create count = %d, want %d", got, want)
		}
		call := sessions.createCall(0)
		if got, want := call.AgentName, "frontend-engineer"; got != want {
			t.Fatalf("CreateOpts.AgentName = %q, want %q", got, want)
		}
		if got, want := call.Provider, "cursor"; got != want {
			t.Fatalf("CreateOpts.Provider = %q, want %q", got, want)
		}
		if got, want := call.Model, "grok-4.5[effort=high,fast=true]"; got != want {
			t.Fatalf("CreateOpts.Model = %q, want %q", got, want)
		}
		if got, want := call.Permissions, aghconfig.PermissionModeApproveAll; got != want {
			t.Fatalf("CreateOpts.Permissions = %q, want %q", got, want)
		}
		if got, want := call.SandboxRef, "evidence-lab"; got != want {
			t.Fatalf("CreateOpts.SandboxRef = %q, want %q", got, want)
		}
		if !strings.Contains(call.PromptOverlay, "Runtime evidence mode is enabled") {
			t.Fatalf("PromptOverlay missing runtime evidence guidance:\n%s", call.PromptOverlay)
		}
		claimCommand := "agh task next --run-id 'run-profile-worker' --wait -o json --capability 'frontend'"
		if !strings.Contains(call.PromptOverlay, claimCommand) {
			t.Fatalf("PromptOverlay missing required capability claim:\n%s", call.PromptOverlay)
		}
	})

	t.Run("Should recover queued pool-owned runs on boot", func(t *testing.T) {
		t.Parallel()

		frontendTask := taskRoleRuntimeTask("task-frontend", "frontend-engineer-agent")
		frontendRun := taskRoleRuntimeRun("run-frontend", frontendTask.ID, "design-review")
		analyticsTask := taskRoleRuntimeTask("task-analytics", "analytics-engineer-agent")
		analyticsRun := taskRoleRuntimeRun("run-analytics", analyticsTask.ID, "data-watch")
		humanTask := taskRoleRuntimeTask("task-human", "human-owner")
		humanTask.Owner = &taskpkg.Ownership{Kind: taskpkg.OwnerKindHuman, Ref: "local-user"}
		humanRun := taskRoleRuntimeRun("run-human", humanTask.ID, "ops")
		store := newTaskRoleRuntimeStore(frontendTask, frontendRun, analyticsTask, analyticsRun, humanTask, humanRun)
		sessions := &taskRoleRuntimeSessions{}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)

		runtime.Recover(context.Background())

		if got, want := sessions.createCount(), 2; got != want {
			t.Fatalf("create count = %d, want %d", got, want)
		}
		gotAgents := []string{sessions.createCall(0).AgentName, sessions.createCall(1).AgentName}
		if !slices.Contains(gotAgents, "frontend-engineer-agent") ||
			!slices.Contains(gotAgents, "analytics-engineer-agent") {
			t.Fatalf("created agents = %#v, want frontend and analytics role sessions", gotAgents)
		}
	})
}

var taskRoleRuntimeClock = time.Date(2026, 5, 6, 12, 5, 0, 0, time.UTC)

func TestTaskRoleRuntimeActivateForStarvation(t *testing.T) {
	t.Parallel()

	t.Run("Should spawn the pool owner with a TTL-bounded spawn budget lineage", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-starved", "frontend-engineer-agent")
		run := taskRoleRuntimeRun("run-starved", taskRecord.ID, "design-review")
		store := newTaskRoleRuntimeStore(taskRecord, run)
		sessions := &taskRoleRuntimeSessions{}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)

		if err := runtime.activateForStarvation(
			context.Background(),
			taskRecord,
			run,
			starvationSpawner{},
		); err != nil {
			t.Fatalf("activateForStarvation() error = %v", err)
		}
		if got, want := sessions.createCount(), 1; got != want {
			t.Fatalf("create count = %d, want %d", got, want)
		}
		call := sessions.createCall(0)
		if got, want := call.AgentName, "frontend-engineer-agent"; got != want {
			t.Fatalf("CreateOpts.AgentName = %q, want %q", got, want)
		}
		if got, want := call.Workspace, taskRecord.WorkspaceID; got != want {
			t.Fatalf("CreateOpts.Workspace = %q, want run workspace %q", got, want)
		}
		if got, want := call.Type, session.SessionTypeSystem; got != want {
			t.Fatalf("CreateOpts.Type = %q, want %q", got, want)
		}
		if call.Lineage == nil {
			t.Fatal("CreateOpts.Lineage = nil, want TTL + spawn budget")
		}
		if call.Lineage.ParentSessionID != "" {
			t.Fatalf(
				"CreateOpts.Lineage.ParentSessionID = %q, want empty (parent-less worker)",
				call.Lineage.ParentSessionID,
			)
		}
		if call.Lineage.TTLExpiresAt == nil || !call.Lineage.TTLExpiresAt.After(taskRoleRuntimeClock) {
			t.Fatalf("CreateOpts.Lineage.TTLExpiresAt = %v, want a future deadline", call.Lineage.TTLExpiresAt)
		}
		if call.Lineage.SpawnBudget.TTLSeconds <= 0 || call.Lineage.SpawnBudget.MaxChildren <= 0 {
			t.Fatalf("CreateOpts.Lineage.SpawnBudget = %#v, want positive ttl + children", call.Lineage.SpawnBudget)
		}
		if got, want := sessions.promptCount(), 1; got != want {
			t.Fatalf("synthetic prompt count = %d, want %d", got, want)
		}
		prompt := sessions.promptCall(0)
		if got, want := prompt.opts.Metadata.TaskID, taskRecord.ID; got != want {
			t.Fatalf("PromptSynthetic() task id = %q, want %q", got, want)
		}
		if got, want := prompt.opts.Metadata.TaskRunID, run.ID; got != want {
			t.Fatalf("PromptSynthetic() run id = %q, want %q", got, want)
		}
		if got, want := prompt.opts.Metadata.Reason, taskRoleActivationReasonStarvation; got != want {
			t.Fatalf("PromptSynthetic() reason = %q, want %q", got, want)
		}
		if !strings.Contains(prompt.opts.Message, run.ID) {
			t.Fatalf("PromptSynthetic() message missing run id %q: %s", run.ID, prompt.opts.Message)
		}
	})

	t.Run("Should stop a fresh session when initial prompt dispatch fails and allow retry", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-prompt-retry", "frontend-engineer-agent")
		run := taskRoleRuntimeRun("run-prompt-retry", taskRecord.ID, "design-review")
		store := newTaskRoleRuntimeStore(taskRecord, run)
		dispatchErr := errors.New("synthetic prompt unavailable")
		sessions := &taskRoleRuntimeSessions{promptErrors: []error{dispatchErr, nil}}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)

		err := runtime.activateForStarvation(context.Background(), taskRecord, run, starvationSpawner{})
		if !errors.Is(err, dispatchErr) {
			t.Fatalf("first activateForStarvation() error = %v, want %v", err, dispatchErr)
		}
		if got, want := sessions.stopCount(), 1; got != want {
			t.Fatalf("stop count after failed prompt = %d, want %d", got, want)
		}
		stop := sessions.stopCall(0)
		if got, want := stop.sessionID, "role-1"; got != want {
			t.Fatalf("StopWithCause() session id = %q, want %q", got, want)
		}
		if got, want := stop.cause, session.CauseFailed; got != want {
			t.Fatalf("StopWithCause() cause = %v, want %v", got, want)
		}
		if !strings.Contains(stop.detail, run.ID) {
			t.Fatalf("StopWithCause() detail missing run id %q: %s", run.ID, stop.detail)
		}

		if err := runtime.activateForStarvation(
			context.Background(),
			taskRecord,
			run,
			starvationSpawner{},
		); err != nil {
			t.Fatalf("second activateForStarvation() error = %v", err)
		}
		if got, want := sessions.createCount(), 2; got != want {
			t.Fatalf("create count after retry = %d, want %d", got, want)
		}
		if got, want := sessions.promptCount(), 2; got != want {
			t.Fatalf("synthetic prompt count after retry = %d, want %d", got, want)
		}
	})

	t.Run("Should preserve prompt and cleanup failures when stopping the fresh session fails", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-prompt-cleanup", "frontend-engineer-agent")
		run := taskRoleRuntimeRun("run-prompt-cleanup", taskRecord.ID, "design-review")
		store := newTaskRoleRuntimeStore(taskRecord, run)
		dispatchErr := errors.New("synthetic prompt unavailable")
		cleanupErr := errors.New("session stop unavailable")
		sessions := &taskRoleRuntimeSessions{
			promptErrors: []error{dispatchErr},
			stopError:    cleanupErr,
		}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)

		err := runtime.activateForStarvation(context.Background(), taskRecord, run, starvationSpawner{})
		if !errors.Is(err, dispatchErr) {
			t.Fatalf("activateForStarvation() error = %v, want prompt error %v", err, dispatchErr)
		}
		if !errors.Is(err, cleanupErr) {
			t.Fatalf("activateForStarvation() error = %v, want cleanup error %v", err, cleanupErr)
		}
		if got, want := sessions.stopCount(), 1; got != want {
			t.Fatalf("stop count after cleanup failure = %d, want %d", got, want)
		}
	})

	t.Run(
		"Should prefer a capability-matched agent over the pool owner when the run requires capabilities",
		func(t *testing.T) {
			t.Parallel()

			taskRecord := taskRoleRuntimeTask("task-cap-owner", "frontend-engineer-agent")
			run := taskRoleRuntimeRun("run-cap-owner", taskRecord.ID, "design-review")
			run.RequiredCapabilities = []string{"sqlite"}
			store := newTaskRoleRuntimeStore(taskRecord, run)
			sessions := &taskRoleRuntimeSessions{}
			runtime := newTaskRoleRuntimeForTest(t, store, sessions)
			spawner := starvationSpawner{
				workspaces: &fakeSpawnWorkspaceResolver{
					resolved: workspacepkg.ResolvedWorkspace{Agents: []aghconfig.AgentDef{
						spawnAgentDef("frontend-engineer-agent", "typescript"),
						spawnAgentDef("storage-agent", "sqlite"),
					}},
				},
				agents: reviewRouterAgentResolverStub{
					"frontend-engineer-agent": spawnAgentDef("frontend-engineer-agent", "typescript"),
					"storage-agent":           spawnAgentDef("storage-agent", "sqlite"),
				},
			}

			if err := runtime.activateForStarvation(context.Background(), taskRecord, run, spawner); err != nil {
				t.Fatalf("activateForStarvation() error = %v", err)
			}
			if got, want := sessions.createCount(), 1; got != want {
				t.Fatalf("create count = %d, want %d", got, want)
			}
			if got, want := sessions.createCall(0).AgentName, "storage-agent"; got != want {
				t.Fatalf("CreateOpts.AgentName = %q, want capability-matched %q", got, want)
			}
		},
	)

	t.Run(
		"Should spawn an eligible workspace agent when the run has no required capabilities or owner",
		func(t *testing.T) {
			t.Parallel()

			taskRecord := taskRoleRuntimeTask("task-no-cap-owner", "")
			taskRecord.Owner = nil
			run := taskRoleRuntimeRun("run-no-cap-owner", taskRecord.ID, "design-review")
			store := newTaskRoleRuntimeStore(taskRecord, run)
			sessions := &taskRoleRuntimeSessions{}
			runtime := newTaskRoleRuntimeForTest(t, store, sessions)
			spawner := starvationSpawner{
				workspaces: &fakeSpawnWorkspaceResolver{
					resolved: workspacepkg.ResolvedWorkspace{Agents: []aghconfig.AgentDef{
						spawnAgentDef("zeta-agent"),
						spawnAgentDef("alpha-agent"),
					}},
				},
				agents: reviewRouterAgentResolverStub{
					"zeta-agent":  spawnAgentDef("zeta-agent"),
					"alpha-agent": spawnAgentDef("alpha-agent"),
				},
			}

			if err := runtime.activateForStarvation(context.Background(), taskRecord, run, spawner); err != nil {
				t.Fatalf("activateForStarvation() error = %v", err)
			}
			if got, want := sessions.createCount(), 1; got != want {
				t.Fatalf("create count = %d, want %d", got, want)
			}
			if got, want := sessions.createCall(0).AgentName, "alpha-agent"; got != want {
				t.Fatalf("CreateOpts.AgentName = %q, want eligible workspace agent %q", got, want)
			}
		},
	)

	t.Run("Should skip spawning when no agent covers the required capabilities", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-cap", "")
		taskRecord.Owner = nil
		run := taskRoleRuntimeRun("run-cap", taskRecord.ID, "design-review")
		run.RequiredCapabilities = []string{"go"}
		store := newTaskRoleRuntimeStore(taskRecord, run)
		sessions := &taskRoleRuntimeSessions{}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)
		spawner := starvationSpawner{
			workspaces: &fakeSpawnWorkspaceResolver{
				resolved: workspacepkg.ResolvedWorkspace{
					Agents: []aghconfig.AgentDef{spawnAgentDef("docs-agent", "docs")},
				},
			},
			agents: reviewRouterAgentResolverStub{"docs-agent": spawnAgentDef("docs-agent", "docs")},
		}

		err := runtime.activateForStarvation(context.Background(), taskRecord, run, spawner)
		if !errors.Is(err, errStarvationSpawnUnresolvable) {
			t.Fatalf("activateForStarvation() error = %v, want errStarvationSpawnUnresolvable", err)
		}
		if got := sessions.createCount(); got != 0 {
			t.Fatalf("create count = %d, want 0 (no capable agent)", got)
		}
	})

	t.Run("Should dispatch once after the first activation prompt completes", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-dup", "frontend-engineer-agent")
		run := taskRoleRuntimeRun("run-dup", taskRecord.ID, "design-review")
		store := newTaskRoleRuntimeStore(taskRecord, run)
		sessions := &taskRoleRuntimeSessions{}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)

		if err := runtime.activateForStarvation(
			context.Background(),
			taskRecord,
			run,
			starvationSpawner{},
		); err != nil {
			t.Fatalf("first activateForStarvation() error = %v", err)
		}
		runtime.wg.Wait()
		if err := runtime.activateForStarvation(
			context.Background(),
			taskRecord,
			run,
			starvationSpawner{},
		); err != nil {
			t.Fatalf("second activateForStarvation() error = %v", err)
		}
		if got, want := sessions.createCount(), 1; got != want {
			t.Fatalf("create count = %d, want %d (dedup is the per-role cap)", got, want)
		}
		if got, want := sessions.promptCount(), 1; got != want {
			t.Fatalf("synthetic prompt count = %d, want %d after the first turn completed", got, want)
		}
	})

	t.Run("Should preserve a completed activation across runtime reconstruction", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-reconstructed", "frontend-engineer-agent")
		run := taskRoleRuntimeRun("run-reconstructed", taskRecord.ID, "design-review")
		store := newTaskRoleRuntimeStore(taskRecord, run)
		sessions := &taskRoleRuntimeSessions{}
		firstRuntime := newTaskRoleRuntimeForTest(t, store, sessions)

		if err := firstRuntime.activateForStarvation(
			context.Background(),
			taskRecord,
			run,
			starvationSpawner{},
		); err != nil {
			t.Fatalf("first activateForStarvation() error = %v", err)
		}
		firstRuntime.wg.Wait()

		reconstructed := newTaskRoleRuntimeForTest(t, store, sessions)
		if err := reconstructed.activateForStarvation(
			context.Background(),
			taskRecord,
			run,
			starvationSpawner{},
		); err != nil {
			t.Fatalf("reconstructed activateForStarvation() error = %v", err)
		}
		if got, want := sessions.promptCount(), 1; got != want {
			t.Fatalf("synthetic prompt count = %d, want %d after runtime reconstruction", got, want)
		}
	})

	t.Run("Should dispatch once for a replacement session", func(t *testing.T) {
		t.Parallel()

		taskRecord := taskRoleRuntimeTask("task-replacement", "frontend-engineer-agent")
		run := taskRoleRuntimeRun("run-replacement", taskRecord.ID, "design-review")
		store := newTaskRoleRuntimeStore(taskRecord, run)
		sessions := &taskRoleRuntimeSessions{}
		runtime := newTaskRoleRuntimeForTest(t, store, sessions)

		if err := runtime.activateForStarvation(
			context.Background(),
			taskRecord,
			run,
			starvationSpawner{},
		); err != nil {
			t.Fatalf("first activateForStarvation() error = %v", err)
		}
		runtime.wg.Wait()
		if err := sessions.StopWithCause(
			context.Background(),
			"role-1",
			session.CauseUserRequested,
			"explicit replacement",
		); err != nil {
			t.Fatalf("StopWithCause() error = %v", err)
		}

		if err := runtime.activateForStarvation(
			context.Background(),
			taskRecord,
			run,
			starvationSpawner{},
		); err != nil {
			t.Fatalf("replacement activateForStarvation() error = %v", err)
		}
		runtime.wg.Wait()
		if err := runtime.activateForStarvation(
			context.Background(),
			taskRecord,
			run,
			starvationSpawner{},
		); err != nil {
			t.Fatalf("repeated replacement activateForStarvation() error = %v", err)
		}
		if got, want := sessions.createCount(), 2; got != want {
			t.Fatalf("create count = %d, want %d after session replacement", got, want)
		}
		if got, want := sessions.promptCount(), 2; got != want {
			t.Fatalf("synthetic prompt count = %d, want one per session assignment", got)
		}
	})
}

func TestEscalationActorAdapterRequestWorkerSpawn(t *testing.T) {
	t.Parallel()

	t.Run("Should retry later instead of coalescing when task roles are not booted", func(t *testing.T) {
		t.Parallel()

		adapter := escalationActorAdapter{tasks: &taskRuntime{}}
		err := adapter.RequestWorkerSpawn(context.Background(), &schedulerpkg.RunSnapshot{})
		if !errors.Is(err, schedulerpkg.ErrSpawnUnresolvable) {
			t.Fatalf("RequestWorkerSpawn() error = %v, want %v", err, schedulerpkg.ErrSpawnUnresolvable)
		}
	})
}

func newTaskRoleRuntimeForTest(
	t *testing.T,
	store *taskRoleRuntimeStore,
	sessions taskRoleSessionManager,
) *taskRoleRuntime {
	t.Helper()

	runtime, err := newTaskRoleRuntime(
		store,
		sessions,
		t.TempDir(),
		discardLogger(),
		func() time.Time { return taskRoleRuntimeClock },
	)
	if err != nil {
		t.Fatalf("newTaskRoleRuntime() error = %v", err)
	}
	t.Cleanup(func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := runtime.shutdown(shutdownCtx); err != nil {
			t.Errorf("task role runtime shutdown error = %v", err)
		}
	})
	return runtime
}

func taskRoleRuntimeTask(id string, ownerRef string) taskpkg.Task {
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	return taskpkg.Task{
		ID:          id,
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: "ws-growth",
		Title:       "Task " + id,
		Status:      taskpkg.TaskStatusReady,
		Priority:    taskpkg.PriorityMedium,
		Owner:       &taskpkg.Ownership{Kind: taskpkg.OwnerKindPool, Ref: ownerRef},
		CreatedBy:   taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"},
		Origin:      taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "task-role-test"},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func taskRoleRuntimeRun(id string, taskID string, channel string) taskpkg.Run {
	run := taskpkg.Run{
		ID:       id,
		TaskID:   taskID,
		Status:   taskpkg.TaskRunStatusQueued,
		RunKind:  taskpkg.RunKindWorker,
		Attempt:  1,
		Origin:   taskpkg.Origin{Kind: taskpkg.OriginKindCLI, Ref: "task-role-test"},
		QueuedAt: time.Date(2026, 5, 6, 12, 1, 0, 0, time.UTC),
	}
	run.SetNetworkState(daemonTestLiveParticipation("ws-growth", channel), "", "", "")
	return run
}

type taskRoleRuntimeStore struct {
	mu       sync.Mutex
	tasks    map[string]taskpkg.Task
	runs     map[string]taskpkg.Run
	profiles map[string]taskpkg.ExecutionProfile
}

func newTaskRoleRuntimeStore(records ...any) *taskRoleRuntimeStore {
	store := &taskRoleRuntimeStore{
		tasks:    make(map[string]taskpkg.Task),
		runs:     make(map[string]taskpkg.Run),
		profiles: make(map[string]taskpkg.ExecutionProfile),
	}
	for _, record := range records {
		switch value := record.(type) {
		case taskpkg.Task:
			store.tasks[value.ID] = value
		case taskpkg.Run:
			store.runs[value.ID] = value
		case taskpkg.ExecutionProfile:
			store.profiles[value.TaskID] = value
		}
	}
	return store
}

func (s *taskRoleRuntimeStore) GetTask(_ context.Context, id string) (taskpkg.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	taskRecord, ok := s.tasks[strings.TrimSpace(id)]
	if !ok {
		return taskpkg.Task{}, taskpkg.ErrTaskNotFound
	}
	return taskRecord, nil
}

func (s *taskRoleRuntimeStore) GetTaskRun(_ context.Context, id string) (taskpkg.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	run, ok := s.runs[strings.TrimSpace(id)]
	if !ok {
		return taskpkg.Run{}, taskpkg.ErrTaskRunNotFound
	}
	return run, nil
}

func (s *taskRoleRuntimeStore) GetExecutionProfile(
	_ context.Context,
	taskID string,
) (taskpkg.ExecutionProfile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	profile, ok := s.profiles[strings.TrimSpace(taskID)]
	if !ok {
		return taskpkg.ExecutionProfile{}, taskpkg.ErrExecutionProfileNotFound
	}
	return profile, nil
}

func (s *taskRoleRuntimeStore) ListTaskRunsByStatus(
	_ context.Context,
	statuses []taskpkg.RunStatus,
) ([]taskpkg.Run, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	allowed := make(map[taskpkg.RunStatus]struct{}, len(statuses))
	for _, status := range statuses {
		allowed[status.Normalize()] = struct{}{}
	}
	runs := make([]taskpkg.Run, 0, len(s.runs))
	for _, run := range s.runs {
		if _, ok := allowed[run.Status.Normalize()]; ok {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

type taskRoleRuntimeSessions struct {
	mu           sync.Mutex
	infos        []*session.Info
	createCalls  []session.CreateOpts
	promptCalls  []taskRoleRuntimePromptCall
	promptErrors []error
	stopCalls    []taskRoleRuntimeStopCall
	stopError    error
	events       map[string][]store.SessionEvent
}

type taskRoleRuntimePromptCall struct {
	sessionID string
	opts      session.SyntheticPromptOpts
}

type taskRoleRuntimeStopCall struct {
	sessionID string
	cause     session.StopCause
	detail    string
}

type blockingTaskRoleRuntimeSessions struct {
	*taskRoleRuntimeSessions
	createStarted  chan struct{}
	createRelease  chan struct{}
	createCanceled chan struct{}
	startOnce      sync.Once
	cancelOnce     sync.Once
}

type shutdownOrderingSessionManager struct {
	*fakeSessionManager
	createStarted  chan struct{}
	createCanceled chan struct{}
	listCalled     chan struct{}
	startOnce      sync.Once
	cancelOnce     sync.Once
	listOnce       sync.Once
}

func newShutdownOrderingSessionManager() *shutdownOrderingSessionManager {
	return &shutdownOrderingSessionManager{
		fakeSessionManager: &fakeSessionManager{},
		createStarted:      make(chan struct{}),
		createCanceled:     make(chan struct{}),
		listCalled:         make(chan struct{}),
	}
}

func (s *shutdownOrderingSessionManager) Create(
	ctx context.Context,
	_ session.CreateOpts,
) (*session.Session, error) {
	s.startOnce.Do(func() { close(s.createStarted) })
	<-ctx.Done()
	s.cancelOnce.Do(func() { close(s.createCanceled) })
	return nil, fmt.Errorf("shutdown ordering create: %w", ctx.Err())
}

func (s *shutdownOrderingSessionManager) List() []*session.Info {
	s.listOnce.Do(func() { close(s.listCalled) })
	return s.fakeSessionManager.List()
}

func newBlockingTaskRoleRuntimeSessions() *blockingTaskRoleRuntimeSessions {
	return &blockingTaskRoleRuntimeSessions{
		taskRoleRuntimeSessions: &taskRoleRuntimeSessions{},
		createStarted:           make(chan struct{}),
		createRelease:           make(chan struct{}),
		createCanceled:          make(chan struct{}),
	}
}

func (s *blockingTaskRoleRuntimeSessions) Create(
	ctx context.Context,
	opts session.CreateOpts,
) (*session.Session, error) {
	s.startOnce.Do(func() { close(s.createStarted) })
	select {
	case <-s.createRelease:
		return s.taskRoleRuntimeSessions.Create(ctx, opts)
	case <-ctx.Done():
		s.cancelOnce.Do(func() { close(s.createCanceled) })
		return nil, fmt.Errorf("blocked task role session create: %w", ctx.Err())
	}
}

func (s *taskRoleRuntimeSessions) Create(_ context.Context, opts session.CreateOpts) (*session.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.createCalls = append(s.createCalls, opts)
	id := fmt.Sprintf("role-%d", len(s.createCalls))
	info := &session.Info{
		ID:                   id,
		Name:                 opts.Name,
		AgentName:            opts.AgentName,
		Provider:             opts.Provider,
		WorkspaceID:          opts.Workspace,
		Workspace:            firstNonEmpty(opts.Workspace, opts.WorkspacePath),
		NetworkParticipation: daemonTestParticipationFromCreateOpts(opts),
		Type:                 opts.Type,
		State:                session.StateActive,
		CreatedAt:            time.Date(2026, 5, 6, 12, 2, 0, len(s.createCalls), time.UTC),
	}
	s.infos = append(s.infos, info)
	return &session.Session{
		ID:                   info.ID,
		Name:                 info.Name,
		AgentName:            info.AgentName,
		Provider:             info.Provider,
		WorkspaceID:          info.WorkspaceID,
		Workspace:            info.Workspace,
		NetworkParticipation: info.NetworkParticipation,
		Type:                 info.Type,
		State:                info.State,
		CreatedAt:            info.CreatedAt,
	}, nil
}

func (s *taskRoleRuntimeSessions) ListAll(context.Context) ([]*session.Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	infos := make([]*session.Info, len(s.infos))
	copy(infos, s.infos)
	return infos, nil
}

func (s *taskRoleRuntimeSessions) Events(
	_ context.Context,
	id string,
	query store.EventQuery,
) ([]store.SessionEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := s.events[strings.TrimSpace(id)]
	filtered := make([]store.SessionEvent, 0, len(events))
	for _, event := range events {
		if query.Type != "" && event.Type != query.Type {
			continue
		}
		if query.TurnID != "" && event.TurnID != query.TurnID {
			continue
		}
		filtered = append(filtered, event)
		if query.Limit > 0 && len(filtered) == query.Limit {
			break
		}
	}
	return filtered, nil
}

func (s *taskRoleRuntimeSessions) PromptSynthetic(
	_ context.Context,
	id string,
	opts session.SyntheticPromptOpts,
) (<-chan acp.AgentEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.promptCalls = append(s.promptCalls, taskRoleRuntimePromptCall{sessionID: id, opts: opts})
	if len(s.promptErrors) > 0 {
		err := s.promptErrors[0]
		s.promptErrors = s.promptErrors[1:]
		if err != nil {
			return nil, err
		}
	}
	if s.events == nil {
		s.events = make(map[string][]store.SessionEvent)
	}
	target := strings.TrimSpace(id)
	s.events[target] = append(s.events[target], store.SessionEvent{
		SessionID: target,
		Sequence:  int64(len(s.events[target]) + 1),
		TurnID:    opts.TurnID,
		Type:      acp.EventTypeDone,
	})
	events := make(chan acp.AgentEvent)
	close(events)
	return events, nil
}

func (s *taskRoleRuntimeSessions) StopWithCause(
	_ context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopCalls = append(s.stopCalls, taskRoleRuntimeStopCall{sessionID: id, cause: cause, detail: detail})
	if s.stopError != nil {
		return s.stopError
	}
	for _, info := range s.infos {
		if info != nil && info.ID == id {
			info.State = session.StateStopped
		}
	}
	return nil
}

func (s *taskRoleRuntimeSessions) createCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.createCalls)
}

func (s *taskRoleRuntimeSessions) createCall(index int) session.CreateOpts {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.createCalls) {
		return session.CreateOpts{}
	}
	return s.createCalls[index]
}

func (s *taskRoleRuntimeSessions) promptCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.promptCalls)
}

func (s *taskRoleRuntimeSessions) promptCall(index int) taskRoleRuntimePromptCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.promptCalls) {
		return taskRoleRuntimePromptCall{}
	}
	return s.promptCalls[index]
}

func (s *taskRoleRuntimeSessions) stopCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.stopCalls)
}

func (s *taskRoleRuntimeSessions) stopCall(index int) taskRoleRuntimeStopCall {
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(s.stopCalls) {
		return taskRoleRuntimeStopCall{}
	}
	return s.stopCalls[index]
}
