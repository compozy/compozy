//go:build embedded_temporal

package temporal

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/sethvargo/go-retry"
	uiserver "github.com/temporalio/ui-server/v2/server"
	uiconfig "github.com/temporalio/ui-server/v2/server/config"
	uiserveroptions "github.com/temporalio/ui-server/v2/server/server_options"
	"go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/server/common/authorization"
	"go.temporal.io/server/common/cluster"
	srvconfig "go.temporal.io/server/common/config"
	"go.temporal.io/server/common/dynamicconfig"
	temporallog "go.temporal.io/server/common/log"
	"go.temporal.io/server/common/membership/static"
	sqliteplugin "go.temporal.io/server/common/persistence/sql/sqlplugin/sqlite"
	"go.temporal.io/server/common/primitives"
	sqliteschema "go.temporal.io/server/schema/sqlite"
	"go.temporal.io/server/temporal"
	"google.golang.org/protobuf/types/known/durationpb"
)

// Server wraps an embedded Temporal development server lifecycle.
type Server struct {
	srv temporal.Server
	ui  *uiserver.Server
	hp  string
	uip int
}

const (
	ensureNSInitialDelay = 100 * time.Millisecond
	ensureNSMaxDelay     = 1 * time.Second
	ensureNSMaxDuration  = 5 * time.Second
	historyPortOffset    = 1
	matchingPortOffset   = 2
	workerPortOffset     = 3
	uiPortOffset         = 1000
)

