package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
	"github.com/compozy/compozy/engine/llm/telemetry"
)

type orchestrator struct {
	cfg      Config
	settings settings

	memory  MemoryManager
	builder RequestBuilder
	client  ClientManager
	loop    *conversationLoop
	trace   telemetry.RunRecorder
}

//nolint:gocritic // Config must be passed by value to ensure defensive copy semantics.
func New(cfg Config) (Orchestrator, error) {
	if err := validateConfigDependencies(&cfg); err != nil {
		return nil, err
	}
	cfgCopy := cfg
	settings := buildSettings(&cfgCopy)
	memoryManager := NewMemoryManager(cfgCopy.MemoryProvider, cfgCopy.MemorySync, cfgCopy.AsyncHook)
	reqBuilder := NewRequestBuilder(
		cfgCopy.PromptBuilder,
		cfgCopy.SystemPromptRenderer,
		cfgCopy.ToolRegistry,
		memoryManager,
		settings.toolSuggestionLimit,
	)
	toolExec := NewToolExecutor(cfgCopy.ToolRegistry, &settings)
	responseHandler := NewResponseHandler(&settings)
	llmInvoker := NewLLMInvoker(&settings)
	conv := newConversationLoop(&settings, toolExec, responseHandler, llmInvoker, memoryManager)
	trace, err := ensureTelemetryRecorder(&cfgCopy)
	if err != nil {
		return nil, err
	}
	return &orchestrator{
		cfg:      cfgCopy,
		settings: settings,
		memory:   memoryManager,
		builder:  reqBuilder,
		client:   NewClientManager(),
		loop:     conv,
		trace:    trace,
	}, nil
}

//nolint:gocritic // Request copied to isolate orchestrator execution from caller mutations.
func (o *orchestrator) Execute(ctx context.Context, request Request) (_ *core.Output, err error) {
	if err := validateRequest(ctx, request); err != nil {
		return nil, err
	}
	if err := o.attachCapabilities(&request); err != nil {
		return nil, err
	}
	recorder := o.resolveRecorder()
	ctx, run, startErr := recorder.StartRun(ctx, o.runMetadata(&request))
	if startErr != nil {
		return nil, startErr
	}
	defer o.closeRun(ctx, recorder, run, &err)
	memoryCtx, buildResult, client, cleanup, prepErr := o.prepareExecution(ctx, &request)
	if prepErr != nil {
		return nil, prepErr
	}
	defer cleanup(ctx)
	request.Prompt.DynamicContext = buildResult.PromptContext
	state := newLoopState(&o.settings, memoryCtx, request.Action)
	output, response, runErr := o.loop.Run(
		ctx,
		client,
		&buildResult.Request,
		request,
		state,
		buildResult.PromptTemplate,
	)
	if runErr != nil {
		return nil, runErr
	}
	o.persistResponse(ctx, memoryCtx, &buildResult, &request, response)
	o.recordRunOutput(ctx, recorder, &buildResult, response)
	return output, nil
}

func validateConfigDependencies(cfg *Config) error {
	if cfg == nil {
		return core.NewError(
			fmt.Errorf("orchestrator config cannot be nil"),
			ErrCodeInvalidConfig,
			map[string]any{"field": "Config"},
		)
	}
	switch {
	case cfg.PromptBuilder == nil:
		return missingDependencyError("PromptBuilder", "prompt builder")
	case cfg.SystemPromptRenderer == nil:
		return missingDependencyError("SystemPromptRenderer", "system prompt renderer")
	case cfg.ToolRegistry == nil:
		return missingDependencyError("ToolRegistry", "tool registry")
	case cfg.LLMFactory == nil:
		return missingDependencyError("LLMFactory", "llm factory")
	default:
		return nil
	}
}

func missingDependencyError(field, name string) error {
	return core.NewError(
		fmt.Errorf("%s cannot be nil", name),
		ErrCodeInvalidConfig,
		map[string]any{"field": field},
	)
}

func ensureTelemetryRecorder(cfg *Config) (telemetry.RunRecorder, error) {
	if cfg.TelemetryRecorder != nil {
		return cfg.TelemetryRecorder, nil
	}
	opts := telemetry.Options{ProjectRoot: cfg.ProjectRoot}
	if cfg.TelemetryOptions != nil {
		opts = *cfg.TelemetryOptions
		opts.ProjectRoot = cfg.ProjectRoot
	}
	recorder, err := telemetry.NewRecorder(&opts)
	if err != nil {
		return nil, err
	}
	return recorder, nil
}

func (o *orchestrator) attachCapabilities(request *Request) error {
	caps, err := o.cfg.LLMFactory.Capabilities(request.Agent.Model.Config.Provider)
	if err != nil {
		return err
	}
	request.Execution.ProviderCaps = caps
	return nil
}

