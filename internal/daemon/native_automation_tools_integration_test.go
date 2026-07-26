//go:build integration

package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/compozy/agh/internal/acp"
	apitest "github.com/compozy/agh/internal/api/testutil"
	automationpkg "github.com/compozy/agh/internal/automation"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store/globaldb"
	"github.com/compozy/agh/internal/testutil"
	toolspkg "github.com/compozy/agh/internal/tools"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestDaemonNativeAutomationToolsIntegrationLifecycleParity(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	manager := newNativeAutomationIntegrationManager(t, ctx)
	registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
		Automation: manager,
	}, nativeApproveAllPolicyInputs())

	jobCreateResult, err := registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationJobsCreate,
			Input: json.RawMessage(
				`{"scope":"global","name":"integration-daily","agent_name":"codex","prompt":"run integration","schedule":{"mode":"every","interval":"1h"}}`,
			),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_jobs_create) error = %v", err)
	}
	jobID := nativeAutomationResultResourceID(t, jobCreateResult, "job")
	job, err := manager.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("manager.GetJob(created) error = %v", err)
	}
	if job.Name != "integration-daily" || job.Source != automationpkg.JobSourceDynamic {
		t.Fatalf("created job = %#v, want dynamic integration job", job)
	}

	_, err = registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationJobsUpdate,
			Input:  json.RawMessage(fmt.Sprintf(`{"job_id":%q,"name":"integration-daily-updated"}`, jobID)),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_jobs_update) error = %v", err)
	}
	job, err = manager.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("manager.GetJob(updated) error = %v", err)
	}
	if job.Name != "integration-daily-updated" {
		t.Fatalf("job.Name = %q, want updated name", job.Name)
	}

	_, err = registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationJobsDisable,
			Input:  json.RawMessage(fmt.Sprintf(`{"job_id":%q}`, jobID)),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_jobs_disable) error = %v", err)
	}
	job, err = manager.GetJob(ctx, jobID)
	if err != nil {
		t.Fatalf("manager.GetJob(disabled) error = %v", err)
	}
	if job.Enabled {
		t.Fatal("job.Enabled = true, want false after disable tool")
	}

	_, err = registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationJobsEnable,
			Input:  json.RawMessage(fmt.Sprintf(`{"job_id":%q}`, jobID)),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_jobs_enable) error = %v", err)
	}

	jobRunResult, err := registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationJobsTrigger,
			Input:  json.RawMessage(fmt.Sprintf(`{"job_id":%q}`, jobID)),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_jobs_trigger) error = %v", err)
	}
	runID := nativeAutomationResultResourceID(t, jobRunResult, "run")
	run, err := manager.GetRun(ctx, runID)
	if err != nil {
		t.Fatalf("manager.GetRun(triggered) error = %v", err)
	}
	if run.JobID != jobID || run.Status != automationpkg.RunCompleted {
		t.Fatalf("triggered run = %#v, want completed run for %q", run, jobID)
	}

	historyResult, err := registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationJobsHistory,
			Input:  json.RawMessage(fmt.Sprintf(`{"job_id":%q,"status":"completed"}`, jobID)),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_jobs_history) error = %v", err)
	}
	requireNativeStructuredContains(t, historyResult, []byte(runID))

	runsResult, err := registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationRunsList,
			Input:  json.RawMessage(fmt.Sprintf(`{"job_id":%q}`, jobID)),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_runs_list) error = %v", err)
	}
	requireNativeStructuredContains(t, runsResult, []byte(runID))

	runGetResult, err := registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationRunsGet,
			Input:  json.RawMessage(fmt.Sprintf(`{"run_id":%q}`, runID)),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_runs_get) error = %v", err)
	}
	requireNativeStructuredContains(t, runGetResult, []byte(jobID))

	triggerCreateResult, err := registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationTriggersCreate,
			Input: json.RawMessage(
				`{"scope":"global","name":"integration-trigger","agent_name":"codex","prompt":"trigger {{ .Kind }}","event":"session.created"}`,
			),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_triggers_create) error = %v", err)
	}
	triggerID := nativeAutomationResultResourceID(t, triggerCreateResult, "trigger")
	trigger, err := manager.GetTrigger(ctx, triggerID)
	if err != nil {
		t.Fatalf("manager.GetTrigger(created) error = %v", err)
	}
	if trigger.Name != "integration-trigger" || trigger.Source != automationpkg.JobSourceDynamic {
		t.Fatalf("created trigger = %#v, want dynamic integration trigger", trigger)
	}

	_, err = registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationTriggersUpdate,
			Input:  json.RawMessage(fmt.Sprintf(`{"trigger_id":%q,"name":"integration-trigger-updated"}`, triggerID)),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_triggers_update) error = %v", err)
	}
	trigger, err = manager.GetTrigger(ctx, triggerID)
	if err != nil {
		t.Fatalf("manager.GetTrigger(updated) error = %v", err)
	}
	if trigger.Name != "integration-trigger-updated" {
		t.Fatalf("trigger.Name = %q, want updated name", trigger.Name)
	}

	_, err = registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationTriggersDisable,
			Input:  json.RawMessage(fmt.Sprintf(`{"trigger_id":%q}`, triggerID)),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_triggers_disable) error = %v", err)
	}
	trigger, err = manager.GetTrigger(ctx, triggerID)
	if err != nil {
		t.Fatalf("manager.GetTrigger(disabled) error = %v", err)
	}
	if trigger.Enabled {
		t.Fatal("trigger.Enabled = true, want false after disable tool")
	}

	triggerHistoryResult, err := registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationTriggersHistory,
			Input:  json.RawMessage(fmt.Sprintf(`{"trigger_id":%q}`, triggerID)),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_triggers_history) error = %v", err)
	}
	requireNativeStructuredContains(t, triggerHistoryResult, []byte(`"runs":[]`))

	_, err = registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationTriggersDelete,
			Input:  json.RawMessage(fmt.Sprintf(`{"trigger_id":%q}`, triggerID)),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_triggers_delete) error = %v", err)
	}
	if _, err = manager.GetTrigger(ctx, triggerID); !errors.Is(err, automationpkg.ErrTriggerNotFound) {
		t.Fatalf("manager.GetTrigger(deleted) error = %v, want ErrTriggerNotFound", err)
	}

	_, err = registry.Call(
		ctx,
		toolspkg.Scope{},
		toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDAutomationJobsDelete,
			Input:  json.RawMessage(fmt.Sprintf(`{"job_id":%q}`, jobID)),
		},
	)
	if err != nil {
		t.Fatalf("Registry.Call(automation_jobs_delete) error = %v", err)
	}
	if _, err = manager.GetJob(ctx, jobID); !errors.Is(err, automationpkg.ErrJobNotFound) {
		t.Fatalf("manager.GetJob(deleted) error = %v, want ErrJobNotFound", err)
	}
}

