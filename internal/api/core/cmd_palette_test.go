package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/api/cmd-palette/commands?workspace=alpha&client=client-a",
			http.NoBody,
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
				name:       "invalid arguments",
				err:        &cmdpalette.InvalidArgumentsError{Fields: map[string]string{"title": "required"}},
				wantStatus: 422,
				wantCode:   "invalid_arguments",
			},
			{
				name: "unavailable", err: &cmdpalette.UnavailableError{Reason: "needs two windows"},
				wantStatus: 412, wantCode: "command_unavailable",
			},
			{name: "no shell", err: cmdpalette.ErrNoAttachedShell, wantStatus: 412, wantCode: "no_attached_shell"},
			{
				name:       "multiple clients",
				err:        &cmdpalette.MultipleClientsError{Clients: []cmdpalette.ClientID{"a", "b"}},
				wantStatus: 409,
				wantCode:   "multiple_clients",
			},
			{name: "running", err: cmdpalette.ErrAlreadyRunning, wantStatus: 409, wantCode: "already_running"},
			{
				name:       "forged token",
				err:        cmdpalette.ErrClientUnauthorized,
				wantStatus: 401,
				wantCode:   "client_unauthorized",
			},
		}
		for _, test := range tests {
			t.Run("Should map "+test.name, func(t *testing.T) {
				t.Parallel()
				registry := &cmdPaletteRegistryStub{invokeErr: test.err}
				handlers := newCmdPaletteHandlers(registry, nil)
				engine := gin.New()
				engine.POST("/api/cmd-palette/commands/:id/invoke", handlers.InvokeCmdPaletteCommand)
				body := bytes.NewBufferString(`{"workspace":"alpha","args":{},"client":"client-a"}`)
				request := httptest.NewRequestWithContext(
					t.Context(),
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
		engine.ServeHTTP(get, httptest.NewRequestWithContext(
			t.Context(), http.MethodGet, "/api/tools/approvals/apr_test", http.NoBody,
		))
		if get.Code != http.StatusOK {
			t.Fatalf("GET status = %d, want 200; body=%s", get.Code, get.Body.String())
		}
		cancel := httptest.NewRecorder()
		engine.ServeHTTP(cancel, httptest.NewRequestWithContext(
			t.Context(), http.MethodPost, "/api/tools/approvals/apr_test/cancel", http.NoBody,
		))
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
		request := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/api/cmd-palette/stream?workspace=alpha",
			http.NoBody,
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

	t.Run("Should serve the frozen personalization route family on the shared handlers", func(t *testing.T) {
		t.Parallel()
		registry := &cmdPaletteRegistryStub{
			snapshot: cmdpalette.Snapshot{
				Weights: cmdpalette.WeightsV1,
				Usage: []cmdpalette.UsageSignal{{
					CommandID: "session.new", Weight: 2.5, LastUsedAt: 1765995000123,
				}},
				QueryHits: []cmdpalette.QueryHit{{Query: "ns", CommandID: "session.new", Weight: 1.5}},
				Pins:      []cmdpalette.Pin{{CommandID: "session.new", PinnedAt: 1}},
				Revision:  "ps_test",
			},
			summary: cmdpalette.PersonalizationSummary{
				Workspace: "workspace-canonical", Pins: []cmdpalette.CommandID{"session.new"},
				Recents: 1, FrecencyEntries: 1, QueryAssociations: 1,
			},
		}
		handlers := newCmdPaletteHandlers(registry, nil)
		engine := gin.New()
		engine.GET("/api/cmd-palette/rank-signals", handlers.GetCmdPaletteRankSignals)
		engine.POST("/api/cmd-palette/usage", handlers.RecordCmdPaletteUsage)
		engine.PUT("/api/cmd-palette/pins/:id", handlers.PinCmdPaletteCommand)
		engine.DELETE("/api/cmd-palette/pins/:id", handlers.UnpinCmdPaletteCommand)
		engine.GET("/api/cmd-palette/personalization", handlers.GetCmdPalettePersonalization)
		engine.DELETE("/api/cmd-palette/personalization", handlers.ResetCmdPalettePersonalization)

		rank := httptest.NewRecorder()
		engine.ServeHTTP(rank, httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/api/cmd-palette/rank-signals?workspace=alpha",
			http.NoBody,
		))
		if rank.Code != http.StatusOK {
			t.Fatalf("rank signals status = %d, want 200; body=%s", rank.Code, rank.Body.String())
		}
		var signals contract.CmdPaletteRankSignalsResponse
		if err := json.Unmarshal(rank.Body.Bytes(), &signals); err != nil {
			t.Fatalf("json.Unmarshal(rank signals) error = %v", err)
		}
		if signals.Revision != "ps_test" || signals.Weights.Version != 1 ||
			len(signals.Pins) != 1 || signals.Pins[0] != "session.new" {
			t.Fatalf("rank signals = %#v, want frozen snapshot", signals)
		}

		usage := httptest.NewRecorder()
		usageRequest := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/api/cmd-palette/usage",
			bytes.NewBufferString(`{"workspace":"alpha","command_id":"session.new","query":"ns"}`),
		)
		usageRequest.Header.Set("Content-Type", "application/json")
		engine.ServeHTTP(usage, usageRequest)
		if usage.Code != http.StatusNoContent || usage.Body.Len() != 0 {
			t.Fatalf("usage response = status %d body %q, want 204/empty", usage.Code, usage.Body.String())
		}
		if registry.usage.WorkspaceID != "workspace-canonical" ||
			registry.usage.CommandID != "session.new" || registry.usage.Query != "ns" {
			t.Fatalf("usage domain request = %#v", registry.usage)
		}

		for _, method := range []string{http.MethodPut, http.MethodDelete} {
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequestWithContext(
				t.Context(),
				method,
				"/api/cmd-palette/pins/session.new?workspace=alpha",
				http.NoBody,
			))
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s pin status = %d, want 200; body=%s", method, recorder.Code, recorder.Body.String())
			}
			var response contract.CmdPalettePinResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("json.Unmarshal(%s pin) error = %v", method, err)
			}
			if response.Pinned != (method == http.MethodPut) {
				t.Fatalf("%s pin response = %#v", method, response)
			}
		}

		getSummary := httptest.NewRecorder()
		engine.ServeHTTP(getSummary, httptest.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"/api/cmd-palette/personalization?workspace=alpha",
			http.NoBody,
		))
		if getSummary.Code != http.StatusOK {
			t.Fatalf("personalization status = %d, want 200; body=%s", getSummary.Code, getSummary.Body.String())
		}
		var summary contract.CmdPalettePersonalizationResponse
		if err := json.Unmarshal(getSummary.Body.Bytes(), &summary); err != nil {
			t.Fatalf("json.Unmarshal(personalization) error = %v", err)
		}
		if summary.Workspace != "workspace-canonical" || summary.QueryAssociations != 1 {
			t.Fatalf("personalization summary = %#v", summary)
		}

		reset := httptest.NewRecorder()
		engine.ServeHTTP(reset, httptest.NewRequestWithContext(
			t.Context(),
			http.MethodDelete,
			"/api/cmd-palette/personalization?workspace=alpha",
			http.NoBody,
		))
		if reset.Code != http.StatusOK || reset.Body.String() != "{\"status\":\"reset\"}" {
			t.Fatalf("reset response = status %d body %q", reset.Code, reset.Body.String())
		}
		if registry.resetWorkspace != "workspace-canonical" {
			t.Fatalf("reset workspace = %q, want workspace-canonical", registry.resetWorkspace)
		}
	})

	t.Run("Should serve a validated workspace-scoped declarative view", func(t *testing.T) {
		t.Parallel()
		registry := &cmdPaletteRegistryStub{viewSnapshot: cmdpalette.ViewSnapshot{
			Descriptor: cmdpalette.ViewDescriptor{
				ID: "ext.notes.recent", Title: "Recent notes", Kind: cmdpalette.ViewKindList,
			},
			Payload: cmdpalette.ViewPayload{
				View: cmdpalette.ViewContractVersion,
				Sections: []cmdpalette.Section{{Rows: []cmdpalette.Row{{
					ID: "note-1", Title: "Standup follow-ups",
				}}}},
			},
			Revision: "vr_1", StreamEpoch: "vse_1",
		}}
		handlers := newCmdPaletteHandlers(registry, nil)
		engine := gin.New()
		engine.GET("/api/cmd-palette/views/:id", handlers.GetCmdPaletteView)
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequestWithContext(
			t.Context(), http.MethodGet,
			"/api/cmd-palette/views/ext.notes.recent?workspace=alpha", http.NoBody,
		))
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
		}
		var response contract.CmdPaletteViewEnvelope
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("json.Unmarshal(response) error = %v", err)
		}
		if response.ViewID != "ext.notes.recent" || response.Revision != "vr_1" ||
			len(response.Payload.Sections) != 1 {
			t.Fatalf("response = %#v, want validated view envelope", response)
		}
		if registry.viewWorkspace != "workspace-canonical" || registry.viewID != "ext.notes.recent" {
			t.Fatalf(
				"OpenSource() scope = %q/%q, want canonical workspace and view",
				registry.viewWorkspace,
				registry.viewID,
			)
		}
	})

	t.Run("Should guard view stream cursors and reset an epoch mismatch", func(t *testing.T) {
		t.Parallel()
		registry := &cmdPaletteRegistryStub{viewSnapshot: cmdpalette.ViewSnapshot{
			Descriptor:  cmdpalette.ViewDescriptor{ID: "ext.notes.recent", Kind: cmdpalette.ViewKindList},
			Payload:     cmdpalette.ViewPayload{View: cmdpalette.ViewContractVersion},
			Revision:    "vr_current",
			StreamEpoch: "vse_current",
		}}
		handlers := newCmdPaletteHandlers(registry, nil)
		engine := gin.New()
		engine.GET("/api/cmd-palette/views/:id/stream", handlers.StreamCmdPaletteView)

		guarded := httptest.NewRecorder()
		engine.ServeHTTP(guarded, httptest.NewRequestWithContext(
			t.Context(), http.MethodGet,
			"/api/cmd-palette/views/ext.notes.recent/stream?workspace=alpha&after=4", http.NoBody,
		))
		if guarded.Code != http.StatusBadRequest {
			t.Fatalf("guard status = %d, want 400; body=%s", guarded.Code, guarded.Body.String())
		}

		reset := httptest.NewRecorder()
		engine.ServeHTTP(reset, httptest.NewRequestWithContext(
			t.Context(), http.MethodGet,
			"/api/cmd-palette/views/ext.notes.recent/stream?workspace=alpha&after=4&stream_epoch=vse_stale",
			http.NoBody,
		))
		if reset.Code != http.StatusOK || !strings.Contains(reset.Body.String(), "event: cmd_palette.view.reset") {
			t.Fatalf("reset stream = status %d body %q, want full reset event", reset.Code, reset.Body.String())
		}
		if !strings.Contains(reset.Body.String(), `"stream_epoch":"vse_current"`) ||
			!strings.Contains(reset.Body.String(), `"revision":"vr_current"`) {
			t.Fatalf("reset stream body = %q, want current fence", reset.Body.String())
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
	snapshot         cmdpalette.Snapshot
	summary          cmdpalette.PersonalizationSummary
	usage            cmdpalette.Usage
	pinWorkspace     cmdpalette.WorkspaceID
	pinCommand       cmdpalette.CommandID
	pinned           bool
	resetWorkspace   cmdpalette.WorkspaceID
	viewSnapshot     cmdpalette.ViewSnapshot
	viewWorkspace    cmdpalette.WorkspaceID
	viewID           string
	viewEvents       <-chan cmdpalette.ViewPatchEvent
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

func (s *cmdPaletteRegistryStub) RecordUsage(_ context.Context, usage cmdpalette.Usage) error {
	s.usage = usage
	return nil
}

func (s *cmdPaletteRegistryStub) Personalization(
	context.Context,
	cmdpalette.WorkspaceID,
) (cmdpalette.Snapshot, error) {
	return s.snapshot, nil
}

func (s *cmdPaletteRegistryStub) PersonalizationSummary(
	context.Context,
	cmdpalette.WorkspaceID,
) (cmdpalette.PersonalizationSummary, error) {
	return s.summary, nil
}

func (s *cmdPaletteRegistryStub) ResetPersonalization(
	_ context.Context,
	workspaceID cmdpalette.WorkspaceID,
) error {
	s.resetWorkspace = workspaceID
	return nil
}

func (s *cmdPaletteRegistryStub) Pin(
	_ context.Context,
	workspaceID cmdpalette.WorkspaceID,
	commandID cmdpalette.CommandID,
) error {
	s.pinWorkspace = workspaceID
	s.pinCommand = commandID
	s.pinned = true
	return nil
}

func (s *cmdPaletteRegistryStub) Unpin(
	_ context.Context,
	workspaceID cmdpalette.WorkspaceID,
	commandID cmdpalette.CommandID,
) error {
	s.pinWorkspace = workspaceID
	s.pinCommand = commandID
	s.pinned = false
	return nil
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

func (s *cmdPaletteRegistryStub) ResolveView(
	_ context.Context,
	workspaceID cmdpalette.WorkspaceID,
	viewID string,
) (cmdpalette.ViewDescriptor, error) {
	s.viewWorkspace = workspaceID
	s.viewID = viewID
	if s.viewSnapshot.Descriptor.ID == "" {
		return cmdpalette.ViewDescriptor{}, &cmdpalette.ViewNotFoundError{ViewID: viewID}
	}
	return s.viewSnapshot.Descriptor, nil
}

func (s *cmdPaletteRegistryStub) OpenSource(
	_ context.Context,
	workspaceID cmdpalette.WorkspaceID,
	viewID string,
) (cmdpalette.ViewSnapshot, error) {
	s.viewWorkspace = workspaceID
	s.viewID = viewID
	if s.viewSnapshot.Descriptor.ID == "" {
		return cmdpalette.ViewSnapshot{}, &cmdpalette.ViewNotFoundError{ViewID: viewID}
	}
	return s.viewSnapshot, nil
}

func (s *cmdPaletteRegistryStub) SubscribeViewPatches(
	context.Context,
	cmdpalette.WorkspaceID,
	string,
) (<-chan cmdpalette.ViewPatchEvent, func(), error) {
	if s.viewEvents != nil {
		return s.viewEvents, func() {}, nil
	}
	events := make(chan cmdpalette.ViewPatchEvent)
	close(events)
	return events, func() {}, nil
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
