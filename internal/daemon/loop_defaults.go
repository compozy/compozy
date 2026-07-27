package daemon

import (
	"context"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	loopdsl "github.com/compozy/compozy/internal/loop/dsl"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func newLoopDefaultsResolver(
	homePaths aghconfig.HomePaths,
	workspaceResolver workspacepkg.RuntimeResolver,
) looppkg.DefaultsResolver {
	return func(ctx context.Context, ws looppkg.WorkspaceID) (looppkg.LoopDefaults, error) {
		cfg, err := resolveLoopServiceConfig(ctx, homePaths, workspaceResolver, ws)
		if err != nil {
			return looppkg.LoopDefaults{}, err
		}
		return loopDefaultsFromConfig(cfg.Loops), nil
	}
}

func newLoopInputDefaultsResolver(
	homePaths aghconfig.HomePaths,
	workspaceResolver workspacepkg.RuntimeResolver,
) looppkg.InputDefaultsResolver {
	return func(
		ctx context.Context,
		ws looppkg.WorkspaceID,
		loopName string,
	) (looppkg.InputDefaultLayers, error) {
		cfg, err := resolveLoopServiceConfig(ctx, homePaths, workspaceResolver, ws)
		if err != nil {
			return looppkg.InputDefaultLayers{}, err
		}
		global, workspace := cfg.Loops.InputDefaultLayers(loopName)
		return looppkg.InputDefaultLayers{Global: global, Workspace: workspace}, nil
	}
}

func resolveLoopServiceConfig(
	ctx context.Context,
	homePaths aghconfig.HomePaths,
	workspaceResolver workspacepkg.RuntimeResolver,
	ws looppkg.WorkspaceID,
) (aghconfig.Config, error) {
	workspaceID := strings.TrimSpace(string(ws))
	if workspaceResolver != nil && workspaceID != "" {
		resolved, err := workspaceResolver.Resolve(ctx, workspaceID)
		if err != nil {
			return aghconfig.Config{}, fmt.Errorf("daemon: resolve Loop service workspace %q: %w", workspaceID, err)
		}
		return resolved.Config, nil
	}
	cfg, err := aghconfig.LoadForHome(homePaths)
	if err != nil {
		return aghconfig.Config{}, fmt.Errorf("daemon: load Loop service config: %w", err)
	}
	return cfg, nil
}

func loopDefaultsFromConfig(cfg aghconfig.LoopsConfig) looppkg.LoopDefaults {
	return looppkg.LoopDefaults{
		Delivery: loopDefaultConfigFromConfig(cfg.Defaults.Delivery, true),
		Watch:    loopDefaultConfigFromConfig(cfg.Defaults.Watch, false),
	}
}

func loopDefaultConfigFromConfig(cfg aghconfig.LoopDefaultConfig, includeZeroGate bool) looppkg.LoopConfig {
	result := looppkg.LoopConfig{
		IterationCap:     new(cfg.IterationCap),
		NoProgressWindow: new(cfg.NoProgress.Window),
		BudgetTokens:     new(cfg.Budget.Tokens),
		BudgetWallSec:    new(cfg.Budget.WallClockSec),
		BudgetOnExceeded: budgetExceededPtr(cfg.Budget.OnExceeded),
		RuntimeDefaults:  loopRuntimeDefaultsFromConfig(cfg.RuntimeDefaults),
		RuntimeRules:     loopRuntimeRulesFromConfig(cfg.RuntimeRules),
		FanOutWidth:      new(cfg.FanOutWidth),
	}
	if includeZeroGate || cfg.Gates.MaxRevisions > 0 {
		result.GateMaxRevisions = new(cfg.Gates.MaxRevisions)
	}
	return result
}

func loopRuntimeDefaultsFromConfig(cfg loopdsl.RuntimeDefaults) *looppkg.RuntimeDefaults {
	worker := runtimeSpecFromConfig(cfg.Worker)
	judge := runtimeSpecFromConfig(cfg.Judge)
	if runtimeSpecEmpty(worker) && runtimeSpecEmpty(judge) {
		return nil
	}
	return &looppkg.RuntimeDefaults{Worker: worker, Judge: judge}
}

func runtimeSpecEmpty(runtime looppkg.RuntimeSpec) bool {
	return runtime.Provider == "" && runtime.Model == "" && runtime.Reasoning == ""
}

func runtimeSpecFromConfig(cfg loopdsl.RuntimeSpec) looppkg.RuntimeSpec {
	return looppkg.RuntimeSpec{
		Provider:  strings.TrimSpace(cfg.Provider),
		Model:     strings.TrimSpace(cfg.Model),
		Reasoning: strings.TrimSpace(cfg.Reasoning),
	}
}

func loopRuntimeRulesFromConfig(rules []loopdsl.RuntimeRule) []looppkg.RuntimeRule {
	if len(rules) == 0 {
		return nil
	}
	cloned := make([]looppkg.RuntimeRule, len(rules))
	copy(cloned, rules)
	return cloned
}

func budgetExceededPtr(value string) *loopdsl.BudgetExceeded {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = string(loopdsl.BudgetExceededHalt)
	}
	parsed := loopdsl.BudgetExceeded(trimmed)
	return &parsed
}
