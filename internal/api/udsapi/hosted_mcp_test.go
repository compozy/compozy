package udsapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/gin-gonic/gin"
)

func TestHostedMCPStreamErrorData(t *testing.T) {
	t.Parallel()

	t.Run("Should emit stable stream error without raw backend details", func(t *testing.T) {
		t.Parallel()

		payload := hostedMCPStreamErrorData(
			fmt.Errorf("bind failed for compozy_claim_secret: %w", mcppkg.ErrHostedBindNotFound),
		)
		encoded, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("json.Marshal(hosted MCP stream error) error = %v", err)
		}
		if strings.Contains(string(encoded), "compozy_claim_secret") ||
			strings.Contains(string(encoded), "bind failed") {
			t.Fatalf("hosted MCP stream error payload leaked backend detail: %s", encoded)
		}
		if payload.Error != "hosted_mcp_projection_failed" ||
			payload.Status != http.StatusForbidden ||
			payload.Message != http.StatusText(http.StatusForbidden) {
			t.Fatalf("hosted MCP stream error payload = %#v, want stable forbidden error", payload)
		}
	})
}

func TestHostedMCPProjectionStreamGenerationCache(t *testing.T) {
	t.Parallel()

	t.Run("Should reuse one registry projection across stable stream polls", func(t *testing.T) {
		t.Parallel()

		registry, err := toolspkg.NewRegistry()
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		countingRegistry := &countingHostedMCPRegistry{Registry: registry}
		done := make(chan struct{})
		var projectionSamples atomic.Int32
		var closeDone sync.Once
		generation := func(context.Context, toolspkg.Scope) (string, bool) {
			if projectionSamples.Add(1) >= 5 {
				closeDone.Do(func() { close(done) })
			}
			return "stable", true
		}
		router, service, peer := newHostedMCPRouteTestHarnessWithConfig(t, mcppkg.HostedConfig{
			Registry: func() toolspkg.Registry {
				return countingRegistry
			},
			ProjectionGeneration: generation,
		}, &core.BaseHandlerConfig{
			PollInterval: time.Millisecond,
			StreamDone:   done,
		})
		bind := launchAndBindHostedMCP(t, service, peer, "sess-stream-cache")

		recorder := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(
			context.Background(),
			http.MethodGet,
			"/api/internal/hosted-mcp/projection/stream?bind_id="+bind.BindID,
			http.NoBody,
		)
		request = request.WithContext(mcppkg.ContextWithPeerInfo(request.Context(), peer, nil))
		router.ServeHTTP(recorder, request)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
		}
		if got, want := recorder.Header().Get("Content-Type"), "text/event-stream"; got != want {
			t.Fatalf("Content-Type = %q, want %q", got, want)
		}
		if got := projectionSamples.Load(); got < 5 {
			t.Fatalf("projection generation samples = %d, want at least 5", got)
		}
		if got, want := countingRegistry.listCalls.Load(), int32(1); got != want {
			t.Fatalf("registry List() calls = %d, want %d", got, want)
		}
		records := parseSSE(t, recorder.Body.String())
		if len(records) != 1 || records[0].Event != "projection" {
			t.Fatalf("stream records = %#v, want one projection event", records)
		}
	})
}

func TestHostedMCPJSONToolErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should return structured not found tool errors from hosted tool calls", func(t *testing.T) {
		t.Parallel()

		registry, err := toolspkg.NewRegistry()
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		router, service, peer := newHostedMCPRouteTestHarnessWithRegistry(t, registry)
		bind := launchAndBindHostedMCP(t, service, peer, "sess-tool-not-found")

		recorder := postHostedMCPToolCall(t, router, peer, bind.BindID, "compozy__missing_tool")

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotFound, recorder.Body.String())
		}
		payload := decodeHostedMCPToolError(t, recorder)
		if payload.Error.Code != toolspkg.ErrorCodeNotFound {
			t.Fatalf("error code = %q, want %q; payload=%#v", payload.Error.Code, toolspkg.ErrorCodeNotFound, payload)
		}
		if payload.Error.ToolID != "compozy__missing_tool" {
			t.Fatalf("tool_id = %q, want compozy__missing_tool; payload=%#v", payload.Error.ToolID, payload)
		}
		if !slices.Contains(payload.Error.ReasonCodes, toolspkg.ReasonToolUnknown) {
			t.Fatalf("reason_codes = %#v, want %q", payload.Error.ReasonCodes, toolspkg.ReasonToolUnknown)
		}
		if strings.Contains(recorder.Body.String(), "hosted-mcp backend error") {
			t.Fatalf("tool error payload used opaque backend fallback: %s", recorder.Body.String())
		}
	})

	t.Run("Should return structured denied tool errors from hosted tool calls", func(t *testing.T) {
		t.Parallel()

		registry := newDeniedHostedMCPToolRegistry(t)
		router, service, peer := newHostedMCPRouteTestHarnessWithRegistry(t, registry)
		bind := launchAndBindHostedMCP(t, service, peer, "sess-tool-denied")

		recorder := postHostedMCPToolCall(t, router, peer, bind.BindID, "compozy__hosted_denied")

		if recorder.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusForbidden, recorder.Body.String())
		}
		payload := decodeHostedMCPToolError(t, recorder)
		if payload.Error.Code != toolspkg.ErrorCodeDenied {
			t.Fatalf("error code = %q, want %q; payload=%#v", payload.Error.Code, toolspkg.ErrorCodeDenied, payload)
		}
		if payload.Error.ToolID != "compozy__hosted_denied" {
			t.Fatalf("tool_id = %q, want compozy__hosted_denied; payload=%#v", payload.Error.ToolID, payload)
		}
		if !slices.Contains(payload.Error.ReasonCodes, toolspkg.ReasonPolicyDenied) {
			t.Fatalf("reason_codes = %#v, want %q", payload.Error.ReasonCodes, toolspkg.ReasonPolicyDenied)
		}
		if strings.Contains(recorder.Body.String(), "hosted-mcp backend error") {
			t.Fatalf("tool error payload used opaque backend fallback: %s", recorder.Body.String())
		}
	})
}

func TestHostedMCPJSONRouteErrors(t *testing.T) {
	t.Parallel()

	t.Run("Should expose stable caller-correctable errors without backend details", func(t *testing.T) {
		t.Parallel()

		router, peer := newHostedMCPRouteTestHarness(t)
		for _, tt := range []struct {
			name       string
			method     string
			path       string
			body       string
			statusCode int
			wantError  string
		}{
			{
				name:       "Should report missing session id on bind",
				method:     http.MethodPost,
				path:       "/api/internal/hosted-mcp/bind",
				body:       `{}`,
				statusCode: http.StatusBadRequest,
				wantError:  "hosted_mcp_session_required",
			},
			{
				name:       "Should report missing bind id on projection",
				method:     http.MethodGet,
				path:       "/api/internal/hosted-mcp/projection?bind_id=",
				statusCode: http.StatusBadRequest,
				wantError:  "hosted_mcp_bind_required",
			},
			{
				name:       "Should report missing bind id on tool call",
				method:     http.MethodPost,
				path:       "/api/internal/hosted-mcp/tools/call",
				body:       `{"tool_name":"shell.exec"}`,
				statusCode: http.StatusBadRequest,
				wantError:  "hosted_mcp_bind_required",
			},
			{
				name:       "Should report missing bind id on release",
				method:     http.MethodPost,
				path:       "/api/internal/hosted-mcp/release",
				body:       `{}`,
				statusCode: http.StatusBadRequest,
				wantError:  "hosted_mcp_bind_required",
			},
		} {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				recorder := httptest.NewRecorder()
				request := httptest.NewRequestWithContext(
					context.Background(),
					tt.method,
					tt.path,
					bytes.NewBufferString(tt.body),
				)
				request = request.WithContext(mcppkg.ContextWithPeerInfo(request.Context(), peer, nil))
				router.ServeHTTP(recorder, request)

				if recorder.Code != tt.statusCode {
					t.Fatalf("status = %d, want %d; body=%s", recorder.Code, tt.statusCode, recorder.Body.String())
				}
				var payload contract.ErrorPayload
				if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
					t.Fatalf("Unmarshal(error payload) error = %v; body=%s", err, recorder.Body.String())
				}
				if !strings.Contains(payload.Error, tt.wantError) {
					t.Fatalf("error payload = %#v, want %q", payload, tt.wantError)
				}
				if strings.Contains(payload.Error, "hosted-mcp backend error") {
					t.Fatalf("error payload used opaque backend fallback for 4xx: %#v", payload)
				}
			})
		}
	})

	t.Run("Should keep unexpected backend errors redacted", func(t *testing.T) {
		t.Parallel()

		if got := hostedMCPSafeError().Error(); got != errHostedMCPBackend.Error() {
			t.Fatalf("hostedMCPSafeError().Error() = %q, want generic backend error", got)
		}
		if strings.Contains(hostedMCPSafeError().Error(), errors.New("database leaked compozy_claim_secret").Error()) {
			t.Fatal("hostedMCPSafeError leaked backend details")
		}
	})
}

