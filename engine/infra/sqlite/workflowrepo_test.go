package sqlite

import (
	"database/sql"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/store"
	"github.com/compozy/compozy/engine/llm/usage"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/workflow"
)

func TestWorkflowRepo_UpsertState(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	assert.Equal(t, state.WorkflowID, stored.WorkflowID)
	assert.Equal(t, state.Status, stored.Status)
}

func TestWorkflowRepo_GetStateByExecID(t *testing.T) {
	repo, db := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))
	insertTask(t, db, &taskInsertOptions{
		ExecID:     state.WorkflowExecID,
		WorkflowID: state.WorkflowID,
		TaskID:     "agent",
		Status:     core.StatusSuccess,
	})

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	require.Len(t, stored.Tasks, 1)
	assert.Equal(t, core.StatusSuccess, stored.Tasks["agent"].Status)
}

func TestWorkflowRepo_ListStatesByStatus(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	running := sampleWorkflowState()
	success := sampleWorkflowState()
	success.Status = core.StatusSuccess
	require.NoError(t, repo.UpsertState(ctx, running))
	require.NoError(t, repo.UpsertState(ctx, success))

	filter := &workflow.StateFilter{Status: &success.Status}
	states, err := repo.ListStates(ctx, filter)
	require.NoError(t, err)
	require.Len(t, states, 1)
	assert.Equal(t, success.WorkflowExecID, states[0].WorkflowExecID)
}

func TestWorkflowRepo_ListStatesByWorkflowID(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	target := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, target))
	require.NoError(t, repo.UpsertState(ctx, sampleWorkflowState()))

	filter := &workflow.StateFilter{WorkflowID: &target.WorkflowID}
	states, err := repo.ListStates(ctx, filter)
	require.NoError(t, err)
	require.Len(t, states, 1)
	assert.Equal(t, target.WorkflowExecID, states[0].WorkflowExecID)
}

func TestWorkflowRepo_UpdateStatus(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))
	require.NoError(t, repo.UpdateStatus(ctx, state.WorkflowExecID, core.StatusSuccess))

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	assert.Equal(t, core.StatusSuccess, stored.Status)
}

func TestWorkflowRepo_GetStateByTaskID(t *testing.T) {
	repo, db := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))
	insertTask(t, db, &taskInsertOptions{
		ExecID:     state.WorkflowExecID,
		WorkflowID: state.WorkflowID,
		TaskID:     "plan",
		Status:     core.StatusSuccess,
	})

	stored, err := repo.GetStateByTaskID(ctx, state.WorkflowID, "plan")
	require.NoError(t, err)
	assert.Equal(t, state.WorkflowExecID, stored.WorkflowExecID)
}

func TestWorkflowRepo_GetStateByAgentID(t *testing.T) {
	repo, db := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))
	agentID := "agent-1"
	actionID := "action-1"
	insertTask(t, db, &taskInsertOptions{
		ExecID:     state.WorkflowExecID,
		WorkflowID: state.WorkflowID,
		TaskID:     "agent-task",
		AgentID:    &agentID,
		ActionID:   &actionID,
		Status:     core.StatusSuccess,
	})

	stored, err := repo.GetStateByAgentID(ctx, state.WorkflowID, agentID)
	require.NoError(t, err)
	assert.Equal(t, state.WorkflowExecID, stored.WorkflowExecID)
}

func TestWorkflowRepo_GetStateByToolID(t *testing.T) {
	repo, db := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))
	toolID := "tool-1"
	insertTask(t, db, &taskInsertOptions{
		ExecID:     state.WorkflowExecID,
		WorkflowID: state.WorkflowID,
		TaskID:     "tool-task",
		ToolID:     &toolID,
		Status:     core.StatusSuccess,
	})

	stored, err := repo.GetStateByToolID(ctx, state.WorkflowID, toolID)
	require.NoError(t, err)
	assert.Equal(t, state.WorkflowExecID, stored.WorkflowExecID)
}

func TestWorkflowRepo_CompleteWorkflow(t *testing.T) {
	repo, db := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))
	insertTask(t, db, &taskInsertOptions{
		ExecID:     state.WorkflowExecID,
		WorkflowID: state.WorkflowID,
		TaskID:     "t1",
		Status:     core.StatusSuccess,
		Output:     map[string]any{"value": 1},
	})

	transformer := func(st *workflow.State) (*core.Output, error) {
		out := core.Output{"tasks": len(st.Tasks)}
		return &out, nil
	}
	completed, err := repo.CompleteWorkflow(ctx, state.WorkflowExecID, transformer)
	require.NoError(t, err)
	assert.Equal(t, core.StatusSuccess, completed.Status)
	require.NotNil(t, completed.Output)
	assert.Equal(t, float64(1), (*completed.Output)["tasks"])
}

