package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	mcpauth "github.com/compozy/agh/internal/mcp/auth"
	toolspkg "github.com/compozy/agh/internal/tools"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	mcpsdk "github.com/mark3labs/mcp-go/mcp"
	mcpsrv "github.com/mark3labs/mcp-go/server"
)

const (
	stdioHelperEnv            = "AGH_MCP_STDIO_HELPER"
	stdioEnvHelperEnv         = "AGH_MCP_STDIO_ENV_HELPER"
	stdioFailedHelperEnv      = "AGH_MCP_STDIO_FAILED_HELPER"
	stdioParentSecretEnv      = "AGH_PARENT_SECRET_TOKEN"
	stdioExplicitSecretEnv    = "AGH_EXPLICIT_SECRET_TOKEN"
	stdioExplicitSecretSource = "AGH_EXPLICIT_SECRET_SOURCE"
)

func TestMCPCallExecutor(t *testing.T) {
	t.Run("Should List And Call Streamable HTTP Server", func(t *testing.T) {
		t.Parallel()

		testServer := mcpsrv.NewTestStreamableHTTPServer(newFakeSDKServer(nil))
		t.Cleanup(testServer.Close)
		executor := newTestMCPExecutor(t, aghconfig.MCPServer{
			Name:      "GitHub",
			Transport: aghconfig.MCPServerTransportHTTP,
			URL:       testServer.URL,
		})

		descriptor := requireMCPDescriptor(t, executor, "GitHub", "mcp__github__echo")
		result := callMCPTool(t, executor, descriptor, json.RawMessage(`{"message":"hello"}`))
		if got, want := result.Preview, "echo: hello"; got != want {
			t.Fatalf("result.Preview = %q, want %q", got, want)
		}
		requireJSONContainsPath(t, descriptor.OutputSchema, "message")
	})

	t.Run("Should List And Call SSE Server", func(t *testing.T) {
		t.Parallel()

		testServer := mcpsrv.NewTestServer(newFakeSDKServer(nil))
		t.Cleanup(testServer.Close)
		executor := newTestMCPExecutor(t, aghconfig.MCPServer{
			Name:      "Linear",
			Transport: aghconfig.MCPServerTransportSSE,
			URL:       testServer.URL + "/sse",
		})

		descriptor := requireMCPDescriptor(t, executor, "Linear", "mcp__linear__echo")
		result := callMCPTool(t, executor, descriptor, json.RawMessage(`{"message":"from-sse"}`))
		if got, want := result.Preview, "echo: from-sse"; got != want {
			t.Fatalf("result.Preview = %q, want %q", got, want)
		}
	})

	t.Run("Should List And Call Stdio Server", func(t *testing.T) {
		t.Parallel()

		executor := newTestMCPExecutor(t, aghconfig.MCPServer{
			Name:      "Local",
			Transport: aghconfig.MCPServerTransportStdio,
			Command:   os.Args[0],
			Args:      []string{"-test.run=TestMCPStdioHelperProcess"},
			Env: map[string]string{
				stdioHelperEnv: "1",
			},
		})

		descriptor := requireMCPDescriptor(t, executor, "Local", "mcp__local__echo")
		result := callMCPTool(t, executor, descriptor, json.RawMessage(`{"message":"from-stdio"}`))
		if got, want := result.Preview, "echo: from-stdio"; got != want {
			t.Fatalf("result.Preview = %q, want %q", got, want)
		}
	})

	t.Run("Should Inject Authorization Header Only Inside MCP And Return Redacted Data", func(t *testing.T) {
		t.Parallel()

		var mu sync.Mutex
		seenHeaders := make([]string, 0)
		handler := mcpsrv.NewStreamableHTTPServer(newFakeSDKServer(nil))
		testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header != "Bearer secret-access" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}
			mu.Lock()
			seenHeaders = append(seenHeaders, header)
			mu.Unlock()
			handler.ServeHTTP(w, r)
		}))
		t.Cleanup(testServer.Close)

		configuredServer := authEnabledServer("secure", aghconfig.MCPServerTransportHTTP, testServer.URL)
		target := globalMCPExecutorTarget("secure")
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(context.Background(), mcpauth.TokenRecord{
			Target: target, DefinitionFingerprint: definitionFingerprint(t, target, configuredServer),
			ClientID: "client-id", Scopes: []string{"tools.read"}, AccessToken: "secret-access",
			RefreshToken: "secret-refresh", TokenType: "Bearer",
			ExpiresAt: time.Now().Add(time.Hour), UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken() error = %v", err)
		}
		executor := newTestMCPExecutor(
			t,
			configuredServer,
			WithTokenStore(store),
		)

		descriptor := requireMCPDescriptor(t, executor, "secure", "mcp__secure__echo")
		result := callMCPTool(t, executor, descriptor, json.RawMessage(`{"message":"redacted"}`))
		mu.Lock()
		headerCount := len(seenHeaders)
		mu.Unlock()
		if headerCount == 0 {
			t.Fatal("authorization header was not observed by fake MCP server")
		}
		assertJSONDoesNotContain(t, "descriptors", descriptor, "secret-access", "secret-refresh")
		assertJSONDoesNotContain(t, "result", result, "secret-access", "secret-refresh")
	})

	t.Run("Should never send a token bound to a replaced server definition", func(t *testing.T) {
		t.Parallel()

		target := globalMCPExecutorTarget("secure")
		original := authEnabledServer(
			"secure",
			aghconfig.MCPServerTransportHTTP,
			"https://original.example/mcp",
		)
		replacement := authEnabledServer(
			"secure",
			aghconfig.MCPServerTransportHTTP,
			"https://replacement.example/mcp",
		)
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(t.Context(), mcpauth.TokenRecord{
			Target: target, DefinitionFingerprint: definitionFingerprint(t, target, original),
			ClientID: "client-id", AccessToken: "original-secret", TokenType: "Bearer",
			ExpiresAt: time.Now().Add(time.Hour), UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken(original) error = %v", err)
		}
		executor := newTestMCPExecutor(t, replacement, WithTokenStore(store))
		resolved := ResolvedServer{Server: replacement, Target: target}
		if got := executor.authorizationHeader(t.Context(), resolved); got != "" {
			t.Fatalf("authorizationHeader(replacement) = %q, want empty", got)
		}
		_, err := executor.ListTools(t.Context(), toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: "secure", RawServerName: "secure",
		})
		requireReason(t, err, toolspkg.ReasonMCPAuthRequired)
	})

	t.Run("Should isolate same-named workspace credentials at executor resolution", func(t *testing.T) {
		t.Parallel()

		workspaceA := mcpauth.Target{
			Scope: mcpauth.ScopeWorkspace, WorkspaceID: "workspace-a", ServerName: "linear",
		}
		workspaceB := mcpauth.Target{
			Scope: mcpauth.ScopeWorkspace, WorkspaceID: "workspace-b", ServerName: "linear",
		}
		serverA := newAuthorizedMCPExecutorServer(t, "workspace-a-token")
		serverB := newAuthorizedMCPExecutorServer(t, "workspace-b-token")
		store := newMemoryTokenStore()
		for _, fixture := range []struct {
			record mcpauth.TokenRecord
			server aghconfig.MCPServer
		}{
			{
				record: mcpauth.TokenRecord{
					Target: workspaceA, AccessToken: "workspace-a-token", TokenType: "Bearer",
					ExpiresAt: time.Now().Add(time.Hour), UpdatedAt: time.Now(),
				},
				server: authEnabledServer("linear", aghconfig.MCPServerTransportHTTP, serverA.URL),
			},
			{
				record: mcpauth.TokenRecord{
					Target: workspaceB, AccessToken: "workspace-b-token", TokenType: "Bearer",
					ExpiresAt: time.Now().Add(time.Hour), UpdatedAt: time.Now(),
				},
				server: authEnabledServer("linear", aghconfig.MCPServerTransportHTTP, serverB.URL),
			},
		} {
			fixture.record.DefinitionFingerprint = definitionFingerprint(t, fixture.record.Target, fixture.server)
			if err := store.SaveMCPAuthToken(context.Background(), fixture.record); err != nil {
				t.Fatalf("SaveMCPAuthToken(%#v) error = %v", fixture.record.Target, err)
			}
		}
		resolver := ServerResolverFunc(func(
			_ context.Context,
			source toolspkg.SourceRef,
		) (ResolvedServer, error) {
			switch source.ResourceID {
			case "mcp-workspace-a":
				return ResolvedServer{
					Server: authEnabledServer("linear", aghconfig.MCPServerTransportHTTP, serverA.URL),
					Target: workspaceA,
				}, nil
			case "mcp-workspace-b":
				return ResolvedServer{
					Server: authEnabledServer("linear", aghconfig.MCPServerTransportHTTP, serverB.URL),
					Target: workspaceB,
				}, nil
			default:
				return ResolvedServer{}, errors.New("unexpected MCP resource")
			}
		})
		executor, err := NewMCPCallExecutor(resolver, WithTokenStore(store))
		if err != nil {
			t.Fatalf("NewMCPCallExecutor() error = %v", err)
		}
		for _, source := range []toolspkg.SourceRef{
			{
				Kind: toolspkg.SourceMCP, Owner: "linear", RawServerName: "linear",
				ResourceID: "mcp-workspace-a", Scope: "workspace", WorkspaceID: "workspace-a",
			},
			{
				Kind: toolspkg.SourceMCP, Owner: "linear", RawServerName: "linear",
				ResourceID: "mcp-workspace-b", Scope: "workspace", WorkspaceID: "workspace-b",
			},
		} {
			descriptors, err := executor.ListTools(testContext(t), source)
			if err != nil {
				t.Fatalf("ListTools(%s) error = %v", source.WorkspaceID, err)
			}
			if len(descriptors) != 1 {
				t.Fatalf("ListTools(%s) count = %d, want 1", source.WorkspaceID, len(descriptors))
			}
		}

		if err := store.DeleteMCPAuthToken(context.Background(), workspaceA); err != nil {
			t.Fatalf("DeleteMCPAuthToken(workspace-a) error = %v", err)
		}
		_, err = executor.ListTools(testContext(t), toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: "linear", RawServerName: "linear",
			ResourceID: "mcp-workspace-a", Scope: "workspace", WorkspaceID: "workspace-a",
		})
		requireReason(t, err, toolspkg.ReasonMCPAuthRequired)
		if _, err := executor.ListTools(testContext(t), toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: "linear", RawServerName: "linear",
			ResourceID: "mcp-workspace-b", Scope: "workspace", WorkspaceID: "workspace-b",
		}); err != nil {
			t.Fatalf("ListTools(workspace-b after workspace-a logout) error = %v", err)
		}
	})

	t.Run("Should Return Auth Required Without Starting Login Flow", func(t *testing.T) {
		t.Parallel()

		store := newMemoryTokenStore()
		executor := newTestMCPExecutor(
			t,
			authEnabledServer("secure", aghconfig.MCPServerTransportHTTP, "http://127.0.0.1:1/mcp"),
			WithTokenStore(store),
		)

		_, err := executor.ListTools(testContext(t), toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "secure",
			RawServerName: "secure",
		})
		requireReason(t, err, toolspkg.ReasonMCPAuthRequired)
		if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "token") {
			t.Fatalf("ListTools() error = %q, want redacted auth-required error", err.Error())
		}
	})

	t.Run("Should Return Invalid Auth For Token Missing Access Token", func(t *testing.T) {
		t.Parallel()

		server := authEnabledServer("secure", aghconfig.MCPServerTransportHTTP, "http://127.0.0.1:1/mcp")
		target := globalMCPExecutorTarget("secure")
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(context.Background(), mcpauth.TokenRecord{
			Target: target, DefinitionFingerprint: definitionFingerprint(t, target, server),
			ClientID: "client-id", RefreshToken: "secret-refresh", TokenType: "Bearer",
			ExpiresAt: time.Now().Add(time.Hour), UpdatedAt: time.Now(),
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken() error = %v", err)
		}
		executor := newTestMCPExecutor(
			t,
			server,
			WithTokenStore(store),
		)

		_, err := executor.ListTools(testContext(t), toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "secure",
			RawServerName: "secure",
		})
		requireReason(t, err, toolspkg.ReasonMCPAuthInvalid)
	})

	t.Run("Should Attempt One Refresh And Return Redacted Refresh Failure", func(t *testing.T) {
		t.Parallel()

		fakeAuth := &fakeAuthService{
			status: mcpauth.Status{
				ServerName:   "secure",
				Status:       mcpauth.StatusExpired,
				AuthType:     string(aghconfig.MCPAuthTypeOAuth2PKCE),
				ClientID:     "client-id",
				TokenPresent: true,
				Refreshable:  true,
			},
			refreshErr: errors.New("token endpoint leaked secret-refresh"),
		}
		executor := newTestMCPExecutor(
			t,
			authEnabledServer("secure", aghconfig.MCPServerTransportHTTP, "http://127.0.0.1:1/mcp"),
			withAuthService(fakeAuth),
		)

		_, err := executor.ListTools(testContext(t), toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "secure",
			RawServerName: "secure",
		})
		requireReason(t, err, toolspkg.ReasonMCPAuthRefreshFailed)
		if got, want := fakeAuth.refreshCallCount(), 1; got != want {
			t.Fatalf("Refresh() calls = %d, want %d", got, want)
		}
		if strings.Contains(err.Error(), "secret-refresh") {
			t.Fatalf("ListTools() error = %q, want redacted refresh failure", err.Error())
		}
	})

	t.Run("Should Normalize Cancellation And Timeout", func(t *testing.T) {
		t.Parallel()

		executor := newTestMCPExecutor(t, aghconfig.MCPServer{
			Name:      "github",
			Transport: aghconfig.MCPServerTransportHTTP,
			URL:       "http://127.0.0.1:1/mcp",
		})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := executor.ListTools(ctx, toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "github",
			RawServerName: "github",
		})
		requireReason(t, err, toolspkg.ReasonCallCanceled)

		blockingServer := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			timer := time.NewTimer(200 * time.Millisecond)
			defer timer.Stop()
			select {
			case <-r.Context().Done():
			case <-timer.C:
			}
		}))
		t.Cleanup(blockingServer.Close)
		timeoutExecutor := newTestMCPExecutor(
			t,
			aghconfig.MCPServer{
				Name:      "slow",
				Transport: aghconfig.MCPServerTransportHTTP,
				URL:       blockingServer.URL,
			},
			WithTimeout(20*time.Millisecond),
			WithHTTPClient(&http.Client{Timeout: 2 * time.Second}),
		)
		_, err = timeoutExecutor.ListTools(testContext(t), toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "slow",
			RawServerName: "slow",
		})
		requireReason(t, err, toolspkg.ReasonCallTimedOut)
	})

	t.Run("Should Classify A Terminated Stdio Sidecar As Unreachable", func(t *testing.T) {
		t.Parallel()

		executor := newTestMCPExecutor(t, aghconfig.MCPServer{
			Name:      "terminated",
			Transport: aghconfig.MCPServerTransportStdio,
			Command:   os.Args[0],
			Args:      []string{"-test.run=TestMCPStdioHelperProcess"},
			Env: map[string]string{
				stdioFailedHelperEnv: "1",
			},
		})
		_, err := executor.ListTools(testContext(t), toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "terminated",
			RawServerName: "terminated",
		})
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)
	})

	t.Run("Should Use Explicit MCP Constructors And Avoid OAuth Helpers", func(t *testing.T) {
		t.Parallel()

		paths, err := filepath.Glob("executor*.go")
		if err != nil {
			t.Fatalf("filepath.Glob(executor files) error = %v", err)
		}
		var source strings.Builder
		for _, path := range paths {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("os.ReadFile(%s) error = %v", path, readErr)
			}
			source.Write(data)
		}
		required := []string{
			"NewStdioMCPClient",
			"NewStreamableHttpClient",
			"NewSSEMCPClient",
		}
		for _, needle := range required {
			if !strings.Contains(source.String(), needle) {
				t.Fatalf("executor implementation does not contain required constructor %q", needle)
			}
		}
		forbidden := []string{
			"NewOAuthStreamableHttpClient",
			"NewOAuthSSEClient",
			"NewOAuthHandler",
			"MemoryTokenStore",
			"WithHTTPOAuth",
			"WithOAuth(",
		}
		for _, needle := range forbidden {
			if strings.Contains(source.String(), needle) {
				t.Fatalf("executor implementation contains forbidden OAuth helper %q", needle)
			}
		}
		assertInternalToolsDoNotReferenceTokenMaterial(t)
	})
}

