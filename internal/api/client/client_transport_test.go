package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	apicore "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/pkg/compozy/events"
)

func TestTargetAndClientTransportHelpers(t *testing.T) {
	t.Parallel()

	t.Run("target validate string and new", func(t *testing.T) {
		t.Parallel()

		if err := (Target{}).Validate(); err == nil {
			t.Fatal("Target{}.Validate() error = nil, want invalid target error")
		}

		socketTarget := Target{SocketPath: "/tmp/compozy.sock"}
		if err := socketTarget.Validate(); err != nil {
			t.Fatalf("socket target Validate() error = %v", err)
		}
		if got := socketTarget.String(); got != "unix:///tmp/compozy.sock" {
			t.Fatalf("socket target String() = %q, want unix:///tmp/compozy.sock", got)
		}

		httpTarget := Target{HTTPPort: 4317}
		if err := httpTarget.Validate(); err != nil {
			t.Fatalf("http target Validate() error = %v", err)
		}
		if got := httpTarget.String(); got != "http://127.0.0.1:4317" {
			t.Fatalf("http target String() = %q, want http://127.0.0.1:4317", got)
		}

		socketClient, err := New(socketTarget)
		if err != nil {
			t.Fatalf("New(socket target) error = %v", err)
		}
		if socketClient.baseURL != "http://daemon" || socketClient.httpClient == nil ||
			socketClient.httpClient.Transport == nil {
			t.Fatalf("socket client = %#v, want unix transport with daemon base URL", socketClient)
		}
		if got := socketClient.Target(); got != socketTarget {
			t.Fatalf("socket client Target() = %#v, want %#v", got, socketTarget)
		}

		httpClient, err := New(httpTarget)
		if err != nil {
			t.Fatalf("New(http target) error = %v", err)
		}
		if httpClient.baseURL != "http://127.0.0.1:4317" || httpClient.httpClient == nil {
			t.Fatalf("http client = %#v, want localhost base URL", httpClient)
		}
		if got := httpClient.Target(); got != httpTarget {
			t.Fatalf("http client Target() = %#v, want %#v", got, httpTarget)
		}

		var nilClient *Client
		if got := nilClient.Target(); got != (Target{}) {
			t.Fatalf("nil client Target() = %#v, want zero target", got)
		}

		if got := (Target{SocketPath: " "}).String(); got != "unknown" {
			t.Fatalf("blank target String() = %q, want unknown", got)
		}
	})

	t.Run("request path timeout and health semantics", func(t *testing.T) {
		t.Parallel()

		request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://daemon", http.NoBody)
		if err != nil {
			t.Fatalf("http.NewRequest() error = %v", err)
		}
		if err := applyRequestPath(request, "/api/tasks/demo%20alpha/runs?limit=1"); err != nil {
			t.Fatalf("applyRequestPath() error = %v", err)
		}
		if request.URL.Path != "/api/tasks/demo alpha/runs" {
			t.Fatalf("request URL.Path = %q, want decoded task path", request.URL.Path)
		}
		if request.URL.EscapedPath() != "/api/tasks/demo%20alpha/runs" || request.URL.RawQuery != "limit=1" {
			t.Fatalf(
				"request URL = %s?%s, want escaped path and query preserved",
				request.URL.EscapedPath(),
				request.URL.RawQuery,
			)
		}
		if err := applyRequestPath(nil, "/api/tasks/demo/runs"); err == nil {
			t.Fatal("applyRequestPath(nil) error = nil, want request-required error")
		}
		if err := applyRequestPath(request, "https://example.com/api/tasks/demo/runs"); err == nil {
			t.Fatal("applyRequestPath(abs url) error = nil, want daemon path validation error")
		}
		if err := applyRequestPath(request, "/health"); err == nil {
			t.Fatal("applyRequestPath(non-daemon path) error = nil, want daemon path validation error")
		}

		ctxWithTimeout, cancel := withRequestTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if _, ok := ctxWithTimeout.Deadline(); !ok {
			t.Fatal("withRequestTimeout() deadline missing, want new timeout")
		}

		existingCtx, existingCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer existingCancel()
		existingDeadline, _ := existingCtx.Deadline()
		preservedCtx, preservedCancel := withRequestTimeout(existingCtx, time.Minute)
		defer preservedCancel()
		preservedDeadline, ok := preservedCtx.Deadline()
		if !ok || !preservedDeadline.Equal(existingDeadline) {
			t.Fatalf("withRequestTimeout() deadline = %v, want preserved %v", preservedDeadline, existingDeadline)
		}

		client := &Client{
			target:  Target{SocketPath: "/tmp/compozy.sock"},
			baseURL: "http://daemon",
			httpClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					if req.URL.Path != "/api/daemon/health" {
						t.Fatalf("path = %s, want /api/daemon/health", req.URL.Path)
					}
					return jsonStructResponse(t, http.StatusServiceUnavailable, contract.DaemonHealthResponse{
						Health: contract.DaemonHealth{
							Ready:    false,
							Degraded: true,
							Details:  []contract.HealthDetail{{Code: "booting", Message: "warming", Severity: "info"}},
						},
					}), nil
				}),
			},
		}

		health, err := client.Health(context.Background())
		if err != nil {
			t.Fatalf("Health() error = %v", err)
		}
		if health.Ready || !health.Degraded || len(health.Details) != 1 || health.Details[0].Code != "booting" {
			t.Fatalf("health = %#v, want degraded booting response", health)
		}
	})

	t.Run("status and decode helpers preserve transport errors", func(t *testing.T) {
		t.Parallel()

		if err := decodeResponseBody([]byte(`{"broken"`), &struct{}{}); err == nil {
			t.Fatal("decodeResponseBody(invalid json) error = nil, want decode error")
		}

		var response contract.DaemonHealthResponse
		err := (&Client{}).handleStatus("/api/daemon/health", http.StatusInternalServerError, []byte("boom"), &response)
		if err == nil || !strings.Contains(err.Error(), "status 500") {
			t.Fatalf("handleStatus(non-json) error = %v, want status 500 failure", err)
		}

		remoteErr := &RemoteError{
			StatusCode: http.StatusConflict,
			Envelope:   contract.TransportError{RequestID: "req-1"},
		}
		if got := remoteErr.Error(); got != "Conflict (request_id=req-1)" {
			t.Fatalf("RemoteError.Error() = %q, want status text fallback with request id", got)
		}

		if got := (*RemoteError)(nil).Error(); got != "" {
			t.Fatalf("nil RemoteError.Error() = %q, want empty string", got)
		}
	})
}

