package automation

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	aghconfig "github.com/compozy/agh/internal/config"
	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/testutil"
	"github.com/compozy/agh/internal/vault"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestManagerStartSyncsConfigDefinitionsAndPreservesDynamicEntries(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	cfg := aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
		Jobs: []aghconfig.AutomationJob{
			managerConfigJob(AutomationScopeWorkspace, "config-job", h.workspaceRoot, ScheduleSpec{
				Mode:     ScheduleModeEvery,
				Interval: "1h",
			}),
		},
		Triggers: []aghconfig.AutomationTrigger{
			managerConfigTrigger(AutomationScopeWorkspace, "config-trigger", h.workspaceRoot, "session.stopped"),
		},
	}

	dynamicJob, err := h.db.CreateJob(h.ctx, testJob(AutomationScopeGlobal, "dynamic-job", ""))
	if err != nil {
		t.Fatalf("CreateJob(dynamic) error = %v", err)
	}
	dynamicTrigger := Trigger{
		ID:        "trigger-dynamic-session-stopped",
		Scope:     AutomationScopeGlobal,
		Name:      "dynamic-trigger",
		AgentName: "reviewer",
		Prompt:    `Review session {{ index .Data "session_id" }}`,
		Event:     "session.stopped",
		Enabled:   true,
		Retry:     DefaultRetryConfig(),
		FireLimit: DefaultFireLimitConfig(),
		Source:    JobSourceDynamic,
	}
	dynamicTrigger, err = h.db.CreateTrigger(h.ctx, dynamicTrigger)
	if err != nil {
		t.Fatalf("CreateTrigger(dynamic) error = %v", err)
	}

	manager := h.newManager(t, cfg)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	jobs, err := manager.Jobs(h.ctx)
	if err != nil {
		t.Fatalf("manager.Jobs() error = %v", err)
	}
	if got, want := len(jobs), 2; got != want {
		t.Fatalf("len(jobs) = %d, want %d", got, want)
	}
	if findJobByID(jobs, dynamicJob.ID) == nil {
		t.Fatalf("jobs missing dynamic job %q", dynamicJob.ID)
	}

	configJob, err := manager.resolveConfigJob(h.ctx, cfg.Jobs[0])
	if err != nil {
		t.Fatalf("resolveConfigJob() error = %v", err)
	}
	gotConfigJob := findJobByID(jobs, configJob.ID)
	if gotConfigJob == nil {
		t.Fatalf("jobs missing config job %q", configJob.ID)
		return
	}
	if got, want := gotConfigJob.Source, JobSourceConfig; got != want {
		t.Fatalf("config job source = %q, want %q", got, want)
	}
	if got, want := gotConfigJob.WorkspaceID, h.workspace.ID; got != want {
		t.Fatalf("config job workspace_id = %q, want %q", got, want)
	}

	triggers, err := manager.Triggers(h.ctx)
	if err != nil {
		t.Fatalf("manager.Triggers() error = %v", err)
	}
	if got, want := len(triggers), 2; got != want {
		t.Fatalf("len(triggers) = %d, want %d", got, want)
	}
	if findTriggerByID(triggers, dynamicTrigger.ID) == nil {
		t.Fatalf("triggers missing dynamic trigger %q", dynamicTrigger.ID)
	}

	configTrigger, err := manager.resolveConfigTrigger(h.ctx, cfg.Triggers[0])
	if err != nil {
		t.Fatalf("resolveConfigTrigger() error = %v", err)
	}
	gotConfigTrigger := findTriggerByID(triggers, configTrigger.ID)
	if gotConfigTrigger == nil {
		t.Fatalf("triggers missing config trigger %q", configTrigger.ID)
		return
	}
	if got, want := gotConfigTrigger.Source, JobSourceConfig; got != want {
		t.Fatalf("config trigger source = %q, want %q", got, want)
	}
	if got, want := gotConfigTrigger.WorkspaceID, h.workspace.ID; got != want {
		t.Fatalf("config trigger workspace_id = %q, want %q", got, want)
	}
}

func TestManagerStartPreservesCallerContextValuesInRuntimeContext(t *testing.T) {
	t.Parallel()

	type contextKey string

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})

	const key contextKey = "trace"
	startCtx := context.WithValue(h.ctx, key, "automation-runtime")

	if err := manager.Start(startCtx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	if got, want := manager.runtimeCtx.Value(key), any("automation-runtime"); got != want {
		t.Fatalf("manager.runtimeCtx.Value(%q) = %#v, want %#v", key, got, want)
	}
	if err := manager.runtimeCtx.Err(); err != nil {
		t.Fatalf("manager.runtimeCtx.Err() = %v, want nil", err)
	}
}

func TestManagerStartUpdatesConfigDefinitionsAndPreservesEnabledOverlays(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	cfg := aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
		Jobs: []aghconfig.AutomationJob{
			managerConfigJob(AutomationScopeWorkspace, "config-job", h.workspaceRoot, ScheduleSpec{
				Mode:     ScheduleModeEvery,
				Interval: "45m",
			}),
		},
		Triggers: []aghconfig.AutomationTrigger{
			managerConfigTrigger(AutomationScopeWorkspace, "config-trigger", h.workspaceRoot, "session.stopped"),
		},
	}

	manager := h.newManager(t, cfg)

	oldJobRaw := cfg.Jobs[0]
	oldJobRaw.Prompt = "old prompt"
	oldJobRaw.Schedule = ScheduleSpec{
		Mode:     ScheduleModeEvery,
		Interval: "30m",
	}
	oldJob, err := manager.resolveConfigJob(h.ctx, oldJobRaw)
	if err != nil {
		t.Fatalf("resolveConfigJob(old) error = %v", err)
	}
	if _, err := h.db.CreateJob(h.ctx, oldJob); err != nil {
		t.Fatalf("CreateJob(old config) error = %v", err)
	}
	if _, err := h.db.SetJobEnabledOverlay(h.ctx, JobEnabledOverlay{
		JobID:           oldJob.ID,
		EnabledOverride: false,
	}); err != nil {
		t.Fatalf("SetJobEnabledOverlay() error = %v", err)
	}

	oldTriggerRaw := cfg.Triggers[0]
	oldTriggerRaw.Prompt = `old trigger {{ index .Data "session_id" }}`
	oldTriggerRaw.Filter = map[string]string{"data.agent_name": "old-agent"}
	oldTrigger, err := manager.resolveConfigTrigger(h.ctx, oldTriggerRaw)
	if err != nil {
		t.Fatalf("resolveConfigTrigger(old) error = %v", err)
	}
	if _, err := h.db.CreateTrigger(h.ctx, oldTrigger); err != nil {
		t.Fatalf("CreateTrigger(old config) error = %v", err)
	}
	if _, err := h.db.SetTriggerEnabledOverlay(h.ctx, TriggerEnabledOverlay{
		TriggerID:       oldTrigger.ID,
		EnabledOverride: false,
	}); err != nil {
		t.Fatalf("SetTriggerEnabledOverlay() error = %v", err)
	}

	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	updatedJob, err := h.db.GetJob(h.ctx, oldJob.ID)
	if err != nil {
		t.Fatalf("GetJob(updated config) error = %v", err)
	}
	if got, want := updatedJob.Prompt, cfg.Jobs[0].Prompt; got != want {
		t.Fatalf("updated job prompt = %q, want %q", got, want)
	}
	if updatedJob.Schedule == nil || updatedJob.Schedule.Interval != cfg.Jobs[0].Schedule.Interval {
		t.Fatalf("updated job schedule = %#v, want interval %q", updatedJob.Schedule, cfg.Jobs[0].Schedule.Interval)
	}
	if !updatedJob.Enabled {
		t.Fatal("updated config job enabled default = false, want true")
	}

	effectiveJobs, err := manager.Jobs(h.ctx)
	if err != nil {
		t.Fatalf("manager.Jobs() error = %v", err)
	}
	effectiveJob := findJobByID(effectiveJobs, oldJob.ID)
	if effectiveJob == nil || effectiveJob.Enabled {
		t.Fatalf("effective config job = %#v, want enabled overlay false", effectiveJob)
	}

	updatedTrigger, err := h.db.GetTrigger(h.ctx, oldTrigger.ID)
	if err != nil {
		t.Fatalf("GetTrigger(updated config) error = %v", err)
	}
	if got, want := updatedTrigger.Prompt, cfg.Triggers[0].Prompt; got != want {
		t.Fatalf("updated trigger prompt = %q, want %q", got, want)
	}
	if got, want := updatedTrigger.Filter["data.agent_name"], cfg.Triggers[0].Filter["data.agent_name"]; got != want {
		t.Fatalf("updated trigger filter = %q, want %q", got, want)
	}
	if !updatedTrigger.Enabled {
		t.Fatal("updated config trigger enabled default = false, want true")
	}

	effectiveTriggers, err := manager.Triggers(h.ctx)
	if err != nil {
		t.Fatalf("manager.Triggers() error = %v", err)
	}
	effectiveTrigger := findTriggerByID(effectiveTriggers, oldTrigger.ID)
	if effectiveTrigger == nil || effectiveTrigger.Enabled {
		t.Fatalf("effective config trigger = %#v, want enabled overlay false", effectiveTrigger)
	}
}

func TestManagerStatusReportsCountsAndNextFire(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	nextFire := time.Now().UTC().Add(2 * time.Hour).Truncate(time.Second)
	syncTime := time.Date(2026, 4, 11, 12, 0, 0, 0, time.UTC)
	cfg := aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
		Jobs: []aghconfig.AutomationJob{
			managerConfigJob(AutomationScopeWorkspace, "enabled-job", h.workspaceRoot, ScheduleSpec{
				Mode: ScheduleModeAt,
				Time: nextFire.Format(time.RFC3339),
			}),
			func() aghconfig.AutomationJob {
				job := managerConfigJob(AutomationScopeWorkspace, "disabled-job", h.workspaceRoot, ScheduleSpec{
					Mode:     ScheduleModeEvery,
					Interval: "2h",
				})
				job.Enabled = false
				return job
			}(),
		},
		Triggers: []aghconfig.AutomationTrigger{
			managerConfigTrigger(AutomationScopeWorkspace, "enabled-trigger", h.workspaceRoot, "session.stopped"),
			func() aghconfig.AutomationTrigger {
				trigger := managerConfigTrigger(
					AutomationScopeWorkspace,
					"disabled-trigger",
					h.workspaceRoot,
					"memory.consolidated",
				)
				trigger.Enabled = false
				trigger.Filter = nil
				return trigger
			}(),
		},
	}

	manager := h.newManager(
		t,
		cfg,
		WithManagerNow(func() time.Time { return syncTime }),
		WithDispatcherOptions(WithDispatcherMaxConcurrent(2)),
		WithSchedulerOptions(WithSchedulerStopTimeout(time.Second)),
		WithTriggerEngineOptions(WithTriggerEngineWebhookFreshnessWindow(2*time.Minute)),
	)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	status, err := manager.Status(h.ctx)
	if err != nil {
		t.Fatalf("manager.Status() error = %v", err)
	}
	if !status.Running {
		t.Fatal("status.Running = false, want true")
	}
	if !status.SchedulerRunning {
		t.Fatal("status.SchedulerRunning = false, want true")
	}
	if got, want := status.Jobs, (ResourceStatus{Total: 2, Enabled: 1}); got != want {
		t.Fatalf("status.Jobs = %#v, want %#v", got, want)
	}
	if got, want := status.Triggers, (ResourceStatus{Total: 2, Enabled: 1}); got != want {
		t.Fatalf("status.Triggers = %#v, want %#v", got, want)
	}
	if got, want := len(status.ScheduledJobs), 1; got != want {
		t.Fatalf("len(status.ScheduledJobs) = %d, want %d", got, want)
	}
	if status.NextFire == nil {
		t.Fatal("status.NextFire = nil, want non-nil")
	}
	if got := status.NextFire.UTC(); got.Sub(nextFire) > time.Second || nextFire.Sub(got) > time.Second {
		t.Fatalf("status.NextFire = %s, want about %s", got.Format(time.RFC3339), nextFire.Format(time.RFC3339))
	}
	if got, want := status.LastSync.JobsSynced, 2; got != want {
		t.Fatalf("status.LastSync.JobsSynced = %d, want %d", got, want)
	}
	if got, want := status.LastSync.TriggersSynced, 2; got != want {
		t.Fatalf("status.LastSync.TriggersSynced = %d, want %d", got, want)
	}
	if got, want := status.LastSync.SyncedAt, syncTime; got != want {
		t.Fatalf("status.LastSync.SyncedAt = %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
	}
}

