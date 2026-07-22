package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/taskgroups"
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
		Short:        "Validate task workflow artifacts before execution",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Validate task files and graph manifests in an ordinary or Task Group workflow.

Task Group initiatives include the root _task_groups.md plan and every manifest-declared child task suite.

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

	report, err := validateTaskWorkflow(ctx, resolvedTasksDir, registry)
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

func validateTaskWorkflow(
	ctx context.Context,
	tasksDir string,
	registry *tasks.TypeRegistry,
) (tasks.Report, error) {
	if report, handled, err := validateDirectTaskGroupSuite(ctx, tasksDir, registry); handled {
		return report, err
	}

	report, err := tasks.Validate(ctx, tasksDir, registry)
	if err != nil {
		return tasks.Report{}, err
	}

	planPath := filepath.Join(tasksDir, taskgroups.ManifestFileName)
	content, err := os.ReadFile(planPath)
	if errors.Is(err, os.ErrNotExist) {
		return report, nil
	}
	if err != nil {
		return tasks.Report{}, fmt.Errorf("read task group plan %s: %w", planPath, err)
	}

	initiative := filepath.Base(tasksDir)
	plan, err := taskgroups.ParsePlanForInitiative(string(content), initiative)
	if err != nil {
		var planErr *taskgroups.Error
		if !errors.As(err, &planErr) {
			return tasks.Report{}, fmt.Errorf("validate task group plan %s: %w", planPath, err)
		}
		appendTaskGroupIssues(&report, planPath, planErr.Issues)
		sortTaskValidationIssues(report.Issues)
		return report, nil
	}

	for index := range plan.TaskGroups {
		taskGroup := &plan.TaskGroups[index]
		taskGroupDir := filepath.Join(tasksDir, filepath.FromSlash(taskGroup.Directory))
		taskGroupReport, validateErr := tasks.ValidateWithOptions(
			ctx,
			taskGroupDir,
			registry,
			tasks.ValidateOptions{
				Recursive:        false,
				ExpectedWorkflow: initiative + "/" + taskGroup.ID,
			},
		)
		if errors.Is(validateErr, os.ErrNotExist) {
			report.Issues = append(report.Issues, tasks.Issue{
				Path:    planPath,
				Field:   "graph.nodes." + taskGroup.ID + ".directory",
				Message: fmt.Sprintf("declared task group directory %q does not exist", taskGroup.Directory),
			})
			continue
		}
		if validateErr != nil {
			return tasks.Report{}, fmt.Errorf("validate task group %s: %w", taskGroup.ID, validateErr)
		}
		report.Scanned += taskGroupReport.Scanned
		report.Issues = append(report.Issues, taskGroupReport.Issues...)
	}

	sortTaskValidationIssues(report.Issues)
	return report, nil
}

func validateDirectTaskGroupSuite(
	ctx context.Context,
	tasksDir string,
	registry *tasks.TypeRegistry,
) (tasks.Report, bool, error) {
	taskGroupsDir := filepath.Dir(tasksDir)
	if filepath.Base(taskGroupsDir) != "_task_groups" {
		return tasks.Report{}, false, nil
	}

	manifest, err := tasks.ReadTaskGraphManifest(tasksDir)
	if errors.Is(err, tasks.ErrTaskGraphManifestMissing) {
		return tasks.Report{}, false, nil
	}
	if err != nil {
		return tasks.Report{}, true, err
	}
	report, err := tasks.ValidateWithOptions(
		ctx,
		tasksDir,
		registry,
		tasks.ValidateOptions{Recursive: false, ExpectedWorkflow: manifest.Workflow},
	)
	if err != nil {
		return tasks.Report{}, true, err
	}

	workflow := strings.TrimSpace(manifest.Workflow)
	if workflow == "" {
		return report, true, nil
	}
	initiative := filepath.Base(filepath.Dir(taskGroupsDir))
	ref, refErr := taskgroups.ParseTaskGroupRef(workflow)
	if refErr != nil || ref.Initiative != initiative {
		report.Issues = append(report.Issues, tasks.Issue{
			Path:    manifest.Path,
			Field:   "workflow",
			Message: fmt.Sprintf("workflow %q must be a valid %s/TG-NNN reference", workflow, initiative),
		})
		sortTaskValidationIssues(report.Issues)
	}
	return report, true, nil
}

