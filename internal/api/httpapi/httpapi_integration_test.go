//go:build integration

package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/acp"
	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	apitestutil "github.com/compozy/agh/internal/api/testutil"
	automationpkg "github.com/compozy/agh/internal/automation"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/memory"
	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/observe"
	"github.com/compozy/agh/internal/resources"
	sandboxlocal "github.com/compozy/agh/internal/sandbox/local"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/store/globaldb"
	taskpkg "github.com/compozy/agh/internal/task"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	"github.com/compozy/agh/internal/transcript"
	vaultpkg "github.com/compozy/agh/internal/vault"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestHTTPFullRoundTripWithRealSessionManager(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	indexResp := mustHTTPRequest(t, runtime.client, http.MethodGet, mustURL(runtime.host, runtime.port, "/"), nil, nil)
	if indexResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(indexResp.Body)
		_ = indexResp.Body.Close()
		t.Fatalf("root status = %d, want %d; body=%s", indexResp.StatusCode, http.StatusOK, string(body))
	}
	indexBody, err := io.ReadAll(indexResp.Body)
	_ = indexResp.Body.Close()
	if err != nil {
		t.Fatalf("io.ReadAll(root) error = %v", err)
	}
	if !strings.Contains(string(indexBody), `<div id="app"></div>`) {
		t.Fatalf("root body = %q, want SPA shell", string(indexBody))
	}

	deepLinkResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/session/demo"),
		nil,
		nil,
	)
	if deepLinkResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(deepLinkResp.Body)
		_ = deepLinkResp.Body.Close()
		t.Fatalf("deep link status = %d, want %d; body=%s", deepLinkResp.StatusCode, http.StatusOK, string(body))
	}
	deepLinkBody, err := io.ReadAll(deepLinkResp.Body)
	_ = deepLinkResp.Body.Close()
	if err != nil {
		t.Fatalf("io.ReadAll(deep link) error = %v", err)
	}
	if !strings.Contains(string(deepLinkBody), `<div id="app"></div>`) {
		t.Fatalf("deep link body = %q, want SPA shell", string(deepLinkBody))
	}

	statusResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/status"),
		nil,
		nil,
	)
	if statusResp.StatusCode != http.StatusOK {
		t.Fatalf("daemon status = %d, want %d", statusResp.StatusCode, http.StatusOK)
	}
	var statusPayload contract.StatusPayload
	decodeHTTPJSON(t, statusResp, &statusPayload)
	if statusPayload.Daemon.PID <= 0 ||
		statusPayload.Daemon.HTTPHost != runtime.host ||
		statusPayload.Daemon.HTTPPort != runtime.port {
		t.Fatalf(
			"daemon status payload = %#v, want populated daemon fields for %s:%d",
			statusPayload,
			runtime.host,
			runtime.port,
		)
	}

	origin := fmt.Sprintf("http://%s:%d", runtime.host, runtime.port)
	createResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/sessions"),
		[]byte(`{"agent_name":"coder","name":"demo","workspace_path":"`+runtime.workspace+`"}`),
		map[string]string{"Origin": origin},
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
	if got := createResp.Header.Get("Access-Control-Allow-Origin"); got != origin {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, origin)
	}
	var created struct {
		Session sessionPayload `json:"session"`
	}
	decodeHTTPJSON(t, createResp, &created)
	if created.Session.ID == "" {
		t.Fatal("expected created session id")
	}

	listResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/sessions"),
		nil,
		nil,
	)
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
	canonicalWorkspace, err := filepath.EvalSymlinks(runtime.workspace)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(%q) error = %v", runtime.workspace, err)
	}
	if listed.Sessions[0].WorkspaceID == "" || listed.Sessions[0].WorkspacePath != canonicalWorkspace {
		t.Fatalf("listed session workspace = %#v", listed.Sessions[0])
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

	filteredResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/sessions?workspace="+created.Session.WorkspaceID),
		nil,
		nil,
	)
	if filteredResp.StatusCode != http.StatusOK {
		t.Fatalf("filtered sessions status = %d, want %d", filteredResp.StatusCode, http.StatusOK)
	}
	var filtered struct {
		Sessions []sessionPayload `json:"sessions"`
	}
	decodeHTTPJSON(t, filteredResp, &filtered)
	if len(filtered.Sessions) != 1 || filtered.Sessions[0].ID != created.Session.ID {
		t.Fatalf("filtered sessions = %#v", filtered.Sessions)
	}

	promptResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+created.Session.ID+"/prompt"),
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
	if len(promptEvents) < 6 {
		t.Fatalf("prompt SSE events = %d, want at least 6; body=%s", len(promptEvents), string(promptBody))
	}
	if promptEvents[len(promptEvents)-1].Event != "" || string(promptEvents[len(promptEvents)-1].Data) != "[DONE]" {
		t.Fatalf("last prompt record = %#v, want [DONE]", promptEvents[len(promptEvents)-1])
	}

	eventsResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+created.Session.ID+"/events"),
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
	if len(events.Events) < 4 {
		t.Fatalf("persisted session events = %d, want at least 4", len(events.Events))
	}
}

func TestHTTPPromptPersistsTerminalEventsAfterClientDisconnect(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	sessionID := createIntegrationSession(t, runtime)

	requestCtx, cancel := context.WithCancel(context.Background())
	req, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/prompt"),
		strings.NewReader(`{"message":"hello"}`),
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := runtime.client.Do(req)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("prompt status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, string(body))
	}

	scanner := bufio.NewScanner(resp.Body)
	seenToolStart := false
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		if payload == "" || payload == "[DONE]" {
			continue
		}

		var part struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal([]byte(payload), &part); err != nil {
			continue
		}
		if part.Type == "tool-input-start" {
			seenToolStart = true
			break
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner.Err() error = %v", err)
	}
	if !seenToolStart {
		t.Fatal("tool-input-start was not observed before client disconnect")
	}

	cancel()
	_ = resp.Body.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		events, err := runtime.manager.Events(context.Background(), sessionID, store.EventQuery{})
		if err == nil && integrationPromptEventsContainTerminalToolEvents(events) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	events, err := runtime.manager.Events(context.Background(), sessionID, store.EventQuery{})
	if err != nil {
		t.Fatalf("Events() error after disconnect = %v", err)
	}
	t.Fatalf("persisted events after disconnect = %#v, want tool_result and done", events)
}

func TestHTTPPromptRejectsConcurrentRequestWithConflictAndNoGhostInput(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	sessionID := createIntegrationSession(t, runtime)

	firstPromptEntered := make(chan struct{})
	releaseFirstPrompt := make(chan struct{})
	runtime.driver.promptHook = func(proc *session.AgentProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		events := make(chan acp.AgentEvent)
		go func() {
			defer close(events)
			if req.Message != "first prompt" {
				return
			}

			close(firstPromptEntered)
			<-releaseFirstPrompt

			ts := time.Now().UTC()
			events <- acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: proc.SessionID,
				TurnID:    req.TurnID,
				Timestamp: ts,
				Text:      "first prompt reply",
			}
			events <- acp.AgentEvent{
				Type:             acp.EventTypeDone,
				SessionID:        proc.SessionID,
				TurnID:           req.TurnID,
				Timestamp:        ts,
				StopReason:       string(acp.PromptStopReasonEndTurn),
				PromptStopReason: acp.PromptStopReasonEndTurn,
			}
		}()
		return events, nil
	}

	type promptResult struct {
		resp *http.Response
		err  error
	}
	firstResultCh := make(chan promptResult, 1)
	go func() {
		req, err := http.NewRequest(
			http.MethodPost,
			mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/prompt"),
			strings.NewReader(`{"message":"first prompt"}`),
		)
		if err != nil {
			firstResultCh <- promptResult{err: err}
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := runtime.client.Do(req)
		firstResultCh <- promptResult{resp: resp, err: err}
	}()

	<-firstPromptEntered

	secondResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/prompt"),
		[]byte(`{"message":"second prompt"}`),
		nil,
	)
	if secondResp.StatusCode != http.StatusConflict {
		body, _ := io.ReadAll(secondResp.Body)
		_ = secondResp.Body.Close()
		t.Fatalf(
			"second prompt status = %d, want %d; body=%s",
			secondResp.StatusCode,
			http.StatusConflict,
			string(body),
		)
	}
	var secondErr contract.ErrorPayload
	decodeHTTPJSON(t, secondResp, &secondErr)
	if !strings.Contains(secondErr.Error, "prompt already in progress") {
		t.Fatalf("second prompt error = %q, want prompt already in progress", secondErr.Error)
	}

	eventsWhileBusy, err := runtime.manager.Events(context.Background(), sessionID, store.EventQuery{})
	if err != nil {
		t.Fatalf("Events(while busy) error = %v", err)
	}
	if got, want := countSessionEventsByType(eventsWhileBusy, acp.EventTypeUserMessage), 1; got != want {
		t.Fatalf("countSessionEventsByType(user_message) while busy = %d, want %d", got, want)
	}

	close(releaseFirstPrompt)

	firstResult := <-firstResultCh
	if firstResult.err != nil {
		t.Fatalf("first prompt request error = %v", firstResult.err)
	}
	if firstResult.resp == nil {
		t.Fatal("first prompt response = nil")
	}
	if firstResult.resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(firstResult.resp.Body)
		_ = firstResult.resp.Body.Close()
		t.Fatalf("first prompt status = %d, want %d; body=%s", firstResult.resp.StatusCode, http.StatusOK, string(body))
	}
	firstBody, err := io.ReadAll(firstResult.resp.Body)
	_ = firstResult.resp.Body.Close()
	if err != nil {
		t.Fatalf("io.ReadAll(first prompt) error = %v", err)
	}
	firstEvents := parseSSE(t, string(firstBody))
	if len(firstEvents) == 0 {
		t.Fatalf("first prompt SSE events = 0; body=%s", string(firstBody))
	}
	if firstEvents[len(firstEvents)-1].Event != "" || string(firstEvents[len(firstEvents)-1].Data) != "[DONE]" {
		t.Fatalf("last first prompt record = %#v, want [DONE]", firstEvents[len(firstEvents)-1])
	}

	eventsAfterRelease, err := runtime.manager.Events(context.Background(), sessionID, store.EventQuery{})
	if err != nil {
		t.Fatalf("Events(after release) error = %v", err)
	}
	if got, want := countSessionEventsByType(eventsAfterRelease, acp.EventTypeUserMessage), 1; got != want {
		t.Fatalf("countSessionEventsByType(user_message) after release = %d, want %d", got, want)
	}
}