func TestDaemonNativeAutomationToolsIntegrationRejectsDaemonLifecycleJob(t *testing.T) {
	t.Run("Should return the blocked class and persist no job", func(t *testing.T) {
		t.Parallel()

		ctx := testutil.Context(t)
		manager := newNativeAutomationIntegrationManager(t, ctx)
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Automation: manager,
		}, nativeApproveAllPolicyInputs())

		_, err := registry.Call(
			ctx,
			toolspkg.Scope{},
			toolspkg.CallRequest{
				ToolID: toolspkg.ToolIDAutomationJobsCreate,
				Input: json.RawMessage(
					`{"scope":"global","name":"blocked-restart","agent_name":"codex","prompt":"Run agh daemon restart now","schedule":{"mode":"every","interval":"1h"}}`,
				),
			},
		)
		if err == nil {
			t.Fatal("Registry.Call(automation_jobs_create) error = nil, want lifecycle command rejection")
		}
		if !errors.Is(err, automationpkg.ErrDaemonLifecycleCommandBlocked) {
			t.Fatalf("Registry.Call(automation_jobs_create) error = %v, want ErrDaemonLifecycleCommandBlocked", err)
		}
		var blockedErr *automationpkg.DaemonLifecycleCommandError
		if !errors.As(err, &blockedErr) {
			t.Fatalf("Registry.Call(automation_jobs_create) error = %T, want *DaemonLifecycleCommandError", err)
		}
		if blockedErr.Class != automationpkg.DaemonLifecycleCommandClassAGHDaemon {
			t.Fatalf("blocked command class = %q, want %q", blockedErr.Class, automationpkg.DaemonLifecycleCommandClassAGHDaemon)
		}
		reason, ok := toolspkg.ReasonOf(err)
		if !ok || reason != toolspkg.ReasonAutomationValidationFailed {
			t.Fatalf("Registry.Call(automation_jobs_create) reason = %q, %v, want %q", reason, ok, toolspkg.ReasonAutomationValidationFailed)
		}

		page, listErr := manager.ListJobs(ctx, automationpkg.JobListQuery{})
		if listErr != nil {
			t.Fatalf("manager.ListJobs() error = %v", listErr)
		}
		if len(page.Jobs) != 0 {
			t.Fatalf("manager.ListJobs() jobs = %#v, want no persisted job", page.Jobs)
		}
	})
}