func TestManagerObserversHandleNilManagerAndAgentEvents(t *testing.T) {
	t.Parallel()

	var sessionObserver managerSessionObserver
	sessionObserver.OnSessionCreated(testutil.Context(t), nil)
	sessionObserver.OnSessionStopped(testutil.Context(t), nil)
	sessionObserver.OnAgentEvent(testutil.Context(t), "agent.event", map[string]any{"k": "v"})

	var hookSink managerHookTelemetrySink
	if err := hookSink.WriteHookRecord(testutil.Context(t), "sess", hookspkg.HookRunRecord{}); err != nil {
		t.Fatalf("WriteHookRecord(nil manager) error = %v", err)
	}

	var memoryObserver managerMemoryObserver
	if err := memoryObserver.OnMemoryConsolidated(testutil.Context(t), MemoryConsolidatedEvent{}); err != nil {
		t.Fatalf("OnMemoryConsolidated(nil manager) error = %v", err)
	}
}

func TestManagerSetEnabledForConfigBackedDefinitionsUsesOverlaysOnly(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	cfg := aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
		Jobs: []aghconfig.AutomationJob{
			managerConfigJob(AutomationScopeWorkspace, "config-job", h.workspaceRoot, ScheduleSpec{
				Mode:     ScheduleModeEvery,
				Interval: "1h",
			}),
		},
		Triggers: []aghconfig.AutomationTrigger{
			managerConfigTrigger(AutomationScopeWorkspace, "config-trigger", h.workspaceRoot, "session.stopped"),
		},
	}

	manager := h.newManager(t, cfg)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	configJob, err := manager.resolveConfigJob(h.ctx, cfg.Jobs[0])
	if err != nil {
		t.Fatalf("resolveConfigJob() error = %v", err)
	}
	configTrigger, err := manager.resolveConfigTrigger(h.ctx, cfg.Triggers[0])
	if err != nil {
		t.Fatalf("resolveConfigTrigger() error = %v", err)
	}

	updatedJob, err := manager.SetJobEnabled(h.ctx, configJob.ID, false)
	if err != nil {
		t.Fatalf("SetJobEnabled() error = %v", err)
	}
	if updatedJob.Enabled {
		t.Fatalf("SetJobEnabled() returned enabled=%v, want false", updatedJob.Enabled)
	}

	updatedTrigger, err := manager.SetTriggerEnabled(h.ctx, configTrigger.ID, false)
	if err != nil {
		t.Fatalf("SetTriggerEnabled() error = %v", err)
	}
	if updatedTrigger.Enabled {
		t.Fatalf("SetTriggerEnabled() returned enabled=%v, want false", updatedTrigger.Enabled)
	}

	storedJob, err := h.db.GetJob(h.ctx, configJob.ID)
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if !storedJob.Enabled {
		t.Fatal("stored config job enabled default = false, want true")
	}
	jobOverlay, err := h.db.GetJobEnabledOverlay(h.ctx, configJob.ID)
	if err != nil {
		t.Fatalf("GetJobEnabledOverlay() error = %v", err)
	}
	if jobOverlay.EnabledOverride {
		t.Fatal("job overlay enabled_override = true, want false")
	}

	storedTrigger, err := h.db.GetTrigger(h.ctx, configTrigger.ID)
	if err != nil {
		t.Fatalf("GetTrigger() error = %v", err)
	}
	if !storedTrigger.Enabled {
		t.Fatal("stored config trigger enabled default = false, want true")
	}
	triggerOverlay, err := h.db.GetTriggerEnabledOverlay(h.ctx, configTrigger.ID)
	if err != nil {
		t.Fatalf("GetTriggerEnabledOverlay() error = %v", err)
	}
	if triggerOverlay.EnabledOverride {
		t.Fatal("trigger overlay enabled_override = true, want false")
	}

	status, err := manager.Status(h.ctx)
	if err != nil {
		t.Fatalf("manager.Status() error = %v", err)
	}
	if got, want := status.Jobs.Enabled, 0; got != want {
		t.Fatalf("status.Jobs.Enabled = %d, want %d", got, want)
	}
	if got, want := status.Triggers.Enabled, 0; got != want {
		t.Fatalf("status.Triggers.Enabled = %d, want %d", got, want)
	}
	if got := len(status.ScheduledJobs); got != 0 {
		t.Fatalf("len(status.ScheduledJobs) = %d, want 0 after disabling config job", got)
	}

	if _, err := manager.SetJobEnabled(h.ctx, configJob.ID, true); err != nil {
		t.Fatalf("SetJobEnabled(re-enable) error = %v", err)
	}
	if _, err := manager.SetTriggerEnabled(h.ctx, configTrigger.ID, true); err != nil {
		t.Fatalf("SetTriggerEnabled(re-enable) error = %v", err)
	}
	if _, err := h.db.GetJobEnabledOverlay(h.ctx, configJob.ID); err == nil {
		t.Fatal("GetJobEnabledOverlay() error = nil after re-enable, want overlay removed")
	}
	if _, err := h.db.GetTriggerEnabledOverlay(h.ctx, configTrigger.ID); err == nil {
		t.Fatal("GetTriggerEnabledOverlay() error = nil after re-enable, want overlay removed")
	}
}

func TestManagerObserversAndRunsRouteTriggerEvents(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	cfg := aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
		Triggers: []aghconfig.AutomationTrigger{
			func() aghconfig.AutomationTrigger {
				trigger := managerConfigTrigger(
					AutomationScopeWorkspace,
					"session-created",
					h.workspaceRoot,
					"session.created",
				)
				trigger.Filter = map[string]string{"data.agent_name": "reviewer"}
				return trigger
			}(),
			managerConfigTrigger(AutomationScopeWorkspace, "session-stopped", h.workspaceRoot, "session.stopped"),
			func() aghconfig.AutomationTrigger {
				trigger := managerConfigTrigger(
					AutomationScopeWorkspace,
					"hook-completed",
					h.workspaceRoot,
					"hook.test-hook.completed",
				)
				trigger.Filter = map[string]string{"data.hook_outcome": "applied"}
				return trigger
			}(),
			managerConfigTrigger(AutomationScopeWorkspace, "memory", h.workspaceRoot, "memory.consolidated"),
		},
	}

	manager := h.newManager(t, cfg)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	h.sessions.setStatus(&session.Info{
		ID:          "sess-hook",
		Name:        "hook-session",
		AgentName:   "reviewer",
		WorkspaceID: h.workspace.ID,
		Type:        session.SessionTypeUser,
	})

	sessionObserver := manager.SessionObserver()
	sessionObserver.OnSessionCreated(h.ctx, &session.Session{
		ID:          "sess-created",
		AgentName:   "reviewer",
		WorkspaceID: h.workspace.ID,
		Workspace:   h.workspace.RootDir,
		Type:        session.SessionTypeUser,
		State:       session.StateActive,
	})
	sessionObserver.OnSessionStopped(h.ctx, &session.Session{
		ID:          "sess-stopped",
		AgentName:   "reviewer",
		WorkspaceID: h.workspace.ID,
		Workspace:   h.workspace.RootDir,
		Type:        session.SessionTypeUser,
		State:       session.StateStopped,
	})
	sessionObserver.OnAgentEvent(h.ctx, "sess-created", acp.AgentEvent{})

	if err := manager.HookTelemetrySink().WriteHookRecord(h.ctx, "sess-hook", hookspkg.HookRunRecord{
		HookName:   "test-hook",
		Event:      hookspkg.HookSessionPostStop,
		Source:     hookspkg.HookSourceConfig,
		Mode:       hookspkg.HookModeSync,
		Outcome:    hookspkg.HookRunOutcomeApplied,
		RecordedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("WriteHookRecord() error = %v", err)
	}
	if err := manager.MemoryObserver().OnMemoryConsolidated(h.ctx, MemoryConsolidatedEvent{
		WorkspaceID: h.workspace.ID,
		Timestamp:   time.Now().UTC(),
	}); err != nil {
		t.Fatalf("OnMemoryConsolidated() error = %v", err)
	}

	if got, want := h.sessions.promptCount(), 4; got != want {
		t.Fatalf("Prompt() call count = %d, want %d", got, want)
	}

	runs, err := manager.Runs(h.ctx, RunQuery{})
	if err != nil {
		t.Fatalf("manager.Runs() error = %v", err)
	}
	if got, want := len(runs), 4; got != want {
		t.Fatalf("len(runs) = %d, want %d", got, want)
	}
}

func TestManagerSessionTaskActorLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should record, load, and delete a session task actor", func(t *testing.T) {
		t.Parallel()

		h := newManagerHarness(t)
		manager := h.newManager(t, aghconfig.AutomationConfig{
			Enabled:           true,
			Timezone:          DefaultTimezone,
			MaxConcurrentJobs: DefaultMaxConcurrentJobs,
			DefaultFireLimit:  DefaultFireLimitConfig(),
		})

		actor, err := taskpkg.DeriveAutomationLinkedAgentSessionActorContext(
			"sess-actor-1",
			"ws-1",
			"run:run-1",
		)
		if err != nil {
			t.Fatalf("DeriveAutomationLinkedAgentSessionActorContext() error = %v", err)
		}
		if err := manager.RecordAutomationSessionTaskActor("sess-actor-1", actor); err != nil {
			t.Fatalf("RecordAutomationSessionTaskActor() error = %v", err)
		}

		loaded, err := manager.TaskActorContextForSession("sess-actor-1")
		if err != nil {
			t.Fatalf("TaskActorContextForSession() error = %v", err)
		}
		if loaded != actor {
			t.Fatalf("TaskActorContextForSession() = %#v, want %#v", loaded, actor)
		}

		manager.DeleteAutomationSessionTaskActor("sess-actor-1")
		if _, err := manager.TaskActorContextForSession("sess-actor-1"); !errors.Is(err, ErrSessionTaskActorNotFound) {
			t.Fatalf("TaskActorContextForSession(after delete) error = %v, want ErrSessionTaskActorNotFound", err)
		}
	})
}

