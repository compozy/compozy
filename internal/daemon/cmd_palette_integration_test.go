//go:build integration

package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/testutil"
	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/cmdpalette/corecmds"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/compozy/compozy/internal/windowmanager"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
	"github.com/gin-gonic/gin"
)

// Suite: command-palette daemon composition.
// Invariant: the real registry, WindowManager authority, HTTP contract, tool executor, and policy gate
// agree on canonical workspace, client context, authorization, arguments, and terminal envelopes.
// Owning layer: daemon composition. Canonical suite: this file.
func TestCmdPaletteDaemonIntegration(t *testing.T) {
	t.Parallel()

	t.Run("Should serve the seeded core and extension catalog with structural metadata [IT-001]", func(t *testing.T) {
		t.Parallel()
		coreProvider, err := corecmds.New()
		if err != nil {
			t.Fatalf("corecmds.New() error = %v", err)
		}
		extensionDescriptor := integrationToolDescriptor()
		registry, err := cmdpalette.NewRegistry(
			[]cmdpalette.ProviderRegistration{
				{Source: cmdpalette.Source{Kind: cmdpalette.SourceKindCore}, Provider: coreProvider},
				{
					Source: extensionDescriptor.Source,
					Provider: cmdPaletteIntegrationStaticProvider{
						commands: []cmdpalette.Descriptor{extensionDescriptor},
					},
				},
			},
			nil,
			cmdPaletteIntegrationBindings{},
			&cmdPaletteIntegrationExecutor{},
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		response := performCmdPaletteIntegrationRequest(
			t,
			newCmdPaletteIntegrationEngine(registry),
			http.MethodGet,
			"/api/cmd-palette/commands?workspace=acme",
			"",
			"",
		)
		if response.Code != http.StatusOK {
			t.Fatalf("catalog status = %d, want 200; body=%s", response.Code, response.Body.String())
		}
		var catalog contract.CmdPaletteCommandsResponse
		decodeCmdPaletteIntegrationResponse(t, response, &catalog)
		if len(catalog.Commands) != len(coreProvider.StaticCommands())+1 || catalog.CatalogRevision == "" ||
			len(catalog.Sources) != 2 {
			t.Fatalf(
				"catalog summary = commands %d revision %q sources %#v",
				len(catalog.Commands),
				catalog.CatalogRevision,
				catalog.Sources,
			)
		}
		var extensionCommand *contract.CmdPaletteCommand
		for index := range catalog.Commands {
			if catalog.Commands[index].ID == extensionDescriptor.ID {
				extensionCommand = &catalog.Commands[index]
				break
			}
		}
		if extensionCommand == nil || len(extensionCommand.Bindings) != 1 ||
			extensionCommand.Bindings[0] != "meta+shift+KeyN" || extensionCommand.Alias == nil ||
			*extensionCommand.Alias != "capture-note" || len(extensionCommand.Arguments) != 1 {
			t.Fatalf("extension command = %#v", extensionCommand)
		}
	})

	t.Run(
		"Should invoke form-mapped and inline tool arguments through the real HTTP registry [IT-005][IT-007][IT-009]",
		func(t *testing.T) {
			t.Parallel()
			toolRegistry := &recordingCmdPaletteToolRegistry{result: toolspkg.ToolResult{
				Structured: json.RawMessage(`{"title":"Standup follow-ups"}`),
			}}
			descriptor := integrationToolDescriptor()
			registry := newCmdPaletteIntegrationRegistry(t, descriptor, nil, &cmdPaletteActionExecutor{
				tools: toolRegistry, now: time.Now, approvalTTL: time.Minute,
			})
			engine := newCmdPaletteIntegrationEngine(registry)

			response := performCmdPaletteIntegrationRequest(
				t,
				engine,
				http.MethodPost,
				"/api/cmd-palette/commands/ext.notes.capture/invoke",
				`{"workspace":"acme","args":{"title":"Standup follow-ups"}}`,
				"",
			)
			if response.Code != http.StatusOK {
				t.Fatalf("invoke status = %d, want 200; body=%s", response.Code, response.Body.String())
			}
			var result contract.CmdPaletteInvokeResult
			decodeCmdPaletteIntegrationResponse(t, response, &result)
			if result.Status != cmdpalette.InvokeStatusOK || string(result.Result) != `{"title":"Standup follow-ups"}` {
				t.Fatalf("invoke result = %#v", result)
			}
			var input map[string]any
			if err := json.Unmarshal(toolRegistry.call.Input, &input); err != nil {
				t.Fatalf("json.Unmarshal(tool input) error = %v", err)
			}
			if input["title"] != "Standup follow-ups" {
				t.Fatalf("tool input = %#v", input)
			}

			invalid := performCmdPaletteIntegrationRequest(
				t,
				engine,
				http.MethodPost,
				"/api/cmd-palette/commands/ext.notes.capture/invoke",
				`{"workspace":"acme","args":{}}`,
				"",
			)
			var invalidPayload contract.CmdPaletteError
			decodeCmdPaletteIntegrationResponse(t, invalid, &invalidPayload)
			if invalid.Code != http.StatusUnprocessableEntity || invalidPayload.Error != "invalid_arguments" ||
				invalidPayload.Fields["title"] != "required" {
				t.Fatalf("invalid response = status %d payload %#v", invalid.Code, invalidPayload)
			}

			unknown := performCmdPaletteIntegrationRequest(
				t,
				engine,
				http.MethodPost,
				"/api/cmd-palette/commands/ext.notes.missing/invoke",
				`{"workspace":"acme","args":{}}`,
				"",
			)
			var unknownPayload contract.CmdPaletteError
			decodeCmdPaletteIntegrationResponse(t, unknown, &unknownPayload)
			if unknown.Code != http.StatusNotFound || unknownPayload.Error != "command_not_found" {
				t.Fatalf("unknown response = status %d payload %#v", unknown.Code, unknownPayload)
			}
		},
	)

	t.Run("Should target divergent clients and reject forged or stale identity [IT-031]", func(t *testing.T) {
		t.Parallel()
		manager := newCmdPaletteIntegrationWindowManager(t)
		clientA := windowmanager.ClientID("client-a")
		clientB := windowmanager.ClientID("client-b")
		registeredA, err := manager.RegisterClient(t.Context(), windowmanager.ClientRegistration{
			WorkspaceID: "workspace-acme", ClientID: clientA, Kind: windowmanager.ClientKindShell,
			Context: windowmanager.ClientContextInput{WorkspaceTrusted: true},
		})
		if err != nil {
			t.Fatalf("RegisterClient(A) error = %v", err)
		}
		if _, err := manager.Execute(t.Context(), windowmanager.CommandRequest{
			WorkspaceID: "workspace-acme", ExpectedRevision: 0, ClientID: &clientA,
			Payload: windowmanager.OpenWindowCommand{Window: windowmanager.WindowSpec{
				ID:        "window-a",
				App:       "sessions",
				DesktopID: "desktop-default",
				Route: windowmanager.RouteIntent{
					Pathname: "/sessions/session-a",
					Search:   windowmanager.RouteSearch{},
				},
				FloatingRect: windowmanager.NormalizedRect{X: 0.1, Y: 0.1, Width: 0.5, Height: 0.5},
			}},
		}); err != nil {
			t.Fatalf("Execute(window.open) error = %v", err)
		}
		if _, err := manager.Execute(t.Context(), windowmanager.CommandRequest{
			WorkspaceID: "workspace-acme", ExpectedRevision: 1,
			Payload: windowmanager.CreateDesktopCommand{
				DesktopID: "desktop-empty", Name: "Empty", Purpose: windowmanager.DesktopPurposeStandard,
			},
		}); err != nil {
			t.Fatalf("Execute(desktop.create) error = %v", err)
		}
		if _, err := manager.RegisterClient(t.Context(), windowmanager.ClientRegistration{
			WorkspaceID: "workspace-acme", ClientID: clientB, Kind: windowmanager.ClientKindBrowser,
			ActiveDesktopID: "desktop-empty",
			Context:         windowmanager.ClientContextInput{WorkspaceTrusted: true},
		}); err != nil {
			t.Fatalf("RegisterClient(B) error = %v", err)
		}

		executor := &cmdPaletteIntegrationExecutor{result: json.RawMessage(`{"closed":true}`)}
		descriptor := integrationFocusedClientDescriptor()
		registry := newCmdPaletteIntegrationRegistry(
			t,
			descriptor,
			&cmdPaletteClientDirectory{windowManager: manager},
			executor,
		)
		engine := newCmdPaletteIntegrationEngine(registry)

		catalogA := getCmdPaletteIntegrationCatalog(t, engine, "client-a")
		catalogB := getCmdPaletteIntegrationCatalog(t, engine, "client-b")
		if catalogA.CatalogRevision != catalogB.CatalogRevision ||
			len(catalogA.Commands) != 1 || len(catalogB.Commands) != 1 ||
			!catalogA.Commands[0].Available || catalogB.Commands[0].Available ||
			catalogB.Commands[0].Reason != "requires a focused window" {
			t.Fatalf("divergent catalogs = A %#v / B %#v", catalogA, catalogB)
		}

		multiple := invokeCmdPaletteIntegration(t, engine, "", "")
		var multiplePayload contract.CmdPaletteError
		decodeCmdPaletteIntegrationResponse(t, multiple, &multiplePayload)
		if multiple.Code != http.StatusConflict || multiplePayload.Error != "multiple_clients" ||
			len(multiplePayload.Clients) != 2 {
			t.Fatalf("multiple clients = status %d payload %#v", multiple.Code, multiplePayload)
		}

		unavailable := invokeCmdPaletteIntegration(t, engine, "client-b", "")
		var unavailablePayload contract.CmdPaletteError
		decodeCmdPaletteIntegrationResponse(t, unavailable, &unavailablePayload)
		if unavailable.Code != http.StatusPreconditionFailed ||
			unavailablePayload.Reason != catalogB.Commands[0].Reason {
			t.Fatalf("client B invoke = status %d payload %#v", unavailable.Code, unavailablePayload)
		}

		forged := invokeCmdPaletteIntegration(t, engine, "client-a", "forged-token")
		var forgedPayload contract.CmdPaletteError
		decodeCmdPaletteIntegrationResponse(t, forged, &forgedPayload)
		if forged.Code != http.StatusUnauthorized || forgedPayload.Error != "client_unauthorized" {
			t.Fatalf("forged invoke = status %d payload %#v", forged.Code, forgedPayload)
		}

		controlPlane := invokeCmdPaletteIntegration(t, engine, "client-a", "")
		if controlPlane.Code != http.StatusOK || executor.last().ClientID != "client-a" {
			t.Fatalf("control-plane invoke = status %d request %#v", controlPlane.Code, executor.last())
		}
		if registeredA.AttachmentToken == "" {
			t.Fatal("client A registration did not mint an attachment token")
		}

		if err := manager.UnregisterClient(t.Context(), "workspace-acme", clientB); err != nil {
			t.Fatalf("UnregisterClient(B) error = %v", err)
		}
		clients := performCmdPaletteIntegrationRequest(
			t,
			engine,
			http.MethodGet,
			"/api/cmd-palette/clients?workspace=acme",
			"",
			"",
		)
		var listed []contract.CmdPaletteClient
		decodeCmdPaletteIntegrationResponse(t, clients, &listed)
		stale := invokeCmdPaletteIntegration(t, engine, "client-b", "")
		var stalePayload contract.CmdPaletteError
		decodeCmdPaletteIntegrationResponse(t, stale, &stalePayload)
		if len(listed) != 1 || listed[0].ClientID != "client-a" ||
			stale.Code != http.StatusPreconditionFailed || stalePayload.Error != "no_attached_shell" {
			t.Fatalf("stale client = listed %#v status %d payload %#v", listed, stale.Code, stalePayload)
		}
	})

	t.Run("Should resolve public refs and isolate workspace catalogs [IT-032]", func(t *testing.T) {
		t.Parallel()
		provider := cmdPaletteWorkspaceProvider{commands: map[cmdpalette.WorkspaceID][]cmdpalette.Descriptor{
			"workspace-a": {integrationWorkspaceDescriptor("alpha")},
			"workspace-b": {integrationWorkspaceDescriptor("beta")},
		}}
		registry, err := cmdpalette.NewRegistry(
			[]cmdpalette.ProviderRegistration{{
				Source:   cmdpalette.Source{Kind: cmdpalette.SourceKindExtension, Extension: "fixture"},
				Provider: provider,
			}},
			nil,
			nil,
			&cmdPaletteIntegrationExecutor{},
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		engine := newCmdPaletteIntegrationEngineWithResolver(
			registry,
			func(ref string) (workspacepkg.ResolvedWorkspace, error) {
				switch ref {
				case "workspace-a", "acme", "/repo/acme":
					return workspacepkg.ResolvedWorkspace{
						Workspace:   workspacepkg.Workspace{ID: "workspace-a", Name: "acme"},
						WorkspaceID: "workspace-a",
					}, nil
				case "workspace-b", "beta", "/repo/beta":
					return workspacepkg.ResolvedWorkspace{
						Workspace:   workspacepkg.Workspace{ID: "workspace-b", Name: "beta"},
						WorkspaceID: "workspace-b",
					}, nil
				default:
					return workspacepkg.ResolvedWorkspace{}, workspacepkg.ErrWorkspaceNotFound
				}
			},
		)
		var revisionA string
		for _, ref := range []string{"workspace-a", "acme", "/repo/acme"} {
			response := performCmdPaletteIntegrationRequest(
				t,
				engine,
				http.MethodGet,
				"/api/cmd-palette/commands?workspace="+ref,
				"",
				"",
			)
			var catalog contract.CmdPaletteCommandsResponse
			decodeCmdPaletteIntegrationResponse(t, response, &catalog)
			if response.Code != http.StatusOK || len(catalog.Commands) != 1 ||
				catalog.Commands[0].ID != "ext.fixture.alpha" {
				t.Fatalf("workspace A ref %q = status %d catalog %#v", ref, response.Code, catalog)
			}
			if revisionA == "" {
				revisionA = catalog.CatalogRevision
			} else if catalog.CatalogRevision != revisionA {
				t.Fatalf("workspace A revision via %q = %q, want %q", ref, catalog.CatalogRevision, revisionA)
			}
		}
		responseB := performCmdPaletteIntegrationRequest(
			t,
			engine,
			http.MethodGet,
			"/api/cmd-palette/commands?workspace=beta",
			"",
			"",
		)
		var catalogB contract.CmdPaletteCommandsResponse
		decodeCmdPaletteIntegrationResponse(t, responseB, &catalogB)
		if responseB.Code != http.StatusOK || len(catalogB.Commands) != 1 ||
			catalogB.Commands[0].ID != "ext.fixture.beta" {
			t.Fatalf("workspace B catalog = status %d %#v", responseB.Code, catalogB)
		}
	})
}

type cmdPaletteIntegrationStaticProvider struct {
	commands []cmdpalette.Descriptor
}

type cmdPaletteWorkspaceProvider struct {
	commands map[cmdpalette.WorkspaceID][]cmdpalette.Descriptor
}

func (p cmdPaletteWorkspaceProvider) ProvideCommands(
	_ context.Context,
	workspaceID cmdpalette.WorkspaceID,
) ([]cmdpalette.Descriptor, error) {
	return append([]cmdpalette.Descriptor(nil), p.commands[workspaceID]...), nil
}

type cmdPaletteIntegrationBindings struct{}

func (cmdPaletteIntegrationBindings) Bindings(
	context.Context,
	cmdpalette.WorkspaceID,
) (map[cmdpalette.CommandID][]string, map[cmdpalette.CommandID]string, error) {
	return map[cmdpalette.CommandID][]string{
			"ext.notes.capture": {"meta+shift+KeyN"},
		}, map[cmdpalette.CommandID]string{
			"ext.notes.capture": "capture-note",
		}, nil
}

func (p cmdPaletteIntegrationStaticProvider) ProvideCommands(
	context.Context,
	cmdpalette.WorkspaceID,
) ([]cmdpalette.Descriptor, error) {
	return append([]cmdpalette.Descriptor(nil), p.commands...), nil
}

func (p cmdPaletteIntegrationStaticProvider) StaticCommands() []cmdpalette.Descriptor {
	return append([]cmdpalette.Descriptor(nil), p.commands...)
}

type cmdPaletteIntegrationExecutor struct {
	mu       sync.Mutex
	requests []cmdpalette.ExecutionRequest
	result   json.RawMessage
}

func (e *cmdPaletteIntegrationExecutor) ExecuteAction(
	_ context.Context,
	request cmdpalette.ExecutionRequest,
) (cmdpalette.ExecutionResult, error) {
	e.mu.Lock()
	e.requests = append(e.requests, request)
	e.mu.Unlock()
	return cmdpalette.ExecutionResult{Result: append(json.RawMessage(nil), e.result...)}, nil
}

func (e *cmdPaletteIntegrationExecutor) last() cmdpalette.ExecutionRequest {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.requests) == 0 {
		return cmdpalette.ExecutionRequest{}
	}
	return e.requests[len(e.requests)-1]
}

func integrationToolDescriptor() cmdpalette.Descriptor {
	return cmdpalette.Descriptor{
		ID: "ext.notes.capture", Title: "Capture note", Section: "Notes", Icon: "notebook-pen",
		Source:    cmdpalette.Source{Kind: cmdpalette.SourceKindExtension, Extension: "notes"},
		Action:    cmdpalette.Action{Kind: cmdpalette.ActionKindTool, Tool: "compozy__notes_capture"},
		Arguments: []cmdpalette.Argument{{Name: "title", Type: cmdpalette.ArgumentTypeText, Required: true}},
		Policy:    cmdpalette.ExecutionPolicy{RetrySafe: true},
	}
}

func integrationFocusedClientDescriptor() cmdpalette.Descriptor {
	return cmdpalette.Descriptor{
		ID: "window.close", Title: "Close window", Section: "Window", Icon: "x-square",
		Source:    cmdpalette.Source{Kind: cmdpalette.SourceKindCore},
		Action:    cmdpalette.Action{Kind: cmdpalette.ActionKindClientOp, Op: "window.close"},
		Arguments: []cmdpalette.Argument{},
		When: []cmdpalette.Predicate{{
			Key: cmdpalette.ContextWindowFocused, Value: true, Reason: "requires a focused window",
		}},
		Policy: cmdpalette.ExecutionPolicy{SingleFlight: true},
	}
}

func integrationWorkspaceDescriptor(suffix string) cmdpalette.Descriptor {
	return cmdpalette.Descriptor{
		ID: cmdpalette.CommandID("ext.fixture." + suffix), Title: suffix,
		Section: "Fixture", Icon: "box",
		Source:    cmdpalette.Source{Kind: cmdpalette.SourceKindExtension, Extension: "fixture"},
		Action:    cmdpalette.Action{Kind: cmdpalette.ActionKindTool, Tool: "compozy__fixture"},
		Arguments: []cmdpalette.Argument{}, Policy: cmdpalette.ExecutionPolicy{RetrySafe: true},
	}
}

func newCmdPaletteIntegrationRegistry(
	t *testing.T,
	descriptor cmdpalette.Descriptor,
	clients cmdpalette.ClientDirectory,
	executor cmdpalette.ActionExecutor,
) *cmdpalette.Service {
	t.Helper()
	registry, err := cmdpalette.NewRegistry(
		[]cmdpalette.ProviderRegistration{{
			Source:   descriptor.Source,
			Provider: cmdPaletteIntegrationStaticProvider{commands: []cmdpalette.Descriptor{descriptor}},
		}},
		clients,
		nil,
		executor,
		cmdpalette.WithInvocationIDGenerator(func() string { return "invocation-integration" }),
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}

func newCmdPaletteIntegrationEngine(registry cmdpalette.Registry) *gin.Engine {
	return newCmdPaletteIntegrationEngineWithResolver(
		registry,
		func(ref string) (workspacepkg.ResolvedWorkspace, error) {
			return workspacepkg.ResolvedWorkspace{
				Workspace:   workspacepkg.Workspace{ID: "workspace-acme", Name: ref},
				WorkspaceID: "workspace-acme",
			}, nil
		},
	)
}

func newCmdPaletteIntegrationEngineWithResolver(
	registry cmdpalette.Registry,
	resolve func(string) (workspacepkg.ResolvedWorkspace, error),
) *gin.Engine {
	handlers := core.NewBaseHandlers(&core.BaseHandlerConfig{
		CmdPalette: registry,
		Workspaces: testutil.StubWorkspaceService{ResolveFn: func(
			_ context.Context,
			ref string,
		) (workspacepkg.ResolvedWorkspace, error) {
			return resolve(ref)
		}},
	})
	engine := gin.New()
	engine.GET("/api/cmd-palette/commands", handlers.ListCmdPaletteCommands)
	engine.GET("/api/cmd-palette/clients", handlers.ListCmdPaletteClients)
	engine.POST("/api/cmd-palette/commands/:id/invoke", handlers.InvokeCmdPaletteCommand)
	return engine
}

func newCmdPaletteIntegrationWindowManager(t *testing.T) *windowmanager.Manager {
	t.Helper()
	manager, err := windowmanager.NewService(
		windowmanager.NewMemoryRepository(),
		windowmanager.NewMemoryWorkspaceResolver("workspace-acme"),
		nil,
		windowmanager.DefaultConfig(),
	)
	if err != nil {
		t.Fatalf("windowmanager.NewService() error = %v", err)
	}
	t.Cleanup(func() {
		if err := manager.Close(); err != nil {
			t.Errorf("Manager.Close() error = %v", err)
		}
	})
	return manager
}

func performCmdPaletteIntegrationRequest(
	t *testing.T,
	engine http.Handler,
	method string,
	path string,
	body string,
	token string,
) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	request.Header.Set("Content-Type", "application/json")
	if token != "" {
		request.Header.Set("X-Compozy-Client-Token", token)
	}
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)
	return response
}

