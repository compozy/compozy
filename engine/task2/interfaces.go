package task2

import (
	"context"

	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	task2core "github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
)

// Factory provides unified creation methods for all task2 components
type Factory interface {
	// Task normalizer creation
	CreateNormalizer(taskType task.Type) (contracts.TaskNormalizer, error)

	// Component normalizers
	CreateAgentNormalizer() *task2core.AgentNormalizer
	CreateToolNormalizer() *task2core.ToolNormalizer
	CreateSuccessTransitionNormalizer() *task2core.SuccessTransitionNormalizer
	CreateErrorTransitionNormalizer() *task2core.ErrorTransitionNormalizer
	CreateOutputTransformer() *task2core.OutputTransformer

	// Response handler creation
	CreateResponseHandler(ctx context.Context, taskType task.Type) (shared.TaskResponseHandler, error)

	// Domain service creation
	CreateCollectionExpander() shared.CollectionExpander

	// Infrastructure service creation
	CreateTaskConfigRepository(
		configStore task2core.ConfigStore,
		cwd *enginecore.PathCWD,
	) (shared.TaskConfigRepository, error)
}
