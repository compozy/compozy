package task2

import (
	"context"
	"fmt"

	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task2/aggregate"
	"github.com/compozy/compozy/engine/task2/basic"
	"github.com/compozy/compozy/engine/task2/collection"
	"github.com/compozy/compozy/engine/task2/composite"
	"github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/parallel"
	"github.com/compozy/compozy/engine/task2/router"
	"github.com/compozy/compozy/engine/task2/shared"
	"github.com/compozy/compozy/engine/task2/signal"
	"github.com/compozy/compozy/engine/task2/wait"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/tplengine"
)

// DefaultNormalizerFactory creates appropriate normalizers based on task type
type DefaultNormalizerFactory struct {
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
	envMerger      *core.EnvMerger
	// Optional dependencies for extended functionality
	workflowRepo workflow.Repository
	taskRepo     task.Repository
}

// factoryAdapter bridges Factory to shared.NormalizerFactoryInterface
// This is a minimal adapter needed only for parallel/composite normalizers
type factoryAdapter struct {
	factory Factory
}

func (a *factoryAdapter) CreateNormalizer(taskType task.Type) (shared.TaskNormalizerInterface, error) {
	normalizer, err := a.factory.CreateNormalizer(taskType)
	if err != nil {
		return nil, err
	}
	// Since both interfaces have the same methods, we can use a type assertion
	// This works because all our normalizers implement both interfaces
	return normalizer, nil
}

// FactoryConfig contains configuration options for the extended factory
type FactoryConfig struct {
	TemplateEngine *tplengine.TemplateEngine
	EnvMerger      *core.EnvMerger
	WorkflowRepo   workflow.Repository
	TaskRepo       task.Repository
}

// NewFactory creates a new unified factory
func NewFactory(engine *tplengine.TemplateEngine, merger *core.EnvMerger) (Factory, error) {
	builder, err := shared.NewContextBuilder()
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	return &DefaultNormalizerFactory{
		templateEngine: engine,
		contextBuilder: builder,
		envMerger:      merger,
	}, nil
}

// NewFactoryWithConfig creates a new factory with full dependency injection
func NewFactoryWithConfig(config *FactoryConfig) (Factory, error) {
	if config.TemplateEngine == nil {
		return nil, fmt.Errorf("template engine is required")
	}
	if config.EnvMerger == nil {
		return nil, fmt.Errorf("env merger is required")
	}
	builder, err := shared.NewContextBuilder()
	if err != nil {
		return nil, fmt.Errorf("failed to create context builder: %w", err)
	}
	return &DefaultNormalizerFactory{
		templateEngine: config.TemplateEngine,
		contextBuilder: builder,
		envMerger:      config.EnvMerger,
		workflowRepo:   config.WorkflowRepo,
		taskRepo:       config.TaskRepo,
	}, nil
}

