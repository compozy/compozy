package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/testutil/mcpfixture"
	toolspkg "github.com/compozy/compozy/internal/tools"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	stdioHelperEnv            = "COMPOZY_MCP_STDIO_HELPER"
	stdioEnvHelperEnv         = "COMPOZY_MCP_STDIO_ENV_HELPER"
	stdioFailedHelperEnv      = "COMPOZY_MCP_STDIO_FAILED_HELPER"
	stdioParentSecretEnv      = "COMPOZY_PARENT_SECRET_TOKEN"
	stdioExplicitSecretEnv    = "COMPOZY_EXPLICIT_SECRET_TOKEN"
	stdioExplicitSecretSource = "COMPOZY_EXPLICIT_SECRET_SOURCE"
)

type testMCPSecretError struct{ secret string }

func (e *testMCPSecretError) Error() string {
	return "mcp upstream leaked " + e.secret
}

func TestMCPCallExecutor(t *testing.T) {
	t.Run("Should negotiate modern protocol and paginate tools", func(t *testing.T) {
		t.Parallel()

		fixture := mcpfixture.MustNew(mcpfixture.ProfileModern2026)
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "fixture",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       fixture.StartHTTP(t).URL,
		})
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture"}
		descriptors, protocolVersion, err := executor.ListToolsWithProtocol(t.Context(), source)
		if err != nil {
			t.Fatalf("ListToolsWithProtocol() error = %v", err)
		}
		if got, want := protocolVersion, mcpfixture.ModernProtocolVersion; got != want {
			t.Fatalf("protocol version = %q, want %q", got, want)
		}
		if got, want := len(descriptors), 2; got != want {
			t.Fatalf("descriptor count = %d, want %d after pagination", got, want)
		}
	})

	t.Run("Should negotiate legacy fallback and reject MRTR input requests", func(t *testing.T) {
		t.Parallel()

		legacy := mcpfixture.MustNew(mcpfixture.ProfileLegacy2025)
		legacyExecutor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "legacy",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       legacy.StartHTTP(t).URL,
		})
		legacySource := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "legacy", RawServerName: "legacy"}
		_, protocolVersion, err := legacyExecutor.ListToolsWithProtocol(t.Context(), legacySource)
		if err != nil {
			t.Fatalf("ListToolsWithProtocol(legacy) error = %v", err)
		}
		if got, want := protocolVersion, mcpfixture.LegacyProtocolVersion; got != want {
			t.Fatalf("legacy protocol version = %q, want %q", got, want)
		}

		inputRequired := mcpfixture.MustNew(mcpfixture.ProfileInputRequired)
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "input-required",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       inputRequired.StartHTTP(t).URL,
		})
		_, err = executor.CallTool(
			t.Context(),
			toolspkg.SourceRef{
				Kind:          toolspkg.SourceMCP,
				Owner:         "input-required",
				RawServerName: "input-required",
			},
			toolspkg.MCPToolCallRequest{
				ToolID:      "mcp__input_required__confirm",
				RawToolName: "confirm",
				Input:       json.RawMessage(`{}`),
			},
		)
		unsupported, unsupportedMatched := errors.AsType[*UnsupportedCapabilityError](err)
		if !unsupportedMatched || unsupported.Capability != "input_requests" {
			t.Fatalf("CallTool(confirm) error = %v, want unsupported input_requests capability", err)
		}
	})

	t.Run("Should List And Call Streamable HTTP Server", func(t *testing.T) {
		t.Parallel()

		testServer := newTestMCPHTTPServer(newFakeSDKServer(nil))
		t.Cleanup(testServer.Close)
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "GitHub",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       testServer.URL,
		})

		descriptor := requireMCPDescriptor(t, executor, "GitHub", "mcp__github__echo")
		result := callMCPTool(t, executor, descriptor, json.RawMessage(`{"message":"hello"}`))
		if got, want := result.Preview, "echo: hello"; got != want {
			t.Fatalf("result.Preview = %q, want %q", got, want)
		}
		requireJSONContainsPath(t, descriptor.OutputSchema, "message")
	})

	t.Run("Should block catalog HTTP servers from reaching loopback", func(t *testing.T) {
		t.Parallel()

		testServer := newTestMCPHTTPServer(newFakeSDKServer(nil))
		t.Cleanup(testServer.Close)
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:         "catalog-remote",
			Transport:    compozyconfig.MCPServerTransportHTTP,
			URL:          testServer.URL,
			CatalogEntry: "registry.example/catalog-remote",
		})
		_, err := executor.ListTools(t.Context(), toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "catalog-remote",
			RawServerName: "catalog-remote",
		})
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)
	})

	t.Run("Should surface insufficient scope challenge without authorizing", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			writer.Header().Set(
				"WWW-Authenticate",
				`Bearer error="insufficient_scope", scope="issues.read issues.write"`,
			)
			writer.WriteHeader(http.StatusForbidden)
		}))
		t.Cleanup(server.Close)
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "scope-challenge",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       server.URL,
		})
		_, err := executor.ListTools(t.Context(), toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "scope-challenge",
			RawServerName: "scope-challenge",
		})

		scopeErr, scopeErrMatched := errors.AsType[*InsufficientScopeError](err)
		if !scopeErrMatched {
			t.Fatalf("ListTools() error = %v, want insufficient scope challenge", err)
		}
		requireReason(t, err, toolspkg.ReasonMCPAuthRequired)
		if got, want := scopeErr.Scopes, []string{"issues.read", "issues.write"}; !slices.Equal(got, want) {
			t.Fatalf("challenge scopes = %#v, want %#v", got, want)
		}
	})

	t.Run("Should preserve authorization challenge drain and close failures", func(t *testing.T) {
		t.Parallel()

		drainErr := errors.New("challenge drain failed")
		closeErr := errors.New("challenge close failed")
		body := &scriptedMCPResponseBody{readErr: drainErr, closeErr: closeErr}
		transport := authorizationRoundTripper{
			next: mcpRoundTripperFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Status:     "401 Unauthorized",
					Header:     make(http.Header),
					Body:       body,
					Request:    request,
				}, nil
			}),
			header: "Bearer rejected",
		}
		request, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"https://mcp.example.test",
			http.NoBody,
		)
		if err != nil {
			t.Fatalf("http.NewRequestWithContext() error = %v", err)
		}
		//nolint:bodyclose // The authorization-challenge path returns no response and owns body cleanup.
		response, err := transport.RoundTrip(request)
		if response != nil {
			t.Fatalf("RoundTrip() response = %#v, want nil", response)
		}
		for _, want := range []error{ErrAuthorizationRequired, drainErr, closeErr} {
			if !errors.Is(err, want) {
				t.Fatalf("RoundTrip() error = %v, want %v", err, want)
			}
		}
		if body.closeCalls != 1 {
			t.Fatalf("response body close calls = %d, want 1", body.closeCalls)
		}
	})

	t.Run("Should classify a rejected bearer token as authentication required", func(t *testing.T) {
		t.Parallel()

		seenAuthorization := make(chan string, 1)
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			select {
			case seenAuthorization <- request.Header.Get("Authorization"):
			default:
			}
			writer.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			writer.WriteHeader(http.StatusUnauthorized)
		}))
		t.Cleanup(server.Close)
		configuredServer := authEnabledServer("rejected-token", compozyconfig.MCPServerTransportHTTP, server.URL)
		target := globalMCPExecutorTarget("rejected-token")
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(t.Context(), mcpauth.TokenRecord{
			Target:                target,
			DefinitionFingerprint: definitionFingerprint(t, target, configuredServer),
			Issuer:                "https://issuer.example.test",
			ClientID:              "client-id",
			Scopes:                []string{"tools.read"},
			AccessToken:           "rejected-access",
			RefreshToken:          "refresh-token",
			TokenType:             "Bearer",
			ExpiresAt:             time.Now().Add(time.Hour),
			UpdatedAt:             time.Now(),
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken() error = %v", err)
		}
		executor := newTestMCPExecutor(t, configuredServer, WithTokenStore(store))
		_, err := executor.ListTools(t.Context(), toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "rejected-token",
			RawServerName: "rejected-token",
		})
		requireReason(t, err, toolspkg.ReasonMCPAuthRequired)
		select {
		case got := <-seenAuthorization:
			if want := "Bearer rejected-access"; got != want {
				t.Fatalf("Authorization = %q, want %q", got, want)
			}
		default:
			t.Fatal("remote MCP server did not receive an authorization header")
		}
	})

	t.Run("Should List And Call Stdio Server", func(t *testing.T) {
		t.Parallel()

		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "Local",
			Transport: compozyconfig.MCPServerTransportStdio,
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
		handler := newTestMCPHandler(newFakeSDKServer(nil))
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

		configuredServer := authEnabledServer("secure", compozyconfig.MCPServerTransportHTTP, testServer.URL)
		target := globalMCPExecutorTarget("secure")
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(context.Background(), mcpauth.TokenRecord{
			Target:                target,
			DefinitionFingerprint: definitionFingerprint(t, target, configuredServer),
			Issuer:                "https://issuer.example.test",
			ClientID:              "client-id",
			Scopes:                []string{"tools.read"},
			AccessToken:           "secret-access",
			RefreshToken:          "secret-refresh",
			TokenType:             "Bearer",
			ExpiresAt:             time.Now().Add(time.Hour),
			UpdatedAt:             time.Now(),
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

	t.Run("Should resolve a persisted DCR client secret through the executor auth service", func(t *testing.T) {
		t.Parallel()

		oauthFixture := mcpfixture.StartOAuthHTTP(
			t,
			mcpfixture.MustNew(mcpfixture.ProfileModern2026),
			mcpfixture.OAuthConfig{},
		)
		server := compozyconfig.MCPServer{
			Name:      "fixture",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       oauthFixture.Endpoints.MCPURL,
			Auth: compozyconfig.MCPAuthConfig{
				Registration: compozyconfig.MCPAuthRegistrationAuto,
				Scopes:       []string{"tools.read"},
			},
		}
		target := globalMCPExecutorTarget("fixture")
		cfg, err := mcpauth.ServerConfigFromMCP(t.Context(), target, server, nil)
		if err != nil {
			t.Fatalf("ServerConfigFromMCP() error = %v", err)
		}
		fingerprint, err := mcpauth.ServerDefinitionFingerprint(cfg)
		if err != nil {
			t.Fatalf("ServerDefinitionFingerprint() error = %v", err)
		}
		const (
			redirectURL = "http://127.0.0.1:2123/api/mcp/oauth/callback"
			secretRef   = "vault:mcp/test-dcr-client-secret"
			secretValue = "persisted-client-secret"
		)
		store := newMemoryTokenStore()
		_, err = store.SaveMCPAuthRegistration(t.Context(), mcpauth.ClientRegistration{
			Target:                  target,
			DefinitionFingerprint:   fingerprint,
			ResourceURL:             oauthFixture.Endpoints.MCPURL,
			Issuer:                  oauthFixture.Endpoints.IssuerURL,
			ClientID:                "persisted-client-id",
			ClientSecretRef:         secretRef,
			TokenEndpointAuthMethod: "client_secret_post",
			RedirectURL:             redirectURL,
			Scopes:                  []string{"tools.read"},
		}, mcpauth.RegistrationSecrets{})
		if err != nil {
			t.Fatalf("SaveMCPAuthRegistration() error = %v", err)
		}
		resolvedRefs := make(chan string, 1)
		executor, err := NewMCPCallExecutor(
			ServerResolverFunc(func(context.Context, toolspkg.SourceRef) (ResolvedServer, error) {
				return ResolvedServer{Server: server, Target: target}, nil
			}),
			WithTokenStore(store),
			WithHTTPClient(oauthFixture.Server.Client()),
			WithSecretResolver(secretRefResolverFunc(func(_ context.Context, ref string) (string, error) {
				select {
				case resolvedRefs <- ref:
				default:
				}
				return secretValue, nil
			})),
		)
		if err != nil {
			t.Fatalf("NewMCPCallExecutor() error = %v", err)
		}
		service, ok := executor.auth.(*mcpauth.Service)
		if !ok {
			t.Fatalf("executor auth service = %T, want *auth.Service", executor.auth)
		}
		state, err := service.BeginLogin(t.Context(), cfg, redirectURL)
		if err != nil {
			t.Fatalf("BeginLogin() error = %v", err)
		}
		select {
		case got := <-resolvedRefs:
			if want := secretRef; got != want {
				t.Fatalf("resolved secret ref = %q, want %q", got, want)
			}
		default:
			t.Fatal("persisted DCR client secret was not resolved")
		}
		if got, want := state.Config.ClientSecret, secretValue; got != want {
			t.Fatalf("resolved client secret = %q, want configured persisted secret", got)
		}
		if got, want := state.Config.ClientID, "persisted-client-id"; got != want {
			t.Fatalf("resolved client id = %q, want %q", got, want)
		}
	})

	t.Run("Should never send a token bound to a replaced server definition", func(t *testing.T) {
		t.Parallel()

		target := globalMCPExecutorTarget("secure")
		original := authEnabledServer(
			"secure",
			compozyconfig.MCPServerTransportHTTP,
			"https://original.example/mcp",
		)
		replacement := authEnabledServer(
			"secure",
			compozyconfig.MCPServerTransportHTTP,
			"https://replacement.example/mcp",
		)
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(t.Context(), mcpauth.TokenRecord{
			Target:                target,
			DefinitionFingerprint: definitionFingerprint(t, target, original),
			Issuer:                "https://issuer.example.test",
			ClientID:              "client-id",
			AccessToken:           "original-secret",
			TokenType:             "Bearer",
			ExpiresAt:             time.Now().Add(time.Hour),
			UpdatedAt:             time.Now(),
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
		requireReason(t, err, toolspkg.ReasonMCPAuthInvalid)
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
			server compozyconfig.MCPServer
		}{
			{
				record: mcpauth.TokenRecord{
					Target:      workspaceA,
					Issuer:      "https://issuer.example.test",
					ClientID:    "client-id",
					Scopes:      []string{"tools.read"},
					AccessToken: "workspace-a-token",
					TokenType:   "Bearer",
					ExpiresAt:   time.Now().Add(time.Hour),
					UpdatedAt:   time.Now(),
				},
				server: authEnabledServer("linear", compozyconfig.MCPServerTransportHTTP, serverA.URL),
			},
			{
				record: mcpauth.TokenRecord{
					Target:      workspaceB,
					Issuer:      "https://issuer.example.test",
					ClientID:    "client-id",
					Scopes:      []string{"tools.read"},
					AccessToken: "workspace-b-token",
					TokenType:   "Bearer",
					ExpiresAt:   time.Now().Add(time.Hour),
					UpdatedAt:   time.Now(),
				},
				server: authEnabledServer("linear", compozyconfig.MCPServerTransportHTTP, serverB.URL),
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
					Server: authEnabledServer("linear", compozyconfig.MCPServerTransportHTTP, serverA.URL),
					Target: workspaceA,
				}, nil
			case "mcp-workspace-b":
				return ResolvedServer{
					Server: authEnabledServer("linear", compozyconfig.MCPServerTransportHTTP, serverB.URL),
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
			authEnabledServer("secure", compozyconfig.MCPServerTransportHTTP, "http://127.0.0.1:1/mcp"),
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

		server := authEnabledServer("secure", compozyconfig.MCPServerTransportHTTP, "http://127.0.0.1:1/mcp")
		target := globalMCPExecutorTarget("secure")
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(context.Background(), mcpauth.TokenRecord{
			Target:                target,
			DefinitionFingerprint: definitionFingerprint(t, target, server),
			Issuer:                "https://issuer.example.test",
			ClientID:              "client-id",
			Scopes:                []string{"tools.read"},
			RefreshToken:          "secret-refresh",
			TokenType:             "Bearer",
			ExpiresAt:             time.Now().Add(time.Hour),
			UpdatedAt:             time.Now(),
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
				AuthType:     "oauth",
				ClientID:     "client-id",
				TokenPresent: true,
				Refreshable:  true,
			},
			refreshErr: errors.New("token endpoint leaked secret-refresh"),
		}
		executor := newTestMCPExecutor(
			t,
			authEnabledServer("secure", compozyconfig.MCPServerTransportHTTP, "http://127.0.0.1:1/mcp"),
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

		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "github",
			Transport: compozyconfig.MCPServerTransportHTTP,
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
			compozyconfig.MCPServer{
				Name:      "slow",
				Transport: compozyconfig.MCPServerTransportHTTP,
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

		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "terminated",
			Transport: compozyconfig.MCPServerTransportStdio,
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
}

func TestMCPRequestSecretLifecycle(t *testing.T) {
	t.Run("Should scope HTTP bearer redaction to successful failed and canceled requests", func(t *testing.T) {
		t.Parallel()

		for _, test := range []struct {
			name   string
			mode   string
			secret string
		}{
			{
				name:   "Should redact a short bearer during a successful request",
				mode:   "success",
				secret: "abc",
			},
			{
				name:   "Should redact a bearer during a failed request",
				mode:   "failure",
				secret: "mcp-lifecycle-credential-failure",
			},
			{
				name:   "Should redact a bearer during a canceled request",
				mode:   "canceled",
				secret: "mcp-lifecycle-credential-canceled",
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				secret := test.secret
				if got := diagnostics.Redact("diagnostic " + secret); !strings.Contains(got, secret) {
					t.Fatalf("diagnostic baseline = %q, want unregistered secret", got)
				}

				observedRedaction := make(chan string, 1)
				sdkServer := newFakeSDKServer(nil)
				releaseCanceledRequest := func() {}
				if test.mode == "canceled" {
					requestRelease := make(chan struct{})
					var releaseOnce sync.Once
					releaseCanceledRequest = func() {
						releaseOnce.Do(func() {
							close(requestRelease)
						})
					}
					t.Cleanup(releaseCanceledRequest)
					sdkServer.AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
						return func(
							ctx context.Context,
							method string,
							request mcpsdk.Request,
						) (mcpsdk.Result, error) {
							if method != "tools/list" {
								return next(ctx, method, request)
							}
							select {
							case observedRedaction <- diagnostics.Redact("diagnostic " + secret):
							default:
							}
							<-requestRelease
							return nil, errors.New("test: released tools/list request")
						}
					})
				}
				mcpHandler := newTestMCPHandler(sdkServer)
				server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					if got, want := request.Header.Get("Authorization"), "Bearer "+secret; got != want {
						http.Error(writer, "missing authorization header", http.StatusUnauthorized)
						return
					}
					if test.mode != "canceled" {
						select {
						case observedRedaction <- diagnostics.Redact("diagnostic " + secret):
						default:
						}
					}
					switch test.mode {
					case "failure":
						http.Error(
							writer,
							"upstream failure "+base64.StdEncoding.EncodeToString([]byte(secret)),
							http.StatusInternalServerError,
						)
					default:
						mcpHandler.ServeHTTP(writer, request)
					}
				}))
				t.Cleanup(server.Close)

				configuredServer := authEnabledServer(
					"lifecycle-"+strings.ReplaceAll(test.mode, "_", "-"),
					compozyconfig.MCPServerTransportHTTP,
					server.URL,
				)
				target := globalMCPExecutorTarget(configuredServer.Name)
				store := newMemoryTokenStore()
				if err := store.SaveMCPAuthToken(t.Context(), mcpauth.TokenRecord{
					Target:                target,
					DefinitionFingerprint: definitionFingerprint(t, target, configuredServer),
					Issuer:                "https://issuer.example.test",
					ClientID:              "client-id",
					Scopes:                []string{"tools.read"},
					AccessToken:           secret,
					TokenType:             "Bearer",
					ExpiresAt:             time.Now().Add(time.Hour),
					UpdatedAt:             time.Now(),
				}); err != nil {
					t.Fatalf("SaveMCPAuthToken() error = %v", err)
				}
				executor := newTestMCPExecutor(t, configuredServer, WithTokenStore(store))
				source := toolspkg.SourceRef{
					Kind:          toolspkg.SourceMCP,
					Owner:         configuredServer.Name,
					RawServerName: configuredServer.Name,
				}

				var err error
				switch test.mode {
				case "canceled":
					ctx, cancel := context.WithCancel(t.Context())
					defer cancel()
					result := make(chan error, 1)
					go func() {
						_, callErr := executor.ListTools(ctx, source)
						result <- callErr
					}()
					select {
					case redacted := <-observedRedaction:
						if strings.Contains(redacted, secret) {
							t.Fatalf("in-flight diagnostic = %q, leaked bearer", redacted)
						}
					case <-time.After(time.Second):
						t.Fatal("server did not observe an in-flight bearer redaction")
					}
					cancel()
					select {
					case err = <-result:
						releaseCanceledRequest()
					case <-time.After(time.Second):
						releaseCanceledRequest()
						select {
						case <-result:
						case <-time.After(time.Second):
							t.Fatal("ListTools() remained blocked after the server request was released")
						}
						t.Fatal("ListTools() did not return after cancellation")
					}
					requireReason(t, err, toolspkg.ReasonCallCanceled)
				default:
					_, err = executor.ListTools(t.Context(), source)
					if test.mode == "success" && err != nil {
						t.Fatalf("ListTools() error = %v", err)
					}
					if test.mode == "failure" && err == nil {
						t.Fatal("ListTools() error = nil, want upstream failure")
					}
					select {
					case redacted := <-observedRedaction:
						if strings.Contains(redacted, secret) {
							t.Fatalf("in-flight diagnostic = %q, leaked bearer", redacted)
						}
					case <-time.After(time.Second):
						t.Fatal("server did not observe an in-flight bearer redaction")
					}
				}
				if err != nil {
					requireSecretAbsentFromReversibleString(t, "ListTools() error", err.Error(), secret)
				}
				if got := diagnostics.Redact("diagnostic " + secret); !strings.Contains(got, secret) {
					t.Fatalf("post-request diagnostic = %q, retained bearer", got)
				}
			})
		}
	})

	t.Run("Should sanitize a reflected short bearer before result cache and provider projection", func(t *testing.T) {
		const secret = "a?c"
		if got := diagnostics.Redact("diagnostic " + secret); !strings.Contains(got, secret) {
			t.Fatalf("diagnostic baseline = %q, want unregistered bearer", got)
		}

		var listCalls atomic.Int32
		sdkServer := newReflectingSDKServer(secret, &listCalls)
		mcpHandler := newTestMCPHandler(sdkServer)
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if got, want := request.Header.Get("Authorization"), "Bearer "+secret; got != want {
				http.Error(writer, "missing authorization header", http.StatusUnauthorized)
				return
			}
			mcpHandler.ServeHTTP(writer, request)
		}))
		t.Cleanup(server.Close)

		configuredServer := authEnabledServer(
			"reflection",
			compozyconfig.MCPServerTransportHTTP,
			server.URL,
		)
		target := globalMCPExecutorTarget(configuredServer.Name)
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(t.Context(), mcpauth.TokenRecord{
			Target:                target,
			DefinitionFingerprint: definitionFingerprint(t, target, configuredServer),
			Issuer:                "https://issuer.example.test",
			ClientID:              "client-id",
			Scopes:                []string{"tools.read"},
			AccessToken:           secret,
			TokenType:             "Bearer",
			ExpiresAt:             time.Now().Add(time.Hour),
			UpdatedAt:             time.Now(),
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken() error = %v", err)
		}
		executor := newTestMCPExecutor(t, configuredServer, WithTokenStore(store))
		source := toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         configuredServer.Name,
			RawServerName: configuredServer.Name,
		}

		descriptors, err := executor.ListTools(t.Context(), source)
		if err != nil {
			t.Fatalf("ListTools() error = %v", err)
		}
		if got, want := len(descriptors), 1; got != want {
			t.Fatalf("descriptor count = %d, want %d", got, want)
		}
		requireSecretAbsentFromJSON(t, "discovered descriptors", descriptors, secret)

		cached, err := executor.ListTools(t.Context(), source)
		if err != nil {
			t.Fatalf("ListTools(cached) error = %v", err)
		}
		requireSecretAbsentFromJSON(t, "cached descriptors", cached, secret)
		if got, want := listCalls.Load(), int32(1); got != want {
			t.Fatalf("tools/list calls = %d, want %d after cache hit", got, want)
		}

		provider, err := toolspkg.NewMCPProvider(
			toolspkg.MCPSourceListerFunc(func(context.Context) ([]toolspkg.SourceRef, error) {
				return []toolspkg.SourceRef{source}, nil
			}),
			executor,
			executor,
		)
		if err != nil {
			t.Fatalf("NewMCPProvider() error = %v", err)
		}
		projected, err := provider.List(t.Context(), toolspkg.Scope{})
		if err != nil {
			t.Fatalf("MCPProvider.List() error = %v", err)
		}
		requireSecretAbsentFromJSON(t, "provider projection", projected, secret)
		if got, want := listCalls.Load(), int32(1); got != want {
			t.Fatalf("tools/list calls = %d, want %d after provider projection", got, want)
		}

		result, err := executor.CallTool(t.Context(), source, toolspkg.MCPToolCallRequest{
			ToolID:      descriptors[0].ID,
			RawToolName: descriptors[0].RawName,
			Input:       json.RawMessage(`{}`),
		})
		if err != nil {
			t.Fatalf("CallTool() error = %v", err)
		}
		requireSecretAbsentFromJSON(t, "tool result", result, secret)
		requireMCPBinaryContentRedacted(t, result, secret)
		requireMCPBinaryEnvelopesRedacted(t, result)
		if len(result.Redactions) == 0 {
			t.Fatal("CallTool() redactions = nil, want reflected-credential audit records")
		}
		if err := result.Validate(-1); err != nil {
			t.Fatalf("ToolResult.Validate() error = %v", err)
		}
		if got := diagnostics.Redact("diagnostic " + secret); !strings.Contains(got, secret) {
			t.Fatalf("post-request diagnostic = %q, retained reflected bearer", got)
		}
	})

	t.Run("Should sanitize every exportable tool result field", func(t *testing.T) {
		const secret = "secret?metadata"
		ctx, releaseRedactions := withMCPRequestRedactions(t.Context())
		t.Cleanup(releaseRedactions)
		registerMCPRequestSecret(ctx, secret)
		encodedSecret := base64.StdEncoding.EncodeToString([]byte(secret))
		hexSecret := hex.EncodeToString([]byte(secret))
		dataURISecret := "data:application/octet-stream;base64," + encodedSecret
		percentSecret := url.PathEscape(secret)

		secretJSON, err := json.Marshal(map[string]string{
			encodedSecret:     "value " + hexSecret,
			"data_uri":        dataURISecret,
			"percent_encoded": "credential=" + percentSecret,
			"percent_mixed":   "credential=" + percentSecret + " trailing=%",
		})
		if err != nil {
			t.Fatalf("json.Marshal(secret object) error = %v", err)
		}
		secretValue, err := json.Marshal("value " + dataURISecret)
		if err != nil {
			t.Fatalf("json.Marshal(secret value) error = %v", err)
		}
		result, err := redactMCPToolResult(toolspkg.ToolResult{
			Content: []toolspkg.ToolContent{{
				Type:     "type/" + encodedSecret,
				Text:     "text " + hexSecret,
				Data:     secretJSON,
				MIMEType: "mime/" + dataURISecret,
				Metadata: map[string]json.RawMessage{encodedSecret: secretValue},
			}},
			Structured: secretJSON,
			Preview:    "preview " + encodedSecret,
			Artifacts: []toolspkg.ArtifactRef{{
				URI:      "artifact://" + encodedSecret,
				Name:     "name " + hexSecret,
				MIMEType: "mime/" + dataURISecret,
				SHA256:   "sha256-" + encodedSecret,
			}},
			Metadata: map[string]json.RawMessage{hexSecret: secretValue},
			Redactions: []toolspkg.Redaction{{
				Path:   "$." + dataURISecret,
				Reason: toolspkg.ReasonCode(encodedSecret),
				Bytes:  int64(len(secret)),
			}},
		})
		if err != nil {
			t.Fatalf("redactMCPToolResult() error = %v", err)
		}
		requireSecretAbsentFromJSON(t, "fully populated tool result", result, secret)
		if err := result.Validate(-1); err != nil {
			t.Fatalf("ToolResult.Validate() error = %v", err)
		}

		releaseRedactions()
		if got := diagnostics.Redact("diagnostic " + secret); !strings.Contains(got, secret) {
			t.Fatalf("post-result diagnostic = %q, retained result secret", got)
		}
	})

	t.Run("Should release two required credentials once in LIFO order", func(t *testing.T) {
		t.Parallel()

		const firstSecret = "r1!"
		const secondSecret = "s2?"
		cleanupOrder := make([]string, 0, 2)
		redactions := &mcpRequestRedactions{
			registerSecret: func(value string) func() {
				cleanup := diagnostics.RegisterRequiredSecret(value)
				return func() {
					cleanup()
					cleanupOrder = append(cleanupOrder, value)
				}
			},
		}
		t.Cleanup(redactions.cleanup)

		redactions.register(firstSecret)
		redactions.register(secondSecret)
		for _, secret := range []string{firstSecret, secondSecret} {
			if got := diagnostics.Redact("diagnostic " + secret); strings.Contains(got, secret) {
				t.Fatalf("in-flight diagnostic = %q, leaked required credential", got)
			}
		}

		redactions.cleanup()
		redactions.cleanup()
		if want := []string{secondSecret, firstSecret}; !slices.Equal(cleanupOrder, want) {
			t.Fatalf("cleanup order = %#v, want %#v", cleanupOrder, want)
		}
		for _, secret := range []string{firstSecret, secondSecret} {
			if got := diagnostics.Redact("diagnostic " + secret); !strings.Contains(got, secret) {
				t.Fatalf("post-cleanup diagnostic = %q, retained required credential", got)
			}
		}
	})

	t.Run("Should rebuild tool errors without retaining dynamically resolved causes", func(t *testing.T) {
		t.Parallel()

		const secret = "mcp?resolved-lifecycle-secret"
		ctx, releaseRedactions := withMCPRequestRedactions(t.Context())
		t.Cleanup(releaseRedactions)
		executor := &CallExecutor{lookupSecret: func(string) string { return secret }}
		for range 2 {
			if _, err := executor.resolveSecretRef(ctx, "env:MCP_LIFECYCLE_SECRET"); err != nil {
				t.Fatalf("resolveSecretRef() error = %v", err)
			}
		}
		if got := diagnostics.Redact("diagnostic " + secret); strings.Contains(got, secret) {
			t.Fatalf("in-flight diagnostic = %q, leaked resolved secret", got)
		}
		encodedSecret := base64.StdEncoding.EncodeToString([]byte(secret))
		hexSecret := hex.EncodeToString([]byte(secret))
		dataURISecret := "data:application/octet-stream;base64," + encodedSecret
		percentSecret := url.PathEscape(secret)

		original := toolspkg.NewToolError(
			toolspkg.ErrorCodeTimedOut,
			toolspkg.ToolID("mcp__fixture__"+encodedSecret),
			"upstream leaked credential="+percentSecret,
			fmt.Errorf("upstream leaked %s: %w", hexSecret, context.DeadlineExceeded),
			toolspkg.ReasonCallTimedOut,
		)
		original.Operator = &toolspkg.OperatorFailure{
			Cause:    "operator saw prefix " + dataURISecret + " suffix",
			Recovery: "remove " + hexSecret,
		}
		original.PartialResult = &toolspkg.ToolResult{
			Content: []toolspkg.ToolContent{{Type: "text", Text: encodedSecret}},
		}
		redacted := redactMCPError(original)

		toolErr, toolErrMatched := errors.AsType[*toolspkg.ToolError](redacted)
		if !toolErrMatched {
			t.Fatalf("redactMCPError() error = %T, want *tools.ToolError", redacted)
		}
		if got, want := toolErr.Code, toolspkg.ErrorCodeTimedOut; got != want {
			t.Fatalf("redacted tool error code = %q, want %q", got, want)
		}
		if toolErr.PartialResult != nil {
			t.Fatalf("redacted tool error partial result = %#v, want omitted opaque result", toolErr.PartialResult)
		}
		if toolErr.Operator == nil {
			t.Fatalf("redacted tool error operator = %#v, leaked secret", toolErr.Operator)
		}
		requireSecretAbsentFromReversibleString(t, "tool error operator cause", toolErr.Operator.Cause, secret)
		requireSecretAbsentFromReversibleString(t, "tool error operator recovery", toolErr.Operator.Recovery, secret)
		if !errors.Is(redacted, toolspkg.ErrToolTimedOut) {
			t.Fatalf("redactMCPError() = %v, want timeout classification", redacted)
		}
		if !errors.Is(redacted, context.DeadlineExceeded) {
			t.Fatalf("redactMCPError() = %v, want context deadline cause", redacted)
		}
		requireMCPErrorTreeRedacted(t, redacted, secret)

		unknown := redactMCPError(&testMCPSecretError{secret: "credential=" + percentSecret})
		if !errors.Is(unknown, toolspkg.ErrToolBackendFailed) {
			t.Fatalf("opaque upstream error = %v, want backend failure classification", unknown)
		}
		originalType, originalTypeMatched := errors.AsType[*testMCPSecretError](unknown)
		if originalTypeMatched {
			t.Fatalf("opaque upstream error = %T, retained unsupported secret-bearing type", originalType)
		}
		requireMCPErrorTreeRedacted(t, unknown, secret)

		releaseRedactions()
		if got := diagnostics.Redact("diagnostic " + secret); !strings.Contains(got, secret) {
			t.Fatalf("post-request diagnostic = %q, retained resolved secret", got)
		}
	})

	t.Run("Should admit required secret registration before cleanup can return", func(t *testing.T) {
		t.Parallel()

		const secret = "mcp-registration-admission-secret"
		registrationEntered := make(chan struct{})
		allowRegistration := make(chan struct{})
		registrationDone := make(chan struct{})
		cleanupStarted := make(chan struct{})
		cleanupDone := make(chan struct{})
		unregistered := make(chan struct{})
		redactions := &mcpRequestRedactions{
			registerSecret: func(value string) func() {
				close(registrationEntered)
				<-allowRegistration
				cleanup := diagnostics.RegisterRequiredSecret(value)
				return func() {
					cleanup()
					close(unregistered)
				}
			},
		}

		go func() {
			redactions.register(secret)
			close(registrationDone)
		}()
		<-registrationEntered
		go func() {
			close(cleanupStarted)
			redactions.cleanup()
			close(cleanupDone)
		}()
		<-cleanupStarted
		select {
		case <-cleanupDone:
			t.Fatal("cleanup returned while dynamic secret registration was still being admitted")
		default:
		}

		close(allowRegistration)
		select {
		case <-registrationDone:
		case <-time.After(time.Second):
			t.Fatal("required secret registration did not complete")
		}
		select {
		case <-cleanupDone:
		case <-time.After(time.Second):
			t.Fatal("dynamic secret cleanup did not complete")
		}
		select {
		case <-unregistered:
		case <-time.After(time.Second):
			t.Fatal("required secret cleanup was not released")
		}
		if got := diagnostics.Redact("diagnostic " + secret); !strings.Contains(got, secret) {
			t.Fatalf("post-cleanup diagnostic = %q, retained admitted secret", got)
		}
	})
}

func TestMCPClientCleanup(t *testing.T) {
	t.Parallel()

	t.Run("Should join one cleanup failure without losing the primary cause", func(t *testing.T) {
		t.Parallel()

		primary := errors.New("primary failure")
		cleanupFailure := errors.New("cleanup failure")
		cleanupCalls := 0
		err := joinMCPClientCleanup(primary, &managedMCPClient{cleanup: func() error {
			cleanupCalls++
			return cleanupFailure
		}})
		if got, want := cleanupCalls, 1; got != want {
			t.Fatalf("cleanup calls = %d, want %d", got, want)
		}
		if !errors.Is(err, primary) {
			t.Fatalf("joined cleanup error = %v, want primary failure", err)
		}
		if !errors.Is(err, cleanupFailure) {
			t.Fatalf("joined cleanup error = %v, want cleanup failure", err)
		}
	})
}

// Invariant: authoritative MCP projections change only with logical descriptor or credential state and retain no expired cache state.
// Owning layer: internal/mcp unit cache.
// Canonical suite: internal/mcp/executor_test.go.
func TestMCPToolListCache(t *testing.T) {
	t.Run("Should isolate cached descriptor schemas from caller mutation", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		executor := &CallExecutor{toolCache: mcpToolListCache{entries: make(map[string]mcpToolListCacheEntry)}}
		descriptors := []toolspkg.MCPToolDescriptor{{
			ID:           "mcp__fixture__echo",
			InputSchema:  json.RawMessage(`{"type":"object"}`),
			OutputSchema: json.RawMessage(`{"type":"string"}`),
		}}
		executor.cacheTools("fixture", descriptors, 1_000, now)
		descriptors[0].InputSchema[0] = '['

		first, ok := executor.cachedTools("fixture", now)
		if !ok {
			t.Fatal("cachedTools() miss, want hit")
		}
		if got, want := string(first[0].InputSchema), `{"type":"object"}`; got != want {
			t.Fatalf("cached input schema = %q, want %q", got, want)
		}
		first[0].OutputSchema[0] = '['
		second, ok := executor.cachedTools("fixture", now)
		if !ok {
			t.Fatal("cachedTools() second miss, want hit")
		}
		if got, want := string(second[0].OutputSchema), `{"type":"string"}`; got != want {
			t.Fatalf("cached output schema = %q, want %q", got, want)
		}
	})

	t.Run("Should prune detached expired entries while caching a new binding", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		executor := &CallExecutor{toolCache: mcpToolListCache{entries: make(map[string]mcpToolListCacheEntry)}}
		executor.cacheTools("expired-binding", nil, 1, now)
		executor.cacheTools("current-binding", nil, 1_000, now.Add(time.Millisecond))

		executor.toolCache.mu.Lock()
		_, retainedExpired := executor.toolCache.entries["expired-binding"]
		entryCount := len(executor.toolCache.entries)
		entries := maps.Clone(executor.toolCache.entries)
		executor.toolCache.mu.Unlock()
		if retainedExpired || entryCount != 1 {
			t.Fatalf("cache entries = %#v, want only current binding", entries)
		}
	})

	t.Run("Should expose only live cacheable tool projections as authoritative", func(t *testing.T) {
		t.Parallel()

		executor := &CallExecutor{toolCache: mcpToolListCache{
			entries:          make(map[string]mcpToolListCacheEntry),
			projectionStates: make(map[string]mcpToolProjectionState),
		}}
		source := toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "fixture",
			RawServerName: "fixture",
			Scope:         "global",
		}
		first := []toolspkg.MCPToolDescriptor{{
			ID:          "mcp__fixture__alpha",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}}
		if err := executor.recordToolProjection(source, "", first, 60_000, time.Now()); err != nil {
			t.Fatalf("recordToolProjection(first) error = %v", err)
		}
		firstGeneration, known := executor.ProjectionGeneration(t.Context(), []toolspkg.SourceRef{source})
		if !known || firstGeneration == "" {
			t.Fatalf("ProjectionGeneration(first) = %q, %t, want authoritative token", firstGeneration, known)
		}

		second := []toolspkg.MCPToolDescriptor{{
			ID:          "mcp__fixture__beta",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}}
		if err := executor.recordToolProjection(source, "", second, 60_000, time.Now()); err != nil {
			t.Fatalf("recordToolProjection(second) error = %v", err)
		}
		secondGeneration, known := executor.ProjectionGeneration(t.Context(), []toolspkg.SourceRef{source})
		if !known || secondGeneration == firstGeneration {
			t.Fatalf(
				"ProjectionGeneration(second) = %q, %t, want token different from %q",
				secondGeneration,
				known,
				firstGeneration,
			)
		}

		if err := executor.recordToolProjection(source, "", second, 0, time.Now()); err != nil {
			t.Fatalf("recordToolProjection(uncacheable) error = %v", err)
		}
		if generation, authoritative := executor.ProjectionGeneration(
			t.Context(),
			[]toolspkg.SourceRef{source},
		); authoritative || generation != "" {
			t.Fatalf(
				"ProjectionGeneration(uncacheable) = %q, %t, want unknown",
				generation,
				authoritative,
			)
		}
		executor.toolCache.mu.Lock()
		projectionStateCount := len(executor.toolCache.projectionStates)
		executor.toolCache.mu.Unlock()
		if projectionStateCount != 0 {
			t.Fatalf("projection state count = %d, want 0 after uncacheable response", projectionStateCount)
		}
	})

	t.Run("Should preserve generation for reordered equivalent descriptors", func(t *testing.T) {
		t.Parallel()

		executor := &CallExecutor{toolCache: mcpToolListCache{
			projectionStates: make(map[string]mcpToolProjectionState),
		}}
		source := toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "fixture",
			RawServerName: "fixture",
			Scope:         "global",
		}
		alpha := toolspkg.MCPToolDescriptor{ID: "mcp__fixture__alpha"}
		beta := toolspkg.MCPToolDescriptor{ID: "mcp__fixture__beta"}
		now := time.Now()
		if err := executor.recordToolProjection(
			source,
			"",
			[]toolspkg.MCPToolDescriptor{alpha, beta},
			60_000,
			now,
		); err != nil {
			t.Fatalf("recordToolProjection(initial) error = %v", err)
		}
		initial, known := executor.ProjectionGeneration(t.Context(), []toolspkg.SourceRef{source})
		if !known || initial == "" {
			t.Fatalf("ProjectionGeneration(initial) = %q, %t, want authoritative generation", initial, known)
		}

		reorderedDescriptors := []toolspkg.MCPToolDescriptor{beta, alpha}
		if err := executor.recordToolProjection(
			source,
			"",
			reorderedDescriptors,
			60_000,
			now.Add(time.Millisecond),
		); err != nil {
			t.Fatalf("recordToolProjection(reordered) error = %v", err)
		}
		if got, want := []toolspkg.ToolID{
			reorderedDescriptors[0].ID,
			reorderedDescriptors[1].ID,
		}, []toolspkg.ToolID{beta.ID, alpha.ID}; !slices.Equal(got, want) {
			t.Fatalf("reordered descriptors = %#v, want original server order %#v", got, want)
		}
		reordered, known := executor.ProjectionGeneration(t.Context(), []toolspkg.SourceRef{source})
		if !known || reordered != initial {
			t.Fatalf(
				"ProjectionGeneration(reordered) = %q, %t, want unchanged %q",
				reordered,
				known,
				initial,
			)
		}
	})

	t.Run("Should reject a projection state recorded with a replaced authorization token", func(t *testing.T) {
		t.Parallel()

		server := authEnabledServer(
			"fixture",
			compozyconfig.MCPServerTransportHTTP,
			"https://mcp.example.test/mcp",
		)
		auth := &fakeAuthService{}
		auth.setAuthorizationToken(mcpauth.TokenRecord{AccessToken: "first-token", TokenType: "Bearer"})
		executor := newTestMCPExecutor(t, server, withAuthService(auth))
		source := toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "fixture",
			RawServerName: "fixture",
			Scope:         "global",
		}
		descriptors := []toolspkg.MCPToolDescriptor{{
			ID:          "mcp__fixture__alpha",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}}
		if err := executor.recordToolProjection(
			source,
			"Bearer first-token",
			descriptors,
			60_000,
			time.Now(),
		); err != nil {
			t.Fatalf("recordToolProjection(first token) error = %v", err)
		}
		firstGeneration, known := executor.ProjectionGeneration(t.Context(), []toolspkg.SourceRef{source})
		if !known || firstGeneration == "" {
			t.Fatalf("ProjectionGeneration(first token) = %q, %t, want authoritative token", firstGeneration, known)
		}

		auth.setAuthorizationToken(mcpauth.TokenRecord{AccessToken: "second-token", TokenType: "Bearer"})
		if generation, authoritative := executor.ProjectionGeneration(
			t.Context(),
			[]toolspkg.SourceRef{source},
		); authoritative || generation != "" {
			t.Fatalf(
				"ProjectionGeneration(replaced token) = %q, %t, want unknown",
				generation,
				authoritative,
			)
		}
	})

	t.Run("Should bind one authorization snapshot to discovery and cache writes", func(t *testing.T) {
		t.Parallel()

		var listCalls atomic.Int32
		sdkServer := newFakeSDKServer(nil)
		sdkServer.AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
			return func(ctx context.Context, method string, request mcpsdk.Request) (mcpsdk.Result, error) {
				result, err := next(ctx, method, request)
				if err != nil || method != mcpToolsListMethod {
					return result, err
				}
				listCalls.Add(1)
				listResult, ok := result.(*mcpsdk.ListToolsResult)
				if !ok {
					return nil, errors.New("test: tools/list returned an unexpected result type")
				}
				listResult.TTLMs = 60_000
				return listResult, nil
			}
		})
		mcpHandler := newTestMCPHandler(sdkServer)
		store := newMemoryTokenStore()
		var rotatedToken mcpauth.TokenRecord
		rotationResult := make(chan error, 1)
		var rotateOnce sync.Once
		var headersMu sync.Mutex
		headers := make([]string, 0, 4)
		server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			headersMu.Lock()
			headers = append(headers, request.Header.Get("Authorization"))
			headersMu.Unlock()
			rotateOnce.Do(func() {
				rotationResult <- store.SaveMCPAuthToken(request.Context(), rotatedToken)
			})
			mcpHandler.ServeHTTP(writer, request)
		}))
		t.Cleanup(server.Close)

		configuredServer := authEnabledServer(
			"credential-snapshot",
			compozyconfig.MCPServerTransportHTTP,
			server.URL,
		)
		target := globalMCPExecutorTarget(configuredServer.Name)
		firstToken := mcpauth.TokenRecord{
			Target:                target,
			DefinitionFingerprint: definitionFingerprint(t, target, configuredServer),
			Issuer:                "https://issuer.example.test",
			ClientID:              "client-id",
			Scopes:                []string{"tools.read"},
			AccessToken:           "first-token",
			TokenType:             "Bearer",
			ExpiresAt:             time.Now().Add(time.Hour),
			UpdatedAt:             time.Now(),
		}
		rotatedToken = firstToken
		rotatedToken.AccessToken = "second-token"
		rotatedToken.UpdatedAt = firstToken.UpdatedAt.Add(time.Second)
		if err := store.SaveMCPAuthToken(t.Context(), firstToken); err != nil {
			t.Fatalf("SaveMCPAuthToken(first) error = %v", err)
		}
		executor := newTestMCPExecutor(t, configuredServer, WithTokenStore(store))
		source := toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         configuredServer.Name,
			RawServerName: configuredServer.Name,
		}

		if _, err := executor.ListTools(t.Context(), source); err != nil {
			t.Fatalf("ListTools(first token) error = %v", err)
		}
		if err := <-rotationResult; err != nil {
			t.Fatalf("SaveMCPAuthToken(second) error = %v", err)
		}
		headersMu.Lock()
		firstRequestCount := len(headers)
		firstHeaders := append([]string(nil), headers...)
		headersMu.Unlock()
		if firstRequestCount == 0 {
			t.Fatal("first discovery sent no HTTP requests")
		}
		for index, header := range firstHeaders {
			if want := "Bearer first-token"; header != want {
				t.Fatalf("first discovery Authorization[%d] = %q, want %q", index, header, want)
			}
		}

		if _, err := executor.ListTools(t.Context(), source); err != nil {
			t.Fatalf("ListTools(second token) error = %v", err)
		}
		if got, want := listCalls.Load(), int32(2); got != want {
			t.Fatalf("tools/list calls = %d, want %d after credential rotation", got, want)
		}
		headersMu.Lock()
		secondHeaders := append([]string(nil), headers[firstRequestCount:]...)
		headersMu.Unlock()
		if len(secondHeaders) == 0 {
			t.Fatal("second discovery sent no HTTP requests")
		}
		for index, header := range secondHeaders {
			if want := "Bearer second-token"; header != want {
				t.Fatalf("second discovery Authorization[%d] = %q, want %q", index, header, want)
			}
		}
	})

	t.Run("Should prune expired projection states", func(t *testing.T) {
		t.Parallel()

		executor := &CallExecutor{toolCache: mcpToolListCache{
			projectionStates: make(map[string]mcpToolProjectionState),
		}}
		source := toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "fixture",
			RawServerName: "fixture",
			Scope:         "global",
		}
		if err := executor.recordToolProjection(
			source,
			"",
			[]toolspkg.MCPToolDescriptor{{ID: "mcp__fixture__alpha"}},
			1,
			time.Now().Add(-time.Second),
		); err != nil {
			t.Fatalf("recordToolProjection(expired) error = %v", err)
		}
		if generation, authoritative := executor.ProjectionGeneration(
			t.Context(),
			[]toolspkg.SourceRef{source},
		); authoritative || generation != "" {
			t.Fatalf("ProjectionGeneration(expired) = %q, %t, want unknown", generation, authoritative)
		}
		executor.toolCache.mu.Lock()
		projectionStateCount := len(executor.toolCache.projectionStates)
		executor.toolCache.mu.Unlock()
		if projectionStateCount != 0 {
			t.Fatalf("projection state count = %d, want 0 after expiry", projectionStateCount)
		}
	})
}