func TestManagerHandleWebhookWithSecretResolver(t *testing.T) {
	h := newManagerHarness(t)
	t.Setenv("AGH_TEST_WEBHOOK_SECRET", "super-secret")
	cfg := aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
		Triggers: []aghconfig.AutomationTrigger{
			func() aghconfig.AutomationTrigger {
				trigger := managerConfigTrigger(AutomationScopeWorkspace, "webhook-trigger", h.workspaceRoot, "webhook")
				trigger.EndpointSlug = "deploy-review"
				trigger.WebhookSecretRef = "env:AGH_TEST_WEBHOOK_SECRET"
				trigger.Filter = map[string]string{"data.payload": "deploy"}
				return trigger
			}(),
		},
	}

	const webhookSecret = "super-secret"
	manager := h.newManager(t, cfg)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	trigger, err := manager.resolveConfigTrigger(h.ctx, cfg.Triggers[0])
	if err != nil {
		t.Fatalf("resolveConfigTrigger() error = %v", err)
	}
	endpoint, err := FormatWebhookEndpoint(trigger.EndpointSlug, trigger.WebhookID)
	if err != nil {
		t.Fatalf("FormatWebhookEndpoint() error = %v", err)
	}

	payload := []byte(`{"payload":"deploy"}`)
	timestamp := time.Now().UTC()
	signature, err := SignWebhookPayload(webhookSecret, timestamp, payload)
	if err != nil {
		t.Fatalf("SignWebhookPayload() error = %v", err)
	}

	result, err := manager.HandleWebhook(h.ctx, WebhookRequest{
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: h.workspace.ID,
		Endpoint:    endpoint,
		DeliveryID:  "delivery-1",
		Timestamp:   timestamp,
		Signature:   signature,
		Payload:     payload,
		Data: map[string]any{
			"payload": "deploy",
		},
	})
	if err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}
	if got, want := result.Matched, 1; got != want {
		t.Fatalf("result.Matched = %d, want %d", got, want)
	}
	if got, want := h.sessions.promptCount(), 1; got != want {
		t.Fatalf("Prompt() call count = %d, want %d", got, want)
	}
}

func TestManagerHandleWebhookWithConfigSecretEnv(t *testing.T) {
	h := newManagerHarness(t)
	t.Setenv("AGH_TEST_CONFIG_WEBHOOK_SECRET", "config-super-secret")
	cfg := aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
		Triggers: []aghconfig.AutomationTrigger{
			func() aghconfig.AutomationTrigger {
				trigger := managerConfigTrigger(
					AutomationScopeWorkspace,
					"config-webhook-trigger",
					h.workspaceRoot,
					"webhook",
				)
				trigger.EndpointSlug = "config-deploy-review"
				trigger.WebhookSecretRef = "env:AGH_TEST_CONFIG_WEBHOOK_SECRET"
				trigger.Filter = map[string]string{"data.payload": "deploy"}
				return trigger
			}(),
		},
	}

	manager := h.newManager(t, cfg)
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	trigger, err := manager.resolveConfigTrigger(h.ctx, cfg.Triggers[0])
	if err != nil {
		t.Fatalf("resolveConfigTrigger() error = %v", err)
	}
	endpoint, err := FormatWebhookEndpoint(trigger.EndpointSlug, trigger.WebhookID)
	if err != nil {
		t.Fatalf("FormatWebhookEndpoint() error = %v", err)
	}

	payload := []byte(`{"payload":"deploy"}`)
	timestamp := time.Now().UTC()
	signature, err := SignWebhookPayload("config-super-secret", timestamp, payload)
	if err != nil {
		t.Fatalf("SignWebhookPayload() error = %v", err)
	}

	result, err := manager.HandleWebhook(h.ctx, WebhookRequest{
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: h.workspace.ID,
		Endpoint:    endpoint,
		DeliveryID:  "delivery-config-1",
		Timestamp:   timestamp,
		Signature:   signature,
		Payload:     payload,
		Data: map[string]any{
			"payload": "deploy",
		},
	})
	if err != nil {
		t.Fatalf("HandleWebhook() error = %v", err)
	}
	if got, want := result.Matched, 1; got != want {
		t.Fatalf("result.Matched = %d, want %d", got, want)
	}
	if got, want := h.sessions.promptCount(), 1; got != want {
		t.Fatalf("Prompt() call count = %d, want %d", got, want)
	}
}

func TestManagerSetEnabledForDynamicDefinitionsUpdatesStoredStateAndRuntime(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	dynamicJob, err := h.db.CreateJob(h.ctx, testJob(AutomationScopeWorkspace, "dynamic-runtime-job", h.workspace.ID))
	if err != nil {
		t.Fatalf("CreateJob(dynamic) error = %v", err)
	}
	dynamicTrigger, err := h.db.CreateTrigger(h.ctx, Trigger{
		ID:          "trigger-dynamic-runtime",
		Scope:       AutomationScopeWorkspace,
		Name:        "dynamic-runtime-trigger",
		AgentName:   "reviewer",
		WorkspaceID: h.workspace.ID,
		Prompt:      `Review session {{ index .Data "session_id" }}`,
		Event:       "session.stopped",
		Enabled:     true,
		Retry:       DefaultRetryConfig(),
		FireLimit:   DefaultFireLimitConfig(),
		Source:      JobSourceDynamic,
	})
	if err != nil {
		t.Fatalf("CreateTrigger(dynamic) error = %v", err)
	}

	jobDisabled, err := manager.SetJobEnabled(h.ctx, dynamicJob.ID, false)
	if err != nil {
		t.Fatalf("SetJobEnabled(false) error = %v", err)
	}
	if jobDisabled.Enabled {
		t.Fatalf("SetJobEnabled(false) returned enabled=%v, want false", jobDisabled.Enabled)
	}
	jobEnabled, err := manager.SetJobEnabled(h.ctx, dynamicJob.ID, true)
	if err != nil {
		t.Fatalf("SetJobEnabled(true) error = %v", err)
	}
	if !jobEnabled.Enabled {
		t.Fatalf("SetJobEnabled(true) returned enabled=%v, want true", jobEnabled.Enabled)
	}

	triggerDisabled, err := manager.SetTriggerEnabled(h.ctx, dynamicTrigger.ID, false)
	if err != nil {
		t.Fatalf("SetTriggerEnabled(false) error = %v", err)
	}
	if triggerDisabled.Enabled {
		t.Fatalf("SetTriggerEnabled(false) returned enabled=%v, want false", triggerDisabled.Enabled)
	}
	triggerEnabled, err := manager.SetTriggerEnabled(h.ctx, dynamicTrigger.ID, true)
	if err != nil {
		t.Fatalf("SetTriggerEnabled(true) error = %v", err)
	}
	if !triggerEnabled.Enabled {
		t.Fatalf("SetTriggerEnabled(true) returned enabled=%v, want true", triggerEnabled.Enabled)
	}

	status, err := manager.Status(h.ctx)
	if err != nil {
		t.Fatalf("manager.Status() error = %v", err)
	}
	if got, want := status.Jobs.Enabled, 1; got != want {
		t.Fatalf("status.Jobs.Enabled = %d, want %d", got, want)
	}
	if got, want := status.Triggers.Enabled, 1; got != want {
		t.Fatalf("status.Triggers.Enabled = %d, want %d", got, want)
	}
	if got, want := len(status.ScheduledJobs), 1; got != want {
		t.Fatalf("len(status.ScheduledJobs) = %d, want %d", got, want)
	}
}

func TestManagerDynamicJobCRUDAndRunHistory(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	job := testJob(AutomationScopeWorkspace, "crud-job", h.workspace.ID)
	created, err := manager.CreateJob(h.ctx, job)
	if err != nil {
		t.Fatalf("manager.CreateJob() error = %v", err)
	}
	if created.ID == "" {
		t.Fatal("manager.CreateJob() id = empty, want non-empty")
	}
	if got, want := created.Scope, job.Scope; got != want {
		t.Fatalf("created job scope = %q, want %q", got, want)
	}
	if got, want := created.WorkspaceID, h.workspace.ID; got != want {
		t.Fatalf("created job workspace_id = %q, want %q", got, want)
	}
	if got, want := created.Source, JobSourceDynamic; got != want {
		t.Fatalf("created job source = %q, want %q", got, want)
	}
	if got, want := created.AgentName, job.AgentName; got != want {
		t.Fatalf("created job agent_name = %q, want %q", got, want)
	}

	gotJob, err := manager.GetJob(h.ctx, created.ID)
	if err != nil {
		t.Fatalf("manager.GetJob() error = %v", err)
	}
	if gotJob.Name != created.Name {
		t.Fatalf("manager.GetJob() name = %q, want %q", gotJob.Name, created.Name)
	}

	jobs, err := manager.ListJobs(h.ctx, JobListQuery{
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: h.workspace.ID,
	})
	if err != nil {
		t.Fatalf("manager.ListJobs() error = %v", err)
	}
	if got, want := len(jobs.Jobs), 1; got != want {
		t.Fatalf("len(manager.ListJobs()) = %d, want %d", got, want)
	}

	updatedInput := created
	updatedInput.Name = "crud-job-updated"
	updatedInput.Prompt = "Summarize the updated state."
	updatedInput.Schedule = &ScheduleSpec{
		Mode:     ScheduleModeEvery,
		Interval: "45m",
	}
	updated, err := manager.UpdateJob(h.ctx, updatedInput)
	if err != nil {
		t.Fatalf("manager.UpdateJob() error = %v", err)
	}
	if got, want := updated.Name, "crud-job-updated"; got != want {
		t.Fatalf("updated job name = %q, want %q", got, want)
	}
	if updated.Schedule == nil || updated.Schedule.Interval != "45m" {
		t.Fatalf("updated job schedule = %#v, want interval 45m", updated.Schedule)
	}

	run, err := manager.TriggerJob(h.ctx, updated.ID)
	if err != nil {
		t.Fatalf("manager.TriggerJob() error = %v", err)
	}
	if got, want := run.JobID, updated.ID; got != want {
		t.Fatalf("run.JobID = %q, want %q", got, want)
	}
	if got, want := h.sessions.promptCount(), 1; got != want {
		t.Fatalf("Prompt() call count = %d, want %d", got, want)
	}

	gotRun, err := manager.GetRun(h.ctx, run.ID)
	if err != nil {
		t.Fatalf("manager.GetRun() error = %v", err)
	}
	if got, want := gotRun.ID, run.ID; got != want {
		t.Fatalf("manager.GetRun() id = %q, want %q", got, want)
	}

	runs, err := manager.ListRuns(h.ctx, RunQuery{JobID: updated.ID})
	if err != nil {
		t.Fatalf("manager.ListRuns() error = %v", err)
	}
	if got, want := len(runs), 1; got != want {
		t.Fatalf("len(manager.ListRuns()) = %d, want %d", got, want)
	}

	if err := manager.DeleteJob(h.ctx, updated.ID); err != nil {
		t.Fatalf("manager.DeleteJob() error = %v", err)
	}
	if _, err := manager.GetJob(h.ctx, updated.ID); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("manager.GetJob(deleted) error = %v, want ErrJobNotFound", err)
	}
}

