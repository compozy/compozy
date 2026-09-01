package extensionpkg

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	memcontract "github.com/compozy/compozy/internal/memory/contract"

	"github.com/compozy/compozy/internal/acp"
	apicontract "github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	bridgepkg "github.com/compozy/compozy/internal/bridges"
	bridgecontract "github.com/compozy/compozy/internal/bridges/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	protocol "github.com/compozy/compozy/internal/extensionprotocol"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/memory"
	"github.com/compozy/compozy/internal/network/participation"
	observepkg "github.com/compozy/compozy/internal/observe"
	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/sandbox"
	sandboxlocal "github.com/compozy/compozy/internal/sandbox/local"
	"github.com/compozy/compozy/internal/session"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/subprocess"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/compozy/compozy/internal/testutil"
	toolspkg "github.com/compozy/compozy/internal/tools"
	transcriptpkg "github.com/compozy/compozy/internal/transcript"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func hostAPITestTaskCatalogFilterMapper(query *taskpkg.CatalogQuery, includeLoop bool, loopRunID string) {
	if query == nil {
		return
	}
	query.LoopRunID = strings.TrimSpace(loopRunID)
	if includeLoop || query.LoopRunID != "" || strings.TrimSpace(query.ParentTaskID) != "" {
		query.ExcludeCreatedBy = nil
		return
	}
	query.ExcludeCreatedBy = []taskpkg.ActorRef{{Kind: taskpkg.ActorKindDaemon, Ref: "loop-coordinator"}}
}

func TestHostAPIHandlerSessionsListReturnsAuthorizedSessions(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-allowed", []string{"sessions/list"}, []string{"session.read"})

	sess := env.createSession(t)
	result, err := env.call(t, "ext-allowed", "sessions/list", map[string]string{"workspace": env.workspaceID})
	if err != nil {
		t.Fatalf("Handle(sessions/list) error = %v", err)
	}

	var sessionsList []hostAPISessionSummary
	decodeResult(t, result, &sessionsList)
	if len(sessionsList) != 1 {
		t.Fatalf("sessions/list len = %d, want 1", len(sessionsList))
	}
	if sessionsList[0].ID != sess.ID {
		t.Fatalf("sessions/list[0].ID = %q, want %q", sessionsList[0].ID, sess.ID)
	}
	if sessionsList[0].Agent != "coder" {
		t.Fatalf("sessions/list[0].Agent = %q, want coder", sessionsList[0].Agent)
	}
	if sessionsList[0].Runtime.Status != sess.Info().RuntimeStatus {
		t.Fatalf(
			"sessions/list[0].Runtime.Status = %q, want %q",
			sessionsList[0].Runtime.Status,
			sess.Info().RuntimeStatus,
		)
	}
}

func TestHostAPIHandlerBindsWorkspaceScopedExtensionCalls(t *testing.T) {
	t.Parallel()

	t.Run("Should bind workspace-scoped extension calls", func(t *testing.T) {
		t.Parallel()

		for _, method := range protocol.AllHostAPIMethods() {
			if _, ok := hostAPIWorkspaceBindings[method]; !ok {
				t.Fatalf("workspace binding decision missing for %q", method)
			}
		}

		env := newHostAPITestEnv(t)
		const extensionName = "ext-workspace-bound"
		env.grant(extensionName, []string{
			"sessions/list",
			"automation/jobs/get",
			"automation/jobs/update",
			"automation/jobs/delete",
			"automation/jobs/trigger",
			"automation/jobs/runs",
			"automation/triggers/get",
			"automation/triggers/update",
			"automation/triggers/delete",
			"automation/triggers/runs",
			"automation/runs",
		}, []string{"session.read", "automation.read", "automation.write"})

		owned := env.createSession(t)
		foreignWorkspace := env.addForeignWorkspace(t)
		foreign, err := env.sessions.Create(testutil.Context(t), session.CreateOpts{
			AgentName: "coder",
			Workspace: foreignWorkspace.ID,
			Type:      session.SessionTypeSystem,
		})
		if err != nil {
			t.Fatalf("sessions.Create(foreign) error = %v", err)
		}
		t.Cleanup(func() {
			if err := env.sessions.Stop(testutil.Context(t), foreign.ID); err != nil {
				t.Errorf("sessions.Stop(foreign) cleanup error = %v", err)
			}
		})

		ctx := withHostAPIResourceSession(testutil.Context(t), &hostAPIResourceSession{
			Actor: resources.MutationActor{
				Kind: resources.MutationActorKindExtension,
				ID:   extensionName,
				MaxScope: resources.ResourceScope{
					Kind: resources.ResourceScopeKindWorkspace,
					ID:   env.workspace.ID,
				},
			},
		})
		result, err := env.callWithContext(ctx, t, extensionName, "sessions/list", nil)
		if err != nil {
			t.Fatalf("Handle(sessions/list omitted workspace) error = %v", err)
		}
		var sessionsList []hostAPISessionSummary
		decodeResult(t, result, &sessionsList)
		if len(sessionsList) != 1 || sessionsList[0].ID != owned.ID {
			t.Fatalf("sessions/list = %#v, want only owned session %q", sessionsList, owned.ID)
		}

		_, err = env.callWithContext(ctx, t, extensionName, "sessions/list", map[string]string{
			"workspace": foreignWorkspace.ID,
		})
		assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
		assertErrorContains(t, err, "conflicts with the bound workspace")

		foreignJob, err := env.automation.CreateJob(testutil.Context(t), automationpkg.Job{
			Name:        "foreign-boundary-job",
			Scope:       automationpkg.AutomationScopeWorkspace,
			WorkspaceID: foreignWorkspace.ID,
			AgentName:   "coder",
			Prompt:      "foreign",
			Schedule: &automationpkg.ScheduleSpec{
				Mode:     automationpkg.ScheduleModeEvery,
				Interval: "1h",
			},
			Enabled:   true,
			Retry:     automationpkg.DefaultRetryConfig(),
			FireLimit: automationpkg.DefaultFireLimitConfig(),
			Source:    automationpkg.JobSourceDynamic,
		})
		if err != nil {
			t.Fatalf("CreateJob(foreign) error = %v", err)
		}
		ownedJob := foreignJob
		ownedJob.ID = ""
		ownedJob.Name = "owned-boundary-job"
		ownedJob.WorkspaceID = env.workspace.ID
		ownedJob, err = env.automation.CreateJob(testutil.Context(t), ownedJob)
		if err != nil {
			t.Fatalf("CreateJob(owned) error = %v", err)
		}
		foreignTrigger, err := env.automation.CreateTrigger(testutil.Context(t), automationpkg.Trigger{
			Name:        "foreign-boundary-trigger",
			Scope:       automationpkg.AutomationScopeWorkspace,
			WorkspaceID: foreignWorkspace.ID,
			AgentName:   "coder",
			Event:       "ext.boundary.foreign",
			Prompt:      "foreign",
			Enabled:     true,
			Retry:       automationpkg.DefaultRetryConfig(),
			FireLimit:   automationpkg.DefaultFireLimitConfig(),
			Source:      automationpkg.JobSourceDynamic,
		}, automationpkg.WebhookSecretWrite{})
		if err != nil {
			t.Fatalf("CreateTrigger(foreign) error = %v", err)
		}

		for _, tc := range []struct {
			name          string
			method        string
			foreignParams map[string]any
			missingParams map[string]any
		}{
			{
				name:          "Should hide a foreign job read",
				method:        "automation/jobs/get",
				foreignParams: map[string]any{"id": foreignJob.ID},
				missingParams: map[string]any{"id": "job-missing"},
			},
			{
				name:          "Should hide a foreign job update",
				method:        "automation/jobs/update",
				foreignParams: map[string]any{"id": foreignJob.ID, "prompt": "changed"},
				missingParams: map[string]any{"id": "job-missing", "prompt": "changed"},
			},
			{
				name:          "Should hide a foreign job deletion",
				method:        "automation/jobs/delete",
				foreignParams: map[string]any{"id": foreignJob.ID},
				missingParams: map[string]any{"id": "job-missing"},
			},
			{
				name:          "Should hide a foreign job trigger",
				method:        "automation/jobs/trigger",
				foreignParams: map[string]any{"id": foreignJob.ID},
				missingParams: map[string]any{"id": "job-missing"},
			},
			{
				name:          "Should hide foreign job runs",
				method:        "automation/jobs/runs",
				foreignParams: map[string]any{"id": foreignJob.ID},
				missingParams: map[string]any{"id": "job-missing"},
			},
			{
				name:          "Should hide a foreign trigger read",
				method:        "automation/triggers/get",
				foreignParams: map[string]any{"id": foreignTrigger.ID},
				missingParams: map[string]any{"id": "trigger-missing"},
			},
			{
				name:          "Should hide a foreign trigger update",
				method:        "automation/triggers/update",
				foreignParams: map[string]any{"id": foreignTrigger.ID, "prompt": "changed"},
				missingParams: map[string]any{"id": "trigger-missing", "prompt": "changed"},
			},
			{
				name:          "Should hide a foreign trigger deletion",
				method:        "automation/triggers/delete",
				foreignParams: map[string]any{"id": foreignTrigger.ID},
				missingParams: map[string]any{"id": "trigger-missing"},
			},
			{
				name:          "Should hide foreign trigger runs",
				method:        "automation/triggers/runs",
				foreignParams: map[string]any{"id": foreignTrigger.ID},
				missingParams: map[string]any{"id": "trigger-missing"},
			},
			{
				name:          "Should hide foreign filtered runs",
				method:        "automation/runs",
				foreignParams: map[string]any{"job_id": foreignJob.ID},
				missingParams: map[string]any{"job_id": "job-missing"},
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				_, missingErr := env.callWithContext(ctx, t, extensionName, tc.method, tc.missingParams)
				if missingErr == nil {
					t.Fatalf("Handle(%s missing) error = nil, want not found", tc.method)
				}
				_, foreignErr := env.callWithContext(ctx, t, extensionName, tc.method, tc.foreignParams)
				if foreignErr == nil {
					t.Fatalf("Handle(%s foreign) error = nil, want not found", tc.method)
				}
				if fmt.Sprintf("%T:%v", foreignErr, foreignErr) != fmt.Sprintf("%T:%v", missingErr, missingErr) {
					t.Fatalf(
						"Handle(%s foreign) error = %T:%v, want missing-target result %T:%v",
						tc.method,
						foreignErr,
						foreignErr,
						missingErr,
						missingErr,
					)
				}
			})
		}

		ownedResult, err := env.callWithContext(
			ctx,
			t,
			extensionName,
			"automation/jobs/get",
			map[string]any{"id": ownedJob.ID},
		)
		if err != nil {
			t.Fatalf("Handle(automation/jobs/get owned) error = %v", err)
		}
		var fetchedOwned automationpkg.Job
		decodeResult(t, ownedResult, &fetchedOwned)
		if fetchedOwned.ID != ownedJob.ID {
			t.Fatalf("automation/jobs/get owned id = %q, want %q", fetchedOwned.ID, ownedJob.ID)
		}

		canceledCtx, cancel := context.WithCancel(ctx)
		cancel()
		if _, err := env.handler.automationJobForBoundSession(
			canceledCtx,
			env.automation,
			ownedJob.ID,
		); !errors.Is(err, context.Canceled) {
			t.Fatalf("automationJobForBoundSession(canceled) error = %v, want context.Canceled", err)
		}
		if _, err := env.handler.automationTriggerForBoundSession(
			canceledCtx,
			env.automation,
			foreignTrigger.ID,
		); !errors.Is(err, context.Canceled) {
			t.Fatalf("automationTriggerForBoundSession(canceled) error = %v, want context.Canceled", err)
		}
	})
}

func TestHostAPIHandlerSessionsListReturnsCapabilityDeniedWithoutSessionRead(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-denied", nil, nil)

	_, err := env.call(t, "ext-denied", "sessions/list", nil)
	assertCapabilityDenied(t, err, "sessions/list")
}

func TestHostAPIHandlerSessionsCreateReturnsSessionID(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t, withHostAPITestLiveParticipation())
	env.grant("ext-create", []string{"sessions/create"}, []string{"session.write"})

	result, err := env.call(t, "ext-create", "sessions/create", map[string]any{
		"agent":     "coder",
		"workspace": env.workspaceID,
		"network_participation": map[string]any{
			"mode":             "live",
			"channel_strategy": "named",
			"channel_id":       "builders",
		},
	})
	if err != nil {
		t.Fatalf("Handle(sessions/create) error = %v", err)
	}

	var created hostAPISessionCreateResult
	decodeResult(t, result, &created)
	if created.SessionID == "" {
		t.Fatal("sessions/create session_id = empty, want non-empty")
	}
	info := waitForHostAPISessionState(t, env.sessions, created.SessionID, session.StateActive)
	if info.NetworkParticipation.Mode != participation.ModeLive ||
		info.NetworkParticipation.ChannelID != "builders" {
		t.Fatalf(
			"created session participation = %#v, want Live builders",
			info.NetworkParticipation,
		)
	}

	_, err = env.call(t, "ext-create", "sessions/create", map[string]any{
		"agent":           "coder",
		"workspace":       env.workspaceID,
		"network_channel": "legacy",
	})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "network_channel")
}

func TestHostAPIHandlerSessionsCreateUsesDurableLogicalAcceptance(t *testing.T) {
	t.Parallel()

	t.Run("Should accept a logical session without prompt or runtime", func(t *testing.T) {
		t.Parallel()

		sessions := &recordingHostAPISessionManager{}
		handler := &HostAPIHandler{sessions: sessions}
		raw := json.RawMessage(`{
			"agent":"coder",
			"workspace":"ws-alpha"
		}`)

		result, err := handler.handleSessionsCreate(testutil.Context(t), raw)
		if err != nil {
			t.Fatalf("handleSessionsCreate() error = %v", err)
		}
		if len(sessions.createCalls) != 0 {
			t.Fatalf("Create() calls = %#v, want atomic accepted create only", sessions.createCalls)
		}
		if len(sessions.acceptedCreateCalls) != 1 {
			t.Fatalf("CreateAccepted() calls = %#v, want one", sessions.acceptedCreateCalls)
		}
		accepted := sessions.acceptedCreateCalls[0]
		if accepted.Session.AgentName != "coder" || accepted.Session.Workspace != "ws-alpha" {
			t.Fatalf("CreateAccepted() Session = %#v", accepted.Session)
		}
		var created hostAPISessionCreateResult
		decodeResult(t, result, &created)
		if created.SessionID != "sess-accepted" {
			t.Fatalf("sessions/create result = %#v", created)
		}
	})
}

func TestDecodeHostAPIParamsRejectsUnknownFieldsByDefault(t *testing.T) {
	t.Parallel()

	var params struct {
		WorkspaceID string `json:"workspace_id"`
	}
	err := decodeHostAPIParams(json.RawMessage(`{"workspace_id":"ws-1","legacy":true}`), &params)
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "legacy")
}

func TestHostAPIHandlerSessionsCreateReturnsCapabilityDeniedWithoutSessionWrite(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-denied", nil, nil)

	_, err := env.call(t, "ext-denied", "sessions/create", map[string]string{
		"agent":     "coder",
		"workspace": env.workspaceID,
	})
	assertCapabilityDenied(t, err, "sessions/create")
}

func TestHostAPIHandlerSessionsPromptReturnsTurnIDAndPersistsEvents(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-prompt", []string{"sessions/prompt"}, []string{"session.write"})

	sess := env.createSession(t)
	result, err := env.call(t, "ext-prompt", "sessions/prompt", map[string]string{
		"workspace_id":    env.workspaceID,
		"session_id":      sess.ID,
		"message":         "hello from extension",
		"message_id":      "msg-extension",
		"idempotency_key": "idem-extension",
	})
	if err != nil {
		t.Fatalf("Handle(sessions/prompt) error = %v", err)
	}

	var prompt hostAPISessionPromptResult
	decodeResult(t, result, &prompt)
	if prompt.TurnID == "" {
		t.Fatal("sessions/prompt turn_id = empty, want non-empty")
	}
	if prompt.MessageID != "msg-extension" || prompt.IdempotencyKey != "idem-extension" || prompt.Replayed {
		t.Fatalf("sessions/prompt identity result = %#v", prompt)
	}

	events, err := env.sessions.Events(testutil.Context(t), sess.ID, store.EventQuery{TurnID: prompt.TurnID})
	if err != nil {
		t.Fatalf("sessions.Events(%q) error = %v", sess.ID, err)
	}
	if len(events) == 0 {
		t.Fatal("sessions events = empty, want prompt events")
	}
	if events[0].TurnID != prompt.TurnID {
		t.Fatalf("events[0].TurnID = %q, want %q", events[0].TurnID, prompt.TurnID)
	}
}

func TestHostAPIHandlerSessionsPromptRequiresDurableIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		params map[string]any
		want   string
	}{
		{
			name: "Should reject a missing message id",
			params: map[string]any{
				"workspace_id": "ws-1", "session_id": "sess-1", "message": "hello", "idempotency_key": "idem-1",
			},
			want: "message_id is required",
		},
		{
			name: "Should reject a missing idempotency key",
			params: map[string]any{
				"workspace_id": "ws-1", "session_id": "sess-1", "message": "hello", "message_id": "msg-1",
			},
			want: "idempotency_key is required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := &HostAPIHandler{}
			_, err := handler.handleSessionsPrompt(testutil.Context(t), mustMarshalRawMessage(t, tt.params))
			assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
			assertErrorContains(t, err, tt.want)
		})
	}
}

func TestHostAPIHandlerSessionsStopStopsSession(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-stop", []string{"sessions/stop"}, []string{"session.write"})

	sess := env.createSession(t)
	if _, err := env.call(t, "ext-stop", "sessions/stop", map[string]string{
		"workspace_id": env.workspaceID,
		"session_id":   sess.ID,
	}); err != nil {
		t.Fatalf("Handle(sessions/stop) error = %v", err)
	}

	info, err := env.sessions.Status(testutil.Context(t), sess.ID)
	if err != nil {
		t.Fatalf("sessions.Status(%q) error = %v", sess.ID, err)
	}
	if info.State != session.StateStopped {
		t.Fatalf("stopped session state = %q, want %q", info.State, session.StateStopped)
	}
}

func TestHostAPIHandlerSessionsArchiveRoundTrip(t *testing.T) {
	t.Run("Should archive and unarchive a stopped session", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		env.grant(
			"ext-archive",
			[]string{"sessions/archive", "sessions/unarchive", "sessions/list"},
			[]string{"session.write", "session.read"},
		)
		sess := env.createSession(t)
		if err := env.sessions.Stop(testutil.Context(t), sess.ID); err != nil {
			t.Fatalf("sessions.Stop() error = %v", err)
		}

		result, err := env.call(t, "ext-archive", "sessions/archive", map[string]string{
			"workspace_id": env.workspaceID,
			"session_id":   sess.ID,
		})
		if err != nil {
			t.Fatalf("Handle(sessions/archive) error = %v", err)
		}
		var archived hostAPISessionStatus
		decodeResult(t, result, &archived)
		if archived.ArchivedAt == nil || archived.State != session.StateStopped {
			t.Fatalf("sessions/archive result = %#v, want archived stopped session", archived)
		}

		defaultResult, err := env.call(
			t,
			"ext-archive",
			"sessions/list",
			map[string]string{"workspace": env.workspaceID},
		)
		if err != nil {
			t.Fatalf("Handle(sessions/list default) error = %v", err)
		}
		var defaultList []hostAPISessionSummary
		decodeResult(t, defaultResult, &defaultList)
		if len(defaultList) != 0 {
			t.Fatalf("sessions/list default = %#v, want archived session excluded", defaultList)
		}

		archiveResult, err := env.call(t, "ext-archive", "sessions/list", map[string]string{
			"workspace": env.workspaceID,
			"archive":   "only",
		})
		if err != nil {
			t.Fatalf("Handle(sessions/list archive=only) error = %v", err)
		}
		var archivedList []hostAPISessionSummary
		decodeResult(t, archiveResult, &archivedList)
		if len(archivedList) != 1 || archivedList[0].ID != sess.ID || archivedList[0].ArchivedAt == nil {
			t.Fatalf("sessions/list archive=only = %#v, want archived session", archivedList)
		}

		result, err = env.call(t, "ext-archive", "sessions/unarchive", map[string]string{
			"workspace_id": env.workspaceID,
			"session_id":   sess.ID,
		})
		if err != nil {
			t.Fatalf("Handle(sessions/unarchive) error = %v", err)
		}
		var restored hostAPISessionStatus
		decodeResult(t, result, &restored)
		if restored.ArchivedAt != nil {
			t.Fatalf("sessions/unarchive result = %#v, want archive marker cleared", restored)
		}
	})
}

func TestHostAPIHandlerSessionsStatusReturnsAuthorizedState(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-status", []string{"sessions/status"}, []string{"session.read"})

	sess := env.createSession(t)
	result, err := env.call(t, "ext-status", "sessions/status", map[string]string{
		"workspace_id": env.workspaceID,
		"session_id":   sess.ID,
	})
	if err != nil {
		t.Fatalf("Handle(sessions/status) error = %v", err)
	}

	var status hostAPISessionStatus
	decodeResult(t, result, &status)
	if status.SessionID != sess.ID {
		t.Fatalf("sessions/status session_id = %q, want %q", status.SessionID, sess.ID)
	}
	if status.State != session.StateActive {
		t.Fatalf("sessions/status state = %q, want %q", status.State, session.StateActive)
	}
	if status.Runtime.Status != sess.Info().RuntimeStatus {
		t.Fatalf(
			"sessions/status runtime status = %q, want %q",
			status.Runtime.Status,
			sess.Info().RuntimeStatus,
		)
	}

	foreign := env.workspace
	foreign.ID = "ws-foreign-registry"
	foreign.WorkspaceID = "ws-foreign"
	foreign.ID = "ws-foreign-registry"
	foreign.RootDir = t.TempDir()
	foreign.Name = "foreign"
	env.workspaces.upsert(&foreign)
	_, err = env.call(t, "ext-status", "sessions/status", map[string]string{
		"workspace_id": foreign.WorkspaceID,
		"session_id":   sess.ID,
	})
	assertRPCErrorCode(t, err, HostAPINotFoundCode)
}

func TestHostAPIHandlerSessionRuntime(t *testing.T) {
	t.Parallel()
	t.Run("Should project nested runtime through session reads", func(t *testing.T) {
		t.Parallel()

		const workspaceID = "ws-runtime"
		info := &session.Info{
			ID:                "sess-runtime",
			Name:              "Runtime session",
			AgentName:         "coder",
			Provider:          "codex",
			Model:             "gpt-5.6",
			ReasoningEffort:   "high",
			Speed:             "fast",
			RuntimeStatus:     session.RuntimeStatusReady,
			RuntimeTransition: session.RuntimeTransitionLiveConfiguration,
			RuntimeFailure:    "runtime warning",
			WorkspaceID:       workspaceID,
			Workspace:         "/tmp/runtime",
			State:             session.StateActive,
			ACPSessionID:      "acp-runtime",
			ACPCapsKnown:      true,
			ACPCaps: acp.Caps{
				SupportsLoadSession:   true,
				PromptImage:           true,
				PromptAudio:           true,
				PromptEmbeddedContext: true,
				SupportedModes:        []string{"edit"},
			},
			CreatedAt: time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 7, 30, 12, 1, 0, 0, time.UTC),
		}
		handler := &HostAPIHandler{sessions: promptSessionManagerStub{
			listAllFn: func(context.Context) ([]*session.Info, error) {
				return []*session.Info{info}, nil
			},
			statusFn: func(context.Context, string) (*session.Info, error) {
				return info, nil
			},
		}}

		listResult, err := handler.handleSessionsList(testutil.Context(t), nil)
		if err != nil {
			t.Fatalf("handleSessionsList() error = %v", err)
		}
		var listed []hostAPISessionSummary
		decodeResult(t, listResult, &listed)
		if len(listed) != 1 {
			t.Fatalf("sessions/list len = %d, want 1", len(listed))
		}
		assertHostAPISessionRuntimePayload(t, listed[0].Runtime)

		statusResult, err := handler.handleSessionsStatus(
			testutil.Context(t),
			mustMarshalRawMessage(t, map[string]string{
				"workspace_id": workspaceID,
				"session_id":   info.ID,
			}),
		)
		if err != nil {
			t.Fatalf("handleSessionsStatus() error = %v", err)
		}
		var status hostAPISessionStatus
		decodeResult(t, statusResult, &status)
		assertHostAPISessionRuntimePayload(t, status.Runtime)
	})

	t.Run("Should set and clear the durable runtime selection with CAS", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		env.grant(
			"ext-runtime",
			[]string{"sessions/runtime/set", "sessions/runtime/clear"},
			[]string{"session.write"},
		)
		sess := env.createSession(t)
		startsBeforeMutation := env.driver.startSeq.Load()

		setResult, err := env.call(t, "ext-runtime", "sessions/runtime/set", map[string]any{
			"workspace_id": env.workspaceID,
			"session_id":   sess.ID,
			"runtime": map[string]any{
				"provider":         "fake-alt",
				"model":            "model-max",
				"reasoning_effort": "max",
				"speed":            "fast",
			},
			"expected_revision": 0,
		})
		if err != nil {
			t.Fatalf("Handle(sessions/runtime/set) error = %v", err)
		}
		var selected hostAPISessionStatus
		decodeResult(t, setResult, &selected)
		if selected.Runtime.Selected == nil || selected.Runtime.Selected.Provider != "fake-alt" ||
			selected.Runtime.Selected.Model != "model-max" ||
			selected.Runtime.Selected.ReasoningEffort != "max" ||
			selected.Runtime.Selected.Speed != "fast" ||
			selected.Runtime.SelectionRevision != 1 {
			t.Fatalf("sessions/runtime/set runtime = %#v, want selected fake-alt at revision 1", selected.Runtime)
		}

		_, err = env.call(t, "ext-runtime", "sessions/runtime/set", map[string]any{
			"workspace_id": env.workspaceID,
			"session_id":   sess.ID,
			"runtime": map[string]any{
				"provider": "fake",
				"model":    "stale-model",
			},
			"expected_revision": 0,
		})
		assertRPCErrorCode(t, err, 409)

		_, err = env.call(t, "ext-runtime", "sessions/runtime/clear", map[string]any{
			"workspace_id":      env.workspaceID,
			"session_id":        sess.ID,
			"expected_revision": 0,
		})
		assertRPCErrorCode(t, err, 409)

		unchanged, err := env.sessions.Status(testutil.Context(t), sess.ID)
		if err != nil {
			t.Fatalf("Status(after stale runtime mutations) error = %v", err)
		}
		if unchanged.SelectedRuntime == nil || unchanged.SelectedRuntime.Provider != "fake-alt" ||
			unchanged.SelectedRuntime.Model != "model-max" ||
			unchanged.SelectedRuntime.ReasoningEffort != "max" ||
			unchanged.SelectedRuntime.Speed != "fast" || unchanged.RuntimeSelectionRevision != 1 {
			t.Fatalf("runtime after stale mutations = %#v, want unchanged revision 1", unchanged)
		}

		clearResult, err := env.call(t, "ext-runtime", "sessions/runtime/clear", map[string]any{
			"workspace_id":      env.workspaceID,
			"session_id":        sess.ID,
			"expected_revision": 1,
		})
		if err != nil {
			t.Fatalf("Handle(sessions/runtime/clear) error = %v", err)
		}
		var cleared hostAPISessionStatus
		decodeResult(t, clearResult, &cleared)
		if cleared.Runtime.Selected != nil || cleared.Runtime.SelectionRevision != 2 {
			t.Fatalf("sessions/runtime/clear runtime = %#v, want no selection at revision 2", cleared.Runtime)
		}
		if got := env.driver.startSeq.Load(); got != startsBeforeMutation {
			t.Fatalf("runtime selection process starts = %d, want unchanged %d", got, startsBeforeMutation)
		}
	})
}

