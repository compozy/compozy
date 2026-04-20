package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	apiclient "github.com/compozy/compozy/internal/api/client"
	apicore "github.com/compozy/compozy/internal/api/core"
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
	coreRun "github.com/compozy/compozy/internal/core/run"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"
	"github.com/spf13/cobra"
)

const (
	execStatusCompleted = "completed"
	execStatusSucceeded = "succeeded"
	execStatusFailed    = "failed"
	execStatusCanceled  = "canceled"
	execStatusCrashed   = "crashed"

	execEventRunStarted      = "run.started"
	execEventSessionAttached = "session.attached"
	execEventRunSucceeded    = "run.succeeded"
	execEventRunFailed       = "run.failed"

	daemonRunTerminalPollInterval = 10 * time.Millisecond
)

func newReviewsCommand() *cobra.Command {
	return newReviewsCommandWithDefaults(defaultCommandStateDefaults())
}

func newReviewsCommandWithDefaults(defaults commandStateDefaults) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "reviews",
		Short:        "Fetch, inspect, and remediate review workflows",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newReviewsFetchCommandWithDefaults(defaults),
		newReviewsListCommandWithDefaults(defaults),
		newReviewsShowCommandWithDefaults(defaults),
		newReviewsFixCommandWithDefaults(defaults),
	)
	return cmd
}

func newReviewsFetchCommandWithDefaults(defaults commandStateDefaults) *cobra.Command {
	state := newCommandStateWithDefaults(commandKindFetchReviews, core.ModePRReview, defaults)
	cmd := &cobra.Command{
		Use:          "fetch [slug]",
		Short:        "Import provider feedback into a review round",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Long: "Fetch review comments from a provider and write them into .compozy/tasks/<name>/reviews-NNN/.\n\n" +
			"When --provider is omitted, Compozy can load its default from ~/.compozy/config.toml or .compozy/config.toml.",
		Example: `  compozy reviews fetch my-feature --provider coderabbit --pr 259
  compozy reviews fetch --name my-feature --provider coderabbit --pr 259 --round 2
  compozy reviews fetch --name my-feature`,
		RunE: state.fetchReviewsDaemon,
	}
	cmd.Flags().StringVar(&state.provider, "provider", "", "Review provider name")
	cmd.Flags().StringVar(&state.pr, "pr", "", "Pull request number")
	cmd.Flags().StringVar(&state.name, "name", "", "Workflow name (used for .compozy/tasks/<name>)")
	cmd.Flags().IntVar(&state.round, "round", 0, "Review round number (default: next available round)")
	return cmd
}

func newReviewsListCommandWithDefaults(defaults commandStateDefaults) *cobra.Command {
	state := newCommandStateWithDefaults(commandKindFetchReviews, core.ModePRReview, defaults)
	cmd := &cobra.Command{
		Use:          "list [slug]",
		Short:        "Show the latest daemon-backed review summary for a workflow",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE:         state.listReviewsDaemon,
	}
	cmd.Flags().StringVar(&state.name, "name", "", "Workflow name (used for .compozy/tasks/<name>)")
	return cmd
}

func newReviewsShowCommandWithDefaults(defaults commandStateDefaults) *cobra.Command {
	state := newCommandStateWithDefaults(commandKindFetchReviews, core.ModePRReview, defaults)
	cmd := &cobra.Command{
		Use:          "show [slug] [round]",
		Short:        "Show one daemon-backed review round and its issue rows",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(2),
		RunE:         state.showReviewsDaemon,
	}
	cmd.Flags().StringVar(&state.name, "name", "", "Workflow name (used for .compozy/tasks/<name>)")
	cmd.Flags().IntVar(&state.round, "round", 0, "Review round number")
	return cmd
}

func newReviewsFixCommandWithDefaults(defaults commandStateDefaults) *cobra.Command {
	state := newCommandStateWithDefaults(commandKindFixReviews, core.ModePRReview, defaults)
	cmd := &cobra.Command{
		Use:          "fix [slug]",
		Short:        "Start a daemon-backed review-fix run",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Long: `Process review issue markdown files from .compozy/tasks/<name>/reviews-NNN/ and run the configured AI agent
to remediate review feedback.

Most runtime defaults can be supplied by ~/.compozy/config.toml and overridden by
.compozy/config.toml. In interactive terminals the command
opens the run cockpit by default; in non-TTY environments it falls back to headless streaming.`,
		Example: `  compozy reviews fix my-feature --ide codex --concurrent 2 --batch-size 3
  compozy reviews fix --name my-feature --round 2
  compozy reviews fix my-feature --format json --round 2
  compozy reviews fix --reviews-dir .compozy/tasks/my-feature/reviews-001
  compozy reviews fix --name my-feature`,
		RunE: state.runReviewWorkflowDaemon,
	}
	addCommonFlags(cmd, state, commonFlagOptions{includeConcurrent: true})
	addWorkflowOutputFlags(cmd, state)
	cmd.Flags().StringVar(&state.name, "name", "", "Workflow name (used for .compozy/tasks/<name>)")
	cmd.Flags().IntVar(&state.round, "round", 0, "Review round number (default: latest existing round)")
	cmd.Flags().StringVar(
		&state.reviewsDir,
		"reviews-dir",
		"",
		"Path to a review round directory (.compozy/tasks/<name>/reviews-NNN)",
	)
	cmd.Flags().IntVar(&state.batchSize, "batch-size", 1, "Number of file groups to batch together (default: 1)")
	cmd.Flags().BoolVar(&state.includeResolved, "include-resolved", false, "Include already-resolved review issues")
	cmd.Flags().StringVar(&state.attachMode, "attach", attachModeAuto, "Attach mode: auto, ui, stream, or detach")
	cmd.Flags().Bool("ui", false, "Force interactive UI attach mode")
	cmd.Flags().Bool("stream", false, "Force textual stream attach mode")
	cmd.Flags().Bool("detach", false, "Start the run without attaching a client")
	return cmd
}

