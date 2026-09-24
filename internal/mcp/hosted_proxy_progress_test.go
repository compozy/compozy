package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"testing/synctest"

	"github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

func newProgressKeepaliveFixture(t *testing.T, sending ...sdkmcp.Middleware) (
	*sdkmcp.ClientSession,
	chan *sdkmcp.ProgressNotificationParams,
	chan HostedCallResponse,
) {
	t.Helper()

	release := make(chan HostedCallResponse, 1)
	stub := newHostedProxyClientStub(HostedBindResponse{BindID: "bind-1"})
	stub.block = func(HostedCallRequest) (HostedCallResponse, error) {
		return <-release, nil
	}
	server := newHostedProxyTestServer()
	server.AddSendingMiddleware(sending...)
	applyHostedTools(server, stub, "bind-1", []tools.ToolView{hostedToolView("compozy__hosted_echo")})

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	serverSession, err := server.Connect(t.Context(), serverTransport, nil)
	if err != nil {
		t.Fatalf("server.Connect() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := serverSession.Close(); closeErr != nil {
			t.Errorf("serverSession.Close() error = %v", closeErr)
		}
	})
	progress := make(chan *sdkmcp.ProgressNotificationParams, 8)
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "progress-test", Version: "1.0.0"},
		&sdkmcp.ClientOptions{
			ProgressNotificationHandler: func(_ context.Context, req *sdkmcp.ProgressNotificationClientRequest) {
				progress <- req.Params
			},
		},
	)
	clientSession, err := client.Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := clientSession.Close(); closeErr != nil {
			t.Errorf("clientSession.Close() error = %v", closeErr)
		}
	})
	return clientSession, progress, release
}

func releaseHostedCall(t *testing.T, release chan<- HostedCallResponse, callErr <-chan error) {
	t.Helper()

	release <- HostedCallResponse{Result: tools.ToolResult{
		Structured: json.RawMessage(`{"ok":true}`),
		Trust:      tools.ResultTrustTrusted,
	}}
	if err := <-callErr; err != nil {
		t.Fatalf("CallTool() error = %v", err)
	}
}

func TestHostedToolProgressKeepalive(t *testing.T) {
	t.Run("Should finish when progress delivery stalls", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			entered := make(chan struct{})
			blockProgress := func(next sdkmcp.MethodHandler) sdkmcp.MethodHandler {
				return func(ctx context.Context, method string, req sdkmcp.Request) (sdkmcp.Result, error) {
					if method == "notifications/progress" {
						close(entered)
						<-ctx.Done()
						return nil, ctx.Err()
					}
					return next(ctx, method, req)
				}
			}
			clientSession, _, release := newProgressKeepaliveFixture(t, blockProgress)
			params := &sdkmcp.CallToolParams{Name: "compozy__hosted_echo"}
			params.SetProgressToken("keepalive-token")
			callErr := make(chan error, 1)
			go func() {
				_, err := clientSession.CallTool(t.Context(), params)
				callErr <- err
			}()

			<-entered
			releaseHostedCall(t, release, callErr)
		})
	})

	t.Run("Should send progress notifications while a blocked call waits", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			clientSession, progress, release := newProgressKeepaliveFixture(t)

			params := &sdkmcp.CallToolParams{
				Name:      "compozy__hosted_echo",
				Arguments: map[string]any{"message": "hi"},
			}
			params.SetProgressToken("keepalive-token")
			callErr := make(chan error, 1)
			go func() {
				_, err := clientSession.CallTool(t.Context(), params)
				callErr <- err
			}()

			first := <-progress
			if got := first.ProgressToken; got != "keepalive-token" {
				t.Fatalf("first progress token = %#v, want keepalive-token", got)
			}
			if got := first.Progress; got != 1 {
				t.Fatalf("first progress = %v, want 1", got)
			}
			second := <-progress
			if got := second.Progress; got != 2 {
				t.Fatalf("second progress = %v, want 2", got)
			}

			releaseHostedCall(t, release, callErr)
			synctest.Wait()
			select {
			case unexpected := <-progress:
				t.Fatalf("unexpected progress after completion: %#v", unexpected)
			default:
			}
		})
	})

	t.Run("Should send no progress without a client progress token", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			clientSession, progress, release := newProgressKeepaliveFixture(t)

			callErr := make(chan error, 1)
			go func() {
				_, err := clientSession.CallTool(t.Context(), &sdkmcp.CallToolParams{
					Name:      "compozy__hosted_echo",
					Arguments: map[string]any{"message": "hi"},
				})
				callErr <- err
			}()

			synctest.Wait()
			select {
			case unexpected := <-progress:
				t.Fatalf("unexpected progress without token: %#v", unexpected)
			default:
			}

			releaseHostedCall(t, release, callErr)
		})
	})
}
