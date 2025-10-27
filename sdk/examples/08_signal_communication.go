package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	engineproject "github.com/compozy/compozy/engine/project"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	"github.com/compozy/compozy/sdk/agent"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/model"
	"github.com/compozy/compozy/sdk/project"
	"github.com/compozy/compozy/sdk/task"
	"github.com/compozy/compozy/sdk/workflow"
)

const (
	projectID             = "signal-coordination"
	modelProvider         = "openai"
	modelName             = "gpt-4o-mini"
	processorAgentID      = "data-processor-agent"
	processorActionID     = "process-payload"
	processorTaskID       = "process-data"
	signalSenderTaskID    = "notify-ready"
	analysisAgentID       = "data-analyst-agent"
	analysisActionID      = "analyze-payload"
	analysisTaskID        = "analyze-data"
	receiverWorkflowID    = "data-analyzer"
	senderWorkflowID      = "data-producer"
	sharedSignalChannelID = "data-ready"
)

func main() {
	ctx, cleanup, err := initializeContext()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize context: %v\n", err)
		os.Exit(1)
	}
	defer cleanup()
	if err := run(ctx); err != nil {
		logger.FromContext(ctx).Error("signal communication example failed", "error", err)
		os.Exit(1)
	}
}

func initializeContext() (context.Context, func(), error) {
	baseCtx, cancel := context.WithCancel(context.WithoutCancel(context.Background()))
	log := logger.NewLogger(nil)
	ctx := logger.ContextWithLogger(baseCtx, log)
	manager := config.NewManager(ctx, config.NewService())
	if _, err := manager.Load(ctx, config.NewDefaultProvider(), config.NewEnvProvider()); err != nil {
		cancel()
		_ = manager.Close(ctx)
		return nil, nil, fmt.Errorf("load configuration: %w", err)
	}
	ctx = config.ContextWithManager(ctx, manager)
	cleanup := func() {
		if err := manager.Close(ctx); err != nil {
			logger.FromContext(ctx).Warn("failed to close configuration manager", "error", err)
		}
		cancel()
	}
	return ctx, cleanup, nil
}

func run(ctx context.Context) error {
	log := logger.FromContext(ctx)
	environment := "unknown"
	if cfg := config.FromContext(ctx); cfg != nil {
		environment = cfg.Runtime.Environment
	}
	log.Info("running signal coordination example", "environment", environment)
	modelCfg, err := buildSharedModel(ctx)
	if err != nil {
		return handleBuildError(ctx, "model", err)
	}
	sender, err := assembleSenderResources(ctx)
	if err != nil {
		return err
	}
	receiver, err := assembleReceiverResources(ctx)
	if err != nil {
		return err
	}
	projectCfg, err := buildSignalProject(
		ctx,
		modelCfg,
		sender.agent,
		receiver.agent,
		sender.workflow,
		receiver.workflow,
	)
	if err != nil {
		return handleBuildError(ctx, "project", err)
	}
	summarizeSignalFlows(ctx, projectCfg, sender.workflow, receiver.workflow, sender.signalTask, receiver.waitTask)
	return nil
}

type senderResources struct {
	agent       *engineagent.Config
	processTask *enginetask.Config
	signalTask  *enginetask.Config
	workflow    *engineworkflow.Config
}

func assembleSenderResources(ctx context.Context) (*senderResources, error) {
	action, err := buildProcessorAction(ctx)
	if err != nil {
		return nil, handleBuildError(ctx, "processor_action", err)
	}
	agent, err := buildProcessorAgent(ctx, action)
	if err != nil {
		return nil, handleBuildError(ctx, "processor_agent", err)
	}
	processTask, err := buildProcessingTask(ctx, agent)
	if err != nil {
		return nil, handleBuildError(ctx, "processing_task", err)
	}
	signalTask, err := buildSignalSenderTask(ctx)
	if err != nil {
		return nil, handleBuildError(ctx, "signal_sender", err)
	}
	workflowCfg, err := buildSenderWorkflow(ctx, agent, processTask, signalTask)
	if err != nil {
		return nil, handleBuildError(ctx, "sender_workflow", err)
	}
	return &senderResources{
		agent:       agent,
		processTask: processTask,
		signalTask:  signalTask,
		workflow:    workflowCfg,
	}, nil
}

