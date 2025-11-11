package temporal

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/worker/embedded"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/test/helpers"
	enumspb "go.temporal.io/api/enums/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	testTaskQueue   = "temporal-standalone-integration"
	workflowTimeout = 30 * time.Second
)

func init() {
	if os.Getenv("TEMPORAL_TEST_PORT_BASE") == "" {
		_ = os.Setenv("TEMPORAL_TEST_PORT_BASE", "45000")
	}
}

type workflowExecution struct {
	WorkflowID string
	RunID      string
	Input      string
	Result     string
}

type workflowInput struct {
	Name string `json:"name"`
}

func TestStandaloneMemoryMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	t.Run("Should execute workflow using in-memory persistence", func(t *testing.T) {
		t.Helper()
		ctx := helpers.NewTestContext(t)
		cfg := newEmbeddedConfigFromDefaults()
		cfg.DatabaseFile = filepath.Join(t.TempDir(), fmt.Sprintf("temporal-%s.db", uuid.NewString()))
		cfg.EnableUI = false
		cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)
		server := startStandaloneServer(ctx, t, cfg)
		exec := executeTestWorkflow(ctx, t, server.FrontendAddress(), cfg.Namespace)
		require.Equal(t, strings.ToUpper(exec.Input), exec.Result)
	})
}

func TestStandaloneFileMode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	t.Run("Should persist workflow results to disk", func(t *testing.T) {
		t.Helper()
		ctx := helpers.NewTestContext(t)
		dbPath := filepath.Join(t.TempDir(), "temporal.db")
		cfg := newEmbeddedConfigFromDefaults()
		cfg.DatabaseFile = dbPath
		cfg.EnableUI = false
		cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)
		server := startStandaloneServer(ctx, t, cfg)
		exec := executeTestWorkflow(ctx, t, server.FrontendAddress(), cfg.Namespace)
		require.Equal(t, strings.ToUpper(exec.Input), exec.Result)
		require.Eventually(t, func() bool {
			info, err := os.Stat(dbPath)
			return err == nil && info.Size() > 0
		}, 5*time.Second, 100*time.Millisecond)
	})
}

func TestStandaloneCustomPorts(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	t.Run("Should honor custom port selection", func(t *testing.T) {
		t.Helper()
		ctx := helpers.NewTestContext(t)
		frontendPort := findAvailablePortRange(ctx, t, 4)
		cfg := newEmbeddedConfigFromDefaults()
		cfg.DatabaseFile = filepath.Join(t.TempDir(), fmt.Sprintf("temporal-%s.db", uuid.NewString()))
		cfg.FrontendPort = frontendPort
		cfg.EnableUI = false
		server := startStandaloneServer(ctx, t, cfg)
		require.Equal(t, fmt.Sprintf("%s:%d", cfg.BindIP, cfg.FrontendPort), server.FrontendAddress())
		exec := executeTestWorkflow(ctx, t, server.FrontendAddress(), cfg.Namespace)
		require.Equal(t, strings.ToUpper(exec.Input), exec.Result)
	})
}

func TestStandaloneWorkflowExecution(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	t.Run("Should report completed workflow execution", func(t *testing.T) {
		t.Helper()
		ctx := helpers.NewTestContext(t)
		cfg := newEmbeddedConfigFromDefaults()
		cfg.DatabaseFile = filepath.Join(t.TempDir(), fmt.Sprintf("temporal-%s.db", uuid.NewString()))
		cfg.EnableUI = false
		cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)
		server := startStandaloneServer(ctx, t, cfg)
		exec := executeTestWorkflow(ctx, t, server.FrontendAddress(), cfg.Namespace)
		require.Equal(t, strings.ToUpper(exec.Input), exec.Result)
		desc, err := describeWorkflow(ctx, t, server.FrontendAddress(), cfg.Namespace, exec.WorkflowID, exec.RunID)
		require.NoError(t, err)
		require.Equal(t, enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED, desc.WorkflowExecutionInfo.Status)
	})
}

func startStandaloneServer(ctx context.Context, t *testing.T, cfg *embedded.Config) *embedded.Server {
	t.Helper()
	var lastErr error
	for attempts := 0; attempts < 5; attempts++ {
		server, err := embedded.NewServer(ctx, cfg)
		if isAddressInUseErr(err) {
			cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)
			lastErr = err
			continue
		}
		require.NoError(t, err)
		startErr := server.Start(ctx)
		if isAddressInUseErr(startErr) {
			lastErr = startErr
			stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			server.Stop(stopCtx) // ignore error; attempting retry
			cancel()
			cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)
			continue
		}
		require.NoError(t, startErr)
		t.Cleanup(func() {
			stopTemporalServer(ctx, t, server)
		})
		return server
	}
	if lastErr != nil {
		require.FailNow(t, "failed to start embedded temporal server after retries", lastErr)
	}
	require.FailNow(t, "failed to start embedded temporal server after retries")
	return nil
}

func isAddressInUseErr(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "address already in use")
}

func stopTemporalServer(ctx context.Context, t *testing.T, server *embedded.Server) {
	t.Helper()
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
	defer cancel()
	require.NoError(t, server.Stop(stopCtx))
}

func executeTestWorkflow(
	ctx context.Context,
	t *testing.T,
	address string,
	namespace string,
	workflowID ...string,
) workflowExecution {
	t.Helper()
	id := ""
	if len(workflowID) > 0 {
		id = workflowID[0]
	}
	if id == "" {
		id = fmt.Sprintf("standalone-%s", uuid.NewString())
	}
	exec, err := runWorkflow(ctx, t, address, namespace, id)
	require.NoError(t, err)
	return exec
}