func appendTaskGroupIssues(report *tasks.Report, planPath string, issues []taskgroups.Issue) {
	for _, issue := range issues {
		path := issue.Path
		if path == "" {
			path = planPath
		}
		report.Issues = append(report.Issues, tasks.Issue{
			Path:    path,
			Field:   issue.Field,
			Message: issue.Message,
		})
	}
}

func sortTaskValidationIssues(issues []tasks.Issue) {
	slices.SortStableFunc(issues, func(left, right tasks.Issue) int {
		if result := strings.Compare(left.Path, right.Path); result != 0 {
			return result
		}
		if result := strings.Compare(left.Field, right.Field); result != 0 {
			return result
		}
		return strings.Compare(left.Message, right.Message)
	})
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
		payload.FixPrompt = validateTasksFixPrompt(report, registry)
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
	case report.OK() && report.Scanned > 0:
		_, err := fmt.Fprintf(out, "all tasks valid (%d scanned)\n", report.Scanned)
		return err
	case report.OK():
		_, err := fmt.Fprintf(out, "no tasks found in %s\n", report.TasksDir)
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

	_, err = fmt.Fprintf(out, "\nFix prompt:\n%s\n", validateTasksFixPrompt(report, registry))
	return err
}

func validateTasksFixPrompt(report tasks.Report, registry *tasks.TypeRegistry) string {
	planIssues := make([]tasks.Issue, 0)
	taskIssues := make([]tasks.Issue, 0, len(report.Issues))
	for _, issue := range report.Issues {
		if filepath.Base(issue.Path) == taskgroups.ManifestFileName {
			planIssues = append(planIssues, issue)
			continue
		}
		taskIssues = append(taskIssues, issue)
	}
	if len(planIssues) == 0 {
		return tasks.FixPrompt(report, registry)
	}

	var prompt strings.Builder
	prompt.WriteString("Fix the Compozy Task Group plan below.\n")
	prompt.WriteString(
		"Use `schema_version: " + taskgroups.SchemaVersion + "` with root `initiative`, " +
			"not `workflow`. Every YAML graph node must have exactly one Markdown heading " +
			"in the form `## [ ] TG-NNN — Title` or `## [x] TG-NNN — Title`. " +
			"Each body must include `Reference`, `Outcome`, a non-empty `Owns` list, and " +
			"`Dependencies`; dependency IDs and rationales must exactly mirror incoming YAML edges.\n\n",
	)
	writeValidationIssues(&prompt, planIssues)
	prompt.WriteString("\nReturn only the corrected Task Group plan Markdown.")

	if len(taskIssues) > 0 {
		taskReport := report
		taskReport.Issues = taskIssues
		prompt.WriteString("\n\n")
		prompt.WriteString(tasks.FixPrompt(taskReport, registry))
	}
	return prompt.String()
}

func writeValidationIssues(output *strings.Builder, issues []tasks.Issue) {
	currentPath := ""
	for _, issue := range issues {
		if issue.Path != currentPath {
			if currentPath != "" {
				output.WriteByte('\n')
			}
			currentPath = issue.Path
			output.WriteString(currentPath + "\n")
		}
		fmt.Fprintf(output, "- %s: %s\n", issue.Field, issue.Message)
	}
}

func validateTasksMessage(report tasks.Report) string {
	switch {
	case !report.OK():
		return "task validation failed"
	case report.Scanned > 0:
		return "all tasks valid"
	default:
		return "no tasks found"
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