// StartEmbedded starts an embedded Temporal server for standalone mode using
// Temporal's programmatic server with a SQLite backend. It binds frontend to
// cfg.Temporal.HostPort and ensures the configured namespace exists.
func StartEmbedded(ctx context.Context, cfg *config.Config, dataDir string) (*Server, error) {
	log := logger.FromContext(ctx)
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	host, port, err := splitHostPort(cfg.Temporal.HostPort)
	if err != nil {
		return nil, fmt.Errorf("invalid temporal host_port: %w", err)
	}
	// Choose service ports via fixed offsets for dev cluster, ensuring ports are free.
	historyPort, matchingPort, workerPort, uiPort := derivePorts(port)
	frontendPort := ensureFreePort(host, port)
	historyPort = ensureFreePort(host, historyPort)
	matchingPort = ensureFreePort(host, matchingPort)
	workerPort = ensureFreePort(host, workerPort)
	uiPort = ensureFreePort(host, uiPort)
	frontendHP := net.JoinHostPort(host, fmt.Sprintf("%d", frontendPort))

	// Configure SQLite persistence (single file, execution + visibility)
	// Align with Temporal's LiteServer: both stores share one DB file.
	defaultDBPath := filepath.Join(dataDir, "temporal.db")

	conf := &srvconfig.Config{
		Persistence: srvconfig.Persistence{
			DefaultStore:     sqliteplugin.PluginName,
			VisibilityStore:  sqliteplugin.PluginName,
			NumHistoryShards: 1,
			DataStores: map[string]srvconfig.DataStore{
				sqliteplugin.PluginName: {
					SQL: &srvconfig.SQL{
						PluginName:      sqliteplugin.PluginName,
						DatabaseName:    defaultDBPath,
						ConnectAddr:     "localhost",
						ConnectProtocol: "tcp",
						ConnectAttributes: map[string]string{
							"cache":        "private",
							"journal_mode": "wal",
							"synchronous":  "2",
						},
						MaxConns:        1,
						MaxIdleConns:    1,
						MaxConnLifetime: time.Hour,
					},
				},
			},
		},
		ClusterMetadata: &cluster.Config{
			EnableGlobalNamespace:    false,
			FailoverVersionIncrement: 10,
			MasterClusterName:        "active",
			CurrentClusterName:       "active",
			ClusterInformation: map[string]cluster.ClusterInformation{
				"active": {
					Enabled:                true,
					InitialFailoverVersion: 1,
					RPCAddress:             frontendHP,
				},
			},
		},
		DCRedirectionPolicy: srvconfig.DCRedirectionPolicy{Policy: "noop"},
		Services: map[string]srvconfig.Service{
			"frontend": {RPC: srvconfig.RPC{GRPCPort: frontendPort, BindOnIP: host}},
			"history":  {RPC: srvconfig.RPC{GRPCPort: historyPort, BindOnIP: host}},
			"matching": {RPC: srvconfig.RPC{GRPCPort: matchingPort, BindOnIP: host}},
			"worker":   {RPC: srvconfig.RPC{GRPCPort: workerPort, BindOnIP: host}},
		},
		PublicClient: srvconfig.PublicClient{HostPort: frontendHP},
		Global: srvconfig.Global{
			Membership: srvconfig.Membership{BroadcastAddress: "127.0.0.1"},
		},
	}

	// Ensure schema exists only for a new DB file (idempotent)
	if _, statErr := os.Stat(defaultDBPath); os.IsNotExist(statErr) {
		if err := os.MkdirAll(filepath.Dir(defaultDBPath), 0o755); err != nil {
			return nil, fmt.Errorf("failed to create temporal data dir: %w", err)
		}
		if err := sqliteschema.SetupSchema(conf.Persistence.DataStores[sqliteplugin.PluginName].SQL); err != nil {
			return nil, fmt.Errorf("failed to setup sqlite schema: %w", err)
		}
	}
	nsName := cfg.Temporal.Namespace
	if nsName == "" {
		nsName = "default"
	}
	nsCfg, err := sqliteschema.NewNamespaceConfig("active", nsName, false, map[string]enums.IndexedValueType{})
	if err != nil {
		return nil, fmt.Errorf("failed to build namespace config: %w", err)
	}
	if err := sqliteschema.CreateNamespaces(conf.Persistence.DataStores[sqliteplugin.PluginName].SQL, nsCfg); err != nil {
		return nil, fmt.Errorf("failed to create temporal namespace: %w", err)
	}

	authorizer, err := authorization.GetAuthorizerFromConfig(&conf.Global.Authorization)
	if err != nil {
		return nil, fmt.Errorf("failed to init temporal authorizer: %w", err)
	}
	tl := temporallog.NewNoopLogger().With()
	claimMapper, err := authorization.GetClaimMapperFromConfig(&conf.Global.Authorization, tl)
	if err != nil {
		return nil, fmt.Errorf("failed to init claim mapper: %w", err)
	}
	dynConf := dynamicconfig.StaticClient{}

	srv, err := temporal.NewServer(
		temporal.WithConfig(conf),
		temporal.ForServices(temporal.DefaultServices),
		temporal.WithStaticHosts(map[primitives.ServiceName]static.Hosts{
			primitives.FrontendService: static.SingleLocalHost(frontendHP),
			primitives.HistoryService:  static.SingleLocalHost(net.JoinHostPort(host, fmt.Sprintf("%d", historyPort))),
			primitives.MatchingService: static.SingleLocalHost(net.JoinHostPort(host, fmt.Sprintf("%d", matchingPort))),
			primitives.WorkerService:   static.SingleLocalHost(net.JoinHostPort(host, fmt.Sprintf("%d", workerPort))),
		}),
		temporal.WithLogger(tl),
		temporal.WithAuthorizer(authorizer),
		temporal.WithClaimMapper(func(*srvconfig.Config) authorization.ClaimMapper { return claimMapper }),
		temporal.WithDynamicConfigClient(dynConf),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporal programmatic server: %w", err)
	}
	start := time.Now()
	if err := srv.Start(); err != nil {
		return nil, fmt.Errorf("failed to start temporal programmatic server: %w", err)
	}
	if err := ensureNamespace(ctx, frontendHP, nsName); err != nil {
		srv.Stop()
		return nil, err
	}
	ui := uiserver.NewServer(
		uiserveroptions.WithConfigProvider(
			&uiconfig.Config{
				TemporalGRPCAddress: frontendHP,
				Host:                host,
				Port:                uiPort,
				EnableUI:            true,
				CORS:                uiconfig.CORS{CookieInsecure: true},
				HideLogs:            true,
			},
		),
	)
	go func() {
		if err := ui.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.FromContext(ctx).Warn("UI server error", "error", err)
		}
	}()
	log.Info(
		"Embedded Temporal server started",
		"host_port", frontendHP,
		"frontend_port", frontendPort,
		"history_port", historyPort,
		"matching_port", matchingPort,
		"worker_port", workerPort,
		"ui", fmt.Sprintf("%s:%d", host, uiPort),
		"db", defaultDBPath,
		"duration", time.Since(start),
	)
	return &Server{srv: srv, ui: ui, hp: frontendHP, uip: uiPort}, nil
}

