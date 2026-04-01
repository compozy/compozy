package prompt

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

func buildPRDTaskPrompt(task model.IssueEntry, autoCommit bool) string {
	taskData, err := ParseTaskFile(task.Content)
	if err != nil {
		taskData = model.TaskEntry{Content: task.Content, Status: "UNCONFIRMED"}
	}
	prdDir := filepath.Dir(task.AbsPath)
	tasksFile := filepath.Join(prdDir, "_tasks.md")

	sections := []string{
		fmt.Sprintf("# Implementation Task: %s", task.Name),
		buildTaskContextSection(taskData),
		buildPRDRequiredSkillsSection(),
		buildPRDExecutionRulesSection(prdDir, autoCommit),
		fmt.Sprintf("## Task Specification\n\n%s", task.Content),
		buildTaskFilesSection(task.AbsPath, tasksFile, prdDir, autoCommit),
	}
	return strings.Join(sections, "\n\n")
}

func buildTaskContextSection(taskData model.TaskEntry) string {
	var sb strings.Builder
	sb.WriteString("## Task Context\n\n")
	fmt.Fprintf(&sb, "- **Domain**: %s\n", taskData.Domain)
	fmt.Fprintf(&sb, "- **Type**: %s\n", taskData.TaskType)
	fmt.Fprintf(&sb, "- **Scope**: %s\n", taskData.Scope)
	fmt.Fprintf(&sb, "- **Complexity**: %s\n", taskData.Complexity)
	if len(taskData.Dependencies) > 0 {
		fmt.Fprintf(&sb, "- **Dependencies**: %s\n", strings.Join(taskData.Dependencies, ", "))
	}
	return sb.String()
}

func buildPRDRequiredSkillsSection() string {
	return `<required_skills>
- ` + "`execute-prd-task`" + `: required end-to-end workflow for a PRD task
- ` + "`verification-before-completion`" + `: required before any completion claim or automatic commit
</required_skills>`
}

func buildPRDExecutionRulesSection(prdDir string, autoCommit bool) string {
	var sb strings.Builder
	sb.WriteString("<critical>\n")
	sb.WriteString("- Use installed `execute-prd-task` as the execution workflow for this task.\n")
	sb.WriteString(
		"- Read `AGENTS.md`, `CLAUDE.md`, and the PRD documents under `" + prdDir + "` before editing code.\n",
	)
	sb.WriteString(
		"- Treat the task specification below plus the supporting PRD documents, especially `_techspec.md` and `_tasks.md`, as the source of truth.\n",
	)
	sb.WriteString(
		"- Keep scope tight to this task and record meaningful follow-up work instead of expanding scope silently.\n",
	)
	sb.WriteString(
		"- Use installed `verification-before-completion` before any completion claim or automatic commit.\n",
	)
	if autoCommit {
		sb.WriteString("- Automatic commits are enabled for this run, " +
			"but only after clean verification, self-review, and tracking updates.\n")
	} else {
		sb.WriteString("- Automatic commits are disabled for this run (`--auto-commit=false`).\n")
	}
	sb.WriteString("</critical>")
	return sb.String()
}

func buildTaskFilesSection(taskAbsPath, tasksFile, prdDir string, autoCommit bool) string {
	var sb strings.Builder
	sb.WriteString("## Task Files\n\n")
	fmt.Fprintf(&sb, "- PRD directory: `%s`\n", prdDir)
	fmt.Fprintf(&sb, "- Task file: `%s`\n", taskAbsPath)
	fmt.Fprintf(&sb, "- Master tasks file: `%s`\n", tasksFile)
	sb.WriteString("- Use these exact paths when `execute-prd-task` updates task tracking.\n")
	sb.WriteString(
		"- Execute every explicit `Validation`, `Test Plan`, or `Testing` item from the task and supporting PRD docs.\n",
	)
	sb.WriteString("- Update task checkboxes and task status only after " +
		"implementation, verification evidence, and self-review are complete.\n")
	sb.WriteString("- Update the master tasks file only when the current task is actually complete.\n")
	sb.WriteString(
		"- Keep tracking-only files out of automatic commits unless the repository explicitly requires them to be staged.\n",
	)
	if autoCommit {
		sb.WriteString("- Create one local commit after clean verification, " +
			"self-review, and tracking updates. Do not push automatically.\n")
	} else {
		sb.WriteString("- Do not create an automatic commit for this run. Leave the diff ready for manual review.\n")
	}
	return sb.String()
}
