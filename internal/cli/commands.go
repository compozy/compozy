package cli

import (
	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/kernel"
	"github.com/spf13/cobra"
)

func newStartCommand() *cobra.Command {
	return newStartCommandWithDefaults(nil, defaultCommandStateDefaults())
}

func newStartCommandWithDefaults(dispatcher *kernel.Dispatcher, defaults commandStateDefaults) *cobra.Command {
	state := newCommandStateWithDefaults(commandKindStart, core.ModePRDTasks, defaults)
	state.runWorkflow = newRunWorkflow(dispatcher)
	cmd := &cobra.Command{
		Use:          "start",
		Short:        "Execute PRD task files from a PRD directory",
		SilenceUsage: true,
		Long: `Execute task markdown files from a PRD workflow directory and dispatch them to the configured
AI agent one task at a time.

Most runtime defaults can be supplied by ~/.compozy/config.toml and overridden by
.compozy/config.toml. In interactive terminals the command
opens the run cockpit by default; in non-TTY environments it falls back to headless streaming.`,
		Example: `  compozy start --name multi-repo --tasks-dir .compozy/tasks/multi-repo --ide claude
  compozy start --format json --name multi-repo --tasks-dir .compozy/tasks/multi-repo
  compozy start`,
		RunE: state.run,
	}

	addCommonFlags(cmd, state, commonFlagOptions{})
	addWorkflowOutputFlags(cmd, state)
	cmd.Flags().Var(
		newTaskRuntimeFlagValue(&state.executionTaskRuntimeRules),
		"task-runtime",
		`Per-task runtime override rule for start (repeatable). Use key=value pairs such as type=frontend,ide=codex,model=gpt-5.4 or id=task_01,reasoning-effort=xhigh`,
	)
	cmd.Flags().StringVar(&state.name, "name", "", "Task workflow name (used for .compozy/tasks/<name>)")
	cmd.Flags().StringVar(&state.tasksDir, "tasks-dir", "", "Path to tasks directory (.compozy/tasks/<name>)")
	cmd.Flags().BoolVar(&state.includeCompleted, "include-completed", false, "Include completed tasks")
	cmd.Flags().BoolVar(
		&state.skipValidation,
		"skip-validation",
		false,
		"Skip task metadata preflight; use only when tasks were validated separately",
	)
	cmd.Flags().BoolVar(
		&state.force,
		"force",
		false,
		"Continue after task metadata validation fails in non-interactive mode",
	)
	return cmd
}

func addWorkflowOutputFlags(cmd *cobra.Command, state *commandState) {
	cmd.Flags().StringVar(
		&state.outputFormat,
		"format",
		string(core.OutputFormatText),
		"Output format: text, json, or raw-json",
	)
	cmd.Flags().BoolVar(
		&state.tui,
		"tui",
		true,
		"Open the interactive TUI when the terminal supports it; otherwise stream headless output",
	)
}

func newExecCommandWithDefaults(defaults commandStateDefaults) *cobra.Command {
	state := newCommandStateWithDefaults(commandKindExec, core.ModeExec, defaults)
	cmd := &cobra.Command{
		Use:          "exec [prompt]",
		Short:        "Execute one ad hoc prompt through the shared ACP runtime",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		Long: `Execute a single ad hoc prompt using the shared Compozy planning and ACP execution pipeline.

Provide the prompt as one positional argument, with --prompt-file, or via stdin. By default the
command is headless and ephemeral: text mode writes only the final assistant response to stdout and
json mode streams lean JSONL events to stdout, while raw-json preserves the full event stream.
Operational runtime logs stay silent unless you opt into --verbose. Use --tui to open the
interactive TUI and --persist to save resumable artifacts under
~/.compozy/runs/<run-id>/. Use --run-id to resume a previously persisted exec session.`,
		Example: `  compozy exec "Summarize the current repository changes"
  compozy exec --agent council "Decide between two designs"
  compozy exec --prompt-file prompt.md
  cat prompt.md | compozy exec --format json
  compozy exec --format raw-json "Inspect every streamed event"
  compozy exec --persist "Review the latest changes"
  compozy exec --run-id exec-20260405-120000-000000000 "Continue from the previous session"`,
		RunE: state.execDaemon,
	}

	addCommonFlags(cmd, state, commonFlagOptions{})
	cmd.Flags().StringVar(
		&state.agentName,
		"agent",
		"",
		"Reusable agent to execute from .compozy/agents or ~/.compozy/agents",
	)
	cmd.Flags().StringVar(&state.promptFile, "prompt-file", "", "Path to a file containing the prompt text")
	cmd.Flags().StringVar(
		&state.outputFormat,
		"format",
		string(core.OutputFormatText),
		"Output format: text, json, or raw-json",
	)
	cmd.Flags().BoolVar(&state.verbose, "verbose", false, "Emit operational runtime logs to stderr during exec")
	cmd.Flags().BoolVar(&state.tui, "tui", false, "Open the interactive TUI instead of using headless stdout output")
	cmd.Flags().BoolVar(&state.persist, "persist", false, "Persist exec artifacts under ~/.compozy/runs/<run-id>/")
	cmd.Flags().BoolVar(&state.extensionsEnabled, "extensions", false, "Enable executable extensions for this exec run")
	cmd.Flags().StringVar(&state.runID, "run-id", "", "Resume a previously persisted exec session by run id")
	return cmd
}
