// Suite: Window manager CLI contract
// Invariant: public desktop, window, and layout verbs preserve workspace, revision, client, payload, and stream ordering.
// Boundary IN: Cobra parsing, CLI transport dispatch, structured output, and deterministic client-side validation.
// Boundary OUT: command reduction and topology persistence are owned by internal/windowmanager and daemon integration suites.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/gorilla/websocket"
)

func TestUnixSocketClientWindowManagerRoutes(t *testing.T) {
	t.Parallel()

	t.Run("Should target every canonical UDS route and verb", func(t *testing.T) {
		t.Parallel()

		type requestTarget struct {
			method string
			path   string
			body   any
		}
		encode := func(value any) string {
			t.Helper()
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatalf("marshal response fixture: %v", err)
			}
			return string(data)
		}
		normalize := func(value any) any {
			t.Helper()
			data, err := json.Marshal(value)
			if err != nil {
				t.Fatalf("marshal request fixture: %v", err)
			}
			var normalized any
			if err := json.Unmarshal(data, &normalized); err != nil {
				t.Fatalf("normalize request fixture: %v", err)
			}
			return normalized
		}

		snapshot := windowManagerTestSnapshot(7)
		result := windowManagerTestResult()
		document := windowManagerTestLayoutDocument()
		clientView := windowManagerTestClientView("browser-a")
		seen := make([]requestTarget, 0, 9)
		transport := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			var body any
			if req.Body != nil && req.Body != http.NoBody {
				if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
					t.Fatalf("decode %s %s request body: %v", req.Method, req.URL.EscapedPath(), err)
				}
			}
			seen = append(seen, requestTarget{method: req.Method, path: req.URL.EscapedPath(), body: body})
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/api/workspaces/w1/window-manager":
				return newHTTPResponse(http.StatusOK, encode(snapshot)), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/workspaces/w1/window-manager/preview":
				return newHTTPResponse(http.StatusOK, encode(contract.WindowManagerPreview{Snapshot: snapshot})), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/workspaces/w1/window-manager/commands":
				return newHTTPResponse(http.StatusOK, encode(result)), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/workspaces/w1/window-manager/clients":
				response := contract.WindowManagerClientsResponse{WorkspaceID: "w1"}
				return newHTTPResponse(http.StatusOK, encode(response)), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/workspaces/w1/window-manager/clients":
				return newHTTPResponse(http.StatusOK, encode(clientView)), nil
			case req.Method == http.MethodDelete && req.URL.EscapedPath() == "/api/workspaces/w1/window-manager/clients/browser%2Fa":
				return newHTTPResponse(http.StatusNoContent, ""), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/workspaces/w1/window-manager/layout":
				return newHTTPResponse(http.StatusOK, encode(document)), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/workspaces/w1/window-manager/layout/validate":
				response := contract.WindowManagerLayoutValidationResponse{WorkspaceID: "w1", Valid: true}
				return newHTTPResponse(http.StatusOK, encode(response)), nil
			case req.Method == http.MethodPut && req.URL.Path == "/api/workspaces/w1/window-manager/layout":
				return newHTTPResponse(http.StatusOK, encode(result)), nil
			default:
				t.Fatalf("unexpected request = %s %s", req.Method, req.URL.EscapedPath())
				return nil, errors.New("unexpected window-manager request")
			}
		})
		client := &unixSocketClient{
			socketPath: "/tmp/agh-window-manager-test.sock",
			httpClient: &http.Client{Transport: transport},
		}

		command := contract.WindowManagerCommandRequest{
			WorkspaceID: "w1",
			CommandID:   contract.WindowManagerCommandDesktopCreate,
			Payload:     json.RawMessage(`{"desktop_id":"d2"}`),
		}
		if _, err := client.GetWindowManagerSnapshot(t.Context(), "w1"); err != nil {
			t.Fatalf("GetWindowManagerSnapshot() error = %v", err)
		}
		if _, err := client.PreviewWindowManagerCommand(t.Context(), "w1", command); err != nil {
			t.Fatalf("PreviewWindowManagerCommand() error = %v", err)
		}
		if _, err := client.ExecuteWindowManagerCommand(t.Context(), "w1", command); err != nil {
			t.Fatalf("ExecuteWindowManagerCommand() error = %v", err)
		}
		if _, err := client.ListWindowManagerClients(t.Context(), "w1"); err != nil {
			t.Fatalf("ListWindowManagerClients() error = %v", err)
		}
		registration := contract.WindowManagerClientRegistration{
			WorkspaceID:     "w1",
			ClientID:        "browser-a",
			ActiveDesktopID: "d1",
		}
		if _, err := client.RegisterWindowManagerClient(t.Context(), "w1", registration); err != nil {
			t.Fatalf("RegisterWindowManagerClient() error = %v", err)
		}
		if err := client.UnregisterWindowManagerClient(t.Context(), "w1", "browser/a"); err != nil {
			t.Fatalf("UnregisterWindowManagerClient() error = %v", err)
		}
		if _, err := client.ExportWindowManagerLayout(t.Context(), "w1"); err != nil {
			t.Fatalf("ExportWindowManagerLayout() error = %v", err)
		}
		validation := contract.WindowManagerLayoutValidationRequest{WorkspaceID: "w1", Document: document}
		if _, err := client.ValidateWindowManagerLayout(t.Context(), "w1", validation); err != nil {
			t.Fatalf("ValidateWindowManagerLayout() error = %v", err)
		}
		replacement := contract.WindowManagerLayoutReplaceRequest{WorkspaceID: "w1", Document: document}
		if _, err := client.ApplyWindowManagerLayout(t.Context(), "w1", replacement); err != nil {
			t.Fatalf("ApplyWindowManagerLayout() error = %v", err)
		}

		want := []requestTarget{
			{method: http.MethodGet, path: "/api/workspaces/w1/window-manager"},
			{method: http.MethodPost, path: "/api/workspaces/w1/window-manager/preview", body: normalize(command)},
			{method: http.MethodPost, path: "/api/workspaces/w1/window-manager/commands", body: normalize(command)},
			{method: http.MethodGet, path: "/api/workspaces/w1/window-manager/clients"},
			{method: http.MethodPost, path: "/api/workspaces/w1/window-manager/clients", body: normalize(registration)},
			{method: http.MethodDelete, path: "/api/workspaces/w1/window-manager/clients/browser%2Fa"},
			{method: http.MethodGet, path: "/api/workspaces/w1/window-manager/layout"},
			{
				method: http.MethodPost,
				path:   "/api/workspaces/w1/window-manager/layout/validate",
				body:   normalize(validation),
			},
			{method: http.MethodPut, path: "/api/workspaces/w1/window-manager/layout", body: normalize(replacement)},
		}
		if len(seen) != len(want) {
			t.Fatalf("request count = %d, want %d: %#v", len(seen), len(want), seen)
		}
		for index := range want {
			if !reflect.DeepEqual(seen[index], want[index]) {
				t.Fatalf("request %d = %#v, want %#v", index, seen[index], want[index])
			}
		}
	})

	t.Run("Should send the bound watch query and enforce the initial fence over a Unix socket", func(t *testing.T) {
		t.Parallel()

		socketDir, err := os.MkdirTemp("/tmp", "agh-wm-cli-")
		if err != nil {
			t.Fatalf("os.MkdirTemp() error = %v", err)
		}
		t.Cleanup(func() {
			if removeErr := os.RemoveAll(socketDir); removeErr != nil {
				t.Errorf("os.RemoveAll(%q) error = %v", socketDir, removeErr)
			}
		})
		socketPath := filepath.Join(socketDir, "wm.sock")
		listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", socketPath)
		if err != nil {
			t.Fatalf("Listen(unix) error = %v", err)
		}
		requests := make(chan *http.Request, 2)
		handlerResults := make(chan error, 2)
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		server := &http.Server{Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			requests <- request.Clone(request.Context())
			connection, upgradeErr := upgrader.Upgrade(writer, request, nil)
			if upgradeErr != nil {
				handlerResults <- upgradeErr
				return
			}
			var handlerErr error
			defer func() {
				handlerResults <- errors.Join(handlerErr, connection.Close())
			}()

			if request.URL.Query().Get("client_id") == "" {
				handlerErr = connection.WriteJSON(contract.WindowManagerEventFrame{
					Type: contract.WindowManagerFrameEvent, WorkspaceID: "w1", Revision: 8,
				})
				return
			}
			clientView := windowManagerTestClientView("browser-a")
			frames := []any{
				contract.WindowManagerSnapshotFrame{
					Type: contract.WindowManagerFrameSnapshot, WorkspaceID: "w1", Revision: 7,
					Snapshot: windowManagerTestSnapshot(7), Client: &clientView,
				},
				contract.WindowManagerEventFrame{
					Type: contract.WindowManagerFrameEvent, WorkspaceID: "w1", Revision: 8,
					Event: contract.WindowManagerEvent{
						WorkspaceID: "w1", Revision: 8,
						CommandID: contract.WindowManagerCommandDesktopCreate,
					},
				},
				contract.WindowManagerClientFrame{
					Type: contract.WindowManagerFrameClient, WorkspaceID: "w1", Revision: 8,
					Client: clientView,
				},
			}
			for _, frame := range frames {
				if writeErr := connection.WriteJSON(frame); writeErr != nil {
					handlerErr = writeErr
					return
				}
			}
		})}
		serveResult := make(chan error, 1)
		go func() {
			serveResult <- server.Serve(listener)
		}()
		t.Cleanup(func() {
			shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
			defer cancel()
			if shutdownErr := server.Shutdown(shutdownCtx); shutdownErr != nil {
				t.Errorf("server.Shutdown() error = %v", shutdownErr)
			}
			if serveErr := <-serveResult; serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
				t.Errorf("server.Serve() error = %v", serveErr)
			}
		})

		client := &unixSocketClient{socketPath: socketPath}
		afterRevision := contract.WindowManagerRevision(4)
		clientID := windowmanager.ClientID("browser-a")
		stopWatch := errors.New("stop after client frame")
		seenFrames := make([]string, 0, 3)
		err = client.WatchWindowManager(
			t.Context(),
			"w1",
			&clientID,
			&afterRevision,
			WindowManagerStreamHandlers{
				Snapshot: func(contract.WindowManagerSnapshotFrame) error {
					seenFrames = append(seenFrames, contract.WindowManagerFrameSnapshot)
					return nil
				},
				Event: func(contract.WindowManagerEventFrame) error {
					seenFrames = append(seenFrames, contract.WindowManagerFrameEvent)
					return nil
				},
				Client: func(contract.WindowManagerClientFrame) error {
					seenFrames = append(seenFrames, contract.WindowManagerFrameClient)
					return stopWatch
				},
			},
		)
		if !errors.Is(err, stopWatch) {
			t.Fatalf("WatchWindowManager() error = %v, want callback stop", err)
		}
		if !reflect.DeepEqual(
			seenFrames,
			[]string{
				contract.WindowManagerFrameSnapshot,
				contract.WindowManagerFrameEvent,
				contract.WindowManagerFrameClient,
			},
		) {
			t.Fatalf("watch frames = %v, want snapshot event client", seenFrames)
		}
		boundRequest := <-requests
		if boundRequest.URL.EscapedPath() != "/api/workspaces/w1/window-manager/stream" ||
			boundRequest.URL.Query().Get("after_revision") != "4" ||
			boundRequest.URL.Query().Get("client_id") != "browser-a" {
			t.Fatalf("bound watch request = %s?%s", boundRequest.URL.EscapedPath(), boundRequest.URL.RawQuery)
		}

		err = client.WatchWindowManager(
			t.Context(),
			"w1",
			nil,
			nil,
			WindowManagerStreamHandlers{
				Snapshot: func(contract.WindowManagerSnapshotFrame) error { return nil },
				Event:    func(contract.WindowManagerEventFrame) error { return nil },
			},
		)
		if err == nil || !strings.Contains(err.Error(), "event arrived before snapshot") {
			t.Fatalf("unbound WatchWindowManager() error = %v, want initial-fence violation", err)
		}
		unboundRequest := <-requests
		if unboundRequest.URL.Query().Has("client_id") || unboundRequest.URL.Query().Has("after_revision") {
			t.Fatalf("unbound watch query = %q, want empty", unboundRequest.URL.RawQuery)
		}

		for range 2 {
			if handlerErr := <-handlerResults; handlerErr != nil && !errors.Is(handlerErr, net.ErrClosed) {
				t.Fatalf("window-manager WebSocket handler error = %v", handlerErr)
			}
		}
	})
}

func TestWindowManagerMutationCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		args            []string
		commandID       contract.WindowManagerCommandID
		expectedPayload string
		expectedClient  string
		expectedRoute   *windowmanager.RouteIntent
	}{
		{
			name:            "Should create a desktop",
			commandID:       contract.WindowManagerCommandDesktopCreate,
			expectedPayload: `{"desktop_id":"d2","name":"Build","purpose":"standard"}`,
			args: []string{
				"desktop",
				"create",
				"--workspace",
				"w1",
				"--revision",
				"7",
				"--id",
				"d2",
				"--name",
				"Build",
			},
		},
		{
			name:            "Should update a desktop",
			commandID:       contract.WindowManagerCommandDesktopUpdate,
			expectedPayload: `{"desktop_id":"d1","name":"Plan"}`,
			args: []string{
				"desktop",
				"update",
				"--workspace",
				"w1",
				"--revision",
				"7",
				"--id",
				"d1",
				"--name",
				"Plan",
			},
		},
		{
			name:            "Should reorder a desktop to zero",
			commandID:       contract.WindowManagerCommandDesktopReorder,
			expectedPayload: `{"desktop_id":"d2","order":0}`,
			args: []string{
				"desktop",
				"reorder",
				"--workspace",
				"w1",
				"--revision",
				"7",
				"--id",
				"d2",
				"--order",
				"0",
			},
		},
		{
			name:            "Should switch an explicit client desktop",
			commandID:       contract.WindowManagerCommandDesktopSwitch,
			expectedPayload: `{"desktop_id":"d2"}`,
			expectedClient:  "browser-a",
			args: []string{
				"desktop", "switch", "--workspace", "w1", "--revision", "7", "--client", "browser-a", "--id", "d2",
			},
		},
		{
			name:            "Should delete a desktop with a destination",
			commandID:       contract.WindowManagerCommandDesktopDelete,
			expectedPayload: `{"desktop_id":"d2","destination_id":"d1"}`,
			args: []string{
				"desktop", "delete", "--workspace", "w1", "--revision", "7", "--id", "d2", "--destination", "d1",
			},
		},
		{
			name:            "Should open a window",
			commandID:       contract.WindowManagerCommandWindowOpen,
			expectedPayload: `{"window":{"id":"win-3","app":"tasks","route":{"pathname":"/tasks","search":{"status":"open"}},"desktop_id":"d1","floating_rect":{"x":0.1,"y":0.2,"width":0.5,"height":0.6},"insert_tiled":false}}`,
			args: []string{
				"window", "open", "--workspace", "w1", "--revision", "7", "--id", "win-3", "--app", "tasks",
				"--desktop", "d1", "--rect", "0.1,0.2,0.5,0.6", "--pathname", "/tasks",
				"--search-json", `{"status":"open"}`,
			},
			expectedRoute: &windowmanager.RouteIntent{
				Pathname: "/tasks",
				Search:   windowmanager.RouteSearch{"status": json.RawMessage(`"open"`)},
			},
		},
		{
			name:            "Should navigate a window and optionally focus its client",
			commandID:       contract.WindowManagerCommandWindowNavigate,
			expectedPayload: `{"window_id":"win-1","route":{"pathname":"/tasks/detail","search":{"task_id":"task-123"}}}`,
			expectedClient:  "browser-a",
			args: []string{
				"window", "navigate", "--workspace", "w1", "--revision", "7", "--client", "browser-a",
				"--id", "win-1", "--pathname", "/tasks/detail",
				"--search-json", `{"task_id":"task-123"}`,
			},
			expectedRoute: &windowmanager.RouteIntent{
				Pathname: "/tasks/detail",
				Search:   windowmanager.RouteSearch{"task_id": json.RawMessage(`"task-123"`)},
			},
		},
		{
			name:            "Should close a window",
			commandID:       contract.WindowManagerCommandWindowClose,
			expectedPayload: `{"window_id":"win-1","minimize":true}`,
			args: []string{
				"window",
				"close",
				"--workspace",
				"w1",
				"--revision",
				"7",
				"--id",
				"win-1",
				"--minimize",
			},
		},
		{
			name:            "Should focus a window for an explicit client",
			commandID:       contract.WindowManagerCommandWindowFocus,
			expectedPayload: `{"window_id":"win-1"}`,
			expectedClient:  "browser-a",
			args: []string{
				"window", "focus", "--workspace", "w1", "--revision", "7", "--client", "browser-a", "--id", "win-1",
			},
		},
		{
			name:            "Should move a floating window",
			commandID:       contract.WindowManagerCommandWindowMove,
			expectedPayload: `{"window_id":"win-1","destination_desktop_id":"d2","placement":"floating","move_group":false}`,
			args: []string{
				"window", "move", "--workspace", "w1", "--revision", "7", "--id", "win-1", "--desktop", "d2",
				"--placement", "floating",
			},
		},
		{
			name:            "Should swap two windows",
			commandID:       contract.WindowManagerCommandWindowSwap,
			expectedPayload: `{"first_window_id":"win-1","second_window_id":"win-2"}`,
			args: []string{
				"window", "swap", "--workspace", "w1", "--revision", "7", "--first", "win-1", "--second", "win-2",
			},
		},
		{
			name:            "Should toggle floating placement",
			commandID:       contract.WindowManagerCommandWindowToggleFloating,
			expectedPayload: `{"window_id":"win-1"}`,
			args:            []string{"window", "float", "--workspace", "w1", "--revision", "7", "--id", "win-1"},
		},
		{
			name:            "Should zoom a window for an explicit client",
			commandID:       contract.WindowManagerCommandWindowZoom,
			expectedPayload: `{"window_id":"win-1"}`,
			expectedClient:  "browser-a",
			args: []string{
				"window", "zoom", "--workspace", "w1", "--revision", "7", "--client", "browser-a", "--id", "win-1",
			},
		},
		{
			name:            "Should arrange explicit windows",
			commandID:       contract.WindowManagerCommandLayoutArrange,
			expectedPayload: `{"desktop_id":"d1","window_ids":["win-1","win-2"],"arrangement":"horizontal","frame":{"x":0,"y":0,"width":0,"height":0},"group_id":""}`,
			args: []string{
				"layout", "arrange", "--workspace", "w1", "--revision", "7", "--desktop", "d1", "--window", "win-1",
				"--window", "win-2", "--arrangement", "horizontal",
			},
		},
		{
			name:            "Should arrange from one declarative resource",
			commandID:       contract.WindowManagerCommandLayoutArrange,
			expectedPayload: `{"resource_id":"focused-work"}`,
			args: []string{
				"layout", "arrange", "--workspace", "w1", "--revision", "7", "--resource", "focused-work",
			},
		},
		{
			name:            "Should resize a shared boundary",
			commandID:       contract.WindowManagerCommandLayoutResize,
			expectedPayload: `{"split_id":"node-1","boundary_index":0,"delta":0.1}`,
			args: []string{
				"layout", "resize", "--workspace", "w1", "--revision", "7", "--split", "node-1", "--boundary", "0",
				"--delta", "0.1",
			},
		},
		{
			name:            "Should balance a group",
			commandID:       contract.WindowManagerCommandLayoutBalance,
			expectedPayload: `{"group_id":"group-1"}`,
			args: []string{
				"layout", "balance", "--workspace", "w1", "--revision", "7", "--group", "group-1",
			},
		},
		{
			name:            "Should undo one operation",
			commandID:       contract.WindowManagerCommandLayoutUndo,
			expectedPayload: `{}`,
			args:            []string{"layout", "undo", "--workspace", "w1", "--revision", "7"},
		},
		{
			name:            "Should redo one operation",
			commandID:       contract.WindowManagerCommandLayoutRedo,
			expectedPayload: `{}`,
			args:            []string{"layout", "redo", "--workspace", "w1", "--revision", "7"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var captured contract.WindowManagerCommandRequest
			client := newWindowManagerCommandStub()
			client.executeFn = func(
				_ context.Context,
				workspace string,
				request contract.WindowManagerCommandRequest,
			) (contract.WindowManagerResult, error) {
				if workspace != "w1" {
					t.Fatalf("workspace = %q, want w1", workspace)
				}
				captured = request
				return windowManagerTestResult(), nil
			}

			args := append(append([]string(nil), tt.args...), "-o", "json")
			stdout, _, err := executeRootCommand(t, newTestDeps(t, client), args...)
			if err != nil {
				t.Fatalf("command error = %v", err)
			}
			if captured.CommandID != tt.commandID || captured.WorkspaceID != "w1" ||
				captured.ExpectedRevision == nil || *captured.ExpectedRevision != 7 {
				t.Fatalf("request = %#v, want %s in w1 at revision 7", captured, tt.commandID)
			}
			if tt.expectedClient == "" {
				if captured.ClientID != nil {
					t.Fatalf("client = %v, want nil", captured.ClientID)
				}
			} else if captured.ClientID == nil || string(*captured.ClientID) != tt.expectedClient {
				t.Fatalf("client = %v, want %q", captured.ClientID, tt.expectedClient)
			}
			if !windowManagerJSONEqual(captured.Payload, []byte(tt.expectedPayload)) {
				t.Fatalf("payload = %s, want %s", captured.Payload, tt.expectedPayload)
			}
			if tt.expectedRoute != nil {
				route := decodeWindowManagerCommandRoute(t, captured.CommandID, captured.Payload)
				if !reflect.DeepEqual(route, *tt.expectedRoute) {
					t.Fatalf("route = %#v, want %#v", route, *tt.expectedRoute)
				}
			}
			var result contract.WindowManagerResult
			decodeWindowManagerCommandJSON(t, stdout, &result)
			if !result.Applied || result.Snapshot.Revision != 8 {
				t.Fatalf("result = %#v, want applied revision 8", result)
			}
		})
	}
}

func TestWindowManagerReadAndClientCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should list desktops and windows from the authoritative snapshot", func(t *testing.T) {
		t.Parallel()
		client := newWindowManagerCommandStub()
		client.snapshotFn = func(_ context.Context, workspace string) (contract.WindowManagerSnapshot, error) {
			if workspace != "w1" {
				t.Fatalf("workspace = %q, want w1", workspace)
			}
			return windowManagerTestSnapshot(7), nil
		}
		deps := newTestDeps(t, client)

		desktopOutput, _, err := executeRootCommand(t, deps, "desktop", "list", "--workspace", "w1", "-o", "json")
		if err != nil {
			t.Fatalf("desktop list error = %v", err)
		}
		var desktops []contract.WindowManagerDesktop
		decodeWindowManagerCommandJSON(t, desktopOutput, &desktops)
		if len(desktops) != 2 || desktops[1].ID != "d2" {
			t.Fatalf("desktops = %#v, want d1 and d2", desktops)
		}

		windowOutput, _, err := executeRootCommand(t, deps, "window", "list", "--workspace", "w1", "-o", "json")
		if err != nil {
			t.Fatalf("window list error = %v", err)
		}
		var windows []contract.WindowManagerWindow
		decodeWindowManagerCommandJSON(t, windowOutput, &windows)
		if len(windows) != 2 || windows[0].ID != "win-1" || windows[1].ID != "win-2" {
			t.Fatalf("windows = %#v, want stable ID order", windows)
		}
	})

	t.Run("Should register list and unregister explicit clients", func(t *testing.T) {
		t.Parallel()
		client := newWindowManagerCommandStub()
		view := windowManagerTestClientView("browser-a")
		client.registerClientFn = func(
			_ context.Context,
			workspace string,
			request contract.WindowManagerClientRegistration,
		) (contract.WindowManagerClientView, error) {
			if workspace != "w1" || request.WorkspaceID != "w1" || request.ClientID != "browser-a" ||
				request.ActiveDesktopID != "d2" {
				t.Fatalf("registration = %#v for workspace %q", request, workspace)
			}
			return view, nil
		}
		client.listClientsFn = func(_ context.Context, workspace string) (contract.WindowManagerClientsResponse, error) {
			return contract.WindowManagerClientsResponse{
				WorkspaceID: windowmanager.WorkspaceID(workspace),
				Clients:     []contract.WindowManagerClientView{view},
			}, nil
		}
		client.unregisterClientFn = func(_ context.Context, workspace string, clientID string) error {
			if workspace != "w1" || clientID != "browser-a" {
				t.Fatalf("unregister workspace/client = %q/%q", workspace, clientID)
			}
			return nil
		}
		deps := newTestDeps(t, client)

		if _, _, err := executeRootCommand(
			t,
			deps,
			"desktop",
			"clients",
			"register",
			"--workspace",
			"w1",
			"--client",
			"browser-a",
			"--desktop",
			"d2",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("clients register error = %v", err)
		}
		listOutput, _, err := executeRootCommand(
			t,
			deps,
			"desktop",
			"clients",
			"list",
			"--workspace",
			"w1",
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("clients list error = %v", err)
		}
		var clients []contract.WindowManagerClientView
		decodeWindowManagerCommandJSON(t, listOutput, &clients)
		if len(clients) != 1 || clients[0].ClientID != "browser-a" {
			t.Fatalf("clients = %#v, want browser-a", clients)
		}
		if _, _, err := executeRootCommand(
			t,
			deps,
			"desktop",
			"clients",
			"unregister",
			"--workspace",
			"w1",
			"--client",
			"browser-a",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("clients unregister error = %v", err)
		}
	})
}