func TestWorkflowRepo_CompleteWorkflowTransformerError(t *testing.T) {
	repo, db := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))
	insertTask(t, db, &taskInsertOptions{
		ExecID:     state.WorkflowExecID,
		WorkflowID: state.WorkflowID,
		TaskID:     "t1",
		Status:     core.StatusSuccess,
	})

	transformer := func(*workflow.State) (*core.Output, error) {
		return nil, assert.AnError
	}
	completed, err := repo.CompleteWorkflow(ctx, state.WorkflowExecID, transformer)
	require.NoError(t, err)
	assert.Equal(t, core.StatusFailed, completed.Status)
	require.NotNil(t, completed.Error)
}

func TestWorkflowRepo_UsageJSON(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	state.Usage = usageSummary(10, 5)
	require.NoError(t, repo.UpsertState(ctx, state))

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	require.NotNil(t, stored.Usage)
	assert.Equal(t, 15, stored.Usage.Entries[0].TotalTokens)
}

func TestWorkflowRepo_InputJSON(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	input := core.Input{"prompt": "hello"}
	state.Input = &input
	require.NoError(t, repo.UpsertState(ctx, state))

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	require.NotNil(t, stored.Input)
	assert.Equal(t, "hello", (*stored.Input)["prompt"])
}

func TestWorkflowRepo_OutputJSON(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	output := core.Output{"result": "world"}
	state.Output = &output
	require.NoError(t, repo.UpsertState(ctx, state))

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	require.NotNil(t, stored.Output)
	assert.Equal(t, "world", (*stored.Output)["result"])
}

func TestWorkflowRepo_ErrorJSON(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	state.Error = core.NewError(assert.AnError, "TEST", map[string]any{"code": 500})
	require.NoError(t, repo.UpsertState(ctx, state))

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	require.NotNil(t, stored.Error)
	assert.Equal(t, "TEST", stored.Error.Code)
}

func TestWorkflowRepo_NullJSONFields(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	assert.Nil(t, stored.Usage)
	assert.Nil(t, stored.Input)
	assert.Nil(t, stored.Output)
	assert.Nil(t, stored.Error)
}

func TestWorkflowRepo_MergeUsage(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	state.Usage = usageSummary(5, 5)
	require.NoError(t, repo.UpsertState(ctx, state))
	require.NoError(t, repo.MergeUsage(ctx, state.WorkflowExecID, usageSummary(3, 2)))

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	require.NotNil(t, stored.Usage)
	assert.Equal(t, 15, stored.Usage.Entries[0].TotalTokens)
}

func TestWorkflowRepo_TransactionAtomic(t *testing.T) {
	repo, db := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))
	insertTask(t, db, &taskInsertOptions{
		ExecID:     state.WorkflowExecID,
		WorkflowID: state.WorkflowID,
		TaskID:     "t1",
		Status:     core.StatusSuccess,
	})

	completed, err := repo.CompleteWorkflow(ctx, state.WorkflowExecID, nil)
	require.NoError(t, err)
	assert.Equal(t, core.StatusSuccess, completed.Status)

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	assert.Equal(t, completed.Status, stored.Status)
}

func TestWorkflowRepo_RollbackOnError(t *testing.T) {
	repo, db := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))
	insertTask(t, db, &taskInsertOptions{
		ExecID:     state.WorkflowExecID,
		WorkflowID: state.WorkflowID,
		TaskID:     "t1",
		Status:     core.StatusRunning,
	})

	_, err := repo.CompleteWorkflow(ctx, state.WorkflowExecID, nil)
	require.ErrorIs(t, err, store.ErrWorkflowNotReady)

	stored, getErr := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, getErr)
	assert.Equal(t, core.StatusRunning, stored.Status)
}

func TestWorkflowRepo_HandleConcurrentUpdates(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))
	wg := sync.WaitGroup{}
	errCh := make(chan error, 5)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var lastErr error
			for attempt := 0; attempt < 100; attempt++ {
				err := repo.MergeUsage(ctx, state.WorkflowExecID, usageSummary(1, 1))
				if err == nil {
					errCh <- nil
					return
				}
				lastErr = err
				if isBusyError(err) {
					time.Sleep(time.Duration(attempt+1) * 5 * time.Millisecond)
					continue
				}
				errCh <- err
				return
			}
			errCh <- lastErr
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	require.NotNil(t, stored.Usage)
	assert.Equal(t, 6, stored.Usage.Entries[0].TotalTokens)
}