func newHostedMCPRouteTestHarness(t *testing.T) (*gin.Engine, mcppkg.PeerInfo) {
	t.Helper()

	registry, err := toolspkg.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	router, _, peer := newHostedMCPRouteTestHarnessWithRegistry(t, registry)
	return router, peer
}

func newHostedMCPRouteTestHarnessWithRegistry(
	t *testing.T,
	registry toolspkg.Registry,
) (*gin.Engine, *mcppkg.HostedService, mcppkg.PeerInfo) {
	t.Helper()

	return newHostedMCPRouteTestHarnessWithConfig(t, mcppkg.HostedConfig{
		Registry: func() toolspkg.Registry {
			return registry
		},
	}, nil)
}

func newHostedMCPRouteTestHarnessWithConfig(
	t *testing.T,
	config mcppkg.HostedConfig,
	baseConfig *core.BaseHandlerConfig,
) (*gin.Engine, *mcppkg.HostedService, mcppkg.PeerInfo) {
	t.Helper()

	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("Executable() error = %v", err)
	}
	config.Enabled = true
	config.ExpectedBinary = executable
	service, err := mcppkg.NewHostedService(config)
	if err != nil {
		t.Fatalf("NewHostedService() error = %v", err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerHostedMCPRoutes(router.Group("/api"), &Handlers{
		BaseHandlers: core.NewBaseHandlers(baseConfig),
		HostedMCP:    service,
	})
	peer := mcppkg.PeerInfo{
		PID:            os.Getpid(),
		UID:            os.Getuid(),
		GID:            os.Getgid(),
		ExecutablePath: executable,
		Supported:      true,
	}
	return router, service, peer
}

type countingHostedMCPRegistry struct {
	toolspkg.Registry
	listCalls atomic.Int32
}

func (r *countingHostedMCPRegistry) List(
	ctx context.Context,
	scope toolspkg.Scope,
) ([]toolspkg.ToolView, error) {
	r.listCalls.Add(1)
	return r.Registry.List(ctx, scope)
}

func launchAndBindHostedMCP(
	t *testing.T,
	service *mcppkg.HostedService,
	peer mcppkg.PeerInfo,
	sessionID string,
) mcppkg.HostedBindResponse {
	t.Helper()

	launch, err := service.Launch(t.Context(), mcppkg.HostedLaunchRequest{
		SessionID:   sessionID,
		WorkspaceID: "ws-tool-errors",
		AgentName:   "coder",
	})
	if err != nil {
		t.Fatalf("HostedService.Launch() error = %v", err)
	}
	if err = service.ArmLaunch(t.Context(), sessionID); err != nil {
		t.Fatalf("HostedService.ArmLaunch() error = %v", err)
	}
	bind, err := service.Bind(t.Context(), mcppkg.HostedBindRequest{
		SessionID: sessionID,
		Nonce:     hostedMCPTestNonce(t, launch.Args),
	}, peer)
	if err != nil {
		t.Fatalf("HostedService.Bind() error = %v", err)
	}
	return bind
}

func hostedMCPTestNonce(t *testing.T, args []string) string {
	t.Helper()

	for i := range args {
		if args[i] == "--bind-nonce" && i+1 < len(args) {
			return args[i+1]
		}
	}
	t.Fatalf("launch args = %#v, want --bind-nonce", args)
	return ""
}

func postHostedMCPToolCall(
	t *testing.T,
	router *gin.Engine,
	peer mcppkg.PeerInfo,
	bindID string,
	toolName string,
) *httptest.ResponseRecorder {
	t.Helper()

	body := fmt.Sprintf(`{"bind_id":%q,"tool_name":%q,"input":{}}`, bindID, toolName)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodPost,
		"/api/internal/hosted-mcp/tools/call",
		bytes.NewBufferString(body),
	)
	request = request.WithContext(mcppkg.ContextWithPeerInfo(request.Context(), peer, nil))
	router.ServeHTTP(recorder, request)
	return recorder
}

