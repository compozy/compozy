// Suite: Window-manager shared transport handlers.
// Invariants: WM-TRANSPORT-002 strict command schemas and status/body envelopes;
// WM-GO-014 workspace/client isolation.
// Owning layer: shared HTTP/UDS core handlers. Canonical suite: this file.
// Boundary IN: strict wire decoding, path/body identity, and public error mapping.
// Boundary OUT: reducer, persistence, and topology semantics belong to internal/windowmanager.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

func TestWindowManagerHandlers(t *testing.T) {
	t.Run("Should preview without writing, commit once, and expose revision conflicts", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)

		initial := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodGet,
			windowManagerTestPath("workspace-a"),
			"",
		)
		requireWindowManagerStatus(t, initial, http.StatusOK)
		var snapshot contract.WindowManagerSnapshot
		decodeWindowManagerTestBody(t, initial, &snapshot)
		if snapshot.WorkspaceID != "workspace-a" || snapshot.Revision != 0 || len(snapshot.Desktops) != 1 {
			t.Fatalf("initial snapshot = %+v", snapshot)
		}

		body := createDesktopCommandBody("workspace-a", 0, "desktop-second")
		preview := performWindowManagerTestRequest(
			t, fixture.router, http.MethodPost, windowManagerTestPath("workspace-a")+"/preview", body,
		)
		requireWindowManagerStatus(t, preview, http.StatusOK)
		var proposal contract.WindowManagerPreview
		decodeWindowManagerTestBody(t, preview, &proposal)
		if !proposal.Changed || proposal.Snapshot.Revision != 1 || len(proposal.Snapshot.Desktops) != 2 {
			t.Fatalf("preview = %+v", proposal)
		}

		unchanged := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodGet,
			windowManagerTestPath("workspace-a"),
			"",
		)
		requireWindowManagerStatus(t, unchanged, http.StatusOK)
		decodeWindowManagerTestBody(t, unchanged, &snapshot)
		if snapshot.Revision != 0 || len(snapshot.Desktops) != 1 {
			t.Fatalf("snapshot after preview = %+v", snapshot)
		}

		committed := performWindowManagerTestRequest(
			t, fixture.router, http.MethodPost, windowManagerTestPath("workspace-a")+"/commands", body,
		)
		requireWindowManagerStatus(t, committed, http.StatusOK)
		var result contract.WindowManagerResult
		decodeWindowManagerTestBody(t, committed, &result)
		if !result.Applied || result.Snapshot.Revision != 1 || len(result.Snapshot.Desktops) != 2 {
			t.Fatalf("command result = %+v", result)
		}

		stale := performWindowManagerTestRequest(
			t, fixture.router, http.MethodPost, windowManagerTestPath("workspace-a")+"/commands", body,
		)
		errorPayload := requireWindowManagerError(
			t, stale, http.StatusConflict, contract.WindowManagerErrorRevisionConflict,
		)
		if errorPayload.CurrentRevision == nil || *errorPayload.CurrentRevision != 1 {
			t.Fatalf("current_revision = %v", errorPayload.CurrentRevision)
		}
	})

	t.Run("Should reject unknown, mismatched, trailing, and unsafe command input", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)
		valid := createDesktopCommandBody("workspace-a", 0, "desktop-second")
		cases := map[string]string{
			"unknown top-level field": strings.TrimSuffix(valid, "}") + `,"surprise":true}`,
			"mismatched payload": `{"workspace_id":"workspace-a","command_id":"layout.undo","expected_revision":0,` +
				`"actor":{"kind":"test","id":"actor"},"origin":"core-test","payload":{"desktop_id":"desktop-default"}}`,
			"unknown command":     strings.Replace(valid, `"desktop.create"`, `"desktop.unknown"`, 1),
			"trailing JSON value": valid + `{}`,
			"unsafe revision": strings.Replace(
				valid,
				`"expected_revision":0`,
				`"expected_revision":9007199254740992`,
				1,
			),
		}
		for name, body := range cases {
			t.Run("Should reject "+name, func(t *testing.T) {
				t.Parallel()
				response := performWindowManagerTestRequest(
					t, fixture.router, http.MethodPost, windowManagerTestPath("workspace-a")+"/commands", body,
				)
				requireWindowManagerError(
					t, response, http.StatusUnprocessableEntity, contract.WindowManagerErrorInvalidCommand,
				)
			})
		}
	})

	t.Run("Should open and navigate a durable route through the discriminated wire contract", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)

		opened := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodPost,
			windowManagerTestPath("workspace-a")+"/commands",
			`{"workspace_id":"workspace-a","command_id":"window.open","expected_revision":0,`+
				`"actor":{"kind":"test","id":"actor"},"origin":"core-test","payload":{"window":{`+
				`"id":"window-a","app":"tasks","route":{"pathname":"/tasks","search":{"status":"open"}},`+
				`"desktop_id":"desktop-default","floating_rect":{"x":0.1,"y":0.1,"width":0.5,"height":0.5},`+
				`"insert_tiled":false}}}`,
		)
		requireWindowManagerStatus(t, opened, http.StatusOK)
		var result contract.WindowManagerResult
		decodeWindowManagerTestBody(t, opened, &result)
		window := result.Snapshot.Windows["window-a"]
		if window.Route.Pathname != "/tasks" ||
			string(window.Route.Search["status"]) != `"open"` ||
			window.Placement != windowmanager.WindowPlacementFloating {
			t.Fatalf("opened window = %+v", window)
		}

		navigated := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodPost,
			windowManagerTestPath("workspace-a")+"/commands",
			`{"workspace_id":"workspace-a","command_id":"window.navigate","expected_revision":1,`+
				`"actor":{"kind":"test","id":"actor"},"origin":"core-test",`+
				`"payload":{"window_id":"window-a","route":{"pathname":"/tasks/detail",`+
				`"search":{"task_id":"task-123"}}}}`,
		)
		requireWindowManagerStatus(t, navigated, http.StatusOK)
		decodeWindowManagerTestBody(t, navigated, &result)
		window = result.Snapshot.Windows["window-a"]
		if !result.Applied ||
			result.Snapshot.Revision != 2 ||
			window.Route.Pathname != "/tasks/detail" ||
			string(window.Route.Search["task_id"]) != `"task-123"` {
			t.Fatalf("navigate result = %+v", result)
		}

		for name, payload := range map[string]string{
			"legacy location":   `{"window_id":"window-a","location":"/legacy"}`,
			"non-object search": `{"window_id":"window-a","route":{"pathname":"/tasks","search":[]}}`,
		} {
			t.Run("Should reject "+name, func(t *testing.T) {
				t.Parallel()
				response := performWindowManagerTestRequest(
					t,
					fixture.router,
					http.MethodPost,
					windowManagerTestPath("workspace-a")+"/commands",
					`{"workspace_id":"workspace-a","command_id":"window.navigate","expected_revision":2,`+
						`"actor":{"kind":"test","id":"actor"},"origin":"core-test","payload":`+payload+`}`,
				)
				requireWindowManagerError(
					t,
					response,
					http.StatusUnprocessableEntity,
					contract.WindowManagerErrorInvalidCommand,
				)
			})
		}
	})

	t.Run("Should require registered presentation clients and preserve workspace isolation", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)

		missingClient := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodPost,
			windowManagerTestPath("workspace-a")+"/commands",
			`{"workspace_id":"workspace-a","command_id":"desktop.switch","expected_revision":0,`+
				`"actor":{"kind":"test","id":"actor"},"origin":"core-test",`+
				`"payload":{"desktop_id":"desktop-default"}}`,
		)
		requireWindowManagerError(t, missingClient, http.StatusNotFound, contract.WindowManagerErrorClientNotFound)

		registered := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodPost,
			windowManagerTestPath("workspace-a")+"/clients",
			`{"workspace_id":"workspace-a","client_id":" client-a "}`,
		)
		requireWindowManagerStatus(t, registered, http.StatusCreated)
		var client contract.WindowManagerClientView
		decodeWindowManagerTestBody(t, registered, &client)
		if client.WorkspaceID != "workspace-a" || client.ClientID != "client-a" ||
			client.PresentationRevision != 1 {
			t.Fatalf("registered client = %+v", client)
		}

		mismatch := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodPost,
			windowManagerTestPath("workspace-a")+"/clients",
			`{"workspace_id":"workspace-b","client_id":"client-b"}`,
		)
		requireWindowManagerError(
			t,
			mismatch,
			http.StatusUnprocessableEntity,
			contract.WindowManagerErrorInvalidCommand,
		)

		crossWorkspace := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodDelete,
			windowManagerTestPath("workspace-b")+"/clients/client-a",
			"",
		)
		requireWindowManagerError(t, crossWorkspace, http.StatusNotFound, contract.WindowManagerErrorClientNotFound)

		workspaceA := listWindowManagerTestClients(t, fixture.router, "workspace-a")
		workspaceB := listWindowManagerTestClients(t, fixture.router, "workspace-b")
		if len(workspaceA.Clients) != 1 || len(workspaceB.Clients) != 0 {
			t.Fatalf("client partitions: workspace-a=%+v workspace-b=%+v", workspaceA, workspaceB)
		}

		removed := performWindowManagerTestRequest(
			t, fixture.router, http.MethodDelete, windowManagerTestPath("workspace-a")+"/clients/client-a", "",
		)
		requireWindowManagerStatus(t, removed, http.StatusNoContent)
		if clients := listWindowManagerTestClients(t, fixture.router, "workspace-a"); len(clients.Clients) != 0 {
			t.Fatalf("clients after delete = %+v", clients)
		}
	})

	t.Run("Should serialize required empty arrays instead of null", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)

		snapshotResponse := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodGet,
			windowManagerTestPath("workspace-a"),
			"",
		)
		requireWindowManagerStatus(t, snapshotResponse, http.StatusOK)
		var snapshot struct {
			Desktops []struct {
				Floating json.RawMessage `json:"floating"`
			} `json:"desktops"`
		}
		decodeWindowManagerTestBody(t, snapshotResponse, &snapshot)
		if len(snapshot.Desktops) != 1 {
			t.Fatalf("snapshot desktops = %+v, want one", snapshot.Desktops)
		}
		if string(snapshot.Desktops[0].Floating) != "[]" {
			t.Fatalf("desktop floating JSON = %s, want []", snapshot.Desktops[0].Floating)
		}
		convertedSnapshot, err := contract.WindowManagerSnapshotFromDomain(windowmanager.Snapshot{
			WorkspaceID: "workspace-a",
			Desktops: []windowmanager.Desktop{{
				ID: "desktop-empty", Floating: nil,
			}},
		})
		if err != nil {
			t.Fatalf("WindowManagerSnapshotFromDomain() error = %v", err)
		}
		convertedJSON, err := json.Marshal(convertedSnapshot)
		if err != nil {
			t.Fatalf("json.Marshal(converted snapshot) error = %v", err)
		}
		var converted struct {
			Desktops []struct {
				Floating json.RawMessage `json:"floating"`
			} `json:"desktops"`
		}
		if err := json.Unmarshal(convertedJSON, &converted); err != nil {
			t.Fatalf("json.Unmarshal(converted snapshot) error = %v", err)
		}
		if len(converted.Desktops) != 1 {
			t.Fatalf("converted snapshot desktops = %+v, want one", converted.Desktops)
		}
		if string(converted.Desktops[0].Floating) != "[]" {
			t.Fatalf("converted desktop floating JSON = %s, want []", converted.Desktops[0].Floating)
		}

		clientResponse := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodPost,
			windowManagerTestPath("workspace-a")+"/clients",
			`{"workspace_id":"workspace-a","client_id":"client-a"}`,
		)
		requireWindowManagerStatus(t, clientResponse, http.StatusCreated)
		var client struct {
			FocusOrder json.RawMessage `json:"focus_order"`
		}
		decodeWindowManagerTestBody(t, clientResponse, &client)
		if string(client.FocusOrder) != "[]" {
			t.Fatalf("client focus_order JSON = %s, want []", client.FocusOrder)
		}
	})

	t.Run("Should export and validate a workspace-bound layout", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)
		exported := performWindowManagerTestRequest(
			t, fixture.router, http.MethodGet, windowManagerTestPath("workspace-a")+"/layout", "",
		)
		requireWindowManagerStatus(t, exported, http.StatusOK)
		var document contract.WindowManagerLayoutDocument
		decodeWindowManagerTestBody(t, exported, &document)
		if document.WorkspaceID != "workspace-a" || len(document.Desktops) != 1 {
			t.Fatalf("layout document = %+v", document)
		}

		validationBody := marshalWindowManagerTestJSON(t, contract.WindowManagerLayoutValidationRequest{
			WorkspaceID: "workspace-a", Document: document,
		})
		validated := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodPost,
			windowManagerTestPath("workspace-a")+"/layout/validate",
			validationBody,
		)
		requireWindowManagerStatus(t, validated, http.StatusOK)
		var validation contract.WindowManagerLayoutValidationResponse
		decodeWindowManagerTestBody(t, validated, &validation)
		if !validation.Valid || validation.WorkspaceID != "workspace-a" {
			t.Fatalf("layout validation = %+v", validation)
		}
	})

	t.Run("Should expose only global and route-workspace layout profiles", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)
		now := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
		records := map[string]resources.RawRecord{
			"global-profile": {
				Kind: windowmanager.WindowLayoutResourceKind, ID: "global-profile", Version: 1,
				Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
				SpecJSON: []byte(`{"version":1}`), CreatedAt: now, UpdatedAt: now,
			},
			"workspace-a-profile": {
				Kind: windowmanager.WindowLayoutResourceKind, ID: "workspace-a-profile", Version: 2,
				Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-a"},
				SpecJSON: []byte(`{"version":1}`), CreatedAt: now, UpdatedAt: now,
			},
			"workspace-b-profile": {
				Kind: windowmanager.WindowLayoutResourceKind, ID: "workspace-b-profile", Version: 3,
				Scope:    resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "workspace-b"},
				SpecJSON: []byte(`{"version":1}`), CreatedAt: now, UpdatedAt: now,
			},
		}
		putCalled := false
		fixture.handlers.Resources = stubResourceService{
			ListFn: func(_ context.Context, filter resources.ResourceFilter) ([]resources.RawRecord, error) {
				if filter.Kind != windowmanager.WindowLayoutResourceKind || filter.Scope == nil {
					t.Fatalf("layout-profile filter = %+v", filter)
				}
				if filter.Scope.Kind == resources.ResourceScopeKindGlobal {
					return []resources.RawRecord{records["global-profile"]}, nil
				}
				if filter.Scope.ID == "workspace-a" {
					return []resources.RawRecord{records["workspace-a-profile"]}, nil
				}
				return []resources.RawRecord{records["workspace-b-profile"]}, nil
			},
			GetFn: func(_ context.Context, _ resources.ResourceKind, id string) (resources.RawRecord, error) {
				record, ok := records[id]
				if !ok {
					return resources.RawRecord{}, resources.ErrNotFound
				}
				return record, nil
			},
			PutFn: func(_ context.Context, _ resources.RawDraft) (resources.RawRecord, error) {
				putCalled = true
				return resources.RawRecord{}, nil
			},
		}

		listed := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodGet,
			windowManagerTestPath("workspace-a")+"/layout-profiles",
			"",
		)
		requireWindowManagerStatus(t, listed, http.StatusOK)
		var payload contract.ResourcesResponse
		decodeWindowManagerTestBody(t, listed, &payload)
		if len(payload.Records) != 2 ||
			payload.Records[0].ID != "global-profile" ||
			payload.Records[1].ID != "workspace-a-profile" {
			t.Fatalf("visible layout profiles = %+v", payload.Records)
		}

		foreignPut := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodPut,
			windowManagerTestPath("workspace-a")+"/layout-profiles/foreign-profile",
			`{"scope":{"kind":"workspace","id":"workspace-b"},"expected_version":0,"spec":{"version":1}}`,
		)
		requireWindowManagerStatus(t, foreignPut, http.StatusForbidden)
		if putCalled {
			t.Fatal("foreign workspace profile reached ResourceService.Put")
		}

		foreignDelete := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodDelete,
			windowManagerTestPath("workspace-a")+"/layout-profiles/workspace-b-profile",
			`{"expected_version":3}`,
		)
		requireWindowManagerStatus(t, foreignDelete, http.StatusNotFound)
	})

	t.Run("Should map stream lifecycle failures to stable structured errors", func(t *testing.T) {
		t.Parallel()
		status, payload := windowManagerErrorPayload("workspace-a", windowmanager.ErrSlowConsumer)
		if status != http.StatusConflict || payload.Code != contract.WindowManagerErrorSlowConsumer {
			t.Fatalf("slow-consumer error = status %d payload %+v", status, payload)
		}
		status, payload = windowManagerErrorPayload(
			"workspace-a",
			windowmanager.ErrPresentationRevisionExhausted,
		)
		if status != http.StatusServiceUnavailable ||
			payload.Code != contract.WindowManagerErrorUnavailable {
			t.Fatalf("presentation-revision error = status %d payload %+v", status, payload)
		}
		if !isExpectedWindowManagerSocketError(net.ErrClosed) {
			t.Fatal("net.ErrClosed was classified as an unexpected socket failure")
		}
	})
}