func (s *commandState) fetchReviewsDaemon(cmd *cobra.Command, args []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.applyWorkspaceDefaults(ctx, cmd); err != nil {
		return withExitCode(2, fmt.Errorf("apply workspace defaults for %s: %w", cmd.CommandPath(), err))
	}
	if err := s.maybeCollectInteractiveParams(cmd); err != nil {
		return err
	}
	if err := s.resolveWorkflowNameArg(cmd, args); err != nil {
		return withExitCode(1, err)
	}

	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return withExitCode(2, err)
	}

	result, err := client.FetchReview(ctx, s.workspaceRoot, s.name, apicore.ReviewFetchRequest{
		Workspace: s.workspaceRoot,
		Provider:  s.provider,
		PRRef:     s.pr,
		Round:     intPointerOrNil(s.round),
	})
	if err != nil {
		return mapDaemonCommandError(err)
	}

	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"Fetched review issues from %s for PR %s into %s (round %03d)\n",
		result.Summary.Provider,
		result.Summary.PRRef,
		reviewRoundDirForWorkflow(s.workspaceRoot, s.name, result.Summary.RoundNumber),
		result.Summary.RoundNumber,
	); err != nil {
		return withExitCode(2, fmt.Errorf("write fetch summary: %w", err))
	}
	return nil
}

func (s *commandState) listReviewsDaemon(cmd *cobra.Command, args []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.applyWorkspaceDefaults(ctx, cmd); err != nil {
		return withExitCode(2, fmt.Errorf("apply workspace defaults for %s: %w", cmd.CommandPath(), err))
	}
	if err := s.resolveWorkflowNameArg(cmd, args); err != nil {
		return withExitCode(1, err)
	}

	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return withExitCode(2, err)
	}

	review, err := client.GetLatestReview(ctx, s.workspaceRoot, s.name)
	if err != nil {
		return mapDaemonCommandError(err)
	}

	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"%s round %03d | provider=%s pr=%s unresolved=%d resolved=%d\n",
		review.WorkflowSlug,
		review.RoundNumber,
		review.Provider,
		review.PRRef,
		review.UnresolvedCount,
		review.ResolvedCount,
	); err != nil {
		return withExitCode(2, fmt.Errorf("write review summary: %w", err))
	}
	return nil
}

func (s *commandState) showReviewsDaemon(cmd *cobra.Command, args []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.applyWorkspaceDefaults(ctx, cmd); err != nil {
		return withExitCode(2, fmt.Errorf("apply workspace defaults for %s: %w", cmd.CommandPath(), err))
	}
	if err := s.resolveWorkflowNameAndRoundArgs(cmd, args); err != nil {
		return withExitCode(1, err)
	}

	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return withExitCode(2, err)
	}

	round, err := client.GetReviewRound(ctx, s.workspaceRoot, s.name, s.round)
	if err != nil {
		return mapDaemonCommandError(err)
	}
	issues, err := client.ListReviewIssues(ctx, s.workspaceRoot, s.name, s.round)
	if err != nil {
		return mapDaemonCommandError(err)
	}

	if _, err := fmt.Fprintf(
		cmd.OutOrStdout(),
		"%s round %03d | provider=%s pr=%s unresolved=%d resolved=%d\n",
		round.WorkflowSlug,
		round.RoundNumber,
		round.Provider,
		round.PRRef,
		round.UnresolvedCount,
		round.ResolvedCount,
	); err != nil {
		return withExitCode(2, fmt.Errorf("write review round summary: %w", err))
	}
	for _, issue := range issues {
		if _, err := fmt.Fprintf(
			cmd.OutOrStdout(),
			"- issue_%03d | status=%s severity=%s path=%s\n",
			issue.IssueNumber,
			issue.Status,
			issue.Severity,
			issue.SourcePath,
		); err != nil {
			return withExitCode(2, fmt.Errorf("write review issue summary: %w", err))
		}
	}
	return nil
}

