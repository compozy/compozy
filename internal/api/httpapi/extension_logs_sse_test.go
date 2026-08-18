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
	t.Run("Should publish an empty atomic reset for a replaced ring", testExtensionLogsEmptyReset)
	t.Run("Should publish an atomic reset when the followed ring changes", testExtensionLogsLiveEpochReset)
	t.Run("Should close a followed stream when the transport shuts down", testExtensionLogsTransportShutdown)
}

func testExtensionLogsTransportShutdown(t *testing.T) {
	actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator",
		"workspace-a",
		taskpkg.OriginKindHTTP,
		"extensions.logs",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}
	service := stubExtensionService{
		ExtensionLogsFn: func(
			context.Context,
			string,
			int64,
			string,
			taskpkg.ActorContext,
		) (contract.ExtensionLogsResponse, error) {
			return contract.ExtensionLogsResponse{StreamEpoch: "epoch-current"}, nil
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
	streamDone := make(chan struct{})
	handlers.SetStreamDone(streamDone)
	server := httptest.NewServer(newTestRouter(t, handlers))
	t.Cleanup(server.Close)
	request, err := http.NewRequestWithContext(
		t.Context(),
		http.MethodGet,
		server.URL+"/api/extensions/dev-extension/logs?follow=1",
		http.NoBody,
	)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := response.Body.Close(); closeErr != nil {
			t.Errorf("response.Body.Close() error = %v", closeErr)
		}
	})
	reader := bufio.NewReader(response.Body)
	ready := readExtensionSSEFrame(t, reader)
	if !strings.HasPrefix(ready, ": extension log stream ready") {
		t.Fatalf("SSE handshake frame = %q, want leading stream-ready comment", ready)
	}

	readDone := make(chan error, 1)
	go func() {
		_, readErr := io.Copy(io.Discard, reader)
		readDone <- readErr
	}()
	close(streamDone)
	select {
	case readErr := <-readDone:
		if readErr != nil {
			t.Fatalf("read stream after transport shutdown error = %v", readErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("extension log stream remained open after transport shutdown")
	}
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
		StreamEpoch:    "epoch-current",
	}
	service := stubExtensionService{
		ExtensionLogsFn: func(
			_ context.Context,
			name string,
			after int64,
			streamEpoch string,
			gotActor taskpkg.ActorContext,
		) (contract.ExtensionLogsResponse, error) {
			if name != "dev-extension" {
				t.Fatalf("extension name = %q, want dev-extension", name)
			}
			if gotActor.Scope.WorkspaceID != "workspace-a" || !gotActor.Scope.Operator {
				t.Fatalf("actor scope = %#v, want operator workspace A", gotActor.Scope)
			}
			if streamEpoch == entry.StreamEpoch && after >= entry.Sequence {
				return contract.ExtensionLogsResponse{
					Logs: []contract.ExtensionLogPayload{}, StreamEpoch: entry.StreamEpoch,
				}, nil
			}
			return contract.ExtensionLogsResponse{
				Logs: []contract.ExtensionLogPayload{entry}, StreamEpoch: entry.StreamEpoch,
			}, nil
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
	for _, want := range []string{
		"[REDACTED]", "safe=visible", entry.Timestamp.Format(time.RFC3339), entry.StreamEpoch,
	} {
		if !strings.Contains(rest.Body.String(), want) {
			t.Fatalf("REST body = %q, want %q", rest.Body.String(), want)
		}
	}
	if strings.Contains(rest.Body.String(), "provider-token") {
		t.Fatalf("REST body leaked raw token: %s", rest.Body.String())
	}
	invalidCursor := performRequest(
		t, engine, http.MethodGet, "/api/extensions/dev-extension/logs?after=1", nil,
	)
	if invalidCursor.Code != http.StatusBadRequest {
		t.Fatalf(
			"cursor without stream_epoch status = %d, want %d; body=%s",
			invalidCursor.Code,
			http.StatusBadRequest,
			invalidCursor.Body.String(),
		)
	}

	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	requestCtx, cancel := context.WithCancel(t.Context())
	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodGet,
		server.URL+"/api/extensions/dev-extension/logs?follow=1&after=9&stream_epoch=epoch-old",
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
	ready := readExtensionSSEFrame(t, reader)
	if !strings.HasPrefix(ready, ": extension log stream ready") {
		t.Fatalf("SSE handshake frame = %q, want leading stream-ready comment", ready)
	}
	frame := readExtensionSSEFrame(t, reader)
	cancel()
	for _, want := range []string{
		"event: extension_log_reset", "\"stream_epoch\":\"epoch-current\"", "[REDACTED]", "safe=visible",
	} {
		if !strings.Contains(frame, want) {
			t.Fatalf("SSE frame = %q, want %q", frame, want)
		}
	}
	if strings.Contains(frame, "provider-token") {
		t.Fatalf("SSE frame leaked raw token: %s", frame)
	}
}

func testExtensionLogsEmptyReset(t *testing.T) {
	actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator",
		"workspace-a",
		taskpkg.OriginKindHTTP,
		"extensions.logs",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}
	service := stubExtensionService{
		ExtensionLogsFn: func(
			_ context.Context,
			_ string,
			_ int64,
			_ string,
			_ taskpkg.ActorContext,
		) (contract.ExtensionLogsResponse, error) {
			return contract.ExtensionLogsResponse{
				Logs: []contract.ExtensionLogPayload{}, StreamEpoch: "epoch-empty",
			}, nil
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
	server := httptest.NewServer(newTestRouter(t, handlers))
	t.Cleanup(server.Close)
	requestCtx, cancel := context.WithCancel(t.Context())
	request, err := http.NewRequestWithContext(
		requestCtx,
		http.MethodGet,
		server.URL+"/api/extensions/dev-extension/logs?follow=1&after=7&stream_epoch=epoch-old",
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
	reader := bufio.NewReader(response.Body)
	ready := readExtensionSSEFrame(t, reader)
	if !strings.HasPrefix(ready, ": extension log stream ready") {
		t.Fatalf("SSE handshake frame = %q, want leading stream-ready comment", ready)
	}
	frame := readExtensionSSEFrame(t, reader)
	for _, want := range []string{
		"event: extension_log_reset", `"stream_epoch":"epoch-empty"`, `"logs":[]`,
	} {
		if !strings.Contains(frame, want) {
			t.Fatalf("SSE frame = %q, want %q", frame, want)
		}
	}
}

func testExtensionLogsLiveEpochReset(t *testing.T) {
	actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		"operator",
		"workspace-a",
		taskpkg.OriginKindHTTP,
		"extensions.logs",
	)
	if err != nil {
		t.Fatalf("DeriveHumanActorContextForWorkspace() error = %v", err)
	}
	calls := 0
	service := stubExtensionService{
		ExtensionLogsFn: func(
			_ context.Context,
			_ string,
			_ int64,
			_ string,
			_ taskpkg.ActorContext,
		) (contract.ExtensionLogsResponse, error) {
			calls++
			switch calls {
			case 1:
				return contract.ExtensionLogsResponse{
					Logs: []contract.ExtensionLogPayload{}, StreamEpoch: "epoch-a",
				}, nil
			case 2:
				return contract.ExtensionLogsResponse{
					Logs: []contract.ExtensionLogPayload{}, StreamEpoch: "epoch-b",
				}, nil
			}
			return contract.ExtensionLogsResponse{
				Logs: []contract.ExtensionLogPayload{{
					Sequence:    1,
					Timestamp:   time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC),
					Message:     "new ring",
					StreamEpoch: "epoch-b",
				}},
				StreamEpoch: "epoch-b",
			}, nil
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
	server := httptest.NewServer(newTestRouter(t, handlers))
	t.Cleanup(server.Close)
	requestCtx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
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
	reader := bufio.NewReader(response.Body)
	ready := readExtensionSSEFrame(t, reader)
	if !strings.HasPrefix(ready, ": extension log stream ready") {
		t.Fatalf("SSE handshake frame = %q, want leading stream-ready comment", ready)
	}
	resetFrame := readExtensionSSEFrame(t, reader)
	for _, want := range []string{
		"event: extension_log_reset", `"stream_epoch":"epoch-b"`, `"logs":[]`,
	} {
		if !strings.Contains(resetFrame, want) {
			t.Fatalf("SSE reset frame = %q, want %q", resetFrame, want)
		}
	}
	deltaFrame := readExtensionSSEFrame(t, reader)
	for _, want := range []string{
		"event: extension_log", `"stream_epoch":"epoch-b"`, `"message":"new ring"`,
	} {
		if !strings.Contains(deltaFrame, want) {
			t.Fatalf("SSE delta frame = %q, want %q", deltaFrame, want)
		}
	}
}

func readExtensionSSEFrame(t *testing.T, reader *bufio.Reader) string {
	t.Helper()
	var frame strings.Builder
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("ReadString(SSE) error = %v", err)
		}
		frame.WriteString(line)
		if line == "\n" {
			return frame.String()
		}
	}
}
