// Suite: Window-manager shared WebSocket stream.
// Invariant: WM-GO-018 snapshot fencing, ordered workspace-scoped events,
// terminal preflight errors, slow-consumer error mapping, and leak-free shutdown.
// Owning layer: shared HTTP/UDS stream handler. Canonical suite: this file.
// Boundary IN: frame ordering, workspace identity, and stream lifecycle.
// Boundary OUT: hub backpressure mechanics belong to internal/windowmanager.
package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func TestWindowManagerWebSocketStream(t *testing.T) {
	t.Run("Should fence with a snapshot, isolate events, and drain on shutdown", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)
		server := httptest.NewServer(fixture.router)
		t.Cleanup(server.Close)

		streamURL := "ws" + strings.TrimPrefix(server.URL, "http") + windowManagerTestPath("workspace-a") + "/stream"
		connection, response, err := websocket.DefaultDialer.DialContext(t.Context(), streamURL, nil)
		if err != nil {
			if response != nil && response.Body != nil {
				if closeErr := response.Body.Close(); closeErr != nil {
					t.Errorf("close failed websocket response: %v", closeErr)
				}
			}
			t.Fatalf("DialContext() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := connection.Close(); closeErr != nil {
				t.Errorf("websocket Close() error = %v", closeErr)
			}
		})

		var snapshotFrame contract.WindowManagerSnapshotFrame
		if readErr := connection.ReadJSON(&snapshotFrame); readErr != nil {
			t.Fatalf("ReadJSON(snapshot) error = %v", readErr)
		}
		if snapshotFrame.Type != contract.WindowManagerFrameSnapshot ||
			snapshotFrame.WorkspaceID != "workspace-a" || snapshotFrame.Revision != 0 {
			t.Fatalf("snapshot frame = %+v", snapshotFrame)
		}

		executeWindowManagerStreamCommand(t, fixture.manager, "workspace-b", "desktop-b")
		executeWindowManagerStreamCommand(t, fixture.manager, "workspace-a", "desktop-a")

		var eventFrame contract.WindowManagerEventFrame
		if readErr := connection.ReadJSON(&eventFrame); readErr != nil {
			t.Fatalf("ReadJSON(event) error = %v", readErr)
		}
		if eventFrame.Type != contract.WindowManagerFrameEvent ||
			eventFrame.WorkspaceID != "workspace-a" || eventFrame.Revision != 1 ||
			eventFrame.Event.WorkspaceID != "workspace-a" || eventFrame.Event.Revision != 1 {
			t.Fatalf("event frame = %+v", eventFrame)
		}

		shutdownContext, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cancel()
		if shutdownErr := fixture.handlers.ShutdownWindowManagerStreams(shutdownContext); shutdownErr != nil {
			t.Fatalf("ShutdownWindowManagerStreams() error = %v", shutdownErr)
		}
		if deadlineErr := connection.SetReadDeadline(time.Now().Add(time.Second)); deadlineErr != nil {
			t.Fatalf("SetReadDeadline() error = %v", deadlineErr)
		}
		messageType, message, readErr := connection.ReadMessage()
		if readErr == nil {
			t.Fatalf("ReadMessage() = type %d payload %q, want shutdown close", messageType, message)
		}
	})

	t.Run("Should fence one client and stream only its presentation revisions", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)
		created := executeWindowManagerStreamCommand(t, fixture.manager, "workspace-a", "desktop-a")
		clientA := windowmanager.ClientID("client-a")
		clientB := windowmanager.ClientID("client-b")
		for _, clientID := range []windowmanager.ClientID{clientA, clientB} {
			view, err := fixture.manager.RegisterClient(t.Context(), windowmanager.ClientRegistration{
				WorkspaceID: "workspace-a", ClientID: clientID,
			})
			if err != nil || view.PresentationRevision != 1 {
				t.Fatalf("RegisterClient(%s) = %+v, error = %v", clientID, view, err)
			}
		}

		server := httptest.NewServer(fixture.router)
		t.Cleanup(server.Close)
		streamURL := "ws" + strings.TrimPrefix(server.URL, "http") +
			windowManagerTestPath("workspace-a") +
			"/stream?after_revision=1&client_id=client-a"
		connection, response, err := websocket.DefaultDialer.DialContext(t.Context(), streamURL, nil)
		if err != nil {
			closeWindowManagerDialResponse(t, response)
			t.Fatalf("DialContext() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := connection.Close(); closeErr != nil {
				t.Errorf("websocket Close() error = %v", closeErr)
			}
		})

		var snapshotFrame contract.WindowManagerSnapshotFrame
		if readErr := connection.ReadJSON(&snapshotFrame); readErr != nil {
			t.Fatalf("ReadJSON(snapshot) error = %v", readErr)
		}
		if snapshotFrame.Type != contract.WindowManagerFrameSnapshot ||
			snapshotFrame.Revision != 1 ||
			snapshotFrame.Client == nil ||
			snapshotFrame.Client.ClientID != clientA ||
			snapshotFrame.Client.PresentationRevision != 1 {
			t.Fatalf("client snapshot frame = %+v", snapshotFrame)
		}

		switched := executeWindowManagerPresentationSwitch(
			t,
			fixture.manager,
			clientA,
			created.Snapshot.Revision,
		)
		if switched.Snapshot.Revision != created.Snapshot.Revision ||
			switched.Client == nil ||
			switched.Client.PresentationRevision != 2 {
			t.Fatalf("client-a switch result = %+v", switched)
		}
		var clientFrame contract.WindowManagerClientFrame
		if readErr := connection.ReadJSON(&clientFrame); readErr != nil {
			t.Fatalf("ReadJSON(client) error = %v", readErr)
		}
		if clientFrame.Type != contract.WindowManagerFrameClient ||
			clientFrame.WorkspaceID != "workspace-a" ||
			clientFrame.Revision != 2 ||
			clientFrame.Client.ClientID != clientA ||
			clientFrame.Client.PresentationRevision != 2 {
			t.Fatalf("client frame = %+v", clientFrame)
		}

		executeWindowManagerPresentationSwitch(
			t,
			fixture.manager,
			clientB,
			created.Snapshot.Revision,
		)
		topology, err := fixture.manager.Execute(t.Context(), windowmanager.CommandRequest{
			WorkspaceID: "workspace-a", ExpectedRevision: created.Snapshot.Revision,
			Payload: windowmanager.CreateDesktopCommand{
				DesktopID: "desktop-b", Name: "Third",
			},
		})
		if err != nil {
			t.Fatalf("Execute(topology) error = %v", err)
		}
		var eventFrame contract.WindowManagerEventFrame
		if readErr := connection.ReadJSON(&eventFrame); readErr != nil {
			t.Fatalf("ReadJSON(event) error = %v", readErr)
		}
		if eventFrame.Type != contract.WindowManagerFrameEvent ||
			eventFrame.Revision != contract.WindowManagerRevision(topology.Snapshot.Revision) {
			t.Fatalf("event after client-b mutation = %+v", eventFrame)
		}

		if err := fixture.manager.UnregisterClient(t.Context(), "workspace-a", clientA); err != nil {
			t.Fatalf("UnregisterClient() error = %v", err)
		}
		var errorFrame contract.WindowManagerErrorFrame
		if readErr := connection.ReadJSON(&errorFrame); readErr != nil {
			t.Fatalf("ReadJSON(error) error = %v", readErr)
		}
		if errorFrame.Type != contract.WindowManagerFrameError ||
			errorFrame.Error.Code != contract.WindowManagerErrorClientNotFound {
			t.Fatalf("unregister error frame = %+v", errorFrame)
		}
	})

	t.Run("Should replace client-owned palette context over the bound stream", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)
		registered, err := fixture.manager.RegisterClient(t.Context(), windowmanager.ClientRegistration{
			WorkspaceID: "workspace-a",
			ClientID:    "client-a",
			Kind:        windowmanager.ClientKindBrowser,
			Context: windowmanager.ClientContextInput{
				WorkspaceTrusted: true,
			},
		})
		if err != nil {
			t.Fatalf("RegisterClient() error = %v", err)
		}

		server := httptest.NewServer(fixture.router)
		t.Cleanup(server.Close)
		streamURL := "ws" + strings.TrimPrefix(server.URL, "http") +
			windowManagerTestPath("workspace-a") + "/stream?client_id=client-a"
		connection, response, err := websocket.DefaultDialer.DialContext(t.Context(), streamURL, nil)
		if err != nil {
			closeWindowManagerDialResponse(t, response)
			t.Fatalf("DialContext() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := connection.Close(); closeErr != nil {
				t.Errorf("websocket Close() error = %v", closeErr)
			}
		})

		var snapshotFrame contract.WindowManagerSnapshotFrame
		if readErr := connection.ReadJSON(&snapshotFrame); readErr != nil {
			t.Fatalf("ReadJSON(snapshot) error = %v", readErr)
		}
		if snapshotFrame.Client == nil ||
			snapshotFrame.Client.ContextRevision != contract.WindowManagerRevision(registered.ContextRevision) {
			t.Fatalf("client snapshot frame = %+v", snapshotFrame)
		}

		if writeErr := connection.WriteJSON(contract.WindowManagerClientContextFrame{
			Type: contract.WindowManagerFrameClientContext,
			Context: contract.WindowManagerClientContextInput{
				ScopeGlobal:         true,
				FocusedSessionState: "running",
				WorkspaceTrusted:    true,
				DestinationIntent: &windowmanager.RouteIntent{
					Pathname: "/sessions/session-a",
					Search:   windowmanager.RouteSearch{},
				},
			},
		}); writeErr != nil {
			t.Fatalf("WriteJSON(client context) error = %v", writeErr)
		}

		var clientFrame contract.WindowManagerClientFrame
		if readErr := connection.ReadJSON(&clientFrame); readErr != nil {
			t.Fatalf("ReadJSON(client) error = %v", readErr)
		}
		context := clientFrame.Client.PaletteContext
		if clientFrame.Type != contract.WindowManagerFrameClient ||
			clientFrame.Client.ContextRevision != 2 ||
			!context.ScopeGlobal ||
			context.FocusedSessionState != "running" ||
			!context.WorkspaceTrusted ||
			context.DestinationIntent == nil ||
			context.DestinationIntent.Pathname != "/sessions/session-a" ||
			context.WindowFocused || context.DesktopWindowCount != 0 {
			t.Fatalf("client context frame = %+v", clientFrame)
		}
	})

	t.Run("Should broadcast grouped and closed tab revisions to every subscriber", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)
		revision := windowmanager.Revision(0)
		for _, windowID := range []windowmanager.WindowID{"window-a", "window-b"} {
			result, err := fixture.manager.Execute(t.Context(), windowmanager.CommandRequest{
				WorkspaceID: "workspace-a", ExpectedRevision: revision,
				Payload: windowmanager.OpenWindowCommand{Window: windowmanager.WindowSpec{
					ID: windowID, App: "tasks", DesktopID: "desktop-default",
					Route: windowmanager.RouteIntent{
						Pathname: "/tasks/" + string(windowID), Search: windowmanager.RouteSearch{},
					},
					FloatingRect: windowmanager.NormalizedRect{X: 0.1, Y: 0.1, Width: 0.5, Height: 0.5},
				}},
			})
			if err != nil {
				t.Fatalf("Execute(window.open %s) error = %v", windowID, err)
			}
			revision = result.Snapshot.Revision
		}

		server := httptest.NewServer(fixture.router)
		t.Cleanup(server.Close)
		streamURL := "ws" + strings.TrimPrefix(server.URL, "http") +
			windowManagerTestPath("workspace-a") + "/stream?after_revision=2"
		connections := make([]*websocket.Conn, 0, 2)
		for range 2 {
			connection, response, err := websocket.DefaultDialer.DialContext(t.Context(), streamURL, nil)
			if err != nil {
				closeWindowManagerDialResponse(t, response)
				t.Fatalf("DialContext() error = %v", err)
			}
			connections = append(connections, connection)
			t.Cleanup(func() {
				if closeErr := connection.Close(); closeErr != nil {
					t.Errorf("websocket Close() error = %v", closeErr)
				}
			})
			var snapshotFrame contract.WindowManagerSnapshotFrame
			if readErr := connection.ReadJSON(&snapshotFrame); readErr != nil {
				t.Fatalf("ReadJSON(snapshot) error = %v", readErr)
			}
			if snapshotFrame.Revision != 2 {
				t.Fatalf("snapshot revision = %d, want 2", snapshotFrame.Revision)
			}
		}

		grouped, err := fixture.manager.Execute(t.Context(), windowmanager.CommandRequest{
			WorkspaceID: "workspace-a", ExpectedRevision: 2,
			Payload: windowmanager.GroupWindowsCommand{
				TargetWindowID: "window-a", WindowIDs: []windowmanager.WindowID{"window-b"},
			},
		})
		if err != nil {
			t.Fatalf("Execute(window.stack.group) error = %v", err)
		}
		for _, connection := range connections {
			var frame contract.WindowManagerEventFrame
			if readErr := connection.ReadJSON(&frame); readErr != nil {
				t.Fatalf("ReadJSON(group event) error = %v", readErr)
			}
			if frame.Revision != 3 || frame.Event.CommandID != contract.WindowManagerCommandWindowStackGroup {
				t.Fatalf("group event frame = %+v", frame)
			}
		}

		closed, err := fixture.manager.Execute(t.Context(), windowmanager.CommandRequest{
			WorkspaceID: "workspace-a", ExpectedRevision: grouped.Snapshot.Revision,
			Payload: windowmanager.CloseWindowCommand{
				WindowID: "window-b",
			},
		})
		if err != nil {
			t.Fatalf("Execute(window.close) error = %v", err)
		}
		if closed.Snapshot.Revision != 4 {
			t.Fatalf("close revision = %d, want 4", closed.Snapshot.Revision)
		}
		for _, connection := range connections {
			var frame contract.WindowManagerEventFrame
			if readErr := connection.ReadJSON(&frame); readErr != nil {
				t.Fatalf("ReadJSON(close event) error = %v", readErr)
			}
			if frame.Revision != 4 || frame.Event.CommandID != contract.WindowManagerCommandWindowClose {
				t.Fatalf("close event frame = %+v", frame)
			}
		}
	})

	t.Run("Should upgrade a missing bound client to a terminal error and drain", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)
		server := httptest.NewServer(fixture.router)
		t.Cleanup(server.Close)

		streamURL := "ws" + strings.TrimPrefix(server.URL, "http") +
			windowManagerTestPath("workspace-a") + "/stream?client_id=missing"
		connection, response, err := websocket.DefaultDialer.DialContext(t.Context(), streamURL, nil)
		if err != nil {
			closeWindowManagerDialResponse(t, response)
			t.Fatalf("DialContext() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := connection.Close(); closeErr != nil {
				t.Errorf("websocket Close() error = %v", closeErr)
			}
		})

		var errorFrame contract.WindowManagerErrorFrame
		if readErr := connection.ReadJSON(&errorFrame); readErr != nil {
			t.Fatalf("ReadJSON(error) error = %v", readErr)
		}
		if errorFrame.Type != contract.WindowManagerFrameError ||
			errorFrame.Error.Code != contract.WindowManagerErrorClientNotFound ||
			errorFrame.Error.WorkspaceID != "workspace-a" {
			t.Fatalf("missing-client error frame = %+v", errorFrame)
		}
		if deadlineErr := connection.SetReadDeadline(time.Now().Add(time.Second)); deadlineErr != nil {
			t.Fatalf("SetReadDeadline() error = %v", deadlineErr)
		}
		_, _, readErr := connection.ReadMessage()
		if !websocket.IsCloseError(readErr, websocket.ClosePolicyViolation) {
			t.Fatalf("ReadMessage() error = %v, want policy-violation close", readErr)
		}

		shutdownContext, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cancel()
		if shutdownErr := fixture.handlers.ShutdownWindowManagerStreams(shutdownContext); shutdownErr != nil {
			t.Fatalf("ShutdownWindowManagerStreams() error = %v", shutdownErr)
		}
	})

	t.Run("Should upgrade a missing workspace to a terminal error and drain", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)
		server := httptest.NewServer(fixture.router)
		t.Cleanup(server.Close)

		streamURL := "ws" + strings.TrimPrefix(server.URL, "http") +
			windowManagerTestPath("missing-workspace") + "/stream"
		connection, response, err := websocket.DefaultDialer.DialContext(t.Context(), streamURL, nil)
		if err != nil {
			closeWindowManagerDialResponse(t, response)
			t.Fatalf("DialContext() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := connection.Close(); closeErr != nil {
				t.Errorf("websocket Close() error = %v", closeErr)
			}
		})

		var errorFrame contract.WindowManagerErrorFrame
		if readErr := connection.ReadJSON(&errorFrame); readErr != nil {
			t.Fatalf("ReadJSON(error) error = %v", readErr)
		}
		if errorFrame.Type != contract.WindowManagerFrameError ||
			errorFrame.Error.Code != contract.WindowManagerErrorWorkspaceNotFound ||
			errorFrame.Error.WorkspaceID != "missing-workspace" {
			t.Fatalf("missing-workspace error frame = %+v", errorFrame)
		}
		if deadlineErr := connection.SetReadDeadline(time.Now().Add(time.Second)); deadlineErr != nil {
			t.Fatalf("SetReadDeadline() error = %v", deadlineErr)
		}
		_, _, readErr := connection.ReadMessage()
		if !websocket.IsCloseError(readErr, websocket.ClosePolicyViolation) {
			t.Fatalf("ReadMessage() error = %v, want policy-violation close", readErr)
		}

		shutdownContext, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
		defer cancel()
		if shutdownErr := fixture.handlers.ShutdownWindowManagerStreams(shutdownContext); shutdownErr != nil {
			t.Fatalf("ShutdownWindowManagerStreams() error = %v", shutdownErr)
		}
	})

	t.Run("Should keep a missing bound client as JSON 404 without WebSocket upgrade", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)
		response := performWindowManagerTestRequest(
			t,
			fixture.router,
			http.MethodGet,
			windowManagerTestPath("workspace-a")+"/stream?client_id=missing",
			"",
		)
		requireWindowManagerError(
			t,
			response,
			http.StatusNotFound,
			contract.WindowManagerErrorClientNotFound,
		)
	})

	t.Run("Should reject unsafe revisions before upgrade", func(t *testing.T) {
		t.Parallel()
		fixture := newWindowManagerHandlerFixture(t)
		server := httptest.NewServer(fixture.router)
		t.Cleanup(server.Close)
		streamURL := "ws" + strings.TrimPrefix(server.URL, "http") +
			windowManagerTestPath("workspace-a") +
			"/stream?after_revision=9007199254740992"

		connection, response, err := websocket.DefaultDialer.DialContext(t.Context(), streamURL, nil)
		if connection != nil {
			if closeErr := connection.Close(); closeErr != nil {
				t.Errorf("websocket Close() error = %v", closeErr)
			}
		}
		if err == nil || response == nil {
			closeWindowManagerDialResponse(t, response)
			t.Fatalf("DialContext() error = %v, response = %+v", err, response)
		}
		defer closeWindowManagerDialResponse(t, response)
		if response.StatusCode != http.StatusUnprocessableEntity {
			t.Fatalf(
				"response status = %d, want %d",
				response.StatusCode,
				http.StatusUnprocessableEntity,
			)
		}
		var payload contract.WindowManagerErrorPayload
		if decodeErr := json.NewDecoder(response.Body).Decode(&payload); decodeErr != nil {
			t.Fatalf("decode error payload: %v", decodeErr)
		}
		if payload.Code != contract.WindowManagerErrorInvalidCommand ||
			payload.WorkspaceID != "workspace-a" {
			t.Fatalf("error payload = %+v", payload)
		}
	})
}