func TestManagerCreateJobRejectsDaemonLifecycleCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		prompt          string
		taskDescription string
		wantClass       DaemonLifecycleCommandClass
	}{
		{
			name:      "Should reject the AGH daemon restart command",
			prompt:    "Run `agh daemon restart` now.",
			wantClass: DaemonLifecycleCommandClassAGHDaemon,
		},
		{
			name:      "Should reject the AGH daemon stop command after persistent flags",
			prompt:    "Run `agh --output json daemon stop` now.",
			wantClass: DaemonLifecycleCommandClassAGHDaemon,
		},
		{
			name:      "Should reject a process kill targeting AGH",
			prompt:    "Execute `pkill -f agh` if the daemon is unresponsive.",
			wantClass: DaemonLifecycleCommandClassProcessSignal,
		},
		{
			name:      "Should reject a systemd restart targeting AGH",
			prompt:    "Execute `systemctl restart agh` after the report.",
			wantClass: DaemonLifecycleCommandClassServiceManager,
		},
		{
			name:      "Should reject a launchd restart targeting AGH",
			prompt:    "Execute `launchctl kickstart -k gui/$UID/com.compozy.agh` after the report.",
			wantClass: DaemonLifecycleCommandClassServiceManager,
		},
		{
			name:            "Should reject a direct task description that restarts AGH",
			prompt:          "",
			taskDescription: "Run `service agh restart` to recover the daemon.",
			wantClass:       DaemonLifecycleCommandClassServiceManager,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			h := newManagerHarness(t)
			manager := h.newManager(t, aghconfig.AutomationConfig{
				Enabled:           true,
				Timezone:          DefaultTimezone,
				MaxConcurrentJobs: DefaultMaxConcurrentJobs,
				DefaultFireLimit:  DefaultFireLimitConfig(),
			})
			if err := manager.Start(h.ctx); err != nil {
				t.Fatalf("manager.Start() error = %v", err)
			}
			t.Cleanup(func() {
				if err := manager.Shutdown(testutil.Context(t)); err != nil {
					t.Errorf("manager.Shutdown() error = %v", err)
				}
			})

			job := testJob(AutomationScopeWorkspace, "blocked-lifecycle", h.workspace.ID)
			job.Prompt = tt.prompt
			if tt.taskDescription != "" {
				job.Task = &JobTaskConfig{Description: tt.taskDescription}
			}
			_, err := manager.CreateJob(h.ctx, job)
			if err == nil {
				t.Fatal("manager.CreateJob() error = nil, want lifecycle command rejection")
			}
			if !errors.Is(err, ErrDaemonLifecycleCommandBlocked) {
				t.Fatalf("manager.CreateJob() error = %v, want ErrDaemonLifecycleCommandBlocked", err)
			}
			var blockedErr *DaemonLifecycleCommandError
			if !errors.As(err, &blockedErr) {
				t.Fatalf("manager.CreateJob() error = %T, want *DaemonLifecycleCommandError", err)
			}
			if blockedErr.Class != tt.wantClass {
				t.Fatalf("manager.CreateJob() blocked class = %q, want %q", blockedErr.Class, tt.wantClass)
			}

			page, err := manager.ListJobs(h.ctx, JobListQuery{
				Scope:       AutomationScopeWorkspace,
				WorkspaceID: h.workspace.ID,
			})
			if err != nil {
				t.Fatalf("manager.ListJobs() error = %v", err)
			}
			if len(page.Jobs) != 0 {
				t.Fatalf("manager.ListJobs() jobs = %#v, want no persisted job", page.Jobs)
			}
		})
	}

	t.Run("Should accept lifecycle prose in non-command fields", func(t *testing.T) {
		t.Parallel()

		h := newManagerHarness(t)
		manager := h.newManager(t, aghconfig.AutomationConfig{
			Enabled:           true,
			Timezone:          DefaultTimezone,
			MaxConcurrentJobs: DefaultMaxConcurrentJobs,
			DefaultFireLimit:  DefaultFireLimitConfig(),
		})
		if err := manager.Start(h.ctx); err != nil {
			t.Fatalf("manager.Start() error = %v", err)
		}
		t.Cleanup(func() {
			if err := manager.Shutdown(testutil.Context(t)); err != nil {
				t.Errorf("manager.Shutdown() error = %v", err)
			}
		})

		job := testJob(AutomationScopeWorkspace, "Review agh daemon restart behavior", h.workspace.ID)
		job.Prompt = "Summarize the supervisor lifecycle design."
		if _, err := manager.CreateJob(h.ctx, job); err != nil {
			t.Fatalf("manager.CreateJob(prose) error = %v", err)
		}
	})
}

func TestManagerUpdateJobRejectsDaemonLifecycleCommands(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})

	created, err := manager.CreateJob(
		h.ctx,
		testJob(AutomationScopeWorkspace, "safe-before-blocked-update", h.workspace.ID),
	)
	if err != nil {
		t.Fatalf("manager.CreateJob() error = %v", err)
	}

	blocked := created
	blocked.Prompt = "Run `agh daemon stop` now."
	if _, err := manager.UpdateJob(h.ctx, blocked); !errors.Is(err, ErrDaemonLifecycleCommandBlocked) {
		t.Fatalf("manager.UpdateJob() error = %v, want ErrDaemonLifecycleCommandBlocked", err)
	}

	stored, err := manager.GetJob(h.ctx, created.ID)
	if err != nil {
		t.Fatalf("manager.GetJob() error = %v", err)
	}
	if stored.Prompt != created.Prompt {
		t.Fatalf("stored job prompt = %q, want unchanged %q", stored.Prompt, created.Prompt)
	}
}

func TestManagerDynamicLoopTargetCRUDValidatesLoopStarter(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	starter := &recordingLoopStarter{}
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	}, WithLoopStarter(starter))

	job := testJob(AutomationScopeWorkspace, "crud-loop-job", h.workspace.ID)
	job.AgentName = ""
	job.Prompt = ""
	job.TargetKind = TargetKindLoop
	job.LoopTarget = &LoopTarget{
		WorkspaceID: h.workspace.ID,
		LoopName:    "triage",
		Inputs: map[string]any{
			"tasks": "task-ref",
		},
	}
	createdJob, err := manager.CreateJob(h.ctx, job)
	if err != nil {
		t.Fatalf("manager.CreateJob(loop target) error = %v", err)
	}

	updatedJob := createdJob
	updatedJob.LoopTarget = &LoopTarget{
		WorkspaceID: h.workspace.ID,
		LoopName:    "triage",
		Inputs: map[string]any{
			"tasks": "updated-task-ref",
		},
	}
	if _, err := manager.UpdateJob(h.ctx, updatedJob); err != nil {
		t.Fatalf("manager.UpdateJob(loop target) error = %v", err)
	}
	changedJobTarget := updatedJob
	changedJobTarget.LoopTarget = cloneLoopTarget(updatedJob.LoopTarget)
	changedJobTarget.LoopTarget.LoopName = "triage-v2"
	if _, err := manager.UpdateJob(h.ctx, changedJobTarget); !errors.Is(err, ErrTargetIdentityImmutable) {
		t.Fatalf("manager.UpdateJob(changed target) error = %v, want ErrTargetIdentityImmutable", err)
	}

	trigger := testTrigger(AutomationScopeWorkspace, "crud-loop-trigger", h.workspace.ID)
	trigger.AgentName = ""
	trigger.Prompt = ""
	trigger.WebhookSecretRef = ""
	trigger.TargetKind = TargetKindLoop
	trigger.LoopTarget = &LoopTarget{
		WorkspaceID: h.workspace.ID,
		LoopName:    "deploy",
		InputMapping: map[string]string{
			"title": "{{ .trigger.payload.title }}",
		},
	}
	createdTrigger, err := manager.CreateTrigger(h.ctx, trigger, webhookSecretWrite("secret-v1"))
	if err != nil {
		t.Fatalf("manager.CreateTrigger(loop target) error = %v", err)
	}
	changedTriggerTarget := createdTrigger
	changedTriggerTarget.LoopTarget = cloneLoopTarget(createdTrigger.LoopTarget)
	changedTriggerTarget.LoopTarget.WorkspaceID = "ws-other"
	if _, err := manager.UpdateTrigger(h.ctx, changedTriggerTarget, nil); !errors.Is(err, ErrTargetIdentityImmutable) {
		t.Fatalf("manager.UpdateTrigger(changed target) error = %v, want ErrTargetIdentityImmutable", err)
	}

	validateCalls := starter.validateCallSnapshot()
	if got, want := len(validateCalls), 3; got != want {
		t.Fatalf("len(ValidateLoopTarget calls) = %d, want %d", got, want)
	}
	if got, want := validateCalls[0].Kind, LoopStartKindSchedule; got != want {
		t.Fatalf("ValidateLoopTarget(job create).Kind = %q, want %q", got, want)
	}
	if got, want := validateCalls[1].LoopName, "triage"; got != want {
		t.Fatalf("ValidateLoopTarget(job update).LoopName = %q, want %q", got, want)
	}
	if got, want := validateCalls[2].Kind, LoopStartKindWebhook; got != want {
		t.Fatalf("ValidateLoopTarget(trigger create).Kind = %q, want %q", got, want)
	}
	if got, want := validateCalls[2].InputMapping["title"], "{{ .trigger.payload.title }}"; got != want {
		t.Fatalf("ValidateLoopTarget(trigger create).InputMapping[title] = %q, want %q", got, want)
	}

	starter.validateErr = errors.New("start_kind_not_allowed: schedule not declared")
	rejectedJob := testJob(AutomationScopeWorkspace, "crud-loop-rejected", h.workspace.ID)
	rejectedJob.AgentName = ""
	rejectedJob.Prompt = ""
	rejectedJob.TargetKind = TargetKindLoop
	rejectedJob.LoopTarget = &LoopTarget{
		WorkspaceID: h.workspace.ID,
		LoopName:    "ops",
	}
	if _, err := manager.CreateJob(h.ctx, rejectedJob); err == nil {
		t.Fatal("manager.CreateJob(rejected loop target) error = nil, want validation error")
	} else if !strings.Contains(err.Error(), "start_kind_not_allowed") {
		t.Fatalf("manager.CreateJob(rejected loop target) error = %v, want start_kind_not_allowed", err)
	}
}