func TestWindowManagerLayoutRawCommands(t *testing.T) {
	t.Parallel()

	t.Run("Should preview a typed raw command", func(t *testing.T) {
		t.Parallel()
		client := newWindowManagerCommandStub()
		client.previewFn = func(
			_ context.Context,
			workspace string,
			request contract.WindowManagerCommandRequest,
		) (contract.WindowManagerPreview, error) {
			if workspace != "w1" || request.CommandID != contract.WindowManagerCommandWindowZoom ||
				request.ClientID == nil || *request.ClientID != "browser-a" || string(request.Payload) != `{"window_id":"win-1"}` {
				t.Fatalf("preview request = %#v payload=%s", request, request.Payload)
			}
			return contract.WindowManagerPreview{Snapshot: windowManagerTestSnapshot(8), Changed: true}, nil
		}
		output, _, err := executeRootCommand(
			t, newTestDeps(t, client), "layout", "preview", "--workspace", "w1", "--revision", "7",
			"--client", "browser-a", "--command", "window.zoom", "--payload", `{"window_id":"win-1"}`, "-o", "json",
		)
		if err != nil {
			t.Fatalf("layout preview error = %v", err)
		}
		var preview contract.WindowManagerPreview
		decodeWindowManagerCommandJSON(t, output, &preview)
		if !preview.Changed || preview.Snapshot.Revision != 8 {
			t.Fatalf("preview = %#v, want changed revision 8", preview)
		}
	})

	t.Run("Should export validate and apply the same declarative document", func(t *testing.T) {
		t.Parallel()
		document := windowManagerTestLayoutDocument()
		data, err := json.Marshal(document)
		if err != nil {
			t.Fatalf("marshal layout document: %v", err)
		}
		path := filepath.Join(t.TempDir(), "layout.json")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("write layout document: %v", err)
		}

		client := newWindowManagerCommandStub()
		client.exportLayoutFn = func(_ context.Context, workspace string) (contract.WindowManagerLayoutDocument, error) {
			if workspace != "w1" {
				t.Fatalf("export workspace = %q, want w1", workspace)
			}
			return document, nil
		}
		client.validateLayoutFn = func(
			_ context.Context,
			workspace string,
			request contract.WindowManagerLayoutValidationRequest,
		) (contract.WindowManagerLayoutValidationResponse, error) {
			if workspace != "w1" || request.WorkspaceID != "w1" || request.Document.WorkspaceID != "w1" {
				t.Fatalf("validation request = %#v for workspace %q", request, workspace)
			}
			return contract.WindowManagerLayoutValidationResponse{WorkspaceID: "w1", Valid: true}, nil
		}
		client.applyLayoutFn = func(
			_ context.Context,
			workspace string,
			request contract.WindowManagerLayoutReplaceRequest,
		) (contract.WindowManagerResult, error) {
			if workspace != "w1" || request.ExpectedRevision == nil || *request.ExpectedRevision != 7 ||
				request.Document.WorkspaceID != "w1" {
				t.Fatalf("apply request = %#v for workspace %q", request, workspace)
			}
			return windowManagerTestResult(), nil
		}
		deps := newTestDeps(t, client)

		exportOutput, _, err := executeRootCommand(t, deps, "layout", "export", "--workspace", "w1", "-o", "json")
		if err != nil {
			t.Fatalf("layout export error = %v", err)
		}
		var exported contract.WindowManagerLayoutDocument
		decodeWindowManagerCommandJSON(t, exportOutput, &exported)
		if exported.WorkspaceID != "w1" || exported.Version != windowmanager.SnapshotVersion {
			t.Fatalf("exported = %#v, want v2 workspace w1", exported)
		}
		validationOutput, _, err := executeRootCommand(
			t, deps, "layout", "validate", "--workspace", "w1", "--file", path, "-o", "json",
		)
		if err != nil {
			t.Fatalf("layout validate error = %v", err)
		}
		var validation contract.WindowManagerLayoutValidationResponse
		decodeWindowManagerCommandJSON(t, validationOutput, &validation)
		if !validation.Valid || validation.WorkspaceID != "w1" {
			t.Fatalf("layout validation = %#v, want valid workspace w1", validation)
		}

		applyOutput, _, err := executeRootCommand(
			t,
			deps,
			"layout",
			"apply",
			"--workspace",
			"w1",
			"--revision",
			"7",
			"--file",
			path,
			"-o",
			"json",
		)
		if err != nil {
			t.Fatalf("layout apply error = %v", err)
		}
		var applied contract.WindowManagerResult
		decodeWindowManagerCommandJSON(t, applyOutput, &applied)
		if !applied.Applied || applied.Snapshot.Revision != 8 {
			t.Fatalf("layout apply result = %#v, want applied revision 8", applied)
		}
	})

	t.Run("Should render diagnostic codes paths and messages for an invalid layout", func(t *testing.T) {
		t.Parallel()
		document := windowManagerTestLayoutDocument()
		data, err := json.Marshal(document)
		if err != nil {
			t.Fatalf("json.Marshal(layout document) error = %v", err)
		}
		path := filepath.Join(t.TempDir(), "invalid-layout.json")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatalf("os.WriteFile(layout document) error = %v", err)
		}
		client := newWindowManagerCommandStub()
		client.validateLayoutFn = func(
			context.Context,
			string,
			contract.WindowManagerLayoutValidationRequest,
		) (contract.WindowManagerLayoutValidationResponse, error) {
			return contract.WindowManagerLayoutValidationResponse{
				WorkspaceID: "w1",
				Valid:       false,
				Diagnostics: []windowmanager.Diagnostic{{
					Code:    "window_manager_duplicate_window",
					Path:    "desktops[0].groups[0].root.children[1]",
					Message: "window win-1 appears more than once",
				}},
			}, nil
		}
		output, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"layout",
			"validate",
			"--workspace",
			"w1",
			"--file",
			path,
		)
		if err != nil {
			t.Fatalf("layout validate error = %v", err)
		}
		for _, fragment := range []string{
			"window_manager_duplicate_window",
			"desktops[0].groups[0].root.children[1]",
			"window win-1 appears more than once",
		} {
			if !strings.Contains(output, fragment) {
				t.Fatalf("layout validation output = %q, want %q", output, fragment)
			}
		}
	})
}

func TestWindowManagerWatchAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should emit the snapshot fence before later events as JSON lines", func(t *testing.T) {
		t.Parallel()
		client := newWindowManagerCommandStub()
		client.watchFn = func(
			_ context.Context,
			workspace string,
			clientID *windowmanager.ClientID,
			after *contract.WindowManagerRevision,
			handlers WindowManagerStreamHandlers,
		) error {
			if workspace != "w1" || clientID != nil || after == nil || *after != 4 {
				t.Fatalf("watch workspace/client/after = %q/%v/%v, want w1/nil/4", workspace, clientID, after)
			}
			if err := handlers.Snapshot(contract.WindowManagerSnapshotFrame{
				Type: contract.WindowManagerFrameSnapshot, WorkspaceID: "w1", Revision: 7,
				Snapshot: windowManagerTestSnapshot(7),
			}); err != nil {
				return err
			}
			return handlers.Event(contract.WindowManagerEventFrame{
				Type: contract.WindowManagerFrameEvent, WorkspaceID: "w1", Revision: 8,
				Event: contract.WindowManagerEvent{
					WorkspaceID: "w1",
					Revision:    8,
					CommandID:   contract.WindowManagerCommandDesktopCreate,
				},
			})
		}
		output, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"layout",
			"watch",
			"--workspace",
			"w1",
			"--after-revision",
			"4",
			"-o",
			"jsonl",
		)
		if err != nil {
			t.Fatalf("layout watch error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) != 2 {
			t.Fatalf("watch output = %q, want snapshot and event", output)
		}
		var first, second struct {
			Type string `json:"type"`
		}
		decodeWindowManagerCommandJSON(t, lines[0], &first)
		decodeWindowManagerCommandJSON(t, lines[1], &second)
		if first.Type != contract.WindowManagerFrameSnapshot || second.Type != contract.WindowManagerFrameEvent {
			t.Fatalf("watch frame order = %q then %q", first.Type, second.Type)
		}
	})

	t.Run("Should emit a bound client fence and presentation frame as JSON lines", func(t *testing.T) {
		t.Parallel()
		client := newWindowManagerCommandStub()
		client.watchFn = func(
			_ context.Context,
			workspace string,
			clientID *windowmanager.ClientID,
			after *contract.WindowManagerRevision,
			handlers WindowManagerStreamHandlers,
		) error {
			if workspace != "w1" || clientID == nil || *clientID != "browser-a" || after != nil {
				t.Fatalf("watch workspace/client/after = %q/%v/%v, want w1/browser-a/nil", workspace, clientID, after)
			}
			view := windowManagerTestClientView("browser-a")
			view.PresentationRevision = 3
			if err := handlers.Snapshot(contract.WindowManagerSnapshotFrame{
				Type: contract.WindowManagerFrameSnapshot, WorkspaceID: "w1", Revision: 7,
				Snapshot: windowManagerTestSnapshot(7), Client: &view,
			}); err != nil {
				return err
			}
			view.PresentationRevision = 4
			return handlers.Client(contract.WindowManagerClientFrame{
				Type: contract.WindowManagerFrameClient, WorkspaceID: "w1", Revision: 7, Client: view,
			})
		}
		output, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"layout",
			"watch",
			"--workspace",
			"w1",
			"--client",
			"browser-a",
			"-o",
			"jsonl",
		)
		if err != nil {
			t.Fatalf("layout watch error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) != 2 {
			t.Fatalf("watch output = %q, want snapshot and client frame", output)
		}
		var fence contract.WindowManagerSnapshotFrame
		decodeWindowManagerCommandJSON(t, lines[0], &fence)
		var frame contract.WindowManagerClientFrame
		decodeWindowManagerCommandJSON(t, lines[1], &frame)
		if fence.Client == nil || fence.Client.PresentationRevision != 3 ||
			frame.Type != contract.WindowManagerFrameClient || frame.Client.PresentationRevision != 4 {
			t.Fatalf("bound watch frames = %#v / %#v", fence, frame)
		}
	})

	t.Run("Should render revision conflicts as structured errors", func(t *testing.T) {
		t.Parallel()
		client := newWindowManagerCommandStub()
		current := contract.WindowManagerRevision(9)
		client.executeFn = func(
			context.Context,
			string,
			contract.WindowManagerCommandRequest,
		) (contract.WindowManagerResult, error) {
			return contract.WindowManagerResult{}, &windowManagerAPIError{
				statusCode: http.StatusConflict,
				payload: contract.WindowManagerErrorPayload{
					Error: "revision conflict", Code: contract.WindowManagerErrorRevisionConflict,
					WorkspaceID: "w1", CurrentRevision: &current,
				},
			}
		}
		exitCode, _, stderr := executeRootCommandWithExit(
			t, newTestDeps(t, client), "desktop", "create", "--workspace", "w1", "--revision", "7", "-o", "json",
		)
		if exitCode == 0 {
			t.Fatalf("exit code = 0; stderr=%s", stderr)
		}
		var payload contract.WindowManagerErrorPayload
		decodeWindowManagerCommandJSON(t, stderr, &payload)
		if payload.Code != contract.WindowManagerErrorRevisionConflict || payload.CurrentRevision == nil ||
			*payload.CurrentRevision != 9 {
			t.Fatalf("error payload = %#v, want conflict at revision 9", payload)
		}
	})
}

