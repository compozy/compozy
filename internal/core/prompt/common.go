package prompt

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/core/frontmatter"
	"github.com/compozy/compozy/internal/core/model"
	"gopkg.in/yaml.v3"
)

var (
	ErrLegacyTaskMetadata   = errors.New("legacy XML task metadata detected")
	ErrV1TaskMetadata       = errors.New("v1 task front matter detected")
	ErrLegacyReviewMetadata = errors.New("legacy XML review metadata detected")
)

type BatchParams struct {
	Name        string
	Round       int
	Provider    string
	PR          string
	ReviewsDir  string
	BatchGroups map[string][]model.IssueEntry
	AutoCommit  bool
	Mode        model.ExecutionMode
	Memory      *WorkflowMemoryContext
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

func ParseTaskFile(content string) (model.TaskEntry, error) {
	var node yaml.Node
	if _, err := frontmatter.Parse(content, &node); err != nil {
		if LooksLikeLegacyTaskFile(content) {
			return model.TaskEntry{}, ErrLegacyTaskMetadata
		}
		return model.TaskEntry{}, fmt.Errorf("parse task front matter: %w", err)
	}
	if hasTaskV1FrontMatterKeys(&node) {
		return model.TaskEntry{}, ErrV1TaskMetadata
	}

	var meta model.TaskFileMeta
	if err := node.Decode(&meta); err != nil {
		return model.TaskEntry{}, fmt.Errorf("decode task front matter: %w", err)
	}

	task := model.TaskEntry{
		Content:      content,
		Status:       strings.TrimSpace(meta.Status),
		Title:        strings.TrimSpace(meta.Title),
		TaskType:     strings.TrimSpace(meta.TaskType),
		Complexity:   strings.TrimSpace(meta.Complexity),
		Dependencies: normalizeDependencies(meta.Dependencies),
	}
	if task.Status == "" {
		return model.TaskEntry{}, errors.New("task front matter missing status")
	}
	return task, nil
}

func ParseLegacyTaskFile(content string) (model.TaskEntry, error) {
	if !LooksLikeLegacyTaskFile(content) {
		return model.TaskEntry{}, errors.New("legacy task metadata not found")
	}

	task := model.TaskEntry{Content: content}
	statusRe := regexp.MustCompile(`(?m)^##\s*status:\s*(\w+)`)
	if m := statusRe.FindStringSubmatch(content); len(m) > 1 {
		task.Status = strings.TrimSpace(m[1])
	}

	contextStart := strings.Index(content, "<task_context>")
	contextEnd := strings.Index(content, "</task_context>")
	if contextStart == -1 || contextEnd <= contextStart {
		return model.TaskEntry{}, errors.New("task_context block not found")
	}

	contextBlock := content[contextStart : contextEnd+len("</task_context>")]
	task.TaskType = extractXMLTag(contextBlock, "type")
	task.Complexity = extractXMLTag(contextBlock, "complexity")
	task.Dependencies = normalizeLegacyDependencies(extractXMLTag(contextBlock, "dependencies"))
	if task.Status == "" {
		return model.TaskEntry{}, errors.New("legacy task status not found")
	}
	return task, nil
}

func hasTaskV1FrontMatterKeys(node *yaml.Node) bool {
	mapping := node
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return false
		}
		mapping = node.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return false
	}
	for idx := 0; idx+1 < len(mapping.Content); idx += 2 {
		keyNode := mapping.Content[idx]
		if keyNode.Kind != yaml.ScalarNode {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(keyNode.Value)) {
		case "domain", "scope":
			return true
		}
	}
	return false
}

func LooksLikeLegacyTaskFile(content string) bool {
	return strings.Contains(content, "<task_context>") ||
		regexp.MustCompile(`(?mi)^##\s*status:`).FindStringIndex(content) != nil
}

