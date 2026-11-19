package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
	"github.com/compozy/compozy/internal/mcp/securehttp"
	"github.com/compozy/compozy/internal/redact"
	"github.com/compozy/compozy/internal/testutil/mcpfixture"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
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
	// Invariant: a bound pre-registered token reaches a Streamable HTTP peer that negotiates 2025-06-18.
	// Owner: outbound MCP discovery; canonical suite: TestMCPCallExecutor.
	t.Run("Should discover tools from a pre-registered OAuth peer negotiating June 2025", func(t *testing.T) {
		t.Parallel()
		peer := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fixture", Version: "1.0.0"}, &mcpsdk.ServerOptions{
			SupportedProtocolVersions: []string{"2025-06-18"},
		})
		peer.AddTool(&mcpsdk.Tool{Name: "echo", InputSchema: json.RawMessage(`{"type":"object"}`)},
			func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
				return &mcpsdk.CallToolResult{
					Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "legacy response"}},
				}, nil
			})
		handler := mcpsdk.NewStreamableHTTPHandler(func(*http.Request) *mcpsdk.Server { return peer },
			&mcpsdk.StreamableHTTPOptions{JSONResponse: true})
		const access = "fixture-opaque-access"
		remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+access {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			handler.ServeHTTP(w, r)
		}))
		t.Cleanup(remote.Close)
		server := authEnabledServer("fixture", compozyconfig.MCPServerTransportHTTP, remote.URL)
		target := userMCPExecutorTarget("fixture")
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(t.Context(), mcpauth.TokenRecord{
			Target: target, DefinitionFingerprint: definitionFingerprint(t, target, server),
			Issuer: server.Auth.IssuerURL, ClientID: server.Auth.ClientID, Scopes: server.Auth.Scopes,
			AccessToken: access, TokenType: "Bearer",
		}); err != nil {
			t.Fatal(err)
		}
		executor := newTestMCPExecutor(t, server, WithTokenStore(store))
		discovery, err := executor.DiscoverTools(t.Context(), toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture",
		})
		if err != nil {
			if versionErr, ok := errors.AsType[*UnsupportedProtocolVersionError](err); ok {
				t.Fatalf("DiscoverTools() error = %v, cause = %v", err, versionErr)
			}
			t.Fatal(err)
		}
		if discovery.ProtocolVersion != "2025-06-18" || discovery.ToolCount != 1 ||
			len(discovery.Descriptors) != 1 || discovery.Descriptors[0].RawName != "echo" {
			t.Fatalf("unexpected discovery: %+v", discovery)
		}
		if result := callMCPTool(
			t,
			executor,
			discovery.Descriptors[0],
			json.RawMessage(`{}`),
		); result.Preview != "legacy response" {
			t.Fatalf("unexpected call preview %q", result.Preview)
		}
	})

	// Invariant: consent and transport use one immutable definition; owner: executor I/O boundary.
	for _, operation := range []string{"list", "call"} {
		t.Run(
			"Should launch the consented definition when configuration changes during "+operation,
			func(t *testing.T) {
				t.Parallel()
				approved := newTestMCPHTTPServer(newFakeSDKServer(nil))
				t.Cleanup(approved.Close)
				var unexpected atomic.Int32
				replacement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					unexpected.Add(1)
					w.WriteHeader(http.StatusForbidden)
				}))
				t.Cleanup(replacement.Close)
				current := compozyconfig.MCPServer{Name: "fixture", Transport: "http", URL: approved.URL}
				resolutions, checks := 0, 0
				executor, err := NewMCPCallExecutor(
					ServerResolverFunc(func(context.Context, toolspkg.SourceRef, string) (ResolvedServer, error) {
						resolutions++
						return ResolvedServer{Server: current, Target: userMCPExecutorTarget("fixture")}, nil
					}),
					WithDefinitionDigestKey([]byte("test installation key")),
					WithConsentCheck(func(_ context.Context, _ toolspkg.SourceRef, resolved ResolvedServer) error {
						checks++
						if resolved.Server.URL != approved.URL {
							return errors.New("definition was not approved")
						}
						current.URL = replacement.URL
						return nil
					}),
				)
				if err != nil {
					t.Fatal(err)
				}
				source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture"}
				if operation == "list" {
					_, err = executor.ListTools(t.Context(), source)
				} else {
					_, err = executor.CallTool(
						t.Context(),
						source,
						toolspkg.MCPToolCallRequest{
							ToolID:      "mcp__fixture__echo",
							RawToolName: "echo",
							Input:       json.RawMessage(`{"message":"approved"}`),
						},
					)
				}
				if err != nil || resolutions != 1 || checks != 1 || unexpected.Load() != 0 {
					t.Fatalf(
						"err=%v resolutions=%d checks=%d unexpected=%d",
						err,
						resolutions,
						checks,
						unexpected.Load(),
					)
				}
			},
		)
	}
	t.Run("Should negotiate modern protocol and paginate tools", func(t *testing.T) {
		t.Parallel()

		fixture := mcpfixture.MustNew(mcpfixture.ProfileModern2026)
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "fixture",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       fixture.StartHTTP(t).URL,
		})
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture"}
		discovery, err := executor.DiscoverTools(t.Context(), source)
		if err != nil {
			t.Fatalf("DiscoverTools() error = %v", err)
		}
		if got, want := discovery.ProtocolVersion, mcpfixture.ModernProtocolVersion; got != want {
			t.Fatalf("protocol version = %q, want %q", got, want)
		}
		if got, want := len(discovery.Descriptors), 2; got != want {
			t.Fatalf("descriptor count = %d, want %d after pagination", got, want)
		}
	})

	for _, version := range []string{protocolVersionModern, protocolVersionLegacy} {
		t.Run("Should discover encoded tools and preserve callable protocol names over "+version, func(t *testing.T) {
			t.Parallel()

			server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "names", Version: "1.0.0"}, &mcpsdk.ServerOptions{
				SupportedProtocolVersions: []string{version},
			})
			fixtureNames := []string{
				"_dd_gff",
				"dd_gff",
				"echo",
				" spaced_tool ",
				"Search.Read",
				"search_read",
				strings.Repeat("t", 64),
			}
			for _, name := range fixtureNames {
				server.AddTool(&mcpsdk.Tool{
					Name:        name,
					InputSchema: json.RawMessage(`{"type":"object"}`),
				}, func(_ context.Context, request *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
					return &mcpsdk.CallToolResult{
						Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: request.Params.Name}},
					}, nil
				})
			}
			server.AddReceivingMiddleware(withTestMCPListTTL)
			remote := newTestMCPHTTPServer(server)
			t.Cleanup(remote.Close)
			executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
				Name: "fixture", Transport: compozyconfig.MCPServerTransportHTTP, URL: remote.URL,
			})
			source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture"}
			roundTrip := map[toolspkg.ToolID]string{"mcp__fixture__raw__xl5sgix3hmzta": "_dd_gff"}
			want := map[string]toolspkg.ToolID{
				"_dd_gff":     "mcp__fixture__raw__xl5sgix3hmzta",
				"dd_gff":      "mcp__fixture__dd_gff",
				"echo":        "mcp__fixture__echo",
				"Search.Read": "mcp__fixture__search_read",
				"search_read": "mcp__fixture__search_read",
			}
			wantCount, skipped := 3, 4
			messages := []string{"exceeds 64 characters", "mcp__fixture__search_read"}
			if version == protocolVersionLegacy {
				roundTrip["mcp__fixture__spaced_tool"] = " spaced_tool "
				want[" spaced_tool "] = "mcp__fixture__spaced_tool"
				wantCount, skipped = 4, 3
			} else {
				messages = append(messages, "cannot be sent unchanged in the Mcp-Name header")
			}
			messages = append(messages, fmt.Sprintf("Skipped %d MCP tool entries", skipped))
			for range 2 {
				discovery, err := executor.DiscoverTools(t.Context(), source)
				if err != nil || discovery.ProtocolVersion != version {
					t.Fatalf("negotiated version = %q, %v, want %q", discovery.ProtocolVersion, err, version)
				}
				names := make(map[string]toolspkg.ToolID, len(discovery.Descriptors))
				for _, descriptor := range discovery.Descriptors {
					names[descriptor.Source.RawToolName] = descriptor.ID
					if wantName, ok := roundTrip[descriptor.ID]; ok {
						if descriptor.RawName != wantName || descriptor.Source.RawToolName != wantName {
							t.Fatalf("descriptor names = %q, %q, want %q",
								descriptor.RawName, descriptor.Source.RawToolName, wantName)
						}
						result := callMCPTool(t, executor, descriptor, json.RawMessage(`{}`))
						if got := result.Content[0].Text; got != wantName {
							t.Fatalf("tools/call name = %q, want %q", got, wantName)
						}
					}
				}
				if len(discovery.Descriptors) != len(want) || !maps.Equal(names, want) ||
					discovery.ToolCount != wantCount {
					t.Fatalf("discovered tools = %v, count = %d, want %v and %d available",
						names, discovery.ToolCount, want, wantCount)
				}
				for _, message := range messages {
					if !strings.Contains(discovery.Diagnostic, message) {
						t.Fatalf("diagnostic = %q, want %q", discovery.Diagnostic, message)
					}
				}
			}
			if version == protocolVersionModern {
				_, err := executor.CallTool(t.Context(), source, toolspkg.MCPToolCallRequest{
					ToolID: "mcp__fixture__spaced_tool", RawToolName: " spaced_tool ",
				})
				requireReason(t, err, toolspkg.ReasonIDInvalidFormat)
			}
		})
	}

	t.Run("Should negotiate legacy fallback and reject MRTR input requests", func(t *testing.T) {
		t.Parallel()

		legacy := mcpfixture.MustNew(mcpfixture.ProfileLegacy2025)
		legacyExecutor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "legacy",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       legacy.StartHTTP(t).URL,
		})
		legacySource := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "legacy", RawServerName: "legacy"}
		discovery, err := legacyExecutor.DiscoverTools(t.Context(), legacySource)
		if err != nil {
			t.Fatalf("DiscoverTools(legacy) error = %v", err)
		}
		if got, want := discovery.ProtocolVersion, mcpfixture.LegacyProtocolVersion; got != want {
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

	t.Run("Should attach fixed and connection-time secret headers without mutating declarations", func(t *testing.T) {
		t.Parallel()

		const secret = "resolved-header-secret-81c4"
		handler := newTestMCPHandler(newFakeSDKServer(nil))
		testServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if got := request.Header.Get("X-Tenant"); got != "acme" {
				http.Error(writer, "missing fixed header", http.StatusUnauthorized)
				return
			}
			if got := request.Header.Get("X-Deployment-Key"); got != secret {
				http.Error(writer, "missing secret header", http.StatusUnauthorized)
				return
			}
			handler.ServeHTTP(writer, request)
		}))
		t.Cleanup(testServer.Close)

		server := compozyconfig.MCPServer{
			Name: "deployment-api", Transport: compozyconfig.MCPServerTransportHTTP, URL: testServer.URL,
			Headers: map[string]string{"X-Tenant": "acme"},
			SecretHeaders: map[string]string{
				"X-Deployment-Key": "vault:extensions/global/kit/env/DEPLOYMENT_KEY",
			},
		}
		executor := newTestMCPExecutor(t, server, WithSecretResolver(secretRefResolverFunc(
			func(_ context.Context, ref string) (string, error) {
				if ref != server.SecretHeaders["X-Deployment-Key"] {
					return "", fmt.Errorf("unexpected secret ref %q", ref)
				}
				return secret, nil
			},
		)))
		if _, err := executor.ListTools(t.Context(), toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name,
		}); err != nil {
			t.Fatalf("ListTools() error = %v", err)
		}
		if got := server.SecretHeaders["X-Deployment-Key"]; got != "vault:extensions/global/kit/env/DEPLOYMENT_KEY" {
			t.Fatalf("declarative SecretHeaders changed to %q", got)
		}
		serialized, err := json.Marshal(server)
		if err != nil {
			t.Fatalf("json.Marshal(server) error = %v", err)
		}
		if bytes.Contains(serialized, []byte(secret)) {
			t.Fatalf("serialized server leaked resolved secret: %s", serialized)
		}
	})

	t.Run("Should block credential-bearing redirects to a different origin", func(t *testing.T) {
		t.Parallel()

		var targetCalls atomic.Int32
		target := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			targetCalls.Add(1)
			if request.Header.Get("X-Deployment-Key") != "" {
				t.Errorf("cross-origin target received X-Deployment-Key")
			}
			writer.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(target.Close)
		redirect := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
		}))
		t.Cleanup(redirect.Close)

		server := compozyconfig.MCPServer{
			Name: "deployment-api", Transport: compozyconfig.MCPServerTransportHTTP, URL: redirect.URL,
			SecretHeaders: map[string]string{
				"X-Deployment-Key": "vault:extensions/global/kit/env/DEPLOYMENT_KEY",
			},
		}
		executor := newTestMCPExecutor(t, server, WithSecretResolver(secretRefResolverFunc(
			func(context.Context, string) (string, error) { return "resolved-secret", nil },
		)))
		_, err := executor.ListTools(t.Context(), toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name,
		})
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)
		if targetCalls.Load() != 0 {
			t.Fatalf("cross-origin redirect target calls = %d, want 0", targetCalls.Load())
		}
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
		testCases := []struct {
			name        string
			agentPlugin bool
			headers     map[string]string
		}{
			{name: "Should retain requested scopes for a plain remote"},
			{
				name:        "Should retain requested scopes for an Agent Plugin without advertised metadata",
				agentPlugin: true,
				headers:     map[string]string{"X-Fixture": "plugin"},
			},
		}
		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				t.Parallel()
				executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
					Name:        "scope-challenge",
					Transport:   compozyconfig.MCPServerTransportHTTP,
					URL:         server.URL,
					AgentPlugin: testCase.agentPlugin,
					Headers:     testCase.headers,
				})
				_, err := executor.ListTools(t.Context(), toolspkg.SourceRef{
					Kind:          toolspkg.SourceMCP,
					Owner:         "scope-challenge",
					RawServerName: "scope-challenge",
				})
				scopeErr, ok := errors.AsType[*InsufficientScopeError](err)
				if !ok {
					t.Fatalf("ListTools() error = %v, want insufficient scope challenge", err)
				}
				requireReason(t, err, toolspkg.ReasonMCPAuthRequired)
				if got, want := scopeErr.Scopes, []string{"issues.read", "issues.write"}; !slices.Equal(got, want) {
					t.Fatalf("challenge scopes = %#v, want %#v", got, want)
				}
			})
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
		target := userMCPExecutorTarget("rejected-token")
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

	// Invariant: a resource 401 makes a still-unexpired credential refreshable before the next request.
	// Owner: outbound OAuth and MCP execution; canonical suite: TestMCPCallExecutor.
	t.Run("Should refresh a rejected bearer before the next discovery", func(t *testing.T) {
		t.Parallel()
		fixture := mcpfixture.StartOAuthHTTP(
			t,
			mcpfixture.MustNew(mcpfixture.ProfileModern2026),
			mcpfixture.OAuthConfig{},
		)
		server := authEnabledServer("fixture", compozyconfig.MCPServerTransportHTTP, fixture.Endpoints.MCPURL)
		server.Auth.IssuerURL = fixture.Endpoints.IssuerURL
		target := userMCPExecutorTarget(server.Name)
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(t.Context(), mcpauth.TokenRecord{
			Target: target, DefinitionFingerprint: definitionFingerprint(t, target, server),
			Issuer: server.Auth.IssuerURL, ClientID: server.Auth.ClientID, Scopes: server.Auth.Scopes,
			AccessToken: "rejected-fixture-access", RefreshToken: mcpfixture.RefreshToken, TokenType: "Bearer",
			ExpiresAt: time.Date(2126, 1, 1, 0, 0, 0, 0, time.UTC),
		}); err != nil {
			t.Fatal(err)
		}
		executor := newTestMCPExecutor(t, server, WithTokenStore(store))
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name}
		_, err := executor.CallTool(t.Context(), source, toolspkg.MCPToolCallRequest{
			ToolID: "mcp__fixture__echo", RawToolName: "echo", Input: json.RawMessage(`{"message":"rejected"}`),
		})
		requireReason(t, err, toolspkg.ReasonMCPAuthRequired)
		status, err := executor.Status(t.Context(), source, "")
		if err != nil || status.Status != string(mcpauth.StatusExpired) || !status.Refreshable || !status.TokenPresent {
			t.Fatalf("auth after rejection = %+v, %v", status, err)
		}
		if discovery, err := executor.DiscoverTools(t.Context(), source); err != nil || discovery.ToolCount == 0 {
			t.Fatalf("discovery after rejection = %+v, %v", discovery, err)
		}
		refreshes := fixture.Requests().Token
		if len(refreshes) != 1 || refreshes[0].Get("grant_type") != "refresh_token" {
			t.Fatal("discovery did not refresh the rejected credential exactly once")
		}
		status, err = executor.Status(t.Context(), source, "")
		if err != nil || status.Status != string(mcpauth.StatusAuthenticated) {
			t.Fatalf("auth after recovery = %+v, %v", status, err)
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
		target := userMCPExecutorTarget("secure")
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(t.Context(), mcpauth.TokenRecord{
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

	t.Run("Should sign in and use a bare remote without changing its definition", func(t *testing.T) {
		t.Parallel()
		fixture := mcpfixture.StartOAuthHTTP(
			t,
			mcpfixture.MustNew(mcpfixture.ProfileModern2026),
			mcpfixture.OAuthConfig{},
		)
		server := compozyconfig.MCPServer{Name: "fixture", Transport: "http", URL: fixture.Endpoints.MCPURL}
		target := userMCPExecutorTarget(server.Name)
		store := newMemoryTokenStore()
		service, err := mcpauth.NewService(store, mcpauth.WithRegistrationStore(store),
			mcpauth.WithSecureHTTPClient(securehttp.NewClient(securehttp.WithAllowLoopback(true))))
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := mcpauth.ServerConfigFromMCP(t.Context(), target, server, nil)
		if err != nil {
			t.Fatal(err)
		}
		before, err := service.Status(t.Context(), cfg)
		if err != nil || before.Status != mcpauth.StatusUnconfigured {
			t.Fatalf("before = %#v, %v", before, err)
		}
		login, err := service.BeginLogin(t.Context(), cfg, "http://127.0.0.1:2123/api/mcp/oauth/callback")
		if err != nil {
			t.Fatal(err)
		}
		status, err := service.Exchange(
			t.Context(),
			&login,
			login.RedirectURL+"?state="+url.QueryEscape(login.State)+"&code="+mcpfixture.AuthorizationCode,
		)
		if err != nil || status.Status != mcpauth.StatusAuthenticated {
			t.Fatalf("exchange = %#v, %v", status, err)
		}
		executor := newTestMCPExecutor(t, server, WithTokenStore(store),
			WithSecretResolver(secretRefResolverFunc(func(context.Context, string) (string, error) {
				return login.Config.ClientSecret, nil
			})))
		tools, err := executor.ListTools(
			t.Context(),
			toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name},
		)
		if err != nil || len(tools) == 0 {
			t.Fatalf("tools after sign in = %d, %v", len(tools), err)
		}
		token, err := store.GetMCPAuthToken(t.Context(), target)
		if err != nil {
			t.Fatal(err)
		}
		token.ExpiresAt = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
		if err := store.SaveMCPAuthToken(t.Context(), token); err != nil {
			t.Fatal(err)
		}
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name}
		statusView, err := executor.Status(t.Context(), source, "")
		if err != nil || statusView.Status != "expired" || !statusView.Refreshable {
			t.Fatalf("expired status = %+v, %v", statusView, err)
		}
		if reason, denied := toolspkg.MCPAuthStatusReason(statusView); denied {
			t.Fatalf("refreshable tool gated before call: %s", reason)
		}
		beforeRefresh := len(fixture.Requests().Token)
		result, err := executor.CallTool(t.Context(), source, toolspkg.MCPToolCallRequest{
			ToolID: "mcp__fixture__echo", RawToolName: "echo", Input: json.RawMessage(`{"message":"renewed-call"}`),
		})
		if err != nil || !strings.Contains(result.Preview, "renewed-call") {
			t.Fatalf("call across expiry = %+v, %v", result, err)
		}
		refreshes := fixture.Requests().Token
		if len(refreshes) != beforeRefresh+1 || refreshes[beforeRefresh].Get("grant_type") != "refresh_token" {
			t.Fatal("expired call did not refresh exactly once")
		}
		server.Auth = compozyconfig.MCPAuthConfig{Registration: "auto"}
		server.Headers = map[string]string{"X-API-Version": "1"}
		beforeHeaders := len(fixture.Requests().MCPHeaders)
		explicit := newTestMCPExecutor(t, server, WithTokenStore(store))
		if tools, err := explicit.ListTools(
			t.Context(),
			toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name},
		); err != nil ||
			len(tools) == 0 {
			t.Fatalf("explicit OAuth with auxiliary headers lost its credential: %v", err)
		}
		received := fixture.Requests().MCPHeaders
		if len(received) <= beforeHeaders {
			t.Fatal("explicit OAuth did not send an MCP request")
		}
		for _, headers := range received[beforeHeaders:] {
			if headers.Get("X-API-Version") != "1" {
				t.Fatal("explicit OAuth dropped its auxiliary header")
			}
		}
	})

	t.Run("Should probe a bare remote without credentials when the token store fails", func(t *testing.T) {
		t.Parallel()
		fixture := mcpfixture.MustNew(mcpfixture.ProfileModern2026)
		handler := fixture.StreamableHTTPHandler()
		var requests atomic.Int32
		remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			if r.Header.Get("Authorization") != "" {
				t.Error("unavailable token store sent an authorization header")
			}
			handler.ServeHTTP(w, r)
		}))
		defer remote.Close()
		server := compozyconfig.MCPServer{Name: "open", Transport: "http", URL: remote.URL}
		store := newMemoryTokenStore()
		store.getErr = errors.New("token store unavailable")
		executor := newTestMCPExecutor(t, server, WithTokenStore(store))
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name}
		if tools, err := executor.ListTools(t.Context(), source); err != nil || len(tools) == 0 {
			t.Fatalf("bare remote tools = %d, %v", len(tools), err)
		}
		before := requests.Load()
		if before == 0 {
			t.Fatal("bare remote did not reach the MCP endpoint")
		}
		server.Auth = compozyconfig.MCPAuthConfig{Registration: "auto"}
		explicit := newTestMCPExecutor(t, server, WithTokenStore(store))
		if _, err := explicit.ListTools(
			t.Context(),
			source,
		); err == nil ||
			!strings.Contains(err.Error(), "token store unavailable") {
			t.Fatalf("explicit OAuth lost its storage error: %v", err)
		}
		if requests.Load() != before {
			t.Fatal("explicit OAuth bypassed unavailable storage")
		}
	})

	// Invariant: an implicit remote can discover anonymously after its old definition is replaced.
	// Owner: outbound MCP authorization; canonical suite: TestMCPCallExecutor.
	t.Run("Should discover an implicit remote without sending a credential for its old definition", func(t *testing.T) {
		t.Parallel()
		handler := mcpfixture.MustNew(mcpfixture.ProfileModern2026).StreamableHTTPHandler()
		remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "" {
				t.Error("request sent a credential for the old MCP definition")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			handler.ServeHTTP(w, r)
		}))
		t.Cleanup(remote.Close)
		server := compozyconfig.MCPServer{
			Name:      "fixture",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       remote.URL,
		}
		previous := server
		previous.URL = "https://previous.example.test/mcp"
		target := userMCPExecutorTarget(server.Name)
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(t.Context(), mcpauth.TokenRecord{
			Target: target, DefinitionFingerprint: definitionFingerprint(t, target, previous),
			AccessToken: "previous-fixture-access", TokenType: "Bearer",
		}); err != nil {
			t.Fatal(err)
		}
		executor := newTestMCPExecutor(t, server, WithTokenStore(store))
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name}
		if found, err := executor.DiscoverTools(t.Context(), source); err != nil || found.ToolCount == 0 {
			t.Fatalf("anonymous discovery = %+v, %v", found, err)
		}
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
		target := userMCPExecutorTarget("fixture")
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
			ServerResolverFunc(func(context.Context, toolspkg.SourceRef, string) (ResolvedServer, error) {
				return ResolvedServer{Server: server, Target: target}, nil
			}),
			WithDefinitionDigestKey([]byte("test installation key")),
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

		target := userMCPExecutorTarget("secure")
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
		header, headerErr := executor.authorizationHeader(t.Context(), &resolved)
		if header != "" {
			t.Fatal("authorizationHeader(replacement) sent a credential")
		}
		requireReason(t, headerErr, toolspkg.ReasonMCPAuthInvalid)
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
			if err := store.SaveMCPAuthToken(t.Context(), fixture.record); err != nil {
				t.Fatalf("SaveMCPAuthToken(%#v) error = %v", fixture.record.Target, err)
			}
		}
		resolver := ServerResolverFunc(func(
			_ context.Context,
			source toolspkg.SourceRef, _ string,
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
		executor, err := NewMCPCallExecutor(resolver,
			WithDefinitionDigestKey([]byte("test installation key")), WithTokenStore(store))
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

		if err := store.DeleteMCPAuthToken(t.Context(), workspaceA); err != nil {
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
		target := userMCPExecutorTarget("secure")
		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(t.Context(), mcpauth.TokenRecord{
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

	// Invariant: discovery uses the bound token through real expiry and renews once within the bounded window.
	// Owner: MCP executor discovery and authorization; canonical suite: TestMCPCallExecutor.
	for _, tc := range []struct {
		name                 string
		lifetime             time.Duration
		window               time.Duration
		retryAfter           time.Duration
		windowCap            time.Duration
		noRefresh            bool
		rejected             bool
		disabled             bool
		disableDuringBackoff bool
	}{
		{name: "Should renew within the maximum window", lifetime: time.Hour, window: 30 * time.Second},
		{name: "Should scale the window to short lifetimes", lifetime: 20 * time.Second, window: 2 * time.Second},
		{name: "Should honor a shorter renewal cap", lifetime: time.Hour, window: 5 * time.Second, windowCap: 5 * time.Second},
		{
			name:     "Should use the still-valid token during transient renewal failure and backoff",
			lifetime: time.Minute, window: 6 * time.Second, retryAfter: 2 * time.Second,
		},
		{
			name:     "Should withhold a bearer that expires during renewal backoff",
			lifetime: time.Minute, window: 6 * time.Second, retryAfter: 10 * time.Second,
		},
		{
			name:     "Should stop before connecting when backoff returns a still-valid bearer",
			lifetime: time.Minute, window: 6 * time.Second, retryAfter: 2 * time.Second,
			disableDuringBackoff: true,
		},
		{name: "Should allow an access-only grant until expiry", lifetime: time.Minute, window: 6 * time.Second, noRefresh: true},
		{name: "Should stop before connecting when renewal observes the switch off", lifetime: time.Minute,
			window: 6 * time.Second, disabled: true},
		{name: "Should withdraw a definitively rejected grant before expiry", lifetime: time.Minute, window: 6 * time.Second, rejected: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var renewals atomic.Int32
			var mcpRequests atomic.Int32
			var failRefresh, off atomic.Bool
			off.Store(tc.disabled)
			failRefresh.Store(tc.retryAfter > 0)
			handler := newTestMCPHandler(newFakeSDKServer(nil))
			remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/.well-known/oauth-authorization-server":
					base := "http://" + r.Host
					w.Header().Set("Content-Type", "application/json")
					if err := json.NewEncoder(w).Encode(map[string]any{
						"issuer":                           base,
						"authorization_endpoint":           base + "/authorize",
						"token_endpoint":                   base + "/token",
						"code_challenge_methods_supported": []string{"S256"},
					}); err != nil {
						t.Error(err)
					}
				case "/token":
					renewals.Add(1)
					w.Header().Set("Content-Type", "application/json")
					if tc.rejected {
						w.WriteHeader(http.StatusBadRequest)
						if _, err := w.Write([]byte(`{"error":"invalid_grant"}`)); err != nil {
							t.Error(err)
						}
						return
					}
					if failRefresh.Load() {
						w.WriteHeader(http.StatusServiceUnavailable)
						if _, err := w.Write([]byte(`{"error":"temporarily_unavailable"}`)); err != nil {
							t.Error(err)
						}
						return
					}
					if _, err := w.Write([]byte(`{"access_token":"renewed-access","expires_in":3600}`)); err != nil {
						t.Error(err)
					}
				default:
					mcpRequests.Add(1)
					want := "Bearer current-access"
					if renewals.Load() > 0 && !failRefresh.Load() {
						want = "Bearer renewed-access"
					}
					if r.Header.Get("Authorization") != want {
						http.Error(w, "invalid bearer", http.StatusUnauthorized)
						return
					}
					handler.ServeHTTP(w, r)
				}
			}))
			t.Cleanup(remote.Close)
			issuedAt := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
			expiresAt := issuedAt.Add(tc.lifetime)
			now := expiresAt.Add(-tc.window - time.Second)
			server := authEnabledServer("secure", compozyconfig.MCPServerTransportHTTP, remote.URL)
			server.Auth.IssuerURL = remote.URL
			target := userMCPExecutorTarget(server.Name)
			store := newMemoryTokenStore()
			token := mcpauth.TokenRecord{
				Target: target, DefinitionFingerprint: definitionFingerprint(t, target, server),
				Issuer: remote.URL, ClientID: server.Auth.ClientID, Scopes: server.Auth.Scopes,
				AccessToken: "current-access", ExpiresAt: expiresAt, UpdatedAt: issuedAt,
				// ObtainedAt may date back to an earlier grant. UpdatedAt measures this token's lifetime.
				ObtainedAt: issuedAt.Add(-time.Hour),
			}
			if !tc.noRefresh {
				token.RefreshToken = "fixture-refresh"
			}
			if err := store.SaveMCPAuthToken(t.Context(), token); err != nil {
				t.Fatal(err)
			}
			service, err := mcpauth.NewService(store, mcpauth.WithNow(func() time.Time { return now }),
				mcpauth.WithRefreshBackoff(tc.retryAfter, tc.retryAfter), mcpauth.WithEarlyRenewalWindow(tc.windowCap),
				mcpauth.WithSecureHTTPClient(securehttp.NewClient(securehttp.WithAllowLoopback(true))),
				mcpauth.WithNetworkAuthorization(func(context.Context, mcpauth.ServerConfig) error {
					if off.Load() {
						return mcpauth.ErrServerDisabled
					}
					return nil
				}),
			)
			if err != nil {
				t.Fatal(err)
			}
			recoveries := 0
			executor := newTestMCPExecutor(t, server, withAuthService(service),
				WithAuthRecovery(func(context.Context, *ResolvedServer) error {
					recoveries++
					return nil
				}))
			source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name}
			requireMCPDescriptor(t, executor, server.Name, "mcp__secure__echo")
			if renewals.Load() != 0 {
				t.Fatal("renewal began before the window")
			}
			now = expiresAt.Add(-tc.window)
			status, err := executor.Status(t.Context(), source, "")
			if err != nil || status.Status != string(mcpauth.StatusAuthenticated) ||
				status.ExpiresAt == nil || !status.ExpiresAt.Equal(expiresAt) || renewals.Load() != 0 {
				t.Fatalf("passive status before expiry = %+v, err = %v", status, err)
			}
			if tc.disabled {
				before := mcpRequests.Load()
				_, err := executor.ListTools(t.Context(), source)
				requireReason(t, err, toolspkg.ReasonSourceDisabled)
				stored, err := store.GetMCPAuthToken(t.Context(), target)
				if mcpRequests.Load() != before || renewals.Load() != 0 || err != nil ||
					stored.AccessToken != token.AccessToken {
					t.Fatal("disabled renewal contacted the server or changed persisted authorization")
				}
				return
			}
			if tc.rejected {
				_, err := executor.ListTools(t.Context(), source)
				requireReason(t, err, toolspkg.ReasonMCPAuthRequired)
				if _, err := store.GetMCPAuthToken(t.Context(), target); !errors.Is(err, mcpauth.ErrTokenNotFound) {
					t.Fatalf("rejected credential remains stored: %v", err)
				}
				return
			}
			requireMCPDescriptor(t, executor, server.Name, "mcp__secure__echo")
			if tc.noRefresh {
				now = expiresAt
				_, err := executor.ListTools(t.Context(), source)
				requireReason(t, err, toolspkg.ReasonMCPAuthExpired)
				if renewals.Load() != 0 {
					t.Fatal("access-only grant attempted renewal")
				}
				return
			}
			if tc.disableDuringBackoff {
				off.Store(true)
				before := mcpRequests.Load()
				_, err := executor.ListTools(t.Context(), source)
				requireReason(t, err, toolspkg.ReasonSourceDisabled)
				if mcpRequests.Load() != before || renewals.Load() != 1 {
					t.Fatal("discovery started while off during renewal backoff")
				}
				return
			}
			if renewals.Load() != 1 {
				t.Fatalf("renewals at the boundary = %d, want 1", renewals.Load())
			}
			requireMCPDescriptor(t, executor, server.Name, "mcp__secure__echo")
			if renewals.Load() != 1 {
				t.Fatal("renewal repeated while backed off or freshly renewed")
			}
			if tc.retryAfter > 0 {
				if recoveries != 0 {
					t.Fatal("transient failure was reported as auth recovery")
				}
				if tc.retryAfter > tc.window {
					requestsBeforeExpiry := mcpRequests.Load()
					now = expiresAt
					_, err := executor.ListTools(t.Context(), source)
					requireReason(t, err, toolspkg.ReasonMCPAuthRefreshFailed)
					if renewals.Load() != 1 || mcpRequests.Load() != requestsBeforeExpiry {
						t.Fatal(
							"discovery during backoff contacted the issuer or sent a request with an expired bearer",
						)
					}
					return
				}
				failRefresh.Store(false)
				now = now.Add(tc.retryAfter)
				requireMCPDescriptor(t, executor, server.Name, "mcp__secure__echo")
				if renewals.Load() != 2 {
					t.Fatal("renewal did not recover after backoff")
				}
			}
			if recoveries != 1 {
				t.Fatalf("auth recovery calls = %d, want 1", recoveries)
			}
		})
	}

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
		ctx, cancel := context.WithCancel(t.Context())
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
					releaseCanceledRequest = sync.OnceFunc(func() {
						close(requestRelease)
					})
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
				target := userMCPExecutorTarget(configuredServer.Name)
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
		target := userMCPExecutorTarget(configuredServer.Name)
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

		fresh, err := executor.ListTools(toolspkg.WithFreshMCPDiscovery(t.Context()), source)
		if err != nil || len(fresh) != 1 || listCalls.Load() != 2 {
			t.Fatalf("fresh probe: %v, %v, calls=%d", fresh, err, listCalls.Load())
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

func recordTestToolProjection(
	executor *CallExecutor,
	key, listingKey string,
	descriptors []toolspkg.MCPToolDescriptor,
	ttl time.Duration,
	now time.Time,
) error {
	content, err := toolspkg.FingerprintMCPToolDescriptors(descriptors)
	if err != nil {
		return err
	}
	executor.recordToolProjectionState(key, listingKey, content, ttl, now)
	return nil
}

// Invariant: authoritative MCP projections change only with logical descriptor or credential state and retain no expired cache state.
// Owning layer: internal/mcp unit cache.
// Canonical suite: internal/mcp/executor_test.go.
func TestMCPToolListCache(t *testing.T) {
	// Invariant: a failed poll is shared by callers of the same binding, with a fresh operator check still allowed.
	// Owner: MCP executor fallback discovery; canonical suite: TestMCPToolListCache.
	t.Run("Should share failed fallback attempts without delaying an explicit check", func(t *testing.T) {
		t.Parallel()
		var lists atomic.Int32
		upstream := newFakeSDKServer(nil)
		upstream.AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
			return func(ctx context.Context, method string, request mcpsdk.Request) (mcpsdk.Result, error) {
				if method != mcpToolsListMethod {
					return next(ctx, method, request)
				}
				if lists.Add(1) == 1 {
					return nil, &jsonrpc.Error{Code: -32000, Message: "fixture unavailable"}
				}
				return &mcpsdk.ListToolsResult{Tools: syntheticMCPTools(1)}, nil
			}
		})
		server := newTestMCPHTTPServer(upstream)
		t.Cleanup(server.Close)
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{Name: "fixture", URL: server.URL})
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", ProfileID: "first"}
		ctx := testContext(t)
		_, firstErr := executor.listResolvedTools(ctx, source, true)
		requireReason(t, firstErr, toolspkg.ReasonMCPUnreachable)
		if firstErr == nil || !strings.Contains(firstErr.Error(), "fixture unavailable") {
			t.Fatalf("first poll error = %v, want fixture rejection", firstErr)
		}
		peer := source
		peer.ProfileID = "second"
		_, err := executor.listResolvedTools(ctx, peer, true)
		if _, deferred := errors.AsType[*toolspkg.MCPCatalogPollDeferred](err); !deferred || lists.Load() != 1 {
			t.Fatalf(
				"same-binding poll error = %v, upstream lists = %d, want deferral after one list",
				err,
				lists.Load(),
			)
		}
		discovery, err := executor.DiscoverTools(toolspkg.WithFreshMCPDiscovery(ctx), source)
		if err != nil || len(discovery.Descriptors) != 1 || lists.Load() != 2 {
			t.Fatalf(
				"explicit check = %d tools, %v, upstream lists = %d",
				len(discovery.Descriptors),
				err,
				lists.Load(),
			)
		}
		discovery, err = executor.listResolvedTools(ctx, peer, true)
		if err != nil || len(discovery.Descriptors) != 1 || lists.Load() != 2 {
			t.Fatalf(
				"shared recovery = %d tools, %v, upstream lists = %d",
				len(discovery.Descriptors),
				err,
				lists.Load(),
			)
		}
	})

	// Invariant: retries share a server binding and honor the previous TTL through a failure.
	// Owner: MCP executor fallback discovery; canonical suite: TestMCPToolListCache.
	t.Run("Should retain the fallback interval across child replacement and failed polls", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			server := compozyconfig.MCPServer{Name: "fixture", Command: "fixture"}
			executor := newTestMCPExecutor(t, server)
			resolved := &ResolvedServer{
				Server: server, Target: userMCPExecutorTarget(server.Name),
				listingSpec: &mcpStdioSpec{digest: "stdio-fixture"}, listingBinding: "stdio-fixture:1",
			}
			key, err := executor.toolPollKey(t.Context(), resolved, "")
			if err != nil {
				t.Fatal(err)
			}
			if got := executor.effectiveToolListTTL(1); got != time.Minute {
				t.Fatalf("short hint expiry = %v, want 1 minute", got)
			}
			if got := executor.effectiveToolListTTL(0); got != 2*time.Minute {
				t.Fatalf("expiry without a hint = %v, want 2 minutes", got)
			}
			interval := executor.effectiveToolListTTL(180_000)
			now := time.Now()
			executor.recordCatalogListing("old-child", key, interval, true, now)
			resolved.listingBinding = "stdio-fixture:2"
			replacementKey, err := executor.toolPollKey(t.Context(), resolved, "")
			if err != nil {
				t.Fatal(err)
			}
			previous := executor.catalogPoll(replacementKey)
			if got, want := previous.expiresAt, now.Add(3*time.Minute); !got.Equal(want) {
				t.Fatalf("replacement deadline = %v, want %v", got, want)
			}
			time.Sleep(3 * time.Minute)
			attempt := executor.beginToolList(catalogPollAttemptKey(key), false)
			defer executor.finishToolList(catalogPollAttemptKey(key))
			executor.recordCatalogPollFailure(replacementKey, previous, attempt, errors.New("upstream failed"))
			if got, want := executor.catalogPoll(key).expiresAt, now.Add(6*time.Minute); !got.Equal(want) {
				t.Fatalf("shared retry deadline = %v, want %v", got, want)
			}
		})
	})

	// Invariant: retired subscriptions cannot invalidate an active binding, even on the first failed poll.
	// Owner: executor HTTP catalog cache; canonical suite: TestMCPToolListCache.
	t.Run("Should fence subscription callbacks and failed polls across replacement", func(t *testing.T) {
		t.Parallel()
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{}, WithToolCatalogNotifications())
		catalog := executor.catalogHTTP
		key := "catalog-binding"
		active := &catalogNotificationStream{}
		body := &catalogNotificationBody{}
		body.acknowledged.Store(true)
		active.body.Store(body)
		catalog.sessions[key] = mcpCatalogHTTPEntry{stream: active}
		stale := &catalogNotificationStream{}
		stale.body.Store(body)

		attemptKey := catalogPollAttemptKey(key)
		attempt := executor.beginToolList(attemptKey, false)
		defer executor.finishToolList(attemptKey)
		catalog.streamEstablished(key, stale)
		catalog.notify(key, stale)
		select {
		case <-executor.CatalogChanges():
			t.Fatal("replaced stream woke the catalog")
		default:
		}

		catalog.streamEstablished(key, active)
		select {
		case <-executor.CatalogChanges():
		default:
			t.Fatal("establishment did not schedule a fresh listing")
		}
		catalog.notify(key, active)
		executor.recordCatalogPollFailure(key, mcpCatalogPoll{}, attempt, errors.New("old request failed"))
		if poll := executor.catalogPoll(key); !poll.expiresAt.IsZero() {
			t.Fatalf("superseded failure installed expiry %v", poll.expiresAt)
		}
	})

	// Invariant: an advertised subscription keeps finite expiry until the transport has a live channel.
	// Owner: executor HTTP catalog cache; canonical suite: TestMCPToolListCache.
	t.Run("Should cache by push only after the notification stream opens", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name, method string
			status       int
			acknowledge  bool
		}{
			{name: "Should fall back when a legacy GET is refused", method: http.MethodGet,
				status: http.StatusMethodNotAllowed},
			{name: "Should use an established legacy stream", method: http.MethodGet,
				status: http.StatusOK, acknowledge: true},
			{name: "Should require acknowledgement on a modern stream", method: http.MethodPost,
				status: http.StatusOK, acknowledge: true},
			{name: "Should poll when the acknowledged filter omits tool changes", method: http.MethodPost,
				status: http.StatusOK},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				drops := 0
				stream := &catalogNotificationStream{
					handshakeTimeout: time.Second,
					next: mcpRoundTripperFunc(func(*http.Request) (*http.Response, error) {
						return &http.Response{StatusCode: tc.status,
							Header: http.Header{"Content-Type": []string{"text/event-stream"}},
							Body:   io.NopCloser(strings.NewReader(""))}, nil
					}),
					ended: func() { drops++ },
				}
				req, err := http.NewRequestWithContext(t.Context(), tc.method, "https://example.com/mcp",
					strings.NewReader(`{"method":"subscriptions/listen"}`))
				if err != nil {
					t.Fatal(err)
				}
				resp, err := stream.RoundTrip(req)
				if err != nil {
					t.Fatal(err)
				}
				if tc.method == http.MethodPost {
					if stream.live() {
						t.Fatal("unacknowledged stream was treated as live")
					}
					acknowledge := stream.observeAcknowledgement(
						func(context.Context, string, mcpsdk.Request) (mcpsdk.Result, error) { return nil, nil },
					)
					req := &mcpsdk.ServerRequest[*mcpsdk.SubscriptionsAcknowledgedParams]{
						Params: &mcpsdk.SubscriptionsAcknowledgedParams{
							Notifications: mcpsdk.NotificationSubscriptions{ToolsListChanged: tc.acknowledge},
						},
					}
					if _, err := acknowledge(t.Context(), "notifications/subscriptions/acknowledged", req); err != nil {
						t.Fatal(err)
					}
				}
				if stream.live() != tc.acknowledge {
					t.Fatalf("stream live = %v, want %v", stream.live(), tc.acknowledge)
				}
				if err := resp.Body.Close(); err != nil {
					t.Fatal(err)
				}
				if stream.live() || (drops == 1) != (tc.status == http.StatusOK) {
					t.Fatalf("stream live = %v after close, drops = %d", stream.live(), drops)
				}
			})
		}
	})

	// Invariant: one upstream notification invalidates its source across a large catalog.
	// Owner: executor discovery cache; canonical suite: TestMCPToolListCache.
	t.Run("Should relist only the changed source for incident-scale cached readers", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		const servers, toolsPerServer, readers = 5, 220, 18
		var calls [servers]atomic.Int32
		var sdk [servers]*mcpsdk.Server
		configured := make(map[string]compozyconfig.MCPServer, servers)
		sources := make([]toolspkg.SourceRef, 0, servers)
		for i := range servers {
			name := fmt.Sprintf("catalog%d", i)
			sdk[i] = mcpsdk.NewServer(&mcpsdk.Implementation{Name: name}, &mcpsdk.ServerOptions{
				Capabilities: &mcpsdk.ServerCapabilities{Tools: &mcpsdk.ToolCapabilities{ListChanged: i == 0}},
			})
			for _, tool := range syntheticMCPTools(toolsPerServer) {
				sdk[i].AddTool(tool, func(context.Context, *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
					return &mcpsdk.CallToolResult{}, nil
				})
			}
			sdk[i].AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
				return func(ctx context.Context, method string, request mcpsdk.Request) (mcpsdk.Result, error) {
					if method == mcpToolsListMethod {
						calls[i].Add(1)
					}
					return next(ctx, method, request)
				}
			})
			server := newTestMCPHTTPServer(sdk[i])
			t.Cleanup(server.Close)
			configured[name] = compozyconfig.MCPServer{
				Name: name, Transport: compozyconfig.MCPServerTransportHTTP, URL: server.URL,
			}
			sources = append(sources, toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: name, RawServerName: name})
		}
		executor, err := NewMCPCallExecutor(
			ServerResolverFunc(func(_ context.Context, source toolspkg.SourceRef, _ string) (ResolvedServer, error) {
				return ResolvedServer{
					Server: configured[source.Owner], Target: userMCPExecutorTarget(source.Owner),
				}, nil
			}),
			WithDefinitionDigestKey([]byte("catalog notification test key")),
			WithToolCatalogNotifications(),
		)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := executor.CloseCatalogConnections(context.WithoutCancel(t.Context())); err != nil {
				t.Error(err)
			}
		})
		establishTestCatalogSubscription(t, ctx, executor, sources[0])
		for range readers {
			for _, source := range sources {
				got, err := executor.ListTools(ctx, source)
				if err != nil || len(got) != toolsPerServer {
					t.Fatalf("ListTools(%q) = %d tools, %v", source.Owner, len(got), err)
				}
			}
		}
		sdk[0].RemoveTools("catalog_tool_0000")
		select {
		case <-executor.CatalogChanges():
		case <-ctx.Done():
			t.Fatal("executor did not receive tools/list_changed")
		}
		for range readers {
			for i, source := range sources {
				want := toolsPerServer
				if i == 0 {
					want--
				}
				got, err := executor.ListTools(ctx, source)
				if err != nil || len(got) != want {
					t.Fatalf("ListTools(%q) after change = %d tools, %v, want %d", source.Owner, len(got), err, want)
				}
			}
		}
		for i := range servers {
			want := int32(1)
			if i == 0 {
				want++
			}
			if got := calls[i].Load(); got != want {
				t.Errorf("server %d tools/list calls = %d, want %d", i, got, want)
			}
		}
	})

	// Invariant: repeat registrations reuse encoded schemas and a new secret performs one pass.
	// Owner: executor descriptor normalization; canonical suite: TestMCPToolListCache.
	t.Run("Should process registered secret material once per server catalog", func(t *testing.T) {
		const size, readers = 1100, 18
		const stable = "synthetic-request-secret-a5237a9d"
		const late = "synthetic-catalog-secret-45fe9a81"
		server := compozyconfig.MCPServer{Name: "fixture", URL: "https://example.com/mcp"}
		executor := newTestMCPExecutor(t, server)
		resolved := &ResolvedServer{Server: server, Target: userMCPExecutorTarget(server.Name)}
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name}
		var encodes int
		raw := syntheticMCPTools(size)
		for _, tool := range raw {
			tool.InputSchema = countMCPJSONEncodes{count: &encodes}
		}
		raw[0].Description = late
		read := func() ToolDiscovery {
			t.Helper()
			discovery, err := executor.normalizeToolListing("fixture", source, resolved, protocolVersionModern, raw)
			if err != nil || len(discovery.Descriptors) != size {
				t.Fatalf("normalized %d tools, error %v", len(discovery.Descriptors), err)
			}
			return discovery
		}
		for range readers {
			cleanup := redact.RegisterRequiredSecret(stable)
			read()
			cleanup()
		}
		initial := encodes
		if initial != size*2 {
			t.Fatalf("schema encodes = %d, want one fingerprint and one normalization pass", initial)
		}
		stableCleanup := redact.RegisterRequiredSecret(stable)
		defer stableCleanup()
		raw = slices.Clone(raw)
		slices.Reverse(raw)
		read()
		if got := encodes - initial; got != size {
			t.Fatalf("schema encodes after an unchanged, reordered listing = %d, want one fingerprint pass", got)
		}
		encodes = 0
		lateCleanup := redact.RegisterRequiredSecret(late)
		defer lateCleanup()
		for range readers {
			descriptors := read().Descriptors
			marked := slices.IndexFunc(descriptors, func(descriptor toolspkg.MCPToolDescriptor) bool {
				return descriptor.RawName == "catalog_tool_0000"
			})
			if marked < 0 || strings.Contains(descriptors[marked].Description, late) {
				t.Fatal("marked descriptor is absent or still contains the late credential")
			}
		}
		if got := encodes; got != initial {
			t.Fatalf("schema encodes after a new credential = %d, want one new pass", got)
		}
	})

	// Invariant: only a newly registered secret that appears in the listing requires another normalization pass.
	// Owner: executor descriptor normalization; canonical suite: TestMCPToolListCache.
	for _, tc := range []struct {
		name      string
		related   bool
		wantCount int
	}{
		{name: "Should return safe descriptors when unrelated secrets churn", wantCount: 2},
		{name: "Should redact a credential registered during listing", related: true, wantCount: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const secret = "listing-registered-secret-fcb0278e"
			server := compozyconfig.MCPServer{Name: "fixture", URL: "https://example.com/mcp"}
			executor := newTestMCPExecutor(t, server)
			resolved := &ResolvedServer{Server: server, Target: userMCPExecutorTarget(server.Name)}
			source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name}
			raw := syntheticMCPTools(1)
			var encodes int
			cleanup := func() {}
			raw[0].InputSchema = countMCPJSONEncodes{count: &encodes, beforeEncode: func() {
				if tc.related {
					if encodes == 1 {
						cleanup = redact.RegisterRequiredSecret(secret)
						t.Cleanup(cleanup)
					}
					return
				}
				cleanup()
				cleanup = redact.RegisterRequiredSecret(fmt.Sprintf("%s-%d", secret, encodes))
				t.Cleanup(cleanup)
			}}
			if tc.related {
				raw[0].Description = secret
			}
			listing, err := executor.normalizeToolListing("fixture", source, resolved, protocolVersionModern, raw)
			if err != nil || len(listing.Descriptors) != 1 {
				t.Fatalf("normalized %d descriptors, err = %v", len(listing.Descriptors), err)
			}
			if got := listing.Descriptors[0].Description; strings.Contains(got, secret) {
				t.Fatal("newly registered credential appears in a descriptor")
			}
			if encodes != tc.wantCount {
				t.Fatalf("schema encodes = %d, want %d", encodes, tc.wantCount)
			}
		})
	}

	// Invariant: a waiter's deadline is an MCP timeout and carries no upstream health failure.
	// Owner: MCP executor listing coordination; canonical suite: TestMCPToolListCache.
	t.Run("Should classify a listing wait deadline as an MCP timeout", func(t *testing.T) {
		t.Parallel()
		synctest.Test(t, func(t *testing.T) {
			server := compozyconfig.MCPServer{
				Name: "fixture", Transport: compozyconfig.MCPServerTransportHTTP, URL: "https://example.com/mcp",
			}
			executor := newTestMCPExecutor(t, server)
			resolved := &ResolvedServer{Server: server, Target: userMCPExecutorTarget(server.Name)}
			source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name}
			key, err := executor.toolCacheKey(t.Context(), resolved, protocolVersionModern, "")
			if err != nil {
				t.Fatal(err)
			}
			release, err := executor.acquireToolListing(t.Context(), key)
			if err != nil {
				t.Fatal(err)
			}
			defer release()

			ctx, cancel := context.WithTimeout(t.Context(), time.Second)
			defer cancel()
			_, healthErr, err := executor.listToolsFromClient(
				ctx,
				nil,
				source,
				resolved,
				protocolVersionModern,
				"",
				key,
				false,
			)
			if !errors.Is(err, toolspkg.ErrToolTimedOut) || healthErr != nil {
				t.Fatalf("listing wait error = %v, upstream health error = %v", err, healthErr)
			}
			requireReason(t, err, toolspkg.ReasonCallTimedOut)
		})
	})

	// Invariant: the configured normalization limit bounds retained raw tool listings and their descriptors.
	// Owner: MCP executor listing cache; canonical suite: TestMCPToolListCache.
	t.Run("Should bound retained normalizations to the configured capacity", func(t *testing.T) {
		t.Parallel()
		server := compozyconfig.MCPServer{
			Name: "fixture", Transport: compozyconfig.MCPServerTransportHTTP, URL: "https://example.com/mcp",
		}
		executor := newTestMCPExecutor(t, server, WithToolListNormalizationCapacity(1))
		resolved := &ResolvedServer{Server: server, Target: userMCPExecutorTarget(server.Name)}
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name}
		raw := []*mcpsdk.Tool{{Name: "echo", InputSchema: json.RawMessage(`{"type":"object"}`)}}
		for _, key := range []string{"first", "second"} {
			discovery, err := executor.normalizeToolListing(key, source, resolved, protocolVersionModern, raw)
			if err != nil || len(discovery.Descriptors) != 1 {
				t.Fatalf("discovery for %q = %+v, err = %v", key, discovery, err)
			}
		}
		if count := len(executor.toolCache.normalizations); count != 1 {
			t.Fatalf("retained normalizations = %d, want 1", count)
		}
	})

	t.Run("Should answer an HTTP cache hit without reconnecting", func(t *testing.T) {
		t.Parallel()
		sdkServer := newFakeSDKServer(nil)
		server := newTestMCPHTTPServer(sdkServer)
		t.Cleanup(server.Close)
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name: "fixture", Transport: compozyconfig.MCPServerTransportHTTP, URL: server.URL,
		})
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture"}
		initial, err := executor.DiscoverTools(t.Context(), source)
		if err != nil || len(initial.Descriptors) == 0 || initial.ProtocolVersion == "" {
			t.Fatalf("initial discovery=%+v, err=%v", initial, err)
		}
		server.Close()
		cached, err := executor.DiscoverTools(t.Context(), source)
		if err != nil || !reflect.DeepEqual(cached, initial) {
			t.Fatalf("cached discovery=%+v, err=%v", cached, err)
		}
		_, err = executor.ListTools(toolspkg.WithFreshMCPDiscovery(t.Context()), source)
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)
	})
	// Invariant: short upstream TTLs keep authoritative projection state for the listing floor.
	// Owner: MCP executor discovery; canonical suite: tool-list cache.
	t.Run("Should retain projection generation for the effective listing lifetime", func(t *testing.T) {
		t.Parallel()
		server := newTestMCPHTTPServer(newFakeSDKServer(nil))
		t.Cleanup(server.Close)
		executor := newTestMCPExecutor(
			t,
			compozyconfig.MCPServer{Name: "fixture", Transport: compozyconfig.MCPServerTransportHTTP, URL: server.URL},
			withToolListFloor(5*time.Second),
		)
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture"}
		started := time.Now()
		if _, err := executor.ListTools(t.Context(), source); err != nil {
			t.Fatal(err)
		}
		if generation, known := executor.ProjectionGeneration(
			t.Context(),
			[]toolspkg.SourceRef{source},
		); !known ||
			generation == "" {
			t.Fatalf("listing floor has no authoritative generation: %q, %t", generation, known)
		}
		executor.toolCache.mu.Lock()
		defer executor.toolCache.mu.Unlock()
		for _, listing := range executor.toolCache.entries {
			for _, projection := range executor.toolCache.projectionStates {
				if !listing.expiresAt.Equal(projection.expiresAt) ||
					projection.expiresAt.Before(started.Add(5*time.Second)) {
					t.Fatalf("listing expiry %v differs from projection %v", listing.expiresAt, projection.expiresAt)
				}
			}
		}
	})
	// Invariant: credential rotation invalidates raw listings and hosted projection generations immediately.
	// Owner: MCP executor credential binding; canonical suite: tool-list cache.
	t.Run("Should relist HTTP tools when a referenced header secret rotates", func(t *testing.T) {
		t.Parallel()
		var version atomic.Int32
		var calls atomic.Int32
		version.Store(1)
		sdkServer := newFakeSDKServer(nil)
		sdkServer.AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
			return func(ctx context.Context, method string, request mcpsdk.Request) (mcpsdk.Result, error) {
				if method != mcpToolsListMethod {
					return next(ctx, method, request)
				}
				calls.Add(1)
				return &mcpsdk.ListToolsResult{
					Tools: []*mcpsdk.Tool{
						{
							Name:        fmt.Sprintf("version_%d", version.Load()),
							InputSchema: map[string]any{"type": "object"},
						},
					},
				}, nil
			}
		})
		handler := newTestMCPHandler(sdkServer)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
			if request.Header.Get("X-Key") != fmt.Sprintf("credential-%d", version.Load()) {
				http.Error(w, "wrong credential", http.StatusUnauthorized)
				return
			}
			handler.ServeHTTP(w, request)
		}))
		t.Cleanup(server.Close)
		executor := newTestMCPExecutor(
			t,
			compozyconfig.MCPServer{
				Name:          "fixture",
				Transport:     compozyconfig.MCPServerTransportHTTP,
				URL:           server.URL,
				SecretHeaders: map[string]string{"X-Key": "vault:fixture"},
			},
			WithSecretResolver(secretRefResolverFunc(func(context.Context, string) (string, error) {
				return fmt.Sprintf("credential-%d", version.Load()), nil
			})),
		)
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture"}
		if _, err := executor.ListTools(t.Context(), source); err != nil {
			t.Fatal(err)
		}
		initial, known := executor.ProjectionGeneration(t.Context(), []toolspkg.SourceRef{source})
		if !known {
			t.Fatal("initial projection is unknown")
		}
		version.Store(2)
		if generation, known := executor.ProjectionGeneration(
			t.Context(),
			[]toolspkg.SourceRef{source},
		); known ||
			generation != "" {
			t.Fatalf("rotated credential retained generation %q", generation)
		}
		listed, err := executor.ListTools(t.Context(), source)
		if err != nil || len(listed) != 1 || listed[0].RawName != "version_2" || calls.Load() != 2 {
			t.Fatalf("rotated listing=%+v, calls=%d, error=%v", listed, calls.Load(), err)
		}
		updated, known := executor.ProjectionGeneration(t.Context(), []toolspkg.SourceRef{source})
		if !known || updated == initial {
			t.Fatalf("rotated projection=%q, known=%t", updated, known)
		}
		if _, err := executor.ListTools(toolspkg.WithFreshMCPDiscovery(t.Context()), source); err != nil {
			t.Fatal(err)
		}
		if calls.Load() != 3 {
			t.Fatalf("fresh discovery calls=%d, want 3", calls.Load())
		}
	})
	t.Run("Should rebind pooled stdio listings after a referenced secret rotates", func(t *testing.T) {
		t.Parallel()
		fixture := newStdioPoolFixture(t, StdioPoolConfig{}, nil)
		var version atomic.Int32
		version.Store(1)
		executor := newTestMCPExecutor(
			t,
			fixture.server,
			WithStdioSessionPool(fixture.pool),
			WithSecretLookup(func(key string) string {
				if key == stdioPoolSecretSource {
					return fmt.Sprintf("pool-credential-%d", version.Load())
				}
				return ""
			}),
		)
		source := toolspkg.SourceRef{
			Kind:          toolspkg.SourceMCP,
			Owner:         fixture.server.Name,
			RawServerName: fixture.server.Name,
		}
		if _, err := executor.ListTools(t.Context(), source); err != nil {
			t.Fatal(err)
		}
		initial, known := executor.ProjectionGeneration(t.Context(), []toolspkg.SourceRef{source})
		if !known {
			t.Fatal("initial stdio projection is unknown")
		}
		version.Store(2)
		if generation, known := executor.ProjectionGeneration(
			t.Context(),
			[]toolspkg.SourceRef{source},
		); known ||
			generation != "" {
			t.Fatalf("rotated stdio credential retained generation %q", generation)
		}
		if _, err := executor.ListTools(t.Context(), source); err != nil {
			t.Fatal(err)
		}
		updated, known := executor.ProjectionGeneration(t.Context(), []toolspkg.SourceRef{source})
		if !known || updated == initial {
			t.Fatalf("rebound stdio projection=%q, known=%t", updated, known)
		}
		if launches := fixture.launches(); launches != 2 {
			t.Fatalf("stdio launches after credential rotation = %d, want 2", launches)
		}
	})
	// Invariant: a successful empty listing removes all previously discovered tools, including cached reads.
	// Owner: MCP executor discovery; canonical suite: tool-list cache.
	t.Run("Should accept an empty fresh listing after tools are removed", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		sdkServer := newFakeSDKServer(nil)
		sdkServer.AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
			return func(ctx context.Context, method string, request mcpsdk.Request) (mcpsdk.Result, error) {
				if method != mcpToolsListMethod {
					return next(ctx, method, request)
				}
				if calls.Add(1) == 1 {
					return &mcpsdk.ListToolsResult{
						Tools: []*mcpsdk.Tool{{Name: "old", InputSchema: map[string]any{"type": "object"}}},
					}, nil
				}
				return &mcpsdk.ListToolsResult{}, nil
			}
		})
		server := newTestMCPHTTPServer(sdkServer)
		t.Cleanup(server.Close)
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name: "fixture", Transport: compozyconfig.MCPServerTransportHTTP, URL: server.URL,
		})
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture"}
		initial, err := executor.ListTools(t.Context(), source)
		if err != nil || len(initial) != 1 {
			t.Fatalf("initial tools=%+v, err=%v", initial, err)
		}
		for _, ctx := range []context.Context{toolspkg.WithFreshMCPDiscovery(t.Context()), t.Context()} {
			listed, err := executor.ListTools(ctx, source)
			if err != nil || len(listed) != 0 {
				t.Fatalf("removed tools remain=%+v, err=%v", listed, err)
			}
		}
	})
	// Invariant: concurrent ordinary discovery observes one listing per floor, but explicit checks always list.
	// Owner: MCP executor discovery; canonical suite: tool-list cache.
	t.Run("Should bound ordinary fanout and refresh redactions without relisting", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		sdkServer := newFakeSDKServer(nil)
		sdkServer.AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
			return func(ctx context.Context, method string, request mcpsdk.Request) (mcpsdk.Result, error) {
				if method != mcpToolsListMethod {
					return next(ctx, method, request)
				}
				calls.Add(1)
				return &mcpsdk.ListToolsResult{Tools: []*mcpsdk.Tool{{Name: "echo",
					Description: "listing-private-marker-7b9ad1", InputSchema: map[string]any{"type": "object"}}}}, nil
			}
		})
		server := newTestMCPHTTPServer(sdkServer)
		t.Cleanup(server.Close)
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name: "fixture", Transport: compozyconfig.MCPServerTransportHTTP, URL: server.URL,
		})
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture"}
		results := make(chan error, 16)
		for range 16 {
			go func() { _, err := executor.ListTools(t.Context(), source); results <- err }()
		}
		for range 16 {
			if err := <-results; err != nil {
				t.Fatal(err)
			}
		}
		if calls.Load() != 1 {
			t.Fatalf("tools/list calls=%d, want 1", calls.Load())
		}
		release := diagnostics.RegisterRequiredSecret("listing-private-marker-7b9ad1")
		t.Cleanup(release)
		listed, err := executor.ListTools(t.Context(), source)
		if err != nil || len(listed) != 1 || strings.Contains(listed[0].Description, "listing-private-marker-7b9ad1") {
			t.Fatalf("redaction did not refresh: %+v, %v", listed, err)
		}
		if calls.Load() != 1 {
			t.Fatal("secret registration forced tools/list")
		}
		if _, err := executor.ListTools(toolspkg.WithFreshMCPDiscovery(t.Context()), source); err != nil {
			t.Fatal(err)
		}
		if calls.Load() != 2 {
			t.Fatal("explicit discovery did not bypass the floor")
		}
	})
	for _, checkFirst := range []bool{false, true} {
		t.Run(
			fmt.Sprintf("Should preserve fresh cache and projection when checkFirst=%v", checkFirst),
			func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				started, release := make(chan struct{}), make(chan struct{})
				unblock := sync.OnceFunc(func() { close(release) })
				var calls atomic.Int32
				sdkServer := newFakeSDKServer(nil)
				sdkServer.AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
					return func(ctx context.Context, method string, request mcpsdk.Request) (mcpsdk.Result, error) {
						if method != mcpToolsListMethod {
							return next(ctx, method, request)
						}
						first := calls.Add(1) == 1
						if first {
							close(started)
							select {
							case <-release:
							case <-ctx.Done():
								return nil, ctx.Err()
							}
						}
						name := "old"
						if first == checkFirst {
							name = "fresh"
						}
						return &mcpsdk.ListToolsResult{
							Tools: []*mcpsdk.Tool{{Name: name, InputSchema: map[string]any{"type": "object"}}},
							TTLMs: 60_000,
						}, nil
					}
				})
				server := newTestMCPHTTPServer(sdkServer)
				t.Cleanup(server.Close)
				configured := compozyconfig.MCPServer{
					Name:      "fixture",
					Transport: compozyconfig.MCPServerTransportHTTP,
					URL:       server.URL,
				}
				executor := newTestMCPExecutor(t, configured)
				source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture"}
				done, joined := make(chan error, 1), make(chan struct{})
				t.Cleanup(func() {
					unblock()
					cancel()
					select {
					case <-joined:
					case <-time.After(5 * time.Second):
						t.Error("listing did not exit")
					}
				})
				firstCtx, secondCtx := ctx, toolspkg.WithFreshMCPDiscovery(ctx)
				if checkFirst {
					firstCtx, secondCtx = secondCtx, firstCtx
				}
				go func() { defer close(joined); _, err := executor.ListTools(firstCtx, source); done <- err }()
				select {
				case <-started:
				case err := <-done:
					t.Fatalf("listing exited before request: %v", err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				if _, err := executor.ListTools(secondCtx, source); err != nil {
					t.Fatal(err)
				}
				generation, authoritative := executor.ProjectionGeneration(ctx, []toolspkg.SourceRef{source})
				if !authoritative {
					t.Fatal("missing projection")
				}
				unblock()
				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
				cached, err := executor.ListTools(ctx, source)
				if err != nil || len(cached) != 1 || cached[0].RawName != "fresh" {
					t.Fatalf("cache=%+v err=%v", cached, err)
				}
				finalGeneration, authoritative := executor.ProjectionGeneration(ctx, []toolspkg.SourceRef{source})
				if !authoritative || (!checkFirst && finalGeneration != generation) ||
					(checkFirst && finalGeneration == generation) {
					t.Fatalf(
						"projection before=%q after=%q authoritative=%v",
						generation,
						finalGeneration,
						authoritative,
					)
				}
				if calls.Load() != 2 {
					t.Fatalf("list calls=%d, want cache hit after two discoveries", calls.Load())
				}
			},
		)
	}

	// Invariant: an explicit refresh must perform upstream I/O even on a live subscription within its TTL.
	// Owner: executor discovery cache; canonical suite: TestMCPToolListCache.
	t.Run("Should bypass an HTTP subscription cache and retry a rejected listing", func(t *testing.T) {
		t.Parallel()
		var calls atomic.Int32
		sdkServer := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "fixture"}, &mcpsdk.ServerOptions{
			Capabilities: &mcpsdk.ServerCapabilities{Tools: &mcpsdk.ToolCapabilities{ListChanged: true}},
		})
		sdkServer.AddReceivingMiddleware(func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
			return func(ctx context.Context, method string, request mcpsdk.Request) (mcpsdk.Result, error) {
				if method != mcpToolsListMethod {
					return next(ctx, method, request)
				}
				switch calls.Add(1) {
				case 1:
					return &mcpsdk.ListToolsResult{
						Tools: []*mcpsdk.Tool{{Name: "old", InputSchema: map[string]any{"type": "object"}}},
						TTLMs: 60_000,
					}, nil
				case 2:
					return nil, errors.New("fixture rejected the borrowed listing")
				}
				return &mcpsdk.ListToolsResult{
					Tools: []*mcpsdk.Tool{{Name: "fresh", InputSchema: map[string]any{"type": "object"}}},
				}, nil
			}
		})
		server := newTestMCPHTTPServer(sdkServer)
		t.Cleanup(server.Close)
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{
			Name:      "fixture",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       server.URL,
		}, WithToolCatalogNotifications())
		t.Cleanup(func() {
			if err := executor.CloseCatalogConnections(context.WithoutCancel(t.Context())); err != nil {
				t.Error(err)
			}
		})
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture"}
		ctx := testContext(t)
		key := establishTestCatalogSubscription(t, ctx, executor, source)
		if _, err := executor.ListTools(ctx, source); err != nil {
			t.Fatal(err)
		}
		entry, found := executor.cachedToolListing(key, time.Now())
		if !found || !entry.expiresAt.IsZero() {
			t.Fatal("initial listing did not use the live subscription")
		}
		if _, err := executor.ListTools(toolspkg.WithFreshMCPDiscovery(ctx), source); err != nil {
			t.Fatal(err)
		}
		listed, err := executor.ListTools(ctx, source)
		if err != nil || len(listed) != 1 || listed[0].RawName != "fresh" || calls.Load() != 3 {
			t.Fatalf(
				"listed=%+v upstream calls=%d err=%v, want the fresh check's tools and 3 calls",
				listed,
				calls.Load(),
				err,
			)
		}
	})

	t.Run("Should isolate cached descriptor schemas from caller mutation", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		executor := &CallExecutor{toolCache: mcpToolListCache{entries: make(map[string]mcpToolListCacheEntry)}}
		descriptors := []toolspkg.MCPToolDescriptor{{
			ID:           "mcp__fixture__echo",
			InputSchema:  json.RawMessage(`{"type":"object"}`),
			OutputSchema: json.RawMessage(`{"type":"string"}`),
		}}
		executor.cacheTools("fixture", ToolDiscovery{Descriptors: descriptors}, nil, 1_000, now)
		descriptors[0].InputSchema[0] = '['

		first, ok := executor.cachedTools("fixture", now)
		if !ok {
			t.Fatal("cachedTools() miss, want hit")
		}
		if got, want := string(first.Descriptors[0].InputSchema), `{"type":"object"}`; got != want {
			t.Fatalf("cached input schema = %q, want %q", got, want)
		}
		first.Descriptors[0].OutputSchema[0] = '['
		second, ok := executor.cachedTools("fixture", now)
		if !ok {
			t.Fatal("cachedTools() second miss, want hit")
		}
		if got, want := string(second.Descriptors[0].OutputSchema), `{"type":"string"}`; got != want {
			t.Fatalf("cached output schema = %q, want %q", got, want)
		}
	})

	// Invariant: removing a declaration releases its listing and projection without a normalization owner.
	// Owner: executor listing cache; canonical suite: TestMCPToolListCache.
	t.Run("Should prune removed listings after their normalizations were evicted", func(t *testing.T) {
		t.Parallel()
		executor := newTestMCPExecutor(t, compozyconfig.MCPServer{})
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: "removed"}
		now := time.Now()
		executor.cacheTools("removed", ToolDiscovery{source: source}, nil, mcpSubscribedListTTL, now)
		executor.recordToolProjectionState("removed-projection", "removed", [sha256.Size]byte{}, 0, now)
		executor.RetainCatalogSources(nil)
		if _, found := executor.cachedToolListing("removed", now); found {
			t.Fatal("removed source retained its cache entry")
		}
		if len(executor.toolCache.projectionStates) != 0 {
			t.Fatal("removed source retained its projection generation")
		}
	})

	t.Run("Should prune detached expired entries while caching a new binding", func(t *testing.T) {
		t.Parallel()

		now := time.Now()
		executor := &CallExecutor{toolCache: mcpToolListCache{entries: make(map[string]mcpToolListCacheEntry)}}
		executor.cacheTools("expired-binding", ToolDiscovery{}, nil, 1, now)
		executor.cacheTools("current-binding", ToolDiscovery{}, nil, 1_000, now.Add(mcpToolListFloor))

		executor.toolCache.mu.Lock()
		_, retainedExpired := executor.toolCache.entries["expired-binding"]
		entryCount := len(executor.toolCache.entries)
		entries := maps.Clone(executor.toolCache.entries)
		executor.toolCache.mu.Unlock()
		if retainedExpired || entryCount != 1 {
			t.Fatalf("cache entries = %#v, want only current binding", entries)
		}
	})

	t.Run("Should change the projection generation when descriptors change", func(t *testing.T) {
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
		if err := recordTestToolProjection(executor,
			mcpProjectionSourceKey(source, "", ""),
			"",
			first,
			time.Minute,
			time.Now(),
		); err != nil {
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
		if err := recordTestToolProjection(executor,
			mcpProjectionSourceKey(source, "", ""),
			"",
			second,
			time.Minute,
			time.Now(),
		); err != nil {
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
		if err := recordTestToolProjection(executor,
			mcpProjectionSourceKey(source, "", ""),
			"",
			[]toolspkg.MCPToolDescriptor{alpha, beta},
			time.Minute,
			now,
		); err != nil {
			t.Fatalf("recordToolProjection(initial) error = %v", err)
		}
		initial, known := executor.ProjectionGeneration(t.Context(), []toolspkg.SourceRef{source})
		if !known || initial == "" {
			t.Fatalf("ProjectionGeneration(initial) = %q, %t, want authoritative generation", initial, known)
		}

		reorderedDescriptors := []toolspkg.MCPToolDescriptor{beta, alpha}
		if err := recordTestToolProjection(executor,
			mcpProjectionSourceKey(source, "", ""),
			"",
			reorderedDescriptors,
			time.Minute,
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
		if err := recordTestToolProjection(executor,
			mcpProjectionSourceKey(source, "Bearer first-token", ""),
			"",
			descriptors,
			time.Minute,
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
		target := userMCPExecutorTarget(configuredServer.Name)
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
		if err := recordTestToolProjection(executor,
			mcpProjectionSourceKey(source, "", ""),
			"",
			[]toolspkg.MCPToolDescriptor{{ID: "mcp__fixture__alpha"}},
			time.Millisecond,
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

func establishTestCatalogSubscription(
	t *testing.T,
	ctx context.Context,
	executor *CallExecutor,
	source toolspkg.SourceRef,
) string {
	t.Helper()
	resolved, err := executor.resolveServer(ctx, source, "")
	if err != nil {
		t.Fatal(err)
	}
	client, err := executor.openToolListingClient(ctx, &resolved, "")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-executor.CatalogChanges():
	case <-ctx.Done():
		t.Fatal("executor never acknowledged the tool catalog subscription")
	}
	if client.catalogLive == nil || !client.catalogLive() {
		t.Fatal("tool catalog subscription is not live")
	}
	key, err := executor.toolPollKey(ctx, &resolved, "")
	if err != nil {
		t.Fatal(err)
	}
	return key
}

type countMCPJSONEncodes struct {
	count        *int
	beforeEncode func()
}

func (s countMCPJSONEncodes) MarshalJSON() ([]byte, error) {
	(*s.count)++
	if s.beforeEncode != nil {
		s.beforeEncode()
	}
	return []byte(`{"type":"object","properties":{"query":{"type":"string"}}}`), nil
}

func syntheticMCPTools(count int) []*mcpsdk.Tool {
	tools := make([]*mcpsdk.Tool, 0, count)
	for i := range count {
		tools = append(tools, &mcpsdk.Tool{
			Name:        fmt.Sprintf("catalog_tool_%04d", i),
			InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string"}}}`),
		})
	}
	return tools
}

func TestProtocolVersionMiddleware(t *testing.T) {
	t.Run("Should advertise only the modern and legacy contract versions", func(t *testing.T) {
		t.Parallel()

		versions := []string{
			protocolVersionModern,
			protocolVersionJune2025,
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
			protocolVersionJune2025,
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
				if _, ok := errors.AsType[*InsufficientScopeError](normalized); !ok {
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
		serverCtx, cancelServer := context.WithCancel(context.Background())
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
	// Invariant: tool discovery uses the allocated runtime name, retaining resource routing identity.
	// Owner: MCP descriptor projection; canonical suite: executor_test.go.
	t.Run("Should keep allocated and manual tool names distinct", func(t *testing.T) {
		t.Parallel()
		executor := &CallExecutor{}
		tool := mcpsdk.Tool{Name: "lookup", InputSchema: json.RawMessage(`{"type":"object"}`)}
		manual, err := executor.descriptorFromTool(
			toolspkg.SourceRef{},
			compozyconfig.MCPServer{Name: "github"},
			tool,
			redact.RegisteredSecretSnapshot(),
		)
		if err != nil {
			t.Fatal(err)
		}
		owned, err := executor.descriptorFromTool(
			toolspkg.SourceRef{ResourceID: "extension-record"},
			compozyconfig.MCPServer{
				Name:        "github",
				Owner:       "extension:github",
				RuntimeName: "github.github",
			},
			tool,
			redact.RegisteredSecretSnapshot(),
		)
		if err != nil {
			t.Fatal(err)
		}
		if owned.ID == manual.ID || owned.Source.RawServerName != "github.github" ||
			owned.Source.ResourceID != "extension-record" {
			t.Fatalf("allocated tool lost its runtime/resource identity: %#v", owned)
		}
	})
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
			redact.RegisteredSecretSnapshot(),
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

	// Invariant: refreshing metadata preserves the existing upstream dispatch name.
	// Owner: MCP descriptor redaction; canonical suite: executor descriptor projection.
	t.Run("Should retain an upstream dispatch name carried by the source", func(t *testing.T) {
		t.Parallel()
		descriptor := safeMCPDescriptorForRedactionTest()
		descriptor.Source.RawToolName = descriptor.RawName
		descriptor.RawName = ""
		got, err := toolspkg.RedactMCPToolDescriptor(descriptor, redact.RegisteredSecretSnapshot())
		if err != nil || got.Source.RawToolName != descriptor.Source.RawToolName {
			t.Fatalf(
				"redacted dispatch name = %q, error = %v, want %q",
				got.Source.RawToolName,
				err,
				descriptor.Source.RawToolName,
			)
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
			redact.RegisteredSecretSnapshot(),
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
				_, redactErr := toolspkg.RedactMCPToolDescriptor(descriptor, redact.RegisteredSecretSnapshot())
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
		_, err = toolspkg.RedactMCPToolDescriptor(descriptor, redact.RegisteredSecretSnapshot())
		if err == nil || !strings.Contains(err.Error(), "JSON keys collide after exact redaction") {
			t.Fatalf("RedactMCPToolDescriptor(collision schema) error = %v, want redacted key collision", err)
		}
	})
}

func TestMCPStdioServerEnvPassesCATrustBundles(t *testing.T) {
	t.Parallel()

	t.Run("Should pass every set CA trust bundle and let the server definition override it", func(t *testing.T) {
		t.Parallel()

		daemon := map[string]string{
			"NODE_EXTRA_CA_CERTS": "/etc/ssl/certs/ca-certificates.crt",
			"REQUESTS_CA_BUNDLE":  "/etc/ssl/certs/ca-certificates.crt",
			"SSL_CERT_DIR":        "/etc/ssl/certs",
			"SSL_CERT_FILE":       "/etc/ssl/daemon-ca.crt",
			"UNLISTED_SETTING":    "value",
		}
		lookup := func(key string) (string, bool) {
			value, ok := daemon[key]
			return value, ok
		}
		env := mcpServerEnvWith(map[string]string{"SSL_CERT_FILE": "/server/ca.crt"}, lookup)
		for _, want := range []string{
			"NODE_EXTRA_CA_CERTS=/etc/ssl/certs/ca-certificates.crt",
			"REQUESTS_CA_BUNDLE=/etc/ssl/certs/ca-certificates.crt",
			"SSL_CERT_DIR=/etc/ssl/certs",
			"SSL_CERT_FILE=/server/ca.crt",
		} {
			if !slices.Contains(env, want) {
				t.Fatalf("mcpServerEnvWith() = %#v, missing %q", env, want)
			}
		}
		if slices.Contains(env, "UNLISTED_SETTING=value") {
			t.Fatalf("mcpServerEnvWith() passed a non-allowlisted variable: %#v", env)
		}
	})
}

func TestMCPCallExecutorStdioEnvironmentBoundary(t *testing.T) {
	t.Run("Should report a PATH-miss launch failure for only the target server", func(t *testing.T) {
		t.Parallel()

		registry := NewRuntimeHealthRegistry()
		server := compozyconfig.MCPServer{
			Name:      "missing-command",
			Transport: compozyconfig.MCPServerTransportStdio,
			Command:   "compozy-mcp-command-that-does-not-exist",
		}
		healthKey := RuntimeHealthKey{
			InstanceName: "kit", BundleGeneration: "generation-path-miss", ServerName: server.Name,
		}
		executor, err := NewMCPCallExecutor(
			ServerResolverFunc(func(context.Context, toolspkg.SourceRef, string) (ResolvedServer, error) {
				return ResolvedServer{
					Server: server, Target: userMCPExecutorTarget(server.Name), HealthKey: healthKey,
				}, nil
			}),
			WithDefinitionDigestKey([]byte("test installation key")),
			WithRuntimeHealthRegistry(registry),
		)
		if err != nil {
			t.Fatalf("NewMCPCallExecutor() error = %v", err)
		}
		_, err = executor.ListTools(t.Context(), toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name,
		})
		if err == nil {
			t.Fatal("ListTools() error = nil, want PATH-miss launch failure")
		}
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)
		entries := registry.Entries("kit", "", "generation-path-miss")
		if len(entries) != 1 || entries[0].Key.ServerName != server.Name ||
			!strings.Contains(entries[0].Message, server.Command) {
			t.Fatalf("runtime health entries = %#v, want only %q", entries, server.Name)
		}
	})

	t.Run("Should report first-launch data-directory failure for only the target server", func(t *testing.T) {
		t.Parallel()

		registry := NewRuntimeHealthRegistry()
		server := compozyconfig.MCPServer{
			Name: "broken-stdio", Transport: compozyconfig.MCPServerTransportStdio,
			Command: os.Args[0], Env: map[string]string{pluginDataEnvironmentName: filepath.Join(t.TempDir(), "data")},
		}
		healthKey := RuntimeHealthKey{
			InstanceName: "kit", BundleGeneration: "generation-a", ServerName: server.Name,
		}
		executor, err := NewMCPCallExecutor(
			ServerResolverFunc(func(context.Context, toolspkg.SourceRef, string) (ResolvedServer, error) {
				return ResolvedServer{
					Server: server, Target: userMCPExecutorTarget(server.Name), HealthKey: healthKey,
				}, nil
			}),
			WithDefinitionDigestKey([]byte("test installation key")),
			WithRuntimeHealthRegistry(registry),
			WithDataDirCreator(func(string) error { return errors.New("injected data directory failure") }),
		)
		if err != nil {
			t.Fatalf("NewMCPCallExecutor() error = %v", err)
		}
		_, err = executor.ListTools(t.Context(), toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name,
		})
		if err == nil {
			t.Fatal("ListTools() error = nil, want data-directory failure")
		}
		requireReason(t, err, toolspkg.ReasonMCPUnreachable)
		entries := registry.Entries("kit", "", "generation-a")
		if len(entries) != 1 || entries[0].Key.ServerName != "broken-stdio" ||
			!strings.Contains(entries[0].Message, "injected data directory failure") {
			t.Fatalf("runtime health entries = %#v, want only broken-stdio", entries)
		}
		if sibling := registry.Entries("kit", "", "generation-b"); len(sibling) != 0 {
			t.Fatalf("sibling generation health = %#v, want empty", sibling)
		}
	})

	t.Run("Should clear a remote transport failure after the next successful exchange", func(t *testing.T) {
		t.Parallel()

		registry := NewRuntimeHealthRegistry()
		failedServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
			http.Error(writer, "temporarily unavailable", http.StatusServiceUnavailable)
		}))
		failedURL := failedServer.URL
		failedServer.Close()
		healthKey := RuntimeHealthKey{
			InstanceName: "kit", BundleGeneration: "generation-http", ServerName: "deployment-api",
		}
		var serverMu sync.RWMutex
		server := compozyconfig.MCPServer{
			Name: healthKey.ServerName, Transport: compozyconfig.MCPServerTransportHTTP, URL: failedURL,
		}
		executor, err := NewMCPCallExecutor(
			ServerResolverFunc(func(context.Context, toolspkg.SourceRef, string) (ResolvedServer, error) {
				serverMu.RLock()
				resolved := server
				serverMu.RUnlock()
				return ResolvedServer{
					Server: resolved, Target: userMCPExecutorTarget(resolved.Name), HealthKey: healthKey,
				}, nil
			}),
			WithDefinitionDigestKey([]byte("test installation key")),
			WithRuntimeHealthRegistry(registry),
		)
		if err != nil {
			t.Fatalf("NewMCPCallExecutor() error = %v", err)
		}
		source := toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name,
		}
		if _, err := executor.ListTools(t.Context(), source); err == nil {
			t.Fatal("ListTools(failed endpoint) error = nil, want connection failure")
		}
		entries := registry.Entries("kit", "", "generation-http")
		if len(entries) != 1 || entries[0].Key.ServerName != healthKey.ServerName {
			t.Fatalf("runtime health entries = %#v, want only %q", entries, healthKey.ServerName)
		}

		healthyServer := newTestMCPHTTPServer(newFakeSDKServer(nil))
		t.Cleanup(healthyServer.Close)
		serverMu.Lock()
		server.URL = healthyServer.URL
		serverMu.Unlock()
		if _, err := executor.ListTools(t.Context(), source); err != nil {
			t.Fatalf("ListTools(recovered endpoint) error = %v", err)
		}
		if entries := registry.Entries("kit", "", "generation-http"); len(entries) != 0 {
			t.Fatalf("runtime health after successful exchange = %#v, want empty", entries)
		}
	})

	t.Run("Should not record invalid caller input as a server outage", func(t *testing.T) {
		t.Parallel()

		server := newTestMCPHTTPServer(newFakeSDKServer(nil))
		t.Cleanup(server.Close)
		health := NewRuntimeHealthRegistry()
		healthKey := RuntimeHealthKey{
			InstanceName: "kit", BundleGeneration: "generation-input", ServerName: "fixture",
		}
		executor, err := NewMCPCallExecutor(
			ServerResolverFunc(func(context.Context, toolspkg.SourceRef, string) (ResolvedServer, error) {
				resolved := compozyconfig.MCPServer{
					Name: "fixture", Transport: compozyconfig.MCPServerTransportHTTP, URL: server.URL,
				}
				return ResolvedServer{
					Server: resolved, Target: userMCPExecutorTarget(resolved.Name), HealthKey: healthKey,
				}, nil
			}),
			WithDefinitionDigestKey([]byte("test installation key")),
			WithRuntimeHealthRegistry(health),
		)
		if err != nil {
			t.Fatalf("NewMCPCallExecutor() error = %v", err)
		}
		_, err = executor.CallTool(t.Context(), toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: "fixture", RawServerName: "fixture",
		}, toolspkg.MCPToolCallRequest{
			ToolID: "mcp__fixture__echo", RawToolName: "echo", Input: json.RawMessage(`[`),
		})
		if !errors.Is(err, toolspkg.ErrToolInvalidInput) || !strings.Contains(err.Error(), "invalid JSON") {
			t.Fatalf("CallTool(invalid input) error = %v, want invalid JSON input", err)
		}
		if entries := health.Entries("kit", "", "generation-input"); len(entries) != 0 {
			t.Fatalf("runtime health entries = %#v, want caller error excluded", entries)
		}
	})

	t.Run("Should isolate subprocess homes by target and remove them after use", func(t *testing.T) {
		setMCPTestEnv(t, "HOME", "/operator-home")

		executor := &CallExecutor{}
		server := compozyconfig.MCPServer{
			Name:      "local",
			Transport: compozyconfig.MCPServerTransportStdio,
			Command:   os.Args[0],
		}
		firstTarget := userMCPExecutorTarget("first")
		secondTarget := userMCPExecutorTarget("second")
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
		t.Parallel()

		const secret = "q7Z"
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
			WithSecretLookup(func(key string) string {
				if key == stdioExplicitSecretSource {
					return secret
				}
				return ""
			}),
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
		t.Parallel()

		const secret = "m8#"
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
		executor := newTestMCPExecutor(t, server, WithSecretLookup(func(key string) string {
			if key == stdioExplicitSecretSource {
				return secret
			}
			return ""
		}))
		resolved := ResolvedServer{Server: server, Target: userMCPExecutorTarget(server.Name)}
		client, err := executor.openClient(ctx, &resolved, "")
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

	t.Run("Should launch stdio servers in the projected working directory", func(t *testing.T) {
		t.Parallel()

		workingDir := t.TempDir()
		executor := &CallExecutor{}
		server := compozyconfig.MCPServer{
			Name: "local", Transport: compozyconfig.MCPServerTransportStdio,
			Command: os.Args[0], CWD: workingDir,
		}
		launch, err := executor.newMCPStdioLaunch(
			t.Context(),
			server,
			userMCPExecutorTarget(server.Name),
		)
		if err != nil {
			t.Fatalf("newMCPStdioLaunch() error = %v", err)
		}
		t.Cleanup(func() {
			if cleanupErr := launch.cleanup(); cleanupErr != nil {
				t.Errorf("stdio home cleanup error = %v", cleanupErr)
			}
		})
		if launch.command.Dir != workingDir {
			t.Fatalf("command.Dir = %q, want %q", launch.command.Dir, workingDir)
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
	// Invariant: a 401 is attributed to the credential actually read during a concurrent replacement.
	// Owner: MCP request authorization; canonical suite: TestMCPCallExecutorHelpers.
	t.Run("Should record rejection for a token replaced during authorization lookup", func(t *testing.T) {
		t.Parallel()
		server := authEnabledServer("fixture", compozyconfig.MCPServerTransportHTTP, "https://mcp.example.test")
		target := userMCPExecutorTarget(server.Name)
		generation := mcpauth.NewMutationGeneration()
		store := &tokenReadHookStore{memoryTokenStore: newMemoryTokenStore()}
		token := mcpauth.TokenRecord{
			Target: target, DefinitionFingerprint: definitionFingerprint(t, target, server),
			Issuer: server.Auth.IssuerURL, ClientID: server.Auth.ClientID, Scopes: server.Auth.Scopes,
			AccessToken: "original-fixture-access", TokenType: "Bearer",
		}
		if err := store.SaveMCPAuthToken(t.Context(), token); err != nil {
			t.Fatal(err)
		}
		store.beforeRead = func(ctx context.Context) error {
			store.beforeRead = nil
			current, err := generation.Snapshot(target)
			if err != nil {
				return err
			}
			token.AccessToken = "replacement-fixture-access"
			committed, err := generation.CommitAndAdvanceIfCurrent(target, current, func() error {
				return store.SaveMCPAuthToken(ctx, token)
			})
			if err == nil && !committed {
				return mcpauth.ErrTokenMutationStale
			}
			return err
		}
		executor := newTestMCPExecutor(t, server, WithTokenStore(store), WithAuthMutationGeneration(generation))
		resolved := ResolvedServer{Server: server, Target: target}
		header, err := executor.authorizationHeader(t.Context(), &resolved)
		if err != nil || header != "Bearer replacement-fixture-access" {
			t.Fatalf("did not read the replacement credential: %v", err)
		}
		executor.recordBearerRejection(t.Context(), &resolved, mcpauth.ErrTokenRejected)
		status, err := executor.authStatus(t.Context(), &resolved)
		if err != nil || status.Status != string(mcpauth.StatusNeedsLogin) {
			t.Fatalf("replacement rejection produced auth status %+v, %v", status, err)
		}
	})
	t.Run("Should reject construction without an installation digest key", func(t *testing.T) {
		t.Parallel()
		resolver := ServerResolverFunc(func(context.Context, toolspkg.SourceRef, string) (ResolvedServer, error) {
			return ResolvedServer{}, nil
		})
		if executor, err := NewMCPCallExecutor(resolver); err == nil || executor != nil {
			t.Fatal("executor accepted a missing digest key")
		}
	})
	t.Run("Should bind auth to the installation digest of the unresolved definition", func(t *testing.T) {
		t.Parallel()
		key := []byte("installation auth key")
		server := compozyconfig.MCPServer{
			Name: "remote", Transport: "http", URL: "https://example.com/${TENANT}",
			InputValues: map[string]compozyconfig.MCPInputValue{"TENANT": {Value: "private-tenant"}},
		}
		executor := newTestMCPExecutor(t, server, WithDefinitionDigestKey(key))
		cfg, err := executor.serverAuthConfig(
			t.Context(),
			&ResolvedServer{Server: server, Target: userMCPExecutorTarget(server.Name)},
			nil,
		)
		if err != nil {
			t.Fatal(err)
		}
		expected, err := compozyconfig.MCPDefinitionDigest(server, key)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.DefinitionDigest != expected || cfg.RemoteURL != "https://example.com/private-tenant" {
			t.Fatalf("auth definition identity differs: %+v", cfg)
		}
	})
	t.Run("Should preserve the explicit definition owner through auth diagnostics", func(t *testing.T) {
		t.Parallel()
		server := compozyconfig.MCPServer{Name: "github", Owner: "extension:bundle", Command: "test-mcp"}
		executor := newTestMCPExecutor(t, server)
		executor.servers = ServerResolverFunc(
			func(_ context.Context, source toolspkg.SourceRef, owner string) (ResolvedServer, error) {
				if owner != "extension:bundle" || source.RawServerName != "github" {
					t.Fatalf("diagnostic selection = %q/%q", owner, source.RawServerName)
				}
				target := userMCPExecutorTarget(server.Name)
				target.Owner = owner
				return ResolvedServer{Server: server, Target: target}, nil
			},
		)
		status, err := executor.Status(testContext(t), toolspkg.SourceRef{
			Kind: toolspkg.SourceMCP, Owner: "github", RawServerName: "github",
		}, "extension:bundle")
		if err != nil || status.Owner != "extension:bundle" || status.ServerName != "github" {
			t.Fatalf("diagnostic status = %#v, error = %v", status, err)
		}
	})
	// Invariant: the native status reader can report an off server with an unavailable OAuth secret.
	// Owner: MCP executor status; canonical suite: TestMCPCallExecutor.
	t.Run("Should report off when the OAuth client secret is unavailable", func(t *testing.T) {
		t.Parallel()
		server := authEnabledServer("secure", compozyconfig.MCPServerTransportHTTP, "https://mcp.example.test/mcp")
		server.Disabled = new(true)
		server.Auth.ClientSecretRef = "env:MCP_CLIENT_SECRET"
		executor := newTestMCPExecutor(t, server, withAuthService(&fakeAuthService{}),
			WithSecretLookup(func(string) string { return "" }))
		source := toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name}
		status, err := executor.Status(t.Context(), source, "")
		if err != nil || !status.Disabled || status.ServerName != server.Name ||
			status.Scope != string(mcpauth.ScopeUser) {
			t.Fatalf("disabled status = %+v, err = %v", status, err)
		}
	})
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
		}, "")
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
		if status.Owner != cfg.Target.Owner || status.Scope != string(cfg.Target.Scope) ||
			status.WorkspaceID != cfg.Target.WorkspaceID {
			t.Fatalf("redacted status lost auth target identity: %#v, want %#v", status, cfg.Target)
		}
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
		}, "")
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
		_, err = executor.Status(nilContext, toolspkg.SourceRef{RawServerName: "github"}, "")
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
		resolved := ResolvedServer{Server: server, Target: userMCPExecutorTarget("secure")}
		if err := executor.ensureAuthorized(testContext(t), &resolved); err != nil {
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
		err := executor.ensureAuthorized(testContext(t), &resolved)
		requireReason(t, err, toolspkg.ReasonMCPAuthInvalid)
	})

	t.Run("Should Handle Authorization Header Edge Cases And Server Matching", func(t *testing.T) {
		t.Parallel()

		store := newMemoryTokenStore()
		if err := store.SaveMCPAuthToken(t.Context(), mcpauth.TokenRecord{
			Target: userMCPExecutorTarget("github"),
			DefinitionFingerprint: definitionFingerprint(t, userMCPExecutorTarget("github"),
				authEnabledServer("github", compozyconfig.MCPServerTransportHTTP, "https://mcp.example.test/mcp")),
			Issuer: "https://issuer.example.test", ClientID: "client-id", Scopes: []string{"tools.read"},
			AccessToken: "token", TokenType: "mac",
		}); err != nil {
			t.Fatalf("SaveMCPAuthToken(non-bearer) error = %v", err)
		}
		server := authEnabledServer("github", compozyconfig.MCPServerTransportHTTP, "https://mcp.example.test/mcp")
		executor := newTestMCPExecutor(t, server, WithTokenStore(store))
		resolved := ResolvedServer{
			Server: server,
			Target: userMCPExecutorTarget("github"),
		}
		got, err := executor.authorizationHeader(testContext(t), &resolved)
		if got != "" {
			t.Fatal("authorizationHeader(non-bearer) sent a credential")
		}
		requireReason(t, err, toolspkg.ReasonMCPAuthInvalid)
		if got, err := (&CallExecutor{}).authorizationHeader(
			testContext(t),
			&resolved,
		); got != "" || err != nil {
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
		target := userMCPExecutorTarget("secure")
		for _, fixture := range []struct {
			name   string
			mutate func(*mcpauth.TokenRecord)
			want   string
			reason toolspkg.ReasonCode
		}{
			{
				name: "Should reject a mismatched issuer",
				mutate: func(token *mcpauth.TokenRecord) {
					token.Issuer = "https://other-issuer.example.test"
				},
			},
			{
				name: "Should reject a mismatched client",
				mutate: func(token *mcpauth.TokenRecord) {
					token.ClientID = "other-client"
				},
			},
			{
				name: "Should reject missing baseline scope",
				mutate: func(token *mcpauth.TokenRecord) {
					token.Scopes = []string{"other.scope"}
				},
				reason: toolspkg.ReasonMCPAuthInvalid,
			},
			{
				name: "Should accept a step-up scope superset",
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
				got, err := executor.authorizationHeader(t.Context(), &resolved)
				if got != fixture.want {
					t.Fatal("authorizationHeader() sent unexpected bearer material")
				}
				if fixture.want == "" {
					requireReason(t, err, toolspkg.ReasonMCPAuthInvalid)
				} else if err != nil {
					t.Fatal(err)
				}
				if fixture.reason != "" {
					requireReason(t, executor.ensureAuthorized(t.Context(), &resolved), fixture.reason)
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

	options = append([]CallExecutorOption{WithDefinitionDigestKey([]byte("test installation key"))}, options...)
	executor, err := NewMCPCallExecutor(
		ServerResolverFunc(func(context.Context, toolspkg.SourceRef, string) (ResolvedServer, error) {
			return ResolvedServer{Server: server, Target: userMCPExecutorTarget(server.Name)}, nil
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

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
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
	if status.Owner == "" {
		status.Owner = cfg.Target.Owner
	}
	if status.Scope == "" {
		status.Scope = cfg.Target.Scope
	}
	if status.WorkspaceID == "" {
		status.WorkspaceID = cfg.Target.WorkspaceID
	}
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
	if status.Owner == "" {
		status.Owner = cfg.Target.Owner
	}
	if status.Scope == "" {
		status.Scope = cfg.Target.Scope
	}
	if status.WorkspaceID == "" {
		status.WorkspaceID = cfg.Target.WorkspaceID
	}
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
	getErr        error
	tokens        map[string]mcpauth.TokenRecord
	registrations map[string]mcpauth.ClientRegistration
}

type tokenReadHookStore struct {
	*memoryTokenStore
	beforeRead func(context.Context) error
}

func (s *tokenReadHookStore) GetMCPAuthToken(ctx context.Context, target mcpauth.Target) (mcpauth.TokenRecord, error) {
	if s.beforeRead != nil {
		if err := s.beforeRead(ctx); err != nil {
			return mcpauth.TokenRecord{}, err
		}
	}
	return s.memoryTokenStore.GetMCPAuthToken(ctx, target)
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
	if s.getErr != nil {
		return mcpauth.TokenRecord{}, s.getErr
	}

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

func userMCPExecutorTarget(serverName string) mcpauth.Target {
	return mcpauth.Target{Scope: mcpauth.ScopeUser, ServerName: serverName}
}

func TestCheckInReadOnlyOverride(t *testing.T) {
	t.Run("Should accept only operator overrides and preserve destructive hints [UT-002]", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct {
			name, owner, catalog      string
			origin                    toolspkg.MCPReadOnlyOverrideOrigin
			destructive, wantOverride bool
		}{
			{name: "Should retain operator override for authority mapping", wantOverride: true, origin: toolspkg.MCPReadOnlyOverrideOperatorConfig},
			{name: "Should retain agent origin for authority mapping", wantOverride: true, origin: toolspkg.MCPReadOnlyOverrideAgentDefinition},
			{name: "Should ignore extension override", owner: "extension:fixture"},
			{name: "Should ignore catalog override", catalog: "fixture"},
			{name: "Should preserve destructive hint", destructive: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				executor := &CallExecutor{}
				tool := mcpsdk.Tool{
					Name:        "read",
					InputSchema: json.RawMessage(`{"type":"object"}`),
					Annotations: &mcpsdk.ToolAnnotations{DestructiveHint: new(tc.destructive)},
				}
				discovery, err := executor.descriptorsFromTools(
					toolspkg.SourceRef{},
					&ResolvedServer{ReadOnlyOverrideOrigin: new(tc.origin), Server: compozyconfig.MCPServer{
						Name:          "fixture",
						Owner:         tc.owner,
						CatalogEntry:  tc.catalog,
						ReadOnlyTools: []string{"read"},
					}},
					protocolVersionLegacy,
					[]*mcpsdk.Tool{&tool},
					redact.RegisteredSecretSnapshot(),
				)
				if err != nil {
					t.Fatal(err)
				}
				d := discovery.Descriptors[0]
				if d.ReadOnlyOverrideOrigin != tc.origin || d.ReadOnly || d.ReadOnlyOverride != tc.wantOverride ||
					d.Destructive != tc.destructive {
					t.Fatalf("descriptor=%+v", d)
				}
			})
		}
	})
}

func TestClientInputConnection(t *testing.T) {
	t.Parallel()
	t.Run("Should resolve derived URL and bearer input against a real MCP server", func(t *testing.T) {
		t.Parallel()
		const secret = "private-${HOST}-token"
		handler := newTestMCPHandler(newFakeSDKServer(nil))
		remote := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+secret {
				http.Error(w, "missing input", http.StatusUnauthorized)
				return
			}
			handler.ServeHTTP(w, r)
		}))
		t.Cleanup(remote.Close)
		server := compozyconfig.MCPServer{
			Name:      "client",
			Owner:     "extension:client",
			Transport: compozyconfig.MCPServerTransportHTTP,
			URL:       "http://${HOST}/mcp",
			Headers:   map[string]string{"Authorization": "Bearer ${API_KEY}"},
			InputValues: map[string]compozyconfig.MCPInputValue{
				"HOST": {
					Value: strings.TrimPrefix(remote.URL, "http://"),
				},
				"API_KEY": {SecretRef: "vault:extensions/global/client/env/API_KEY"},
			},
		}
		executor := newTestMCPExecutor(
			t,
			server,
			WithSecretResolver(
				secretRefResolverFunc(func(context.Context, string) (string, error) { return secret, nil }),
			),
		)
		if _, err := executor.ListTools(
			t.Context(),
			toolspkg.SourceRef{Kind: toolspkg.SourceMCP, Owner: server.Name, RawServerName: server.Name},
		); err != nil {
			t.Fatal(err)
		}
		serialized, err := json.Marshal(server)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(serialized, []byte(secret)) {
			t.Fatal("published declaration leaked secret")
		}
	})
}

func TestClientStdioLiteralInputs(t *testing.T) {
	t.Parallel()
	t.Run("Should preserve spaces and empty rendered arguments", func(t *testing.T) {
		t.Parallel()
		server := compozyconfig.MCPServer{
			Name:        "literal",
			Command:     "node",
			Args:        []string{" ${QUERY} ", "${EMPTY}"},
			InputValues: map[string]compozyconfig.MCPInputValue{"QUERY": {Value: "  query  "}, "EMPTY": {Value: ""}},
		}
		executor := newTestMCPExecutor(t, server)
		spec, err := executor.resolveMCPStdioSpec(t.Context(), server, userMCPExecutorTarget(server.Name))
		if err != nil || !slices.Equal(spec.args, []string{"  query  ", ""}) {
			t.Fatalf("literal arguments = %q, %v", spec.args, err)
		}
	})
}