func (s *commandState) runReviewWorkflowDaemon(cmd *cobra.Command, args []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	assets, cleanup, err := s.prepareWorkspaceContext(ctx, cmd)
	if err != nil {
		return withExitCode(2, fmt.Errorf("apply workspace defaults for %s: %w", cmd.CommandPath(), err))
	}
	defer cleanup()
	if err := s.maybeCollectInteractiveParams(cmd); err != nil {
		return err
	}
	if err := s.resolveWorkflowNameArg(cmd, args); err != nil {
		return withExitCode(1, err)
	}
	if err := s.resolveReviewRound(ctx); err != nil {
		return withExitCode(1, err)
	}
	s.explicitRuntime = captureExplicitRuntimeFlags(cmd)

	cfg, err := s.buildConfig()
	if err != nil {
		return withExitCode(2, err)
	}

	effectiveExtensionPacks, err := effectiveExtensionSkillSources(assets.Discovery)
	if err != nil {
		return withExitCode(2, err)
	}
	if err := s.preflightBundledSkills(cmd, cfg, effectiveExtensionPacks); err != nil {
		return err
	}

	presentationMode, err := s.resolveTaskPresentationMode(cmd)
	if err != nil {
		return withExitCode(1, err)
	}
	runtimeOverrides, err := s.buildReviewRunRuntimeOverrides(cmd)
	if err != nil {
		return withExitCode(2, err)
	}
	batching, err := s.buildReviewBatchingOverrides(cmd)
	if err != nil {
		return withExitCode(2, err)
	}

	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return withExitCode(2, err)
	}

	run, err := client.StartReviewRun(ctx, s.workspaceRoot, s.name, s.round, apicore.ReviewRunRequest{
		Workspace:        s.workspaceRoot,
		PresentationMode: presentationMode,
		RuntimeOverrides: runtimeOverrides,
		Batching:         batching,
	})
	if err != nil {
		return mapDaemonCommandError(err)
	}

	switch strings.TrimSpace(s.outputFormat) {
	case string(core.OutputFormatJSON):
		return s.streamDaemonWorkflowEvents(ctx, cmd.OutOrStdout(), client, run.RunID, false)
	case string(core.OutputFormatRawJSON):
		return s.streamDaemonWorkflowEvents(ctx, cmd.OutOrStdout(), client, run.RunID, true)
	default:
		return handleStartedTaskRun(ctx, cmd, client, run)
	}
}

func (s *commandState) execDaemon(cmd *cobra.Command, args []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	assets, cleanup, err := s.prepareWorkspaceContext(ctx, cmd)
	if err != nil {
		return s.handleExecError(
			cmd,
			withExitCode(2, fmt.Errorf("apply workspace defaults for %s: %w", cmd.CommandPath(), err)),
		)
	}
	defer cleanup()
	if err := s.resolveExecPromptSource(cmd, args); err != nil {
		return s.handleExecError(cmd, withExitCode(1, err))
	}
	s.explicitRuntime = captureExplicitRuntimeFlags(cmd)

	cfg, err := s.buildConfig()
	if err != nil {
		return s.handleExecError(cmd, withExitCode(2, err))
	}
	if err := s.applyPersistedExecConfig(cmd, &cfg); err != nil {
		return s.handleExecError(cmd, withExitCode(1, err))
	}
	if err := cfg.Validate(); err != nil {
		return s.handleExecError(cmd, withExitCode(1, err))
	}

	effectiveExtensionPacks, err := effectiveExtensionSkillSources(assets.Discovery)
	if err != nil {
		return s.handleExecError(cmd, withExitCode(2, err))
	}
	if err := s.preflightBundledSkills(cmd, cfg, effectiveExtensionPacks); err != nil {
		return s.handleExecError(cmd, err)
	}

	presentationMode, err := s.resolveExecPresentationMode(cmd)
	if err != nil {
		return s.handleExecError(cmd, withExitCode(1, err))
	}
	runtimeOverrides, err := s.buildExecRuntimeOverrides(cmd)
	if err != nil {
		return s.handleExecError(cmd, withExitCode(2, err))
	}

	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return s.handleExecError(cmd, withExitCode(2, err))
	}

	run, err := client.StartExecRun(ctx, apicore.ExecRequest{
		WorkspacePath:    s.workspaceRoot,
		Prompt:           s.resolvedPromptText,
		PresentationMode: presentationMode,
		RuntimeOverrides: runtimeOverrides,
	})
	if err != nil {
		return s.handleExecError(cmd, decorateReusableAgentError(cmd, s.agentName, mapDaemonCommandError(err)))
	}

	switch strings.TrimSpace(s.outputFormat) {
	case string(core.OutputFormatJSON):
		err = s.streamDaemonExecEvents(ctx, cmd.OutOrStdout(), client, run.RunID, false)
	case string(core.OutputFormatRawJSON):
		err = s.streamDaemonExecEvents(ctx, cmd.OutOrStdout(), client, run.RunID, true)
	default:
		err = s.waitAndPrintExecResult(ctx, cmd.OutOrStdout(), client, run.RunID)
	}
	return s.handleExecError(cmd, decorateReusableAgentError(cmd, s.agentName, err))
}

