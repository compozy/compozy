package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	extension "github.com/compozy/compozy/sdk/extension"
)

const (
	extensionName    = "cy-qa-workflow"
	extensionVersion = "0.1.0"

	reportMarker    = "<!-- compozy-qa-workflow:qa-report -->"
	executionMarker = "<!-- compozy-qa-workflow:qa-execution -->"

	defaultQAReportIDE       = "claude"
	defaultQAReportModel     = "opus"
	defaultQAExecutionIDE    = "codex"
	defaultQAExecutionModel  = "gpt-5.5"
	defaultReasoningEffort   = "xhigh"
	claudeEffortEnv          = "CLAUDE_CODE_EFFORT_LEVEL"
	qaReportTaskType         = "docs"
	qaExecutionTaskType      = "test"
	qaReportComplexity       = "high"
	qaExecutionComplexity    = "critical"
	goalCommandWithObjective = "/goal Execute the Compozy QA execution task end-to-end for this workflow: " +
		"consume the generated QA report artifacts, validate the product like a real user, " +
		"fix root-cause issues, persist evidence under the workflow qa/ directory, " +
		"and finish only after make verify passes."
)

type taskClient interface {
	List(context.Context, extension.TaskListRequest) ([]extension.Task, error)
	Create(context.Context, extension.TaskCreateRequest) (*extension.Task, error)
}