func TestCallExecutorDescriptorFromToolPreservesPresentationMetadata(t *testing.T) {
	t.Run("Should preserve valid presentation metadata", func(t *testing.T) {
		t.Parallel()

		executor := &CallExecutor{}
		descriptor, err := executor.descriptorFromTool(
			toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "github"},
			aghconfig.MCPServer{Name: "github"},
			mcpsdk.Tool{
				Name:        "lookup",
				Description: "Look up an issue",
				RawInputSchema: json.RawMessage(
					`{"type":"object","properties":{"query":{"type":"string"}}}`,
				),
				Meta: &mcpsdk.Meta{AdditionalFields: map[string]any{
					mcpFriendlyVerbMetadataKey: "Looking up",
					mcpPreviewMetadataKey:      "arg:query",
				}},
			},
		)
		if err != nil {
			t.Fatalf("descriptorFromTool() error = %v", err)
		}
		if got, want := descriptor.FriendlyVerb, "Looking up"; got != want {
			t.Fatalf("descriptor.FriendlyVerb = %q, want %q", got, want)
		}
		if got, want := descriptor.Preview, "arg:query"; got != want {
			t.Fatalf("descriptor.Preview = %q, want %q", got, want)
		}
	})

	t.Run("Should reject non-string presentation metadata", func(t *testing.T) {
		t.Parallel()

		_, _, err := mcpToolPresentationMetadata(mcpsdk.Tool{
			Meta: &mcpsdk.Meta{AdditionalFields: map[string]any{
				mcpFriendlyVerbMetadataKey: true,
			}},
		})
		if err == nil {
			t.Fatal("mcpToolPresentationMetadata() error = nil, want invalid type failure")
		}
	})
}