func (s *commandState) resolveExecPresentationMode(cmd *cobra.Command) (string, error) {
	if s.tui {
		isInteractive := s.isInteractive
		if isInteractive == nil {
			isInteractive = isInteractiveTerminal
		}
		if !isInteractive() {
			return "", fmt.Errorf("%s requires an interactive terminal for tui mode", cmd.CommandPath())
		}
		return attachModeUI, nil
	}
	if isJSONOutputFormat(s.outputFormat) {
		return attachModeStream, nil
	}
	return attachModeDetach, nil
}

func (s *commandState) buildReviewRunRuntimeOverrides(cmd *cobra.Command) (json.RawMessage, error) {
	overrides := daemonRuntimeOverrides{}
	hasOverrides := false
	set := func(changed bool, apply func()) {
		if !changed {
			return
		}
		apply()
		hasOverrides = true
	}

	set(commandFlagChanged(cmd, "dry-run"), func() { overrides.DryRun = boolPointer(s.dryRun) })
	set(commandFlagChanged(cmd, "auto-commit"), func() { overrides.AutoCommit = boolPointer(s.autoCommit) })
	set(commandFlagChanged(cmd, "ide"), func() { overrides.IDE = stringPointer(s.ide) })
	set(commandFlagChanged(cmd, "model"), func() { overrides.Model = stringPointer(s.model) })
	set(commandFlagChanged(cmd, "add-dir"), func() {
		addDirs := core.NormalizeAddDirs(s.addDirs)
		overrides.AddDirs = &addDirs
	})
	set(commandFlagChanged(cmd, "tail-lines"), func() { overrides.TailLines = intPointer(s.tailLines) })
	set(commandFlagChanged(cmd, "reasoning-effort"), func() {
		overrides.ReasoningEffort = stringPointer(s.reasoningEffort)
	})
	set(commandFlagChanged(cmd, "access-mode"), func() { overrides.AccessMode = stringPointer(s.accessMode) })
	set(commandFlagChanged(cmd, "timeout"), func() { overrides.Timeout = stringPointer(s.timeout) })
	set(commandFlagChanged(cmd, "max-retries"), func() { overrides.MaxRetries = intPointer(s.maxRetries) })
	set(commandFlagChanged(cmd, "retry-backoff-multiplier"), func() {
		overrides.RetryBackoffMultiplier = float64Pointer(s.retryBackoffMultiplier)
	})
	if explicit := explicitRuntimeOverridesPayload(s.explicitRuntime); explicit != nil {
		overrides.ExplicitRuntime = explicit
		hasOverrides = true
	}

	if !hasOverrides {
		return nil, nil
	}
	payload, err := json.Marshal(overrides)
	if err != nil {
		return nil, fmt.Errorf("encode runtime overrides: %w", err)
	}
	return payload, nil
}

func (s *commandState) buildReviewBatchingOverrides(cmd *cobra.Command) (json.RawMessage, error) {
	payload := map[string]any{}
	if commandFlagChanged(cmd, "concurrent") {
		payload["concurrent"] = s.concurrent
	}
	if commandFlagChanged(cmd, "batch-size") {
		payload["batch_size"] = s.batchSize
	}
	if commandFlagChanged(cmd, "include-resolved") {
		payload["include_resolved"] = s.includeResolved
	}
	if len(payload) == 0 {
		return nil, nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode review batching: %w", err)
	}
	return raw, nil
}