func TestHTTPSessionTranscriptEndpointWithRealSessionManager(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	sessionID := createIntegrationSession(t, runtime)
	sendPrompt(t, runtime, sessionID, "hello")

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/transcript?limit=1"),
		nil,
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			t.Fatalf("read transcript error body: %v", readErr)
		}
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Fatalf("close transcript error body: %v", closeErr)
		}
		t.Fatalf("transcript status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, string(body))
	}

	var tail contract.SessionTranscriptResponse
	decodeHTTPJSON(t, resp, &tail)
	if len(tail.Entries) != 1 || !tail.HasOlder || tail.NextBeforeSequence == 0 || tail.Limit != 1 {
		t.Fatalf("tail page = %#v, want one entry and an older cursor", tail)
	}
	olderResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(
			runtime.host,
			runtime.port,
			"/api/workspaces/ws-workspace/sessions/"+sessionID+"/transcript?limit=1&before_sequence="+
				strconv.FormatInt(tail.NextBeforeSequence, 10),
		),
		nil,
		nil,
	)
	if olderResp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(olderResp.Body)
		if readErr != nil {
			t.Fatalf("read older transcript error body: %v", readErr)
		}
		if closeErr := olderResp.Body.Close(); closeErr != nil {
			t.Fatalf("close older transcript error body: %v", closeErr)
		}
		t.Fatalf("older transcript status = %d, want %d; body=%s", olderResp.StatusCode, http.StatusOK, string(body))
	}
	var older contract.SessionTranscriptResponse
	decodeHTTPJSON(t, olderResp, &older)
	if len(older.Entries) != 1 || older.HasOlder || older.Limit != 1 {
		t.Fatalf("older page = %#v, want final one-entry page", older)
	}
	if older.Epoch != tail.Epoch || older.Generation != tail.Generation || older.MaxSequence != tail.MaxSequence {
		t.Fatalf("page identity changed across walk: older=%#v tail=%#v", older, tail)
	}
	if older.Entries[0].StartSequence >= tail.Entries[0].StartSequence {
		t.Fatalf("page cursors overlap or regress: older=%#v tail=%#v", older.Entries, tail.Entries)
	}
	entries := append(append([]transcript.Entry(nil), older.Entries...), tail.Entries...)
	messages := transcript.MessagesFromEntries(entries)
	if len(messages) != 2 {
		t.Fatalf("len(messages) = %d, want 2", len(messages))
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
	if !httpTranscriptHasToolPart(messages[1]) {
		t.Fatalf("messages[1] = %#v, want assistant tool part", messages[1])
	}
}

func TestHTTPSessionTranscriptEndpointIncludesSyntheticTurns(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	sessionID := createLiveIntegrationSession(t, runtime)

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

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/transcript"),
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
	if got := messages[2].Role; got != transcript.UIRoleUser {
		t.Fatalf("messages[2].Role = %q, want %q", got, transcript.UIRoleUser)
	}
	if got := transcript.UIMessageText(messages[2]); got != "network hello" {
		t.Fatalf("messages[2] text = %q, want %q", got, "network hello")
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
	if !httpTranscriptHasToolPart(messages[5]) {
		t.Fatalf("messages[5] = %#v, want assistant tool part", messages[5])
	}
}

func httpTranscriptHasToolPart(message transcript.UIMessage) bool {
	for _, part := range message.Parts {
		if strings.HasPrefix(part.Type, "tool-") || part.Type == "dynamic-tool" {
			return true
		}
	}
	return false
}

func TestHTTPResourceMutationRoutesRemainUnavailableWithoutOperatorAuth(t *testing.T) {
	runtime := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{})

	putResp := mustHTTPRequest(
		t,
		runtime.HTTPClient,
		http.MethodPut,
		runtime.HTTPURL("/api/resources/bundle.activation/demo"),
		[]byte(`{"scope":{"kind":"global"},"spec":{"enabled":true}}`),
		nil,
	)
	defer func() { _ = putResp.Body.Close() }()
	if putResp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(putResp.Body)
		t.Fatalf("PUT status = %d, want %d; body=%s", putResp.StatusCode, http.StatusNotFound, string(body))
	}

	deleteResp := mustHTTPRequest(
		t,
		runtime.HTTPClient,
		http.MethodDelete,
		runtime.HTTPURL("/api/resources/bundle.activation/demo"),
		[]byte(`{"expected_version":1}`),
		nil,
	)
	defer func() { _ = deleteResp.Body.Close() }()
	if deleteResp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(deleteResp.Body)
		t.Fatalf("DELETE status = %d, want %d; body=%s", deleteResp.StatusCode, http.StatusNotFound, string(body))
	}
}

func TestHTTPSessionStreamReconnectsWithLastEventID(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	sessionID := createIntegrationSession(t, runtime)
	sendPrompt(t, runtime, sessionID, "hello")
	stopIntegrationSession(t, runtime, sessionID)

	streamResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/stream?frames=raw"),
		nil,
		nil,
	)
	if streamResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(streamResp.Body)
		_ = streamResp.Body.Close()
		t.Fatalf("session stream status = %d, want %d; body=%s", streamResp.StatusCode, http.StatusOK, string(body))
	}
	initial := collectLiveSSE(t, streamResp.Body, 6, 2*time.Second)
	_ = streamResp.Body.Close()
	if len(initial) < 6 {
		t.Fatalf("initial stream events = %d, want 6", len(initial))
	}
	if initial[len(initial)-1].Event != session.EventTypeSessionStopped {
		t.Fatalf("last event = %q, want %q", initial[len(initial)-1].Event, session.EventTypeSessionStopped)
	}

	headers := map[string]string{"Last-Event-ID": initial[0].ID}
	replayResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/stream?frames=raw"),
		nil,
		headers,
	)
	if replayResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(replayResp.Body)
		_ = replayResp.Body.Close()
		t.Fatalf("replay stream status = %d, want %d; body=%s", replayResp.StatusCode, http.StatusOK, string(body))
	}
	replayed := collectLiveSSE(t, replayResp.Body, 5, 2*time.Second)
	_ = replayResp.Body.Close()
	if len(replayed) < 5 {
		t.Fatalf("replayed events = %d, want 5", len(replayed))
	}
	if replayed[0].ID != initial[1].ID {
		t.Fatalf("replayed first id = %q, want %q", replayed[0].ID, initial[1].ID)
	}
}

func TestHTTPSessionStreamReconnectPreservesCursorWhenNoNewEventsExistYet(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	sessionID := createIntegrationSession(t, runtime)

	firstEventDelivered := make(chan struct{})
	releaseRemaining := make(chan struct{})
	runtime.driver.promptHook = func(proc *session.AgentProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error) {
		events := make(chan acp.AgentEvent)
		go func() {
			defer close(events)

			ts := time.Now().UTC()
			events <- acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: proc.SessionID,
				TurnID:    req.TurnID,
				Timestamp: ts,
				Text:      "first chunk",
			}
			close(firstEventDelivered)

			<-releaseRemaining

			ts = time.Now().UTC()
			events <- acp.AgentEvent{
				Type:      acp.EventTypeAgentMessage,
				SessionID: proc.SessionID,
				TurnID:    req.TurnID,
				Timestamp: ts,
				Text:      "second chunk",
			}
			events <- acp.AgentEvent{
				Type:             acp.EventTypeDone,
				SessionID:        proc.SessionID,
				TurnID:           req.TurnID,
				Timestamp:        ts,
				StopReason:       string(acp.PromptStopReasonEndTurn),
				PromptStopReason: acp.PromptStopReasonEndTurn,
			}
		}()
		return events, nil
	}

	type promptRequestResult struct {
		resp *http.Response
		err  error
	}
	promptResultCh := make(chan promptRequestResult, 1)
	go func() {
		req, err := http.NewRequest(
			http.MethodPost,
			mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/prompt"),
			strings.NewReader(`{"message":"hello"}`),
		)
		if err != nil {
			promptResultCh <- promptRequestResult{err: err}
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := runtime.client.Do(req)
		promptResultCh <- promptRequestResult{resp: resp, err: err}
	}()

	<-firstEventDelivered

	streamResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/stream?frames=raw"),
		nil,
		nil,
	)
	if streamResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(streamResp.Body)
		_ = streamResp.Body.Close()
		t.Fatalf("session stream status = %d, want %d; body=%s", streamResp.StatusCode, http.StatusOK, string(body))
	}
	initial := collectLiveSSE(t, streamResp.Body, 2, 2*time.Second)
	_ = streamResp.Body.Close()
	if len(initial) != 2 {
		t.Fatalf("initial stream events = %d, want 2", len(initial))
	}
	lastEventID := initial[len(initial)-1].ID
	if lastEventID == "" {
		t.Fatal("initial last event id is empty")
	}

	replayResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/stream?frames=raw"),
		nil,
		map[string]string{"Last-Event-ID": lastEventID},
	)
	if replayResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(replayResp.Body)
		_ = replayResp.Body.Close()
		t.Fatalf("replay stream status = %d, want %d; body=%s", replayResp.StatusCode, http.StatusOK, string(body))
	}

	time.Sleep(100 * time.Millisecond)
	close(releaseRemaining)

	replayed := collectLiveSSE(t, replayResp.Body, 2, 2*time.Second)
	_ = replayResp.Body.Close()
	if len(replayed) != 2 {
		t.Fatalf("replayed events = %d, want 2", len(replayed))
	}
	if replayed[0].ID != "3" || replayed[1].ID != "4" {
		t.Fatalf("replayed ids = [%q %q], want [\"3\" \"4\"]", replayed[0].ID, replayed[1].ID)
	}
	if got, err := strconv.ParseInt(replayed[0].ID, 10, 64); err != nil {
		t.Fatalf("strconv.ParseInt(replayed[0].ID) error = %v", err)
	} else if got <= 2 {
		t.Fatalf("replayed first sequence = %d, want > 2", got)
	}

	promptRequest := <-promptResultCh
	if promptRequest.err != nil {
		t.Fatalf("prompt request error = %v", promptRequest.err)
	}
	if promptRequest.resp == nil {
		t.Fatal("prompt response = nil")
	}
	body, err := io.ReadAll(promptRequest.resp.Body)
	_ = promptRequest.resp.Body.Close()
	if err != nil {
		t.Fatalf("io.ReadAll(prompt SSE) error = %v", err)
	}
	events := parseSSE(t, string(body))
	if len(events) == 0 {
		t.Fatalf("prompt SSE events = 0; body=%s", string(body))
	}
	if events[len(events)-1].Event != "" || string(events[len(events)-1].Data) != "[DONE]" {
		t.Fatalf("last prompt record = %#v, want [DONE]", events[len(events)-1])
	}
}