func TestMCPCallExecutorStdioEnvironmentBoundary(t *testing.T) {
	t.Run("Should exclude ambient daemon secrets from stdio MCP processes", func(t *testing.T) {
		setMCPTestEnv(t, stdioParentSecretEnv, "ambient-secret")

		executor := newTestMCPExecutor(t, aghconfig.MCPServer{
			Name:      "Local",
			Transport: aghconfig.MCPServerTransportStdio,
			Command:   os.Args[0],
			Args:      []string{"-test.run=TestMCPStdioHelperProcess"},
			Env: map[string]string{
				stdioEnvHelperEnv: "1",
			},
		})

		descriptor := requireMCPDescriptor(t, executor, "Local", "mcp__local__echo")
		result := callMCPTool(
			t,
			executor,
			descriptor,
			json.RawMessage("{\"message\":\""+stdioParentSecretEnv+"\"}"),
		)
		if got, want := result.Preview, "env:"; got != want {
			t.Fatalf("result.Preview = %q, want %q", got, want)
		}
	})

	t.Run("Should expose explicitly bound secret env to stdio MCP processes", func(t *testing.T) {
		setMCPTestEnv(t, stdioExplicitSecretSource, "bound-secret")

		executor := newTestMCPExecutor(
			t,
			aghconfig.MCPServer{
				Name:      "Local",
				Transport: aghconfig.MCPServerTransportStdio,
				Command:   os.Args[0],
				Args:      []string{"-test.run=TestMCPStdioHelperProcess"},
				Env: map[string]string{
					stdioEnvHelperEnv: "1",
				},
				SecretEnv: map[string]string{
					stdioExplicitSecretEnv: "env:" + stdioExplicitSecretSource,
				},
			},
			WithSecretLookup(os.Getenv),
		)

		descriptor := requireMCPDescriptor(t, executor, "Local", "mcp__local__echo")
		result := callMCPTool(
			t,
			executor,
			descriptor,
			json.RawMessage("{\"message\":\""+stdioExplicitSecretEnv+"\"}"),
		)
		if got, want := result.Preview, "env: bound-secret"; got != want {
			t.Fatalf("result.Preview = %q, want %q", got, want)
		}
	})
}

