package plan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/prompt"
	"github.com/compozy/compozy/internal/core/reviews"
)

// ErrNoWork indicates that no unresolved issues or pending PRD tasks were found.
var ErrNoWork = errors.New("no issues to process")

func Prepare(_ context.Context, cfg *model.RuntimeConfig) (*model.SolvePreparation, error) {
	prep := &model.SolvePreparation{}

	var err error
	prep.ResolvedName, prep.InputDir, prep.InputDirPath, err = resolveInputs(cfg)
	if err != nil {
		return nil, err
	}
	if cfg.Mode == model.ExecutionModePRReview {
		meta, metaErr := reviews.ReadRoundMeta(prep.InputDirPath)
		if metaErr != nil {
			return nil, metaErr
		}
		cfg.Provider = meta.Provider
		cfg.PR = meta.PR
		cfg.Round = meta.Round
		cfg.ReviewsDir = prep.InputDirPath
		prep.ResolvedProvider = meta.Provider
		prep.ResolvedPR = meta.PR
		prep.ResolvedRound = meta.Round
	} else {
		cfg.TasksDir = prep.InputDirPath
	}
	if err := agent.EnsureAvailable(cfg); err != nil {
		return nil, err
	}

	entries, err := readIssueEntries(prep.InputDirPath, cfg.Mode, cfg.IncludeCompleted)
	if err != nil {
		return nil, err
	}
	entries, err = validateAndFilterEntries(entries, cfg)
	if err != nil {
		return nil, err
	}

	groups := groupIssues(entries)
	promptRoot, err := initPromptRoot(cfg)
	if err != nil {
		return nil, err
	}

	prep.Jobs, prep.GroupedSummarized, err = prepareJobs(
		cfg,
		groups,
		promptRoot,
		prep.InputDirPath,
	)
	if err != nil {
		return nil, err
	}

	return prep, nil
}

func prepareJobs(
	cfg *model.RuntimeConfig,
	groups map[string][]model.IssueEntry,
	promptRoot string,
	issuesDir string,
) ([]model.Job, bool, error) {
	effectiveBatchSize := cfg.BatchSize
	effectiveGrouped := cfg.Grouped
	if cfg.Mode == model.ExecutionModePRDTasks {
		effectiveBatchSize = 1
		effectiveGrouped = false
	}
	if effectiveBatchSize <= 0 {
		effectiveBatchSize = 1
	}

	collected := prompt.FlattenAndSortIssues(groups, cfg.Mode)
	batches := createIssueBatches(collected, effectiveBatchSize)
	if len(batches) == 0 {
		return nil, false, errors.New("no batches created for prompt preparation")
	}

	groupedWritten := false
	if effectiveGrouped {
		if err := reviews.WriteGroupedSummaries(issuesDir, groups); err != nil {
			return nil, false, fmt.Errorf("write grouped summaries: %w", err)
		}
		groupedWritten = true
	}

	jobs := make([]model.Job, 0, len(batches))
	for idx, batchIssues := range batches {
		job, err := buildBatchJob(cfg, promptRoot, effectiveGrouped, idx, batchIssues)
		if err != nil {
			return nil, groupedWritten, err
		}
		jobs = append(jobs, job)
	}
	if len(jobs) == 0 {
		return nil, groupedWritten, errors.New("no jobs finalized")
	}
	return jobs, groupedWritten, nil
}

func buildBatchJob(
	cfg *model.RuntimeConfig,
	promptRoot string,
	grouped bool,
	batchIdx int,
	batchIssues []model.IssueEntry,
) (model.Job, error) {
	batchGroups, batchFiles := groupIssuesByCodeFile(batchIssues)
	safeName := determineBatchName(batchIdx, batchFiles, cfg.Mode)
	promptText := prompt.Build(prompt.BatchParams{
		Name:        cfg.Name,
		Round:       cfg.Round,
		Provider:    cfg.Provider,
		PR:          cfg.PR,
		ReviewsDir:  cfg.ReviewsDir,
		BatchGroups: batchGroups,
		Grouped:     grouped,
		AutoCommit:  cfg.AutoCommit,
		Mode:        cfg.Mode,
	})
	outPromptPath, outLog, errLog, err := writeBatchArtifacts(promptRoot, safeName, promptText)
	if err != nil {
		return model.Job{}, err
	}
	return model.Job{
		CodeFiles:     batchFiles,
		Groups:        batchGroups,
		SafeName:      safeName,
		Prompt:        []byte(promptText),
		OutPromptPath: outPromptPath,
		OutLog:        outLog,
		ErrLog:        errLog,
	}, nil
}

func determineBatchName(batchIdx int, batchFiles []string, mode model.ExecutionMode) string {
	if mode == model.ExecutionModePRDTasks {
		if len(batchFiles) == 1 {
			return prompt.SafeFileName(batchFiles[0])
		}
		return fmt.Sprintf("task_%03d", batchIdx+1)
	}
	if len(batchFiles) == 1 {
		filename := batchFiles[0]
		if strings.HasPrefix(filename, "__unknown__") {
			filename = model.UnknownFileName
		}
		return prompt.SafeFileName(filename)
	}
	return fmt.Sprintf("batch_%03d", batchIdx+1)
}

func writeBatchArtifacts(promptRoot, safeName, promptText string) (string, string, string, error) {
	outPromptPath := filepath.Join(promptRoot, fmt.Sprintf("%s.prompt.md", safeName))
	if err := os.WriteFile(outPromptPath, []byte(promptText), 0o600); err != nil {
		return "", "", "", fmt.Errorf("write prompt: %w", err)
	}
	outLog := filepath.Join(promptRoot, fmt.Sprintf("%s.out.log", safeName))
	errLog := filepath.Join(promptRoot, fmt.Sprintf("%s.err.log", safeName))
	return outPromptPath, outLog, errLog, nil
}

func createIssueBatches(allIssues []model.IssueEntry, batchSize int) [][]model.IssueEntry {
	batches := make([][]model.IssueEntry, 0)
	for i := 0; i < len(allIssues); i += batchSize {
		end := i + batchSize
		if end > len(allIssues) {
			end = len(allIssues)
		}
		batches = append(batches, allIssues[i:end])
	}
	return batches
}

func groupIssuesByCodeFile(issues []model.IssueEntry) (map[string][]model.IssueEntry, []string) {
	batchGroups := make(map[string][]model.IssueEntry)
	for _, issue := range issues {
		batchGroups[issue.CodeFile] = append(batchGroups[issue.CodeFile], issue)
	}
	batchFiles := make([]string, 0, len(batchGroups))
	for codeFile := range batchGroups {
		batchFiles = append(batchFiles, codeFile)
	}
	sort.Strings(batchFiles)
	return batchGroups, batchFiles
}
