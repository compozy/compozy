package loop

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/tools"
)

type actionRegistryConfig struct {
	runtime       tools.Registry
	runAgent      ActionExecutor
	runLoop       ActionExecutor
	transform     ActionExecutor
	goal          ActionExecutor
	sessionBinder ActionSessionBinder
	loopStarter   ActionLoopStarter
	eventReader   ActionEventRangeReader
	channel       ChannelResultHarvester
	channelStore  ChannelResultConversationStore
}

// ActionRegistryOption configures the loop action registry.
type ActionRegistryOption func(*actionRegistryConfig)

// WithActionRunAgentExecutor overrides the reserved run-agent executor.
func WithActionRunAgentExecutor(executor ActionExecutor) ActionRegistryOption {
	return func(cfg *actionRegistryConfig) {
		cfg.runAgent = executor
	}
}

// WithActionRunLoopExecutor overrides the reserved run-loop executor.
func WithActionRunLoopExecutor(executor ActionExecutor) ActionRegistryOption {
	return func(cfg *actionRegistryConfig) {
		cfg.runLoop = executor
	}
}

// WithActionTransformExecutor overrides the reserved transform executor.
func WithActionTransformExecutor(executor ActionExecutor) ActionRegistryOption {
	return func(cfg *actionRegistryConfig) {
		cfg.transform = executor
	}
}

// WithActionGoalExecutor wires the Goal convergence executor without importing the child package.
func WithActionGoalExecutor(executor ActionExecutor) ActionRegistryOption {
	return func(cfg *actionRegistryConfig) {
		cfg.goal = executor
	}
}

// WithActionSessionBinder wires ACP session binding for run-agent.
func WithActionSessionBinder(binder ActionSessionBinder) ActionRegistryOption {
	return func(cfg *actionRegistryConfig) {
		cfg.sessionBinder = binder
	}
}

// WithActionLoopStarter wires child loop starts for run-loop.
func WithActionLoopStarter(starter ActionLoopStarter) ActionRegistryOption {
	return func(cfg *actionRegistryConfig) {
		cfg.loopStarter = starter
	}
}

// WithActionEventRangeReader wires async event-range harvest.
func WithActionEventRangeReader(reader ActionEventRangeReader) ActionRegistryOption {
	return func(cfg *actionRegistryConfig) {
		cfg.eventReader = reader
	}
}

// WithActionChannelResultHarvester wires channel_result harvest.
func WithActionChannelResultHarvester(harvester ChannelResultHarvester) ActionRegistryOption {
	return func(cfg *actionRegistryConfig) {
		cfg.channel = harvester
	}
}

// WithActionChannelResultStore wires durable conversation storage for channel_result harvest.
func WithActionChannelResultStore(conversations ChannelResultConversationStore) ActionRegistryOption {
	return func(cfg *actionRegistryConfig) {
		cfg.channelStore = conversations
	}
}

// ActionRegistry resolves open action kinds through reserved executors or RuntimeRegistry.
type ActionRegistry struct {
	runtime   tools.Registry
	runAgent  ActionExecutor
	runLoop   ActionExecutor
	transform ActionExecutor
	goal      ActionExecutor
	events    ActionEventRangeReader
	channel   ChannelResultHarvester
}

// NewActionRegistry creates the runtime action adapter over the existing tools registry.
func NewActionRegistry(runtime tools.Registry, opts ...ActionRegistryOption) (*ActionRegistry, error) {
	if runtime == nil {
		return nil, reasonError(
			ReasonCodeActionDependencyMissing,
			ErrActionDependencyMissing,
			map[string]string{actionDependencyMetaKey: "runtime_registry"},
		)
	}
	cfg := &actionRegistryConfig{runtime: runtime}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	if cfg.transform == nil {
		cfg.transform = TransformActionExecutor{}
	}
	if cfg.runLoop == nil {
		cfg.runLoop = &RunLoopActionExecutor{starter: cfg.loopStarter}
	}
	if cfg.runAgent == nil {
		cfg.runAgent = &RunAgentActionExecutor{binder: cfg.sessionBinder}
	}
	if cfg.channel == nil && cfg.channelStore != nil {
		channel, err := NewStoreChannelResultHarvester(cfg.channelStore)
		if err != nil {
			return nil, err
		}
		cfg.channel = channel
	}
	return &ActionRegistry{
		runtime:   runtime,
		runAgent:  cfg.runAgent,
		runLoop:   cfg.runLoop,
		transform: cfg.transform,
		goal:      cfg.goal,
		events:    cfg.eventReader,
		channel:   cfg.channel,
	}, nil
}

