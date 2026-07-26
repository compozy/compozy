//go:build integration

package udsapi

import (
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

	aghcontract "github.com/compozy/agh/internal/api/contract"
	apispec "github.com/compozy/agh/internal/api/spec"
	automationpkg "github.com/compozy/agh/internal/automation"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/compozy/agh/internal/testutil/acpmock"
	e2etest "github.com/compozy/agh/internal/testutil/e2e"
	transcriptpkg "github.com/compozy/agh/internal/transcript"
	"github.com/compozy/agh/internal/windowmanager"
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

func TestUDSTransportDaemonDrainMatchesHTTPAndCLI(t *testing.T) {
	t.Parallel()
	acpmock.RequireDriver(t)

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var httpDraining aghcontract.DrainStatusResponse
	if err := runtimeHarness.HTTPJSON(ctx, http.MethodPost, "/api/drain", nil, &httpDraining); err != nil {
		t.Fatalf("HTTP drain error = %v", err)
	}
	var udsDraining aghcontract.DrainStatusResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodPost, "/api/drain", nil, &udsDraining); err != nil {
		t.Fatalf("UDS drain error = %v", err)
	}
	var cliDraining aghcontract.DrainStatusResponse
	if err := runtimeHarness.CLI.RunJSON(ctx, &cliDraining, "drain", "-o", "json"); err != nil {
		t.Fatalf("CLI drain error = %v", err)
	}
	if httpDraining.State != aghcontract.DrainStateDraining ||
		httpDraining != udsDraining || udsDraining != cliDraining {
		t.Fatalf("drain parity mismatch: HTTP=%#v UDS=%#v CLI=%#v", httpDraining, udsDraining, cliDraining)
	}

	var cliActive aghcontract.DrainStatusResponse
	if err := runtimeHarness.CLI.RunJSON(ctx, &cliActive, "undrain", "-o", "json"); err != nil {
		t.Fatalf("CLI undrain error = %v", err)
	}
	var udsActive aghcontract.DrainStatusResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodPost, "/api/undrain", nil, &udsActive); err != nil {
		t.Fatalf("UDS undrain error = %v", err)
	}
	var httpActive aghcontract.DrainStatusResponse
	if err := runtimeHarness.HTTPJSON(ctx, http.MethodPost, "/api/undrain", nil, &httpActive); err != nil {
		t.Fatalf("HTTP undrain error = %v", err)
	}
	if cliActive.State != aghcontract.DrainStateActive ||
		cliActive != udsActive || udsActive != httpActive {
		t.Fatalf("undrain parity mismatch: CLI=%#v UDS=%#v HTTP=%#v", cliActive, udsActive, httpActive)
	}
}

