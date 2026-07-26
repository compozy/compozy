//go:build integration

package udsapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/api/core"
	apitestutil "github.com/compozy/agh/internal/api/testutil"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/resources"
	sandboxlocal "github.com/compozy/agh/internal/sandbox/local"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	toolspkg "github.com/compozy/agh/internal/tools"
	"github.com/compozy/agh/internal/transcript"
	"github.com/compozy/agh/internal/windowmanager"
	workspacepkg "github.com/compozy/agh/internal/workspace"
	"github.com/gorilla/websocket"
)

func TestUDSFullRoundTripWithRealSessionManager(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	statusResp := mustUnixRequest(t, runtime.client, http.MethodGet, "http://unix/api/status", nil, nil)
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("daemon status = %d, want %d", statusResp.StatusCode, http.StatusOK)
	}
	_ = statusResp.Body.Close()

	createResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/sessions",
		[]byte(`{"agent_name":"coder","name":"demo","workspace_path":"`+runtime.workspace+`"}`),
		nil,
	)
	if createResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResp.Body)
		_ = createResp.Body.Close()
		t.Fatalf(
			"create session status = %d, want %d; body=%s",
			createResp.StatusCode,
			http.StatusCreated,
			string(body),
		)
	}
	var created struct {
		Session sessionPayload `json:"session"`
	}
	decodeHTTPJSON(t, createResp, &created)
	if created.Session.ID == "" {
		t.Fatal("expected created session id")
	}
	if created.Session.WorkspaceID == "" {
		t.Fatal("expected created session workspace id")
	}

	listResp := mustUnixRequest(t, runtime.client, http.MethodGet, "http://unix/api/sessions", nil, nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list sessions status = %d, want %d", listResp.StatusCode, http.StatusOK)
	}
	var listed struct {
		Sessions []sessionPayload `json:"sessions"`
	}
	decodeHTTPJSON(t, listResp, &listed)
	if len(listed.Sessions) != 1 || listed.Sessions[0].ID != created.Session.ID {
		t.Fatalf("listed sessions = %#v", listed.Sessions)
	}
	tasksAfterManualSession, err := runtime.registry.ListTasks(context.Background(), taskpkg.Query{Limit: 10})
	if err != nil {
		t.Fatalf("ListTasks(after manual session) error = %v", err)
	}
	if len(tasksAfterManualSession) != 0 {
		t.Fatalf("tasks after manual session = %#v, want none", tasksAfterManualSession)
	}
	runsAfterManualSession, err := runtime.registry.ListTaskRunsByStatus(
		context.Background(),
		[]taskpkg.RunStatus{
			taskpkg.TaskRunStatusQueued,
			taskpkg.TaskRunStatusClaimed,
			taskpkg.TaskRunStatusStarting,
			taskpkg.TaskRunStatusRunning,
		},
	)
	if err != nil {
		t.Fatalf("ListTaskRunsByStatus(after manual session) error = %v", err)
	}
	if len(runsAfterManualSession) != 0 {
		t.Fatalf("open task runs after manual session = %#v, want none", runsAfterManualSession)
	}

	promptResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		sessionAPIPath(created.Session.WorkspaceID, created.Session.ID, "/prompt"),
		[]byte(`{"message":"hello"}`),
		nil,
	)
	if promptResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(promptResp.Body)
		_ = promptResp.Body.Close()
		t.Fatalf("prompt status = %d, want %d; body=%s", promptResp.StatusCode, http.StatusOK, string(body))
	}
	if got := promptResp.Header.Get("x-vercel-ai-ui-message-stream"); got != "v1" {
		t.Fatalf("x-vercel-ai-ui-message-stream = %q, want v1", got)
	}
	promptBody, err := io.ReadAll(promptResp.Body)
	_ = promptResp.Body.Close()
	if err != nil {
		t.Fatalf("io.ReadAll(prompt SSE) error = %v", err)
	}
	promptEvents := parseSSE(t, string(promptBody))
	if len(promptEvents) < 5 {
		t.Fatalf("prompt SSE events = %d, want at least 5; body=%s", len(promptEvents), string(promptBody))
	}
	if string(promptEvents[len(promptEvents)-1].Data) != "[DONE]" {
		t.Fatalf("last prompt SSE data = %q, want [DONE]", string(promptEvents[len(promptEvents)-1].Data))
	}

	partTypes := make([]string, 0, len(promptEvents))
	for _, record := range promptEvents[:len(promptEvents)-1] {
		if len(record.Data) == 0 || string(record.Data) == "[DONE]" {
			continue
		}
		var payload struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(record.Data, &payload); err != nil {
			t.Fatalf("json.Unmarshal(prompt part) error = %v; data=%s", err, string(record.Data))
		}
		partTypes = append(partTypes, payload.Type)
	}
	hasType := func(target string) bool {
		for _, value := range partTypes {
			if value == target {
				return true
			}
		}
		return false
	}
	if !hasType("start") || !hasType("text-start") || !hasType("text-delta") ||
		!hasType("text-end") || !hasType("finish") {
		t.Fatalf("prompt SSE part types = %#v", partTypes)
	}

	eventsResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		sessionAPIPath(created.Session.WorkspaceID, created.Session.ID, "/events"),
		nil,
		nil,
	)
	if eventsResp.StatusCode != http.StatusOK {
		t.Fatalf("session events status = %d, want %d", eventsResp.StatusCode, http.StatusOK)
	}
	var events struct {
		Events []sessionEventPayload `json:"events"`
	}
	decodeHTTPJSON(t, eventsResp, &events)
	if len(events.Events) < 2 {
		t.Fatalf("persisted session events = %d, want at least 2", len(events.Events))
	}

	stopResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodDelete,
		sessionAPIPath(created.Session.WorkspaceID, created.Session.ID, ""),
		nil,
		nil,
	)
	if stopResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(stopResp.Body)
		_ = stopResp.Body.Close()
		t.Fatalf("stop session status = %d, want %d; body=%s", stopResp.StatusCode, http.StatusNoContent, string(body))
	}
	_ = stopResp.Body.Close()
}

func TestUDSWindowManagerWebSocketStream(t *testing.T) {
	t.Run("Should stream one client fence and drain over the Unix socket", func(t *testing.T) {
		t.Parallel()
		runtime := newIntegrationRuntime(t)
		const workspaceID = "window-manager-workspace"
		streamURL := "ws://unix/api/workspaces/" + workspaceID + "/window-manager/stream"
		dialer := websocket.Dialer{
			NetDialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var netDialer net.Dialer
				return netDialer.DialContext(ctx, "unix", runtime.socket)
			},
		}

		missingDialContext, cancelMissingDial := context.WithTimeout(t.Context(), time.Second)
		missingConnection, response, err := dialer.DialContext(
			missingDialContext,
			streamURL+"?client_id=missing",
			nil,
		)
		cancelMissingDial()
		if err != nil {
			if response != nil && response.Body != nil {
				closeHTTPBody(t, response.Body)
			}
			t.Fatalf("DialContext(missing client) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := missingConnection.Close(); closeErr != nil {
				t.Errorf("missing-client websocket Close() error = %v", closeErr)
			}
		})
		if deadlineErr := missingConnection.SetReadDeadline(time.Now().Add(time.Second)); deadlineErr != nil {
			t.Fatalf("SetReadDeadline(missing-client frame) error = %v", deadlineErr)
		}
		var terminal contract.WindowManagerErrorFrame
		if readErr := missingConnection.ReadJSON(&terminal); readErr != nil {
			t.Fatalf("ReadJSON(missing-client error) error = %v", readErr)
		}
		if terminal.Type != contract.WindowManagerFrameError ||
			terminal.Error.Code != contract.WindowManagerErrorClientNotFound ||
			terminal.Error.WorkspaceID != workspaceID {
			t.Fatalf("missing-client terminal frame = %+v", terminal)
		}
		if deadlineErr := missingConnection.SetReadDeadline(time.Now().Add(time.Second)); deadlineErr != nil {
			t.Fatalf("SetReadDeadline(missing client) error = %v", deadlineErr)
		}
		_, _, readErr := missingConnection.ReadMessage()
		if !websocket.IsCloseError(readErr, websocket.ClosePolicyViolation) {
			t.Fatalf("ReadMessage(missing client) error = %v, want policy-violation close", readErr)
		}

		registerResponse := mustUnixRequest(
			t,
			runtime.client,
			http.MethodPost,
			"http://unix/api/workspaces/"+workspaceID+"/window-manager/clients",
			[]byte(`{"workspace_id":"`+workspaceID+`","client_id":"client-a"}`),
			nil,
		)
		if registerResponse.StatusCode != http.StatusCreated {
			body := readAndCloseHTTPBody(t, registerResponse)
			t.Fatalf(
				"register client status = %d, want %d; body=%s",
				registerResponse.StatusCode,
				http.StatusCreated,
				string(body),
			)
		}
		var registered contract.WindowManagerClientView
		decodeHTTPJSON(t, registerResponse, &registered)
		if registered.ClientID != "client-a" || registered.PresentationRevision != 1 {
			t.Fatalf("registered client = %+v", registered)
		}

		registeredDialContext, cancelRegisteredDial := context.WithTimeout(t.Context(), time.Second)
		connection, response, err := dialer.DialContext(
			registeredDialContext,
			streamURL+"?client_id=client-a",
			nil,
		)
		cancelRegisteredDial()
		if err != nil {
			if response != nil && response.Body != nil {
				closeHTTPBody(t, response.Body)
			}
			t.Fatalf("DialContext(registered client) error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := connection.Close(); closeErr != nil {
				t.Errorf("registered-client websocket Close() error = %v", closeErr)
			}
		})
		if deadlineErr := connection.SetReadDeadline(time.Now().Add(time.Second)); deadlineErr != nil {
			t.Fatalf("SetReadDeadline(client fence) error = %v", deadlineErr)
		}
		var fence contract.WindowManagerSnapshotFrame
		if readErr := connection.ReadJSON(&fence); readErr != nil {
			t.Fatalf("ReadJSON(client fence) error = %v", readErr)
		}
		if fence.Type != contract.WindowManagerFrameSnapshot ||
			fence.WorkspaceID != workspaceID ||
			fence.Client == nil ||
			fence.Client.ClientID != "client-a" ||
			fence.Client.PresentationRevision != 1 {
			t.Fatalf("client-bound fence = %+v", fence)
		}

		shutdownContext, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), 2*time.Second)
		defer cancel()
		if shutdownErr := runtime.server.Shutdown(shutdownContext); shutdownErr != nil {
			t.Fatalf("server.Shutdown() error = %v", shutdownErr)
		}
		if deadlineErr := connection.SetReadDeadline(time.Now().Add(time.Second)); deadlineErr != nil {
			t.Fatalf("SetReadDeadline(shutdown) error = %v", deadlineErr)
		}
		_, _, readErr = connection.ReadMessage()
		if !websocket.IsCloseError(readErr, websocket.CloseGoingAway) {
			t.Fatalf("ReadMessage(shutdown) error = %v, want going-away close", readErr)
		}
	})
}

func TestUDSSessionTranscriptEndpointIncludesSyntheticTurns(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	created := createLiveIntegrationSessionPayload(t, runtime)
	sessionID := created.ID

	const promptTimeout = 5 * time.Second

	userCtx, cancelUser := context.WithTimeout(context.Background(), promptTimeout)
	userEvents, userErr := runtime.manager.Prompt(userCtx, sessionID, "hello")
	collectIntegrationPromptEvents(t, mustIntegrationPrompt(t, userEvents, userErr), promptTimeout)
	cancelUser()

	networkCtx, cancelNetwork := context.WithTimeout(context.Background(), promptTimeout)
	networkEvents, networkErr := runtime.manager.PromptNetwork(networkCtx, sessionID, "network hello")
	collectIntegrationPromptEvents(t, mustIntegrationPrompt(t, networkEvents, networkErr), promptTimeout)
	cancelNetwork()

	syntheticCtx, cancelSynthetic := context.WithTimeout(context.Background(), promptTimeout)
	syntheticEvents, syntheticErr := runtime.manager.PromptSynthetic(
		syntheticCtx,
		sessionID,
		session.SyntheticPromptOpts{
			Message: "daemon wake-up",
			Metadata: acp.PromptSyntheticMeta{
				TaskRunID: "run-1",
				Reason:    "task_run_completed",
				Summary:   "background work finished",
			},
		},
	)
	collectIntegrationPromptEvents(t, mustIntegrationPrompt(t, syntheticEvents, syntheticErr), promptTimeout)
	cancelSynthetic()

	resp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		sessionAPIPath(created.WorkspaceID, sessionID, "/transcript"),
		nil,
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("transcript status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, string(body))
	}

	var payload contract.SessionTranscriptResponse
	decodeHTTPJSON(t, resp, &payload)
	messages := transcript.MessagesFromEntries(payload.Entries)
	if len(messages) != 6 {
		t.Fatalf("len(messages) = %d, want 6", len(messages))
	}
	if got := messages[0].Role; got != transcript.UIRoleUser {
		t.Fatalf("messages[0].Role = %q, want %q", got, transcript.UIRoleUser)
	}
	if got := transcript.UIMessageText(messages[0]); got != "hello" {
		t.Fatalf("messages[0] text = %q, want %q", got, "hello")
	}
	if got := messages[1].Role; got != transcript.UIRoleAssistant {
		t.Fatalf("messages[1].Role = %q, want %q", got, transcript.UIRoleAssistant)
	}
	if got := messages[2].Role; got != transcript.UIRoleUser {
		t.Fatalf("messages[2].Role = %q, want %q", got, transcript.UIRoleUser)
	}
	if got := transcript.UIMessageText(messages[2]); got != "network hello" {
		t.Fatalf("messages[2] text = %q, want %q", got, "network hello")
	}
	if got := messages[3].Role; got != transcript.UIRoleAssistant {
		t.Fatalf("messages[3].Role = %q, want %q", got, transcript.UIRoleAssistant)
	}
	if got := messages[4].Role; got != transcript.UIRoleSystem {
		t.Fatalf("messages[4].Role = %q, want %q", got, transcript.UIRoleSystem)
	}
	if got := transcript.UIMessageText(messages[4]); got != "daemon wake-up" {
		t.Fatalf("messages[4] text = %q, want %q", got, "daemon wake-up")
	}
	if got := messages[5].Role; got != transcript.UIRoleAssistant {
		t.Fatalf("messages[5].Role = %q, want %q", got, transcript.UIRoleAssistant)
	}
}