func executeWindowManagerStreamCommand(
	t *testing.T,
	manager *windowmanager.Manager,
	workspaceID windowmanager.WorkspaceID,
	desktopID windowmanager.DesktopID,
) windowmanager.Result {
	t.Helper()
	result, err := manager.Execute(t.Context(), windowmanager.CommandRequest{
		WorkspaceID: workspaceID, CommandID: windowmanager.CommandDesktopCreate, ExpectedRevision: 0,
		Actor: windowmanager.Actor{Kind: "test", ID: "actor"}, Origin: "stream-test",
		Payload: windowmanager.CreateDesktopCommand{
			DesktopID: desktopID, Name: "Second",
		},
	})
	if err != nil {
		t.Fatalf("Execute(%s) error = %v", workspaceID, err)
	}
	if !result.Applied || result.Snapshot.Revision != 1 {
		t.Fatalf("Execute(%s) result = %+v", workspaceID, result)
	}
	return result
}

func executeWindowManagerPresentationSwitch(
	t *testing.T,
	manager *windowmanager.Manager,
	clientID windowmanager.ClientID,
	revision windowmanager.Revision,
) windowmanager.Result {
	t.Helper()
	result, err := manager.Execute(t.Context(), windowmanager.CommandRequest{
		WorkspaceID: "workspace-a", ExpectedRevision: revision, ClientID: &clientID,
		Payload: windowmanager.SwitchDesktopCommand{DesktopID: "desktop-a"},
	})
	if err != nil {
		t.Fatalf("Execute(desktop.switch, %s) error = %v", clientID, err)
	}
	return result
}

