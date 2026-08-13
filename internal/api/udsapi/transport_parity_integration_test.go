//go:build integration

package udsapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	apispec "github.com/compozy/compozy/internal/api/spec"
	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/testutil/acpmock"
	e2etest "github.com/compozy/compozy/internal/testutil/e2e"
	transcriptpkg "github.com/compozy/compozy/internal/transcript"
	"github.com/compozy/compozy/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

const (
	transportUDSApprovalAgent    = "transport-uds-approver"
	transportUDSAutoTitleAgent   = "transport-uds-auto-title"
	transportUDSAutomationAgent  = "transport-uds-automation-runner"
	transportUDSFaultyAgent      = "transport-uds-faulty-runner"
	transportUDSObserveAgent     = "transport-uds-observe"
	transportUDSOverrideProvider = "qa-transport-override"
)

func TestUDSTransportSessionOwnerProjectionMatchesHTTP(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should return the same minimal session owner over HTTP and UDS", func(t *testing.T) {
		t.Parallel()

		runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  transportMockFixturePath(t, "automation_task_fixture.json"),
				FixtureAgent: "automation-runner",
				AgentName:    transportUDSAutomationAgent,
			}},
		})

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		sessionPayload, err := runtimeHarness.CreateSession(ctx, compozycontract.CreateSessionRequest{
			AgentName:     transportUDSAutomationAgent,
			WorkspacePath: runtimeHarness.WorkspaceRoot,
		})
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		sessionPayload = waitForTransportSessionActive(t, ctx, runtimeHarness, sessionPayload)

		clients, err := runtimeHarness.TransportClients()
		if err != nil {
			t.Fatalf("TransportClients() error = %v", err)
		}
		path := "/api/sessions/" + url.PathEscape(sessionPayload.ID) + "/owner"
		httpResponse := mustUnixRequest(t, clients.HTTPClient, http.MethodGet, runtimeHarness.HTTPURL(path), nil, nil)
		udsResponse := mustUnixRequest(t, clients.UDSClient, http.MethodGet, runtimeHarness.UDSURL(path), nil, nil)
		httpBody := readAndCloseHTTPBody(t, httpResponse)
		udsBody := readAndCloseHTTPBody(t, udsResponse)

		if httpResponse.StatusCode != http.StatusOK || udsResponse.StatusCode != http.StatusOK {
			t.Fatalf(
				"owner status parity = HTTP %d / UDS %d, want %d; bodies=%s / %s",
				httpResponse.StatusCode,
				udsResponse.StatusCode,
				http.StatusOK,
				httpBody,
				udsBody,
			)
		}
		if !bytes.Equal(httpBody, udsBody) {
			t.Fatalf("owner payloads differ: HTTP=%s UDS=%s", httpBody, udsBody)
		}

		var fields map[string]json.RawMessage
		if err := json.Unmarshal(httpBody, &fields); err != nil {
			t.Fatalf("json.Unmarshal(owner fields) error = %v; body=%s", err, httpBody)
		}
		if len(fields) != 3 || fields["session_id"] == nil || fields["workspace_id"] == nil ||
			fields["workspace_name"] == nil {
			t.Fatalf("owner fields = %v, want exactly session_id workspace_id workspace_name", fields)
		}

		var owner compozycontract.SessionOwner
		if err := json.Unmarshal(httpBody, &owner); err != nil {
			t.Fatalf("json.Unmarshal(owner) error = %v", err)
		}
		if owner.SessionID != sessionPayload.ID || owner.WorkspaceID != sessionPayload.WorkspaceID ||
			strings.TrimSpace(owner.WorkspaceName) == "" {
			t.Fatalf(
				"owner = %#v, want session %q workspace %q and non-empty name",
				owner,
				sessionPayload.ID,
				sessionPayload.WorkspaceID,
			)
		}
	})
}

func TestUDSTransportSessionCommandsProjectionMatchesHTTP(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	t.Run("Should return the same session command catalog and workspace fence over HTTP and UDS", func(t *testing.T) {
		t.Parallel()

		runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			MockAgents: []e2etest.MockAgentSpec{{
				FixturePath:  transportMockFixturePath(t, "automation_task_fixture.json"),
				FixtureAgent: "automation-runner",
				AgentName:    transportUDSAutomationAgent,
			}},
		})

		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		created, err := runtimeHarness.CreateSession(ctx, compozycontract.CreateSessionRequest{
			AgentName:     transportUDSAutomationAgent,
			WorkspacePath: runtimeHarness.WorkspaceRoot,
		})
		if err != nil {
			t.Fatalf("CreateSession() error = %v", err)
		}
		created = waitForTransportSessionActive(t, ctx, runtimeHarness, created)

		clients, err := runtimeHarness.TransportClients()
		if err != nil {
			t.Fatalf("TransportClients() error = %v", err)
		}
		path := transportHarnessSessionPath(t, runtimeHarness, created.ID, "/commands")
		httpResponse := mustUnixRequest(t, clients.HTTPClient, http.MethodGet, runtimeHarness.HTTPURL(path), nil, nil)
		udsResponse := mustUnixRequest(t, clients.UDSClient, http.MethodGet, runtimeHarness.UDSURL(path), nil, nil)
		httpBody := readAndCloseHTTPBody(t, httpResponse)
		udsBody := readAndCloseHTTPBody(t, udsResponse)
		if httpResponse.StatusCode != http.StatusOK || udsResponse.StatusCode != http.StatusOK {
			t.Fatalf(
				"session commands status parity = HTTP %d / UDS %d, want %d; bodies=%s / %s",
				httpResponse.StatusCode,
				udsResponse.StatusCode,
				http.StatusOK,
				httpBody,
				udsBody,
			)
		}
		if !jsonEqual(httpBody, udsBody) {
			t.Fatalf("session commands payloads differ: HTTP=%s UDS=%s", httpBody, udsBody)
		}

		var catalog compozycontract.SessionCommandsResponse
		if err := json.Unmarshal(httpBody, &catalog); err != nil {
			t.Fatalf("json.Unmarshal(session commands) error = %v; body=%s", err, httpBody)
		}
		if strings.TrimSpace(catalog.Revision) == "" || len(catalog.Commands) == 0 {
			t.Fatalf("session commands catalog = %#v, want non-empty revision and commands", catalog)
		}
		for _, command := range catalog.Commands {
			if strings.Contains(command.Source.Key, string(filepath.Separator)) ||
				strings.Contains(command.Source.Key, "/") {
				t.Fatalf("session command source key = %q, want path-free identity", command.Source.Key)
			}
		}

		foreignPath := "/api/workspaces/foreign-workspace/sessions/" + url.PathEscape(created.ID) + "/commands"
		httpForeign := mustUnixRequest(t, clients.HTTPClient, http.MethodGet, runtimeHarness.HTTPURL(foreignPath), nil, nil)
		udsForeign := mustUnixRequest(t, clients.UDSClient, http.MethodGet, runtimeHarness.UDSURL(foreignPath), nil, nil)
		httpForeignBody := readAndCloseHTTPBody(t, httpForeign)
		udsForeignBody := readAndCloseHTTPBody(t, udsForeign)
		if httpForeign.StatusCode != http.StatusNotFound || udsForeign.StatusCode != http.StatusNotFound {
			t.Fatalf(
				"foreign session commands status parity = HTTP %d / UDS %d, want %d; bodies=%s / %s",
				httpForeign.StatusCode,
				udsForeign.StatusCode,
				http.StatusNotFound,
				httpForeignBody,
				udsForeignBody,
			)
		}
		if !jsonEqual(httpForeignBody, udsForeignBody) {
			t.Fatalf("foreign session commands errors differ: HTTP=%s UDS=%s", httpForeignBody, udsForeignBody)
		}
	})
}

func TestUDSTransportDaemonDrainMatchesHTTPAndCLI(t *testing.T) {
	t.Run("Should expose identical safe-to-stop readouts", func(t *testing.T) {
		t.Parallel()
		acpmock.RequireDriver(t)

		runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		var httpDraining compozycontract.DrainStatusResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodPost, "/api/drain", nil, &httpDraining); err != nil {
			t.Fatalf("HTTP drain error = %v", err)
		}
		var udsDraining compozycontract.DrainStatusResponse
		if err := runtimeHarness.UDSJSON(ctx, http.MethodPost, "/api/drain", nil, &udsDraining); err != nil {
			t.Fatalf("UDS drain error = %v", err)
		}
		var cliDraining compozycontract.DrainStatusResponse
		if err := runtimeHarness.CLI.RunJSON(ctx, &cliDraining, "drain", "-o", "json"); err != nil {
			t.Fatalf("CLI drain error = %v", err)
		}
		if httpDraining.State != compozycontract.DrainStateDraining ||
			httpDraining != udsDraining || udsDraining != cliDraining {
			t.Fatalf("drain parity mismatch: HTTP=%#v UDS=%#v CLI=%#v", httpDraining, udsDraining, cliDraining)
		}
		if !httpDraining.AdmissionClosed ||
			!httpDraining.AuthoritativeClaimsSettled ||
			!httpDraining.SessionExecutionSettled ||
			!httpDraining.SafeToStop {
			t.Fatalf("drain safety status = %#v, want closed and safe to stop", httpDraining)
		}

		var cliActive compozycontract.DrainStatusResponse
		if err := runtimeHarness.CLI.RunJSON(ctx, &cliActive, "undrain", "-o", "json"); err != nil {
			t.Fatalf("CLI undrain error = %v", err)
		}
		var udsActive compozycontract.DrainStatusResponse
		if err := runtimeHarness.UDSJSON(ctx, http.MethodPost, "/api/undrain", nil, &udsActive); err != nil {
			t.Fatalf("UDS undrain error = %v", err)
		}
		var httpActive compozycontract.DrainStatusResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodPost, "/api/undrain", nil, &httpActive); err != nil {
			t.Fatalf("HTTP undrain error = %v", err)
		}
		if cliActive.State != compozycontract.DrainStateActive ||
			cliActive != udsActive || udsActive != httpActive {
			t.Fatalf("undrain parity mismatch: CLI=%#v UDS=%#v HTTP=%#v", cliActive, udsActive, httpActive)
		}
		if cliActive.AdmissionClosed || cliActive.SafeToStop {
			t.Fatalf("undrain safety status = %#v, want open and unsafe to stop", cliActive)
		}
	})
}

func TestUDSTransportRuntimeMemoryDoctorMatchesHTTPAndCLI(t *testing.T) {
	t.Run("Should return identical runtime memory data over HTTP UDS and CLI", func(t *testing.T) {
		t.Parallel()
		acpmock.RequireDriver(t)

		runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		var httpPayload compozycontract.DoctorPayload
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, "/api/doctor", nil, &httpPayload); err != nil {
			t.Fatalf("HTTP doctor error = %v", err)
		}
		var udsPayload compozycontract.DoctorPayload
		if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, "/api/doctor", nil, &udsPayload); err != nil {
			t.Fatalf("UDS doctor error = %v", err)
		}
		var cliPayload compozycontract.DoctorPayload
		if err := runtimeHarness.CLI.RunJSON(ctx, &cliPayload, "doctor", "-o", "json"); err != nil {
			t.Fatalf("CLI doctor error = %v", err)
		}

		httpItem := transportDoctorItem(t, httpPayload, "runtime.memory")
		udsItem := transportDoctorItem(t, udsPayload, "runtime.memory")
		cliItem := transportDoctorItem(t, cliPayload, "runtime.memory")
		if !reflect.DeepEqual(httpItem, udsItem) || !reflect.DeepEqual(udsItem, cliItem) {
			t.Fatalf("runtime.memory parity mismatch: HTTP=%#v UDS=%#v CLI=%#v", httpItem, udsItem, cliItem)
		}
		if httpItem.Evidence["enabled"] != true || httpItem.Evidence["active"] != true ||
			httpItem.Evidence["resident_memory_available"] != true ||
			!transportPositiveNumber(httpItem.Evidence["resident_memory_bytes"]) ||
			!transportPositiveNumber(httpItem.Evidence["heap_alloc_bytes"]) ||
			!transportPositiveNumber(httpItem.Evidence["goroutines"]) {
			t.Fatalf("runtime.memory evidence = %#v, want populated daemon snapshot", httpItem.Evidence)
		}
	})
}