type windowManagerHandlerFixture struct {
	router   *gin.Engine
	handlers *BaseHandlers
	manager  *windowmanager.Manager
}

func newWindowManagerHandlerFixture(t *testing.T) windowManagerHandlerFixture {
	t.Helper()
	var sequence atomic.Int64
	manager, err := windowmanager.NewService(
		windowmanager.NewMemoryRepository(),
		windowmanager.NewMemoryWorkspaceResolver("workspace-a", "workspace-b"),
		nil,
		windowmanager.DefaultConfig(),
		windowmanager.WithClock(func() time.Time { return time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC) }),
		windowmanager.WithIDGenerator(func(kind string) (string, error) {
			return fmt.Sprintf("%s-%03d", kind, sequence.Add(1)), nil
		}),
	)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	handlers := NewBaseHandlers(&BaseHandlerConfig{WindowManager: manager})
	router := gin.New()
	registerWindowManagerTestRoutes(router, handlers)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cancel()
		if shutdownErr := handlers.ShutdownWindowManagerStreams(ctx); shutdownErr != nil {
			t.Errorf("ShutdownWindowManagerStreams() error = %v", shutdownErr)
		}
		if closeErr := manager.Close(); closeErr != nil {
			t.Errorf("Manager.Close() error = %v", closeErr)
		}
	})
	return windowManagerHandlerFixture{router: router, handlers: handlers, manager: manager}
}