func setMCPTestEnv(t *testing.T, key string, value string) {
	t.Helper()

	original, hadOriginal := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("Setenv(%q) error = %v", key, err)
	}
	t.Cleanup(func() {
		if hadOriginal {
			if err := os.Setenv(key, original); err != nil {
				t.Fatalf("restore Setenv(%q) error = %v", key, err)
			}
			return
		}
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("Unsetenv(%q) error = %v", key, err)
		}
	})
}

func TestMCPStdioHelperProcess(t *testing.T) {
	t.Run("Should Serve Stdio When Requested", func(_ *testing.T) {
		if os.Getenv(stdioFailedHelperEnv) == "1" {
			os.Exit(17)
		}
		if os.Getenv(stdioEnvHelperEnv) == "1" {
			server := newFakeSDKServer(
				func(_ context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
					key := mcpsdk.ParseString(req, "message", "")
					value := os.Getenv(key)
					return mcpsdk.NewToolResultStructured(
						map[string]string{"message": value},
						"env: "+value,
					), nil
				},
			)
			if err := mcpsrv.ServeStdio(server); err != nil {
				if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
					os.Exit(3)
				}
				os.Exit(2)
			}
			os.Exit(0)
		}
		if os.Getenv(stdioHelperEnv) != "1" {
			return
		}
		if err := mcpsrv.ServeStdio(newFakeSDKServer(nil)); err != nil {
			if _, writeErr := fmt.Fprintln(os.Stderr, err); writeErr != nil {
				os.Exit(3)
			}
			os.Exit(2)
		}
		os.Exit(0)
	})
}

