package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	compozycontract "github.com/compozy/compozy/internal/api/contract"

	sessionpkg "github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	taskpkg "github.com/compozy/compozy/internal/task"
)

func TestRuntimeHarnessCaptureHelpersPersistArtifacts(t *testing.T) {
	t.Parallel()
	t.Run("Should persist the captured runtime artifacts", func(t *testing.T) {
		t.Parallel()

		server := newHarnessTestServer(t)

		homePaths := NewHomePaths(t)
		harness := &RuntimeHarness{
			HomePaths:   homePaths,
			Artifacts:   NewArtifactCollector(t),
			HTTPBaseURL: server.URL,
			HTTPClient:  server.Client(),
			UDSBaseURL:  server.URL,
			UDSClient:   server.Client(),
		}

		metaPath := store.SessionMetaFile(filepath.Join(homePaths.SessionsDir, "sess-1"))
		if err := store.WriteSessionMeta(metaPath, &store.SessionMeta{
			ID:            "sess-1",
			AgentName:     "coder",
			WorkspaceID:   "ws-1",
			RuntimeStatus: store.SessionRuntimeUnbound,

			State: "stopped",
		}); err != nil {
			t.Fatalf("WriteSessionMeta(%q) error = %v", metaPath, err)
		}

		workspace, err := harness.ResolveWorkspace(testContext(t), "/workspace")
		if err != nil {
			t.Fatalf("ResolveWorkspace() error = %v", err)
		}
		if got, want := workspace.ID, "ws-1"; got != want {
			t.Fatalf("workspace.ID = %q, want %q", got, want)
		}

		gotWorkspace, err := harness.GetWorkspace(testContext(t), "ws-1")
		if err != nil {
			t.Fatalf("GetWorkspace() error = %v", err)
		}
		if got, want := gotWorkspace.RootDir, "/workspace"; got != want {
			t.Fatalf("gotWorkspace.RootDir = %q, want %q", got, want)
		}

		extensions, err := harness.ListExtensions(testContext(t))
		if err != nil {
			t.Fatalf("ListExtensions() error = %v", err)
		}
		if got, want := len(extensions), 1; got != want {
			t.Fatalf("len(extensions) = %d, want %d", got, want)
		}

		installedExtension, err := harness.InstallExtension(testContext(t), compozycontract.InstallExtensionRequest{
			Source: compozycontract.InstallExtensionSourceLocalPath,
			Ref:    "/extensions/tool-provider",
		})
		if err != nil {
			t.Fatalf("InstallExtension() error = %v", err)
		}
		if got, want := installedExtension.Name, "tool-provider"; got != want {
			t.Fatalf("installedExtension.Name = %q, want %q", got, want)
		}

		gotExtension, err := harness.GetExtension(testContext(t), "tool-provider")
		if err != nil {
			t.Fatalf("GetExtension() error = %v", err)
		}
		if got, want := gotExtension.State, "registered"; got != want {
			t.Fatalf("gotExtension.State = %q, want %q", got, want)
		}

		enabledExtension, err := harness.SetExtensionEnablement(
			testContext(t), "tool-provider", "default", true,
		)
		if err != nil {
			t.Fatalf("SetExtensionEnablement(enabled) error = %v", err)
		}
		if !enabledExtension.Enabled || enabledExtension.Profile != "default" {
			t.Fatalf("enabledExtension = %#v, want default enabled", enabledExtension)
		}

		disabledExtension, err := harness.SetExtensionEnablement(
			testContext(t), "tool-provider", "default", false,
		)
		if err != nil {
			t.Fatalf("SetExtensionEnablement(disabled) error = %v", err)
		}
		if disabledExtension.Enabled || disabledExtension.Profile != "default" {
			t.Fatalf("disabledExtension = %#v, want default disabled", disabledExtension)
		}

		session, err := harness.CreateSession(testContext(t), compozycontract.CreateSessionRequest{
			AgentName:     "coder",
			WorkspacePath: "/workspace",
		})
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		if got, want := session.ID, "sess-1"; got != want {
			t.Fatalf("session.ID = %q, want %q", got, want)
		}

		gotSession, err := harness.GetSession(testContext(t), "sess-1")
		if err != nil {
			t.Fatalf("GetSession() error = %v", err)
		}
		if got, want := gotSession.ID, "sess-1"; got != want {
			t.Fatalf("gotSession.ID = %q, want %q", got, want)
		}
		resumedSession, err := harness.ResumeSession(testContext(t), "sess-1")
		if err != nil {
			t.Fatalf("ResumeSession() error = %v", err)
		}
		if got, want := resumedSession.State, sessionpkg.StateActive; got != want {
			t.Fatalf("resumedSession.State = %q, want %q", got, want)
		}

		stream, err := harness.PromptSession(testContext(t), "sess-1", "hello world")
		if err != nil {
			t.Fatalf("PromptSession() error = %v", err)
		}
		if got, want := len(stream), 2; got != want {
			t.Fatalf("len(stream) = %d, want %d", got, want)
		}
		if got, want := stream[0].Event, "agent_message"; got != want {
			t.Fatalf("stream[0].Event = %q, want %q", got, want)
		}

		if _, err := harness.SessionTranscript(testContext(t), "sess-1"); err != nil {
			t.Fatalf("SessionTranscript() error = %v", err)
		}
		if _, err := harness.SessionEvents(testContext(t), "sess-1"); err != nil {
			t.Fatalf("SessionEvents() error = %v", err)
		}
		if err := harness.StopSession(testContext(t), "sess-1"); err != nil {
			t.Fatalf("StopSession() error = %v", err)
		}

		if err := harness.CaptureSessionTranscript(testContext(t), "sess-1"); err != nil {
			t.Fatalf("CaptureSessionTranscript() error = %v", err)
		}
		if err := harness.CaptureSessionEvents(testContext(t), "sess-1"); err != nil {
			t.Fatalf("CaptureSessionEvents() error = %v", err)
		}

		if err := harness.CaptureAutomationRuns(testContext(t), url.Values{"status": {"completed"}}); err != nil {
			t.Fatalf("CaptureAutomationRuns() error = %v", err)
		}
		if err := harness.CaptureTasks(testContext(t), url.Values{"limit": {"10"}}); err != nil {
			t.Fatalf("CaptureTasks() error = %v", err)
		}
		if err := harness.CaptureTaskRuns(testContext(t), "task-1", url.Values{"status": {"completed"}}); err != nil {
			t.Fatalf("CaptureTaskRuns() error = %v", err)
		}
		if err := harness.CaptureToolHostDiagnosticsJSON(ToolHostDiagnosticsArtifact{
			SessionID: "sess-1",
			Operations: []ToolHostOperationDiagnostic{{
				Operation:        "write_text_file",
				Path:             "toolhost/output.txt",
				Outcome:          ToolHostOutcomeAllowed,
				SideEffectPath:   "/workspace/toolhost/output.txt",
				SideEffectExists: true,
			}},
		}); err != nil {
			t.Fatalf("CaptureToolHostDiagnosticsJSON() error = %v", err)
		}
		if err := harness.CaptureCombinedFlowJSON(CombinedFlowArtifact{
			Scenario:  "session-automation-task",
			SessionID: "sess-1",

			AutomationRunID: "run-1",
			TaskID:          "task-1",
			TaskRunID:       "task-run-1",

			SideEffectPaths: []string{"/workspace/toolhost/output.txt"},
		}); err != nil {
			t.Fatalf("CaptureCombinedFlowJSON() error = %v", err)
		}

		providerLog := filepath.Join(t.TempDir(), "provider.log")
		if err := os.WriteFile(providerLog, []byte("provider call"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", providerLog, err)
		}
		if err := harness.CaptureProviderCallsFile(providerLog, "text/plain"); err != nil {
			t.Fatalf("CaptureProviderCallsFile() error = %v", err)
		}

		tracePath := filepath.Join(t.TempDir(), "trace.zip")
		if err := os.WriteFile(tracePath, []byte("trace-bytes"), 0o644); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", tracePath, err)
		}
		if err := harness.CaptureBrowserTraceFile(tracePath); err != nil {
			t.Fatalf("CaptureBrowserTraceFile() error = %v", err)
		}

		screenshotOne := filepath.Join(t.TempDir(), "screen-1.png")
		screenshotTwo := filepath.Join(t.TempDir(), "screen-2.png")
		for _, item := range []struct {
			path string
			data string
		}{
			{path: screenshotOne, data: "one"},
			{path: screenshotTwo, data: "two"},
		} {
			if err := os.WriteFile(item.path, []byte(item.data), 0o644); err != nil {
				t.Fatalf("os.WriteFile(%q) error = %v", item.path, err)
			}
		}
		if err := harness.CaptureBrowserScreenshots([]string{screenshotOne, screenshotTwo}); err != nil {
			t.Fatalf("CaptureBrowserScreenshots() error = %v", err)
		}
		if err := harness.CaptureBrowserConsoleJSON([]map[string]string{{"level": "error"}}); err != nil {
			t.Fatalf("CaptureBrowserConsoleJSON() error = %v", err)
		}
		if err := harness.CaptureBrowserNetworkJSON([]map[string]string{{"url": "/api/demo"}}); err != nil {
			t.Fatalf("CaptureBrowserNetworkJSON() error = %v", err)
		}
		if err := harness.StopSession(testContext(t), "sess-1"); err != nil {
			t.Fatalf("StopSession() error = %v", err)
		}

		manifest := harness.Artifacts.Manifest()
		if got, wantMin := len(manifest.Artifacts), 12; got < wantMin {
			t.Fatalf("len(manifest.Artifacts) = %d, want at least %d", got, wantMin)
		}

		manifestPath := harness.Artifacts.ManifestPath()
		manifestBytes, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", manifestPath, err)
		}
		for _, want := range []string{
			"automation_runs.json",
			"tasks.json",
			"task_runs.json",
		} {
			if !strings.Contains(string(manifestBytes), want) {
				t.Fatalf("manifest = %s, want %s entry", string(manifestBytes), want)
			}
		}

		automationRunsPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindAutomationRuns)
		if !ok {
			t.Fatal("ArtifactPath(automation_runs) = missing, want present")
		}
		automationRunsBytes, err := os.ReadFile(automationRunsPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", automationRunsPath, err)
		}
		if !strings.Contains(string(automationRunsBytes), `"session_id": "sess-1"`) {
			t.Fatalf("automation runs artifact = %s, want linked session id", string(automationRunsBytes))
		}

		tasksPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindTasks)
		if !ok {
			t.Fatal("ArtifactPath(tasks) = missing, want present")
		}
		tasksBytes, err := os.ReadFile(tasksPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", tasksPath, err)
		}
		if !strings.Contains(string(tasksBytes), `"ref": "run:run-1"`) {
			t.Fatalf("tasks artifact = %s, want automation origin linkage", string(tasksBytes))
		}

		taskRunsPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindTaskRuns)
		if !ok {
			t.Fatal("ArtifactPath(task_runs) = missing, want present")
		}
		taskRunsBytes, err := os.ReadFile(taskRunsPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", taskRunsPath, err)
		}
		if !strings.Contains(string(taskRunsBytes), `"session_id": "sess-1"`) ||
			!strings.Contains(string(taskRunsBytes), `"idempotency_key": "automation-run:run-1"`) {
			t.Fatalf("task runs artifact = %s, want session linkage and idempotency key", string(taskRunsBytes))
		}

		combinedFlowPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindCombinedFlow)
		if !ok {
			t.Fatal("ArtifactPath(combined_flow) = missing, want present")
		}
		combinedFlowBytes, err := os.ReadFile(combinedFlowPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", combinedFlowPath, err)
		}
		if !strings.Contains(string(combinedFlowBytes), `"scenario": "session-automation-task"`) ||
			!strings.Contains(string(combinedFlowBytes), `"task_id": "task-1"`) {
			t.Fatalf("combined flow artifact = %s, want cross-domain scenario summary", string(combinedFlowBytes))
		}

		toolHostDiagnosticsPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindToolHostDiagnostics)
		if !ok {
			t.Fatal("ArtifactPath(tool_host_diagnostics) = missing, want present")
		}
		toolHostDiagnosticsBytes, err := os.ReadFile(toolHostDiagnosticsPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", toolHostDiagnosticsPath, err)
		}
		if !strings.Contains(string(toolHostDiagnosticsBytes), `"operation": "write_text_file"`) {
			t.Fatalf(
				"tool host diagnostics artifact = %s, want tool-host operation diagnostics",
				string(toolHostDiagnosticsBytes),
			)
		}

		providerCallsPath, ok := harness.Artifacts.ArtifactPath(ArtifactKindProviderCalls)
		if !ok {
			t.Fatal("ArtifactPath(provider_calls) = missing, want present")
		}
		providerCallsBytes, err := os.ReadFile(providerCallsPath)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", providerCallsPath, err)
		}
		if !strings.Contains(string(providerCallsBytes), "provider call") {
			t.Fatalf("provider calls artifact = %s, want provider log capture", string(providerCallsBytes))
		}
	})
}

