package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/user"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/api/udsapi"
	compozyconfig "github.com/compozy/compozy/internal/config"
	mcppkg "github.com/compozy/compozy/internal/mcp"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/gin-gonic/gin"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBuildHostedMCPServiceLogsDisabledConfiguration(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		toolsEnabled     bool
		hostedMCPEnabled bool
		wantReason       string
	}{
		"tools disabled": {
			toolsEnabled:     false,
			hostedMCPEnabled: true,
			wantReason:       "tools_disabled",
		},
		"hosted MCP disabled": {
			toolsEnabled:     true,
			hostedMCPEnabled: false,
			wantReason:       "hosted_mcp_disabled",
		},
	}

	for name, testCase := range testCases {
		t.Run("Should log disabled hosted MCP gate for "+name, func(t *testing.T) {
			t.Parallel()

			cfg := compozyconfig.Config{Tools: compozyconfig.DefaultToolsConfig()}
			cfg.Tools.Enabled = testCase.toolsEnabled
			cfg.Tools.HostedMCPEnabled = testCase.hostedMCPEnabled
			var logOutput bytes.Buffer
			service, err := (&Daemon{}).buildHostedMCPService(&bootState{
				cfg:    cfg,
				logger: slog.New(slog.NewJSONHandler(&logOutput, nil)),
			})
			if err != nil {
				t.Fatalf("buildHostedMCPService() error = %v", err)
			}
			if service != nil {
				t.Fatalf("buildHostedMCPService() = %#v, want nil service", service)
			}

			got := logOutput.String()
			for _, want := range []string{
				`"msg":"daemon.hosted_mcp.disabled"`,
				`"reason":"` + testCase.wantReason + `"`,
				`"tools_enabled":` + boolString(testCase.toolsEnabled),
				`"hosted_mcp_enabled":` + boolString(testCase.hostedMCPEnabled),
			} {
				if !strings.Contains(got, want) {
					t.Fatalf("log output = %q, want substring %q", got, want)
				}
			}
		})
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

// Invariant: the daemon adapter identifies hosted readiness failures without reclassifying cancellation.
// Owner: daemon hosted MCP launcher adapter; canonical suite: hosted_mcp_test.go.
func TestHostedMCPLauncherReadiness(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		cause error
	}{
		{"Should identify an expired bind as a readiness failure", mcppkg.ErrHostedBindTimeout},
		{"Should identify a failed projection as a readiness failure", mcppkg.ErrHostedProjectionFailed},
		{"Should preserve cancellation without classifying it as a readiness failure", context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
			service, err := mcppkg.NewHostedService(&mcppkg.HostedConfig{
				Enabled:        true,
				ExpectedBinary: executable,
				BindNonceTTL:   time.Minute,
				Now:            func() time.Time { return now },
			})
			if err != nil {
				t.Fatal(err)
			}
			launcher := hostedMCPLauncherAdapter{service: service}
			const id = "session-hosted-readiness"
			launch, err := launcher.Launch(t.Context(), session.HostedMCPLaunchRequest{SessionID: id, EagerTools: true})
			if err != nil {
				t.Fatal(err)
			}
			if err := launcher.ArmLaunch(t.Context(), id); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch tc.cause {
			case mcppkg.ErrHostedBindTimeout:
				now = now.Add(time.Minute)
			case mcppkg.ErrHostedProjectionFailed:
				index := slices.Index(launch.Args, "--bind-nonce")
				if index < 0 || index+1 >= len(launch.Args) {
					t.Fatal("hosted launch did not include a bind nonce")
				}
				currentUser, err := user.Current()
				if err != nil {
					t.Fatal(err)
				}
				peer := mcppkg.PeerInfo{
					PID: os.Getpid(), UID: os.Getuid(), GID: os.Getgid(), UserSID: currentUser.Uid,
					ExecutablePath: executable, Supported: true,
				}
				if _, err := service.Bind(
					ctx,
					mcppkg.HostedBindRequest{SessionID: id, Nonce: launch.Args[index+1]},
					peer,
				); !errors.Is(
					err,
					mcppkg.ErrHostedRegistryRequired,
				) {
					t.Fatalf("Bind() = %v, want missing projection registry", err)
				}
			case context.Canceled:
				cancel()
			}
			_, digestErr := launcher.WaitForBindToolDigest(ctx, id)
			for _, err := range []error{launcher.WaitForBind(ctx, id), digestErr} {
				_, readiness := errors.AsType[*session.HostedMCPReadinessError](err)
				if !errors.Is(err, tc.cause) || readiness != (tc.cause != context.Canceled) {
					t.Fatalf("readiness = %v (classified %t), want %v", err, readiness, tc.cause)
				}
			}
		})
	}
}