func TestMCPCallExecutorHelpers(t *testing.T) {
	t.Run("Should Return Redacted Status And Use Secret Lookup Internally", func(t *testing.T) {
		t.Parallel()

		expiresAt := time.Now().Add(time.Hour).UTC()
		fakeAuth := &fakeAuthService{
			status: mcpauth.Status{
				ServerName:   "secure",
				Status:       mcpauth.StatusAuthenticated,
				AuthType:     string(aghconfig.MCPAuthTypeOAuth2PKCE),
				ClientID:     "client-id",
				Scopes:       []string{" tools.read ", "issues.write"},
				ExpiresAt:    &expiresAt,
				TokenPresent: true,
				Refreshable:  true,
			},
		}
		server := authEnabledServer("secure", aghconfig.MCPServerTransportHTTP, "https://mcp.example.test/mcp")
		server.Auth.ClientSecretRef = "env:MCP_CLIENT_SECRET"
		executor := newTestMCPExecutor(
			t,
			server,
			withAuthService(fakeAuth),
			WithSecretLookup(func(name string) string {
				if name == "MCP_CLIENT_SECRET" {
					return "client-secret"
				}
				return ""
			}),
		)

		status, err := executor.Status(testContext(t), toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "secure",
			RawServerName: "secure",
		})
		if err != nil {
			t.Fatalf("Status() error = %v", err)
		}
		if got, want := status.Status, string(mcpauth.StatusAuthenticated); got != want {
			t.Fatalf("status.Status = %q, want %q", got, want)
		}
		if status.ExpiresAt == nil || !status.ExpiresAt.Equal(expiresAt) {
			t.Fatalf("status.ExpiresAt = %v, want %v", status.ExpiresAt, expiresAt)
		}
		expiresAt = expiresAt.Add(-time.Hour)
		if status.ExpiresAt.Equal(expiresAt) {
			t.Fatal("status.ExpiresAt aliases auth service input pointer")
		}
		cfg := fakeAuth.lastServerConfig()
		if got, want := cfg.ClientSecret, "client-secret"; got != want {
			t.Fatalf("auth cfg ClientSecret = %q, want %q", got, want)
		}
		assertJSONDoesNotContain(t, "status", status, "client-secret")

		unconfigured := newTestMCPExecutor(t, aghconfig.MCPServer{
			Name:      "plain",
			Transport: aghconfig.MCPServerTransportHTTP,
			URL:       "https://mcp.example.test/mcp",
		})
		plainStatus, err := unconfigured.Status(testContext(t), toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "plain",
			RawServerName: "plain",
		})
		if err != nil {
			t.Fatalf("Status(unconfigured) error = %v", err)
		}
		if got, want := plainStatus.Status, string(mcpauth.StatusUnconfigured); got != want {
			t.Fatalf("plainStatus.Status = %q, want %q", got, want)
		}
	})

	t.Run("Should Reject Missing Resolver Nil Context Unknown Server And Unsupported Transport", func(t *testing.T) {
		t.Parallel()

		if _, err := NewMCPCallExecutor(nil); err == nil {
			t.Fatal("NewMCPCallExecutor(nil) error = nil, want error")
		}
		executor := newTestMCPExecutor(t, aghconfig.MCPServer{
			Name:      "github",
			Transport: aghconfig.MCPServerTransportHTTP,
			URL:       "http://127.0.0.1:1/mcp",
		})
		var nilContext context.Context
		_, err := executor.ListTools(nilContext, toolspkg.SourceRef{RawServerName: "github"})
		requireReason(t, err, toolspkg.ReasonCallCanceled)
		_, err = executor.CallTool(nilContext, toolspkg.SourceRef{RawServerName: "github"}, toolspkg.MCPToolCallRequest{
			ToolID: "mcp__github__echo",
		})
		requireReason(t, err, toolspkg.ReasonCallCanceled)
		_, err = executor.Status(nilContext, toolspkg.SourceRef{RawServerName: "github"})
		requireReason(t, err, toolspkg.ReasonCallCanceled)
		_, err = executor.ListTools(testContext(t), toolspkg.SourceRef{RawServerName: "missing"})
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)

		unsupported := newTestMCPExecutor(t, aghconfig.MCPServer{
			Name:      "github",
			Transport: aghconfig.MCPServerTransport("websocket"),
			URL:       "https://mcp.example.test/ws",
		})
		_, err = unsupported.ListTools(testContext(t), toolspkg.SourceRef{RawServerName: "github"})
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)
	})

	t.Run("Should Normalize Schemas Arguments Content Preview And Errors", func(t *testing.T) {
		t.Parallel()

		rawSchema := json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`)
		tool := mcpsdk.NewTool("lookup", mcpsdk.WithRawInputSchema(rawSchema))
		inputSchema, err := inputSchemaBytes(tool)
		if err != nil {
			t.Fatalf("inputSchemaBytes(raw) error = %v", err)
		}
		if got, want := string(inputSchema), string(rawSchema); got != want {
			t.Fatalf("inputSchemaBytes(raw) = %s, want %s", got, want)
		}
		if _, err := inputSchemaBytes(mcpsdk.Tool{Name: "bad"}); err == nil {
			t.Fatal("inputSchemaBytes(missing) error = nil, want error")
		}
		output, err := outputSchemaBytes(mcpsdk.NewTool("lookup"))
		if err != nil {
			t.Fatalf("outputSchemaBytes(empty) error = %v", err)
		}
		if output != nil {
			t.Fatalf("outputSchemaBytes(empty) = %s, want nil", output)
		}
		rawOutputSchema := json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}}}`)
		output, err = outputSchemaBytes(mcpsdk.NewTool("lookup", mcpsdk.WithRawOutputSchema(rawOutputSchema)))
		if err != nil {
			t.Fatalf("outputSchemaBytes(raw) error = %v", err)
		}
		if got, want := string(output), string(rawOutputSchema); got != want {
			t.Fatalf("outputSchemaBytes(raw) = %s, want %s", got, want)
		}
		if got := cloneRaw(rawSchema); string(got) != string(rawSchema) {
			t.Fatalf("cloneRaw() = %s, want %s", got, rawSchema)
		}
		if got := cloneRaw(nil); got != nil {
			t.Fatalf("cloneRaw(nil) = %s, want nil", got)
		}

		args, err := decodeArguments(json.RawMessage(`{"q":"x"}`))
		if err != nil {
			t.Fatalf("decodeArguments(valid) error = %v", err)
		}
		if _, ok := args.(map[string]any); !ok {
			t.Fatalf("decodeArguments(valid) type = %T, want map[string]any", args)
		}
		emptyArgs, err := decodeArguments(nil)
		if err != nil {
			t.Fatalf("decodeArguments(empty) error = %v", err)
		}
		if got, want := len(emptyArgs.(map[string]any)), 0; got != want {
			t.Fatalf("len(decodeArguments(empty)) = %d, want %d", got, want)
		}
		_, err = decodeArguments(json.RawMessage(`{`))
		requireReason(t, err, toolspkg.ReasonSchemaInvalid)

		result, err := toolResultFromMCP(&mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				mcpsdk.TextContent{Type: "text", Text: "ok"},
				mcpsdk.ImageContent{Type: "image", Data: "aW1n", MIMEType: "image/png"},
				mcpsdk.AudioContent{Type: "audio", Data: "YXVkaW8=", MIMEType: "audio/mpeg"},
			},
			StructuredContent: map[string]string{"ok": "true"},
			IsError:           true,
		})
		if err != nil {
			t.Fatalf("toolResultFromMCP() error = %v", err)
		}
		if got, want := result.Preview, "error: ok"; got != want {
			t.Fatalf("result.Preview = %q, want %q", got, want)
		}
		if len(result.Content) != 3 || len(result.Structured) == 0 || len(result.Metadata) == 0 {
			t.Fatalf("result = %#v, want content structured metadata", result)
		}
		emptyResult, err := toolResultFromMCP(nil)
		if err != nil {
			t.Fatalf("toolResultFromMCP(nil) error = %v", err)
		}
		if len(emptyResult.Content) != 0 {
			t.Fatalf("toolResultFromMCP(nil).Content = %#v, want empty", emptyResult.Content)
		}
		embedded, err := toolContentFromMCP(mcpsdk.EmbeddedResource{
			Type: "resource",
			Resource: mcpsdk.TextResourceContents{
				URI:  "file://note.txt",
				Text: "hello",
			},
		})
		if err != nil {
			t.Fatalf("toolContentFromMCP(embedded) error = %v", err)
		}
		if got, want := embedded.Type, "mcp"; got != want {
			t.Fatalf("embedded.Type = %q, want %q", got, want)
		}
		if got, want := mcpPreview(nil, false), "empty MCP result"; got != want {
			t.Fatalf("mcpPreview(empty) = %q, want %q", got, want)
		}
		if got, want := mcpPreview(
			[]toolspkg.ToolContent{{Type: "image"}},
			false,
		), "1 MCP content blocks"; got != want {
			t.Fatalf("mcpPreview(image) = %q, want %q", got, want)
		}

		requireReason(t, normalizeMCPError("", context.Canceled), toolspkg.ReasonCallCanceled)
		requireReason(t, normalizeMCPError("", context.DeadlineExceeded), toolspkg.ReasonCallTimedOut)
		requireReason(t, normalizeMCPError("", mcptransport.ErrAuthorizationRequired), toolspkg.ReasonMCPAuthRequired)
	})

	t.Run("Should Handle Expired Auth Refresh Outcomes", func(t *testing.T) {
		t.Parallel()

		server := authEnabledServer("secure", aghconfig.MCPServerTransportHTTP, "https://mcp.example.test/mcp")
		fakeAuth := &fakeAuthService{
			status: mcpauth.Status{
				ServerName:   "secure",
				Status:       mcpauth.StatusExpired,
				AuthType:     string(aghconfig.MCPAuthTypeOAuth2PKCE),
				ClientID:     "client-id",
				TokenPresent: true,
				Refreshable:  true,
			},
			refresh: mcpauth.Status{
				ServerName:   "secure",
				Status:       mcpauth.StatusAuthenticated,
				AuthType:     string(aghconfig.MCPAuthTypeOAuth2PKCE),
				ClientID:     "client-id",
				TokenPresent: true,
				Refreshable:  true,
			},
		}
		executor := newTestMCPExecutor(t, server, withAuthService(fakeAuth))
		resolved := ResolvedServer{Server: server, Target: globalMCPExecutorTarget("secure")}
		if err := executor.ensureAuthorized(testContext(t), resolved); err != nil {
			t.Fatalf("ensureAuthorized(refresh success) error = %v", err)
		}
		if got, want := fakeAuth.refreshCallCount(), 1; got != want {
			t.Fatalf("Refresh() calls = %d, want %d", got, want)
		}

		invalidAuth := &fakeAuthService{
			status: mcpauth.Status{
				ServerName:   "secure",
				Status:       mcpauth.StatusExpired,
				AuthType:     string(aghconfig.MCPAuthTypeOAuth2PKCE),
				ClientID:     "client-id",
				TokenPresent: true,
				Refreshable:  true,
			},
			refresh: mcpauth.Status{
				ServerName: "secure",
				Status:     mcpauth.StatusInvalid,
			},
		}
		executor = newTestMCPExecutor(t, server, withAuthService(invalidAuth))
		err := executor.ensureAuthorized(testContext(t), resolved)
		requireReason(t, err, toolspkg.ReasonMCPAuthInvalid)
	})

	t.Run("Should Handle Authorization Header Edge Cases And Server Matching", func(t *testing.T) {
		t.Parallel()

		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(context.Background(), mcpauth.TokenRecord{
			Target:      globalMCPExecutorTarget("github"),
			AccessToken: "token",
			TokenType:   "mac",
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken(non-bearer) error = %v", err)
		}
		executor := newTestMCPExecutor(
			t,
			aghconfig.MCPServer{
				Name:      "github",
				Transport: aghconfig.MCPServerTransportHTTP,
				URL:       "https://mcp.example.test/mcp",
			},
			WithTokenStore(store),
		)
		resolved := ResolvedServer{
			Server: aghconfig.MCPServer{Name: "github"},
			Target: globalMCPExecutorTarget("github"),
		}
		if got := executor.authorizationHeader(testContext(t), resolved); got != "" {
			t.Fatalf("authorizationHeader(non-bearer) = %q, want empty", got)
		}
		if got := (&CallExecutor{}).authorizationHeader(
			testContext(t),
			resolved,
		); got != "" {
			t.Fatalf("authorizationHeader(nil store) = %q, want empty", got)
		}
		if !mcpServerMatches(aghconfig.MCPServer{Name: "GitHub"}, "github") {
			t.Fatal("mcpServerMatches() = false, want true for canonical owner")
		}
		if mcpServerMatches(aghconfig.MCPServer{Name: "9bad"}, "bad") {
			t.Fatal("mcpServerMatches(invalid canonical name) = true, want false")
		}
	})
}

