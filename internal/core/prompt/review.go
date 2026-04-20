package prompt

import (
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
)

type reviewPromptContext struct {
	Name          string
	Round         int
	Provider      string
	PR            string
	ReviewsDir    string
	CodeFiles     []string
	BatchIssues   []model.IssueEntry
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
		Name:          p.Name,
		Round:         p.Round,
		Provider:      p.Provider,
		PR:            p.PR,
		ReviewsDir:    p.ReviewsDir,
		CodeFiles:     codeFiles,
		BatchIssues:   batchIssues,
		AutoCommit:    p.AutoCommit,
		MinIssue:      minIssue,
		MaxIssue:      maxIssue,
		HasIssueRange: hasIssueRange,
	}

	sections := []string{
		buildBatchHeader(p),
		buildReviewRequiredSkillsSection(),
		buildReviewScopeSection(ctx),
		buildBatchIssueFilesSection(batchIssues),
		buildReviewExecutionSection(ctx),
		buildBatchChecklist(p),
	}
	return strings.Join(sections, "\n\n")
}

func buildReviewRequiredSkillsSection() string {
	return `<required_skills>
- ` + "`cy-fix-reviews`" + `: required remediation workflow for review issue batches
- ` + "`cy-final-verify`" + `: required before any completion claim or automatic commit
</required_skills>`
}

func buildReviewScopeSection(ctx reviewPromptContext) string {
	var sb strings.Builder
	sb.WriteString("<critical>\n")
	sb.WriteString("- Use installed `cy-fix-reviews` as the source of truth for this review workflow.\n")
	sb.WriteString("- The files listed in `<batch_issue_files>` are the entire scope for this run.\n")
	sb.WriteString(
		"- Do not call provider-specific scripts, `gh` mutations, or other external resolution commands. Compozy resolves provider threads after the batch succeeds.\n",
	)
	sb.WriteString("- Do not edit issue files outside this batch.\n")
	sb.WriteString(
		"- Use installed `cy-final-verify` before claiming this batch is complete or creating an automatic commit.\n",
	)
	sb.WriteString("</critical>\n\n")

	sb.WriteString("<batch_scope>\n")
	if ctx.Name != "" {
		fmt.Fprintf(&sb, "- PRD name: `%s`\n", ctx.Name)
	}
	if ctx.Provider != "" {
		fmt.Fprintf(&sb, "- Provider: `%s`\n", ctx.Provider)
	}
	if ctx.PR != "" {
		fmt.Fprintf(&sb, "- Pull request: `%s`\n", ctx.PR)
	}
	if ctx.Round > 0 {
		fmt.Fprintf(&sb, "- Review round: `%03d`\n", ctx.Round)
	}
	if ctx.ReviewsDir != "" {
		fmt.Fprintf(&sb, "- Reviews directory: `%s`\n", NormalizeForPrompt(ctx.ReviewsDir))
	}
	if ctx.HasIssueRange {
		fmt.Fprintf(&sb, "- Issue range: `issue_%03d.md` → `issue_%03d.md`\n", ctx.MinIssue, ctx.MaxIssue)
	} else {
		sb.WriteString("- Issue range: `UNCONFIRMED`; use the explicit file list below\n")
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
	sb.WriteString("1. Read every listed issue file completely before editing code.\n")
	sb.WriteString(
		"2. Triage every listed issue file and update frontmatter `status` from `pending` to `valid` or `invalid` with concrete technical reasoning in the `## Triage` section.\n",
	)
	sb.WriteString(
		"3. Implement complete production fixes and add or update tests for every `valid` issue in this batch.\n",
	)
	sb.WriteString(
		"4. If an issue is `invalid`, document the reasoning clearly and still finish the issue file by setting frontmatter `status: resolved` once the analysis is complete.\n",
	)
	sb.WriteString(
		"5. For every completed `valid` issue, finish the issue file with frontmatter `status: resolved` only after the code and verification are done.\n",
	)
	sb.WriteString(
		"6. Use `cy-final-verify` to identify and run the repository's real verification commands before finishing or committing this batch.\n",
	)
	if ctx.AutoCommit {
		sb.WriteString(
			"7. Create exactly one local commit for this batch after clean verification. Do not push automatically.\n",
		)
	} else {
		sb.WriteString("7. Leave the changes ready for manual review and commit. Do not create an automatic commit.\n")
	}
	sb.WriteString("</execution_contract>")
	return sb.String()
}
