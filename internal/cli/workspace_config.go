package cli

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/spf13/cobra"
)

func resolveWorkspaceContext(ctx context.Context) (workspace.Context, error) {
	workspaceCtx, err := workspace.Resolve(ctx, "")
	if err != nil {
		return workspace.Context{}, fmt.Errorf("resolve workspace: %w", err)
	}
	return workspaceCtx, nil
}

func (s *commandState) applyWorkspaceDefaults(ctx context.Context, cmd *cobra.Command) error {
	workspaceCtx, err := resolveWorkspaceContext(ctx)
	if err != nil {
		return err
	}

	s.workspaceRoot = workspaceCtx.Root
	s.projectConfig = workspaceCtx.Config
	s.applyProjectConfig(cmd, workspaceCtx.Config)
	return nil
}

func (s *migrateCommandState) loadWorkspaceRoot(ctx context.Context) error {
	workspaceCtx, err := resolveWorkspaceContext(ctx)
	if err != nil {
		return err
	}
	s.workspaceRoot = workspaceCtx.Root
	return nil
}

func (s *syncCommandState) loadWorkspaceRoot(ctx context.Context) error {
	workspaceCtx, err := resolveWorkspaceContext(ctx)
	if err != nil {
		return err
	}
	s.workspaceRoot = workspaceCtx.Root
	return nil
}

func (s *archiveCommandState) loadWorkspaceRoot(ctx context.Context) error {
	workspaceCtx, err := resolveWorkspaceContext(ctx)
	if err != nil {
		return err
	}
	s.workspaceRoot = workspaceCtx.Root
	return nil
}

func (s *commandState) applyProjectConfig(cmd *cobra.Command, cfg workspace.ProjectConfig) {
	applyStringConfig(cmd, "ide", cfg.Defaults.IDE, func(val string) { s.ide = val })
	applyStringConfig(cmd, "model", cfg.Defaults.Model, func(val string) { s.model = val })
	applyStringConfig(cmd, "reasoning-effort", cfg.Defaults.ReasoningEffort, func(val string) {
		s.reasoningEffort = val
	})
	applyStringConfig(cmd, "timeout", cfg.Defaults.Timeout, func(val string) { s.timeout = val })
	applyIntConfig(cmd, "tail-lines", cfg.Defaults.TailLines, func(val int) { s.tailLines = val })
	applyStringSliceConfig(cmd, "add-dir", cfg.Defaults.AddDirs, func(val []string) { s.addDirs = val })
	applyBoolConfig(cmd, "auto-commit", cfg.Defaults.AutoCommit, func(val bool) { s.autoCommit = val })
	applyIntConfig(cmd, "max-retries", cfg.Defaults.MaxRetries, func(val int) { s.maxRetries = val })
	applyFloat64Config(
		cmd,
		"retry-backoff-multiplier",
		cfg.Defaults.RetryBackoffMultiplier,
		func(val float64) { s.retryBackoffMultiplier = val },
	)

	switch s.kind {
	case commandKindStart:
		applyBoolConfig(
			cmd,
			"include-completed",
			cfg.Start.IncludeCompleted,
			func(val bool) { s.includeCompleted = val },
		)
	case commandKindFixReviews:
		applyIntConfig(cmd, "concurrent", cfg.FixReviews.Concurrent, func(val int) { s.concurrent = val })
		applyIntConfig(cmd, "batch-size", cfg.FixReviews.BatchSize, func(val int) { s.batchSize = val })
		applyBoolConfig(cmd, "grouped", cfg.FixReviews.Grouped, func(val bool) { s.grouped = val })
		applyBoolConfig(
			cmd,
			"include-resolved",
			cfg.FixReviews.IncludeResolved,
			func(val bool) { s.includeResolved = val },
		)
	case commandKindFetchReviews:
		applyStringConfig(cmd, "provider", cfg.FetchReviews.Provider, func(val string) { s.provider = val })
	}
}

func applyStringConfig(cmd *cobra.Command, flagName string, value *string, setter func(string)) {
	if value == nil || cmd.Flags().Lookup(flagName) == nil || cmd.Flags().Changed(flagName) {
		return
	}
	setter(*value)
}

func applyIntConfig(cmd *cobra.Command, flagName string, value *int, setter func(int)) {
	if value == nil || cmd.Flags().Lookup(flagName) == nil || cmd.Flags().Changed(flagName) {
		return
	}
	setter(*value)
}

func applyFloat64Config(cmd *cobra.Command, flagName string, value *float64, setter func(float64)) {
	if value == nil || cmd.Flags().Lookup(flagName) == nil || cmd.Flags().Changed(flagName) {
		return
	}
	setter(*value)
}

func applyBoolConfig(cmd *cobra.Command, flagName string, value *bool, setter func(bool)) {
	if value == nil || cmd.Flags().Lookup(flagName) == nil || cmd.Flags().Changed(flagName) {
		return
	}
	setter(*value)
}

func applyStringSliceConfig(cmd *cobra.Command, flagName string, value *[]string, setter func([]string)) {
	if value == nil || cmd.Flags().Lookup(flagName) == nil || cmd.Flags().Changed(flagName) {
		return
	}
	resolved := append([]string(nil), (*value)...)
	setter(resolved)
}
