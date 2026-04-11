package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	core "github.com/compozy/compozy/internal/core"
	reusableagents "github.com/compozy/compozy/internal/core/agents"
	coreRun "github.com/compozy/compozy/internal/core/run"
	"github.com/spf13/cobra"
)

func (s *commandState) run(cmd *cobra.Command, _ []string) error {
	return s.prepareAndRun(cmd, func(cmd *cobra.Command) error {
		return s.maybeCollectInteractiveParams(cmd)
	}, false)
}

func (s *commandState) exec(cmd *cobra.Command, args []string) error {
	return s.prepareAndRun(cmd, func(cmd *cobra.Command) error {
		return s.resolveExecPromptSource(cmd, args)
	}, true)
}

func (s *commandState) prepareAndRun(
	cmd *cobra.Command,
	setupFn func(*cobra.Command) error,
	handleSetupErrors bool,
) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	assets, cleanup, err := s.prepareWorkspaceContext(ctx, cmd)
	if err != nil {
		wrapped := fmt.Errorf("apply workspace defaults for %s: %w", cmd.Name(), err)
		if handleSetupErrors {
			return s.handleExecError(cmd, wrapped)
		}
		return wrapped
	}
	defer cleanup()
	if setupFn != nil {
		if err := setupFn(cmd); err != nil {
			if handleSetupErrors {
				return s.handleExecError(cmd, err)
			}
			return err
		}
	}
	if err := s.normalizePresentationMode(cmd); err != nil {
		if handleSetupErrors {
			return s.handleExecError(cmd, err)
		}
		return err
	}
	s.explicitRuntime = captureExplicitRuntimeFlags(cmd)

	cfg, err := s.buildConfig()
	if err != nil {
		return s.handleExecError(cmd, err)
	}
	if err := s.applyPersistedExecConfig(cmd, &cfg); err != nil {
		return s.handleExecError(cmd, err)
	}
	if err := cfg.Validate(); err != nil {
		return s.handleExecError(cmd, err)
	}

	if err := s.runPrepared(ctx, cmd, cfg, assets); err != nil {
		return s.handleExecError(cmd, decorateReusableAgentError(cmd, cfg.AgentName, err))
	}
	return nil
}

func (s *commandState) fetchReviews(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	_, cleanup, err := s.prepareWorkspaceContext(ctx, cmd)
	if err != nil {
		return fmt.Errorf("apply workspace defaults for %s: %w", cmd.Name(), err)
	}
	defer cleanup()
	if err := s.maybeCollectInteractiveParams(cmd); err != nil {
		return err
	}

	cfg, err := s.buildConfig()
	if err != nil {
		return err
	}

	fetchReviewsFn := s.fetchReviewsFn
	if fetchReviewsFn == nil {
		fetchReviewsFn = core.FetchReviews
	}

	result, err := fetchReviewsFn(ctx, cfg)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"Fetched %d review issues from %s for PR %s into %s (round %03d)\n",
		result.Total,
		result.Provider,
		result.PR,
		result.ReviewsDir,
		result.Round,
	); err != nil {
		return fmt.Errorf("write fetch summary: %w", err)
	}
	return nil
}

func (s *commandState) runPrepared(
	ctx context.Context,
	cmd *cobra.Command,
	cfg core.Config,
	assets ...declarativeAssets,
) error {
	var discovery declarativeAssets
	if len(assets) > 0 {
		discovery = assets[0]
	}

	if err := s.preflightBundledSkills(
		cmd,
		cfg,
		extensionSkillSources(discovery.Discovery.SkillPacks.Packs),
	); err != nil {
		return err
	}
	if err := s.preflightTaskMetadata(ctx, cmd, cfg); err != nil {
		return err
	}

	runWorkflow := s.runWorkflow
	if runWorkflow == nil {
		runWorkflow = core.Run
	}
	return runWorkflow(ctx, cfg)
}

func decorateReusableAgentError(cmd *cobra.Command, agentName string, err error) error {
	if err == nil || strings.TrimSpace(agentName) == "" {
		return err
	}

	rootPath := "compozy"
	if cmd != nil && cmd.Root() != nil {
		rootPath = cmd.Root().CommandPath()
	}

	if reason, ok := reusableagents.BlockedReasonForError(err); ok {
		err = fmt.Errorf("reusable agent blocked (%s): %w", reason, err)
	}

	switch {
	case errors.Is(err, reusableagents.ErrAgentNotFound):
		return fmt.Errorf("%w; run `%s agents list` to inspect available reusable agents", err, rootPath)
	case isReusableAgentValidationError(err):
		return fmt.Errorf(
			"%w; run `%s agents inspect %s` to inspect the resolved definition and validation details",
			err,
			rootPath,
			strings.TrimSpace(agentName),
		)
	default:
		return err
	}
}

func isReusableAgentValidationError(err error) bool {
	return reusableagents.IsValidationError(err)
}

func (s *commandState) preflightTaskMetadata(ctx context.Context, cmd *cobra.Command, cfg core.Config) error {
	if s.kind != commandKindStart || cfg.Mode != core.ModePRDTasks {
		return nil
	}

	preflightCfg := coreRun.PreflightConfig{
		Force:          s.force,
		SkipValidation: s.skipValidation,
		IsInteractive:  s.isInteractive,
		Stderr:         cmd.ErrOrStderr(),
		Logger:         slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), nil)),
	}
	if !s.skipValidation {
		registry, err := taskTypeRegistryFromConfig(s.projectConfig)
		if err != nil {
			return fmt.Errorf("resolve task type registry: %w", err)
		}
		resolvedTasksDir, err := resolveTaskWorkflowDir(s.workspaceRoot, cfg.Name, cfg.TasksDir)
		if err != nil {
			return err
		}
		preflightCfg.TasksDir = resolvedTasksDir
		preflightCfg.Registry = registry
	}

	decision, err := coreRun.PreflightCheckConfig(ctx, preflightCfg)
	if err != nil {
		return err
	}
	if decision == coreRun.PreflightAborted {
		return withExitCode(1, fmt.Errorf("task validation failed"))
	}
	return nil
}
