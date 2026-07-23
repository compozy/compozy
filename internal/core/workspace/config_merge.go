package workspace

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

func buildEffectiveProjectConfig(global, workspace ProjectConfig) ProjectConfig {
	defaults := mergeDefaultsConfig(global.Defaults, workspace.Defaults)
	return ProjectConfig{
		Defaults: defaults,
		Tasks:    buildEffectiveTasksConfig(global.Defaults, global.Tasks, workspace.Defaults, workspace.Tasks),
		FixReviews: buildEffectiveFixReviewsConfig(
			global.Defaults,
			global.FixReviews,
			workspace.Defaults,
			workspace.FixReviews,
		),
		FetchReviews: mergeFetchReviewsConfig(global.FetchReviews, workspace.FetchReviews),
		WatchReviews: mergeWatchReviewsConfig(global.WatchReviews, workspace.WatchReviews),
		Exec:         buildEffectiveExecConfig(global.Defaults, global.Exec, workspace.Defaults, workspace.Exec),
		Runs:         mergeRunsConfig(global.Runs, workspace.Runs),
		Recovery:     mergeRecoveryConfig(global.Recovery, workspace.Recovery).ApplyDefaults(),
		Sound:        mergeSoundConfig(global.Sound, workspace.Sound),
	}
}

func mergeDefaultsConfig(base, overlay DefaultsConfig) DefaultsConfig {
	return DefaultsConfig{
		RuntimeOverrides: mergeRuntimeOverrides(base.RuntimeOverrides, overlay.RuntimeOverrides),
		ByComplexity:     mergeTaskRuntimeByComplexity(base.ByComplexity, overlay.ByComplexity),
	}
}

func mergeTaskRuntimeByComplexity(
	base TaskRuntimeByComplexityConfig,
	overlay TaskRuntimeByComplexityConfig,
) TaskRuntimeByComplexityConfig {
	return TaskRuntimeByComplexityConfig{
		Low:      mergeTaskRuntimeOverrides(base.Low, overlay.Low),
		Medium:   mergeTaskRuntimeOverrides(base.Medium, overlay.Medium),
		High:     mergeTaskRuntimeOverrides(base.High, overlay.High),
		Critical: mergeTaskRuntimeOverrides(base.Critical, overlay.Critical),
	}
}

func mergeTaskRuntimeOverrides(base, overlay TaskRuntimeOverrides) TaskRuntimeOverrides {
	return TaskRuntimeOverrides{
		IDE:             cloneOptionalValue(preferOverlay(base.IDE, overlay.IDE)),
		Model:           cloneOptionalValue(preferOverlay(base.Model, overlay.Model)),
		ReasoningEffort: cloneOptionalValue(preferOverlay(base.ReasoningEffort, overlay.ReasoningEffort)),
	}
}

func buildEffectiveTasksConfig(
	globalDefaults DefaultsConfig,
	global TasksConfig,
	workspaceDefaults DefaultsConfig,
	workspace TasksConfig,
) TasksConfig {
	return TasksConfig{
		Types: cloneStringSlicePointer(preferOverlay(global.Types, workspace.Types)),
		Run: buildEffectiveTaskRunConfig(
			globalDefaults,
			global.Run,
			workspaceDefaults,
			workspace.Run,
		),
	}
}

func mergeFetchReviewsConfig(base, overlay FetchReviewsConfig) FetchReviewsConfig {
	return FetchReviewsConfig{
		Provider: cloneOptionalValue(preferOverlay(base.Provider, overlay.Provider)),
		Nitpicks: cloneOptionalValue(preferOverlay(base.Nitpicks, overlay.Nitpicks)),
	}
}

func mergeWatchReviewsConfig(base, overlay WatchReviewsConfig) WatchReviewsConfig {
	return WatchReviewsConfig{
		MaxRounds:     cloneOptionalValue(preferOverlay(base.MaxRounds, overlay.MaxRounds)),
		PollInterval:  cloneOptionalValue(preferOverlay(base.PollInterval, overlay.PollInterval)),
		ReviewTimeout: cloneOptionalValue(preferOverlay(base.ReviewTimeout, overlay.ReviewTimeout)),
		QuietPeriod:   cloneOptionalValue(preferOverlay(base.QuietPeriod, overlay.QuietPeriod)),
		AutoPush:      cloneOptionalValue(preferOverlay(base.AutoPush, overlay.AutoPush)),
		UntilClean:    cloneOptionalValue(preferOverlay(base.UntilClean, overlay.UntilClean)),
		PushRemote:    cloneOptionalValue(preferOverlay(base.PushRemote, overlay.PushRemote)),
		PushBranch:    cloneOptionalValue(preferOverlay(base.PushBranch, overlay.PushBranch)),
	}
}