func TestClientOperatorRequestsUseCanonicalContract(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 18, 8, 0, 0, 0, time.UTC)
	workspace := contract.Workspace{
		ID:        "ws-1",
		RootDir:   "/tmp/workspace",
		Name:      "Demo",
		CreatedAt: now,
		UpdatedAt: now,
	}
	workflow := contract.WorkflowSummary{
		ID:          "wf-1",
		WorkspaceID: workspace.ID,
		Slug:        "demo",
	}
	syncedAt := now.Add(time.Minute)

	registerCalls := 0
	stopCalls := 0
	client := &Client{
		target:  Target{SocketPath: "/tmp/compozy.sock"},
		baseURL: "http://daemon",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.Method + " " + req.URL.Path {
				case http.MethodGet + " /api/daemon/status":
					return jsonStructResponse(t, http.StatusOK, contract.DaemonStatusResponse{
						Daemon: contract.DaemonStatus{
							PID:            42,
							HTTPPort:       4317,
							WorkspaceCount: 1,
							StartedAt:      now,
						},
					}), nil
				case http.MethodPost + " /api/daemon/stop":
					stopCalls++
					force := req.URL.Query().Get("force")
					switch stopCalls {
					case 1:
						if force != "" {
							t.Fatalf("StopDaemon(false) force query = %q, want empty", force)
						}
					case 2:
						if force != "true" {
							t.Fatalf("StopDaemon(true) force query = %q, want true", force)
						}
					default:
						t.Fatalf("unexpected stop call count %d", stopCalls)
					}
					return jsonResponse(http.StatusAccepted, `{"accepted":true}`), nil
				case http.MethodPost + " /api/workspaces":
					var payload contract.WorkspaceRegisterRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode register workspace request: %v", err)
					}
					if payload.Path != "/tmp/workspace" || payload.Name != "Demo" {
						t.Fatalf("register workspace payload = %#v, want trimmed values", payload)
					}
					registerCalls++
					status := http.StatusCreated
					if registerCalls == 2 {
						status = http.StatusOK
					}
					return jsonStructResponse(t, status, contract.WorkspaceResponse{Workspace: workspace}), nil
				case http.MethodGet + " /api/workspaces":
					return jsonStructResponse(t, http.StatusOK, contract.WorkspaceListResponse{
						Workspaces: []contract.Workspace{workspace},
					}), nil
				case http.MethodGet + " /api/workspaces/ws 1":
					return jsonStructResponse(t, http.StatusOK, contract.WorkspaceResponse{Workspace: workspace}), nil
				case http.MethodDelete + " /api/workspaces/ws 1":
					return &http.Response{
						StatusCode: http.StatusNoContent,
						Header:     make(http.Header),
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				case http.MethodPost + " /api/workspaces/resolve":
					var payload contract.WorkspaceResolveRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode resolve workspace request: %v", err)
					}
					if payload.Path != "/tmp/resolve" {
						t.Fatalf("resolve workspace payload = %#v, want trimmed path", payload)
					}
					return jsonStructResponse(t, http.StatusOK, contract.WorkspaceResponse{Workspace: workspace}), nil
				case http.MethodGet + " /api/tasks":
					if got := req.URL.Query().Get("workspace"); got != "/tmp/workspace" {
						t.Fatalf("workspace query = %q, want /tmp/workspace", got)
					}
					return jsonStructResponse(t, http.StatusOK, contract.TaskWorkflowListResponse{
						Workflows: []contract.WorkflowSummary{workflow},
					}), nil
				case http.MethodPost + " /api/tasks/demo/archive":
					var payload contract.WorkflowArchiveRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode archive request: %v", err)
					}
					if payload.Workspace != "/tmp/workspace" || payload.Force {
						t.Fatalf("archive payload = %#v, want workspace ref", payload)
					}
					return jsonStructResponse(t, http.StatusOK, contract.ArchiveResponse{
						Archived:   true,
						ArchivedAt: &syncedAt,
					}), nil
				case http.MethodPost + " /api/sync":
					var payload contract.SyncRequest
					if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
						t.Fatalf("decode sync request: %v", err)
					}
					if payload.Workspace != "/tmp/workspace" || payload.WorkflowSlug != "demo" {
						t.Fatalf("sync payload = %#v, want workspace and slug", payload)
					}
					return jsonStructResponse(t, http.StatusOK, contract.SyncResponse{
						WorkspaceID:      workspace.ID,
						WorkflowSlug:     "demo",
						SyncedAt:         &syncedAt,
						WorkflowsScanned: 3,
						SyncedPaths:      []string{"/tmp/workspace/.compozy/tasks/demo"},
					}), nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.RequestURI())
					return nil, nil
				}
			}),
		},
	}

	status, err := client.DaemonStatus(context.Background())
	if err != nil {
		t.Fatalf("DaemonStatus() error = %v", err)
	}
	if status.PID != 42 || status.HTTPPort != 4317 || status.WorkspaceCount != 1 {
		t.Fatalf("DaemonStatus() = %#v, want canonical daemon status", status)
	}

	if err := client.StopDaemon(context.Background(), false); err != nil {
		t.Fatalf("StopDaemon(false) error = %v", err)
	}
	if err := client.StopDaemon(context.Background(), true); err != nil {
		t.Fatalf("StopDaemon(true) error = %v", err)
	}
	if stopCalls != 2 {
		t.Fatalf("stop calls = %d, want 2", stopCalls)
	}

	registered, err := client.RegisterWorkspace(context.Background(), " /tmp/workspace ", " Demo ")
	if err != nil {
		t.Fatalf("RegisterWorkspace(created) error = %v", err)
	}
	if !registered.Created || registered.Workspace.ID != workspace.ID {
		t.Fatalf("RegisterWorkspace(created) = %#v, want created workspace", registered)
	}

	registered, err = client.RegisterWorkspace(context.Background(), " /tmp/workspace ", " Demo ")
	if err != nil {
		t.Fatalf("RegisterWorkspace(existing) error = %v", err)
	}
	if registered.Created {
		t.Fatalf("RegisterWorkspace(existing) = %#v, want created=false on 200", registered)
	}

	workspaces, err := client.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}
	if len(workspaces) != 1 || workspaces[0].ID != workspace.ID {
		t.Fatalf("ListWorkspaces() = %#v, want one canonical workspace", workspaces)
	}

	gotWorkspace, err := client.GetWorkspace(context.Background(), " ws 1 ")
	if err != nil {
		t.Fatalf("GetWorkspace() error = %v", err)
	}
	if gotWorkspace.ID != workspace.ID {
		t.Fatalf("GetWorkspace() = %#v, want workspace %q", gotWorkspace, workspace.ID)
	}

	if err := client.DeleteWorkspace(context.Background(), " ws 1 "); err != nil {
		t.Fatalf("DeleteWorkspace() error = %v", err)
	}

	resolved, err := client.ResolveWorkspace(context.Background(), " /tmp/resolve ")
	if err != nil {
		t.Fatalf("ResolveWorkspace() error = %v", err)
	}
	if resolved.ID != workspace.ID {
		t.Fatalf("ResolveWorkspace() = %#v, want workspace %q", resolved, workspace.ID)
	}

	workflows, err := client.ListTaskWorkflows(context.Background(), "/tmp/workspace")
	if err != nil {
		t.Fatalf("ListTaskWorkflows() error = %v", err)
	}
	if len(workflows) != 1 || workflows[0].Slug != "demo" {
		t.Fatalf("ListTaskWorkflows() = %#v, want demo workflow", workflows)
	}

	archiveResult, err := client.ArchiveTaskWorkflow(context.Background(), "/tmp/workspace", " demo ")
	if err != nil {
		t.Fatalf("ArchiveTaskWorkflow() error = %v", err)
	}
	if !archiveResult.Archived || archiveResult.ArchivedAt == nil || !archiveResult.ArchivedAt.Equal(syncedAt) {
		t.Fatalf("ArchiveTaskWorkflow() = %#v, want archived result", archiveResult)
	}

	syncResult, err := client.SyncWorkflow(context.Background(), apicore.SyncRequest{
		Workspace:    "/tmp/workspace",
		WorkflowSlug: "demo",
	})
	if err != nil {
		t.Fatalf("SyncWorkflow() error = %v", err)
	}
	if syncResult.WorkspaceID != workspace.ID || syncResult.WorkflowSlug != "demo" || syncResult.WorkflowsScanned != 3 {
		t.Fatalf("SyncWorkflow() = %#v, want canonical sync result", syncResult)
	}

	if _, err := client.GetWorkspace(
		context.Background(),
		" ",
	); err == nil ||
		!strings.Contains(err.Error(), "workspace ref is required") {
		t.Fatalf("GetWorkspace(blank) error = %v, want workspace ref guard", err)
	}
	if err := client.DeleteWorkspace(
		context.Background(),
		" ",
	); err == nil ||
		!strings.Contains(err.Error(), "workspace ref is required") {
		t.Fatalf("DeleteWorkspace(blank) error = %v, want workspace ref guard", err)
	}
	if _, err := client.ArchiveTaskWorkflow(
		context.Background(),
		"/tmp/workspace",
		" ",
	); !errors.Is(
		err,
		ErrWorkflowSlugRequired,
	) {
		t.Fatalf("ArchiveTaskWorkflow(blank slug) error = %v, want ErrWorkflowSlugRequired", err)
	}
}

