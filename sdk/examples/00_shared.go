package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	engineknowledge "github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
	sdkagent "github.com/compozy/compozy/sdk/v2/agent"
	compozy "github.com/compozy/compozy/sdk/v2/compozy"
)

const fallbackShutdown = 5 * time.Second

func withEngine(ctx context.Context, opts []compozy.Option, run func(context.Context, *compozy.Engine) error) error {
	if len(opts) == 0 {
		return fmt.Errorf("at least one engine option must be provided")
	}
	engine, err := compozy.New(ctx, opts...)
	if err != nil {
		return fmt.Errorf("create engine: %w", err)
	}
	report, err := engine.ValidateReferences()
	if err != nil {
		return fmt.Errorf("validate references: %w", err)
	}
	if err := ensureValid(report); err != nil {
		return err
	}
	if err := engine.Start(ctx); err != nil {
		return fmt.Errorf("start engine: %w", err)
	}
	defer stopEngine(ctx, engine)
	return run(ctx, engine)
}

func ensureValid(report *compozy.ValidationReport) error {
	if report == nil || report.Valid {
		return nil
	}
	issues := make([]string, 0, len(report.Errors)+len(report.MissingRefs)+len(report.CircularDeps))
	for _, item := range report.Errors {
		issues = append(issues, fmt.Sprintf("error:%s:%s:%s", item.ResourceType, item.ResourceID, item.Message))
	}
	for _, miss := range report.MissingRefs {
		issues = append(issues, fmt.Sprintf("missing:%s:%s->%s", miss.ResourceType, miss.ResourceID, miss.Reference))
	}
	for _, cycle := range report.CircularDeps {
		issues = append(issues, fmt.Sprintf("cycle:%s", strings.Join(cycle.Chain, "->")))
	}
	if len(issues) == 0 {
		return fmt.Errorf("engine validation failed without diagnostics")
	}
	return fmt.Errorf("engine validation failed: %s", strings.Join(issues, "; "))
}

func stopEngine(ctx context.Context, engine *compozy.Engine) {
	if engine == nil {
		return
	}
	cfg := config.FromContext(ctx)
	deadline := fallbackShutdown
	if cfg != nil && cfg.Server.Timeouts.ServerShutdown > 0 {
		deadline = cfg.Server.Timeouts.ServerShutdown
	}
	stopCtx := ctx
	var cancel context.CancelFunc
	if deadline > 0 {
		stopCtx, cancel = context.WithTimeout(context.WithoutCancel(ctx), deadline)
	}
	if cancel != nil {
		defer cancel()
	}
	if err := engine.Stop(stopCtx); err != nil {
		logger.FromContext(ctx).Warn("engine stop failed", "error", err)
	}
}

func inputPtr(values map[string]any) *core.Input {
	if len(values) == 0 {
		return nil
	}
	clone := core.CopyMaps(values)
	result := core.NewInput(clone)
	return &result
}

func outputPtr(values map[string]any) *core.Output {
	if len(values) == 0 {
		return nil
	}
	clone := core.CopyMaps(values)
	result := core.Output(clone)
	return &result
}

func stringOutput(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	text, ok := values[key].(string)
	if !ok {
		return ""
	}
	return text
}

func ensureOpenAIKey(ctx context.Context) (string, error) {
	key := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if key == "" {
		return "", fmt.Errorf("OPENAI_API_KEY environment variable is required")
	}
	if log := logger.FromContext(ctx); log != nil {
		log.Debug("OPENAI_API_KEY detected")
	}
	return key, nil
}

func newOpenAIModel(ctx context.Context, model string) (*engineagent.Model, error) {
	key, err := ensureOpenAIKey(ctx)
	if err != nil {
		return nil, err
	}
	return &engineagent.Model{
		Config: core.ProviderConfig{Provider: core.ProviderOpenAI, Model: model, APIKey: key},
	}, nil
}

func newAgentWithModel(
	ctx context.Context,
	id string,
	instructions string,
	model *engineagent.Model,
	extra ...sdkagent.Option,
) (*engineagent.Config, error) {
	if model == nil {
		return nil, fmt.Errorf("model configuration is required")
	}
	options := []sdkagent.Option{
		sdkagent.WithInstructions(strings.TrimSpace(instructions)),
		sdkagent.WithModel(*model),
	}
	options = append(options, extra...)
	cfg, err := sdkagent.New(ctx, id, options...)
	if err != nil {
		return nil, fmt.Errorf("build agent %s: %w", id, err)
	}
	return cfg, nil
}

func knowledgeSource(path string) engineknowledge.SourceConfig {
	return engineknowledge.SourceConfig{Type: engineknowledge.SourceTypeMarkdownGlob, Path: path}
}