func TestUDSTransportRuntimeMemoryDoctorMatchesHTTPAndCLI(t *testing.T) {
	t.Run("Should return identical runtime memory data over HTTP UDS and CLI", func(t *testing.T) {
		t.Parallel()
		acpmock.RequireDriver(t)

		runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{})
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		var httpPayload aghcontract.DoctorPayload
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, "/api/doctor", nil, &httpPayload); err != nil {
			t.Fatalf("HTTP doctor error = %v", err)
		}
		var udsPayload aghcontract.DoctorPayload
		if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, "/api/doctor", nil, &udsPayload); err != nil {
			t.Fatalf("UDS doctor error = %v", err)
		}
		var cliPayload aghcontract.DoctorPayload
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
	t.Run("Should preserve window manager reads mutations clients layouts and errors across real transports", func(t *testing.T) {
		t.Parallel()
		acpmock.RequireDriver(t)

		runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{})
		clients, err := runtimeHarness.TransportClients()
		if err != nil {
			t.Fatalf("TransportClients() error = %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		basePath := "/api/workspaces/" + url.PathEscape(runtimeHarness.WorkspaceID) + "/window-manager"
		var httpSnapshot aghcontract.WindowManagerSnapshot
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, basePath, nil, &httpSnapshot); err != nil {
			t.Fatalf("HTTP window manager snapshot error = %v", err)
		}
		var udsSnapshot aghcontract.WindowManagerSnapshot
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
		var httpPreview aghcontract.WindowManagerPreview
		if err := runtimeHarness.HTTPJSON(
			ctx,
			http.MethodPost,
			basePath+"/preview",
			createRequest,
			&httpPreview,
		); err != nil {
			t.Fatalf("HTTP window manager preview error = %v", err)
		}
		var udsPreview aghcontract.WindowManagerPreview
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

		var httpMutation aghcontract.WindowManagerResult
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
			t.Fatalf("HTTP window manager mutation = %#v, want applied revision %d", httpMutation, httpSnapshot.Revision+1)
		}

		udsCreateRequest := windowManagerTransportCreateDesktopRequest(
			runtimeHarness.WorkspaceID,
			httpMutation.Snapshot.Revision,
			"desktop-uds",
		)
		var udsMutation aghcontract.WindowManagerResult
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

		registration := aghcontract.WindowManagerClientRegistration{
			WorkspaceID: windowmanager.WorkspaceID(runtimeHarness.WorkspaceID),
			ClientID:    "transport-parity-client",
		}
		var httpClient aghcontract.WindowManagerClientView
		if err := runtimeHarness.HTTPJSON(
			ctx,
			http.MethodPost,
			basePath+"/clients",
			registration,
			&httpClient,
		); err != nil {
			t.Fatalf("HTTP window manager client registration error = %v", err)
		}
		var udsClient aghcontract.WindowManagerClientView
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

		var httpLayout aghcontract.WindowManagerLayoutDocument
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, basePath+"/layout", nil, &httpLayout); err != nil {
			t.Fatalf("HTTP window manager layout export error = %v", err)
		}
		var udsLayout aghcontract.WindowManagerLayoutDocument
		if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, basePath+"/layout", nil, &udsLayout); err != nil {
			t.Fatalf("UDS window manager layout export error = %v", err)
		}
		if !reflect.DeepEqual(httpLayout, udsLayout) {
			t.Fatalf("window manager layout parity mismatch: HTTP=%#v UDS=%#v", httpLayout, udsLayout)
		}

		validationRequest := aghcontract.WindowManagerLayoutValidationRequest{
			WorkspaceID: windowmanager.WorkspaceID(runtimeHarness.WorkspaceID),
			Document:    httpLayout,
		}
		var httpValidation aghcontract.WindowManagerLayoutValidationResponse
		if err := runtimeHarness.HTTPJSON(
			ctx,
			http.MethodPost,
			basePath+"/layout/validate",
			validationRequest,
			&httpValidation,
		); err != nil {
			t.Fatalf("HTTP window manager layout validation error = %v", err)
		}
		var udsValidation aghcontract.WindowManagerLayoutValidationResponse
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
	})
}