func newNativeAutomationIntegrationManager(t *testing.T, ctx context.Context) *automationpkg.Manager {
	t.Helper()

	homePaths := testHomePaths(t)
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
		workspacepkg.WithLogger(discardLogger()),
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
	if _, err := resolver.ResolveOrRegister(ctx, workspaceRoot); err != nil {
		t.Fatalf("ResolveOrRegister(%q) error = %v", workspaceRoot, err)
	}

	manager, err := automationpkg.New(
		automationpkg.WithStore(db),
		automationpkg.WithSessions(newNativeAutomationSessionManager()),
		automationpkg.WithWorkspaceResolver(resolver),
		automationpkg.WithConfig(aghconfig.AutomationConfig{
			Enabled:           true,
			Timezone:          automationpkg.DefaultTimezone,
			MaxConcurrentJobs: automationpkg.DefaultMaxConcurrentJobs,
			DefaultFireLimit:  automationpkg.DefaultFireLimitConfig(),
		}),
		automationpkg.WithLogger(discardLogger()),
		automationpkg.WithGlobalWorkspacePath(homePaths.HomeDir),
	)
	if err != nil {
		t.Fatalf("automation.New() error = %v", err)
	}
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("manager.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("manager.Shutdown() error = %v", err)
		}
	})
	return manager
}

func newNativeAutomationSessionManager() apitest.StubSessionManager {
	var mu sync.Mutex
	statuses := map[string]*session.Info{}
	var seq int
	return apitest.StubSessionManager{
		CreateFn: func(_ context.Context, opts session.CreateOpts) (*session.Session, error) {
			mu.Lock()
			defer mu.Unlock()
			seq++
			id := fmt.Sprintf("sess-integration-%d", seq)
			created := &session.Session{
				ID:          id,
				AgentName:   opts.AgentName,
				WorkspaceID: opts.Workspace,
				Workspace:   opts.WorkspacePath,
				Type:        opts.Type,
				State:       session.StateActive,
			}
			statuses[id] = created.Info()
			return created, nil
		},
		PromptFn: func(context.Context, string, string) (<-chan acp.AgentEvent, error) {
			events := make(chan acp.AgentEvent)
			close(events)
			return events, nil
		},
		StopWithCauseFn: func(_ context.Context, id string, cause session.StopCause, detail string) error {
			mu.Lock()
			defer mu.Unlock()
			info := statuses[id]
			if info == nil {
				return nil
			}
			next := *info
			next.State = session.StateStopped
			next.StopDetail = detail
			statuses[id] = &next
			return nil
		},
		StatusFn: func(_ context.Context, id string) (*session.Info, error) {
			mu.Lock()
			defer mu.Unlock()
			info := statuses[id]
			if info == nil {
				return nil, session.ErrSessionNotFound
			}
			return info, nil
		},
	}
}

func nativeAutomationResultResourceID(t *testing.T, result toolspkg.ToolResult, key string) string {
	t.Helper()

	var payload map[string]struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(result.Structured, &payload); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", result.Structured, err)
	}
	resource, ok := payload[key]
	if !ok {
		t.Fatalf("structured result = %s, want %q resource", result.Structured, key)
	}
	if resource.ID == "" {
		t.Fatalf("structured result = %s, want non-empty %s.id", result.Structured, key)
	}
	return resource.ID
}