func TestHTTPSessionStopReasonPropagatesToGlobalDBAndAPI(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	sessionID := createIntegrationSession(t, runtime)

	stopIntegrationSession(t, runtime, sessionID)

	sessions, err := runtime.registry.ListSessions(context.Background(), store.SessionListQuery{State: "stopped"})
	if err != nil {
		t.Fatalf("runtime.registry.ListSessions() error = %v", err)
	}
	if got, want := len(sessions), 1; got != want {
		t.Fatalf("len(stopped sessions) = %d, want %d", got, want)
	}
	if sessions[0].ID != sessionID {
		t.Fatalf("sessions[0].ID = %q, want %q", sessions[0].ID, sessionID)
	}
	if sessions[0].StopReason != store.StopUserCanceled {
		t.Fatalf("sessions[0].StopReason = %q, want %q", sessions[0].StopReason, store.StopUserCanceled)
	}

	listResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/sessions"),
		nil,
		nil,
	)
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
	if listed.Sessions[0].StopReason != store.StopUserCanceled {
		t.Fatalf("listed.Sessions[0].StopReason = %q, want %q", listed.Sessions[0].StopReason, store.StopUserCanceled)
	}

	statusResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID),
		nil,
		nil,
	)
	if statusResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(statusResp.Body)
		_ = statusResp.Body.Close()
		t.Fatalf("status session response = %d, want %d; body=%s", statusResp.StatusCode, http.StatusOK, string(body))
	}
	var detail struct {
		Session sessionPayload `json:"session"`
	}
	decodeHTTPJSON(t, statusResp, &detail)
	if detail.Session.ID != sessionID {
		t.Fatalf("detail.Session.ID = %q, want %q", detail.Session.ID, sessionID)
	}
	if detail.Session.StopReason != store.StopUserCanceled {
		t.Fatalf("detail.Session.StopReason = %q, want %q", detail.Session.StopReason, store.StopUserCanceled)
	}
}

func TestHTTPSessionParticipationRoundTrip(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	apitestutil.EnableIntegrationLiveNetwork(t, runtime.registry)

	createResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/sessions"),
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

	listResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/sessions"),
		nil,
		nil,
	)
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

	stopIntegrationSession(t, runtime, created.Session.ID)

	statusResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+created.Session.ID),
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

	indexed, err := runtime.registry.ListSessions(context.Background(), store.SessionListQuery{State: "stopped"})
	if err != nil {
		t.Fatalf("runtime.registry.ListSessions() error = %v", err)
	}
	if got, want := len(indexed), 1; got != want {
		t.Fatalf("len(indexed stopped sessions) = %d, want %d", got, want)
	}
	indexedParticipation := indexed[0].NetworkSpecSnapshot()
	apitestutil.AssertResolvedParticipationChannel(
		t,
		&indexedParticipation,
		created.Session.WorkspaceID,
		"builders",
		participation.SourceExplicitRequest,
	)

	resumeResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+created.Session.ID+"/attach"),
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

func TestHTTPSessionCrashStopReasonPropagatesToGlobalDBAndAPI(t *testing.T) {
	runtime := newIntegrationRuntime(t)
	sessionID := createIntegrationSession(t, runtime)

	sess, ok := runtime.manager.Get(sessionID)
	if !ok {
		t.Fatalf("manager.Get(%q) = missing, want active session", sessionID)
	}
	if err := runtime.driver.Crash(sess.Info().ACPSessionID, errors.New("integration crash")); err != nil {
		t.Fatalf("driver.Crash() error = %v", err)
	}

	waitForRegistryStopReason(t, runtime, sessionID, store.StopAgentCrashed)

	meta, err := store.ReadSessionMeta(sess.MetaPath())
	if err != nil {
		t.Fatalf("ReadSessionMeta(%q) error = %v", sess.MetaPath(), err)
	}
	if meta.StopReason == nil {
		t.Fatal("meta.StopReason = nil, want non-nil")
	}
	if *meta.StopReason != store.StopAgentCrashed {
		t.Fatalf("meta.StopReason = %q, want %q", *meta.StopReason, store.StopAgentCrashed)
	}

	listResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/sessions"),
		nil,
		nil,
	)
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
	if listed.Sessions[0].StopReason != store.StopAgentCrashed {
		t.Fatalf("listed.Sessions[0].StopReason = %q, want %q", listed.Sessions[0].StopReason, store.StopAgentCrashed)
	}

	statusResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID),
		nil,
		nil,
	)
	if statusResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(statusResp.Body)
		_ = statusResp.Body.Close()
		t.Fatalf("status session response = %d, want %d; body=%s", statusResp.StatusCode, http.StatusOK, string(body))
	}
	var detail struct {
		Session sessionPayload `json:"session"`
	}
	decodeHTTPJSON(t, statusResp, &detail)
	if detail.Session.StopReason != store.StopAgentCrashed {
		t.Fatalf("detail.Session.StopReason = %q, want %q", detail.Session.StopReason, store.StopAgentCrashed)
	}
}

func TestHTTPApprovePermissionFullFlow(t *testing.T) {
	runtime := newIntegrationRuntimeWithPermissionWait(t, 250*time.Millisecond)
	sessionID := createIntegrationSession(t, runtime)

	promptResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/prompt"),
		[]byte(`{"message":"request permission"}`),
		nil,
	)
	if promptResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(promptResp.Body)
		_ = promptResp.Body.Close()
		t.Fatalf("prompt status = %d, want %d; body=%s", promptResp.StatusCode, http.StatusOK, string(body))
	}

	requestIDCh := make(chan string, 1)
	resultCh := make(chan []sseRecord, 1)
	go streamPermissionPrompt(t, promptResp.Body, requestIDCh, resultCh)

	var requestID string
	select {
	case requestID = <-requestIDCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for permission request id from SSE stream")
	}
	if requestID == "" {
		t.Fatal("permission request_id = empty, want non-empty")
	}

	approveResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/approve"),
		[]byte(fmt.Sprintf(`{"request_id":"%s","decision":"allow-always"}`, requestID)),
		nil,
	)
	if approveResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(approveResp.Body)
		_ = approveResp.Body.Close()
		t.Fatalf("approve status = %d, want %d; body=%s", approveResp.StatusCode, http.StatusOK, string(body))
	}
	_ = approveResp.Body.Close()

	var records []sseRecord
	select {
	case records = <-resultCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for completed SSE stream")
	}

	permissionPayloads := extractPermissionPayloads(t, records)
	if len(permissionPayloads) < 2 {
		t.Fatalf("permission payloads = %#v, want initial and final permission events", permissionPayloads)
	}
	if permissionPayloads[0].RequestID != requestID || permissionPayloads[0].Decision != "" {
		t.Fatalf("initial permission payload = %#v", permissionPayloads[0])
	}
	if permissionPayloads[len(permissionPayloads)-1].Decision != "allow-always" {
		t.Fatalf("final permission payload = %#v, want allow-always", permissionPayloads[len(permissionPayloads)-1])
	}
	if !recordsContainTextDelta(records, "allow-always") {
		t.Fatalf("records = %#v, want allow-always text delta", records)
	}
}

