package tasks

import (
	"context"
	"fmt"

	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/task/tasks/aggregate"
	"github.com/compozy/compozy/engine/task/tasks/basic"
	"github.com/compozy/compozy/engine/task/tasks/collection"
	"github.com/compozy/compozy/engine/task/tasks/composite"
	"github.com/compozy/compozy/engine/task/tasks/contracts"
	"github.com/compozy/compozy/engine/task/tasks/core"
	"github.com/compozy/compozy/engine/task/tasks/memory"
	"github.com/compozy/compozy/engine/task/tasks/parallel"
	"github.com/compozy/compozy/engine/task/tasks/router"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/engine/task/tasks/signal"
	"github.com/compozy/compozy/engine/task/tasks/wait"
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
func NewFactory(ctx context.Context, config *FactoryConfig) (Factory, error) {
	if config.TemplateEngine == nil {
		return nil, fmt.Errorf("template engine is required")
	}
	if config.EnvMerger == nil {
		return nil, fmt.Errorf("env merger is required")
	}
	builder, err := shared.NewContextBuilder(ctx)
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
func (f *DefaultNormalizerFactory) CreateNormalizer(
	ctx context.Context,
	taskType task.Type,
) (contracts.TaskNormalizer, error) {
	switch taskType {
	case task.TaskTypeBasic, "": // Empty type defaults to basic
		return basic.NewNormalizer(ctx, f.templateEngine), nil
	case task.TaskTypeParallel:
		return parallel.NewNormalizer(ctx, f.templateEngine, f.contextBuilder, f), nil
	case task.TaskTypeCollection:
		return collection.NewNormalizer(ctx, f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeRouter:
		return router.NewNormalizer(ctx, f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeWait:
		return wait.NewNormalizer(ctx, f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeAggregate:
		return aggregate.NewNormalizer(ctx, f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeComposite:
		return composite.NewNormalizer(ctx, f.templateEngine, f.contextBuilder, f), nil
	case task.TaskTypeSignal:
		return signal.NewNormalizer(ctx, f.templateEngine, f.contextBuilder), nil
	case task.TaskTypeMemory:
		return memory.NewNormalizer(ctx, f.templateEngine), nil
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
	parentStatusManager := f.createParentStatusManager(ctx)
	outputTransformer := f.createOutputTransformer()
	baseHandler := shared.NewBaseResponseHandler(
		f.templateEngine,
		f.contextBuilder,
		parentStatusManager,
		f.workflowRepo,
		f.taskRepo,
		outputTransformer,
	)
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
func (f *DefaultNormalizerFactory) CreateCollectionExpander(ctx context.Context) shared.CollectionExpander {
	normalizer := collection.NewNormalizer(ctx, f.templateEngine, f.contextBuilder)
	configBuilder := collection.NewConfigBuilder(f.templateEngine)
	return collection.NewExpander(normalizer, f.contextBuilder, configBuilder)
}

// CreateTaskConfigRepository creates a task configuration repository
func (f *DefaultNormalizerFactory) CreateTaskConfigRepository(
	configStore core.ConfigStore,
	cwd *enginecore.PathCWD,
) (shared.TaskConfigRepository, error) {
	if cwd == nil {
		return nil, fmt.Errorf("cwd cannot be nil: pass project CWD to CreateTaskConfigRepository")
	}
	return core.NewTaskConfigRepository(configStore, cwd), nil
}

// -----------------------------------------------------------------------------
// Helper Methods
// -----------------------------------------------------------------------------

// createParentStatusManager creates a parent status manager
func (f *DefaultNormalizerFactory) createParentStatusManager(ctx context.Context) shared.ParentStatusManager {
	if f.taskRepo != nil {
		return shared.NewParentStatusManager(ctx, f.taskRepo)
	}
	// NOTE: The shared base response handler tolerates a nil status manager for tests.
	return nil
}

// createOutputTransformer creates an output transformer adapter
func (f *DefaultNormalizerFactory) createOutputTransformer() shared.OutputTransformer {
	transformer := core.NewOutputTransformer(f.templateEngine)
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
	if config.GetOutputs() == nil || state.Output == nil {
		if state.Output != nil {
			return state.Output.AsMap(), nil
		}
		return make(map[string]any), nil
	}
	workflowState, err := a.workflowRepo.GetState(ctx, state.WorkflowExecID)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow state for output transformation: %w", err)
	}
	normCtx := a.contextBuilder.BuildContext(ctx, workflowState, workflowConfig, config)
	transformedOutput, err := a.transformer.TransformOutput(
		ctx,
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