func TestWindowManagerCommandsRejectInvalidInputBeforeTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		args          []string
		wantKind      windowManagerCLIValidationKind
		wantField     string
		wantError     string
		wantJSONCause bool
	}{
		{
			name:      "Should require revisions for mutations",
			args:      []string{"desktop", "create", "--workspace", "w1"},
			wantKind:  windowManagerCLIValidationRequired,
			wantField: windowManagerRevisionFlag,
			wantError: "--revision is required",
		},
		{
			name:     "Should require a client for desktop switching",
			args:     []string{"desktop", "switch", "--workspace", "w1", "--revision", "1", "--id", "d2"},
			wantKind: windowManagerCLIValidationRequired, wantField: windowManagerClientFlag,
			wantError: "--client is required for presentation commands",
		},
		{
			name: "Should reject ambiguous focus targets",
			args: []string{
				"window",
				"focus",
				"--workspace",
				"w1",
				"--revision",
				"1",
				"--client",
				"c1",
				"--id",
				"win-1",
				"--direction",
				"left",
			},
			wantKind: windowManagerCLIValidationConflicting, wantField: "focus_target",
			wantError: "exactly one of --id or --direction is required",
		},
		{
			name: "Should require a structural move target",
			args: []string{
				"window",
				"move",
				"--workspace",
				"w1",
				"--revision",
				"1",
				"--id",
				"win-1",
				"--desktop",
				"d1",
				"--placement",
				"left",
			},
			wantKind: windowManagerCLIValidationRequired, wantField: "target",
			wantError: "--target is required for structural placement",
		},
		{
			name: "Should reject rectangles outside the unit square",
			args: []string{
				"window",
				"open",
				"--workspace",
				"w1",
				"--revision",
				"1",
				"--app",
				"tasks",
				"--rect",
				"0.8,0.8,0.5,0.5",
				"--pathname",
				"/tasks",
				"--search-json",
				`{}`,
			},
			wantKind: windowManagerCLIValidationOutOfBounds, wantField: windowManagerRectFlag,
			wantError: "contained in the normalized unit square",
		},
		{
			name: "Should require a pathname when opening a window",
			args: []string{
				"window", "open", "--workspace", "w1", "--revision", "1", "--app", "tasks",
				"--search-json", `{}`,
			},
			wantKind: windowManagerCLIValidationRequired, wantField: windowManagerPathnameFlag,
			wantError: "--pathname is required",
		},
		{
			name: "Should reject external navigation pathnames",
			args: []string{
				"window", "navigate", "--workspace", "w1", "--revision", "1", "--id", "win-1",
				"--pathname", "//example.com/tasks", "--search-json", `{}`,
			},
			wantKind: windowManagerCLIValidationInvalidValue, wantField: windowManagerPathnameFlag,
			wantError: "invalid window route",
		},
		{
			name: "Should reject non-object navigation search state",
			args: []string{
				"window", "navigate", "--workspace", "w1", "--revision", "1", "--id", "win-1",
				"--pathname", "/tasks", "--search-json", `[]`,
			},
			wantKind: windowManagerCLIValidationInvalidValue, wantField: windowManagerSearchJSONFlag,
			wantError:     "decode --search-json",
			wantJSONCause: true,
		},
		{
			name: "Should reject unknown preview commands",
			args: []string{
				"layout",
				"preview",
				"--workspace",
				"w1",
				"--revision",
				"1",
				"--command",
				"window.teleport",
				"--payload",
				`{}`,
			},
			wantKind: windowManagerCLIValidationUnsupported, wantField: "command",
			wantError: "unsupported --command",
		},
		{
			name: "Should reject mixed resource and inline arrangement modes",
			args: []string{
				"layout", "arrange", "--workspace", "w1", "--revision", "1", "--resource", "focused-work",
				"--desktop", "d1",
			},
			wantKind: windowManagerCLIValidationConflicting, wantField: windowManagerResourceFlag,
			wantError: "--resource cannot be combined with --desktop",
		},
		{
			name: "Should reject incomplete inline arrangement mode",
			args: []string{
				"layout", "arrange", "--workspace", "w1", "--revision", "1", "--desktop", "d1", "--window", "win-1",
			},
			wantKind: windowManagerCLIValidationRequired, wantField: windowManagerArrangementFlag,
			wantError: "--arrangement is required",
		},
		{
			name: "Should reject an explicitly blank client registration",
			args: []string{
				"desktop", "clients", "register", "--workspace", "w1", "--client", " ",
			},
			wantKind:  windowManagerCLIValidationInvalidValue,
			wantField: windowManagerClientFlag,
			wantError: "--client must not be blank",
		},
		{
			name: "Should reject an explicit false tiled flag during restore",
			args: []string{
				"window", "open", "--workspace", "w1", "--revision", "1", "--restore", "win-1", "--tiled=false",
			},
			wantKind:  windowManagerCLIValidationConflicting,
			wantField: "restore",
			wantError: "--restore cannot be combined with new-window flags",
		},
		{
			name: "Should reject a target during group relocation",
			args: []string{
				"window", "move", "--workspace", "w1", "--revision", "1", "--id", "win-1",
				"--desktop", "d2", "--group", "--target", "win-2",
			},
			wantKind:  windowManagerCLIValidationConflicting,
			wantField: windowManagerGroupFlag,
			wantError: "--group cannot be combined with --target",
		},
		{
			name: "Should reject placement during group relocation",
			args: []string{
				"window", "move", "--workspace", "w1", "--revision", "1", "--id", "win-1",
				"--desktop", "d2", "--group", "--placement", "floating",
			},
			wantKind:  windowManagerCLIValidationConflicting,
			wantField: windowManagerGroupFlag,
			wantError: "--group cannot be combined with --placement",
		},
		{
			name: "Should reject a rectangle during group relocation",
			args: []string{
				"window", "move", "--workspace", "w1", "--revision", "1", "--id", "win-1",
				"--desktop", "d2", "--group", "--rect", "0.1,0.1,0.5,0.5",
			},
			wantKind:  windowManagerCLIValidationConflicting,
			wantField: windowManagerGroupFlag,
			wantError: "--group cannot be combined with --rect",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			called := false
			client := newWindowManagerCommandStub()
			client.executeFn = func(
				context.Context,
				string,
				contract.WindowManagerCommandRequest,
			) (contract.WindowManagerResult, error) {
				called = true
				return contract.WindowManagerResult{}, errors.New("unexpected execute")
			}
			client.previewFn = func(
				context.Context,
				string,
				contract.WindowManagerCommandRequest,
			) (contract.WindowManagerPreview, error) {
				called = true
				return contract.WindowManagerPreview{}, errors.New("unexpected preview")
			}
			_, _, err := executeRootCommand(t, newTestDeps(t, client), tt.args...)
			if err == nil {
				t.Fatal("command error = nil, want validation error")
			}
			var validationErr *windowManagerCLIValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("command error = %T, want *windowManagerCLIValidationError", err)
			}
			if validationErr.Kind != tt.wantKind || validationErr.Field != tt.wantField {
				t.Fatalf(
					"validation identity = %q/%q, want %q/%q",
					validationErr.Kind,
					validationErr.Field,
					tt.wantKind,
					tt.wantField,
				)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("command error = %q, want fragment %q", err, tt.wantError)
			}
			if tt.wantJSONCause {
				var jsonTypeErr *json.UnmarshalTypeError
				if !errors.As(err, &jsonTypeErr) {
					t.Fatalf("command error = %v, want *json.UnmarshalTypeError", err)
				}
			}
			if called {
				t.Fatal("transport was called for invalid input")
			}
		})
	}
}