func TestHostedMCPSkillView(t *testing.T) {
	t.Parallel()

	t.Run("Should execute discoverable skill reads and identify a missing skill", func(t *testing.T) {
		t.Parallel()

		const sessionID, workspaceID = "sess-fixture", "ws-fixture"
		workspace := nativeTestWorkspaceService(t)
		sessions := &nativeSessionAgentManager{
			StubSessionManager: *nativeTestSessionManager(workspaceID),
			agent:              testPromptAgent("Read fixture skills."),
		}
		registry := newDaemonNativeRegistry(t, &daemonNativeToolsDeps{
			Skills: newLoadedNativeSkillRegistry(t), Sessions: sessions,
			WorkspaceResolver: workspace, Workspaces: workspace,
		}, nativeApproveAllPolicyInputs())
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		currentUser, err := user.Current()
		if err != nil {
			t.Fatal(err)
		}
		service, err := mcppkg.NewHostedService(&mcppkg.HostedConfig{
			Enabled: true, ExpectedBinary: executable, BindNonceTTL: time.Minute,
			Registry: func() toolspkg.Registry { return registry },
		})
		if err != nil {
			t.Fatal(err)
		}
		launch, err := service.Launch(t.Context(), mcppkg.HostedLaunchRequest{
			SessionID: sessionID, ProfileID: store.DefaultProfileID, WorkspaceID: workspaceID,
			AgentName: sessions.agent.Name, EagerTools: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := service.ArmLaunch(t.Context(), sessionID); err != nil {
			t.Fatal(err)
		}
		index := slices.Index(launch.Args, "--bind-nonce")
		if index < 0 || index+1 >= len(launch.Args) {
			t.Fatal("hosted launch did not include a bind nonce")
		}
		router := gin.New()
		udsapi.RegisterRoutes(router, &udsapi.Handlers{
			BaseHandlers: core.NewBaseHandlers(&core.BaseHandlerConfig{}),
			HostedMCP:    service,
		})
		client := connectHostedSkillTestProxy(t, &hostedSkillTestClient{
			service: service, router: router,
			peer: mcppkg.PeerInfo{
				PID: os.Getpid(), UID: os.Getuid(), GID: os.Getgid(), UserSID: currentUser.Uid,
				ExecutablePath: executable, Supported: true,
			},
		}, sessionID, launch.Args[index+1])
		call := func(id toolspkg.ToolID, arguments map[string]string) *sdkmcp.CallToolResult {
			t.Helper()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			result, err := client.CallTool(ctx, &sdkmcp.CallToolParams{Name: id.String(), Arguments: arguments})
			if err != nil {
				t.Fatalf("CallTool(%s): %v", id, err)
			}
			return result
		}
		search := call(toolspkg.ToolIDToolSearch, map[string]string{"query": toolspkg.ToolIDSkillView.String()})
		var page compactToolSearchPage
		decodeHostedSkillTestContent(t, search.StructuredContent, &page)
		if search.IsError || !slices.ContainsFunc(page.Tools, func(view compactToolSearchView) bool {
			return view.Descriptor.ID == toolspkg.ToolIDSkillView && view.Decision.Callable
		}) {
			t.Fatalf("tool_search did not report skill_view callable: %+v", page)
		}

		known := call(toolspkg.ToolIDSkillView, map[string]string{"name": "compozy"})
		var skill struct {
			Skill   contract.SkillPayload `json:"skill"`
			Content string                `json:"content"`
		}
		decodeHostedSkillTestContent(t, known.StructuredContent, &skill)
		if known.IsError || skill.Skill.Name != "compozy" || skill.Content == "" {
			t.Fatal("skill_view did not return the bundled compozy skill")
		}

		missing := call(toolspkg.ToolIDSkillView, map[string]string{"name": "missing-fixture"})
		const wantMessage = `skill not found: "missing-fixture"`
		var payload contract.ToolErrorResponse
		decodeHostedSkillTestContent(t, missing.StructuredContent, &payload)
		if !missing.IsError || payload.Error.Code != toolspkg.ErrorCodeNotFound ||
			payload.Error.ToolID != toolspkg.ToolIDSkillView || payload.Error.Message != wantMessage ||
			slices.Contains(payload.Error.ReasonCodes, toolspkg.ReasonToolUnknown) {
			t.Fatalf("missing skill payload = %+v, want a named skill lookup failure", payload.Error)
		}
		if len(missing.Content) == 0 {
			t.Fatal("missing skill response did not include display text")
		}
		display, ok := missing.Content[0].(*sdkmcp.TextContent)
		if !ok || !strings.Contains(display.Text, wantMessage) ||
			strings.Contains(display.Text, string(toolspkg.ReasonToolUnknown)) {
			t.Fatalf("missing skill display = %+v, want the named skill lookup failure", missing.Content[0])
		}
	})
}

func connectHostedSkillTestProxy(
	t *testing.T, client mcppkg.HostedProxyClient, sessionID, nonce string,
) *sdkmcp.ClientSession {
	t.Helper()

	clientConn, proxyConn := net.Pipe()
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	done := make(chan struct{})
	go func() {
		defer close(done)
		err := mcppkg.RunHostedProxy(ctx, client, mcppkg.HostedProxyOptions{
			SessionID: sessionID, Nonce: nonce, Stdin: proxyConn, Stdout: proxyConn, Version: "test",
		})
		if err != nil && ctx.Err() == nil {
			t.Errorf("RunHostedProxy(): %v", err)
		}
	}()
	t.Cleanup(func() {
		cancel()
		if err := clientConn.Close(); err != nil {
			t.Error(err)
		}
		if err := proxyConn.Close(); err != nil {
			t.Error(err)
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("timed out waiting for hosted proxy shutdown")
		}
	})
	sdkClient := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "hosted-skill-test", Version: "test"}, nil)
	clientSession, err := sdkClient.Connect(ctx, &sdkmcp.IOTransport{Reader: clientConn, Writer: clientConn}, nil)
	if err != nil {
		t.Fatalf("connect to hosted proxy: %v", err)
	}
	return clientSession
}