func TestProtocolVersionMiddleware(t *testing.T) {
	t.Run("Should advertise only the modern and legacy contract versions", func(t *testing.T) {
		t.Parallel()

		versions := []string{
			protocolVersionModern,
			protocolVersionProviderLegacy,
			protocolVersionLegacy,
		}
		handler := protocolVersionMiddleware()(
			func(context.Context, string, mcpsdk.Request) (mcpsdk.Result, error) {
				return &mcpsdk.DiscoverResult{
					SupportedVersions: versions,
				}, nil
			},
		)
		result, err := handler(
			t.Context(),
			"server/discover",
			&mcpsdk.ServerRequest[*mcpsdk.DiscoverParams]{Params: &mcpsdk.DiscoverParams{}},
		)
		if err != nil {
			t.Fatalf("server/discover middleware error = %v", err)
		}
		discover, ok := result.(*mcpsdk.DiscoverResult)
		if !ok {
			t.Fatalf("server/discover result = %T, want *DiscoverResult", result)
		}
		if got, want := discover.SupportedVersions, []string{
			protocolVersionModern,
			protocolVersionLegacy,
		}; !slices.Equal(got, want) {
			t.Fatalf("supported versions = %#v, want %#v", got, want)
		}
		if got, want := versions, []string{
			protocolVersionModern,
			protocolVersionProviderLegacy,
			protocolVersionLegacy,
		}; !slices.Equal(got, want) {
			t.Fatalf("handler versions = %#v, want %#v", got, want)
		}
	})

	t.Run("Should reject legacy initialize versions outside the contract", func(t *testing.T) {
		t.Parallel()

		nextCalled := false
		handler := protocolVersionMiddleware()(func(context.Context, string, mcpsdk.Request) (mcpsdk.Result, error) {
			nextCalled = true
			return nil, nil
		})
		_, err := handler(t.Context(), "initialize", &mcpsdk.ServerRequest[*mcpsdk.InitializeParams]{
			Params: &mcpsdk.InitializeParams{ProtocolVersion: "2025-06-18"},
		})
		unsupported, unsupportedMatched := errors.AsType[*UnsupportedProtocolVersionError](err)
		if !unsupportedMatched || unsupported.Version != "2025-06-18" {
			t.Fatalf("initialize middleware error = %v, want unsupported 2025-06-18", err)
		}
		if nextCalled {
			t.Fatal("initialize middleware called the server for an unsupported version")
		}
	})
}

func TestNormalizeMCPErrorWithContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		wantScope bool
	}{
		{
			name:      "Should preserve an insufficient scope error after the deadline",
			err:       &InsufficientScopeError{Scopes: []string{"tools.read"}},
			wantScope: true,
		},
		{
			name: "Should preserve an authorization requirement after the deadline",
			err:  ErrAuthorizationRequired,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
			defer cancel()
			normalized := normalizeMCPErrorWithContext(ctx, "mcp__fixture__echo", test.err, true)
			requireReason(t, normalized, toolspkg.ReasonMCPAuthRequired)
			if test.wantScope {
				var scopeErr *InsufficientScopeError
				if !errors.As(normalized, &scopeErr) {
					t.Fatalf("normalizeMCPErrorWithContext() error = %v, want InsufficientScopeError", normalized)
				}
				return
			}
			if !errors.Is(normalized, ErrAuthorizationRequired) {
				t.Fatalf("normalizeMCPErrorWithContext() error = %v, want ErrAuthorizationRequired", normalized)
			}
		})
	}
}

func TestConnectClientRejectsUnsupportedNegotiatedProtocol(t *testing.T) {
	t.Run("Should reject an unsupported negotiated protocol", func(t *testing.T) {
		t.Parallel()

		serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
		server := newFakeSDKServer(nil)
		server.AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
			return func(ctx context.Context, method string, request mcpsdk.Request) (mcpsdk.Result, error) {
				if method == "server/discover" {
					return nil, errors.New("legacy peer does not support server/discover")
				}
				result, err := next(ctx, method, request)
				if err != nil || method != "initialize" {
					return result, err
				}
				if initialize, ok := result.(*mcpsdk.InitializeResult); ok {
					initialize.ProtocolVersion = "2025-06-18"
				}
				return result, nil
			}
		})
		serverCtx, cancelServer := context.WithCancel(t.Context())
		defer cancelServer()
		serverErr := make(chan error, 1)
		go func() {
			serverErr <- server.Run(serverCtx, serverTransport)
		}()

		_, err := (&CallExecutor{}).connectClient(t.Context(), clientTransport)
		unsupported, unsupportedMatched := errors.AsType[*UnsupportedProtocolVersionError](err)
		if !unsupportedMatched || unsupported.Version != "2025-06-18" {
			t.Fatalf("connectClient() error = %v, want unsupported 2025-06-18", err)
		}
		cancelServer()
		select {
		case <-serverErr:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for unsupported-protocol peer to close")
		}
	})
}

