package daemon

import (
	"context"
	"fmt"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	loopdsl "github.com/compozy/agh/internal/loop/dsl"
	workspacepkg "github.com/compozy/agh/internal/workspace"
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
		ModelDefaults:    loopModelDefaultsFromConfig(cfg.ModelDefaults),
		FanOutWidth:      new(cfg.FanOutWidth),
	}
	if includeZeroGate || cfg.Gates.MaxRevisions > 0 {
		result.GateMaxRevisions = new(cfg.Gates.MaxRevisions)
	}
	return result
}

func loopModelDefaultsFromConfig(cfg aghconfig.LoopModelDefaultsConfig) *looppkg.ModelDefaults {
	worker := strings.TrimSpace(cfg.Worker)
	judge := strings.TrimSpace(cfg.Judge)
	if worker == "" && judge == "" {
		return nil
	}
	defaults := looppkg.ModelDefaults{}
	if worker != "" {
		defaults.Worker = new(worker)
	}
	if judge != "" {
		defaults.Judge = new(judge)
	}
	return &defaults
}

func budgetExceededPtr(value string) *loopdsl.BudgetExceeded {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = string(loopdsl.BudgetExceededHalt)
	}
	parsed := loopdsl.BudgetExceeded(trimmed)
	return &parsed
}
