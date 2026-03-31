package run

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestSignalServerJobDoneReturnsEvent(t *testing.T) {
	t.Parallel()

	eventCh := make(chan SignalEvent, 1)
	server := NewSignalServer(9877, eventCh, []string{"batch-001"})

	resp := executeSignalRequest(
		t,
		server,
		newSignalHTTPRequest(t, http.MethodPost, "/job/done", strings.NewReader(`{"id":"batch-001"}`)),
	)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status code = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	event := waitForSignalEvent(t, eventCh)
	if event.Type != SignalEventTypeDone {
		t.Fatalf("event type = %q, want %q", event.Type, SignalEventTypeDone)
	}
	if event.JobID != "batch-001" {
		t.Fatalf("job id = %q, want %q", event.JobID, "batch-001")
	}
}

func TestSignalServerJobDoneRejectsUnknownJob(t *testing.T) {
	t.Parallel()

	server := NewSignalServer(9877, make(chan SignalEvent, 1), []string{"batch-001"})

	resp := executeSignalRequest(
		t,
		server,
		newSignalHTTPRequest(t, http.MethodPost, "/job/done", strings.NewReader(`{"id":"batch-999"}`)),
	)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status code = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestSignalServerJobDoneRejectsMalformedJSON(t *testing.T) {
	t.Parallel()

	server := NewSignalServer(9877, make(chan SignalEvent, 1), []string{"batch-001"})

	resp := executeSignalRequest(
		t,
		server,
		newSignalHTTPRequest(t, http.MethodPost, "/job/done", strings.NewReader(`{"id":`)),
	)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSignalServerJobStatusReturnsEvent(t *testing.T) {
	t.Parallel()

	eventCh := make(chan SignalEvent, 1)
	server := NewSignalServer(9877, eventCh, []string{"batch-001"})

	resp := executeSignalRequest(
		t,
		server,
		newSignalHTTPRequest(
			t,
			http.MethodPost,
			"/job/status",
			strings.NewReader(`{"id":"batch-001","status":"working on tests"}`),
		),
	)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status code = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	event := waitForSignalEvent(t, eventCh)
	if event.Type != SignalEventTypeStatus {
		t.Fatalf("event type = %q, want %q", event.Type, SignalEventTypeStatus)
	}
	if event.JobID != "batch-001" {
		t.Fatalf("job id = %q, want %q", event.JobID, "batch-001")
	}
	if got := event.Data["status"]; got != "working on tests" {
		t.Fatalf("status = %q, want %q", got, "working on tests")
	}
}

func TestSignalServerJobStatusRejectsMissingStatus(t *testing.T) {
	t.Parallel()

	server := NewSignalServer(9877, make(chan SignalEvent, 1), []string{"batch-001"})

	resp := executeSignalRequest(
		t,
		server,
		newSignalHTTPRequest(t, http.MethodPost, "/job/status", strings.NewReader(`{"id":"batch-001"}`)),
	)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestSignalServerHealthEndpoint(t *testing.T) {
	t.Parallel()

	server := NewSignalServer(9877, make(chan SignalEvent, 1), nil)

	resp := executeSignalRequest(t, server, newSignalHTTPRequest(t, http.MethodGet, "/health", http.NoBody))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status code = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read health body: %v", err)
	}
	if !strings.Contains(string(body), `"status":"ok"`) {
		t.Fatalf("health response = %q, want JSON ok payload", string(body))
	}
}

func TestSignalServerQueueFullDoesNotBlockHandler(t *testing.T) {
	t.Parallel()

	eventCh := make(chan SignalEvent, 1)
	eventCh <- SignalEvent{Type: SignalEventTypeDone, JobID: "already-buffered"}

	server := NewSignalServer(9877, eventCh, []string{"batch-001"})
	req := newSignalHTTPRequest(t, http.MethodPost, "/job/done", strings.NewReader(`{"id":"batch-001"}`))

	start := time.Now()
	resp := executeSignalRequest(t, server, req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status code = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("handler blocked for %v with a full queue", elapsed)
	}
}

func TestSignalServerGracefulShutdownOnContextCancel(t *testing.T) {
	t.Parallel()

	port := freeSignalServerPort(t)
	server := NewSignalServer(port, make(chan SignalEvent, 1), []string{"batch-001"})

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(ctx)
	}()

	waitForSignalServerHealth(t, server.Port())
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for signal server shutdown")
	}
}

