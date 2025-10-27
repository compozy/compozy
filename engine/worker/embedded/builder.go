package embedded

import (
	"fmt"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/google/uuid"
	"go.temporal.io/server/common/cluster"
	"go.temporal.io/server/common/config"
	"go.temporal.io/server/common/log"
	"go.temporal.io/server/common/membership/static"
	"go.temporal.io/server/common/metrics"
	sqliteplugin "go.temporal.io/server/common/persistence/sql/sqlplugin/sqlite"
	"go.temporal.io/server/common/primitives"
)

const (
	sqliteStoreName = "sqlite-default"
	connectProtocol = "tcp"
)

func buildTemporalConfig(cfg *Config) (*config.Config, error) {
	metricsPort, err := calculateMetricsPort(cfg)
	if err != nil {
		return nil, err
	}

	sqlCfg := buildSQLiteSQLConfig(cfg)
	persistence := buildPersistenceConfig(sqlCfg)
	services := buildServiceConfig(cfg)

	temporalCfg := &config.Config{
		Global:              buildGlobalConfig(cfg, metricsPort),
		Persistence:         persistence,
		Log:                 buildLogConfig(cfg),
		ClusterMetadata:     buildClusterMetadata(cfg),
		DCRedirectionPolicy: config.DCRedirectionPolicy{Policy: "noop"},
		Services:            services,
		Archival:            buildArchivalConfig(),
		NamespaceDefaults:   buildNamespaceDefaultsConfig(),
		PublicClient: config.PublicClient{
			HostPort: fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort),
		},
	}

	return temporalCfg, nil
}

func calculateMetricsPort(cfg *Config) (int, error) {
	metricsPort := cfg.FrontendPort + 1000
	if metricsPort > maxPort {
		return 0, fmt.Errorf("metrics port %d exceeds maximum", metricsPort)
	}
	return metricsPort, nil
}

func buildPersistenceConfig(sqlCfg *config.SQL) config.Persistence {
	return config.Persistence{
		DefaultStore:     sqliteStoreName,
		VisibilityStore:  sqliteStoreName,
		NumHistoryShards: 1,
		DataStores: map[string]config.DataStore{
			sqliteStoreName: {SQL: sqlCfg},
		},
	}
}

func buildGlobalConfig(cfg *Config, metricsPort int) config.Global {
	return config.Global{
		Membership: config.Membership{
			MaxJoinDuration:  cfg.StartTimeout,
			BroadcastAddress: cfg.BindIP,
		},
		Metrics: &metrics.Config{
			Prometheus: &metrics.PrometheusConfig{
				ListenAddress: fmt.Sprintf("%s:%d", cfg.BindIP, metricsPort),
				HandlerPath:   "/metrics",
			},
		},
	}
}

func buildLogConfig(cfg *Config) log.Config {
	return log.Config{
		Stdout: true,
		Level:  cfg.LogLevel,
		Format: "console",
	}
}

func buildClusterMetadata(cfg *Config) *cluster.Config {
	return &cluster.Config{
		EnableGlobalNamespace:    false,
		FailoverVersionIncrement: 10,
		MasterClusterName:        cfg.ClusterName,
		CurrentClusterName:       cfg.ClusterName,
		ClusterInformation: map[string]cluster.ClusterInformation{
			cfg.ClusterName: {
				Enabled:                true,
				InitialFailoverVersion: 1,
				RPCAddress:             fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort),
				ClusterID:              uuid.NewString(),
			},
		},
	}
}

func buildArchivalConfig() config.Archival {
	return config.Archival{
		History:    config.HistoryArchival{State: "disabled"},
		Visibility: config.VisibilityArchival{State: "disabled"},
	}
}

func buildNamespaceDefaultsConfig() config.NamespaceDefaults {
	return config.NamespaceDefaults{
		Archival: config.ArchivalNamespaceDefaults{
			History:    config.HistoryArchivalNamespaceDefaults{State: "disabled"},
			Visibility: config.VisibilityArchivalNamespaceDefaults{State: "disabled"},
		},
	}
}

func buildSQLiteSQLConfig(cfg *Config) *config.SQL {
	attrs := core.CloneMap(buildSQLiteConnectAttrs(cfg))
	return &config.SQL{
		PluginName:        sqliteplugin.PluginName,
		DatabaseName:      cfg.DatabaseFile,
		ConnectAddr:       cfg.BindIP,
		ConnectProtocol:   connectProtocol,
		ConnectAttributes: attrs,
		MaxConns:          1,
		MaxIdleConns:      1,
		MaxConnLifetime:   time.Hour,
	}
}

func buildSQLiteConnectAttrs(cfg *Config) map[string]string {
	if cfg.DatabaseFile == ":memory:" {
		return map[string]string{
			"mode":  "memory",
			"cache": "shared",
		}
	}
	return map[string]string{
		"cache":        "private",
		"journal_mode": "wal",
		"synchronous":  "2",
		"setup":        "true",
	}
}

func buildServiceConfig(cfg *Config) map[string]config.Service {
	historyPort := cfg.FrontendPort + 1
	matchingPort := cfg.FrontendPort + 2
	workerPort := cfg.FrontendPort + 3
	return map[string]config.Service{
		"frontend": {RPC: config.RPC{GRPCPort: cfg.FrontendPort, BindOnIP: cfg.BindIP}},
		"history":  {RPC: config.RPC{GRPCPort: historyPort, BindOnIP: cfg.BindIP}},
		"matching": {RPC: config.RPC{GRPCPort: matchingPort, BindOnIP: cfg.BindIP}},
		"worker":   {RPC: config.RPC{GRPCPort: workerPort, BindOnIP: cfg.BindIP}},
	}
}

func buildStaticHosts(cfg *Config) map[primitives.ServiceName]static.Hosts {
	frontend := fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort)
	history := fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort+1)
	matching := fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort+2)
	worker := fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort+3)
	return map[primitives.ServiceName]static.Hosts{
		primitives.FrontendService: static.SingleLocalHost(frontend),
		primitives.HistoryService:  static.SingleLocalHost(history),
		primitives.MatchingService: static.SingleLocalHost(matching),
		primitives.WorkerService:   static.SingleLocalHost(worker),
	}
}