func decodeCmdPaletteIntegrationResponse(t *testing.T, response *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", response.Body.String(), err)
	}
}

func getCmdPaletteIntegrationCatalog(
	t *testing.T,
	engine http.Handler,
	clientID string,
) contract.CmdPaletteCommandsResponse {
	t.Helper()
	response := performCmdPaletteIntegrationRequest(
		t,
		engine,
		http.MethodGet,
		"/api/cmd-palette/commands?workspace=acme&client="+clientID,
		"",
		"",
	)
	if response.Code != http.StatusOK {
		t.Fatalf("catalog status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	var catalog contract.CmdPaletteCommandsResponse
	decodeCmdPaletteIntegrationResponse(t, response, &catalog)
	return catalog
}

func invokeCmdPaletteIntegration(
	t *testing.T,
	engine http.Handler,
	clientID string,
	token string,
) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(contract.CmdPaletteInvokeRequest{
		Workspace: "acme", Args: map[string]any{}, Client: clientID,
	})
	if err != nil {
		t.Fatalf("json.Marshal(invoke) error = %v", err)
	}
	return performCmdPaletteIntegrationRequest(
		t,
		engine,
		http.MethodPost,
		"/api/cmd-palette/commands/window.close/invoke",
		string(body),
		token,
	)
}