func TestUDSTransportWindowManagerMatchesHTTP(t *testing.T) {
	t.Run(
		"Should preserve window manager reads mutations clients layouts and errors across real transports",
		func(t *testing.T) {
			t.Parallel()
			acpmock.RequireDriver(t)

			runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
				MockAgents: []e2etest.MockAgentSpec{{
					FixturePath:  transportMockFixturePath(t, "automation_task_fixture.json"),
					FixtureAgent: "automation-runner",
					AgentName:    transportUDSAutomationAgent,
				}},
			})
			clients, err := runtimeHarness.TransportClients()
			if err != nil {
				t.Fatalf("TransportClients() error = %v", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()

			basePath := "/api/workspaces/" + url.PathEscape(runtimeHarness.WorkspaceID) + "/window-manager"
			var httpSnapshot compozycontract.WindowManagerSnapshot
			if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, basePath, nil, &httpSnapshot); err != nil {
				t.Fatalf("HTTP window manager snapshot error = %v", err)
			}
			var udsSnapshot compozycontract.WindowManagerSnapshot
			if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, basePath, nil, &udsSnapshot); err != nil {
				t.Fatalf("UDS window manager snapshot error = %v", err)
			}
			if !reflect.DeepEqual(httpSnapshot, udsSnapshot) {
				t.Fatalf("window manager snapshot parity mismatch: HTTP=%#v UDS=%#v", httpSnapshot, udsSnapshot)
			}

			createRequest := windowManagerTransportCreateDesktopRequest(
				runtimeHarness.WorkspaceID,
				httpSnapshot.Revision,
				"desktop-http",
			)
			var httpPreview compozycontract.WindowManagerPreview
			if err := runtimeHarness.HTTPJSON(
				ctx,
				http.MethodPost,
				basePath+"/preview",
				createRequest,
				&httpPreview,
			); err != nil {
				t.Fatalf("HTTP window manager preview error = %v", err)
			}
			var udsPreview compozycontract.WindowManagerPreview
			if err := runtimeHarness.UDSJSON(
				ctx,
				http.MethodPost,
				basePath+"/preview",
				createRequest,
				&udsPreview,
			); err != nil {
				t.Fatalf("UDS window manager preview error = %v", err)
			}
			if !reflect.DeepEqual(httpPreview, udsPreview) {
				t.Fatalf("window manager preview parity mismatch: HTTP=%#v UDS=%#v", httpPreview, udsPreview)
			}

			var httpMutation compozycontract.WindowManagerResult
			if err := runtimeHarness.HTTPJSON(
				ctx,
				http.MethodPost,
				basePath+"/commands",
				createRequest,
				&httpMutation,
			); err != nil {
				t.Fatalf("HTTP window manager command error = %v", err)
			}
			if !httpMutation.Applied || httpMutation.Snapshot.Revision != httpSnapshot.Revision+1 {
				t.Fatalf(
					"HTTP window manager mutation = %#v, want applied revision %d",
					httpMutation,
					httpSnapshot.Revision+1,
				)
			}

			udsCreateRequest := windowManagerTransportCreateDesktopRequest(
				runtimeHarness.WorkspaceID,
				httpMutation.Snapshot.Revision,
				"desktop-uds",
			)
			var udsMutation compozycontract.WindowManagerResult
			if err := runtimeHarness.UDSJSON(
				ctx,
				http.MethodPost,
				basePath+"/commands",
				udsCreateRequest,
				&udsMutation,
			); err != nil {
				t.Fatalf("UDS window manager command error = %v", err)
			}
			if !udsMutation.Applied || udsMutation.Snapshot.Revision != httpMutation.Snapshot.Revision+1 {
				t.Fatalf(
					"UDS window manager mutation = %#v, want applied revision %d",
					udsMutation,
					httpMutation.Snapshot.Revision+1,
				)
			}

			openRequest := func(
				revision compozycontract.WindowManagerRevision,
				windowID string,
			) compozycontract.WindowManagerCommandRequest {
				return windowManagerTransportCommandRequest(
					runtimeHarness.WorkspaceID,
					revision,
					compozycontract.WindowManagerCommandWindowOpen,
					json.RawMessage(fmt.Sprintf(
						`{"window":{"id":%q,"app":"tasks","route":{"pathname":%q,"search":{}},`+
							`"desktop_id":"desktop-default","floating_rect":{"x":0.1,"y":0.1,`+
							`"width":0.5,"height":0.5},"insert_tiled":false}}`,
						windowID,
						"/tasks/"+windowID,
					)),
				)
			}
			var firstOpen compozycontract.WindowManagerResult
			if err := runtimeHarness.HTTPJSON(
				ctx,
				http.MethodPost,
				basePath+"/commands",
				openRequest(udsMutation.Snapshot.Revision, "window-a"),
				&firstOpen,
			); err != nil {
				t.Fatalf("HTTP window.open error = %v", err)
			}
			var secondOpen compozycontract.WindowManagerResult
			if err := runtimeHarness.UDSJSON(
				ctx,
				http.MethodPost,
				basePath+"/commands",
				openRequest(firstOpen.Snapshot.Revision, "window-b"),
				&secondOpen,
			); err != nil {
				t.Fatalf("UDS window.open error = %v", err)
			}
			groupRequest := windowManagerTransportCommandRequest(
				runtimeHarness.WorkspaceID,
				secondOpen.Snapshot.Revision,
				compozycontract.WindowManagerCommandWindowStackGroup,
				json.RawMessage(`{"target_window_id":"window-a","window_ids":["window-b"]}`),
			)
			var httpGroupPreview compozycontract.WindowManagerPreview
			if err := runtimeHarness.HTTPJSON(
				ctx,
				http.MethodPost,
				basePath+"/preview",
				groupRequest,
				&httpGroupPreview,
			); err != nil {
				t.Fatalf("HTTP window.stack.group preview error = %v", err)
			}
			var udsGroupPreview compozycontract.WindowManagerPreview
			if err := runtimeHarness.UDSJSON(
				ctx,
				http.MethodPost,
				basePath+"/preview",
				groupRequest,
				&udsGroupPreview,
			); err != nil {
				t.Fatalf("UDS window.stack.group preview error = %v", err)
			}
			if !reflect.DeepEqual(httpGroupPreview, udsGroupPreview) {
				t.Fatalf(
					"window.stack.group preview parity mismatch: HTTP=%#v UDS=%#v",
					httpGroupPreview,
					udsGroupPreview,
				)
			}
			var grouped compozycontract.WindowManagerResult
			if err := runtimeHarness.HTTPJSON(
				ctx,
				http.MethodPost,
				basePath+"/commands",
				groupRequest,
				&grouped,
			); err != nil {
				t.Fatalf("HTTP window.stack.group error = %v", err)
			}
			groupedDesktop := requireWindowManagerDesktop(t, grouped.Snapshot, "desktop-default")
			if !grouped.Applied || grouped.Snapshot.Version != windowmanager.SnapshotVersion ||
				len(groupedDesktop.FloatingStacks) != 1 {
				t.Fatalf("window.stack.group result = %#v", grouped)
			}

			var thirdOpen compozycontract.WindowManagerResult
			if err := runtimeHarness.HTTPJSON(
				ctx,
				http.MethodPost,
				basePath+"/commands",
				openRequest(grouped.Snapshot.Revision, "window-c"),
				&thirdOpen,
			); err != nil {
				t.Fatalf("HTTP third window.open error = %v", err)
			}

			var cliWindows []struct {
				ID          windowmanager.WindowID `json:"id"`
				StackID     *windowmanager.NodeID  `json:"stack_id,omitempty"`
				MemberOrder *int                   `json:"member_order,omitempty"`
				Active      bool                   `json:"active"`
				Pinned      bool                   `json:"pinned"`
				NavDepth    int                    `json:"nav_depth"`
			}
			if err := runtimeHarness.CLI.RunJSON(
				ctx,
				&cliWindows,
				"window", "list", "--workspace", runtimeHarness.WorkspaceID, "-o", "json",
			); err != nil {
				t.Fatalf("CLI window list error = %v", err)
			}
			if len(cliWindows) != 3 {
				t.Fatalf("CLI window list length = %d, want 3: %#v", len(cliWindows), cliWindows)
			}
			listedByID := make(map[windowmanager.WindowID]struct {
				stackID     *windowmanager.NodeID
				memberOrder *int
			}, len(cliWindows))
			for _, item := range cliWindows {
				listedByID[item.ID] = struct {
					stackID     *windowmanager.NodeID
					memberOrder *int
				}{stackID: item.StackID, memberOrder: item.MemberOrder}
			}
			if listedByID["window-a"].stackID == nil || listedByID["window-a"].memberOrder == nil ||
				listedByID["window-b"].stackID == nil || listedByID["window-b"].memberOrder == nil ||
				listedByID["window-c"].stackID != nil {
				t.Fatalf("CLI window list topology = %#v, want grouped a/b and standalone c", cliWindows)
			}

			assertCLIHTTPParity := func(label string, cliSnapshot compozycontract.WindowManagerSnapshot) {
				t.Helper()
				var current compozycontract.WindowManagerSnapshot
				if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, basePath, nil, &current); err != nil {
					t.Fatalf("HTTP snapshot after %s error = %v", label, err)
				}
				cliJSON, err := json.Marshal(cliSnapshot)
				if err != nil {
					t.Fatalf("json.Marshal(CLI snapshot after %s) error = %v", label, err)
				}
				httpJSON, err := json.Marshal(current)
				if err != nil {
					t.Fatalf("json.Marshal(HTTP snapshot after %s) error = %v", label, err)
				}
				if !bytes.Equal(cliJSON, httpJSON) {
					t.Fatalf("CLI/HTTP snapshot parity after %s mismatch: CLI=%s HTTP=%s", label, cliJSON, httpJSON)
				}
			}

			var cliGrouped compozycontract.WindowManagerResult
			if err := runtimeHarness.CLI.RunJSON(
				ctx,
				&cliGrouped,
				"window", "group", "--target", "window-a", "--windows", "window-c",
				"--workspace", runtimeHarness.WorkspaceID,
				"--revision", fmt.Sprint(thirdOpen.Snapshot.Revision),
				"-o", "json",
			); err != nil {
				t.Fatalf("CLI window group error = %v", err)
			}
			assertCLIHTTPParity("group", cliGrouped.Snapshot)

			cliDesktop := requireWindowManagerDesktop(t, cliGrouped.Snapshot, "desktop-default")
			cliStackIndex := -1
			for idx := range cliDesktop.FloatingStacks {
				candidate := &cliDesktop.FloatingStacks[idx]
				if slices.Contains(candidate.WindowIDs, windowmanager.WindowID("window-a")) &&
					slices.Contains(candidate.WindowIDs, windowmanager.WindowID("window-c")) {
					cliStackIndex = idx
					break
				}
			}
			if cliStackIndex < 0 {
				t.Fatalf(
					"CLI grouped stacks = %#v, want stack containing window-a and window-c",
					cliDesktop.FloatingStacks,
				)
			}
			cliStack := cliDesktop.FloatingStacks[cliStackIndex]
			activateID := cliStack.WindowIDs[0]
			if cliStack.ActiveID != nil && activateID == *cliStack.ActiveID {
				activateID = cliStack.WindowIDs[1]
			}
			var cliActivated compozycontract.WindowManagerResult
			if err := runtimeHarness.CLI.RunJSON(
				ctx,
				&cliActivated,
				"window", "activate", string(activateID),
				"--workspace", runtimeHarness.WorkspaceID,
				"--revision", fmt.Sprint(cliGrouped.Snapshot.Revision),
				"-o", "json",
			); err != nil {
				t.Fatalf("CLI window activate error = %v", err)
			}
			assertCLIHTTPParity("activate", cliActivated.Snapshot)

			var cliPinned compozycontract.WindowManagerResult
			if err := runtimeHarness.CLI.RunJSON(
				ctx,
				&cliPinned,
				"window", "pin", string(activateID),
				"--workspace", runtimeHarness.WorkspaceID,
				"--revision", fmt.Sprint(cliActivated.Snapshot.Revision),
				"-o", "json",
			); err != nil {
				t.Fatalf("CLI window pin error = %v", err)
			}
			assertCLIHTTPParity("pin", cliPinned.Snapshot)

			closeID := windowmanager.WindowID("window-c")
			if closeID == activateID {
				closeID = "window-b"
			}
			var cliClosed compozycontract.WindowManagerResult
			if err := runtimeHarness.CLI.RunJSON(
				ctx,
				&cliClosed,
				"window", "close", "--id", string(closeID),
				"--workspace", runtimeHarness.WorkspaceID,
				"--revision", fmt.Sprint(cliPinned.Snapshot.Revision),
				"-o", "json",
			); err != nil {
				t.Fatalf("CLI window close error = %v", err)
			}
			var cliReopened compozycontract.WindowManagerResult
			if err := runtimeHarness.CLI.RunJSON(
				ctx,
				&cliReopened,
				"window", "reopen",
				"--workspace", runtimeHarness.WorkspaceID,
				"--revision", fmt.Sprint(cliClosed.Snapshot.Revision),
				"-o", "json",
			); err != nil {
				t.Fatalf("CLI window reopen error = %v", err)
			}
			assertCLIHTTPParity("reopen", cliReopened.Snapshot)

			_, staleStderr, staleErr := runtimeHarness.CLI.Run(
				ctx,
				"window", "activate", string(activateID),
				"--workspace", runtimeHarness.WorkspaceID,
				"--revision", fmt.Sprint(cliGrouped.Snapshot.Revision),
				"-o", "json",
			)
			if staleErr == nil ||
				!strings.Contains(staleStderr, string(compozycontract.WindowManagerErrorRevisionConflict)) {
				t.Fatalf("stale CLI activation error = %v, stderr=%q", staleErr, staleStderr)
			}

			_, malformedStderr, malformedErr := runtimeHarness.CLI.Run(
				ctx,
				"window", "group", "--target", "window-a", "--windows", "window-b,,window-c",
				"--workspace", runtimeHarness.WorkspaceID,
				"--revision", fmt.Sprint(cliReopened.Snapshot.Revision),
				"-o", "json",
			)
			if malformedErr == nil || !strings.Contains(malformedStderr, "--windows must contain non-empty") {
				t.Fatalf("malformed CLI group error = %v, stderr=%q", malformedErr, malformedStderr)
			}

			var exported compozycontract.WindowManagerLayoutDocument
			if err := runtimeHarness.CLI.RunJSON(
				ctx,
				&exported,
				"layout", "export", "--workspace", runtimeHarness.WorkspaceID, "-o", "json",
			); err != nil {
				t.Fatalf("CLI layout export error = %v", err)
			}
			exportedJSON, err := json.Marshal(exported)
			if err != nil {
				t.Fatalf("json.Marshal(exported layout) error = %v", err)
			}
			layoutPath := filepath.Join(t.TempDir(), "window-layout.json")
			if err := os.WriteFile(layoutPath, exportedJSON, 0o600); err != nil {
				t.Fatalf("os.WriteFile(%q) error = %v", layoutPath, err)
			}

			var cliClosedAgain compozycontract.WindowManagerResult
			if err := runtimeHarness.CLI.RunJSON(
				ctx,
				&cliClosedAgain,
				"window", "close", "--id", string(closeID),
				"--workspace", runtimeHarness.WorkspaceID,
				"--revision", fmt.Sprint(cliReopened.Snapshot.Revision),
				"-o", "json",
			); err != nil {
				t.Fatalf("CLI second window close error = %v", err)
			}
			var cliApplied compozycontract.WindowManagerResult
			if err := runtimeHarness.CLI.RunJSON(
				ctx,
				&cliApplied,
				"layout", "apply", "--file", layoutPath,
				"--workspace", runtimeHarness.WorkspaceID,
				"--revision", fmt.Sprint(cliClosedAgain.Snapshot.Revision),
				"-o", "json",
			); err != nil {
				t.Fatalf("CLI layout apply error = %v", err)
			}
			assertCLIHTTPParity("layout apply", cliApplied.Snapshot)
			var appliedLayout compozycontract.WindowManagerLayoutDocument
			if err := runtimeHarness.HTTPJSON(
				ctx,
				http.MethodGet,
				basePath+"/layout",
				nil,
				&appliedLayout,
			); err != nil {
				t.Fatalf("HTTP layout export after CLI apply error = %v", err)
			}
			if !reflect.DeepEqual(exported, appliedLayout) {
				t.Fatalf("layout round-trip mismatch: exported=%#v applied=%#v", exported, appliedLayout)
			}
			assertWindowManagerLayoutProfileRoundTrip(
				t,
				ctx,
				runtimeHarness,
				basePath,
				exported,
			)
			assertWindowManagerSessionReopenAcrossClients(
				t,
				ctx,
				runtimeHarness,
				basePath,
				cliApplied.Snapshot.Revision,
			)

			registration := compozycontract.WindowManagerClientRegistration{
				WorkspaceID: windowmanager.WorkspaceID(runtimeHarness.WorkspaceID),
				ClientID:    "transport-parity-client",
			}
			var httpClient compozycontract.WindowManagerClientView
			if err := runtimeHarness.HTTPJSON(
				ctx,
				http.MethodPost,
				basePath+"/clients",
				registration,
				&httpClient,
			); err != nil {
				t.Fatalf("HTTP window manager client registration error = %v", err)
			}
			var udsClient compozycontract.WindowManagerClientView
			if err := runtimeHarness.UDSJSON(
				ctx,
				http.MethodPost,
				basePath+"/clients",
				registration,
				&udsClient,
			); err != nil {
				t.Fatalf("UDS window manager client registration error = %v", err)
			}
			if !reflect.DeepEqual(httpClient, udsClient) {
				t.Fatalf("window manager client parity mismatch: HTTP=%#v UDS=%#v", httpClient, udsClient)
			}

			var httpLayout compozycontract.WindowManagerLayoutDocument
			if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, basePath+"/layout", nil, &httpLayout); err != nil {
				t.Fatalf("HTTP window manager layout export error = %v", err)
			}
			var udsLayout compozycontract.WindowManagerLayoutDocument
			if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, basePath+"/layout", nil, &udsLayout); err != nil {
				t.Fatalf("UDS window manager layout export error = %v", err)
			}
			if !reflect.DeepEqual(httpLayout, udsLayout) {
				t.Fatalf("window manager layout parity mismatch: HTTP=%#v UDS=%#v", httpLayout, udsLayout)
			}

			validationRequest := compozycontract.WindowManagerLayoutValidationRequest{
				WorkspaceID: windowmanager.WorkspaceID(runtimeHarness.WorkspaceID),
				Document:    httpLayout,
			}
			var httpValidation compozycontract.WindowManagerLayoutValidationResponse
			if err := runtimeHarness.HTTPJSON(
				ctx,
				http.MethodPost,
				basePath+"/layout/validate",
				validationRequest,
				&httpValidation,
			); err != nil {
				t.Fatalf("HTTP window manager layout validation error = %v", err)
			}
			var udsValidation compozycontract.WindowManagerLayoutValidationResponse
			if err := runtimeHarness.UDSJSON(
				ctx,
				http.MethodPost,
				basePath+"/layout/validate",
				validationRequest,
				&udsValidation,
			); err != nil {
				t.Fatalf("UDS window manager layout validation error = %v", err)
			}
			if !reflect.DeepEqual(httpValidation, udsValidation) || !httpValidation.Valid {
				t.Fatalf("window manager validation parity mismatch: HTTP=%#v UDS=%#v", httpValidation, udsValidation)
			}

			invalidRequest := windowManagerTransportCreateDesktopRequest(
				runtimeHarness.WorkspaceID,
				udsMutation.Snapshot.Revision,
				"desktop-invalid",
			)
			invalidRequest.CommandID = "desktop.unknown"
			invalidBody, err := json.Marshal(invalidRequest)
			if err != nil {
				t.Fatalf("json.Marshal(invalid window manager request) error = %v", err)
			}
			httpErrorResponse := mustUnixRequest(
				t,
				clients.HTTPClient,
				http.MethodPost,
				runtimeHarness.HTTPURL(basePath+"/commands"),
				invalidBody,
				nil,
			)
			udsErrorResponse := mustUnixRequest(
				t,
				clients.UDSClient,
				http.MethodPost,
				runtimeHarness.UDSURL(basePath+"/commands"),
				invalidBody,
				nil,
			)
			httpErrorBody := readAndCloseHTTPBody(t, httpErrorResponse)
			udsErrorBody := readAndCloseHTTPBody(t, udsErrorResponse)
			if httpErrorResponse.StatusCode != http.StatusUnprocessableEntity ||
				udsErrorResponse.StatusCode != http.StatusUnprocessableEntity ||
				!jsonEqual(httpErrorBody, udsErrorBody) {
				t.Fatalf(
					"window manager error parity mismatch: HTTP=(%d %s) UDS=(%d %s)",
					httpErrorResponse.StatusCode,
					httpErrorBody,
					udsErrorResponse.StatusCode,
					udsErrorBody,
				)
			}
		},
	)
}