// CreateNormalizer creates a normalizer for the given task type
func (f *DefaultNormalizerFactory) CreateNormalizer(taskType task.Type) (TaskNormalizer, error) {
	switch taskType {
	case task.TaskTypeBasic, "": // Empty type defaults to basic
		return basic.NewNormalizer(f.templateEngine), nil
	case task.TaskTypeParallel:
		adapter := &factoryAdapter{factory: f}
		return parallel.NewNormalizer(f.templateEngine, f.contextBuilder, adapter), nil
	case task.TaskTypeCollection:
		return collection.NewNormalizer(f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeRouter:
		return router.NewNormalizer(f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeWait:
		return wait.NewNormalizer(f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeAggregate:
		return aggregate.NewNormalizer(f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeComposite:
		adapter := &factoryAdapter{factory: f}
		return composite.NewNormalizer(f.templateEngine, f.contextBuilder, adapter), nil
	case task.TaskTypeSignal:
		return signal.NewNormalizer(f.templateEngine, f.contextBuilder), nil
	default:
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}
}

// Component normalizers from core package

// CreateAgentNormalizer creates a normalizer for agent components
func (f *DefaultNormalizerFactory) CreateAgentNormalizer() *core.AgentNormalizer {
	return core.NewAgentNormalizer(f.templateEngine, f.envMerger)
}

// CreateToolNormalizer creates a normalizer for tool components
func (f *DefaultNormalizerFactory) CreateToolNormalizer() *core.ToolNormalizer {
	return core.NewToolNormalizer(f.templateEngine, f.envMerger)
}

// CreateSuccessTransitionNormalizer creates a normalizer for success transitions
func (f *DefaultNormalizerFactory) CreateSuccessTransitionNormalizer() *core.SuccessTransitionNormalizer {
	return core.NewSuccessTransitionNormalizer(f.templateEngine)
}

// CreateErrorTransitionNormalizer creates a normalizer for error transitions
func (f *DefaultNormalizerFactory) CreateErrorTransitionNormalizer() *core.ErrorTransitionNormalizer {
	return core.NewErrorTransitionNormalizer(f.templateEngine)
}

// CreateOutputTransformer creates an output transformer
func (f *DefaultNormalizerFactory) CreateOutputTransformer() *core.OutputTransformer {
	return core.NewOutputTransformer(f.templateEngine)
}

// -----------------------------------------------------------------------------
// Extended Factory Methods
// -----------------------------------------------------------------------------

// CreateResponseHandler creates a response handler for the given task type
func (f *DefaultNormalizerFactory) CreateResponseHandler(taskType task.Type) (shared.TaskResponseHandler, error) {
	// Create dependencies
	parentStatusManager := f.createParentStatusManager()
	outputTransformer := f.createOutputTransformer()

	// Create base handler with all dependencies
	baseHandler := shared.NewBaseResponseHandler(
		f.templateEngine,
		f.contextBuilder,
		parentStatusManager,
		f.workflowRepo,
		f.taskRepo,
		outputTransformer,
	)

	// Create task-specific handler
	switch taskType {
	case task.TaskTypeBasic:
		return basic.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
	case task.TaskTypeParallel:
		return parallel.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
	case task.TaskTypeCollection:
		return collection.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
	case task.TaskTypeComposite:
		return composite.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
	case task.TaskTypeRouter:
		return router.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
	case task.TaskTypeWait:
		return wait.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
	case task.TaskTypeSignal:
		return signal.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
	case task.TaskTypeAggregate:
		return aggregate.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler), nil
	default:
		return nil, fmt.Errorf("unsupported task type for response handler: %s", taskType)
	}
}

// CreateCollectionExpander creates a collection expander service
func (f *DefaultNormalizerFactory) CreateCollectionExpander() shared.CollectionExpander {
	normalizer := collection.NewNormalizer(f.templateEngine, f.contextBuilder)
	configBuilder := collection.NewConfigBuilder(f.templateEngine)
	return collection.NewExpander(normalizer, f.contextBuilder, configBuilder)
}

// CreateTaskConfigRepository creates a task configuration repository
func (f *DefaultNormalizerFactory) CreateTaskConfigRepository(
	configStore core.ConfigStore,
) shared.TaskConfigRepository {
	// Create PathCWD with empty value - will be set during execution
	cwd, err := enginecore.CWDFromPath("")
	if err != nil {
		// Use empty CWD if path creation fails
		cwd = &enginecore.PathCWD{}
	}
	return core.NewTaskConfigRepository(configStore, cwd)
}

// -----------------------------------------------------------------------------
// Helper Methods
// -----------------------------------------------------------------------------

// createParentStatusManager creates a parent status manager
func (f *DefaultNormalizerFactory) createParentStatusManager() shared.ParentStatusManager {
	// Use injected taskRepo if available, otherwise return nil
	// The BaseResponseHandler will handle nil gracefully
	if f.taskRepo != nil {
		return shared.NewParentStatusManager(f.taskRepo)
	}
	return nil
}

// createOutputTransformer creates an output transformer adapter
func (f *DefaultNormalizerFactory) createOutputTransformer() shared.OutputTransformer {
	// Create an adapter that implements the shared.OutputTransformer interface
	return &outputTransformerAdapter{
		templateEngine: f.templateEngine,
		contextBuilder: f.contextBuilder,
	}
}

// outputTransformerAdapter adapts the output transformation to the shared interface
type outputTransformerAdapter struct {
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
}

// TransformOutput implements shared.OutputTransformer
func (a *outputTransformerAdapter) TransformOutput(
	_ context.Context,
	state *task.State,
	_ *task.Config,
	_ *workflow.Config,
) (map[string]any, error) {
	// For now, return state output as-is
	// The actual output transformation logic is handled by task-specific handlers
	if state.Output != nil {
		return state.Output.AsMap(), nil
	}

	return make(map[string]any), nil
}
