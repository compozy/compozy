package utils

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
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

func SetupIntegrationTestBed(
	t *testing.T,
	testTimeout time.Duration,
	_ []core.ComponentType,
) *IntegrationTestBed {
	t.Helper()

	// Initialize logger for tests
	logger.Init(&logger.Config{
		Level:  logger.ErrorLevel, // Use error level to reduce noise in tests
		Output: os.Stderr,
		JSON:   false,
	})

	if GlobalBaseTestDir == "" {
		var err error
		GlobalBaseTestDir, err = os.MkdirTemp("", "compozy_integration_fallback_")
		require.NoError(t, err, "Failed to create global base test directory (fallback)")
		t.Logf("Warning: GlobalBaseTestDir created by fallback in SetupIntegrationTestBed for test %s. "+
			"Consider running tests at package level.", t.Name())
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	ns, nc := SetupNatsServer(ctx, t)
	require.NotNil(t, ns, "NATS test server should not be nil")
	require.NotNil(t, nc, "NATS client should not be nil")

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
	tb.cancelCtx()

	if tb.NatsClient != nil {
		err := tb.NatsClient.Close()
		if err != nil {
			tb.T.Logf("Error closing NATS client: %s", err)
		}
	}
	if tb.NatsServer != nil {
		err := tb.NatsServer.Shutdown()
		if err != nil {
			tb.T.Logf("Error shutting down NATS server: %s", err)
		}
	}
	if tb.Store != nil {
		err := tb.Store.Close()
		if err != nil {
			tb.T.Logf("Error closing store: %s", err)
		}
	}
}

func SetupStateManagerForSubtest(
	t *testing.T,
	parentBaseDir string,
	_ *nats.Server,
	_ *nats.Client,
	_ []core.ComponentType,
) (*store.Store, *project.Config) {
	t.Helper()

	// Initialize logger for tests if not already done
	logger.Init(&logger.Config{
		Level:  logger.ErrorLevel, // Use error level to reduce noise in tests
		Output: os.Stderr,
		JSON:   false,
	})

	subtestStateDir := filepath.Join(parentBaseDir, t.Name())
	err := os.MkdirAll(subtestStateDir, 0o750)
	require.NoError(t, err)

	// Create the database file path (not just the directory)
	dbFilePath := filepath.Join(subtestStateDir, "compozy.db")
	testStore, err := store.NewStore(dbFilePath)
	require.NoError(t, err)

	// Setup the store (run migrations)
	err = testStore.Setup()
	require.NoError(t, err)

	// Create a minimal project config for testing
	projectConfig := &project.Config{}
	err = projectConfig.SetCWD(subtestStateDir)
	require.NoError(t, err)

	return testStore, projectConfig
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
