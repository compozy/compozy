package tasks

import (
	"context"

	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/contracts"
	taskscore "github.com/compozy/compozy/engine/task/tasks/core"
	"github.com/compozy/compozy/engine/task/tasks/shared"
)

// Factory provides unified creation methods for all tasks components
type Factory interface {
	// Task normalizer creation
	CreateNormalizer(ctx context.Context, taskType task.Type) (contracts.TaskNormalizer, error)

	// Component normalizers
	CreateAgentNormalizer() *taskscore.AgentNormalizer
	CreateToolNormalizer() *taskscore.ToolNormalizer
	CreateSuccessTransitionNormalizer() *taskscore.SuccessTransitionNormalizer
	CreateErrorTransitionNormalizer() *taskscore.ErrorTransitionNormalizer
	CreateOutputTransformer() *taskscore.OutputTransformer

	// Response handler creation
	CreateResponseHandler(ctx context.Context, taskType task.Type) (shared.TaskResponseHandler, error)

	// Domain service creation
	CreateCollectionExpander(ctx context.Context) shared.CollectionExpander

	// Infrastructure service creation
	CreateTaskConfigRepository(
		configStore taskscore.ConfigStore,
		cwd *enginecore.PathCWD,
	) (shared.TaskConfigRepository, error)
}