func TestWindowManagerHardCutCommandTree(t *testing.T) {
	t.Parallel()

	t.Run("Should expose only the new root command groups", func(t *testing.T) {
		t.Parallel()

		cmd := newRootCommand(commandDeps{}.withDefaults())
		for _, name := range []string{"desktop", "window", "layout"} {
			found, _, err := cmd.Find([]string{name})
			if err != nil || found == nil || found.Name() != name {
				t.Fatalf("find %q = %v/%v, want public root group", name, found, err)
			}
		}
		if found, _, err := cmd.Find([]string{"desktop-state"}); err == nil && found != nil &&
			found.Name() == "desktop-state" {
			t.Fatal("legacy desktop-state command remains registered")
		}
	})
}

type windowManagerCommandStub struct {
	*stubClient
	snapshotFn func(context.Context, string) (contract.WindowManagerSnapshot, error)
	previewFn  func(
		context.Context,
		string,
		contract.WindowManagerCommandRequest,
	) (contract.WindowManagerPreview, error)
	executeFn func(
		context.Context,
		string,
		contract.WindowManagerCommandRequest,
	) (contract.WindowManagerResult, error)
	listClientsFn    func(context.Context, string) (contract.WindowManagerClientsResponse, error)
	registerClientFn func(
		context.Context,
		string,
		contract.WindowManagerClientRegistration,
	) (contract.WindowManagerClientView, error)
	unregisterClientFn func(context.Context, string, string) error
	exportLayoutFn     func(context.Context, string) (contract.WindowManagerLayoutDocument, error)
	listProfilesFn     func(context.Context, string) (contract.ResourcesResponse, error)
	putProfileFn       func(
		context.Context,
		string,
		string,
		contract.PutResourceRequest,
	) (contract.ResourceResponse, error)
	deleteProfileFn  func(context.Context, string, string, contract.DeleteResourceRequest) error
	validateLayoutFn func(
		context.Context,
		string,
		contract.WindowManagerLayoutValidationRequest,
	) (contract.WindowManagerLayoutValidationResponse, error)
	applyLayoutFn func(
		context.Context,
		string,
		contract.WindowManagerLayoutReplaceRequest,
	) (contract.WindowManagerResult, error)
	watchFn func(
		context.Context,
		string,
		*windowmanager.ClientID,
		*contract.WindowManagerRevision,
		WindowManagerStreamHandlers,
	) error
}

func newWindowManagerCommandStub() *windowManagerCommandStub {
	return &windowManagerCommandStub{stubClient: &stubClient{}}
}

func (s *windowManagerCommandStub) GetWindowManagerSnapshot(
	ctx context.Context,
	workspace string,
) (contract.WindowManagerSnapshot, error) {
	if s.snapshotFn == nil {
		return contract.WindowManagerSnapshot{}, errors.New("unexpected GetWindowManagerSnapshot call")
	}
	return s.snapshotFn(ctx, workspace)
}

func (s *windowManagerCommandStub) PreviewWindowManagerCommand(
	ctx context.Context,
	workspace string,
	request contract.WindowManagerCommandRequest,
) (contract.WindowManagerPreview, error) {
	if s.previewFn == nil {
		return contract.WindowManagerPreview{}, errors.New("unexpected PreviewWindowManagerCommand call")
	}
	return s.previewFn(ctx, workspace, request)
}

func (s *windowManagerCommandStub) ExecuteWindowManagerCommand(
	ctx context.Context,
	workspace string,
	request contract.WindowManagerCommandRequest,
) (contract.WindowManagerResult, error) {
	if s.executeFn == nil {
		return contract.WindowManagerResult{}, errors.New("unexpected ExecuteWindowManagerCommand call")
	}
	return s.executeFn(ctx, workspace, request)
}

func (s *windowManagerCommandStub) ListWindowManagerClients(
	ctx context.Context,
	workspace string,
) (contract.WindowManagerClientsResponse, error) {
	if s.listClientsFn == nil {
		return contract.WindowManagerClientsResponse{}, errors.New("unexpected ListWindowManagerClients call")
	}
	return s.listClientsFn(ctx, workspace)
}

func (s *windowManagerCommandStub) RegisterWindowManagerClient(
	ctx context.Context,
	workspace string,
	request contract.WindowManagerClientRegistration,
) (contract.WindowManagerClientView, error) {
	if s.registerClientFn == nil {
		return contract.WindowManagerClientView{}, errors.New("unexpected RegisterWindowManagerClient call")
	}
	return s.registerClientFn(ctx, workspace, request)
}

