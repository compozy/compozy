package prompt

import (
	"context"
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
	Name        string                        `json:"name,omitempty"`
	Round       int                           `json:"round,omitempty"`
	Provider    string                        `json:"provider,omitempty"`
	PR          string                        `json:"pr,omitempty"`
	ReviewsDir  string                        `json:"reviews_dir,omitempty"`
	BatchGroups map[string][]model.IssueEntry `json:"batch_groups,omitempty"`
	AutoCommit  bool                          `json:"auto_commit,omitempty"`
	Mode        model.ExecutionMode           `json:"mode,omitempty"`
	Memory      *WorkflowMemoryContext        `json:"memory,omitempty"`
	Context     context.Context               `json:"-"`
	RunID       string                        `json:"-"`
	JobID       string                        `json:"-"`
	RuntimeMgr  model.RuntimeManager          `json:"-"`
}

type promptPreBuildPayload struct {
	RunID       string      `json:"run_id"`
	JobID       string      `json:"job_id"`
	BatchParams BatchParams `json:"batch_params"`
}

type promptPostBuildPayload struct {
	RunID       string      `json:"run_id"`
	JobID       string      `json:"job_id"`
	PromptText  string      `json:"prompt_text"`
	BatchParams BatchParams `json:"batch_params"`
}

type promptPreSystemPayload struct {
	RunID          string      `json:"run_id"`
	JobID          string      `json:"job_id"`
	SystemAddendum string      `json:"system_addendum"`
	BatchParams    BatchParams `json:"batch_params"`
}

func Build(p BatchParams) (string, error) {
	p, err := dispatchPromptPreBuild(p)
	if err != nil {
		return "", err
	}

	var rendered string
	if p.Mode == model.ExecutionModePRDTasks {
		rendered = buildPRDTasksPrompt(p)
	} else {
		rendered = buildCodeReviewPrompt(p)
	}
	return dispatchPromptPostBuild(p, rendered)
}

func BuildSystemPromptAddendum(p BatchParams) (string, error) {
	if p.Mode != model.ExecutionModePRDTasks {
		return "", nil
	}

	addendum := buildPRDSystemPromptAddendum(p.Memory)
	return dispatchPromptPreSystem(p, addendum)
}

func (p BatchParams) withHookContextFrom(src BatchParams) BatchParams {
	p.Context = src.Context
	p.RunID = src.RunID
	p.JobID = src.JobID
	p.RuntimeMgr = src.RuntimeMgr
	return p
}

func dispatchPromptPreBuild(p BatchParams) (BatchParams, error) {
	if p.RuntimeMgr == nil {
		return p, nil
	}

	payload, err := model.DispatchMutableHook(
		p.context(),
		p.RuntimeMgr,
		"prompt.pre_build",
		promptPreBuildPayload{
			RunID:       p.RunID,
			JobID:       p.JobID,
			BatchParams: p,
		},
	)
	if err != nil {
		return BatchParams{}, err
	}
	return payload.BatchParams.withHookContextFrom(p), nil
}

func dispatchPromptPostBuild(p BatchParams, promptText string) (string, error) {
	if p.RuntimeMgr == nil {
		return promptText, nil
	}

	payload, err := model.DispatchMutableHook(
		p.context(),
		p.RuntimeMgr,
		"prompt.post_build",
		promptPostBuildPayload{
			RunID:       p.RunID,
			JobID:       p.JobID,
			PromptText:  promptText,
			BatchParams: p,
		},
	)
	if err != nil {
		return "", err
	}
	return payload.PromptText, nil
}

func dispatchPromptPreSystem(p BatchParams, systemAddendum string) (string, error) {
	if p.RuntimeMgr == nil {
		return systemAddendum, nil
	}

	payload, err := model.DispatchMutableHook(
		p.context(),
		p.RuntimeMgr,
		"prompt.pre_system",
		promptPreSystemPayload{
			RunID:          p.RunID,
			JobID:          p.JobID,
			SystemAddendum: systemAddendum,
			BatchParams:    p,
		},
	)
	if err != nil {
		return "", err
	}
	return payload.SystemAddendum, nil
}

func (p BatchParams) context() context.Context {
	if p.Context != nil {
		return p.Context
	}
	return context.Background()
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
	return buildPRDTaskPrompt(task, p.AutoCommit, p.Memory)
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