// Resolve returns the executor for a scoped action kind. Reserved kinds win before
// falling through to the existing RuntimeRegistry.
func (r *ActionRegistry) Resolve(ctx context.Context, scope tools.Scope, kind string) (ActionExecutor, error) {
	return r.resolve(ctx, scope, kind, nil, false)
}

// ResolvePinned returns an executor only when an open tool still matches the Run-owned schema contract.
func (r *ActionRegistry) ResolvePinned(
	ctx context.Context,
	scope tools.Scope,
	kind string,
	expected *ToolSchemaSnapshot,
) (ActionExecutor, error) {
	return r.resolve(ctx, scope, kind, expected, true)
}

func (r *ActionRegistry) resolve(
	ctx context.Context,
	scope tools.Scope,
	kind string,
	expected *ToolSchemaSnapshot,
	requirePinned bool,
) (ActionExecutor, error) {
	if ctx == nil {
		return nil, fmt.Errorf("%w: resolve action context is required", ErrValidation)
	}
	trimmed := strings.TrimSpace(kind)
	switch dsl.ActionKind(trimmed) {
	case dsl.ActionRunAgent:
		return r.runAgent, nil
	case dsl.ActionRunLoop:
		return r.runLoop, nil
	case dsl.ActionTransform:
		return r.transform, nil
	case dsl.ActionGoal:
		if r.goal == nil {
			return nil, reasonError(
				ReasonCodeActionDependencyMissing,
				ErrActionDependencyMissing,
				map[string]string{actionDependencyMetaKey: "goal_executor"},
			)
		}
		return r.goal, nil
	}
	id := tools.ToolID(trimmed)
	if err := id.Validate(); err != nil {
		return nil, unknownActionKindError(trimmed, err)
	}
	view, err := r.runtime.Get(ctx, scope, id)
	if err != nil {
		return nil, unknownActionKindError(trimmed, err)
	}
	if requirePinned {
		if err := validatePinnedActionContract(trimmed, view.Descriptor, expected); err != nil {
			return nil, err
		}
	}
	return &ToolCallActionExecutor{
		runtime:     r.runtime,
		toolID:      id,
		eventReader: r.events,
		channel:     r.channel,
	}, nil
}

func validatePinnedActionContract(
	kind string,
	descriptor tools.Descriptor,
	expected *ToolSchemaSnapshot,
) error {
	if expected != nil && strings.TrimSpace(expected.ToolID) == kind &&
		strings.TrimSpace(expected.InputSchemaDigest) == strings.TrimSpace(descriptor.InputSchemaDigest) &&
		strings.TrimSpace(expected.OutputSchemaDigest) == strings.TrimSpace(descriptor.OutputSchemaDigest) {
		return nil
	}
	meta := map[string]string{actionKindMetaKey: kind}
	if expected != nil {
		meta["expected_input_schema_digest"] = strings.TrimSpace(expected.InputSchemaDigest)
		meta["expected_output_schema_digest"] = strings.TrimSpace(expected.OutputSchemaDigest)
	}
	meta["current_input_schema_digest"] = strings.TrimSpace(descriptor.InputSchemaDigest)
	meta["current_output_schema_digest"] = strings.TrimSpace(descriptor.OutputSchemaDigest)
	return reasonError(
		ReasonCodeActionContractStale,
		fmt.Errorf("%w: action tool contract changed after Start", ErrActionSchemaInvalid),
		meta,
	)
}

func unknownActionKindError(kind string, err error) error {
	return reasonError(
		ReasonCodeUnknownActionKind,
		errors.Join(ErrActionUnknownKind, err),
		map[string]string{actionKindMetaKey: kind},
	)
}
