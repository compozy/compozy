package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/cmdpalette"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

func TestBaseHandlersCmdPalette(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve list scope and flatten the canonical catalog wire", func(t *testing.T) {
		t.Parallel()
		registry := &cmdPaletteRegistryStub{catalog: cmdpalette.Catalog{
			Commands: []cmdpalette.ResolvedCommand{{
				Descriptor: cmdpalette.Descriptor{
					ID: "window.close", Title: "Close window", Section: "Window", Icon: "x-square",
					Source:    cmdpalette.Source{Kind: cmdpalette.SourceKindCore},
					Action:    cmdpalette.Action{Kind: cmdpalette.ActionKindClientOp, Op: "window.close"},
					Arguments: []cmdpalette.Argument{},
				},
				Available: true, Bindings: []string{"meta+KeyW"},
			}},
			Sources:  []cmdpalette.SourceStatus{{Source: "core", Status: cmdpalette.SourceHealthy}},
			Revision: "cr_test", ContextRevision: "ctx_4",
		}}
		handlers := newCmdPaletteHandlers(registry, nil)
		engine := gin.New()
		engine.GET("/api/cmd-palette/commands", handlers.ListCmdPaletteCommands)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(
			http.MethodGet,
			"/api/cmd-palette/commands?workspace=alpha&client=client-a",
			nil,
		)
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		var response contract.CmdPaletteCommandsResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("json.Unmarshal(response) error = %v", err)
		}
		if registry.catalogWorkspace != "workspace-canonical" || registry.catalogClient != "client-a" {
			t.Fatalf(
				"Catalog() scope = %q/%q, want workspace-canonical/client-a",
				registry.catalogWorkspace,
				registry.catalogClient,
			)
		}
		if len(response.Commands) != 1 || response.Commands[0].Source != "core" ||
			response.CatalogRevision != "cr_test" || response.ContextRevision != "ctx_4" {
			t.Fatalf("response = %#v, want flattened canonical catalog", response)
		}
	})

	t.Run("Should map invoke authorization and frozen domain errors", func(t *testing.T) {
		t.Parallel()
		tests := []struct {
			name       string
			err        error
			wantStatus int
			wantCode   string
		}{
			{name: "unknown", err: cmdpalette.ErrCommandNotFound, wantStatus: 404, wantCode: "command_not_found"},
			{
				name: "invalid arguments", err: &cmdpalette.InvalidArgumentsError{Fields: map[string]string{"title": "required"}},
				wantStatus: 422, wantCode: "invalid_arguments",
			},
			{
				name: "unavailable", err: &cmdpalette.UnavailableError{Reason: "needs two windows"},
				wantStatus: 412, wantCode: "command_unavailable",
			},
			{name: "no shell", err: cmdpalette.ErrNoAttachedShell, wantStatus: 412, wantCode: "no_attached_shell"},
			{
				name: "multiple clients", err: &cmdpalette.MultipleClientsError{Clients: []cmdpalette.ClientID{"a", "b"}},
				wantStatus: 409, wantCode: "multiple_clients",
			},
			{name: "running", err: cmdpalette.ErrAlreadyRunning, wantStatus: 409, wantCode: "already_running"},
			{name: "forged token", err: cmdpalette.ErrClientUnauthorized, wantStatus: 401, wantCode: "client_unauthorized"},
		}
		for _, test := range tests {
			t.Run("Should map "+test.name, func(t *testing.T) {
				t.Parallel()
				registry := &cmdPaletteRegistryStub{invokeErr: test.err}
				handlers := newCmdPaletteHandlers(registry, nil)
				engine := gin.New()
				engine.POST("/api/cmd-palette/commands/:id/invoke", handlers.InvokeCmdPaletteCommand)
				body := bytes.NewBufferString(`{"workspace":"alpha","args":{},"client":"client-a"}`)
				request := httptest.NewRequest(
					http.MethodPost,
					"/api/cmd-palette/commands/window.close/invoke",
					body,
				)
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("X-Compozy-Client-Token", "attachment-token")
				recorder := httptest.NewRecorder()
				engine.ServeHTTP(recorder, request)
				if recorder.Code != test.wantStatus {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, test.wantStatus, recorder.Body.String())
				}
				var payload contract.CmdPaletteError
				if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
					t.Fatalf("json.Unmarshal(error) error = %v", err)
				}
				if payload.Error != test.wantCode {
					t.Fatalf("error = %q, want %q", payload.Error, test.wantCode)
				}
				if registry.invokeRequest.WorkspaceID != "workspace-canonical" ||
					registry.invokeRequest.Caller != cmdpalette.CallerAttachedClient ||
					registry.invokeRequest.ClientToken != "attachment-token" {
					t.Fatalf("Invoke() request = %#v, want canonical attached caller", registry.invokeRequest)
				}
			})
		}
	})

	t.Run("Should expose and cancel tools-owned pending approvals", func(t *testing.T) {
		t.Parallel()
		expiresAt := time.Now().UTC().Add(time.Minute)
		coordinator := &approvalCoordinatorStub{status: toolspkg.ApprovalStatus{
			ApprovalID: "apr_test", ApprovalStatus: toolspkg.ApprovalPending, ExpiresAt: expiresAt,
		}}
		handlers := newCmdPaletteHandlers(nil, coordinator)
		engine := gin.New()
		engine.GET("/api/tools/approvals/:id", handlers.GetPendingToolApproval)
		engine.POST("/api/tools/approvals/:id/cancel", handlers.CancelPendingToolApproval)

		get := httptest.NewRecorder()
		engine.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/api/tools/approvals/apr_test", nil))
		if get.Code != http.StatusOK {
			t.Fatalf("GET status = %d, want 200; body=%s", get.Code, get.Body.String())
		}
		cancel := httptest.NewRecorder()
		engine.ServeHTTP(cancel, httptest.NewRequest(http.MethodPost, "/api/tools/approvals/apr_test/cancel", nil))
		if cancel.Code != http.StatusOK || coordinator.canceled != "apr_test" {
			t.Fatalf("cancel = status %d id %q; body=%s", cancel.Code, coordinator.canceled, cancel.Body.String())
		}
		var response contract.ToolApprovalStatusResponse
		if err := json.Unmarshal(cancel.Body.Bytes(), &response); err != nil {
			t.Fatalf("json.Unmarshal(cancel) error = %v", err)
		}
		if response.ApprovalStatus != toolspkg.ApprovalCanceled {
			t.Fatalf("approval_status = %q, want canceled", response.ApprovalStatus)
		}
	})

	t.Run("Should reconcile the catalog revision when an SSE stream opens", func(t *testing.T) {
		t.Parallel()
		updates := make(chan cmdpalette.Event)
		close(updates)
		registry := &cmdPaletteRegistryStub{
			catalog:      cmdpalette.Catalog{Revision: "cr_current"},
			eventUpdates: updates,
		}
		handlers := newCmdPaletteHandlers(registry, nil)
		engine := gin.New()
		engine.GET("/api/cmd-palette/stream", handlers.StreamCmdPalette)
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(
			http.MethodGet,
			"/api/cmd-palette/stream?workspace=alpha",
			nil,
		)
		engine.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Type") != "text/event-stream" {
			t.Fatalf("stream = status %d content-type %q", recorder.Code, recorder.Header().Get("Content-Type"))
		}
		want := "event: cmd_palette.catalog.changed\ndata: {\"workspace\":\"workspace-canonical\",\"catalog_revision\":\"cr_current\"}\n\n"
		if recorder.Body.String() != want {
			t.Fatalf("stream body = %q, want %q", recorder.Body.String(), want)
		}
	})
}