func main() {
	ext := extension.New(extensionName, extensionVersion).
		WithCapabilities(
			extension.CapabilityPlanMutate,
			extension.CapabilityAgentMutate,
			extension.CapabilityTasksRead,
			extension.CapabilityTasksCreate,
		).
		OnPlanPreDiscover(handlePlanPreDiscover).
		OnPlanPreResolveTaskRuntime(handlePlanPreResolveTaskRuntime).
		OnAgentPreSessionCreate(handleAgentPreSessionCreate)

	if err := ext.Start(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func handlePlanPreDiscover(
	ctx context.Context,
	hook extension.HookContext,
	payload extension.PlanPreDiscoverPayload,
) (extension.ExtraSourcesPatch, error) {
	if payload.Mode != extension.ExecutionModePRDTasks {
		return extension.ExtraSourcesPatch{}, nil
	}
	workflow := strings.TrimSpace(payload.Workflow)
	if workflow == "" {
		return extension.ExtraSourcesPatch{}, nil
	}
	if hook.Host == nil || hook.Host.Tasks == nil {
		return extension.ExtraSourcesPatch{}, fmt.Errorf("qa workflow extension: host tasks API is unavailable")
	}
	if err := ensureQATasks(ctx, hook.Host.Tasks, workflow); err != nil {
		return extension.ExtraSourcesPatch{}, err
	}
	return extension.ExtraSourcesPatch{}, nil
}

func ensureQATasks(ctx context.Context, client taskClient, workflow string) error {
	tasks, err := client.List(ctx, extension.TaskListRequest{Workflow: workflow})
	if err != nil {
		return fmt.Errorf("list workflow tasks for qa extension: %w", err)
	}

	reportTask, executionTask := findQATasks(tasks)
	if reportTask == nil {
		created, err := client.Create(ctx, buildQAReportRequest(workflow, tasks))
		if err != nil {
			return fmt.Errorf("create qa report task: %w", err)
		}
		reportTask = created
	}
	if executionTask == nil {
		_, err := client.Create(ctx, buildQAExecutionRequest(workflow, reportTask))
		if err != nil {
			return fmt.Errorf("create qa execution task: %w", err)
		}
	}
	return nil
}

func findQATasks(tasks []extension.Task) (*extension.Task, *extension.Task) {
	var report *extension.Task
	var execution *extension.Task
	for idx := range tasks {
		task := &tasks[idx]
		switch {
		case report == nil && isQAReportTask(*task):
			report = task
		case execution == nil && isQAExecutionTask(*task):
			execution = task
		}
	}
	return report, execution
}

func buildQAReportRequest(workflow string, tasks []extension.Task) extension.TaskCreateRequest {
	return extension.TaskCreateRequest{
		Workflow:    workflow,
		Title:       fmt.Sprintf("%s QA plan and regression artifacts", workflowTitle(workflow)),
		Body:        qaReportBody(workflow),
		UpdateIndex: true,
		Frontmatter: extension.TaskFrontmatter{
			Status:       "pending",
			Type:         qaReportTaskType,
			Complexity:   qaReportComplexity,
			Dependencies: implementationDependencies(tasks),
		},
	}
}

func buildQAExecutionRequest(workflow string, reportTask *extension.Task) extension.TaskCreateRequest {
	dependencies := []string{}
	if reportTask != nil && reportTask.Number > 0 {
		dependencies = []string{taskRef(reportTask.Number)}
	}
	return extension.TaskCreateRequest{
		Workflow:    workflow,
		Title:       fmt.Sprintf("%s QA execution and operator-flow validation", workflowTitle(workflow)),
		Body:        qaExecutionBody(workflow),
		UpdateIndex: true,
		Frontmatter: extension.TaskFrontmatter{
			Status:       "pending",
			Type:         qaExecutionTaskType,
			Complexity:   qaExecutionComplexity,
			Dependencies: dependencies,
		},
	}
}

func qaReportBody(workflow string) string {
	outputPath := qaOutputPath(workflow)
	return strings.Join([]string{
		reportMarker,
		"",
		"## Overview",
		"",
		"Generate reusable QA planning artifacts for this workflow before live execution begins. " +
			"Leave the repo with a concrete test plan, traceable execution cases, and regression-suite " +
			"definitions stored under the same feature-local QA root that the execution task will consume.",
		"",
		"<critical>",
		fmt.Sprintf(
			"- ACTIVATE `/qa-report` with `qa-output-path=%s` before writing or revising any QA artifact",
			outputPath,
		),
		fmt.Sprintf(
			"- KEEP THE SAME `qa-output-path` FOR `/qa-execution`; all planning and execution artifacts must live under `%s/qa/`",
			outputPath,
		),
		"- FOCUS ON WHAT: define coverage, risks, automation targets, and evidence layout; " +
			"do not execute validation flows or fix bugs in this task",
		"- CLASSIFY critical flows explicitly as `E2E`, `Integration`, `Manual-only`, or `Blocked`, with reasons",
		"</critical>",
		"",
		"## Requirements",
		"",
		fmt.Sprintf("1. MUST use `/qa-report` with `qa-output-path=%s`.", outputPath),
		fmt.Sprintf("2. MUST generate a feature-level test plan under `%s/qa/test-plans/`.", outputPath),
		"3. MUST generate execution-ready test cases under the workflow QA root.",
		"4. MUST create at least one regression-suite document that defines smoke, targeted, and full validation priorities.",
		"5. MUST identify P0/P1 flows that `/qa-execution` must run first, including any blocked or manual-only coverage.",
		"",
		"## Success Criteria",
		"",
		"- QA artifacts are complete, traceable, and ready for the QA execution task.",
		"- The QA execution task can start without redefining scope, paths, or validation priorities.",
	}, "\n")
}

func qaExecutionBody(workflow string) string {
	outputPath := qaOutputPath(workflow)
	return strings.Join([]string{
		executionMarker,
		"",
		"## Overview",
		"",
		"Execute the QA plan for this workflow against the real repository. Validate user-visible " +
			"and operator-critical behavior, fix root-cause defects discovered by the tests, " +
			"and persist evidence under the shared QA root.",
		"",
		"<critical>",
		fmt.Sprintf("- ACTIVATE `/qa-execution` with `qa-output-path=%s` before executing QA", outputPath),
		fmt.Sprintf(
			"- CONSUME the QA report artifacts under `%s/qa/test-plans/` and `%s/qa/test-cases/`",
			outputPath,
			outputPath,
		),
		"- FIX production code for real bugs uncovered by QA; do not weaken tests to match broken behavior",
		"- RUN `make verify` after fixes and keep the final verification evidence in the QA output path",
		"</critical>",
		"",
		"## Requirements",
		"",
		fmt.Sprintf("1. MUST use `/qa-execution` with `qa-output-path=%s`.", outputPath),
		"2. MUST execute the generated smoke and P0/P1 regression cases first.",
		"3. MUST create bug reports for confirmed failures and link evidence to the originating test cases.",
		"4. MUST fix root causes for regressions in production code before declaring the task complete.",
		"5. MUST finish only after `make verify` passes.",
		"",
		"## Success Criteria",
		"",
		"- QA execution evidence is persisted under the workflow QA root.",
		"- Confirmed product defects are fixed at the root cause.",
		"- `make verify` passes with no warnings or failures.",
	}, "\n")
}

func implementationDependencies(tasks []extension.Task) []string {
	numbers := make([]int, 0, len(tasks))
	seen := make(map[int]struct{}, len(tasks))
	for idx := range tasks {
		task := tasks[idx]
		if task.Number <= 0 || isQAReportTask(task) || isQAExecutionTask(task) {
			continue
		}
		if _, ok := seen[task.Number]; ok {
			continue
		}
		seen[task.Number] = struct{}{}
		numbers = append(numbers, task.Number)
	}
	sort.Ints(numbers)

	dependencies := make([]string, 0, len(numbers))
	for _, number := range numbers {
		dependencies = append(dependencies, taskRef(number))
	}
	return dependencies
}

func handlePlanPreResolveTaskRuntime(
	_ context.Context,
	_ extension.HookContext,
	payload extension.PlanPreResolveTaskRuntimePayload,
) (extension.TaskRuntimePatch, error) {
	runtime, ok := runtimeForTask(payload.Task)
	if !ok {
		return extension.TaskRuntimePatch{}, nil
	}
	return extension.TaskRuntimePatch{Runtime: &runtime}, nil
}

func runtimeForTask(task extension.TaskRuntimeTask) (extension.TaskRuntime, bool) {
	switch {
	case isQAReportRuntimeTask(task):
		return extension.TaskRuntime{
			IDE:             defaultQAReportIDE,
			Model:           defaultQAReportModel,
			ReasoningEffort: defaultReasoningEffort,
		}, true
	case isQAExecutionRuntimeTask(task):
		return extension.TaskRuntime{
			IDE:             defaultQAExecutionIDE,
			Model:           defaultQAExecutionModel,
			ReasoningEffort: defaultReasoningEffort,
		}, true
	default:
		return extension.TaskRuntime{}, false
	}
}

func handleAgentPreSessionCreate(
	_ context.Context,
	_ extension.HookContext,
	payload extension.AgentPreSessionCreatePayload,
) (extension.SessionRequestPatch, error) {
	request, changed := mutateSessionRequest(payload.SessionRequest)
	if !changed {
		return extension.SessionRequestPatch{}, nil
	}
	return extension.SessionRequestPatch{SessionRequest: &request}, nil
}

func mutateSessionRequest(request extension.SessionRequest) (extension.SessionRequest, bool) {
	prompt := string(request.Prompt)
	switch {
	case isQAExecutionPrompt(prompt):
		request.Prompt = []byte(goalPrefixedPrompt(prompt))
		return request, true
	case isQAReportPrompt(prompt):
		request.ExtraEnv = cloneEnvWith(request.ExtraEnv, claudeEffortEnv, defaultReasoningEffort)
		return request, true
	default:
		return request, false
	}
}

func goalPrefixedPrompt(prompt string) string {
	trimmed := strings.TrimLeft(prompt, " \t\r\n")
	if strings.HasPrefix(trimmed, "/goal") {
		return trimmed
	}
	return goalCommandWithObjective + "\n\n" + prompt
}

func cloneEnvWith(env map[string]string, key string, value string) map[string]string {
	clone := make(map[string]string, len(env)+1)
	for k, v := range env {
		clone[k] = v
	}
	clone[key] = value
	return clone
}

func isQAReportTask(task extension.Task) bool {
	if strings.Contains(task.Body, reportMarker) {
		return true
	}
	title := strings.ToLower(task.Title)
	return strings.EqualFold(task.Type, qaReportTaskType) &&
		strings.Contains(title, "qa") &&
		(strings.Contains(title, "plan") || strings.Contains(title, "report"))
}

func isQAExecutionTask(task extension.Task) bool {
	if strings.Contains(task.Body, executionMarker) {
		return true
	}
	title := strings.ToLower(task.Title)
	return strings.EqualFold(task.Type, qaExecutionTaskType) &&
		strings.Contains(title, "qa") &&
		strings.Contains(title, "execution")
}

func isQAReportRuntimeTask(task extension.TaskRuntimeTask) bool {
	title := strings.ToLower(task.Title)
	return strings.EqualFold(task.Type, qaReportTaskType) &&
		strings.Contains(title, "qa") &&
		(strings.Contains(title, "plan") || strings.Contains(title, "report"))
}

func isQAExecutionRuntimeTask(task extension.TaskRuntimeTask) bool {
	title := strings.ToLower(task.Title)
	return strings.EqualFold(task.Type, qaExecutionTaskType) &&
		strings.Contains(title, "qa") &&
		strings.Contains(title, "execution")
}

func isQAReportPrompt(prompt string) bool {
	lower := strings.ToLower(prompt)
	return strings.Contains(prompt, reportMarker) ||
		(strings.Contains(lower, "qa plan") && strings.Contains(lower, "qa-output-path") &&
			(strings.Contains(lower, "/qa-report") || strings.Contains(lower, "$qa-report")))
}

func isQAExecutionPrompt(prompt string) bool {
	lower := strings.ToLower(prompt)
	return strings.Contains(prompt, executionMarker) ||
		(strings.Contains(lower, "qa execution") && strings.Contains(lower, "qa-output-path") &&
			(strings.Contains(lower, "/qa-execution") || strings.Contains(lower, "$qa-execution")))
}

func qaOutputPath(workflow string) string {
	return ".compozy/tasks/" + strings.TrimSpace(workflow)
}

func taskRef(number int) string {
	return fmt.Sprintf("task_%02d", number)
}

func workflowTitle(workflow string) string {
	parts := strings.FieldsFunc(strings.TrimSpace(workflow), func(r rune) bool {
		return r == '-' || r == '_'
	})
	for idx, part := range parts {
		if part == "" {
			continue
		}
		parts[idx] = strings.ToUpper(part[:1]) + part[1:]
	}
	if len(parts) == 0 {
		return "Workflow"
	}
	return strings.Join(parts, " ")
}
