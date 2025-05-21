package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/stmanager"
	"github.com/compozy/compozy/engine/store"
	"github.com/compozy/compozy/pkg/nats"
	"github.com/compozy/compozy/pkg/utils"
	"github.com/stretchr/testify/require"
)

const (
	DefaultTestTimeout = 10 * time.Second
)

var GlobalBaseTestDir string

func TestMain(m *testing.M) {
	os.Exit(MainTestRunner(m))
}

type IntegrationTestBed struct {
	T            *testing.T
	Ctx          context.Context
	cancelCtx    context.CancelFunc
	NatsServer   *nats.Server
	NatsClient   *nats.Client
	StateManager *stmanager.Manager
	StateDir     string
}

func SetupIntegrationTestBed(t *testing.T, testTimeout time.Duration, componentsToWatch []nats.ComponentType) *IntegrationTestBed {
	t.Helper()

	if GlobalBaseTestDir == "" {
		var err error
		GlobalBaseTestDir, err = os.MkdirTemp("", "compozy_integration_fallback_")
		require.NoError(t, err, "Failed to create global base test directory (fallback)")
		t.Logf("Warning: GlobalBaseTestDir created by fallback in SetupIntegrationTestBed for test %s. Consider running tests at package level.", t.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	natsTestSrv, natsClient := utils.SetupNatsServer(ctx, t)
	require.NotNil(t, natsTestSrv, "NATS test server should not be nil")
	require.NotNil(t, natsClient, "NATS client should not be nil")
	stateDir := filepath.Join(GlobalBaseTestDir, t.Name())
	err := os.MkdirAll(stateDir, 0o750)
	require.NoError(t, err)

	store, err := store.NewStore(stateDir)
	require.NoError(t, err)

	managerOpts := []stmanager.ManagerOption{
		stmanager.WithStore(store),
		stmanager.WithNatsClient(natsClient),
	}
	if len(componentsToWatch) > 0 {
		managerOpts = append(managerOpts, stmanager.WithComponents(componentsToWatch))
	}

	stateManager, err := stmanager.NewManager(managerOpts...)
	require.NoError(t, err)
	require.NotNil(t, stateManager, "State manager should not be nil")

	return &IntegrationTestBed{
		T:            t,
		Ctx:          ctx,
		cancelCtx:    cancel,
		NatsServer:   natsTestSrv,
		NatsClient:   natsClient,
		StateManager: stateManager,
		StateDir:     stateDir,
	}
}

func (tb *IntegrationTestBed) Cleanup() {
	tb.T.Helper()
	tb.cancelCtx()

	if tb.StateManager != nil {
		err := tb.StateManager.Close()
		if err != nil {
			tb.T.Logf("Error closing state manager: %s", err)
		}
	}
	if tb.NatsClient != nil {
		err := tb.NatsClient.Close()
		if err != nil {
			tb.T.Logf("Error closing NATS client: %s", err)
		}
	}
	if tb.NatsServer != nil {
		tb.NatsServer.Shutdown()
	}
}

func SetupStateManagerForSubtest(t *testing.T, parentBaseDir string, natsClient *nats.Client, componentsToWatch []nats.ComponentType) *stmanager.Manager {
	t.Helper()

	subtestStateDir := filepath.Join(parentBaseDir, t.Name())
	err := os.MkdirAll(subtestStateDir, 0o750)
	require.NoError(t, err)

	store, err := store.NewStore(subtestStateDir)
	require.NoError(t, err)

	managerOpts := []stmanager.ManagerOption{
		stmanager.WithStore(store),
		stmanager.WithNatsClient(natsClient),
	}
	if len(componentsToWatch) > 0 {
		managerOpts = append(managerOpts, stmanager.WithComponents(componentsToWatch))
	}

	stateManager, err := stmanager.NewManager(managerOpts...)
	require.NoError(t, err)
	require.NotNil(t, stateManager)

	return stateManager
}

func MainTestRunner(m *testing.M) int {
	var err error
	GlobalBaseTestDir, err = os.MkdirTemp("", "compozy_integration_tests_global_")
	if err != nil {
		panic("Failed to create global base test directory for integration tests: " + err.Error())
	}

	exitCode := m.Run()

	err = os.RemoveAll(GlobalBaseTestDir)
	if err != nil {
		os.Stderr.WriteString("Failed to remove global base test directory: " + err.Error() + "\n")
	}
	return exitCode
}
