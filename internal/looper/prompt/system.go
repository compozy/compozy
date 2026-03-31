package prompt

import (
	"fmt"
	"strings"

	"github.com/compozy/looper/internal/looper/model"
)

func BuildSystemPrompt(mode model.ExecutionMode, jobID string, serverPort int) string {
	sections := []string{
		buildSystemPromptPreamble(),
		buildModeSystemPrompt(mode),
		buildJobDoneSection(jobID, serverPort),
	}
	return strings.Join(sections, "\n\n")
}

func buildSystemPromptPreamble() string {
	return strings.Join([]string{
		"You are running inside Looper's interactive terminal workflow.",
		"The first composer message will point you to the full task or review prompt file for this job.",
		"Read that referenced file completely before editing code, and treat it as the task-specific source of truth.",
	}, "\n")
}

func buildModeSystemPrompt(mode model.ExecutionMode) string {
	switch mode {
	case model.ExecutionModePRDTasks:
		return buildPRDTaskSystemPrompt()
	case model.ExecutionModePRReview:
		return buildPRReviewSystemPrompt()
	default:
		return buildGenericSystemPrompt()
	}
}

func buildPRDTaskSystemPrompt() string {
	return strings.Join([]string{
		"<required_skills>",
		"- `execute-prd-task`: required end-to-end workflow for a PRD task",
		"- `verification-before-completion`: required before any completion claim or automatic commit",
		"</required_skills>",
		"",
		"<critical>",
		"- Use installed `execute-prd-task` as the execution workflow for this task.",
		"- Treat the referenced task specification and supporting PRD docs, especially `_techspec.md` and `_tasks.md`, as the source of truth.",
		"- Keep scope tight to the current task and record meaningful follow-up work instead of expanding scope silently.",
		"- Use installed `verification-before-completion` before any completion claim or automatic commit.",
		"- Update task tracking only after implementation, verification evidence, and self-review are complete.",
		"</critical>",
	}, "\n")
}

func buildPRReviewSystemPrompt() string {
	return strings.Join([]string{
		"<required_skills>",
		"- `fix-reviews`: required remediation workflow for review issue batches",
		"- `verification-before-completion`: required before any completion claim or automatic commit",
		"</required_skills>",
		"",
		"<critical>",
		"- Use installed `fix-reviews` as the source of truth for this review workflow.",
		"- Read every issue file referenced in the composer-provided prompt completely before editing code.",
		"- Triage each issue, implement complete fixes for every valid issue, " +
			"and update issue status files only after verification.",
		"- Do not call provider-specific resolution scripts or external resolution commands; " +
			"Looper resolves provider threads after the batch succeeds.",
		"- Use installed `verification-before-completion` before any completion claim or automatic commit.",
		"</critical>",
	}, "\n")
}

func buildGenericSystemPrompt() string {
	return strings.Join([]string{
		"<critical>",
		"- Follow the referenced prompt file as the source of truth for this job.",
		"- Use `verification-before-completion` before any completion claim.",
		"</critical>",
	}, "\n")
}

func buildJobDoneSection(jobID string, serverPort int) string {
	return fmt.Sprintf(strings.Join([]string{
		"When you have completed the assigned work and verification, you MUST run this command exactly once:",
		"curl -sS -X POST http://localhost:%d/job/done -H 'Content-Type: application/json' -d '{\"id\":\"%s\"}'",
		"Do NOT skip this step and do NOT claim the job is done before sending the signal.",
	}, "\n"), serverPort, jobID)
}