func newFakeSDKServer(handler mcpsrv.ToolHandlerFunc) *mcpsrv.MCPServer {
	if handler == nil {
		handler = func(_ context.Context, req mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			message := mcpsdk.ParseString(req, "message", "")
			return mcpsdk.NewToolResultStructured(
				map[string]string{"message": message},
				"echo: "+message,
			), nil
		}
	}
	server := mcpsrv.NewMCPServer("fake", "1.0.0", mcpsrv.WithToolCapabilities(true))
	server.AddTool(
		mcpsdk.NewTool(
			"echo",
			mcpsdk.WithDescription("Echo message"),
			mcpsdk.WithString("message"),
			mcpsdk.WithRawOutputSchema(json.RawMessage(
				`{"type":"object","properties":{"message":{"type":"string"}}}`,
			)),
			mcpsdk.WithReadOnlyHintAnnotation(true),
		),
		handler,
	)
	return server
}

func newAuthorizedMCPExecutorServer(t *testing.T, token string) *httptest.Server {
	t.Helper()

	handler := mcpsrv.NewStreamableHTTPServer(newFakeSDKServer(nil))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer "+token; got != want {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server
}

func newTestMCPExecutor(
	t *testing.T,
	server aghconfig.MCPServer,
	options ...CallExecutorOption,
) *CallExecutor {
	t.Helper()

	executor, err := NewMCPCallExecutor(
		ServerResolverFunc(func(context.Context, toolspkg.SourceRef) (ResolvedServer, error) {
			return ResolvedServer{Server: server, Target: globalMCPExecutorTarget(server.Name)}, nil
		}),
		options...,
	)
	if err != nil {
		t.Fatalf("NewMCPCallExecutor() error = %v", err)
	}
	return executor
}