func TestManagerDynamicTriggerCRUDWebhookAndExtensionFire(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	webhookTrigger := Trigger{
		ID:           "trigger-webhook-crud",
		Scope:        AutomationScopeWorkspace,
		Name:         "webhook-crud",
		AgentName:    "reviewer",
		WorkspaceID:  h.workspace.ID,
		Prompt:       `Review payload {{ index .Data "payload" }}`,
		Event:        "webhook",
		Filter:       map[string]string{"data.payload": "deploy"},
		Enabled:      true,
		Retry:        DefaultRetryConfig(),
		FireLimit:    DefaultFireLimitConfig(),
		Source:       JobSourceDynamic,
		EndpointSlug: "deploy-review",
	}
	createdWebhook, err := manager.CreateTrigger(h.ctx, webhookTrigger, webhookSecretWrite("secret-v1"))
	if err != nil {
		t.Fatalf("manager.CreateTrigger(webhook) error = %v", err)
	}
	if createdWebhook.WebhookID == "" {
		t.Fatal("manager.CreateTrigger(webhook) webhook_id = empty, want non-empty")
	}

	gotWebhook, err := manager.GetTrigger(h.ctx, createdWebhook.ID)
	if err != nil {
		t.Fatalf("manager.GetTrigger(webhook) error = %v", err)
	}
	if gotWebhook.EndpointSlug != "deploy-review" {
		t.Fatalf("manager.GetTrigger(webhook) endpoint_slug = %q, want deploy-review", gotWebhook.EndpointSlug)
	}

	listedTriggers, err := manager.ListTriggers(h.ctx, TriggerListQuery{
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: h.workspace.ID,
		Event:       "webhook",
	})
	if err != nil {
		t.Fatalf("manager.ListTriggers(webhook) error = %v", err)
	}
	if got, want := len(listedTriggers.Triggers), 1; got != want {
		t.Fatalf("len(manager.ListTriggers(webhook)) = %d, want %d", got, want)
	}

	storedSecret := h.resolveWebhookSecret(t, createdWebhook.WebhookSecretRef)
	if got, want := storedSecret, "secret-v1"; got != want {
		t.Fatalf("stored webhook secret = %q, want %q", got, want)
	}

	webhookUpdate := createdWebhook
	webhookUpdate.Prompt = `Updated payload {{ index .Data "payload" }}`
	webhookUpdate.EndpointSlug = "deploy-review-updated"
	updatedWebhook, err := manager.UpdateTrigger(h.ctx, webhookUpdate, webhookSecretWritePtr("secret-v2"))
	if err != nil {
		t.Fatalf("manager.UpdateTrigger(webhook) error = %v", err)
	}
	if got, want := updatedWebhook.EndpointSlug, "deploy-review-updated"; got != want {
		t.Fatalf("updated webhook endpoint_slug = %q, want %q", got, want)
	}
	storedSecret = h.resolveWebhookSecret(t, updatedWebhook.WebhookSecretRef)
	if got, want := storedSecret, "secret-v2"; got != want {
		t.Fatalf("updated webhook secret = %q, want %q", got, want)
	}

	endpoint, err := FormatWebhookEndpoint(updatedWebhook.EndpointSlug, updatedWebhook.WebhookID)
	if err != nil {
		t.Fatalf("FormatWebhookEndpoint() error = %v", err)
	}
	timestamp := time.Now().UTC()
	payload := []byte(`{"payload":"deploy"}`)
	signature, err := SignWebhookPayload("secret-v2", timestamp, payload)
	if err != nil {
		t.Fatalf("SignWebhookPayload() error = %v", err)
	}
	webhookResult, err := manager.HandleWebhook(h.ctx, WebhookRequest{
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: h.workspace.ID,
		Endpoint:    endpoint,
		DeliveryID:  "delivery-updated",
		Timestamp:   timestamp,
		Signature:   signature,
		Payload:     payload,
		Data: map[string]any{
			"payload": "deploy",
		},
	})
	if err != nil {
		t.Fatalf("manager.HandleWebhook() error = %v", err)
	}
	if got, want := webhookResult.Matched, 1; got != want {
		t.Fatalf("webhook result.Matched = %d, want %d", got, want)
	}

	if err := manager.DeleteTrigger(h.ctx, updatedWebhook.ID); err != nil {
		t.Fatalf("manager.DeleteTrigger(webhook) error = %v", err)
	}
	h.assertWebhookSecretMissing(t, updatedWebhook.WebhookSecretRef)

	extensionTrigger := Trigger{
		ID:          "trigger-extension-crud",
		Scope:       AutomationScopeWorkspace,
		Name:        "extension-crud",
		AgentName:   "reviewer",
		WorkspaceID: h.workspace.ID,
		Prompt:      `Review repo {{ index .Data "repo" }}`,
		Event:       "ext.github.push",
		Filter:      map[string]string{"data.repo": "acme/api"},
		Enabled:     true,
		Retry:       DefaultRetryConfig(),
		FireLimit:   DefaultFireLimitConfig(),
		Source:      JobSourceDynamic,
	}
	createdExtension, err := manager.CreateTrigger(h.ctx, extensionTrigger, WebhookSecretWrite{})
	if err != nil {
		t.Fatalf("manager.CreateTrigger(extension) error = %v", err)
	}

	fireResult, err := manager.FireExtensionTrigger(h.ctx, ExtensionTriggerRequest{
		Event:       "ext.github.push",
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: h.workspace.ID,
		Payload: map[string]any{
			"repo": "acme/api",
		},
	})
	if err != nil {
		t.Fatalf("manager.FireExtensionTrigger() error = %v", err)
	}
	if got, want := fireResult.Matched, 1; got != want {
		t.Fatalf("extension fire result.Matched = %d, want %d", got, want)
	}
	if got, want := len(fireResult.Runs), 1; got != want {
		t.Fatalf("len(extension fire result.Runs) = %d, want %d", got, want)
	}

	extensionRuns, err := manager.ListRuns(h.ctx, RunQuery{TriggerID: createdExtension.ID})
	if err != nil {
		t.Fatalf("manager.ListRuns(extension) error = %v", err)
	}
	if got, want := len(extensionRuns), 1; got != want {
		t.Fatalf("len(manager.ListRuns(extension)) = %d, want %d", got, want)
	}
}

func TestManagerCRUDRejectsNilContextAndReadOnlyDefinitions(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	}, WithHooks(hookspkg.NewHooks()))
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	configJob := testJob(AutomationScopeWorkspace, "readonly-job", h.workspace.ID)
	configJob.Source = JobSourceConfig
	if _, err := h.db.CreateJob(h.ctx, configJob); err != nil {
		t.Fatalf("CreateJob(config) error = %v", err)
	}

	configTrigger := Trigger{
		ID:          "trigger-readonly",
		Scope:       AutomationScopeWorkspace,
		Name:        "readonly-trigger",
		AgentName:   "reviewer",
		WorkspaceID: h.workspace.ID,
		Prompt:      `Review {{ index .Data "payload" }}`,
		Event:       "ext.github.push",
		Enabled:     true,
		Retry:       DefaultRetryConfig(),
		FireLimit:   DefaultFireLimitConfig(),
		Source:      JobSourceConfig,
	}
	if _, err := h.db.CreateTrigger(h.ctx, configTrigger); err != nil {
		t.Fatalf("CreateTrigger(config) error = %v", err)
	}

	nilCtx := nilContextForTests()
	assertContextError := func(name string, err error, want string) {
		t.Helper()
		if err == nil {
			t.Fatalf("%s error = nil, want %q", name, want)
		}
		if err.Error() != want {
			t.Fatalf("%s error = %q, want %q", name, err.Error(), want)
		}
	}

	_, err := manager.CreateJob(nilCtx, testJob(AutomationScopeWorkspace, "nil-job", h.workspace.ID))
	assertContextError("manager.CreateJob(nil)", err, "automation: create job context is required")
	_, err = manager.ListJobs(nilCtx, JobListQuery{})
	assertContextError("manager.ListJobs(nil)", err, "automation: list jobs context is required")
	_, err = manager.GetJob(nilCtx, configJob.ID)
	assertContextError("manager.GetJob(nil)", err, "automation: get job context is required")
	_, err = manager.UpdateJob(nilCtx, configJob)
	assertContextError("manager.UpdateJob(nil)", err, "automation: update job context is required")
	err = manager.DeleteJob(nilCtx, configJob.ID)
	assertContextError("manager.DeleteJob(nil)", err, "automation: delete job context is required")
	_, err = manager.TriggerJob(nilCtx, configJob.ID)
	assertContextError("manager.TriggerJob(nil)", err, "automation: trigger job context is required")
	_, err = manager.SetJobEnabled(nilCtx, configJob.ID, false)
	assertContextError("manager.SetJobEnabled(nil)", err, "automation: set job enabled context is required")

	_, err = manager.CreateTrigger(nilCtx, configTrigger, webhookSecretWrite("secret"))
	assertContextError("manager.CreateTrigger(nil)", err, "automation: create trigger context is required")
	_, err = manager.ListTriggers(nilCtx, TriggerListQuery{})
	assertContextError("manager.ListTriggers(nil)", err, "automation: list triggers context is required")
	_, err = manager.GetTrigger(nilCtx, configTrigger.ID)
	assertContextError("manager.GetTrigger(nil)", err, "automation: get trigger context is required")
	_, err = manager.UpdateTrigger(nilCtx, configTrigger, nil)
	assertContextError("manager.UpdateTrigger(nil)", err, "automation: update trigger context is required")
	err = manager.DeleteTrigger(nilCtx, configTrigger.ID)
	assertContextError("manager.DeleteTrigger(nil)", err, "automation: delete trigger context is required")
	_, err = manager.SetTriggerEnabled(nilCtx, configTrigger.ID, false)
	assertContextError("manager.SetTriggerEnabled(nil)", err, "automation: set trigger enabled context is required")

	_, err = manager.ListRuns(nilCtx, RunQuery{})
	assertContextError("manager.ListRuns(nil)", err, "automation: list runs context is required")
	_, err = manager.GetRun(nilCtx, "run-id")
	assertContextError("manager.GetRun(nil)", err, "automation: get run context is required")
	_, err = manager.Status(nilCtx)
	assertContextError("manager.Status(nil)", err, "automation: status context is required")

	if _, err := manager.CreateJob(h.ctx, configJob); !errors.Is(err, ErrDefinitionReadOnly) {
		t.Fatalf("manager.CreateJob(config) error = %v, want ErrDefinitionReadOnly", err)
	}
	if _, err := manager.UpdateJob(h.ctx, configJob); !errors.Is(err, ErrDefinitionReadOnly) {
		t.Fatalf("manager.UpdateJob(config) error = %v, want ErrDefinitionReadOnly", err)
	}
	if err := manager.DeleteJob(h.ctx, configJob.ID); !errors.Is(err, ErrDefinitionReadOnly) {
		t.Fatalf("manager.DeleteJob(config) error = %v, want ErrDefinitionReadOnly", err)
	}

	if _, err := manager.CreateTrigger(
		h.ctx,
		configTrigger,
		webhookSecretWrite("secret"),
	); !errors.Is(
		err,
		ErrDefinitionReadOnly,
	) {
		t.Fatalf("manager.CreateTrigger(config) error = %v, want ErrDefinitionReadOnly", err)
	}
	if _, err := manager.UpdateTrigger(h.ctx, configTrigger, nil); !errors.Is(err, ErrDefinitionReadOnly) {
		t.Fatalf("manager.UpdateTrigger(config) error = %v, want ErrDefinitionReadOnly", err)
	}
	if err := manager.DeleteTrigger(h.ctx, configTrigger.ID); !errors.Is(err, ErrDefinitionReadOnly) {
		t.Fatalf("manager.DeleteTrigger(config) error = %v, want ErrDefinitionReadOnly", err)
	}

	webhookTrigger := Trigger{
		ID:           "trigger-webhook-missing-secret",
		Scope:        AutomationScopeWorkspace,
		Name:         "missing-secret",
		AgentName:    "reviewer",
		WorkspaceID:  h.workspace.ID,
		Prompt:       `Review {{ index .Data "payload" }}`,
		Event:        "webhook",
		Enabled:      true,
		Retry:        DefaultRetryConfig(),
		FireLimit:    DefaultFireLimitConfig(),
		Source:       JobSourceDynamic,
		EndpointSlug: "missing-secret",
	}
	if _, err := manager.CreateTrigger(
		h.ctx,
		webhookTrigger,
		WebhookSecretWrite{},
	); !errors.Is(
		err,
		ErrWebhookSecretRequired,
	) {
		t.Fatalf("manager.CreateTrigger(webhook missing secret) error = %v, want ErrWebhookSecretRequired", err)
	}
}