func (s *commandState) buildExecRuntimeOverrides(cmd *cobra.Command) (json.RawMessage, error) {
	overrides := daemonRuntimeOverrides{}
	hasOverrides := false
	set := func(changed bool, apply func()) {
		if !changed {
			return
		}
		apply()
		hasOverrides = true
	}

	set(commandFlagChanged(cmd, "dry-run"), func() { overrides.DryRun = boolPointer(s.dryRun) })
	set(commandFlagChanged(cmd, "run-id"), func() { overrides.RunID = stringPointer(s.runID) })
	set(commandFlagChanged(cmd, "auto-commit"), func() { overrides.AutoCommit = boolPointer(s.autoCommit) })
	set(commandFlagChanged(cmd, "ide"), func() { overrides.IDE = stringPointer(s.ide) })
	set(commandFlagChanged(cmd, "model"), func() { overrides.Model = stringPointer(s.model) })
	set(commandFlagChanged(cmd, "agent"), func() { overrides.AgentName = stringPointer(s.agentName) })
	set(commandFlagChanged(cmd, "format"), func() { overrides.OutputFormat = stringPointer(s.outputFormat) })
	set(commandFlagChanged(cmd, "add-dir"), func() {
		addDirs := core.NormalizeAddDirs(s.addDirs)
		overrides.AddDirs = &addDirs
	})
	set(commandFlagChanged(cmd, "tail-lines"), func() { overrides.TailLines = intPointer(s.tailLines) })
	set(commandFlagChanged(cmd, "reasoning-effort"), func() {
		overrides.ReasoningEffort = stringPointer(s.reasoningEffort)
	})
	set(commandFlagChanged(cmd, "access-mode"), func() { overrides.AccessMode = stringPointer(s.accessMode) })
	set(commandFlagChanged(cmd, "timeout"), func() { overrides.Timeout = stringPointer(s.timeout) })
	set(commandFlagChanged(cmd, "max-retries"), func() { overrides.MaxRetries = intPointer(s.maxRetries) })
	set(commandFlagChanged(cmd, "retry-backoff-multiplier"), func() {
		overrides.RetryBackoffMultiplier = float64Pointer(s.retryBackoffMultiplier)
	})
	set(commandFlagChanged(cmd, "verbose"), func() { overrides.Verbose = boolPointer(s.verbose) })
	set(commandFlagChanged(cmd, "persist"), func() { overrides.Persist = boolPointer(s.persist) })
	set(commandFlagChanged(cmd, "extensions"), func() {
		overrides.EnableExecutableExtensions = boolPointer(s.extensionsEnabled)
	})
	if explicit := explicitRuntimeOverridesPayload(s.explicitRuntime); explicit != nil {
		overrides.ExplicitRuntime = explicit
		hasOverrides = true
	}

	if !hasOverrides {
		return nil, nil
	}
	payload, err := json.Marshal(overrides)
	if err != nil {
		return nil, fmt.Errorf("encode runtime overrides: %w", err)
	}
	return payload, nil
}

func explicitRuntimeOverridesPayload(flags model.ExplicitRuntimeFlags) *model.ExplicitRuntimeFlags {
	if !flags.IDE && !flags.Model && !flags.ReasoningEffort && !flags.AccessMode {
		return nil
	}
	explicit := flags
	return &explicit
}

func (s *commandState) waitAndPrintExecResult(
	ctx context.Context,
	dst io.Writer,
	client daemonCommandClient,
	runID string,
) error {
	status, err := waitForDaemonRunTerminal(ctx, client, runID)
	if err != nil {
		return mapDaemonCommandError(err)
	}
	output, loadErr := loadExecResponseText(s.workspaceRoot, runID)
	if loadErr == nil && strings.TrimSpace(output) != "" {
		if _, err := fmt.Fprintln(dst, output); err != nil {
			return withExitCode(2, fmt.Errorf("write exec stdout: %w", err))
		}
	}
	if isTerminalFailureStatus(status) {
		return withExitCode(1, errors.New(strings.TrimSpace(status.ErrorText)))
	}
	return nil
}

func (s *commandState) streamDaemonWorkflowEvents(
	ctx context.Context,
	dst io.Writer,
	client daemonCommandClient,
	runID string,
	raw bool,
) error {
	encoder := json.NewEncoder(dst)
	var terminalErr error
	err := consumeDaemonRunStream(ctx, client, runID, func(item apiclient.RunStreamItem) error {
		if item.Overflow != nil {
			return nil
		}
		if item.Event == nil {
			return nil
		}
		if raw {
			if err := encoder.Encode(item.Event); err != nil {
				return err
			}
		} else if shouldEmitLeanWorkflowEvent(*item.Event) {
			payload := map[string]any{
				"type":   string(item.Event.Kind),
				"run_id": item.Event.RunID,
				"seq":    item.Event.Seq,
				"time":   item.Event.Timestamp,
			}
			if len(item.Event.Payload) > 0 {
				payload["payload"] = item.Event.Payload
			}
			if err := encoder.Encode(payload); err != nil {
				return err
			}
		}
		switch item.Event.Kind {
		case eventspkg.EventKindRunFailed, eventspkg.EventKindRunCancelled, eventspkg.EventKindRunCrashed:
			terminalErr = workflowTerminalError(*item.Event)
		}
		return nil
	})
	if err != nil {
		return mapDaemonCommandError(err)
	}
	if terminalErr != nil {
		return withExitCode(1, terminalErr)
	}
	return nil
}