func requireMCPDescriptor(
	t *testing.T,
	executor *CallExecutor,
	serverName string,
	wantID toolspkg.ToolID,
) toolspkg.MCPToolDescriptor {
	t.Helper()

	descriptors, err := executor.ListTools(testContext(t), toolspkg.SourceRef{
		Kind:          toolspkg.SourceMCP,
		Owner:         serverName,
		RawServerName: serverName,
	})
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if got, want := len(descriptors), 1; got != want {
		t.Fatalf("len(ListTools()) = %d, want %d", got, want)
	}
	descriptor := descriptors[0]
	if got := descriptor.ID; got != wantID {
		t.Fatalf("descriptor.ID = %q, want %q", got, wantID)
	}
	if got, want := descriptor.RawName, "echo"; got != want {
		t.Fatalf("descriptor.RawName = %q, want %q", got, want)
	}
	if got := descriptor.Source.RawServerName; got != serverName {
		t.Fatalf("descriptor.Source.RawServerName = %q, want %q", got, serverName)
	}
	if got, want := descriptor.Source.RawToolName, "echo"; got != want {
		t.Fatalf("descriptor.Source.RawToolName = %q, want %q", got, want)
	}
	if len(descriptor.InputSchema) == 0 {
		t.Fatal("descriptor.InputSchema is empty")
	}
	if len(descriptor.OutputSchema) == 0 {
		t.Fatal("descriptor.OutputSchema is empty")
	}
	return descriptor
}

func callMCPTool(
	t *testing.T,
	executor *CallExecutor,
	descriptor toolspkg.MCPToolDescriptor,
	input json.RawMessage,
) toolspkg.ToolResult {
	t.Helper()

	result, err := executor.CallTool(testContext(t), descriptor.Source, toolspkg.MCPToolCallRequest{
		ToolID:      descriptor.ID,
		RawToolName: descriptor.RawName,
		Input:       input,
	})
	if err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("CallTool() returned no content")
	}
	return result
}