func TestUDSMemoryRoundTripAndConsolidate(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	writeResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/memory",
		[]byte(
			`{"scope":"global","type":"user","name":"Integration","description":"desc","content":"hello integration"}`,
		),
		nil,
	)
	if writeResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(writeResp.Body)
		_ = writeResp.Body.Close()
		t.Fatalf("write status = %d, want %d; body=%s", writeResp.StatusCode, http.StatusOK, string(body))
	}
	var writePayload memoryMutationDecisionResponse
	decodeHTTPJSON(t, writeResp, &writePayload)
	targetFilename := writePayload.Decision.TargetFilename
	if targetFilename == "" {
		t.Fatalf("write payload = %#v, want target filename", writePayload)
	}

	readResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/memory/"+targetFilename+"?scope=global",
		nil,
		nil,
	)
	if readResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(readResp.Body)
		_ = readResp.Body.Close()
		t.Fatalf("read status = %d, want %d; body=%s", readResp.StatusCode, http.StatusOK, string(body))
	}
	var readPayload memoryEntryResponse
	decodeHTTPJSON(t, readResp, &readPayload)
	if !strings.Contains(readPayload.Memory.Content, "hello integration") {
		t.Fatalf("content = %q, want written body", readPayload.Memory.Content)
	}

	deleteResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodDelete,
		"http://unix/api/memory/"+targetFilename+"?scope=global",
		nil,
		nil,
	)
	if deleteResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(deleteResp.Body)
		_ = deleteResp.Body.Close()
		t.Fatalf("delete status = %d, want %d; body=%s", deleteResp.StatusCode, http.StatusOK, string(body))
	}
	_ = deleteResp.Body.Close()

	resp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/memory/dreams/trigger",
		[]byte(`{"workspace_id":"`+runtime.workspace+`"}`),
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("dream trigger status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, string(body))
	}

	var payload memoryDreamTriggerResponse
	decodeHTTPJSON(t, resp, &payload)
	if !payload.Triggered || runtime.dream.calls != 1 {
		t.Fatalf("payload = %#v dream.calls=%d, want triggered once", payload, runtime.dream.calls)
	}
}

func TestUDSResourceCRUDRoundTrip(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	createResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPut,
		"http://unix/api/resources/integration.fixture/demo",
		[]byte(`{"scope":{"kind":"global"},"spec":{"enabled":true}}`),
		nil,
	)
	if createResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResp.Body)
		_ = createResp.Body.Close()
		t.Fatalf(
			"create resource status = %d, want %d; body=%s",
			createResp.StatusCode,
			http.StatusCreated,
			string(body),
		)
	}
	var created contract.ResourceResponse
	decodeHTTPJSON(t, createResp, &created)
	if created.Record.Version != 1 {
		t.Fatalf("created version = %d, want 1", created.Record.Version)
	}
	if strings.TrimSpace(string(created.Record.Spec)) != `{"enabled":true}` {
		t.Fatalf("created spec = %s, want enabled true", string(created.Record.Spec))
	}

	updateResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPut,
		"http://unix/api/resources/integration.fixture/demo",
		[]byte(
			fmt.Sprintf(
				`{"scope":{"kind":"global"},"expected_version":%d,"spec":{"enabled":false}}`,
				created.Record.Version,
			),
		),
		nil,
	)
	if updateResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(updateResp.Body)
		_ = updateResp.Body.Close()
		t.Fatalf("update resource status = %d, want %d; body=%s", updateResp.StatusCode, http.StatusOK, string(body))
	}
	var updated contract.ResourceResponse
	decodeHTTPJSON(t, updateResp, &updated)
	if updated.Record.Version != 2 {
		t.Fatalf("updated version = %d, want 2", updated.Record.Version)
	}
	if strings.TrimSpace(string(updated.Record.Spec)) != `{"enabled":false}` {
		t.Fatalf("updated spec = %s, want enabled false", string(updated.Record.Spec))
	}

	getResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/resources/integration.fixture/demo",
		nil,
		nil,
	)
	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		_ = getResp.Body.Close()
		t.Fatalf("get resource status = %d, want %d; body=%s", getResp.StatusCode, http.StatusOK, string(body))
	}
	var fetched contract.ResourceResponse
	decodeHTTPJSON(t, getResp, &fetched)
	if fetched.Record.Version != updated.Record.Version {
		t.Fatalf("fetched version = %d, want %d", fetched.Record.Version, updated.Record.Version)
	}

	listResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/resources/integration.fixture?scope_kind=global",
		nil,
		nil,
	)
	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		_ = listResp.Body.Close()
		t.Fatalf("list resources status = %d, want %d; body=%s", listResp.StatusCode, http.StatusOK, string(body))
	}
	var listed contract.ResourcesResponse
	decodeHTTPJSON(t, listResp, &listed)
	if len(listed.Records) != 1 || listed.Records[0].ID != "demo" ||
		listed.Records[0].Version != updated.Record.Version {
		t.Fatalf("listed records = %#v, want updated demo record", listed.Records)
	}
}

func TestUDSToolResourceCRUDRoundTripTriggersProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should create update tool resource and trigger projection", func(t *testing.T) {
		t.Parallel()

		runtime := newIntegrationRuntime(t)

		createResp := mustUnixRequest(
			t,
			runtime.client,
			http.MethodPut,
			"http://unix/api/resources/tool/lookup",
			[]byte(`{
				"scope":{"kind":"global"},
				"spec":{
					"id":" dyn__lookup ",
					"backend":{"kind":"native_go","native_name":" lookup "},
					"description":" search workspace ",
					"input_schema":{"type":"object"},
					"read_only":true,
					"source":{"kind":"dynamic","owner":" udsapi "},
					"visibility":"operator",
					"risk":"read"
				}
			}`),
			nil,
		)
		if createResp.StatusCode != http.StatusCreated {
			body, err := io.ReadAll(createResp.Body)
			if err != nil {
				t.Fatalf("io.ReadAll(create tool resource body) error = %v", err)
			}
			if err := createResp.Body.Close(); err != nil {
				t.Fatalf("create tool resource body close error = %v", err)
			}
			t.Fatalf(
				"create tool resource status = %d, want %d; body=%s",
				createResp.StatusCode,
				http.StatusCreated,
				string(body),
			)
		}
		var created contract.ResourceResponse
		decodeHTTPJSON(t, createResp, &created)
		var createdSpec map[string]any
		if err := json.Unmarshal(created.Record.Spec, &createdSpec); err != nil {
			t.Fatalf("json.Unmarshal(created tool spec) error = %v", err)
		}
		backend, ok := createdSpec["backend"].(map[string]any)
		if !ok {
			t.Fatalf("created tool backend = %#v, want object", createdSpec["backend"])
		}
		source, ok := createdSpec["source"].(map[string]any)
		if !ok {
			t.Fatalf("created tool source = %#v, want object", createdSpec["source"])
		}
		inputSchema, ok := createdSpec["input_schema"].(map[string]any)
		if !ok {
			t.Fatalf("created tool input_schema = %#v, want object", createdSpec["input_schema"])
		}
		if createdSpec["id"] != "dyn__lookup" ||
			createdSpec["description"] != "search workspace" ||
			createdSpec["visibility"] != "operator" ||
			createdSpec["risk"] != "read" ||
			createdSpec["read_only"] != true ||
			inputSchema["type"] != "object" ||
			backend["kind"] != "native_go" ||
			backend["native_name"] != "lookup" ||
			source["kind"] != "dynamic" ||
			source["owner"] != "udsapi" {
			t.Fatalf("created tool spec = %#v, want normalized fields", createdSpec)
		}

		waitForProjectedToolRevision(t, runtime, 1)
		revision, records := runtime.toolCatalog.snapshot()
		if revision != 1 || len(records) != 1 || records[0].Spec.ID != toolspkg.ToolID("dyn__lookup") {
			t.Fatalf("tool projection after create = revision:%d records:%#v, want lookup@1", revision, records)
		}

		updateResp := mustUnixRequest(
			t,
			runtime.client,
			http.MethodPut,
			"http://unix/api/resources/tool/lookup",
			[]byte(fmt.Sprintf(`{
				"scope":{"kind":"global"},
				"expected_version":%d,
				"spec":{
					"id":"dyn__lookup",
					"backend":{"kind":"native_go","native_name":"lookup"},
					"description":"search workspace v2",
					"input_schema":{"type":"object"},
					"read_only":true,
					"source":{"kind":"dynamic","owner":"udsapi"},
					"visibility":"operator",
					"risk":"read"
				}
			}`, created.Record.Version)),
			nil,
		)
		if updateResp.StatusCode != http.StatusOK {
			body, err := io.ReadAll(updateResp.Body)
			if err != nil {
				t.Fatalf("io.ReadAll(update tool resource body) error = %v", err)
			}
			if err := updateResp.Body.Close(); err != nil {
				t.Fatalf("update tool resource body close error = %v", err)
			}
			t.Fatalf(
				"update tool resource status = %d, want %d; body=%s",
				updateResp.StatusCode,
				http.StatusOK,
				string(body),
			)
		}
		var updated contract.ResourceResponse
		decodeHTTPJSON(t, updateResp, &updated)
		waitForProjectedToolRevision(t, runtime, 2)

		_, records = runtime.toolCatalog.snapshot()
		if got, want := len(records), 1; got != want {
			t.Fatalf("projected tool count after update = %d, want %d", got, want)
		}
		if got, want := records[0].Spec.ID, toolspkg.ToolID("dyn__lookup"); got != want {
			t.Fatalf("projected tool ID after update = %q, want %q", got, want)
		}
		if got, want := records[0].Spec.Description, "search workspace v2"; got != want {
			t.Fatalf("projected tool description = %q, want %q", got, want)
		}
	})
}

func TestUDSDeleteResourceRejectsStaleVersionAndRequiresCurrentVersion(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	createResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPut,
		"http://unix/api/resources/integration.fixture/demo",
		[]byte(`{"scope":{"kind":"global"},"spec":{"enabled":true}}`),
		nil,
	)
	if createResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResp.Body)
		_ = createResp.Body.Close()
		t.Fatalf(
			"create resource status = %d, want %d; body=%s",
			createResp.StatusCode,
			http.StatusCreated,
			string(body),
		)
	}
	var created contract.ResourceResponse
	decodeHTTPJSON(t, createResp, &created)

	updateResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPut,
		"http://unix/api/resources/integration.fixture/demo",
		[]byte(
			fmt.Sprintf(
				`{"scope":{"kind":"global"},"expected_version":%d,"spec":{"enabled":false}}`,
				created.Record.Version,
			),
		),
		nil,
	)
	if updateResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(updateResp.Body)
		_ = updateResp.Body.Close()
		t.Fatalf("update resource status = %d, want %d; body=%s", updateResp.StatusCode, http.StatusOK, string(body))
	}
	var updated contract.ResourceResponse
	decodeHTTPJSON(t, updateResp, &updated)

	staleDelete := mustUnixRequest(
		t,
		runtime.client,
		http.MethodDelete,
		"http://unix/api/resources/integration.fixture/demo",
		[]byte(fmt.Sprintf(`{"expected_version":%d}`, created.Record.Version)),
		nil,
	)
	if staleDelete.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(staleDelete.Body)
		_ = staleDelete.Body.Close()
		t.Fatalf(
			"stale delete status = %d, want %d; body=%s",
			staleDelete.StatusCode,
			http.StatusConflict,
			string(body),
		)
	}
	_ = staleDelete.Body.Close()

	deleteResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodDelete,
		"http://unix/api/resources/integration.fixture/demo",
		[]byte(fmt.Sprintf(`{"expected_version":%d}`, updated.Record.Version)),
		nil,
	)
	if deleteResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(deleteResp.Body)
		_ = deleteResp.Body.Close()
		t.Fatalf(
			"delete resource status = %d, want %d; body=%s",
			deleteResp.StatusCode,
			http.StatusNoContent,
			string(body),
		)
	}
	_ = deleteResp.Body.Close()

	getResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/resources/integration.fixture/demo",
		nil,
		nil,
	)
	if getResp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(getResp.Body)
		_ = getResp.Body.Close()
		t.Fatalf(
			"get deleted resource status = %d, want %d; body=%s",
			getResp.StatusCode,
			http.StatusNotFound,
			string(body),
		)
	}
}

func TestUDSAutomationJobsRoundTrip(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	createResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/automation/jobs",
		[]byte(
			`{"scope":"global","name":"nightly-review","agent_name":"coder","prompt":"review repo","schedule":{"mode":"every","interval":"1h"}}`,
		),
		nil,
	)
	if createResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResp.Body)
		_ = createResp.Body.Close()
		t.Fatalf("create job status = %d, want %d; body=%s", createResp.StatusCode, http.StatusCreated, string(body))
	}
	var created contract.JobResponse
	decodeHTTPJSON(t, createResp, &created)
	if created.Job.ID == "" {
		t.Fatal("expected created automation job id")
	}

	updateResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPatch,
		"http://unix/api/automation/jobs/"+created.Job.ID,
		[]byte(`{"prompt":"review repo now"}`),
		nil,
	)
	if updateResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(updateResp.Body)
		_ = updateResp.Body.Close()
		t.Fatalf("update job status = %d, want %d; body=%s", updateResp.StatusCode, http.StatusOK, string(body))
	}
	var updated contract.JobResponse
	decodeHTTPJSON(t, updateResp, &updated)
	if updated.Job.Prompt != "review repo now" {
		t.Fatalf("updated job prompt = %q, want %q", updated.Job.Prompt, "review repo now")
	}

	triggerResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/automation/jobs/"+created.Job.ID+"/trigger",
		nil,
		nil,
	)
	if triggerResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(triggerResp.Body)
		_ = triggerResp.Body.Close()
		t.Fatalf("trigger job status = %d, want %d; body=%s", triggerResp.StatusCode, http.StatusOK, string(body))
	}
	var run contract.RunResponse
	decodeHTTPJSON(t, triggerResp, &run)
	if run.Run.JobID != created.Job.ID {
		t.Fatalf("job run = %#v, want job_id %q", run.Run, created.Job.ID)
	}

	runsResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/automation/jobs/"+created.Job.ID+"/runs",
		nil,
		nil,
	)
	if runsResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(runsResp.Body)
		_ = runsResp.Body.Close()
		t.Fatalf("job runs status = %d, want %d; body=%s", runsResp.StatusCode, http.StatusOK, string(body))
	}
	var runs contract.RunsResponse
	decodeHTTPJSON(t, runsResp, &runs)
	if !containsAutomationRun(runs.Runs, run.Run.ID) {
		t.Fatalf("job runs missing %q: %#v", run.Run.ID, runs.Runs)
	}

	deleteResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodDelete,
		"http://unix/api/automation/jobs/"+created.Job.ID,
		nil,
		nil,
	)
	if deleteResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(deleteResp.Body)
		_ = deleteResp.Body.Close()
		t.Fatalf("delete job status = %d, want %d; body=%s", deleteResp.StatusCode, http.StatusNoContent, string(body))
	}
	_ = deleteResp.Body.Close()
}