func TestHTTPApprovePermissionTimeout(t *testing.T) {
	runtime := newIntegrationRuntimeWithPermissionWait(t, 25*time.Millisecond)
	sessionID := createIntegrationSession(t, runtime)

	promptResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/prompt"),
		[]byte(`{"message":"request permission"}`),
		nil,
	)
	if promptResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(promptResp.Body)
		_ = promptResp.Body.Close()
		t.Fatalf("prompt status = %d, want %d; body=%s", promptResp.StatusCode, http.StatusOK, string(body))
	}

	requestIDCh := make(chan string, 1)
	resultCh := make(chan []sseRecord, 1)
	go streamPermissionPrompt(t, promptResp.Body, requestIDCh, resultCh)

	select {
	case <-requestIDCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for permission request id from SSE stream")
	}

	var records []sseRecord
	select {
	case records = <-resultCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for completed SSE stream")
	}

	permissionPayloads := extractPermissionPayloads(t, records)
	if len(permissionPayloads) < 2 {
		t.Fatalf("permission payloads = %#v, want initial and final permission events", permissionPayloads)
	}
	if permissionPayloads[len(permissionPayloads)-1].Decision != "reject-once" {
		t.Fatalf("final permission payload = %#v, want reject-once", permissionPayloads[len(permissionPayloads)-1])
	}
	if !recordsContainTextDelta(records, "reject-once") {
		t.Fatalf("records = %#v, want reject-once text delta", records)
	}
}

func TestHTTPMemoryRoundTripAndDelete(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	writeResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/memory"),
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

	readResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/memory/"+targetFilename+"?scope=global"),
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

	listResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/memory?scope=global"),
		nil,
		nil,
	)
	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		_ = listResp.Body.Close()
		t.Fatalf("list status = %d, want %d; body=%s", listResp.StatusCode, http.StatusOK, string(body))
	}
	var listPayload memoryListResponse
	decodeHTTPJSON(t, listResp, &listPayload)
	if len(listPayload.Memories) != 1 || listPayload.Memories[0].Filename != targetFilename {
		t.Fatalf("memories = %#v, want %s", listPayload.Memories, targetFilename)
	}

	deleteResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodDelete,
		mustURL(runtime.host, runtime.port, "/api/memory/"+targetFilename+"?scope=global"),
		nil,
		nil,
	)
	if deleteResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(deleteResp.Body)
		_ = deleteResp.Body.Close()
		t.Fatalf("delete status = %d, want %d; body=%s", deleteResp.StatusCode, http.StatusOK, string(body))
	}
	_ = deleteResp.Body.Close()

	emptyList := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/memory?scope=global"),
		nil,
		nil,
	)
	if emptyList.StatusCode != http.StatusOK {
		t.Fatalf("post-delete list status = %d, want %d", emptyList.StatusCode, http.StatusOK)
	}
	decodeHTTPJSON(t, emptyList, &listPayload)
	if len(listPayload.Memories) != 0 {
		t.Fatalf("memories = %#v, want empty list after delete", listPayload.Memories)
	}
}

func TestHTTPMemoryDreamTriggerIntegration(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/memory/dreams/trigger"),
		mustIntegrationJSON(map[string]any{
			"scope":        "workspace",
			"workspace_id": runtime.workspace,
		}),
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("dream trigger status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, string(body))
	}

	var payload memoryDreamTriggerResponse
	decodeHTTPJSON(t, resp, &payload)
	if !payload.Triggered || runtime.dream.calls != 1 || runtime.dream.recordedWorkspace != runtime.workspace {
		t.Fatalf(
			"payload = %#v dream.calls=%d workspace=%q, want triggered once for %q",
			payload,
			runtime.dream.calls,
			runtime.dream.recordedWorkspace,
			runtime.workspace,
		)
	}
	if payload.Dream.Scope != memcontract.ScopeWorkspace || payload.Dream.WorkspaceID != runtime.workspace {
		t.Fatalf("dream payload = %#v, want workspace-scoped dream for %q", payload.Dream, runtime.workspace)
	}
}

func TestHTTPAutomationJobsRoundTrip(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	createResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/automation/jobs"),
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
	if created.Job.Source != automationpkg.JobSourceDynamic {
		t.Fatalf("created job source = %q, want %q", created.Job.Source, automationpkg.JobSourceDynamic)
	}

	getResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/automation/jobs/"+created.Job.ID),
		nil,
		nil,
	)
	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		_ = getResp.Body.Close()
		t.Fatalf("get job status = %d, want %d; body=%s", getResp.StatusCode, http.StatusOK, string(body))
	}
	var fetched contract.JobResponse
	decodeHTTPJSON(t, getResp, &fetched)
	if fetched.Job.NextRun == nil {
		t.Fatalf("expected next_run for fetched job: %#v", fetched.Job)
	}

	listResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/automation/jobs?scope=global&source=dynamic"),
		nil,
		nil,
	)
	if listResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(listResp.Body)
		_ = listResp.Body.Close()
		t.Fatalf("list jobs status = %d, want %d; body=%s", listResp.StatusCode, http.StatusOK, string(body))
	}
	var listed contract.JobsResponse
	decodeHTTPJSON(t, listResp, &listed)
	if len(listed.Jobs) != 1 || listed.Jobs[0].ID != created.Job.ID {
		t.Fatalf("listed jobs = %#v", listed.Jobs)
	}

	updateResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPatch,
		mustURL(runtime.host, runtime.port, "/api/automation/jobs/"+created.Job.ID),
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

	triggerResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/automation/jobs/"+created.Job.ID+"/trigger"),
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
	if run.Run.ID == "" || run.Run.JobID != created.Job.ID {
		t.Fatalf("trigger run = %#v", run.Run)
	}

	jobRunsResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/automation/jobs/"+created.Job.ID+"/runs"),
		nil,
		nil,
	)
	if jobRunsResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(jobRunsResp.Body)
		_ = jobRunsResp.Body.Close()
		t.Fatalf("job runs status = %d, want %d; body=%s", jobRunsResp.StatusCode, http.StatusOK, string(body))
	}
	var jobRuns contract.RunsResponse
	decodeHTTPJSON(t, jobRunsResp, &jobRuns)
	if !containsAutomationRun(jobRuns.Runs, run.Run.ID) {
		t.Fatalf("job run history missing %q: %#v", run.Run.ID, jobRuns.Runs)
	}

	runsResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/automation/runs?job_id="+created.Job.ID),
		nil,
		nil,
	)
	if runsResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(runsResp.Body)
		_ = runsResp.Body.Close()
		t.Fatalf("list runs status = %d, want %d; body=%s", runsResp.StatusCode, http.StatusOK, string(body))
	}
	var runs contract.RunsResponse
	decodeHTTPJSON(t, runsResp, &runs)
	if !containsAutomationRun(runs.Runs, run.Run.ID) {
		t.Fatalf("runs list missing %q: %#v", run.Run.ID, runs.Runs)
	}

	runResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/automation/runs/"+run.Run.ID),
		nil,
		nil,
	)
	if runResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(runResp.Body)
		_ = runResp.Body.Close()
		t.Fatalf("get run status = %d, want %d; body=%s", runResp.StatusCode, http.StatusOK, string(body))
	}
	var fetchedRun contract.RunResponse
	decodeHTTPJSON(t, runResp, &fetchedRun)
	if fetchedRun.Run.ID != run.Run.ID || fetchedRun.Run.JobID != created.Job.ID {
		t.Fatalf("fetched run = %#v", fetchedRun.Run)
	}

	deleteResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodDelete,
		mustURL(runtime.host, runtime.port, "/api/automation/jobs/"+created.Job.ID),
		nil,
		nil,
	)
	if deleteResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(deleteResp.Body)
		_ = deleteResp.Body.Close()
		t.Fatalf("delete job status = %d, want %d; body=%s", deleteResp.StatusCode, http.StatusNoContent, string(body))
	}
	_ = deleteResp.Body.Close()

	emptyResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/automation/jobs"),
		nil,
		nil,
	)
	if emptyResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(emptyResp.Body)
		_ = emptyResp.Body.Close()
		t.Fatalf("final list jobs status = %d, want %d; body=%s", emptyResp.StatusCode, http.StatusOK, string(body))
	}
	var empty contract.JobsResponse
	decodeHTTPJSON(t, emptyResp, &empty)
	if len(empty.Jobs) != 0 {
		t.Fatalf("expected no remaining jobs, got %#v", empty.Jobs)
	}
}

