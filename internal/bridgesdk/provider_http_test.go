package bridgesdk

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"sync"
	"testing"
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
	})

	t.Run("Should release the listener when lifecycle ownership rejects startup", func(t *testing.T) {
		t.Parallel()

		var boundAddress string
		var server *ProviderHTTPServer
		var err error
		server, err = NewProviderHTTPServer(ProviderHTTPConfig{
			Handler: http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}),
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