func TestListMCPTools(t *testing.T) {
	t.Parallel()

	t.Run("Should reject a server that never finishes distinct cursor pagination", func(t *testing.T) {
		t.Parallel()

		serverTransport, clientTransport := mcpsdk.NewInMemoryTransports()
		server := newFakeSDKServer(nil)
		calls := 0
		server.AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
			return func(ctx context.Context, method string, request mcpsdk.Request) (mcpsdk.Result, error) {
				if method == mcpToolsListMethod {
					calls++
					return &mcpsdk.ListToolsResult{NextCursor: fmt.Sprintf("page-%d", calls)}, nil
				}
				return next(ctx, method, request)
			}
		})
		serverCtx, cancelServer := context.WithCancel(t.Context())
		serverErr := make(chan error, 1)
		go func() {
			serverErr <- server.Run(serverCtx, serverTransport)
		}()
		t.Cleanup(func() {
			cancelServer()
			select {
			case <-serverErr:
			case <-time.After(time.Second):
				t.Error("timed out waiting for paginated tool server shutdown")
			}
		})

		session, err := (&CallExecutor{}).connectClient(t.Context(), clientTransport)
		if err != nil {
			t.Fatalf("connectClient() error = %v", err)
		}
		t.Cleanup(func() {
			if closeErr := session.Close(); closeErr != nil {
				t.Errorf("session.Close() error = %v", closeErr)
			}
		})

		_, _, err = listMCPTools(t.Context(), session)
		if err == nil || !strings.Contains(err.Error(), "exceeded maximum page count") {
			t.Fatalf("listMCPTools() error = %v, want page limit error", err)
		}
		if got, want := calls, maxMCPToolListPages; got != want {
			t.Fatalf("tools/list calls = %d, want %d", got, want)
		}
	})
}