func assertWindowManagerLayoutProfileRoundTrip(
	t *testing.T,
	ctx context.Context,
	runtimeHarness *e2etest.RuntimeHarness,
	basePath string,
	exported compozycontract.WindowManagerLayoutDocument,
) {
	t.Helper()

	const profileID = "transport-tabs-profile"
	profileDocument := exported
	profileDocument.WorkspaceID = ""
	profileSpec := windowManagerTransportPayload(t, windowmanager.LayoutResource{
		Version:        windowmanager.LayoutResourceVersion,
		ID:             profileID,
		DisplayName:    "Transport tabs profile",
		AspectVariant:  windowmanager.LayoutAspectAny,
		OverflowPolicy: windowmanager.LayoutOverflowStack,
		Document:       profileDocument.Domain(),
	})
	var stored compozycontract.ResourceResponse
	if err := runtimeHarness.HTTPJSON(
		ctx,
		http.MethodPut,
		basePath+"/layout-profiles/"+profileID,
		compozycontract.PutResourceRequest{
			Scope: resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
			Spec:  profileSpec,
		},
		&stored,
	); err != nil {
		t.Fatalf("HTTP layout profile put error = %v", err)
	}
	if stored.Record.ID != profileID || stored.Record.Scope.Kind != resources.ResourceScopeKindGlobal {
		t.Fatalf("stored layout profile = %#v, want global profile %q", stored.Record, profileID)
	}

	originalWorkspaceID := runtimeHarness.WorkspaceID
	defer func() { runtimeHarness.WorkspaceID = originalWorkspaceID }()
	freshWorkspace, err := runtimeHarness.ResolveWorkspace(ctx, t.TempDir())
	if err != nil {
		t.Fatalf("ResolveWorkspace(fresh layout target) error = %v", err)
	}
	freshBasePath := "/api/workspaces/" + url.PathEscape(freshWorkspace.ID) + "/window-manager"
	var freshSnapshot compozycontract.WindowManagerSnapshot
	if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, freshBasePath, nil, &freshSnapshot); err != nil {
		t.Fatalf("HTTP fresh workspace snapshot error = %v", err)
	}
	applyRequest := windowManagerTransportCommandRequest(
		freshWorkspace.ID,
		freshSnapshot.Revision,
		compozycontract.WindowManagerCommandLayoutArrange,
		windowManagerTransportPayload(t, compozycontract.WindowManagerArrangeLayoutPayload{
			ResourceID: profileID,
		}),
	)
	var applied compozycontract.WindowManagerResult
	if err := runtimeHarness.HTTPJSON(
		ctx,
		http.MethodPost,
		freshBasePath+"/commands",
		applyRequest,
		&applied,
	); err != nil {
		t.Fatalf("HTTP layout profile apply error = %v", err)
	}
	var reconstructed compozycontract.WindowManagerLayoutDocument
	if err := runtimeHarness.HTTPJSON(
		ctx,
		http.MethodGet,
		freshBasePath+"/layout",
		nil,
		&reconstructed,
	); err != nil {
		t.Fatalf("HTTP reconstructed profile export error = %v", err)
	}
	wantReconstructed := exported
	wantReconstructed.WorkspaceID = windowmanager.WorkspaceID(freshWorkspace.ID)
	if !applied.Applied || !reflect.DeepEqual(reconstructed, wantReconstructed) {
		t.Fatalf(
			"profile reconstruction mismatch: applied=%t got=%#v want=%#v",
			applied.Applied,
			reconstructed,
			wantReconstructed,
		)
	}

	v2Document := reconstructed
	v2Document.Version = 2
	v2Revision := applied.Snapshot.Revision
	v2Body := windowManagerTransportPayload(t, compozycontract.WindowManagerLayoutReplaceRequest{
		WorkspaceID:      windowmanager.WorkspaceID(freshWorkspace.ID),
		ExpectedRevision: &v2Revision,
		Actor:            compozycontract.WindowManagerActor{Kind: "test", ID: "transport-parity"},
		Origin:           "transport-parity",
		Document:         v2Document,
	})
	response := mustUnixRequest(
		t,
		runtimeHarness.HTTPClient,
		http.MethodPut,
		runtimeHarness.HTTPURL(freshBasePath+"/layout"),
		v2Body,
		nil,
	)
	body := readAndCloseHTTPBody(t, response)
	var payload compozycontract.WindowManagerErrorPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("json.Unmarshal(v2 layout error) error = %v; body=%s", err, body)
	}
	if response.StatusCode != http.StatusUnprocessableEntity ||
		payload.Code != compozycontract.WindowManagerErrorInvalidTopology ||
		len(payload.Diagnostics) == 0 ||
		payload.Diagnostics[0].Code != "topology.unsupported_version" {
		t.Fatalf("v2 layout apply = status %d payload %#v", response.StatusCode, payload)
	}
}

func assertWindowManagerSessionReopenAcrossClients(
	t *testing.T,
	ctx context.Context,
	runtimeHarness *e2etest.RuntimeHarness,
	basePath string,
	revision compozycontract.WindowManagerRevision,
) {
	t.Helper()

	sessionPayload, err := runtimeHarness.CreateSession(ctx, compozycontract.CreateSessionRequest{
		AgentName:     transportUDSAutomationAgent,
		WorkspacePath: runtimeHarness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession(window tab identity) error = %v", err)
	}
	sessionPayload = waitForTransportSessionActive(t, ctx, runtimeHarness, sessionPayload)

	clientA := windowmanager.ClientID("session-tab-client-a")
	clientB := windowmanager.ClientID("session-tab-client-b")
	for _, clientID := range []windowmanager.ClientID{clientA, clientB} {
		var registered compozycontract.WindowManagerClientView
		if err := runtimeHarness.HTTPJSON(
			ctx,
			http.MethodPost,
			basePath+"/clients",
			compozycontract.WindowManagerClientRegistration{
				WorkspaceID: windowmanager.WorkspaceID(runtimeHarness.WorkspaceID),
				ClientID:    clientID,
			},
			&registered,
		); err != nil {
			t.Fatalf("HTTP register window client %q error = %v", clientID, err)
		}
	}

	instanceKey := sessionPayload.ID
	openRequest := windowManagerTransportCommandRequest(
		runtimeHarness.WorkspaceID,
		revision,
		compozycontract.WindowManagerCommandWindowOpen,
		windowManagerTransportPayload(t, compozycontract.WindowManagerOpenWindowPayload{
			Window: compozycontract.WindowManagerWindowSpecPayload{
				ID:          "window-session",
				App:         "sessions",
				InstanceKey: &instanceKey,
				Route: windowmanager.RouteIntent{
					Pathname: "/sessions/" + sessionPayload.ID,
					Search:   windowmanager.RouteSearch{},
				},
				DesktopID:    "desktop-default",
				FloatingRect: windowmanager.NormalizedRect{X: 0.2, Y: 0.2, Width: 0.6, Height: 0.6},
			},
		}),
	)
	openRequest.ClientID = &clientA
	var opened compozycontract.WindowManagerResult
	if err := runtimeHarness.HTTPJSON(
		ctx,
		http.MethodPost,
		basePath+"/commands",
		openRequest,
		&opened,
	); err != nil {
		t.Fatalf("HTTP session window open error = %v", err)
	}
	navigateRequest := windowManagerTransportCommandRequest(
		runtimeHarness.WorkspaceID,
		opened.Snapshot.Revision,
		compozycontract.WindowManagerCommandWindowNavigate,
		windowManagerTransportPayload(t, compozycontract.WindowManagerNavigateWindowPayload{
			WindowID: "window-session",
			Route: windowmanager.RouteIntent{
				Pathname: "/sessions/" + sessionPayload.ID + "/transcript",
				Search:   windowmanager.RouteSearch{"view": json.RawMessage(`"full"`)},
			},
			Mode: windowmanager.NavigatePush,
		}),
	)
	navigateRequest.ClientID = &clientA
	var navigated compozycontract.WindowManagerResult
	if err := runtimeHarness.HTTPJSON(
		ctx,
		http.MethodPost,
		basePath+"/commands",
		navigateRequest,
		&navigated,
	); err != nil {
		t.Fatalf("HTTP session window navigate error = %v", err)
	}
	wantWindow := navigated.Snapshot.Windows["window-session"]

	sessionPath := "/api/sessions/" + url.PathEscape(sessionPayload.ID)
	var beforeClose compozycontract.SessionResponse
	if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, sessionPath, nil, &beforeClose); err != nil {
		t.Fatalf("HTTP session state before close error = %v", err)
	}
	closeRequest := windowManagerTransportCommandRequest(
		runtimeHarness.WorkspaceID,
		navigated.Snapshot.Revision,
		compozycontract.WindowManagerCommandWindowClose,
		windowManagerTransportPayload(t, compozycontract.WindowManagerCloseWindowPayload{
			WindowID: "window-session",
			Scope:    windowmanager.CloseScopeTab,
		}),
	)
	closeRequest.ClientID = &clientA
	var closed compozycontract.WindowManagerResult
	if err := runtimeHarness.HTTPJSON(
		ctx,
		http.MethodPost,
		basePath+"/commands",
		closeRequest,
		&closed,
	); err != nil {
		t.Fatalf("HTTP session window close error = %v", err)
	}
	var afterClose compozycontract.SessionResponse
	if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, sessionPath, nil, &afterClose); err != nil {
		t.Fatalf("HTTP session state after close error = %v", err)
	}
	if beforeClose.Session.ID != afterClose.Session.ID ||
		beforeClose.Session.State != afterClose.Session.State ||
		beforeClose.Session.WorkspaceID != afterClose.Session.WorkspaceID ||
		beforeClose.Session.AgentName != afterClose.Session.AgentName {
		t.Fatalf("stable session identity changed when its tab closed: before=%#v after=%#v", beforeClose, afterClose)
	}

	reopenRequest := windowManagerTransportCommandRequest(
		runtimeHarness.WorkspaceID,
		closed.Snapshot.Revision,
		compozycontract.WindowManagerCommandWindowReopen,
		windowManagerTransportPayload(t, compozycontract.WindowManagerReopenWindowPayload{}),
	)
	reopenRequest.ClientID = &clientB
	var reopened compozycontract.WindowManagerResult
	if err := runtimeHarness.HTTPJSON(
		ctx,
		http.MethodPost,
		basePath+"/commands",
		reopenRequest,
		&reopened,
	); err != nil {
		t.Fatalf("HTTP session window reopen from second client error = %v", err)
	}
	if !reopened.Applied || !reflect.DeepEqual(reopened.Snapshot.Windows["window-session"], wantWindow) {
		t.Fatalf(
			"reopened session window = %#v, want preserved route and nav stack %#v",
			reopened.Snapshot.Windows["window-session"],
			wantWindow,
		)
	}
}