func registerWindowManagerTestRoutes(router gin.IRouter, handlers *BaseHandlers) {
	manager := router.Group("/api/workspaces/:workspace_id/window-manager")
	manager.GET("", handlers.GetWindowManagerSnapshot)
	manager.POST("/preview", handlers.PreviewWindowManagerCommand)
	manager.POST("/commands", handlers.ExecuteWindowManagerCommand)
	manager.GET("/clients", handlers.ListWindowManagerClients)
	manager.POST("/clients", handlers.RegisterWindowManagerClient)
	manager.DELETE("/clients/:client_id", handlers.UnregisterWindowManagerClient)
	manager.GET("/layout", handlers.ExportWindowManagerLayout)
	manager.POST("/layout/validate", handlers.ValidateWindowManagerLayout)
	manager.PUT("/layout", handlers.ReplaceWindowManagerLayout)
	manager.GET("/layout-profiles", handlers.ListWindowManagerLayoutProfiles)
	manager.PUT("/layout-profiles/:profile_id", handlers.PutWindowManagerLayoutProfile)
	manager.DELETE("/layout-profiles/:profile_id", handlers.DeleteWindowManagerLayoutProfile)
	manager.GET("/stream", handlers.StreamWindowManager)
}

func windowManagerTestPath(workspaceID string) string {
	return "/api/workspaces/" + workspaceID + "/window-manager"
}

