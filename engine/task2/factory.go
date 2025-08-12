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
	"github.com/compozy/compozy/engine/task2/contracts"
	"github.com/compozy/compozy/engine/task2/core"
	"github.com/compozy/compozy/engine/task2/memory"
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

// FactoryConfig contains configuration options for the extended factory
type FactoryConfig struct {
	TemplateEngine *tplengine.TemplateEngine
	EnvMerger      *core.EnvMerger
	WorkflowRepo   workflow.Repository
	TaskRepo       task.Repository
}

// NewFactoryWithConfig creates a new factory with full dependency injection
func NewFactory(config *FactoryConfig) (Factory, error) {
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
func (f *DefaultNormalizerFactory) CreateNormalizer(taskType task.Type) (contracts.TaskNormalizer, error) {
	switch taskType {
	case task.TaskTypeBasic, "": // Empty type defaults to basic
		return basic.NewNormalizer(f.templateEngine), nil
	case task.TaskTypeParallel:
		return parallel.NewNormalizer(f.templateEngine, f.contextBuilder, f), nil
	case task.TaskTypeCollection:
		return collection.NewNormalizer(f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeRouter:
		return router.NewNormalizer(f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeWait:
		return wait.NewNormalizer(f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeAggregate:
		return aggregate.NewNormalizer(f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeComposite:
		return composite.NewNormalizer(f.templateEngine, f.contextBuilder, f), nil
	case task.TaskTypeSignal:
		return signal.NewNormalizer(f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeMemory:
		return memory.NewNormalizer(f.templateEngine), nil
	default:
		return nil, fmt.Errorf("unsupported task type: %s", taskType)
	}
}

// Component normalizers from core package

// CreateAgentNormalizer creates a normalizer for agent components
func (f *DefaultNormalizerFactory) CreateAgentNormalizer() *core.AgentNormalizer {
	return core.NewAgentNormalizer(f.envMerger)
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
func (f *DefaultNormalizerFactory) CreateResponseHandler(
	ctx context.Context,
	taskType task.Type,
) (shared.TaskResponseHandler, error) {
	// Create dependencies
	parentStatusManager := f.createParentStatusManager(ctx)
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
		return basic.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler)
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
	case task.TaskTypeMemory:
		return memory.NewResponseHandler(f.templateEngine, f.contextBuilder, baseHandler)
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
) (shared.TaskConfigRepository, error) {
	cwd, err := enginecore.CWDFromPath("")
	if err != nil {
		return nil, fmt.Errorf("failed to create CWD from path: %w", err)
	}
	return core.NewTaskConfigRepository(configStore, cwd), nil
}

// -----------------------------------------------------------------------------
// Helper Methods
// -----------------------------------------------------------------------------

// createParentStatusManager creates a parent status manager
func (f *DefaultNormalizerFactory) createParentStatusManager(ctx context.Context) shared.ParentStatusManager {
	// Use injected taskRepo if available, otherwise return nil
	// The BaseResponseHandler will handle nil gracefully
	if f.taskRepo != nil {
		return shared.NewParentStatusManager(ctx, f.taskRepo)
	}
	return nil
}

// createOutputTransformer creates an output transformer adapter
func (f *DefaultNormalizerFactory) createOutputTransformer() shared.OutputTransformer {
	// Create the actual output transformer
	transformer := core.NewOutputTransformer(f.templateEngine)
	// Create an adapter that implements the shared.OutputTransformer interface
	return &outputTransformerAdapter{
		templateEngine: f.templateEngine,
		contextBuilder: f.contextBuilder,
		transformer:    transformer,
		workflowRepo:   f.workflowRepo,
	}
}

// outputTransformerAdapter adapts the output transformation to the shared interface
type outputTransformerAdapter struct {
	templateEngine *tplengine.TemplateEngine
	contextBuilder *shared.ContextBuilder
	transformer    *core.OutputTransformer
	workflowRepo   workflow.Repository
}

// TransformOutput implements shared.OutputTransformer
func (a *outputTransformerAdapter) TransformOutput(
	ctx context.Context,
	state *task.State,
	config *task.Config,
	workflowConfig *workflow.Config,
) (map[string]any, error) {
	// If no outputs configuration, return state output as-is
	if config.GetOutputs() == nil || state.Output == nil {
		if state.Output != nil {
			return state.Output.AsMap(), nil
		}
		return make(map[string]any), nil
	}
	// Get the actual workflow state
	workflowState, err := a.workflowRepo.GetState(ctx, state.WorkflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state for output transformation: %w", err)
	}
	// Build normalization context for transformation
	normCtx := a.contextBuilder.BuildContext(workflowState, workflowConfig, config)
	// Apply output transformation
	transformedOutput, err := a.transformer.TransformOutput(
		state.Output,
		config.GetOutputs(),
		normCtx,
		config,
	)
	if err != nil {
		return nil, err
	}
	if transformedOutput != nil {
		return transformedOutput.AsMap(), nil
	}
	return make(map[string]any), nil
}
