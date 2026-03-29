package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
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
	Name        string
	Round       int
	Provider    string
	PR          string
	ReviewsDir  string
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
	return extractFileNumber(filename, regexp.MustCompile(`^task_(\d+)\.md$`))
}

func ExtractIssueNumber(filename string) int {
	return extractFileNumber(filename, regexp.MustCompile(`^issue_(\d+)\.md$`))
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
		numI := ExtractIssueNumber(allIssues[i].Name)
		numJ := ExtractIssueNumber(allIssues[j].Name)
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
	return buildPRDTaskPrompt(task, p.AutoCommit)
}

func batchIssueRange(batchIssues []model.IssueEntry) (int, int, bool) {
	minIssue := 0
	maxIssue := 0
	hasIssueRange := false
	for _, issue := range batchIssues {
		issueNum := ExtractIssueNumber(issue.Name)
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

func ParseReviewContext(content string) (model.ReviewContext, error) {
	start := strings.Index(content, "<review_context>")
	end := strings.Index(content, "</review_context>")
	if start == -1 || end == -1 || end < start {
		return model.ReviewContext{}, fmt.Errorf("review_context block not found")
	}

	block := content[start : end+len("</review_context>")]
	var parsed struct {
		XMLName     xml.Name `xml:"review_context"`
		File        string   `xml:"file"`
		Line        int      `xml:"line"`
		Severity    string   `xml:"severity"`
		Author      string   `xml:"author"`
		ProviderRef string   `xml:"provider_ref"`
	}
	if err := xml.Unmarshal([]byte(block), &parsed); err != nil {
		return model.ReviewContext{}, fmt.Errorf("parse review_context: %w", err)
	}

	return model.ReviewContext{
		File:        strings.TrimSpace(parsed.File),
		Line:        parsed.Line,
		Severity:    strings.TrimSpace(parsed.Severity),
		Author:      strings.TrimSpace(parsed.Author),
		ProviderRef: strings.TrimSpace(parsed.ProviderRef),
	}, nil
}

func ParseReviewStatus(content string) string {
	statusRe := regexp.MustCompile(`(?mi)^##\s*status:\s*([a-z]+)\b`)
	if matches := statusRe.FindStringSubmatch(content); len(matches) > 1 {
		return strings.ToLower(strings.TrimSpace(matches[1]))
	}
	return ""
}

func IsReviewResolved(content string) bool {
	if ParseReviewStatus(content) == "resolved" {
		return true
	}
	reResolvedLegacy := regexp.MustCompile(`(?mi)^\s*(status|state)\s*:\s*resolved\b`)
	reResolvedTask := regexp.MustCompile(`(?mi)^\s*-\s*\[(x|X)\]\s*resolved\b`)
	if strings.Contains(strings.ToUpper(content), "RESOLVED ✓") {
		return true
	}
	if reResolvedLegacy.FindStringIndex(content) != nil {
		return true
	}
	return reResolvedTask.FindStringIndex(content) != nil
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
		numI := ExtractIssueNumber(allIssues[i].Name)
		numJ := ExtractIssueNumber(allIssues[j].Name)
		if numI != 0 && numJ != 0 && numI != numJ {
			return numI < numJ
		}
		return allIssues[i].Name < allIssues[j].Name
	})

	var checklistPaths []string
	if p.Grouped {
		seenGrouped := make(map[string]bool)
		for _, issue := range allIssues {
			groupedPath := NormalizeForPrompt(
				filepath.Join(p.ReviewsDir, "grouped", fmt.Sprintf("group_%s.md", SafeFileName(issue.CodeFile))),
			)
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

func extractFileNumber(filename string, pattern *regexp.Regexp) int {
	matches := pattern.FindStringSubmatch(filepath.Base(filename))
	if len(matches) < 2 {
		return 0
	}
	num, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0
	}
	return num
}

func extractXMLTag(content, tag string) string {
	re := regexp.MustCompile(fmt.Sprintf(`<%s>(.*?)</%s>`, tag, tag))
	if m := re.FindStringSubmatch(content); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
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