type receiverResources struct {
	agent    *engineagent.Config
	waitTask *enginetask.Config
	analysis *enginetask.Config
	workflow *engineworkflow.Config
}

func assembleReceiverResources(ctx context.Context) (*receiverResources, error) {
	action, err := buildAnalysisAction(ctx)
	if err != nil {
		return nil, handleBuildError(ctx, "analysis_action", err)
	}
	agent, err := buildAnalysisAgent(ctx, action)
	if err != nil {
		return nil, handleBuildError(ctx, "analysis_agent", err)
	}
	waitTask, err := buildSignalWaitTask(ctx)
	if err != nil {
		return nil, handleBuildError(ctx, "signal_wait", err)
	}
	analysisTask, err := buildAnalysisTask(ctx, agent)
	if err != nil {
		return nil, handleBuildError(ctx, "analysis_task", err)
	}
	workflowCfg, err := buildReceiverWorkflow(ctx, agent, waitTask, analysisTask)
	if err != nil {
		return nil, handleBuildError(ctx, "receiver_workflow", err)
	}
	return &receiverResources{
		agent:    agent,
		waitTask: waitTask,
		analysis: analysisTask,
		workflow: workflowCfg,
	}, nil
}

func buildSharedModel(ctx context.Context) (*core.ProviderConfig, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" {
		logger.FromContext(ctx).Warn("OPENAI_API_KEY is not set; LLM calls will fail if executed")
	}
	return model.New(modelProvider, modelName).
		WithAPIKey(apiKey).
		WithDefault(true).
		Build(ctx)
}

func buildProcessorAction(ctx context.Context) (*engineagent.ActionConfig, error) {
	return agent.NewAction(processorActionID).
		WithPrompt("Summarize recent ingestion results and emit a status payload.").
		Build(ctx)
}

func buildProcessorAgent(ctx context.Context, actionCfg *engineagent.ActionConfig) (*engineagent.Config, error) {
	if actionCfg == nil {
		return nil, fmt.Errorf("processor action config is required")
	}
	return agent.New(processorAgentID).
		WithInstructions("You transform batch data and confirm readiness once processing succeeds.").
		WithModel(modelProvider, modelName).
		AddAction(actionCfg).
		Build(ctx)
}

func buildProcessingTask(ctx context.Context, agentCfg *engineagent.Config) (*enginetask.Config, error) {
	if agentCfg == nil {
		return nil, fmt.Errorf("processor agent config is required")
	}
	return task.NewBasic(processorTaskID).
		WithAgent(agentCfg.ID).
		WithAction(processorActionID).
		Build(ctx)
}

func buildSignalSenderTask(ctx context.Context) (*enginetask.Config, error) {
	// Signal senders notify downstream workflows when upstream processing completes and share structured payloads.
	payload := map[string]any{
		"status":       "completed",
		"result_id":    "{{ .tasks.process-data.output.id }}",
		"completed_at": "{{ now }}",
	}
	return task.NewSignal(signalSenderTaskID).
		Send(sharedSignalChannelID, payload).
		Build(ctx)
}

func buildSenderWorkflow(
	ctx context.Context,
	agentCfg *engineagent.Config,
	processingTask *enginetask.Config,
	signalSender *enginetask.Config,
) (*engineworkflow.Config, error) {
	if agentCfg == nil || processingTask == nil || signalSender == nil {
		return nil, fmt.Errorf("sender workflow requires agent, processing task, and signal sender")
	}
	return workflow.New(senderWorkflowID).
		WithDescription("Processes inbound data and broadcasts readiness signals").
		AddAgent(agentCfg).
		AddTask(processingTask).
		AddTask(signalSender).
		Build(ctx)
}

func buildAnalysisAction(ctx context.Context) (*engineagent.ActionConfig, error) {
	return agent.NewAction(analysisActionID).
		WithPrompt("Review processed data metadata and compile an executive summary.").
		Build(ctx)
}

func buildAnalysisAgent(ctx context.Context, actionCfg *engineagent.ActionConfig) (*engineagent.Config, error) {
	if actionCfg == nil {
		return nil, fmt.Errorf("analysis action config is required")
	}
	return agent.New(analysisAgentID).
		WithInstructions("You analyze processed datasets after downstream signals confirm readiness.").
		WithModel(modelProvider, modelName).
		AddAction(actionCfg).
		Build(ctx)
}