func (s *commandState) streamDaemonExecEvents(
	ctx context.Context,
	dst io.Writer,
	client daemonCommandClient,
	runID string,
	raw bool,
) error {
	encoder := json.NewEncoder(dst)
	var terminalErr error
	err := consumeDaemonRunStream(ctx, client, runID, func(item apiclient.RunStreamItem) error {
		if item.Overflow != nil || item.Event == nil {
			return nil
		}
		events, err := translateDaemonExecEvent(s.workspaceRoot, runID, *item.Event, raw, s.dryRun)
		if err != nil {
			return err
		}
		for _, payload := range events {
			if payload == nil {
				continue
			}
			if err := encoder.Encode(payload); err != nil {
				return err
			}
		}
		switch item.Event.Kind {
		case eventspkg.EventKindRunFailed, eventspkg.EventKindRunCancelled, eventspkg.EventKindRunCrashed:
			terminalErr = execTerminalError(*item.Event)
		}
		return nil
	})
	if err != nil {
		return mapDaemonCommandError(err)
	}
	if terminalErr != nil {
		return withExitCode(1, terminalErr)
	}
	return nil
}

func consumeDaemonRunStream(
	ctx context.Context,
	client daemonCommandClient,
	runID string,
	handle func(apiclient.RunStreamItem) error,
) error {
	stream, err := client.OpenRunStream(ctx, runID, apicore.StreamCursor{})
	if err != nil {
		return err
	}
	defer func() {
		_ = stream.Close()
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err, ok := <-stream.Errors():
			if !ok {
				return nil
			}
			if err != nil {
				return err
			}
		case item, ok := <-stream.Items():
			if !ok {
				return nil
			}
			if err := handle(item); err != nil {
				return err
			}
			if item.Event != nil && isTerminalDaemonEvent(item.Event.Kind) {
				return nil
			}
		}
	}
}

func waitForDaemonRunTerminal(ctx context.Context, client daemonCommandClient, runID string) (apicore.Run, error) {
	var (
		terminal         apicore.Run
		sawTerminalEvent bool
	)
	err := consumeDaemonRunStream(ctx, client, runID, func(item apiclient.RunStreamItem) error {
		if item.Event != nil && isTerminalDaemonEvent(item.Event.Kind) {
			sawTerminalEvent = true
		}
		return nil
	})
	if err != nil {
		return terminal, err
	}
	if sawTerminalEvent {
		return waitForTerminalDaemonRunSnapshot(ctx, client, runID)
	}
	if isTerminalDaemonRun(terminal.Status) {
		return terminal, nil
	}

	snapshot, snapshotErr := client.GetRunSnapshot(ctx, runID)
	if snapshotErr != nil {
		return terminal, snapshotErr
	}
	if isTerminalDaemonRun(snapshot.Run.Status) {
		return snapshot.Run, nil
	}
	return terminal, nil
}