func TestHTTPAutomationTriggersWebhookAndHealth(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	createResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/automation/triggers"),
		[]byte(
			`{"scope":"global","name":"deploy-review","agent_name":"coder","prompt":"review {{ index .Data \"payload\" }}","event":"webhook","endpoint_slug":"deploy-review","webhook_secret_value":"shared-secret"}`,
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
	createBody, err := io.ReadAll(createResp.Body)
	_ = createResp.Body.Close()
	if err != nil {
		t.Fatalf("io.ReadAll(create trigger response) error = %v", err)
	}
	if strings.Contains(string(createBody), "shared-secret") {
		t.Fatalf("create trigger response leaked webhook secret: %s", string(createBody))
	}
	var created contract.TriggerResponse
	if err := json.Unmarshal(createBody, &created); err != nil {
		t.Fatalf("json.Unmarshal(create trigger response) error = %v; body=%s", err, string(createBody))
	}
	if created.Trigger.ID == "" || created.Trigger.WebhookID == "" {
		t.Fatalf("created trigger = %#v", created.Trigger)
	}

	endpoint, err := automationpkg.FormatWebhookEndpoint(created.Trigger.EndpointSlug, created.Trigger.WebhookID)
	if err != nil {
		t.Fatalf("FormatWebhookEndpoint() error = %v", err)
	}

	getResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/automation/triggers/"+created.Trigger.ID),
		nil,
		nil,
	)
	if getResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(getResp.Body)
		_ = getResp.Body.Close()
		t.Fatalf("get trigger status = %d, want %d; body=%s", getResp.StatusCode, http.StatusOK, string(body))
	}
	getBody, err := io.ReadAll(getResp.Body)
	_ = getResp.Body.Close()
	if err != nil {
		t.Fatalf("io.ReadAll(get trigger response) error = %v", err)
	}
	if strings.Contains(string(getBody), "shared-secret") {
		t.Fatalf("get trigger response leaked webhook secret: %s", string(getBody))
	}
	var fetched contract.TriggerResponse
	if err := json.Unmarshal(getBody, &fetched); err != nil {
		t.Fatalf("json.Unmarshal(get trigger response) error = %v; body=%s", err, string(getBody))
	}
	if fetched.Trigger.EndpointSlug != "deploy-review" {
		t.Fatalf("fetched trigger endpoint_slug = %q, want %q", fetched.Trigger.EndpointSlug, "deploy-review")
	}

	updateResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPatch,
		mustURL(runtime.host, runtime.port, "/api/automation/triggers/"+created.Trigger.ID),
		[]byte(`{"prompt":"triage {{ index .Data \"payload\" }}"}`),
		nil,
	)
	if updateResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(updateResp.Body)
		_ = updateResp.Body.Close()
		t.Fatalf("update trigger status = %d, want %d; body=%s", updateResp.StatusCode, http.StatusOK, string(body))
	}
	updateBody, err := io.ReadAll(updateResp.Body)
	_ = updateResp.Body.Close()
	if err != nil {
		t.Fatalf("io.ReadAll(update trigger response) error = %v", err)
	}
	if strings.Contains(string(updateBody), "shared-secret") {
		t.Fatalf("update trigger response leaked webhook secret: %s", string(updateBody))
	}
	var updated contract.TriggerResponse
	if err := json.Unmarshal(updateBody, &updated); err != nil {
		t.Fatalf("json.Unmarshal(update trigger response) error = %v; body=%s", err, string(updateBody))
	}
	if updated.Trigger.Prompt != `triage {{ index .Data "payload" }}` {
		t.Fatalf("updated trigger prompt = %q", updated.Trigger.Prompt)
	}

	payload := []byte(`{"payload":"deploy"}`)
	timestamp := time.Now().UTC()
	invalidResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/webhooks/global/"+endpoint),
		payload,
		map[string]string{
			core.WebhookTimestampHeader:  timestamp.Format(time.RFC3339),
			core.WebhookSignatureHeader:  "sha256=deadbeef",
			core.WebhookDeliveryIDHeader: "delivery-invalid",
		},
	)
	if invalidResp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(invalidResp.Body)
		_ = invalidResp.Body.Close()
		t.Fatalf(
			"invalid webhook status = %d, want %d; body=%s",
			invalidResp.StatusCode,
			http.StatusUnauthorized,
			string(body),
		)
	}
	var invalidPayload contract.ErrorPayload
	decodeHTTPJSON(t, invalidResp, &invalidPayload)
	if !strings.Contains(strings.ToLower(invalidPayload.Error), "signature") {
		t.Fatalf("invalid webhook error = %#v, want signature failure detail", invalidPayload)
	}

	signature, err := automationpkg.SignWebhookPayload("shared-secret", timestamp, payload)
	if err != nil {
		t.Fatalf("SignWebhookPayload() error = %v", err)
	}
	validResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/webhooks/global/"+endpoint),
		payload,
		map[string]string{
			core.WebhookTimestampHeader:  timestamp.Format(time.RFC3339),
			core.WebhookSignatureHeader:  signature,
			core.WebhookDeliveryIDHeader: "delivery-valid",
		},
	)
	if validResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(validResp.Body)
		_ = validResp.Body.Close()
		t.Fatalf("valid webhook status = %d, want %d; body=%s", validResp.StatusCode, http.StatusOK, string(body))
	}
	var delivery contract.WebhookDeliveryResponse
	decodeHTTPJSON(t, validResp, &delivery)
	if delivery.Result.Matched != 1 || len(delivery.Result.Runs) != 1 {
		t.Fatalf("webhook delivery = %#v", delivery.Result)
	}

	runID := delivery.Result.Runs[0].ID
	triggerRunsResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/automation/triggers/"+created.Trigger.ID+"/runs"),
		nil,
		nil,
	)
	if triggerRunsResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(triggerRunsResp.Body)
		_ = triggerRunsResp.Body.Close()
		t.Fatalf("trigger runs status = %d, want %d; body=%s", triggerRunsResp.StatusCode, http.StatusOK, string(body))
	}
	var triggerRuns contract.RunsResponse
	decodeHTTPJSON(t, triggerRunsResp, &triggerRuns)
	if !containsAutomationRun(triggerRuns.Runs, runID) {
		t.Fatalf("trigger run history missing %q: %#v", runID, triggerRuns.Runs)
	}

	runResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/automation/runs/"+runID),
		nil,
		nil,
	)
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

	healthResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/status"),
		nil,
		nil,
	)
	if healthResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(healthResp.Body)
		_ = healthResp.Body.Close()
		t.Fatalf("health status = %d, want %d; body=%s", healthResp.StatusCode, http.StatusOK, string(body))
	}
	var health contract.StatusPayload
	decodeHTTPJSON(t, healthResp, &health)
	if !health.Automation.Enabled || !health.Automation.SchedulerRunning {
		t.Fatalf("automation health = %#v", health.Automation)
	}
	if health.Automation.Triggers.Total != 1 || health.Automation.Triggers.Enabled != 1 {
		t.Fatalf("automation trigger health = %#v", health.Automation.Triggers)
	}

	deleteResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodDelete,
		mustURL(runtime.host, runtime.port, "/api/automation/triggers/"+created.Trigger.ID),
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

