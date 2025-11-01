package main

import (
	"context"
	"fmt"

	engineagent "github.com/compozy/compozy/engine/agent"
	enginecore "github.com/compozy/compozy/engine/core"
	enginememory "github.com/compozy/compozy/engine/memory"
	memcore "github.com/compozy/compozy/engine/memory/core"
	enginetask "github.com/compozy/compozy/engine/task"
	engineworkflow "github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/logger"
	sdkagent "github.com/compozy/compozy/sdk/v2/agent"
	compozy "github.com/compozy/compozy/sdk/v2/compozy"
	sdkmemory "github.com/compozy/compozy/sdk/v2/memory"
	sdktask "github.com/compozy/compozy/sdk/v2/task"
	sdkworkflow "github.com/compozy/compozy/sdk/v2/workflow"
)

// RunMemoryConversation demonstrates shared memory usage across sequential agent turns.
func RunMemoryConversation(ctx context.Context) error {
	memoryCfg, agentCfg, workflowCfg, err := buildMemoryConversation(ctx)
	if err != nil {
		return err
	}
	options := []compozy.Option{
		compozy.WithMemory(memoryCfg),
		compozy.WithAgent(agentCfg),
		compozy.WithWorkflow(workflowCfg),
	}
	return withEngine(ctx, options, func(execCtx context.Context, engine *compozy.Engine) error {
		input := map[string]any{
			"session_id":     "alpha",
			"first_message":  "I joined Helios yesterday, any onboarding tips?",
			"second_message": "Thanks! How should I prepare for the design partner sync?",
		}
		req := &compozy.ExecuteSyncRequest{Input: input}
		resp, err := engine.ExecuteWorkflowSync(execCtx, workflowCfg.ID, req)
		if err != nil {
			return fmt.Errorf("execute workflow: %w", err)
		}
		firstReply := stringOutput(resp.Output, "opening_reply")
		followupReply := stringOutput(resp.Output, "followup_reply")
		fmt.Println("Conversation summary:")
		fmt.Printf("- Assistant (turn 1): %s\n", firstReply)
		fmt.Printf("- Assistant (turn 2): %s\n", followupReply)
		logger.FromContext(execCtx).Info("memory workflow completed", "turn1", firstReply, "turn2", followupReply)
		return nil
	})
}

func buildMemoryConversation(
	ctx context.Context,
) (*enginememory.Config, *engineagent.Config, *engineworkflow.Config, error) {
	memoryCfg, err := sdkmemory.New(ctx, "conversation-store", "message_count_based",
		sdkmemory.WithPersistence(memcore.PersistenceConfig{Type: memcore.InMemoryPersistence, TTL: "30m"}),
		sdkmemory.WithMaxMessages(20),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create memory config: %w", err)
	}
	agentModel := &engineagent.Model{
		Config: enginecore.ProviderConfig{Provider: enginecore.ProviderMock, Model: "mock-conversation"},
	}
	memRef := enginecore.MemoryReference{
		ID:   memoryCfg.ID,
		Mode: enginecore.MemoryModeReadWrite,
		Key:  "session:{{ .workflow.input.session_id }}",
	}
	agentCfg, err := newAgentWithModel(
		ctx,
		"conversation-guide",
		"Maintain warm, contextual dialog across turns.",
		agentModel,
		sdkagent.WithMemory([]enginecore.MemoryReference{memRef}),
	)
	if err != nil {
		return nil, nil, nil, err
	}
	openingTask, err := sdktask.New(
		ctx,
		"opening-turn",
		sdktask.WithAgent(agentCfg),
		sdktask.WithPrompt(
			"User: {{ .workflow.input.first_message }}. Respond with a concise welcome and a practical suggestion.",
		),
		sdktask.WithOutputs(inputPtr(map[string]any{"reply": "{{ .task.output }}"})),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create opening task: %w", err)
	}
	followTask, err := sdktask.New(
		ctx,
		"followup-turn",
		sdktask.WithAgent(agentCfg),
		sdktask.WithPrompt(
			"Review prior exchange stored in memory. Earlier you said '{{ .tasks.opening-turn.output.reply }}'. "+
				"Now the user adds '{{ .workflow.input.second_message }}'. Continue the conversation building on prior advice.",
		),
		sdktask.WithOutputs(inputPtr(map[string]any{"reply": "{{ .task.output }}"})),
		sdktask.WithFinal(true),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create follow-up task: %w", err)
	}
	workflowCfg, err := sdkworkflow.New(ctx, "memory-conversation",
		sdkworkflow.WithDescription("Shows multi-turn conversation with shared memory state"),
		sdkworkflow.WithTasks([]enginetask.Config{*openingTask, *followTask}),
		sdkworkflow.WithOutputs(outputPtr(map[string]any{
			"opening_reply":  "{{ .tasks.opening-turn.output.reply }}",
			"followup_reply": "{{ .tasks.followup-turn.output.reply }}",
		})),
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create workflow: %w", err)
	}
	return memoryCfg, agentCfg, workflowCfg, nil
}