func TestUDSAutomationResourceWritesProjectJobsAndTriggers(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	jobID := "resource-job"
	createJobResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPut,
		"http://unix/api/resources/automation.job/"+jobID,
		[]byte(
			`{"scope":{"kind":"global"},"spec":{"scope":"global","name":"resource-job","agent_name":"coder","prompt":"review from resource","schedule":{"mode":"every","interval":"1h"},"enabled":true,"source":"dynamic"}}`,
		),
		nil,
	)
	if createJobResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createJobResp.Body)
		_ = createJobResp.Body.Close()
		t.Fatalf(
			"create automation.job resource status = %d, want %d; body=%s",
			createJobResp.StatusCode,
			http.StatusCreated,
			string(body),
		)
	}
	var createdJobResource contract.ResourceResponse
	decodeHTTPJSON(t, createJobResp, &createdJobResource)
	projectedJob := waitForAutomationJobPrompt(t, runtime, jobID, "review from resource")
	if projectedJob.Source != automationpkg.JobSourceDynamic {
		t.Fatalf("projected job source = %q, want %q", projectedJob.Source, automationpkg.JobSourceDynamic)
	}

	triggerRunResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/automation/jobs/"+jobID+"/trigger",
		nil,
		nil,
	)
	if triggerRunResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(triggerRunResp.Body)
		_ = triggerRunResp.Body.Close()
		t.Fatalf(
			"trigger resource job status = %d, want %d; body=%s",
			triggerRunResp.StatusCode,
			http.StatusOK,
			string(body),
		)
	}
	var jobRun contract.RunResponse
	decodeHTTPJSON(t, triggerRunResp, &jobRun)
	if jobRun.Run.JobID != jobID {
		t.Fatalf("resource job run = %#v, want job_id %q", jobRun.Run, jobID)
	}

	updateJobResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPut,
		"http://unix/api/resources/automation.job/"+jobID,
		[]byte(fmt.Sprintf(
			`{"scope":{"kind":"global"},"expected_version":%d,"spec":{"scope":"global","name":"resource-job","agent_name":"coder","prompt":"review after resource update","schedule":{"mode":"every","interval":"1h"},"enabled":true,"source":"dynamic"}}`,
			createdJobResource.Record.Version,
		)),
		nil,
	)
	if updateJobResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(updateJobResp.Body)
		_ = updateJobResp.Body.Close()
		t.Fatalf(
			"update automation.job resource status = %d, want %d; body=%s",
			updateJobResp.StatusCode,
			http.StatusOK,
			string(body),
		)
	}
	var updatedJobResource contract.ResourceResponse
	decodeHTTPJSON(t, updateJobResp, &updatedJobResource)
	waitForAutomationJobPrompt(t, runtime, jobID, "review after resource update")

	runResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/automation/runs/"+jobRun.Run.ID,
		nil,
		nil,
	)
	if runResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(runResp.Body)
		_ = runResp.Body.Close()
		t.Fatalf("get resource job run status = %d, want %d; body=%s", runResp.StatusCode, http.StatusOK, string(body))
	}
	var fetchedRun contract.RunResponse
	decodeHTTPJSON(t, runResp, &fetchedRun)
	if fetchedRun.Run.ID != jobRun.Run.ID || fetchedRun.Run.JobID != jobID {
		t.Fatalf("fetched resource job run = %#v, want run %q for job %q", fetchedRun.Run, jobRun.Run.ID, jobID)
	}

	triggerID := "resource-trigger"
	createTriggerResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPut,
		"http://unix/api/resources/automation.trigger/"+triggerID,
		[]byte(
			`{"scope":{"kind":"global"},"spec":{"scope":"global","name":"resource-trigger","agent_name":"coder","prompt":"inspect {{ index .Data \"session_id\" }}","event":"session.stopped","enabled":true,"source":"dynamic"}}`,
		),
		nil,
	)
	if createTriggerResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createTriggerResp.Body)
		_ = createTriggerResp.Body.Close()
		t.Fatalf(
			"create automation.trigger resource status = %d, want %d; body=%s",
			createTriggerResp.StatusCode,
			http.StatusCreated,
			string(body),
		)
	}
	var createdTriggerResource contract.ResourceResponse
	decodeHTTPJSON(t, createTriggerResp, &createdTriggerResource)
	projectedTrigger := waitForAutomationTriggerPrompt(t, runtime, triggerID, `inspect {{ index .Data "session_id" }}`)
	if projectedTrigger.Source != automationpkg.JobSourceDynamic {
		t.Fatalf("projected trigger source = %q, want %q", projectedTrigger.Source, automationpkg.JobSourceDynamic)
	}

	updateTriggerResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPut,
		"http://unix/api/resources/automation.trigger/"+triggerID,
		[]byte(fmt.Sprintf(
			`{"scope":{"kind":"global"},"expected_version":%d,"spec":{"scope":"global","name":"resource-trigger","agent_name":"coder","prompt":"inspect resource {{ index .Data \"session_id\" }}","event":"session.stopped","enabled":true,"source":"dynamic"}}`,
			createdTriggerResource.Record.Version,
		)),
		nil,
	)
	if updateTriggerResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(updateTriggerResp.Body)
		_ = updateTriggerResp.Body.Close()
		t.Fatalf(
			"update automation.trigger resource status = %d, want %d; body=%s",
			updateTriggerResp.StatusCode,
			http.StatusOK,
			string(body),
		)
	}
	var updatedTriggerResource contract.ResourceResponse
	decodeHTTPJSON(t, updateTriggerResp, &updatedTriggerResource)
	waitForAutomationTriggerPrompt(t, runtime, triggerID, `inspect resource {{ index .Data "session_id" }}`)

	deleteTriggerResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodDelete,
		"http://unix/api/resources/automation.trigger/"+triggerID,
		[]byte(fmt.Sprintf(`{"expected_version":%d}`, updatedTriggerResource.Record.Version)),
		nil,
	)
	if deleteTriggerResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(deleteTriggerResp.Body)
		_ = deleteTriggerResp.Body.Close()
		t.Fatalf(
			"delete automation.trigger resource status = %d, want %d; body=%s",
			deleteTriggerResp.StatusCode,
			http.StatusNoContent,
			string(body),
		)
	}
	_ = deleteTriggerResp.Body.Close()
	waitForAutomationTriggerMissing(t, runtime, triggerID)

	deleteJobResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodDelete,
		"http://unix/api/resources/automation.job/"+jobID,
		[]byte(fmt.Sprintf(`{"expected_version":%d}`, updatedJobResource.Record.Version)),
		nil,
	)
	if deleteJobResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(deleteJobResp.Body)
		_ = deleteJobResp.Body.Close()
		t.Fatalf(
			"delete automation.job resource status = %d, want %d; body=%s",
			deleteJobResp.StatusCode,
			http.StatusNoContent,
			string(body),
		)
	}
	_ = deleteJobResp.Body.Close()
	waitForAutomationJobMissing(t, runtime, jobID)

	runAfterDeleteResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/automation/runs/"+jobRun.Run.ID,
		nil,
		nil,
	)
	if runAfterDeleteResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(runAfterDeleteResp.Body)
		_ = runAfterDeleteResp.Body.Close()
		t.Fatalf(
			"get run after resource delete status = %d, want %d; body=%s",
			runAfterDeleteResp.StatusCode,
			http.StatusOK,
			string(body),
		)
	}
	var runAfterDelete contract.RunResponse
	decodeHTTPJSON(t, runAfterDeleteResp, &runAfterDelete)
	if runAfterDelete.Run.ID != jobRun.Run.ID || runAfterDelete.Run.JobID != jobID {
		t.Fatalf("run after resource delete = %#v, want run %q for job %q", runAfterDelete.Run, jobRun.Run.ID, jobID)
	}
}

func TestUDSAutomationTriggerRunsAndOmitsWebhookRoutes(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	resolveResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/workspaces/resolve",
		[]byte(`{"path":"`+runtime.workspace+`"}`),
		nil,
	)
	if resolveResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resolveResp.Body)
		_ = resolveResp.Body.Close()
		t.Fatalf("resolve workspace status = %d, want %d; body=%s", resolveResp.StatusCode, http.StatusOK, string(body))
	}
	var resolved contract.WorkspaceResponse
	decodeHTTPJSON(t, resolveResp, &resolved)
	if resolved.Workspace.ID == "" {
		t.Fatal("expected resolved workspace id")
	}
	createdSession := createIntegrationSessionPayload(t, runtime)
	if createdSession.WorkspaceID != resolved.Workspace.ID {
		t.Fatalf(
			"created session workspace_id = %q, want resolved workspace id %q",
			createdSession.WorkspaceID,
			resolved.Workspace.ID,
		)
	}

	createResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/automation/triggers",
		[]byte(
			`{"scope":"workspace","workspace_id":"`+createdSession.WorkspaceID+`","name":"session-stop-review","agent_name":"coder","prompt":"review {{ index .Data \"session_id\" }}","event":"session.stopped","filter":{"data.session_type":"user"}}`,
		),
		nil,
	)
	if createResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResp.Body)
		_ = createResp.Body.Close()
		t.Fatalf(
			"create trigger status = %d, want %d; body=%s",
			createResp.StatusCode,
			http.StatusCreated,
			string(body),
		)
	}
	var created contract.TriggerResponse
	decodeHTTPJSON(t, createResp, &created)
	if created.Trigger.ID == "" {
		t.Fatal("expected created automation trigger id")
	}

	updateResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPatch,
		"http://unix/api/automation/triggers/"+created.Trigger.ID,
		[]byte(`{"prompt":"inspect {{ index .Data \"session_id\" }}"}`),
		nil,
	)
	if updateResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(updateResp.Body)
		_ = updateResp.Body.Close()
		t.Fatalf("update trigger status = %d, want %d; body=%s", updateResp.StatusCode, http.StatusOK, string(body))
	}
	var updated contract.TriggerResponse
	decodeHTTPJSON(t, updateResp, &updated)
	if updated.Trigger.Prompt != `inspect {{ index .Data "session_id" }}` {
		t.Fatalf("updated trigger prompt = %q", updated.Trigger.Prompt)
	}

	stopIntegrationSession(t, runtime, createdSession.WorkspaceID, createdSession.ID)

	var runs contract.RunsResponse
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		runsResp := mustUnixRequest(
			t,
			runtime.client,
			http.MethodGet,
			"http://unix/api/automation/triggers/"+created.Trigger.ID+"/runs",
			nil,
			nil,
		)
		if runsResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(runsResp.Body)
			_ = runsResp.Body.Close()
			t.Fatalf("trigger runs status = %d, want %d; body=%s", runsResp.StatusCode, http.StatusOK, string(body))
		}

		runs = contract.RunsResponse{}
		decodeHTTPJSON(t, runsResp, &runs)
		if len(runs.Runs) > 0 {
			break
		}

		select {
		case <-deadline:
			t.Fatalf("expected trigger run history, got %#v", runs.Runs)
		case <-ticker.C:
		}
	}
	runID := runs.Runs[0].ID

	runResp := mustUnixRequest(t, runtime.client, http.MethodGet, "http://unix/api/automation/runs/"+runID, nil, nil)
	if runResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(runResp.Body)
		_ = runResp.Body.Close()
		t.Fatalf("get trigger run status = %d, want %d; body=%s", runResp.StatusCode, http.StatusOK, string(body))
	}
	var run contract.RunResponse
	decodeHTTPJSON(t, runResp, &run)
	if run.Run.TriggerID != created.Trigger.ID {
		t.Fatalf("trigger run = %#v, want trigger_id %q", run.Run, created.Trigger.ID)
	}

	webhookResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/webhooks/global/deploy-review--wbh_test",
		[]byte(`{"payload":"deploy"}`),
		nil,
	)
	if webhookResp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(webhookResp.Body)
		_ = webhookResp.Body.Close()
		t.Fatalf(
			"webhook route status = %d, want %d; body=%s",
			webhookResp.StatusCode,
			http.StatusNotFound,
			string(body),
		)
	}
	_ = webhookResp.Body.Close()

	deleteResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodDelete,
		"http://unix/api/automation/triggers/"+created.Trigger.ID,
		nil,
		nil,
	)
	if deleteResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(deleteResp.Body)
		_ = deleteResp.Body.Close()
		t.Fatalf(
			"delete trigger status = %d, want %d; body=%s",
			deleteResp.StatusCode,
			http.StatusNoContent,
			string(body),
		)
	}
	_ = deleteResp.Body.Close()
}

func TestUDSSessionStreamReconnectsWithLastEventID(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	created := createIntegrationSessionPayload(t, runtime)
	sessionID := created.ID
	sendPrompt(t, runtime, created.WorkspaceID, sessionID, "hello")
	stopIntegrationSession(t, runtime, created.WorkspaceID, sessionID)

	streamResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		sessionAPIPath(created.WorkspaceID, sessionID, "/stream?frames=raw"),
		nil,
		nil,
	)
	if streamResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(streamResp.Body)
		_ = streamResp.Body.Close()
		t.Fatalf("session stream status = %d, want %d; body=%s", streamResp.StatusCode, http.StatusOK, string(body))
	}
	initial := collectLiveSSEUntilEvent(t, streamResp.Body, session.EventTypeSessionStopped, 2*time.Second)
	_ = streamResp.Body.Close()
	if len(initial) < 3 {
		t.Fatalf("initial stream events = %d, want 3", len(initial))
	}
	if initial[len(initial)-1].Event != session.EventTypeSessionStopped {
		t.Fatalf("last event = %q, want %q", initial[len(initial)-1].Event, session.EventTypeSessionStopped)
	}

	headers := map[string]string{"Last-Event-ID": initial[0].ID}
	replayResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		sessionAPIPath(created.WorkspaceID, sessionID, "/stream?frames=raw"),
		nil,
		headers,
	)
	if replayResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(replayResp.Body)
		_ = replayResp.Body.Close()
		t.Fatalf("replay stream status = %d, want %d; body=%s", replayResp.StatusCode, http.StatusOK, string(body))
	}
	replayed := collectLiveSSE(t, replayResp.Body, 2, 2*time.Second)
	_ = replayResp.Body.Close()
	if len(replayed) < 2 {
		t.Fatalf("replayed events = %d, want 2", len(replayed))
	}
	if replayed[0].ID != initial[1].ID {
		t.Fatalf("replayed first id = %q, want %q", replayed[0].ID, initial[1].ID)
	}
	if replayed[0].ID == initial[0].ID {
		t.Fatalf("replayed first id = %q, should skip Last-Event-ID", replayed[0].ID)
	}
}

func TestUDSShutdownWaitsForInflightRequests(t *testing.T) {
	homePaths := newTestHomePaths(t)
	socketPath := shortSocketPath(t)
	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.Daemon.Socket = socketPath

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	server, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithSocketPath(socketPath),
		WithLogger(discardLogger()),
		WithSessionManager(stubSessionManager{
			ListAllFn: func(context.Context) ([]*session.Info, error) {
				entered <- struct{}{}
				<-release
				return []*session.Info{newSessionInfo("sess-1")}, nil
			},
		}),
		WithTaskService(&stubTaskManager{}),
		WithObserver(stubObserver{
			HealthFn: func(context.Context) (observe.Health, error) { return observe.Health{Status: "ok"}, nil },
		}),
		WithWorkspaceResolver(stubWorkspaceService{}),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	released := false
	defer func() {
		if !released {
			close(release)
		}
		_ = server.Shutdown(context.Background())
	}()

	client := newUnixClient(t, socketPath)
	respCh := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := client.Get("http://unix/api/sessions")
		if err != nil {
			errCh <- err
			return
		}
		respCh <- resp
	}()

	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for in-flight request")
	}

	shutdownDone := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		shutdownDone <- server.Shutdown(ctx)
	}()

	select {
	case err := <-shutdownDone:
		t.Fatalf("Shutdown() returned early: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	released = true

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for shutdown")
	}

	select {
	case err := <-errCh:
		t.Fatalf("request error = %v", err)
	case resp := <-respCh:
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			t.Fatalf("request status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, string(body))
		}
		_ = resp.Body.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for request response")
	}
}