func runWorkflow(
	ctx context.Context,
	t *testing.T,
	address string,
	namespace string,
	workflowID string,
) (workflowExecution, error) {
	t.Helper()
	c := dialTemporalClient(t, address, namespace)
	defer closeTemporalClient(t, c)
	w := startTestWorker(t, c)
	defer stopWorker(w)
	input := loadWorkflowInput(t)
	runCtx, cancel := context.WithTimeout(ctx, workflowTimeout)
	defer cancel()
	opts := client.StartWorkflowOptions{
		ID:                    workflowID,
		TaskQueue:             testTaskQueue,
		WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE,
	}
	if opts.WorkflowIDReusePolicy == enumspb.WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED {
		opts.WorkflowIDReusePolicy = enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE
	}
	run, err := c.ExecuteWorkflow(runCtx, opts, integrationWorkflow, input.Name)
	if err != nil {
		return workflowExecution{}, err
	}
	var result string
	if err := run.Get(runCtx, &result); err != nil {
		return workflowExecution{}, err
	}
	return workflowExecution{WorkflowID: opts.ID, RunID: run.GetRunID(), Input: input.Name, Result: result}, nil
}

func describeWorkflow(
	ctx context.Context,
	t *testing.T,
	address string,
	namespace string,
	workflowID string,
	runID string,
) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
	t.Helper()
	client := dialTemporalClient(t, address, namespace)
	defer closeTemporalClient(t, client)
	describeCtx, cancel := context.WithTimeout(ctx, workflowTimeout)
	defer cancel()
	return client.DescribeWorkflowExecution(describeCtx, workflowID, runID)
}

func dialTemporalClient(t *testing.T, address string, namespace string) client.Client {
	t.Helper()
	c, err := client.Dial(client.Options{HostPort: address, Namespace: namespace})
	require.NoError(t, err)
	return c
}

func closeTemporalClient(t *testing.T, c client.Client) {
	t.Helper()
	c.Close()
}

func startTestWorker(t *testing.T, c client.Client) worker.Worker {
	t.Helper()
	w := worker.New(c, testTaskQueue, worker.Options{})
	w.RegisterWorkflow(integrationWorkflow)
	w.RegisterActivity(integrationActivity)
	require.NoError(t, w.Start())
	return w
}

func stopWorker(w worker.Worker) {
	w.Stop()
}

func integrationWorkflow(ctx workflow.Context, name string) (string, error) {
	options := workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Second}
	ctx = workflow.WithActivityOptions(ctx, options)
	var result string
	if err := workflow.ExecuteActivity(ctx, integrationActivity, name).Get(ctx, &result); err != nil {
		return "", err
	}
	return result, nil
}

func integrationActivity(_ context.Context, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("name is required")
	}
	return strings.ToUpper(name), nil
}

func loadWorkflowInput(t *testing.T) workflowInput {
	t.Helper()
	path := filepath.Join("testdata", "workflow_input.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var input workflowInput
	require.NoError(t, json.Unmarshal(data, &input))
	return input
}

func findAvailablePortRange(ctx context.Context, t *testing.T, size int) int {
	t.Helper()
	start := temporalPortSearchStart()
	deadline := time.Now().Add(5 * time.Second)
	maxSequential := 60000
	maxPort := 65535

	tryReserve := func(port int) bool {
		if port <= 0 || port+size+1000 > maxPort {
			return false
		}
		if !reservePorts(ctx, port, size) {
			return false
		}
		if !reservePorts(ctx, port+1000, 1) {
			return false
		}
		return true
	}

	// First attempt sequential scan to keep behavior deterministic.
	for port := start; port < maxSequential; port++ {
		if time.Now().After(deadline) {
			break
		}
		if tryReserve(port) {
			return port
		}
	}

	// Fallback to random probing before giving up.
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for attempts := 0; attempts < 200; attempts++ {
		if time.Now().After(deadline) {
			break
		}
		candidate := start + rng.Intn(maxPort-start-size-1000)
		// Align candidate to avoid wrapping when checking contiguous ports.
		candidate -= candidate % size
		if tryReserve(candidate) {
			return candidate
		}
	}

	t.Fatalf("no available port range found after retries (start=%d, size=%d)", start, size)
	return 0
}

func temporalPortSearchStart() int {
	raw := strings.TrimSpace(os.Getenv("TEMPORAL_TEST_PORT_BASE"))
	if raw == "" {
		return 15000
	}
	base, err := strconv.Atoi(raw)
	if err != nil || base < 1024 || base > 60000 {
		return 15000
	}
	return base
}

func reservePorts(ctx context.Context, start int, size int) bool {
	listeners := make([]net.Listener, 0, size)
	for offset := 0; offset < size; offset++ {
		ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", start+offset))
		if err != nil {
			for _, listener := range listeners {
				_ = listener.Close()
			}
			return false
		}
		listeners = append(listeners, ln)
	}
	for _, listener := range listeners {
		_ = listener.Close()
	}
	return true
}

func newEmbeddedConfigFromDefaults() *embedded.Config {
	defaults := config.Default().Temporal.Standalone
	return &embedded.Config{
		DatabaseFile: defaults.DatabaseFile,
		FrontendPort: defaults.FrontendPort,
		BindIP:       defaults.BindIP,
		Namespace:    defaults.Namespace,
		ClusterName:  defaults.ClusterName,
		EnableUI:     defaults.EnableUI,
		RequireUI:    defaults.RequireUI,
		UIPort:       defaults.UIPort,
		LogLevel:     defaults.LogLevel,
		StartTimeout: defaults.StartTimeout,
	}
}

func defaultNamespace() string {
	return config.Default().Temporal.Namespace
}