func (s *windowManagerCommandStub) UnregisterWindowManagerClient(
	ctx context.Context,
	workspace string,
	clientID string,
) error {
	if s.unregisterClientFn == nil {
		return errors.New("unexpected UnregisterWindowManagerClient call")
	}
	return s.unregisterClientFn(ctx, workspace, clientID)
}

func (s *windowManagerCommandStub) ExportWindowManagerLayout(
	ctx context.Context,
	workspace string,
) (contract.WindowManagerLayoutDocument, error) {
	if s.exportLayoutFn == nil {
		return contract.WindowManagerLayoutDocument{}, errors.New("unexpected ExportWindowManagerLayout call")
	}
	return s.exportLayoutFn(ctx, workspace)
}

func (s *windowManagerCommandStub) ValidateWindowManagerLayout(
	ctx context.Context,
	workspace string,
	request contract.WindowManagerLayoutValidationRequest,
) (contract.WindowManagerLayoutValidationResponse, error) {
	if s.validateLayoutFn == nil {
		return contract.WindowManagerLayoutValidationResponse{}, errors.New(
			"unexpected ValidateWindowManagerLayout call",
		)
	}
	return s.validateLayoutFn(ctx, workspace, request)
}

func (s *windowManagerCommandStub) ApplyWindowManagerLayout(
	ctx context.Context,
	workspace string,
	request contract.WindowManagerLayoutReplaceRequest,
) (contract.WindowManagerResult, error) {
	if s.applyLayoutFn == nil {
		return contract.WindowManagerResult{}, errors.New("unexpected ApplyWindowManagerLayout call")
	}
	return s.applyLayoutFn(ctx, workspace, request)
}

func (s *windowManagerCommandStub) ListWindowManagerLayoutProfiles(
	ctx context.Context,
	workspace string,
) (contract.ResourcesResponse, error) {
	if s.listProfilesFn == nil {
		return contract.ResourcesResponse{}, errors.New("unexpected ListWindowManagerLayoutProfiles call")
	}
	return s.listProfilesFn(ctx, workspace)
}

func (s *windowManagerCommandStub) PutWindowManagerLayoutProfile(
	ctx context.Context,
	workspace string,
	profileID string,
	request contract.PutResourceRequest,
) (contract.ResourceResponse, error) {
	if s.putProfileFn == nil {
		return contract.ResourceResponse{}, errors.New("unexpected PutWindowManagerLayoutProfile call")
	}
	return s.putProfileFn(ctx, workspace, profileID, request)
}

func (s *windowManagerCommandStub) DeleteWindowManagerLayoutProfile(
	ctx context.Context,
	workspace string,
	profileID string,
	request contract.DeleteResourceRequest,
) error {
	if s.deleteProfileFn == nil {
		return errors.New("unexpected DeleteWindowManagerLayoutProfile call")
	}
	return s.deleteProfileFn(ctx, workspace, profileID, request)
}

func (s *windowManagerCommandStub) WatchWindowManager(
	ctx context.Context,
	workspace string,
	clientID *windowmanager.ClientID,
	after *contract.WindowManagerRevision,
	handlers WindowManagerStreamHandlers,
) error {
	if s.watchFn == nil {
		return errors.New("unexpected WatchWindowManager call")
	}
	return s.watchFn(ctx, workspace, clientID, after, handlers)
}

func windowManagerTestSnapshot(revision contract.WindowManagerRevision) contract.WindowManagerSnapshot {
	return contract.WindowManagerSnapshot{
		Version: windowmanager.SnapshotVersion, WorkspaceID: "w1", Revision: revision,
		Desktops: []contract.WindowManagerDesktop{
			{
				ID:       "d1",
				Name:     "Desktop 1",
				Order:    0,
				Purpose:  windowmanager.DesktopPurposeStandard,
				Groups:   []contract.WindowManagerLayoutGroup{},
				Floating: []windowmanager.WindowID{"win-1"},
			},
			{
				ID:       "d2",
				Name:     "Desktop 2",
				Order:    1,
				Purpose:  windowmanager.DesktopPurposeStandard,
				Groups:   []contract.WindowManagerLayoutGroup{},
				Floating: []windowmanager.WindowID{"win-2"},
			},
		},
		Windows: map[string]contract.WindowManagerWindow{
			"win-2": {
				ID: "win-2", App: "settings", Route: windowmanager.RouteIntent{
					Pathname: "/settings", Search: windowmanager.RouteSearch{},
				}, DesktopID: "d2", Placement: windowmanager.WindowPlacementFloating,
			},
			"win-1": {
				ID: "win-1", App: "tasks", Route: windowmanager.RouteIntent{
					Pathname: "/tasks", Search: windowmanager.RouteSearch{},
				}, DesktopID: "d1", Placement: windowmanager.WindowPlacementFloating,
			},
		},
		UpdatedAt: time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC),
	}
}

func windowManagerTestResult() contract.WindowManagerResult {
	return contract.WindowManagerResult{Snapshot: windowManagerTestSnapshot(8), Applied: true}
}

func windowManagerTestClientView(clientID windowmanager.ClientID) contract.WindowManagerClientView {
	return contract.WindowManagerClientView{
		WorkspaceID: "w1", ClientID: clientID, ActiveDesktopID: "d2",
		FocusOrder: []windowmanager.WindowID{}, ConnectedAt: time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC),
	}
}

func windowManagerTestLayoutDocument() contract.WindowManagerLayoutDocument {
	snapshot := windowManagerTestSnapshot(7)
	return contract.WindowManagerLayoutDocument{
		Version: snapshot.Version, WorkspaceID: snapshot.WorkspaceID,
		Desktops: snapshot.Desktops, Windows: snapshot.Windows, Overrides: snapshot.Overrides,
	}
}

func decodeWindowManagerCommandRoute(
	t *testing.T,
	commandID contract.WindowManagerCommandID,
	payload json.RawMessage,
) windowmanager.RouteIntent {
	t.Helper()
	switch commandID {
	case contract.WindowManagerCommandWindowOpen:
		var decoded contract.WindowManagerOpenWindowPayload
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("decode window.open payload: %v", err)
		}
		return decoded.Window.Route
	case contract.WindowManagerCommandWindowNavigate:
		var decoded contract.WindowManagerNavigateWindowPayload
		if err := json.Unmarshal(payload, &decoded); err != nil {
			t.Fatalf("decode window.navigate payload: %v", err)
		}
		return decoded.Route
	default:
		t.Fatalf("command %q does not carry a route", commandID)
		return windowmanager.RouteIntent{}
	}
}

func windowManagerJSONEqual(left []byte, right []byte) bool {
	var leftValue any
	if err := json.Unmarshal(left, &leftValue); err != nil {
		return false
	}
	var rightValue any
	if err := json.Unmarshal(right, &rightValue); err != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func decodeWindowManagerCommandJSON(t *testing.T, value string, target any) {
	t.Helper()
	if err := json.Unmarshal([]byte(strings.TrimSpace(value)), target); err != nil {
		t.Fatalf("decode JSON: %v; value=%q", err, value)
	}
}