func ExtractLegacyTaskBody(content string) (string, error) {
	if !LooksLikeLegacyTaskFile(content) {
		return "", errors.New("legacy task metadata not found")
	}

	lines := strings.Split(content, "\n")
	body := make([]string, 0, len(lines))
	inContext := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case regexp.MustCompile(`(?mi)^##\s*status:`).MatchString(line):
			continue
		case trimmed == "<task_context>":
			inContext = true
			continue
		case trimmed == "</task_context>":
			inContext = false
			continue
		case inContext:
			continue
		default:
			body = append(body, line)
		}
	}

	return strings.TrimLeft(strings.Join(body, "\n"), "\n"), nil
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
	return buildPRDTaskPrompt(task, p.AutoCommit, p.Memory)
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
	var meta model.ReviewFileMeta
	if _, err := frontmatter.Parse(content, &meta); err != nil {
		if LooksLikeLegacyReviewFile(content) {
			return model.ReviewContext{}, ErrLegacyReviewMetadata
		}
		return model.ReviewContext{}, fmt.Errorf("parse review front matter: %w", err)
	}

	ctx := model.ReviewContext{
		Status:      strings.ToLower(strings.TrimSpace(meta.Status)),
		File:        strings.TrimSpace(meta.File),
		Line:        meta.Line,
		Severity:    strings.TrimSpace(meta.Severity),
		Author:      strings.TrimSpace(meta.Author),
		ProviderRef: strings.TrimSpace(meta.ProviderRef),
	}
	if ctx.Status == "" {
		return model.ReviewContext{}, errors.New("review front matter missing status")
	}
	return ctx, nil
}

func ParseLegacyReviewContext(content string) (model.ReviewContext, error) {
	if !LooksLikeLegacyReviewFile(content) {
		return model.ReviewContext{}, errors.New("legacy review metadata not found")
	}

	ctx := model.ReviewContext{
		Status:      strings.ToLower(strings.TrimSpace(extractLegacyStatus(content))),
		File:        extractXMLTag(content, "file"),
		Severity:    extractXMLTag(content, "severity"),
		Author:      extractXMLTag(content, "author"),
		ProviderRef: extractXMLTag(content, "provider_ref"),
	}
	lineValue := strings.TrimSpace(extractXMLTag(content, "line"))
	if lineValue != "" {
		lineNumber, err := strconv.Atoi(lineValue)
		if err != nil {
			return model.ReviewContext{}, fmt.Errorf("parse legacy review line: %w", err)
		}
		ctx.Line = lineNumber
	}
	if ctx.Status == "" {
		return model.ReviewContext{}, errors.New("legacy review status not found")
	}
	return ctx, nil
}

func LooksLikeLegacyReviewFile(content string) bool {
	return strings.Contains(content, "<review_context>") ||
		regexp.MustCompile(`(?mi)^##\s*status:`).FindStringIndex(content) != nil
}

func ExtractLegacyReviewBody(content string) (string, error) {
	if !LooksLikeLegacyReviewFile(content) {
		return "", errors.New("legacy review metadata not found")
	}

	lines := strings.Split(content, "\n")
	body := make([]string, 0, len(lines))
	inContext := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case regexp.MustCompile(`(?mi)^##\s*status:`).MatchString(line):
			continue
		case trimmed == "<review_context>":
			inContext = true
			continue
		case trimmed == "</review_context>":
			inContext = false
			continue
		case inContext:
			continue
		default:
			body = append(body, line)
		}
	}

	return strings.TrimLeft(strings.Join(body, "\n"), "\n"), nil
}

func ParseReviewStatus(content string) (string, error) {
	ctx, err := ParseReviewContext(content)
	if err != nil {
		return "", err
	}
	return ctx.Status, nil
}

func IsReviewResolved(content string) (bool, error) {
	status, err := ParseReviewStatus(content)
	if err != nil {
		return false, err
	}
	return status == "resolved", nil
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

func normalizeDependencies(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || strings.EqualFold(trimmed, "none") {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func normalizeLegacyDependencies(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return normalizeDependencies(strings.Split(raw, ","))
}

func extractLegacyStatus(content string) string {
	statusRe := regexp.MustCompile(`(?mi)^##\s*status:\s*([a-z]+)\b`)
	if matches := statusRe.FindStringSubmatch(content); len(matches) > 1 {
		return matches[1]
	}
	return ""
}