func newCmdPaletteHandlers(
	registry cmdpalette.Registry,
	coordinator toolspkg.ApprovalCoordinator,
) *core.BaseHandlers {
	return core.NewBaseHandlers(&core.BaseHandlerConfig{
		CmdPalette: registry, ApprovalCoordinator: coordinator,
		Workspaces: testutil.StubWorkspaceService{
			ResolveFn: func(_ context.Context, ref string) (workspacepkg.ResolvedWorkspace, error) {
				if ref != "alpha" {
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
				return workspacepkg.ResolvedWorkspace{
					Workspace:   workspacepkg.Workspace{ID: "workspace-canonical", Name: "alpha"},
					WorkspaceID: "workspace-canonical",
				}, nil
			},
		},
	})
}

type cmdPaletteRegistryStub struct {
	catalog          cmdpalette.Catalog
	catalogWorkspace cmdpalette.WorkspaceID
	catalogClient    cmdpalette.ClientID
	invokeRequest    cmdpalette.InvokeRequest
	invokeResult     cmdpalette.InvokeResult
	invokeErr        error
	eventUpdates     <-chan cmdpalette.Event
}

func (s *cmdPaletteRegistryStub) Catalog(
	_ context.Context,
	workspaceID cmdpalette.WorkspaceID,
	clientID cmdpalette.ClientID,
) (cmdpalette.Catalog, error) {
	s.catalogWorkspace = workspaceID
	s.catalogClient = clientID
	return s.catalog, nil
}

func (s *cmdPaletteRegistryStub) Clients(
	_ context.Context,
	_ cmdpalette.WorkspaceID,
) ([]cmdpalette.Client, error) {
	return []cmdpalette.Client{}, nil
}

func (s *cmdPaletteRegistryStub) Invoke(
	_ context.Context,
	request cmdpalette.InvokeRequest,
) (cmdpalette.InvokeResult, error) {
	s.invokeRequest = request
	return s.invokeResult, s.invokeErr
}

func (s *cmdPaletteRegistryStub) SubscribeCmdPaletteEvents(
	context.Context,
	cmdpalette.WorkspaceID,
) (<-chan cmdpalette.Event, func(), error) {
	if s.eventUpdates != nil {
		return s.eventUpdates, func() {}, nil
	}
	updates := make(chan cmdpalette.Event)
	close(updates)
	return updates, func() {}, nil
}

type approvalCoordinatorStub struct {
	status   toolspkg.ApprovalStatus
	canceled string
}

func (s *approvalCoordinatorStub) Begin(
	context.Context,
	toolspkg.ApprovalRequest,
) (toolspkg.ApprovalTicket, error) {
	return toolspkg.ApprovalTicket{}, errors.New("unexpected Begin call")
}

func (s *approvalCoordinatorStub) Resolve(context.Context, string, toolspkg.ApprovalOutcome) error {
	return errors.New("unexpected Resolve call")
}

func (s *approvalCoordinatorStub) Status(_ context.Context, id string) (toolspkg.ApprovalStatus, error) {
	if id != s.status.ApprovalID {
		return toolspkg.ApprovalStatus{}, toolspkg.ErrApprovalNotFound
	}
	return s.status, nil
}

func (s *approvalCoordinatorStub) Cancel(_ context.Context, id string) error {
	if id != s.status.ApprovalID {
		return toolspkg.ErrApprovalNotFound
	}
	s.canceled = id
	s.status.ApprovalStatus = toolspkg.ApprovalCanceled
	return nil
}

func (s *approvalCoordinatorStub) Recover(context.Context) error { return nil }
func (s *approvalCoordinatorStub) Close() error                  { return nil }