func TestUDSShutdownCancelsPersistentStreams(t *testing.T) {
	t.Run("Should cancel an active session catalog stream before the shutdown deadline", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		socketPath := shortSocketPath(t)
		cfg := aghconfig.DefaultWithHome(homePaths)
		cfg.Daemon.Socket = socketPath
		events := make(chan session.CatalogEvent)
		server, err := New(
			WithHomePaths(homePaths),
			WithConfig(&cfg),
			WithSocketPath(socketPath),
			WithLogger(discardLogger()),
			WithSessionManager(stubSessionManager{
				SubscribeCatalogFn: func(context.Context) (<-chan session.CatalogEvent, func(), error) {
					return events, func() {}, nil
				},
			}),
			WithTaskService(&stubTaskManager{}),
			WithObserver(stubObserver{}),
			WithWorkspaceResolver(stubWorkspaceService{}),
		)
		if err != nil {
			t.Fatalf("New() error = %v", err)
		}
		if err := server.Start(t.Context()); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		t.Cleanup(func() {
			cleanupCtx, cancelCleanup := context.WithTimeout(context.WithoutCancel(t.Context()), 2*time.Second)
			defer cancelCleanup()
			if err := server.Shutdown(cleanupCtx); err != nil {
				t.Errorf("Shutdown() during cleanup error = %v", err)
			}
		})

		streamResponse := mustUnixRequest(
			t,
			newUnixClient(t, socketPath),
			http.MethodGet,
			"http://unix/api/sessions/catalog-stream",
			nil,
			nil,
		)
		if streamResponse.StatusCode != http.StatusOK {
			body := readAndCloseHTTPBody(t, streamResponse)
			t.Fatalf("stream status = %d, want %d; body=%s", streamResponse.StatusCode, http.StatusOK, body)
		}
		if contentType := streamResponse.Header.Get("Content-Type"); contentType != "text/event-stream" {
			closeHTTPBody(t, streamResponse.Body)
			t.Fatalf("stream content type = %q, want text/event-stream", contentType)
		}

		streamDone := make(chan error, 1)
		go func() {
			_, readErr := io.Copy(io.Discard, streamResponse.Body)
			streamDone <- errors.Join(readErr, streamResponse.Body.Close())
		}()

		shutdownCtx, cancel := context.WithTimeout(t.Context(), 500*time.Millisecond)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}

		select {
		case err := <-streamDone:
			if err != nil {
				t.Fatalf("stream close error = %v", err)
			}
		case <-shutdownCtx.Done():
			t.Fatalf("stream remained active after shutdown: %v", shutdownCtx.Err())
		}
	})
}

func TestUDSSessionParticipationRoundTrip(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	apitestutil.EnableIntegrationLiveNetwork(t, runtime.registry)

	createResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/sessions",
		[]byte(
			`{"agent_name":"coder","workspace_path":"`+runtime.workspace+`","network_participation":{"mode":"live","channel_strategy":"named","channel_id":"builders"}}`,
		),
		nil,
	)
	if createResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(createResp.Body)
		_ = createResp.Body.Close()
		t.Fatalf(
			"create session status = %d, want %d; body=%s",
			createResp.StatusCode,
			http.StatusCreated,
			string(body),
		)
	}
	var created struct {
		Session sessionPayload `json:"session"`
	}
	decodeHTTPJSON(t, createResp, &created)
	apitestutil.AssertResolvedParticipationChannel(
		t,
		created.Session.ResolvedNetworkParticipation,
		created.Session.WorkspaceID,
		"builders",
		participation.SourceExplicitRequest,
	)
	if created.Session.WorkspaceID == "" {
		t.Fatal("expected created session workspace id")
	}

	listResp := mustUnixRequest(t, runtime.client, http.MethodGet, "http://unix/api/sessions", nil, nil)
	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		_ = listResp.Body.Close()
		t.Fatalf("list sessions status = %d, want %d; body=%s", listResp.StatusCode, http.StatusOK, string(body))
	}
	var listed struct {
		Sessions []sessionPayload `json:"sessions"`
	}
	decodeHTTPJSON(t, listResp, &listed)
	if got, want := len(listed.Sessions), 1; got != want {
		t.Fatalf("len(listed.Sessions) = %d, want %d", got, want)
	}
	apitestutil.AssertResolvedParticipationChannel(
		t,
		listed.Sessions[0].ResolvedNetworkParticipation,
		created.Session.WorkspaceID,
		"builders",
		participation.SourceExplicitRequest,
	)

	stopIntegrationSession(t, runtime, created.Session.WorkspaceID, created.Session.ID)

	statusResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		sessionAPIPath(created.Session.WorkspaceID, created.Session.ID, ""),
		nil,
		nil,
	)
	if statusResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(statusResp.Body)
		_ = statusResp.Body.Close()
		t.Fatalf("status after stop = %d, want %d; body=%s", statusResp.StatusCode, http.StatusOK, string(body))
	}
	var stopped struct {
		Session sessionPayload `json:"session"`
	}
	decodeHTTPJSON(t, statusResp, &stopped)
	apitestutil.AssertResolvedParticipationChannel(
		t,
		stopped.Session.ResolvedNetworkParticipation,
		created.Session.WorkspaceID,
		"builders",
		participation.SourceExplicitRequest,
	)
	if stopped.Session.State != session.StateStopped {
		t.Fatalf("stopped session state = %q, want %q", stopped.Session.State, session.StateStopped)
	}

	resumeResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		sessionAPIPath(created.Session.WorkspaceID, created.Session.ID, "/attach"),
		nil,
		nil,
	)
	if resumeResp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(resumeResp.Body)
		_ = resumeResp.Body.Close()
		t.Fatalf(
			"attach stopped session status = %d, want %d; body=%s",
			resumeResp.StatusCode,
			http.StatusConflict,
			string(body),
		)
	}
}

func TestUDSTaskRoutesRoundTrip(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	created := createIntegrationTask(t, runtime, []byte(`{
		"scope":"global",
		"title":"Ship task routes",
		"description":"Expose the transport routes",
		"owner":{"kind":"pool","ref":"ops"},
		"metadata":{"priority":"high"}
	}`))
	if created.ID == "" {
		t.Fatal("expected created task id")
	}
	if created.Scope != taskpkg.ScopeGlobal {
		t.Fatalf("created scope = %q, want %q", created.Scope, taskpkg.ScopeGlobal)
	}
	if created.ResolvedNetworkParticipation != nil {
		t.Fatalf(
			"created resolved_network_participation = %#v, want nil without an active run",
			created.ResolvedNetworkParticipation,
		)
	}
	if created.Owner == nil || created.Owner.Kind != taskpkg.OwnerKindPool || created.Owner.Ref != "ops" {
		t.Fatalf("created owner = %#v, want pool/ops", created.Owner)
	}
	if created.Origin.Kind != taskpkg.OriginKindUDS {
		t.Fatalf("created origin.kind = %q, want %q", created.Origin.Kind, taskpkg.OriginKindUDS)
	}
	if created.CreatedBy.Ref != "local-user" {
		t.Fatalf("created created_by.ref = %q, want %q", created.CreatedBy.Ref, "local-user")
	}
	if got := strings.TrimSpace(string(created.Metadata)); got != `{"priority":"high"}` {
		t.Fatalf("created metadata = %s, want %s", got, `{"priority":"high"}`)
	}

	listResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/tasks?scope=global&status=ready&owner_kind=pool&owner_ref=ops",
		nil,
		nil,
	)
	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		_ = listResp.Body.Close()
		t.Fatalf("list tasks status = %d, want %d; body=%s", listResp.StatusCode, http.StatusOK, string(body))
	}
	var listed contract.TasksResponse
	decodeHTTPJSON(t, listResp, &listed)
	if len(listed.Tasks) != 1 || listed.Tasks[0].ID != created.ID {
		t.Fatalf("listed tasks = %#v, want created task", listed.Tasks)
	}

	getResp := mustUnixRequest(t, runtime.client, http.MethodGet, "http://unix/api/tasks/"+created.ID, nil, nil)
	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		_ = getResp.Body.Close()
		t.Fatalf("get task status = %d, want %d; body=%s", getResp.StatusCode, http.StatusOK, string(body))
	}
	var detail contract.TaskDetailResponse
	decodeHTTPJSON(t, getResp, &detail)
	if detail.Task.Task.ID != created.ID {
		t.Fatalf("detail task id = %q, want %q", detail.Task.Task.ID, created.ID)
	}
	if len(detail.Task.Children) != 0 || len(detail.Task.Runs) != 0 {
		t.Fatalf("detail task children/runs = %#v, want empty", detail.Task)
	}

	updateResp := mustUnixRequest(t, runtime.client, http.MethodPatch, "http://unix/api/tasks/"+created.ID, []byte(`{
		"title":"Ship task routes now",
		"description":"Expose the task and run transports everywhere",
		"clear_owner":true
	}`), nil)
	if updateResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(updateResp.Body)
		_ = updateResp.Body.Close()
		t.Fatalf("update task status = %d, want %d; body=%s", updateResp.StatusCode, http.StatusOK, string(body))
	}
	var updated contract.TaskResponse
	decodeHTTPJSON(t, updateResp, &updated)
	if updated.Task.Title != "Ship task routes now" {
		t.Fatalf("updated title = %q, want %q", updated.Task.Title, "Ship task routes now")
	}
	if updated.Task.Description != "Expose the task and run transports everywhere" {
		t.Fatalf("updated description = %q", updated.Task.Description)
	}
	if updated.Task.ResolvedNetworkParticipation != nil {
		t.Fatalf(
			"updated resolved_network_participation = %#v, want nil without an active run",
			updated.Task.ResolvedNetworkParticipation,
		)
	}
	if updated.Task.Owner != nil {
		t.Fatalf("updated owner = %#v, want nil", updated.Task.Owner)
	}

	updatedListResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/tasks?scope=global&status=ready",
		nil,
		nil,
	)
	if updatedListResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(updatedListResp.Body)
		_ = updatedListResp.Body.Close()
		t.Fatalf(
			"updated list tasks status = %d, want %d; body=%s",
			updatedListResp.StatusCode,
			http.StatusOK,
			string(body),
		)
	}
	var updatedList contract.TasksResponse
	decodeHTTPJSON(t, updatedListResp, &updatedList)
	if len(updatedList.Tasks) != 1 || updatedList.Tasks[0].ID != created.ID {
		t.Fatalf("updated list tasks = %#v, want created task", updatedList.Tasks)
	}
}

func TestUDSTaskParticipationRoundTrip(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	workspaceID := createIntegrationSessionPayload(t, runtime).WorkspaceID
	apitestutil.EnableIntegrationLiveNetwork(t, runtime.registry)

	created := createIntegrationTask(t, runtime, fmt.Appendf(nil, `{
		"id":"task-uds-live",
		"scope":"workspace",
		"workspace":%q,
		"title":"Verify UDS participation",
		"network_participation":{
			"mode":"live",
			"channel_strategy":"named",
			"channel_id":"builders"
		}
	}`, workspaceID))
	if created.WorkspaceID != workspaceID {
		t.Fatalf("created workspace_id = %q, want %q", created.WorkspaceID, workspaceID)
	}
	if created.ResolvedNetworkParticipation != nil {
		t.Fatalf(
			"created resolved_network_participation = %#v, want nil before a run",
			created.ResolvedNetworkParticipation,
		)
	}

	getProfile := func() taskpkg.ExecutionProfile {
		t.Helper()
		resp := mustUnixRequest(
			t,
			runtime.client,
			http.MethodGet,
			"http://unix/api/tasks/"+created.ID+"/execution-profile",
			nil,
			nil,
		)
		if resp.StatusCode != http.StatusOK {
			body := readAndCloseHTTPBody(t, resp)
			t.Fatalf("get execution profile status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, body)
		}
		var payload contract.TaskExecutionProfileResponse
		decodeHTTPJSON(t, resp, &payload)
		return payload.Profile
	}
	assertProfileChannel := func(wantChannel string) {
		t.Helper()
		profile := getProfile()
		request := profile.NetworkParticipation
		if request == nil || request.Mode == nil || *request.Mode != participation.ModeLive ||
			request.ChannelStrategy == nil || *request.ChannelStrategy != participation.StrategyNamed ||
			request.ChannelID == nil || *request.ChannelID != wantChannel {
			t.Fatalf("profile network_participation = %#v, want named Live channel %q", request, wantChannel)
		}
	}
	assertProfileChannel("builders")

	invalidUpdate := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPatch,
		"http://unix/api/tasks/"+created.ID,
		[]byte(`{"title":"Must not persist","network_participation":{"mode":"live"}}`),
		nil,
	)
	if invalidUpdate.StatusCode != http.StatusBadRequest {
		body := readAndCloseHTTPBody(t, invalidUpdate)
		t.Fatalf("invalid update status = %d, want %d; body=%s", invalidUpdate.StatusCode, http.StatusBadRequest, body)
	}
	closeHTTPBody(t, invalidUpdate.Body)

	detailResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/tasks/"+created.ID,
		nil,
		nil,
	)
	if detailResp.StatusCode != http.StatusOK {
		body := readAndCloseHTTPBody(t, detailResp)
		t.Fatalf(
			"get task after invalid update status = %d, want %d; body=%s",
			detailResp.StatusCode,
			http.StatusOK,
			body,
		)
	}
	var unchanged contract.TaskDetailResponse
	decodeHTTPJSON(t, detailResp, &unchanged)
	if unchanged.Task.Task.Title != created.Title {
		t.Fatalf("title after invalid update = %q, want %q", unchanged.Task.Task.Title, created.Title)
	}
	assertProfileChannel("builders")

	validUpdate := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPatch,
		"http://unix/api/tasks/"+created.ID,
		[]byte(`{"network_participation":{"mode":"live","channel_strategy":"named","channel_id":"reviewers"}}`),
		nil,
	)
	if validUpdate.StatusCode != http.StatusOK {
		body := readAndCloseHTTPBody(t, validUpdate)
		t.Fatalf("valid update status = %d, want %d; body=%s", validUpdate.StatusCode, http.StatusOK, body)
	}
	closeHTTPBody(t, validUpdate.Body)
	assertProfileChannel("reviewers")

	queued := enqueueIntegrationTaskRun(t, runtime, created.ID, `{
		"idempotency_key":"uds-live-enqueue",
		"network_participation":{
			"mode":"live",
			"channel_strategy":"named",
			"channel_id":"builders"
		}
	}`)
	apitestutil.AssertResolvedParticipationChannel(
		t,
		queued.ResolvedNetworkParticipation,
		workspaceID,
		"builders",
		participation.SourceExplicitRequest,
	)

	assertTaskAbsent := func(id string) {
		t.Helper()
		resp := mustUnixRequest(t, runtime.client, http.MethodGet, "http://unix/api/tasks/"+id, nil, nil)
		if resp.StatusCode != http.StatusNotFound {
			body := readAndCloseHTTPBody(t, resp)
			t.Fatalf(
				"get rejected task %q status = %d, want %d; body=%s",
				id,
				resp.StatusCode,
				http.StatusNotFound,
				body,
			)
		}
		closeHTTPBody(t, resp.Body)
	}
	for _, rejected := range []struct {
		name string
		id   string
		body []byte
	}{
		{
			name: "Should reject an invalid participation request atomically",
			id:   "task-uds-invalid-participation",
			body: fmt.Appendf(
				nil,
				`{"id":"task-uds-invalid-participation","scope":"workspace","workspace":%q,"title":"Invalid participation","network_participation":{"mode":"live"}}`,
				workspaceID,
			),
		},
		{
			name: "Should reject a removed flat channel field atomically",
			id:   "task-uds-legacy-channel",
			body: fmt.Appendf(
				nil,
				`{"id":"task-uds-legacy-channel","scope":"workspace","workspace":%q,"title":"Legacy channel","channel":"builders"}`,
				workspaceID,
			),
		},
	} {
		t.Run(rejected.name, func(t *testing.T) {
			resp := mustUnixRequest(
				t,
				runtime.client,
				http.MethodPost,
				"http://unix/api/tasks",
				rejected.body,
				nil,
			)
			if resp.StatusCode != http.StatusBadRequest {
				body := readAndCloseHTTPBody(t, resp)
				t.Fatalf("rejected create status = %d, want %d; body=%s", resp.StatusCode, http.StatusBadRequest, body)
			}
			closeHTTPBody(t, resp.Body)
			assertTaskAbsent(rejected.id)
		})
	}
}