func TestHTTPShutdownWaitsForInflightRequests(t *testing.T) {
	homePaths := newTestHomePaths(t)
	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = freeTCPPort(t)

	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	server, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithHost(cfg.HTTP.Host),
		WithPort(cfg.HTTP.Port),
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

	client := &http.Client{}
	respCh := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := client.Get(mustURL(cfg.HTTP.Host, server.Port(), "/api/sessions"))
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

func TestHTTPShutdownCancelsPersistentStreams(t *testing.T) {
	t.Run("Should cancel an active session catalog stream before the shutdown deadline", func(t *testing.T) {
		t.Parallel()

		homePaths := newTestHomePaths(t)
		cfg := aghconfig.DefaultWithHome(homePaths)
		cfg.HTTP.Host = "127.0.0.1"
		cfg.HTTP.Port = freeTCPPort(t)
		events := make(chan session.CatalogEvent)
		server, err := New(
			WithHomePaths(homePaths),
			WithConfig(&cfg),
			WithHost(cfg.HTTP.Host),
			WithPort(cfg.HTTP.Port),
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

		streamResponse := mustHTTPRequest(
			t,
			&http.Client{},
			http.MethodGet,
			mustURL(cfg.HTTP.Host, server.Port(), "/api/sessions/catalog-stream"),
			nil,
			nil,
		)
		if streamResponse.StatusCode != http.StatusOK {
			body, readErr := io.ReadAll(streamResponse.Body)
			closeErr := streamResponse.Body.Close()
			if responseErr := errors.Join(readErr, closeErr); responseErr != nil {
				t.Fatalf("read stream error response: %v", responseErr)
			}
			t.Fatalf("stream status = %d, want %d; body=%s", streamResponse.StatusCode, http.StatusOK, body)
		}
		if contentType := streamResponse.Header.Get("Content-Type"); contentType != "text/event-stream" {
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

func TestHTTPTaskRoutesRoundTrip(t *testing.T) {
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
	if created.Origin.Kind != taskpkg.OriginKindHTTP {
		t.Fatalf("created origin.kind = %q, want %q", created.Origin.Kind, taskpkg.OriginKindHTTP)
	}
	if created.CreatedBy.Ref != "local-user" {
		t.Fatalf("created created_by.ref = %q, want %q", created.CreatedBy.Ref, "local-user")
	}
	if got := strings.TrimSpace(string(created.Metadata)); got != `{"priority":"high"}` {
		t.Fatalf("created metadata = %s, want %s", got, `{"priority":"high"}`)
	}

	listResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(
			runtime.host,
			runtime.port,
			"/api/tasks?scope=global&status=ready&owner_kind=pool&owner_ref=ops",
		),
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

	getResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+created.ID),
		nil,
		nil,
	)
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

	updateResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPatch,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+created.ID),
		[]byte(`{
		"title":"Ship task routes now",
		"description":"Expose the task and run transports everywhere",
		"clear_owner":true
	}`),
		nil,
	)
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

	updatedListResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/tasks?scope=global&status=ready"),
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

func TestHTTPTaskRunLifecycleRoutesRoundTrip(t *testing.T) {
	t.Run("Should enqueue claim start and complete a task run", func(t *testing.T) {
		t.Parallel()

		runtime := newIntegrationRuntime(t)
		created := createIntegrationTask(
			t,
			runtime,
			[]byte(`{"scope":"workspace","workspace":"ws-workspace","title":"Run task routes"}`),
		)

		enqueueResp := mustHTTPRequest(
			t,
			runtime.client,
			http.MethodPost,
			mustURL(runtime.host, runtime.port, "/api/tasks/"+created.ID+"/runs"),
			[]byte(`{"idempotency_key":"enqueue-1"}`),
			nil,
		)
		if enqueueResp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(enqueueResp.Body)
			_ = enqueueResp.Body.Close()
			t.Fatalf(
				"enqueue run status = %d, want %d; body=%s",
				enqueueResp.StatusCode,
				http.StatusCreated,
				string(body),
			)
		}
		var queued contract.TaskRunResponse
		decodeHTTPJSON(t, enqueueResp, &queued)
		if queued.Run.Status != taskpkg.TaskRunStatusQueued {
			t.Fatalf("queued status = %q, want %q", queued.Run.Status, taskpkg.TaskRunStatusQueued)
		}
		apitestutil.AssertLocalResolvedParticipation(t, queued.Run.ResolvedNetworkParticipation)

		listQueuedResp := mustHTTPRequest(
			t,
			runtime.client,
			http.MethodGet,
			mustURL(runtime.host, runtime.port, "/api/tasks/"+created.ID+"/runs?status=queued&limit=1"),
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
		if len(queuedList.Runs) != 1 || queuedList.Runs[0].ID != queued.Run.ID {
			t.Fatalf("queued runs = %#v, want queued run", queuedList.Runs)
		}

		seedIntegrationTaskRunClaimed(t, runtime, queued.Run.ID)
		claimedRun, err := runtime.registry.GetTaskRun(context.Background(), queued.Run.ID)
		if err != nil {
			t.Fatalf("GetTaskRun(%q) error = %v", queued.Run.ID, err)
		}
		claimed := core.TaskRunPayloadFromRun(&claimedRun)
		if claimed.Status != taskpkg.TaskRunStatusClaimed {
			t.Fatalf("claimed status = %q, want %q", claimed.Status, taskpkg.TaskRunStatusClaimed)
		}
		if claimed.ClaimedBy == nil || claimed.ClaimedBy.Ref != "local-user" {
			t.Fatalf("claimed claimed_by = %#v, want local-user", claimed.ClaimedBy)
		}

		startResp := mustHTTPRequest(
			t,
			runtime.client,
			http.MethodPost,
			mustURL(runtime.host, runtime.port, "/api/task-runs/"+queued.Run.ID+"/start"),
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

		completeResp := mustHTTPRequest(
			t,
			runtime.client,
			http.MethodPost,
			mustURL(runtime.host, runtime.port, "/api/task-runs/"+queued.Run.ID+"/complete"),
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
	})

	t.Run("Should attach a claimed run session and then fail it", func(t *testing.T) {
		t.Parallel()

		runtime := newIntegrationRuntime(t)
		created := createIntegrationTask(
			t,
			runtime,
			[]byte(`{"scope":"workspace","workspace":"ws-workspace","title":"Run task routes"}`),
		)
		run := enqueueIntegrationTaskRun(t, runtime, created.ID, `{"idempotency_key":"enqueue-2"}`)
		seedIntegrationTaskRunClaimed(t, runtime, run.ID)

		attachResp := mustHTTPRequest(
			t,
			runtime.client,
			http.MethodPost,
			mustURL(runtime.host, runtime.port, "/api/task-runs/"+run.ID+"/attach-session"),
			[]byte(`{"session_id":"sess-resume-1"}`),
			nil,
		)
		if attachResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(attachResp.Body)
			_ = attachResp.Body.Close()
			t.Fatalf(
				"attach run session status = %d, want %d; body=%s",
				attachResp.StatusCode,
				http.StatusOK,
				string(body),
			)
		}
		var attached contract.TaskRunResponse
		decodeHTTPJSON(t, attachResp, &attached)
		if attached.Run.Status != taskpkg.TaskRunStatusStarting {
			t.Fatalf("attached status = %q, want %q", attached.Run.Status, taskpkg.TaskRunStatusStarting)
		}
		if attached.Run.SessionID != "sess-resume-1" {
			t.Fatalf("attached session_id = %q, want %q", attached.Run.SessionID, "sess-resume-1")
		}

		failResp := mustHTTPRequest(
			t,
			runtime.client,
			http.MethodPost,
			mustURL(runtime.host, runtime.port, "/api/task-runs/"+run.ID+"/fail"),
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
	})

	t.Run("Should cancel one queued task run", func(t *testing.T) {
		t.Parallel()

		runtime := newIntegrationRuntime(t)
		created := createIntegrationTask(
			t,
			runtime,
			[]byte(`{"scope":"workspace","workspace":"ws-workspace","title":"Run task routes"}`),
		)
		run := enqueueIntegrationTaskRun(t, runtime, created.ID, `{"idempotency_key":"enqueue-3"}`)

		cancelResp := mustHTTPRequest(
			t,
			runtime.client,
			http.MethodPost,
			mustURL(runtime.host, runtime.port, "/api/task-runs/"+run.ID+"/cancel"),
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

		finalRunsResp := mustHTTPRequest(
			t,
			runtime.client,
			http.MethodGet,
			mustURL(runtime.host, runtime.port, "/api/tasks/"+created.ID+"/runs"),
			nil,
			nil,
		)
		if finalRunsResp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(finalRunsResp.Body)
			_ = finalRunsResp.Body.Close()
			t.Fatalf(
				"final list runs status = %d, want %d; body=%s",
				finalRunsResp.StatusCode,
				http.StatusOK,
				string(body),
			)
		}
		var finalRuns contract.TaskRunsResponse
		decodeHTTPJSON(t, finalRunsResp, &finalRuns)
		if len(finalRuns.Runs) != 1 || finalRuns.Runs[0].Status != taskpkg.TaskRunStatusCanceled {
			t.Fatalf("final runs = %#v, want one cancelled run", finalRuns.Runs)
		}
	})
}

func TestHTTPTaskPublishRunDetailAndLiveRoutesRoundTrip(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	draft := createIntegrationTask(t, runtime, []byte(`{
		"scope":"global",
		"title":"Draft live task routes",
		"draft":true
	}`))
	if draft.Status != taskpkg.TaskStatusDraft {
		t.Fatalf("draft status = %q, want %q", draft.Status, taskpkg.TaskStatusDraft)
	}

	publishResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+draft.ID+"/publish"),
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

	startResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/task-runs/"+run.ID+"/start"),
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

	runDetailResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/task-runs/"+run.ID),
		nil,
		nil,
	)
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

	timelineResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+draft.ID+"/timeline?limit=20"),
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

	treeResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+draft.ID+"/tree"),
		nil,
		nil,
	)
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

	streamResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+draft.ID+"/stream?after_sequence=0"),
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

func TestHTTPTaskDashboardInboxApprovalAndTriageRoutesRoundTrip(t *testing.T) {
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

	dashboardResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/observe/tasks/dashboard"),
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

	inboxResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/observe/tasks/inbox?lane=approvals&limit=10"),
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
	approvalsGroup := requireHTTPInboxGroup(t, inbox.Inbox.Groups, contract.TaskInboxLaneApprovals)
	if approvalsGroup.Count < 2 {
		t.Fatalf("approvals count = %d, want at least 2", approvalsGroup.Count)
	}
	if !httpInboxGroupHasTask(approvalsGroup, approvalTask.ID) ||
		!httpInboxGroupHasTask(approvalsGroup, rejectTask.ID) {
		t.Fatalf("approvals group items = %#v, want approval and reject tasks", approvalsGroup.Items)
	}

	approveBody := []byte(`{"idempotency_key":"approve-approval-task"}`)
	approveResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+approvalTask.ID+"/approve"),
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

	approveAgainResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+approvalTask.ID+"/approve"),
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

	rejectResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+rejectTask.ID+"/reject"),
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

	readResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+triageTask.ID+"/triage/read"),
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

	archiveResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+triageTask.ID+"/triage/archive"),
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

	dismissResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+dismissTask.ID+"/triage/dismiss"),
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

	readMissingResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/tasks/task-missing/triage/read"),
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

	inboxAfterResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/observe/tasks/inbox?lane=approvals&limit=10"),
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
	if got := requireHTTPInboxGroup(t, inboxAfter.Inbox.Groups, contract.TaskInboxLaneApprovals).Count; got != 0 {
		t.Fatalf("approvals count after approve/reject = %d, want 0", got)
	}

	archivedInboxResp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodGet,
		mustURL(runtime.host, runtime.port, "/api/observe/tasks/inbox?lane=archived&limit=10"),
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
	if !httpInboxGroupHasTask(
		requireHTTPInboxGroup(t, archivedInbox.Inbox.Groups, contract.TaskInboxLaneArchived),
		triageTask.ID,
	) {
		t.Fatalf("archived inbox groups = %#v, want task %q", archivedInbox.Inbox.Groups, triageTask.ID)
	}
}

type integrationRuntime struct {
	client    *http.Client
	server    *Server
	manager   *session.Manager
	tasks     *taskpkg.Service
	driver    *integrationDriver
	observer  *observe.Observer
	registry  *globaldb.GlobalDB
	bridges   *integrationBridgeService
	memory    *memory.Store
	dream     *integrationDreamTrigger
	host      string
	port      int
	workspace string
}

type integrationTaskSessionExecutor struct {
	mu      sync.Mutex
	started int
}

func (e *integrationTaskSessionExecutor) StartTaskSession(
	_ context.Context,
	_ *taskpkg.StartTaskSession,
) (*taskpkg.SessionRef, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
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
	enabled           bool
	triggered         bool
	reason            string
	last              time.Time
	calls             int
	recordedWorkspace string
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
	broker            *bridgepkg.Broker
	providers         []bridgepkg.BridgeProvider
}

var _ core.BridgeService = (*integrationBridgeService)(nil)

func newIntegrationBridgeService(store bridgepkg.RegistryStore) *integrationBridgeService {
	taskSubscriptions, _ := store.(bridgepkg.BridgeTaskSubscriptionStore)
	catalogStore, _ := store.(integrationBridgeCatalogStore)
	return &integrationBridgeService{
		Service:           bridgepkg.NewRegistry(store),
		store:             bridgeSecretStore(store),
		catalogStore:      catalogStore,
		taskSubscriptions: taskSubscriptions,
		broker:            bridgepkg.NewBroker(nil),
	}
}