func TestHostAPISessionRuntimePayloadFromInfoCapabilities(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name  string
		known bool
		caps  acp.Caps
		want  *apicontract.ACPCapsPayload
	}{
		{
			name: "Should omit capabilities before ACP negotiation",
		},
		{
			name:  "Should project negotiated all-false capabilities",
			known: true,
			want:  &apicontract.ACPCapsPayload{},
		},
		{
			name:  "Should project an image-only prompt capability",
			known: true,
			caps:  acp.Caps{PromptImage: true},
			want:  &apicontract.ACPCapsPayload{PromptImage: true},
		},
		{
			name:  "Should project an embedded-context-only prompt capability",
			known: true,
			caps:  acp.Caps{PromptEmbeddedContext: true},
			want:  &apicontract.ACPCapsPayload{PromptEmbeddedContext: true},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := hostAPISessionRuntimePayloadFromInfo(&session.Info{
				ACPCaps:      tc.caps,
				ACPCapsKnown: tc.known,
			}).ACPCaps
			if tc.want == nil {
				if got != nil {
					t.Fatalf("hostAPISessionRuntimePayloadFromInfo() capabilities = %#v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("hostAPISessionRuntimePayloadFromInfo() capabilities = nil, want payload")
			}
			if got.SupportsLoadSession != tc.want.SupportsLoadSession ||
				got.PromptImage != tc.want.PromptImage ||
				got.PromptAudio != tc.want.PromptAudio ||
				got.PromptEmbeddedContext != tc.want.PromptEmbeddedContext ||
				len(got.SupportedModes) != 0 || len(got.ConfigOptions) != 0 {
				t.Fatalf("hostAPISessionRuntimePayloadFromInfo() capabilities = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestHostAPIHandlerSessionsPromptReturnsInterruptRuntimeAdmission(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve the busy-input mode and expected turn in prompt admission", func(t *testing.T) {
		t.Parallel()

		const workspaceID = "ws-queued-runtime"
		var received session.SendPromptOpts
		handler := &HostAPIHandler{sessions: promptSessionManagerStub{
			statusFn: func(context.Context, string) (*session.Info, error) {
				return &session.Info{ID: "sess-queued-runtime", WorkspaceID: workspaceID}, nil
			},
			eventsFn: func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error) {
				return nil, nil
			},
			sendPromptFn: func(_ context.Context, _ string, opts session.SendPromptOpts) (session.SendPromptResult, error) {
				received = opts
				return session.SendPromptResult{
					Status:          "interrupting",
					Mode:            session.BusyInputModeInterrupt,
					MessageID:       "msg-queued-runtime",
					IdempotencyKey:  "idem-queued-runtime",
					QueueEntryID:    "input-queued",
					QueueGeneration: 7,
					Delivery:        store.SessionInputDeliveryInterruptThenPrompt,
				}, nil
			},
		}}

		result, err := handler.handleSessionsPrompt(testutil.Context(t), mustMarshalRawMessage(t, map[string]any{
			"workspace_id":     workspaceID,
			"session_id":       "sess-queued-runtime",
			"message":          "Replace the active turn with this runtime.",
			"message_id":       "msg-queued-runtime",
			"idempotency_key":  "idem-queued-runtime",
			"mode":             "interrupt",
			"expected_turn_id": "turn-active",
			"runtime": map[string]string{
				"provider":         "codex",
				"model":            "gpt-5.6",
				"reasoning_effort": "high",
				"speed":            "fast",
			},
		}))
		if err != nil {
			t.Fatalf("handleSessionsPrompt() error = %v", err)
		}
		if received.MessageID != "msg-queued-runtime" || received.IdempotencyKey != "idem-queued-runtime" ||
			received.Mode != session.BusyInputModeInterrupt || received.ExpectedTurnID != "turn-active" ||
			received.Runtime == nil || received.Runtime.Provider != "codex" ||
			received.Runtime.Model != "gpt-5.6" || received.Runtime.ReasoningEffort != "high" ||
			received.Runtime.Speed != "fast" {
			t.Fatalf("SendPrompt runtime = %#v, want requested runtime snapshot", received.Runtime)
		}

		var admission hostAPISessionPromptResult
		decodeResult(t, result, &admission)
		if admission.Status != "interrupting" || admission.Mode != apicontract.PromptModeInterrupt ||
			admission.Delivery != store.SessionInputDeliveryInterruptThenPrompt || admission.QueueEntryID != "input-queued" ||
			admission.QueueGeneration != 7 {
			t.Fatalf("sessions/prompt admission = %#v, want interrupt admission metadata", admission)
		}
		if admission.TurnID != "" {
			t.Fatalf("sessions/prompt interrupt turn_id = %q, want empty before delivery", admission.TurnID)
		}
	})
}

func TestHostAPIHandlerSessionInputsUseDurableManagerOperations(t *testing.T) {
	t.Parallel()

	t.Run("Should list replace cancel and promote pending input without local state", func(t *testing.T) {
		t.Parallel()

		const (
			workspaceID = "ws-session-inputs"
			sessionID   = "sess-session-inputs"
			entryID     = "input-session-1"
		)
		enqueuedAt := time.Date(2026, 8, 3, 15, 4, 0, 0, time.UTC)
		pending := session.PendingInput{
			ID:              entryID,
			SessionID:       sessionID,
			MessageID:       "msg-original",
			IdempotencyKey:  "idem-original",
			TargetTurnID:    "turn-active",
			Status:          "queued",
			Mode:            session.BusyInputModeQueue,
			Delivery:        store.SessionInputDeliveryAfterTurn,
			Text:            "Run the next check.",
			QueueGeneration: 5,
			EnqueuedAt:      enqueuedAt,
			Runtime: &session.RuntimeSelection{
				Provider: "codex", Model: "gpt-5.6", ReasoningEffort: "high", Speed: "fast",
			},
		}
		handler := &HostAPIHandler{sessions: promptSessionManagerStub{
			statusFn: func(context.Context, string) (*session.Info, error) {
				return &session.Info{ID: sessionID, WorkspaceID: workspaceID}, nil
			},
			listPendingInputsFn: func(_ context.Context, gotSessionID string) ([]session.PendingInput, error) {
				if gotSessionID != sessionID {
					t.Fatalf("ListPendingInputs() session id = %q, want %q", gotSessionID, sessionID)
				}
				return []session.PendingInput{pending}, nil
			},
			replacePendingInputFn: func(
				_ context.Context,
				gotSessionID string,
				gotEntryID string,
				opts session.ReplacePendingInputOpts,
			) (session.PendingInput, error) {
				if gotSessionID != sessionID || gotEntryID != entryID || opts.Text != "Use the narrow check." ||
					opts.MessageID != "msg-replaced" || opts.IdempotencyKey != "idem-replaced" {
					t.Fatalf("ReplacePendingInput() = (%q, %q, %#v)", gotSessionID, gotEntryID, opts)
				}
				updated := pending
				updated.MessageID = opts.MessageID
				updated.IdempotencyKey = opts.IdempotencyKey
				updated.Text = opts.Text
				return updated, nil
			},
			cancelQueuedPromptFn: func(
				_ context.Context,
				gotSessionID string,
				gotEntryID string,
			) (session.SendPromptResult, error) {
				if gotSessionID != sessionID || gotEntryID != entryID {
					t.Fatalf("CancelQueuedPrompt() = (%q, %q)", gotSessionID, gotEntryID)
				}
				return session.SendPromptResult{
					Status: "canceled", Mode: session.BusyInputModeQueue, Delivery: store.SessionInputDeliveryNone,
					QueueEntryID: entryID, QueueGeneration: 5,
				}, nil
			},
			promotePendingInputFn: func(
				_ context.Context,
				gotSessionID string,
				gotEntryID string,
				opts session.PromotePendingInputOpts,
			) (session.SendPromptResult, error) {
				if gotSessionID != sessionID || gotEntryID != entryID || opts.Text != "Prioritize this." ||
					opts.MessageID != "msg-promoted" || opts.IdempotencyKey != "idem-promoted" ||
					opts.ExpectedTurnID != "turn-active" {
					t.Fatalf("PromotePendingInputToSteer() = (%q, %q, %#v)", gotSessionID, gotEntryID, opts)
				}
				return session.SendPromptResult{
					Status: "steering", Mode: session.BusyInputModeSteer,
					Delivery:  store.SessionInputDeliveryInterruptThenPrompt,
					MessageID: "msg-promoted", IdempotencyKey: "idem-promoted", QueueEntryID: entryID,
					QueueGeneration: 6,
				}, nil
			},
		}}

		listResult, err := handler.handleSessionsInputsList(
			testutil.Context(t),
			mustMarshalRawMessage(t, map[string]string{
				"workspace_id": workspaceID, "session_id": sessionID,
			}),
		)
		if err != nil {
			t.Fatalf("handleSessionsInputsList() error = %v", err)
		}
		listed, ok := listResult.(hostAPISessionInputListResult)
		if !ok || len(listed.Inputs) != 1 || listed.Inputs[0].ID != entryID ||
			listed.Inputs[0].Delivery != store.SessionInputDeliveryAfterTurn ||
			listed.Inputs[0].Runtime == nil || listed.Inputs[0].Runtime.Provider != "codex" {
			t.Fatalf("sessions/inputs/list result = %#v", listResult)
		}

		replaceResult, err := handler.handleSessionsInputsReplace(
			testutil.Context(t),
			mustMarshalRawMessage(t, map[string]string{
				"workspace_id": workspaceID, "session_id": sessionID, "queue_entry_id": entryID,
				"text": "Use the narrow check.", "message_id": "msg-replaced", "idempotency_key": "idem-replaced",
			}),
		)
		if err != nil {
			t.Fatalf("handleSessionsInputsReplace() error = %v", err)
		}
		replaced, ok := replaceResult.(hostAPISessionInputResult)
		if !ok || replaced.Input.MessageID != "msg-replaced" || replaced.Input.IdempotencyKey != "idem-replaced" {
			t.Fatalf("sessions/inputs/replace result = %#v", replaceResult)
		}

		cancelResult, err := handler.handleSessionsInputsCancel(
			testutil.Context(t),
			mustMarshalRawMessage(t, map[string]string{
				"workspace_id": workspaceID, "session_id": sessionID, "queue_entry_id": entryID,
			}),
		)
		if err != nil {
			t.Fatalf("handleSessionsInputsCancel() error = %v", err)
		}
		canceled, ok := cancelResult.(hostAPISessionPromptResult)
		if !ok || canceled.Status != "canceled" || canceled.Delivery != store.SessionInputDeliveryNone ||
			canceled.QueueEntryID != entryID {
			t.Fatalf("sessions/inputs/cancel result = %#v", cancelResult)
		}

		promoteResult, err := handler.handleSessionsInputsPromote(
			testutil.Context(t),
			mustMarshalRawMessage(t, map[string]string{
				"workspace_id": workspaceID, "session_id": sessionID, "queue_entry_id": entryID,
				"text": "Prioritize this.", "message_id": "msg-promoted", "idempotency_key": "idem-promoted",
				"expected_turn_id": "turn-active",
			}),
		)
		if err != nil {
			t.Fatalf("handleSessionsInputsPromote() error = %v", err)
		}
		promoted, ok := promoteResult.(hostAPISessionPromptResult)
		if !ok || promoted.Status != "steering" || promoted.Delivery != store.SessionInputDeliveryInterruptThenPrompt ||
			promoted.MessageID != "msg-promoted" || promoted.IdempotencyKey != "idem-promoted" {
			t.Fatalf("sessions/inputs/promote result = %#v", promoteResult)
		}
	})
}

func assertHostAPISessionRuntimePayload(t testing.TB, runtime apicontract.SessionRuntimePayload) {
	t.Helper()

	if runtime.Status != session.RuntimeStatusReady ||
		runtime.Transition != session.RuntimeTransitionLiveConfiguration ||
		runtime.Failure != "runtime warning" || runtime.ACPSessionID != "acp-runtime" {
		t.Fatalf("runtime lifecycle payload = %#v", runtime)
	}
	if runtime.Effective == nil || runtime.Effective.Provider != "codex" ||
		runtime.Effective.Model != "gpt-5.6" || runtime.Effective.ReasoningEffort != "high" ||
		runtime.Effective.Speed != "fast" {
		t.Fatalf("runtime effective payload = %#v", runtime.Effective)
	}
	if runtime.ACPCaps == nil || !runtime.ACPCaps.SupportsLoadSession ||
		!runtime.ACPCaps.PromptImage || !runtime.ACPCaps.PromptAudio ||
		!runtime.ACPCaps.PromptEmbeddedContext ||
		len(runtime.ACPCaps.SupportedModes) != 1 || runtime.ACPCaps.SupportedModes[0] != "edit" {
		t.Fatalf("runtime ACP caps payload = %#v", runtime.ACPCaps)
	}
}

func TestHostAPIHandlerCreateBridgeSessionUsesExplicitEmptyProvider(t *testing.T) {
	t.Run("Should use an explicit empty provider for bridge sessions", func(t *testing.T) {
		t.Parallel()

		sessions := &recordingHostAPISessionManager{}
		handler := &HostAPIHandler{
			sessions: sessions,
			workspaces: newHostAPIFakeWorkspaceResolver(&workspacepkg.ResolvedWorkspace{
				Workspace: workspacepkg.Workspace{ID: "ws-alpha", RootDir: t.TempDir()},
				Config: compozyconfig.Config{
					Defaults: compozyconfig.DefaultsConfig{Agent: "coder"},
				},
			}),
		}

		created, err := handler.createBridgeSession(testutil.Context(t), bridgepkg.BridgeInstance{
			WorkspaceID: "ws-alpha",
		})
		if err != nil {
			t.Fatalf("createBridgeSession() error = %v", err)
		}
		if created == nil {
			t.Fatal("createBridgeSession() = nil, want session")
		}
		if got, want := len(sessions.createCalls), 1; got != want {
			t.Fatalf("len(createCalls) = %d, want %d", got, want)
		}
		createCall := sessions.createCalls[0]
		if got, want := createCall.AgentName, "coder"; got != want {
			t.Fatalf("Create().AgentName = %q, want %q", got, want)
		}
		if got, want := createCall.Workspace, "ws-alpha"; got != want {
			t.Fatalf("Create().Workspace = %q, want %q", got, want)
		}
		if got := createCall.Provider; got != "" {
			t.Fatalf("Create().Provider = %q, want explicit empty provider", got)
		}
	})
}

func TestHostAPIHandlerSandboxListReturnsActiveSandboxInstances(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-env-list", []string{"sandbox/list"}, nil)
	sess := env.createSession(t)

	result, err := env.call(t, "ext-env-list", "sandbox/list", nil)
	if err != nil {
		t.Fatalf("Handle(sandbox/list) error = %v", err)
	}

	var listed hostAPISandboxListResult
	decodeResult(t, result, &listed)
	if len(listed.Sandboxes) != 1 {
		t.Fatalf("len(sandbox/list) = %d, want 1", len(listed.Sandboxes))
	}
	got := listed.Sandboxes[0]
	if got.SessionID != sess.ID {
		t.Fatalf("sandbox/list session_id = %q, want %q", got.SessionID, sess.ID)
	}
	if got.SandboxID == "" {
		t.Fatal("sandbox/list sandbox_id = empty, want allocated id")
	}
	if got.Backend != string(sandbox.BackendLocal) {
		t.Fatalf("sandbox/list backend = %q, want local", got.Backend)
	}
	if got.SyncState != "synced" {
		t.Fatalf("sandbox/list sync_state = %q, want synced", got.SyncState)
	}
}

func TestHostAPIHandlerSandboxListFiltersWorkspaceAndSkipsStopped(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-env-list-filtered", []string{"sandbox/list"}, nil)
	stopped := env.createSession(t)
	active := env.createSession(t)
	if err := env.sessions.Stop(testutil.Context(t), stopped.ID); err != nil {
		t.Fatalf("sessions.Stop(%q) error = %v", stopped.ID, err)
	}

	result, err := env.call(
		t,
		"ext-env-list-filtered",
		"sandbox/list",
		map[string]string{"workspace": env.workspace.Name},
	)
	if err != nil {
		t.Fatalf("Handle(sandbox/list filtered) error = %v", err)
	}

	var listed hostAPISandboxListResult
	decodeResult(t, result, &listed)
	if len(listed.Sandboxes) != 1 {
		t.Fatalf("len(sandbox/list filtered) = %d, want 1", len(listed.Sandboxes))
	}
	if got := listed.Sandboxes[0].SessionID; got != active.ID {
		t.Fatalf("sandbox/list filtered session_id = %q, want active session %q", got, active.ID)
	}
}

func TestHostAPIHandlerSandboxInfoReturnsRuntimeState(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-env-info", []string{"sandbox/info"}, nil)
	sess := env.createSession(t)

	meta := sess.Info().Sandbox
	if meta == nil {
		t.Fatal("session sandbox = nil, want prepared sandbox")
		return
	}

	result, err := env.call(t, "ext-env-info", "sandbox/info", map[string]string{
		"workspace_id": env.workspaceID,
		"session_id":   sess.ID,
	})
	if err != nil {
		t.Fatalf("Handle(sandbox/info) error = %v", err)
	}

	var info hostAPISandboxInfoResult
	decodeResult(t, result, &info)
	if info.SandboxID != meta.SandboxID {
		t.Fatalf("sandbox/info sandbox_id = %q, want %q", info.SandboxID, meta.SandboxID)
	}
	if info.RuntimeRoot != meta.RuntimeRootDir {
		t.Fatalf("sandbox/info runtime_root = %q, want %q", info.RuntimeRoot, meta.RuntimeRootDir)
	}
	if info.SyncState != "synced" {
		t.Fatalf("sandbox/info sync_state = %q, want synced", info.SyncState)
	}
	if info.LastSyncError != "" {
		t.Fatalf("sandbox/info last_sync_error = %q, want empty", info.LastSyncError)
	}
	var raw map[string]any
	decodeResult(t, result, &raw)
	if _, ok := raw["last_sync_error"]; !ok {
		t.Fatalf("sandbox/info result keys = %#v, want last_sync_error key", raw)
	}
}

func TestHostAPIHandlerSandboxInfoReturnsNotFoundForInvalidSession(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-env-info", []string{"sandbox/info"}, nil)

	_, err := env.call(t, "ext-env-info", "sandbox/info", map[string]string{
		"workspace_id": env.workspaceID,
		"session_id":   "missing",
	})
	assertRPCErrorCode(t, err, HostAPINotFoundCode)
}

func TestHostAPIHandlerSandboxInfoRejectsForeignWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Should reject foreign workspace sandbox info requests", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		env.grant("ext-env-info", []string{"sandbox/info"}, nil)
		sess := env.createSession(t)
		foreign := env.addForeignWorkspace(t)

		_, err := env.call(t, "ext-env-info", "sandbox/info", map[string]string{
			"workspace_id": foreign.WorkspaceID,
			"session_id":   sess.ID,
		})
		assertRPCErrorCode(t, err, HostAPINotFoundCode)
		assertErrorContains(t, err, "Not found")
	})
}

func TestHostAPIHandlerSandboxInfoValidatesSessionID(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-env-info-invalid", []string{"sandbox/info"}, nil)

	_, err := env.call(t, "ext-env-info-invalid", "sandbox/info", map[string]string{"session_id": " "})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
}

func TestHostAPISandboxSyncStateValues(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	tests := []struct {
		name string
		meta *store.SessionSandboxMeta
		want string
	}{
		{name: "nil", want: ""},
		{name: "pending", meta: &store.SessionSandboxMeta{}, want: "pending"},
		{name: "synced", meta: &store.SessionSandboxMeta{LastSyncAt: &now}, want: "synced"},
		{name: "error", meta: &store.SessionSandboxMeta{LastSyncError: "failed"}, want: extensionStateError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := hostAPISandboxSyncState(tc.meta); got != tc.want {
				t.Fatalf("hostAPISandboxSyncState() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHostAPIHandlerResolveSandboxWorkspaceFilter(t *testing.T) {
	t.Parallel()

	ctx := testutil.Context(t)
	handler := &HostAPIHandler{}
	id, root, err := handler.resolveSandboxWorkspaceFilter(ctx, " workspace-raw ")
	if err != nil {
		t.Fatalf("resolveSandboxWorkspaceFilter(raw) error = %v", err)
	}
	if id != "workspace-raw" || root != "workspace-raw" {
		t.Fatalf("resolveSandboxWorkspaceFilter(raw) = (%q, %q), want raw fallback", id, root)
	}

	workspace := &workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      "ws-id",
			Name:    "workspace-name",
			RootDir: filepath.Join(t.TempDir(), "workspace"),
		},
	}
	handler.workspaces = newHostAPIFakeWorkspaceResolver(workspace)
	id, root, err = handler.resolveSandboxWorkspaceFilter(ctx, "workspace-name")
	if err != nil {
		t.Fatalf("resolveSandboxWorkspaceFilter(resolved) error = %v", err)
	}
	if id != workspace.ID || root != workspace.RootDir {
		t.Fatalf("resolveSandboxWorkspaceFilter(resolved) = (%q, %q), want (%q, %q)",
			id,
			root,
			workspace.ID,
			workspace.RootDir,
		)
	}

	if _, _, err := handler.resolveSandboxWorkspaceFilter(ctx, "missing"); err == nil {
		t.Fatal("resolveSandboxWorkspaceFilter(missing) error = nil, want error")
	}
}

func TestHostAPIHandlerSandboxMethodsRequireSessionManager(t *testing.T) {
	t.Parallel()

	handler := &HostAPIHandler{}
	ctx := testutil.Context(t)
	for _, method := range []struct {
		name string
		call func(context.Context, json.RawMessage) (any, error)
	}{
		{name: "list", call: handler.handleSandboxList},
		{name: "info", call: handler.handleSandboxInfo},
		{name: "exec", call: handler.handleSandboxExec},
	} {
		t.Run(method.name, func(t *testing.T) {
			if _, err := method.call(ctx, nil); err == nil {
				t.Fatal("sandbox Host API handler error = nil, want missing session manager error")
			}
		})
	}
}

func TestHostAPIHandlerSandboxExecRequiresPermission(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-env-exec-denied", nil, nil)
	sess := env.createSession(t)

	_, err := env.call(t, "ext-env-exec-denied", "sandbox/exec", map[string]any{
		"workspace_id": env.workspaceID,
		"session_id":   sess.ID,
		"command":      "printf denied",
		"timeout":      1,
	})
	assertCapabilityDenied(t, err, "sandbox/exec")
}

func TestHostAPIHandlerSandboxExecRunsCommandInSandbox(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-env-exec", []string{"sandbox/exec"}, []string{"sandbox.exec"})
	sess := env.createSession(t)

	result, err := env.call(t, "ext-env-exec", "sandbox/exec", map[string]any{
		"workspace_id": env.workspaceID,
		"session_id":   sess.ID,
		"command":      "printf host-api-env",
		"timeout":      5,
	})
	if err != nil {
		t.Fatalf("Handle(sandbox/exec) error = %v", err)
	}

	var execResult hostAPISandboxExecResult
	decodeResult(t, result, &execResult)
	if execResult.ExitCode != 0 {
		t.Fatalf("sandbox/exec exit_code = %d, want 0", execResult.ExitCode)
	}
	if strings.TrimSpace(execResult.Stdout) != "host-api-env" {
		t.Fatalf("sandbox/exec stdout = %q, want host-api-env", execResult.Stdout)
	}
	if execResult.Stderr != "" {
		t.Fatalf("sandbox/exec stderr = %q, want empty", execResult.Stderr)
	}
}

func TestHostAPIHandlerSandboxExecRejectsForeignWorkspace(t *testing.T) {
	t.Parallel()

	t.Run("Should reject foreign workspace sandbox exec requests", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		env.grant("ext-env-exec", []string{"sandbox/exec"}, []string{"sandbox.exec"})
		sess := env.createSession(t)
		foreign := env.addForeignWorkspace(t)

		_, err := env.call(t, "ext-env-exec", "sandbox/exec", map[string]any{
			"workspace_id": foreign.WorkspaceID,
			"session_id":   sess.ID,
			"command":      "printf should-not-run",
			"timeout":      5,
		})
		assertRPCErrorCode(t, err, HostAPINotFoundCode)
		assertErrorContains(t, err, "Not found")
	})
}

func TestHostAPIHandlerSandboxExecValidatesParams(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-env-exec-invalid", []string{"sandbox/exec"}, []string{"sandbox.exec"})

	tests := []struct {
		name   string
		params map[string]any
	}{
		{
			name:   "missing session id",
			params: map[string]any{"command": "pwd"},
		},
		{
			name:   "missing command",
			params: map[string]any{"workspace_id": env.workspaceID, "session_id": "sess-1"},
		},
		{
			name: "negative timeout",
			params: map[string]any{
				"workspace_id": env.workspaceID,
				"session_id":   "sess-1",
				"command":      "pwd",
				"timeout":      -1,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := env.call(t, "ext-env-exec-invalid", "sandbox/exec", tc.params)
			assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
		})
	}
}

func TestHostAPIHandlerSessionsEventsSupportsSinceFilter(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-events", []string{"sessions/events", "sessions/prompt"}, []string{"session.read", "session.write"})

	sess := env.createSession(t)
	if _, err := env.call(t, "ext-events", "sessions/events", map[string]any{
		"workspace_id": env.workspaceID,
		"session_id":   sess.ID,
		"limit":        10,
	}); err != nil {
		t.Fatalf("Handle(sessions/events baseline) error = %v", err)
	}

	since := env.currentTime().Add(-time.Second).Format(time.RFC3339Nano)
	if _, err := env.submitPrompt(t, "ext-events", sess.ID, "show me the timeline"); err != nil {
		t.Fatalf("submitPrompt() error = %v", err)
	}

	result, err := env.call(t, "ext-events", "sessions/events", map[string]any{
		"workspace_id": env.workspaceID,
		"session_id":   sess.ID,
		"since":        since,
		"limit":        10,
	})
	if err != nil {
		t.Fatalf("Handle(sessions/events) error = %v", err)
	}

	var events []hostAPISessionEvent
	decodeResult(t, result, &events)
	if len(events) == 0 {
		t.Fatal("sessions/events len = 0, want prompt-related events")
	}
}

func TestHostAPIHandlerSessionsMethodsRequireConfiguredManager(t *testing.T) {
	t.Parallel()

	checker := &CapabilityChecker{}
	checker.Register("ext-sessions", SourceUser, &Manifest{
		Permissions: PermissionsConfig{Requires: []string{"sessions/stop", "sessions/status", "sessions/events"}},
	})

	handler := NewHostAPIHandler(
		nil,
		nil,
		nil,
		nil,
		WithHostAPICapabilityChecker(checker),
		WithHostAPIRateLimit(1000, 1000),
	)

	tests := []struct {
		name   string
		method string
		params any
	}{
		{
			name:   "ShouldRejectStopWithoutManager",
			method: "sessions/stop",
			params: map[string]any{"workspace_id": "ws-1", "session_id": "sess-1"},
		},
		{
			name:   "ShouldRejectStatusWithoutManager",
			method: "sessions/status",
			params: map[string]any{"workspace_id": "ws-1", "session_id": "sess-1"},
		},
		{
			name:   "ShouldRejectEventsWithoutManager",
			method: "sessions/events",
			params: map[string]any{"workspace_id": "ws-1", "session_id": "sess-1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params, err := marshalParams(tt.params)
			if err != nil {
				t.Fatalf("marshalParams() error = %v", err)
			}

			_, err = handler.Handle(testutil.Context(t), "ext-sessions", tt.method, params)
			assertErrorContains(t, err, "session manager is not configured")
		})
	}
}

func TestHostAPIHandlerResourcesListAndGetEnforceSameSourceAndGrantedKinds(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grantWithResources(
		t,
		"ext-resources",
		[]string{"resources/list", "resources/get", "resources/snapshot"},
		[]string{"resources.read", "resources.write"},
		[]string{"tools"},
		resources.ResourceScopeKindWorkspace,
	)

	sessionNonce := "nonce-resources"
	env.activateResourceSession(t, "ext-resources", sessionNonce)

	if _, err := env.callResource(t, "ext-resources", sessionNonce, "resources/snapshot", map[string]any{
		"source_version": 1,
		"records": []map[string]any{
			{
				"kind":  "tool",
				"id":    "grep",
				"scope": map[string]any{"kind": "workspace", "id": env.workspaceID},
				"spec":  hostAPITestToolSpec("grep", "search workspace", toolspkg.ToolSourceExtension.String()),
			},
		},
	}); err != nil {
		t.Fatalf("Handle(resources/snapshot) error = %v", err)
	}

	if _, err := env.resources.PutRaw(testutil.Context(t), resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "host-api-tests",
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   "host-api-tests",
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
	}, resources.RawDraft{
		Kind:  resources.ResourceKind("tool"),
		ID:    "foreign",
		Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: env.workspaceID},
		SpecJSON: mustMarshalJSON(t, map[string]any{
			"command": "foreign",
		}),
	}); err != nil {
		t.Fatalf("PutRaw(foreign) error = %v", err)
	}

	listResult, err := env.callResource(t, "ext-resources", sessionNonce, "resources/list", map[string]any{
		"kind": "tool",
	})
	if err != nil {
		t.Fatalf("Handle(resources/list) error = %v", err)
	}

	var listed []hostAPIResourceRecord
	decodeResult(t, listResult, &listed)
	if got, want := len(listed), 1; got != want {
		t.Fatalf("len(resources/list) = %d, want %d", got, want)
	}
	if got, want := listed[0].ID, "grep"; got != want {
		t.Fatalf("resources/list[0].id = %q, want %q", got, want)
	}

	getResult, err := env.callResource(t, "ext-resources", sessionNonce, "resources/get", map[string]any{
		"kind": "tool",
		"id":   "grep",
	})
	if err != nil {
		t.Fatalf("Handle(resources/get own) error = %v", err)
	}

	var own hostAPIResourceRecord
	decodeResult(t, getResult, &own)
	if got, want := own.ID, "grep"; got != want {
		t.Fatalf("resources/get own id = %q, want %q", got, want)
	}

	_, err = env.callResource(t, "ext-resources", sessionNonce, "resources/get", map[string]any{
		"kind": "tool",
		"id":   "foreign",
	})
	assertRPCErrorCode(t, err, 403)

	_, err = env.callResource(t, "ext-resources", sessionNonce, "resources/list", map[string]any{
		"kind": "mcp_server",
	})
	assertRPCErrorCode(t, err, 403)
}

func TestHostAPIHandlerResourcesSnapshotRejectsStaleVersionAndInactiveNonce(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grantWithResources(
		t,
		"ext-snapshot",
		[]string{"resources/snapshot"},
		[]string{"resources.write"},
		[]string{"tools"},
		resources.ResourceScopeKindWorkspace,
	)

	sessionNonce := "nonce-active"
	env.activateResourceSession(t, "ext-snapshot", sessionNonce)

	params := map[string]any{
		"source_version": 1,
		"records": []map[string]any{
			{
				"kind":  "tool",
				"id":    "grep",
				"scope": map[string]any{"kind": "workspace", "id": env.workspaceID},
				"spec":  hostAPITestToolSpec("grep", "search workspace", toolspkg.ToolSourceExtension.String()),
			},
		},
	}
	if _, err := env.callResource(t, "ext-snapshot", sessionNonce, "resources/snapshot", params); err != nil {
		t.Fatalf("first Handle(resources/snapshot) error = %v", err)
	}

	_, err := env.callResource(t, "ext-snapshot", sessionNonce, "resources/snapshot", params)
	assertRPCErrorCode(t, err, 409)

	env.activateResourceSession(t, "ext-snapshot", "nonce-next")

	_, err = env.callResource(t, "ext-snapshot", sessionNonce, "resources/snapshot", map[string]any{
		"source_version": 2,
		"records":        params["records"],
	})
	assertRPCErrorCode(t, err, 409)
}

func TestHostAPIHandlerResourcesMethodsCoexistWithBridgeOperationalMethods(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grantWithResources(
		t,
		"telegram-adapter",
		[]string{"resources/list", "bridges/instances/list", "bridges/instances/get"},
		[]string{"resources.read", "bridge.read"},
		[]string{"tools"},
		resources.ResourceScopeKindWorkspace,
	)

	sessionNonce := "nonce-bridge"
	env.activateResourceSession(t, "telegram-adapter", sessionNonce)
	if _, err := env.callResource(t, "telegram-adapter", sessionNonce, "resources/snapshot", map[string]any{
		"source_version": 1,
		"records": []map[string]any{
			{
				"kind":  "tool",
				"id":    "grep",
				"scope": map[string]any{"kind": "workspace", "id": env.workspaceID},
				"spec":  hostAPITestToolSpec("grep", "search workspace", toolspkg.ToolSourceExtension.String()),
			},
		},
	}); err == nil {
		t.Fatal("Handle(resources/snapshot) error = nil, want capability denial without resources/snapshot action")
	} else {
		assertRPCErrorCode(t, err, 403)
	}

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-coexist",
		ExtensionName: "telegram-adapter",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, instance)

	listedResult, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/list", nil)
	if err != nil {
		t.Fatalf("Handle(bridges/instances/list) error = %v", err)
	}

	var listed []bridgecontract.BridgeInstance
	decodeResult(t, listedResult, &listed)
	if got, want := len(listed), 1; got != want {
		t.Fatalf("len(bridges/instances/list) = %d, want %d", got, want)
	}

	_, err = env.callResource(t, "telegram-adapter", sessionNonce, "resources/list", map[string]any{
		"kind": "bridge.instance",
	})
	assertRPCErrorCode(t, err, 403)
}

func TestHostAPIHandlerMemoryStorePersistsContentWithTags(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-memory", []string{"memory/store"}, []string{"memory.write"})

	if _, err := env.call(t, "ext-memory", "memory/store", map[string]any{
		"key":     "deploy-script",
		"content": "The deploy script is documented in the release handbook as deploy.sh.",
		"tags":    []string{"project-knowledge", "reference"},
	}); err != nil {
		t.Fatalf("Handle(memory/store) error = %v", err)
	}

	content, err := env.memory.Read(t.Context(), memcontract.ScopeProfile, "deploy-script.md")
	if err != nil {
		t.Fatalf("memory.Read() error = %v", err)
	}
	if !strings.Contains(string(content), "deploy.sh") {
		t.Fatalf("stored content = %q, want deploy script reference", string(content))
	}
	if !strings.Contains(string(content), "compozy-tags: project-knowledge, reference") {
		t.Fatalf("stored content = %q, want persisted tag comment", string(content))
	}
}

func TestHostAPIHandlerMemoryRecallReturnsRankedMatches(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-memory", []string{"memory/store", "memory/recall"}, []string{"memory.write", "memory.read"})

	if _, err := env.call(t, "ext-memory", "memory/store", map[string]any{
		"key":     "deploy-script",
		"content": "The deploy script is documented in the release handbook as deploy.sh.",
		"tags":    []string{"reference"},
	}); err != nil {
		t.Fatalf("Handle(memory/store) error = %v", err)
	}

	result, err := env.call(t, "ext-memory", "memory/recall", map[string]any{
		"query": "deploy script release handbook",
		"limit": 5,
	})
	if err != nil {
		t.Fatalf("Handle(memory/recall) error = %v", err)
	}

	var entries []hostAPIMemoryRecallEntry
	decodeResult(t, result, &entries)
	if len(entries) == 0 {
		t.Fatal("memory/recall entries = 0, want at least one match")
	}
	if !strings.Contains(entries[0].Content, "deploy.sh") {
		t.Fatalf("memory/recall first content = %q, want deploy.sh", entries[0].Content)
	}
	if entries[0].Score <= 0 {
		t.Fatalf("memory/recall first score = %f, want > 0", entries[0].Score)
	}

	t.Run("Should isolate profile memory by the bridge owner", func(t *testing.T) {
		t.Parallel()

		profileEnv := newHostAPITestEnv(t)
		profileEnv.grant(
			"ext-memory",
			[]string{"memory/store", "memory/recall"},
			[]string{"memory.write", "memory.read"},
		)
		ctx := profileEnv.bridgeContext(t, &bridgepkg.BridgeInstance{
			ID: "bridge-marketing-memory", ExtensionName: "ext-memory", ProfileID: profileEnv.marketingID,
		})
		if _, err := profileEnv.callWithContext(ctx, t, "ext-memory", "memory/store", map[string]any{
			"key": "campaign", "content": "The marketing campaign uses the aurora launch message.",
		}); err != nil {
			t.Fatalf("Handle(marketing memory/store) error = %v", err)
		}
		if _, err := profileEnv.memory.Read(t.Context(), memcontract.ScopeProfile, "campaign.md"); err == nil {
			t.Fatal("default profile memory contains marketing campaign, want isolation")
		}
		result, err := profileEnv.callWithContext(ctx, t, "ext-memory", "memory/recall", map[string]any{
			"query": "aurora launch campaign", "limit": 5,
		})
		if err != nil {
			t.Fatalf("Handle(marketing memory/recall) error = %v", err)
		}
		var profileEntries []hostAPIMemoryRecallEntry
		decodeResult(t, result, &profileEntries)
		if len(profileEntries) == 0 || !strings.Contains(profileEntries[0].Content, "aurora") {
			t.Fatalf("marketing memory/recall entries = %#v, want isolated campaign", profileEntries)
		}
	})
}

func TestHostAPIHandlerMemoryRecallUsesActiveProvider(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-memory", []string{"memory/recall"}, []string{"memory.read"})
	provider := &recordingHostAPIRecallProvider{
		packaged: memcontract.Packaged{Blocks: []memcontract.Block{{
			Scope: memcontract.ScopeWorkspace,
			Entries: []memcontract.PackagedEntry{{
				ID:    "provider/chunk-1",
				Body:  "Provider-backed recall result",
				Title: "Provider Result",
			}},
		}}},
	}
	registry := NewMemoryProviderRegistry()
	if err := registry.Register(testutil.Context(t), MemoryProviderRegistration{
		Name:          "local",
		Version:       "test",
		ExtensionName: "provider-ext",
		Provider:      provider,
	}); err != nil {
		t.Fatalf("Register(provider) error = %v", err)
	}
	env.handler.memoryProviders = registry

	result, err := env.call(t, "ext-memory", "memory/recall", map[string]any{
		"query":     "provider recall result",
		"workspace": env.workspaceID,
		"limit":     1,
	})
	if err != nil {
		t.Fatalf("Handle(memory/recall provider) error = %v", err)
	}

	if got := provider.lastRequest().WorkspaceID; got != env.workspaceID {
		t.Fatalf("provider workspace_id = %q, want %q", got, env.workspaceID)
	}
	var entries []hostAPIMemoryRecallEntry
	decodeResult(t, result, &entries)
	if got, want := len(entries), 1; got != want {
		t.Fatalf("provider recall entries = %d, want %d", got, want)
	}
	if entries[0].Content != "Provider-backed recall result" {
		t.Fatalf("provider recall content = %q", entries[0].Content)
	}
}

func TestHostAPIHandlerMemoryRecallRequiresConfiguredStore(t *testing.T) {
	t.Parallel()

	checker := &CapabilityChecker{}
	checker.Register("ext-memory", SourceUser, &Manifest{
		Permissions: PermissionsConfig{Requires: []string{"memory/recall"}},
	})

	handler := NewHostAPIHandler(
		nil,
		nil,
		nil,
		nil,
		WithHostAPICapabilityChecker(checker),
		WithHostAPIRateLimit(1000, 1000),
	)

	params, err := marshalParams(map[string]any{"query": "needle"})
	if err != nil {
		t.Fatalf("marshalParams() error = %v", err)
	}

	_, err = handler.Handle(testutil.Context(t), "ext-memory", "memory/recall", params)
	assertErrorContains(t, err, "memory store is not configured")
}

func TestHostAPIHandlerMemoryForgetRemovesEntries(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-memory", []string{"memory/store", "memory/forget"}, []string{"memory.write"})

	if _, err := env.call(t, "ext-memory", "memory/store", map[string]any{
		"key":     "scratch",
		"content": "temporary note",
	}); err != nil {
		t.Fatalf("Handle(memory/store) error = %v", err)
	}
	if _, err := env.call(t, "ext-memory", "memory/forget", map[string]any{"key": "scratch"}); err != nil {
		t.Fatalf("Handle(memory/forget) error = %v", err)
	}

	if _, err := env.memory.Read(t.Context(), memcontract.ScopeProfile, "scratch.md"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("memory.Read() error = %v, want os.ErrNotExist", err)
	}
}

func TestHostAPIHandlerObserveHealthReturnsSnapshot(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-observe", []string{"observe/health"}, []string{"observe.read"})

	env.createSession(t)
	result, err := env.call(t, "ext-observe", "observe/health", nil)
	if err != nil {
		t.Fatalf("Handle(observe/health) error = %v", err)
	}

	var health observepkg.Health
	decodeResult(t, result, &health)
	if health.ActiveSessions != 1 {
		t.Fatalf("observe/health active_sessions = %d, want 1", health.ActiveSessions)
	}
	if health.Status != "ok" {
		t.Fatalf("observe/health status = %q, want ok", health.Status)
	}
}

