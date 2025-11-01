package temporal

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/test/helpers"
	enumspb "go.temporal.io/api/enums/v1"
)

func TestEmbeddedPersistence(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping temporal integration tests in short mode")
	}

	t.Run("Should persist workflows across restarts", func(t *testing.T) {
		ctx := helpers.NewTestContext(t)
		dbPath := filepath.Join(t.TempDir(), "temporal.db")
		cfg := newEmbeddedConfigFromDefaults()
		cfg.DatabaseFile = dbPath
		cfg.EnableUI = false
		cfg.FrontendPort = findAvailablePortRange(ctx, t, 4)
		server := startEmbeddedServer(ctx, t, cfg)
		workflowID := "persistent-workflow"
		firstRun, err := runWorkflow(ctx, t, server.FrontendAddress(), cfg.Namespace, workflowID)
		require.NoError(t, err)
		require.Equal(t, strings.ToUpper(firstRun.Input), firstRun.Result)
		stopTemporalServer(ctx, t, server)

		restartCtx := helpers.NewTestContext(t)
		restartCfg := newEmbeddedConfigFromDefaults()
		restartCfg.DatabaseFile = dbPath
		restartCfg.EnableUI = false
		restartCfg.FrontendPort = findAvailablePortRange(restartCtx, t, 4)
		restartCfg.Namespace = cfg.Namespace
		restarted := startEmbeddedServer(restartCtx, t, restartCfg)
		t.Cleanup(func() {
			stopTemporalServer(restartCtx, t, restarted)
		})
		resp, err := describeWorkflow(
			restartCtx,
			t,
			restarted.FrontendAddress(),
			restartCfg.Namespace,
			firstRun.WorkflowID,
			firstRun.RunID,
		)
		require.NoError(t, err)
		require.Equal(t, enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED, resp.WorkflowExecutionInfo.Status)
	})
}
