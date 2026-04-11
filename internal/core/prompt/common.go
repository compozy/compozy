package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/reviews"
	"github.com/compozy/compozy/internal/core/tasks"
)

type BatchParams struct {
	Name            string
	Round           int
	Provider        string
	PR              string
	ReviewsDir      string
	BatchGroups     map[string][]model.IssueEntry
	AutoCommit      bool
	CloseOnComplete bool
	Mode            model.ExecutionMode
	Memory          *WorkflowMemoryContext
}

func Build(p BatchParams) string {
	if p.Mode == model.ExecutionModePRDTasks {
		return buildPRDTasksPrompt(p)
	}
	return buildCodeReviewPrompt(p)
}

func BuildSystemPromptAddendum(p BatchParams) string {
	if p.Mode != model.ExecutionModePRDTasks {
		return ""
	}
	return buildPRDSystemPromptAddendum(p.Memory)
}

func FlattenAndSortIssues(groups map[string][]model.IssueEntry, mode model.ExecutionMode) []model.IssueEntry {
	allIssues := make([]model.IssueEntry, 0)
	for _, items := range groups {
		allIssues = append(allIssues, items...)
	}

	if mode == model.ExecutionModePRDTasks {
		sort.SliceStable(allIssues, func(i, j int) bool {
			numI := tasks.ExtractTaskNumber(allIssues[i].Name)
			numJ := tasks.ExtractTaskNumber(allIssues[j].Name)
			if numI != numJ {
				return numI < numJ
			}
			return allIssues[i].Name < allIssues[j].Name
		})
		return allIssues
	}

	sort.SliceStable(allIssues, func(i, j int) bool {
		numI := reviews.ExtractIssueNumber(allIssues[i].Name)
		numJ := reviews.ExtractIssueNumber(allIssues[j].Name)
		if numI != 0 && numJ != 0 && numI != numJ {
			return numI < numJ
		}
		return allIssues[i].Name < allIssues[j].Name
	})
	return allIssues
}

func SafeFileName(path string) string {
	norm := strings.ReplaceAll(path, "\\", "/")
	base := sanitizePath(norm)
	sum := sha256.Sum256([]byte(norm))
	hash := hex.EncodeToString(sum[:])[:6]
	return fmt.Sprintf("%s-%s", base, hash)
}

func NormalizeForPrompt(absPath string) string {
	resolvedPath, err := filepath.Abs(absPath)
	if err != nil {
		return absPath
	}
	cwd, err := os.Getwd()
	if err != nil {
		return resolvedPath
	}
	cwd = filepath.Clean(cwd)
	resolvedPath = filepath.Clean(resolvedPath)
	prefix := cwd + string(os.PathSeparator)
	if strings.HasPrefix(resolvedPath, prefix) {
		return resolvedPath[len(prefix):]
	}
	return resolvedPath
}

func buildPRDTasksPrompt(p BatchParams) string {
	var task model.IssueEntry
	for _, items := range p.BatchGroups {
		if len(items) > 0 {
			task = items[0]
			break
		}
	}
	return buildPRDTaskPrompt(task, p.AutoCommit, p.CloseOnComplete, p.Memory)
}

func batchIssueRange(batchIssues []model.IssueEntry) (int, int, bool) {
	minIssue := 0
	maxIssue := 0
	hasIssueRange := false
	for _, issue := range batchIssues {
		issueNum := reviews.ExtractIssueNumber(issue.Name)
		if issueNum == 0 {
			continue
		}
		if !hasIssueRange {
			minIssue = issueNum
			maxIssue = issueNum
			hasIssueRange = true
			continue
		}
		if issueNum < minIssue {
			minIssue = issueNum
		}
		if issueNum > maxIssue {
			maxIssue = issueNum
		}
	}
	return minIssue, maxIssue, hasIssueRange
}

func sortCodeFiles(batchGroups map[string][]model.IssueEntry) []string {
	codeFiles := make([]string, 0, len(batchGroups))
	for codeFile := range batchGroups {
		codeFiles = append(codeFiles, codeFile)
	}
	sort.Strings(codeFiles)
	return codeFiles
}

func buildBatchHeader(p BatchParams) string {
	totalIssues := 0
	for _, items := range p.BatchGroups {
		totalIssues += len(items)
	}
	codeFiles := sortCodeFiles(p.BatchGroups)

	lines := []string{
		"<arguments>",
		"  <type>batched-reviews</type>",
		fmt.Sprintf("  <files>%d</files>", len(codeFiles)),
		fmt.Sprintf("  <total-issues>%d</total-issues>", totalIssues),
	}
	if p.Name != "" {
		lines = append(lines, fmt.Sprintf("  <name>%s</name>", p.Name))
	}
	if p.Provider != "" {
		lines = append(lines, fmt.Sprintf("  <provider>%s</provider>", p.Provider))
	}
	if p.PR != "" {
		lines = append(lines, fmt.Sprintf("  <pr>%s</pr>", p.PR))
	}
	if p.Round > 0 {
		lines = append(lines, fmt.Sprintf("  <round>%d</round>", p.Round))
	}
	lines = append(lines, "</arguments>")
	return strings.Join(lines, "\n")
}

func buildBatchChecklist(p BatchParams) string {
	allIssues := make([]model.IssueEntry, 0)
	for _, items := range p.BatchGroups {
		allIssues = append(allIssues, items...)
	}
	sort.Slice(allIssues, func(i, j int) bool {
		numI := reviews.ExtractIssueNumber(allIssues[i].Name)
		numJ := reviews.ExtractIssueNumber(allIssues[j].Name)
		if numI != 0 && numJ != 0 && numI != numJ {
			return numI < numJ
		}
		return allIssues[i].Name < allIssues[j].Name
	})

	var checklistPaths []string
	for _, item := range allIssues {
		checklistPaths = append(checklistPaths, NormalizeForPrompt(item.AbsPath))
	}

	var chk strings.Builder
	chk.WriteString("\n<checklist>\n  <title>Progress Files to Update</title>\n")
	for _, path := range checklistPaths {
		chk.WriteString("  <path>")
		chk.WriteString(path)
		chk.WriteString("</path>\n")
	}
	chk.WriteString("</checklist>\n")
	return chk.String()
}

func sanitizePath(path string) string {
	runes := make([]rune, 0, len(path))
	for _, r := range path {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' ||
			r == '-' {
			runes = append(runes, r)
			continue
		}
		runes = append(runes, '_')
	}
	return string(runes)
}