func TestHostAPIHandlerListLogsReturnsFilteredEventsWithSince(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-observe", []string{"sessions/prompt", "logs/list"}, []string{"session.write", "logs.read"})

	sess := env.createSession(t)
	since := env.currentTime().Add(-time.Second).Format(time.RFC3339Nano)
	if _, err := env.submitPrompt(t, "ext-observe", sess.ID, "collect observe event"); err != nil {
		t.Fatalf("submitPrompt() error = %v", err)
	}

	result, err := env.call(t, "ext-observe", "logs/list", map[string]any{
		"workspace_id": env.workspaceID,
		"session_id":   sess.ID,
		"since":        since,
		"limit":        20,
	})
	if err != nil {
		t.Fatalf("Handle(logs/list) error = %v", err)
	}

	var events []hostAPISessionEvent
	decodeResult(t, result, &events)
	if len(events) == 0 {
		t.Fatal("logs/list len = 0, want at least one event")
	}
}

func TestHostAPIHandlerListLogsRequiresPermission(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-observe", []string{"observe/health"}, nil)

	_, err := env.call(t, "ext-observe", "logs/list", map[string]any{
		"workspace_id": env.workspaceID,
		"limit":        1,
	})
	assertCapabilityDenied(t, err, "logs/list")
	data := decodeRPCData(t, err)
	required, ok := data["required"].([]any)
	if !ok || len(required) != 1 || required[0] != "logs/list" {
		t.Fatalf("rpc data required = %#v, want [logs/list]", data["required"])
	}
}

func TestHostAPIHandlerSkillsListReturnsWorkspaceSkills(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-skills", []string{"skills/list"}, []string{"skills.read"})

	result, err := env.call(t, "ext-skills", "skills/list", map[string]any{"workspace": env.workspaceID})
	if err != nil {
		t.Fatalf("Handle(skills/list) error = %v", err)
	}

	var listed []hostAPISkillSummary
	decodeResult(t, result, &listed)
	if len(listed) == 0 {
		t.Fatal("skills/list len = 0, want workspace skill")
	}
	if listed[0].Name != "workspace-review" {
		t.Fatalf("skills/list[0].Name = %q, want workspace-review", listed[0].Name)
	}
	if !listed[0].Enabled || !listed[0].Activation.Active {
		t.Fatalf("skills/list[0] = %#v, want enabled and active", listed[0])
	}
	if listed[0].Origin != "" {
		t.Fatalf("skills/list[0].Origin = %q, want compozy-native empty origin", listed[0].Origin)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal(skills/list result) error = %v", err)
	}
	if !bytes.Contains(encoded, []byte(`"origin":""`)) {
		t.Fatalf("skills/list result = %s, want explicit origin field", encoded)
	}
}

func TestHostAPIHandlerBridgesMessagesIngestRejectsInvalidPayloads(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-ingest-invalid",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	tests := []struct {
		name       string
		params     map[string]any
		wantText   string
		wantCode   int
		promptWant int
	}{
		{
			name: "MissingBridgeInstanceID",
			params: map[string]any{
				"scope":               instance.Scope,
				"workspace_id":        instance.WorkspaceID,
				"peer_id":             "peer-1",
				"platform_message_id": "msg-1",
				"received_at":         env.currentTime().Format(time.RFC3339Nano),
				"idempotency_key":     "idem-1",
				"content":             map[string]any{"text": "hello"},
			},
			wantText:   "bridge instance id",
			wantCode:   HostAPIInvalidParamsCode,
			promptWant: 0,
		},
		{
			name: "MissingPolicyRequiredPeer",
			params: map[string]any{
				"bridge_instance_id":  instance.ID,
				"scope":               instance.Scope,
				"workspace_id":        instance.WorkspaceID,
				"platform_message_id": "msg-2",
				"received_at":         env.currentTime().Format(time.RFC3339Nano),
				"idempotency_key":     "idem-2",
				"content":             map[string]any{"text": "hello"},
			},
			wantText:   "routing policy requires peer id",
			wantCode:   HostAPIInvalidParamsCode,
			promptWant: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := env.bridgeContext(t, instance)
			_, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", tt.params)
			assertRPCErrorCode(t, err, tt.wantCode)
			assertErrorContains(t, err, tt.wantText)
			if got := env.driver.promptCount(); got != tt.promptWant {
				t.Fatalf("driver.promptCount() = %d, want %d", got, tt.promptWant)
			}
		})
	}
}

func TestHostAPIHandlerBridgesMessagesIngestRejectsDisabledOrUnknownInstances(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

	disabled := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-ingest-disabled",
		Enabled:       false,
		Status:        bridgepkg.BridgeStatusDisabled,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	disabledCtx := env.bridgeContext(t, disabled)

	_, err := env.callWithContext(disabledCtx, t, "telegram-adapter", "bridges/messages/ingest", map[string]any{
		"bridge_instance_id":  disabled.ID,
		"scope":               disabled.Scope,
		"workspace_id":        disabled.WorkspaceID,
		"peer_id":             "peer-1",
		"platform_message_id": "msg-disabled",
		"received_at":         env.currentTime().Format(time.RFC3339Nano),
		"idempotency_key":     "idem-disabled",
		"content":             map[string]any{"text": "hello"},
	})
	assertRPCErrorCode(t, err, HostAPIUnavailableCode)
	assertErrorContains(t, err, "disabled")
	if got := env.driver.promptCount(); got != 0 {
		t.Fatalf("driver.promptCount() = %d, want 0", got)
	}

	ready := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-ingest-ready",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	readyCtx := env.bridgeContext(t, ready)

	_, err = env.callWithContext(readyCtx, t, "telegram-adapter", "bridges/messages/ingest", map[string]any{
		"bridge_instance_id":  "brg-missing",
		"scope":               ready.Scope,
		"workspace_id":        ready.WorkspaceID,
		"peer_id":             "peer-1",
		"platform_message_id": "msg-missing",
		"received_at":         env.currentTime().Format(time.RFC3339Nano),
		"idempotency_key":     "idem-missing",
		"content":             map[string]any{"text": "hello"},
	})
	assertRPCErrorCode(t, err, HostAPINotFoundCode)
	if got := env.driver.promptCount(); got != 0 {
		t.Fatalf("driver.promptCount() after unknown instance = %d, want 0", got)
	}
}

func TestHostAPIHandlerBridgesMessagesIngestSuppressesDuplicateWebhookRetries(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-ingest-dedup",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, instance)
	params := map[string]any{
		"bridge_instance_id":  instance.ID,
		"scope":               instance.Scope,
		"workspace_id":        instance.WorkspaceID,
		"peer_id":             "peer-1",
		"platform_message_id": "msg-dedup",
		"received_at":         env.currentTime().Format(time.RFC3339Nano),
		"idempotency_key":     "idem-dedup",
		"content":             map[string]any{"text": "hello"},
	}

	first, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", params)
	if err != nil {
		t.Fatalf("first ingest error = %v", err)
	}
	var firstResult bridgecontract.BridgesMessagesIngestResult
	decodeResult(t, first, &firstResult)

	firstRoute, err := env.bridges.ResolveRoute(
		testutil.Context(t),
		bridgeRoutingKeyDomain(firstResult.RoutingKey),
	)
	if err != nil {
		t.Fatalf("bridges.ResolveRoute(first) error = %v", err)
	}

	env.advanceTime(5 * time.Minute)

	second, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", params)
	if err != nil {
		t.Fatalf("duplicate ingest error = %v", err)
	}
	var secondResult bridgecontract.BridgesMessagesIngestResult
	decodeResult(t, second, &secondResult)

	secondRoute, err := env.bridges.ResolveRoute(
		testutil.Context(t),
		bridgeRoutingKeyDomain(secondResult.RoutingKey),
	)
	if err != nil {
		t.Fatalf("bridges.ResolveRoute(second) error = %v", err)
	}

	routes, err := env.bridges.ListRoutes(testutil.Context(t), instance.ID)
	if err != nil {
		t.Fatalf("bridges.ListRoutes() error = %v", err)
	}
	if got := len(routes); got != 1 {
		t.Fatalf("len(routes) = %d, want 1", got)
	}
	if got := env.driver.promptCount(); got != 1 {
		t.Fatalf("driver.promptCount() = %d, want 1", got)
	}
	if secondResult.SessionID != firstResult.SessionID {
		t.Fatalf("duplicate session_id = %q, want %q", secondResult.SessionID, firstResult.SessionID)
	}
	if !secondRoute.UpdatedAt.Equal(firstRoute.UpdatedAt) {
		t.Fatalf("duplicate retry updated route from %s to %s", firstRoute.UpdatedAt, secondRoute.UpdatedAt)
	}
}

func TestHostAPIHandlerBridgesInstancesReportStateRejectsInvalidUpdates(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/instances/report_state"}, []string{"bridge.write"})

	ready := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-report-state-ready",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	readyCtx := env.bridgeContext(t, ready)

	_, err := env.callWithContext(readyCtx, t, "telegram-adapter", "bridges/instances/report_state", map[string]any{
		"bridge_instance_id": ready.ID,
		"status":             "disabled",
	})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "operator-controlled")

	_, err = env.callWithContext(readyCtx, t, "telegram-adapter", "bridges/instances/report_state", map[string]any{
		"bridge_instance_id": ready.ID,
		"status":             "bogus",
	})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "unsupported bridge status")

	_, err = env.callWithContext(readyCtx, t, "telegram-adapter", "bridges/instances/report_state", map[string]any{
		"status": "ready",
	})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "bridge_instance_id is required")
}

func TestHostAPIHandlerBridgesInstancesReportStateRejectsConflictingDegradationControls(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/instances/report_state"}, []string{"bridge.write"})

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-report-state-conflict",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, instance)

	_, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/report_state", map[string]any{
		"bridge_instance_id": instance.ID,
		"status":             "degraded",
		"clear_degradation":  true,
		"degradation": map[string]any{
			"reason": "rate_limited",
		},
	})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "cannot be cleared and set together")
}

func TestHostAPIHandlerBridgesInstancesReportStateClearsDegradationOnRecovery(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant(
		"telegram-adapter",
		[]string{"bridges/instances/report_state", "bridges/instances/get"},
		[]string{"bridge.write", "bridge.read"},
	)

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID: store.DefaultProfileID,
		ID:        "brg-report-state-recovery",
		Enabled:   true,
		Status:    bridgepkg.BridgeStatusAuthRequired,
		Degradation: &bridgepkg.BridgeDegradation{
			Reason:  bridgepkg.BridgeDegradationReasonAuthFailed,
			Message: "expired",
		},
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, instance)

	result, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/report_state", map[string]any{
		"bridge_instance_id": instance.ID,
		"status":             "starting",
	})
	if err != nil {
		t.Fatalf("Handle(bridges/instances/report_state recovery) error = %v", err)
	}

	var updated bridgecontract.BridgeInstance
	decodeResult(t, result, &updated)
	if updated.Degradation != nil {
		t.Fatalf("updated.Degradation = %#v, want nil", updated.Degradation)
	}

	fetched, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/get", map[string]any{
		"bridge_instance_id": instance.ID,
	})
	if err != nil {
		t.Fatalf("Handle(bridges/instances/get recovery) error = %v", err)
	}

	var loaded bridgecontract.BridgeInstance
	decodeResult(t, fetched, &loaded)
	if loaded.Degradation != nil {
		t.Fatalf("loaded.Degradation = %#v, want nil", loaded.Degradation)
	}
}

func TestHostAPIHandlerBridgesInstancesGetRejectsMismatchedRuntimeOwnership(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/instances/get"}, []string{"bridge.read"})

	other := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-other-owner",
		ExtensionName: "discord-adapter",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, other)

	_, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/get", map[string]any{
		"bridge_instance_id": other.ID,
	})
	assertRPCErrorCode(t, err, HostAPINotFoundCode)
}

func TestHostAPIHandlerMethodHandlersExposeBridgeRuntimeAwareInstanceLookup(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/instances/get"}, []string{"bridge.read"})

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-method-handler",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})

	handlers := env.handler.MethodHandlers()
	handler, ok := handlers["bridges/instances/get"]
	if !ok {
		t.Fatal("MethodHandlers()[bridges/instances/get] = missing, want handler")
	}

	ctx := withHostAPIExtensionName(env.bridgeContext(t, instance), "telegram-adapter")
	result, err := handler(ctx, mustMarshalRawMessage(t, map[string]any{
		"bridge_instance_id": instance.ID,
	}))
	if err != nil {
		t.Fatalf("MethodHandlers()[bridges/instances/get]() error = %v", err)
	}

	var loaded bridgecontract.BridgeInstance
	decodeResult(t, result, &loaded)
	if loaded.ID != instance.ID {
		t.Fatalf("loaded.ID = %q, want %q", loaded.ID, instance.ID)
	}
}

func TestHostAPIHandlerBridgesInstancesListReturnsOwnedInstancesForProviderRuntime(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/instances/list", "bridges/instances/get"}, []string{"bridge.read"})

	first := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-owned-a",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	second := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-owned-b",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	_ = env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-foreign",
		ExtensionName: "discord-adapter",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})

	ctx := env.bridgeContextForInstances(t, first, second)

	listedResult, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/list", nil)
	if err != nil {
		t.Fatalf("Handle(bridges/instances/list) error = %v", err)
	}

	var listed []bridgecontract.BridgeInstance
	decodeResult(t, listedResult, &listed)
	if got := len(listed); got != 2 {
		t.Fatalf("len(listed) = %d, want 2", got)
	}
	if got, want := []string{listed[0].ID, listed[1].ID}, []string{first.ID, second.ID}; !slices.Equal(got, want) {
		t.Fatalf("listed ids = %#v, want %#v", got, want)
	}

	fetchedResult, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/get", map[string]any{
		"bridge_instance_id": second.ID,
	})
	if err != nil {
		t.Fatalf("Handle(bridges/instances/get) error = %v", err)
	}

	var fetched bridgecontract.BridgeInstance
	decodeResult(t, fetchedResult, &fetched)
	if got, want := fetched.ID, second.ID; got != want {
		t.Fatalf("fetched.ID = %q, want %q", got, want)
	}
}

func TestHostAPIHandlerBridgesInstancesListAllowsZeroManagedInstances(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/instances/list"}, []string{"bridge.read"})

	ctx := withHostAPIBridgeRuntime(testutil.Context(t), &subprocess.InitializeBridgeRuntime{
		RuntimeVersion: subprocess.InitializeBridgeRuntimeVersion2,
		Purpose:        subprocess.BridgeRuntimePurposeService,
		Provider:       "telegram-adapter",
		Platform:       "telegram",
	})

	result, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/instances/list", nil)
	if err != nil {
		t.Fatalf("Handle(bridges/instances/list zero) error = %v", err)
	}

	var listed []bridgecontract.BridgeInstance
	decodeResult(t, result, &listed)
	if len(listed) != 0 {
		t.Fatalf("len(listed) = %d, want 0", len(listed))
	}
}

func TestHostAPIHandlerBridgesMessagesIngestConcurrentSameRoutingKeyCreatesOneSessionAndRoute(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.useSessionsWithoutObserver(t)
	env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-ingest-concurrent",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, instance)

	type ingestResult struct {
		result bridgecontract.BridgesMessagesIngestResult
		err    error
	}

	results := make([]ingestResult, 2)
	var wg sync.WaitGroup
	for idx := range results {
		wg.Go(func() {
			res, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", map[string]any{
				"bridge_instance_id":  instance.ID,
				"scope":               instance.Scope,
				"workspace_id":        instance.WorkspaceID,
				"peer_id":             "peer-1",
				"platform_message_id": fmt.Sprintf("msg-%d", idx),
				"received_at":         env.currentTime().Format(time.RFC3339Nano),
				"idempotency_key":     fmt.Sprintf("idem-%d", idx),
				"content":             map[string]any{"text": fmt.Sprintf("hello-%d", idx)},
			})
			if err != nil {
				results[idx].err = err
				return
			}
			decodeResult(t, res, &results[idx].result)
		})
	}
	wg.Wait()

	for idx, result := range results {
		if result.err != nil {
			t.Fatalf("ingest[%d] error = %v", idx, result.err)
		}
	}

	routes, err := env.bridges.ListRoutes(testutil.Context(t), instance.ID)
	if err != nil {
		t.Fatalf("bridges.ListRoutes() error = %v", err)
	}
	if got := len(routes); got != 1 {
		t.Fatalf("len(routes) = %d, want 1", got)
	}

	sessions, err := env.sessions.ListAll(testutil.Context(t))
	if err != nil {
		t.Fatalf("sessions.ListAll() error = %v", err)
	}
	if got := len(sessions); got != 1 {
		t.Fatalf("len(sessions) = %d, want 1", got)
	}
	if results[0].result.SessionID != results[1].result.SessionID {
		t.Fatalf("session IDs = %q and %q, want same session", results[0].result.SessionID, results[1].result.SessionID)
	}
	if got := env.driver.promptCount(); got != 2 {
		t.Fatalf("driver.promptCount() = %d, want 2", got)
	}
}

func TestHostAPIHandlerBridgesMessagesIngestRebindsStaleRouteToReplacementSession(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-ingest-rebind",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, instance)

	key, err := env.bridges.BuildRoutingKey(testutil.Context(t), bridgepkg.RoutingKey{
		BridgeInstanceID: instance.ID,
		Scope:            instance.Scope,
		WorkspaceID:      instance.WorkspaceID,
		PeerID:           "peer-1",
	})
	if err != nil {
		t.Fatalf("bridges.BuildRoutingKey() error = %v", err)
	}
	if _, err := env.bridges.UpsertRoute(testutil.Context(t), bridgepkg.BridgeRoute{
		Scope:            key.Scope,
		WorkspaceID:      key.WorkspaceID,
		BridgeInstanceID: key.BridgeInstanceID,
		PeerID:           key.PeerID,
		SessionID:        "missing-session",
		AgentName:        "coder",
	}); err != nil {
		t.Fatalf("bridges.UpsertRoute() error = %v", err)
	}

	result, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", map[string]any{
		"bridge_instance_id":  instance.ID,
		"scope":               instance.Scope,
		"workspace_id":        instance.WorkspaceID,
		"peer_id":             "peer-1",
		"platform_message_id": "msg-rebind",
		"received_at":         env.currentTime().Format(time.RFC3339Nano),
		"idempotency_key":     "idem-rebind",
		"content":             map[string]any{"text": "hello"},
	})
	if err != nil {
		t.Fatalf("Handle(bridges/messages/ingest) error = %v", err)
	}

	var ingest bridgecontract.BridgesMessagesIngestResult
	decodeResult(t, result, &ingest)
	if ingest.SessionID == "missing-session" {
		t.Fatal("ingest session_id = missing-session, want replacement session")
	}

	route, err := env.bridges.ResolveRoute(testutil.Context(t), key)
	if err != nil {
		t.Fatalf("bridges.ResolveRoute() error = %v", err)
	}
	if route.SessionID != ingest.SessionID {
		t.Fatalf("route.SessionID = %q, want %q", route.SessionID, ingest.SessionID)
	}
	if got := env.driver.promptCount(); got != 1 {
		t.Fatalf("driver.promptCount() = %d, want 1", got)
	}
}

func TestHostAPIHandlerBridgesMessagesIngestExpiredDedupAllowsReingest(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-ingest-expiry",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, instance)
	params := map[string]any{
		"bridge_instance_id":  instance.ID,
		"scope":               instance.Scope,
		"workspace_id":        instance.WorkspaceID,
		"peer_id":             "peer-1",
		"platform_message_id": "msg-expiry",
		"received_at":         env.currentTime().Format(time.RFC3339Nano),
		"idempotency_key":     "idem-expiry",
		"content":             map[string]any{"text": "hello"},
	}

	if _, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", params); err != nil {
		t.Fatalf("first ingest error = %v", err)
	}
	if got := env.driver.promptCount(); got != 1 {
		t.Fatalf("driver.promptCount() after first ingest = %d, want 1", got)
	}

	env.advanceTime(20 * time.Minute)
	if _, err := env.registry.GetBridgeIngestDedup(
		testutil.Context(t),
		"idem-expiry",
		env.currentTime(),
	); !errors.Is(
		err,
		bridgepkg.ErrIngestDedupRecordNotFound,
	) {
		t.Fatalf("GetBridgeIngestDedup(expired) error = %v, want ErrIngestDedupRecordNotFound", err)
	}

	if _, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", params); err != nil {
		t.Fatalf("second ingest after expiry error = %v", err)
	}
	if got := env.driver.promptCount(); got != 2 {
		t.Fatalf("driver.promptCount() after reingest = %d, want 2", got)
	}

	if _, err := env.registry.GetBridgeIngestDedup(testutil.Context(t), "idem-expiry", env.currentTime()); err != nil {
		t.Fatalf("GetBridgeIngestDedup(refreshed) error = %v", err)
	}
}

func TestHostAPIHandlerBridgesMessagesIngestRegistersPromptDelivery(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("telegram-adapter", []string{"bridges/messages/ingest"}, []string{"bridge.write"})

	broker := &recordingPromptDeliveryBroker{}
	env.handler = NewHostAPIHandler(
		env.sessions,
		env.memory,
		env.observer,
		env.skills,
		WithHostAPICapabilityChecker(env.checker),
		WithHostAPIWorkspaceResolver(env.workspaces),
		WithHostAPIBridgeRegistry(env.bridges),
		WithHostAPIBridgeDedupStore(env.registry),
		WithHostAPIDeliveryBroker(broker),
		WithHostAPINow(func() time.Time { return env.currentTime() }),
		WithHostAPIBridgeIngressConfig(15*time.Minute, time.Minute),
		WithHostAPIRateLimit(1000, 1000),
	)

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-ingest-register",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	ctx := env.bridgeContext(t, instance)
	params := map[string]any{
		"bridge_instance_id":  instance.ID,
		"scope":               instance.Scope,
		"workspace_id":        instance.WorkspaceID,
		"peer_id":             "peer-1",
		"platform_message_id": "msg-register",
		"received_at":         env.currentTime().Format(time.RFC3339Nano),
		"idempotency_key":     "idem-register",
		"content":             map[string]any{"text": "hello"},
	}

	if _, err := env.callWithContext(ctx, t, "telegram-adapter", "bridges/messages/ingest", params); err != nil {
		t.Fatalf("Handle(bridges/messages/ingest) error = %v", err)
	}

	regs := broker.snapshotRegistrations()
	if len(regs) != 1 {
		t.Fatalf("len(prompt delivery registrations) = %d, want 1", len(regs))
	}
	reg := regs[0]
	if reg.SessionID == "" {
		t.Fatal("registration session id = empty, want non-empty")
	}
	if reg.TurnID == "" {
		t.Fatal("registration turn id = empty, want non-empty")
	}
	if got, want := reg.ExtensionName, instance.ExtensionName; got != want {
		t.Fatalf("registration extension = %q, want %q", got, want)
	}
	if got, want := reg.RoutingKey.BridgeInstanceID, instance.ID; got != want {
		t.Fatalf("registration routing key instance = %q, want %q", got, want)
	}
	if got, want := reg.RoutingKey.PeerID, "peer-1"; got != want {
		t.Fatalf("registration routing key peer = %q, want %q", got, want)
	}
	if got, want := reg.DeliveryTarget.Mode, bridgepkg.DeliveryModeReply; got != want {
		t.Fatalf("registration delivery mode = %q, want %q", got, want)
	}

	eventTypes := make([]string, 0, len(reg.SeedEvents))
	for _, event := range reg.SeedEvents {
		eventTypes = append(eventTypes, event.Type)
	}
	if !slices.Contains(eventTypes, acp.EventTypeUserMessage) {
		t.Fatalf("registration seed event types = %#v, want user_message from prompt boundary seed", eventTypes)
	}
}

func TestHostAPIHandlerRegisterPromptDeliveryReplaysStoredPromptEvents(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("delivery-replayer", []string{"sessions/prompt"}, []string{"session.write"})
	turnEnded := make(chan string, 1)
	env.sessions.SetTurnEndNotifier(func(sessionID string) {
		select {
		case turnEnded <- sessionID:
		default:
		}
	})

	broker := &recordingPromptDeliveryBroker{}
	env.handler = NewHostAPIHandler(
		env.sessions,
		env.memory,
		env.observer,
		env.skills,
		WithHostAPICapabilityChecker(env.checker),
		WithHostAPIWorkspaceResolver(env.workspaces),
		WithHostAPIBridgeRegistry(env.bridges),
		WithHostAPIBridgeDedupStore(env.registry),
		WithHostAPIDeliveryBroker(broker),
		WithHostAPINow(func() time.Time { return env.currentTime() }),
		WithHostAPIBridgeIngressConfig(15*time.Minute, time.Minute),
		WithHostAPIRateLimit(1000, 1000),
	)

	sess := env.createSession(t)
	prompt, err := env.submitPrompt(t, "delivery-replayer", sess.ID, "replay me")
	if err != nil {
		t.Fatalf("submitPrompt() error = %v", err)
	}

	select {
	case notifiedSessionID := <-turnEnded:
		if got, want := notifiedSessionID, sess.ID; got != want {
			t.Fatalf("turn end notifier session id = %q, want %q", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for prompt completion")
	}

	promptEvents, err := env.sessions.Events(testutil.Context(t), sess.ID, store.EventQuery{TurnID: prompt.TurnID})
	if err != nil {
		t.Fatalf("sessions.Events(%q) error = %v", sess.ID, err)
	}
	if !slices.ContainsFunc(promptEvents, func(storedEvent store.SessionEvent) bool {
		return strings.TrimSpace(storedEvent.Type) == acp.EventTypeDone
	}) {
		t.Fatalf("prompt events = %#v, want done event after turn completion notification", promptEvents)
	}

	instance := env.createBridgeInstance(t, bridgepkg.CreateInstanceRequest{
		ProfileID:     store.DefaultProfileID,
		ID:            "brg-register-replay",
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	routingKey, err := env.bridges.BuildRoutingKey(testutil.Context(t), bridgepkg.RoutingKey{
		Scope:            instance.Scope,
		WorkspaceID:      instance.WorkspaceID,
		BridgeInstanceID: instance.ID,
		PeerID:           "peer-1",
	})
	if err != nil {
		t.Fatalf("BuildRoutingKey() error = %v", err)
	}

	if err := env.handler.registerPromptDeliveryAfterSubmission(
		testutil.Context(t),
		*instance,
		routingKey,
		sess.ID,
		hostAPIPromptSubmission{
			TurnID: prompt.TurnID,
			SeedEvents: []bridgepkg.DeliveryProjectionEvent{{
				Type:      acp.EventTypeUserMessage,
				TurnID:    prompt.TurnID,
				Timestamp: env.currentTime(),
				Text:      "replay me",
			}},
		},
	); err != nil {
		t.Fatalf("registerPromptDelivery() error = %v", err)
	}

	projected := broker.snapshotProjectedEvents()
	projectedTypes := make([]string, 0, len(projected))
	for _, event := range projected {
		projectedTypes = append(projectedTypes, event.Type)
	}
	if !slices.Contains(projectedTypes, acp.EventTypeAgentMessage) {
		t.Fatalf("projected event types = %#v, want agent_message replay", projectedTypes)
	}
	if !slices.Contains(projectedTypes, acp.EventTypeDone) {
		t.Fatalf("projected event types = %#v, want done replay", projectedTypes)
	}
}

func TestPromptSubmissionFromStoredEventsUsesSyntheticBoundary(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 18, 14, 0, 0, 0, time.UTC)
	events := []store.SessionEvent{
		mustStoredPromptEvent(t, "ev-synth", 1, acp.AgentEvent{
			Type:      acp.EventTypeSyntheticReentry,
			TurnID:    "turn-synth",
			Timestamp: now,
			Text:      "daemon wake-up",
			Synthetic: &acp.PromptSyntheticMeta{
				TaskRunID: "run-1",
				Reason:    "task_run_completed",
				Summary:   "background work finished",
			},
		}),
		mustStoredPromptEvent(t, "ev-agent", 2, acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			TurnID:    "turn-synth",
			Timestamp: now.Add(time.Second),
			Text:      "ready",
		}),
	}

	submission, err := promptSubmissionFromStoredEvents(events)
	if err != nil {
		t.Fatalf("promptSubmissionFromStoredEvents() error = %v", err)
	}
	if got, want := submission.TurnID, "turn-synth"; got != want {
		t.Fatalf("submission.TurnID = %q, want %q", got, want)
	}
	if got, want := len(submission.SeedEvents), 2; got != want {
		t.Fatalf("len(submission.SeedEvents) = %d, want %d", got, want)
	}
	if got, want := submission.SeedEvents[0].Type, acp.EventTypeSyntheticReentry; got != want {
		t.Fatalf("seedEvents[0].Type = %q, want %q", got, want)
	}
	if got, want := submission.SeedEvents[0].Text, "daemon wake-up"; got != want {
		t.Fatalf("seedEvents[0].Text = %q, want %q", got, want)
	}
}

func TestPromptTurnIDFromStoredEventsPrefersFirstPromptBoundary(t *testing.T) {
	t.Parallel()

	events := []store.SessionEvent{
		{Type: acp.EventTypeToolCall, TurnID: "turn-tool"},
		{Type: acp.EventTypeUserMessage, TurnID: "turn-user"},
		{Type: acp.EventTypeSyntheticReentry, TurnID: "turn-synth"},
	}

	if got, want := promptTurnIDFromStoredEvents(events), "turn-user"; got != want {
		t.Fatalf("promptTurnIDFromStoredEvents() = %q, want %q", got, want)
	}
}

func TestPromptSubmissionFromStoredEventsRejectsMissingPromptBoundary(t *testing.T) {
	t.Parallel()

	_, err := promptSubmissionFromStoredEvents([]store.SessionEvent{{
		Type:   acp.EventTypeAgentMessage,
		TurnID: "turn-agent",
	}})
	if err == nil {
		t.Fatal("promptSubmissionFromStoredEvents() error = nil, want missing boundary error")
	}
	if !strings.Contains(err.Error(), "turn id not found") {
		t.Fatalf("promptSubmissionFromStoredEvents() error = %v, want turn id failure", err)
	}
}

func TestPromptProjectionEventFromStoredEventUsesStoredFallbacks(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 18, 14, 5, 0, 0, time.UTC)
	projected, err := promptProjectionEventFromStoredEvent(store.SessionEvent{
		ID:        "ev-fallback",
		Type:      acp.EventTypeSyntheticReentry,
		TurnID:    "turn-synth",
		Timestamp: now,
		Content:   `{"schema":"compozy.session.event.v1","text":"daemon wake-up"}`,
	})
	if err != nil {
		t.Fatalf("promptProjectionEventFromStoredEvent() error = %v", err)
	}
	if got, want := projected.Type, acp.EventTypeSyntheticReentry; got != want {
		t.Fatalf("projected.Type = %q, want %q", got, want)
	}
	if got, want := projected.TurnID, "turn-synth"; got != want {
		t.Fatalf("projected.TurnID = %q, want %q", got, want)
	}
	if got, want := projected.Text, "daemon wake-up"; got != want {
		t.Fatalf("projected.Text = %q, want %q", got, want)
	}
	if got := projected.Timestamp; !got.Equal(now) {
		t.Fatalf("projected.Timestamp = %s, want %s", got, now)
	}
}

func TestPromptProjectionEventFromStoredEventReturnsDecodeError(t *testing.T) {
	t.Parallel()

	_, err := promptProjectionEventFromStoredEvent(store.SessionEvent{
		ID:      "ev-invalid",
		Type:    acp.EventTypeSyntheticReentry,
		TurnID:  "turn-synth",
		Content: "{",
	})
	if err == nil {
		t.Fatal("promptProjectionEventFromStoredEvent() error = nil, want decode error")
	}
}

func TestPromptSeedEventsFromStoredEventsFiltersOtherTurns(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 18, 14, 7, 0, 0, time.UTC)
	events := []store.SessionEvent{
		mustStoredPromptEvent(t, "ev-other", 1, acp.AgentEvent{
			Type:      acp.EventTypeUserMessage,
			TurnID:    "turn-other",
			Timestamp: now,
			Text:      "other",
		}),
		mustStoredPromptEvent(t, "ev-synth", 2, acp.AgentEvent{
			Type:      acp.EventTypeSyntheticReentry,
			TurnID:    "turn-synth",
			Timestamp: now.Add(time.Second),
			Text:      "daemon wake-up",
		}),
	}

	seedEvents, err := promptSeedEventsFromStoredEvents(events, "turn-synth")
	if err != nil {
		t.Fatalf("promptSeedEventsFromStoredEvents() error = %v", err)
	}
	if got, want := len(seedEvents), 1; got != want {
		t.Fatalf("len(seedEvents) = %d, want %d", got, want)
	}
	if got, want := seedEvents[0].Type, acp.EventTypeSyntheticReentry; got != want {
		t.Fatalf("seedEvents[0].Type = %q, want %q", got, want)
	}
}

func TestHostAPIHandlerSubmitPromptRejectsMissingSessionManager(t *testing.T) {
	t.Parallel()

	var handler HostAPIHandler
	_, err := handler.submitPrompt(testutil.Context(t), "sess-1", hostAPIPromptRequest{
		Message: "hello", MessageID: "msg-1", IdempotencyKey: "idem-1",
	})
	if err == nil {
		t.Fatal("submitPrompt() error = nil, want missing session manager error")
	}
	if !strings.Contains(err.Error(), "session manager is not configured") {
		t.Fatalf("submitPrompt() error = %v, want session manager configuration failure", err)
	}
}

func TestHostAPIHandlerSubmitPromptRejectsMissingBoundaryEvents(t *testing.T) {
	t.Parallel()

	promptEvents := make(chan acp.AgentEvent)
	close(promptEvents)

	handler := &HostAPIHandler{
		sessions: promptSessionManagerStub{
			sendPromptFn: func(
				_ context.Context,
				_ string,
				_ session.SendPromptOpts,
			) (session.SendPromptResult, error) {
				return session.SendPromptResult{Events: promptEvents}, nil
			},
			eventsFn: func(_ context.Context, _ string, query store.EventQuery) ([]store.SessionEvent, error) {
				if query.Limit == 1 {
					return nil, nil
				}
				return []store.SessionEvent{{
					ID:        "ev-agent",
					Sequence:  1,
					TurnID:    "turn-agent",
					Type:      acp.EventTypeAgentMessage,
					AgentName: "coder",
					Content:   `{"schema":"compozy.session.event.v1","type":"agent_message","text":"reply"}`,
					Timestamp: time.Date(2026, 4, 18, 14, 6, 0, 0, time.UTC),
				}}, nil
			},
		},
	}

	_, err := handler.submitPrompt(testutil.Context(t), "sess-1", hostAPIPromptRequest{
		Message: "hello", MessageID: "msg-1", IdempotencyKey: "idem-1",
	})
	if err == nil {
		t.Fatal("submitPrompt() error = nil, want missing boundary error")
	}
	if !strings.Contains(err.Error(), "turn id not found") {
		t.Fatalf("submitPrompt() error = %v, want turn id failure", err)
	}
}

