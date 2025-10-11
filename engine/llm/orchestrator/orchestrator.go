package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/compozy/engine/core"
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
	if cfg.PromptBuilder == nil {
		return nil, core.NewError(
			fmt.Errorf("prompt builder cannot be nil"),
			ErrCodeInvalidConfig,
			map[string]any{"field": "PromptBuilder"},
		)
	}
	if cfg.SystemPromptRenderer == nil {
		return nil, core.NewError(
			fmt.Errorf("system prompt renderer cannot be nil"),
			ErrCodeInvalidConfig,
			map[string]any{"field": "SystemPromptRenderer"},
		)
	}
	if cfg.ToolRegistry == nil {
		return nil, core.NewError(
			fmt.Errorf("tool registry cannot be nil"),
			ErrCodeInvalidConfig,
			map[string]any{"field": "ToolRegistry"},
		)
	}
	if cfg.LLMFactory == nil {
		return nil, core.NewError(
			fmt.Errorf("llm factory cannot be nil"),
			ErrCodeInvalidConfig,
			map[string]any{"field": "LLMFactory"},
		)
	}
	cfgCopy := cfg
	settings := buildSettings(&cfgCopy)
	memoryManager := NewMemoryManager(cfgCopy.MemoryProvider, cfgCopy.MemorySync, cfgCopy.AsyncHook)
	reqBuilder := NewRequestBuilder(
		cfgCopy.PromptBuilder,
		cfgCopy.SystemPromptRenderer,
		cfgCopy.ToolRegistry,
		memoryManager,
	)
	toolExec := NewToolExecutor(cfgCopy.ToolRegistry, &settings)
	responseHandler := NewResponseHandler(&settings)
	llmInvoker := NewLLMInvoker(&settings)
	conv := newConversationLoop(&settings, toolExec, responseHandler, llmInvoker, memoryManager)
	trace := cfgCopy.TelemetryRecorder
	if trace == nil {
		opts := telemetry.Options{
			ProjectRoot: cfgCopy.ProjectRoot,
		}
		if cfgCopy.TelemetryOptions != nil {
			opts = *cfgCopy.TelemetryOptions
			opts.ProjectRoot = cfgCopy.ProjectRoot
		}
		recorder, err := telemetry.NewRecorder(&opts)
		if err != nil {
			return nil, err
		}
		trace = recorder
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
	caps, capErr := o.cfg.LLMFactory.Capabilities(request.Agent.Model.Config.Provider)
	if capErr != nil {
		return nil, capErr
	}
	request.ProviderCaps = caps
	recorder := o.trace
	if recorder == nil {
		recorder = telemetry.NopRecorder()
	}
	runMeta := telemetry.RunMetadata{
		AgentID:     request.Agent.ID,
		ActionID:    request.Action.ID,
		WorkflowID:  "",
		ExecutionID: "",
	}
	ctx, run, startErr := recorder.StartRun(ctx, runMeta)
	if startErr != nil {
		return nil, startErr
	}
	defer func() {
		closeErr := recorder.CloseRun(ctx, run, telemetry.RunResult{
			Success: err == nil,
			Error:   err,
		})
		if closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	memoryCtx := o.memory.Prepare(ctx, request)
	buildResult, err := o.builder.Build(ctx, request, memoryCtx)
	if err != nil {
		return nil, err
	}
	client, err := o.client.Create(ctx, request, o.cfg.LLMFactory)
	if err != nil {
		return nil, err
	}
	defer o.client.Close(ctx, client)
	state := newLoopState(&o.settings, memoryCtx, request.Action)
	request.PromptContext = buildResult.PromptContext
	output, response, err := o.loop.Run(ctx, client, &buildResult.Request, request, state, buildResult.PromptTemplate)
	if err != nil {
		return nil, err
	}
	if o.loop.memory == nil {
		o.memory.StoreAsync(ctx, memoryCtx, response, buildResult.Request.Messages, request)
	}
	if response != nil {
		telemetryPayload := map[string]any{
			"final_messages": len(buildResult.Request.Messages),
		}
		if telemetry.CaptureContentEnabled(ctx) {
			telemetryPayload["final_response"] = response.Content
		}
		recorder.RecordEvent(ctx, &telemetry.Event{
			Stage:    "run_output_ready",
			Severity: telemetry.SeverityInfo,
			Payload:  telemetryPayload,
		})
	}
	return output, nil
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