func createDesktopCommandBody(workspaceID string, revision uint64, desktopID string) string {
	return fmt.Sprintf(
		`{"workspace_id":%q,"command_id":"desktop.create","expected_revision":%d,`+
			`"actor":{"kind":"test","id":"actor"},"origin":"core-test",`+
			`"payload":{"desktop_id":%q,"name":"Second","purpose":"standard"}}`,
		workspaceID,
		revision,
		desktopID,
	)
}

func performWindowManagerTestRequest(
	t *testing.T,
	router http.Handler,
	method string,
	path string,
	body string,
) *httptest.ResponseRecorder {
	t.Helper()
	request, err := http.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequestWithContext() error = %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func requireWindowManagerStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, want, response.Body.String())
	}
}

func requireWindowManagerError(
	t *testing.T,
	response *httptest.ResponseRecorder,
	status int,
	code contract.WindowManagerErrorCode,
) contract.WindowManagerErrorPayload {
	t.Helper()
	requireWindowManagerStatus(t, response, status)
	var payload contract.WindowManagerErrorPayload
	decodeWindowManagerTestBody(t, response, &payload)
	if payload.Code != code || payload.Error != string(code) {
		t.Fatalf("error payload = %+v, want code %q", payload, code)
	}
	return payload
}

func decodeWindowManagerTestBody(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response body %q: %v", response.Body.String(), err)
	}
}

func marshalWindowManagerTestJSON(t *testing.T, value any) string {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return string(payload)
}

func listWindowManagerTestClients(
	t *testing.T,
	router http.Handler,
	workspaceID string,
) contract.WindowManagerClientsResponse {
	t.Helper()
	response := performWindowManagerTestRequest(
		t, router, http.MethodGet, windowManagerTestPath(workspaceID)+"/clients", "",
	)
	requireWindowManagerStatus(t, response, http.StatusOK)
	var clients contract.WindowManagerClientsResponse
	decodeWindowManagerTestBody(t, response, &clients)
	return clients
}