func TestHostAPIHandlerSubmitPromptRejectsUnexpectedStubCalls(t *testing.T) {
	t.Parallel()

	closedPromptEvents := func() <-chan acp.AgentEvent {
		ch := make(chan acp.AgentEvent)
		close(ch)
		return ch
	}

	tests := []struct {
		name     string
		sessions promptSessionManagerStub
		wantErr  string
	}{
		{
			name: "ShouldRejectMissingPromptCallback",
			sessions: promptSessionManagerStub{
				eventsFn: func(_ context.Context, _ string, _ store.EventQuery) ([]store.SessionEvent, error) {
					return []store.SessionEvent{{
						ID:        "ev-user",
						Sequence:  1,
						TurnID:    "turn-user",
						Type:      acp.EventTypeUserMessage,
						Content:   `{"schema":"compozy.session.event.v1","type":"user_message","text":"hello"}`,
						Timestamp: time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC),
					}}, nil
				},
			},
			wantErr: "unexpected send prompt call",
		},
		{
			name: "ShouldRejectMissingEventsCallback",
			sessions: promptSessionManagerStub{
				sendPromptFn: func(
					_ context.Context,
					_ string,
					_ session.SendPromptOpts,
				) (session.SendPromptResult, error) {
					return session.SendPromptResult{Events: closedPromptEvents()}, nil
				},
			},
			wantErr: "unexpected events call",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			handler := &HostAPIHandler{sessions: tt.sessions}
			_, err := handler.submitPrompt(testutil.Context(t), "sess-1", hostAPIPromptRequest{
				Message: "hello", MessageID: "msg-1", IdempotencyKey: "idem-1",
			})
			if err == nil {
				t.Fatalf("submitPrompt() error = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("submitPrompt() error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestBridgeHostAPIHelpersMapErrorsAndFormatInboundMetadata(t *testing.T) {
	t.Parallel()

	attachmentSummary := summarizeInboundAttachment(bridgepkg.MessageAttachment{
		ID:       "att-1",
		Name:     "report.pdf",
		MIMEType: "application/pdf",
		URL:      "https://example.com/report.pdf",
	})
	if !strings.Contains(attachmentSummary, "report.pdf") || !strings.Contains(attachmentSummary, "application/pdf") {
		t.Fatalf("summarizeInboundAttachment() = %q, want attachment name and mime type", attachmentSummary)
	}

	prompt := renderInboundMessagePrompt(bridgepkg.InboundMessageEnvelope{
		PlatformMessageID: "msg-1",
		ReceivedAt:        time.Date(2026, 4, 10, 18, 0, 0, 0, time.UTC),
		PeerID:            "peer-1",
		Sender:            bridgepkg.MessageSender{DisplayName: "Alice", Username: "alice"},
		Content:           bridgepkg.MessageContent{},
		Attachments: []bridgepkg.MessageAttachment{{
			Name:     "report.pdf",
			MIMEType: "application/pdf",
		}},
	})
	if !strings.Contains(prompt, "[No text body]") || !strings.Contains(prompt, "Attachments:") {
		t.Fatalf("renderInboundMessagePrompt() = %q, want attachment block and empty-body marker", prompt)
	}

	assertRPCErrorCode(t, mapBridgeLookupError("brg-1", bridgepkg.ErrBridgeInstanceNotFound), HostAPINotFoundCode)
	assertRPCErrorCode(t, mapBridgeRouteError("brg-1", bridgepkg.ErrBridgeInstanceUnavailable), HostAPIUnavailableCode)
	assertRPCErrorCode(
		t,
		mapBridgeStateUpdateError("brg-1", bridgepkg.ErrInvalidBridgeStateTransition),
		HostAPIInvalidParamsCode,
	)

	env := newHostAPITestEnv(t)
	if err := env.handler.stopBridgeSession(testutil.Context(t), "missing-session"); err != nil {
		t.Fatalf("stopBridgeSession(missing) error = %v, want nil", err)
	}
}

func TestHostAPIHandlerUnknownMethodReturnsMethodNotFound(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	_, err := env.call(t, "ext-any", "sessions/missing", nil)
	assertRPCErrorCode(t, err, HostAPIMethodNotFoundCode)
}

func TestHostAPIHandlerRateLimitExceededReturnsRetryAfter(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-rate", []string{"observe/health"}, []string{"observe.read"})

	handler := NewHostAPIHandler(
		env.sessions,
		env.memory,
		env.observer,
		env.skills,
		WithHostAPICapabilityChecker(env.checker),
		WithHostAPIWorkspaceResolver(env.workspaces),
		WithHostAPINow(func() time.Time { return env.currentTime() }),
		WithHostAPIRateLimit(1, 1),
	)

	if _, err := handler.Handle(testutil.Context(t), "ext-rate", "observe/health", nil); err != nil {
		t.Fatalf("first Handle(observe/health) error = %v, want nil", err)
	}
	_, err := handler.Handle(testutil.Context(t), "ext-rate", "observe/health", nil)
	assertRPCErrorCode(t, err, HostAPIRateLimitedCode)

	data := decodeRPCData(t, err)
	if _, ok := data["retry_after_ms"]; !ok {
		t.Fatalf("rate limit data = %#v, want retry_after_ms", data)
	}
	if got := data["scope"]; got != "host_api.observe/health" {
		t.Fatalf("rate limit scope = %v, want host_api.observe/health", got)
	}
}

func TestHostAPIHandlerRateLimitUsesConfiguredClockRegardlessOfOptionOrder(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-rate", []string{"observe/health"}, []string{"observe.read"})

	handler := NewHostAPIHandler(
		env.sessions,
		env.memory,
		env.observer,
		env.skills,
		WithHostAPICapabilityChecker(env.checker),
		WithHostAPIWorkspaceResolver(env.workspaces),
		WithHostAPIRateLimit(1, 1),
		WithHostAPINow(func() time.Time { return env.currentTime() }),
	)

	if _, err := handler.Handle(testutil.Context(t), "ext-rate", "observe/health", nil); err != nil {
		t.Fatalf("first Handle(observe/health) error = %v, want nil", err)
	}

	env.advanceTime(2 * time.Second)
	if _, err := handler.Handle(testutil.Context(t), "ext-rate", "observe/health", nil); err != nil {
		t.Fatalf("second Handle(observe/health) error = %v, want nil after refill from injected clock", err)
	}
}

func TestHostAPIHandlerCapabilityErrorsCarryMethodAndRequiredCapabilities(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-denied", nil, nil)

	tests := []struct {
		method string
		params any
	}{
		{method: "sessions/list", params: nil},
		{method: "sessions/create", params: map[string]any{"agent": "coder", "workspace": env.workspaceID}},
		{
			method: "sessions/prompt",
			params: map[string]any{"workspace_id": env.workspaceID, "session_id": "sess-1", "message": "hello"},
		},
		{method: "sessions/stop", params: map[string]any{"workspace_id": env.workspaceID, "session_id": "sess-1"}},
		{method: "sessions/status", params: map[string]any{"workspace_id": env.workspaceID, "session_id": "sess-1"}},
		{method: "sessions/events", params: map[string]any{"workspace_id": env.workspaceID, "session_id": "sess-1"}},
		{method: "memory/recall", params: map[string]any{"query": "needle"}},
		{method: "memory/store", params: map[string]any{"key": "note", "content": "body"}},
		{method: "memory/forget", params: map[string]any{"key": "note"}},
		{method: "observe/health", params: nil},
		{method: "logs/list", params: map[string]any{"limit": 1}},
		{method: "skills/list", params: map[string]any{"workspace": env.workspaceID}},
		{method: "automation/jobs", params: map[string]any{"scope": "workspace", "workspace_id": env.workspaceID}},
		{method: "automation/jobs/create", params: map[string]any{
			"name":         "host-api-job",
			"scope":        "workspace",
			"workspace_id": env.workspaceID,
			"agent_name":   "coder",
			"prompt":       "do work",
			"schedule": map[string]any{
				"mode":     "every",
				"interval": "5m",
			},
		}},
		{method: "automation/triggers/fire", params: map[string]any{
			"event":        "ext.github.push",
			"scope":        "workspace",
			"workspace_id": env.workspaceID,
		}},
		{method: "bridges/messages/ingest", params: map[string]any{
			"bridge_instance_id":  "brg-1",
			"scope":               "workspace",
			"workspace_id":        env.workspaceID,
			"peer_id":             "peer-1",
			"platform_message_id": "msg-1",
			"received_at":         env.currentTime().Format(time.RFC3339Nano),
			"idempotency_key":     "idem-1",
		}},
		{method: "bridges/instances/list", params: nil},
		{method: "bridges/instances/get", params: map[string]any{"bridge_instance_id": "brg-1"}},
		{
			method: "bridges/instances/report_state",
			params: map[string]any{"bridge_instance_id": "brg-1", "status": "ready"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			t.Parallel()

			_, err := env.call(t, "ext-denied", tt.method, tt.params)
			assertCapabilityDenied(t, err, tt.method)
		})
	}
}

func TestManagerWrapHostHandlerInjectsExtensionNameForHostAPIHandler(t *testing.T) {
	t.Parallel()
	t.Run("Should authorize the session grant while preserving the extension name", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		key := GlobalInstanceKey("ext-wrapped")
		grantID := extensionCapabilityGrantID(key, "session-nonce")
		env.grant(grantID, []string{"observe/health"}, []string{"observe.read"})

		var handledExtensionName string
		env.handler.methods["observe/health"] = func(ctx context.Context, _ json.RawMessage) (any, error) {
			handledExtensionName = hostAPIExtensionNameFromContext(ctx)
			return observepkg.Health{Status: "ok"}, nil
		}

		manager := NewManager(nil, WithCapabilityChecker(env.checker))
		wrapped := manager.wrapHostHandler(
			key,
			"observe/health",
			nil,
			&hostAPIResourceSession{Actor: resources.MutationActor{ID: grantID}},
			env.handler.HandleMethod("observe/health"),
		)

		result, err := wrapped(testutil.Context(t), nil)
		if err != nil {
			t.Fatalf("wrapHostHandler(observe/health) error = %v", err)
		}

		var health observepkg.Health
		decodeResult(t, result, &health)
		if health.Status != "ok" {
			t.Fatalf("wrapped observe/health status = %q, want ok", health.Status)
		}
		if handledExtensionName != key.Name {
			t.Fatalf("handled extension name = %q, want %q", handledExtensionName, key.Name)
		}
	})
}

func TestNormalizeHostAPIHandlerDefaultsFillsZeroValues(t *testing.T) {
	t.Parallel()

	normalizeHostAPIHandlerDefaults(nil)

	handler := &HostAPIHandler{}
	normalizeHostAPIHandlerDefaults(handler)

	if handler.now == nil {
		t.Fatal("normalizeHostAPIHandlerDefaults() left now nil")
	}
	if handler.capChecker == nil {
		t.Fatal("normalizeHostAPIHandlerDefaults() left capChecker nil")
	}
	if handler.bridgeIngestDedupTTL != defaultHostAPIBridgeIngestDedupTTL {
		t.Fatalf(
			"bridgeIngestDedupTTL = %v, want %v",
			handler.bridgeIngestDedupTTL,
			defaultHostAPIBridgeIngestDedupTTL,
		)
	}
	if handler.bridgeCleanupInterval != defaultHostAPIBridgeCleanupInterval {
		t.Fatalf(
			"bridgeCleanupInterval = %v, want %v",
			handler.bridgeCleanupInterval,
			defaultHostAPIBridgeCleanupInterval,
		)
	}
	if handler.bridgeLocks == nil {
		t.Fatal("normalizeHostAPIHandlerDefaults() left bridgeLocks nil")
	}
}

func TestHostAPIContextHelpersCloneBridgeAndResourceSession(t *testing.T) {
	t.Parallel()

	baseCtx := context.Background()
	if got := withHostAPIBridgeRuntime(baseCtx, nil); got != baseCtx {
		t.Fatalf("withHostAPIBridgeRuntime(background, nil) = %#v, want background context", got)
	}
	if got := withHostAPIResourceSession(baseCtx, nil); got != baseCtx {
		t.Fatalf("withHostAPIResourceSession(background, nil) = %#v, want background context", got)
	}
	if _, ok := hostAPIResourceSessionFromContext(baseCtx); ok {
		t.Fatal("hostAPIResourceSessionFromContext(background) = ok, want false")
	}
	if runtime := hostAPIBridgeRuntimeFromContext(baseCtx); runtime != nil {
		t.Fatalf("hostAPIBridgeRuntimeFromContext(background) = %#v, want nil", runtime)
	}

	runtime := &subprocess.InitializeBridgeRuntime{
		ManagedInstances: []subprocess.InitializeBridgeManagedInstance{
			{
				Instance: bridgepkg.BridgeInstanceToContract(bridgepkg.BridgeInstance{
					ID:            "brg-1",
					ExtensionName: "ext-runtime",
				}),
			},
		},
	}
	session := &hostAPIResourceSession{
		Actor: resources.MutationActor{
			Kind:         resources.MutationActorKindExtension,
			ID:           "ext-runtime",
			SessionNonce: "nonce-1",
			Source: resources.ResourceSource{
				Kind: resources.ResourceSourceKind("extension"),
				ID:   "ext-runtime",
			},
			MaxScope:      resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			GrantedKinds:  []resources.ResourceKind{"tool.definition"},
			GrantedScopes: []resources.ResourceScopeKind{resources.ResourceScopeKindUser},
		},
	}

	ctx := withHostAPIBridgeRuntime(withHostAPIResourceSession(baseCtx, session), runtime)

	session.Actor.GrantedKinds[0] = "tool.call"
	runtime.ManagedInstances[0].Instance.ID = "mutated"

	storedSession, ok := hostAPIResourceSessionFromContext(ctx)
	if !ok {
		t.Fatal("hostAPIResourceSessionFromContext(ctx) = false, want true")
	}
	if got, want := storedSession.Actor.GrantedKinds, []resources.ResourceKind{
		"tool.definition",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("storedSession.Actor.GrantedKinds = %#v, want %#v", got, want)
	}
	storedSession.Actor.GrantedKinds[0] = "tool.call"
	reloadedSession, ok := hostAPIResourceSessionFromContext(ctx)
	if !ok {
		t.Fatal("hostAPIResourceSessionFromContext(ctx) after mutation = false, want true")
	}
	if got, want := reloadedSession.Actor.GrantedKinds, []resources.ResourceKind{
		"tool.definition",
	}; !slices.Equal(
		got,
		want,
	) {
		t.Fatalf("reloadedSession.Actor.GrantedKinds = %#v, want %#v", got, want)
	}

	storedRuntime := hostAPIBridgeRuntimeFromContext(ctx)
	if storedRuntime == nil {
		t.Fatal("hostAPIBridgeRuntimeFromContext(ctx) = nil, want runtime")
		return
	}
	if got, want := storedRuntime.ManagedInstances[0].Instance.ID, "brg-1"; got != want {
		t.Fatalf("storedRuntime.ManagedInstances[0].Instance.ID = %q, want %q", got, want)
	}

	t.Run("Should preserve workspace identity and deep-copy inbound bridge wire payloads", func(t *testing.T) {
		t.Parallel()

		wire := bridgecontract.InboundMessageEnvelope{
			BridgeInstanceID: "brg-wire", Scope: bridgecontract.ScopeWorkspace,
			WorkspaceID: "ws-wire", ReceivedAt: time.Date(2026, 7, 13, 12, 0, 0, 0, time.UTC),
			Attachments: []bridgecontract.MessageAttachment{{ID: "att-1", Name: "report.pdf"}},
			Command:     &bridgecontract.InboundCommand{Command: "/summarize"},
			Conversation: &bridgecontract.NetworkConversationRef{
				Channel: "general", Surface: bridgecontract.NetworkConversationSurfaceThread,
				ThreadID: "thread_1",
			},
			ProviderMetadata: json.RawMessage(`{"update_id":1}`), IdempotencyKey: "idem-wire",
		}

		domain := bridgeInboundEnvelopeDomain(wire)
		if got, want := domain.Scope, bridgepkg.ScopeWorkspace; got != want {
			t.Fatalf("bridgeInboundEnvelopeDomain().Scope = %q, want %q", got, want)
		}
		if got, want := domain.WorkspaceID, "ws-wire"; got != want {
			t.Fatalf("bridgeInboundEnvelopeDomain().WorkspaceID = %q, want %q", got, want)
		}

		domain.Attachments[0].Name = "mutated"
		domain.Command.Command = "/mutated"
		domain.Conversation.Channel = "mutated"
		domain.ProviderMetadata[0] = '['
		if got, want := wire.Attachments[0].Name, "report.pdf"; got != want {
			t.Fatalf("wire.Attachments[0].Name = %q after domain mutation, want %q", got, want)
		}
		if got, want := wire.Command.Command, "/summarize"; got != want {
			t.Fatalf("wire.Command.Command = %q after domain mutation, want %q", got, want)
		}
		if got, want := wire.Conversation.Channel, "general"; got != want {
			t.Fatalf("wire.Conversation.Channel = %q after domain mutation, want %q", got, want)
		}
		if got, want := string(wire.ProviderMetadata), `{"update_id":1}`; got != want {
			t.Fatalf("wire.ProviderMetadata = %q after domain mutation, want %q", got, want)
		}
	})
}

func TestNormalizeHostAPIRPCErrorMapsResourceStatuses(t *testing.T) {
	t.Parallel()

	sameRPC := subprocess.NewRPCError(499, "unchanged", map[string]string{"error": "keep"})
	sameErr := errors.New("boom")
	const rawToken = "compozy_claim_host-api-error-secret"

	tests := []struct {
		name         string
		method       string
		err          error
		wantCode     int
		wantMessage  string
		wantSame     bool
		wantRedacted bool
	}{
		{name: "nil", method: "resources/list", err: nil},
		{name: "non resource", method: "observe/health", err: sameErr, wantSame: true},
		{name: "rpc passthrough", method: "resources/list", err: sameRPC, wantSame: true},
		{
			name:         "non resource raw claim error",
			method:       "tasks/runs/start",
			err:          fmt.Errorf("provider returned %s", rawToken),
			wantRedacted: true,
		},
		{
			name:   "rpc message and nested data raw claim error",
			method: "tasks/runs/start",
			err: subprocess.NewRPCError(451, "provider returned "+rawToken, map[string]any{
				"claim_token": rawToken,
				"nested": []any{
					map[string]any{"detail": "provider returned " + rawToken},
				},
				rawToken: "discarded",
			}),
			wantCode:     451,
			wantMessage:  "provider returned compozy_claim_[REDACTED]",
			wantRedacted: true,
		},
		{
			name:   "rate limited",
			method: "resources/list",
			err: subprocess.NewRPCError(
				HostAPIRateLimitedCode,
				"slow down",
				map[string]string{"error": "slow"},
			),
			wantCode:    429,
			wantMessage: "Rate limited",
		},
		{
			name:        "forbidden",
			method:      "resources/get",
			err:         resources.ErrPermissionDenied,
			wantCode:    403,
			wantMessage: "Forbidden",
		},
		{
			name:        "conflict",
			method:      "resources/snapshot",
			err:         resources.ErrSessionNotActive,
			wantCode:    409,
			wantMessage: "Conflict",
		},
		{
			name:        "payload too large",
			method:      "resources/snapshot",
			err:         resources.ErrPayloadTooLarge,
			wantCode:    413,
			wantMessage: "Payload too large",
		},
		{
			name:        "not found",
			method:      "resources/get",
			err:         resources.ErrNotFound,
			wantCode:    HostAPINotFoundCode,
			wantMessage: "Not found",
		},
		{
			name:        "invalid params",
			method:      "resources/list",
			err:         resources.ErrValidation,
			wantCode:    HostAPIInvalidParamsCode,
			wantMessage: "Invalid params",
		},
		{name: "default passthrough", method: "resources/list", err: sameErr, wantSame: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := normalizeHostAPIRPCError(tt.method, tt.err)
			if tt.err == nil {
				if got != nil {
					t.Fatalf("normalizeHostAPIRPCError() = %v, want nil", got)
				}
				return
			}
			if tt.wantSame {
				if got != tt.err {
					t.Fatalf("normalizeHostAPIRPCError() = %#v, want original %#v", got, tt.err)
				}
				return
			}
			if tt.wantRedacted {
				if strings.Contains(got.Error(), rawToken) {
					t.Fatalf("normalizeHostAPIRPCError() = %q, leaked raw bearer", got)
				}
				if errors.Unwrap(got) != nil {
					t.Fatalf("normalizeHostAPIRPCError() = %#v, want public error without raw cause", got)
				}
				rpcErr, rpcErrMatched := errors.AsType[*subprocess.RPCError](got)
				if rpcErrMatched {
					if strings.Contains(rpcErr.Message, rawToken) || strings.Contains(string(rpcErr.Data), rawToken) {
						t.Fatalf("sanitized rpc error = %#v, leaked raw bearer", rpcErr)
					}
					if rpcErr.Code != tt.wantCode {
						t.Fatalf("sanitized rpc error code = %d, want %d", rpcErr.Code, tt.wantCode)
					}
					if tt.wantMessage != "" && rpcErr.Message != tt.wantMessage {
						t.Fatalf("sanitized rpc error message = %q, want %q", rpcErr.Message, tt.wantMessage)
					}
					var decoded any
					if len(rpcErr.Data) > 0 && json.Unmarshal(rpcErr.Data, &decoded) != nil {
						t.Fatalf("sanitized rpc data = %s, want valid JSON", rpcErr.Data)
					}
				}
				return
			}

			rpcErr, rpcErrMatched2 := errors.AsType[*subprocess.RPCError](got)
			if !rpcErrMatched2 {
				t.Fatalf("normalizeHostAPIRPCError() type = %T, want *subprocess.RPCError", got)
			}
			if rpcErr.Code != tt.wantCode {
				t.Fatalf("rpcErr.Code = %d, want %d", rpcErr.Code, tt.wantCode)
			}
			if rpcErr.Message != tt.wantMessage {
				t.Fatalf("rpcErr.Message = %q, want %q", rpcErr.Message, tt.wantMessage)
			}
		})
	}
}

func TestRPCCapabilityDeniedUsesHTTPStatusForResourceMethods(t *testing.T) {
	t.Parallel()

	resourceErr := rpcCapabilityDenied(newCapabilityDeniedError("resources/get", []string{"resources.read"}, nil))
	assertRPCErrorCode(t, resourceErr, 403)
	data := decodeRPCData(t, resourceErr)
	if got := data["method"]; got != "resources/get" {
		t.Fatalf("rpc data method = %#v, want resources/get", got)
	}

	observeErr := rpcCapabilityDenied(newCapabilityDeniedError("observe/health", []string{"observe.read"}, nil))
	assertRPCErrorCode(t, observeErr, CapabilityDeniedCode)
}

func TestHostAPIHandlerAutomationTriggerFireRejectsNonExtensionEvent(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-automation", []string{"automation/triggers/fire"}, []string{"automation.write"})

	_, err := env.call(t, "ext-automation", "automation/triggers/fire", map[string]any{
		"event": "session.stopped",
		"scope": "workspace",
		"payload": map[string]any{
			"session_id": "sess-1",
		},
		"workspace_id": env.workspaceID,
	})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	data := decodeRPCData(t, err)
	if got := data["error"]; got != `trigger_fire.event must start with "ext."` {
		t.Fatalf("rpc data error = %#v, want ext prefix validation", got)
	}
}

func TestHostAPIHandlerAutomationJobCRUDAndRunQueries(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant(
		"ext-automation",
		[]string{
			"automation/jobs",
			"automation/jobs/get",
			"automation/jobs/create",
			"automation/jobs/update",
			"automation/jobs/delete",
			"automation/jobs/trigger",
			"automation/jobs/runs",
			"automation/runs",
		},
		[]string{"automation.read", "automation.write"},
	)

	createResult, err := env.call(t, "ext-automation", "automation/jobs/create", map[string]any{
		"name":         "host-api-job",
		"scope":        "workspace",
		"workspace_id": env.workspace.RootDir,
		"agent_name":   "coder",
		"prompt":       "Original host API job prompt",
		"schedule": map[string]any{
			"mode":     "every",
			"interval": "5m",
		},
	})
	if err != nil {
		t.Fatalf("Handle(automation/jobs/create) error = %v", err)
	}

	var created automationpkg.Job
	decodeResult(t, createResult, &created)
	if got, want := created.WorkspaceID, env.workspaceID; got != want {
		t.Fatalf("created workspace_id = %q, want %q", got, want)
	}

	listResult, err := env.call(t, "ext-automation", "automation/jobs", map[string]any{
		"scope":        "workspace",
		"workspace_id": env.workspace.RootDir,
		"enabled":      true,
	})
	if err != nil {
		t.Fatalf("Handle(automation/jobs) error = %v", err)
	}
	var listed extensioncontract.AutomationJobsResult
	decodeResult(t, listResult, &listed)
	if got, want := len(listed.Jobs), 1; got != want {
		t.Fatalf("len(automation/jobs) = %d, want %d", got, want)
	}
	if got, want := listed.Page.Total, 1; got != want {
		t.Fatalf("automation/jobs page total = %d, want %d", got, want)
	}

	getResult, err := env.call(t, "ext-automation", "automation/jobs/get", map[string]any{"id": created.ID})
	if err != nil {
		t.Fatalf("Handle(automation/jobs/get) error = %v", err)
	}
	var fetched automationpkg.Job
	decodeResult(t, getResult, &fetched)
	if got, want := fetched.ID, created.ID; got != want {
		t.Fatalf("automation/jobs/get id = %q, want %q", got, want)
	}

	updateResult, err := env.call(t, "ext-automation", "automation/jobs/update", map[string]any{
		"id":     created.ID,
		"prompt": "Updated host API job prompt",
		"schedule": map[string]any{
			"mode":     "every",
			"interval": "15m",
		},
	})
	if err != nil {
		t.Fatalf("Handle(automation/jobs/update) error = %v", err)
	}
	var updated automationpkg.Job
	decodeResult(t, updateResult, &updated)
	if got, want := updated.Prompt, "Updated host API job prompt"; got != want {
		t.Fatalf("updated prompt = %q, want %q", got, want)
	}
	if updated.Schedule == nil || updated.Schedule.Interval != "15m" {
		t.Fatalf("updated schedule = %#v, want interval 15m", updated.Schedule)
	}

	triggerResult, err := env.call(t, "ext-automation", "automation/jobs/trigger", map[string]any{"id": created.ID})
	if err != nil {
		t.Fatalf("Handle(automation/jobs/trigger) error = %v", err)
	}
	var run automationpkg.Run
	decodeResult(t, triggerResult, &run)
	if got, want := run.JobID, created.ID; got != want {
		t.Fatalf("triggered run job_id = %q, want %q", got, want)
	}

	runsByJobResult, err := env.call(t, "ext-automation", "automation/jobs/runs", map[string]any{"id": created.ID})
	if err != nil {
		t.Fatalf("Handle(automation/jobs/runs) error = %v", err)
	}
	var runsByJob []automationpkg.Run
	decodeResult(t, runsByJobResult, &runsByJob)
	if got, want := len(runsByJob), 1; got != want {
		t.Fatalf("len(automation/jobs/runs) = %d, want %d", got, want)
	}

	allRunsResult, err := env.call(t, "ext-automation", "automation/runs", map[string]any{"job_id": created.ID})
	if err != nil {
		t.Fatalf("Handle(automation/runs) error = %v", err)
	}
	var allRuns []automationpkg.Run
	decodeResult(t, allRunsResult, &allRuns)
	if got, want := len(allRuns), 1; got != want {
		t.Fatalf("len(automation/runs) = %d, want %d", got, want)
	}

	if _, err := env.call(t, "ext-automation", "automation/jobs/delete", map[string]any{"id": created.ID}); err != nil {
		t.Fatalf("Handle(automation/jobs/delete) error = %v", err)
	}
}

// Invariant: profile-scoped automation reads and writes use the bridge-owned
// profile identity, so resources never cross profile boundaries.
// Owner: Host API automation handlers.
// Canonical suite: extension Host API integration tests.
func TestHostAPIHandlerAutomationUsesBridgeProfile(t *testing.T) {
	t.Parallel()
	t.Run("Should list only automation resources for the bridge profile", func(t *testing.T) {
		t.Parallel()
		env := newHostAPITestEnv(t)
		env.grant(
			"ext-automation",
			[]string{
				"automation/jobs",
				"automation/jobs/create",
				"automation/triggers",
				"automation/triggers/create",
			},
			[]string{"automation.read", "automation.write"},
		)
		for _, profileID := range []string{store.DefaultProfileID, env.marketingID} {
			_, err := env.automation.CreateJob(testutil.Context(t), automationpkg.Job{
				ProfileID: profileID,
				Scope:     automationpkg.AutomationScopeGlobal,
				AgentName: "coder",
				Prompt:    "profile job",
				Name:      "profile-job-" + profileID,
				Schedule:  &automationpkg.ScheduleSpec{Mode: automationpkg.ScheduleModeEvery, Interval: "1h"},
				Enabled:   true,
				Retry:     automationpkg.DefaultRetryConfig(),
				FireLimit: automationpkg.DefaultFireLimitConfig(),
				Source:    automationpkg.JobSourceDynamic,
			})
			if err != nil {
				t.Fatalf("CreateJob(%q) error = %v", profileID, err)
			}
			_, err = env.automation.CreateTrigger(testutil.Context(t), automationpkg.Trigger{
				ProfileID: profileID,
				Scope:     automationpkg.AutomationScopeGlobal,
				AgentName: "coder",
				Name:      "profile-trigger-" + profileID,
				Event:     "ext.profile." + profileID,
				Prompt:    "profile trigger",
				Enabled:   true,
				Retry:     automationpkg.DefaultRetryConfig(),
				FireLimit: automationpkg.DefaultFireLimitConfig(),
				Source:    automationpkg.JobSourceDynamic,
			}, automationpkg.WebhookSecretWrite{})
			if err != nil {
				t.Fatalf("CreateTrigger(%q) error = %v", profileID, err)
			}
		}

		ctx := env.bridgeContext(t, &bridgepkg.BridgeInstance{
			ID: "bridge-marketing", ExtensionName: "ext-automation", ProfileID: env.marketingID,
		})
		jobsResult, err := env.callWithContext(ctx, t, "ext-automation", "automation/jobs", map[string]any{
			"scope": "global",
		})
		if err != nil {
			t.Fatalf("Handle(automation/jobs profile) error = %v", err)
		}
		var jobs extensioncontract.AutomationJobsResult
		decodeResult(t, jobsResult, &jobs)
		if len(jobs.Jobs) != 1 || jobs.Jobs[0].ProfileID != env.marketingID {
			t.Fatalf("profile automation/jobs = %#v, want only profile-marketing", jobs.Jobs)
		}

		triggersResult, err := env.callWithContext(ctx, t, "ext-automation", "automation/triggers", map[string]any{
			"scope": "global",
		})
		if err != nil {
			t.Fatalf("Handle(automation/triggers profile) error = %v", err)
		}
		var triggers extensioncontract.AutomationTriggersResult
		decodeResult(t, triggersResult, &triggers)
		if len(triggers.Triggers) != 1 || triggers.Triggers[0].ProfileID != env.marketingID {
			t.Fatalf("profile automation/triggers = %#v, want only profile-marketing", triggers.Triggers)
		}

		createdJobResult, err := env.callWithContext(
			ctx,
			t,
			"ext-automation",
			"automation/jobs/create",
			map[string]any{
				"name":       "marketing-created-job",
				"scope":      "global",
				"agent_name": "coder",
				"prompt":     "marketing job",
				"schedule": map[string]any{
					"mode": "every", "interval": "1h",
				},
			},
		)
		if err != nil {
			t.Fatalf("Handle(automation/jobs/create profile) error = %v", err)
		}
		var createdJob automationpkg.Job
		decodeResult(t, createdJobResult, &createdJob)
		if got, want := createdJob.ProfileID, env.marketingID; got != want {
			t.Fatalf("created job ProfileID = %q, want %q", got, want)
		}

		createdTriggerResult, err := env.callWithContext(
			ctx,
			t,
			"ext-automation",
			"automation/triggers/create",
			map[string]any{
				"name":       "marketing-created-trigger",
				"scope":      "global",
				"agent_name": "coder",
				"event":      "ext.profile.created",
				"prompt":     "marketing trigger",
			},
		)
		if err != nil {
			t.Fatalf("Handle(automation/triggers/create profile) error = %v", err)
		}
		var createdTrigger automationpkg.Trigger
		decodeResult(t, createdTriggerResult, &createdTrigger)
		if got, want := createdTrigger.ProfileID, env.marketingID; got != want {
			t.Fatalf("created trigger ProfileID = %q, want %q", got, want)
		}
	})
}

func TestHostAPIHandlerAutomationCreateTargetParity(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t, withHostAPITestLoopStarter(&hostAPITestLoopStarter{}))
	env.grant(
		"ext-automation-targets",
		[]string{"automation/jobs/create", "automation/triggers/create"},
		[]string{"automation.write"},
	)

	jobResult, err := env.call(t, "ext-automation-targets", "automation/jobs/create", map[string]any{
		"name":         "host-api-task-job",
		"scope":        "workspace",
		"workspace_id": env.workspace.RootDir,
		"target_kind":  "agent",
		"schedule": map[string]any{
			"mode":     "every",
			"interval": "5m",
		},
		"task": map[string]any{
			"title": "Scheduled task",
			"network_participation": map[string]any{
				"mode": "local",
			},
		},
	})
	if err != nil {
		t.Fatalf("Handle(automation/jobs/create task target) error = %v", err)
	}
	var job automationpkg.Job
	decodeResult(t, jobResult, &job)
	if job.TargetKind != automationpkg.TargetKindAgent || job.Task == nil || job.Task.Title != "Scheduled task" {
		t.Fatalf("created task job = %#v, want preserved target kind and task config", job)
	}

	triggerResult, err := env.call(
		t,
		"ext-automation-targets",
		"automation/triggers/create",
		map[string]any{
			"name":         "host-api-loop-trigger",
			"scope":        "workspace",
			"workspace_id": env.workspace.RootDir,
			"target_kind":  "loop",
			"event":        "ext.release.ready",
			"loop_target": map[string]any{
				"workspace_id": env.workspaceID,
				"loop_name":    "release",
				"inputs":       map[string]any{"environment": "staging"},
				"input_mapping": map[string]string{
					"commit": "data.sha",
				},
				"network_participation": map[string]any{"mode": "local"},
			},
		},
	)
	if err != nil {
		t.Fatalf("Handle(automation/triggers/create loop target) error = %v", err)
	}
	var trigger apicontract.TriggerPayload
	decodeResult(t, triggerResult, &trigger)
	if trigger.TargetKind != automationpkg.TargetKindLoop || trigger.LoopTarget == nil {
		t.Fatalf("created loop trigger = %#v, want preserved loop target", trigger)
	}
	if got, want := trigger.LoopTarget.LoopName, "release"; got != want {
		t.Fatalf("created loop target name = %q, want %q", got, want)
	}
	if trigger.LoopTarget.NetworkParticipation == nil ||
		trigger.LoopTarget.NetworkParticipation.Mode == nil ||
		*trigger.LoopTarget.NetworkParticipation.Mode != participation.ModeLocal {
		t.Fatalf(
			"created loop target participation = %#v, want local",
			trigger.LoopTarget.NetworkParticipation,
		)
	}
}

func TestHostAPIHandlerAutomationTriggerCRUDAndConfigGuardrails(t *testing.T) {
	t.Parallel()

	t.Run("Should manage automation trigger CRUD and config guardrails", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		env.grant(
			"ext-automation",
			[]string{
				"automation/triggers",
				"automation/triggers/get",
				"automation/triggers/create",
				"automation/triggers/update",
				"automation/triggers/delete",
				"automation/triggers/runs",
				"automation/triggers/fire",
				"automation/jobs/delete",
				"automation/jobs/update",
			},
			[]string{"automation.read", "automation.write"},
		)

		createResult, err := env.call(t, "ext-automation", "automation/triggers/create", map[string]any{
			"name":         "host-api-trigger",
			"scope":        "workspace",
			"workspace_id": env.workspace.RootDir,
			"agent_name":   "coder",
			"event":        "ext.github.push",
			"prompt":       `Review {{ index .Data "repo" }}`,
			"filter": map[string]string{
				"data.repo": "acme/api",
			},
		})
		if err != nil {
			t.Fatalf("Handle(automation/triggers/create) error = %v", err)
		}

		var created apicontract.TriggerPayload
		decodeResult(t, createResult, &created)
		if got, want := created.WorkspaceID, env.workspaceID; got != want {
			t.Fatalf("created trigger workspace_id = %q, want %q", got, want)
		}
		var createdRaw map[string]any
		decodeResult(t, createResult, &createdRaw)
		if _, ok := createdRaw["webhook_secret_ref"]; ok {
			t.Fatalf("created trigger result includes webhook_secret_ref: %#v", createdRaw)
		}

		listResult, err := env.call(t, "ext-automation", "automation/triggers", map[string]any{
			"scope":        "workspace",
			"workspace_id": env.workspace.RootDir,
			"event":        "ext.github.push",
			"enabled":      true,
		})
		if err != nil {
			t.Fatalf("Handle(automation/triggers) error = %v", err)
		}
		var listed extensioncontract.AutomationTriggersResult
		decodeResult(t, listResult, &listed)
		if got, want := len(listed.Triggers), 1; got != want {
			t.Fatalf("len(automation/triggers) = %d, want %d", got, want)
		}
		if got, want := listed.Page.Total, 1; got != want {
			t.Fatalf("automation/triggers page total = %d, want %d", got, want)
		}

		getResult, err := env.call(t, "ext-automation", "automation/triggers/get", map[string]any{"id": created.ID})
		if err != nil {
			t.Fatalf("Handle(automation/triggers/get) error = %v", err)
		}
		var fetched apicontract.TriggerPayload
		decodeResult(t, getResult, &fetched)
		if got, want := fetched.ID, created.ID; got != want {
			t.Fatalf("automation/triggers/get id = %q, want %q", got, want)
		}

		updateResult, err := env.call(t, "ext-automation", "automation/triggers/update", map[string]any{
			"id":     created.ID,
			"prompt": `Updated {{ index .Data "repo" }}`,
			"filter": map[string]string{
				"data.repo": "acme/api",
			},
		})
		if err != nil {
			t.Fatalf("Handle(automation/triggers/update) error = %v", err)
		}
		var updated apicontract.TriggerPayload
		decodeResult(t, updateResult, &updated)
		if got, want := updated.Prompt, `Updated {{ index .Data "repo" }}`; got != want {
			t.Fatalf("updated trigger prompt = %q, want %q", got, want)
		}

		fireResult, err := env.call(t, "ext-automation", "automation/triggers/fire", map[string]any{
			"event":        "ext.github.push",
			"scope":        "workspace",
			"workspace_id": env.workspaceID,
			"payload": map[string]any{
				"repo": "acme/api",
			},
		})
		if err != nil {
			t.Fatalf("Handle(automation/triggers/fire) error = %v", err)
		}
		var fire automationpkg.TriggerResult
		decodeResult(t, fireResult, &fire)
		if got, want := fire.Matched, 1; got != want {
			t.Fatalf("automation/triggers/fire matched = %d, want %d", got, want)
		}
		if _, err := env.call(t, "ext-automation", "automation/triggers/fire", map[string]any{
			"event":        " ext.github.push",
			"scope":        "workspace",
			"workspace_id": env.workspaceID,
		}); err == nil || !strings.Contains(err.Error(), "Invalid params") {
			t.Fatalf("Handle(automation/triggers/fire padded event) error = %v, want invalid params", err)
		}

		runsResult, err := env.call(t, "ext-automation", "automation/triggers/runs", map[string]any{"id": created.ID})
		if err != nil {
			t.Fatalf("Handle(automation/triggers/runs) error = %v", err)
		}
		var triggerRuns []automationpkg.Run
		decodeResult(t, runsResult, &triggerRuns)
		if got, want := len(triggerRuns), 1; got != want {
			t.Fatalf("len(automation/triggers/runs) = %d, want %d", got, want)
		}

		configJob, err := env.registry.CreateJob(testutil.Context(t), automationpkg.Job{
			ProfileID:   store.DefaultProfileID,
			ID:          "job-config-host-api",
			Scope:       automationpkg.AutomationScopeWorkspace,
			Name:        "config-host-api-job",
			AgentName:   "coder",
			WorkspaceID: env.workspaceID,
			Prompt:      "Config-backed prompt",
			Schedule: &automationpkg.ScheduleSpec{
				Mode:     automationpkg.ScheduleModeEvery,
				Interval: "1h",
			},
			Enabled:   true,
			Retry:     automationpkg.DefaultRetryConfig(),
			FireLimit: automationpkg.DefaultFireLimitConfig(),
			Source:    automationpkg.JobSourceConfig,
		})
		if err != nil {
			t.Fatalf("CreateJob(config) error = %v", err)
		}
		if _, err := env.call(t, "ext-automation", "automation/jobs/update", map[string]any{
			"id":      configJob.ID,
			"enabled": false,
		}); err != nil {
			t.Fatalf("Handle(automation/jobs/update enabled-only) error = %v", err)
		}
		_, err = env.call(t, "ext-automation", "automation/jobs/update", map[string]any{
			"id":     configJob.ID,
			"prompt": "should fail",
		})
		assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
		_, err = env.call(t, "ext-automation", "automation/jobs/delete", map[string]any{"id": configJob.ID})
		assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)

		configTrigger, err := env.registry.CreateTrigger(testutil.Context(t), automationpkg.Trigger{
			ProfileID:   store.DefaultProfileID,
			ID:          "trigger-config-host-api",
			Scope:       automationpkg.AutomationScopeWorkspace,
			Name:        "config-host-api-trigger",
			AgentName:   "coder",
			WorkspaceID: env.workspaceID,
			Prompt:      `Config {{ index .Data "repo" }}`,
			Event:       "ext.github.push",
			Enabled:     true,
			Retry:       automationpkg.DefaultRetryConfig(),
			FireLimit:   automationpkg.DefaultFireLimitConfig(),
			Source:      automationpkg.JobSourceConfig,
		})
		if err != nil {
			t.Fatalf("CreateTrigger(config) error = %v", err)
		}
		if _, err := env.call(t, "ext-automation", "automation/triggers/update", map[string]any{
			"id":      configTrigger.ID,
			"enabled": false,
		}); err != nil {
			t.Fatalf("Handle(automation/triggers/update enabled-only) error = %v", err)
		}
		_, err = env.call(t, "ext-automation", "automation/triggers/update", map[string]any{
			"id":     configTrigger.ID,
			"prompt": "should fail",
		})
		assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
		_, err = env.call(t, "ext-automation", "automation/triggers/delete", map[string]any{"id": configTrigger.ID})
		assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)

		if _, err := env.call(
			t,
			"ext-automation",
			"automation/triggers/delete",
			map[string]any{"id": created.ID},
		); err != nil {
			t.Fatalf("Handle(automation/triggers/delete) error = %v", err)
		}
	})
}

