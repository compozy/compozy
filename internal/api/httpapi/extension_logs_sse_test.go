package httpapi

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

func TestExtensionLogsRouteAndSSEFollow(t *testing.T) {
	t.Run("Should serve redacted history and follow frames", testExtensionLogsRouteAndSSEFollow)
}

func testExtensionLogsRouteAndSSEFollow(t *testing.T) {
	// Not parallel: newTestRouter changes Gin's process-global mode.
	actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator",
		"workspace-a",
		taskpkg.OriginKindHTTP,
		"extensions.logs",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}
	entry := contract.ExtensionLogPayload{
		Sequence:       1,
		Timestamp:      time.Date(2026, 7, 28, 12, 30, 0, 0, time.UTC),
		Message:        "token=[REDACTED] safe=visible\n",
		GenerationHash: strings.Repeat("a", 64),
	}
	service := stubExtensionService{
		ExtensionLogsFn: func(
			_ context.Context,
			name string,
			after int64,
			gotActor taskpkg.ActorContext,
		) ([]contract.ExtensionLogPayload, error) {
			if name != "dev-extension" {
				t.Fatalf("extension name = %q, want dev-extension", name)
			}
			if gotActor.Scope.WorkspaceID != "workspace-a" || !gotActor.Scope.Operator {
				t.Fatalf("actor scope = %#v, want operator workspace A", gotActor.Scope)
			}
			if after >= entry.Sequence {
				return []contract.ExtensionLogPayload{}, nil
			}
			return []contract.ExtensionLogPayload{entry}, nil
		},
	}
	handlers := newTestHandlersWithSettingsAndExtensions(
		t,
		"127.0.0.1",
		nil,
		nil,
		service,
		newTestHomePaths(t),
	)
	handlers.TaskActorContextResolver = func(_ *gin.Context, _ string) (taskpkg.ActorContext, error) {
		return actor, nil
	}
	engine := newTestRouter(t, handlers)

	rest := performRequest(t, engine, http.MethodGet, "/api/extensions/dev-extension/logs", nil)
	if rest.Code != http.StatusOK {
		t.Fatalf("REST status = %d, want %d; body=%s", rest.Code, http.StatusOK, rest.Body.String())
	}
	for _, want := range []string{"[REDACTED]", "safe=visible", entry.Timestamp.Format(time.RFC3339)} {
		if !strings.Contains(rest.Body.String(), want) {
			t.Fatalf("REST body = %q, want %q", rest.Body.String(), want)
		}
	}
	if strings.Contains(rest.Body.String(), "provider-token") {
		t.Fatalf("REST body leaked raw token: %s", rest.Body.String())
	}

	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	requestCtx, cancel := context.WithCancel(t.Context())
	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodGet,
		server.URL+"/api/extensions/dev-extension/logs?follow=1",
		http.NoBody,
	)
	if err != nil {
		cancel()
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		cancel()
		t.Fatalf("client.Do() error = %v", err)
	}
	t.Cleanup(func() {
		cancel()
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Errorf("response.Body.Close() error = %v", closeErr)
		}
	})
	if response.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(response.Body)
		if readErr != nil {
			t.Fatalf("io.ReadAll(error response) error = %v", readErr)
		}
		t.Fatalf("SSE status = %d, want %d; body=%s", response.StatusCode, http.StatusOK, string(body))
	}
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}

	reader := bufio.NewReader(response.Body)
	var frame strings.Builder
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			t.Fatalf("ReadString(SSE) error = %v", readErr)
		}
		frame.WriteString(line)
		if line == "\n" {
			break
		}
	}
	cancel()
	for _, want := range []string{"event: extension_log", "id: 1", "[REDACTED]", "safe=visible"} {
		if !strings.Contains(frame.String(), want) {
			t.Fatalf("SSE frame = %q, want %q", frame.String(), want)
		}
	}
	if strings.Contains(frame.String(), "provider-token") {
		t.Fatalf("SSE frame leaked raw token: %s", frame.String())
	}
}