func TestCallExecutorDescriptorFromToolPreservesPresentationMetadata(t *testing.T) {
	t.Run("Should preserve valid presentation metadata", func(t *testing.T) {
		t.Parallel()

		executor := &CallExecutor{}
		descriptor, err := executor.descriptorFromTool(
			toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "github"},
			compozyconfig.MCPServer{Name: "github"},
			mcpsdk.Tool{
				Name:        "lookup",
				Description: "Look up an issue",
				InputSchema: json.RawMessage(
					`{"type":"object","properties":{"query":{"type":"string"}}}`,
				),
				Meta: mcpsdk.Meta{
					mcpFriendlyVerbMetadataKey: "Looking up",
					mcpPreviewMetadataKey:      "arg:query",
				},
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
			Meta: mcpsdk.Meta{
				mcpFriendlyVerbMetadataKey: true,
			},
		})
		if err == nil {
			t.Fatal("mcpToolPresentationMetadata() error = nil, want invalid type failure")
		}
	})

	t.Run("Should reject credential material in descriptor identity fields", func(t *testing.T) {
		t.Parallel()

		const secret = "identity_z9"
		cleanup := diagnostics.RegisterRequiredSecret(secret)
		t.Cleanup(cleanup)
		encodedSecret := base64.RawStdEncoding.EncodeToString([]byte(secret))

		executor := &CallExecutor{}
		_, err := executor.descriptorFromTool(
			toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "github"},
			compozyconfig.MCPServer{Name: "github"},
			mcpsdk.Tool{
				Name:        "credential_" + encodedSecret,
				InputSchema: json.RawMessage(`{"type":"object"}`),
			},
		)
		requireReason(t, err, toolspkg.ReasonSecretMetadata)
		requireSecretAbsentFromReversibleString(t, "descriptorFromTool() error", err.Error(), secret)

		for _, test := range []struct {
			name   string
			mutate func(*toolspkg.MCPToolDescriptor)
		}{
			{
				name: "Should reject hexadecimal source owner",
				mutate: func(descriptor *toolspkg.MCPToolDescriptor) {
					descriptor.Source.Owner = hex.EncodeToString([]byte(secret))
				},
			},
			{
				name: "Should reject data URI resource identity",
				mutate: func(descriptor *toolspkg.MCPToolDescriptor) {
					descriptor.Source.ResourceID = "data:application/octet-stream;base64," +
						base64.StdEncoding.EncodeToString([]byte(secret))
				},
			},
		} {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()

				descriptor := safeMCPDescriptorForRedactionTest()
				test.mutate(&descriptor)
				_, redactErr := redactMCPToolDescriptor(descriptor)
				requireReason(t, redactErr, toolspkg.ReasonSecretMetadata)
				requireSecretAbsentFromReversibleString(t, "descriptor identity error", redactErr.Error(), secret)
			})
		}

		collisionSchema, err := json.Marshal(map[string]bool{
			base64.StdEncoding.EncodeToString([]byte(secret)): true,
			"[REDACTED]": false,
		})
		if err != nil {
			t.Fatalf("json.Marshal(collision schema) error = %v", err)
		}
		descriptor := safeMCPDescriptorForRedactionTest()
		descriptor.InputSchema = collisionSchema
		if _, err := redactMCPToolDescriptor(descriptor); err == nil {
			t.Fatal("redactMCPToolDescriptor(collision schema) error = nil, want fail-closed collision")
		}
	})
}