func TestDescribeExtensionProjectsHealthAndState(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 11, 10, 0, 0, 0, time.UTC)
	payload := DescribeExtension(&Extension{
		Manifest: &Manifest{
			Capabilities: CapabilitiesConfig{Provides: []string{"runtime"}},
			Permissions:  PermissionsConfig{Requires: []string{"automation/jobs"}},
			Subprocess:   SubprocessConfig{Command: "ext-runtime"},
		},
		Info: ExtensionInfo{
			Name:    "ext-runtime",
			Version: "1.0.0",
			Enabled: true,
			Source:  SourceUser,
			Capabilities: CapabilitiesConfig{
				Provides: []string{"runtime"},
			},
			Permissions: PermissionsConfig{Requires: []string{"automation/jobs"}},
		},
		Status: ExtensionStatus{
			Active:        true,
			Healthy:       true,
			Registered:    true,
			PID:           42,
			LastStartedAt: now.Add(-5 * time.Minute),
		},
	}, true, now)

	if got, want := payload.Type, "subprocess"; got != want {
		t.Fatalf("DescribeExtension() type = %q, want %q", got, want)
	}
	if got, want := payload.State, "active"; got != want {
		t.Fatalf("DescribeExtension() state = %q, want %q", got, want)
	}
	if got, want := payload.Health, "healthy"; got != want {
		t.Fatalf("DescribeExtension() health = %q, want %q", got, want)
	}
	if payload.UptimeSeconds <= 0 {
		t.Fatalf("DescribeExtension() uptime_seconds = %d, want positive", payload.UptimeSeconds)
	}

	disabled := DescribeExtension(&Extension{
		Info: ExtensionInfo{
			Name:    "ext-disabled",
			Version: "1.0.0",
			Enabled: false,
			Source:  SourceWorkspace,
		},
		Status: ExtensionStatus{Registered: true},
	}, false, now)
	if got, want := disabled.Type, "resource"; got != want {
		t.Fatalf("DescribeExtension(disabled) type = %q, want %q", got, want)
	}
	if got, want := disabled.State, "disabled"; got != want {
		t.Fatalf("DescribeExtension(disabled) state = %q, want %q", got, want)
	}
	if got, want := disabled.Health, "unknown"; got != want {
		t.Fatalf("DescribeExtension(disabled) health = %q, want %q", got, want)
	}

	registered := DescribeExtension(&Extension{
		Info: ExtensionInfo{
			Name:    "ext-registered",
			Version: "1.0.0",
			Enabled: true,
			Source:  SourceUser,
		},
		Status: ExtensionStatus{
			Registered: true,
		},
	}, true, now)
	if got, want := registered.State, "registered"; got != want {
		t.Fatalf("DescribeExtension(registered) state = %q, want %q", got, want)
	}
	if got, want := registered.Health, "healthy"; got != want {
		t.Fatalf("DescribeExtension(registered) health = %q, want %q", got, want)
	}

	unhealthy := DescribeExtension(&Extension{
		Manifest: &Manifest{
			Capabilities: CapabilitiesConfig{Provides: []string{"runtime"}},
			Subprocess:   SubprocessConfig{Command: "ext-runtime"},
		},
		Info: ExtensionInfo{
			Name:    "ext-unhealthy",
			Version: "1.0.0",
			Enabled: true,
			Source:  SourceUser,
			Capabilities: CapabilitiesConfig{
				Provides: []string{"runtime"},
			},
		},
		Status: ExtensionStatus{
			LastError: "boom",
		},
	}, true, now)
	if got, want := unhealthy.State, "error"; got != want {
		t.Fatalf("DescribeExtension(unhealthy) state = %q, want %q", got, want)
	}
	if got, want := unhealthy.Health, "unhealthy"; got != want {
		t.Fatalf("DescribeExtension(unhealthy) health = %q, want %q", got, want)
	}

	enabled := DescribeExtension(&Extension{
		Info: ExtensionInfo{
			Name:    "ext-enabled",
			Version: "1.0.0",
			Enabled: true,
			Source:  SourceUser,
		},
	}, false, now)
	if got, want := enabled.State, "enabled"; got != want {
		t.Fatalf("DescribeExtension(enabled daemon stopped) state = %q, want %q", got, want)
	}
}

func TestHostAPIHandlerAutomationGetterAndMethodHandlers(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	handler := NewHostAPIHandler(
		env.sessions,
		env.memory,
		env.observer,
		env.skills,
		WithHostAPICapabilityChecker(env.checker),
		WithHostAPIWorkspaceResolver(env.workspaces),
		WithHostAPIAutomationGetter(func() HostAPIAutomationManager {
			return env.automation
		}),
	)

	handlers := handler.MethodHandlers()
	if _, ok := handlers[string(protocol.HostAPIMethodAutomationJobs)]; !ok {
		t.Fatal("MethodHandlers() missing automation/jobs handler")
	}
	for _, method := range []protocol.HostAPIMethod{
		protocol.HostAPIMethodTasksTimeline,
		protocol.HostAPIMethodTasksTree,
		protocol.HostAPIMethodTasksDashboard,
		protocol.HostAPIMethodTasksInbox,
		protocol.HostAPIMethodTasksRunsGet,
	} {
		if _, ok := handlers[string(method)]; !ok {
			t.Fatalf("MethodHandlers() missing %s handler", method)
		}
	}

	env.checker.Register("ext-automation", SourceUser, &Manifest{
		Permissions: PermissionsConfig{Requires: []string{"automation/jobs"}},
	})

	result, err := handler.Handle(testutil.Context(t), "ext-automation", "automation/jobs", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Handle(automation/jobs via getter) error = %v", err)
	}

	var jobsResult extensioncontract.AutomationJobsResult
	decodeResult(t, result, &jobsResult)
	if jobsResult.Jobs == nil {
		t.Fatal("automation/jobs result = nil, want empty slice")
	}
	if got, want := jobsResult.Page.Limit, automationpkg.DefaultListLimit; got != want {
		t.Fatalf("automation/jobs page limit = %d, want %d", got, want)
	}
	if jobsResult.Page.Total != 0 || jobsResult.Page.HasMore || jobsResult.Page.NextCursor != "" {
		t.Fatalf("automation/jobs page = %#v, want empty terminal page", jobsResult.Page)
	}
}

func TestHostAPIHandlerTaskOperationsRequireCapabilities(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-denied", nil, nil)

	tests := []struct {
		name   string
		method string
		params map[string]any
	}{
		{
			name:   "ShouldDenyCreate",
			method: "tasks/create",
			params: map[string]any{"scope": taskpkg.ScopeGlobal, "title": "Denied create"},
		},
		{
			name:   "ShouldDenyUpdate",
			method: "tasks/update",
			params: map[string]any{"id": "task-denied", "title": "Denied update"},
		},
		{
			name:   "ShouldDenyRunStart",
			method: "tasks/runs/start",
			params: map[string]any{"id": "run-denied", "idempotency_key": "idem-denied"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := env.call(t, "ext-denied", tt.method, tt.params)
			assertCapabilityDenied(t, err, tt.method)
		})
	}
}

func TestHostAPIHandlerTasksCreateUsesTrustedExtensionIdentity(t *testing.T) {
	t.Parallel()

	t.Run("Should reject client-supplied identity fields under strict decode", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			field string
			value map[string]any
		}{
			{
				name:  "Should reject created_by independently",
				field: "created_by",
				value: map[string]any{"kind": "human", "ref": "spoofed-user"},
			},
			{
				name:  "Should reject origin independently",
				field: "origin",
				value: map[string]any{"kind": "cli", "ref": "spoofed-origin"},
			},
		}
		for _, testCase := range tests {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()

				env := newHostAPITestEnv(t)
				env.grant("ext-tasks", []string{"tasks/create"}, []string{"task.write"})
				params := map[string]any{
					"scope": taskpkg.ScopeGlobal,
					"title": "Spoofed extension task",
				}
				params[testCase.field] = testCase.value
				_, err := env.call(t, "ext-tasks", "tasks/create", params)
				if err == nil {
					t.Fatalf("Handle(tasks/create with %s) error = nil, want invalid params", testCase.field)
				}
				assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
				assertErrorContains(t, err, testCase.field)
			})
		}
	})

	t.Run("Should stamp trusted extension identity on create", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		env.grant("ext-tasks", []string{"tasks/create"}, []string{"task.write"})
		result, err := env.call(t, "ext-tasks", "tasks/create", map[string]any{
			"scope": taskpkg.ScopeGlobal,
			"title": "Trusted extension task",
		})
		if err != nil {
			t.Fatalf("Handle(tasks/create) error = %v", err)
		}

		var created apicontract.TaskPayload
		decodeResult(t, result, &created)
		stored, err := env.registry.GetTask(testutil.Context(t), created.ID)
		if err != nil {
			t.Fatalf("registry.GetTask(%q) error = %v", created.ID, err)
		}
		if got, want := stored.CreatedBy.Kind, taskpkg.ActorKindExtension; got != want {
			t.Fatalf("stored.CreatedBy.Kind = %q, want %q", got, want)
		}
		if got, want := stored.CreatedBy.Ref, "ext-tasks"; got != want {
			t.Fatalf("stored.CreatedBy.Ref = %q, want %q", got, want)
		}
		if got, want := stored.Origin.Kind, taskpkg.OriginKindExtension; got != want {
			t.Fatalf("stored.Origin.Kind = %q, want %q", got, want)
		}
		if got, want := stored.Origin.Ref, "ext-tasks"; got != want {
			t.Fatalf("stored.Origin.Ref = %q, want %q", got, want)
		}
	})

	// Invariant: a Host API task response carries the identity of the profile
	// bound to the calling bridge. Owner: extension Host API task responses.
	// Canonical suite: extension Host API integration tests.
	t.Run("Should return the bridge-bound profile identity", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		env.grant("ext-profile-task", []string{"tasks/create"}, []string{"task.write"})
		ctx := env.bridgeContext(t, &bridgepkg.BridgeInstance{
			ID: "bridge-profile-task", ExtensionName: "ext-profile-task", ProfileID: env.marketingID,
		})
		result, err := env.callWithContext(ctx, t, "ext-profile-task", "tasks/create", map[string]any{
			"scope": taskpkg.ScopeGlobal,
			"title": "Marketing task",
		})
		if err != nil {
			t.Fatalf("Handle(tasks/create profile) error = %v", err)
		}

		var created apicontract.TaskPayload
		decodeResult(t, result, &created)
		if created.ProfileID != env.marketingID || created.ProfileName != "marketing" ||
			created.ProfileColor != "#e8572a" || created.ProfileIcon != "megaphone" {
			t.Fatalf("tasks/create profile owner = %#v, want marketing identity", created)
		}
	})
}

func TestHostAPIHandlerTaskRunStartAdmitsDirectExecutionWithoutClaimToken(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant(
		"ext-tasks",
		[]string{"tasks/create", "tasks/runs/enqueue", "tasks/runs/start"},
		[]string{"task.write"},
	)

	createResult, err := env.callFromWorkspace(t, "ext-tasks", "tasks/create", map[string]any{
		"scope":     taskpkg.ScopeWorkspace,
		"title":     "Lifecycle guard task",
		"workspace": env.workspaceID,
	})
	if err != nil {
		t.Fatalf("Handle(tasks/create) error = %v", err)
	}

	var created apicontract.TaskPayload
	decodeResult(t, createResult, &created)

	enqueueResult, err := env.callFromWorkspace(t, "ext-tasks", "tasks/runs/enqueue", map[string]any{
		"task_id":         created.ID,
		"idempotency_key": "enqueue-guard",
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs/enqueue) error = %v", err)
	}

	var run apicontract.TaskRunPayload
	decodeResult(t, enqueueResult, &run)

	startResult, err := env.callFromWorkspace(t, "ext-tasks", "tasks/runs/start", map[string]any{
		"id":              run.ID,
		"idempotency_key": "start-guard",
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs/start) error = %v", err)
	}

	var started apicontract.TaskRunPayload
	decodeResult(t, startResult, &started)
	if got, want := started.Status, taskpkg.TaskRunStatusRunning; got != want {
		t.Fatalf("tasks/runs/start status = %q, want %q", got, want)
	}
	if strings.TrimSpace(started.SessionID) == "" {
		t.Fatal("tasks/runs/start session_id = empty, want direct execution session")
	}
	env.assertDirectExecutionAdmission(t, created.ID, run.ID)
}

func TestHostAPIHandlerTasksListAndGetReturnFilteredDetail(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-reader", []string{"tasks", "tasks/get"}, []string{"task.read"})

	actor := mustExtensionTaskActorContext(t, "seed-writer", env.workspaceID)
	maxAttempts := 3
	parent, err := env.tasks.CreateTask(testutil.Context(t), taskpkg.CreateTask{
		ProfileID:   store.DefaultProfileID,
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: env.workspaceID,
		Title:       "Parent task",
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindExtension,
			Ref:  "ops",
		},
	}, actor)
	if err != nil {
		t.Fatalf("tasks.CreateTask(parent) error = %v", err)
	}

	child, err := env.tasks.CreateChildTask(testutil.Context(t), parent.ID, taskpkg.CreateTask{
		ProfileID:      store.DefaultProfileID,
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    env.workspaceID,
		Title:          "Filtered child",
		Priority:       taskpkg.PriorityHigh,
		MaxAttempts:    &maxAttempts,
		ApprovalPolicy: taskpkg.ApprovalPolicyManual,
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindExtension,
			Ref:  "ops",
		},
	}, actor)
	if err != nil {
		t.Fatalf("tasks.CreateChildTask(filtered) error = %v", err)
	}

	if _, err := env.tasks.CreateChildTask(testutil.Context(t), parent.ID, taskpkg.CreateTask{
		ProfileID:   store.DefaultProfileID,
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: env.workspaceID,
		Title:       "Draft child",
		Draft:       true,
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindExtension,
			Ref:  "ops",
		},
	}, actor); err != nil {
		t.Fatalf("tasks.CreateChildTask(draft) error = %v", err)
	}

	if _, err := env.tasks.CreateChildTask(testutil.Context(t), parent.ID, taskpkg.CreateTask{
		ProfileID:   store.DefaultProfileID,
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: env.workspaceID,
		Title:       "Other child",
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindPool,
			Ref:  "backlog",
		},
	}, actor); err != nil {
		t.Fatalf("tasks.CreateChildTask(other) error = %v", err)
	}

	blocker, err := env.tasks.CreateTask(testutil.Context(t), taskpkg.CreateTask{
		ProfileID:   store.DefaultProfileID,
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: env.workspaceID,
		Title:       "Blocking task",
	}, actor)
	if err != nil {
		t.Fatalf("tasks.CreateTask(blocker) error = %v", err)
	}
	if err := env.tasks.AddDependency(testutil.Context(t), taskpkg.AddDependency{
		TaskID:          child.ID,
		DependsOnTaskID: blocker.ID,
		Kind:            taskpkg.DependencyKindBlocks,
	}, actor); err != nil {
		t.Fatalf("tasks.AddDependency() error = %v", err)
	}

	run, err := env.tasks.EnqueueRun(testutil.Context(t), taskpkg.EnqueueRun{
		TaskID:         child.ID,
		IdempotencyKey: "seed-list-detail",
	}, actor)
	if err != nil {
		t.Fatalf("tasks.EnqueueRun() error = %v", err)
	}

	listResult, err := env.callFromWorkspace(t, "ext-reader", "tasks", map[string]any{
		"scope":          taskpkg.ScopeWorkspace,
		"workspace":      env.workspaceID,
		"priority":       taskpkg.PriorityHigh,
		"approval_state": taskpkg.ApprovalStatePending,
		"owner_kind":     taskpkg.OwnerKindExtension,
		"owner_ref":      "ops",
		"parent_task_id": parent.ID,
		"query":          "Filtered",
		"limit":          10,
	})
	if err != nil {
		t.Fatalf("Handle(tasks) error = %v", err)
	}

	var listedPage apicontract.TasksResponse
	decodeResult(t, listResult, &listedPage)
	listed := listedPage.Tasks
	if got, want := len(listed), 1; got != want {
		t.Fatalf("len(tasks) = %d, want %d", got, want)
	}
	if got, want := listedPage.Page.Total, 1; got != want {
		t.Fatalf("tasks.page.total = %d, want %d", got, want)
	}
	if got, want := listedPage.Page.Limit, 10; got != want {
		t.Fatalf("tasks.page.limit = %d, want %d", got, want)
	}
	if listedPage.Page.HasMore {
		t.Fatal("tasks.page.has_more = true, want false")
	}
	if got, want := listed[0].ID, child.ID; got != want {
		t.Fatalf("tasks[0].ID = %q, want %q", got, want)
	}
	if listed[0].Owner == nil {
		t.Fatal("tasks[0].Owner = nil, want extension owner")
	}
	if got, want := listed[0].Owner.Ref, "ops"; got != want {
		t.Fatalf("tasks[0].Owner.Ref = %q, want %q", got, want)
	}
	if got, want := listed[0].Priority, taskpkg.PriorityHigh; got != want {
		t.Fatalf("tasks[0].Priority = %q, want %q", got, want)
	}
	if got, want := listed[0].MaxAttempts, maxAttempts; got != want {
		t.Fatalf("tasks[0].MaxAttempts = %d, want %d", got, want)
	}
	if got, want := listed[0].ApprovalPolicy, taskpkg.ApprovalPolicyManual; got != want {
		t.Fatalf("tasks[0].ApprovalPolicy = %q, want %q", got, want)
	}
	if got, want := listed[0].ApprovalState, taskpkg.ApprovalStatePending; got != want {
		t.Fatalf("tasks[0].ApprovalState = %q, want %q", got, want)
	}
	if listed[0].Draft {
		t.Fatal("tasks[0].Draft = true, want filtered non-draft task")
	}
	if got, want := listed[0].DependencyCount, 1; got != want {
		t.Fatalf("tasks[0].DependencyCount = %d, want %d", got, want)
	}
	if listed[0].ActiveRun == nil {
		t.Fatal("tasks[0].ActiveRun = nil, want active run summary")
	}
	if listed[0].LastActivityAt == nil {
		t.Fatal("tasks[0].LastActivityAt = nil, want latest activity timestamp")
	}

	withDraftsResult, err := env.callFromWorkspace(t, "ext-reader", "tasks", map[string]any{
		"scope":          taskpkg.ScopeWorkspace,
		"workspace":      env.workspaceID,
		"owner_kind":     taskpkg.OwnerKindExtension,
		"owner_ref":      "ops",
		"parent_task_id": parent.ID,
		"include_drafts": true,
		"limit":          10,
	})
	if err != nil {
		t.Fatalf("Handle(tasks include_drafts) error = %v", err)
	}

	var withDraftsPage apicontract.TasksResponse
	decodeResult(t, withDraftsResult, &withDraftsPage)
	withDrafts := withDraftsPage.Tasks
	if got, want := len(withDrafts), 2; got != want {
		t.Fatalf("len(tasks include_drafts) = %d, want %d", got, want)
	}
	if got, want := withDraftsPage.Page.Total, 2; got != want {
		t.Fatalf("tasks include_drafts page.total = %d, want %d", got, want)
	}
	if !slices.ContainsFunc(withDrafts, func(item apicontract.TaskCatalogItemPayload) bool {
		return item.Draft && item.Status == taskpkg.TaskStatusDraft
	}) {
		t.Fatal("tasks include_drafts missing draft payload")
	}

	getResult, err := env.callFromWorkspace(t, "ext-reader", "tasks/get", map[string]any{"id": child.ID})
	if err != nil {
		t.Fatalf("Handle(tasks/get) error = %v", err)
	}

	var detail apicontract.TaskDetailPayload
	decodeResult(t, getResult, &detail)
	if got, want := detail.Summary.ID, child.ID; got != want {
		t.Fatalf("tasks/get.summary.id = %q, want %q", got, want)
	}
	if got, want := detail.Task.ID, child.ID; got != want {
		t.Fatalf("tasks/get.task.id = %q, want %q", got, want)
	}
	if got, want := detail.Task.Priority, taskpkg.PriorityHigh; got != want {
		t.Fatalf("tasks/get.task.priority = %q, want %q", got, want)
	}
	if got, want := detail.Task.MaxAttempts, maxAttempts; got != want {
		t.Fatalf("tasks/get.task.max_attempts = %d, want %d", got, want)
	}
	if got, want := detail.Task.ApprovalPolicy, taskpkg.ApprovalPolicyManual; got != want {
		t.Fatalf("tasks/get.task.approval_policy = %q, want %q", got, want)
	}
	if got, want := detail.Task.ApprovalState, taskpkg.ApprovalStatePending; got != want {
		t.Fatalf("tasks/get.task.approval_state = %q, want %q", got, want)
	}
	if got, want := len(detail.Dependencies), 1; got != want {
		t.Fatalf("len(tasks/get.dependencies) = %d, want %d", got, want)
	}
	if got, want := detail.Dependencies[0].DependsOnTaskID, blocker.ID; got != want {
		t.Fatalf("tasks/get.dependencies[0].depends_on_task_id = %q, want %q", got, want)
	}
	if got, want := len(detail.DependencyReferences), 1; got != want {
		t.Fatalf("len(tasks/get.dependency_references) = %d, want %d", got, want)
	}
	if got, want := detail.DependencyReferences[0].DependsOn.ID, blocker.ID; got != want {
		t.Fatalf("tasks/get.dependency_references[0].depends_on.id = %q, want %q", got, want)
	}
	if got, want := len(detail.Runs), 1; got != want {
		t.Fatalf("len(tasks/get.runs) = %d, want %d", got, want)
	}
	if got, want := detail.Runs[0].ID, run.ID; got != want {
		t.Fatalf("tasks/get.runs[0].id = %q, want %q", got, want)
	}
	if detail.Summary.ActiveRun == nil {
		t.Fatal("tasks/get.summary.active_run = nil, want run summary")
	}
	if len(detail.Events) == 0 {
		t.Fatal("tasks/get.events = 0, want audit events")
	}
}