func closeWindowManagerDialResponse(t *testing.T, response *http.Response) {
	t.Helper()
	if response == nil || response.Body == nil {
		return
	}
	if err := response.Body.Close(); err != nil {
		t.Errorf("close failed websocket response: %v", err)
	}
}

func TestWindowManagerWebSocketHeartbeat(t *testing.T) {
	t.Run("Should send heartbeat frames that carry the newest announced revision", func(t *testing.T) {
		t.Parallel()
		var sequence atomic.Int64
		manager, err := windowmanager.NewService(
			windowmanager.NewMemoryRepository(),
			windowmanager.NewMemoryWorkspaceResolver("workspace-a"),
			nil,
			windowmanager.DefaultConfig(),
			windowmanager.WithIDGenerator(func(kind string) (string, error) {
				return fmt.Sprintf("%s-%03d", kind, sequence.Add(1)), nil
			}),
		)
		if err != nil {
			t.Fatalf("NewService() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := manager.Close(); closeErr != nil {
				t.Errorf("Manager.Close() error = %v", closeErr)
			}
		})
		handlers := NewBaseHandlers(&BaseHandlerConfig{
			WindowManager:             &singleProfileWindowManagers{manager: manager},
			Profiles:                  windowManagerProfileServiceStub{},
			WindowManagerPingInterval: 20 * time.Millisecond,
		})
		router := gin.New()
		registerWindowManagerTestRoutes(router, handlers)
		server := httptest.NewServer(router)
		t.Cleanup(server.Close)
		t.Cleanup(func() {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(t.Context()), time.Second)
			defer cancel()
			if shutdownErr := handlers.ShutdownWindowManagerStreams(ctx); shutdownErr != nil {
				t.Errorf("ShutdownWindowManagerStreams() error = %v", shutdownErr)
			}
		})

		streamURL := "ws" + strings.TrimPrefix(server.URL, "http") + windowManagerTestPath("workspace-a") + "/stream"
		connection, response, err := websocket.DefaultDialer.DialContext(t.Context(), streamURL, nil)
		if err != nil {
			closeWindowManagerDialResponse(t, response)
			t.Fatalf("DialContext() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := connection.Close(); closeErr != nil {
				t.Errorf("websocket Close() error = %v", closeErr)
			}
		})
		var snapshotFrame contract.WindowManagerSnapshotFrame
		if readErr := connection.ReadJSON(&snapshotFrame); readErr != nil {
			t.Fatalf("ReadJSON(snapshot) error = %v", readErr)
		}
		if deadlineErr := connection.SetReadDeadline(time.Now().Add(2 * time.Second)); deadlineErr != nil {
			t.Fatalf("SetReadDeadline() error = %v", deadlineErr)
		}
		heartbeat := readWindowManagerHeartbeat(t, connection)
		if heartbeat.WorkspaceID != "workspace-a" || heartbeat.Revision != 0 {
			t.Fatalf("initial heartbeat = %+v", heartbeat)
		}

		executeWindowManagerStreamCommand(t, manager, "workspace-a", "desktop-a")
		for {
			heartbeat = readWindowManagerHeartbeat(t, connection)
			if heartbeat.Revision == 1 {
				break
			}
		}
	})
}

// readWindowManagerHeartbeat skips event frames and returns the next heartbeat.
func readWindowManagerHeartbeat(t *testing.T, connection *websocket.Conn) contract.WindowManagerHeartbeatFrame {
	t.Helper()
	for {
		_, payload, err := connection.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage() error = %v", err)
		}
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatalf("decode frame envelope: %v", err)
		}
		if envelope.Type != contract.WindowManagerFrameHeartbeat {
			continue
		}
		var frame contract.WindowManagerHeartbeatFrame
		if err := json.Unmarshal(payload, &frame); err != nil {
			t.Fatalf("decode heartbeat frame: %v", err)
		}
		return frame
	}
}