func TestMCPCallExecutorStdioEnvironmentBoundary(t *testing.T) {
	t.Run("Should isolate subprocess homes by target and remove them after use", func(t *testing.T) {
		setMCPTestEnv(t, "HOME", "/operator-home")

		executor := &CallExecutor{}
		server := compozyconfig.MCPServer{
			Name:      "local",
			Transport: compozyconfig.MCPServerTransportStdio,
			Command:   os.Args[0],
		}
		firstTarget := globalMCPExecutorTarget("first")
		secondTarget := globalMCPExecutorTarget("second")
		first, err := executor.newMCPStdioLaunch(t.Context(), server, firstTarget)
		if err != nil {
			t.Fatalf("newMCPStdioLaunch(first) error = %v", err)
		}
		t.Cleanup(func() {
			if cleanupErr := first.cleanup(); cleanupErr != nil {
				t.Errorf("first stdio home cleanup error = %v", cleanupErr)
			}
		})
		second, err := executor.newMCPStdioLaunch(t.Context(), server, secondTarget)
		if err != nil {
			if cleanupErr := first.cleanup(); cleanupErr != nil {
				t.Fatalf("newMCPStdioLaunch(second) error = %v; first cleanup error = %v", err, cleanupErr)
			}
			t.Fatalf("newMCPStdioLaunch(second) error = %v", err)
		}
		t.Cleanup(func() {
			if cleanupErr := second.cleanup(); cleanupErr != nil {
				t.Errorf("second stdio home cleanup error = %v", cleanupErr)
			}
		})

		firstHome := commandEnvironmentValue(t, first.command, "HOME")
		secondHome := commandEnvironmentValue(t, second.command, "HOME")
		if firstHome == "/operator-home" || secondHome == "/operator-home" {
			t.Fatalf("stdio HOME inherited operator home: first=%q second=%q", firstHome, secondHome)
		}
		if firstHome == secondHome {
			t.Fatalf("stdio targets share HOME %q", firstHome)
		}
		for _, pair := range []struct {
			home   string
			target mcpauth.Target
		}{
			{home: firstHome, target: firstTarget},
			{home: secondHome, target: secondTarget},
		} {
			key, err := pair.target.Key()
			if err != nil {
				t.Fatalf("target key error = %v", err)
			}
			base := filepath.Base(pair.home)
			if !strings.HasPrefix(base, mcpStdioHomePrefix(key)) {
				t.Fatalf("isolated stdio HOME base = %q, want portable hashed prefix", base)
			}
		}
		for _, launch := range []mcpStdioLaunch{first, second} {
			home := commandEnvironmentValue(t, launch.command, "HOME")
			for key, suffix := range map[string]string{
				"XDG_CONFIG_HOME":  "config",
				"XDG_CACHE_HOME":   "cache",
				"npm_config_cache": "npm-cache",
				"UV_CACHE_DIR":     "uv-cache",
			} {
				got := commandEnvironmentValue(t, launch.command, key)
				want := filepath.Join(home, suffix)
				if got != want {
					t.Fatalf("%s = %q, want %q", key, got, want)
				}
			}
			if err := launch.cleanup(); err != nil {
				t.Fatalf("stdio home cleanup error = %v", err)
			}
			if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("isolated stdio HOME %q remains after cleanup: %v", home, err)
			}
		}
	})

	t.Run("Should exclude ambient daemon secrets from stdio MCP processes", func(t *testing.T) {
		setMCPTestEnv(t, stdioParentSecretEnv, "ambient-secret")

		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "Local",
			Transport: compozyconfig.MCPServerTransportStdio,
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
		const secret = "q7Z"
		setMCPTestEnv(t, stdioExplicitSecretSource, secret)
		if got := diagnostics.Redact("diagnostic " + secret); !strings.Contains(got, secret) {
			t.Fatalf("diagnostic baseline = %q, want unregistered stdio secret", got)
		}

		executor := newTestMCPExecutor(
			t,
			compozyconfig.MCPServer{
				Name:      "Local",
				Transport: compozyconfig.MCPServerTransportStdio,
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
		requireSecretAbsentFromJSON(t, "stdio discovered descriptor", descriptor, secret)
		if got := diagnostics.Redact("diagnostic " + secret); !strings.Contains(got, secret) {
			t.Fatalf("post-discovery diagnostic = %q, retained stdio secret", got)
		}
		source := toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "Local",
			RawServerName: "Local",
		}
		cached, err := executor.ListTools(t.Context(), source)
		if err != nil {
			t.Fatalf("ListTools(cached stdio) error = %v", err)
		}
		requireSecretAbsentFromJSON(t, "stdio cached descriptors", cached, secret)
		provider, err := toolspkg.NewMCPProvider(
			toolspkg.MCPSourceListerFunc(func(context.Context) ([]toolspkg.SourceRef, error) {
				return []toolspkg.SourceRef{source}, nil
			}),
			executor,
			executor,
		)
		if err != nil {
			t.Fatalf("NewMCPProvider(stdio) error = %v", err)
		}
		projected, err := provider.List(t.Context(), toolspkg.Scope{})
		if err != nil {
			t.Fatalf("MCPProvider.List(stdio) error = %v", err)
		}
		requireSecretAbsentFromJSON(t, "stdio provider projection", projected, secret)
		result := callMCPTool(
			t,
			executor,
			descriptor,
			json.RawMessage("{\"message\":\""+stdioExplicitSecretEnv+"\"}"),
		)
		if got, want := result.Preview, "env: [REDACTED]"; got != want {
			t.Fatalf("result.Preview = %q, want %q", got, want)
		}
		requireSecretAbsentFromJSON(t, "stdio tool result", result, secret)
		requireMCPBinaryContentRedacted(t, result, secret)
		requireMCPEmbeddedBlobRedacted(t, result)
		if len(result.Redactions) == 0 {
			t.Fatal("stdio result redactions = nil, want reflected-credential audit records")
		}
		if got := diagnostics.Redact("diagnostic " + secret); !strings.Contains(got, secret) {
			t.Fatalf("post-call diagnostic = %q, retained stdio secret", got)
		}
	})

	t.Run("Should redact secret env values through client close and cleanup failures", func(t *testing.T) {
		const secret = "m8#"
		setMCPTestEnv(t, stdioExplicitSecretSource, secret)
		encodedSecret := base64.StdEncoding.EncodeToString([]byte(secret))
		hexSecret := hex.EncodeToString([]byte(secret))
		dataURISecret := "data:application/octet-stream;base64," + encodedSecret

		ctx, releaseRedactions := withMCPRequestRedactions(t.Context())
		t.Cleanup(releaseRedactions)
		server := compozyconfig.MCPServer{
			Name:      "local-cleanup",
			Transport: compozyconfig.MCPServerTransportStdio,
			Command:   os.Args[0],
			Args:      []string{"-test.run=TestMCPStdioHelperProcess"},
			Env: map[string]string{
				stdioEnvHelperEnv: "1",
			},
			SecretEnv: map[string]string{
				stdioExplicitSecretEnv: "env:" + stdioExplicitSecretSource,
			},
		}
		executor := newTestMCPExecutor(t, server, WithSecretLookup(os.Getenv))
		resolved := ResolvedServer{Server: server, Target: globalMCPExecutorTarget(server.Name)}
		client, err := executor.openClient(ctx, resolved, "")
		if err != nil {
			t.Fatalf("openClient() error = %v", err)
		}
		if got := diagnostics.Redact("diagnostic " + secret); strings.Contains(got, secret) {
			t.Fatalf("in-flight diagnostic = %q, leaked short secret_env value", got)
		}
		clientClosed := false
		t.Cleanup(func() {
			if clientClosed {
				return
			}
			if closeErr := closeMCPClient(client); closeErr != nil {
				t.Errorf("closeMCPClient() error = %v", redactMCPError(closeErr))
			}
		})
		cleanup := client.cleanup
		client.cleanup = func() error {
			return errors.Join(
				fmt.Errorf("mcp stdio cleanup leaked %s", encodedSecret),
				cleanup(),
			)
		}
		primary := toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			toolspkg.ToolID("mcp__local_cleanup__"+encodedSecret),
			"mcp call leaked "+hexSecret,
			&testMCPSecretError{secret: dataURISecret},
			toolspkg.ReasonMCPUnreachable,
		)
		redacted := redactMCPError(joinMCPClientCleanup(primary, client))
		clientClosed = true
		if !errors.Is(redacted, toolspkg.ErrToolUnavailable) {
			t.Fatalf("joined client cleanup error = %v, want unavailable classification", redacted)
		}
		if !errors.Is(redacted, toolspkg.ErrToolBackendFailed) {
			t.Fatalf("joined client cleanup error = %v, want cleanup backend classification", redacted)
		}
		requireMCPErrorTreeRedacted(t, redacted, secret)

		releaseRedactions()
		if got := diagnostics.Redact("diagnostic " + secret); !strings.Contains(got, secret) {
			t.Fatalf("post-client-cleanup diagnostic = %q, retained stdio secret", got)
		}
	})
}

func commandEnvironmentValue(t *testing.T, command *exec.Cmd, key string) string {
	t.Helper()
	prefix := key + "="
	for _, value := range command.Env {
		if result, ok := strings.CutPrefix(value, prefix); ok {
			return result
		}
	}
	t.Fatalf("command environment has no %s", key)
	return ""
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
			handler := func(
				_ context.Context,
				req *mcpsdk.CallToolRequest,
			) (*mcpsdk.CallToolResult, error) {
				var input struct {
					Message string `json:"message"`
				}
				if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
					return nil, fmt.Errorf("decode environment helper input: %w", err)
				}
				key := input.Message
				value := os.Getenv(key)
				encodedValue := base64.StdEncoding.EncodeToString([]byte(value))
				hexValue := hex.EncodeToString([]byte(value))
				dataURIValue := "data:application/octet-stream;base64," + encodedValue
				textValue := value
				imageMIME := "image/png"
				audioMIME := "audio/mpeg"
				resourceURI := "memory://stdio-secret"
				resourceMIME := "application/octet-stream"
				if value != "" {
					textValue = encodedValue
					imageMIME = "image/" + hexValue
					audioMIME = "audio/" + encodedValue
					resourceURI = "memory://" + encodedValue
					resourceMIME = "application/" + hexValue
				}
				return &mcpsdk.CallToolResult{
					Content: []mcpsdk.Content{
						&mcpsdk.TextContent{Text: "env: " + textValue},
						&mcpsdk.ImageContent{Data: []byte(value), MIMEType: imageMIME},
						&mcpsdk.AudioContent{Data: []byte(value), MIMEType: audioMIME},
						&mcpsdk.EmbeddedResource{
							Resource: &mcpsdk.ResourceContents{
								URI:      resourceURI,
								MIMEType: resourceMIME,
								Blob:     []byte("prefix:" + value + ":suffix"),
							},
							Meta: mcpsdk.Meta{
								"binary":      []byte(value),
								"encoded_key": map[string]any{encodedValue: "visible"},
							},
						},
					},
					StructuredContent: map[string]any{
						"binary": []byte(value),
						"encoded_keys": []any{
							map[string]any{encodedValue: "base64"},
							map[string]any{hex.EncodeToString([]byte(value)): "hex"},
							map[string]any{
								dataURIValue: "data_uri",
							},
						},
						"message": textValue,
					},
				}, nil
			}
			server := newFakeSDKServer(handler)
			reflectedValue := os.Getenv(stdioExplicitSecretEnv)
			if reflectedValue != "" {
				encodedValue := base64.StdEncoding.EncodeToString([]byte(reflectedValue))
				hexValue := hex.EncodeToString([]byte(reflectedValue))
				dataURIValue := "data:application/octet-stream;base64," + encodedValue
				server = newFakeSDKServerWithSchemas(
					handler,
					map[string]any{
						"type":       "object",
						"properties": map[string]any{"message": map[string]any{"type": "string"}},
						"x-encoded-reflections": map[string]any{
							"value": encodedValue,
							"key":   map[string]any{encodedValue: "visible"},
						},
					},
					map[string]any{
						"type": "object",
						"x-encoded-reflections": map[string]any{
							"value": "output " + hexValue,
							"key":   map[string]any{dataURIValue: "visible"},
						},
					},
					"Echo "+encodedValue,
					"Title "+hexValue,
					mcpsdk.Meta{
						mcpFriendlyVerbMetadataKey: "Reading " + encodedValue,
						mcpPreviewMetadataKey:      "auto",
					},
				)
			}
			server.AddReceivingMiddleware(withTestMCPListTTL)
			if err := server.Run(context.Background(), &mcpsdk.StdioTransport{}); err != nil {
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
		if err := newFakeSDKServer(nil).Run(context.Background(), &mcpsdk.StdioTransport{}); err != nil {
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
				AuthType:     "oauth",
				ClientID:     "client-id",
				Scopes:       []string{" tools.read ", "issues.write"},
				ExpiresAt:    &expiresAt,
				TokenPresent: true,
				Refreshable:  true,
			},
		}
		server := authEnabledServer("secure", compozyconfig.MCPServerTransportHTTP, "https://mcp.example.test/mcp")
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

		unconfigured := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "plain",
			Transport: compozyconfig.MCPServerTransportHTTP,
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
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "github",
			Transport: compozyconfig.MCPServerTransportHTTP,
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

		unsupported := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "github",
			Transport: compozyconfig.MCPServerTransport("websocket"),
			URL:       "https://mcp.example.test/ws",
		})
		_, err = unsupported.ListTools(testContext(t), toolspkg.SourceRef{RawServerName: "github"})
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)
	})

	t.Run("Should Normalize Schemas Arguments Content Preview And Errors", func(t *testing.T) {
		t.Parallel()

		rawSchema := json.RawMessage(`{"type":"object","properties":{"q":{"type":"string"}}}`)
		tool := mcpsdk.Tool{Name: "lookup", InputSchema: rawSchema}
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
		output, err := outputSchemaBytes(mcpsdk.Tool{Name: "lookup"})
		if err != nil {
			t.Fatalf("outputSchemaBytes(empty) error = %v", err)
		}
		if output != nil {
			t.Fatalf("outputSchemaBytes(empty) = %s, want nil", output)
		}
		rawOutputSchema := json.RawMessage(`{"type":"object","properties":{"answer":{"type":"string"}}}`)
		output, err = outputSchemaBytes(mcpsdk.Tool{Name: "lookup", OutputSchema: rawOutputSchema})
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
				&mcpsdk.TextContent{Text: "ok"},
				&mcpsdk.ImageContent{Data: []byte("img"), MIMEType: "image/png"},
				&mcpsdk.AudioContent{Data: []byte("audio"), MIMEType: "audio/mpeg"},
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
		embedded, err := toolContentFromMCP(&mcpsdk.EmbeddedResource{})
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
	})

	t.Run("Should Handle Expired Auth Refresh Outcomes", func(t *testing.T) {
		t.Parallel()

		server := authEnabledServer("secure", compozyconfig.MCPServerTransportHTTP, "https://mcp.example.test/mcp")
		fakeAuth := &fakeAuthService{
			status: mcpauth.Status{
				ServerName:   "secure",
				Status:       mcpauth.StatusExpired,
				AuthType:     "oauth",
				ClientID:     "client-id",
				TokenPresent: true,
				Refreshable:  true,
			},
			refresh: mcpauth.Status{
				ServerName:   "secure",
				Status:       mcpauth.StatusAuthenticated,
				AuthType:     "oauth",
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
				AuthType:     "oauth",
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
			compozyconfig.MCPServer{
				Name:      "github",
				Transport: compozyconfig.MCPServerTransportHTTP,
				URL:       "https://mcp.example.test/mcp",
			},
			WithTokenStore(store),
		)
		resolved := ResolvedServer{
			Server: compozyconfig.MCPServer{Name: "github"},
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
		if !mcpServerMatches(compozyconfig.MCPServer{Name: "GitHub"}, "github") {
			t.Fatal("mcpServerMatches() = false, want true for canonical owner")
		}
		if mcpServerMatches(compozyconfig.MCPServer{Name: "9bad"}, "bad") {
			t.Fatal("mcpServerMatches(invalid canonical name) = true, want false")
		}
	})

	t.Run("Should inject bearer only after full pre-registered credential binding", func(t *testing.T) {
		t.Parallel()

		server := authEnabledServer("secure", compozyconfig.MCPServerTransportHTTP, "https://mcp.example.test/mcp")
		target := globalMCPExecutorTarget("secure")
		for _, fixture := range []struct {
			name   string
			mutate func(*mcpauth.TokenRecord)
			want   string
			reason toolspkg.ReasonCode
		}{
			{
				name: "mismatched issuer",
				mutate: func(token *mcpauth.TokenRecord) {
					token.Issuer = "https://other-issuer.example.test"
				},
			},
			{
				name: "mismatched client",
				mutate: func(token *mcpauth.TokenRecord) {
					token.ClientID = "other-client"
				},
			},
			{
				name: "missing baseline scope",
				mutate: func(token *mcpauth.TokenRecord) {
					token.Scopes = []string{"other.scope"}
				},
				reason: toolspkg.ReasonMCPAuthInvalid,
			},
			{
				name: "step-up scope superset",
				mutate: func(token *mcpauth.TokenRecord) {
					token.Scopes = []string{"tools.read", "issues.write"}
				},
				want: "Bearer scoped-token",
			},
		} {
			t.Run(fixture.name, func(t *testing.T) {
				t.Parallel()

				store := newMemoryTokenStore()
				token := mcpauth.TokenRecord{
					Target:                target,
					DefinitionFingerprint: definitionFingerprint(t, target, server),
					Issuer:                "https://issuer.example.test",
					ClientID:              "client-id",
					Scopes:                []string{"tools.read"},
					AccessToken:           "scoped-token",
					TokenType:             "Bearer",
					ExpiresAt:             time.Now().Add(time.Hour),
				}
				fixture.mutate(&token)
				if err := store.SaveMCPAuthToken(t.Context(), token); err != nil {
					t.Fatalf("SaveMCPAuthToken() error = %v", err)
				}
				executor := newTestMCPExecutor(t, server, WithTokenStore(store))
				resolved := ResolvedServer{Server: server, Target: target}
				if got := executor.authorizationHeader(t.Context(), resolved); got != fixture.want {
					t.Fatalf("authorizationHeader() = %q, want %q", got, fixture.want)
				}
				if fixture.reason != "" {
					requireReason(t, executor.ensureAuthorized(t.Context(), resolved), fixture.reason)
				}
			})
		}
	})
}

func newFakeSDKServer(handler mcpsdk.ToolHandler) *mcpsdk.Server {
	return newFakeSDKServerWithSchemas(
		handler,
		json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"}}}`),
		json.RawMessage(`{"type":"object","properties":{"message":{"type":"string"}}}`),
		"Echo message",
		"",
		nil,
	)
}

func newFakeSDKServerWithSchemas(
	handler mcpsdk.ToolHandler,
	inputSchema any,
	outputSchema any,
	description string,
	title string,
	meta mcpsdk.Meta,
) *mcpsdk.Server {
	if handler == nil {
		handler = func(_ context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
			var input struct {
				Message string `json:"message"`
			}
			if err := json.Unmarshal(req.Params.Arguments, &input); err != nil {
				return nil, fmt.Errorf("decode echo input: %w", err)
			}
			return &mcpsdk.CallToolResult{
				Content:           []mcpsdk.Content{&mcpsdk.TextContent{Text: "echo: " + input.Message}},
				StructuredContent: map[string]string{"message": input.Message},
			}, nil
		}
	}
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fake", Version: "1.0.0"}, &mcpsdk.ServerOptions{
		Capabilities: &mcpsdk.ServerCapabilities{Tools: &mcpsdk.ToolCapabilities{}},
	})
	server.AddTool(&mcpsdk.Tool{
		Name:         "echo",
		Description:  description,
		InputSchema:  inputSchema,
		OutputSchema: outputSchema,
		Annotations:  &mcpsdk.ToolAnnotations{Title: title, ReadOnlyHint: true},
		Meta:         meta,
	}, handler)
	return server
}