func bridgeSecretStore(store bridgepkg.RegistryStore) integrationBridgeSecretStore {
	secretStore, _ := store.(integrationBridgeSecretStore)
	return secretStore
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
	secretValue *string,
) error {
	if s == nil || s.store == nil {
		return errors.New("integration bridge secret store is not configured")
	}
	if secretValue != nil {
		return errors.New("integration bridge secret store should not receive raw secret values")
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

func (s *integrationBridgeService) DeliveryMetrics() map[string]bridgepkg.BridgeDeliveryMetrics {
	if s == nil || s.broker == nil {
		return nil
	}
	return s.broker.DeliveryMetrics()
}

func (s *integrationBridgeService) DeliveryMetricsFor(
	bridgeInstanceIDs []string,
) (map[string]bridgepkg.BridgeDeliveryMetrics, error) {
	if s == nil || s.broker == nil {
		return nil, nil
	}
	return s.broker.DeliveryMetricsFor(bridgeInstanceIDs)
}

func (s *integrationBridgeService) Broker() *bridgepkg.Broker {
	if s == nil {
		return nil
	}
	return s.broker
}

func TestIntegrationBridgeServiceLifecycleTransitionsReachReady(t *testing.T) {
	runtime := newIntegrationRuntime(t)

	created, err := runtime.bridges.CreateInstance(context.Background(), bridgepkg.CreateInstanceRequest{
		ID:            "brg-lifecycle-ready",
		Scope:         bridgepkg.ScopeGlobal,
		Platform:      "telegram",
		ExtensionName: "ext-telegram",
		DisplayName:   "Lifecycle Ready",
		Enabled:       false,
		Status:        bridgepkg.BridgeStatusDisabled,
		RoutingPolicy: bridgepkg.RoutingPolicy{IncludePeer: true},
	})
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	started, err := runtime.bridges.StartInstance(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("StartInstance() error = %v", err)
	}
	if !started.Enabled || started.Status != bridgepkg.BridgeStatusReady {
		t.Fatalf("StartInstance() = %#v, want enabled ready instance", started)
	}

	restarted, err := runtime.bridges.RestartInstance(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("RestartInstance() error = %v", err)
	}
	if !restarted.Enabled || restarted.Status != bridgepkg.BridgeStatusReady {
		t.Fatalf("RestartInstance() = %#v, want enabled ready instance", restarted)
	}
}

func (t *integrationDreamTrigger) Trigger(_ context.Context, workspaceID string) (bool, string, error) {
	t.calls++
	t.recordedWorkspace = workspaceID
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
	mu             sync.Mutex
	nextPID        int
	nextSess       int
	permissionWait time.Duration
	promptHook     func(proc *session.AgentProcess, req acp.PromptRequest) (<-chan acp.AgentEvent, error)
	states         map[*session.AgentProcess]chan struct{}
	approvals      map[*session.AgentProcess]chan acp.ApproveRequest
	waitErrs       map[*session.AgentProcess]error
	bySessionID    map[string]*session.AgentProcess
}

func newIntegrationDriver(permissionWait time.Duration) *integrationDriver {
	if permissionWait <= 0 {
		permissionWait = 100 * time.Millisecond
	}
	return &integrationDriver{
		nextPID:        2000,
		nextSess:       1,
		permissionWait: permissionWait,
		states:         make(map[*session.AgentProcess]chan struct{}),
		approvals:      make(map[*session.AgentProcess]chan acp.ApproveRequest),
		waitErrs:       make(map[*session.AgentProcess]error),
		bySessionID:    make(map[string]*session.AgentProcess),
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

	var proc *session.AgentProcess
	proc = session.NewAgentProcess(session.AgentProcessOptions{
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
			d.mu.Lock()
			defer d.mu.Unlock()

			err := d.waitErrs[proc]
			delete(d.waitErrs, proc)
			delete(d.states, proc)
			delete(d.approvals, proc)
			delete(d.bySessionID, proc.SessionID)
			return err
		},
		ApprovePermission: func(ctx context.Context, req acp.ApproveRequest) error {
			d.mu.Lock()
			approvalCh := d.approvals[proc]
			d.mu.Unlock()
			if approvalCh == nil {
				return session.ErrPendingPermissionNotFound
			}

			select {
			case approvalCh <- req:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	d.states[proc] = done
	d.bySessionID[proc.SessionID] = proc
	return proc, nil
}

func (d *integrationDriver) Prompt(
	_ context.Context,
	proc *session.AgentProcess,
	req acp.PromptRequest,
) (<-chan acp.AgentEvent, error) {
	d.mu.Lock()
	hook := d.promptHook
	d.mu.Unlock()
	if hook != nil {
		return hook(proc, req)
	}

	if strings.Contains(req.Message, "request permission") {
		events := make(chan acp.AgentEvent, 6)
		requestID := req.TurnID + ":tool-1"
		approvalCh := make(chan acp.ApproveRequest, 1)

		d.mu.Lock()
		d.approvals[proc] = approvalCh
		d.mu.Unlock()

		go func() {
			defer close(events)
			defer func() {
				d.mu.Lock()
				delete(d.approvals, proc)
				d.mu.Unlock()
			}()

			raw := mustIntegrationJSON(tPermissionRaw(requestID))
			ts := time.Now().UTC()
			events <- acp.AgentEvent{
				Type:       "permission",
				SessionID:  proc.SessionID,
				TurnID:     req.TurnID,
				RequestID:  requestID,
				Timestamp:  ts,
				Title:      "permission request",
				ToolCallID: "tool-1",
				Action:     "session/request_permission",
				Resource:   "/tmp/demo.txt",
				Raw:        raw,
			}

			finalDecision := "reject-once"
			select {
			case approval := <-approvalCh:
				finalDecision = approval.Decision
			case <-time.After(d.permissionWait):
			}

			ts = time.Now().UTC()
			events <- acp.AgentEvent{
				Type:       "permission",
				SessionID:  proc.SessionID,
				TurnID:     req.TurnID,
				RequestID:  requestID,
				Timestamp:  ts,
				Title:      "permission request",
				ToolCallID: "tool-1",
				Action:     "session/request_permission",
				Resource:   "/tmp/demo.txt",
				Decision:   finalDecision,
				Raw:        mustIntegrationJSON(tPermissionRawWithDecision(requestID, finalDecision)),
			}
			events <- acp.AgentEvent{
				Type:      "agent_message",
				SessionID: proc.SessionID,
				TurnID:    req.TurnID,
				Timestamp: ts,
				Text:      finalDecision,
			}
			events <- acp.AgentEvent{
				Type:       "done",
				SessionID:  proc.SessionID,
				TurnID:     req.TurnID,
				Timestamp:  ts,
				StopReason: "end_turn",
			}
		}()
		return events, nil
	}

	ch := make(chan acp.AgentEvent, 4)
	ch <- acp.AgentEvent{
		Type:      "agent_message",
		SessionID: proc.SessionID,
		TurnID:    req.TurnID,
		Timestamp: time.Now().UTC(),
		Text:      req.Message,
	}
	ch <- acp.AgentEvent{
		Type:       "tool_call",
		SessionID:  proc.SessionID,
		TurnID:     req.TurnID,
		Timestamp:  time.Now().UTC(),
		Title:      "read_file",
		ToolCallID: "call-1",
	}
	ch <- acp.AgentEvent{
		Type:       "tool_result",
		SessionID:  proc.SessionID,
		TurnID:     req.TurnID,
		Timestamp:  time.Now().UTC(),
		ToolCallID: "call-1",
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

func countSessionEventsByType(events []store.SessionEvent, want string) int {
	count := 0
	for _, event := range events {
		if event.Type == want {
			count++
		}
	}
	return count
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
	return nil
}

func (d *integrationDriver) Crash(acpSessionID string, waitErr error) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	proc := d.bySessionID[strings.TrimSpace(acpSessionID)]
	if proc == nil {
		return fmt.Errorf("integration driver: session %q not found", acpSessionID)
	}
	done := d.states[proc]
	if done == nil {
		return fmt.Errorf("integration driver: runtime for session %q not found", acpSessionID)
	}
	if waitErr == nil {
		waitErr = errors.New("integration crash")
	}
	d.waitErrs[proc] = waitErr
	select {
	case <-done:
	default:
		close(done)
	}
	return nil
}

func newIntegrationRuntime(t *testing.T) integrationRuntime {
	return newIntegrationRuntimeWithPermissionWait(t, 100*time.Millisecond)
}

func newIntegrationRuntimeWithPermissionWait(t *testing.T, permissionWait time.Duration) integrationRuntime {
	t.Helper()

	homePaths := newTestHomePaths(t)
	writeAgentDef(t, homePaths, "coder")

	workspace := t.TempDir()
	cfg := aghconfig.DefaultWithHome(homePaths)
	cfg.HTTP.Host = "127.0.0.1"
	cfg.HTTP.Port = freeTCPPort(t)
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
	if _, err := resolver.Register(context.Background(), workspacepkg.RegisterOptions{
		RootDir: workspace,
		Name:    "ws-workspace",
	}); err != nil {
		t.Fatalf("workspace.Register(%q) error = %v", workspace, err)
	}
	driver := newIntegrationDriver(permissionWait)
	sandboxRegistry, err := sandboxlocal.NewRegistry()
	if err != nil {
		t.Fatalf("local.NewRegistry() error = %v", err)
	}
	participationResolver := apitestutil.NewIntegrationParticipationResolver(t, registry)
	manager, err := session.NewManager(
		session.WithHomePaths(homePaths),
		session.WithWorkspaceResolver(resolver),
		session.WithLogger(discardLogger()),
		session.WithDriver(driver),
		session.WithNotifier(fanout),
		session.WithSandboxRegistry(sandboxRegistry),
		session.WithSessionCatalog(registry),
		session.WithParticipationResolver(participationResolver),
	)
	if err != nil {
		t.Fatalf("session.NewManager() error = %v", err)
	}
	bridgeService := newIntegrationBridgeService(registry)
	t.Cleanup(func() {
		if broker := bridgeService.Broker(); broker != nil {
			broker.Close()
		}
	})

	observer, err := observe.New(
		context.Background(),
		observe.WithHomePaths(homePaths),
		observe.WithRegistry(registry),
		observe.WithSessionSource(manager),
		observe.WithBridgeSource(bridgeService),
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
	if err := memoryStore.OpenCatalog(t.Context()); err != nil {
		t.Fatalf("memoryStore.OpenCatalog() error = %v", err)
	}
	t.Cleanup(func() {
		if err := memoryStore.CloseCatalog(context.Background()); err != nil {
			t.Fatalf("memoryStore.CloseCatalog() error = %v", err)
		}
	})
	dreamTrigger := &integrationDreamTrigger{
		enabled:   true,
		triggered: true,
		last:      time.Date(2026, 4, 4, 3, 30, 0, 0, time.UTC),
	}
	lookupEnv := func(key string) (string, bool) {
		value, ok := os.LookupEnv(key)
		return value, ok && strings.TrimSpace(value) != ""
	}
	vaultService, err := vaultpkg.NewService(
		registry,
		vaultpkg.NewFileKeyProvider(homePaths.HomeDir, lookupEnv),
		vaultpkg.WithLookupEnv(lookupEnv),
	)
	if err != nil {
		t.Fatalf("vault.NewService() error = %v", err)
	}

	automationManager, err := automationpkg.New(
		automationpkg.WithStore(registry),
		automationpkg.WithSessions(manager),
		automationpkg.WithWorkspaceResolver(resolver),
		automationpkg.WithConfig(cfg.Automation),
		automationpkg.WithLogger(discardLogger()),
		automationpkg.WithGlobalWorkspacePath(homePaths.HomeDir),
		automationpkg.WithWebhookSecretStore(vaultService),
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

	resourceKernel, err := resources.NewKernel(registry.DB())
	if err != nil {
		t.Fatalf("resources.NewKernel() error = %v", err)
	}
	resourceService, err := core.NewOperatorResourceService(&core.ResourceServiceConfig{RawStore: resourceKernel})
	if err != nil {
		t.Fatalf("core.NewOperatorResourceService() error = %v", err)
	}

	server, err := New(
		WithHomePaths(homePaths),
		WithConfig(&cfg),
		WithHost(cfg.HTTP.Host),
		WithPort(cfg.HTTP.Port),
		WithLogger(discardLogger()),
		WithSessionManager(manager),
		WithSessionCatalog(registry),
		WithTaskService(taskManager),
		WithObserver(observer),
		WithResourceService(resourceService),
		WithAutomation(automationManager),
		WithBridgeService(bridgeService),
		WithVaultService(vaultService),
		WithWorkspaceResolver(resolver),
		WithMemoryStore(memoryStore),
		WithDreamTrigger(dreamTrigger),
		WithPollInterval(10*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("httpapi.New() error = %v", err)
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
		client:    &http.Client{},
		server:    server,
		manager:   manager,
		tasks:     taskManager,
		driver:    driver,
		observer:  observer,
		registry:  registry,
		bridges:   bridgeService,
		memory:    memoryStore,
		dream:     dreamTrigger,
		host:      cfg.HTTP.Host,
		port:      server.Port(),
		workspace: workspace,
	}
}

type permissionStreamPayload struct {
	RequestID string `json:"request_id"`
	Decision  string `json:"decision,omitempty"`
}

func streamPermissionPrompt(t *testing.T, body io.ReadCloser, requestIDCh chan<- string, resultCh chan<- []sseRecord) {
	t.Helper()
	defer func() {
		_ = body.Close()
	}()

	scanner := bufio.NewScanner(body)
	records := make([]sseRecord, 0, 8)
	current := sseRecord{}
	requestIDSent := false

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if current.Event != "" || current.ID != "" || len(current.Data) > 0 {
				records = append(records, current)
				if !requestIDSent {
					if payload, ok := extractPermissionPayloadFromRecord(
						current,
					); ok && payload.Decision == "" &&
						payload.RequestID != "" {
						requestIDCh <- payload.RequestID
						requestIDSent = true
					}
				}
			}
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
		records = append(records, current)
	}
	if !requestIDSent {
		requestIDCh <- ""
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan prompt SSE error = %v", err)
	}
	resultCh <- records
}

func extractPermissionPayloads(t *testing.T, records []sseRecord) []permissionStreamPayload {
	t.Helper()

	payloads := make([]permissionStreamPayload, 0, len(records))
	for _, record := range records {
		if payload, ok := extractPermissionPayloadFromRecord(record); ok {
			payloads = append(payloads, payload)
		}
	}
	return payloads
}

func extractPermissionPayloadFromRecord(record sseRecord) (permissionStreamPayload, bool) {
	if len(record.Data) == 0 || string(record.Data) == "[DONE]" {
		return permissionStreamPayload{}, false
	}

	var envelope struct {
		Type string                  `json:"type"`
		Data permissionStreamPayload `json:"data"`
	}
	if err := json.Unmarshal(record.Data, &envelope); err != nil || envelope.Type != "data-agh-permission" {
		return permissionStreamPayload{}, false
	}
	return envelope.Data, true
}

func recordsContainTextDelta(records []sseRecord, want string) bool {
	for _, record := range records {
		if len(record.Data) == 0 || string(record.Data) == "[DONE]" {
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(record.Data, &payload); err != nil {
			continue
		}
		if payload["type"] == "text-delta" && payload["delta"] == want {
			return true
		}
	}
	return false
}

func tPermissionRaw(requestID string) map[string]any {
	return map[string]any{
		"request_id": requestID,
		"tool_input": map[string]any{"command": "demo"},
		"options": []map[string]any{
			{"decision": "allow-once", "kind": "allow_once", "option_id": "allow-once", "label": "allow once"},
			{"decision": "allow-always", "kind": "allow_always", "option_id": "allow-always", "label": "allow always"},
			{"decision": "reject-once", "kind": "reject_once", "option_id": "reject-once", "label": "reject once"},
			{
				"decision":  "reject-always",
				"kind":      "reject_always",
				"option_id": "reject-always",
				"label":     "reject always",
			},
		},
	}
}

func tPermissionRawWithDecision(requestID string, decision string) map[string]any {
	payload := tPermissionRaw(requestID)
	payload["decision"] = decision
	return payload
}

func mustIntegrationJSON(value any) json.RawMessage {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func createIntegrationSession(t *testing.T, runtime integrationRuntime) string {
	t.Helper()
	return createIntegrationSessionFromRequest(t, runtime, map[string]any{
		"agent_name":     "coder",
		"workspace_path": runtime.workspace,
	})
}

func createLiveIntegrationSession(t *testing.T, runtime integrationRuntime) string {
	t.Helper()
	return createIntegrationSessionFromRequest(t, runtime, map[string]any{
		"agent_name":     "coder",
		"workspace_path": runtime.workspace,
		"network_participation": map[string]any{
			"mode":             "live",
			"channel_strategy": "named",
			"channel_id":       "builders",
		},
	})
}

func createIntegrationSessionFromRequest(
	t *testing.T,
	runtime integrationRuntime,
	request map[string]any,
) string {
	t.Helper()

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/sessions"),
		mustIntegrationJSON(request),
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
	return created.Session.ID
}

func createIntegrationTask(t *testing.T, runtime integrationRuntime, body []byte) contract.TaskPayload {
	t.Helper()

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/tasks"),
		body,
		nil,
	)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("create task status = %d, want %d; body=%s", resp.StatusCode, http.StatusCreated, string(body))
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

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/tasks/"+taskID+"/runs"),
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

func requireHTTPInboxGroup(
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

func httpInboxGroupHasTask(group contract.TaskInboxLaneGroupPayload, taskID string) bool {
	for _, item := range group.Items {
		if item.Task.ID == taskID {
			return true
		}
	}
	return false
}

func sendPrompt(t *testing.T, runtime integrationRuntime, sessionID string, message string) {
	t.Helper()

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/prompt"),
		mustIntegrationJSON(map[string]any{"message": message}),
		nil,
	)
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		t.Fatalf("prompt status = %d, want %d; body=%s", resp.StatusCode, http.StatusOK, string(body))
	}
	_, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
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

func stopIntegrationSession(t *testing.T, runtime integrationRuntime, sessionID string) {
	t.Helper()

	resp := mustHTTPRequest(
		t,
		runtime.client,
		http.MethodPost,
		mustURL(runtime.host, runtime.port, "/api/workspaces/ws-workspace/sessions/"+sessionID+"/stop"),
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

func containsAutomationRun(runs []contract.RunPayload, id string) bool {
	for _, run := range runs {
		if run.ID == id {
			return true
		}
	}
	return false
}

func integrationPromptEventsContainTerminalToolEvents(events []store.SessionEvent) bool {
	var hasToolResult bool
	var hasDone bool

	for _, event := range events {
		switch event.Type {
		case acp.EventTypeToolResult:
			hasToolResult = true
		case acp.EventTypeDone:
			hasDone = true
		}
	}

	return hasToolResult && hasDone
}

func waitForRegistryStopReason(t *testing.T, runtime integrationRuntime, sessionID string, want store.StopReason) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		sessions, err := runtime.registry.ListSessions(context.Background(), store.SessionListQuery{State: "stopped"})
		if err == nil {
			for _, item := range sessions {
				if item.ID == sessionID && item.StopReason == want {
					return
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for stopped session %q with stop reason %q", sessionID, want)
}