func buildEffectiveTaskRunConfig(
	globalDefaults DefaultsConfig,
	global TaskRunConfig,
	workspaceDefaults DefaultsConfig,
	workspace TaskRunConfig,
) TaskRunConfig {
	return TaskRunConfig{
		IncludeCompleted: cloneOptionalValue(preferOverlay(global.IncludeCompleted, workspace.IncludeCompleted)),
		Recursive:        cloneOptionalValue(preferOverlay(global.Recursive, workspace.Recursive)),
		OutputFormat: effectiveCommandOverride(
			globalDefaults.OutputFormat,
			global.OutputFormat,
			workspaceDefaults.OutputFormat,
			workspace.OutputFormat,
		),
		RunMultipleMode: cloneOptionalValue(preferOverlay(global.RunMultipleMode, workspace.RunMultipleMode)),
		RunMultipleParallelLimit: cloneOptionalValue(
			preferOverlay(global.RunMultipleParallelLimit, workspace.RunMultipleParallelLimit),
		),
		Parallel: mergeParallelTasksConfig(global.Parallel, workspace.Parallel).ApplyDefaults(),
		ParallelTaskGroups: mergeParallelTaskGroupsConfig(
			global.ParallelTaskGroups,
			workspace.ParallelTaskGroups,
		).ApplyDefaults(),
		TUI:              cloneOptionalValue(preferOverlay(global.TUI, workspace.TUI)),
		TaskRuntimeRules: mergeTaskRunRuntimeRules(global.TaskRuntimeRules, workspace.TaskRuntimeRules),
	}
}

func buildEffectiveFixReviewsConfig(
	globalDefaults DefaultsConfig,
	global FixReviewsConfig,
	workspaceDefaults DefaultsConfig,
	workspace FixReviewsConfig,
) FixReviewsConfig {
	return FixReviewsConfig{
		Concurrent:      cloneOptionalValue(preferOverlay(global.Concurrent, workspace.Concurrent)),
		BatchSize:       cloneOptionalValue(preferOverlay(global.BatchSize, workspace.BatchSize)),
		IncludeResolved: cloneOptionalValue(preferOverlay(global.IncludeResolved, workspace.IncludeResolved)),
		OutputFormat: effectiveCommandOverride(
			globalDefaults.OutputFormat,
			global.OutputFormat,
			workspaceDefaults.OutputFormat,
			workspace.OutputFormat,
		),
		TUI:   cloneOptionalValue(preferOverlay(global.TUI, workspace.TUI)),
		Stall: effectiveStallOverride(globalDefaults.Stall, global.Stall, workspaceDefaults.Stall, workspace.Stall),
	}
}

func buildEffectiveExecConfig(
	globalDefaults DefaultsConfig,
	global ExecConfig,
	workspaceDefaults DefaultsConfig,
	workspace ExecConfig,
) ExecConfig {
	return ExecConfig{
		RuntimeOverrides: buildEffectiveExecRuntimeOverrides(
			globalDefaults,
			global.RuntimeOverrides,
			workspaceDefaults,
			workspace.RuntimeOverrides,
		),
		Verbose: cloneOptionalValue(preferOverlay(global.Verbose, workspace.Verbose)),
		TUI:     cloneOptionalValue(preferOverlay(global.TUI, workspace.TUI)),
		Persist: cloneOptionalValue(preferOverlay(global.Persist, workspace.Persist)),
	}
}