func decodeHostedSkillTestContent(t *testing.T, content any, value any) {
	t.Helper()
	raw, err := json.Marshal(content)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, value); err != nil {
		t.Fatal(err)
	}
}

type hostedSkillTestClient struct {
	service *mcppkg.HostedService
	router  http.Handler
	peer    mcppkg.PeerInfo
}

func (c *hostedSkillTestClient) BindHostedMCP(
	ctx context.Context, req mcppkg.HostedBindRequest,
) (mcppkg.HostedBindResponse, error) {
	return c.service.Bind(ctx, req, c.peer)
}

func (c *hostedSkillTestClient) HostedMCPProjection(
	ctx context.Context,
	bindID string,
) (mcppkg.HostedProjectionResponse, error) {
	return c.service.Projection(ctx, bindID, c.peer)
}

func (c *hostedSkillTestClient) StreamHostedMCPProjection(
	ctx context.Context, _, _ string, _ mcppkg.HostedProjectionHandler,
) error {
	<-ctx.Done()
	return ctx.Err()
}

func (c *hostedSkillTestClient) ReportHostedMCPServed(ctx context.Context, req mcppkg.HostedServedRequest) error {
	return c.service.ReportServedCatalog(ctx, req, c.peer)
}

func (c *hostedSkillTestClient) ReleaseHostedMCP(ctx context.Context, req mcppkg.HostedReleaseRequest) error {
	return c.service.ReleaseBindForPeer(ctx, req.BindID, c.peer)
}

func (c *hostedSkillTestClient) CallHostedMCP(
	ctx context.Context, req mcppkg.HostedCallRequest,
) (mcppkg.HostedCallResponse, error) {
	var result mcppkg.HostedCallResponse
	body, err := json.Marshal(req)
	if err != nil {
		return result, err
	}
	ctx = mcppkg.ContextWithPeerInfo(ctx, c.peer, nil)
	request := httptest.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"/api/internal/hosted-mcp/tools/call",
		bytes.NewReader(body),
	)
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	c.router.ServeHTTP(recorder, request)
	switch recorder.Code {
	case http.StatusOK:
		err := json.Unmarshal(recorder.Body.Bytes(), &result)
		return result, err
	case http.StatusNotFound:
		var payload contract.ToolErrorResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			return result, err
		}
		return result, hostedSkillTestResponseError{payload}
	default:
		return result, fmt.Errorf("hosted tool call status %d: %s", recorder.Code, recorder.Body.String())
	}
}

type hostedSkillTestResponseError struct{ payload contract.ToolErrorResponse }

func (e hostedSkillTestResponseError) Error() string                        { return e.payload.Error.Message }
func (e hostedSkillTestResponseError) Response() contract.ToolErrorResponse { return e.payload }
