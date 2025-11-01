package main

import (
	"context"
	"fmt"
	"os"
	"time"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	enginememory "github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	engineproject "github.com/compozy/compozy/engine/project"
	projectschedule "github.com/compozy/compozy/engine/project/schedule"
	enginetask "github.com/compozy/compozy/engine/task"
	enginetool "github.com/compozy/compozy/engine/tool"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	sdkagent "github.com/compozy/compozy/sdk/v2/agent"
	compozy "github.com/compozy/compozy/sdk/v2/compozy"
	sdkmemory "github.com/compozy/compozy/sdk/v2/memory"
	sdkproject "github.com/compozy/compozy/sdk/v2/project"
	sdktask "github.com/compozy/compozy/sdk/v2/task"
	sdktool "github.com/compozy/compozy/sdk/v2/tool"
	sdkworkflow "github.com/compozy/compozy/sdk/v2/workflow"
)

// RunCompleteProject wires agents, tools, knowledge, memory, and schedules into a single execution.
func RunCompleteProject(ctx context.Context) error {
	key, err := ensureOpenAIKey(ctx)
	if err != nil {
		return err
	}
	dir, err := writeKnowledgeDocuments()
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	embedderCfg, vectorCfg, kbCfg, err := createKnowledgeDefinitions(ctx, dir, key)
	if err != nil {
		return err
	}
	memoryCfg, err := sdkmemory.New(ctx, "project-memory", "message_count_based",
		sdkmemory.WithPersistence(memcore.PersistenceConfig{Type: memcore.InMemoryPersistence, TTL: "1h"}),
		sdkmemory.WithMaxMessages(50),
	)
	if err != nil {
		return fmt.Errorf("create memory: %w", err)
	}
	nativeTool, err := sdktool.New(ctx, "report-clock",
		sdktool.WithName("Report Clock"),
		sdktool.WithDescription("Returns a timestamp for the final report"),
		sdktool.WithNativeHandler(func(context.Context, map[string]any, map[string]any) (map[string]any, error) {
			return map[string]any{"timestamp": timeNow()}, nil
		}),
	)
	if err != nil {
		return fmt.Errorf("create tool: %w", err)
	}
	agentCfg, err := buildCompleteAgent(ctx, kbCfg, memoryCfg)
	if err != nil {
		return err
	}
	workflowCfg, err := buildCompleteWorkflow(ctx, agentCfg)
	if err != nil {
		return err
	}
	scheduleCfg := &projectschedule.Config{
		ID:          "daily-summary",
		WorkflowID:  workflowCfg.ID,
		Cron:        "0 8 * * *",
		Description: "Generates the morning launch update",
		Enabled:     boolPtr(true),
	}
	projectCfg, err := createCompleteProject(
		ctx,
		workflowCfg,
		embedderCfg,
		vectorCfg,
		kbCfg,
		memoryCfg,
		nativeTool,
		scheduleCfg,
	)
	if err != nil {
		return err
	}
	options := []compozy.Option{
		compozy.WithProject(projectCfg),
		compozy.WithAgent(agentCfg),
		compozy.WithTool(nativeTool),
		compozy.WithKnowledge(kbCfg),
		compozy.WithMemory(memoryCfg),
		compozy.WithWorkflow(workflowCfg),
		compozy.WithSchedule(scheduleCfg),
	}
	return withEngine(ctx, options, func(execCtx context.Context, engine *compozy.Engine) error {
		input := map[string]any{"session_id": "helios-demo", "topic": "Helios launch readiness"}
		resp, err := engine.ExecuteWorkflowSync(execCtx, workflowCfg.ID, &compozy.ExecuteSyncRequest{Input: input})
		if err != nil {
			return fmt.Errorf("execute complete project workflow: %w", err)
		}
		report := stringOutput(resp.Output, "final_report")
		fmt.Printf("Project report (%s):\n%s\n", input["topic"], report)
		logger.FromContext(execCtx).Info("complete project finished", "exec_id", resp.ExecID)
		return nil
	})
}

