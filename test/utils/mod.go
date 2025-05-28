package utils

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/infra/nats"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
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
	StateDir     string
	NatsServer   *nats.Server
	NatsClient   *nats.Client
	Store        *store.Store
	WorkflowRepo *store.WorkflowRepository
	TaskRepo     *store.TaskRepository
	AgentRepo    *store.AgentRepository
	ToolRepo     *store.ToolRepository
}

// initTestEnvironment initializes common test environment setup
func initTestEnvironment(t *testing.T) {
	t.Helper()
	logger.Init(&logger.Config{
		Level:  logger.ErrorLevel,
		Output: os.Stderr,
		JSON:   false,
	})
	if GlobalBaseTestDir == "" {
		var err error
		GlobalBaseTestDir, err = os.MkdirTemp("", "compozy_integration_fallback_")
		require.NoError(t, err, "Failed to create global base test directory (fallback)")
		t.Logf("Warning: GlobalBaseTestDir created by fallback for test %s. "+
			"Consider running tests at package level.", t.Name())
	}
}

func SetupIntegrationTestBed(t *testing.T, testTimeout time.Duration) *IntegrationTestBed {
	t.Helper()
	initTestEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	ns, nc := GetSharedNatsServer(t)
	require.NotNil(t, ns, "NATS test server should not be nil")
	require.NotNil(t, nc, "NATS client should not be nil")
	return setupIntegrationTestBedCommon(ctx, t, cancel, ns, nc)
}

func SetupIntegrationTestBedWithNats(
	t *testing.T,
	testTimeout time.Duration,
	natsServer *nats.Server,
	natsClient *nats.Client,
) *IntegrationTestBed {
	t.Helper()
	initTestEnvironment(t)
	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	require.NotNil(t, natsServer, "NATS test server should not be nil")
	require.NotNil(t, natsClient, "NATS client should not be nil")
	return setupIntegrationTestBedCommon(ctx, t, cancel, natsServer, natsClient)
}

// setupIntegrationTestBedCommon contains the common setup logic for both functions
func setupIntegrationTestBedCommon(
	ctx context.Context,
	t *testing.T,
	cancel context.CancelFunc,
	ns *nats.Server,
	nc *nats.Client,
) *IntegrationTestBed {
	stateDir := filepath.Join(GlobalBaseTestDir, t.Name())
	err := os.MkdirAll(stateDir, 0o750)
	require.NoError(t, err)

	// Create the database file path (not just the directory)
	dbFilePath := filepath.Join(stateDir, "compozy.db")
	testStore, err := store.NewStore(dbFilePath)
	require.NoError(t, err)

	// Setup the store (run migrations)
	err = testStore.Setup()
	require.NoError(t, err)

	// Create a minimal project config for testing
	projectConfig := &project.Config{}
	err = projectConfig.SetCWD(stateDir)
	require.NoError(t, err)

	// Create empty workflows slice for testing
	workflows := []*workflow.Config{}

	// Create repositories with correct constructor signatures
	workflowRepo := testStore.NewWorkflowRepository(projectConfig, workflows)
	taskRepo := testStore.NewTaskRepository(workflowRepo)
	agentRepo := testStore.NewAgentRepository(workflowRepo, taskRepo)
	toolRepo := testStore.NewToolRepository(workflowRepo, taskRepo)

	return &IntegrationTestBed{
		T:            t,
		Ctx:          ctx,
		cancelCtx:    cancel,
		StateDir:     stateDir,
		NatsServer:   ns,
		NatsClient:   nc,
		Store:        testStore,
		WorkflowRepo: workflowRepo,
		TaskRepo:     taskRepo,
		AgentRepo:    agentRepo,
		ToolRepo:     toolRepo,
	}
}

func (tb *IntegrationTestBed) Cleanup() {
	tb.T.Helper()

	// Cancel context first to signal shutdown
	tb.cancelCtx()

	// Only close NATS client and server if they're not shared instances
	// Check if this is a shared instance by comparing with the global shared instances
	if tb.NatsClient != nil && tb.NatsClient != sharedNatsClient {
		err := tb.NatsClient.Close()
		if err != nil {
			tb.T.Logf("Error closing NATS client: %s", err)
		}
		tb.NatsClient = nil
	}

	if tb.NatsServer != nil && tb.NatsServer != sharedNatsServer {
		err := tb.NatsServer.Shutdown()
		if err != nil {
			tb.T.Logf("Error shutting down NATS server: %s", err)
		}
		// Wait for shutdown with timeout to avoid hanging tests
		shutdownDone := make(chan struct{})
		go func() {
			tb.NatsServer.WaitForShutdown()
			close(shutdownDone)
		}()

		select {
		case <-shutdownDone:
			// Shutdown completed normally
		case <-time.After(5 * time.Second):
			tb.T.Logf("NATS server shutdown timed out after 5 seconds")
		}
		tb.NatsServer = nil
	}

	// Finally close the store
	if tb.Store != nil {
		err := tb.Store.Close()
		if err != nil {
			tb.T.Logf("Error closing store: %s", err)
		}
		tb.Store = nil
	}
}

func MainTestRunner(m *testing.M) int {
	var err error
	GlobalBaseTestDir, err = os.MkdirTemp("", "compozy_integration_tests_global_")
	if err != nil {
		panic("Failed to create global base test directory for integration tests: " + err.Error())
	}

	exitCode := m.Run()

	CleanupSharedNats()

	err = os.RemoveAll(GlobalBaseTestDir)
	if err != nil {
		os.Stderr.WriteString("Failed to remove global base test directory: " + err.Error() + "\n")
	}
	return exitCode
}
