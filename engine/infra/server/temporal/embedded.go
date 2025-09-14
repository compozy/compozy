//go:build temporalite

package temporal

import (
    "context"
    "fmt"
    "net"
    "path/filepath"
    "strings"
    "time"

    "github.com/compozy/compozy/pkg/config"
    "github.com/compozy/compozy/pkg/logger"
    temporalite "github.com/temporalio/temporalite"
    "go.temporal.io/api/enums/v1"
    "go.temporal.io/api/serviceerror"
    "go.temporal.io/api/workflowservice/v1"
    "go.temporal.io/sdk/client"
    "google.golang.org/protobuf/types/known/durationpb"
    "github.com/sethvargo/go-retry"
)

// Server wraps an embedded Temporal development server lifecycle.
type Server struct {
    srv *temporalite.Server
    hp  string
}

const (
    ensureNSInitialDelay = 100 * time.Millisecond
    ensureNSMaxDelay     = 1 * time.Second
    ensureNSMaxDuration  = 5 * time.Second
)

// StartEmbedded starts an embedded Temporal server for standalone mode.
// It binds to the HostPort from cfg.Temporal.HostPort and ensures the configured namespace exists.
func StartEmbedded(ctx context.Context, cfg *config.Config, dataDir string) (*Server, error) {
    log := logger.FromContext(ctx)
    if cfg == nil {
        return nil, fmt.Errorf("nil config")
    }
    host, port, err := splitHostPort(cfg.Temporal.HostPort)
    if err != nil {
        return nil, fmt.Errorf("invalid temporal host_port: %w", err)
    }
    dbFile := filepath.Join(dataDir, "temporal_dev.sqlite")
    opts := []temporalite.ServerOption{
        temporalite.WithFrontendIP(host),
        temporalite.WithFrontendPort(port),
        temporalite.WithDatabaseFilePath(dbFile),
        temporalite.WithNamespaces(cfg.Temporal.Namespace),
    }
    srv, err := temporalite.NewServer(opts...)
    if err != nil {
        return nil, fmt.Errorf("failed to create embedded temporal server: %w", err)
    }
    start := time.Now()
    if err := srv.Start(); err != nil {
        return nil, fmt.Errorf("failed to start embedded temporal server: %w", err)
    }
    hp := net.JoinHostPort(host, fmt.Sprintf("%d", port))
    log.Info("Embedded Temporal server started", "host_port", hp, "db", dbFile, "duration", time.Since(start))
    if err := ensureNamespace(ctx, hp, cfg.Temporal.Namespace); err != nil {
        _ = srv.Stop()
        return nil, err
    }
    return &Server{srv: srv, hp: hp}, nil
}

// HostPort returns the listening address of the embedded server.
func (s *Server) HostPort() string { return s.hp }

// Stop stops the embedded server.
func (s *Server) Stop() error { s.srv.Stop(); return nil }

func splitHostPort(hp string) (string, int, error) {
    if !strings.Contains(hp, ":") {
        return "", 0, fmt.Errorf("missing port in host_port")
    }
    host, p, ok := strings.Cut(hp, ":")
    if !ok {
        return "", 0, fmt.Errorf("invalid host_port format")
    }
    if host == "" {
        host = "127.0.0.1"
    }
    port, err := parsePort(p)
    if err != nil {
        return "", 0, err
    }
    return host, port, nil
}

func parsePort(s string) (int, error) {
    var n int
    for i := 0; i < len(s); i++ {
        c := s[i]
        if c < '0' || c > '9' {
            return 0, fmt.Errorf("invalid port: %s", s)
        }
        n = n*10 + int(c-'0')
        if n > 65535 {
            return 0, fmt.Errorf("port out of range: %d", n)
        }
    }
    if n <= 0 {
        return 0, fmt.Errorf("port must be > 0")
    }
    return n, nil
}

// ensureNamespace ensures the namespace exists with a short backoff to tolerate cold start.
func ensureNamespace(ctx context.Context, hostPort, ns string) error {
    log := logger.FromContext(ctx)
    if ns == "" { ns = "default" }
    backoff := retry.WithCappedDuration(ensureNSMaxDelay, retry.NewExponential(ensureNSInitialDelay))
    err := retry.Do(ctx, retry.WithMaxDuration(ensureNSMaxDuration, backoff), func(ctx context.Context) error {
        nsc, err := client.NewNamespaceClient(client.Options{HostPort: hostPort, Logger: log})
        if err != nil { return retry.RetryableError(err) }
        defer nsc.Close()
        if _, dErr := nsc.Describe(ctx, ns); dErr == nil {
            return nil
        }
        req := &workflowservice.RegisterNamespaceRequest{Namespace: ns, WorkflowExecutionRetentionPeriod: durationpb.New(24 * time.Hour), HistoryArchivalState: enums.ARCHIVAL_STATE_DISABLED, VisibilityArchivalState: enums.ARCHIVAL_STATE_DISABLED}
        if rErr := nsc.Register(ctx, req); rErr != nil {
            if _, ok := rErr.(*serviceerror.NamespaceAlreadyExists); ok { return nil }
            return retry.RetryableError(rErr)
        }
        return nil
    })
    if err != nil { return fmt.Errorf("temporal namespace ensure failed: %w", err) }
    return nil
}

func ptr[T any](v T) *T { return &v }