func decodeHostedMCPToolError(t *testing.T, recorder *httptest.ResponseRecorder) contract.ToolErrorResponse {
	t.Helper()

	var payload contract.ToolErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("Unmarshal(tool error response) error = %v; body=%s", err, recorder.Body.String())
	}
	if payload.Error.Code == "" {
		t.Fatalf("tool error payload missing code: %#v; body=%s", payload, recorder.Body.String())
	}
	return payload
}

func newDeniedHostedMCPToolRegistry(t *testing.T) toolspkg.Registry {
	t.Helper()

	source := toolspkg.SourceRef{Kind: toolspkg.SourceBuiltin, Owner: toolspkg.BuiltinSourceOwner}
	descriptor := toolspkg.Descriptor{
		ID:           "compozy__hosted_denied",
		DisplayTitle: "Hosted Denied",
		Description:  "Denied hosted test tool.",
		InputSchema:  json.RawMessage(`{"type":"object","additionalProperties":false}`),
		OutputSchema: json.RawMessage(`{"type":"object","properties":{"ok":{"type":"boolean"}}}`),
		Backend: toolspkg.BackendRef{
			Kind:       toolspkg.BackendNativeGo,
			NativeName: "hosted_denied",
		},
		Source:          source,
		Visibility:      toolspkg.VisibilityModel,
		Risk:            toolspkg.RiskRead,
		ReadOnly:        true,
		ConcurrencySafe: true,
		Toolsets:        []toolspkg.ToolsetID{toolspkg.ToolsetIDCoordination},
	}
	provider, err := toolspkg.NewNativeProvider(source, toolspkg.NativeTool{
		Descriptor: descriptor,
		Call: func(
			context.Context,
			toolspkg.Scope,
			toolspkg.CallRequest,
		) (toolspkg.ToolResult, error) {
			return toolspkg.ToolResult{Structured: json.RawMessage(`{"ok":true}`)}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewNativeProvider() error = %v", err)
	}
	denyPattern, err := toolspkg.ParseToolPattern(descriptor.ID.String())
	if err != nil {
		t.Fatalf("ParseToolPattern() error = %v", err)
	}
	registry, err := toolspkg.NewRegistry(
		toolspkg.WithProviders(provider),
		toolspkg.WithPolicyInputs(toolspkg.PolicyInputs{
			SystemPermissionMode: toolspkg.PermissionModeApproveAll,
			ApprovalAvailable:    true,
			DenyTools:            []toolspkg.ToolPattern{denyPattern},
		}, toolspkg.ToolsetCatalog{}),
	)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	return registry
}
