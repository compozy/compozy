package tool

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/llm"
	"github.com/compozy/compozy/engine/project"
	coreruntime "github.com/compozy/compozy/engine/runtime"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/tool/resolver"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/test/helpers"
)

// defaultTestTimeout is the maximum duration for any test using SetupTestEnvironment.
const defaultTestTimeout = 30 * time.Second

// TestEnvironment provides a complete test environment for tool inheritance tests
type TestEnvironment struct {
	ctx     context.Context
	pool    *pgxpool.Pool
	cleanup func()
}

// SetupTestEnvironment creates a test environment with real database
func SetupTestEnvironment(t *testing.T) *TestEnvironment {
	t.Helper()
	// Use a cancellable context with timeout to prevent hanging tests.
	ctx, cancel := context.WithTimeout(t.Context(), defaultTestTimeout)
	// Ensure cancel is called even on test panic or early return.
	t.Cleanup(cancel)
	// Use shared container for better performance
	pool, dbCleanup := helpers.GetSharedPostgresDB(t)
	// Ensure tables exist (shared container migrations)
	require.NoError(t, helpers.EnsureTablesExistForTest(pool))
	env := &TestEnvironment{
		ctx:  ctx,
		pool: pool,
		cleanup: func() {
			dbCleanup()
			// Cancel context to clean up any pending operations
			cancel()
		},
	}
	// Prefer t.Cleanup to ensure cleanup runs even on panic
	t.Cleanup(env.Cleanup)
	return env
}

// Cleanup cleans up test environment
func (env *TestEnvironment) Cleanup() {
	if env.cleanup != nil {
		env.cleanup()
	}
}

// CreateTestProjectConfig creates a project config with tools for testing
func CreateTestProjectConfig(tools []tool.Config) *project.Config {
	cfg := &project.Config{
		Name:    "test-project",
		Version: "1.0",
		Tools:   tools,
	}
	// Ensure CWD is set for validation and set CWD on tools
	if err := cfg.SetCWD("."); err == nil {
		for i := range cfg.Tools {
			// Best-effort; ignore errors in helper
			_ = cfg.Tools[i].SetCWD(cfg.CWD.PathStr()) //nolint:errcheck // Best-effort; ignore errors in test helper
		}
	}
	return cfg
}

// CreateTestWorkflowConfig creates a workflow config with tools for testing
func CreateTestWorkflowConfig(tools []tool.Config) *workflow.Config {
	cfg := &workflow.Config{
		ID:          "test-workflow",
		Description: "Test Workflow",
		Tools:       tools,
	}
	// Ensure CWD is set for validation and set CWD on tools
	if err := cfg.SetCWD("."); err == nil {
		for i := range cfg.Tools {
			_ = cfg.Tools[i].SetCWD(cfg.CWD.PathStr()) //nolint:errcheck // Best-effort; ignore errors in test helper
		}
	}
	return cfg
}

// CreateTestAgentConfig creates an agent config with tools for testing
func CreateTestAgentConfig(tools []tool.Config) *agent.Config {
	cfg := &agent.Config{
		ID: "test-agent",
		Model: agent.Model{Config: core.ProviderConfig{
			Provider: core.ProviderMock,
			Model:    "test-model",
		}},
		Instructions: "Test agent for integration testing",
		LLMProperties: agent.LLMProperties{
			Tools: tools,
		},
	}
	// Set CWD for completeness; agent tools short-circuit inheritance
	_ = cfg.SetCWD(".") //nolint:errcheck // Intentionally ignore SetCWD errors in test helper
	return cfg
}

// CreateTestTool creates a test tool configuration
func CreateTestTool(id, description string) tool.Config {
	t := tool.Config{
		ID:          id,
		Description: description,
	}
	// Set a valid CWD so validation passes in resolver
	_ = t.SetCWD(".") //nolint:errcheck // Intentionally ignore SetCWD errors in test helper
	return t
}

// CreateMockRuntime creates a minimal mock runtime for testing
func CreateMockRuntime(ctx context.Context, t *testing.T) coreruntime.Runtime {
	t.Helper()
	config := coreruntime.TestConfig()
	factory := coreruntime.NewDefaultFactory(t.TempDir())
	rtManager, err := factory.CreateRuntime(ctx, config)
	if err != nil {
		panic(fmt.Sprintf("failed to create runtime: %v", err))
	}
	return rtManager
}

// AssertToolsEqual verifies that the resolved tools match expected IDs
// Note: This assumes tools are already in deterministic order
func AssertToolsEqual(t *testing.T, expected []string, actual []tool.Config) {
	t.Helper()
	actualIDs := make([]string, len(actual))
	for i := range actual {
		actualIDs[i] = actual[i].ID
	}
	assert.Equal(t, expected, actualIDs, "Tool IDs should match expected")
}

// AssertToolPrecedence verifies that a tool has the expected description (for override testing)
func AssertToolPrecedence(t *testing.T, tools []tool.Config, toolID, expectedDesc string) {
	t.Helper()
	for i := range tools {
		if tools[i].ID == toolID {
			assert.Equal(t, expectedDesc, tools[i].Description,
				"Tool %s should have description from higher precedence config", toolID)
			return
		}
	}
	t.Fatalf("Tool %s not found in resolved tools", toolID)
}

// AssertDeterministicOrder verifies tools are in alphabetical order
func AssertDeterministicOrder(t *testing.T, tools []tool.Config) {
	t.Helper()
	for i := 1; i < len(tools); i++ {
		assert.True(t, tools[i-1].ID < tools[i].ID,
			"Tools should be in alphabetical order: %s should come before %s",
			tools[i-1].ID, tools[i].ID)
	}
}

// ResolveToolsWithHierarchy resolves tools using the hierarchical resolver
func ResolveToolsWithHierarchy(
	ctx context.Context,
	projectConfig *project.Config,
	workflowConfig *workflow.Config,
	agentConfig *agent.Config,
) ([]tool.Config, error) {
	r := resolver.NewHierarchicalResolver()
	return r.ResolveTools(ctx, projectConfig, workflowConfig, agentConfig)
}

// CreateLLMServiceWithResolvedTools creates an LLM service with resolved tools
func CreateLLMServiceWithResolvedTools(
	ctx context.Context,
	t *testing.T,
	resolvedTools []tool.Config,
	agentConfig *agent.Config,
) *llm.Service {
	mockRuntime := CreateMockRuntime(ctx, t)
	service, err := llm.NewService(ctx, mockRuntime, agentConfig, func(c *llm.Config) {
		c.ResolvedTools = resolvedTools
		c.ProxyURL = "http://test-proxy"
		c.Timeout = 30 * time.Second // Fixed: was 30 nanoseconds, now 30 seconds
		c.MaxConcurrentTools = 5
	})
	require.NoError(t, err)
	require.NotNil(t, service)
	return service
}