func TestClientRunQueriesUseCanonicalContract(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 18, 9, 0, 0, 0, time.UTC)
	after := apicore.StreamCursor{Timestamp: now, Sequence: 3}
	nextCursor := contract.StreamCursor{Timestamp: now.Add(2 * time.Second), Sequence: 5}
	run := contract.Run{
		RunID:     "run-1",
		Status:    "running",
		Mode:      "exec",
		StartedAt: now,
	}
	page := contract.RunEventPage{
		Events: []events.Event{{
			SchemaVersion: events.SchemaVersion,
			RunID:         "run-1",
			Seq:           4,
			Timestamp:     now.Add(time.Second),
			Kind:          events.EventKindRunStarted,
		}},
		NextCursor: &nextCursor,
		HasMore:    true,
	}

	client := &Client{
		target:  Target{SocketPath: "/tmp/compozy.sock"},
		baseURL: "http://daemon",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				switch req.Method + " " + req.URL.Path {
				case http.MethodGet + " /api/runs":
					query := req.URL.Query()
					if query.Get("workspace") != "/tmp/workspace" ||
						query.Get("status") != "running" ||
						query.Get("mode") != "exec" ||
						query.Get("limit") != "25" {
						t.Fatalf("run list query = %#v, want canonical filters", query)
					}
					return jsonStructResponse(
						t,
						http.StatusOK,
						contract.RunListResponse{Runs: []contract.Run{run}},
					), nil
				case http.MethodGet + " /api/runs/run-1":
					return jsonStructResponse(t, http.StatusOK, contract.RunResponse{Run: run}), nil
				case http.MethodPost + " /api/runs/run-1/cancel":
					return jsonResponse(http.StatusAccepted, `{"accepted":true}`), nil
				case http.MethodGet + " /api/runs/run-1/events":
					query := req.URL.Query()
					if query.Get("after") != contract.FormatCursor(after.Timestamp, after.Sequence) ||
						query.Get("limit") != "50" {
						t.Fatalf("run events query = %#v, want canonical cursor and limit", query)
					}
					return jsonStructResponse(t, http.StatusOK, contract.RunEventPageResponseFromPage(page)), nil
				default:
					t.Fatalf("unexpected request %s %s", req.Method, req.URL.RequestURI())
					return nil, nil
				}
			}),
		},
	}

	runs, err := client.ListRuns(context.Background(), RunListOptions{
		Workspace: "/tmp/workspace",
		Status:    "running",
		Mode:      "exec",
		Limit:     25,
	})
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != run.RunID || runs[0].Mode != run.Mode {
		t.Fatalf("ListRuns() = %#v, want canonical run summary", runs)
	}

	gotRun, err := client.GetRun(context.Background(), " run-1 ")
	if err != nil {
		t.Fatalf("GetRun() error = %v", err)
	}
	if gotRun.RunID != run.RunID || gotRun.Status != run.Status {
		t.Fatalf("GetRun() = %#v, want run-1", gotRun)
	}

	if err := client.CancelRun(context.Background(), " run-1 "); err != nil {
		t.Fatalf("CancelRun() error = %v", err)
	}

	eventPage, err := client.ListRunEvents(context.Background(), " run-1 ", after, 50)
	if err != nil {
		t.Fatalf("ListRunEvents() error = %v", err)
	}
	if len(eventPage.Events) != 1 || eventPage.Events[0].Seq != 4 || !eventPage.HasMore {
		t.Fatalf("ListRunEvents() = %#v, want one event and has_more=true", eventPage)
	}
	if eventPage.NextCursor == nil || eventPage.NextCursor.Sequence != 5 ||
		!eventPage.NextCursor.Timestamp.Equal(nextCursor.Timestamp) {
		t.Fatalf("ListRunEvents() next cursor = %#v, want seq 5", eventPage.NextCursor)
	}

	if _, err := client.GetRun(context.Background(), " "); !errors.Is(err, ErrRunIDRequired) {
		t.Fatalf("GetRun(blank) error = %v, want ErrRunIDRequired", err)
	}
	if err := client.CancelRun(context.Background(), " "); !errors.Is(err, ErrRunIDRequired) {
		t.Fatalf("CancelRun(blank) error = %v, want ErrRunIDRequired", err)
	}
	if _, err := client.ListRunEvents(context.Background(), " ", after, 50); !errors.Is(err, ErrRunIDRequired) {
		t.Fatalf("ListRunEvents(blank) error = %v, want ErrRunIDRequired", err)
	}
}