func (o *orchestrator) resolveRecorder() telemetry.RunRecorder {
	if o.trace != nil {
		return o.trace
	}
	return telemetry.NopRecorder()
}

func (o *orchestrator) runMetadata(request *Request) telemetry.RunMetadata {
	if request == nil {
		return telemetry.RunMetadata{}
	}
	return telemetry.RunMetadata{
		AgentID:     request.Agent.ID,
		ActionID:    request.Action.ID,
		WorkflowID:  "",
		ExecutionID: "",
	}
}

func (o *orchestrator) closeRun(
	ctx context.Context,
	recorder telemetry.RunRecorder,
	run *telemetry.Run,
	runErr *error,
) {
	var resultErr error
	if runErr != nil {
		resultErr = *runErr
	}
	closeErr := recorder.CloseRun(ctx, run, telemetry.RunResult{
		Success: resultErr == nil,
		Error:   resultErr,
	})
	if closeErr != nil && runErr != nil && *runErr == nil {
		*runErr = closeErr
	}
}

func (o *orchestrator) prepareExecution(
	ctx context.Context,
	request *Request,
) (*MemoryContext, RequestBuildOutput, llmadapter.LLMClient, func(context.Context), error) {
	if request == nil {
		return nil, RequestBuildOutput{}, nil, func(context.Context) {}, fmt.Errorf("request cannot be nil")
	}
	memoryCtx := o.memory.Prepare(ctx, *request)
	buildResult, err := o.builder.Build(ctx, *request, memoryCtx)
	if err != nil {
		return nil, RequestBuildOutput{}, nil, func(context.Context) {}, err
	}
	client, err := o.client.Create(ctx, *request, o.cfg.LLMFactory)
	if err != nil {
		return nil, RequestBuildOutput{}, nil, func(context.Context) {}, err
	}
	cleanup := func(closeCtx context.Context) { o.client.Close(closeCtx, client) }
	return memoryCtx, buildResult, client, cleanup, nil
}

func (o *orchestrator) persistResponse(
	ctx context.Context,
	memoryCtx *MemoryContext,
	buildResult *RequestBuildOutput,
	request *Request,
	response *llmadapter.LLMResponse,
) {
	if o.loop.memory != nil || buildResult == nil || request == nil {
		return
	}
	o.memory.StoreAsync(ctx, memoryCtx, response, buildResult.Request.Messages, *request)
}

func (o *orchestrator) recordRunOutput(
	ctx context.Context,
	recorder telemetry.RunRecorder,
	buildResult *RequestBuildOutput,
	response *llmadapter.LLMResponse,
) {
	if response == nil || buildResult == nil {
		return
	}
	payload := map[string]any{
		"final_messages": len(buildResult.Request.Messages),
	}
	if telemetry.CaptureContentEnabled(ctx) {
		payload["final_response"] = response.Content
	}
	recorder.RecordEvent(ctx, &telemetry.Event{
		Stage:    "run_output_ready",
		Severity: telemetry.SeverityInfo,
		Payload:  payload,
	})
}

func (o *orchestrator) Close() error {
	if o.cfg.ToolRegistry != nil {
		return o.cfg.ToolRegistry.Close()
	}
	return nil
}

//nolint:gocritic // Request copied to validate snapshot without modifying caller state.
func validateRequest(ctx context.Context, request Request) error {
	if request.Agent == nil {
		return NewValidationError(fmt.Errorf("agent config is required"), "agent", nil)
	}
	if request.Action == nil {
		return NewValidationError(fmt.Errorf("action config is required"), "action", nil)
	}
	if strings.TrimSpace(request.Agent.ID) == "" {
		return NewValidationError(fmt.Errorf("agent ID is required"), "agent.id", nil)
	}
	if strings.TrimSpace(request.Action.ID) == "" {
		return NewValidationError(fmt.Errorf("action ID is required"), "action.id", nil)
	}
	if strings.TrimSpace(request.Agent.Instructions) == "" {
		return NewValidationError(fmt.Errorf("agent instructions are required"), "agent.instructions", nil)
	}
	if strings.TrimSpace(request.Action.Prompt) == "" {
		return NewValidationError(fmt.Errorf("action prompt is required"), "action.prompt", nil)
	}
	if request.Action.InputSchema != nil {
		if err := request.Action.ValidateInput(ctx, request.Action.GetInput()); err != nil {
			return NewValidationError(
				fmt.Errorf("input validation failed: %w", err),
				"action.input",
				request.Action.GetInput(),
			)
		}
	}
	return nil
}