func windowManagerTransportPayload(t *testing.T, payload any) json.RawMessage {
	t.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal(window manager transport payload) error = %v", err)
	}
	return raw
}

func requireWindowManagerDesktop(
	t testing.TB,
	snapshot compozycontract.WindowManagerSnapshot,
	desktopID windowmanager.DesktopID,
) compozycontract.WindowManagerDesktop {
	t.Helper()
	for _, desktop := range snapshot.Desktops {
		if desktop.ID == desktopID {
			return desktop
		}
	}
	t.Fatalf("desktop %q missing from snapshot %#v", desktopID, snapshot.Desktops)
	return compozycontract.WindowManagerDesktop{}
}

func windowManagerTransportCreateDesktopRequest(
	workspaceID string,
	revision compozycontract.WindowManagerRevision,
	desktopID string,
) compozycontract.WindowManagerCommandRequest {
	return windowManagerTransportCommandRequest(
		workspaceID,
		revision,
		compozycontract.WindowManagerCommandDesktopCreate,
		json.RawMessage(fmt.Sprintf(
			`{"desktop_id":%q,"name":%q,"purpose":"standard"}`,
			desktopID,
			desktopID,
		)),
	)
}

func windowManagerTransportCommandRequest(
	workspaceID string,
	revision compozycontract.WindowManagerRevision,
	commandID compozycontract.WindowManagerCommandID,
	payload json.RawMessage,
) compozycontract.WindowManagerCommandRequest {
	return compozycontract.WindowManagerCommandRequest{
		WorkspaceID:      windowmanager.WorkspaceID(workspaceID),
		CommandID:        commandID,
		ExpectedRevision: &revision,
		Actor:            compozycontract.WindowManagerActor{Kind: "test", ID: "transport-parity"},
		Origin:           "transport-parity",
		Payload:          payload,
	}
}

func jsonEqual(left []byte, right []byte) bool {
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

func TestUDSTransportAutomaticSessionTitlePersistsAndMatchesHTTP(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "auto_title_fixture.json"),
			FixtureAgent: "auto-title-agent",
			AgentName:    transportUDSAutoTitleAgent,
		}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	sessionPayload, err := runtimeHarness.CreateSession(ctx, compozycontract.CreateSessionRequest{
		AgentName:     transportUDSAutoTitleAgent,
		WorkspacePath: runtimeHarness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	if sessionPayload.Name != "" {
		t.Fatalf("created session name = %q, want unnamed", sessionPayload.Name)
	}
	sessionPayload = waitForTransportSessionActive(t, ctx, runtimeHarness, sessionPayload)

	if _, err := runtimeHarness.PromptSessionHTTP(
		ctx,
		sessionPayload.ID,
		"Implement checkout retry fencing",
	); err != nil {
		t.Fatalf("PromptSessionHTTP() error = %v", err)
	}

	const generatedTitle = "Checkout Retry Fencing"
	httpCatalog, udsCatalog := waitForTransportSessionTitle(
		t,
		ctx,
		runtimeHarness,
		sessionPayload.ID,
		generatedTitle,
	)
	assertTransportCatalogTitle(t, "HTTP", httpCatalog, sessionPayload.ID, generatedTitle)
	assertTransportCatalogTitle(t, "UDS", udsCatalog, sessionPayload.ID, generatedTitle)

	metaPath := store.SessionMetaFile(filepath.Join(runtimeHarness.HomePaths.SessionsDir, sessionPayload.ID))
	meta, err := store.ReadSessionMeta(metaPath)
	if err != nil {
		t.Fatalf("ReadSessionMeta(%q) error = %v", metaPath, err)
	}
	if meta.Name != generatedTitle {
		t.Fatalf("session meta name = %q, want %q", meta.Name, generatedTitle)
	}
}

func TestUDSTransportApprovalFlowMatchesHTTP(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "permission_env_fixture.json"),
			FixtureAgent: "approver",
			AgentName:    transportUDSApprovalAgent,
		}},
	})

	clients, err := runtimeHarness.TransportClients()
	if err != nil {
		t.Fatalf("TransportClients() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session, err := runtimeHarness.CreateSession(ctx, compozycontract.CreateSessionRequest{
		AgentName:     transportUDSApprovalAgent,
		WorkspacePath: runtimeHarness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	session = waitForTransportSessionActive(t, ctx, runtimeHarness, session)

	var approvedRequestID string
	events, err := runtimeHarness.PromptSessionHTTPWithEvents(
		ctx,
		session.ID,
		"request permission",
		func(record e2etest.SSEEvent) error {
			payload, ok := e2etest.PermissionPayloadFromSSE(record)
			if !ok || payload.Decision != "" || approvedRequestID != "" {
				return nil
			}
			approvedRequestID = payload.RequestID

			resp := mustUnixRequest(
				t,
				clients.UDSClient,
				http.MethodPost,
				clients.UDSBaseURL+transportHarnessSessionPath(t, runtimeHarness, session.ID, "/approve"),
				[]byte(fmt.Sprintf(`{"request_id":"%s","decision":"allow-always"}`, payload.RequestID)),
				nil,
			)
			body, readErr := io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			if readErr != nil && closeErr != nil {
				return errors.Join(
					fmt.Errorf("read UDS approval response: %w", readErr),
					fmt.Errorf("close UDS approval response body: %w", closeErr),
				)
			}
			if readErr != nil {
				return fmt.Errorf("read UDS approval response: %w", readErr)
			}
			if closeErr != nil {
				return fmt.Errorf("close UDS approval response body: %w", closeErr)
			}
			if err := e2etest.ValidateUDSApprovalResponse(resp.StatusCode, body); err != nil {
				return err
			}
			return nil
		},
	)
	if err != nil {
		t.Fatalf("PromptSessionHTTPWithEvents() error = %v", err)
	}
	if approvedRequestID == "" {
		t.Fatal("approvedRequestID = empty, want pending permission request")
	}

	payloads := e2etest.PermissionPayloads(events)
	if got, want := len(payloads), 2; got != want {
		t.Fatalf("len(permission payloads) = %d, want %d; payloads=%#v", got, want, payloads)
	}
	if got, want := payloads[0].RequestID, approvedRequestID; got != want {
		t.Fatalf("payloads[0].RequestID = %q, want %q", got, want)
	}
	if got := payloads[len(payloads)-1].Decision; got != "allow-always" {
		t.Fatalf("final permission decision = %q, want %q", got, "allow-always")
	}
	if !e2etest.RecordsContainTextDelta(events, "allow-always") {
		t.Fatalf("prompt events = %#v, want allow-always text delta", events)
	}
}

func TestUDSTransportSessionRuntimeCreateReadMatchesHTTP(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "automation_task_fixture.json"),
			FixtureAgent: "automation-runner",
			AgentName:    transportUDSAutomationAgent,
		}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var created compozycontract.SessionResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodPost, "/api/sessions", compozycontract.CreateSessionRequest{
		AgentName:     transportUDSAutomationAgent,
		WorkspacePath: runtimeHarness.WorkspaceRoot,
	}, &created); err != nil {
		t.Fatalf("UDS create session error = %v", err)
	}
	if created.Session.ID == "" {
		t.Fatal("UDS create session id = empty, want non-empty")
	}
	if created.Session.Runtime.Status != "unbound" || created.Session.Runtime.Effective != nil {
		t.Fatalf("UDS create runtime = %#v, want unbound without effective selection", created.Session.Runtime)
	}
	if created.Session.State != session.StateActive {
		t.Fatalf("UDS accepted create state = %q, want %q", created.Session.State, session.StateActive)
	}
	created.Session = waitForTransportSessionActive(t, ctx, runtimeHarness, created.Session)

	var udsDetail compozycontract.SessionResponse
	if err := runtimeHarness.UDSJSON(
		ctx,
		http.MethodGet,
		transportHarnessSessionPath(t, runtimeHarness, created.Session.ID, ""),
		nil,
		&udsDetail,
	); err != nil {
		t.Fatalf("UDS get session error = %v", err)
	}
	if udsDetail.Session.Runtime.Status != created.Session.Runtime.Status ||
		udsDetail.Session.Runtime.Effective != nil {
		t.Fatalf(
			"UDS detail runtime = %#v, want unbound without effective selection",
			udsDetail.Session.Runtime,
		)
	}

	var httpDetail compozycontract.SessionResponse
	if err := runtimeHarness.HTTPJSON(
		ctx,
		http.MethodGet,
		transportHarnessSessionPath(t, runtimeHarness, created.Session.ID, ""),
		nil,
		&httpDetail,
	); err != nil {
		t.Fatalf("HTTP get session error = %v", err)
	}
	if httpDetail.Session.Runtime.Status != created.Session.Runtime.Status ||
		httpDetail.Session.Runtime.Effective != nil {
		t.Fatalf(
			"HTTP detail runtime = %#v, want unbound without effective selection",
			httpDetail.Session.Runtime,
		)
	}

	assertTransportSessionProvenanceParity(t, ctx, runtimeHarness, created.Session)
}