func buildCompleteAgent(
	ctx context.Context,
	kbCfg *engineknowledge.BaseConfig,
	memoryCfg *enginememory.Config,
) (*engineagent.Config, error) {
	model, err := newOpenAIModel(ctx, "gpt-4o-mini")
	if err != nil {
		return nil, err
	}
	binding := enginecore.KnowledgeBinding{ID: kbCfg.ID, TopK: intPtr(2), MinScore: floatPtr(0.2)}
	memRef := enginecore.MemoryReference{
		ID:   memoryCfg.ID,
		Mode: enginecore.MemoryModeReadWrite,
		Key:  "session:{{ .workflow.input.session_id }}",
	}
	return newAgentWithModel(
		ctx,
		"project-analyst",
		"Draft concise operational summaries grounded in provided knowledge.",
		model,
		sdkagent.WithKnowledge([]enginecore.KnowledgeBinding{binding}),
		sdkagent.WithMemory([]enginecore.MemoryReference{memRef}),
	)
}

func buildCompleteWorkflow(ctx context.Context, agentCfg *engineagent.Config) (*engineworkflow.Config, error) {
	timeTask, err := sdktask.New(ctx, "capture-time",
		sdktask.WithTool(&enginetool.Config{ID: "report-clock"}),
		sdktask.WithOutputs(inputPtr(map[string]any{"timestamp": "{{ .task.output.timestamp }}"})),
	)
	if err != nil {
		return nil, fmt.Errorf("create time task: %w", err)
	}
	reportTask, err := sdktask.New(
		ctx,
		"compose-report",
		sdktask.WithAgent(agentCfg),
		sdktask.WithPrompt(
			"Use knowledge base notes to summarize '{{ .workflow.input.topic }}' and cite sections. "+
				"Include the timestamp {{ .tasks.capture-time.output.timestamp }} in the first sentence.",
		),
		sdktask.WithOutputs(inputPtr(map[string]any{"report": "{{ .task.output }}"})),
		sdktask.WithFinal(true),
	)
	if err != nil {
		return nil, fmt.Errorf("create report task: %w", err)
	}
	return sdkworkflow.New(ctx, "complete-project",
		sdkworkflow.WithDescription("Generates a launch readiness bulletin"),
		sdkworkflow.WithTasks([]enginetask.Config{*timeTask, *reportTask}),
		sdkworkflow.WithOutputs(outputPtr(map[string]any{"final_report": "{{ .tasks.compose-report.output.report }}"})),
	)
}

func createCompleteProject(
	ctx context.Context,
	workflowCfg *engineworkflow.Config,
	embedderCfg *engineknowledge.EmbedderConfig,
	vectorCfg *engineknowledge.VectorDBConfig,
	kbCfg *engineknowledge.BaseConfig,
	memoryCfg *enginememory.Config,
	toolCfg *enginetool.Config,
	scheduleCfg *projectschedule.Config,
) (*engineproject.Config, error) {
	return sdkproject.New(ctx, "helios-complete",
		sdkproject.WithWorkflows([]*engineproject.WorkflowSourceConfig{{Source: workflowCfg.ID}}),
		sdkproject.WithEmbedders([]engineknowledge.EmbedderConfig{*embedderCfg}),
		sdkproject.WithVectorDBs([]engineknowledge.VectorDBConfig{*vectorCfg}),
		sdkproject.WithKnowledgeBases([]engineknowledge.BaseConfig{*kbCfg}),
		sdkproject.WithMemories([]*enginememory.Config{memoryCfg}),
		sdkproject.WithTools([]enginetool.Config{*toolCfg}),
		sdkproject.WithSchedules([]*projectschedule.Config{scheduleCfg}),
	)
}

func boolPtr(v bool) *bool {
	return &v
}

func timeNow() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}