func TestManagerWebhookSecretHelpers(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})

	webhookTrigger := Trigger{
		ID:               "trigger-secret-helpers",
		Scope:            AutomationScopeWorkspace,
		Name:             "secret-helpers",
		AgentName:        "reviewer",
		WorkspaceID:      h.workspace.ID,
		Prompt:           `Review {{ index .Data "payload" }}`,
		Event:            "webhook",
		Enabled:          true,
		Retry:            DefaultRetryConfig(),
		FireLimit:        DefaultFireLimitConfig(),
		Source:           JobSourceDynamic,
		EndpointSlug:     "secret-helpers",
		WebhookID:        "wbh_secret-helpers",
		WebhookSecretRef: defaultAutomationWebhookSecretRef("trigger-secret-helpers"),
	}
	if _, err := h.db.CreateTrigger(h.ctx, webhookTrigger); err != nil {
		t.Fatalf("CreateTrigger(webhook) error = %v", err)
	}
	h.putWebhookSecret(t, webhookTrigger.WebhookSecretRef, "stored-secret")

	currentSecret, err := manager.currentWebhookSecret(h.ctx, webhookTrigger)
	if err != nil {
		t.Fatalf("currentWebhookSecret() error = %v", err)
	}
	if got, want := currentSecret, "stored-secret"; got != want {
		t.Fatalf("currentWebhookSecret() = %q, want %q", got, want)
	}

	if err := manager.restoreWebhookSecret(h.ctx, webhookTrigger, "restored-secret"); err != nil {
		t.Fatalf("restoreWebhookSecret(set) error = %v", err)
	}
	restored := h.resolveWebhookSecret(t, webhookTrigger.WebhookSecretRef)
	if got, want := restored, "restored-secret"; got != want {
		t.Fatalf("restored webhook secret = %q, want %q", got, want)
	}

	if err := manager.restoreWebhookSecret(h.ctx, webhookTrigger, ""); err != nil {
		t.Fatalf("restoreWebhookSecret(delete) error = %v", err)
	}
	h.assertWebhookSecretMissing(t, webhookTrigger.WebhookSecretRef)

	(&triggerSessionObserver{}).OnAgentEvent(h.ctx, "sess-ignored", acp.AgentEvent{})
}

func TestManagerUpdateTriggerTransitionsWebhookSecretLifecycle(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	trigger := Trigger{
		ID:          "trigger-transition",
		Scope:       AutomationScopeWorkspace,
		Name:        "transition-trigger",
		AgentName:   "reviewer",
		WorkspaceID: h.workspace.ID,
		Prompt:      `Review {{ index .Data "repo" }}`,
		Event:       "ext.github.push",
		Filter:      map[string]string{"data.repo": "acme/api"},
		Enabled:     true,
		Retry:       DefaultRetryConfig(),
		FireLimit:   DefaultFireLimitConfig(),
		Source:      JobSourceDynamic,
	}
	created, err := manager.CreateTrigger(h.ctx, trigger, WebhookSecretWrite{})
	if err != nil {
		t.Fatalf("manager.CreateTrigger() error = %v", err)
	}

	missingSecretUpdate := created
	missingSecretUpdate.Event = "webhook"
	missingSecretUpdate.Filter = map[string]string{"data.payload": "deploy"}
	missingSecretUpdate.EndpointSlug = "transition-trigger"
	if _, err := manager.UpdateTrigger(h.ctx, missingSecretUpdate, nil); !errors.Is(err, ErrWebhookSecretRequired) {
		t.Fatalf("manager.UpdateTrigger(webhook without secret) error = %v, want ErrWebhookSecretRequired", err)
	}

	stillExtension, err := manager.GetTrigger(h.ctx, created.ID)
	if err != nil {
		t.Fatalf("manager.GetTrigger(after failed update) error = %v", err)
	}
	if got, want := stillExtension.Event, "ext.github.push"; got != want {
		t.Fatalf("trigger event after failed update = %q, want %q", got, want)
	}

	webhookUpdate := created
	webhookUpdate.Event = "webhook"
	webhookUpdate.Filter = map[string]string{"data.payload": "deploy"}
	webhookUpdate.EndpointSlug = "transition-trigger"
	webhookUpdate.Prompt = `Webhook {{ index .Data "payload" }}`
	updatedWebhook, err := manager.UpdateTrigger(h.ctx, webhookUpdate, webhookSecretWritePtr("transition-secret"))
	if err != nil {
		t.Fatalf("manager.UpdateTrigger(webhook) error = %v", err)
	}
	if updatedWebhook.WebhookID == "" {
		t.Fatal("updated webhook trigger webhook_id = empty, want non-empty")
	}

	secret := h.resolveWebhookSecret(t, updatedWebhook.WebhookSecretRef)
	if got, want := secret, "transition-secret"; got != want {
		t.Fatalf("stored webhook secret = %q, want %q", got, want)
	}

	backToExtension := updatedWebhook
	backToExtension.Event = "ext.github.release"
	backToExtension.Filter = map[string]string{"data.repo": "acme/api"}
	backToExtension.EndpointSlug = ""
	backToExtension.WebhookID = ""
	updatedExtension, err := manager.UpdateTrigger(h.ctx, backToExtension, nil)
	if err != nil {
		t.Fatalf("manager.UpdateTrigger(back to extension) error = %v", err)
	}
	if got, want := updatedExtension.Event, "ext.github.release"; got != want {
		t.Fatalf("updated extension trigger event = %q, want %q", got, want)
	}
	h.assertWebhookSecretMissing(t, updatedWebhook.WebhookSecretRef)
}

func TestResolveConfigDefinitionsWrapValidationErrors(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})

	invalidJob := aghconfig.AutomationJob{
		Scope:   AutomationScopeGlobal,
		Name:    "invalid-job",
		Enabled: true,
		Schedule: ScheduleSpec{
			Mode: ScheduleModeEvery,
		},
	}
	if _, err := manager.resolveConfigJob(h.ctx, invalidJob); err == nil ||
		!strings.Contains(err.Error(), `automation: resolve config job "invalid-job":`) ||
		!strings.Contains(err.Error(), "job.agent_name is required") {
		t.Fatalf("resolveConfigJob(invalid) error = %v, want wrapped validation context", err)
	}

	invalidTrigger := aghconfig.AutomationTrigger{
		Scope:   AutomationScopeGlobal,
		Name:    "invalid-trigger",
		Enabled: true,
	}
	if _, err := manager.resolveConfigTrigger(h.ctx, invalidTrigger); err == nil ||
		!strings.Contains(err.Error(), `automation: resolve config trigger "invalid-trigger":`) ||
		!strings.Contains(err.Error(), "trigger.agent_name is required") {
		t.Fatalf("resolveConfigTrigger(invalid) error = %v, want wrapped validation context", err)
	}
}

func TestManagerDynamicCRUDHandlesBlankSourcesAndDuplicateNames(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	jobA := testJob(AutomationScopeWorkspace, "duplicate-job-a", h.workspace.ID)
	jobA.Source = ""
	jobA.Name = "duplicate-job"
	createdJobA, err := manager.CreateJob(h.ctx, jobA)
	if err != nil {
		t.Fatalf("manager.CreateJob(jobA) error = %v", err)
	}
	if got, want := createdJobA.Source, JobSourceDynamic; got != want {
		t.Fatalf("created job source = %q, want %q", got, want)
	}

	jobB := testJob(AutomationScopeWorkspace, "duplicate-job-b", h.workspace.ID)
	jobB.Name = "duplicate-job"
	if _, err := manager.CreateJob(h.ctx, jobB); !errors.Is(err, ErrJobNameTaken) {
		t.Fatalf("manager.CreateJob(duplicate) error = %v, want ErrJobNameTaken", err)
	}

	jobC := testJob(AutomationScopeWorkspace, "duplicate-job-c", h.workspace.ID)
	createdJobC, err := manager.CreateJob(h.ctx, jobC)
	if err != nil {
		t.Fatalf("manager.CreateJob(jobC) error = %v", err)
	}
	jobCUpdate := createdJobC
	jobCUpdate.Name = "duplicate-job"
	if _, err := manager.UpdateJob(h.ctx, jobCUpdate); !errors.Is(err, ErrJobNameTaken) {
		t.Fatalf("manager.UpdateJob(duplicate) error = %v, want ErrJobNameTaken", err)
	}

	triggerA := Trigger{
		ID:          "trigger-duplicate-a",
		Scope:       AutomationScopeWorkspace,
		Name:        "duplicate-trigger",
		AgentName:   "reviewer",
		WorkspaceID: h.workspace.ID,
		Prompt:      `Review {{ index .Data "repo" }}`,
		Event:       "ext.github.push",
		Filter:      map[string]string{"data.repo": "acme/api"},
		Enabled:     true,
		Retry:       DefaultRetryConfig(),
		FireLimit:   DefaultFireLimitConfig(),
	}
	createdTriggerA, err := manager.CreateTrigger(h.ctx, triggerA, WebhookSecretWrite{})
	if err != nil {
		t.Fatalf("manager.CreateTrigger(triggerA) error = %v", err)
	}
	if got, want := createdTriggerA.Source, JobSourceDynamic; got != want {
		t.Fatalf("created trigger source = %q, want %q", got, want)
	}

	triggerB := createdTriggerA
	triggerB.ID = "trigger-duplicate-b"
	if _, err := manager.CreateTrigger(h.ctx, triggerB, WebhookSecretWrite{}); !errors.Is(err, ErrTriggerNameTaken) {
		t.Fatalf("manager.CreateTrigger(duplicate) error = %v, want ErrTriggerNameTaken", err)
	}

	triggerC := createdTriggerA
	triggerC.ID = "trigger-duplicate-c"
	triggerC.Name = "unique-trigger"
	triggerC.Filter = map[string]string{"data.repo": "acme/other"}
	createdTriggerC, err := manager.CreateTrigger(h.ctx, triggerC, WebhookSecretWrite{})
	if err != nil {
		t.Fatalf("manager.CreateTrigger(triggerC) error = %v", err)
	}
	triggerCUpdate := createdTriggerC
	triggerCUpdate.Name = "duplicate-trigger"
	if _, err := manager.UpdateTrigger(h.ctx, triggerCUpdate, nil); !errors.Is(err, ErrTriggerNameTaken) {
		t.Fatalf("manager.UpdateTrigger(duplicate) error = %v, want ErrTriggerNameTaken", err)
	}
}