func safeMCPDescriptorForRedactionTest() toolspkg.MCPToolDescriptor {
	return toolspkg.MCPToolDescriptor{
		ID:           "mcp__github__lookup",
		RawName:      "lookup",
		FriendlyVerb: "Looking up",
		Preview:      "auto",
		InputSchema:  json.RawMessage(`{"type":"object"}`),
		Source: toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         "github",
			RawServerName: "github",
			RawToolName:   "lookup",
		},
	}
}

func withTestMCPListTTL(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
	return func(ctx context.Context, method string, request mcpsdk.Request) (mcpsdk.Result, error) {
		result, err := next(ctx, method, request)
		if err != nil || method != mcpToolsListMethod {
			return result, err
		}
		listResult, ok := result.(*mcpsdk.ListToolsResult)
		if !ok {
			return nil, errors.New("test: tools/list returned an unexpected result type")
		}
		listResult.TTLMs = 60_000
		return listResult, nil
	}
}

func newReflectingSDKServer(secret string, listCalls *atomic.Int32) *mcpsdk.Server {
	encodedSecret := base64.StdEncoding.EncodeToString([]byte(secret))
	hexSecret := hex.EncodeToString([]byte(secret))
	dataURISecret := "data:application/octet-stream;base64," + encodedSecret
	percentSecret := url.PathEscape(secret)
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "reflecting", Version: "1.0.0"}, &mcpsdk.ServerOptions{
		Capabilities: &mcpsdk.ServerCapabilities{Tools: &mcpsdk.ToolCapabilities{}},
	})
	server.AddTool(&mcpsdk.Tool{
		Name:        "reflect",
		Description: "description credential=" + percentSecret,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"visible": map[string]any{"type": "string"},
			},
			"x-encoded-reflections": map[string]any{
				"value":   encodedSecret,
				"key":     map[string]any{encodedSecret: "visible"},
				"percent": map[string]any{"credential=" + percentSecret: "visible"},
			},
		},
		OutputSchema: map[string]any{
			"type": "object",
			"x-encoded-reflections": map[string]any{
				"value": "output " + hexSecret,
				"key":   map[string]any{dataURISecret: "visible"},
			},
		},
		Annotations: &mcpsdk.ToolAnnotations{
			Title:        "title " + hexSecret,
			ReadOnlyHint: true,
		},
		Meta: mcpsdk.Meta{
			mcpFriendlyVerbMetadataKey: "Reading " + encodedSecret,
			mcpPreviewMetadataKey:      "auto",
		},
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "text " + encodedSecret},
				&mcpsdk.ImageContent{Data: []byte(secret), MIMEType: "image/" + hexSecret},
				&mcpsdk.AudioContent{Data: []byte(secret), MIMEType: "audio/" + encodedSecret},
				&mcpsdk.EmbeddedResource{
					Resource: &mcpsdk.ResourceContents{
						URI:      "memory://" + encodedSecret,
						MIMEType: "application/" + hexSecret,
						Blob:     []byte("prefix:" + secret + ":suffix"),
					},
					Meta: mcpsdk.Meta{
						"binary":      []byte(secret),
						"encoded_key": map[string]any{encodedSecret: "visible"},
					},
				},
				&mcpsdk.ResourceLink{
					URI:         "memory://" + encodedSecret,
					Name:        "name-" + hexSecret,
					Title:       "title-" + encodedSecret,
					Description: "description-" + hexSecret,
					MIMEType:    "application/" + encodedSecret,
					Icons: []mcpsdk.Icon{{
						Source:   dataURISecret,
						MIMEType: "application/" + hexSecret,
					}},
				},
			},
			StructuredContent: map[string]any{
				encodedSecret:     encodedSecret,
				"binary":          []byte(secret),
				"data_uri":        dataURISecret,
				"percent_encoded": "credential=" + percentSecret,
				"encoded_keys": []any{
					map[string]any{encodedSecret: "base64"},
					map[string]any{hexSecret: "hex"},
					map[string]any{
						dataURISecret: "data_uri",
					},
				},
				"hex":    hexSecret,
				"nested": map[string]any{"value": "structured " + encodedSecret},
			},
		}, nil
	})
	server.AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
		return func(ctx context.Context, method string, request mcpsdk.Request) (mcpsdk.Result, error) {
			result, err := next(ctx, method, request)
			if err != nil || method != mcpToolsListMethod {
				return result, err
			}
			listCalls.Add(1)
			listResult, ok := result.(*mcpsdk.ListToolsResult)
			if !ok {
				return nil, errors.New("test: tools/list returned an unexpected result type")
			}
			listResult.TTLMs = 60_000
			return listResult, nil
		}
	})
	return server
}

func requireSecretAbsentFromJSON(t *testing.T, label string, value any, secret string) {
	t.Helper()

	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(%s) error = %v", label, err)
	}
	if strings.Contains(string(encoded), secret) {
		t.Fatalf("%s = %s, leaked credential %q", label, encoded, secret)
	}
	var decoded any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v", label, err)
	}
	requireSecretAbsentFromReversibleJSONValue(t, label, decoded, secret)
	if !strings.Contains(string(encoded), "[REDACTED]") {
		t.Fatalf("%s = %s, want redaction marker", label, encoded)
	}
}

func requireSecretAbsentFromReversibleJSONValue(t *testing.T, label string, value any, secret string) {
	t.Helper()

	switch typed := value.(type) {
	case string:
		requireSecretAbsentFromReversibleString(t, label, typed, secret)
	case []any:
		for index := range typed {
			requireSecretAbsentFromReversibleJSONValue(t, label, typed[index], secret)
		}
	case map[string]any:
		for key, item := range typed {
			requireSecretAbsentFromReversibleJSONValue(t, label+" JSON key", key, secret)
			requireSecretAbsentFromReversibleJSONValue(t, label, item, secret)
		}
	}
}

func requireSecretAbsentFromReversibleString(t *testing.T, label string, value string, secret string) {
	t.Helper()

	if strings.Contains(value, secret) {
		t.Fatalf("%s retained literal credential %q in %q", label, secret, value)
	}
	secretBytes := []byte(secret)
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		encodedSecret := encoding.EncodeToString(secretBytes)
		if encodedSecret != "" && strings.Contains(value, encodedSecret) {
			t.Fatalf("%s retained base64 credential %q in %q", label, encodedSecret, value)
		}
		decoded, err := encoding.DecodeString(value)
		if err == nil && bytes.Contains(decoded, secretBytes) {
			t.Fatalf("%s retained credential %q after base64 decode of %q", label, secret, value)
		}
	}
	for _, encodedSecret := range []string{
		hex.EncodeToString(secretBytes),
		strings.ToUpper(hex.EncodeToString(secretBytes)),
	} {
		if encodedSecret != "" && strings.Contains(value, encodedSecret) {
			t.Fatalf("%s retained hexadecimal credential %q in %q", label, encodedSecret, value)
		}
	}
	decoded, err := hex.DecodeString(value)
	if err == nil && bytes.Contains(decoded, secretBytes) {
		t.Fatalf("%s retained credential %q after hex decode of %q", label, secret, value)
	}
	for _, candidate := range []string{value, escapeInvalidPercentsForTest(value)} {
		decodedValue, decodeErr := url.PathUnescape(candidate)
		if decodeErr == nil && decodedValue != value && strings.Contains(decodedValue, secret) {
			t.Fatalf("%s retained credential %q after URL decode of %q", label, secret, value)
		}
		decodedValue, decodeErr = url.QueryUnescape(candidate)
		if decodeErr == nil && decodedValue != value && strings.Contains(decodedValue, secret) {
			t.Fatalf("%s retained credential %q after query decode of %q", label, secret, value)
		}
	}
	if len(value) < len("data:") || !strings.EqualFold(value[:len("data:")], "data:") {
		return
	}
	header, payload, found := strings.Cut(value, ",")
	if !found {
		return
	}
	if strings.Contains(strings.ToLower(header), ";base64") {
		requireSecretAbsentFromReversibleString(t, label+" data URI", payload, secret)
		return
	}
	decodedPayload, err := url.PathUnescape(payload)
	if err == nil && strings.Contains(decodedPayload, secret) {
		t.Fatalf("%s retained credential %q after data URI decode of %q", label, secret, value)
	}
}