func TestUDSTaskRunLifecycleRoutesRoundTrip(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	workspaceID := createIntegrationSessionPayload(t, runtime).WorkspaceID
	created := createIntegrationTask(
		t,
		runtime,
		fmt.Appendf(nil, `{"scope":"workspace","workspace":%q,"title":"Run task routes"}`, workspaceID),
	)

	queued := enqueueIntegrationTaskRun(
		t,
		runtime,
		created.ID,
		`{"idempotency_key":"enqueue-1"}`,
	)
	if queued.Status != taskpkg.TaskRunStatusQueued {
		t.Fatalf("queued status = %q, want %q", queued.Status, taskpkg.TaskRunStatusQueued)
	}
	apitestutil.AssertLocalResolvedParticipation(t, queued.ResolvedNetworkParticipation)

	listQueuedResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/tasks/"+created.ID+"/runs?status=queued&limit=1",
		nil,
		nil,
	)
	if listQueuedResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listQueuedResp.Body)
		_ = listQueuedResp.Body.Close()
		t.Fatalf(
			"list queued runs status = %d, want %d; body=%s",
			listQueuedResp.StatusCode,
			http.StatusOK,
			string(body),
		)
	}
	var queuedList contract.TaskRunsResponse
	decodeHTTPJSON(t, listQueuedResp, &queuedList)
	if len(queuedList.Runs) != 1 || queuedList.Runs[0].ID != queued.ID {
		t.Fatalf("queued runs = %#v, want queued run", queuedList.Runs)
	}

	seedIntegrationTaskRunClaimed(t, runtime, queued.ID)
	claimedRun, err := runtime.registry.GetTaskRun(context.Background(), queued.ID)
	if err != nil {
		t.Fatalf("GetTaskRun(%q) error = %v", queued.ID, err)
	}
	claimed := core.TaskRunPayloadFromRun(&claimedRun)
	if claimed.Status != taskpkg.TaskRunStatusClaimed {
		t.Fatalf("claimed status = %q, want %q", claimed.Status, taskpkg.TaskRunStatusClaimed)
	}
	if claimed.ClaimedBy == nil || claimed.ClaimedBy.Ref != "local-user" {
		t.Fatalf("claimed claimed_by = %#v, want local-user", claimed.ClaimedBy)
	}

	startResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/task-runs/"+queued.ID+"/start",
		[]byte(`{"idempotency_key":"start-1"}`),
		nil,
	)
	if startResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(startResp.Body)
		_ = startResp.Body.Close()
		t.Fatalf("start run status = %d, want %d; body=%s", startResp.StatusCode, http.StatusOK, string(body))
	}
	var started contract.TaskRunResponse
	decodeHTTPJSON(t, startResp, &started)
	if started.Run.Status != taskpkg.TaskRunStatusRunning {
		t.Fatalf("started status = %q, want %q", started.Run.Status, taskpkg.TaskRunStatusRunning)
	}
	if started.Run.SessionID == "" {
		t.Fatal("expected started run session id")
	}

	completeResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/task-runs/"+queued.ID+"/complete",
		[]byte(`{"result":{"ok":true}}`),
		nil,
	)
	if completeResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(completeResp.Body)
		_ = completeResp.Body.Close()
		t.Fatalf("complete run status = %d, want %d; body=%s", completeResp.StatusCode, http.StatusOK, string(body))
	}
	var completed contract.TaskRunResponse
	decodeHTTPJSON(t, completeResp, &completed)
	if completed.Run.Status != taskpkg.TaskRunStatusCompleted {
		t.Fatalf("completed status = %q, want %q", completed.Run.Status, taskpkg.TaskRunStatusCompleted)
	}

	secondRun := enqueueIntegrationTaskRun(t, runtime, created.ID, `{"idempotency_key":"enqueue-2"}`)
	seedIntegrationTaskRunClaimed(t, runtime, secondRun.ID)
	attachResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/task-runs/"+secondRun.ID+"/attach-session",
		[]byte(`{"session_id":"sess-resume-1"}`),
		nil,
	)
	if attachResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(attachResp.Body)
		_ = attachResp.Body.Close()
		t.Fatalf("attach run session status = %d, want %d; body=%s", attachResp.StatusCode, http.StatusOK, string(body))
	}
	var attached contract.TaskRunResponse
	decodeHTTPJSON(t, attachResp, &attached)
	if attached.Run.Status != taskpkg.TaskRunStatusStarting {
		t.Fatalf("attached status = %q, want %q", attached.Run.Status, taskpkg.TaskRunStatusStarting)
	}
	if attached.Run.SessionID != "sess-resume-1" {
		t.Fatalf("attached session_id = %q, want %q", attached.Run.SessionID, "sess-resume-1")
	}

	failResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/task-runs/"+secondRun.ID+"/fail",
		[]byte(`{"error":"boom","metadata":{"step":"attach"}}`),
		nil,
	)
	if failResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(failResp.Body)
		_ = failResp.Body.Close()
		t.Fatalf("fail run status = %d, want %d; body=%s", failResp.StatusCode, http.StatusOK, string(body))
	}
	var failed contract.TaskRunResponse
	decodeHTTPJSON(t, failResp, &failed)
	if failed.Run.Status != taskpkg.TaskRunStatusFailed {
		t.Fatalf("failed status = %q, want %q", failed.Run.Status, taskpkg.TaskRunStatusFailed)
	}

	thirdRun := enqueueIntegrationTaskRun(t, runtime, created.ID, `{"idempotency_key":"enqueue-3"}`)
	cancelResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/task-runs/"+thirdRun.ID+"/cancel",
		[]byte(`{"reason":"operator cancelled"}`),
		nil,
	)
	if cancelResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(cancelResp.Body)
		_ = cancelResp.Body.Close()
		t.Fatalf("cancel run status = %d, want %d; body=%s", cancelResp.StatusCode, http.StatusOK, string(body))
	}
	var cancelled contract.TaskRunResponse
	decodeHTTPJSON(t, cancelResp, &cancelled)
	if cancelled.Run.Status != taskpkg.TaskRunStatusCanceled {
		t.Fatalf("cancelled status = %q, want %q", cancelled.Run.Status, taskpkg.TaskRunStatusCanceled)
	}

	finalRunsResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/tasks/"+created.ID+"/runs",
		nil,
		nil,
	)
	if finalRunsResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(finalRunsResp.Body)
		_ = finalRunsResp.Body.Close()
		t.Fatalf("final list runs status = %d, want %d; body=%s", finalRunsResp.StatusCode, http.StatusOK, string(body))
	}
	var finalRuns contract.TaskRunsResponse
	decodeHTTPJSON(t, finalRunsResp, &finalRuns)
	if len(finalRuns.Runs) != 3 {
		t.Fatalf("len(final runs) = %d, want 3", len(finalRuns.Runs))
	}

	seenStatuses := map[taskpkg.RunStatus]int{}
	for _, run := range finalRuns.Runs {
		seenStatuses[run.Status]++
	}
	if seenStatuses[taskpkg.TaskRunStatusCompleted] != 1 || seenStatuses[taskpkg.TaskRunStatusFailed] != 1 ||
		seenStatuses[taskpkg.TaskRunStatusCanceled] != 1 {
		t.Fatalf("final run statuses = %#v, want one completed, failed, cancelled", seenStatuses)
	}
}

func TestUDSTaskPublishRunDetailAndLiveRoutesRoundTrip(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	draft := createIntegrationTask(t, runtime, []byte(`{
		"scope":"global",
		"title":"Draft live task routes",
		"draft":true
	}`))
	if draft.Status != taskpkg.TaskStatusDraft {
		t.Fatalf("draft status = %q, want %q", draft.Status, taskpkg.TaskStatusDraft)
	}

	publishResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/tasks/"+draft.ID+"/publish",
		[]byte(`{"idempotency_key":"publish-live-1"}`),
		nil,
	)
	if publishResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(publishResp.Body)
		_ = publishResp.Body.Close()
		t.Fatalf("publish task status = %d, want %d; body=%s", publishResp.StatusCode, http.StatusOK, string(body))
	}
	var published contract.TaskExecutionResponse
	decodeHTTPJSON(t, publishResp, &published)
	if published.Task.Status != taskpkg.TaskStatusReady {
		t.Fatalf("published status = %q, want %q", published.Task.Status, taskpkg.TaskStatusReady)
	}
	if published.Run.Status != taskpkg.TaskRunStatusQueued {
		t.Fatalf("published run status = %q, want %q", published.Run.Status, taskpkg.TaskRunStatusQueued)
	}

	run := published.Run
	seedIntegrationTaskRunClaimed(t, runtime, run.ID)

	startResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/task-runs/"+run.ID+"/start",
		[]byte(`{"idempotency_key":"start-live-1"}`),
		nil,
	)
	if startResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(startResp.Body)
		_ = startResp.Body.Close()
		t.Fatalf("start run status = %d, want %d; body=%s", startResp.StatusCode, http.StatusOK, string(body))
	}
	var started contract.TaskRunResponse
	decodeHTTPJSON(t, startResp, &started)
	if started.Run.Status != taskpkg.TaskRunStatusRunning {
		t.Fatalf("started run status = %q, want %q", started.Run.Status, taskpkg.TaskRunStatusRunning)
	}

	runDetailResp := mustUnixRequest(t, runtime.client, http.MethodGet, "http://unix/api/task-runs/"+run.ID, nil, nil)
	if runDetailResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(runDetailResp.Body)
		_ = runDetailResp.Body.Close()
		t.Fatalf("run detail status = %d, want %d; body=%s", runDetailResp.StatusCode, http.StatusOK, string(body))
	}
	var runDetail contract.TaskRunDetailResponse
	decodeHTTPJSON(t, runDetailResp, &runDetail)
	if runDetail.Run.Run.ID != run.ID {
		t.Fatalf("run detail run id = %q, want %q", runDetail.Run.Run.ID, run.ID)
	}
	if runDetail.Run.Task.ID != draft.ID {
		t.Fatalf("run detail task id = %q, want %q", runDetail.Run.Task.ID, draft.ID)
	}

	timelineResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/tasks/"+draft.ID+"/timeline?limit=20",
		nil,
		nil,
	)
	if timelineResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(timelineResp.Body)
		_ = timelineResp.Body.Close()
		t.Fatalf("timeline status = %d, want %d; body=%s", timelineResp.StatusCode, http.StatusOK, string(body))
	}
	var timeline contract.TaskTimelineResponse
	decodeHTTPJSON(t, timelineResp, &timeline)
	if len(timeline.Timeline) == 0 {
		t.Fatal("timeline = empty, want task activity")
	}
	foundRunTimeline := false
	for _, item := range timeline.Timeline {
		if item.Task.ID != draft.ID {
			t.Fatalf("timeline task id = %q, want %q", item.Task.ID, draft.ID)
		}
		if item.Run != nil && item.Run.ID == run.ID {
			foundRunTimeline = true
		}
	}
	if !foundRunTimeline {
		t.Fatalf("timeline = %#v, want run %q in at least one item", timeline.Timeline, run.ID)
	}

	treeResp := mustUnixRequest(t, runtime.client, http.MethodGet, "http://unix/api/tasks/"+draft.ID+"/tree", nil, nil)
	if treeResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(treeResp.Body)
		_ = treeResp.Body.Close()
		t.Fatalf("tree status = %d, want %d; body=%s", treeResp.StatusCode, http.StatusOK, string(body))
	}
	var tree contract.TaskTreeResponse
	decodeHTTPJSON(t, treeResp, &tree)
	if tree.Tree.Root.Task.ID != draft.ID {
		t.Fatalf("tree root id = %q, want %q", tree.Tree.Root.Task.ID, draft.ID)
	}

	streamResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/tasks/"+draft.ID+"/stream?after_sequence=0",
		nil,
		nil,
	)
	if streamResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(streamResp.Body)
		_ = streamResp.Body.Close()
		t.Fatalf("task stream status = %d, want %d; body=%s", streamResp.StatusCode, http.StatusOK, string(body))
	}
	if got := streamResp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("task stream content-type = %q, want text/event-stream", got)
	}
	records := collectLiveSSE(t, streamResp.Body, 1, 2*time.Second)
	_ = streamResp.Body.Close()
	if len(records) == 0 {
		t.Fatal("task stream records = 0, want at least one SSE event")
	}
	if records[0].Event == "" {
		t.Fatalf("task stream first record = %#v, want named SSE event", records[0])
	}
	var streamPayload contract.TaskStreamEventPayload
	if err := json.Unmarshal(records[0].Data, &streamPayload); err != nil {
		t.Fatalf("json.Unmarshal(task stream event) error = %v; record=%#v", err, records[0])
	}
	if streamPayload.Timeline.Task.ID != draft.ID {
		t.Fatalf("task stream timeline task id = %q, want %q", streamPayload.Timeline.Task.ID, draft.ID)
	}
}

