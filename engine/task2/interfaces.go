package task2

import (
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/shared"
)

// Factory provides unified creation methods for all task2 components
type Factory interface {
	// Task normalizer creation
	CreateNormalizer(taskType task.Type) (contracts.TaskNormalizer, error)

	// Component normalizers
	CreateAgentNormalizer() *core.AgentNormalizer
	CreateToolNormalizer() *core.ToolNormalizer
	CreateSuccessTransitionNormalizer() *core.SuccessTransitionNormalizer
	CreateErrorTransitionNormalizer() *core.ErrorTransitionNormalizer
	CreateOutputTransformer() *core.OutputTransformer

	// Response handler creation
	CreateResponseHandler(taskType task.Type) (shared.TaskResponseHandler, error)

	// Domain service creation
	CreateCollectionExpander() shared.CollectionExpander

	// Infrastructure service creation
	CreateTaskConfigRepository(configStore core.ConfigStore) shared.TaskConfigRepository
}
