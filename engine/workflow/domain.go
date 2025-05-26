package workflow

import (
	"fmt"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/pb"
	"github.com/compozy/compozy/pkg/tplengine"
)

// -----------------------------------------------------------------------------
// StoreKey
// -----------------------------------------------------------------------------

type StoreKey struct {
	WorkflowExecID core.ID
}

func NewStoreKey(workflowExecID core.ID) StoreKey {
	return StoreKey{
		WorkflowExecID: workflowExecID,
	}
}

func (s *StoreKey) String() string {
	return fmt.Sprintf("workflow:%s", s.WorkflowExecID)
}

func (s *StoreKey) Bytes() []byte {
	return []byte(s.String())
}

// -----------------------------------------------------------------------------
// RequestData
// -----------------------------------------------------------------------------

type RequestData struct {
	*pb.WorkflowMetadata `json:"metadata"`
	ParentInput          *core.Input  `json:"parent_input"`
	Input                *core.Input  `json:"input"`
	ProjectEnv           *core.EnvMap `json:"project_env"`
	WorkflowEnv          *core.EnvMap `json:"workflow_env"`
}

func NewRequestData(
	metadata *pb.WorkflowMetadata,
	parentInput, input *core.Input,
	projectEnv, workflowEnv *core.EnvMap,
) (*RequestData, error) {
	return &RequestData{
		ParentInput:      parentInput,
		Input:            input,
		ProjectEnv:       projectEnv,
		WorkflowEnv:      workflowEnv,
		WorkflowMetadata: metadata,
	}, nil
}

func (c *RequestData) GetWorkflowExecID() core.ID {
	return core.ID(c.WorkflowExecId)
}

func (c *RequestData) ToStoreKey() StoreKey {
	return StoreKey{
		WorkflowExecID: core.ID(c.WorkflowExecId),
	}
}

// -----------------------------------------------------------------------------
// Execution
// -----------------------------------------------------------------------------

type Execution struct {
	*core.BaseExecution
	RequestData *RequestData `json:"request_data,omitempty"`
}

func NewExecution(data *RequestData) (*Execution, error) {
	env, err := data.ProjectEnv.Merge(*data.WorkflowEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge env: %w", err)
	}
	baseExec := core.NewBaseExecution(
		data.WorkflowId,
		core.ID(data.WorkflowExecId),
		// TODO: For now, the parent input is the input
		data.ParentInput,
		data.Input,
		nil,
		&env,
		nil,
	)
	exec := &Execution{
		BaseExecution: baseExec,
		RequestData:   data,
	}
	normalizer := tplengine.NewNormalizer()
	if err := normalizer.ParseExecution(exec); err != nil {
		return nil, fmt.Errorf("failed to parse execution: %w", err)
	}
	return exec, nil
}

func (e *Execution) StoreKey() []byte {
	storeKey := e.RequestData.ToStoreKey()
	return storeKey.Bytes()
}

func (e *Execution) GetID() core.ID {
	return e.WorkflowExecID
}

func (e *Execution) GetComponentID() string {
	return e.WorkflowID
}