func testContext(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func authEnabledServer(name string, transport aghconfig.MCPServerTransport, url string) aghconfig.MCPServer {
	return aghconfig.MCPServer{
		Name:      name,
		Transport: transport,
		URL:       url,
		Auth: aghconfig.MCPAuthConfig{
			Type:             aghconfig.MCPAuthTypeOAuth2PKCE,
			AuthorizationURL: "https://issuer.example.test/authorize",
			TokenURL:         "https://issuer.example.test/token",
			ClientID:         "client-id",
			Scopes:           []string{"tools.read"},
		},
	}
}

func definitionFingerprint(
	t *testing.T,
	target mcpauth.Target,
	server aghconfig.MCPServer,
) string {
	t.Helper()
	cfg, err := mcpauth.ServerConfigFromMCP(t.Context(), target, server, nil)
	if err != nil {
		t.Fatalf("ServerConfigFromMCP() error = %v", err)
	}
	fingerprint, err := mcpauth.ServerDefinitionFingerprint(cfg)
	if err != nil {
		t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
	}
	return fingerprint
}

func requireReason(t *testing.T, err error, want toolspkg.ReasonCode) {
	t.Helper()

	if err == nil {
		t.Fatalf("error = nil, want reason %q", want)
	}
	got, ok := toolspkg.ReasonOf(err)
	if !ok {
		t.Fatalf("ReasonOf(%v) not found, want %q", err, want)
	}
	if got != want {
		t.Fatalf("ReasonOf(%v) = %q, want %q", err, got, want)
	}
}

func requireJSONContainsPath(t *testing.T, raw json.RawMessage, path string) {
	t.Helper()

	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", raw, err)
	}
	properties, ok := decoded["properties"].(map[string]any)
	if !ok {
		t.Fatalf("schema properties = %#v, want object", decoded["properties"])
	}
	if _, ok := properties[path]; !ok {
		t.Fatalf("schema properties missing %q: %#v", path, properties)
	}
}

func assertJSONDoesNotContain(t *testing.T, label string, value any, forbidden ...string) {
	t.Helper()

	data, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%s) error = %v", label, err)
	}
	payload := string(data)
	for _, needle := range forbidden {
		if strings.Contains(payload, needle) {
			t.Fatalf("%s JSON leaked %q: %s", label, needle, payload)
		}
	}
}

func assertInternalToolsDoNotReferenceTokenMaterial(t *testing.T) {
	t.Helper()

	files := []string{
		"../tools/mcp.go",
		"../tools/tool.go",
		"../tools/result.go",
	}
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile(%s) error = %v", path, err)
		}
		source := string(data)
		forbidden := []string{
			"TokenRecord",
			"AccessToken",
			"RefreshToken",
			"Authorization: Bearer",
		}
		for _, needle := range forbidden {
			if strings.Contains(source, needle) {
				t.Fatalf("%s references forbidden token material %q", path, needle)
			}
		}
	}
}

type fakeAuthService struct {
	mu         sync.Mutex
	status     mcpauth.Status
	refresh    mcpauth.Status
	refreshErr error
	calls      int
	lastConfig mcpauth.ServerConfig
}

func (s *fakeAuthService) Status(ctx context.Context, cfg mcpauth.ServerConfig) (mcpauth.Status, error) {
	if err := ctx.Err(); err != nil {
		return mcpauth.Status{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lastConfig = cfg
	status := s.status
	if status.ServerName == "" {
		status.ServerName = cfg.Target.ServerName
	}
	return status, nil
}

func (s *fakeAuthService) Refresh(ctx context.Context, cfg mcpauth.ServerConfig) (mcpauth.Status, error) {
	if err := ctx.Err(); err != nil {
		return mcpauth.Status{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.calls++
	if s.refreshErr != nil {
		return mcpauth.Status{}, s.refreshErr
	}
	status := s.refresh
	if status.ServerName == "" {
		status.ServerName = cfg.Target.ServerName
	}
	return status, nil
}

func (s *fakeAuthService) refreshCallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func (s *fakeAuthService) lastServerConfig() mcpauth.ServerConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastConfig
}

type memoryTokenStore struct {
	mu     sync.Mutex
	tokens map[string]mcpauth.TokenRecord
}

func newMemoryTokenStore() *memoryTokenStore {
	return &memoryTokenStore{tokens: make(map[string]mcpauth.TokenRecord)}
}

func (s *memoryTokenStore) SaveMCPAuthToken(ctx context.Context, token mcpauth.TokenRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key, err := token.Target.Key()
	if err != nil {
		return err
	}
	s.tokens[key] = cloneTokenRecord(token)
	return nil
}

func (s *memoryTokenStore) GetMCPAuthToken(
	ctx context.Context,
	target mcpauth.Target,
) (mcpauth.TokenRecord, error) {
	if err := ctx.Err(); err != nil {
		return mcpauth.TokenRecord{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key, err := target.Key()
	if err != nil {
		return mcpauth.TokenRecord{}, err
	}
	token, ok := s.tokens[key]
	if !ok {
		return mcpauth.TokenRecord{}, mcpauth.ErrTokenNotFound
	}
	return cloneTokenRecord(token), nil
}

func (s *memoryTokenStore) ListMCPAuthTokens(ctx context.Context) ([]mcpauth.TokenRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	tokens := make([]mcpauth.TokenRecord, 0, len(s.tokens))
	for _, token := range s.tokens {
		tokens = append(tokens, cloneTokenRecord(token))
	}
	slices.SortFunc(tokens, func(left, right mcpauth.TokenRecord) int {
		leftKey := string(left.Target.Scope) + "\x00" + left.Target.WorkspaceID + "\x00" + left.Target.ServerName
		rightKey := string(right.Target.Scope) + "\x00" + right.Target.WorkspaceID + "\x00" + right.Target.ServerName
		return strings.Compare(leftKey, rightKey)
	})
	return tokens, nil
}

func (s *memoryTokenStore) DeleteMCPAuthToken(ctx context.Context, target mcpauth.Target) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	key, err := target.Key()
	if err != nil {
		return err
	}
	delete(s.tokens, key)
	return nil
}

func cloneTokenRecord(token mcpauth.TokenRecord) mcpauth.TokenRecord {
	token.Scopes = append([]string(nil), token.Scopes...)
	return token
}

func globalMCPExecutorTarget(serverName string) mcpauth.Target {
	return mcpauth.Target{Scope: mcpauth.ScopeGlobal, ServerName: serverName}
}
