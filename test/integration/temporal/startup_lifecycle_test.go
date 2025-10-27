package temporal

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/worker/embedded"
	"github.com/compozy/compozy/test/helpers"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const lifecycleTaskQueue = "temporal-startup-lifecycle"

func TestGracefulShutdownDuringStartup(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	t.Run("Should cancel startup gracefully", func(t *testing.T) {
		t.Helper()
		ctx := helpers.NewTestContext(t)
		cfg := newEmbeddedConfigFromDefaults()
		cfg.EnableUI = false
		cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)
		server, err := embedded.NewServer(ctx, cfg)
		require.NoError(t, err)
		t.Cleanup(func() {
			stopTemporalServer(ctx, t, server)
		})
		startCtx, cancel := context.WithCancel(ctx)
		errCh := make(chan error, 1)
		go func() {
			errCh <- server.Start(startCtx)
		}()
		cancel()
		startErr := <-errCh
		require.Error(t, startErr)
		require.ErrorContains(t, startErr, "context canceled")
		require.ErrorContains(t, startErr, "wait for ready")
		stopCtx, stopCancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer stopCancel()
		require.NoError(t, server.Stop(stopCtx))
	})
}

func TestMultipleStartCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	t.Run("Should fail when starting an already running server", func(t *testing.T) {
		t.Helper()
		ctx := helpers.NewTestContext(t)
		cfg := newEmbeddedConfigFromDefaults()
		cfg.EnableUI = false
		cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)
		server := startStandaloneServer(ctx, t, cfg)
		err := server.Start(ctx)
		require.Error(t, err)
		require.ErrorContains(t, err, "already started")
	})
}

func TestMultipleStopCalls(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	t.Run("Should allow repeated stop calls", func(t *testing.T) {
		t.Helper()
		ctx := helpers.NewTestContext(t)
		cfg := newEmbeddedConfigFromDefaults()
		cfg.EnableUI = false
		cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)
		server := startStandaloneServer(ctx, t, cfg)
		firstStopCtx, firstCancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
		require.NoError(t, server.Stop(firstStopCtx))
		firstCancel()
		secondStopCtx, secondCancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
		require.NoError(t, server.Stop(secondStopCtx))
		secondCancel()
	})
}

func TestConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	t.Run("Should coordinate shutdown during concurrent workflow execution", func(t *testing.T) {
		t.Helper()
		ctx := helpers.NewTestContext(t)
		cfg := newEmbeddedConfigFromDefaults()
		cfg.EnableUI = false
		cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)
		server := startStandaloneServer(ctx, t, cfg)
		lifecycleClient := dialTemporalClient(t, server.FrontendAddress(), cfg.Namespace)
		defer closeTemporalClient(t, lifecycleClient)
		lifecycleWorker := worker.New(lifecycleClient, lifecycleTaskQueue, worker.Options{})
		lifecycleWorker.RegisterWorkflow(lifecycleWorkflow)
		lifecycleWorker.RegisterActivity(lifecycleActivity)
		require.NoError(t, lifecycleWorker.Start())
		defer lifecycleWorker.Stop()
		const workflowCount = 12
		results := make([]error, 0, workflowCount)
		var resultsMu sync.Mutex
		var wg sync.WaitGroup
		for i := 0; i < workflowCount; i++ {
			idx := i
			wg.Go(func() {
				err := runLifecycleWorkflow(ctx, lifecycleClient, idx)
				resultsMu.Lock()
				results = append(results, err)
				resultsMu.Unlock()
			})
		}
		time.Sleep(3 * time.Second)
		stopCtx, stopCancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer stopCancel()
		stopErrCh := make(chan error, 1)
		go func() {
			stopErrCh <- server.Stop(stopCtx)
		}()
		wg.Wait()
		require.NoError(t, <-stopErrCh)
		successes := 0
		cancellations := 0
		for _, err := range results {
			if err == nil {
				successes++
				continue
			}
			cancellations++
			errMsg := err.Error()
			require.Truef(
				t,
				strings.Contains(errMsg, "context canceled") ||
					strings.Contains(errMsg, "context deadline exceeded") ||
					strings.Contains(errMsg, "transport is closing"),
				"unexpected workflow error: %v",
				err,
			)
		}
		require.Greater(t, successes, 0)
		require.Equal(t, workflowCount, successes+cancellations)
	})
}

func TestServerRestartCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	t.Run("Should restart standalone server without data loss", func(t *testing.T) {
		t.Helper()
		dbPath := filepath.Join(t.TempDir(), "temporal-restart.db")
		for i := 0; i < 2; i++ {
			cycleCtx := helpers.NewTestContext(t)
			cfg := newEmbeddedConfigFromDefaults()
			cfg.EnableUI = false
			cfg.DatabaseFile = dbPath
			cfg.FrontendPort = findAvailablePortRange(cycleCtx, t, 4)
			server := startStandaloneServer(cycleCtx, t, cfg)
			exec := executeTestWorkflow(cycleCtx, t, server.FrontendAddress(), cfg.Namespace)
			require.Equal(t, strings.ToUpper(exec.Input), exec.Result)
			stopTemporalServer(cycleCtx, t, server)
		}
	})
}

func runLifecycleWorkflow(ctx context.Context, c client.Client, idx int) error {
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	opts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("lifecycle-%d-%s", idx, uuid.NewString()),
		TaskQueue: lifecycleTaskQueue,
	}
	run, err := c.ExecuteWorkflow(runCtx, opts, lifecycleWorkflow, fmt.Sprintf("workflow-%d", idx))
	if err != nil {
		return err
	}
	var result string
	if err := run.Get(runCtx, &result); err != nil {
		return err
	}
	if result == "" {
		return fmt.Errorf("empty result")
	}
	return nil
}

// lifecycleWorkflow executes a slower workflow to exercise shutdown behavior.
func lifecycleWorkflow(ctx workflow.Context, name string) (string, error) {
	options := workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Second}
	ctx = workflow.WithActivityOptions(ctx, options)
	var result string
	if err := workflow.ExecuteActivity(ctx, lifecycleActivity, name).Get(ctx, &result); err != nil {
		return "", err
	}
	return result, nil
}

// lifecycleActivity simulates a heavier activity load before delegating to the default integration activity.
func lifecycleActivity(ctx context.Context, name string) (string, error) {
	select {
	case <-time.After(250 * time.Millisecond):
	case <-ctx.Done():
		return "", ctx.Err()
	}
	return integrationActivity(ctx, name)
}
