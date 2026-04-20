package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/tasks"
	"github.com/compozy/compozy/internal/core/workspace"
	"github.com/spf13/cobra"
)

const (
	validateTasksFormatText = "text"
	validateTasksFormatJSON = "json"
)

type validateTasksCommandState struct {
	workspaceRoot string
	projectConfig workspace.ProjectConfig
	name          string
	tasksDir      string
	format        string
}

type validateTasksOutput struct {
	OK        bool          `json:"ok"`
	Message   string        `json:"message"`
	TasksDir  string        `json:"tasks_dir"`
	Scanned   int           `json:"scanned"`
	Issues    []tasks.Issue `json:"issues"`
	FixPrompt string        `json:"fix_prompt,omitempty"`
}

func newTasksValidateCommand() *cobra.Command {
	state := &validateTasksCommandState{format: validateTasksFormatText}
	cmd := &cobra.Command{
		Use:          "validate",
		Short:        "Validate task metadata under a workflow tasks directory",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Validate every task_*.md file in a PRD workflow directory against the metadata v2 schema.

Validation failures return exit code 1. Filesystem, config, or flag errors return exit code 2.`,
		Example: `  compozy tasks validate --name my-feature
  compozy tasks validate --tasks-dir .compozy/tasks/my-feature
  compozy tasks validate --tasks-dir .compozy/tasks/my-feature --format json`,
		RunE: state.run,
	}

	cmd.Flags().StringVar(&state.name, "name", "", "Task workflow name (used for .compozy/tasks/<name>)")
	cmd.Flags().StringVar(&state.tasksDir, "tasks-dir", "", "Path to tasks directory (.compozy/tasks/<name>)")
	cmd.Flags().
		StringVar(&state.format, "format", validateTasksFormatText, "Output format: text or json")
	return cmd
}

func (s *validateTasksCommandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	workspaceCtx, err := resolveWorkspaceContext(ctx)
	if err != nil {
		return withExitCode(2, fmt.Errorf("resolve workspace for %s: %w", cmd.Name(), err))
	}
	s.workspaceRoot = workspaceCtx.Root
	s.projectConfig = workspaceCtx.Config

	if err := s.validateFormat(); err != nil {
		return withExitCode(2, err)
	}

	registry, err := taskTypeRegistryFromConfig(s.projectConfig)
	if err != nil {
		return withExitCode(2, fmt.Errorf("resolve task type registry: %w", err))
	}

	resolvedTasksDir, err := s.resolveTasksDir()
	if err != nil {
		return withExitCode(2, err)
	}

	report, err := tasks.Validate(ctx, resolvedTasksDir, registry)
	if err != nil {
		return withExitCode(2, err)
	}

	if s.format == validateTasksFormatJSON {
		if err := writeValidateTasksJSON(cmd.OutOrStdout(), report, registry); err != nil {
			return withExitCode(2, err)
		}
	} else if err := writeValidateTasksText(cmd.OutOrStdout(), report, registry); err != nil {
		return withExitCode(2, err)
	}

	if report.OK() {
		return nil
	}
	return withExitCode(1, errors.New("task validation failed"))
}

func (s *validateTasksCommandState) validateFormat() error {
	s.format = strings.TrimSpace(s.format)
	switch s.format {
	case validateTasksFormatText, validateTasksFormatJSON:
		return nil
	default:
		return fmt.Errorf(
			"tasks validate format must be one of %q or %q (got %q)",
			validateTasksFormatText,
			validateTasksFormatJSON,
			s.format,
		)
	}
}

func (s *validateTasksCommandState) resolveTasksDir() (string, error) {
	return resolveTaskWorkflowDir(s.workspaceRoot, s.name, s.tasksDir)
}

func taskTypeRegistryFromConfig(cfg workspace.ProjectConfig) (*tasks.TypeRegistry, error) {
	if cfg.Tasks.Types == nil {
		return tasks.NewRegistry(nil)
	}
	return tasks.NewRegistry(*cfg.Tasks.Types)
}

func writeValidateTasksJSON(out io.Writer, report tasks.Report, registry *tasks.TypeRegistry) error {
	payload := validateTasksOutput{
		OK:       report.OK(),
		Message:  validateTasksMessage(report),
		TasksDir: report.TasksDir,
		Scanned:  report.Scanned,
		Issues:   report.Issues,
	}
	if !report.OK() {
		payload.FixPrompt = tasks.FixPrompt(report, registry)
	}

	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("encode validation report: %w", err)
	}
	return nil
}

func writeValidateTasksText(out io.Writer, report tasks.Report, registry *tasks.TypeRegistry) error {
	switch {
	case report.Scanned == 0:
		_, err := fmt.Fprintf(out, "no tasks found in %s\n", report.TasksDir)
		return err
	case report.OK():
		_, err := fmt.Fprintf(out, "all tasks valid (%d scanned)\n", report.Scanned)
		return err
	}

	_, err := fmt.Fprintf(
		out,
		"task validation failed: %d issue(s) across %d file(s)\n",
		len(report.Issues),
		distinctIssuePaths(report.Issues),
	)
	if err != nil {
		return err
	}

	currentPath := ""
	for _, issue := range report.Issues {
		if issue.Path != currentPath {
			currentPath = issue.Path
			if _, err := fmt.Fprintf(out, "\n%s\n", currentPath); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(out, "- %s: %s\n", issue.Field, issue.Message); err != nil {
			return err
		}
	}

	_, err = fmt.Fprintf(out, "\nFix prompt:\n%s\n", tasks.FixPrompt(report, registry))
	return err
}

func validateTasksMessage(report tasks.Report) string {
	switch {
	case report.Scanned == 0:
		return "no tasks found"
	case report.OK():
		return "all tasks valid"
	default:
		return "task validation failed"
	}
}

func distinctIssuePaths(issues []tasks.Issue) int {
	paths := make(map[string]struct{}, len(issues))
	for _, issue := range issues {
		paths[issue.Path] = struct{}{}
	}
	return len(paths)
}

func resolveTaskWorkflowDir(workspaceRoot, name, tasksDir string) (string, error) {
	resolvedName := strings.TrimSpace(name)
	resolvedTasksDir := strings.TrimSpace(tasksDir)
	if resolvedName == "" && resolvedTasksDir == "" {
		return "", errors.New("missing required flags: either --name or --tasks-dir must be provided")
	}
	if resolvedTasksDir == "" {
		resolvedTasksDir = model.TaskDirectoryForWorkspace(workspaceRoot, resolvedName)
	}
	if !filepath.IsAbs(resolvedTasksDir) {
		resolvedTasksDir = filepath.Join(workspaceRoot, resolvedTasksDir)
	}
	absPath, err := filepath.Abs(resolvedTasksDir)
	if err != nil {
		return "", fmt.Errorf("resolve tasks dir: %w", err)
	}
	return absPath, nil
}
