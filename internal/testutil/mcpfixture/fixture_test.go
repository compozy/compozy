// Suite: MCP fixture transports
// Invariant: every profile exposes deterministic official-SDK tools/list and tools/call behavior through its declared transport.
// Boundary IN: mcpfixture's public server, HTTP, and stdio adapters.
// Boundary OUT: runtime descriptor caching and OAuth lifecycle ownership in internal/mcp.
package mcpfixture

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestFixtureStreamableHTTP(t *testing.T) {
	t.Run("Should serve modern tools with stateless pagination and private TTL", func(t *testing.T) {
		t.Parallel()

		fixture := MustNew(ProfileModern2026)
		server := fixture.StartHTTP(t)
		session := connectHTTPClient(t, server.URL)

		if got, want := session.InitializeResult().ProtocolVersion, ModernProtocolVersion; got != want {
			t.Fatalf("negotiated protocol = %q, want %q", got, want)
		}
		firstPage, err := session.ListTools(t.Context(), nil)
		if err != nil {
			t.Fatalf("ListTools(first page) error = %v", err)
		}
		if got, want := len(firstPage.Tools), 1; got != want {
			t.Fatalf("len(first page tools) = %d, want %d", got, want)
		}
		if got, want := firstPage.Tools[0].Name, "echo"; got != want {
			t.Fatalf("first page tool = %q, want %q", got, want)
		}
		if got, want := firstPage.TTLMs, ToolListTTLMs; got != want {
			t.Fatalf("first page ttlMs = %d, want %d", got, want)
		}
		if got, want := firstPage.CacheScope, "private"; got != want {
			t.Fatalf("first page cacheScope = %q, want %q", got, want)
		}
		if firstPage.NextCursor == "" {
			t.Fatal("first page next cursor is empty")
		}
		secondPage, err := session.ListTools(t.Context(), &mcp.ListToolsParams{Cursor: firstPage.NextCursor})
		if err != nil {
			t.Fatalf("ListTools(second page) error = %v", err)
		}
		if got, want := len(secondPage.Tools), 1; got != want {
			t.Fatalf("len(second page tools) = %d, want %d", got, want)
		}
		if got, want := secondPage.Tools[0].Name, "status"; got != want {
			t.Fatalf("second page tool = %q, want %q", got, want)
		}

		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{
			Name:      "echo",
			Arguments: map[string]any{"message": "hello"},
		})
		if err != nil {
			t.Fatalf("CallTool(echo) error = %v", err)
		}
		if result == nil {
			t.Fatal("CallTool(echo) result = nil")
		}
		structured, ok := result.StructuredContent.(map[string]any)
		if !ok {
			t.Fatalf("echo structured content = %#v, want object", result.StructuredContent)
		}
		if got, want := structured["message"], "hello"; got != want {
			t.Fatalf("echo structured message = %#v, want %q", got, want)
		}
	})

	t.Run("Should fall back to legacy initialize negotiation", func(t *testing.T) {
		t.Parallel()

		fixture := MustNew(ProfileLegacy2025)
		session := connectHTTPClient(t, fixture.StartHTTP(t).URL)
		if got, want := session.InitializeResult().ProtocolVersion, LegacyProtocolVersion; got != want {
			t.Fatalf("negotiated protocol = %q, want %q", got, want)
		}
	})
}

func TestFixtureInputRequired(t *testing.T) {
	t.Run("Should expose deterministic input-required result", func(t *testing.T) {
		t.Parallel()

		fixture := MustNew(ProfileInputRequired)
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		serverSession, err := fixture.Server().Connect(t.Context(), serverTransport, nil)
		if err != nil {
			t.Fatalf("Server.Connect() error = %v", err)
		}
		t.Cleanup(func() {
			if err := serverSession.Close(); err != nil {
				t.Errorf("ServerSession.Close() error = %v", err)
			}
		})
		client := newClient()
		session, err := client.Connect(t.Context(), clientTransport, nil)
		if err != nil {
			t.Fatalf("Client.Connect() error = %v", err)
		}
		t.Cleanup(func() {
			if err := session.Close(); err != nil {
				t.Errorf("ClientSession.Close() error = %v", err)
			}
		})

		result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "confirm"})
		if err != nil {
			t.Fatalf("CallTool(confirm) error = %v", err)
		}
		if !result.NeedsInput() {
			t.Fatal("CallTool(confirm) NeedsInput() = false, want true")
		}
		if _, ok := result.InputRequests["confirm"]; !ok {
			t.Fatalf("CallTool(confirm) input requests = %#v, want confirm", result.InputRequests)
		}
		if got, want := result.RequestState, "fixture-confirmation"; got != want {
			t.Fatalf("CallTool(confirm) request state = %q, want %q", got, want)
		}
	})
}