func TestUDSTaskDashboardInboxApprovalAndTriageRoutesRoundTrip(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	approvalTask := createIntegrationTask(t, runtime, []byte(`{
		"scope":"global",
		"title":"Needs approval",
		"approval_policy":"manual"
	}`))
	if approvalTask.ApprovalState != taskpkg.ApprovalStatePending {
		t.Fatalf("approval task approval_state = %q, want %q", approvalTask.ApprovalState, taskpkg.ApprovalStatePending)
	}

	rejectTask := createIntegrationTask(t, runtime, []byte(`{
		"scope":"global",
		"title":"Reject me",
		"approval_policy":"manual"
	}`))
	triageTask := createIntegrationTask(t, runtime, []byte(`{
		"scope":"global",
		"title":"Archive me"
	}`))
	dismissTask := createIntegrationTask(t, runtime, []byte(`{
		"scope":"global",
		"title":"Dismiss me"
	}`))

	dashboardResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/observe/tasks/dashboard",
		nil,
		nil,
	)
	if dashboardResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(dashboardResp.Body)
		_ = dashboardResp.Body.Close()
		t.Fatalf("dashboard status = %d, want %d; body=%s", dashboardResp.StatusCode, http.StatusOK, string(body))
	}
	var dashboard contract.TaskDashboardResponse
	decodeHTTPJSON(t, dashboardResp, &dashboard)
	if dashboard.Dashboard.Totals.TasksTotal < 4 {
		t.Fatalf("dashboard tasks_total = %d, want at least 4", dashboard.Dashboard.Totals.TasksTotal)
	}
	if dashboard.Dashboard.Totals.AwaitingApprovalTasks < 2 {
		t.Fatalf(
			"dashboard awaiting_approval_tasks = %d, want at least 2",
			dashboard.Dashboard.Totals.AwaitingApprovalTasks,
		)
	}

	inboxResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/observe/tasks/inbox?lane=approvals&limit=10",
		nil,
		nil,
	)
	if inboxResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(inboxResp.Body)
		_ = inboxResp.Body.Close()
		t.Fatalf("inbox approvals status = %d, want %d; body=%s", inboxResp.StatusCode, http.StatusOK, string(body))
	}
	var inbox contract.TaskInboxResponse
	decodeHTTPJSON(t, inboxResp, &inbox)
	approvalsGroup := requireUDSInboxGroup(t, inbox.Inbox.Groups, contract.TaskInboxLaneApprovals)
	if approvalsGroup.Count < 2 {
		t.Fatalf("approvals count = %d, want at least 2", approvalsGroup.Count)
	}
	if !udsInboxGroupHasTask(approvalsGroup, approvalTask.ID) || !udsInboxGroupHasTask(approvalsGroup, rejectTask.ID) {
		t.Fatalf("approvals group items = %#v, want approval and reject tasks", approvalsGroup.Items)
	}

	approveBody := []byte(`{"idempotency_key":"approve-uds-approval-task"}`)
	approveResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/tasks/"+approvalTask.ID+"/approve",
		approveBody,
		nil,
	)
	if approveResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(approveResp.Body)
		_ = approveResp.Body.Close()
		t.Fatalf("approve status = %d, want %d; body=%s", approveResp.StatusCode, http.StatusCreated, string(body))
	}
	var approved contract.TaskExecutionResponse
	decodeHTTPJSON(t, approveResp, &approved)
	if approved.Task.ApprovalState != taskpkg.ApprovalStateApproved {
		t.Fatalf("approved approval_state = %q, want %q", approved.Task.ApprovalState, taskpkg.ApprovalStateApproved)
	}
	if approved.Run.Status != taskpkg.TaskRunStatusQueued {
		t.Fatalf("approved run status = %q, want %q", approved.Run.Status, taskpkg.TaskRunStatusQueued)
	}

	approveAgainResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/tasks/"+approvalTask.ID+"/approve",
		approveBody,
		nil,
	)
	if approveAgainResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(approveAgainResp.Body)
		_ = approveAgainResp.Body.Close()
		t.Fatalf(
			"approve again status = %d, want %d; body=%s",
			approveAgainResp.StatusCode,
			http.StatusCreated,
			string(body),
		)
	}
	var approvedAgain contract.TaskExecutionResponse
	decodeHTTPJSON(t, approveAgainResp, &approvedAgain)
	if approvedAgain.Run.ID != approved.Run.ID {
		t.Fatalf("approve again run id = %q, want %q", approvedAgain.Run.ID, approved.Run.ID)
	}

	rejectResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/tasks/"+rejectTask.ID+"/reject",
		nil,
		nil,
	)
	if rejectResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(rejectResp.Body)
		_ = rejectResp.Body.Close()
		t.Fatalf("reject status = %d, want %d; body=%s", rejectResp.StatusCode, http.StatusOK, string(body))
	}
	var rejected contract.TaskResponse
	decodeHTTPJSON(t, rejectResp, &rejected)
	if rejected.Task.ApprovalState != taskpkg.ApprovalStateRejected {
		t.Fatalf("rejected approval_state = %q, want %q", rejected.Task.ApprovalState, taskpkg.ApprovalStateRejected)
	}

	readResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/tasks/"+triageTask.ID+"/triage/read",
		nil,
		nil,
	)
	if readResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(readResp.Body)
		_ = readResp.Body.Close()
		t.Fatalf("triage read status = %d, want %d; body=%s", readResp.StatusCode, http.StatusOK, string(body))
	}
	var readState contract.TaskTriageStateResponse
	decodeHTTPJSON(t, readResp, &readState)
	if !readState.Triage.Read {
		t.Fatalf("triage read payload = %#v, want read=true", readState.Triage)
	}

	archiveResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/tasks/"+triageTask.ID+"/triage/archive",
		nil,
		nil,
	)
	if archiveResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(archiveResp.Body)
		_ = archiveResp.Body.Close()
		t.Fatalf("triage archive status = %d, want %d; body=%s", archiveResp.StatusCode, http.StatusOK, string(body))
	}
	var archived contract.TaskTriageStateResponse
	decodeHTTPJSON(t, archiveResp, &archived)
	if !archived.Triage.Archived {
		t.Fatalf("triage archive payload = %#v, want archived=true", archived.Triage)
	}

	dismissResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/tasks/"+dismissTask.ID+"/triage/dismiss",
		nil,
		nil,
	)
	if dismissResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(dismissResp.Body)
		_ = dismissResp.Body.Close()
		t.Fatalf("triage dismiss status = %d, want %d; body=%s", dismissResp.StatusCode, http.StatusOK, string(body))
	}
	var dismissed contract.TaskTriageStateResponse
	decodeHTTPJSON(t, dismissResp, &dismissed)
	if !dismissed.Triage.Dismissed {
		t.Fatalf("triage dismiss payload = %#v, want dismissed=true", dismissed.Triage)
	}

	readMissingResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/tasks/task-missing/triage/read",
		nil,
		nil,
	)
	if readMissingResp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(readMissingResp.Body)
		_ = readMissingResp.Body.Close()
		t.Fatalf(
			"triage read missing status = %d, want %d; body=%s",
			readMissingResp.StatusCode,
			http.StatusNotFound,
			string(body),
		)
	}
	_ = readMissingResp.Body.Close()

	inboxAfterResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/observe/tasks/inbox?lane=approvals&limit=10",
		nil,
		nil,
	)
	if inboxAfterResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(inboxAfterResp.Body)
		_ = inboxAfterResp.Body.Close()
		t.Fatalf(
			"inbox approvals after actions status = %d, want %d; body=%s",
			inboxAfterResp.StatusCode,
			http.StatusOK,
			string(body),
		)
	}
	var inboxAfter contract.TaskInboxResponse
	decodeHTTPJSON(t, inboxAfterResp, &inboxAfter)
	if got := requireUDSInboxGroup(t, inboxAfter.Inbox.Groups, contract.TaskInboxLaneApprovals).Count; got != 0 {
		t.Fatalf("approvals count after approve/reject = %d, want 0", got)
	}

	archivedInboxResp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodGet,
		"http://unix/api/observe/tasks/inbox?lane=archived&limit=10",
		nil,
		nil,
	)
	if archivedInboxResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(archivedInboxResp.Body)
		_ = archivedInboxResp.Body.Close()
		t.Fatalf(
			"inbox archived status = %d, want %d; body=%s",
			archivedInboxResp.StatusCode,
			http.StatusOK,
			string(body),
		)
	}
	var archivedInbox contract.TaskInboxResponse
	decodeHTTPJSON(t, archivedInboxResp, &archivedInbox)
	if !udsInboxGroupHasTask(
		requireUDSInboxGroup(t, archivedInbox.Inbox.Groups, contract.TaskInboxLaneArchived),
		triageTask.ID,
	) {
		t.Fatalf("archived inbox groups = %#v, want task %q", archivedInbox.Inbox.Groups, triageTask.ID)
	}
}

type integrationRuntime struct {
	client         *http.Client
	server         *Server
	manager        *session.Manager
	tasks          *taskpkg.Service
	observer       *observe.Observer
	registry       *globaldb.GlobalDB
	bridges        *integrationBridgeService
	memory         *memory.Store
	dream          *integrationDreamTrigger
	resourceDriver resources.ReconcileDriver
	toolCatalog    *integrationToolCatalog
	socket         string
	workspace      string
}

type integrationToolCatalog struct {
	mu       sync.Mutex
	revision int64
	records  []resources.Record[toolspkg.Tool]
}

type integrationToolPlan struct {
	revision   int64
	operations int
	records    []resources.Record[toolspkg.Tool]
}

func (p *integrationToolPlan) Kind() resources.ResourceKind { return toolspkg.ToolResourceKind }
func (p *integrationToolPlan) Revision() int64              { return p.revision }
func (p *integrationToolPlan) OperationCount() int          { return p.operations }

type integrationToolProjector struct {
	catalog *integrationToolCatalog
}

func (p *integrationToolProjector) Kind() resources.ResourceKind {
	return toolspkg.ToolResourceKind
}

func (p *integrationToolProjector) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *integrationToolProjector) Build(
	_ context.Context,
	records []resources.Record[toolspkg.Tool],
) (resources.ProjectionPlan, error) {
	var revision int64
	cloned := make([]resources.Record[toolspkg.Tool], 0, len(records))
	for _, record := range records {
		if record.Version > revision {
			revision = record.Version
		}
		next := record
		next.Spec = cloneIntegrationTool(record.Spec)
		cloned = append(cloned, next)
	}
	return &integrationToolPlan{
		revision:   revision,
		operations: len(records),
		records:    cloned,
	}, nil
}

func (p *integrationToolProjector) Apply(_ context.Context, plan resources.ProjectionPlan) error {
	typed, ok := plan.(*integrationToolPlan)
	if !ok {
		return fmt.Errorf("integration tool plan type = %T, want *integrationToolPlan", plan)
	}
	p.catalog.replace(typed.revision, typed.records)
	return nil
}

type integrationAutomationJobProjector struct {
	manager *automationpkg.Manager
}

func (p *integrationAutomationJobProjector) Kind() resources.ResourceKind {
	return automationpkg.JobResourceKind
}

func (p *integrationAutomationJobProjector) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *integrationAutomationJobProjector) Build(
	ctx context.Context,
	records []resources.Record[automationpkg.Job],
) (resources.ProjectionPlan, error) {
	return p.manager.BuildJobResourceState(ctx, records)
}

func (p *integrationAutomationJobProjector) Apply(ctx context.Context, plan resources.ProjectionPlan) error {
	return p.manager.ApplyJobResourceState(ctx, plan)
}

type integrationAutomationTriggerProjector struct {
	manager *automationpkg.Manager
}

func (p *integrationAutomationTriggerProjector) Kind() resources.ResourceKind {
	return automationpkg.TriggerResourceKind
}

func (p *integrationAutomationTriggerProjector) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *integrationAutomationTriggerProjector) Build(
	ctx context.Context,
	records []resources.Record[automationpkg.Trigger],
) (resources.ProjectionPlan, error) {
	return p.manager.BuildTriggerResourceState(ctx, records)
}

func (p *integrationAutomationTriggerProjector) Apply(ctx context.Context, plan resources.ProjectionPlan) error {
	return p.manager.ApplyTriggerResourceState(ctx, plan)
}

func (c *integrationToolCatalog) replace(revision int64, records []resources.Record[toolspkg.Tool]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.revision = revision
	c.records = cloneIntegrationToolRecords(records)
}

func (c *integrationToolCatalog) snapshot() (int64, []resources.Record[toolspkg.Tool]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.revision, cloneIntegrationToolRecords(c.records)
}

func cloneIntegrationToolRecords(records []resources.Record[toolspkg.Tool]) []resources.Record[toolspkg.Tool] {
	if len(records) == 0 {
		return nil
	}
	cloned := make([]resources.Record[toolspkg.Tool], 0, len(records))
	for _, record := range records {
		next := record
		next.Spec = cloneIntegrationTool(record.Spec)
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneIntegrationTool(spec toolspkg.Tool) toolspkg.Tool {
	cloned := spec
	cloned.ToolPresentation = toolspkg.CloneToolPresentation(spec.ToolPresentation)
	if len(spec.InputSchema) > 0 {
		cloned.InputSchema = append([]byte(nil), spec.InputSchema...)
	}
	return cloned
}

func waitForAutomationJobPrompt(
	t *testing.T,
	runtime integrationRuntime,
	jobID string,
	wantPrompt string,
) contract.JobPayload {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var last contract.JobResponse
	var lastStatus int
	for time.Now().Before(deadline) {
		resp := mustUnixRequest(t, runtime.client, http.MethodGet, "http://unix/api/automation/jobs/"+jobID, nil, nil)
		lastStatus = resp.StatusCode
		if resp.StatusCode == http.StatusOK {
			last = contract.JobResponse{}
			decodeHTTPJSON(t, resp, &last)
			if last.Job.Prompt == wantPrompt {
				return last.Job
			}
		} else {
			_ = resp.Body.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf(
		"timed out waiting for automation job %q prompt %q (status=%d, last=%#v)",
		jobID,
		wantPrompt,
		lastStatus,
		last.Job,
	)
	return contract.JobPayload{}
}

func waitForAutomationJobMissing(t *testing.T, runtime integrationRuntime, jobID string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var lastStatus int
	for time.Now().Before(deadline) {
		resp := mustUnixRequest(t, runtime.client, http.MethodGet, "http://unix/api/automation/jobs/"+jobID, nil, nil)
		lastStatus = resp.StatusCode
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for automation job %q to be deleted (last status=%d)", jobID, lastStatus)
}

func waitForAutomationTriggerPrompt(
	t *testing.T,
	runtime integrationRuntime,
	triggerID string,
	wantPrompt string,
) contract.TriggerPayload {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var last contract.TriggerResponse
	var lastStatus int
	for time.Now().Before(deadline) {
		resp := mustUnixRequest(
			t,
			runtime.client,
			http.MethodGet,
			"http://unix/api/automation/triggers/"+triggerID,
			nil,
			nil,
		)
		lastStatus = resp.StatusCode
		if resp.StatusCode == http.StatusOK {
			last = contract.TriggerResponse{}
			decodeHTTPJSON(t, resp, &last)
			if last.Trigger.Prompt == wantPrompt {
				return last.Trigger
			}
		} else {
			_ = resp.Body.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf(
		"timed out waiting for automation trigger %q prompt %q (status=%d, last=%#v)",
		triggerID,
		wantPrompt,
		lastStatus,
		last.Trigger,
	)
	return contract.TriggerPayload{}
}

func waitForAutomationTriggerMissing(t *testing.T, runtime integrationRuntime, triggerID string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	var lastStatus int
	for time.Now().Before(deadline) {
		resp := mustUnixRequest(
			t,
			runtime.client,
			http.MethodGet,
			"http://unix/api/automation/triggers/"+triggerID,
			nil,
			nil,
		)
		lastStatus = resp.StatusCode
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusNotFound {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for automation trigger %q to be deleted (last status=%d)", triggerID, lastStatus)
}

func waitForProjectedToolRevision(t *testing.T, runtime integrationRuntime, want int64) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		revision, _ := runtime.toolCatalog.snapshot()
		if revision >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	revision, records := runtime.toolCatalog.snapshot()
	t.Fatalf("timed out waiting for projected tool revision %d (got %d, records=%#v)", want, revision, records)
}

type integrationTaskSessionExecutor struct {
	started int
}

func (e *integrationTaskSessionExecutor) StartTaskSession(
	_ context.Context,
	_ *taskpkg.StartTaskSession,
) (*taskpkg.SessionRef, error) {
	e.started++
	return &taskpkg.SessionRef{SessionID: fmt.Sprintf("task-sess-%d", e.started)}, nil
}

func (*integrationTaskSessionExecutor) AttachTaskSession(
	_ context.Context,
	_ string,
	sessionID string,
) (*taskpkg.SessionRef, error) {
	return &taskpkg.SessionRef{SessionID: sessionID}, nil
}

func (*integrationTaskSessionExecutor) RequestTaskStop(context.Context, string, taskpkg.StopReason) error {
	return nil
}

func (*integrationTaskSessionExecutor) ForceTaskStop(context.Context, string, taskpkg.StopReason) error {
	return nil
}

type integrationDreamTrigger struct {
	enabled   bool
	triggered bool
	reason    string
	last      time.Time
	calls     int
}

type integrationBridgeSecretStore interface {
	ListBridgeSecretBindings(context.Context, string) ([]bridgepkg.BridgeSecretBinding, error)
	PutBridgeSecretBinding(context.Context, bridgepkg.BridgeSecretBinding) error
	DeleteBridgeSecretBinding(context.Context, string, string) error
}

type integrationBridgeCatalogStore interface {
	CountBridgeRoutes(context.Context, []string) (map[string]int, error)
	ListBridgeSecretBindingsForInstances(
		context.Context,
		[]string,
	) (map[string][]bridgepkg.BridgeSecretBinding, error)
}

type integrationBridgeService struct {
	*bridgepkg.Service
	store             integrationBridgeSecretStore
	catalogStore      integrationBridgeCatalogStore
	taskSubscriptions bridgepkg.BridgeTaskSubscriptionStore
	providers         []bridgepkg.BridgeProvider
}

var _ core.BridgeService = (*integrationBridgeService)(nil)

func newIntegrationBridgeService(store bridgepkg.RegistryStore) *integrationBridgeService {
	secretStore, _ := store.(integrationBridgeSecretStore)
	catalogStore, _ := store.(integrationBridgeCatalogStore)
	taskSubscriptions, _ := store.(bridgepkg.BridgeTaskSubscriptionStore)
	return &integrationBridgeService{
		Service:           bridgepkg.NewRegistry(store),
		store:             secretStore,
		catalogStore:      catalogStore,
		taskSubscriptions: taskSubscriptions,
	}
}

func (s *integrationBridgeService) StartInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	if _, err := s.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:      id,
		Enabled: true,
		Status:  bridgepkg.BridgeStatusStarting,
	}); err != nil {
		return nil, fmt.Errorf("start bridge instance %q: %w", id, err)
	}
	instance, err := s.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:      id,
		Enabled: true,
		Status:  bridgepkg.BridgeStatusReady,
	})
	if err != nil {
		return nil, fmt.Errorf("mark bridge instance %q ready: %w", id, err)
	}
	return instance, nil
}

func (s *integrationBridgeService) StopInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	instance, err := s.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:      id,
		Enabled: false,
		Status:  bridgepkg.BridgeStatusDisabled,
	})
	if err != nil {
		return nil, fmt.Errorf("stop bridge instance %q: %w", id, err)
	}
	return instance, nil
}