func TestHostAPIHandlerTaskReadAndAggregateMethodsReturnParityPayloads(t *testing.T) {
	t.Parallel()

	usageStore := &recordingHostAPINetworkUsageStore{
		report: store.NetworkUsageReport{
			Total: store.NetworkUsageSummary{
				WakeCount:       1,
				ActualWakeCount: 1,
				ChargedWallTime: 3 * time.Second,
				InputTokens:     321,
				OutputTokens:    45,
			},
		},
	}
	env := newHostAPITestEnv(
		t,
		withHostAPITestLiveParticipation(),
		withHostAPITestNetworkUsageStore(usageStore),
	)
	env.grant(
		"ext-reader",
		[]string{
			"tasks",
			"tasks/runs/get",
			"tasks/runs/result",
			"tasks/timeline",
			"tasks/tree",
			"tasks/dashboard",
			"tasks/inbox",
		},
		[]string{"task.read"},
	)

	actor := mustExtensionTaskActorContext(t, "seed-writer", env.workspaceID)
	root, err := env.tasks.CreateTask(testutil.Context(t), taskpkg.CreateTask{
		ProfileID:   store.DefaultProfileID,
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: env.workspaceID,
		Title:       "Root task",
	}, actor)
	if err != nil {
		t.Fatalf("tasks.CreateTask(root) error = %v", err)
	}

	child, err := env.tasks.CreateChildTask(testutil.Context(t), root.ID, taskpkg.CreateTask{
		ProfileID:   store.DefaultProfileID,
		Scope:       taskpkg.ScopeWorkspace,
		WorkspaceID: env.workspaceID,
		Title:       "Child task",
		Priority:    taskpkg.PriorityUrgent,
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindExtension,
			Ref:  "ext-reader",
		},
	}, actor)
	if err != nil {
		t.Fatalf("tasks.CreateChildTask(child) error = %v", err)
	}

	approvalTask, err := env.tasks.CreateTask(testutil.Context(t), taskpkg.CreateTask{
		ProfileID:      store.DefaultProfileID,
		Scope:          taskpkg.ScopeWorkspace,
		WorkspaceID:    env.workspaceID,
		Title:          "Approval needed",
		ApprovalPolicy: taskpkg.ApprovalPolicyManual,
		Owner: &taskpkg.Ownership{
			Kind: taskpkg.OwnerKindExtension,
			Ref:  "ext-reader",
		},
	}, actor)
	if err != nil {
		t.Fatalf("tasks.CreateTask(approval) error = %v", err)
	}

	mode := participation.ModeLive
	strategy := participation.StrategyNamed
	channelID := "builders"
	queued, err := env.tasks.EnqueueRun(testutil.Context(t), taskpkg.EnqueueRun{
		TaskID:         child.ID,
		IdempotencyKey: "host-api-read-run",
		NetworkParticipation: &participation.Request{
			Mode:            &mode,
			ChannelStrategy: &strategy,
			ChannelID:       &channelID,
		},
		NetworkParticipationSource: participation.SourceExplicitRequest,
	}, actor)
	if err != nil {
		t.Fatalf("tasks.EnqueueRun() error = %v", err)
	}
	started, err := env.tasks.StartRun(testutil.Context(t), queued.ID, taskpkg.StartRun{
		IdempotencyKey: "host-api-read-start",
	}, actor)
	if err != nil {
		t.Fatalf("tasks.StartRun() error = %v", err)
	}
	wantParticipation := started.NetworkSpecSnapshot()
	if err := participation.ValidateSpec(wantParticipation); err != nil {
		t.Fatalf("started run participation = %#v, want valid Live snapshot: %v", wantParticipation, err)
	}

	filteredResult, err := env.callFromWorkspace(t, "ext-reader", "tasks", map[string]any{
		"participation_channel": channelID,
	})
	if err != nil {
		t.Fatalf("Handle(tasks participation channel filter) error = %v", err)
	}
	var filteredPage apicontract.TasksResponse
	decodeResult(t, filteredResult, &filteredPage)
	if got, want := len(filteredPage.Tasks), 1; got != want {
		t.Fatalf("len(tasks participation channel filter) = %d, want %d", got, want)
	}
	if got, want := filteredPage.Tasks[0].ID, child.ID; got != want {
		t.Fatalf("tasks participation channel filter id = %q, want %q", got, want)
	}
	if got := filteredPage.Tasks[0].ResolvedNetworkParticipation; got == nil || got.ChannelID != channelID {
		t.Fatalf("tasks participation channel filter snapshot = %#v, want channel %q", got, channelID)
	}

	runDetailResult, err := env.callFromWorkspace(
		t,
		"ext-reader",
		"tasks/runs/get",
		map[string]any{"id": started.ID},
	)
	if err != nil {
		t.Fatalf("Handle(tasks/runs/get) error = %v", err)
	}

	var runDetail apicontract.TaskRunDetailPayload
	decodeResult(t, runDetailResult, &runDetail)
	if got, want := runDetail.Run.ID, started.ID; got != want {
		t.Fatalf("tasks/runs/get.run.id = %q, want %q", got, want)
	}
	if runDetail.Task == nil {
		t.Fatal("tasks/runs/get.task = nil, want child task")
	}
	if got, want := runDetail.Task.ID, child.ID; got != want {
		t.Fatalf("tasks/runs/get.task.id = %q, want %q", got, want)
	}
	if runDetail.Session == nil {
		t.Fatal("tasks/runs/get.session = nil, want attached session")
	}
	if got, want := runDetail.Session.SessionID, started.SessionID; got != want {
		t.Fatalf("tasks/runs/get.session.session_id = %q, want %q", got, want)
	}
	if got := runDetail.Run.ResolvedNetworkParticipation; got == nil || *got != wantParticipation {
		t.Fatalf("tasks/runs/get.run.resolved_network_participation = %#v, want %#v", got, wantParticipation)
	}
	if runDetail.Network == nil {
		t.Fatal("tasks/runs/get.network = nil, want Live conversation and usage")
	}
	conversation := runDetail.Network.Conversation
	if conversation.WorkspaceID != env.workspaceID || conversation.Channel != channelID ||
		conversation.Surface != store.NetworkSurfaceThread ||
		conversation.ThreadID != apicontract.TaskRunConversationThreadID ||
		conversation.StreamURL != "/api/task-runs/"+started.ID+"/conversation/stream" {
		t.Fatalf("tasks/runs/get.network.conversation = %#v, want deterministic Live reference", conversation)
	}
	if got := runDetail.Network.Usage.Total; got.InputTokens != 321 || got.OutputTokens != 45 ||
		got.ActualWakeCount != 1 || got.ChargedWallTime != "3s" {
		t.Fatalf("tasks/runs/get.network.usage.total = %#v, want nonzero bound usage", got)
	}
	usageQuery := usageStore.lastQuery()
	if usageQuery.WorkspaceID != env.workspaceID || usageQuery.RunID != started.ID {
		t.Fatalf("tasks/runs/get usage query = %#v, want workspace/run fence", usageQuery)
	}

	timelineResult, err := env.callFromWorkspace(t, "ext-reader", "tasks/timeline", map[string]any{
		"id":    child.ID,
		"limit": 10,
	})
	if err != nil {
		t.Fatalf("Handle(tasks/timeline) error = %v", err)
	}

	var timeline []apicontract.TaskTimelineItemPayload
	decodeResult(t, timelineResult, &timeline)
	if len(timeline) == 0 {
		t.Fatal("tasks/timeline len = 0, want task events")
	}
	var timelineRun *apicontract.TaskRunSummaryPayload
	for i := range timeline {
		if timeline[i].Task.ID == child.ID && timeline[i].Run != nil && timeline[i].Run.ID == started.ID {
			timelineRun = timeline[i].Run
			break
		}
	}
	if timelineRun == nil {
		t.Fatal("tasks/timeline missing run-linked event for started run")
	}
	if got := timelineRun.ResolvedNetworkParticipation; got == nil || *got != wantParticipation {
		t.Fatalf("tasks/timeline run participation = %#v, want %#v", got, wantParticipation)
	}

	treeResult, err := env.callFromWorkspace(t, "ext-reader", "tasks/tree", map[string]any{"id": root.ID})
	if err != nil {
		t.Fatalf("Handle(tasks/tree) error = %v", err)
	}

	var tree apicontract.TaskTreePayload
	decodeResult(t, treeResult, &tree)
	if got, want := tree.Root.Task.ID, root.ID; got != want {
		t.Fatalf("tasks/tree.root.task.id = %q, want %q", got, want)
	}
	var treeRun *apicontract.TaskRunSummaryPayload
	for i := range tree.Descendants {
		if tree.Descendants[i].Task.ID == child.ID && tree.Descendants[i].ActiveRun != nil &&
			tree.Descendants[i].ActiveRun.ID == started.ID {
			treeRun = tree.Descendants[i].ActiveRun
			break
		}
	}
	if treeRun == nil {
		t.Fatal("tasks/tree missing child node with active run")
	}
	if got := treeRun.ResolvedNetworkParticipation; got == nil || *got != wantParticipation {
		t.Fatalf("tasks/tree active run participation = %#v, want %#v", got, wantParticipation)
	}

	dashboardResult, err := env.callFromWorkspace(t, "ext-reader", "tasks/dashboard", map[string]any{
		"scope":     taskpkg.ScopeWorkspace,
		"workspace": env.workspaceID,
	})
	if err != nil {
		t.Fatalf("Handle(tasks/dashboard) error = %v", err)
	}

	var dashboard apicontract.TaskDashboardPayload
	decodeResult(t, dashboardResult, &dashboard)
	if dashboard.Totals.ActiveRuns < 1 {
		t.Fatalf("tasks/dashboard active_runs = %d, want >= 1", dashboard.Totals.ActiveRuns)
	}
	var dashboardRun *apicontract.TaskDashboardActiveRunPayload
	for i := range dashboard.ActiveRuns.Items {
		if dashboard.ActiveRuns.Items[i].RunID == started.ID &&
			dashboard.ActiveRuns.Items[i].TaskID == child.ID {
			dashboardRun = &dashboard.ActiveRuns.Items[i]
			break
		}
	}
	if dashboardRun == nil {
		t.Fatal("tasks/dashboard active runs missing started run")
	}
	if got := dashboardRun.ResolvedNetworkParticipation; got == nil || *got != wantParticipation {
		t.Fatalf("tasks/dashboard active run participation = %#v, want %#v", got, wantParticipation)
	}
	if dashboardRun.LatestEventSeq == 0 {
		t.Fatal("tasks/dashboard active run latest_event_seq = 0, want durable stream fence")
	}

	inboxResult, err := env.callFromWorkspace(t, "ext-reader", "tasks/inbox", map[string]any{
		"scope":     taskpkg.ScopeWorkspace,
		"workspace": env.workspaceID,
		"lane":      apicontract.TaskInboxLaneApprovals,
		"limit":     10,
	})
	if err != nil {
		t.Fatalf("Handle(tasks/inbox) error = %v", err)
	}

	var inbox apicontract.TaskInboxPayload
	decodeResult(t, inboxResult, &inbox)
	if inbox.Page.Total < 1 || len(inbox.Groups) == 0 {
		t.Fatalf("tasks/inbox = %#v, want approval group", inbox)
	}
	if got, want := inbox.Groups[0].Lane, apicontract.TaskInboxLaneApprovals; got != want {
		t.Fatalf("tasks/inbox.groups[0].lane = %q, want %q", got, want)
	}
	if !slices.ContainsFunc(inbox.Groups[0].Items, func(item apicontract.TaskInboxItemPayload) bool {
		return item.Task.ID == approvalTask.ID && item.ApprovalState == taskpkg.ApprovalStatePending
	}) {
		t.Fatal("tasks/inbox approvals lane missing approval task")
	}

	t.Run("Should return an exact task-run result page", func(t *testing.T) {
		resultPayload := json.RawMessage(`{"message":"olá, mundo"}`)
		if _, err := env.tasks.CompleteRun(
			testutil.Context(t),
			started.ID,
			taskpkg.RunResult{Value: resultPayload},
			actor,
		); err != nil {
			t.Fatalf("tasks.CompleteRun(read result) error = %v", err)
		}
		const resultOffset int64 = 14
		const resultLimit int64 = 1
		resultPageValue, err := env.callFromWorkspace(t, "ext-reader", "tasks/runs/result", map[string]any{
			"id":     started.ID,
			"offset": resultOffset,
			"limit":  resultLimit,
		})
		if err != nil {
			t.Fatalf("Handle(tasks/runs/result) error = %v", err)
		}
		var resultPage apicontract.TaskRunResultPageResponse
		decodeResult(t, resultPageValue, &resultPage)
		decodedPage, err := base64.StdEncoding.DecodeString(resultPage.DataBase64)
		if err != nil {
			t.Fatalf("DecodeString(tasks/runs/result) error = %v", err)
		}
		if got, want := string(decodedPage), string(resultPayload[resultOffset:resultOffset+resultLimit]); got != want {
			t.Fatalf("tasks/runs/result bytes = %q, want %q", got, want)
		}
		if resultPage.Offset != resultOffset || resultPage.Bytes != resultLimit ||
			resultPage.TotalBytes != int64(len(resultPayload)) {
			t.Fatalf("tasks/runs/result page = %#v, want exact page descriptor", resultPage)
		}
	})
}

func TestHostAPIHandlerTasksUpdateAndCancelMutateTask(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant("ext-writer", []string{"tasks/create", "tasks/update", "tasks/cancel"}, []string{"task.write"})

	createResult, err := env.callFromWorkspace(t, "ext-writer", "tasks/create", map[string]any{
		"scope":           taskpkg.ScopeWorkspace,
		"workspace":       env.workspaceID,
		"title":           "Original title",
		"description":     "Original description",
		"priority":        taskpkg.PriorityLow,
		"max_attempts":    2,
		"approval_policy": taskpkg.ApprovalPolicyManual,
		"owner": map[string]any{
			"kind": taskpkg.OwnerKindPool,
			"ref":  "triage",
		},
		"metadata": map[string]any{"phase": "initial"},
	})
	if err != nil {
		t.Fatalf("Handle(tasks/create) error = %v", err)
	}

	var created apicontract.TaskPayload
	decodeResult(t, createResult, &created)

	updateResult, err := env.callFromWorkspace(t, "ext-writer", "tasks/update", map[string]any{
		"id":              created.ID,
		"title":           " Updated title ",
		"description":     " Updated description ",
		"priority":        taskpkg.PriorityHigh,
		"max_attempts":    5,
		"approval_policy": taskpkg.ApprovalPolicyNone,
		"owner": map[string]any{
			"kind": taskpkg.OwnerKindExtension,
			"ref":  "ext-writer",
		},
		"metadata": map[string]any{"phase": "updated"},
	})
	if err != nil {
		t.Fatalf("Handle(tasks/update) error = %v", err)
	}

	var updated apicontract.TaskPayload
	decodeResult(t, updateResult, &updated)
	if got, want := updated.Title, "Updated title"; got != want {
		t.Fatalf("tasks/update title = %q, want %q", got, want)
	}
	if got, want := updated.Description, "Updated description"; got != want {
		t.Fatalf("tasks/update description = %q, want %q", got, want)
	}
	if got, want := updated.Priority, taskpkg.PriorityHigh; got != want {
		t.Fatalf("tasks/update priority = %q, want %q", got, want)
	}
	if got, want := updated.MaxAttempts, 5; got != want {
		t.Fatalf("tasks/update max_attempts = %d, want %d", got, want)
	}
	if got, want := updated.ApprovalPolicy, taskpkg.ApprovalPolicyNone; got != want {
		t.Fatalf("tasks/update approval_policy = %q, want %q", got, want)
	}
	if updated.Owner == nil {
		t.Fatal("tasks/update owner = nil, want extension owner")
	}
	if got, want := updated.Owner.Ref, "ext-writer"; got != want {
		t.Fatalf("tasks/update owner.ref = %q, want %q", got, want)
	}
	if !strings.Contains(string(updated.Metadata), `"updated"`) {
		t.Fatalf("tasks/update metadata = %s, want updated marker", string(updated.Metadata))
	}

	clearOwnerResult, err := env.callFromWorkspace(t, "ext-writer", "tasks/update", map[string]any{
		"id":          created.ID,
		"clear_owner": true,
	})
	if err != nil {
		t.Fatalf("Handle(tasks/update clear_owner) error = %v", err)
	}

	var cleared apicontract.TaskPayload
	decodeResult(t, clearOwnerResult, &cleared)
	if cleared.Owner != nil {
		t.Fatalf("tasks/update clear_owner owner = %#v, want nil", cleared.Owner)
	}

	cancelResult, err := env.callFromWorkspace(t, "ext-writer", "tasks/cancel", map[string]any{
		"id":     created.ID,
		"reason": " user requested ",
		"metadata": map[string]any{
			"source": "host-api",
		},
	})
	if err != nil {
		t.Fatalf("Handle(tasks/cancel) error = %v", err)
	}

	var canceled apicontract.TaskPayload
	decodeResult(t, cancelResult, &canceled)
	if got, want := canceled.Status, taskpkg.TaskStatusCanceled; got != want {
		t.Fatalf("tasks/cancel status = %q, want %q", got, want)
	}
	if canceled.ClosedAt.IsZero() {
		t.Fatal("tasks/cancel closed_at = zero, want terminal timestamp")
	}

	stored, err := env.registry.GetTask(testutil.Context(t), created.ID)
	if err != nil {
		t.Fatalf("registry.GetTask(%q) error = %v", created.ID, err)
	}
	if got, want := stored.Status, taskpkg.TaskStatusCanceled; got != want {
		t.Fatalf("stored.Status = %q, want %q", got, want)
	}
	if stored.Owner != nil {
		t.Fatalf("stored.Owner = %#v, want nil after clear_owner", stored.Owner)
	}
}

func TestHostAPIHandlerTaskRunLifecycleOperationsAndFiltering(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant(
		"ext-runs",
		[]string{
			"tasks/create",
			"tasks/runs",
			"tasks/runs/enqueue",
			"tasks/runs/attach_session",
			"tasks/runs/start",
			"tasks/runs/complete",
			"tasks/runs/fail",
			"tasks/runs/cancel",
		},
		[]string{"task.read", "task.write"},
	)

	createTask := func(title string) apicontract.TaskPayload {
		t.Helper()

		result, err := env.callFromWorkspace(t, "ext-runs", "tasks/create", map[string]any{
			"scope":     taskpkg.ScopeWorkspace,
			"workspace": env.workspaceID,
			"title":     title,
		})
		if err != nil {
			t.Fatalf("Handle(tasks/create %q) error = %v", title, err)
		}
		var task apicontract.TaskPayload
		decodeResult(t, result, &task)
		return task
	}

	enqueueRun := func(taskID string, idempotencyKey string, metadata map[string]any) apicontract.TaskRunPayload {
		t.Helper()

		params := map[string]any{
			"task_id":         taskID,
			"idempotency_key": idempotencyKey,
		}
		if metadata != nil {
			params["metadata"] = metadata
		}
		result, err := env.callFromWorkspace(t, "ext-runs", "tasks/runs/enqueue", params)
		if err != nil {
			t.Fatalf("Handle(tasks/runs/enqueue %q) error = %v", taskID, err)
		}
		var run apicontract.TaskRunPayload
		decodeResult(t, result, &run)
		return run
	}

	assertMetadataPhase := func(label string, raw json.RawMessage, want string) {
		t.Helper()

		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			t.Fatalf("%s metadata unmarshal error = %v", label, err)
		}
		got, ok := decoded["phase"].(string)
		if !ok || got != want {
			t.Fatalf("%s metadata phase = %v, want %q", label, decoded["phase"], want)
		}
	}

	completedTask := createTask("Completed run task")
	completedQueued := enqueueRun(completedTask.ID, "enqueue-complete", map[string]any{
		"phase": "extension",
	})
	assertMetadataPhase("tasks/runs/enqueue", completedQueued.Metadata, "extension")
	boundSession := env.createSession(t)
	attachResult, err := env.callFromWorkspace(t, "ext-runs", "tasks/runs/attach_session", map[string]any{
		"id":         completedQueued.ID,
		"session_id": boundSession.ID,
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs/attach_session) error = %v", err)
	}

	var attached apicontract.TaskRunPayload
	decodeResult(t, attachResult, &attached)
	if got, want := attached.SessionID, boundSession.ID; got != want {
		t.Fatalf("tasks/runs/attach_session session_id = %q, want %q", got, want)
	}

	startResult, err := env.callFromWorkspace(t, "ext-runs", "tasks/runs/start", map[string]any{
		"id":              completedQueued.ID,
		"idempotency_key": "start-complete",
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs/start) error = %v", err)
	}

	var started apicontract.TaskRunPayload
	decodeResult(t, startResult, &started)
	if got, want := started.Status, taskpkg.TaskRunStatusRunning; got != want {
		t.Fatalf("tasks/runs/start status = %q, want %q", got, want)
	}

	completeResult, err := env.callFromWorkspace(t, "ext-runs", "tasks/runs/complete", map[string]any{
		"id":     completedQueued.ID,
		"result": map[string]any{"ok": true},
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs/complete) error = %v", err)
	}

	var completed apicontract.TaskRunPayload
	decodeResult(t, completeResult, &completed)
	if got, want := completed.Status, taskpkg.TaskRunStatusCompleted; got != want {
		t.Fatalf("tasks/runs/complete status = %q, want %q", got, want)
	}
	if !strings.Contains(string(completed.Result), `"ok":true`) {
		t.Fatalf("tasks/runs/complete result = %s, want ok marker", string(completed.Result))
	}
	env.assertDirectExecutionAdmission(t, completedTask.ID, completedQueued.ID)

	failedTask := createTask("Failed run task")
	failedQueued := enqueueRun(failedTask.ID, "enqueue-fail", nil)
	_, err = env.callFromWorkspace(t, "ext-runs", "tasks/runs/start", map[string]any{
		"id":              failedQueued.ID,
		"idempotency_key": "start-fail",
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs/start fail path) error = %v", err)
	}
	failResult, err := env.callFromWorkspace(t, "ext-runs", "tasks/runs/fail", map[string]any{
		"id":    failedQueued.ID,
		"error": " execution failed ",
		"metadata": map[string]any{
			"retryable": false,
		},
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs/fail) error = %v", err)
	}

	var failed apicontract.TaskRunPayload
	decodeResult(t, failResult, &failed)
	if got, want := failed.Status, taskpkg.TaskRunStatusFailed; got != want {
		t.Fatalf("tasks/runs/fail status = %q, want %q", got, want)
	}
	if got, want := failed.Error, "execution failed"; got != want {
		t.Fatalf("tasks/runs/fail error = %q, want %q", got, want)
	}
	env.assertDirectExecutionAdmission(t, failedTask.ID, failedQueued.ID)

	cancelledTask := createTask("Canceled run task")
	cancelledQueued := enqueueRun(cancelledTask.ID, "enqueue-cancel", nil)
	cancelRunResult, err := env.callFromWorkspace(t, "ext-runs", "tasks/runs/cancel", map[string]any{
		"id":     cancelledQueued.ID,
		"reason": " no longer needed ",
		"metadata": map[string]any{
			"source": "extension",
		},
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs/cancel) error = %v", err)
	}

	var canceled apicontract.TaskRunPayload
	decodeResult(t, cancelRunResult, &canceled)
	if got, want := canceled.Status, taskpkg.TaskRunStatusCanceled; got != want {
		t.Fatalf("tasks/runs/cancel status = %q, want %q", got, want)
	}

	runsResult, err := env.callFromWorkspace(t, "ext-runs", "tasks/runs", map[string]any{
		"id":         completedTask.ID,
		"status":     taskpkg.TaskRunStatusCompleted,
		"session_id": boundSession.ID,
		"limit":      1,
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs) error = %v", err)
	}

	var filtered []apicontract.TaskRunPayload
	decodeResult(t, runsResult, &filtered)
	if got, want := len(filtered), 1; got != want {
		t.Fatalf("len(tasks/runs) = %d, want %d", got, want)
	}
	if got, want := filtered[0].ID, completedQueued.ID; got != want {
		t.Fatalf("tasks/runs[0].id = %q, want %q", got, want)
	}
	if got, want := filtered[0].SessionID, boundSession.ID; got != want {
		t.Fatalf("tasks/runs[0].session_id = %q, want %q", got, want)
	}

	runsWithMetadataResult, err := env.callFromWorkspace(t, "ext-runs", "tasks/runs", map[string]any{
		"id":         completedTask.ID,
		"status":     taskpkg.TaskRunStatusCompleted,
		"session_id": boundSession.ID,
		"limit":      1,
	})
	if err != nil {
		t.Fatalf("Handle(tasks/runs metadata list) error = %v", err)
	}

	var runsWithMetadata []apicontract.TaskRunPayload
	decodeResult(t, runsWithMetadataResult, &runsWithMetadata)
	if got, want := len(runsWithMetadata), 1; got != want {
		t.Fatalf("len(tasks/runs metadata list) = %d, want %d", got, want)
	}
	assertMetadataPhase("tasks/runs list", runsWithMetadata[0].Metadata, "extension")
}

func TestHostAPIHandlerTaskMethodsValidateInputsAndConfiguration(t *testing.T) {
	t.Parallel()

	t.Run("ShouldRejectWhenTaskManagerIsMissing", func(t *testing.T) {
		t.Parallel()

		checker := &CapabilityChecker{}
		checker.Register("ext-tasks", SourceUser, &Manifest{
			Permissions: PermissionsConfig{Requires: []string{
				"tasks",
				"tasks/get",
				"tasks/timeline",
				"tasks/tree",
				"tasks/runs",
				"tasks/runs/get",
			}},
		})

		handler := NewHostAPIHandler(
			nil,
			nil,
			nil,
			nil,
			WithHostAPICapabilityChecker(checker),
			WithHostAPIRateLimit(1000, 1000),
		)

		tests := []struct {
			name   string
			method string
			params map[string]any
		}{
			{name: "ShouldRejectList", method: "tasks", params: map[string]any{}},
			{name: "ShouldRejectGet", method: "tasks/get", params: map[string]any{"id": "task-1"}},
			{name: "ShouldRejectTimeline", method: "tasks/timeline", params: map[string]any{"id": "task-1"}},
			{name: "ShouldRejectTree", method: "tasks/tree", params: map[string]any{"id": "task-1"}},
			{name: "ShouldRejectRuns", method: "tasks/runs", params: map[string]any{"id": "task-1"}},
			{name: "ShouldRejectRunGet", method: "tasks/runs/get", params: map[string]any{"id": "run-1"}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				params, err := marshalParams(tt.params)
				if err != nil {
					t.Fatalf("marshalParams() error = %v", err)
				}

				_, err = handler.Handle(testutil.Context(t), "ext-tasks", tt.method, params)
				assertErrorContains(t, err, "task manager is not configured")
			})
		}
	})

	t.Run("ShouldRejectWhenTaskObserverIsMissing", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		env.useSessionsWithoutObserver(t)
		env.grant("ext-tasks", []string{"tasks/dashboard", "tasks/inbox"}, []string{"task.read"})

		tests := []struct {
			name   string
			method string
			params map[string]any
		}{
			{name: "ShouldRejectDashboard", method: "tasks/dashboard", params: map[string]any{}},
			{name: "ShouldRejectInbox", method: "tasks/inbox", params: map[string]any{}},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := env.call(t, "ext-tasks", tt.method, tt.params)
				assertErrorContains(t, err, "task observer is not configured")
			})
		}
	})

	t.Run("ShouldRejectInvalidTaskMethodInputs", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		env.grant(
			"ext-tasks",
			[]string{
				"tasks",
				"tasks/timeline",
				"tasks/dashboard",
				"tasks/inbox",
				"tasks/create",
				"tasks/update",
				"tasks/runs/attach_session",
			},
			[]string{"task.read", "task.write"},
		)

		tests := []struct {
			name     string
			method   string
			params   map[string]any
			wantCode int
			wantText string
		}{
			{
				name:   "ShouldRejectUnknownWorkspace",
				method: "tasks/create",
				params: map[string]any{
					"scope":     taskpkg.ScopeWorkspace,
					"workspace": "ws-missing",
					"title":     "Missing workspace task",
				},
				wantCode: HostAPINotFoundCode,
				wantText: "workspace",
			},
			{
				name:   "ShouldRejectInvalidQueryScopeBeforeWorkspaceLookup",
				method: "tasks",
				params: map[string]any{
					"scope":     "invalid",
					"workspace": "ws-missing",
				},
				wantCode: HostAPIInvalidParamsCode,
				wantText: "task_query.scope",
			},
			{
				name:   "ShouldRejectGlobalCreateWorkspaceBindingBeforeWorkspaceLookup",
				method: "tasks/create",
				params: map[string]any{
					"scope":     taskpkg.ScopeGlobal,
					"workspace": "ws-missing",
					"title":     "Global task",
				},
				wantCode: HostAPIInvalidParamsCode,
				wantText: "create_task.workspace",
			},
			{
				name:   "ShouldRejectInvalidListChannel",
				method: "tasks",
				params: map[string]any{
					"participation_channel": "not valid",
				},
				wantCode: HostAPIInvalidParamsCode,
				wantText: "task_query.participation_channel",
			},
			{
				name:     "ShouldRequireUpdateChanges",
				method:   "tasks/update",
				params:   map[string]any{"id": "task-1"},
				wantCode: HostAPIInvalidParamsCode,
				wantText: "at least one mutable field",
			},
			{
				name:     "ShouldRequireAttachSessionID",
				method:   "tasks/runs/attach_session",
				params:   map[string]any{"id": "run-1"},
				wantCode: HostAPIInvalidParamsCode,
				wantText: "session_id is required",
			},
			{
				name:     "ShouldRejectInvalidTimelineAfterSequence",
				method:   "tasks/timeline",
				params:   map[string]any{"id": "task-1", "after_sequence": -1},
				wantCode: HostAPIInvalidParamsCode,
				wantText: "task_timeline_query.after_sequence",
			},
			{
				name:     "ShouldRejectInvalidDashboardChannel",
				method:   "tasks/dashboard",
				params:   map[string]any{"participation_channel": "not valid"},
				wantCode: HostAPIInvalidParamsCode,
				wantText: "task_dashboard_query.participation_channel",
			},
			{
				name:     "ShouldRejectInvalidInboxLane",
				method:   "tasks/inbox",
				params:   map[string]any{"lane": "bogus"},
				wantCode: HostAPIInvalidParamsCode,
				wantText: "task_inbox_query.lane",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				_, err := env.call(t, "ext-tasks", tt.method, tt.params)
				assertRPCErrorCode(t, err, tt.wantCode)
				assertErrorContains(t, err, tt.wantText)
			})
		}
	})
}

func TestHostAPIHandlerTaskMethodsRequireIdentifiers(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant(
		"ext-ids",
		[]string{
			"tasks/get",
			"tasks/timeline",
			"tasks/tree",
			"tasks/update",
			"tasks/cancel",
			"tasks/runs",
			"tasks/runs/get",
			"tasks/runs/enqueue",
			"tasks/runs/start",
			"tasks/runs/complete",
			"tasks/runs/fail",
			"tasks/runs/cancel",
		},
		[]string{"task.read", "task.write"},
	)

	tests := []struct {
		name     string
		method   string
		params   map[string]any
		wantText string
	}{
		{name: "ShouldRequireTaskIDForGet", method: "tasks/get", params: map[string]any{}, wantText: "id is required"},
		{
			name:     "ShouldRequireTaskIDForTimeline",
			method:   "tasks/timeline",
			params:   map[string]any{},
			wantText: "id is required",
		},
		{
			name:     "ShouldRequireTaskIDForTree",
			method:   "tasks/tree",
			params:   map[string]any{},
			wantText: "id is required",
		},
		{
			name:     "ShouldRequireTaskIDForUpdate",
			method:   "tasks/update",
			params:   map[string]any{"title": "rename"},
			wantText: "id is required",
		},
		{
			name:     "ShouldRequireTaskIDForCancel",
			method:   "tasks/cancel",
			params:   map[string]any{"reason": "stop"},
			wantText: "id is required",
		},
		{
			name:     "ShouldRequireTaskIDForRunsList",
			method:   "tasks/runs",
			params:   map[string]any{},
			wantText: "id is required",
		},
		{
			name:     "ShouldRequireRunIDForRunGet",
			method:   "tasks/runs/get",
			params:   map[string]any{},
			wantText: "id is required",
		},
		{
			name:     "ShouldRequireTaskIDForRunEnqueue",
			method:   "tasks/runs/enqueue",
			params:   map[string]any{"idempotency_key": "idem"},
			wantText: "task_id is required",
		},
		{
			name:     "ShouldRequireTaskIDForRunStart",
			method:   "tasks/runs/start",
			params:   map[string]any{"idempotency_key": "idem"},
			wantText: "id is required",
		},
		{
			name:     "ShouldRequireTaskIDForRunComplete",
			method:   "tasks/runs/complete",
			params:   map[string]any{"result": map[string]any{"ok": true}},
			wantText: "id is required",
		},
		{
			name:     "ShouldRequireTaskIDForRunFail",
			method:   "tasks/runs/fail",
			params:   map[string]any{"error": "boom"},
			wantText: "id is required",
		},
		{
			name:     "ShouldRequireTaskIDForRunCancel",
			method:   "tasks/runs/cancel",
			params:   map[string]any{"reason": "cancel"},
			wantText: "id is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := env.call(t, "ext-ids", tt.method, tt.params)
			assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
			assertErrorContains(t, err, tt.wantText)
		})
	}
}

func TestHostAPIHandlerTaskMethodsReturnNotFoundForMissingRecords(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant(
		"ext-missing",
		[]string{
			"tasks/get",
			"tasks/timeline",
			"tasks/tree",
			"tasks/update",
			"tasks/cancel",
			"tasks/runs",
			"tasks/runs/get",
			"tasks/runs/start",
			"tasks/runs/attach_session",
			"tasks/runs/complete",
			"tasks/runs/fail",
			"tasks/runs/cancel",
		},
		[]string{"task.read", "task.write"},
	)

	tests := []struct {
		name     string
		method   string
		params   map[string]any
		wantText string
	}{
		{
			name:     "ShouldReturnTaskNotFoundForGet",
			method:   "tasks/get",
			params:   map[string]any{"id": "task-missing"},
			wantText: "task not found",
		},
		{
			name:     "ShouldReturnTaskNotFoundForTimeline",
			method:   "tasks/timeline",
			params:   map[string]any{"id": "task-missing"},
			wantText: "task not found",
		},
		{
			name:     "ShouldReturnTaskNotFoundForTree",
			method:   "tasks/tree",
			params:   map[string]any{"id": "task-missing"},
			wantText: "task not found",
		},
		{
			name:     "ShouldReturnTaskNotFoundForUpdate",
			method:   "tasks/update",
			params:   map[string]any{"id": "task-missing", "title": "rename"},
			wantText: "task not found",
		},
		{
			name:     "ShouldReturnTaskNotFoundForCancel",
			method:   "tasks/cancel",
			params:   map[string]any{"id": "task-missing"},
			wantText: "task not found",
		},
		{
			name:     "ShouldReturnTaskNotFoundForListRuns",
			method:   "tasks/runs",
			params:   map[string]any{"id": "task-missing"},
			wantText: "task not found",
		},
		{
			name:     "ShouldReturnRunNotFoundForGetRun",
			method:   "tasks/runs/get",
			params:   map[string]any{"id": "run-missing"},
			wantText: "task run not found",
		},
		{
			name:     "ShouldReturnRunNotFoundForStart",
			method:   "tasks/runs/start",
			params:   map[string]any{"id": "run-missing", "idempotency_key": "idem"},
			wantText: "task run not found",
		},
		{
			name:     "ShouldReturnRunNotFoundForAttach",
			method:   "tasks/runs/attach_session",
			params:   map[string]any{"id": "run-missing", "session_id": "sess-missing"},
			wantText: "task run not found",
		},
		{
			name:     "ShouldReturnRunNotFoundForComplete",
			method:   "tasks/runs/complete",
			params:   map[string]any{"id": "run-missing", "result": map[string]any{"ok": true}},
			wantText: "task run not found",
		},
		{
			name:     "ShouldReturnRunNotFoundForFail",
			method:   "tasks/runs/fail",
			params:   map[string]any{"id": "run-missing", "error": "boom"},
			wantText: "task run not found",
		},
		{
			name:     "ShouldReturnRunNotFoundForCancel",
			method:   "tasks/runs/cancel",
			params:   map[string]any{"id": "run-missing"},
			wantText: "task run not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := env.call(t, "ext-missing", tt.method, tt.params)
			assertRPCErrorCode(t, err, HostAPINotFoundCode)
			assertErrorContains(t, err, tt.wantText)
		})
	}
}