func assertTransportSessionProvenanceParity(
	t *testing.T,
	ctx context.Context,
	runtimeHarness *e2etest.RuntimeHarness,
	parent compozycontract.SessionPayload,
) {
	t.Helper()

	var explicitChild compozycontract.SessionResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodPost, "/api/sessions", compozycontract.CreateSessionRequest{
		AgentName:       parent.AgentName,
		WorkspacePath:   runtimeHarness.WorkspaceRoot,
		ParentSessionID: parent.ID,
	}, &explicitChild); err != nil {
		t.Fatalf("UDS create explicit provenance child error = %v", err)
	}
	if explicitChild.Session.Type != session.SessionTypeUser {
		t.Fatalf("explicit child type = %q, want %q", explicitChild.Session.Type, session.SessionTypeUser)
	}
	if explicitChild.Session.Lineage == nil ||
		explicitChild.Session.Lineage.ParentSessionID != parent.ID ||
		explicitChild.Session.Lineage.RootSessionID != parent.ID ||
		explicitChild.Session.Lineage.SpawnDepth != 1 {
		t.Fatalf(
			"explicit child lineage = %#v, want parent/root %q depth 1",
			explicitChild.Session.Lineage,
			parent.ID,
		)
	}
	if explicitChild.Session.Lineage.TTLExpiresAt != nil || explicitChild.Session.Lineage.AutoStopOnParent {
		t.Fatalf("explicit child lineage = %#v, want no governance coupling", explicitChild.Session.Lineage)
	}

	inferredBody, err := json.Marshal(compozycontract.CreateSessionRequest{
		AgentName:     parent.AgentName,
		WorkspacePath: runtimeHarness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("marshal inferred create request error = %v", err)
	}
	inferredResp := mustUnixRequest(
		t,
		runtimeHarness.UDSClient,
		http.MethodPost,
		runtimeHarness.UDSURL("/api/sessions"),
		inferredBody,
		map[string]string{
			agentidentity.HeaderSessionID: parent.ID,
			agentidentity.HeaderAgent:     parent.AgentName,
		},
	)
	inferredPayload, readErr := io.ReadAll(inferredResp.Body)
	closeErr := inferredResp.Body.Close()
	if readErr != nil {
		t.Fatalf("read inferred create response error = %v", readErr)
	}
	if closeErr != nil {
		t.Errorf("close inferred create response error = %v", closeErr)
	}
	if inferredResp.StatusCode != http.StatusCreated {
		t.Fatalf(
			"inferred create status = %d, want %d; body=%s",
			inferredResp.StatusCode,
			http.StatusCreated,
			strings.TrimSpace(string(inferredPayload)),
		)
	}
	var inferredChild compozycontract.SessionResponse
	if err := json.Unmarshal(inferredPayload, &inferredChild); err != nil {
		t.Fatalf("decode inferred create response error = %v", err)
	}
	if inferredChild.Session.Lineage == nil ||
		inferredChild.Session.Lineage.ParentSessionID != parent.ID {
		t.Fatalf(
			"identity-inferred child lineage = %#v, want caller parent %q",
			inferredChild.Session.Lineage,
			parent.ID,
		)
	}

	childIDs := []string{explicitChild.Session.ID, inferredChild.Session.ID}
	slices.Sort(childIDs)
	parentQuery := "/api/sessions?parent=" + url.QueryEscape(parent.ID)
	rootQuery := "/api/sessions?root=" + url.QueryEscape(parent.ID)

	var udsChildren compozycontract.SessionCatalogResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, parentQuery, nil, &udsChildren); err != nil {
		t.Fatalf("UDS list parent filter error = %v", err)
	}
	var httpChildren compozycontract.SessionCatalogResponse
	if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, parentQuery, nil, &httpChildren); err != nil {
		t.Fatalf("HTTP list parent filter error = %v", err)
	}
	udsChildIDs := transportSessionCatalogIDs(udsChildren)
	httpChildIDs := transportSessionCatalogIDs(httpChildren)
	if !slices.Equal(udsChildIDs, childIDs) || !slices.Equal(httpChildIDs, childIDs) {
		t.Fatalf(
			"parent filter ids uds=%#v http=%#v, want %#v on both transports",
			udsChildIDs,
			httpChildIDs,
			childIDs,
		)
	}
	if udsChildren.Page.Total != 2 || httpChildren.Page.Total != 2 {
		t.Fatalf(
			"parent filter totals uds=%d http=%d, want 2 on both transports",
			udsChildren.Page.Total,
			httpChildren.Page.Total,
		)
	}

	var udsTree compozycontract.SessionCatalogResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, rootQuery, nil, &udsTree); err != nil {
		t.Fatalf("UDS list root filter error = %v", err)
	}
	var httpTree compozycontract.SessionCatalogResponse
	if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, rootQuery, nil, &httpTree); err != nil {
		t.Fatalf("HTTP list root filter error = %v", err)
	}
	wantTree := append([]string{parent.ID}, childIDs...)
	slices.Sort(wantTree)
	udsTreeIDs := transportSessionCatalogIDs(udsTree)
	httpTreeIDs := transportSessionCatalogIDs(httpTree)
	if !slices.Equal(udsTreeIDs, wantTree) || !slices.Equal(httpTreeIDs, wantTree) {
		t.Fatalf(
			"root filter ids uds=%#v http=%#v, want whole tree %#v on both transports",
			udsTreeIDs,
			httpTreeIDs,
			wantTree,
		)
	}
	if udsTree.Page.Total != 3 || httpTree.Page.Total != 3 {
		t.Fatalf(
			"root filter totals uds=%d http=%d, want 3 on both transports",
			udsTree.Page.Total,
			httpTree.Page.Total,
		)
	}
}

func transportSessionCatalogIDs(page compozycontract.SessionCatalogResponse) []string {
	ids := make([]string, 0, len(page.Sessions))
	for _, item := range page.Sessions {
		ids = append(ids, item.ID)
	}
	slices.Sort(ids)
	return ids
}

func TestUDSTransportStoppedSessionRemainsUnattachable(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "automation_task_fixture.json"),
			FixtureAgent: "automation-runner",
			AgentName:    transportUDSAutomationAgent,
		}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var created compozycontract.SessionResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodPost, "/api/sessions", compozycontract.CreateSessionRequest{
		AgentName:     transportUDSAutomationAgent,
		WorkspacePath: runtimeHarness.WorkspaceRoot,
	}, &created); err != nil {
		t.Fatalf("UDS create session error = %v", err)
	}
	created.Session = waitForTransportSessionActive(t, ctx, runtimeHarness, created.Session)

	stopResp := mustUnixRequest(
		t,
		runtimeHarness.UDSClient,
		http.MethodPost,
		runtimeHarness.UDSURL(transportHarnessSessionPath(t, runtimeHarness, created.Session.ID, "/stop")),
		nil,
		nil,
	)
	body, readErr := io.ReadAll(stopResp.Body)
	closeErr := stopResp.Body.Close()
	if readErr != nil {
		t.Fatalf("read UDS stop body error = %v", readErr)
	}
	if closeErr != nil {
		t.Fatalf("close UDS stop body error = %v", closeErr)
	}
	if stopResp.StatusCode != http.StatusNoContent {
		t.Fatalf(
			"UDS stop session status = %d, want %d; body=%s",
			stopResp.StatusCode,
			http.StatusNoContent,
			string(body),
		)
	}

	resumeResp := mustUnixRequest(
		t,
		runtimeHarness.UDSClient,
		http.MethodPost,
		runtimeHarness.UDSURL(transportHarnessSessionPath(t, runtimeHarness, created.Session.ID, "/attach")),
		nil,
		nil,
	)
	resumeBody, resumeReadErr := io.ReadAll(resumeResp.Body)
	resumeCloseErr := resumeResp.Body.Close()
	if resumeReadErr != nil {
		t.Fatalf("read UDS resume body error = %v", resumeReadErr)
	}
	if resumeCloseErr != nil {
		t.Fatalf("close UDS resume body error = %v", resumeCloseErr)
	}
	if resumeResp.StatusCode != http.StatusConflict {
		t.Fatalf(
			"UDS attach status = %d, want %d; body=%s",
			resumeResp.StatusCode,
			http.StatusConflict,
			string(resumeBody),
		)
	}
	if !strings.Contains(string(resumeBody), created.Session.ID) {
		t.Fatalf("UDS attach body = %s, want session id %q", string(resumeBody), created.Session.ID)
	}
	if !strings.Contains(string(resumeBody), "not attachable") {
		t.Fatalf("UDS attach body = %s, want attachability context", string(resumeBody))
	}
}

func TestUDSTransportProjectionParityMatchesHTTPAndCLI(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{
			DefaultAgent: transportUDSAutomationAgent,
		},
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "automation_task_fixture.json"),
			FixtureAgent: "automation-runner",
			AgentName:    transportUDSAutomationAgent,
		}},
	})

	clients, err := runtimeHarness.TransportClients()
	if err != nil {
		t.Fatalf("TransportClients() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	trigger, endpoint := seedTransportWebhookTrigger(t, ctx, runtimeHarness)
	delivery, err := runtimeHarness.DeliverGlobalWebhook(
		ctx,
		endpoint,
		"shared-secret",
		[]byte(`{"payload":"deploy","branch":"main"}`),
		"delivery-uds-transport",
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("DeliverGlobalWebhook() error = %v", err)
	}
	if got, want := len(delivery.Runs), 1; got != want {
		t.Fatalf("len(delivery.Runs) = %d, want %d; delivery=%#v", got, want, delivery)
	}

	runID := delivery.Runs[0].ID
	httpRun := waitForHTTPAutomationRun(t, ctx, runtimeHarness, runID)
	udsRun := waitForUDSAutomationRun(t, ctx, runtimeHarness, runID)
	cliRun := waitForCLIAutomationRun(t, ctx, clients.CLI, runID)

	if err := e2etest.ValidateWebhookRunProjection(delivery, httpRun, udsRun, cliRun); err != nil {
		t.Fatalf("ValidateWebhookRunProjection() error = %v", err)
	}
	if got, want := cliRun.TriggerID, trigger.ID; got != want {
		t.Fatalf("cliRun.TriggerID = %q, want %q", got, want)
	}
}

func TestUDSTransportMarketplaceParityMatchesHTTPAndCLI(t *testing.T) {
	t.Parallel()

	catalogServer := newTransportMarketplaceCatalogServer(t)
	startMarketplaceHarness := func(t testing.TB) *e2etest.RuntimeHarness {
		t.Helper()
		return e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
			ConfigSeed: e2etest.ConfigSeedOptions{
				Mutate: func(cfg *compozyconfig.Config) {
					cfg.Marketplace.Catalog.BaseURL = catalogServer.URL
					cfg.Marketplace.Catalog.TTL = time.Hour.String()
					cfg.Marketplace.Catalog.Timeout = time.Second.String()
				},
			},
		})
	}
	runtimeHarness := startMarketplaceHarness(t)

	clients, err := runtimeHarness.TransportClients()
	if err != nil {
		t.Fatalf("TransportClients() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	t.Run("Should return the same grouped search payload over HTTP, UDS, and CLI", func(t *testing.T) {
		path := "/api/marketplace/search?limit=20"
		var httpValue compozycontract.MarketplaceSearchResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, path, nil, &httpValue); err != nil {
			t.Fatalf("HTTPJSON(%s) error = %v", path, err)
		}
		var udsValue compozycontract.MarketplaceSearchResponse
		if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, path, nil, &udsValue); err != nil {
			t.Fatalf("UDSJSON(%s) error = %v", path, err)
		}
		var cliValue compozycontract.MarketplaceSearchResponse
		if err := clients.CLI.RunJSON(
			ctx,
			&cliValue,
			"marketplace",
			"search",
			"--limit",
			"20",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI marketplace search error = %v", err)
		}
		assertTransportMarketplaceParity(t, path, httpValue, udsValue, cliValue)
	})

	t.Run("Should return the same kind browse payload over HTTP, UDS, and CLI", func(t *testing.T) {
		path := "/api/marketplace/extension?limit=20"
		var httpValue compozycontract.MarketplaceKindResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, path, nil, &httpValue); err != nil {
			t.Fatalf("HTTPJSON(%s) error = %v", path, err)
		}
		var udsValue compozycontract.MarketplaceKindResponse
		if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, path, nil, &udsValue); err != nil {
			t.Fatalf("UDSJSON(%s) error = %v", path, err)
		}
		var cliValue compozycontract.MarketplaceKindResponse
		if err := clients.CLI.RunJSON(
			ctx,
			&cliValue,
			"marketplace",
			"search",
			"--kind",
			"extension",
			"--limit",
			"20",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI marketplace kind search error = %v", err)
		}
		assertTransportMarketplaceParity(t, path, httpValue, udsValue, cliValue)
	})

	t.Run("Should return the same entry detail payload over HTTP, UDS, and CLI", func(t *testing.T) {
		path := "/api/marketplace/extension/bridge-github"
		var httpValue compozycontract.MarketplaceEntryResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, path, nil, &httpValue); err != nil {
			t.Fatalf("HTTPJSON(%s) error = %v", path, err)
		}
		var udsValue compozycontract.MarketplaceEntryResponse
		if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, path, nil, &udsValue); err != nil {
			t.Fatalf("UDSJSON(%s) error = %v", path, err)
		}
		var cliValue compozycontract.MarketplaceEntryResponse
		if err := clients.CLI.RunJSON(
			ctx,
			&cliValue,
			"marketplace",
			"info",
			"extension",
			"bridge-github",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI marketplace info error = %v", err)
		}
		assertTransportMarketplaceParity(t, path, httpValue, udsValue, cliValue)
	})

	t.Run("Should install the same catalog MCP semantics over HTTP, UDS, and CLI", func(t *testing.T) {
		path := "/api/settings/mcp-servers/install"
		request := compozycontract.InstallSettingsMCPServerRequest{
			EntryID: "filesystem",
			Name:    "filesystem-parity",
			Scope:   compozycontract.SettingsWorkspaceScopeGlobal,
			Values:  &compozycontract.SettingsMCPCatalogInstallValuesPayload{},
		}
		var httpValue compozycontract.InstallSettingsMCPServerResponse
		var udsValue compozycontract.InstallSettingsMCPServerResponse
		var cliValue compozycontract.InstallSettingsMCPServerResponse
		t.Run("Should install over HTTP from fresh state", func(t *testing.T) {
			harness := startMarketplaceHarness(t)
			installCtx, cancelInstall := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancelInstall()
			if err := harness.HTTPJSON(installCtx, http.MethodPost, path, request, &httpValue); err != nil {
				t.Fatalf("HTTPJSON(%s) error = %v", path, err)
			}
		})
		t.Run("Should install over UDS from fresh state", func(t *testing.T) {
			harness := startMarketplaceHarness(t)
			installCtx, cancelInstall := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancelInstall()
			if err := harness.UDSJSON(installCtx, http.MethodPost, path, request, &udsValue); err != nil {
				t.Fatalf("UDSJSON(%s) error = %v", path, err)
			}
		})
		t.Run("Should install over CLI from fresh state", func(t *testing.T) {
			harness := startMarketplaceHarness(t)
			installCtx, cancelInstall := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancelInstall()
			if err := harness.CLI.RunJSON(
				installCtx,
				&cliValue,
				"mcp",
				"install",
				"filesystem",
				"--name",
				"filesystem-parity",
				"--scope",
				"global",
				"-o",
				"json",
			); err != nil {
				t.Fatalf("CLI mcp install error = %v", err)
			}
		})
		assertTransportMCPInstallParity(t, path, httpValue, udsValue, cliValue)
		if httpValue.MCPServer.CatalogEntry != "filesystem" ||
			httpValue.MCPServer.CatalogVersion != "1.0.0" ||
			httpValue.NextStep != compozycontract.SettingsMCPInstallNextStepNone {
			t.Fatalf("catalog MCP install response = %#v", httpValue)
		}
	})

	t.Run("Should preserve the current extension search envelope across HTTP, UDS, and CLI", func(t *testing.T) {
		path := "/api/extensions/search?q=bridge&limit=20"
		var httpValue compozycontract.ExtensionSearchResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, path, nil, &httpValue); err != nil {
			t.Fatalf("HTTPJSON(%s) error = %v", path, err)
		}
		var udsValue compozycontract.ExtensionSearchResponse
		if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, path, nil, &udsValue); err != nil {
			t.Fatalf("UDSJSON(%s) error = %v", path, err)
		}
		var cliValue compozycontract.ExtensionSearchResponse
		if err := clients.CLI.RunJSON(
			ctx,
			&cliValue,
			"extension",
			"search",
			"bridge",
			"--limit",
			"20",
			"-o",
			"json",
		); err != nil {
			t.Fatalf("CLI extension search error = %v", err)
		}
		assertTransportMarketplaceParity(t, path, httpValue, udsValue, cliValue)
		if len(cliValue.Items) != 1 {
			t.Fatalf("CLI extension search items = %#v, want one curated entry", cliValue)
		}
		item := cliValue.Items[0]
		if item.Slug != "compozy/bridge-github" ||
			item.Name != "GitHub bridge" ||
			item.Description != "Connect GitHub events to Compozy" ||
			item.Version != "1.0.0" ||
			item.Source != "curated" ||
			item.Tier != "official" ||
			item.Integrity != "catalog_digest" {
			t.Fatalf("CLI extension search item = %#v, want current source-union fields", item)
		}
	})

	t.Run("Should refresh the same curated kind over guarded HTTP and unguarded UDS", func(t *testing.T) {
		path := "/api/marketplace/refresh?kind=extension"
		var httpValue compozycontract.MarketplaceRefreshResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodPost, path, nil, &httpValue); err != nil {
			t.Fatalf("HTTPJSON(%s) error = %v", path, err)
		}
		var udsValue compozycontract.MarketplaceRefreshResponse
		if err := runtimeHarness.UDSJSON(ctx, http.MethodPost, path, nil, &udsValue); err != nil {
			t.Fatalf("UDSJSON(%s) error = %v", path, err)
		}
		assertTransportMarketplaceParity(t, path, httpValue, udsValue)
	})
}