func buildEffectiveExecRuntimeOverrides(
	globalDefaults DefaultsConfig,
	global RuntimeOverrides,
	workspaceDefaults DefaultsConfig,
	workspace RuntimeOverrides,
) RuntimeOverrides {
	return RuntimeOverrides{
		IDE: effectiveCommandOverride(globalDefaults.IDE, global.IDE, workspaceDefaults.IDE, workspace.IDE),
		Model: effectiveCommandOverride(
			globalDefaults.Model, global.Model, workspaceDefaults.Model, workspace.Model,
		),
		OutputFormat: effectiveCommandOverride(
			globalDefaults.OutputFormat, global.OutputFormat, workspaceDefaults.OutputFormat, workspace.OutputFormat,
		),
		ReasoningEffort: effectiveCommandOverride(
			globalDefaults.ReasoningEffort,
			global.ReasoningEffort,
			workspaceDefaults.ReasoningEffort,
			workspace.ReasoningEffort,
		),
		AccessMode: effectiveCommandOverride(
			globalDefaults.AccessMode, global.AccessMode, workspaceDefaults.AccessMode, workspace.AccessMode,
		),
		Timeout: effectiveCommandOverride(
			globalDefaults.Timeout, global.Timeout, workspaceDefaults.Timeout, workspace.Timeout,
		),
		TailLines: effectiveCommandOverride(
			globalDefaults.TailLines, global.TailLines, workspaceDefaults.TailLines, workspace.TailLines,
		),
		AddDirs: effectiveCommandSliceOverride(
			globalDefaults.AddDirs, global.AddDirs, workspaceDefaults.AddDirs, workspace.AddDirs,
		),
		AutoCommit: effectiveCommandOverride(
			globalDefaults.AutoCommit, global.AutoCommit, workspaceDefaults.AutoCommit, workspace.AutoCommit,
		),
		MaxRetries: effectiveCommandOverride(
			globalDefaults.MaxRetries, global.MaxRetries, workspaceDefaults.MaxRetries, workspace.MaxRetries,
		),
		RetryBackoffMultiplier: effectiveCommandOverride(
			globalDefaults.RetryBackoffMultiplier,
			global.RetryBackoffMultiplier,
			workspaceDefaults.RetryBackoffMultiplier,
			workspace.RetryBackoffMultiplier,
		),
		Stall: effectiveStallOverride(globalDefaults.Stall, global.Stall, workspaceDefaults.Stall, workspace.Stall),
	}
}

// effectiveStallOverride resolves stall knobs for a command surface, applying
// the same defaults-vs-command precedence as effectiveCommandOverride per field.
func effectiveStallOverride(globalDefaults, global, workspaceDefaults, workspace StallOverrides) StallOverrides {
	return StallOverrides{
		Enabled: effectiveCommandOverride(
			globalDefaults.Enabled, global.Enabled, workspaceDefaults.Enabled, workspace.Enabled,
		),
		Timeout: effectiveCommandOverride(
			globalDefaults.Timeout, global.Timeout, workspaceDefaults.Timeout, workspace.Timeout,
		),
		ChildTimeout: effectiveCommandOverride(
			globalDefaults.ChildTimeout,
			global.ChildTimeout,
			workspaceDefaults.ChildTimeout,
			workspace.ChildTimeout,
		),
		TerminalCommandTimeout: effectiveCommandOverride(
			globalDefaults.TerminalCommandTimeout,
			global.TerminalCommandTimeout,
			workspaceDefaults.TerminalCommandTimeout,
			workspace.TerminalCommandTimeout,
		),
		Retries: effectiveCommandOverride(
			globalDefaults.Retries, global.Retries, workspaceDefaults.Retries, workspace.Retries,
		),
	}
}

func mergeRunsConfig(base, overlay RunsConfig) RunsConfig {
	return RunsConfig{
		DefaultAttachMode: cloneOptionalValue(preferOverlay(base.DefaultAttachMode, overlay.DefaultAttachMode)),
		KeepTerminalDays:  cloneOptionalValue(preferOverlay(base.KeepTerminalDays, overlay.KeepTerminalDays)),
		KeepMax:           cloneOptionalValue(preferOverlay(base.KeepMax, overlay.KeepMax)),
		ShutdownDrainTimeout: cloneOptionalValue(
			preferOverlay(base.ShutdownDrainTimeout, overlay.ShutdownDrainTimeout),
		),
	}
}

func mergeRecoveryConfig(base, overlay AgentRecoveryConfig) AgentRecoveryConfig {
	return AgentRecoveryConfig{
		Enabled:         cloneOptionalValue(preferOverlay(base.Enabled, overlay.Enabled)),
		IDE:             cloneOptionalValue(preferOverlay(base.IDE, overlay.IDE)),
		Model:           cloneOptionalValue(preferOverlay(base.Model, overlay.Model)),
		ReasoningEffort: cloneOptionalValue(preferOverlay(base.ReasoningEffort, overlay.ReasoningEffort)),
		MaxAttempts:     cloneOptionalValue(preferOverlay(base.MaxAttempts, overlay.MaxAttempts)),
	}
}

func mergeParallelTasksConfig(base, overlay ParallelTasksConfig) ParallelTasksConfig {
	return ParallelTasksConfig{
		Enabled:          cloneOptionalValue(preferOverlay(base.Enabled, overlay.Enabled)),
		MaxConcurrency:   cloneOptionalValue(preferOverlay(base.MaxConcurrency, overlay.MaxConcurrency)),
		ConflictResolver: mergeConflictResolverConfigPointer(base.ConflictResolver, overlay.ConflictResolver),
	}
}

