package cli

import (
	"context"
	"fmt"
	"slices"

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

func discoverWorkspaceRoot(ctx context.Context) (string, error) {
	root, err := workspace.Discover(ctx, "")
	if err != nil {
		return "", fmt.Errorf("discover workspace: %w", err)
	}
	return root, nil
}

func loadWorkspaceProjectConfig(ctx context.Context, root string) (workspace.ProjectConfig, error) {
	cfg, _, err := workspace.LoadConfig(ctx, root)
	if err != nil {
		return workspace.ProjectConfig{}, fmt.Errorf("load workspace config: %w", err)
	}
	return cfg, nil
}

func (s *commandState) applyWorkspaceDefaults(ctx context.Context, cmd *cobra.Command) error {
	root, err := discoverWorkspaceRoot(ctx)
	if err != nil {
		return err
	}
	cfg, err := loadWorkspaceProjectConfig(ctx, root)
	if err != nil {
		return err
	}

	s.workspaceRoot = root
	s.projectConfig = cfg
	s.applyProjectConfig(cmd, cfg)
	return nil
}

func (s *simpleCommandBase) loadWorkspaceRoot(ctx context.Context) error {
	root, err := discoverWorkspaceRoot(ctx)
	if err != nil {
		return err
	}
	cfg, err := loadWorkspaceProjectConfig(ctx, root)
	if err != nil {
		return err
	}
	s.workspaceRoot = root
	s.projectConfig = cfg
	return nil
}

func (s *commandState) prepareWorkspaceContext(
	ctx context.Context,
	cmd *cobra.Command,
) (declarativeAssets, func(), error) {
	root, err := discoverWorkspaceRoot(ctx)
	if err != nil {
		return declarativeAssets{}, nil, err
	}
	s.workspaceRoot = root

	assets, cleanup, err := s.bootstrapDeclarativeAssetsForWorkspaceRoot(ctx, root, commandPath(cmd))
	if err != nil {
		return declarativeAssets{}, nil, err
	}

	cfg, err := loadWorkspaceProjectConfig(ctx, root)
	if err != nil {
		cleanup()
		return declarativeAssets{}, nil, err
	}

	s.projectConfig = cfg
	s.applyProjectConfig(cmd, cfg)
	return assets, cleanup, nil
}

func commandPath(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	return cmd.CommandPath()
}

func (s *commandState) applyProjectConfig(cmd *cobra.Command, cfg workspace.ProjectConfig) {
	applyConfig(cmd, "ide", cfg.Defaults.IDE, func(val string) { s.ide = val })
	applyConfig(cmd, "model", cfg.Defaults.Model, func(val string) { s.model = val })
	applyConfig(cmd, "format", cfg.Defaults.OutputFormat, func(val string) { s.outputFormat = val })
	applyConfig(cmd, "reasoning-effort", cfg.Defaults.ReasoningEffort, func(val string) {
		s.reasoningEffort = val
	})
	applyConfig(cmd, "access-mode", cfg.Defaults.AccessMode, func(val string) { s.accessMode = val })
	applyConfig(cmd, "timeout", cfg.Defaults.Timeout, func(val string) { s.timeout = val })
	applyConfig(cmd, "tail-lines", cfg.Defaults.TailLines, func(val int) { s.tailLines = val })
	applyConfig(cmd, "add-dir", cfg.Defaults.AddDirs, func(val []string) { s.addDirs = val }, slices.Clone[[]string])
	applyConfig(cmd, "auto-commit", cfg.Defaults.AutoCommit, func(val bool) { s.autoCommit = val })
	applyConfig(cmd, "max-retries", cfg.Defaults.MaxRetries, func(val int) { s.maxRetries = val })
	applyConfig(
		cmd,
		"retry-backoff-multiplier",
		cfg.Defaults.RetryBackoffMultiplier,
		func(val float64) { s.retryBackoffMultiplier = val },
	)

	switch s.kind {
	case commandKindStart:
		applyConfig(cmd, "format", cfg.Start.OutputFormat, func(val string) { s.outputFormat = val })
		applyConfig(cmd, "tui", cfg.Start.TUI, func(val bool) { s.tui = val })
		applyConfig(
			cmd,
			"include-completed",
			cfg.Start.IncludeCompleted,
			func(val bool) { s.includeCompleted = val },
		)
	case commandKindFixReviews:
		applyConfig(cmd, "format", cfg.FixReviews.OutputFormat, func(val string) { s.outputFormat = val })
		applyConfig(cmd, "tui", cfg.FixReviews.TUI, func(val bool) { s.tui = val })
		applyConfig(cmd, "concurrent", cfg.FixReviews.Concurrent, func(val int) { s.concurrent = val })
		applyConfig(cmd, "batch-size", cfg.FixReviews.BatchSize, func(val int) { s.batchSize = val })
		applyConfig(
			cmd,
			"include-resolved",
			cfg.FixReviews.IncludeResolved,
			func(val bool) { s.includeResolved = val },
		)
	case commandKindFetchReviews:
		applyConfig(cmd, "provider", cfg.FetchReviews.Provider, func(val string) { s.provider = val })
		applyConfig(cmd, "nitpicks", cfg.FetchReviews.Nitpicks, func(val bool) { s.nitpicks = val })
	case commandKindExec:
		applyConfig(cmd, "ide", cfg.Exec.IDE, func(val string) { s.ide = val })
		applyConfig(cmd, "model", cfg.Exec.Model, func(val string) { s.model = val })
		applyConfig(cmd, "format", cfg.Exec.OutputFormat, func(val string) { s.outputFormat = val })
		applyConfig(cmd, "verbose", cfg.Exec.Verbose, func(val bool) { s.verbose = val })
		applyConfig(cmd, "tui", cfg.Exec.TUI, func(val bool) { s.tui = val })
		applyConfig(cmd, "persist", cfg.Exec.Persist, func(val bool) { s.persist = val })
		applyConfig(cmd, "reasoning-effort", cfg.Exec.ReasoningEffort, func(val string) {
			s.reasoningEffort = val
		})
		applyConfig(cmd, "access-mode", cfg.Exec.AccessMode, func(val string) { s.accessMode = val })
		applyConfig(cmd, "timeout", cfg.Exec.Timeout, func(val string) { s.timeout = val })
		applyConfig(cmd, "tail-lines", cfg.Exec.TailLines, func(val int) { s.tailLines = val })
		applyConfig(cmd, "add-dir", cfg.Exec.AddDirs, func(val []string) { s.addDirs = val }, slices.Clone[[]string])
		applyConfig(cmd, "auto-commit", cfg.Exec.AutoCommit, func(val bool) { s.autoCommit = val })
		applyConfig(cmd, "max-retries", cfg.Exec.MaxRetries, func(val int) { s.maxRetries = val })
		applyConfig(
			cmd,
			"retry-backoff-multiplier",
			cfg.Exec.RetryBackoffMultiplier,
			func(val float64) { s.retryBackoffMultiplier = val },
		)
	}
}

func applyConfig[T any](cmd *cobra.Command, flagName string, value *T, setter func(T), transform ...func(T) T) {
	if value == nil || cmd.Flags().Lookup(flagName) == nil || cmd.Flags().Changed(flagName) {
		return
	}

	resolved := *value
	if len(transform) > 0 && transform[0] != nil {
		resolved = transform[0](resolved)
	}
	setter(resolved)
}