func newTransportMarketplaceCatalogServer(t testing.TB) *httptest.Server {
	t.Helper()

	documents := map[string]string{
		"/mcp.json": `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
			`"entry_id":"filesystem","name":"Filesystem","description":"Read approved local files",` +
			`"version":"1.0.0","launch":{"type":"npm","package":"server-filesystem","version":"1.0.0"},` +
			`"default_scope":"global"}]}`,
		"/extensions.json": `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
			`"entry_id":"bridge-github","name":"GitHub bridge","description":"Connect GitHub events to Compozy",` +
			`"version":"1.0.0","install_slug":"compozy/bridge-github",` +
			`"artifact_url":"https://downloads.example.test/bridge-github-v1.0.0.tar.gz",` +
			`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
			`"tier":"official"}]}`,
		"/skills.json": `{"manifest_version":2,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
			`"entry_id":"compozy","name":"Compozy","display_name":"Compozy operator",` +
			`"description":"Operate Compozy through structured surfaces","version":"1.0.0",` +
			`"install_slug":"compozy/compozy","author":"Compozy","tags":["compozy","operations"]}]}`,
	}
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		document, ok := documents[request.URL.Path]
		if !ok {
			http.NotFound(response, request)
			return
		}
		response.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(response, document); err != nil {
			t.Errorf("write marketplace catalog response error = %v", err)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func assertTransportMarketplaceParity(t testing.TB, path string, values ...any) {
	t.Helper()
	if len(values) < 2 {
		t.Fatalf("%s transport values = %d, want at least 2", path, len(values))
	}
	expected := values[0]
	for index, value := range values[1:] {
		if reflect.DeepEqual(expected, value) {
			continue
		}
		expectedJSON, expectedErr := json.Marshal(expected)
		valueJSON, valueErr := json.Marshal(value)
		if expectedErr != nil || valueErr != nil {
			t.Fatalf(
				"%s parity mismatch at transport %d; expected marshal error = %v, value marshal error = %v",
				path,
				index+1,
				expectedErr,
				valueErr,
			)
		}
		t.Fatalf(
			"%s transport 0 payload = %s, want parity with transport %d payload %s",
			path,
			expectedJSON,
			index+1,
			valueJSON,
		)
	}
}

func assertTransportMCPInstallParity(
	t testing.TB,
	path string,
	values ...compozycontract.InstallSettingsMCPServerResponse,
) {
	t.Helper()

	normalized := make([]any, 0, len(values))
	for index, value := range values {
		if !strings.HasPrefix(value.Apply.ApplyRecordID, "cfgapp-") {
			t.Fatalf("%s transport %d apply_record_id = %q, want cfgapp-*", path, index, value.Apply.ApplyRecordID)
		}
		if len(value.Apply.ActiveConfigHash) != len("sha256:")+64 ||
			!strings.HasPrefix(value.Apply.ActiveConfigHash, "sha256:") {
			t.Fatalf(
				"%s transport %d active_config_hash = %q, want sha256 digest",
				path,
				index,
				value.Apply.ActiveConfigHash,
			)
		}
		value.Apply.ApplyRecordID = ""
		value.Apply.ActiveConfigHash = ""
		normalized = append(normalized, value)
	}
	assertTransportMarketplaceParity(t, path, normalized...)
}

func TestUDSTransportPromptFailureProjectionUsesSharedRuntimeHarness(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "driver_fault_fixture.json"),
			FixtureAgent: "faulty",
			AgentName:    transportUDSFaultyAgent,
		}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session, err := runtimeHarness.CreateSession(ctx, compozycontract.CreateSessionRequest{
		AgentName:     transportUDSFaultyAgent,
		WorkspacePath: runtimeHarness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	session = waitForTransportSessionActive(t, ctx, runtimeHarness, session)

	stream, err := runtimeHarness.PromptSession(ctx, session.ID, "trigger crash mid-stream")
	if err != nil {
		t.Fatalf("PromptSession() error = %v", err)
	}
	if !udsSSEContainsEvent(stream, "error") {
		t.Fatalf("UDS prompt stream = %#v, want error event", stream)
	}

	transcript, err := runtimeHarness.SessionTranscript(ctx, session.ID)
	if err != nil {
		t.Fatalf("SessionTranscript() error = %v", err)
	}
	messages := transcriptpkg.MessagesFromEntries(transcript.Entries)
	if !strings.Contains(joinTransportTranscript(messages), "partial before crash") {
		t.Fatalf("transcript = %#v, want partial assistant output", messages)
	}

	eventsResp, err := runtimeHarness.SessionEvents(ctx, session.ID)
	if err != nil {
		t.Fatalf("SessionEvents() error = %v", err)
	}
	if !udsSessionEventsContainType(eventsResp.Events, "error") {
		t.Fatalf("UDS session events = %#v, want error projection", eventsResp.Events)
	}
}

// Invariant: terminal process-exited sessions reject every attachment path before ACP session/load.
// Owning layer: HTTP/UDS session transport parity.
func TestUDSTransportTerminalProcessFailureRejectsAttachAndPromptBeforeACPResume(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "driver_fault_fixture.json"),
			FixtureAgent: "faulty",
			AgentName:    transportUDSFaultyAgent,
		}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	created, err := runtimeHarness.CreateSession(ctx, compozycontract.CreateSessionRequest{
		AgentName:     transportUDSFaultyAgent,
		WorkspacePath: runtimeHarness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	created = waitForTransportSessionActive(t, ctx, runtimeHarness, created)

	stream, err := runtimeHarness.PromptSession(ctx, created.ID, "trigger crash mid-stream")
	if err != nil {
		t.Fatalf("PromptSession() error = %v", err)
	}
	if !udsSSEContainsEvent(stream, "error") {
		t.Fatalf("UDS prompt stream = %#v, want error event", stream)
	}
	waitForTransportDeadSession(t, ctx, runtimeHarness, created.ID)

	clients, err := runtimeHarness.TransportClients()
	if err != nil {
		t.Fatalf("TransportClients() error = %v", err)
	}
	path := transportHarnessSessionPath(t, runtimeHarness, created.ID, "/attach")
	assertTransportDeadSessionRejection(t, clients.HTTPClient, runtimeHarness.HTTPURL(path), nil)
	assertTransportDeadSessionRejection(t, clients.UDSClient, runtimeHarness.UDSURL(path), nil)

	promptBody, err := json.Marshal(compozycontract.SendPromptRequest{
		Message:        "retry a dead session",
		MessageID:      "message-dead-session-retry",
		IdempotencyKey: "idempotency-dead-session-retry",
	})
	if err != nil {
		t.Fatalf("json.Marshal(dead session prompt) error = %v", err)
	}
	promptPath := transportHarnessSessionPath(t, runtimeHarness, created.ID, "/prompt")
	assertTransportDeadSessionRejection(t, clients.HTTPClient, runtimeHarness.HTTPURL(promptPath), promptBody)
	assertTransportDeadSessionRejection(t, clients.UDSClient, runtimeHarness.UDSURL(promptPath), promptBody)

	registration, ok := runtimeHarness.MockAgentRegistration(transportUDSFaultyAgent)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) not found", transportUDSFaultyAgent)
	}
	diagnostics, err := acpmock.ReadDiagnostics(registration.DiagnosticsPath)
	if err != nil {
		t.Fatalf("ReadDiagnostics(%q) error = %v", registration.DiagnosticsPath, err)
	}
	for _, diagnostic := range acpmock.ProtocolDiagnostics(
		acpmock.DiagnosticsForCompozySession(diagnostics, created.ID),
	) {
		if diagnostic.ProtocolMethod == "session/load" {
			t.Fatalf("terminal session diagnostics = %#v, want no session/load", diagnostics)
		}
	}
}

func TestUDSTransportObserveHarnessLifecycleParityMatchesHTTP(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "multi_agent_fixture.json"),
			FixtureAgent: "alpha",
			AgentName:    transportUDSObserveAgent,
		}},
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session, err := runtimeHarness.CreateSession(ctx, compozycontract.CreateSessionRequest{
		AgentName:     transportUDSObserveAgent,
		WorkspacePath: runtimeHarness.WorkspaceRoot,
	})
	if err != nil {
		t.Fatalf("CreateSession() error = %v", err)
	}
	session = waitForTransportSessionActive(t, ctx, runtimeHarness, session)

	stream, err := runtimeHarness.PromptSessionHTTP(ctx, session.ID, "hello alpha")
	if err != nil {
		t.Fatalf("PromptSessionHTTP() error = %v", err)
	}
	if len(stream) == 0 {
		t.Fatal("prompt stream = empty, want streamed events")
	}

	transcriptResp, err := runtimeHarness.SessionTranscript(ctx, session.ID)
	if err != nil {
		t.Fatalf("SessionTranscript() error = %v", err)
	}
	messages := transcriptpkg.MessagesFromEntries(transcriptResp.Entries)
	if !strings.Contains(joinTransportTranscript(messages), "alpha says hi") {
		t.Fatalf("transcript = %#v, want assistant reply", messages)
	}

	httpHarnessEvents := waitForTransportListLogs(
		t,
		ctx,
		"waitForHTTPListLogs",
		wantTransportObserveHarnessTypes(),
		func(fetchCtx context.Context) ([]compozycontract.LogEventPayload, error) {
			var response compozycontract.LogsListResponse
			err := runtimeHarness.HTTPJSON(
				fetchCtx,
				http.MethodGet,
				transportHarnessLogsPath(t, runtimeHarness, session.ID),
				nil,
				&response,
			)
			return response.Events, err
		},
	)
	udsHarnessEvents := waitForTransportListLogs(
		t,
		ctx,
		"waitForUDSListLogs",
		wantTransportObserveHarnessTypes(),
		func(fetchCtx context.Context) ([]compozycontract.LogEventPayload, error) {
			var response compozycontract.LogsListResponse
			err := runtimeHarness.UDSJSON(
				fetchCtx,
				http.MethodGet,
				transportHarnessLogsPath(t, runtimeHarness, session.ID),
				nil,
				&response,
			)
			return response.Events, err
		},
	)

	if !reflect.DeepEqual(httpHarnessEvents, udsHarnessEvents) {
		t.Fatalf("HTTP harness events = %#v, want UDS parity %#v", httpHarnessEvents, udsHarnessEvents)
	}
	if got, want := logEventTypes(httpHarnessEvents), wantTransportObserveHarnessTypes(); !slices.Equal(got, want) {
		t.Fatalf("harness event types = %#v, want %#v", got, want)
	}
	if !strings.Contains(httpHarnessEvents[0].Summary, "surface=startup") {
		t.Fatalf("startup summary = %q, want startup surface", httpHarnessEvents[0].Summary)
	}
	if !strings.Contains(httpHarnessEvents[1].Summary, "selected=") {
		t.Fatalf("section summary = %q, want selected sections", httpHarnessEvents[1].Summary)
	}
	if !strings.Contains(httpHarnessEvents[2].Summary, "surface=turn") {
		t.Fatalf("turn summary = %q, want turn surface", httpHarnessEvents[2].Summary)
	}
	for _, augmenter := range []string{"workspace_knowledge", "skills", "situation", "durable_memory"} {
		if !slices.ContainsFunc(httpHarnessEvents[3:], func(event compozycontract.LogEventPayload) bool {
			return strings.Contains(event.Summary, "augmenter="+augmenter+" ")
		}) {
			t.Fatalf("harness events = %#v, want %q augmenter metadata", httpHarnessEvents, augmenter)
		}
	}
}