func TestWorkflowRepo_MissingWorkflow(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	_, err := repo.GetState(ctx, core.MustNewID())
	assert.ErrorIs(t, err, store.ErrWorkflowNotFound)
}

func TestWorkflowRepo_EmptyJSONArrays(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	empty := core.Output{}
	state.Output = &empty
	state.Usage = &usage.Summary{Entries: []usage.Entry{}}
	require.NoError(t, repo.UpsertState(ctx, state))

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	require.NotNil(t, stored.Output)
	assert.Empty(t, *stored.Output)
}

func TestWorkflowRepo_ComplexNestedJSON(t *testing.T) {
	repo, _ := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	nested := core.Output{"outer": map[string]any{"inner": []any{1, map[string]any{"deep": true}}}}
	state.Output = &nested
	require.NoError(t, repo.UpsertState(ctx, state))

	stored, err := repo.GetState(ctx, state.WorkflowExecID)
	require.NoError(t, err)
	require.NotNil(t, stored.Output)
	outer := (*stored.Output)["outer"].(map[string]any)
	inner := outer["inner"].([]any)
	assert.Equal(t, true, inner[1].(map[string]any)["deep"])
}

func TestWorkflowRepo_JSONConstraint(t *testing.T) {
	repo, db := setupWorkflowRepoTest(t)
	ctx := t.Context()

	state := sampleWorkflowState()
	require.NoError(t, repo.UpsertState(ctx, state))
	_, err := db.ExecContext(
		ctx,
		`UPDATE workflow_states SET usage = '{}' WHERE workflow_exec_id = ?`,
		state.WorkflowExecID.String(),
	)
	require.Error(t, err)
}

type taskInsertOptions struct {
	ExecID     core.ID
	WorkflowID string
	TaskID     string
	Status     core.StatusType
	AgentID    *string
	ToolID     *string
	ActionID   *string
	Output     map[string]any
}

func setupWorkflowRepoTest(t *testing.T) (*WorkflowRepo, *sql.DB) {
	t.Helper()
	ctx := t.Context()
	dbPath := t.TempDir() + "/workflow.db"

	require.NoError(t, ApplyMigrations(ctx, dbPath))
	store, err := NewStore(ctx, &Config{Path: dbPath})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, store.Close(ctx))
	})
	return NewWorkflowRepo(store.DB()).(*WorkflowRepo), store.DB()
}

func sampleWorkflowState() *workflow.State {
	return &workflow.State{
		WorkflowID:     "wf-" + core.MustNewID().String(),
		WorkflowExecID: core.MustNewID(),
		Status:         core.StatusRunning,
		Tasks:          make(map[string]*task.State),
	}
}

func usageSummary(prompt, completion int) *usage.Summary {
	summary := &usage.Summary{Entries: []usage.Entry{{
		Provider:         "openai",
		Model:            "gpt",
		PromptTokens:     prompt,
		CompletionTokens: completion,
	}}}
	summary.Sort()
	return summary
}

func insertTask(t *testing.T, db *sql.DB, opts *taskInsertOptions) {
	t.Helper()
	ctx := t.Context()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	component := core.ComponentTask
	if opts.AgentID != nil {
		component = core.ComponentAgent
	}
	if opts.ToolID != nil {
		component = core.ComponentTool
	}
	var output any
	if opts.Output != nil {
		data, err := json.Marshal(opts.Output)
		require.NoError(t, err)
		output = string(data)
	}
	_, err := db.ExecContext(ctx, `
        INSERT INTO task_states (
            component, status, task_exec_id, task_id, workflow_exec_id, workflow_id,
            execution_type, usage, agent_id, tool_id, action_id, parent_state_id,
            input, output, error, created_at, updated_at
        ) VALUES (?, ?, ?, ?, ?, ?, 'basic', NULL, ?, ?, ?, NULL, NULL, ?, NULL, ?, ?)
    `,
		string(component),
		string(opts.Status),
		core.MustNewID().String(),
		opts.TaskID,
		opts.ExecID.String(),
		opts.WorkflowID,
		opts.AgentID,
		opts.ToolID,
		opts.ActionID,
		output,
		now,
		now,
	)
	require.NoError(t, err)
}