func waitForTerminalDaemonRunSnapshot(
	ctx context.Context,
	client daemonCommandClient,
	runID string,
) (apicore.Run, error) {
	ticker := time.NewTicker(daemonRunTerminalPollInterval)
	defer ticker.Stop()

	for {
		snapshot, err := client.GetRunSnapshot(ctx, runID)
		if err != nil {
			return apicore.Run{}, err
		}
		if isTerminalDaemonRun(snapshot.Run.Status) {
			return snapshot.Run, nil
		}

		select {
		case <-ctx.Done():
			return apicore.Run{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func translateDaemonExecEvent(
	workspaceRoot string,
	runID string,
	event eventspkg.Event,
	raw bool,
	dryRun bool,
) ([]map[string]any, error) {
	switch event.Kind {
	case eventspkg.EventKindRunStarted:
		return []map[string]any{{
			"type":    execEventRunStarted,
			"run_id":  runID,
			"time":    event.Timestamp,
			"status":  "running",
			"dry_run": dryRun,
		}}, nil
	case eventspkg.EventKindSessionStarted:
		payload, err := decodeDaemonPayload[kinds.SessionStartedPayload](event.Payload)
		if err != nil {
			return nil, err
		}
		out := map[string]any{
			"type":   execEventSessionAttached,
			"run_id": runID,
			"time":   event.Timestamp,
			"turn":   1,
			"session": map[string]any{
				"acp_session_id":   payload.ACPSessionID,
				"agent_session_id": payload.AgentSessionID,
				"resumed":          payload.Resumed,
			},
		}
		return []map[string]any{out}, nil
	case eventspkg.EventKindSessionUpdate:
		return translateDaemonExecSessionUpdate(runID, event, raw)
	case eventspkg.EventKindRunCompleted:
		return translateDaemonExecTerminalEvent(workspaceRoot, runID, event)
	case eventspkg.EventKindRunFailed:
		return translateDaemonExecTerminalEvent(workspaceRoot, runID, event)
	case eventspkg.EventKindRunCancelled:
		return translateDaemonExecTerminalEvent(workspaceRoot, runID, event)
	case eventspkg.EventKindRunCrashed:
		return translateDaemonExecTerminalEvent(workspaceRoot, runID, event)
	default:
		if raw {
			return nil, nil
		}
		return nil, nil
	}
}

func translateDaemonExecSessionUpdate(
	runID string,
	event eventspkg.Event,
	raw bool,
) ([]map[string]any, error) {
	payload, err := decodeDaemonPayload[kinds.SessionUpdatePayload](event.Payload)
	if err != nil {
		return nil, err
	}
	out := map[string]any{
		"type":   "session.update",
		"run_id": runID,
		"time":   event.Timestamp,
		"turn":   1,
		"update": payload.Update,
		"usage":  payload.Update.Usage,
	}
	if raw || shouldEmitLeanExecUpdate(payload.Update) {
		return []map[string]any{out}, nil
	}
	return nil, nil
}

func translateDaemonExecTerminalEvent(
	workspaceRoot string,
	runID string,
	event eventspkg.Event,
) ([]map[string]any, error) {
	result := map[string]any{
		"run_id": runID,
		"time":   event.Timestamp,
	}

	switch event.Kind {
	case eventspkg.EventKindRunCompleted:
		output, err := loadExecResponseText(workspaceRoot, runID)
		if err != nil {
			return nil, err
		}
		result["type"] = execEventRunSucceeded
		result["status"] = execStatusSucceeded
		result["output"] = output
	case eventspkg.EventKindRunFailed:
		payload, err := decodeDaemonPayload[kinds.RunFailedPayload](event.Payload)
		if err != nil {
			return nil, err
		}
		result["type"] = execEventRunFailed
		result["status"] = execStatusFailed
		result["error"] = payload.Error
	case eventspkg.EventKindRunCancelled:
		payload, err := decodeDaemonPayload[kinds.RunCancelledPayload](event.Payload)
		if err != nil {
			return nil, err
		}
		result["type"] = execEventRunFailed
		result["status"] = execStatusCanceled
		result["error"] = payload.Reason
	case eventspkg.EventKindRunCrashed:
		payload, err := decodeDaemonPayload[kinds.RunCrashedPayload](event.Payload)
		if err != nil {
			return nil, err
		}
		result["type"] = execEventRunFailed
		result["status"] = execStatusCrashed
		result["error"] = payload.Error
	default:
		return nil, nil
	}

	return []map[string]any{result}, nil
}

func loadExecResponseText(workspaceRoot string, runID string) (string, error) {
	record, err := coreRun.LoadPersistedExecRun(workspaceRoot, runID)
	if err != nil {
		return "", err
	}
	turnsDir := strings.TrimSpace(record.TurnsDir)
	if turnsDir == "" {
		return "", nil
	}

	if record.TurnCount > 0 {
		responsePath := filepathJoin(turnsDir, fmt.Sprintf("%04d", record.TurnCount), "response.txt")
		body, readErr := os.ReadFile(responsePath)
		switch {
		case readErr == nil && strings.TrimSpace(string(body)) != "":
			return string(body), nil
		case readErr == nil:
			return string(body), nil
		case !errors.Is(readErr, os.ErrNotExist):
			return "", readErr
		}
	}
	return loadLatestExecTurnResponse(turnsDir)
}

func loadLatestExecTurnResponse(turnsDir string) (string, error) {
	entries, err := os.ReadDir(strings.TrimSpace(turnsDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	for idx := len(entries) - 1; idx >= 0; idx-- {
		if !entries[idx].IsDir() {
			continue
		}
		responsePath := filepathJoin(turnsDir, entries[idx].Name(), "response.txt")
		body, readErr := os.ReadFile(responsePath)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return "", readErr
		}
		return string(body), nil
	}
	return "", nil
}

func workflowTerminalError(event eventspkg.Event) error {
	switch event.Kind {
	case eventspkg.EventKindRunFailed:
		payload, err := decodeDaemonPayload[kinds.RunFailedPayload](event.Payload)
		if err == nil && strings.TrimSpace(payload.Error) != "" {
			return errors.New(strings.TrimSpace(payload.Error))
		}
	case eventspkg.EventKindRunCancelled:
		payload, err := decodeDaemonPayload[kinds.RunCancelledPayload](event.Payload)
		if err == nil && strings.TrimSpace(payload.Reason) != "" {
			return errors.New(strings.TrimSpace(payload.Reason))
		}
	case eventspkg.EventKindRunCrashed:
		payload, err := decodeDaemonPayload[kinds.RunCrashedPayload](event.Payload)
		if err == nil && strings.TrimSpace(payload.Error) != "" {
			return errors.New(strings.TrimSpace(payload.Error))
		}
	}
	return nil
}

func execTerminalError(event eventspkg.Event) error {
	return workflowTerminalError(event)
}

func shouldEmitLeanWorkflowEvent(event eventspkg.Event) bool {
	switch event.Kind {
	case eventspkg.EventKindRunStarted,
		eventspkg.EventKindRunCompleted,
		eventspkg.EventKindRunFailed,
		eventspkg.EventKindRunCancelled,
		eventspkg.EventKindJobStarted,
		eventspkg.EventKindJobRetryScheduled,
		eventspkg.EventKindJobCompleted,
		eventspkg.EventKindJobFailed,
		eventspkg.EventKindJobCancelled,
		eventspkg.EventKindSessionStarted,
		eventspkg.EventKindSessionCompleted,
		eventspkg.EventKindSessionFailed:
		return true
	case eventspkg.EventKindSessionUpdate:
		payload, err := decodeDaemonPayload[kinds.SessionUpdatePayload](event.Payload)
		if err != nil {
			return false
		}
		return shouldEmitLeanExecUpdate(payload.Update)
	default:
		return false
	}
}

func shouldEmitLeanExecUpdate(update kinds.SessionUpdate) bool {
	switch update.Kind {
	case kinds.UpdateKindUserMessageChunk,
		kinds.UpdateKindAgentMessageChunk,
		kinds.UpdateKindToolCallStarted,
		kinds.UpdateKindToolCallUpdated:
		return true
	case kinds.UpdateKindUnknown:
		return update.Status == kinds.StatusCompleted || update.Status == kinds.StatusFailed
	default:
		return false
	}
}

func decodeDaemonPayload[T any](raw json.RawMessage) (T, error) {
	var payload T
	if err := json.Unmarshal(raw, &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

func isTerminalDaemonEvent(kind eventspkg.EventKind) bool {
	switch kind {
	case eventspkg.EventKindRunCompleted,
		eventspkg.EventKindRunFailed,
		eventspkg.EventKindRunCancelled,
		eventspkg.EventKindRunCrashed:
		return true
	default:
		return false
	}
}

func isTerminalFailureStatus(run apicore.Run) bool {
	switch strings.TrimSpace(run.Status) {
	case execStatusFailed, execStatusCanceled, execStatusCrashed:
		return true
	default:
		return false
	}
}

func isTerminalDaemonRun(status string) bool {
	switch strings.TrimSpace(status) {
	case execStatusCompleted, execStatusFailed, execStatusCanceled, execStatusCrashed:
		return true
	default:
		return false
	}
}

func (s *commandState) resolveWorkflowNameArg(cmd *cobra.Command, args []string) error {
	if strings.TrimSpace(s.name) != "" {
		return nil
	}
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		s.name = strings.TrimSpace(args[0])
	}
	if strings.TrimSpace(s.name) == "" {
		commandLabel := "reviews"
		if cmd != nil {
			commandLabel = strings.TrimSpace(cmd.CommandPath())
		}
		switch s.kind {
		case commandKindFetchReviews, commandKindFixReviews:
			return fmt.Errorf("%s requires --name", commandLabel)
		default:
			return fmt.Errorf("%s requires a workflow slug via [slug] or --name", commandLabel)
		}
	}
	return nil
}

func (s *commandState) resolveWorkflowNameAndRoundArgs(cmd *cobra.Command, args []string) error {
	if err := s.resolveWorkflowNameArg(cmd, args); err != nil {
		return err
	}
	if s.round > 0 {
		return nil
	}
	if len(args) > 1 {
		parsed, err := strconv.Atoi(strings.TrimSpace(args[1]))
		if err != nil || parsed <= 0 {
			return errors.New("review round must be a positive integer")
		}
		s.round = parsed
	}
	if s.round <= 0 {
		return errors.New("review round is required via [round] or --round")
	}
	return nil
}

func (s *commandState) resolveReviewRound(ctx context.Context) error {
	if s.round > 0 {
		return nil
	}
	if strings.TrimSpace(s.reviewsDir) != "" {
		base := strings.TrimSpace(filepathBase(s.reviewsDir))
		if strings.HasPrefix(base, "reviews-") {
			parsed, err := strconv.Atoi(strings.TrimPrefix(base, "reviews-"))
			if err == nil && parsed > 0 {
				s.round = parsed
			}
		}
	}
	if s.round > 0 {
		return nil
	}

	client, err := newCLIDaemonBootstrap().ensure(ctx)
	if err != nil {
		return err
	}
	review, err := client.GetLatestReview(ctx, s.workspaceRoot, s.name)
	if err != nil {
		return err
	}
	s.round = review.RoundNumber
	return nil
}

func reviewRoundDirForWorkflow(workspaceRoot string, workflowSlug string, round int) string {
	return filepathJoin(workspaceRoot, ".compozy", "tasks", workflowSlug, fmt.Sprintf("reviews-%03d", round))
}

func intPointerOrNil(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func filepathJoin(parts ...string) string {
	trimmed := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}
		trimmed = append(trimmed, part)
	}
	if len(trimmed) == 0 {
		return ""
	}
	return filepath.Join(trimmed...)
}

func filepathBase(path string) string {
	path = strings.TrimRight(strings.TrimSpace(path), string(os.PathSeparator))
	if path == "" {
		return ""
	}
	parts := strings.Split(path, string(os.PathSeparator))
	return parts[len(parts)-1]
}
