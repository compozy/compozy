package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/compozy/looper/internal/looper/model"
)

type BatchParams struct {
	PR          string
	BatchGroups map[string][]model.IssueEntry
	Grouped     bool
	AutoCommit  bool
	Mode        model.ExecutionMode
}

func Build(p BatchParams) string {
	if p.Mode == model.ExecutionModePRDTasks {
		return buildPRDTasksPrompt(p)
	}
	return buildCodeReviewPrompt(p)
}

func ParseTaskFile(content string) model.TaskEntry {
	task := model.TaskEntry{Content: content}
	statusRe := regexp.MustCompile(`(?m)^##\s*status:\s*(\w+)`)
	if m := statusRe.FindStringSubmatch(content); len(m) > 1 {
		task.Status = strings.TrimSpace(m[1])
	}

	contextStart := strings.Index(content, "<task_context>")
	contextEnd := strings.Index(content, "</task_context>")
	if contextStart > 0 && contextEnd > contextStart {
		contextBlock := content[contextStart : contextEnd+15]
		task.Domain = extractXMLTag(contextBlock, "domain")
		task.TaskType = extractXMLTag(contextBlock, "type")
		task.Scope = extractXMLTag(contextBlock, "scope")
		task.Complexity = extractXMLTag(contextBlock, "complexity")
		if deps := extractXMLTag(contextBlock, "dependencies"); deps != "none" {
			task.Dependencies = strings.Split(deps, ",")
			for i := range task.Dependencies {
				task.Dependencies[i] = strings.TrimSpace(task.Dependencies[i])
			}
		}
	}
	return task
}

func IsTaskCompleted(task model.TaskEntry) bool {
	status := strings.ToLower(task.Status)
	return status == "completed" || status == "done" || status == "finished"
}

func ExtractTaskNumber(filename string) int {
	reTaskFile := regexp.MustCompile(`^_task_\d+\.md$`)
	if !reTaskFile.MatchString(filename) {
		return 0
	}

	numStr := strings.TrimPrefix(filename, "_task_")
	numStr = strings.TrimSuffix(numStr, ".md")
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0
	}
	return num
}

func FlattenAndSortIssues(groups map[string][]model.IssueEntry, mode model.ExecutionMode) []model.IssueEntry {
	allIssues := make([]model.IssueEntry, 0)
	for _, items := range groups {
		allIssues = append(allIssues, items...)
	}

	if mode == model.ExecutionModePRDTasks {
		sort.SliceStable(allIssues, func(i, j int) bool {
			numI := ExtractTaskNumber(allIssues[i].Name)
			numJ := ExtractTaskNumber(allIssues[j].Name)
			if numI != numJ {
				return numI < numJ
			}
			return allIssues[i].Name < allIssues[j].Name
		})
		return allIssues
	}

	sort.SliceStable(allIssues, func(i, j int) bool {
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
	return buildPRDTaskPrompt(task, p.AutoCommit)
}

func batchIssueRange(batchIssues []model.IssueEntry) (int, int, bool) {
	minIssue := 0
	maxIssue := 0
	hasIssueRange := false
	for _, issue := range batchIssues {
		issueNum, ok := parseIssueNumber(issue.Name)
		if !ok {
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

func parseIssueNumber(name string) (int, bool) {
	base := filepath.Base(name)
	parts := strings.SplitN(base, "-", 2)
	if len(parts) == 0 || !isAllDigits(parts[0]) {
		return 0, false
	}
	issueNum, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, false
	}
	return issueNum, true
}

func sortCodeFiles(batchGroups map[string][]model.IssueEntry) []string {
	codeFiles := make([]string, 0, len(batchGroups))
	for codeFile := range batchGroups {
		codeFiles = append(codeFiles, codeFile)
	}
	sort.Strings(codeFiles)
	return codeFiles
}

func buildBatchHeader(pr string, codeFiles []string, batchGroups map[string][]model.IssueEntry) string {
	totalIssues := 0
	for _, items := range batchGroups {
		totalIssues += len(items)
	}
	return fmt.Sprintf(`<arguments>
  <type>batched-issues</type>
  <pr>%s</pr>
  <files>%d</files>
  <total-issues>%d</total-issues>
</arguments>`, pr, len(codeFiles), totalIssues)
}

func buildBatchChecklist(pr string, batchGroups map[string][]model.IssueEntry, grouped bool) string {
	allIssues := make([]model.IssueEntry, 0)
	for _, items := range batchGroups {
		allIssues = append(allIssues, items...)
	}
	sort.Slice(allIssues, func(i, j int) bool {
		return allIssues[i].Name < allIssues[j].Name
	})

	var checklistPaths []string
	if grouped {
		seenGrouped := make(map[string]bool)
		for _, issue := range allIssues {
			groupedPath := fmt.Sprintf("ai-docs/reviews-pr-%s/issues/grouped/%s.md", pr, SafeFileName(issue.CodeFile))
			if !seenGrouped[groupedPath] {
				checklistPaths = append(checklistPaths, groupedPath)
				seenGrouped[groupedPath] = true
			}
		}
	}
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

func extractXMLTag(content, tag string) string {
	re := regexp.MustCompile(fmt.Sprintf(`<%s>(.*?)</%s>`, tag, tag))
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func sanitizePath(path string) string {
	runes := make([]rune, 0, len(path))
	for _, r := range path {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			runes = append(runes, r)
			continue
		}
		runes = append(runes, '_')
	}
	return string(runes)
}