func TestSignalServerShutdownStopsRunningServer(t *testing.T) {
	t.Parallel()

	port := freeSignalServerPort(t)
	server := NewSignalServer(port, make(chan SignalEvent, 1), []string{"batch-001"})

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Start(context.Background())
	}()

	waitForSignalServerHealth(t, port)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Start() to return after shutdown")
	}
}

func TestSignalServerShutdownBeforeStartIsNoop(t *testing.T) {
	t.Parallel()

	server := NewSignalServer(9877, make(chan SignalEvent, 1), nil)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		t.Fatalf("Shutdown() error = %v", err)
	}
}

func TestSignalServerBindsSpecifiedPortAndRejectsPortInUse(t *testing.T) {
	t.Parallel()

	t.Run("binds specified port", func(t *testing.T) {
		t.Parallel()

		port := freeSignalServerPort(t)
		server := NewSignalServer(port, make(chan SignalEvent, 1), []string{"batch-001"})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- server.Start(ctx)
		}()

		waitForSignalServerHealth(t, port)

		if got := server.Port(); got != port {
			t.Fatalf("port = %d, want %d", got, port)
		}

		cancel()
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("Start() error = %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for signal server shutdown")
		}
	})

	t.Run("rejects repeated start", func(t *testing.T) {
		t.Parallel()

		port := freeSignalServerPort(t)
		server := NewSignalServer(port, make(chan SignalEvent, 1), []string{"batch-001"})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- server.Start(ctx)
		}()

		waitForSignalServerHealth(t, port)

		err := server.Start(context.Background())
		if err == nil {
			t.Fatal("second Start() unexpectedly succeeded")
		}

		cancel()
		select {
		case startErr := <-errCh:
			if startErr != nil {
				t.Fatalf("Start() error = %v", startErr)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timed out waiting for running server to stop")
		}
	})

	t.Run("rejects port in use", func(t *testing.T) {
		t.Parallel()

		var listenConfig net.ListenConfig
		listener, err := listenConfig.Listen(context.Background(), "tcp", signalServerAddress(0))
		if err != nil {
			t.Fatalf("listen for occupied port: %v", err)
		}
		defer listener.Close()

		port := signalServerPort(listener.Addr(), 0)
		server := NewSignalServer(port, make(chan SignalEvent, 1), []string{"batch-001"})

		err = server.Start(context.Background())
		if err == nil {
			t.Fatal("Start() unexpectedly succeeded while port was already in use")
		}
	})
}

func executeSignalRequest(t *testing.T, server *SignalServer, req *http.Request) *http.Response {
	t.Helper()

	resp, err := server.app.Test(req, fiber.TestConfig{Timeout: time.Second})
	if err != nil {
		t.Fatalf("app.Test() error = %v", err)
	}
	t.Cleanup(func() {
		_ = resp.Body.Close()
	})
	return resp
}

func newSignalHTTPRequest(t *testing.T, method string, target string, body io.Reader) *http.Request {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), method, target, body)
	return req
}

func waitForSignalEvent(t *testing.T, eventCh <-chan SignalEvent) SignalEvent {
	t.Helper()

	select {
	case event := <-eventCh:
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for signal event")
		return SignalEvent{}
	}
}

func TestSignalServerPortFallsBackForNonTCPAddress(t *testing.T) {
	t.Parallel()

	const fallback = 9877

	if got := signalServerPort(staticAddr("socket"), fallback); got != fallback {
		t.Fatalf("signalServerPort() = %d, want %d", got, fallback)
	}
}

func waitForSignalServerHealth(t *testing.T, port int) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	client := &http.Client{Timeout: 250 * time.Millisecond}
	url := "http://" + signalServerAddress(port) + "/health"
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, http.NoBody)
		if err != nil {
			t.Fatalf("build health request: %v", err)
		}
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		<-ticker.C
	}

	t.Fatalf("signal server at %s never became healthy", url)
}

func freeSignalServerPort(t *testing.T) int {
	t.Helper()

	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(context.Background(), "tcp", signalServerAddress(0))
	if err != nil {
		t.Fatalf("reserve local port: %v", err)
	}
	defer listener.Close()

	return signalServerPort(listener.Addr(), 0)
}

type staticAddr string

func (a staticAddr) Network() string { return string(a) }

func (a staticAddr) String() string { return string(a) }