func mergeParallelTaskGroupsConfig(
	base ParallelTaskGroupsConfig,
	overlay ParallelTaskGroupsConfig,
) ParallelTaskGroupsConfig {
	return ParallelTaskGroupsConfig{
		BranchTemplate: cloneOptionalValue(preferOverlay(base.BranchTemplate, overlay.BranchTemplate)),
	}
}

func mergeConflictResolverConfig(base, overlay ConflictResolverConfig) ConflictResolverConfig {
	merged := ConflictResolverConfig{
		Enabled:         cloneOptionalValue(preferOverlay(base.Enabled, overlay.Enabled)),
		IDE:             cloneOptionalValue(preferOverlay(base.IDE, overlay.IDE)),
		Model:           cloneOptionalValue(preferOverlay(base.Model, overlay.Model)),
		ReasoningEffort: cloneOptionalValue(preferOverlay(base.ReasoningEffort, overlay.ReasoningEffort)),
		MaxAttempts:     cloneOptionalValue(preferOverlay(base.MaxAttempts, overlay.MaxAttempts)),
	}
	if overlay.ValidationCommand != nil {
		merged.ValidationCommand = cloneStringSlicePointer(overlay.ValidationCommand)
	} else {
		merged.ValidationCommand = cloneStringSlicePointer(base.ValidationCommand)
	}
	return merged
}

func mergeConflictResolverConfigPointer(base, overlay *ConflictResolverConfig) *ConflictResolverConfig {
	if base == nil && overlay == nil {
		return nil
	}
	baseConfig := ConflictResolverConfig{}
	if base != nil {
		baseConfig = *base
	}
	overlayConfig := ConflictResolverConfig{}
	if overlay != nil {
		overlayConfig = *overlay
	}
	merged := mergeConflictResolverConfig(baseConfig, overlayConfig)
	return &merged
}

func mergeSoundConfig(base, overlay SoundConfig) SoundConfig {
	return SoundConfig{
		Enabled:     cloneOptionalValue(preferOverlay(base.Enabled, overlay.Enabled)),
		OnCompleted: cloneOptionalValue(preferOverlay(base.OnCompleted, overlay.OnCompleted)),
		OnFailed:    cloneOptionalValue(preferOverlay(base.OnFailed, overlay.OnFailed)),
		OnParked:    cloneOptionalValue(preferOverlay(base.OnParked, overlay.OnParked)),
	}
}

func mergeTaskRunRuntimeRules(
	base *[]model.TaskRuntimeRule,
	overlay *[]model.TaskRuntimeRule,
) *[]model.TaskRuntimeRule {
	if base == nil && overlay == nil {
		return nil
	}

	merged := make([]model.TaskRuntimeRule, 0)
	indexByType := make(map[string]int)
	appendRule := func(rule model.TaskRuntimeRule) {
		normalized := rule
		if normalized.Type == nil {
			merged = append(merged, normalized)
			return
		}
		key := strings.TrimSpace(*normalized.Type)
		if existing, ok := indexByType[key]; ok {
			merged[existing] = normalized
			return
		}
		indexByType[key] = len(merged)
		merged = append(merged, normalized)
	}

	for _, rule := range model.CloneTaskRuntimeRules(derefTaskRuntimeRules(base)) {
		appendRule(rule)
	}
	for _, rule := range model.CloneTaskRuntimeRules(derefTaskRuntimeRules(overlay)) {
		appendRule(rule)
	}
	if len(merged) == 0 {
		return nil
	}
	return &merged
}

func mergeRuntimeOverrides(base, overlay RuntimeOverrides) RuntimeOverrides {
	return RuntimeOverrides{
		IDE:             cloneOptionalValue(preferOverlay(base.IDE, overlay.IDE)),
		Model:           cloneOptionalValue(preferOverlay(base.Model, overlay.Model)),
		OutputFormat:    cloneOptionalValue(preferOverlay(base.OutputFormat, overlay.OutputFormat)),
		ReasoningEffort: cloneOptionalValue(preferOverlay(base.ReasoningEffort, overlay.ReasoningEffort)),
		AccessMode:      cloneOptionalValue(preferOverlay(base.AccessMode, overlay.AccessMode)),
		Timeout:         cloneOptionalValue(preferOverlay(base.Timeout, overlay.Timeout)),
		TailLines:       cloneOptionalValue(preferOverlay(base.TailLines, overlay.TailLines)),
		AddDirs:         cloneStringSlicePointer(preferOverlay(base.AddDirs, overlay.AddDirs)),
		AutoCommit:      cloneOptionalValue(preferOverlay(base.AutoCommit, overlay.AutoCommit)),
		MaxRetries:      cloneOptionalValue(preferOverlay(base.MaxRetries, overlay.MaxRetries)),
		RetryBackoffMultiplier: cloneOptionalValue(
			preferOverlay(base.RetryBackoffMultiplier, overlay.RetryBackoffMultiplier),
		),
		Stall: mergeStallOverrides(base.Stall, overlay.Stall),
	}
}