func transportHarnessSessionPath(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	suffix string,
) string {
	t.Helper()

	workspaceID := strings.TrimSpace(harness.WorkspaceID)
	if workspaceID == "" {
		t.Fatal("runtime harness WorkspaceID = empty")
	}
	return "/api/workspaces/" + url.PathEscape(workspaceID) +
		"/sessions/" + url.PathEscape(sessionID) + suffix
}

func transportHarnessLogsPath(
	t testing.TB,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) string {
	t.Helper()

	workspaceID := strings.TrimSpace(harness.WorkspaceID)
	if workspaceID == "" {
		t.Fatal("runtime harness WorkspaceID = empty")
	}
	query := url.Values{}
	query.Set("workspace_id", workspaceID)
	query.Set("session_id", strings.TrimSpace(sessionID))
	query.Set("limit", "20")
	return "/api/logs?" + query.Encode()
}

func TestUDSTransportSettingsReadParityMatchesHTTP(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	workspaceID := runtimeHarness.WorkspaceID

	testCases := []struct {
		name   string
		path   string
		decode func() any
	}{
		{
			name:   "general section",
			path:   "/api/settings/general",
			decode: func() any { return &compozycontract.SettingsGeneralResponse{} },
		},
		{
			name:   "providers collection",
			path:   "/api/settings/providers",
			decode: func() any { return &compozycontract.SettingsProvidersResponse{} },
		},
		{
			name: "workspace mcp servers collection",
			path: "/api/settings/mcp-servers?scope=workspace&workspace_id=" + url.QueryEscape(workspaceID),
			decode: func() any {
				return &compozycontract.SettingsMCPServersResponse{}
			},
		},
		{
			name:   "hooks and extensions section",
			path:   "/api/settings/hooks-extensions",
			decode: func() any { return &compozycontract.SettingsHooksExtensionsResponse{} },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sampleStarted := time.Now()
			httpValue := tc.decode()
			if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, tc.path, nil, httpValue); err != nil {
				t.Fatalf("HTTPJSON(%s) error = %v", tc.path, err)
			}

			udsValue := tc.decode()
			if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, tc.path, nil, udsValue); err != nil {
				t.Fatalf("UDSJSON(%s) error = %v", tc.path, err)
			}
			assertAndNormalizeSettingsReadParityVolatileFields(
				t,
				httpValue,
				udsValue,
				time.Since(sampleStarted),
			)

			if !reflect.DeepEqual(httpValue, udsValue) {
				httpPayload, err := json.Marshal(httpValue)
				if err != nil {
					t.Fatalf("json.Marshal(%s HTTP payload) error = %v", tc.path, err)
				}
				udsPayload, err := json.Marshal(udsValue)
				if err != nil {
					t.Fatalf("json.Marshal(%s UDS payload) error = %v", tc.path, err)
				}
				t.Fatalf("%s HTTP payload = %s, want UDS parity %s", tc.path, httpPayload, udsPayload)
			}
		})
	}
}

func assertAndNormalizeSettingsReadParityVolatileFields(
	t *testing.T,
	httpValue,
	udsValue any,
	sampleDuration time.Duration,
) {
	t.Helper()

	httpGeneral, httpOK := httpValue.(*compozycontract.SettingsGeneralResponse)
	udsGeneral, udsOK := udsValue.(*compozycontract.SettingsGeneralResponse)
	if !httpOK || !udsOK {
		return
	}

	// The transports are sampled sequentially, so runtime uptime can cross a second boundary.
	maxDriftSeconds := int64(sampleDuration/time.Second) + 1
	uptimeDrift := udsGeneral.Runtime.UptimeSeconds - httpGeneral.Runtime.UptimeSeconds
	if uptimeDrift < 0 || uptimeDrift > maxDriftSeconds {
		t.Fatalf(
			"runtime uptime drift = %ds, want UDS after HTTP within %ds sampling window",
			uptimeDrift,
			maxDriftSeconds,
		)
	}
	httpGeneral.Runtime.UptimeSeconds = 0
	udsGeneral.Runtime.UptimeSeconds = 0
}

func TestUDSTransportSettingsDependencyExtensionParityMatchesHTTP(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{})

	clients, err := runtimeHarness.TransportClients()
	if err != nil {
		t.Fatalf("TransportClients() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	httpListResp := mustUnixRequest(
		t,
		clients.HTTPClient,
		http.MethodGet,
		transportSettingsHTTPURL(runtimeHarness, "/api/extensions"),
		nil,
		nil,
	)
	if httpListResp.StatusCode != http.StatusOK {
		body := readAndCloseHTTPBody(t, httpListResp)
		t.Fatalf(
			"HTTP extension list status = %d, want %d; body=%s",
			httpListResp.StatusCode,
			http.StatusOK,
			string(body),
		)
	}
	var httpList compozycontract.ExtensionsResponse
	decodeHTTPJSON(t, httpListResp, &httpList)

	var udsList compozycontract.ExtensionsResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, "/api/extensions", nil, &udsList); err != nil {
		t.Fatalf("UDSJSON(/api/extensions) error = %v", err)
	}
	if !reflect.DeepEqual(httpList.Extensions, udsList.Extensions) {
		t.Fatalf("HTTP extensions = %#v, want UDS parity %#v", httpList.Extensions, udsList.Extensions)
	}
	if len(httpList.Extensions) == 0 {
		return
	}

	name := httpList.Extensions[0].Name
	httpStatusResp := mustUnixRequest(
		t,
		clients.HTTPClient,
		http.MethodGet,
		transportSettingsHTTPURL(runtimeHarness, "/api/extensions/"+url.PathEscape(name)),
		nil,
		nil,
	)
	if httpStatusResp.StatusCode != http.StatusOK {
		body := readAndCloseHTTPBody(t, httpStatusResp)
		t.Fatalf("HTTP extension status = %d, want %d; body=%s", httpStatusResp.StatusCode, http.StatusOK, string(body))
	}
	var httpStatus compozycontract.ExtensionResponse
	decodeHTTPJSON(t, httpStatusResp, &httpStatus)

	var udsStatus compozycontract.ExtensionResponse
	if err := runtimeHarness.UDSJSON(
		ctx,
		http.MethodGet,
		"/api/extensions/"+url.PathEscape(name),
		nil,
		&udsStatus,
	); err != nil {
		t.Fatalf("UDSJSON(/api/extensions/%s) error = %v", name, err)
	}
	if !reflect.DeepEqual(httpStatus.Extension, udsStatus.Extension) {
		t.Fatalf("HTTP extension = %#v, want UDS parity %#v", httpStatus.Extension, udsStatus.Extension)
	}
}

func TestUDSTransportSettingsMutationsRemainPrivilegedWhenHTTPIsNonLoopback(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		ConfigSeed: e2etest.ConfigSeedOptions{
			Host: "0.0.0.0",
		},
	})

	clients, err := runtimeHarness.TransportClients()
	if err != nil {
		t.Fatalf("TransportClients() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	workspaceID := runtimeHarness.WorkspaceID
	putPath := "/api/settings/mcp-servers/server-a?scope=workspace&workspace_id=" +
		url.QueryEscape(workspaceID) + "&target=sidecar"
	putBody, err := json.Marshal(compozycontract.PutSettingsMCPServerRequest{
		Server: compozycontract.SettingsMCPServerPayload{
			Name:    "server-a",
			Command: "mcpd",
			Args:    []string{"serve"},
		},
	})
	if err != nil {
		t.Fatalf("json.Marshal(put settings body) error = %v", err)
	}

	httpPutResp := mustUnixRequest(
		t,
		clients.HTTPClient,
		http.MethodPut,
		transportSettingsHTTPURL(runtimeHarness, putPath),
		putBody,
		nil,
	)
	if httpPutResp.StatusCode != http.StatusForbidden {
		body := readAndCloseHTTPBody(t, httpPutResp)
		t.Fatalf(
			"HTTP PUT settings status = %d, want %d; body=%s",
			httpPutResp.StatusCode,
			http.StatusForbidden,
			string(body),
		)
	}
	var forbidden compozycontract.ErrorPayload
	decodeHTTPJSON(t, httpPutResp, &forbidden)

	var udsMutation compozycontract.SettingsGlobalWorkspaceCollectionMutationResult
	if err := runtimeHarness.UDSJSON(
		ctx,
		http.MethodPut,
		putPath,
		compozycontract.PutSettingsMCPServerRequest{
			Server: compozycontract.SettingsMCPServerPayload{
				Name:    "server-a",
				Command: "mcpd",
				Args:    []string{"serve"},
			},
		},
		&udsMutation,
	); err != nil {
		t.Fatalf("UDSJSON(PUT %s) error = %v", putPath, err)
	}
	if udsMutation.Scope != compozycontract.SettingsWorkspaceScopeWorkspace || udsMutation.WorkspaceID != workspaceID {
		t.Fatalf("UDS mutation = %#v, want workspace-scoped result", udsMutation)
	}

	listPath := "/api/settings/mcp-servers?scope=workspace&workspace_id=" + url.QueryEscape(workspaceID)
	httpListResp := mustUnixRequest(
		t,
		clients.HTTPClient,
		http.MethodGet,
		transportSettingsHTTPURL(runtimeHarness, listPath),
		nil,
		nil,
	)
	if httpListResp.StatusCode != http.StatusForbidden {
		body := readAndCloseHTTPBody(t, httpListResp)
		t.Fatalf(
			"HTTP GET settings list status = %d, want %d; body=%s",
			httpListResp.StatusCode,
			http.StatusForbidden,
			string(body),
		)
	}
	var listForbidden compozycontract.ErrorPayload
	decodeHTTPJSON(t, httpListResp, &listForbidden)

	var udsList compozycontract.SettingsMCPServersResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, listPath, nil, &udsList); err != nil {
		t.Fatalf("UDSJSON(GET %s) error = %v", listPath, err)
	}
	if !settingsMCPServerPresent(udsList.MCPServers, "server-a") {
		t.Fatalf("UDS mcp list = %#v, want server-a after UDS mutation", udsList)
	}

	deletePath := "/api/settings/mcp-servers/server-a?scope=workspace&workspace_id=" +
		url.QueryEscape(workspaceID) + "&target=sidecar"
	var deleteResult compozycontract.SettingsGlobalWorkspaceCollectionMutationResult
	if err := runtimeHarness.UDSJSON(ctx, http.MethodDelete, deletePath, nil, &deleteResult); err != nil {
		t.Fatalf("UDSJSON(DELETE %s) error = %v", deletePath, err)
	}
	if deleteResult.Scope != compozycontract.SettingsWorkspaceScopeWorkspace ||
		deleteResult.WorkspaceID != workspaceID {
		t.Fatalf("deleteResult = %#v, want workspace-scoped delete result", deleteResult)
	}

	httpListResp = mustUnixRequest(
		t,
		clients.HTTPClient,
		http.MethodGet,
		transportSettingsHTTPURL(runtimeHarness, listPath),
		nil,
		nil,
	)
	if httpListResp.StatusCode != http.StatusForbidden {
		body := readAndCloseHTTPBody(t, httpListResp)
		t.Fatalf(
			"HTTP GET settings list after delete status = %d, want %d; body=%s",
			httpListResp.StatusCode,
			http.StatusForbidden,
			string(body),
		)
	}
	var listAfterDeleteForbidden compozycontract.ErrorPayload
	decodeHTTPJSON(t, httpListResp, &listAfterDeleteForbidden)

	var udsListAfterDelete compozycontract.SettingsMCPServersResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, listPath, nil, &udsListAfterDelete); err != nil {
		t.Fatalf("UDSJSON(GET %s after delete) error = %v", listPath, err)
	}
	if settingsMCPServerPresent(udsListAfterDelete.MCPServers, "server-a") {
		t.Fatalf("UDS mcp list after delete = %#v, want server-a removed", udsListAfterDelete)
	}
}

func TestUDSTransportTaskSurfaceMatchesHTTPAndDocumentedSpecOperations(t *testing.T) {
	t.Parallel()

	homePaths := newTestHomePaths(t)
	udsEngine := newTestRouter(t, newTestHandlers(t, stubSessionManager{}, stubObserver{}, homePaths))

	udsRoutes := taskRoutesFromEngine(udsEngine.Routes())
	httpRoutes := documentedTaskRoutesForTransport(apispec.TransportHTTP)
	if !slices.Equal(udsRoutes, httpRoutes) {
		t.Fatalf("UDS task routes = %v, want documented HTTP task routes %v", udsRoutes, httpRoutes)
	}

	specRoutes := documentedTaskRoutesForTransport(apispec.TransportUDS)
	if !slices.Equal(udsRoutes, specRoutes) {
		t.Fatalf("UDS task routes = %v, want documented task routes %v", udsRoutes, specRoutes)
	}
}
func waitForUDSAutomationRun(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
) compozycontract.RunPayload {
	t.Helper()

	return waitForTransportAutomationRun(
		t,
		ctx,
		runID,
		"waitForUDSAutomationRun",
		func(fetchCtx context.Context) (compozycontract.RunPayload, error) {
			return harness.GetAutomationRun(fetchCtx, runID)
		},
	)
}

func transportSettingsHTTPURL(harness *e2etest.RuntimeHarness, path string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", harness.Config.HTTP.Port, path)
}

