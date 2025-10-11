package directexec

import (
	"context"
	"sync"
	"testing"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/infra/server/router/routertest"
	"github.com/compozy/compozy/engine/runtime/toolenv"
	"github.com/compozy/compozy/engine/runtime/toolenvstate"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/testutil"
	wf "github.com/compozy/compozy/engine/workflow"
	"github.com/stretchr/testify/require"
)

type stubWorkflowRepo struct {
	mu     sync.RWMutex
	states map[core.ID]*wf.State
}

func newStubWorkflowRepo() *stubWorkflowRepo {
	return &stubWorkflowRepo{states: make(map[core.ID]*wf.State)}
}

func (s *stubWorkflowRepo) storeCopy(state *wf.State) {
	if state == nil {
		return
	}
	clone := *state
	s.states[state.WorkflowExecID] = &clone
}

func (s *stubWorkflowRepo) ListStates(context.Context, *wf.StateFilter) ([]*wf.State, error) {
	return nil, nil
}

func (s *stubWorkflowRepo) UpsertState(_ context.Context, state *wf.State) error {
	s.mu.Lock()
	s.storeCopy(state)
	s.mu.Unlock()
	return nil
}

func (s *stubWorkflowRepo) UpdateStatus(context.Context, core.ID, core.StatusType) error {
	return nil
}

func (s *stubWorkflowRepo) GetState(_ context.Context, execID core.ID) (*wf.State, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored, ok := s.states[execID]
	if !ok {
		return nil, nil
	}
	clone := *stored
	return &clone, nil
}

func (s *stubWorkflowRepo) GetStateByID(context.Context, string) (*wf.State, error) {
	return nil, nil
}

func (s *stubWorkflowRepo) GetStateByTaskID(context.Context, string, string) (*wf.State, error) {
	return nil, nil
}

func (s *stubWorkflowRepo) GetStateByAgentID(context.Context, string, string) (*wf.State, error) {
	return nil, nil
}

func (s *stubWorkflowRepo) GetStateByToolID(context.Context, string, string) (*wf.State, error) {
	return nil, nil
}

func (s *stubWorkflowRepo) CompleteWorkflow(_ context.Context, _ core.ID, _ wf.OutputTransformer) (*wf.State, error) {
	return nil, nil
}

func TestPrepareExecutionPlanNormalizesAgentInput(t *testing.T) {
	ctx := context.Background()
	state := routertest.NewTestAppState(t)
	toolenvstate.Store(state, toolenv.New(nil, nil, nil))
	taskRepo := testutil.NewInMemoryRepo()
	workflowRepo := newStubWorkflowRepo()
	executor, err := NewDirectExecutor(state, taskRepo, workflowRepo)
	require.NoError(t, err)
	impl, ok := executor.(*directExecutor)
	require.True(t, ok)
	cfgMap := map[string]any{
		"id":   "direct-task",
		"type": task.TaskTypeBasic,
		"agent": map[string]any{
			"id": "echo-agent",
			"with": map[string]any{
				"echo": "{{ .input.message }}",
			},
			"actions": []any{
				map[string]any{"id": "acknowledge"},
			},
		},
		"action": "acknowledge",
		"with": map[string]any{
			"message": "Hello",
		},
	}
	taskCfg := &task.Config{}
	require.NoError(t, taskCfg.FromMap(cfgMap))
	execID := core.MustNewID()
	plan, err := impl.prepareExecutionPlan(ctx, taskCfg, &ExecMetadata{Component: core.ComponentAgent}, execID)
	require.NoError(t, err)
	require.NotNil(t, plan)
	require.NotNil(t, plan.config.Agent)
	require.NotNil(t, plan.config.Agent.With)
	resolved, ok := (*plan.config.Agent.With)["echo"].(string)
	require.True(t, ok)
	require.Equal(t, "Hello", resolved)
	require.NotNil(t, plan.workflowConfig)
	require.Equal(t, plan.meta.WorkflowID, plan.workflowConfig.ID)
}