func mergeStallOverrides(base, overlay StallOverrides) StallOverrides {
	return StallOverrides{
		Enabled:      cloneOptionalValue(preferOverlay(base.Enabled, overlay.Enabled)),
		Timeout:      cloneOptionalValue(preferOverlay(base.Timeout, overlay.Timeout)),
		ChildTimeout: cloneOptionalValue(preferOverlay(base.ChildTimeout, overlay.ChildTimeout)),
		TerminalCommandTimeout: cloneOptionalValue(
			preferOverlay(base.TerminalCommandTimeout, overlay.TerminalCommandTimeout),
		),
		Retries: cloneOptionalValue(preferOverlay(base.Retries, overlay.Retries)),
	}
}

func derefTaskRuntimeRules(value *[]model.TaskRuntimeRule) []model.TaskRuntimeRule {
	if value == nil {
		return nil
	}
	return *value
}

func normalizeProjectConfigPaths(cfg ProjectConfig, baseDir string) (ProjectConfig, error) {
	defaultsAddDirs, err := resolveConfigAddDirs(cfg.Defaults.AddDirs, baseDir)
	if err != nil {
		return ProjectConfig{}, fmt.Errorf("resolve defaults.add_dirs: %w", err)
	}
	execAddDirs, err := resolveConfigAddDirs(cfg.Exec.AddDirs, baseDir)
	if err != nil {
		return ProjectConfig{}, fmt.Errorf("resolve exec.add_dirs: %w", err)
	}

	cfg.Defaults.AddDirs = defaultsAddDirs
	cfg.Exec.AddDirs = execAddDirs
	return cfg, nil
}

func resolveConfigAddDirs(addDirs *[]string, baseDir string) (*[]string, error) {
	if addDirs == nil {
		return nil, nil
	}

	resolvedBaseDir, err := resolveConfigBaseDir(baseDir)
	if err != nil {
		return nil, err
	}

	resolved := make([]string, 0, len(*addDirs))
	for _, dir := range *addDirs {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" {
			resolved = append(resolved, "")
			continue
		}
		if filepath.IsAbs(trimmed) {
			resolved = append(resolved, filepath.Clean(trimmed))
			continue
		}
		resolved = append(resolved, filepath.Join(resolvedBaseDir, trimmed))
	}
	return &resolved, nil
}

func resolveConfigBaseDir(baseDir string) (string, error) {
	trimmed := strings.TrimSpace(baseDir)
	if trimmed == "" {
		return "", fmt.Errorf("base directory is empty")
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed), nil
	}
	absBaseDir, err := filepath.Abs(trimmed)
	if err != nil {
		return "", fmt.Errorf("resolve base directory %q: %w", trimmed, err)
	}
	return absBaseDir, nil
}

func effectiveCommandOverride[T any](
	globalDefault *T,
	globalCommand *T,
	workspaceDefault *T,
	workspaceCommand *T,
) *T {
	if workspaceCommand != nil {
		return cloneOptionalValue(workspaceCommand)
	}
	if workspaceDefault != nil {
		return nil
	}
	if globalCommand != nil {
		return cloneOptionalValue(globalCommand)
	}
	if globalDefault != nil {
		return nil
	}
	return nil
}

func effectiveCommandSliceOverride(
	globalDefault *[]string,
	globalCommand *[]string,
	workspaceDefault *[]string,
	workspaceCommand *[]string,
) *[]string {
	if workspaceCommand != nil {
		return cloneStringSlicePointer(workspaceCommand)
	}
	if workspaceDefault != nil {
		return nil
	}
	if globalCommand != nil {
		return cloneStringSlicePointer(globalCommand)
	}
	if globalDefault != nil {
		return nil
	}
	return nil
}

func preferOverlay[T any](base, overlay *T) *T {
	if overlay != nil {
		return overlay
	}
	return base
}

func cloneOptionalValue[T any](value *T) *T {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneStringSlicePointer(value *[]string) *[]string {
	if value == nil {
		return nil
	}
	cloned := cloneStringSlice(*value)
	return &cloned
}

func cloneStringSlice(value []string) []string {
	if value == nil {
		return nil
	}
	return append([]string(nil), value...)
}
