package task2

import (
	"fmt"

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
)

// DefaultNormalizerFactory creates appropriate normalizers based on task type
type DefaultNormalizerFactory struct {
	templateEngine shared.TemplateEngine
	contextBuilder *shared.ContextBuilder
	envMerger      *core.EnvMerger
}

// sharedNormalizerFactoryAdapter wraps DefaultNormalizerFactory to implement shared.NormalizerFactory
type sharedNormalizerFactoryAdapter struct {
	factory *DefaultNormalizerFactory
}

func (a *sharedNormalizerFactoryAdapter) CreateNormalizer(taskType string) (shared.TaskNormalizer, error) {
	normalizer, err := a.factory.CreateNormalizer(task.Type(taskType))
	if err != nil {
		return nil, err
	}
	return &taskNormalizerAdapter{normalizer: normalizer}, nil
}

// taskNormalizerAdapter wraps TaskNormalizer to implement shared.TaskNormalizer
type taskNormalizerAdapter struct {
	normalizer TaskNormalizer
}

func (a *taskNormalizerAdapter) Normalize(config any, ctx *shared.NormalizationContext) error {
	taskConfig, ok := config.(*task.Config)
	if !ok {
		return fmt.Errorf("expected *task.Config, got %T", config)
	}
	return a.normalizer.Normalize(taskConfig, ctx)
}

func (a *taskNormalizerAdapter) Type() string {
	return string(a.normalizer.Type())
}

// NewNormalizerFactory creates a new normalizer factory
func NewNormalizerFactory(engine shared.TemplateEngine, merger *core.EnvMerger) (NormalizerFactory, error) {
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

// CreateNormalizer creates a normalizer for the given task type
func (f *DefaultNormalizerFactory) CreateNormalizer(taskType task.Type) (TaskNormalizer, error) {
	switch taskType {
	case task.TaskTypeBasic, "": // Empty type defaults to basic
		return basic.NewNormalizer(f.templateEngine), nil
	case task.TaskTypeParallel:
		adapter := &sharedNormalizerFactoryAdapter{factory: f}
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
		adapter := &sharedNormalizerFactoryAdapter{factory: f}
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