func windowManagerTransportCreateDesktopRequest(
	workspaceID string,
	revision aghcontract.WindowManagerRevision,
	desktopID string,
) aghcontract.WindowManagerCommandRequest {
	return aghcontract.WindowManagerCommandRequest{
		WorkspaceID:      windowmanager.WorkspaceID(workspaceID),
		CommandID:        aghcontract.WindowManagerCommandDesktopCreate,
		ExpectedRevision: &revision,
		Actor:            aghcontract.WindowManagerActor{Kind: "test", ID: "transport-parity"},
		Origin:           "transport-parity",
		Payload: json.RawMessage(fmt.Sprintf(
			`{"desktop_id":%q,"name":%q,"purpose":"standard"}`,
			desktopID,
			desktopID,
		)),
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

	sessionPayload, err := runtimeHarness.CreateSession(ctx, aghcontract.CreateSessionRequest{
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

	session, err := runtimeHarness.CreateSession(ctx, aghcontract.CreateSessionRequest{
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

func TestUDSTransportSessionProviderCreateReadMatchesHTTP(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "automation_task_fixture.json"),
			FixtureAgent: "automation-runner",
			AgentName:    transportUDSAutomationAgent,
		}},
	})

	registration, ok := runtimeHarness.MockAgentRegistration(transportUDSAutomationAgent)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) not found", transportUDSAutomationAgent)
	}
	provider := registration.Provider

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var created aghcontract.SessionResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodPost, "/api/sessions", aghcontract.CreateSessionRequest{
		AgentName:     transportUDSAutomationAgent,
		Provider:      provider,
		WorkspacePath: runtimeHarness.WorkspaceRoot,
		Prompt:        "Continue delegated task run",
	}, &created); err != nil {
		t.Fatalf("UDS create session error = %v", err)
	}
	if created.Session.ID == "" {
		t.Fatal("UDS create session id = empty, want non-empty")
	}
	if created.Session.Provider != provider {
		t.Fatalf("UDS create provider = %q, want %q", created.Session.Provider, provider)
	}
	if created.Session.State != session.StateStarting {
		t.Fatalf("UDS accepted create state = %q, want %q", created.Session.State, session.StateStarting)
	}
	created.Session = waitForTransportSessionActive(t, ctx, runtimeHarness, created.Session)
	waitForTransportTranscriptText(
		t,
		ctx,
		runtimeHarness,
		created.Session.ID,
		"Continue delegated task run",
		"Delegated task session responded.",
	)

	var udsDetail aghcontract.SessionResponse
	if err := runtimeHarness.UDSJSON(
		ctx,
		http.MethodGet,
		transportHarnessSessionPath(t, runtimeHarness, created.Session.ID, ""),
		nil,
		&udsDetail,
	); err != nil {
		t.Fatalf("UDS get session error = %v", err)
	}
	if udsDetail.Session.Provider != created.Session.Provider {
		t.Fatalf(
			"UDS detail provider = %q, want create provider %q",
			udsDetail.Session.Provider,
			created.Session.Provider,
		)
	}

	var httpDetail aghcontract.SessionResponse
	if err := runtimeHarness.HTTPJSON(
		ctx,
		http.MethodGet,
		transportHarnessSessionPath(t, runtimeHarness, created.Session.ID, ""),
		nil,
		&httpDetail,
	); err != nil {
		t.Fatalf("HTTP get session error = %v", err)
	}
	if httpDetail.Session.Provider != created.Session.Provider {
		t.Fatalf(
			"HTTP detail provider = %q, want UDS create provider %q",
			httpDetail.Session.Provider,
			created.Session.Provider,
		)
	}
}