func buildSignalWaitTask(ctx context.Context) (*enginetask.Config, error) {
	// Signal waiters block workflow progress until the expected signal arrives or a timeout expires.
	return task.NewSignal("wait-for-ready").
		Wait(sharedSignalChannelID).
		WithTimeout(5 * time.Minute).
		Build(ctx)
}

func buildAnalysisTask(ctx context.Context, agentCfg *engineagent.Config) (*enginetask.Config, error) {
	if agentCfg == nil {
		return nil, fmt.Errorf("analysis agent config is required")
	}
	return task.NewBasic(analysisTaskID).
		WithAgent(agentCfg.ID).
		WithAction(analysisActionID).
		WithFinal(true).
		Build(ctx)
}

func buildReceiverWorkflow(
	ctx context.Context,
	agentCfg *engineagent.Config,
	waitTask *enginetask.Config,
	analysisTask *enginetask.Config,
) (*engineworkflow.Config, error) {
	if agentCfg == nil || waitTask == nil || analysisTask == nil {
		return nil, fmt.Errorf("receiver workflow requires agent, wait task, and analysis task")
	}
	return workflow.New(receiverWorkflowID).
		WithDescription("Waits for readiness signals before running downstream analysis").
		AddAgent(agentCfg).
		AddTask(waitTask).
		AddTask(analysisTask).
		Build(ctx)
}

func buildSignalProject(
	ctx context.Context,
	modelCfg *core.ProviderConfig,
	processorAgent *engineagent.Config,
	analysisAgent *engineagent.Config,
	senderWorkflow *engineworkflow.Config,
	receiverWorkflow *engineworkflow.Config,
) (*engineproject.Config, error) {
	if modelCfg == nil || processorAgent == nil || analysisAgent == nil || senderWorkflow == nil ||
		receiverWorkflow == nil {
		return nil, fmt.Errorf("project configuration requires model, agents, and workflows")
	}
	return project.New(projectID).
		WithDescription("Demonstrates inter-workflow coordination using unified signals").
		AddModel(modelCfg).
		AddAgent(processorAgent).
		AddAgent(analysisAgent).
		AddWorkflow(senderWorkflow).
		AddWorkflow(receiverWorkflow).
		Build(ctx)
}

func summarizeSignalFlows(
	ctx context.Context,
	projectCfg *engineproject.Config,
	senderWorkflow *engineworkflow.Config,
	receiverWorkflow *engineworkflow.Config,
	sendTask *enginetask.Config,
	waitTask *enginetask.Config,
) {
	log := logger.FromContext(ctx)
	log.Info(
		"project configured for signal coordination",
		"project",
		projectCfg.Name,
		"workflows",
		[]string{senderWorkflow.ID, receiverWorkflow.ID},
	)
	log.Info(
		"workflow communication pattern",
		"sender",
		senderWorkflow.ID,
		"receiver",
		receiverWorkflow.ID,
		"shared_signal",
		sharedSignalChannelID,
	)
	if sendTask != nil && sendTask.Signal != nil {
		log.Info(
			"signal sender ready",
			"task_id",
			sendTask.ID,
			"signal",
			sendTask.Signal.ID,
			"payload_keys",
			mapKeys(sendTask.Signal.Payload),
		)
	}
	if waitTask != nil {
		log.Info(
			"signal waiter configured",
			"task_id",
			waitTask.ID,
			"wait_for",
			waitTask.WaitFor,
			"timeout",
			waitTask.Timeout,
		)
	}
}

func mapKeys(input map[string]any) []string {
	if len(input) == 0 {
		return nil
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	return keys
}

func handleBuildError(ctx context.Context, stage string, err error) error {
	var buildErr *sdkerrors.BuildError
	if errors.As(err, &buildErr) {
		log := logger.FromContext(ctx)
		for idx, cause := range buildErr.Errors {
			if cause == nil {
				continue
			}
			log.Error("builder validation failed", "stage", stage, "index", idx+1, "cause", cause.Error())
		}
	}
	return fmt.Errorf("%s build failed: %w", stage, err)
}
