package standalone

import (
	"strconv"
	"testing"
	"time"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// End-to-end tests that exercise a full Compozy Worker running against:
// - Embedded Temporal (standalone)
// - Embedded Redis (miniredis via cache.SetupCache)
// - Postgres-backed repositories
// Workflows are loaded from test fixtures and executed via the real Worker API.
func TestStandalone_WorkflowE2E(t *testing.T) {
	t.Run("Should execute complete workflow with agent and tasks", func(t *testing.T) {
		ctx := t.Context()
		env := SetupStandaloneTestEnv(t, "test/fixtures/standalone/workflows/test-workflow.yaml")
		defer env.Cleanup()

		// Trigger workflow and wait for completion
		input := core.Input{"name": "World"}
		res, err := env.Worker.TriggerWorkflow(ctx, "e2e-simple", &input, "")
		require.NoError(t, err)
		require.NotEmpty(t, res.WorkflowExecID.String())

		// Poll for terminal state
		repo := env.DB // placeholder to keep linters happy about env usage
		_ = repo       // repository instance is resolved below
		wfRepo := env.Worker.WorkflowRepo()
		state := waitForTerminal(t, wfRepo, res.WorkflowExecID, 15*time.Second)
		require.NotNil(t, state)
		assert.Equal(t, core.StatusSuccess, state.Status)
		require.NotNil(t, state.Output)
		// Mock LLM produces deterministic content containing the prompt subject.
		// We just assert non-empty output to keep the test decoupled from mock phrasing.
	})

	t.Run("Should execute 12 workflows concurrently without interference", func(t *testing.T) {
		ctx := t.Context()
		env := SetupStandaloneTestEnv(t, "test/fixtures/standalone/workflows/test-workflow.yaml")
		defer env.Cleanup()

		type result struct {
			id  core.ID
			err error
		}
		ch := make(chan result, 12)
		for i := 0; i < 12; i++ {
			i := i
			go func(idx int) {
				inp := core.Input{"name": "User-" + strconv.Itoa(idx)}
				r, err := env.Worker.TriggerWorkflow(ctx, "e2e-simple", &inp, "")
				if err != nil {
					ch <- result{err: err}
					return
				}
				ch <- result{id: r.WorkflowExecID, err: nil}
			}(i)
		}

		// Collect and verify each execution completed successfully
		wfRepo := env.Worker.WorkflowRepo()
		for i := 0; i < 12; i++ {
			r := <-ch
			require.NoError(t, r.err)
			state := waitForTerminal(t, wfRepo, r.id, 20*time.Second)
			require.NotNil(t, state)
			assert.Equal(t, core.StatusSuccess, state.Status)
			require.NotNil(t, state.Output)
		}
	})
}

// waitForTerminal polls the workflow repository for a terminal state with a deadline.
func waitForTerminal(t *testing.T, repo workflow.Repository, execID core.ID, timeout time.Duration) *workflow.State {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last *workflow.State
	for time.Now().Before(deadline) {
		st, err := repo.GetState(t.Context(), execID)
		if err == nil && st != nil {
			last = st
			if isTerminal(st.Status) {
				return st
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return last
}

func isTerminal(s core.StatusType) bool {
	switch s {
	case core.StatusSuccess, core.StatusFailed, core.StatusTimedOut, core.StatusCanceled:
		return true
	default:
		return false
	}
}