func (s *integrationBridgeService) RestartInstance(ctx context.Context, id string) (*bridgepkg.BridgeInstance, error) {
	if _, err := s.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:      id,
		Enabled: true,
		Status:  bridgepkg.BridgeStatusStarting,
	}); err != nil {
		return nil, fmt.Errorf("restart bridge instance %q: %w", id, err)
	}
	instance, err := s.UpdateInstanceState(ctx, bridgepkg.UpdateInstanceStateRequest{
		ID:      id,
		Enabled: true,
		Status:  bridgepkg.BridgeStatusReady,
	})
	if err != nil {
		return nil, fmt.Errorf("mark restarted bridge instance %q ready: %w", id, err)
	}
	return instance, nil
}

func (s *integrationBridgeService) ListProviders(context.Context) ([]bridgepkg.BridgeProvider, error) {
	providers := make([]bridgepkg.BridgeProvider, 0, len(s.providers))
	providers = append(providers, s.providers...)
	return providers, nil
}

func (s *integrationBridgeService) CheckBridge(
	context.Context,
	string,
	bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	return bridgepkg.BridgeCheckResponse{}, bridgepkg.ErrBridgeControlTransportUnavailable
}

func (s *integrationBridgeService) RegisterBridgeWebhook(
	context.Context,
	string,
	bridgepkg.BridgeWebhookRegistrationRequest,
) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
	return bridgepkg.BridgeWebhookRegistrationResponse{}, bridgepkg.ErrBridgeControlTransportUnavailable
}

func (s *integrationBridgeService) DeliverBridge(
	context.Context,
	string,
	bridgepkg.DeliveryRequest,
) (bridgepkg.DeliveryAck, error) {
	return bridgepkg.DeliveryAck{}, bridgepkg.ErrDeliveryTransportUnavailable
}

func (s *integrationBridgeService) ListSecretBindings(
	ctx context.Context,
	bridgeInstanceID string,
) ([]bridgepkg.BridgeSecretBinding, error) {
	if s == nil || s.store == nil {
		return nil, errors.New("integration bridge secret store is not configured")
	}
	return s.store.ListBridgeSecretBindings(ctx, bridgeInstanceID)
}

func (s *integrationBridgeService) PutSecretBinding(
	ctx context.Context,
	binding bridgepkg.BridgeSecretBinding,
	_ *string,
) error {
	if s == nil || s.store == nil {
		return errors.New("integration bridge secret store is not configured")
	}
	return s.store.PutBridgeSecretBinding(ctx, binding)
}

func (s *integrationBridgeService) DeleteSecretBinding(
	ctx context.Context,
	bridgeInstanceID string,
	bindingName string,
) error {
	if s == nil || s.store == nil {
		return errors.New("integration bridge secret store is not configured")
	}
	return s.store.DeleteBridgeSecretBinding(ctx, bridgeInstanceID, bindingName)
}

func (s *integrationBridgeService) PutBridgeTaskSubscription(
	ctx context.Context,
	subscription bridgepkg.BridgeTaskSubscription,
) error {
	if s == nil || s.taskSubscriptions == nil {
		return errors.New("integration bridge task subscription store is not configured")
	}
	return s.taskSubscriptions.PutBridgeTaskSubscription(ctx, subscription)
}

func (s *integrationBridgeService) GetBridgeTaskSubscription(
	ctx context.Context,
	subscriptionID string,
) (bridgepkg.BridgeTaskSubscription, error) {
	if s == nil || s.taskSubscriptions == nil {
		return bridgepkg.BridgeTaskSubscription{}, errors.New(
			"integration bridge task subscription store is not configured",
		)
	}
	return s.taskSubscriptions.GetBridgeTaskSubscription(ctx, subscriptionID)
}

func (s *integrationBridgeService) ListBridgeTaskSubscriptions(
	ctx context.Context,
	query bridgepkg.BridgeTaskSubscriptionQuery,
) ([]bridgepkg.BridgeTaskSubscription, error) {
	if s == nil || s.taskSubscriptions == nil {
		return nil, errors.New("integration bridge task subscription store is not configured")
	}
	return s.taskSubscriptions.ListBridgeTaskSubscriptions(ctx, query)
}

func (s *integrationBridgeService) DeleteBridgeTaskSubscription(ctx context.Context, subscriptionID string) error {
	if s == nil || s.taskSubscriptions == nil {
		return errors.New("integration bridge task subscription store is not configured")
	}
	return s.taskSubscriptions.DeleteBridgeTaskSubscription(ctx, subscriptionID)
}

func (s *integrationBridgeService) CountBridgeRoutes(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (map[string]int, error) {
	if s == nil || s.catalogStore == nil {
		return nil, errors.New("integration bridge catalog store is not configured")
	}
	return s.catalogStore.CountBridgeRoutes(ctx, bridgeInstanceIDs)
}

func (s *integrationBridgeService) ListSecretBindingsForInstances(
	ctx context.Context,
	bridgeInstanceIDs []string,
) (map[string][]bridgepkg.BridgeSecretBinding, error) {
	if s == nil || s.catalogStore == nil {
		return nil, errors.New("integration bridge catalog store is not configured")
	}
	return s.catalogStore.ListBridgeSecretBindingsForInstances(ctx, bridgeInstanceIDs)
}

func (t *integrationDreamTrigger) Trigger(context.Context, string) (bool, string, error) {
	t.calls++
	return t.triggered, t.reason, nil
}

func (t *integrationDreamTrigger) LastConsolidatedAt() (time.Time, error) {
	return t.last, nil
}

func (t *integrationDreamTrigger) Enabled() bool {
	return t.enabled
}

type integrationNotifierFanout struct {
	notifiers []session.Notifier
}

func (f *integrationNotifierFanout) OnSessionCreated(ctx context.Context, sess *session.Session) {
	for _, notifier := range f.notifiers {
		notifier.OnSessionCreated(ctx, sess)
	}
}

func (f *integrationNotifierFanout) OnSessionStopped(ctx context.Context, sess *session.Session) {
	for _, notifier := range f.notifiers {
		notifier.OnSessionStopped(ctx, sess)
	}
}

func (f *integrationNotifierFanout) OnAgentEvent(ctx context.Context, sessionID string, event any) {
	for _, notifier := range f.notifiers {
		notifier.OnAgentEvent(ctx, sessionID, event)
	}
}

type integrationDriver struct {
	mu       sync.Mutex
	nextPID  int
	nextSess int
	states   map[*session.AgentProcess]chan struct{}
}

func newIntegrationDriver() *integrationDriver {
	return &integrationDriver{
		nextPID:  2000,
		nextSess: 1,
		states:   make(map[*session.AgentProcess]chan struct{}),
	}
}

func (d *integrationDriver) Start(_ context.Context, opts acp.StartOpts) (*session.AgentProcess, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.nextPID++
	d.nextSess++
	done := make(chan struct{})
	sessionID := strings.TrimSpace(opts.ResumeSessionID)
	if sessionID == "" {
		sessionID = fmt.Sprintf("acp-session-%d", d.nextSess)
	}

	proc := session.NewAgentProcess(session.AgentProcessOptions{
		PID:       d.nextPID,
		AgentName: opts.AgentName,
		Command:   opts.Command,
		Cwd:       opts.Cwd,
		SessionID: sessionID,
		Caps: acp.Caps{
			SupportsLoadSession: true,
		},
		StartedAt: time.Now().UTC(),
		Done:      done,
		Wait: func() error {
			<-done
			return nil
		},
	})
	d.states[proc] = done
	return proc, nil
}

func (d *integrationDriver) Prompt(
	_ context.Context,
	proc *session.AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	ch := make(chan acp.AgentEvent, 2)
	ch <- acp.AgentEvent{
		Type:      "agent_message",
		SessionID: proc.SessionID,
		TurnID:    req.TurnID,
		Timestamp: time.Now().UTC(),
		Text:      req.Message,
	}
	ch <- acp.AgentEvent{
		Type:       "done",
		SessionID:  proc.SessionID,
		TurnID:     req.TurnID,
		Timestamp:  time.Now().UTC(),
		StopReason: "end_turn",
	}
	close(ch)
	return ch, nil
}

func (d *integrationDriver) Cancel(context.Context, *session.AgentProcess) error {
	return nil
}

func (d *integrationDriver) Stop(_ context.Context, proc *session.AgentProcess) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	done, ok := d.states[proc]
	if !ok {
		return nil
	}
	select {
	case <-done:
	default:
		close(done)
	}
	delete(d.states, proc)
	return nil
}