func TestUDSTransportResumeMissingProviderReturnsExplicitBadRequest(t *testing.T) {
	acpmock.RequireDriver(t)
	t.Parallel()

	runtimeHarness := e2etest.StartRuntimeHarness(t, &e2etest.RuntimeHarnessOptions{
		MockAgents: []e2etest.MockAgentSpec{{
			FixturePath:  transportMockFixturePath(t, "automation_task_fixture.json"),
			FixtureAgent: "automation-runner",
			AgentName:    transportUDSAutomationAgent,
		}},
	})

	registration, ok := runtimeHarness.MockAgentRegistration(transportUDSAutomationAgent)
	if !ok {
		t.Fatalf("MockAgentRegistration(%q) not found", transportUDSAutomationAgent)
	}

	writeTransportProviderOverrideConfig(
		t,
		runtimeHarness.WorkspaceRoot,
		transportUDSOverrideProvider,
		registration.Command,
		true,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var created aghcontract.SessionResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodPost, "/api/sessions", aghcontract.CreateSessionRequest{
		AgentName:     transportUDSAutomationAgent,
		Provider:      transportUDSOverrideProvider,
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
	if stopResp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(stopResp.Body)
		_ = stopResp.Body.Close()
		t.Fatalf(
			"UDS stop session status = %d, want %d; body=%s",
			stopResp.StatusCode,
			http.StatusNoContent,
			string(body),
		)
	}
	_ = stopResp.Body.Close()

	writeTransportProviderOverrideConfig(
		t,
		runtimeHarness.WorkspaceRoot,
		transportUDSOverrideProvider,
		registration.Command,
		false,
	)

	resumeResp := mustUnixRequest(
		t,
		runtimeHarness.UDSClient,
		http.MethodPost,
		runtimeHarness.UDSURL(transportHarnessSessionPath(t, runtimeHarness, created.Session.ID, "/attach")),
		nil,
		nil,
	)
	body, err := io.ReadAll(resumeResp.Body)
	closeErr := resumeResp.Body.Close()
	if err != nil {
		t.Fatalf("read UDS resume body error = %v", err)
	}
	if closeErr != nil {
		t.Fatalf("close UDS resume body error = %v", closeErr)
	}
	if resumeResp.StatusCode != http.StatusConflict {
		t.Fatalf("UDS attach status = %d, want %d; body=%s", resumeResp.StatusCode, http.StatusConflict, string(body))
	}
	if !strings.Contains(string(body), created.Session.ID) {
		t.Fatalf("UDS attach body = %s, want session id %q", string(body), created.Session.ID)
	}
	if !strings.Contains(string(body), "not attachable") {
		t.Fatalf("UDS attach body = %s, want attachability context", string(body))
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
				Mutate: func(cfg *aghconfig.Config) {
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
		var httpValue aghcontract.MarketplaceSearchResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, path, nil, &httpValue); err != nil {
			t.Fatalf("HTTPJSON(%s) error = %v", path, err)
		}
		var udsValue aghcontract.MarketplaceSearchResponse
		if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, path, nil, &udsValue); err != nil {
			t.Fatalf("UDSJSON(%s) error = %v", path, err)
		}
		var cliValue aghcontract.MarketplaceSearchResponse
		if err := clients.CLI.RunJSON(ctx, &cliValue, "marketplace", "search", "--limit", "20", "-o", "json"); err != nil {
			t.Fatalf("CLI marketplace search error = %v", err)
		}
		assertTransportMarketplaceParity(t, path, httpValue, udsValue, cliValue)
	})

	t.Run("Should return the same kind browse payload over HTTP, UDS, and CLI", func(t *testing.T) {
		path := "/api/marketplace/extension?limit=20"
		var httpValue aghcontract.MarketplaceKindResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, path, nil, &httpValue); err != nil {
			t.Fatalf("HTTPJSON(%s) error = %v", path, err)
		}
		var udsValue aghcontract.MarketplaceKindResponse
		if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, path, nil, &udsValue); err != nil {
			t.Fatalf("UDSJSON(%s) error = %v", path, err)
		}
		var cliValue aghcontract.MarketplaceKindResponse
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
		var httpValue aghcontract.MarketplaceEntryResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, path, nil, &httpValue); err != nil {
			t.Fatalf("HTTPJSON(%s) error = %v", path, err)
		}
		var udsValue aghcontract.MarketplaceEntryResponse
		if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, path, nil, &udsValue); err != nil {
			t.Fatalf("UDSJSON(%s) error = %v", path, err)
		}
		var cliValue aghcontract.MarketplaceEntryResponse
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
		request := aghcontract.InstallSettingsMCPServerRequest{
			EntryID: "filesystem",
			Name:    "filesystem-parity",
			Scope:   aghcontract.SettingsWorkspaceScopeGlobal,
			Values:  &aghcontract.SettingsMCPCatalogInstallValuesPayload{},
		}
		var httpValue aghcontract.InstallSettingsMCPServerResponse
		var udsValue aghcontract.InstallSettingsMCPServerResponse
		var cliValue aghcontract.InstallSettingsMCPServerResponse
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
			httpValue.NextStep != aghcontract.SettingsMCPInstallNextStepNone {
			t.Fatalf("catalog MCP install response = %#v", httpValue)
		}
	})

	t.Run("Should retarget extension search without changing its output fields", func(t *testing.T) {
		var cliValue []struct {
			Slug        string `json:"slug"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Version     string `json:"version"`
			Source      string `json:"source"`
		}
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
		if len(cliValue) != 1 {
			t.Fatalf("CLI extension search items = %#v, want one curated entry", cliValue)
		}
		item := cliValue[0]
		if item.Slug != "compozy/bridge-github" ||
			item.Name != "GitHub bridge" ||
			item.Description != "Connect GitHub events to AGH" ||
			item.Version != "1.0.0" ||
			item.Source != "agh-catalog" {
			t.Fatalf("CLI extension search item = %#v, want unchanged extension search fields", item)
		}
	})

	t.Run("Should refresh the same curated kind over guarded HTTP and unguarded UDS", func(t *testing.T) {
		path := "/api/marketplace/refresh?kind=extension"
		var httpValue aghcontract.MarketplaceRefreshResponse
		if err := runtimeHarness.HTTPJSON(ctx, http.MethodPost, path, nil, &httpValue); err != nil {
			t.Fatalf("HTTPJSON(%s) error = %v", path, err)
		}
		var udsValue aghcontract.MarketplaceRefreshResponse
		if err := runtimeHarness.UDSJSON(ctx, http.MethodPost, path, nil, &udsValue); err != nil {
			t.Fatalf("UDSJSON(%s) error = %v", path, err)
		}
		assertTransportMarketplaceParity(t, path, httpValue, udsValue)
	})
}

func newTransportMarketplaceCatalogServer(t testing.TB) *httptest.Server {
	t.Helper()

	documents := map[string]string{
		"/mcp.json": `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
			`"entry_id":"filesystem","name":"Filesystem","description":"Read approved local files",` +
			`"version":"1.0.0","transport":"stdio","command":"npx","args":["server-filesystem"]}]}`,
		"/extensions.json": `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
			`"entry_id":"bridge-github","name":"GitHub bridge","description":"Connect GitHub events to AGH",` +
			`"version":"1.0.0","install_slug":"compozy/bridge-github",` +
			`"artifact_url":"https://downloads.example.test/bridge-github-v1.0.0.tar.gz",` +
			`"digest_sha256":"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",` +
			`"tier":"official"}]}`,
		"/skills.json": `{"manifest_version":1,"generated_at":"2026-07-13T00:00:00Z","entries":[{` +
			`"entry_id":"agh","name":"AGH","display_name":"AGH operator",` +
			`"description":"Operate AGH through structured surfaces","version":"1.0.0",` +
			`"install_slug":"compozy/agh","author":"Compozy","tags":["agh","operations"]}]}`,
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
		t.Fatalf("%s transport 0 payload = %s, want parity with transport %d payload %s", path, expectedJSON, index+1, valueJSON)
	}
}

func assertTransportMCPInstallParity(
	t testing.TB,
	path string,
	values ...aghcontract.InstallSettingsMCPServerResponse,
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

	session, err := runtimeHarness.CreateSession(ctx, aghcontract.CreateSessionRequest{
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

	session, err := runtimeHarness.CreateSession(ctx, aghcontract.CreateSessionRequest{
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
		func(fetchCtx context.Context) ([]aghcontract.LogEventPayload, error) {
			var response aghcontract.LogsListResponse
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
		func(fetchCtx context.Context) ([]aghcontract.LogEventPayload, error) {
			var response aghcontract.LogsListResponse
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
	if !strings.Contains(httpHarnessEvents[3].Summary, "augmenter=durable_memory") {
		t.Fatalf("augmenter summary = %q, want durable memory metadata", httpHarnessEvents[3].Summary)
	}
	if !strings.Contains(httpHarnessEvents[4].Summary, "augmenter=skills") {
		t.Fatalf("augmenter summary = %q, want skills metadata", httpHarnessEvents[4].Summary)
	}
	if !strings.Contains(httpHarnessEvents[5].Summary, "augmenter=situation") {
		t.Fatalf("augmenter summary = %q, want situation metadata", httpHarnessEvents[5].Summary)
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
			decode: func() any { return &aghcontract.SettingsGeneralResponse{} },
		},
		{
			name:   "providers collection",
			path:   "/api/settings/providers",
			decode: func() any { return &aghcontract.SettingsProvidersResponse{} },
		},
		{
			name: "workspace mcp servers collection",
			path: "/api/settings/mcp-servers?scope=workspace&workspace_id=" + url.QueryEscape(workspaceID),
			decode: func() any {
				return &aghcontract.SettingsMCPServersResponse{}
			},
		},
		{
			name:   "hooks and extensions section",
			path:   "/api/settings/hooks-extensions",
			decode: func() any { return &aghcontract.SettingsHooksExtensionsResponse{} },
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			httpValue := tc.decode()
			if err := runtimeHarness.HTTPJSON(ctx, http.MethodGet, tc.path, nil, httpValue); err != nil {
				t.Fatalf("HTTPJSON(%s) error = %v", tc.path, err)
			}

			udsValue := tc.decode()
			if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, tc.path, nil, udsValue); err != nil {
				t.Fatalf("UDSJSON(%s) error = %v", tc.path, err)
			}

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
		body, _ := io.ReadAll(httpListResp.Body)
		_ = httpListResp.Body.Close()
		t.Fatalf(
			"HTTP extension list status = %d, want %d; body=%s",
			httpListResp.StatusCode,
			http.StatusOK,
			string(body),
		)
	}
	var httpList aghcontract.ExtensionsResponse
	decodeHTTPJSON(t, httpListResp, &httpList)

	var udsList aghcontract.ExtensionsResponse
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
		body, _ := io.ReadAll(httpStatusResp.Body)
		_ = httpStatusResp.Body.Close()
		t.Fatalf("HTTP extension status = %d, want %d; body=%s", httpStatusResp.StatusCode, http.StatusOK, string(body))
	}
	var httpStatus aghcontract.ExtensionResponse
	decodeHTTPJSON(t, httpStatusResp, &httpStatus)

	var udsStatus aghcontract.ExtensionResponse
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
	putBody, err := json.Marshal(aghcontract.PutSettingsMCPServerRequest{
		Server: aghcontract.SettingsMCPServerPayload{
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
		body, _ := io.ReadAll(httpPutResp.Body)
		_ = httpPutResp.Body.Close()
		t.Fatalf(
			"HTTP PUT settings status = %d, want %d; body=%s",
			httpPutResp.StatusCode,
			http.StatusForbidden,
			string(body),
		)
	}
	var forbidden aghcontract.ErrorPayload
	decodeHTTPJSON(t, httpPutResp, &forbidden)

	var udsMutation aghcontract.SettingsGlobalWorkspaceCollectionMutationResult
	if err := runtimeHarness.UDSJSON(
		ctx,
		http.MethodPut,
		putPath,
		aghcontract.PutSettingsMCPServerRequest{
			Server: aghcontract.SettingsMCPServerPayload{
				Name:    "server-a",
				Command: "mcpd",
				Args:    []string{"serve"},
			},
		},
		&udsMutation,
	); err != nil {
		t.Fatalf("UDSJSON(PUT %s) error = %v", putPath, err)
	}
	if udsMutation.Scope != aghcontract.SettingsWorkspaceScopeWorkspace || udsMutation.WorkspaceID != workspaceID {
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
		body, _ := io.ReadAll(httpListResp.Body)
		_ = httpListResp.Body.Close()
		t.Fatalf(
			"HTTP GET settings list status = %d, want %d; body=%s",
			httpListResp.StatusCode,
			http.StatusForbidden,
			string(body),
		)
	}
	var listForbidden aghcontract.ErrorPayload
	decodeHTTPJSON(t, httpListResp, &listForbidden)

	var udsList aghcontract.SettingsMCPServersResponse
	if err := runtimeHarness.UDSJSON(ctx, http.MethodGet, listPath, nil, &udsList); err != nil {
		t.Fatalf("UDSJSON(GET %s) error = %v", listPath, err)
	}
	if !settingsMCPServerPresent(udsList.MCPServers, "server-a") {
		t.Fatalf("UDS mcp list = %#v, want server-a after UDS mutation", udsList)
	}

	deletePath := "/api/settings/mcp-servers/server-a?scope=workspace&workspace_id=" +
		url.QueryEscape(workspaceID) + "&target=sidecar"
	var deleteResult aghcontract.SettingsGlobalWorkspaceCollectionMutationResult
	if err := runtimeHarness.UDSJSON(ctx, http.MethodDelete, deletePath, nil, &deleteResult); err != nil {
		t.Fatalf("UDSJSON(DELETE %s) error = %v", deletePath, err)
	}
	if deleteResult.Scope != aghcontract.SettingsWorkspaceScopeWorkspace || deleteResult.WorkspaceID != workspaceID {
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
		body, _ := io.ReadAll(httpListResp.Body)
		_ = httpListResp.Body.Close()
		t.Fatalf(
			"HTTP GET settings list after delete status = %d, want %d; body=%s",
			httpListResp.StatusCode,
			http.StatusForbidden,
			string(body),
		)
	}
	var listAfterDeleteForbidden aghcontract.ErrorPayload
	decodeHTTPJSON(t, httpListResp, &listAfterDeleteForbidden)

	var udsListAfterDelete aghcontract.SettingsMCPServersResponse
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
) aghcontract.RunPayload {
	t.Helper()

	return waitForTransportAutomationRun(
		t,
		ctx,
		runID,
		"waitForUDSAutomationRun",
		func(fetchCtx context.Context) (aghcontract.RunPayload, error) {
			return harness.GetAutomationRun(fetchCtx, runID)
		},
	)
}

func transportSettingsHTTPURL(harness *e2etest.RuntimeHarness, path string) string {
	return fmt.Sprintf("http://127.0.0.1:%d%s", harness.Config.HTTP.Port, path)
}

func settingsMCPServerPresent(items []aghcontract.SettingsMCPServerItemPayload, name string) bool {
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
) aghcontract.RunPayload {
	t.Helper()

	return waitForTransportAutomationRun(
		t,
		ctx,
		runID,
		"waitForCLIAutomationRun",
		func(fetchCtx context.Context) (aghcontract.RunPayload, error) {
			var run aghcontract.RunPayload
			err := client.RunJSON(fetchCtx, &run, "automation", "runs", "get", runID, "-o", "json")
			return run, err
		},
	)
}

func seedTransportWebhookTrigger(
	t testing.TB,
	ctx context.Context,
	harness *e2etest.RuntimeHarness,
) (aghcontract.TriggerPayload, string) {
	t.Helper()

	state, err := harness.SeedAutomationFixtures(ctx, e2etest.AutomationFixtureSeed{
		Triggers: []aghcontract.CreateTriggerRequest{{
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
) aghcontract.RunPayload {
	t.Helper()

	return waitForTransportAutomationRun(
		t,
		ctx,
		runID,
		"waitForHTTPAutomationRun",
		func(fetchCtx context.Context) (aghcontract.RunPayload, error) {
			var response aghcontract.RunResponse
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
	fetch func(context.Context) (aghcontract.RunPayload, error),
) aghcontract.RunPayload {
	t.Helper()

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	var (
		lastErr error
		lastRun aghcontract.RunPayload
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
) (aghcontract.SessionsResponse, aghcontract.SessionsResponse) {
	t.Helper()

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	path := "/api/sessions?workspace=" + url.QueryEscape(harness.WorkspaceID)

	var (
		httpCatalog aghcontract.SessionsResponse
		udsCatalog  aghcontract.SessionsResponse
		httpErr     error
		udsErr      error
	)
	for {
		httpCatalog = aghcontract.SessionsResponse{}
		httpErr = harness.HTTPJSON(waitCtx, http.MethodGet, path, nil, &httpCatalog)
		udsCatalog = aghcontract.SessionsResponse{}
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
	accepted aghcontract.SessionPayload,
) aghcontract.SessionPayload {
	t.Helper()
	active, err := harness.WaitForSessionActive(ctx, accepted.ID)
	if err != nil {
		t.Fatalf("WaitForSessionActive(%q) error = %v; accepted=%#v", accepted.ID, err, accepted)
	}
	return active
}

func transportCatalogHasSingleTitle(
	catalog aghcontract.SessionsResponse,
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
	catalog aghcontract.SessionsResponse,
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
	fetch func(context.Context) ([]aghcontract.LogEventPayload, error),
) []aghcontract.LogEventPayload {
	t.Helper()

	waitCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	var (
		lastErr    error
		lastEvents []aghcontract.LogEventPayload
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

func udsSessionEventsContainType(events []aghcontract.SessionEventPayload, want string) bool {
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

func filterHarnessListLogs(events []aghcontract.LogEventPayload) []aghcontract.LogEventPayload {
	filtered := make([]aghcontract.LogEventPayload, 0, len(events))
	for _, event := range events {
		if strings.HasPrefix(event.Type, "harness.") {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

func logEventTypes(events []aghcontract.LogEventPayload) []string {
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

	configDir := filepath.Join(workspaceRoot, aghconfig.DirName)
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

	configPath := filepath.Join(configDir, aghconfig.ConfigName)
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
	payload aghcontract.DoctorPayload,
	id string,
) aghcontract.DiagnosticItem {
	t.Helper()
	for _, item := range payload.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("doctor items = %#v, want %q", payload.Items, id)
	return aghcontract.DiagnosticItem{}
}

func transportPositiveNumber(value any) bool {
	number, ok := value.(float64)
	return ok && number > 0
}
