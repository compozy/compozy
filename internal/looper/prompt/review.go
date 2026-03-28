package prompt

import (
	"fmt"
	"strings"

	"github.com/compozy/looper/internal/looper/model"
)

type reviewPromptContext struct {
	PR            string
	CodeFiles     []string
	BatchIssues   []model.IssueEntry
	Grouped       bool
	AutoCommit    bool
	MinIssue      int
	MaxIssue      int
	HasIssueRange bool
}

func buildCodeReviewPrompt(p BatchParams) string {
	codeFiles := sortCodeFiles(p.BatchGroups)
	batchIssues := FlattenAndSortIssues(p.BatchGroups, model.ExecutionModePRReview)
	minIssue, maxIssue, hasIssueRange := batchIssueRange(batchIssues)
	ctx := reviewPromptContext{
		PR:            p.PR,
		CodeFiles:     codeFiles,
		BatchIssues:   batchIssues,
		Grouped:       p.Grouped,
		AutoCommit:    p.AutoCommit,
		MinIssue:      minIssue,
		MaxIssue:      maxIssue,
		HasIssueRange: hasIssueRange,
	}

	sections := []string{
		buildBatchHeader(p.PR, codeFiles, p.BatchGroups),
		buildReviewRequiredSkillsSection(),
		buildReviewScopeSection(ctx),
		buildBatchIssueFilesSection(batchIssues),
		buildReviewExecutionSection(ctx),
		buildBatchChecklist(p.PR, p.BatchGroups, p.Grouped),
	}
	return strings.Join(sections, "\n\n")
}

func buildReviewRequiredSkillsSection() string {
	return `<required_skills>
- ` + "`fix-coderabbit-review`" + `: required remediation workflow for CodeRabbit review batches
- ` + "`verification-before-completion`" + `: required before any completion claim or automatic commit
</required_skills>`
}

func buildReviewScopeSection(ctx reviewPromptContext) string {
	var sb strings.Builder
	sb.WriteString("<critical>\n")
	sb.WriteString("- Use installed `fix-coderabbit-review` as the source of truth for this review workflow.\n")
	sb.WriteString(
		"- Apply the skill in looper batch mode only: the files listed in `<batch_issue_files>` are the entire scope for this run.\n",
	)
	sb.WriteString(
		"- If the skill refers to \"all unresolved issues\", interpret that as \"all unresolved issues from this batch\".\n",
	)
	sb.WriteString("- Skip the export step when the listed batch issue files already exist.\n")
	sb.WriteString(
		"- Use installed `verification-before-completion` before claiming this batch is complete or creating an automatic commit.\n",
	)
	sb.WriteString("- Do not update issue files or grouped trackers outside this batch.\n")
	sb.WriteString("</critical>\n\n")

	sb.WriteString("<batch_scope>\n")
	fmt.Fprintf(&sb, "- PR: `%s`\n", ctx.PR)
	if ctx.HasIssueRange {
		fmt.Fprintf(&sb, "- Issue range: `%03d-%03d`\n", ctx.MinIssue, ctx.MaxIssue)
	} else {
		sb.WriteString("- Issue range: `UNCONFIRMED`; use the explicit file list below\n")
	}
	if ctx.Grouped {
		sb.WriteString("- Grouped summaries: enabled\n")
	} else {
		sb.WriteString("- Grouped summaries: disabled\n")
	}
	if ctx.AutoCommit {
		sb.WriteString("- Automatic commits: enabled after clean verification\n")
	} else {
		sb.WriteString("- Automatic commits: disabled (`--auto-commit=false`)\n")
	}
	sb.WriteString("- Code files in scope:\n")
	for _, codeFile := range ctx.CodeFiles {
		fmt.Fprintf(&sb, "  - `%s`\n", codeFile)
	}
	sb.WriteString("</batch_scope>")
	return sb.String()
}

func buildBatchIssueFilesSection(batchIssues []model.IssueEntry) string {
	var sb strings.Builder
	sb.WriteString("<batch_issue_files>\n")
	for _, issue := range batchIssues {
		fmt.Fprintf(&sb, "- `%s` (%s)\n", NormalizeForPrompt(issue.AbsPath), issue.CodeFile)
	}
	sb.WriteString("</batch_issue_files>")
	return sb.String()
}

func buildReviewExecutionSection(ctx reviewPromptContext) string {
	var sb strings.Builder
	sb.WriteString("<execution_contract>\n")
	sb.WriteString(
		"1. Triage every listed issue file and record `VALID` or `INVALID` with technical reasoning directly in that issue file.\n",
	)
	sb.WriteString(
		"2. Implement complete production fixes and add or update tests for every `VALID` issue in this batch.\n",
	)
	sb.WriteString(
		"3. Use `verification-before-completion` to identify and run the repository's real verification commands before finishing or committing this batch.\n",
	)
	if ctx.HasIssueRange {
		fmt.Fprintf(&sb, "4. Resolve only the review threads for this batch range (`%03d-%03d`).\n",
			ctx.MinIssue,
			ctx.MaxIssue)
	} else {
		sb.WriteString(
			"4. Resolve only the review threads referenced by the issue files listed in `<batch_issue_files>`.\n",
		)
	}
	if ctx.Grouped {
		sb.WriteString("5. Update grouped tracker files only for the touched code files in this batch.\n")
	} else {
		sb.WriteString("5. Grouped tracker updates are disabled for this run.\n")
	}
	if ctx.AutoCommit {
		sb.WriteString(
			"6. Create exactly one local commit for this batch after clean verification. Do not push automatically.\n",
		)
	} else {
		sb.WriteString("6. Leave the changes ready for manual review and commit. Do not create an automatic commit.\n")
	}
	sb.WriteString("</execution_contract>")
	return sb.String()
}
