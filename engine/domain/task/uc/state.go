package taskuc

import (
	"fmt"

	"github.com/compozy/compozy/engine/domain/task"
	"github.com/compozy/compozy/engine/domain/workflow"
	"github.com/compozy/compozy/engine/stmanager"
	pbtask "github.com/compozy/compozy/pkg/pb/task"
)

func CreateInitState(
	stManager *stmanager.Manager,
	cmd *pbtask.CmdTaskDispatch,
	workflows []*workflow.Config,
) (*task.State, *task.Config, error) {
	// Parse task info from command
	info, err := task.InfoFromEvent(cmd)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse task payload info: %w", err)
	}

	// Find workflow state and config
	wConfig, err := workflow.FindConfig(workflows, info.WorkflowID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find workflow config: %w", err)
	}

	// Find task config and create state
	tConfig, err := task.FindConfig(wConfig.Tasks, info.TaskID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to find task config: %w", err)
	}

	// Create task state
	tState, err := stManager.CreateTaskState(info, tConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create task state: %w", err)
	}

	// Validate task input
	if err := tConfig.ValidateParams(*tState.Input); err != nil {
		return nil, nil, fmt.Errorf("failed to validate input: %w", err)
	}
	return tState, tConfig, nil
}