func TestRuntimeHarnessExtensionHelpersSurfaceTransportErrors(t *testing.T) {
	t.Parallel()
	t.Run("Should surface extension transport errors", func(t *testing.T) {
		t.Parallel()
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)
		harness := &RuntimeHarness{
			Artifacts: NewArtifactCollector(t), HTTPBaseURL: server.URL, HTTPClient: server.Client(),
			UDSBaseURL: server.URL, UDSClient: server.Client(),
		}
		_, err := harness.GetExtension(t.Context(), "tool-provider")
		assertErrorContains(t, err, "/api/extensions/tool-provider status 500: boom")
	})
}

type harnessTestServer struct {
	*httptest.Server
	handlerErrs chan error
}

func newHarnessTestServer(t testing.TB) *harnessTestServer {
	t.Helper()

	now := time.Date(2026, 4, 16, 12, 0, 0, 0, time.UTC)
	routeTime := now.Add(5 * time.Minute)
	handlerErrs := make(chan error, 32)
	mux := http.NewServeMux()

	mux.HandleFunc("/api/workspaces/resolve", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, compozycontract.WorkspaceResponse{
			Workspace: compozycontract.WorkspacePayload{
				ID:        "ws-1",
				RootDir:   "/workspace",
				Name:      "workspace",
				CreatedAt: now,
				UpdatedAt: now,
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, compozycontract.WorkspaceResponse{
			Workspace: compozycontract.WorkspacePayload{
				ID:        "ws-1",
				RootDir:   "/workspace",
				Name:      "workspace",
				CreatedAt: now,
				UpdatedAt: now,
			},
		})
	})
	mux.HandleFunc("/api/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, compozycontract.SessionResponse{
			Session: compozycontract.SessionPayload{
				ID:            "sess-1",
				AgentName:     "coder",
				WorkspaceID:   "ws-1",
				WorkspacePath: "/workspace",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/sessions/sess-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, compozycontract.SessionResponse{
			Session: compozycontract.SessionPayload{
				ID:            "sess-1",
				AgentName:     "coder",
				WorkspaceID:   "ws-1",
				WorkspacePath: "/workspace",
				State:         "stopped",
				StopReason:    store.StopCompleted,

				CreatedAt: now,
				UpdatedAt: now,
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/sessions/sess-1/stop", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/workspaces/ws-1/sessions/sess-1/attach", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, compozycontract.SessionAttachResponse{
			Session: compozycontract.SessionPayload{
				ID:            "sess-1",
				AgentName:     "coder",
				WorkspaceID:   "ws-1",
				WorkspacePath: "/workspace",

				State: "active",
				Badge: "idle",

				CreatedAt: now,
				UpdatedAt: routeTime,
			},
			Attach: compozycontract.SessionAttachPayload{
				SessionID:       "sess-1",
				AttachedTo:      "e2e:test",
				AttachExpiresAt: routeTime.Add(time.Minute),
				AttachedAt:      routeTime,
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/sessions/sess-1/transcript", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"messages": []map[string]any{
				{"role": "user", "content": "hello world"},
				{"role": "assistant", "content": "echo: hello world"},
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/sessions/sess-1/events", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{
			"events": []map[string]any{
				{"id": "evt-1", "type": "agent_message"},
				{"id": "evt-2", "type": "session_stopped"},
			},
		})
	})
	mux.HandleFunc("/api/workspaces/ws-1/sessions/sess-1/prompt", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := fmt.Fprint(
			w,
			"event: agent_message\n"+
				"data: {\"type\":\"text-delta\"}\n\n"+
				"event: done\n"+
				"data: [DONE]\n\n",
		); err != nil {
			reportHarnessHandlerError(w, handlerErrs, http.StatusInternalServerError, "write prompt stream: %v", err)
		}
	})

	mux.HandleFunc("/api/automation/runs", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("status"), "completed"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"automation runs query status = %q, want %q",
				got,
				want,
			)
			return
		}
		writeJSON(w, compozycontract.RunsResponse{
			Runs: []compozycontract.RunPayload{{
				ID:        "run-1",
				SessionID: "sess-1",
				Status:    "completed",
				Attempt:   1,
			}},
		})
	})
	mux.HandleFunc("/api/tasks", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("limit"), "10"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"tasks query limit = %q, want %q",
				got,
				want,
			)
			return
		}
		writeJSON(w, compozycontract.TasksResponse{
			Tasks: []compozycontract.TaskCatalogItemPayload{{
				ID:        "task-1",
				Scope:     "workspace",
				Title:     "demo",
				Status:    "in_progress",
				CreatedBy: taskpkg.ActorIdentity{Kind: taskpkg.ActorKindAutomation, Ref: "job-1"},
				Origin:    taskpkg.Origin{Kind: taskpkg.OriginKindAutomation, Ref: "run:run-1"},
				CreatedAt: now,
				UpdatedAt: now,
			}},
		})
	})
	mux.HandleFunc("/api/tasks/task-1/runs", func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.Query().Get("status"), "completed"; got != want {
			reportHarnessHandlerError(
				w,
				handlerErrs,
				http.StatusBadRequest,
				"task runs query status = %q, want %q",
				got,
				want,
			)
			return
		}
		writeJSON(w, compozycontract.TaskRunsResponse{
			Runs: []compozycontract.TaskRunPayload{{
				ID:             "task-run-1",
				TaskID:         "task-1",
				Status:         taskpkg.TaskRunStatusCompleted,
				Attempt:        1,
				SessionID:      "sess-1",
				Origin:         taskpkg.Origin{Kind: taskpkg.OriginKindAutomation, Ref: "run:run-1"},
				IdempotencyKey: "automation-run:run-1",
				QueuedAt:       now,
			}},
		})
	})
	mux.HandleFunc("/api/extensions", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, compozycontract.ExtensionsResponse{
				Extensions: []compozycontract.ExtensionPayload{{
					Name:          "tool-provider",
					Version:       "0.1.0",
					Type:          "local",
					Source:        "user",
					Enabled:       true,
					State:         "registered",
					Capabilities:  []string{"tool.provider"},
					Permissions:   []string{},
					Health:        "healthy",
					DaemonRunning: true,
				}},
			})
		case http.MethodPost:
			var request compozycontract.InstallExtensionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				reportHarnessHandlerError(
					w,
					handlerErrs,
					http.StatusBadRequest,
					"json.Decode(install extension) error = %v",
					err,
				)
				return
			}
			if got, want := request.Ref, "/extensions/tool-provider"; got != want {
				reportHarnessHandlerError(
					w,
					handlerErrs,
					http.StatusBadRequest,
					"install extension ref = %q, want %q",
					got,
					want,
				)
				return
			}
			if got, want := request.Source, compozycontract.InstallExtensionSourceLocalPath; got != want {
				reportHarnessHandlerError(
					w,
					handlerErrs,
					http.StatusBadRequest,
					"install extension source = %q, want %q",
					got,
					want,
				)
				return
			}
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, compozycontract.ExtensionResponse{
				Extension: compozycontract.ExtensionPayload{
					Name:          "tool-provider",
					Version:       "0.1.0",
					Type:          "local",
					Source:        "user",
					Enabled:       true,
					State:         "registered",
					Capabilities:  []string{"tool.provider"},
					Permissions:   []string{},
					Health:        "healthy",
					DaemonRunning: true,
				},
			})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/extensions/tool-provider", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, compozycontract.ExtensionResponse{
			Extension: compozycontract.ExtensionPayload{
				Name:          "tool-provider",
				Version:       "0.1.0",
				Type:          "local",
				Source:        "user",
				Enabled:       true,
				State:         "registered",
				Capabilities:  []string{"tool.provider"},
				Permissions:   []string{},
				Health:        "healthy",
				DaemonRunning: true,
			},
		})
	})
	mux.HandleFunc("/api/extensions/tool-provider/enablement", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var request compozycontract.SetExtensionEnablementRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			reportHarnessHandlerError(w, handlerErrs, http.StatusBadRequest, "decode enablement: %v", err)
			return
		}
		writeJSON(w, compozycontract.ExtensionEnablementPayload(request))
	})

	server := &harnessTestServer{
		Server:      httptest.NewServer(mux),
		handlerErrs: handlerErrs,
	}
	t.Cleanup(func() {
		server.assertNoHandlerErrors(t)
	})
	t.Cleanup(server.Close)
	return server
}

func writeJSON(w http.ResponseWriter, value any) {
	if err := writeJSONResponse(w, value); err != nil {
		panic(err)
	}
}

func writeJSONResponse(w http.ResponseWriter, value any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(value)
}

func testContext(t testing.TB) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func (s *harnessTestServer) assertNoHandlerErrors(t testing.TB) {
	t.Helper()

	close(s.handlerErrs)

	messages := make([]string, 0)
	for err := range s.handlerErrs {
		messages = append(messages, err.Error())
	}
	if len(messages) > 0 {
		t.Fatalf("handler validation errors:\n%s", strings.Join(messages, "\n"))
	}
}

func reportHarnessHandlerError(
	w http.ResponseWriter,
	errCh chan<- error,
	status int,
	format string,
	args ...any,
) {
	err := fmt.Errorf(format, args...)
	errCh <- err
	http.Error(w, err.Error(), status)
}

func assertErrorContains(t testing.TB, err error, want string) {
	t.Helper()

	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want substring %q", err, want)
	}
}
