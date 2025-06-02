package temporal

import (
	"context"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/task"
	taskuc "github.com/compozy/compozy/engine/task/uc"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/pb"
)

// -----------------------------------------------------------------------------
// Activity Definitions
// -----------------------------------------------------------------------------

type Activities struct {
	taskRepo  task.Repository
	agentRepo agent.Repository
	toolRepo  tool.Repository
}

func NewActivities(
	taskRepo task.Repository,
	agentRepo agent.Repository,
	toolRepo tool.Repository,
) *Activities {
	return &Activities{
		taskRepo:  taskRepo,
		agentRepo: agentRepo,
		toolRepo:  toolRepo,
	}
}

// TaskExecuteActivity wraps the existing task execution use case
func (a *Activities) TaskExecuteActivity(ctx context.Context, cmd *pb.CmdTaskExecute) error {
	uc := taskuc.NewHandleExecute(nil, a.taskRepo)
	// The UC's Execute method handles the actual task execution
	return uc.Execute(ctx, nil)
}

// AgentExecuteActivity placeholder - to be implemented when agent execution is migrated
func (a *Activities) AgentExecuteActivity(ctx context.Context, cmd *pb.CmdAgentExecute) error {
	// TODO: Implement when agent execution use case is available
	return nil
}

// ToolExecuteActivity placeholder - to be implemented when tool execution is migrated
func (a *Activities) ToolExecuteActivity(ctx context.Context, cmd *pb.CmdToolExecute) error {
	// TODO: Implement when tool execution use case is available
	return nil
}