func TestMapTaskRPCErrorTranslatesKnownErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resource string
		id       string
		err      error
		wantCode int
		wantText string
		wantNil  bool
		wantSame bool
	}{
		{name: "ShouldReturnNilForNilError", err: nil, wantNil: true},
		{
			name:     "ShouldMapWorkspaceNotFound",
			resource: "workspace",
			id:       "ws-1",
			err:      workspacepkg.ErrWorkspaceNotFound,
			wantCode: HostAPINotFoundCode,
			wantText: "workspace not found",
		},
		{
			name:     "ShouldMapTaskNotFound",
			resource: "task",
			id:       "task-1",
			err:      taskpkg.ErrTaskNotFound,
			wantCode: HostAPINotFoundCode,
			wantText: "task not found",
		},
		{
			name:     "ShouldMapRunNotFound",
			resource: "task_run",
			id:       "run-1",
			err:      taskpkg.ErrTaskRunNotFound,
			wantCode: HostAPINotFoundCode,
			wantText: "task run not found",
		},
		{
			name:     "ShouldMapDependencyNotFound",
			resource: "task_dependency",
			id:       "dep-1",
			err:      taskpkg.ErrTaskDependencyNotFound,
			wantCode: HostAPINotFoundCode,
			wantText: "task dependency not found",
		},
		{
			name:     "ShouldMapPermissionDenied",
			resource: "task",
			id:       "task-1",
			err:      taskpkg.ErrPermissionDenied,
			wantCode: HostAPIInvalidParamsCode,
			wantText: "permission denied",
		},
		{
			name:     "ShouldPassThroughUnknownErrors",
			resource: "task",
			id:       "task-1",
			err:      errors.New("boom"),
			wantSame: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mapped := mapTaskRPCError(tt.id, tt.err)
			if tt.wantNil {
				if mapped != nil {
					t.Fatalf("mapTaskRPCError() = %v, want nil", mapped)
				}
				return
			}
			if tt.wantSame {
				if !errors.Is(mapped, tt.err) {
					t.Fatalf("mapTaskRPCError() = %v, want same error %v", mapped, tt.err)
				}
				return
			}

			assertRPCErrorCode(t, mapped, tt.wantCode)
			assertErrorContains(t, mapped, tt.wantText)
		})
	}
}

func TestHostAPITaskHelpersHandleZeroAndUnavailableCases(t *testing.T) {
	t.Parallel()

	var nilHandler *HostAPIHandler
	_, err := nilHandler.taskManager()
	assertErrorContains(t, err, "host api handler is required")

	_, err = (&HostAPIHandler{}).taskManager()
	assertErrorContains(t, err, "task manager is not configured")

	_, err = (&HostAPIHandler{}).taskObserver()
	assertErrorContains(t, err, "task observer is not configured")

	t.Run("ShouldWrapTaskManagerResolutionError", func(t *testing.T) {
		t.Parallel()

		_, _, err := (&HostAPIHandler{}).taskManagerAndActor(testutil.Context(t))
		assertErrorContains(t, err, "resolve task manager")
		assertErrorContains(t, err, "task manager is not configured")
	})

	t.Run("ShouldWrapTaskActorContextErrorWhenExtensionNameMissing", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		_, _, err := env.handler.taskManagerAndActor(testutil.Context(t))
		assertErrorContains(t, err, "derive task actor context")
		assertErrorContains(t, err, "extension name is not available")
	})

	env := newHostAPITestEnv(t)

	t.Run("Should keep registered and stable workspace identities domain-specific", func(t *testing.T) {
		t.Parallel()

		const stableWorkspaceID = "01JSTABLEWORKSPACEIDENTITY0"
		resolved := env.workspace
		resolved.WorkspaceID = stableWorkspaceID
		env.workspaces.upsert(&resolved)

		workspaceID, err := env.handler.resolveTaskWorkspaceID(testutil.Context(t), stableWorkspaceID)
		if err != nil {
			t.Fatalf("resolveTaskWorkspaceID() error = %v", err)
		}
		if got, want := workspaceID, resolved.ID; got != want {
			t.Fatalf("resolveTaskWorkspaceID() = %q, want registered identity %q", got, want)
		}
		memoryWorkspaceID, err := env.handler.resolveStableWorkspaceID(testutil.Context(t), resolved.ID)
		if err != nil {
			t.Fatalf("resolveStableWorkspaceID() error = %v", err)
		}
		if got, want := memoryWorkspaceID, stableWorkspaceID; got != want {
			t.Fatalf("resolveStableWorkspaceID() = %q, want durable identity %q", got, want)
		}
	})

	raw, err := marshalParams(map[string]any{
		"scope": taskpkg.ScopeGlobal,
		"title": "No context task",
	})
	if err != nil {
		t.Fatalf("marshalParams() error = %v", err)
	}

	_, err = env.handler.handleTasksCreate(testutil.Context(t), raw)
	assertRPCErrorCode(t, err, HostAPIUnavailableCode)
	assertErrorContains(t, err, "extension name is not available")

	zeroTask := taskPayloadFromTask(nil)
	if zeroTask.ID != "" {
		t.Fatalf("taskPayloadFromTask(nil).ID = %q, want empty", zeroTask.ID)
	}

	zeroRun := taskRunPayloadFromRun(nil)
	if zeroRun.ID != "" {
		t.Fatalf("taskRunPayloadFromRun(nil).ID = %q, want empty", zeroRun.ID)
	}

	zeroDetail := taskDetailPayloadFromView(nil)
	if zeroDetail.Task.ID != "" {
		t.Fatalf("taskDetailPayloadFromView(nil).Task.ID = %q, want empty", zeroDetail.Task.ID)
	}

	zeroRunDetail, err := env.handler.taskRunDetailPayloadFromView(testutil.Context(t), nil)
	if err != nil {
		t.Fatalf("taskRunDetailPayloadFromView(nil) error = %v", err)
	}
	if zeroRunDetail.Run.ID != "" {
		t.Fatalf("taskRunDetailPayloadFromView(nil).Run.ID = %q, want empty", zeroRunDetail.Run.ID)
	}

	zeroTree := taskTreePayloadFromView(nil)
	if zeroTree.Root.Task.ID != "" {
		t.Fatalf("taskTreePayloadFromView(nil).Root.Task.ID = %q, want empty", zeroTree.Root.Task.ID)
	}

	zeroDashboard := taskDashboardPayloadFromView(nil)
	if zeroDashboard.Totals.TasksTotal != 0 {
		t.Fatalf("taskDashboardPayloadFromView(zero).Totals.TasksTotal = %d, want 0", zeroDashboard.Totals.TasksTotal)
	}

	zeroInbox := taskInboxPayloadFromView(observepkg.TaskInboxView{})
	if zeroInbox.Page.Total != 0 || len(zeroInbox.Groups) != 0 {
		t.Fatalf("taskInboxPayloadFromView(zero) = %#v, want zero payload", zeroInbox)
	}

	filtered := filterTaskRuns([]taskpkg.Run{
		{ID: "run-1", Status: taskpkg.TaskRunStatusRunning, SessionID: "sess-1"},
		{ID: "run-2", Status: taskpkg.TaskRunStatusCompleted, SessionID: "sess-2"},
		{ID: "run-3", Status: taskpkg.TaskRunStatusCompleted, SessionID: "sess-1"},
	}, taskpkg.RunQuery{
		Status:    taskpkg.TaskRunStatusCompleted,
		SessionID: "sess-1",
		Limit:     1,
	})
	if got, want := len(filtered), 1; got != want {
		t.Fatalf("len(filterTaskRuns) = %d, want %d", got, want)
	}
	if got, want := filtered[0].ID, "run-3"; got != want {
		t.Fatalf("filterTaskRuns()[0].ID = %q, want %q", got, want)
	}
}

func TestHostAPITaskPayloadsRedactRawClaimTokens(t *testing.T) {
	t.Parallel()

	run := taskpkg.Run{
		Error:    "run failed with compozy_claim_error-secret",
		Metadata: json.RawMessage(`{"claim_token":"compozy_claim_run-field","keep":"run-safe"}`),
	}
	run.SetResult(json.RawMessage(`{"note":"compozy_claim_result-secret"}`))
	view := taskpkg.View{
		Summary: taskpkg.Summary{Title: "summary compozy_claim_summary-secret"},
		Task: taskpkg.Task{
			Title:       "task compozy_claim_title-secret",
			Description: "description compozy_claim_description-secret",
			Metadata: json.RawMessage(
				`{"keep":"safe","claim_token":"compozy_claim_task-field","note":"compozy_claim_task-value"}`,
			),
		},
		Runs: []taskpkg.Run{run},
		Events: []taskpkg.Event{{
			Payload: json.RawMessage(`{"note":"compozy_claim_event-secret"}`),
		}},
	}
	timeline := taskTimelineItemPayloadFromItem(taskpkg.TimelineItem{
		Task:    taskpkg.Reference{Title: "timeline compozy_claim_reference-secret"},
		Run:     &taskpkg.RunSummary{Error: "summary compozy_claim_run-summary-secret"},
		Payload: json.RawMessage(`{"note":"compozy_claim_timeline-secret"}`),
	})
	payload := struct {
		Detail   apicontract.TaskDetailPayload       `json:"detail"`
		Timeline apicontract.TaskTimelineItemPayload `json:"timeline"`
	}{
		Detail:   taskDetailPayloadFromView(&view),
		Timeline: timeline,
	}
	content, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(task payloads) error = %v", err)
	}
	for _, secret := range []string{
		"compozy_claim_summary-secret",
		"compozy_claim_title-secret",
		"compozy_claim_description-secret",
		"compozy_claim_task-field",
		"compozy_claim_task-value",
		"compozy_claim_error-secret",
		"compozy_claim_run-field",
		"compozy_claim_result-secret",
		"compozy_claim_event-secret",
		"compozy_claim_reference-secret",
		"compozy_claim_run-summary-secret",
		"compozy_claim_timeline-secret",
	} {
		if strings.Contains(string(content), secret) {
			t.Fatalf("Host task payloads exposed raw claim token %q: %s", secret, content)
		}
	}
	if strings.Contains(string(content), `"claim_token"`) {
		t.Fatalf("Host task payloads exposed raw claim token material: %s", content)
	}
	for _, want := range []string{`"keep":"safe"`, `"keep":"run-safe"`} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("Host task payloads missing safe metadata %s: %s", want, content)
		}
	}
}

func TestHostAPIHandlerTaskMethodsRejectInvalidPayloadCombinations(t *testing.T) {
	t.Parallel()

	env := newHostAPITestEnv(t)
	env.grant(
		"ext-invalid",
		[]string{"tasks/create", "tasks/update", "tasks/runs/enqueue"},
		[]string{"task.write"},
	)

	_, err := env.call(t, "ext-invalid", "tasks/create", map[string]any{
		"scope":           taskpkg.ScopeGlobal,
		"title":           "Invalid channel task",
		"network_channel": "not valid",
	})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "network_channel")

	createResult, err := env.callFromWorkspace(t, "ext-invalid", "tasks/create", map[string]any{
		"scope":     taskpkg.ScopeWorkspace,
		"workspace": env.workspaceID,
		"title":     "Mutable task",
	})
	if err != nil {
		t.Fatalf("Handle(tasks/create mutable task) error = %v", err)
	}

	var created apicontract.TaskPayload
	decodeResult(t, createResult, &created)

	_, err = env.callFromWorkspace(t, "ext-invalid", "tasks/update", map[string]any{
		"id":          created.ID,
		"owner":       map[string]any{"kind": taskpkg.OwnerKindPool, "ref": "triage"},
		"clear_owner": true,
	})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "cannot both be set")

	_, err = env.callFromWorkspace(t, "ext-invalid", "tasks/runs/enqueue", map[string]any{
		"task_id":         created.ID,
		"idempotency_key": "idem-invalid-channel",
		"network_channel": "not valid",
	})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "network_channel")
}

func TestHostAPITaskRequestHelpersRejectInvalidPayloads(t *testing.T) {
	t.Parallel()

	oversizedMetadata := json.RawMessage(fmt.Sprintf("%q", strings.Repeat("m", taskpkg.MaxPayloadBytes+1)))
	oversizedResult := json.RawMessage(fmt.Sprintf("%q", strings.Repeat("r", taskpkg.MaxResultBytes+1)))

	_, err := cancelTaskFromRequest(apicontract.CancelTaskRequest{Metadata: oversizedMetadata})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "cancel_task.metadata")

	_, err = completeTaskRunFromRequest(apicontract.CompleteTaskRunRequest{Result: oversizedResult})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "run_result.value")

	_, err = failTaskRunFromRequest(apicontract.FailTaskRunRequest{})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "run_failure.error")

	_, err = cancelTaskRunFromRequest(apicontract.CancelTaskRunRequest{Metadata: oversizedMetadata})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "cancel_run.metadata")

	query, err := taskRunQueryFromParams(apicontract.TaskRunListQuery{
		ParticipationChannel: " builders ",
		Limit:                2,
	})
	if err != nil {
		t.Fatalf("taskRunQueryFromParams(valid) error = %v", err)
	}
	if query.ParticipationChannel != "builders" || query.Limit != 2 {
		t.Fatalf("taskRunQueryFromParams(valid) = %#v, want shared run filters", query)
	}

	_, err = taskRunQueryFromParams(apicontract.TaskRunListQuery{Limit: -1})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "task_run_query.limit")

	_, err = taskRunQueryFromParams(apicontract.TaskRunListQuery{ParticipationChannel: "not valid"})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "task_run_query.participation_channel")

	t.Run("Should preserve a workspace-bound worktree catalog filter", func(t *testing.T) {
		t.Parallel()
		env := newHostAPITestEnv(t)
		taskQuery, queryErr := env.handler.taskQueryFromParams(testutil.Context(t), hostAPITasksParams{
			Scope:     taskpkg.CatalogScopeWorkspace,
			Workspace: env.workspaceID,
			Worktree:  " wt-alpha ",
		})
		if queryErr != nil {
			t.Fatalf("taskQueryFromParams(worktree) error = %v", queryErr)
		}
		if taskQuery.WorkspaceID != env.workspaceID || taskQuery.WorktreeID != "wt-alpha" {
			t.Fatalf("taskQueryFromParams(worktree) = %#v, want workspace-bound worktree", taskQuery)
		}
	})

	t.Run("Should map Loop filters and calm aggregate defaults", func(t *testing.T) {
		t.Parallel()

		env := newHostAPITestEnv(t)
		ctx := testutil.Context(t)
		defaultQuery, queryErr := env.handler.taskQueryFromParams(ctx, hostAPITasksParams{})
		if queryErr != nil {
			t.Fatalf("taskQueryFromParams(default) error = %v", queryErr)
		}
		got := defaultQuery.ExcludeCreatedBy
		want := []taskpkg.ActorRef{{Kind: taskpkg.ActorKindDaemon, Ref: "loop-coordinator"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("default ExcludeCreatedBy = %#v, want %#v", got, want)
		}

		for _, testCase := range []struct {
			name   string
			params hostAPITasksParams
		}{
			{name: "Should include Loop records", params: hostAPITasksParams{IncludeLoop: true}},
			{name: "Should select a Loop run", params: hostAPITasksParams{LoopRunID: " looprun-host "}},
			{name: "Should select a parent task", params: hostAPITasksParams{ParentTaskID: " task-parent "}},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()
				mapped, mapErr := env.handler.taskQueryFromParams(ctx, testCase.params)
				if mapErr != nil {
					t.Fatalf("taskQueryFromParams(%#v) error = %v", testCase.params, mapErr)
				}
				if len(mapped.ExcludeCreatedBy) != 0 {
					t.Fatalf(
						"taskQueryFromParams(%#v).ExcludeCreatedBy = %#v, want empty",
						testCase.params,
						mapped.ExcludeCreatedBy,
					)
				}
			})
		}
		runQuery, queryErr := env.handler.taskQueryFromParams(ctx, hostAPITasksParams{
			LoopRunID: " looprun-host ",
		})
		if queryErr != nil {
			t.Fatalf("taskQueryFromParams(loop run) error = %v", queryErr)
		}
		if runQuery.LoopRunID != "looprun-host" {
			t.Fatalf("taskQueryFromParams(loop run).LoopRunID = %q, want trimmed id", runQuery.LoopRunID)
		}

		dashboardQuery, queryErr := env.handler.taskDashboardQueryFromParams(ctx, apicontract.TaskDashboardQuery{})
		if queryErr != nil {
			t.Fatalf("taskDashboardQueryFromParams(default) error = %v", queryErr)
		}
		inboxQuery, queryErr := env.handler.taskInboxQueryFromParams(ctx, apicontract.TaskInboxQuery{})
		if queryErr != nil {
			t.Fatalf("taskInboxQueryFromParams(default) error = %v", queryErr)
		}
		wantExclusions := []taskpkg.ActorRef{{Kind: taskpkg.ActorKindDaemon, Ref: "loop-coordinator"}}
		if !reflect.DeepEqual(dashboardQuery.ExcludeCreatedBy, wantExclusions) ||
			!reflect.DeepEqual(inboxQuery.ExcludeCreatedBy, wantExclusions) {
			t.Fatalf(
				"dashboard/inbox exclusions = %#v/%#v, want %#v",
				dashboardQuery.ExcludeCreatedBy, inboxQuery.ExcludeCreatedBy, wantExclusions,
			)
		}

		metadata := json.RawMessage(
			`{"loop_run_id":"looprun-host","loop_name":"host","generation":2,"node_id":"review","item_index":0}`,
		)
		view := taskpkg.View{
			Task: taskpkg.Task{ID: "task-loop-host", Metadata: metadata},
			Runs: []taskpkg.Run{{
				ID: "run-loop-host", TaskID: "task-loop-host", Attempt: 1,
				RunKind: taskpkg.RunKindWorker, LoopRunID: "looprun-host",
			}},
		}
		payload := taskDetailPayloadFromView(&view)
		if payload.Task.Loop == nil || payload.Task.Loop.RunID != "looprun-host" ||
			payload.Task.Loop.Role != apicontract.LoopProvenanceRoleCell ||
			payload.Task.Loop.NodeID != "review" {
			t.Fatalf("taskDetailPayloadFromView().Task.Loop = %#v, want structured cell provenance", payload.Task.Loop)
		}
	})

	env := newHostAPITestEnv(t)
	_, err = env.handler.taskQueryFromParams(testutil.Context(t), hostAPITasksParams{Limit: -1})
	assertRPCErrorCode(t, err, HostAPIInvalidParamsCode)
	assertErrorContains(t, err, "task_query.limit")
}

type hostAPITestEnv struct {
	nowMu          sync.RWMutex
	now            time.Time
	promptSequence atomic.Int64
	homePaths      compozyconfig.HomePaths
	workspaceID    string
	workspace      workspacepkg.ResolvedWorkspace
	registry       *globaldb.GlobalDB
	bridges        *bridgepkg.Service
	sessions       *session.Manager
	automation     HostAPIAutomationManager
	tasks          taskpkg.Manager
	observer       *observepkg.Observer
	memory         *memory.Store
	skills         *skillspkg.Registry
	workspaces     *hostAPIFakeWorkspaceResolver
	driver         *hostAPIFakeDriver
	resources      *resources.Kernel
	checker        *CapabilityChecker
	handler        *HostAPIHandler
	profiles       hostAPIProfileReader
	marketingID    string
}

type hostAPITestEnvConfig struct {
	hooks             *hookspkg.Hooks
	liveParticipation bool
	networkUsage      store.NetworkUsageStore
	loopStarter       automationpkg.LoopStarter
}

type hostAPITestEnvOption func(*hostAPITestEnvConfig)

func withHostAPITestLiveParticipation() hostAPITestEnvOption {
	return func(cfg *hostAPITestEnvConfig) {
		cfg.liveParticipation = true
	}
}

func withHostAPITestNetworkUsageStore(usageStore store.NetworkUsageStore) hostAPITestEnvOption {
	return func(cfg *hostAPITestEnvConfig) {
		cfg.networkUsage = usageStore
	}
}

func withHostAPITestLoopStarter(starter automationpkg.LoopStarter) hostAPITestEnvOption {
	return func(cfg *hostAPITestEnvConfig) {
		cfg.loopStarter = starter
	}
}

type hostAPITestLoopStarter struct{}

func (*hostAPITestLoopStarter) ValidateLoopTarget(
	context.Context,
	automationpkg.LoopTargetValidationRequest,
) error {
	return nil
}

func (*hostAPITestLoopStarter) StartLoop(
	context.Context,
	automationpkg.LoopStartRequest,
) (automationpkg.LoopStartResult, error) {
	return automationpkg.LoopStartResult{RunID: "looprun-host-api"}, nil
}

func newHostAPITestParticipationResolver(t testing.TB) participation.Resolver {
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
		Availability: func(context.Context) (bool, error) {
			return true, nil
		},
		ChannelExists: func(context.Context, string, string) (bool, error) {
			return true, nil
		},
		LiveSupport: func(context.Context, participation.ResolveInput) (bool, error) {
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("participation.NewResolver() error = %v", err)
	}
	return resolver
}

type recordingHostAPINetworkUsageStore struct {
	mu     sync.Mutex
	report store.NetworkUsageReport
	query  store.NetworkUsageQuery
}

func (s *recordingHostAPINetworkUsageStore) GetNetworkUsage(
	_ context.Context,
	query store.NetworkUsageQuery,
) (store.NetworkUsageReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.query = query
	return s.report, nil
}

func (s *recordingHostAPINetworkUsageStore) lastQuery() store.NetworkUsageQuery {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.query
}

type recordingHostAPIRecallProvider struct {
	stubMemoryProvider

	mu       sync.Mutex
	request  memcontract.RecallRequest
	packaged memcontract.Packaged
}

func (p *recordingHostAPIRecallProvider) Recall(
	_ context.Context,
	req memcontract.RecallRequest,
) (memcontract.RecallResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.request = req
	return memcontract.RecallResult{Packaged: p.packaged}, nil
}

func (p *recordingHostAPIRecallProvider) lastRequest() memcontract.RecallRequest {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.request
}

type hostAPITestTaskSessionExecutor struct {
	sessions            *session.Manager
	globalWorkspacePath string
}

func mustExtensionTaskActorContext(
	t testing.TB,
	extensionName string,
	workspaceID string,
) taskpkg.ActorContext {
	t.Helper()

	actor, err := taskpkg.DeriveExtensionActorContextForWorkspace(extensionName, workspaceID, "")
	if err != nil {
		t.Fatalf(
			"DeriveExtensionActorContextForWorkspace(%q, %q) error = %v",
			extensionName,
			workspaceID,
			err,
		)
	}
	return actor
}

func mustLocalSandboxRegistry(t testing.TB) *sandbox.Registry {
	t.Helper()

	registry, err := sandboxlocal.NewRegistry()
	if err != nil {
		t.Fatalf("local.NewRegistry() error = %v", err)
	}
	return registry
}

func (e *hostAPITestTaskSessionExecutor) StartTaskSession(
	ctx context.Context,
	spec *taskpkg.StartTaskSession,
) (*taskpkg.SessionRef, error) {
	if ctx == nil {
		return nil, errors.New("extension: host api test task start context is required")
	}
	if spec == nil {
		return nil, fmt.Errorf("%w: start task session spec is required", taskpkg.ErrValidation)
	}

	networkSpec := spec.Run.NetworkSpecSnapshot()
	opts := session.CreateOpts{
		AgentName:                    "coder",
		Name:                         "task:" + strings.TrimSpace(spec.Task.Title),
		ResolvedNetworkParticipation: &networkSpec,
		Type:                         session.SessionTypeSystem,
	}
	switch spec.Task.Scope.Normalize() {
	case taskpkg.ScopeWorkspace:
		opts.Workspace = strings.TrimSpace(spec.Task.WorkspaceID)
	case taskpkg.ScopeGlobal:
		opts.WorkspacePath = strings.TrimSpace(e.globalWorkspacePath)
	default:
		return nil, fmt.Errorf("%w: unsupported task scope %q", taskpkg.ErrValidation, spec.Task.Scope)
	}

	created, err := e.sessions.Create(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("start task session: create session: %w", err)
	}
	info := created.Info()
	if info == nil {
		return nil, fmt.Errorf("%w: task session create returned nil session info", taskpkg.ErrValidation)
	}
	return &taskpkg.SessionRef{
		SessionID:   info.ID,
		WorkspaceID: info.WorkspaceID,
		StartedAt:   info.CreatedAt,
	}, nil
}

func (e *hostAPITestTaskSessionExecutor) AttachTaskSession(
	ctx context.Context,
	_ string,
	sessionID string,
) (*taskpkg.SessionRef, error) {
	if ctx == nil {
		return nil, errors.New("extension: host api test task attach context is required")
	}

	info, err := e.sessions.Status(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, fmt.Errorf("attach task session: read session status: %w", err)
	}
	if info == nil || info.State != session.StateActive {
		return nil, fmt.Errorf(
			"%w: session %q is not active",
			taskpkg.ErrSessionAttachNotAllowed,
			strings.TrimSpace(sessionID),
		)
	}
	return &taskpkg.SessionRef{
		SessionID:   info.ID,
		WorkspaceID: info.WorkspaceID,
		StartedAt:   info.CreatedAt,
	}, nil
}

func (e *hostAPITestTaskSessionExecutor) RequestTaskStop(
	ctx context.Context,
	sessionID string,
	_ taskpkg.StopReason,
) error {
	if ctx == nil {
		return errors.New("extension: host api test task request stop context is required")
	}
	if err := e.sessions.RequestStopWithCause(
		ctx,
		strings.TrimSpace(sessionID),
		session.CauseUserRequested,
		"task cancellation",
	); err != nil {
		return fmt.Errorf("request task stop: %w", err)
	}
	return nil
}

func (e *hostAPITestTaskSessionExecutor) ForceTaskStop(
	ctx context.Context,
	sessionID string,
	_ taskpkg.StopReason,
) error {
	if ctx == nil {
		return errors.New("extension: host api test task force stop context is required")
	}
	if err := e.sessions.StopWithCause(
		ctx,
		strings.TrimSpace(sessionID),
		session.CauseUserRequested,
		"task cancellation",
	); err != nil {
		return fmt.Errorf("force task stop: %w", err)
	}
	return nil
}