func TestManagerCRUDBeforeStartUsesPersistenceWithoutRuntime(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})

	job := testJob(AutomationScopeWorkspace, "prestart-job", h.workspace.ID)
	job.Source = ""
	createdJob, err := manager.CreateJob(h.ctx, job)
	if err != nil {
		t.Fatalf("manager.CreateJob(pre-start) error = %v", err)
	}
	createdJob.Prompt = "Updated before start"
	updatedJob, err := manager.UpdateJob(h.ctx, createdJob)
	if err != nil {
		t.Fatalf("manager.UpdateJob(pre-start) error = %v", err)
	}
	if got, want := updatedJob.Prompt, "Updated before start"; got != want {
		t.Fatalf("updated pre-start job prompt = %q, want %q", got, want)
	}
	if err := manager.DeleteJob(h.ctx, updatedJob.ID); err != nil {
		t.Fatalf("manager.DeleteJob(pre-start) error = %v", err)
	}

	trigger := Trigger{
		ID:          "trigger-prestart",
		Scope:       AutomationScopeWorkspace,
		Name:        "prestart-trigger",
		AgentName:   "reviewer",
		WorkspaceID: h.workspace.ID,
		Prompt:      `Review {{ index .Data "repo" }}`,
		Event:       "ext.github.push",
		Filter:      map[string]string{"data.repo": "acme/api"},
		Enabled:     true,
		Retry:       DefaultRetryConfig(),
		FireLimit:   DefaultFireLimitConfig(),
	}
	createdTrigger, err := manager.CreateTrigger(h.ctx, trigger, WebhookSecretWrite{})
	if err != nil {
		t.Fatalf("manager.CreateTrigger(pre-start) error = %v", err)
	}
	createdTrigger.Prompt = `Updated {{ index .Data "repo" }}`
	updatedTrigger, err := manager.UpdateTrigger(h.ctx, createdTrigger, nil)
	if err != nil {
		t.Fatalf("manager.UpdateTrigger(pre-start) error = %v", err)
	}
	if got, want := updatedTrigger.Prompt, `Updated {{ index .Data "repo" }}`; got != want {
		t.Fatalf("updated pre-start trigger prompt = %q, want %q", got, want)
	}
	if err := manager.DeleteTrigger(h.ctx, updatedTrigger.ID); err != nil {
		t.Fatalf("manager.DeleteTrigger(pre-start) error = %v", err)
	}

	if _, err := manager.HandleWebhook(h.ctx, WebhookRequest{}); !errors.Is(err, ErrManagerNotRunning) {
		t.Fatalf("manager.HandleWebhook(pre-start) error = %v, want ErrManagerNotRunning", err)
	}
	if _, err := manager.FireExtensionTrigger(h.ctx, ExtensionTriggerRequest{
		Event:       "ext.github.push",
		Scope:       AutomationScopeWorkspace,
		WorkspaceID: h.workspace.ID,
	}); !errors.Is(err, ErrManagerNotRunning) {
		t.Fatalf("manager.FireExtensionTrigger(pre-start) error = %v, want ErrManagerNotRunning", err)
	}
}

func TestManagerStartWrapsSyncConfigDefinitionFailure(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
		Jobs: []aghconfig.AutomationJob{
			managerConfigJob(AutomationScopeWorkspace, "missing-workspace", "", ScheduleSpec{
				Mode:     ScheduleModeEvery,
				Interval: "1h",
			}),
		},
	})

	err := manager.Start(h.ctx)
	if err == nil {
		t.Fatal("manager.Start() error = nil, want non-nil")
	}
	if got := err.Error(); !containsAll(got, "automation: sync config definitions", "workspace reference is required") {
		t.Fatalf("manager.Start() error = %q, want wrapped sync context", got)
	}
}

func TestManagerTriggerJobReturnsStoredRunOnDispatchFailure(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	h.sessions = newManagerSessionStub(sessionAttemptPlan{promptErr: errors.New("prompt failed")})
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})
	if err := manager.Start(h.ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})

	job, err := manager.CreateJob(h.ctx, testJob(AutomationScopeWorkspace, "failing-trigger-job", h.workspace.ID))
	if err != nil {
		t.Fatalf("manager.CreateJob() error = %v", err)
	}

	if _, err := manager.TriggerJob(h.ctx, "job-missing"); !errors.Is(err, ErrJobNotFound) {
		t.Fatalf("manager.TriggerJob(missing) error = %v, want ErrJobNotFound", err)
	}

	run, err := manager.TriggerJob(h.ctx, job.ID)
	if err == nil {
		t.Fatal("manager.TriggerJob(failing) error = nil, want prompt failure")
	}
	if !strings.Contains(err.Error(), "prompt failed") {
		t.Fatalf("manager.TriggerJob(failing) error = %v, want prompt failure", err)
	}
	if got, want := run.JobID, job.ID; got != want {
		t.Fatalf("failed run job_id = %q, want %q", got, want)
	}
}

func TestManagerSetEnabledBeforeStartUsesPersistenceOnly(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})

	job, err := manager.CreateJob(h.ctx, testJob(AutomationScopeWorkspace, "prestart-enable-job", h.workspace.ID))
	if err != nil {
		t.Fatalf("manager.CreateJob() error = %v", err)
	}
	trigger, err := manager.CreateTrigger(h.ctx, Trigger{
		ID:          "trigger-prestart-enable",
		Scope:       AutomationScopeWorkspace,
		Name:        "prestart-enable-trigger",
		AgentName:   "reviewer",
		WorkspaceID: h.workspace.ID,
		Prompt:      `Review {{ index .Data "repo" }}`,
		Event:       "ext.github.push",
		Filter:      map[string]string{"data.repo": "acme/api"},
		Enabled:     true,
		Retry:       DefaultRetryConfig(),
		FireLimit:   DefaultFireLimitConfig(),
	}, WebhookSecretWrite{})
	if err != nil {
		t.Fatalf("manager.CreateTrigger() error = %v", err)
	}

	disabledJob, err := manager.SetJobEnabled(h.ctx, job.ID, false)
	if err != nil {
		t.Fatalf("manager.SetJobEnabled(false) error = %v", err)
	}
	if disabledJob.Enabled {
		t.Fatal("disabled job enabled = true, want false")
	}
	disabledTrigger, err := manager.SetTriggerEnabled(h.ctx, trigger.ID, false)
	if err != nil {
		t.Fatalf("manager.SetTriggerEnabled(false) error = %v", err)
	}
	if disabledTrigger.Enabled {
		t.Fatal("disabled trigger enabled = true, want false")
	}

	reEnabledJob, err := manager.SetJobEnabled(h.ctx, job.ID, true)
	if err != nil {
		t.Fatalf("manager.SetJobEnabled(true) error = %v", err)
	}
	if !reEnabledJob.Enabled {
		t.Fatal("re-enabled job enabled = false, want true")
	}
	reEnabledTrigger, err := manager.SetTriggerEnabled(h.ctx, trigger.ID, true)
	if err != nil {
		t.Fatalf("manager.SetTriggerEnabled(true) error = %v", err)
	}
	if !reEnabledTrigger.Enabled {
		t.Fatal("re-enabled trigger enabled = false, want true")
	}
}

func TestManagerHelperRollbackAndComparisonCoverage(t *testing.T) {
	t.Parallel()

	h := newManagerHarness(t)
	manager := h.newManager(t, aghconfig.AutomationConfig{
		Enabled:           true,
		Timezone:          DefaultTimezone,
		MaxConcurrentJobs: DefaultMaxConcurrentJobs,
		DefaultFireLimit:  DefaultFireLimitConfig(),
	})

	configJob := managerConfigJob(AutomationScopeWorkspace, "rollback-job", h.workspaceRoot, ScheduleSpec{
		Mode:     ScheduleModeEvery,
		Interval: "1h",
	})
	resolvedConfigJob, err := manager.resolveConfigJob(h.ctx, configJob)
	if err != nil {
		t.Fatalf("resolveConfigJob() error = %v", err)
	}
	if _, err := h.db.CreateJob(h.ctx, resolvedConfigJob); err != nil {
		t.Fatalf("CreateJob(config) error = %v", err)
	}
	if err := manager.rollbackJobEnabled(h.ctx, resolvedConfigJob, false); err != nil {
		t.Fatalf("rollbackJobEnabled(config) error = %v", err)
	}
	if _, err := h.db.GetJobEnabledOverlay(h.ctx, resolvedConfigJob.ID); err != nil {
		t.Fatalf("GetJobEnabledOverlay() error = %v", err)
	}

	dynamicJob, err := h.db.CreateJob(h.ctx, testJob(AutomationScopeWorkspace, "rollback-dynamic-job", h.workspace.ID))
	if err != nil {
		t.Fatalf("CreateJob(dynamic) error = %v", err)
	}
	if err := manager.rollbackJobEnabled(h.ctx, dynamicJob, false); err != nil {
		t.Fatalf("rollbackJobEnabled(dynamic) error = %v", err)
	}
	storedDynamicJob, err := h.db.GetJob(h.ctx, dynamicJob.ID)
	if err != nil {
		t.Fatalf("GetJob(dynamic) error = %v", err)
	}
	if storedDynamicJob.Enabled {
		t.Fatal("stored dynamic job enabled = true, want false after rollback helper")
	}

	configTrigger := managerConfigTrigger(
		AutomationScopeWorkspace,
		"rollback-trigger",
		h.workspaceRoot,
		"session.stopped",
	)
	resolvedConfigTrigger, err := manager.resolveConfigTrigger(h.ctx, configTrigger)
	if err != nil {
		t.Fatalf("resolveConfigTrigger() error = %v", err)
	}
	if _, err := h.db.CreateTrigger(h.ctx, resolvedConfigTrigger); err != nil {
		t.Fatalf("CreateTrigger(config) error = %v", err)
	}
	if err := manager.rollbackTriggerEnabled(h.ctx, resolvedConfigTrigger, false); err != nil {
		t.Fatalf("rollbackTriggerEnabled(config) error = %v", err)
	}
	if _, err := h.db.GetTriggerEnabledOverlay(h.ctx, resolvedConfigTrigger.ID); err != nil {
		t.Fatalf("GetTriggerEnabledOverlay() error = %v", err)
	}

	dynamicTrigger, err := h.db.CreateTrigger(h.ctx, Trigger{
		ID:          "trigger-rollback-dynamic",
		Scope:       AutomationScopeWorkspace,
		Name:        "rollback-dynamic-trigger",
		AgentName:   "reviewer",
		WorkspaceID: h.workspace.ID,
		Prompt:      `Review session {{ index .Data "session_id" }}`,
		Event:       "session.stopped",
		Enabled:     true,
		Retry:       DefaultRetryConfig(),
		FireLimit:   DefaultFireLimitConfig(),
		Source:      JobSourceDynamic,
	})
	if err != nil {
		t.Fatalf("CreateTrigger(dynamic) error = %v", err)
	}
	if err := manager.rollbackTriggerEnabled(h.ctx, dynamicTrigger, false); err != nil {
		t.Fatalf("rollbackTriggerEnabled(dynamic) error = %v", err)
	}
	storedDynamicTrigger, err := h.db.GetTrigger(h.ctx, dynamicTrigger.ID)
	if err != nil {
		t.Fatalf("GetTrigger(dynamic) error = %v", err)
	}
	if storedDynamicTrigger.Enabled {
		t.Fatal("stored dynamic trigger enabled = true, want false after rollback helper")
	}

	if sameSchedule(nil, nil) != true {
		t.Fatal("sameSchedule(nil, nil) = false, want true")
	}
	if sameSchedule(&ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1h"}, nil) {
		t.Fatal("sameSchedule(non-nil, nil) = true, want false")
	}
	if !sameSchedule(
		&ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1h"},
		&ScheduleSpec{Mode: ScheduleModeEvery, Interval: "1h"},
	) {
		t.Fatal("sameSchedule(equal) = false, want true")
	}
	if !sameStringMap(map[string]string{"a": "b"}, map[string]string{"a": "b"}) {
		t.Fatal("sameStringMap(equal) = false, want true")
	}
	if sameStringMap(map[string]string{"a": "b"}, map[string]string{"a": "c"}) {
		t.Fatal("sameStringMap(different) = true, want false")
	}

	managerSessionObserver{}.OnAgentEvent(h.ctx, "sess-ignored", acp.AgentEvent{})
}

