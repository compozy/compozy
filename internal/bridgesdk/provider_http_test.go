package bridgesdk

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestProviderHTTPServerOwnsListenerLifecycle(t *testing.T) {
	t.Parallel()

	t.Run("Should serve on the bound address and release it on shutdown", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		server, err := NewProviderHTTPServer(ProviderHTTPConfig{
			Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusNoContent)
			}),
			RoutesReady: closedProviderRoutes(),
			Go: func(run func()) bool {
				wg.Go(run)
				return true
			},
		})
		if err != nil {
			t.Fatalf("NewProviderHTTPServer() error = %v", err)
		}
		if err := server.Start("127.0.0.1:0"); err != nil {
			t.Fatalf("Start() error = %v", err)
		}

		request, err := http.NewRequestWithContext(
			t.Context(),
			http.MethodGet,
			"http://"+server.Address(),
			http.NoBody,
		)
		if err != nil {
			t.Fatalf("NewRequestWithContext() error = %v", err)
		}
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatalf("Do() error = %v", err)
		}
		if _, err := io.Copy(io.Discard, response.Body); err != nil {
			t.Fatalf("Copy(response body) error = %v", err)
		}
		if err := response.Body.Close(); err != nil {
			t.Fatalf("Close(response body) error = %v", err)
		}
		if response.StatusCode != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", response.StatusCode, http.StatusNoContent)
		}
		if err := server.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		wg.Wait()
		if server.Address() != "" {
			t.Fatalf("Address() = %q after shutdown", server.Address())
		}
	})

	t.Run("Should let the HTTP server own an immediate listener shutdown", func(t *testing.T) {
		t.Parallel()

		var wg sync.WaitGroup
		server, err := NewProviderHTTPServer(ProviderHTTPConfig{
			Handler: http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(http.StatusNoContent)
			}),
			RoutesReady: closedProviderRoutes(),
			Go: func(run func()) bool {
				wg.Go(run)
				return true
			},
		})
		if err != nil {
			t.Fatalf("NewProviderHTTPServer() error = %v", err)
		}
		if err := server.Start("127.0.0.1:0"); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		if err := server.Start("127.0.0.1:0"); err != nil {
			t.Fatalf("Start(same address) error = %v", err)
		}
		if err := server.Start("127.0.0.1:1"); err == nil {
			t.Fatal("Start(different address) error = nil")
		}
		if err := server.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		if err := server.Shutdown(context.Background()); err != nil {
			t.Fatalf("Shutdown(second) error = %v", err)
		}
		wg.Wait()
	})

	t.Run("Should interrupt a request waiting for provider routes during shutdown", func(t *testing.T) {
		t.Parallel()

		routesReady := make(chan struct{})
		server, err := NewProviderHTTPServer(ProviderHTTPConfig{
			Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				t.Fatal("provider handler ran before routes were ready")
			}),
			RoutesReady: routesReady,
			Go:          func(func()) bool { return true },
		})
		if err != nil {
			t.Fatalf("NewProviderHTTPServer() error = %v", err)
		}
		shutdown := make(chan struct{})
		server.shutdown = shutdown
		response := httptest.NewRecorder()
		request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", http.NoBody)
		handlerDone := make(chan struct{})
		go func() {
			server.serveWhenRoutesReady(shutdown, response, request)
			close(handlerDone)
		}()
		select {
		case <-handlerDone:
			t.Fatal("request stopped waiting before routes or shutdown were ready")
		case <-time.After(10 * time.Millisecond):
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			t.Fatalf("Shutdown() error = %v", err)
		}
		select {
		case <-handlerDone:
		case <-time.After(time.Second):
			t.Fatal("request remained blocked after shutdown")
		}
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
		}
		if body := response.Body.String(); body != "provider routes are not ready\n" {
			t.Fatalf("body = %q, want %q", body, "provider routes are not ready\n")
		}
	})

	t.Run("Should reject incomplete server configuration", func(t *testing.T) {
		t.Parallel()

		if _, err := NewProviderHTTPServer(ProviderHTTPConfig{}); err == nil {
			t.Fatal("NewProviderHTTPServer(empty) error = nil")
		}
		if _, err := NewProviderHTTPServer(ProviderHTTPConfig{
			Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
		}); err == nil {
			t.Fatal("NewProviderHTTPServer(without Go) error = nil")
		}
		if _, err := NewProviderHTTPServer(ProviderHTTPConfig{
			Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
			Go:      func(func()) bool { return true },
		}); err == nil || err.Error() != "bridgesdk: provider http routes readiness is required" {
			t.Fatalf("NewProviderHTTPServer(without routes readiness) error = %v", err)
		}
	})

	t.Run("Should release the listener when lifecycle ownership rejects startup", func(t *testing.T) {
		t.Parallel()

		var boundAddress string
		var server *ProviderHTTPServer
		var err error
		server, err = NewProviderHTTPServer(ProviderHTTPConfig{
			Handler:     http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
			RoutesReady: closedProviderRoutes(),
			Go: func(func()) bool {
				boundAddress = server.Address()
				return false
			},
		})
		if err != nil {
			t.Fatalf("NewProviderHTTPServer() error = %v", err)
		}
		if err := server.Start("127.0.0.1:0"); !errors.Is(err, ErrProviderStopped) {
			t.Fatalf("Start() error = %v, want ErrProviderStopped", err)
		}
		var listenConfig net.ListenConfig
		listener, err := listenConfig.Listen(t.Context(), "tcp", boundAddress)
		if err != nil {
			t.Fatalf("Listen(%q) after rejected startup error = %v", boundAddress, err)
		}
		if err := listener.Close(); err != nil {
			t.Fatalf("Close(rebound listener) error = %v", err)
		}
	})
}

func closedProviderRoutes() <-chan struct{} {
	ready := make(chan struct{})
	close(ready)
	return ready
}
