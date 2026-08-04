package daemon

import (
	"context"
	"fmt"
	"strings"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	looppkg "github.com/compozy/compozy/internal/loop"
	loopdsl "github.com/compozy/compozy/internal/loop/dsl"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

func newLoopDefaultsResolver(
	homePaths compozyconfig.HomePaths,
	workspaceResolver workspacepkg.RuntimeResolver,
) looppkg.DefaultsResolver {
	return func(ctx context.Context, ws looppkg.WorkspaceID) (looppkg.LoopDefaults, error) {
		cfg, err := resolveLoopServiceConfig(ctx, homePaths, workspaceResolver, ws)
		if err != nil {
			return looppkg.LoopDefaults{}, err
		}
		return loopDefaultsFromConfig(&cfg.Loops)
	}
}

func newLoopInputDefaultsResolver(
	homePaths compozyconfig.HomePaths,
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
	homePaths compozyconfig.HomePaths,
	workspaceResolver workspacepkg.RuntimeResolver,
	ws looppkg.WorkspaceID,
) (compozyconfig.Config, error) {
	workspaceID := strings.TrimSpace(string(ws))
	if workspaceResolver != nil && workspaceID != "" {
		resolved, err := workspaceResolver.Resolve(ctx, workspaceID)
		if err != nil {
			return compozyconfig.Config{}, fmt.Errorf("daemon: resolve Loop service workspace %q: %w", workspaceID, err)
		}
		return resolved.Config, nil
	}
	cfg, err := compozyconfig.LoadForHome(homePaths)
	if err != nil {
		return compozyconfig.Config{}, fmt.Errorf("daemon: load Loop service config: %w", err)
	}
	return cfg, nil
}

func loopDefaultsFromConfig(cfg *compozyconfig.LoopsConfig) (looppkg.LoopDefaults, error) {
	delivery, err := loopDefaultConfigFromConfig(cfg.Defaults.Delivery, true)
	if err != nil {
		return looppkg.LoopDefaults{}, err
	}
	watch, err := loopDefaultConfigFromConfig(cfg.Defaults.Watch, false)
	if err != nil {
		return looppkg.LoopDefaults{}, err
	}
	return looppkg.LoopDefaults{Delivery: delivery, Watch: watch}, nil
}

func loopDefaultConfigFromConfig(
	cfg compozyconfig.LoopDefaultConfig,
	includeZeroGate bool,
) (looppkg.LoopConfig, error) {
	lifecycle, err := loopLifecycleConfigFromConfig(cfg)
	if err != nil {
		return looppkg.LoopConfig{}, err
	}
	result := looppkg.LoopConfig{
		IterationCap:     new(cfg.IterationCap),
		NoProgressWindow: new(cfg.NoProgress.Window),
		BudgetTokens:     new(cfg.Budget.Tokens),
		BudgetWallSec:    new(cfg.Budget.WallClockSec),
		BudgetOnExceeded: budgetExceededPtr(cfg.Budget.OnExceeded),
		RuntimeDefaults:  loopRuntimeDefaultsFromConfig(cfg.RuntimeDefaults),
		RuntimeRules:     loopRuntimeRulesFromConfig(cfg.RuntimeRules),
		FanOutWidth:      new(cfg.FanOutWidth),
		Lifecycle:        &lifecycle,
	}
	if includeZeroGate || cfg.Gates.MaxRevisions > 0 {
		result.GateMaxRevisions = new(cfg.Gates.MaxRevisions)
	}
	return result, nil
}

func loopLifecycleConfigFromConfig(cfg compozyconfig.LoopDefaultConfig) (looppkg.LifecycleConfig, error) {
	backoffBase, err := time.ParseDuration(strings.TrimSpace(cfg.Retry.BackoffBase))
	if err != nil {
		return looppkg.LifecycleConfig{}, fmt.Errorf("daemon: parse Loop retry backoff base: %w", err)
	}
	backoffMax, err := time.ParseDuration(strings.TrimSpace(cfg.Retry.BackoffMax))
	if err != nil {
		return looppkg.LifecycleConfig{}, fmt.Errorf("daemon: parse Loop retry backoff max: %w", err)
	}
	silenceWindow, err := time.ParseDuration(strings.TrimSpace(cfg.Liveness.SilenceWindow))
	if err != nil {
		return looppkg.LifecycleConfig{}, fmt.Errorf("daemon: parse Loop silence window: %w", err)
	}
	waitInterval, err := time.ParseDuration(strings.TrimSpace(cfg.Waits.AdmissionRetryInterval))
	if err != nil {
		return looppkg.LifecycleConfig{}, fmt.Errorf("daemon: parse Loop wait admission interval: %w", err)
	}
	horizon, err := time.ParseDuration(strings.TrimSpace(cfg.Admission.TombstoneHorizon))
	if err != nil {
		return looppkg.LifecycleConfig{}, fmt.Errorf("daemon: parse Loop admission horizon: %w", err)
	}
	rules := make([]looppkg.LifecycleAutopauseRule, len(cfg.Autopause))
	for index, rule := range cfg.Autopause {
		rules[index] = looppkg.LifecycleAutopauseRule{Match: rule.Match, Action: rule.Action}
	}
	return looppkg.LifecycleConfig{
		RetryMaxAttempts:       new(cfg.Retry.MaxAttempts),
		RetryBackoffBase:       new(backoffBase),
		RetryBackoffMax:        new(backoffMax),
		LivenessSilenceWindow:  new(silenceWindow),
		ResumeDeathStreakLimit: new(cfg.Resume.DeathStreakLimit),
		PredicateCostLimit:     new(cfg.Predicates.CostLimit),
		WaitAdmissionAttempts:  new(cfg.Waits.AdmissionAttempts),
		WaitAdmissionInterval:  new(waitInterval),
		AdmissionHorizon:       new(horizon),
		Autopause:              rules,
	}, nil
}

func loopBreakerPolicyFromConfig(cfg compozyconfig.LoopBreakerConfig) (looppkg.GlobalBreakerPolicy, error) {
	probeInterval, err := time.ParseDuration(strings.TrimSpace(cfg.ProbeInterval))
	if err != nil {
		return looppkg.GlobalBreakerPolicy{}, fmt.Errorf("daemon: parse Loop breaker probe interval: %w", err)
	}
	return looppkg.ResolveGlobalBreakerPolicy(cfg.Threshold, probeInterval)
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