func TestManagerSortHelpersKeepDeterministicOrder(t *testing.T) {
	t.Parallel()

	jobs := []Job{
		{ID: "job-b"},
		{ID: "job-a"},
	}
	sortJobs(jobs)
	if jobs[0].ID != "job-a" || jobs[1].ID != "job-b" {
		t.Fatalf("sortJobs() produced %q, %q; want job-a, job-b", jobs[0].ID, jobs[1].ID)
	}

	triggers := []Trigger{
		{ID: "trigger-b"},
		{ID: "trigger-a"},
	}
	sortTriggers(triggers)
	if triggers[0].ID != "trigger-a" || triggers[1].ID != "trigger-b" {
		t.Fatalf("sortTriggers() produced %q, %q; want trigger-a, trigger-b", triggers[0].ID, triggers[1].ID)
	}
}

type managerHarness struct {
	ctx           context.Context
	homePaths     aghconfig.HomePaths
	db            *globaldb.GlobalDB
	resolver      workspacepkg.RuntimeResolver
	workspaceRoot string
	workspace     workspacepkg.ResolvedWorkspace
	sessions      *managerSessionStub
}

func newManagerHarness(t *testing.T) *managerHarness {
	t.Helper()

	ctx := testutil.Context(t)
	homePaths, err := aghconfig.ResolveHomePathsFrom(t.TempDir())
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := aghconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	db, err := globaldb.OpenGlobalDB(ctx, homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(testutil.Context(t)); err != nil {
			t.Fatalf("GlobalDB.Close() error = %v", err)
		}
	})

	resolver, err := workspacepkg.NewResolver(
		db,
		workspacepkg.WithHomePaths(homePaths),
		workspacepkg.WithLogger(discardAutomationLogger()),
		workspacepkg.WithConfigLoader(func(rootDir string) (aghconfig.Config, error) {
			return aghconfig.LoadForHome(homePaths, aghconfig.WithWorkspaceRoot(rootDir))
		}),
	)
	if err != nil {
		t.Fatalf("NewResolver() error = %v", err)
	}

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	if err := os.MkdirAll(workspaceRoot, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", workspaceRoot, err)
	}
	workspace, err := resolver.ResolveOrRegister(ctx, workspaceRoot)
	if err != nil {
		t.Fatalf("ResolveOrRegister(%q) error = %v", workspaceRoot, err)
	}

	return &managerHarness{
		ctx:           ctx,
		homePaths:     homePaths,
		db:            db,
		resolver:      resolver,
		workspaceRoot: workspaceRoot,
		workspace:     workspace,
		sessions:      newManagerSessionStub(),
	}
}

func (h *managerHarness) newManager(t *testing.T, cfg aghconfig.AutomationConfig, opts ...Option) *Manager {
	t.Helper()

	baseOpts := []Option{
		WithStore(h.db),
		WithSessions(h.sessions),
		WithWorkspaceResolver(h.resolver),
		WithConfig(cfg),
		WithLogger(discardAutomationLogger()),
		WithGlobalWorkspacePath(h.homePaths.HomeDir),
		WithWebhookSecretStore(h.webhookSecretStore(t)),
	}
	baseOpts = append(baseOpts, opts...)

	manager, err := New(baseOpts...)
	if err != nil {
		t.Fatalf("automation.New() error = %v", err)
	}
	return manager
}

func (h *managerHarness) webhookSecretStore(t testing.TB) WebhookSecretStore {
	t.Helper()

	lookup := func(key string) (string, bool) {
		value, ok := os.LookupEnv(key)
		return value, ok && strings.TrimSpace(value) != ""
	}
	service, err := vault.NewService(
		h.db,
		vault.NewFileKeyProvider(h.homePaths.HomeDir, lookup),
		vault.WithLookupEnv(lookup),
	)
	if err != nil {
		t.Fatalf("vault.NewService() error = %v", err)
	}
	return service
}

func (h *managerHarness) putWebhookSecret(t testing.TB, ref string, value string) {
	t.Helper()

	if _, err := h.webhookSecretStore(t).PutSecret(h.ctx, ref, "webhook_secret", value); err != nil {
		t.Fatalf("PutSecret(%q) error = %v", ref, err)
	}
}

func (h *managerHarness) resolveWebhookSecret(t testing.TB, ref string) string {
	t.Helper()

	value, err := h.webhookSecretStore(t).ResolveRef(h.ctx, ref)
	if err != nil {
		t.Fatalf("ResolveRef(%q) error = %v", ref, err)
	}
	return value
}

func (h *managerHarness) assertWebhookSecretMissing(t testing.TB, ref string) {
	t.Helper()

	_, err := h.webhookSecretStore(t).ResolveRef(h.ctx, ref)
	if !errors.Is(err, vault.ErrSecretNotFound) && !errors.Is(err, vault.ErrMissingSecret) {
		t.Fatalf("ResolveRef(%q) error = %v, want vault missing secret error", ref, err)
	}
}

type managerSessionStub struct {
	mu       sync.Mutex
	creator  *recordingSessionCreator
	statuses map[string]*session.Info
}

func newManagerSessionStub(plans ...sessionAttemptPlan) *managerSessionStub {
	return &managerSessionStub{
		creator:  newRecordingSessionCreator(plans...),
		statuses: make(map[string]*session.Info),
	}
}

func (s *managerSessionStub) Create(ctx context.Context, opts session.CreateOpts) (*session.Session, error) {
	created, err := s.creator.Create(ctx, opts)
	if err != nil {
		return nil, err
	}
	if created != nil {
		s.mu.Lock()
		s.statuses[created.ID] = created.Info()
		s.mu.Unlock()
	}
	return created, nil
}

func (s *managerSessionStub) Prompt(ctx context.Context, id string, msg string) (<-chan acp.AgentEvent, error) {
	return s.creator.Prompt(ctx, id, msg)
}

func (s *managerSessionStub) StopWithCause(
	ctx context.Context,
	id string,
	cause session.StopCause,
	detail string,
) error {
	if err := s.creator.StopWithCause(ctx, id, cause, detail); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	info, ok := s.statuses[id]
	if !ok || info == nil {
		return nil
	}
	next := *info
	next.State = session.StateStopped
	switch cause {
	case session.CauseCompleted:
		next.StopReason = store.StopCompleted
	case session.CauseFailed:
		next.StopReason = store.StopError
	case session.CauseShutdown:
		next.StopReason = store.StopShutdown
	default:
		next.StopReason = store.StopUserCanceled
	}
	next.StopDetail = strings.TrimSpace(detail)
	s.statuses[id] = &next
	return nil
}

func (s *managerSessionStub) Status(_ context.Context, id string) (*session.Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	info, ok := s.statuses[id]
	if !ok {
		return nil, session.ErrSessionNotFound
	}
	return info, nil
}

func (s *managerSessionStub) setStatus(info *session.Info) {
	if s == nil || info == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statuses[info.ID] = info
}

func (s *managerSessionStub) promptCount() int {
	if s == nil || s.creator == nil {
		return 0
	}
	return len(s.creator.promptCalls())
}

func discardAutomationLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func nilContextForTests() context.Context {
	var ctx context.Context
	return ctx
}

func managerConfigJob(
	scope Scope,
	name string,
	workspace string,
	schedule ScheduleSpec,
) aghconfig.AutomationJob {
	return aghconfig.AutomationJob{
		Scope:     scope,
		Name:      name,
		AgentName: "researcher",
		Workspace: workspace,
		Prompt:    "Summarize the latest state.",
		Schedule:  schedule,
		Enabled:   true,
		Retry:     DefaultRetryConfig(),
		FireLimit: DefaultFireLimitConfig(),
		Source:    JobSourceConfig,
	}
}

func managerConfigTrigger(
	scope Scope,
	name string,
	workspace string,
	event string,
) aghconfig.AutomationTrigger {
	trigger := aghconfig.AutomationTrigger{
		Scope:     scope,
		Name:      name,
		AgentName: "reviewer",
		Workspace: workspace,
		Prompt:    `Review session {{ index .Data "session_id" }}`,
		Event:     event,
		Enabled:   true,
		Retry:     DefaultRetryConfig(),
		FireLimit: DefaultFireLimitConfig(),
		Source:    JobSourceConfig,
	}
	switch event {
	case "session.stopped":
		trigger.Filter = map[string]string{"data.agent_name": "reviewer"}
	case "memory.consolidated":
		trigger.Filter = nil
	}
	return trigger
}

func webhookSecretWrite(value string) WebhookSecretWrite {
	trimmed := strings.TrimSpace(value)
	return WebhookSecretWrite{Value: &trimmed}
}

func webhookSecretWritePtr(value string) *WebhookSecretWrite {
	write := webhookSecretWrite(value)
	return &write
}

func findJobByID(jobs []Job, id string) *Job {
	for idx := range jobs {
		if jobs[idx].ID == id {
			return &jobs[idx]
		}
	}
	return nil
}

func findTriggerByID(triggers []Trigger, id string) *Trigger {
	for idx := range triggers {
		if triggers[idx].ID == id {
			return &triggers[idx]
		}
	}
	return nil
}