func TestOpenRunStreamBodyAndStreamHelpers(t *testing.T) {
	t.Parallel()

	t.Run("open stream body preserves cursor header and remote errors", func(t *testing.T) {
		t.Parallel()

		cursor := apicore.StreamCursor{
			Timestamp: time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC),
			Sequence:  12,
		}
		requests := 0
		client := &Client{
			target:  Target{SocketPath: "/tmp/compozy.sock"},
			baseURL: "http://daemon",
			httpClient: &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					if req.URL.Path != "/api/runs/run-1/stream" {
						t.Fatalf("path = %s, want /api/runs/run-1/stream", req.URL.Path)
					}
					if got := req.Header.Get(
						"Last-Event-ID",
					); got != contract.FormatCursor(
						cursor.Timestamp,
						cursor.Sequence,
					) {
						t.Fatalf("Last-Event-ID = %q, want canonical cursor", got)
					}
					if requests == 1 {
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     make(http.Header),
							Body:       io.NopCloser(strings.NewReader("")),
						}, nil
					}
					return jsonStructResponse(t, http.StatusConflict, contract.TransportError{
						RequestID: "req-stream",
						Code:      "conflict",
						Message:   "stream rejected",
					}), nil
				}),
			},
		}

		body, err := client.openRunStreamBody(context.Background(), "run-1", cursor)
		if err != nil {
			t.Fatalf("openRunStreamBody(ok) error = %v", err)
		}
		if err := body.Close(); err != nil {
			t.Fatalf("stream body Close() error = %v", err)
		}

		_, err = client.openRunStreamBody(context.Background(), "run-1", cursor)
		if err == nil {
			t.Fatal("openRunStreamBody(remote error) error = nil, want *RemoteError")
		}
		var remoteErr *RemoteError
		if !errors.As(err, &remoteErr) || remoteErr.Envelope.Code != "conflict" {
			t.Fatalf("openRunStreamBody(remote error) = %v, want canonical conflict error", err)
		}
	})

	t.Run("stream helpers handle overflow stream errors and cursor ordering", func(t *testing.T) {
		t.Parallel()

		cursor := apicore.StreamCursor{
			Timestamp: time.Date(2026, 4, 18, 10, 1, 0, 0, time.UTC),
			Sequence:  14,
		}
		connection := newStreamConnection(context.Background(), io.NopCloser(strings.NewReader(strings.Join([]string{
			"event: overflow",
			`data: {"cursor":"2026-04-18T10:01:00Z|00000000000000000014","reason":"lagging","ts":"2026-04-18T10:01:00Z"}`,
			"",
			"event: error",
			`data: {"code":"schema_too_new"}`,
			"",
		}, "\n"))))

		item := <-connection.items
		if item.item.Overflow == nil || item.item.Overflow.Cursor.Sequence != 14 ||
			item.item.Overflow.Reason != "lagging" {
			t.Fatalf("overflow item = %#v, want cursor seq 14 and lagging reason", item)
		}
		if item.cursor.Sequence != 14 || !item.cursor.Timestamp.Equal(cursor.Timestamp) {
			t.Fatalf("overflow ack cursor = %#v, want seq 14", item.cursor)
		}

		err := <-connection.errors
		if err == nil || err.Error() != "schema_too_new" {
			t.Fatalf("stream error = %v, want schema_too_new", err)
		}
		<-connection.done

		stream := &clientRunStream{
			ctx:    context.Background(),
			items:  make(chan RunStreamItem, 1),
			errors: make(chan error, 1),
		}
		stream.sendError(errors.New("boom"))
		if got := <-stream.errors; got == nil || got.Error() != "boom" {
			t.Fatalf("clientRunStream.sendError() = %v, want boom", got)
		}

		cases := []struct {
			name  string
			left  apicore.StreamCursor
			right apicore.StreamCursor
			want  bool
		}{
			{
				name: "empty right accepts left",
				left: apicore.StreamCursor{Timestamp: cursor.Timestamp, Sequence: 1},
				want: true,
			},
			{
				name:  "later timestamp sorts after",
				left:  apicore.StreamCursor{Timestamp: cursor.Timestamp.Add(time.Second), Sequence: 1},
				right: cursor,
				want:  true,
			},
			{
				name:  "earlier timestamp sorts before",
				left:  apicore.StreamCursor{Timestamp: cursor.Timestamp.Add(-time.Second), Sequence: 99},
				right: cursor,
				want:  false,
			},
			{
				name:  "same timestamp uses sequence",
				left:  apicore.StreamCursor{Timestamp: cursor.Timestamp, Sequence: cursor.Sequence - 1},
				right: cursor,
				want:  false,
			},
		}

		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				if got := cursorAfterOrEqual(tt.left, tt.right); got != tt.want {
					t.Fatalf("cursorAfterOrEqual(%#v, %#v) = %t, want %t", tt.left, tt.right, got, tt.want)
				}
			})
		}
	})
}