// HostPort returns the listening address of the embedded server.
func (s *Server) HostPort() string { return s.hp }

// UIPort returns the HTTP port for the Temporal UI when embedded; 0 if unknown.
func (s *Server) UIPort() int { return s.uip }

// Stop stops the embedded server.
func (s *Server) Stop() error {
	if s.ui != nil {
		s.ui.Stop()
	}
	if err := s.srv.Stop(); err != nil {
		return fmt.Errorf("failed to stop temporal server: %w", err)
	}
	return nil
}

func splitHostPort(hp string) (string, int, error) {
	host, ps, err := net.SplitHostPort(hp)
	if err != nil {
		return "", 0, fmt.Errorf("invalid host_port: %w", err)
	}
	if host == "" || host == "localhost" {
		host = "127.0.0.1"
	}
	p, err := strconv.Atoi(ps)
	if err != nil || p <= 0 || p > 65535 {
		return "", 0, fmt.Errorf("invalid port: %s", ps)
	}
	return host, p, nil
}

func derivePorts(frontend int) (int, int, int, int) {
	return frontend + historyPortOffset, frontend + matchingPortOffset, frontend + workerPortOffset, frontend + uiPortOffset
}

// ensureNamespace ensures the namespace exists with a short backoff to tolerate cold start.
func ensureNamespace(ctx context.Context, hostPort, ns string) error {
	log := logger.FromContext(ctx)
	if ns == "" {
		ns = "default"
	}
	backoff := retry.WithCappedDuration(ensureNSMaxDelay, retry.NewExponential(ensureNSInitialDelay))
	err := retry.Do(ctx, retry.WithMaxDuration(ensureNSMaxDuration, backoff), func(ctx context.Context) error {
		nsc, err := client.NewNamespaceClient(client.Options{HostPort: hostPort})
		if err != nil {
			return retry.RetryableError(err)
		}
		defer nsc.Close()
		if _, dErr := nsc.Describe(ctx, ns); dErr == nil {
			log.Debug("Temporal namespace exists", "namespace", ns)
			return nil
		}
		req := &workflowservice.RegisterNamespaceRequest{
			Namespace:                        ns,
			WorkflowExecutionRetentionPeriod: durationpb.New(24 * time.Hour),
			HistoryArchivalState:             enums.ARCHIVAL_STATE_DISABLED,
			VisibilityArchivalState:          enums.ARCHIVAL_STATE_DISABLED,
		}
		if rErr := nsc.Register(ctx, req); rErr != nil {
			if _, ok := rErr.(*serviceerror.NamespaceAlreadyExists); ok {
				log.Debug("Temporal namespace already existed", "namespace", ns)
				return nil
			}
			return retry.RetryableError(rErr)
		}
		log.Debug("Temporal namespace registered", "namespace", ns)
		return nil
	})
	if err != nil {
		return fmt.Errorf("temporal namespace ensure failed: %w", err)
	}
	return nil
}

func ptr[T any](v T) *T { return &v }

// ensureFreePort returns a usable TCP port on host, starting at start.
// If the requested port is busy, it probes a small range until it finds
// a free port to reduce gRPC bind failures during service startup.
func ensureFreePort(host string, start int) int {
	const maxProbe = 32
	p := start
	for i := 0; i < maxProbe; i++ {
		addr := net.JoinHostPort(host, strconv.Itoa(p))
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			_ = ln.Close()
			return p
		}
		p++
		if p > 65535 {
			p = 1024
		}
	}
	return start
}