func settingsMCPServerPresent(items []compozycontract.SettingsMCPServerItemPayload, name string) bool {
	for _, item := range items {
		if item.Name == name {
			return true
		}
	}
	return false
}

func waitForCLIAutomationRun(
	t testing.TB,
	ctx context.Context,
	client *e2etest.CLIClient,
	runID string,
) compozycontract.RunPayload {
	t.Helper()

	return waitForTransportAutomationRun(
		t,
		ctx,
		runID,
		"waitForCLIAutomationRun",
		func(fetchCtx context.Context) (compozycontract.RunPayload, error) {
			var run compozycontract.RunPayload
			err := client.RunJSON(fetchCtx, &run, "automation", "runs", "get", runID, "-o", "json")
			return run, err
		},
	)
}

func seedTransportWebhookTrigger(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) (compozycontract.TriggerPayload, string) {
	t.Helper()

	state, err := harness.SeedAutomationFixtures(ctx, e2etest.AutomationFixtureSeed{
		Triggers: []compozycontract.CreateTriggerRequest{{
			Scope:              automationpkg.AutomationScopeGlobal,
			Name:               "deploy-review",
			AgentName:          transportUDSAutomationAgent,
			Prompt:             `Review payload {{ index .Data "payload" }} for {{ index .Data "branch" }}`,
			Event:              "webhook",
			EndpointSlug:       "deploy-review",
			WebhookSecretValue: "shared-secret",
		}},
	})
	if err != nil {
		t.Fatalf("SeedAutomationFixtures() error = %v", err)
	}
	if got, want := len(state.Triggers), 1; got != want {
		t.Fatalf("len(state.Triggers) = %d, want %d", got, want)
	}

	endpoint, err := automationpkg.FormatWebhookEndpoint(
		state.Triggers[0].EndpointSlug,
		state.Triggers[0].WebhookID,
	)
	if err != nil {
		t.Fatalf("FormatWebhookEndpoint() error = %v", err)
	}
	return state.Triggers[0], endpoint
}

func taskRoutesFromEngine(routes gin.RoutesInfo) []string {
	filtered := make([]string, 0)
	for _, route := range routes {
		if isDocumentedTaskRoute(route.Path) {
			filtered = append(filtered, route.Method+" "+route.Path)
		}
	}
	sort.Strings(filtered)
	return filtered
}

func documentedTaskRoutesForTransport(transport apispec.Transport) []string {
	routes := make([]string, 0)
	for _, operation := range apispec.Operations() {
		if !slices.Contains(operation.Transports, transport) {
			continue
		}
		if !isDocumentedTaskRoute(operation.Path) {
			continue
		}
		routes = append(routes, operation.Method+" "+normalizeSpecRoutePath(operation.Path))
	}
	sort.Strings(routes)
	return routes
}

func isDocumentedTaskRoute(path string) bool {
	return strings.HasPrefix(path, "/api/tasks") ||
		strings.HasPrefix(path, "/api/task-runs") ||
		strings.HasPrefix(path, "/api/observe/tasks")
}

func normalizeSpecRoutePath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && len(part) > 2 {
			parts[i] = ":" + part[1:len(part)-1]
		}
	}
	return strings.Join(parts, "/")
}

func waitForHTTPAutomationRun(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	runID string,
) compozycontract.RunPayload {
	t.Helper()

	return waitForTransportAutomationRun(
		t,
		ctx,
		runID,
		"waitForHTTPAutomationRun",
		func(fetchCtx context.Context) (compozycontract.RunPayload, error) {
			var response compozycontract.RunResponse
			err := harness.HTTPJSON(
				fetchCtx,
				http.MethodGet,
				"/api/automation/runs/"+url.PathEscape(runID),
				nil,
				&response,
			)
			return response.Run, err
		},
	)
}

func waitForTransportAutomationRun(
	t testing.TB,
	ctx context.Context,
	runID string,
	label string,
	fetch func(context.Context) (compozycontract.RunPayload, error),
) compozycontract.RunPayload {
	t.Helper()

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	var (
		lastErr error
		lastRun compozycontract.RunPayload
	)
	for {
		run, err := fetch(waitCtx)
		if err == nil && run.ID == runID {
			return run
		}
		lastErr = err
		lastRun = run
		select {
		case <-waitCtx.Done():
			t.Fatalf(
				"%s(%q) timed out: %v; last error=%v last run=%#v",
				label,
				runID,
				waitCtx.Err(),
				lastErr,
				lastRun,
			)
		case <-ticker.C:
		}
	}
}

func waitForTransportSessionTitle(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
	wantTitle string,
) (compozycontract.SessionsResponse, compozycontract.SessionsResponse) {
	t.Helper()

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	path := "/api/sessions?workspace=" + url.QueryEscape(harness.WorkspaceID)

	var (
		httpCatalog compozycontract.SessionsResponse
		udsCatalog  compozycontract.SessionsResponse
		httpErr     error
		udsErr      error
	)
	for {
		httpCatalog = compozycontract.SessionsResponse{}
		httpErr = harness.HTTPJSON(waitCtx, http.MethodGet, path, nil, &httpCatalog)
		udsCatalog = compozycontract.SessionsResponse{}
		udsErr = harness.UDSJSON(waitCtx, http.MethodGet, path, nil, &udsCatalog)
		if httpErr == nil && udsErr == nil &&
			transportCatalogHasSingleTitle(httpCatalog, sessionID, wantTitle) &&
			transportCatalogHasSingleTitle(udsCatalog, sessionID, wantTitle) {
			return httpCatalog, udsCatalog
		}

		select {
		case <-waitCtx.Done():
			t.Fatalf(
				"automatic title %q for session %q timed out: %v; HTTP error=%v catalog=%#v; UDS error=%v catalog=%#v",
				wantTitle,
				sessionID,
				waitCtx.Err(),
				httpErr,
				httpCatalog,
				udsErr,
				udsCatalog,
			)
		case <-ticker.C:
		}
	}
}

func waitForTransportSessionActive(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	accepted compozycontract.SessionPayload,
) compozycontract.SessionPayload {
	t.Helper()
	active, err := harness.WaitForSessionActive(ctx, accepted.ID)
	if err != nil {
		t.Fatalf("WaitForSessionActive(%q) error = %v; accepted=%#v", accepted.ID, err, accepted)
	}
	return active
}

func waitForTransportDeadSession(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
	sessionID string,
) {
	t.Helper()

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	path := transportHarnessSessionPath(t, harness, sessionID, "?include_health=true")
	var last compozycontract.SessionPayload
	var lastErr error
	for {
		var response compozycontract.SessionResponse
		if err := harness.UDSJSON(waitCtx, http.MethodGet, path, nil, &response); err == nil {
			last = response.Session
			if last.State == session.StateStopped &&
				last.Failure != nil && last.Failure.Kind == store.FailureProcess &&
				last.Health != nil && last.Health.Health == compozycontract.SessionHealthDead &&
				!last.Health.Attachable && !last.Health.EligibleForWake {
				return
			}
		} else {
			lastErr = err
		}

		select {
		case <-waitCtx.Done():
			t.Fatalf(
				"terminal session %q did not become dead and non-attachable: %v; last=%#v",
				sessionID,
				lastErr,
				last,
			)
		case <-ticker.C:
		}
	}
}

func assertTransportDeadSessionRejection(
	t *testing.T,
	client *http.Client,
	targetURL string,
	body []byte,
) {
	t.Helper()

	response := mustUnixRequest(t, client, http.MethodPost, targetURL, body, nil)
	payload := readAndCloseHTTPBody(t, response)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("POST %s status = %d, want %d; body=%s", targetURL, response.StatusCode, http.StatusConflict, payload)
	}
	var failure compozycontract.ErrorPayload
	if err := json.Unmarshal(payload, &failure); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v; body=%s", targetURL, err, payload)
	}
	if !strings.Contains(failure.Error, "not attachable") {
		t.Fatalf("POST %s error = %q, want attachability context", targetURL, failure.Error)
	}
}

func transportCatalogHasSingleTitle(
	catalog compozycontract.SessionsResponse,
	sessionID string,
	wantTitle string,
) bool {
	return len(catalog.Sessions) == 1 &&
		catalog.Sessions[0].ID == sessionID &&
		catalog.Sessions[0].Name == wantTitle
}

func assertTransportCatalogTitle(
	t testing.TB,
	transport string,
	catalog compozycontract.SessionsResponse,
	sessionID string,
	wantTitle string,
) {
	t.Helper()

	if !transportCatalogHasSingleTitle(catalog, sessionID, wantTitle) {
		t.Fatalf(
			"%s session catalog = %#v, want exactly one session id=%q name=%q",
			transport,
			catalog.Sessions,
			sessionID,
			wantTitle,
		)
	}
}

func waitForTransportListLogs(
	t testing.TB,
	ctx context.Context,
	label string,
	wantTypes []string,
	fetch func(context.Context) ([]compozycontract.LogEventPayload, error),
) []compozycontract.LogEventPayload {
	t.Helper()

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	var (
		lastErr    error
		lastEvents []compozycontract.LogEventPayload
	)
	for {
		events, err := fetch(waitCtx)
		if err == nil {
			harnessEvents := filterHarnessListLogs(events)
			if slices.Equal(logEventTypes(harnessEvents), wantTypes) {
				return harnessEvents
			}
			lastEvents = harnessEvents
		} else {
			lastErr = err
		}

		select {
		case <-waitCtx.Done():
			t.Fatalf(
				"%s timed out: %v; last error=%v last harness events=%#v",
				label,
				waitCtx.Err(),
				lastErr,
				lastEvents,
			)
		case <-ticker.C:
		}
	}
}

func wantTransportObserveHarnessTypes() []string {
	return []string{
		"harness.context_resolved",
		"harness.section_selected",
		"harness.context_resolved",
		"harness.augmenter_applied",
		"harness.augmenter_applied",
		"harness.augmenter_applied",
		"harness.augmenter_applied",
	}
}

func udsSSEContainsEvent(records []e2etest.SSEEvent, want string) bool {
	for _, record := range records {
		if record.Event == want {
			return true
		}
	}
	return false
}

func udsSessionEventsContainType(events []compozycontract.SessionEventPayload, want string) bool {
	for _, event := range events {
		var payload struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(event.Content, &payload); err == nil && payload.Type == want {
			return true
		}
	}
	return false
}

func filterHarnessListLogs(events []compozycontract.LogEventPayload) []compozycontract.LogEventPayload {
	filtered := make([]compozycontract.LogEventPayload, 0, len(events))
	for _, event := range events {
		if strings.HasPrefix(event.Type, "harness.") {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func logEventTypes(events []compozycontract.LogEventPayload) []string {
	types := make([]string, 0, len(events))
	for _, event := range events {
		types = append(types, event.Type)
	}
	return types
}

func joinTransportTranscript(messages []transcriptpkg.UIMessage) string {
	return transcriptpkg.JoinUIMessageText(messages)
}

func waitForTransportTranscriptText(
	t testing.TB,
	ctx context.Context,
	runtimeHarness *e2etest.RuntimeHarness,
	sessionID string,
	want ...string,
) {
	t.Helper()

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	var (
		lastText string
		lastErr  error
	)
	for {
		page, err := runtimeHarness.SessionTranscript(waitCtx, sessionID)
		if err == nil {
			lastText = joinTransportTranscript(transcriptpkg.MessagesFromEntries(page.Entries))
			matched := true
			for _, expected := range want {
				if !strings.Contains(lastText, expected) {
					matched = false
					break
				}
			}
			if matched {
				return
			}
		} else {
			lastErr = err
		}

		select {
		case <-waitCtx.Done():
			t.Fatalf(
				"timed out waiting for session %q transcript text %q: %v; last error=%v transcript=%q",
				sessionID,
				want,
				waitCtx.Err(),
				lastErr,
				lastText,
			)
		case <-ticker.C:
		}
	}
}

func transportMockFixturePath(t testing.TB, name string) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testutil", "acpmock", "testdata", name)
}

func writeTransportProviderOverrideConfig(
	t testing.TB,
	workspaceRoot string,
	providerName string,
	command string,
	includeProvider bool,
) {
	t.Helper()

	configDir := filepath.Join(workspaceRoot, compozyconfig.DirName)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q) error = %v", configDir, err)
	}

	var builder strings.Builder
	if includeProvider {
		builder.WriteString("[providers." + providerName + "]\n")
		builder.WriteString(`command = "`)
		builder.WriteString(escapeTransportConfigString(command))
		builder.WriteString("\"\n")
		builder.WriteString("[providers." + providerName + ".models]\n")
		builder.WriteString(`default = "transport-override-model"` + "\n")
		builder.WriteString("[[providers." + providerName + ".credential_slots]]\n")
		builder.WriteString(`name = "api_key"` + "\n")
		builder.WriteString(`target_env = "TRANSPORT_OVERRIDE_API_KEY"` + "\n")
		builder.WriteString(`secret_ref = "env:TRANSPORT_OVERRIDE_API_KEY"` + "\n")
		builder.WriteString(`kind = "api_key"` + "\n")
		builder.WriteString(`required = false` + "\n")
	}

	configPath := filepath.Join(configDir, compozyconfig.ConfigName)
	if err := os.WriteFile(configPath, []byte(builder.String()), 0o600); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", configPath, err)
	}
}

func escapeTransportConfigString(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return replacer.Replace(strings.TrimSpace(value))
}

func transportDoctorItem(
	t testing.TB,
	payload compozycontract.DoctorPayload,
	id string,
) compozycontract.DiagnosticItem {
	t.Helper()
	for _, item := range payload.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("doctor items = %#v, want %q", payload.Items, id)
	return compozycontract.DiagnosticItem{}
}

func transportPositiveNumber(value any) bool {
	number, ok := value.(float64)
	return ok && number > 0
}