func newHostAPITestEnv(t *testing.T, opts ...hostAPITestEnvOption) *hostAPITestEnv {
	t.Helper()

	cfg := hostAPITestEnvConfig{}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	homePaths, err := compozyconfig.ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	if err := compozyconfig.EnsureHomeLayout(homePaths); err != nil {
		t.Fatalf("EnsureHomeLayout() error = %v", err)
	}

	workspaceRoot := filepath.Join(t.TempDir(), "workspace")
	skillDir := filepath.Join(workspaceRoot, compozyconfig.DirName, compozyconfig.SkillsDirName, "workspace-review")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(skillDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: workspace-review
description: Review workspace changes
---

Review the workspace changes carefully.
`), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}

	baseNow := time.Date(2026, 4, 10, 18, 0, 0, 0, time.UTC)
	env := &hostAPITestEnv{now: baseNow, homePaths: homePaths}
	resolvedWorkspace := workspacepkg.ResolvedWorkspace{
		Workspace: workspacepkg.Workspace{
			ID:      "ws-host-api",
			RootDir: workspaceRoot,
			Name:    "host-api-workspace",
		},
		WorkspaceID: "ws-host-api",
		Config: compozyconfig.Config{
			Defaults: compozyconfig.DefaultsConfig{Agent: "coder"},
			Providers: map[string]compozyconfig.ProviderConfig{
				"fake":     {Command: "fake-agent"},
				"fake-alt": {Command: "fake-agent"},
			},
			Permissions: compozyconfig.PermissionsConfig{Mode: compozyconfig.PermissionModeApproveAll},
		},
		Agents: []compozyconfig.AgentDef{{
			Name:        "coder",
			Provider:    "fake",
			Permissions: string(compozyconfig.PermissionModeApproveAll),
			Prompt:      "You are a reliable coder.",
		}},
		Skills: []workspacepkg.SkillPath{{
			Dir:    skillDir,
			Source: "workspace",
		}},
		ResolvedAt: baseNow,
	}

	workspaces := newHostAPIFakeWorkspaceResolver(&resolvedWorkspace)
	driver := newHostAPIFakeDriver(baseNow)
	source := &hostAPISessionSource{}
	if err := extensionTestGlobalSeed.Clone(homePaths.DatabaseFile); err != nil {
		t.Fatalf("global store seed Clone() error = %v", err)
	}
	registry, err := globaldb.OpenGlobalDB(testutil.Context(t), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("globaldb.OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := registry.Close(testutil.Context(t)); err != nil {
			t.Errorf("registry.Close() cleanup error = %v", err)
		}
	})
	if err := registry.InsertWorkspace(testutil.Context(t), resolvedWorkspace.Workspace); err != nil {
		t.Fatalf("registry.InsertWorkspace() error = %v", err)
	}
	profileManager, err := profilepkg.NewManager(
		profilepkg.WithStore(registry),
		profilepkg.WithHomePaths(homePaths),
		profilepkg.WithClock(func() time.Time { return env.currentTime() }),
	)
	if err != nil {
		t.Fatalf("profile.NewManager() error = %v", err)
	}
	marketingProfile, err := profileManager.Create(testutil.Context(t), profilepkg.CreateInput{
		Name: "marketing", Color: "#E8572A", Icon: "megaphone",
	})
	if err != nil {
		t.Fatalf("profiles.Create(marketing) error = %v", err)
	}
	bridgeRegistry := bridgepkg.NewRegistry(registry, bridgepkg.WithNow(func() time.Time { return env.currentTime() }))
	resourceKernel, err := resources.NewKernel(
		registry.DB(),
		resources.WithNow(func() time.Time { return env.currentTime() }),
	)
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}
	resourceCodecs := resources.NewCodecRegistry()
	toolCodec, err := toolspkg.NewResourceCodec()
	if err != nil {
		t.Fatalf("toolspkg.NewResourceCodec() error = %v", err)
	}
	if err := resources.RegisterCodec(resourceCodecs, toolCodec); err != nil {
		t.Fatalf("resources.RegisterCodec(tool) error = %v", err)
	}
	mcpCodec, err := compozyconfig.NewMCPServerResourceCodec()
	if err != nil {
		t.Fatalf("compozyconfig.NewMCPServerResourceCodec() error = %v", err)
	}
	if err := resources.RegisterCodec(resourceCodecs, mcpCodec); err != nil {
		t.Fatalf("resources.RegisterCodec(mcp) error = %v", err)
	}

	observer, err := observepkg.New(testutil.Context(t),
		observepkg.WithRegistry(registry),
		observepkg.WithHomePaths(homePaths),
		observepkg.WithSessionSource(source),
		observepkg.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		observepkg.WithNow(func() time.Time { return env.currentTime().Add(time.Hour) }),
		observepkg.WithStartTime(baseNow),
	)
	if err != nil {
		t.Fatalf("observe.New() error = %v", err)
	}
	t.Cleanup(func() {
		if err := observer.Close(testutil.Context(t)); err != nil {
			t.Fatalf("observer.Close() error = %v", err)
		}
	})

	sessionOptions := []session.Option{
		session.WithHomePaths(homePaths),
		session.WithDriver(driver),
		session.WithNotifier(observer),
		session.WithWorkspaceResolver(workspaces),
		session.WithStore(storeSessionDB),
		session.WithSessionCatalog(registry),
		session.WithSessionPromptAdmissionStore(registry),
		session.WithSandboxRegistry(mustLocalSandboxRegistry(t)),
		session.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		session.WithNow(func() time.Time { return env.currentTime() }),
		session.WithSessionIDGenerator(sequentialSessionIDGenerator("sess")),
		session.WithTurnIDGenerator(sequentialSessionIDGenerator("turn")),
	}
	if cfg.liveParticipation {
		sessionOptions = append(
			sessionOptions,
			session.WithParticipationResolver(newHostAPITestParticipationResolver(t)),
		)
	}
	sessions, err := session.NewManager(sessionOptions...)
	if err != nil {
		t.Fatalf("session.NewManager() error = %v", err)
	}
	source.manager = sessions

	memoryCatalogPath := filepath.Join(homePaths.HomeDir, "memory-catalog.db")
	if err := extensionTestMemorySeed.Clone(memoryCatalogPath); err != nil {
		t.Fatalf("memory store seed Clone() error = %v", err)
	}
	memoryStore := memory.NewStore(
		homePaths.MemoryDir,
		memory.WithCatalogDatabasePath(memoryCatalogPath),
	)
	if err := memoryStore.OpenCatalog(testutil.Context(t)); err != nil {
		t.Fatalf("memory.OpenCatalog() error = %v", err)
	}
	t.Cleanup(func() {
		if err := memoryStore.CloseCatalog(testutil.Context(t)); err != nil {
			t.Errorf("memory.CloseCatalog() error = %v", err)
		}
	})
	if err := memoryStore.EnsureDirs(); err != nil {
		t.Fatalf("memory.EnsureDirs() error = %v", err)
	}

	skillsRegistry := skillspkg.NewRegistry(
		skillspkg.RegistryConfig{},
		skillspkg.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
	)
	checker := &CapabilityChecker{}
	automationOpts := []automationpkg.Option{
		automationpkg.WithStore(registry),
		automationpkg.WithSessions(sessions),
		automationpkg.WithWorkspaceResolver(workspaces),
		automationpkg.WithConfig(compozyconfig.AutomationConfig{
			Timezone:          automationpkg.DefaultTimezone,
			MaxConcurrentJobs: automationpkg.DefaultMaxConcurrentJobs,
			DefaultFireLimit:  automationpkg.DefaultFireLimitConfig(),
		}),
		automationpkg.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		automationpkg.WithGlobalWorkspacePath(homePaths.HomeDir),
	}
	if cfg.hooks != nil {
		automationOpts = append(automationOpts, automationpkg.WithHooks(cfg.hooks))
	}
	if cfg.loopStarter != nil {
		automationOpts = append(automationOpts, automationpkg.WithLoopStarter(cfg.loopStarter))
	}
	automationManager, err := automationpkg.New(automationOpts...)
	if err != nil {
		t.Fatalf("automation.New() error = %v", err)
	}
	if err := automationManager.Start(testutil.Context(t)); err != nil {
		t.Fatalf("automation.Start() error = %v", err)
	}
	t.Cleanup(func() {
		if err := automationManager.Shutdown(testutil.Context(t)); err != nil {
			t.Fatalf("automation.Shutdown() error = %v", err)
		}
	})

	taskOptions := []taskpkg.Option{
		taskpkg.WithStore(registry),
		taskpkg.WithSessionExecutor(&hostAPITestTaskSessionExecutor{
			sessions:            sessions,
			globalWorkspacePath: homePaths.HomeDir,
		}),
		taskpkg.WithManagerNow(func() time.Time { return env.currentTime() }),
	}
	if cfg.liveParticipation {
		taskOptions = append(
			taskOptions,
			taskpkg.WithParticipationResolver(newHostAPITestParticipationResolver(t)),
		)
	}
	taskManager, err := taskpkg.NewManager(taskOptions...)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}
	handler := NewHostAPIHandler(
		sessions,
		memoryStore,
		observer,
		skillsRegistry,
		WithHostAPIAutomationManager(automationManager),
		WithHostAPITaskManager(taskManager),
		WithHostAPITaskCatalogFilterMapper(hostAPITestTaskCatalogFilterMapper),
		WithHostAPIProfileReader(profileManager),
		WithHostAPIMemoryStoreResolver(func(ctx context.Context, profileID string) (*memory.Store, error) {
			profiles, err := profileManager.List(ctx)
			if err != nil {
				return nil, err
			}
			for _, profile := range profiles {
				if profile.ID == profileID {
					return memoryStore.ForProfile(
						profileID,
						filepath.Join(homePaths.ProfilesDir, profile.Name, compozyconfig.MemoryDirName),
					), nil
				}
			}
			return nil, fmt.Errorf("test profile %q was not found", profileID)
		}),
		WithHostAPICapabilityChecker(checker),
		WithHostAPIWorkspaceResolver(workspaces),
		WithHostAPIBridgeRegistry(bridgeRegistry),
		WithHostAPIBridgeDedupStore(registry),
		WithHostAPIResourceStore(resourceKernel),
		WithHostAPIResourceCodecRegistry(resourceCodecs),
		WithHostAPINetworkUsageStore(cfg.networkUsage),
		WithHostAPINow(func() time.Time { return env.currentTime() }),
		WithHostAPIBridgeIngressConfig(15*time.Minute, time.Minute),
		WithHostAPIRateLimit(1000, 1000),
	)

	env.workspaceID = resolvedWorkspace.WorkspaceID
	env.workspace = resolvedWorkspace
	env.registry = registry
	env.bridges = bridgeRegistry
	env.sessions = sessions
	env.automation = automationManager
	env.tasks = taskManager
	env.observer = observer
	env.memory = memoryStore
	env.skills = skillsRegistry
	env.workspaces = workspaces
	env.driver = driver
	env.resources = resourceKernel
	env.checker = checker
	env.handler = handler
	env.profiles = profileManager
	env.marketingID = marketingProfile.ID
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		manager := env.sessions
		if manager == nil {
			return
		}
		activeSessions := manager.List()
		if waitForHostAPIPromptsToSettle(ctx, t, manager, activeSessions) {
			if err := manager.WaitForPromptDrains(ctx); err != nil {
				t.Errorf("sessions.WaitForPromptDrains() cleanup error = %v", err)
			}
		}
		for _, info := range activeSessions {
			if info == nil {
				continue
			}
			if err := manager.Stop(ctx, info.ID); err != nil && !errors.Is(err, session.ErrSessionNotFound) {
				t.Errorf("sessions.Stop(%q) cleanup error = %v", info.ID, err)
			}
		}
		if err := manager.WaitForFinalizations(ctx); err != nil {
			t.Errorf("sessions.WaitForFinalizations() cleanup error = %v", err)
		}
		if err := manager.WaitForPromptDrains(ctx); err != nil {
			t.Errorf("sessions.WaitForPromptDrains() after stop cleanup error = %v", err)
		}
	})
	return env
}

func waitForHostAPIPromptsToSettle(
	ctx context.Context,
	t testing.TB,
	manager *session.Manager,
	sessions []*session.Info,
) bool {
	t.Helper()

	if manager == nil || len(sessions) == 0 {
		return true
	}

	for {
		allSettled := true
		for _, info := range sessions {
			if info != nil && manager.IsPrompting(info.ID) {
				allSettled = false
				break
			}
		}
		if allSettled {
			return true
		}
		if ctx != nil && ctx.Err() != nil {
			t.Errorf("timed out waiting for host API prompt cleanup: %v", ctx.Err())
			return false
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitForHostAPISessionState(
	t testing.TB,
	manager *session.Manager,
	sessionID string,
	want session.State,
) *session.Info {
	t.Helper()

	ctx, cancel := context.WithTimeout(testutil.Context(t), 2*time.Second)
	defer cancel()
	for {
		info, err := manager.Status(ctx, sessionID)
		if err == nil && info.State == want {
			return info
		}
		select {
		case <-ctx.Done():
			t.Fatalf("timed out waiting for session %q state %q: last info=%#v error=%v", sessionID, want, info, err)
			return nil
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (e *hostAPITestEnv) grant(extName string, permissions []string, _ []string) {
	e.checker.Register(extName, SourceUser, &Manifest{
		Permissions: PermissionsConfig{Requires: append([]string(nil), permissions...)},
	})
}

func (e *hostAPITestEnv) grantWithResources(
	t testing.TB,
	extName string,
	permissions []string,
	_ []string,
	resourceFamilies []string,
	maxScope resources.ResourceScopeKind,
) {
	t.Helper()

	_, err := e.checker.RegisterForSession(extName, SourceUser, &Manifest{
		Permissions: PermissionsConfig{Requires: append([]string(nil), permissions...)},
		Resources: ResourcesConfig{
			Publish: ResourceGrantRequest{
				Families: append([]string(nil), resourceFamilies...),
				MaxScope: maxScope,
			},
		},
	}, resources.ResourceScopeKindUser)
	if err != nil {
		t.Fatalf("RegisterForSession(%q) error = %v", extName, err)
	}
}

func (e *hostAPITestEnv) currentTime() time.Time {
	e.nowMu.RLock()
	defer e.nowMu.RUnlock()
	return e.now
}

func hostAPITestToolSpec(name string, description string, source string) map[string]any {
	return map[string]any{
		"id":            "ext__host_api__" + name,
		"display_title": name,
		"description":   description,
		"backend": map[string]any{
			"kind":         "extension_host",
			"extension_id": "host-api",
			"handler":      name,
		},
		"input_schema": map[string]any{"type": "object"},
		"source": map[string]any{
			"kind":          source,
			"owner":         "host-api",
			"raw_tool_name": name,
		},
		"visibility":       "operator",
		"risk":             "read",
		"read_only":        true,
		"concurrency_safe": true,
	}
}

func (e *hostAPITestEnv) advanceTime(delta time.Duration) time.Time {
	e.nowMu.Lock()
	defer e.nowMu.Unlock()
	e.now = e.now.Add(delta)
	return e.now
}

func (e *hostAPITestEnv) call(t testing.TB, extName string, method string, params any) (any, error) {
	t.Helper()

	eRaw, err := marshalParams(params)
	if err != nil {
		return nil, err
	}
	return e.handler.Handle(testutil.Context(t), extName, method, eRaw)
}

func (e *hostAPITestEnv) callFromWorkspace(
	t testing.TB,
	extName string,
	method string,
	params any,
) (any, error) {
	t.Helper()

	ctx := withHostAPIResourceSession(testutil.Context(t), &hostAPIResourceSession{
		Actor: resources.MutationActor{
			Kind:     resources.MutationActorKindExtension,
			ID:       extName,
			Source:   extensionResourceSource(extName),
			MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: e.workspaceID},
		},
	})
	return e.callWithContext(ctx, t, extName, method, params)
}

func (e *hostAPITestEnv) callWithContext(
	ctx context.Context,
	t testing.TB,
	extName string,
	method string,
	params any,
) (any, error) {
	t.Helper()

	eRaw, err := marshalParams(params)
	if err != nil {
		return nil, err
	}
	return e.handler.Handle(ctx, extName, method, eRaw)
}

func (e *hostAPITestEnv) resourceContext(t testing.TB, extName string, sessionNonce string) context.Context {
	t.Helper()

	grant := e.checker.Grant(extName)
	return withHostAPIResourceSession(testutil.Context(t), &hostAPIResourceSession{
		Actor: resources.MutationActor{
			Kind:         resources.MutationActorKindExtension,
			ID:           extName,
			SessionNonce: sessionNonce,
			Source: resources.ResourceSource{
				Kind: resources.ResourceSourceKind("extension"),
				ID:   extName,
			},
			MaxScope:      resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
			GrantedKinds:  append([]resources.ResourceKind(nil), grant.ResourceKinds...),
			GrantedScopes: append([]resources.ResourceScopeKind(nil), grant.ResourceScopes...),
		},
	})
}

func (e *hostAPITestEnv) activateResourceSession(t testing.TB, extName string, sessionNonce string) {
	t.Helper()

	if e.resources == nil {
		t.Fatal("resource kernel is not configured")
	}
	if err := e.resources.ActivateSourceSession(testutil.Context(t), resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "host-api-tests",
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   "host-api-tests",
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
	}, resources.ResourceSource{
		Kind: resources.ResourceSourceKind("extension"),
		ID:   extName,
	}, sessionNonce); err != nil {
		t.Fatalf("ActivateSourceSession(%q) error = %v", extName, err)
	}
}

func (e *hostAPITestEnv) callResource(
	t testing.TB,
	extName string,
	sessionNonce string,
	method string,
	params any,
) (any, error) {
	t.Helper()
	return e.callWithContext(e.resourceContext(t, extName, sessionNonce), t, extName, method, params)
}

func (e *hostAPITestEnv) bridgeContext(t testing.TB, instance *bridgepkg.BridgeInstance) context.Context {
	t.Helper()

	return e.bridgeContextForInstances(t, instance)
}

func (e *hostAPITestEnv) bridgeContextForInstances(
	t testing.TB,
	instances ...*bridgepkg.BridgeInstance,
) context.Context {
	t.Helper()

	if len(instances) == 0 {
		t.Fatal("bridge instances = empty, want at least one")
		return testutil.Context(t)
	}

	managed := make([]subprocess.InitializeBridgeManagedInstance, 0, len(instances))
	for _, instance := range instances {
		if instance == nil {
			t.Fatal("bridge instance = nil, want non-nil")
			return testutil.Context(t)
		}
		managed = append(managed, subprocess.InitializeBridgeManagedInstance{
			Instance: bridgepkg.BridgeInstanceToContract(*instance),
		})
	}

	return withHostAPIBridgeRuntime(testutil.Context(t), &subprocess.InitializeBridgeRuntime{
		RuntimeVersion:   subprocess.InitializeBridgeRuntimeVersion2,
		Purpose:          subprocess.BridgeRuntimePurposeService,
		Provider:         instances[0].ExtensionName,
		Platform:         instances[0].Platform,
		ManagedInstances: managed,
	})
}

func (e *hostAPITestEnv) submitPrompt(
	t testing.TB,
	extName string,
	sessionID string,
	message string,
) (hostAPISessionPromptResult, error) {
	return e.submitPromptWithRuntime(t, extName, sessionID, message, nil)
}

func (e *hostAPITestEnv) submitPromptWithRuntime(
	t testing.TB,
	extName string,
	sessionID string,
	message string,
	runtimeSelection *apicontract.PromptRuntimeSelectionPayload,
) (hostAPISessionPromptResult, error) {
	t.Helper()

	sequence := e.promptSequence.Add(1)
	params := map[string]any{
		"workspace_id":    e.workspaceID,
		"session_id":      sessionID,
		"message":         message,
		"message_id":      fmt.Sprintf("msg-host-%d", sequence),
		"idempotency_key": fmt.Sprintf("idem-host-%d", sequence),
	}
	if runtimeSelection != nil {
		params["runtime"] = runtimeSelection
	}
	result, err := e.call(t, extName, "sessions/prompt", params)
	if err != nil {
		return hostAPISessionPromptResult{}, err
	}
	var prompt hostAPISessionPromptResult
	decodeResult(t, result, &prompt)
	return prompt, nil
}

func (e *hostAPITestEnv) addForeignWorkspace(t testing.TB) workspacepkg.ResolvedWorkspace {
	t.Helper()

	foreign := e.workspace
	foreign.ID = "ws-foreign-registry"
	foreign.WorkspaceID = "ws-foreign"
	foreign.RootDir = t.TempDir()
	foreign.Name = "foreign"
	if err := e.registry.InsertWorkspace(testutil.Context(t), foreign.Workspace); err != nil {
		t.Fatalf("registry.InsertWorkspace(foreign) error = %v", err)
	}
	e.workspaces.upsert(&foreign)
	return foreign
}

func (e *hostAPITestEnv) createSession(t *testing.T) *session.Session {
	t.Helper()

	sess, err := e.sessions.Create(testutil.Context(t), session.CreateOpts{
		AgentName: "coder",
		Workspace: e.workspace.ID,
		Type:      session.SessionTypeSystem,
	})
	if err != nil {
		t.Fatalf("sessions.Create() error = %v", err)
	}
	t.Cleanup(func() {
		if stopErr := e.sessions.Stop(testutil.Context(t), sess.ID); stopErr != nil &&
			!errors.Is(stopErr, session.ErrSessionNotFound) {
			t.Errorf("sessions.Stop(%q) cleanup error = %v", sess.ID, stopErr)
		}
	})
	return sess
}

func (e *hostAPITestEnv) assertDirectExecutionAdmission(t testing.TB, taskID string, runID string) {
	t.Helper()

	ctx := testutil.Context(t)
	stored, err := e.registry.GetTaskRun(ctx, runID)
	if err != nil {
		t.Fatalf("registry.GetTaskRun(%q) error = %v", runID, err)
	}
	if strings.TrimSpace(stored.ClaimTokenHash) != "" ||
		!stored.LeaseUntil.IsZero() ||
		!stored.HeartbeatAt.IsZero() {
		t.Fatalf(
			"direct run claim fence = hash:%q lease:%s heartbeat:%s, want tokenless claim",
			stored.ClaimTokenHash,
			stored.LeaseUntil,
			stored.HeartbeatAt,
		)
	}

	events, err := e.registry.ListTaskEvents(ctx, taskpkg.EventQuery{TaskID: taskID, RunID: runID})
	if err != nil {
		t.Fatalf("registry.ListTaskEvents(%q) error = %v", runID, err)
	}
	claimedCount := 0
	for _, event := range events {
		if event.EventType != eventspkg.TaskRunClaimed {
			continue
		}
		claimedCount++
		if strings.Contains(string(event.Payload), "claim_token") {
			t.Fatalf("direct claim event payload exposes claim-token material: %s", event.Payload)
		}
	}
	if claimedCount != 1 {
		t.Fatalf("direct claim event count = %d, want 1", claimedCount)
	}
}

func (e *hostAPITestEnv) createBridgeInstance(
	t *testing.T,
	req bridgepkg.CreateInstanceRequest,
) *bridgepkg.BridgeInstance {
	t.Helper()

	if req.Scope == "" {
		req.Scope = bridgepkg.ScopeWorkspace
	}
	if req.WorkspaceID == "" && req.Scope == bridgepkg.ScopeWorkspace {
		req.WorkspaceID = e.workspaceID
	}
	if req.Platform == "" {
		req.Platform = "telegram"
	}
	if req.ExtensionName == "" {
		req.ExtensionName = "telegram-adapter"
	}
	if req.DisplayName == "" {
		req.DisplayName = "Telegram Test"
	}
	if !req.Enabled && req.Status == "" {
		req.Enabled = true
	}
	if req.Status == "" {
		req.Status = bridgepkg.BridgeStatusReady
	}

	instance, err := e.bridges.CreateInstance(testutil.Context(t), req)
	if err != nil {
		t.Fatalf("bridges.CreateInstance() error = %v", err)
	}
	return instance
}

func (e *hostAPITestEnv) useSessionsWithoutObserver(t *testing.T) {
	t.Helper()

	sessions, err := session.NewManager(
		session.WithHomePaths(e.homePaths),
		session.WithDriver(e.driver),
		session.WithWorkspaceResolver(e.workspaces),
		session.WithStore(storeSessionDB),
		session.WithSessionCatalog(e.registry),
		session.WithSessionPromptAdmissionStore(e.registry),
		session.WithSandboxRegistry(mustLocalSandboxRegistry(t)),
		session.WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))),
		session.WithNow(func() time.Time { return e.currentTime() }),
		session.WithSessionIDGenerator(sequentialSessionIDGenerator("sess")),
		session.WithTurnIDGenerator(sequentialSessionIDGenerator("turn")),
	)
	if err != nil {
		t.Fatalf("session.NewManager(without observer) error = %v", err)
	}

	taskManager, err := taskpkg.NewManager(
		taskpkg.WithStore(e.registry),
		taskpkg.WithSessionExecutor(&hostAPITestTaskSessionExecutor{
			sessions:            sessions,
			globalWorkspacePath: e.homePaths.HomeDir,
		}),
		taskpkg.WithManagerNow(func() time.Time { return e.currentTime() }),
	)
	if err != nil {
		t.Fatalf("task.NewManager(without observer) error = %v", err)
	}

	e.sessions = sessions
	e.tasks = taskManager
	e.handler = NewHostAPIHandler(
		e.sessions,
		e.memory,
		nil,
		e.skills,
		WithHostAPITaskManager(e.tasks),
		WithHostAPITaskCatalogFilterMapper(hostAPITestTaskCatalogFilterMapper),
		WithHostAPIProfileReader(e.profiles),
		WithHostAPICapabilityChecker(e.checker),
		WithHostAPIWorkspaceResolver(e.workspaces),
		WithHostAPIBridgeRegistry(e.bridges),
		WithHostAPIBridgeDedupStore(e.registry),
		WithHostAPIResourceStore(e.resources),
		WithHostAPINow(func() time.Time { return e.currentTime() }),
		WithHostAPIBridgeIngressConfig(15*time.Minute, time.Minute),
		WithHostAPIRateLimit(1000, 1000),
	)
}

type hostAPISessionSource struct {
	manager *session.Manager
}

func (s *hostAPISessionSource) List() []*session.Info {
	if s == nil || s.manager == nil {
		return nil
	}
	return s.manager.List()
}

type recordingHostAPISessionManager struct {
	createCalls         []session.CreateOpts
	acceptedCreateCalls []session.CreateAcceptedOpts
}

func (m *recordingHostAPISessionManager) Create(
	_ context.Context,
	opts session.CreateOpts,
) (*session.Session, error) {
	m.createCalls = append(m.createCalls, opts)
	return &session.Session{
		ID:          "sess-bridge",
		AgentName:   opts.AgentName,
		Provider:    "claude",
		WorkspaceID: opts.Workspace,
		Workspace:   opts.Workspace,
		Type:        opts.Type,
		State:       session.StateActive,
	}, nil
}

func (m *recordingHostAPISessionManager) CreateAccepted(
	_ context.Context,
	opts session.CreateAcceptedOpts,
) (*session.Info, error) {
	m.acceptedCreateCalls = append(m.acceptedCreateCalls, opts)
	return &session.Info{
		ID:        "sess-accepted",
		AgentName: opts.Session.AgentName,
		Provider:  opts.Session.Provider,
		State:     session.StateStarting,
	}, nil
}

func (*recordingHostAPISessionManager) ListAll(context.Context) ([]*session.Info, error) {
	return nil, errors.New("unexpected ListAll call")
}

func (*recordingHostAPISessionManager) Status(context.Context, string) (*session.Info, error) {
	return nil, errors.New("unexpected Status call")
}

func (*recordingHostAPISessionManager) Events(
	context.Context,
	string,
	store.EventQuery,
) ([]store.SessionEvent, error) {
	return nil, errors.New("unexpected Events call")
}

func (*recordingHostAPISessionManager) Stop(context.Context, string) error {
	return errors.New("unexpected Stop call")
}

func (*recordingHostAPISessionManager) Prompt(
	context.Context,
	string,
	string,
) (<-chan acp.AgentEvent, error) {
	return nil, errors.New("unexpected Prompt call")
}

func (*recordingHostAPISessionManager) ExecSandbox(
	context.Context,
	session.SandboxExecRequest,
) (session.SandboxExecResult, error) {
	return session.SandboxExecResult{}, errors.New("unexpected ExecSandbox call")
}

type hostAPIFakeWorkspaceResolver struct {
	mu       sync.Mutex
	resolved map[string]workspacepkg.ResolvedWorkspace
}

type recordingPromptDeliveryBroker struct {
	mu            sync.Mutex
	registrations []bridgepkg.PromptDeliveryRegistration
	projected     []bridgepkg.DeliveryProjectionEvent
}

func (b *recordingPromptDeliveryBroker) RegisterPromptDelivery(
	_ context.Context,
	reg bridgepkg.PromptDeliveryRegistration,
) (*bridgepkg.DeliverySnapshot, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	cloned := reg
	if len(cloned.SeedEvents) > 0 {
		cloned.SeedEvents = append([]bridgepkg.DeliveryProjectionEvent(nil), cloned.SeedEvents...)
	}
	b.registrations = append(b.registrations, cloned)
	return &bridgepkg.DeliverySnapshot{
		DeliveryID:       "del-test",
		SessionID:        reg.SessionID,
		TurnID:           reg.TurnID,
		BridgeInstanceID: reg.RoutingKey.BridgeInstanceID,
		RoutingKey:       reg.RoutingKey,
		DeliveryTarget:   reg.DeliveryTarget,
		LatestEventType:  bridgepkg.DeliveryEventTypeStart,
		UpdatedAt:        time.Now().UTC(),
	}, nil
}

func (b *recordingPromptDeliveryBroker) ProjectEvent(
	_ context.Context,
	_ string,
	event bridgepkg.DeliveryProjectionEvent,
) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.projected = append(b.projected, event)
	return nil
}

func (b *recordingPromptDeliveryBroker) snapshotRegistrations() []bridgepkg.PromptDeliveryRegistration {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]bridgepkg.PromptDeliveryRegistration, 0, len(b.registrations))
	for _, reg := range b.registrations {
		cloned := reg
		if len(cloned.SeedEvents) > 0 {
			cloned.SeedEvents = append([]bridgepkg.DeliveryProjectionEvent(nil), cloned.SeedEvents...)
		}
		out = append(out, cloned)
	}
	return out
}

func (b *recordingPromptDeliveryBroker) snapshotProjectedEvents() []bridgepkg.DeliveryProjectionEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	out := make([]bridgepkg.DeliveryProjectionEvent, 0, len(b.projected))
	out = append(out, b.projected...)
	return out
}

func newHostAPIFakeWorkspaceResolver(workspace *workspacepkg.ResolvedWorkspace) *hostAPIFakeWorkspaceResolver {
	resolver := &hostAPIFakeWorkspaceResolver{
		resolved: make(map[string]workspacepkg.ResolvedWorkspace),
	}
	resolver.upsert(workspace)
	return resolver
}

func (r *hostAPIFakeWorkspaceResolver) Resolve(
	ctx context.Context,
	idOrPath string,
) (workspacepkg.ResolvedWorkspace, error) {
	if err := ctx.Err(); err != nil {
		return workspacepkg.ResolvedWorkspace{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if resolved, ok := r.resolved[strings.TrimSpace(idOrPath)]; ok {
		return cloneResolvedWorkspaceForHostAPITests(&resolved), nil
	}
	if resolved, ok := r.resolved[normalizeHostAPIPath(idOrPath)]; ok {
		return cloneResolvedWorkspaceForHostAPITests(&resolved), nil
	}
	return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
}

func (r *hostAPIFakeWorkspaceResolver) ResolveOrRegister(
	ctx context.Context,
	path string,
) (workspacepkg.ResolvedWorkspace, error) {
	return r.Resolve(ctx, path)
}

func (r *hostAPIFakeWorkspaceResolver) upsert(workspace *workspacepkg.ResolvedWorkspace) {
	if workspace == nil {
		return
	}
	cloned := cloneResolvedWorkspaceForHostAPITests(workspace)
	if strings.TrimSpace(cloned.WorkspaceID) == "" {
		cloned.WorkspaceID = strings.TrimSpace(cloned.ID)
	}
	r.resolved[cloned.ID] = cloned
	if workspaceID := strings.TrimSpace(cloned.WorkspaceID); workspaceID != "" {
		r.resolved[workspaceID] = cloned
	}
	if name := strings.TrimSpace(cloned.Name); name != "" {
		r.resolved[name] = cloned
	}
	if root := normalizeHostAPIPath(cloned.RootDir); root != "" {
		r.resolved[root] = cloned
	}
}

func cloneResolvedWorkspaceForHostAPITests(src *workspacepkg.ResolvedWorkspace) workspacepkg.ResolvedWorkspace {
	if src == nil {
		return workspacepkg.ResolvedWorkspace{}
	}

	dst := *src
	dst.AdditionalDirs = append([]string(nil), src.AdditionalDirs...)
	dst.Agents = append([]compozyconfig.AgentDef(nil), src.Agents...)
	dst.Skills = append([]workspacepkg.SkillPath(nil), src.Skills...)
	return dst
}

func normalizeHostAPIPath(path string) string {
	target := strings.TrimSpace(path)
	if target == "" {
		return ""
	}
	absPath, err := filepath.Abs(target)
	if err != nil {
		return filepath.Clean(target)
	}
	return filepath.Clean(absPath)
}

type hostAPIFakeDriver struct {
	mu        sync.Mutex
	now       time.Time
	processes map[*session.AgentProcess]*hostAPIFakeProcess
	promptLog []acp.PromptRequest
	prompts   []acp.PromptRequest
	startSeq  atomic.Int64
}

type hostAPIFakeProcess struct {
	done sync.Once
	ch   chan struct{}
}

func newHostAPIFakeDriver(now time.Time) *hostAPIFakeDriver {
	return &hostAPIFakeDriver{
		now:       now,
		processes: make(map[*session.AgentProcess]*hostAPIFakeProcess),
	}
}

func (d *hostAPIFakeDriver) Start(_ context.Context, opts acp.StartOpts) (*session.AgentProcess, error) {
	seq := d.startSeq.Add(1)
	procState := &hostAPIFakeProcess{ch: make(chan struct{})}
	proc := session.NewAgentProcess(session.AgentProcessOptions{
		PID:       int(seq),
		AgentName: opts.AgentName,
		Command:   opts.Command,
		Cwd:       opts.Cwd,
		ToolHost:  opts.ToolHost,
		SessionID: fmt.Sprintf("acp-%d", seq),
		StartedAt: d.now.Add(time.Duration(seq) * time.Millisecond),
		Done:      procState.ch,
		Wait: func() error {
			<-procState.ch
			return nil
		},
	})

	d.mu.Lock()
	d.processes[proc] = procState
	d.mu.Unlock()
	return proc, nil
}

func (d *hostAPIFakeDriver) Prompt(
	_ context.Context,
	_ *session.AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	d.mu.Lock()
	d.promptLog = append(d.promptLog, req)
	d.mu.Unlock()

	d.mu.Lock()
	d.prompts = append(d.prompts, req)
	d.mu.Unlock()

	events := make(chan acp.AgentEvent, 2)
	go func() {
		defer close(events)
		events <- acp.AgentEvent{
			Type:      acp.EventTypeAgentMessage,
			TurnID:    req.TurnID,
			Timestamp: time.Now().UTC(),
			Text:      "ack: " + req.Message,
		}
		events <- acp.AgentEvent{
			Type:      acp.EventTypeDone,
			TurnID:    req.TurnID,
			Timestamp: time.Now().UTC(),
		}
	}()
	return events, nil
}

func (d *hostAPIFakeDriver) Cancel(context.Context, *session.AgentProcess) error {
	return nil
}

func (d *hostAPIFakeDriver) Stop(_ context.Context, proc *session.AgentProcess) error {
	d.mu.Lock()
	state := d.processes[proc]
	d.mu.Unlock()
	if state == nil {
		return nil
	}
	state.done.Do(func() { close(state.ch) })
	return nil
}

func (d *hostAPIFakeDriver) promptCount() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.prompts)
}

func storeSessionDB(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) (session.EventRecorder, error) {
	return sessiondbOpen(ctx, owner, path)
}

func sessiondbOpen(
	ctx context.Context,
	owner store.SessionDBOwner,
	path string,
) (session.EventRecorder, error) {
	return sessiondb.OpenSessionDB(ctx, owner, path)
}

func mustStoredPromptEvent(t *testing.T, id string, sequence int64, event acp.AgentEvent) store.SessionEvent {
	t.Helper()

	payload, err := transcriptpkg.MarshalAgentEvent(event)
	if err != nil {
		t.Fatalf("MarshalAgentEvent() error = %v", err)
	}
	return store.SessionEvent{
		ID:        id,
		Sequence:  sequence,
		TurnID:    event.TurnID,
		Type:      event.Type,
		AgentName: "coder",
		Content:   payload,
		Timestamp: event.Timestamp,
	}
}

type promptSessionManagerStub struct {
	sendPromptFn          func(context.Context, string, session.SendPromptOpts) (session.SendPromptResult, error)
	listPendingInputsFn   func(context.Context, string) ([]session.PendingInput, error)
	replacePendingInputFn func(context.Context, string, string, session.ReplacePendingInputOpts) (session.PendingInput, error)
	cancelQueuedPromptFn  func(context.Context, string, string) (session.SendPromptResult, error)
	promotePendingInputFn func(context.Context, string, string, session.PromotePendingInputOpts) (session.SendPromptResult, error)
	eventsFn              func(context.Context, string, store.EventQuery) ([]store.SessionEvent, error)
	statusFn              func(context.Context, string) (*session.Info, error)
	listAllFn             func(context.Context) ([]*session.Info, error)
}

func (s promptSessionManagerStub) Create(context.Context, session.CreateOpts) (*session.Session, error) {
	return nil, errors.New("unexpected create call")
}

func (s promptSessionManagerStub) ListAll(ctx context.Context) ([]*session.Info, error) {
	if s.listAllFn != nil {
		return s.listAllFn(ctx)
	}
	return nil, errors.New("unexpected list call")
}

func (s promptSessionManagerStub) Status(ctx context.Context, id string) (*session.Info, error) {
	if s.statusFn != nil {
		return s.statusFn(ctx, id)
	}
	return nil, errors.New("unexpected status call")
}

func (s promptSessionManagerStub) Events(
	ctx context.Context,
	id string,
	query store.EventQuery,
) ([]store.SessionEvent, error) {
	if s.eventsFn == nil {
		return nil, errors.New("unexpected events call")
	}
	return s.eventsFn(ctx, id, query)
}

func (s promptSessionManagerStub) Stop(context.Context, string) error {
	return errors.New("unexpected stop call")
}

func (promptSessionManagerStub) Prompt(context.Context, string, string) (<-chan acp.AgentEvent, error) {
	return nil, errors.New("unexpected prompt call")
}

func (s promptSessionManagerStub) SendPrompt(
	ctx context.Context,
	id string,
	opts session.SendPromptOpts,
) (session.SendPromptResult, error) {
	if s.sendPromptFn == nil {
		return session.SendPromptResult{}, errors.New("unexpected send prompt call")
	}
	return s.sendPromptFn(ctx, id, opts)
}

func (s promptSessionManagerStub) ListPendingInputs(ctx context.Context, id string) ([]session.PendingInput, error) {
	if s.listPendingInputsFn == nil {
		return nil, errors.New("unexpected list pending inputs call")
	}
	return s.listPendingInputsFn(ctx, id)
}

func (s promptSessionManagerStub) ReplacePendingInput(
	ctx context.Context,
	id string,
	entryID string,
	opts session.ReplacePendingInputOpts,
) (session.PendingInput, error) {
	if s.replacePendingInputFn == nil {
		return session.PendingInput{}, errors.New("unexpected replace pending input call")
	}
	return s.replacePendingInputFn(ctx, id, entryID, opts)
}

func (s promptSessionManagerStub) CancelQueuedPrompt(
	ctx context.Context,
	id string,
	entryID string,
) (session.SendPromptResult, error) {
	if s.cancelQueuedPromptFn == nil {
		return session.SendPromptResult{}, errors.New("unexpected cancel queued prompt call")
	}
	return s.cancelQueuedPromptFn(ctx, id, entryID)
}

func (s promptSessionManagerStub) PromotePendingInputToSteer(
	ctx context.Context,
	id string,
	entryID string,
	opts session.PromotePendingInputOpts,
) (session.SendPromptResult, error) {
	if s.promotePendingInputFn == nil {
		return session.SendPromptResult{}, errors.New("unexpected promote pending input call")
	}
	return s.promotePendingInputFn(ctx, id, entryID, opts)
}

func (s promptSessionManagerStub) ExecSandbox(
	context.Context,
	session.SandboxExecRequest,
) (session.SandboxExecResult, error) {
	return session.SandboxExecResult{}, errors.New("unexpected exec call")
}

func sequentialSessionIDGenerator(prefix string) session.IDGenerator {
	var counter atomic.Int64
	return func() (string, error) {
		return fmt.Sprintf("%s-%d", prefix, counter.Add(1)), nil
	}
}

func marshalParams(params any) (json.RawMessage, error) {
	if params == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(encoded), nil
}

func mustMarshalRawMessage(t testing.TB, params any) json.RawMessage {
	t.Helper()

	raw, err := marshalParams(params)
	if err != nil {
		t.Fatalf("marshalParams() error = %v", err)
	}
	return raw
}

func mustMarshalJSON(t testing.TB, value any) []byte {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return encoded
}

func decodeResult(t testing.TB, result any, target any) {
	t.Helper()

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal(result) error = %v", err)
	}
	if err := json.Unmarshal(encoded, target); err != nil {
		t.Fatalf("json.Unmarshal(result) error = %v", err)
	}
}

func assertCapabilityDenied(t testing.TB, err error, wantMethod string) {
	t.Helper()

	assertRPCErrorCode(t, err, CapabilityDeniedCode)
	data := decodeRPCData(t, err)
	if got := data["method"]; got != wantMethod {
		t.Fatalf("rpc data method = %v, want %q", got, wantMethod)
	}
	required, ok := data["required"].([]any)
	if !ok || len(required) == 0 {
		t.Fatalf("rpc data required = %#v, want non-empty slice", data["required"])
	}
}

func assertRPCErrorCode(t testing.TB, err error, want int) {
	t.Helper()

	rpcErr, rpcErrMatched := errors.AsType[*subprocess.RPCError](err)
	if !rpcErrMatched {
		t.Fatalf("error type = %T, want *subprocess.RPCError", err)
	}
	if rpcErr.Code != want {
		t.Fatalf("rpc error code = %d, want %d", rpcErr.Code, want)
	}
}

func assertErrorContains(t testing.TB, err error, fragment string) {
	t.Helper()

	if err == nil {
		t.Fatalf("error = nil, want containing %q", fragment)
	}
	if strings.Contains(err.Error(), fragment) {
		return
	}

	data := decodeRPCData(t, err)
	if raw, ok := data["error"].(string); ok && strings.Contains(raw, fragment) {
		return
	}
	t.Fatalf("error = %q with data %#v, want containing %q", err.Error(), data, fragment)
}

func decodeRPCData(t testing.TB, err error) map[string]any {
	t.Helper()

	rpcErr, rpcErrMatched := errors.AsType[*subprocess.RPCError](err)
	if !rpcErrMatched {
		t.Fatalf("error type = %T, want *subprocess.RPCError", err)
	}

	var data map[string]any
	if len(rpcErr.Data) == 0 {
		return data
	}
	if unmarshalErr := json.Unmarshal(rpcErr.Data, &data); unmarshalErr != nil {
		t.Fatalf("json.Unmarshal(rpcErr.Data) error = %v", unmarshalErr)
	}
	return data
}
