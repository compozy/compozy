package embedded

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/server/common/cluster"
	"go.temporal.io/server/common/primitives"
)

func TestBuildTemporalConfig(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		DatabaseFile: ":memory:",
		FrontendPort: 7233,
		BindIP:       "127.0.0.1",
		Namespace:    "default",
		ClusterName:  "cluster",
		EnableUI:     true,
		UIPort:       8233,
		LogLevel:     "warn",
		StartTimeout: 45 * time.Second,
	}

	temporalCfg, err := buildTemporalConfig(cfg)
	require.NoError(t, err)

	require.NotNil(t, temporalCfg.Persistence.DataStores[sqliteStoreName].SQL)
	sqlCfg := temporalCfg.Persistence.DataStores[sqliteStoreName].SQL
	assert.Equal(t, cfg.DatabaseFile, sqlCfg.DatabaseName)
	assert.Equal(t, cfg.BindIP, sqlCfg.ConnectAddr)
	assert.Equal(t, connectProtocol, sqlCfg.ConnectProtocol)
	assert.Equal(t, "sqlite", sqlCfg.PluginName)

	assert.Equal(t, sqliteStoreName, temporalCfg.Persistence.DefaultStore)
	assert.Equal(t, sqliteStoreName, temporalCfg.Persistence.VisibilityStore)
	assert.EqualValues(t, 1, temporalCfg.Persistence.NumHistoryShards)

	assert.Contains(t, temporalCfg.Services, "frontend")
	assert.Equal(t, cfg.FrontendPort, temporalCfg.Services["frontend"].RPC.GRPCPort)
	assert.Equal(t, cfg.BindIP, temporalCfg.Services["frontend"].RPC.BindOnIP)
	assert.Equal(t, cfg.FrontendPort+1, temporalCfg.Services["history"].RPC.GRPCPort)
	assert.Equal(t, cfg.FrontendPort+2, temporalCfg.Services["matching"].RPC.GRPCPort)
	assert.Equal(t, cfg.FrontendPort+3, temporalCfg.Services["worker"].RPC.GRPCPort)

	assert.Equal(t, "127.0.0.1:7233", temporalCfg.PublicClient.HostPort)
	assert.Equal(t, "127.0.0.1:8233", temporalCfg.Global.Metrics.Prometheus.ListenAddress)
	assert.Equal(t, "127.0.0.1:7233", temporalCfg.ClusterMetadata.ClusterInformation[cfg.ClusterName].RPCAddress)
	assert.Equal(t, cfg.ClusterName, temporalCfg.ClusterMetadata.CurrentClusterName)
	assert.Equal(t, int64(10), temporalCfg.ClusterMetadata.FailoverVersionIncrement)
}

func TestBuildStaticHostsConfiguration(t *testing.T) {
	t.Parallel()

	cfg := &Config{BindIP: "0.0.0.0", FrontendPort: 7000}
	hosts := buildStaticHosts(cfg)

	require.Len(t, hosts, 4)
	assert.Equal(t, "0.0.0.0:7000", hosts[primitives.FrontendService].Self)
	assert.Equal(t, "0.0.0.0:7001", hosts[primitives.HistoryService].Self)
	assert.Equal(t, "0.0.0.0:7002", hosts[primitives.MatchingService].Self)
	assert.Equal(t, "0.0.0.0:7003", hosts[primitives.WorkerService].Self)
}

func TestBuildSQLiteConnectAttrsModes(t *testing.T) {
	t.Parallel()

	memoryAttrs := buildSQLiteConnectAttrs(&Config{DatabaseFile: ":memory:"})
	assert.Equal(t, "memory", memoryAttrs["mode"])
	assert.Equal(t, "shared", memoryAttrs["cache"])

	fileAttrs := buildSQLiteConnectAttrs(&Config{DatabaseFile: "temporal.db"})
	assert.Equal(t, "private", fileAttrs["cache"])
	assert.Equal(t, "wal", fileAttrs["journal_mode"])
	assert.Equal(t, "2", fileAttrs["synchronous"])
	assert.Equal(t, "true", fileAttrs["setup"])
}

func TestBuildTemporalConfigMetricsPort(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		DatabaseFile: ":memory:",
		FrontendPort: maxPort - 999,
		BindIP:       "127.0.0.1",
		Namespace:    "default",
		ClusterName:  "cluster",
		EnableUI:     true,
		UIPort:       8233,
		LogLevel:     "info",
		StartTimeout: time.Second,
	}

	_, err := buildTemporalConfig(cfg)
	require.Error(t, err)
}

func TestBuildTemporalConfigClusterMetadata(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		DatabaseFile: ":memory:",
		FrontendPort: 7233,
		BindIP:       "127.0.0.1",
		Namespace:    "default",
		ClusterName:  "cluster",
		EnableUI:     true,
		UIPort:       8233,
		LogLevel:     "warn",
		StartTimeout: time.Second,
	}

	temporalCfg, err := buildTemporalConfig(cfg)
	require.NoError(t, err)

	info, ok := temporalCfg.ClusterMetadata.ClusterInformation[cfg.ClusterName]
	require.True(t, ok)
	assert.True(t, info.Enabled)
	assert.Equal(t, int64(1), info.InitialFailoverVersion)
	assert.NotEmpty(t, info.ClusterID)
	assert.IsType(t, &cluster.Config{}, temporalCfg.ClusterMetadata)
}
