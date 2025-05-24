package taskuc

import (
	"fmt"

	"github.com/compozy/compozy/engine/domain/task"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/stmanager"
	"github.com/compozy/compozy/pkg/pb"
)

func CreateInitState(
	stManager *stmanager.Manager,
	cmd *pb.CmdTaskDispatch,
	workflows []*workflow.Config,
) error {
	// Find workflow state and config
	metadata := cmd.GetMetadata()
	wConfig, err := workflow.FindConfig(workflows, metadata.WorkflowId)
	if err != nil {
		return fmt.Errorf("failed to find workflow config: %w", err)
	}

	// Find task config and create state
	tConfig, err := task.FindConfig(wConfig.Tasks, metadata.TaskId)
	if err != nil {
		return fmt.Errorf("failed to find task config: %w", err)
	}

	// Create task state
	_, err = stManager.CreateTaskState(metadata, tConfig)
	if err != nil {
		return fmt.Errorf("failed to create task state: %w", err)
	}
	return nil
}