func TestFixtureRunStdio(t *testing.T) {
	t.Run("Should serve a client over injected stdio streams", func(t *testing.T) {
		t.Parallel()

		fixture := MustNew(ProfileModern2026)
		serverConnection, clientConnection := net.Pipe()
		ctx, cancel := context.WithCancel(t.Context())
		t.Cleanup(cancel)
		serverDone := make(chan error, 1)
		go func() {
			serverDone <- fixture.RunStdio(ctx, serverConnection, serverConnection)
		}()

		session, err := newClient().Connect(ctx, &mcp.IOTransport{
			Reader: clientConnection,
			Writer: clientConnection,
		}, nil)
		if err != nil {
			t.Fatalf("Client.Connect() error = %v", err)
		}
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "echo",
			Arguments: map[string]any{"message": "stdio"},
		})
		if err != nil {
			t.Fatalf("CallTool(echo) error = %v", err)
		}
		if result == nil {
			t.Fatal("CallTool(echo) result = nil")
		}
		structured, ok := result.StructuredContent.(map[string]any)
		if !ok {
			t.Fatalf("echo structured content = %#v, want object", result.StructuredContent)
		}
		if got, want := structured["message"], "stdio"; got != want {
			t.Fatalf("echo structured message = %#v, want %q", got, want)
		}
		if err := session.Close(); err != nil {
			t.Fatalf("ClientSession.Close() error = %v", err)
		}
		cancel()
		select {
		case err := <-serverDone:
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, io.EOF) {
				t.Fatalf("RunStdio() error = %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("RunStdio() did not stop after context cancellation")
		}
	})
}

func TestFixtureValidation(t *testing.T) {
	t.Run("Should reject unsupported profiles and incomplete stdio streams", func(t *testing.T) {
		t.Parallel()

		if _, err := New(Profile("unsupported")); err == nil || !strings.Contains(err.Error(), "profile") {
			t.Fatalf("New(unsupported profile) error = %v, want profile validation", err)
		}
		fixture := MustNew(ProfileModern2026)
		if err := fixture.RunStdio(t.Context(), nil, nil); err == nil || !strings.Contains(err.Error(), "input") {
			t.Fatalf("RunStdio(nil input) error = %v, want input validation", err)
		}
		input := io.NopCloser(strings.NewReader(""))
		if err := fixture.RunStdio(t.Context(), input, nil); err == nil || !strings.Contains(err.Error(), "output") {
			t.Fatalf("RunStdio(nil output) error = %v, want output validation", err)
		}
		if err := input.Close(); err != nil {
			t.Fatalf("stdio input close error = %v", err)
		}
	})
}

func connectHTTPClient(t *testing.T, endpoint string) *mcp.ClientSession {
	t.Helper()
	session, err := newClient().Connect(t.Context(), &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           &http.Client{Timeout: 5 * time.Second},
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	if err != nil {
		t.Fatalf("Client.Connect() error = %v", err)
	}
	t.Cleanup(func() {
		if err := session.Close(); err != nil {
			t.Errorf("ClientSession.Close() error = %v", err)
		}
	})
	return session
}

func newClient() *mcp.Client {
	return mcp.NewClient(&mcp.Implementation{Name: "mcpfixture-test-client", Version: "1.0.0"}, &mcp.ClientOptions{
		Capabilities:   &mcp.ClientCapabilities{},
		MultiRoundTrip: &mcp.MultiRoundTripOptions{Disabled: true},
	})
}