func newIntegrationRuntime(t *testing.T) integrationRuntime {
	t.Helper()

	homePaths := newTestHomePaths(t)
	socketPath := shortSocketPath(t)
	writeAgentDef(t, homePaths, "coder")

	workspace := t.TempDir()
	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.Daemon.Socket = socketPath
	cfg.Network.Enabled = false
	cfg.Providers = map[string]aghconfig.ProviderConfig{
		"fake": {Command: "fake-agent"},
	}

	registry, err := globaldb.OpenGlobalDB(context.Background(), homePaths.DatabaseFile)
	if err != nil {
		t.Fatalf("OpenGlobalDB() error = %v", err)
	}
	t.Cleanup(func() {
		if err := registry.Close(context.Background()); err != nil {
			t.Fatalf("registry.Close() error = %v", err)
		}
	})
	resourceKernel, err := resources.NewKernel(registry.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}
	resourceActor := resources.MutationActor{
		Kind: resources.MutationActorKindDaemon,
		ID:   "uds-integration",
		Source: resources.ResourceSource{
			Kind: resources.ResourceSourceKind("daemon"),
			ID:   "uds-integration",
		},
		MaxScope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
	}
	resourceCodecs := resources.NewCodecRegistry()
	toolCodec, err := toolspkg.NewResourceCodec()
	if err != nil {
		t.Fatalf("toolspkg.NewResourceCodec() error = %v", err)
	}
	if err := resources.RegisterCodec(resourceCodecs, toolCodec); err != nil {
		t.Fatalf("resources.RegisterCodec(tool) error = %v", err)
	}
	jobCodec, err := automationpkg.NewJobResourceCodec()
	if err != nil {
		t.Fatalf("automation.NewJobResourceCodec() error = %v", err)
	}
	if err := resources.RegisterCodec(resourceCodecs, jobCodec); err != nil {
		t.Fatalf("resources.RegisterCodec(automation.job) error = %v", err)
	}
	jobResourceStore, err := resources.NewStore(resourceKernel, jobCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(automation.job) error = %v", err)
	}
	triggerCodec, err := automationpkg.NewTriggerResourceCodec()
	if err != nil {
		t.Fatalf("automation.NewTriggerResourceCodec() error = %v", err)
	}
	if err := resources.RegisterCodec(resourceCodecs, triggerCodec); err != nil {
		t.Fatalf("resources.RegisterCodec(automation.trigger) error = %v", err)
	}
	triggerResourceStore, err := resources.NewStore(resourceKernel, triggerCodec)
	if err != nil {
		t.Fatalf("resources.NewStore(automation.trigger) error = %v", err)
	}
	var resourceDriver resources.ReconcileDriver
	resourceTrigger := func(ctx context.Context, kind resources.ResourceKind, reason resources.ReconcileReason) error {
		switch kind.Normalize() {
		case toolspkg.ToolResourceKind, automationpkg.JobResourceKind, automationpkg.TriggerResourceKind:
		default:
			return nil
		}
		if resourceDriver == nil {
			return nil
		}
		return resourceDriver.Trigger(ctx, kind, reason)
	}

	fanout := &integrationNotifierFanout{}
	resolver, err := workspacepkg.NewResolver(
		registry,
		workspacepkg.WithHomePaths(homePaths),
		workspacepkg.WithLogger(discardLogger()),
		workspacepkg.WithConfigLoader(func(string) (aghconfig.Config, error) { return cfg, nil }),
	)
	if err != nil {
		t.Fatalf("workspace.NewResolver() error = %v", err)
	}
	sandboxRegistry, err := sandboxlocal.NewRegistry()
	if err != nil {
		t.Fatalf("local.NewRegistry() error = %v", err)
	}
	participationResolver := apitestutil.NewIntegrationParticipationResolver(t, registry)
	manager, err := session.NewManager(
		session.WithHomePaths(homePaths),
		session.WithWorkspaceResolver(resolver),
		session.WithLogger(discardLogger()),
		session.WithDriver(newIntegrationDriver()),
		session.WithNotifier(fanout),
		session.WithSandboxRegistry(sandboxRegistry),
		session.WithSessionCatalog(registry),
		session.WithParticipationResolver(participationResolver),
	)
	if err != nil {
		t.Fatalf("session.NewManager() error = %v", err)
	}
	windowManager, err := windowmanager.NewService(
		windowmanager.NewMemoryRepository(),
		windowmanager.NewMemoryWorkspaceResolver("window-manager-workspace"),
		nil,
		windowmanager.DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("windowmanager.NewService() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := windowManager.Close(); closeErr != nil {
			t.Fatalf("windowManager.Close() error = %v", closeErr)
		}
	})

	observer, err := observe.New(
		context.Background(),
		observe.WithHomePaths(homePaths),
		observe.WithRegistry(registry),
		observe.WithSessionSource(manager),
		observe.WithLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("observe.New() error = %v", err)
	}
	fanout.notifiers = append(fanout.notifiers, observer)

	memoryStore := memory.NewStore(
		homePaths.MemoryDir,
		memory.WithCatalogDatabasePath(homePaths.DatabaseFile),
	)
	if err := memoryStore.EnsureDirs(); err != nil {
		t.Fatalf("memoryStore.EnsureDirs() error = %v", err)
	}
	if err := memoryStore.OpenCatalog(context.Background()); err != nil {
		t.Fatalf("memoryStore.OpenCatalog() error = %v", err)
	}
	t.Cleanup(func() {
		if err := memoryStore.CloseCatalog(context.Background()); err != nil {
			t.Fatalf("memoryStore.CloseCatalog() error = %v", err)
		}
	})
	bridgeService := newIntegrationBridgeService(registry)
	dreamTrigger := &integrationDreamTrigger{
		enabled:   true,
		triggered: true,
		last:      time.Date(2026, 4, 4, 3, 30, 0, 0, time.UTC),
	}

	automationManager, err := automationpkg.New(
		automationpkg.WithStore(registry),
		automationpkg.WithSessions(manager),
		automationpkg.WithWorkspaceResolver(resolver),
		automationpkg.WithConfig(cfg.Automation),
		automationpkg.WithLogger(discardLogger()),
		automationpkg.WithGlobalWorkspacePath(homePaths.HomeDir),
		automationpkg.WithResourceDefinitions(jobResourceStore, triggerResourceStore, resourceActor, resourceTrigger),
	)
	if err != nil {
		t.Fatalf("automation.New() error = %v", err)
	}
	if err := automationManager.Start(context.Background()); err != nil {
		t.Fatalf("automationManager.Start() error = %v", err)
	}
	fanout.notifiers = append(fanout.notifiers, automationManager.SessionObserver())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := automationManager.Shutdown(ctx); err != nil {
			t.Fatalf("automationManager.Shutdown() error = %v", err)
		}
	})

	taskExecutor := &integrationTaskSessionExecutor{}
	taskManager, err := taskpkg.NewManager(
		taskpkg.WithStore(registry),
		taskpkg.WithSessionExecutor(taskExecutor),
		taskpkg.WithParticipationResolver(participationResolver),
	)
	if err != nil {
		t.Fatalf("task.NewManager() error = %v", err)
	}

	toolCatalog := &integrationToolCatalog{}
	toolRegistration, err := resources.NewTypedProjectorRegistration(
		toolCodec,
		&integrationToolProjector{catalog: toolCatalog},
	)
	if err != nil {
		t.Fatalf("resources.NewTypedProjectorRegistration(tool) error = %v", err)
	}
	jobRegistration, err := resources.NewTypedProjectorRegistration(
		jobCodec,
		&integrationAutomationJobProjector{manager: automationManager},
	)
	if err != nil {
		t.Fatalf("resources.NewTypedProjectorRegistration(automation.job) error = %v", err)
	}
	triggerRegistration, err := resources.NewTypedProjectorRegistration(
		triggerCodec,
		&integrationAutomationTriggerProjector{manager: automationManager},
	)
	if err != nil {
		t.Fatalf("resources.NewTypedProjectorRegistration(automation.trigger) error = %v", err)
	}
	resourceDriver, err = resources.NewReconcileDriver(
		resourceKernel,
		resourceActor,
		[]resources.ProjectorRegistration{toolRegistration, jobRegistration, triggerRegistration},
		resources.WithReconcileLogger(discardLogger()),
	)
	if err != nil {
		t.Fatalf("resources.NewReconcileDriver() error = %v", err)
	}
	t.Cleanup(func() {
		if err := resourceDriver.Close(context.Background()); err != nil {
			t.Fatalf("resourceDriver.Close() error = %v", err)
		}
	})
	resourceService, err := core.NewOperatorResourceService(&core.ResourceServiceConfig{
		RawStore:      resourceKernel,
		CodecRegistry: resourceCodecs,
		Trigger:       resourceTrigger,
	})
	if err != nil {
		t.Fatalf("core.NewOperatorResourceService() error = %v", err)
	}

	server, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithSocketPath(socketPath),
		WithLogger(discardLogger()),
		WithSessionManager(manager),
		WithSessionCatalog(registry),
		WithTaskService(taskManager),
		WithObserver(observer),
		WithResourceService(resourceService),
		WithAutomation(automationManager),
		WithBridgeService(bridgeService),
		WithWorkspaceResolver(resolver),
		WithMemoryStore(memoryStore),
		WithDreamTrigger(dreamTrigger),
		WithWindowManagerService(windowManager),
		WithPollInterval(10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("udsapi.New() error = %v", err)
	}
	if err := server.Start(context.Background()); err != nil {
		t.Fatalf("server.Start() error = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			t.Fatalf("server.Shutdown() error = %v", err)
		}
	})

	return integrationRuntime{
		client:         newUnixClient(t, socketPath),
		server:         server,
		manager:        manager,
		tasks:          taskManager,
		observer:       observer,
		registry:       registry,
		bridges:        bridgeService,
		memory:         memoryStore,
		dream:          dreamTrigger,
		resourceDriver: resourceDriver,
		toolCatalog:    toolCatalog,
		socket:         socketPath,
		workspace:      workspace,
	}
}

func createIntegrationSession(t *testing.T, runtime integrationRuntime) string {
	t.Helper()

	return createIntegrationSessionPayload(t, runtime).ID
}

func createIntegrationSessionPayload(t *testing.T, runtime integrationRuntime) sessionPayload {
	t.Helper()
	return createIntegrationSessionPayloadFromRequest(t, runtime, map[string]any{
		"agent_name":     "coder",
		"workspace_path": runtime.workspace,
	})
}

func createLiveIntegrationSessionPayload(t *testing.T, runtime integrationRuntime) sessionPayload {
	t.Helper()
	return createIntegrationSessionPayloadFromRequest(t, runtime, map[string]any{
		"agent_name":     "coder",
		"workspace_path": runtime.workspace,
		"network_participation": map[string]any{
			"mode":             "live",
			"channel_strategy": "named",
			"channel_id":       "builders",
		},
	})
}

func createIntegrationSessionPayloadFromRequest(
	t *testing.T,
	runtime integrationRuntime,
	request map[string]any,
) sessionPayload {
	t.Helper()

	body, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("json.Marshal(create session body) error = %v", err)
	}

	resp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/sessions",
		body,
		nil,
	)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("create session status = %d, want %d; body=%s", resp.StatusCode, http.StatusCreated, string(body))
	}
	var created struct {
		Session sessionPayload `json:"session"`
	}
	decodeHTTPJSON(t, resp, &created)
	if created.Session.ID == "" {
		t.Fatal("expected created session id")
	}
	if created.Session.WorkspaceID == "" {
		t.Fatal("expected created session workspace id")
	}
	return created.Session
}

func createIntegrationTask(t *testing.T, runtime integrationRuntime, body []byte) contract.TaskPayload {
	t.Helper()

	resp := mustUnixRequest(t, runtime.client, http.MethodPost, "http://unix/api/tasks", body, nil)
	if resp.StatusCode != http.StatusCreated {
		payload, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("create task status = %d, want %d; body=%s", resp.StatusCode, http.StatusCreated, string(payload))
	}
	var created contract.TaskResponse
	decodeHTTPJSON(t, resp, &created)
	return created.Task
}

func enqueueIntegrationTaskRun(
	t *testing.T,
	runtime integrationRuntime,
	taskID string,
	body string,
) contract.TaskRunPayload {
	t.Helper()

	resp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		"http://unix/api/tasks/"+taskID+"/runs",
		[]byte(body),
		nil,
	)
	if resp.StatusCode != http.StatusCreated {
		payload, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("enqueue run status = %d, want %d; body=%s", resp.StatusCode, http.StatusCreated, string(payload))
	}
	var created contract.TaskRunResponse
	decodeHTTPJSON(t, resp, &created)
	return created.Run
}

func seedIntegrationTaskRunClaimed(t *testing.T, runtime integrationRuntime, runID string) {
	t.Helper()

	ctx := context.Background()
	run, err := runtime.registry.GetTaskRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetTaskRun(%q) error = %v", runID, err)
	}
	run.Status = taskpkg.TaskRunStatusClaimed
	run.ClaimedBy = &taskpkg.ActorIdentity{Kind: taskpkg.ActorKindHuman, Ref: "local-user"}
	run.ClaimedAt = time.Now().UTC()
	if err := runtime.registry.UpdateTaskRun(ctx, run); err != nil {
		t.Fatalf("UpdateTaskRun(%q) error = %v", runID, err)
	}
}

func requireUDSInboxGroup(
	t *testing.T,
	groups []contract.TaskInboxLaneGroupPayload,
	lane contract.TaskInboxLane,
) contract.TaskInboxLaneGroupPayload {
	t.Helper()

	for _, group := range groups {
		if group.Lane == lane {
			return group
		}
	}
	t.Fatalf("task inbox lane %q not found in %#v", lane, groups)
	return contract.TaskInboxLaneGroupPayload{}
}

func udsInboxGroupHasTask(group contract.TaskInboxLaneGroupPayload, taskID string) bool {
	for _, item := range group.Items {
		if item.Task.ID == taskID {
			return true
		}
	}
	return false
}

func sessionAPIPath(workspaceID string, sessionID string, suffix string) string {
	return fmt.Sprintf("http://unix/api/workspaces/%s/sessions/%s%s", workspaceID, sessionID, suffix)
}

func sendPrompt(t *testing.T, runtime integrationRuntime, workspaceID string, sessionID string, message string) {
	t.Helper()

	body, err := json.Marshal(map[string]string{"message": message})
	if err != nil {
		t.Fatalf("json.Marshal(prompt body) error = %v", err)
	}

	resp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		sessionAPIPath(workspaceID, sessionID, "/prompt"),
		body,
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("prompt status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, string(body))
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatalf("io.ReadAll(prompt response) error = %v", err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("prompt response close error = %v", err)
	}
}

func mustIntegrationPrompt(t *testing.T, events <-chan acp.AgentEvent, err error) <-chan acp.AgentEvent {
	t.Helper()
	if err != nil {
		t.Fatalf("prompt submission error = %v", err)
	}
	return events
}

func collectIntegrationPromptEvents(
	t *testing.T,
	events <-chan acp.AgentEvent,
	timeout time.Duration,
) []acp.AgentEvent {
	t.Helper()

	collected := make([]acp.AgentEvent, 0, 4)
	timer := time.NewTimer(timeout)
	defer timer.Stop()

Loop:
	for {
		select {
		case event, ok := <-events:
			if !ok {
				break Loop
			}
			collected = append(collected, event)
		case <-timer.C:
			t.Fatalf("timed out waiting for prompt events after %v", timeout)
		}
	}
	if len(collected) == 0 {
		t.Fatal("prompt events = 0, want completed prompt stream")
	}
	if got := collected[len(collected)-1].Type; got != acp.EventTypeDone {
		t.Fatalf("last prompt event type = %q, want %q", got, acp.EventTypeDone)
	}
	return collected
}

func stopIntegrationSession(t *testing.T, runtime integrationRuntime, workspaceID string, sessionID string) {
	t.Helper()

	resp := mustUnixRequest(
		t,
		runtime.client,
		http.MethodPost,
		sessionAPIPath(workspaceID, sessionID, "/stop"),
		nil,
		nil,
	)
	if resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("stop status = %d, want %d; body=%s", resp.StatusCode, http.StatusNoContent, string(body))
	}
	_ = resp.Body.Close()
}

func mustUnixRequest(
	t *testing.T,
	client *http.Client,
	method, url string,
	body []byte,
	headers map[string]string,
) *http.Response {
	t.Helper()

	var reader io.Reader
	if len(body) > 0 {
		reader = strings.NewReader(string(body))
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	return resp
}

func decodeHTTPJSON(t *testing.T, resp *http.Response, dest any) {
	t.Helper()

	body, err := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err != nil {
		t.Fatalf("io.ReadAll(response) error = %v", err)
	}
	if closeErr != nil {
		t.Fatalf("response body close error = %v", closeErr)
	}
	if err := json.Unmarshal(body, dest); err != nil {
		t.Fatalf("json.Unmarshal(response) error = %v; body=%s", err, string(body))
	}
}

func readAndCloseHTTPBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()

	body, readErr := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		t.Fatalf("read and close HTTP response body: %v", err)
	}
	return body
}

func closeHTTPBody(t *testing.T, body io.Closer) {
	t.Helper()

	if err := body.Close(); err != nil {
		t.Fatalf("close HTTP response body: %v", err)
	}
}

func containsAutomationRun(runs []contract.RunPayload, id string) bool {
	for _, run := range runs {
		if run.ID == id {
			return true
		}
	}
	return false
}

func collectLiveSSE(t *testing.T, body io.ReadCloser, want int, timeout time.Duration) []sseRecord {
	t.Helper()

	records := make([]sseRecord, 0, want)
	recordCh := make(chan sseRecord, want+1)
	errCh := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(body)
		current := sseRecord{}
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				recordCh <- current
				current = sseRecord{}
				continue
			}
			switch {
			case strings.HasPrefix(line, "id: "):
				current.ID = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "event: "):
				current.Event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				current.Data = append(current.Data, []byte(strings.TrimPrefix(line, "data: "))...)
			}
		}
		if current.Event != "" || current.ID != "" || len(current.Data) > 0 {
			recordCh <- current
		}
		if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
			errCh <- err
			return
		}
		close(recordCh)
	}()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for len(records) < want {
		select {
		case record, ok := <-recordCh:
			if !ok {
				return records
			}
			records = append(records, record)
		case err := <-errCh:
			t.Fatalf("SSE scanner error = %v", err)
		case <-deadline.C:
			t.Fatalf("timed out waiting for %d SSE events; got %d", want, len(records))
		}
	}

	return records
}

func collectLiveSSEUntilEvent(t *testing.T, body io.ReadCloser, wantEvent string, timeout time.Duration) []sseRecord {
	t.Helper()

	records := make([]sseRecord, 0, 4)
	recordCh := make(chan sseRecord, 8)
	errCh := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(body)
		current := sseRecord{}
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				recordCh <- current
				current = sseRecord{}
				continue
			}
			switch {
			case strings.HasPrefix(line, "id: "):
				current.ID = strings.TrimPrefix(line, "id: ")
			case strings.HasPrefix(line, "event: "):
				current.Event = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				current.Data = append(current.Data, []byte(strings.TrimPrefix(line, "data: "))...)
			}
		}
		if current.Event != "" || current.ID != "" || len(current.Data) > 0 {
			recordCh <- current
		}
		if err := scanner.Err(); err != nil && !errors.Is(err, os.ErrClosed) {
			errCh <- err
			return
		}
		close(recordCh)
	}()

	deadline := time.NewTimer(timeout)
	defer deadline.Stop()

	for {
		select {
		case record, ok := <-recordCh:
			if !ok {
				t.Fatalf("stream ended before event %q; got %d records", wantEvent, len(records))
			}
			records = append(records, record)
			if record.Event == wantEvent {
				return records
			}
		case err := <-errCh:
			t.Fatalf("SSE scanner error = %v", err)
		case <-deadline.C:
			t.Fatalf("timed out waiting for SSE event %q; got %d records", wantEvent, len(records))
		}
	}
}
