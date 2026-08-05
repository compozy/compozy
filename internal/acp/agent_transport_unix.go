//go:build !windows

package acp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultManagedTransportReadHeaderTimeout = 5 * time.Second

type managedAgentTransport struct {
	listener  net.Listener
	server    *http.Server
	handler   *agentSkillTransportHandler
	socketDir string

	mu       sync.Mutex
	closeErr error
	closed   bool
}

func startManagedAgentTransport(
	ctx context.Context,
	daemonSocket string,
	sessionID string,
	agentName string,
	logger *slog.Logger,
) (*managedAgentTransport, string, error) {
	if strings.TrimSpace(daemonSocket) == "" || strings.TrimSpace(sessionID) == "" ||
		strings.TrimSpace(agentName) == "" {
		return nil, "", nil
	}
	socketDir, err := os.MkdirTemp("", "cz-agent-")
	if err != nil {
		return nil, "", fmt.Errorf("acp: create managed agent transport directory: %w", err)
	}
	socketPath := filepath.Join(socketDir, "transport.sock")
	var listenConfig net.ListenConfig
	listener, err := listenConfig.Listen(ctx, "unix", socketPath)
	if err != nil {
		return nil, "", errors.Join(
			fmt.Errorf("acp: listen on managed agent transport: %w", err),
			os.RemoveAll(socketDir),
		)
	}
	handler := newAgentSkillTransportHandler(daemonSocket, sessionID, agentName)
	server := &http.Server{Handler: handler, ReadHeaderTimeout: defaultManagedTransportReadHeaderTimeout}
	transport := &managedAgentTransport{
		listener:  listener,
		server:    server,
		handler:   handler,
		socketDir: socketDir,
	}
	go serveManagedAgentTransport(server, listener, logger)
	return transport, socketPath, nil
}

func closeManagedAgentTransport(transport *managedAgentTransport) error {
	if transport == nil {
		return nil
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if transport.closed {
		return transport.closeErr
	}
	transport.closed = true
	if transport.handler != nil {
		transport.handler.Close()
	}
	transport.closeErr = joinNonClosedErrors(
		closeHTTPServer(transport.server),
		closeListener(transport.listener),
		removeTransportDirectory(transport.socketDir),
	)
	return transport.closeErr
}

func serveManagedAgentTransport(server *http.Server, listener net.Listener, logger *slog.Logger) {
	err := server.Serve(listener)
	if err == nil || errors.Is(err, http.ErrServerClosed) || errors.Is(err, net.ErrClosed) {
		return
	}
	if logger != nil {
		logger.Warn("acp: managed agent transport stopped", "error", err)
	}
}

func closeHTTPServer(server *http.Server) error {
	if server == nil {
		return nil
	}
	return server.Close()
}

func closeListener(listener net.Listener) error {
	if listener == nil {
		return nil
	}
	return listener.Close()
}

func removeTransportDirectory(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	return os.RemoveAll(path)
}

func joinNonClosedErrors(errs ...error) error {
	filtered := make([]error, 0, len(errs))
	for _, err := range errs {
		if err == nil || errors.Is(err, net.ErrClosed) || errors.Is(err, os.ErrClosed) ||
			errors.Is(err, http.ErrServerClosed) {
			continue
		}
		filtered = append(filtered, err)
	}
	return errors.Join(filtered...)
}