func escapeInvalidPercentsForTest(value string) string {
	var escaped strings.Builder
	escaped.Grow(len(value))
	for index := 0; index < len(value); index++ {
		validEscape := value[index] == '%' &&
			index+2 < len(value) &&
			isHexForTest(value[index+1]) &&
			isHexForTest(value[index+2])
		if !validEscape {
			if value[index] == '%' {
				escaped.WriteString("%25")
			} else {
				escaped.WriteByte(value[index])
			}
			continue
		}
		escaped.WriteString(value[index : index+3])
		index += 2
	}
	return escaped.String()
}

func isHexForTest(value byte) bool {
	return value >= '0' && value <= '9' || value >= 'a' && value <= 'f' || value >= 'A' && value <= 'F'
}

func requireMCPBinaryContentRedacted(t *testing.T, result toolspkg.ToolResult, secret string) {
	t.Helper()

	seen := map[string]bool{hostedProxyImageKey: false, hostedProxyAudioKey: false}
	for index := range result.Content {
		content := result.Content[index]
		if _, tracked := seen[content.Type]; !tracked {
			continue
		}
		var encoded string
		if err := json.Unmarshal(content.Data, &encoded); err != nil {
			t.Fatalf("content[%d].Data = %s: %v", index, content.Data, err)
		}
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("content[%d].Data base64 decode error = %v", index, err)
		}
		if bytes.Contains(decoded, []byte(secret)) {
			t.Fatalf("content[%d].Data decoded = %q, leaked credential", index, decoded)
		}
		if !bytes.Contains(decoded, []byte("[REDACTED]")) {
			t.Fatalf("content[%d].Data decoded = %q, want redaction marker", index, decoded)
		}
		seen[content.Type] = true
	}
	for contentType, found := range seen {
		if !found {
			t.Fatalf("tool result content type %q not found", contentType)
		}
	}
}

func requireMCPBinaryEnvelopesRedacted(t *testing.T, result toolspkg.ToolResult) {
	t.Helper()

	var structured map[string]any
	if err := json.Unmarshal(result.Structured, &structured); err != nil {
		t.Fatalf("result.Structured = %s: %v", result.Structured, err)
	}
	for _, field := range []string{"binary", "data_uri", "hex"} {
		if got, want := structured[field], any("[REDACTED]"); got != want {
			t.Fatalf("result.Structured[%q] = %#v, want %#v", field, got, want)
		}
	}

	seenResource := false
	seenResourceLink := false
	for index := range result.Content {
		content := result.Content[index]
		if content.Type != executorMCPKey {
			continue
		}
		var envelope map[string]any
		if err := json.Unmarshal(content.Data, &envelope); err != nil {
			t.Fatalf("content[%d].Data = %s: %v", index, content.Data, err)
		}
		switch envelope["type"] {
		case "resource":
			seenResource = true
			resource, ok := envelope["resource"].(map[string]any)
			if !ok {
				t.Fatalf("content[%d].Data resource = %#v, want object", index, envelope["resource"])
			}
			blob, ok := resource["blob"].(string)
			if !ok {
				t.Fatalf("content[%d].Data resource.blob = %#v, want string", index, resource["blob"])
			}
			decoded, err := base64.StdEncoding.DecodeString(blob)
			if err != nil {
				t.Fatalf("content[%d].Data resource.blob decode error = %v", index, err)
			}
			if !bytes.Contains(decoded, []byte("[REDACTED]")) {
				t.Fatalf("content[%d].Data resource.blob decoded = %q, want marker", index, decoded)
			}
			metadata, ok := envelope["_meta"].(map[string]any)
			if !ok || metadata["binary"] != "[REDACTED]" {
				t.Fatalf("content[%d].Data _meta = %#v, want redacted binary", index, envelope["_meta"])
			}
		case "resource_link":
			seenResourceLink = true
			icons, ok := envelope["icons"].([]any)
			if !ok || len(icons) != 1 {
				t.Fatalf("content[%d].Data icons = %#v, want one icon", index, envelope["icons"])
			}
			icon, ok := icons[0].(map[string]any)
			if !ok || icon["src"] != "[REDACTED]" {
				t.Fatalf("content[%d].Data icon = %#v, want redacted data URI", index, icons[0])
			}
		}
	}
	if !seenResource || !seenResourceLink {
		t.Fatalf("binary envelopes seen resource=%t resource_link=%t, want both", seenResource, seenResourceLink)
	}
}

func requireMCPEmbeddedBlobRedacted(t *testing.T, result toolspkg.ToolResult) {
	t.Helper()

	for index := range result.Content {
		content := result.Content[index]
		if content.Type != executorMCPKey {
			continue
		}
		var envelope map[string]any
		if err := json.Unmarshal(content.Data, &envelope); err != nil {
			t.Fatalf("content[%d].Data = %s: %v", index, content.Data, err)
		}
		if envelope["type"] != "resource" {
			continue
		}
		resource, ok := envelope["resource"].(map[string]any)
		if !ok {
			t.Fatalf("content[%d].Data resource = %#v, want object", index, envelope["resource"])
		}
		blob, ok := resource["blob"].(string)
		if !ok {
			t.Fatalf("content[%d].Data resource.blob = %#v, want string", index, resource["blob"])
		}
		decoded, err := base64.StdEncoding.DecodeString(blob)
		if err != nil {
			t.Fatalf("content[%d].Data resource.blob decode error = %v", index, err)
		}
		if !bytes.Contains(decoded, []byte("[REDACTED]")) {
			t.Fatalf("content[%d].Data resource.blob decoded = %q, want marker", index, decoded)
		}
		return
	}
	t.Fatal("tool result embedded resource not found")
}

func newTestMCPHandler(server *mcpsdk.Server) http.Handler {
	return mcpsdk.NewStreamableHTTPHandler(
		func(*http.Request) *mcpsdk.Server { return server },
		&mcpsdk.StreamableHTTPOptions{
			Stateless:    true,
			JSONResponse: true,
		},
	)
}

func newTestMCPHTTPServer(server *mcpsdk.Server) *httptest.Server {
	return httptest.NewServer(newTestMCPHandler(server))
}

func newAuthorizedMCPExecutorServer(t *testing.T, token string) *httptest.Server {
	t.Helper()

	handler := newTestMCPHandler(newFakeSDKServer(nil))
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
	server compozyconfig.MCPServer,
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

func authEnabledServer(name string, transport compozyconfig.MCPServerTransport, url string) compozyconfig.MCPServer {
	return compozyconfig.MCPServer{
		Name:      name,
		Transport: transport,
		URL:       url,
		Auth: compozyconfig.MCPAuthConfig{
			Registration: compozyconfig.MCPAuthRegistrationPreRegistered,
			IssuerURL:    "https://issuer.example.test",
			ClientID:     "client-id",
			Scopes:       []string{"tools.read"},
		},
	}
}

func definitionFingerprint(
	t *testing.T,
	target mcpauth.Target,
	server compozyconfig.MCPServer,
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

func requireMCPErrorTreeRedacted(t *testing.T, err error, secret string) {
	t.Helper()

	if err == nil {
		t.Fatal("error tree = nil, want redacted error")
	}
	requireSecretAbsentFromReversibleString(t, "error tree message", err.Error(), secret)
	switch wrapped := err.(type) {
	case interface{ Unwrap() []error }:
		for _, child := range wrapped.Unwrap() {
			requireMCPErrorTreeRedacted(t, child, secret)
		}
	case interface{ Unwrap() error }:
		requireMCPErrorTreeRedacted(t, wrapped.Unwrap(), secret)
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

type fakeAuthService struct {
	mu                    sync.Mutex
	status                mcpauth.Status
	refresh               mcpauth.Status
	refreshErr            error
	calls                 int
	lastConfig            mcpauth.ServerConfig
	authorizationToken    mcpauth.TokenRecord
	hasAuthorizationToken bool
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

func (s *fakeAuthService) AuthorizationToken(
	_ context.Context,
	_ mcpauth.ServerConfig,
) (mcpauth.TokenRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.hasAuthorizationToken {
		return mcpauth.TokenRecord{}, mcpauth.ErrTokenNotFound
	}
	return s.authorizationToken, nil
}

func (s *fakeAuthService) setAuthorizationToken(token mcpauth.TokenRecord) {
	s.mu.Lock()
	s.authorizationToken = token
	s.hasAuthorizationToken = true
	s.mu.Unlock()
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
	mu            sync.Mutex
	tokens        map[string]mcpauth.TokenRecord
	registrations map[string]mcpauth.ClientRegistration
}

func newMemoryTokenStore() *memoryTokenStore {
	return &memoryTokenStore{
		tokens:        make(map[string]mcpauth.TokenRecord),
		registrations: make(map[string]mcpauth.ClientRegistration),
	}
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

func (s *memoryTokenStore) DeleteMCPAuthorizationState(ctx context.Context, _ mcpauth.Target) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

func (s *memoryTokenStore) GetMCPAuthRegistration(
	ctx context.Context,
	target mcpauth.Target,
) (mcpauth.ClientRegistration, error) {
	if err := ctx.Err(); err != nil {
		return mcpauth.ClientRegistration{}, err
	}
	key, err := target.Key()
	if err != nil {
		return mcpauth.ClientRegistration{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	registration, ok := s.registrations[key]
	if !ok {
		return mcpauth.ClientRegistration{}, mcpauth.ErrRegistrationNotFound
	}
	return cloneClientRegistration(registration), nil
}

func (s *memoryTokenStore) SaveMCPAuthRegistration(
	ctx context.Context,
	registration mcpauth.ClientRegistration,
	secrets mcpauth.RegistrationSecrets,
) (mcpauth.ClientRegistration, error) {
	if err := ctx.Err(); err != nil {
		return mcpauth.ClientRegistration{}, err
	}
	key, err := registration.Target.Key()
	if err != nil {
		return mcpauth.ClientRegistration{}, err
	}
	if secrets.ClientSecret != "" && registration.ClientSecretRef == "" {
		registration.ClientSecretRef = "vault:mcp/test-client-secret"
	}
	if secrets.RegistrationAccessToken != "" && registration.RegistrationAccessTokenRef == "" {
		registration.RegistrationAccessTokenRef = "vault:mcp/test-registration-access-token"
	}
	registration = cloneClientRegistration(registration)
	s.mu.Lock()
	s.registrations[key] = registration
	s.mu.Unlock()
	return cloneClientRegistration(registration), nil
}

func (s *memoryTokenStore) DeleteMCPAuthRegistration(ctx context.Context, target mcpauth.Target) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := target.Key()
	if err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.registrations, key)
	s.mu.Unlock()
	return nil
}

func cloneTokenRecord(token mcpauth.TokenRecord) mcpauth.TokenRecord {
	token.Scopes = append([]string(nil), token.Scopes...)
	return token
}

func cloneClientRegistration(registration mcpauth.ClientRegistration) mcpauth.ClientRegistration {
	registration.Scopes = append([]string(nil), registration.Scopes...)
	return registration
}

type secretRefResolverFunc func(context.Context, string) (string, error)

type mcpRoundTripperFunc func(*http.Request) (*http.Response, error)

func (f mcpRoundTripperFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type scriptedMCPResponseBody struct {
	readErr    error
	closeErr   error
	closeCalls int
}

func (b *scriptedMCPResponseBody) Read([]byte) (int, error) {
	return 0, b.readErr
}

func (b *scriptedMCPResponseBody) Close() error {
	b.closeCalls++
	return b.closeErr
}

func (f secretRefResolverFunc) ResolveRef(ctx context.Context, ref string) (string, error) {
	return f(ctx, ref)
}

func globalMCPExecutorTarget(serverName string) mcpauth.Target {
	return mcpauth.Target{Scope: mcpauth.ScopeGlobal, ServerName: serverName}
}