func TestNilReceiverAndUtilityGuards(t *testing.T) {
	t.Parallel()

	var nilClient *Client

	if _, err := nilClient.Health(context.Background()); !errors.Is(err, ErrDaemonClientRequired) {
		t.Fatalf("nil Health() error = %v, want ErrDaemonClientRequired", err)
	}
	if _, err := nilClient.DaemonStatus(context.Background()); !errors.Is(err, ErrDaemonClientRequired) {
		t.Fatalf("nil DaemonStatus() error = %v, want ErrDaemonClientRequired", err)
	}
	if err := nilClient.StopDaemon(context.Background(), false); !errors.Is(err, ErrDaemonClientRequired) {
		t.Fatalf("nil StopDaemon() error = %v, want ErrDaemonClientRequired", err)
	}
	if _, err := nilClient.RegisterWorkspace(
		context.Background(),
		"/tmp/workspace",
		"Demo",
	); !errors.Is(
		err,
		ErrDaemonClientRequired,
	) {
		t.Fatalf("nil RegisterWorkspace() error = %v, want ErrDaemonClientRequired", err)
	}
	if _, err := nilClient.ListWorkspaces(context.Background()); !errors.Is(err, ErrDaemonClientRequired) {
		t.Fatalf("nil ListWorkspaces() error = %v, want ErrDaemonClientRequired", err)
	}
	if _, err := nilClient.ResolveWorkspace(
		context.Background(),
		"/tmp/workspace",
	); !errors.Is(
		err,
		ErrDaemonClientRequired,
	) {
		t.Fatalf("nil ResolveWorkspace() error = %v, want ErrDaemonClientRequired", err)
	}
	if _, err := nilClient.ListTaskWorkflows(
		context.Background(),
		"/tmp/workspace",
	); !errors.Is(
		err,
		ErrDaemonClientRequired,
	) {
		t.Fatalf("nil ListTaskWorkflows() error = %v, want ErrDaemonClientRequired", err)
	}
	if _, err := nilClient.SyncWorkflow(
		context.Background(),
		apicore.SyncRequest{},
	); !errors.Is(
		err,
		ErrDaemonClientRequired,
	) {
		t.Fatalf("nil SyncWorkflow() error = %v, want ErrDaemonClientRequired", err)
	}
	if _, err := nilClient.ListRuns(context.Background(), RunListOptions{}); !errors.Is(err, ErrDaemonClientRequired) {
		t.Fatalf("nil ListRuns() error = %v, want ErrDaemonClientRequired", err)
	}
	if _, err := nilClient.GetRun(context.Background(), "run-1"); !errors.Is(err, ErrDaemonClientRequired) {
		t.Fatalf("nil GetRun() error = %v, want ErrDaemonClientRequired", err)
	}
	if err := nilClient.CancelRun(context.Background(), "run-1"); !errors.Is(err, ErrDaemonClientRequired) {
		t.Fatalf("nil CancelRun() error = %v, want ErrDaemonClientRequired", err)
	}
	if _, err := nilClient.GetRunSnapshot(context.Background(), "run-1"); !errors.Is(err, ErrDaemonClientRequired) {
		t.Fatalf("nil GetRunSnapshot() error = %v, want ErrDaemonClientRequired", err)
	}
	if _, err := nilClient.ListRunEvents(
		context.Background(),
		"run-1",
		apicore.StreamCursor{},
		10,
	); !errors.Is(
		err,
		ErrDaemonClientRequired,
	) {
		t.Fatalf("nil ListRunEvents() error = %v, want ErrDaemonClientRequired", err)
	}
	if _, err := nilClient.OpenRunStream(
		context.Background(),
		"run-1",
		apicore.StreamCursor{},
	); !errors.Is(
		err,
		ErrDaemonClientRequired,
	) {
		t.Fatalf("nil OpenRunStream() error = %v, want ErrDaemonClientRequired", err)
	}

	client := &Client{
		target: Target{SocketPath: "/tmp/compozy.sock"},
	}
	var nilCtx context.Context
	if _, err := client.doJSON(
		nilCtx,
		http.MethodGet,
		"/api/daemon/health",
		nil,
		nil,
	); !errors.Is(
		err,
		ErrDaemonContextRequired,
	) {
		t.Fatalf("doJSON(nil context) error = %v, want ErrDaemonContextRequired", err)
	}

	var nilStream *clientRunStream
	if nilStream.Items() != nil {
		t.Fatal("nil stream Items() != nil, want nil")
	}
	if nilStream.Errors() != nil {
		t.Fatal("nil stream Errors() != nil, want nil")
	}
	if err := nilStream.Close(); err != nil {
		t.Fatalf("nil stream Close() error = %v, want nil", err)
	}

	resetStreamTimer(nil, time.Second)
}

func jsonStructResponse(t *testing.T, status int, payload any) *http.Response {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(%T) error = %v", payload, err)
	}
	return jsonResponse(status, string(body))
}
