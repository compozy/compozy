package mcp

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/server"
)

const (
	serveHTTPReadHeaderTimeout = 5 * time.Second
	serveHTTPReadTimeout       = 30 * time.Second
	serveHTTPWriteTimeout      = 0
	serveHTTPIdleTimeout       = 60 * time.Second
	serveHTTPShutdownTimeout   = 5 * time.Second
)

// ServeHTTP exposes the projected Host API over token-authenticated loopback streamable HTTP.
func ServeHTTP(
	ctx context.Context,
	invoker HostAPIInvoker,
	workspace string,
	listenAddress string,
	token string,
	logger *slog.Logger,
) (err error) {
	if err := validateLoopbackListenAddress(listenAddress); err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return errors.New("mcp: HTTP bearer token is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	serveSessionID := uuid.NewString()
	mcpServer, err := newHostAPIMCPServer(invoker, serveSessionID, workspace)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, closeHostAPISession(invoker, serveSessionID))
	}()
	transport := server.NewStreamableHTTPServer(
		mcpServer,
		server.WithStreamableHTTPLogger(logger),
	)
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "tcp", listenAddress)
	if err != nil {
		return fmt.Errorf("mcp: listen on %q: %w", listenAddress, err)
	}

	httpServer := &http.Server{
		Addr:              listenAddress,
		Handler:           bearerTokenHandler(token, transport),
		ReadHeaderTimeout: serveHTTPReadHeaderTimeout,
		ReadTimeout:       serveHTTPReadTimeout,
		WriteTimeout:      serveHTTPWriteTimeout,
		IdleTimeout:       serveHTTPIdleTimeout,
	}
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- httpServer.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("mcp: serve HTTP: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serveHTTPShutdownTimeout)
		defer cancel()
		transportErr := transport.Shutdown(shutdownCtx)
		httpErr := httpServer.Shutdown(shutdownCtx)
		serverErr := <-serveErr
		if errors.Is(serverErr, http.ErrServerClosed) {
			serverErr = nil
		}
		if joined := errors.Join(transportErr, httpErr, serverErr); joined != nil {
			return fmt.Errorf("mcp: shutdown HTTP: %w", joined)
		}
		return nil
	}
}

func validateLoopbackListenAddress(address string) error {
	host, _, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil {
		return fmt.Errorf("mcp: listen address must be host:port: %w", err)
	}
	host = strings.Trim(strings.TrimSpace(host), "[]")
	if strings.EqualFold(host, "localhost") {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("mcp: listen host %q must be loopback", host)
	}
	return nil
}

func bearerTokenHandler(token string, next http.Handler) http.Handler {
	expected := []byte(token)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		authorization := request.Header.Get("Authorization")
		if !strings.HasPrefix(authorization, "Bearer ") {
			writer.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		provided := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
		if len(provided) != len(expected) || subtle.ConstantTimeCompare([]byte(provided), expected) != 1 {
			writer.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(writer, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(writer, request)
	})
}
