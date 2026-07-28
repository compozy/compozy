package devcycle

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/frontmatter"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type reviewArtifactStore struct {
	now func() time.Time
}

func newReviewArtifactStore() *reviewArtifactStore {
	return &reviewArtifactStore{now: time.Now}
}

func (s *reviewArtifactStore) Write(
	ctx context.Context,
	scope *toolspkg.ExtensionToolWorkspaceScope,
	input writeReviewArtifactsInput,
) (_ writeReviewArtifactsOutput, retErr error) {
	taskName, err := resolveReviewTask(input.TaskName)
	if err != nil {
		return writeReviewArtifactsOutput{}, err
	}
	if err := validateReviewIssues(input.Issues); err != nil {
		return writeReviewArtifactsOutput{}, err
	}
	if scope == nil || strings.TrimSpace(scope.ID) == "" || strings.TrimSpace(scope.Root) == "" {
		return writeReviewArtifactsOutput{}, errors.New("dev-cycle: trusted workspace scope is required")
	}
	empty := writeReviewArtifactsOutput{
		TaskName: taskName,
		Files:    []reviewArtifactFile{},
		Batches:  []reviewArtifactBatch{},
		Issues:   []ReviewIssue{},
	}
	if len(input.Issues) == 0 {
		return empty, nil
	}

	root, err := openReviewArtifactRoot(scope, taskName, true)
	if err != nil {
		return writeReviewArtifactsOutput{}, err
	}
	defer func() {
		if closeErr := root.close(); closeErr != nil {
			retErr = errors.Join(retErr, closeErr)
		}
	}()
	if err := ctx.Err(); err != nil {
		return writeReviewArtifactsOutput{}, err
	}
	round, roundDir, err := createExclusiveReviewRound(root)
	if err != nil {
		return writeReviewArtifactsOutput{}, err
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		if cleanupErr := root.root.RemoveAll(roundDir); cleanupErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("dev-cycle: clean incomplete review round: %w", cleanupErr))
		}
	}()

	createdAt := s.now().UTC()
	files := make([]reviewArtifactFile, 0, len(input.Issues))
	for index, issue := range input.Issues {
		if err := ctx.Err(); err != nil {
			return writeReviewArtifactsOutput{}, err
		}
		artifact, writeErr := writeReviewIssue(root, roundDir, index+1, round, createdAt, issue)
		if writeErr != nil {
			return writeReviewArtifactsOutput{}, writeErr
		}
		files = append(files, artifact)
	}
	committed = true
	return writeReviewArtifactsOutput{
		TaskName: taskName,
		Round:    round,
		RoundDir: roundDir,
		Files:    files,
		Batches:  batchReviewArtifacts(files),
		Issues:   input.Issues,
		Count:    len(files),
	}, nil
}

func resolveReviewTask(taskName string) (string, error) {
	if strings.TrimSpace(taskName) == "" {
		return "", reviewArtifactInputError("task_name", "is required")
	}
	if err := validateReviewTaskName(taskName); err != nil {
		return "", reviewArtifactInputError("task_name", err.Error())
	}
	return taskName, nil
}

func validateReviewIssues(issues []ReviewIssue) error {
	for index, issue := range issues {
		for _, required := range []struct {
			field string
			value string
		}{
			{field: reviewIssueFieldTitle, value: issue.Title},
			{field: reviewIssueFieldBody, value: issue.Body},
			{field: reviewIssueFieldSeverity, value: issue.Severity},
		} {
			if strings.TrimSpace(required.value) == "" {
				return reviewArtifactInputError(
					fmt.Sprintf("issues[%d].%s", index, required.field),
					"is required",
				)
			}
		}
		if issue.Line < 0 {
			return reviewArtifactInputError(
				fmt.Sprintf("issues[%d].line", index),
				"must not be negative",
			)
		}
	}
	return nil
}

func createExclusiveReviewRound(root *reviewArtifactRoot) (int, string, error) {
	for {
		rounds, err := discoverReviewRounds(root)
		if err != nil {
			return 0, "", err
		}
		round := 1
		if len(rounds) > 0 {
			round = rounds[len(rounds)-1] + 1
		}
		relative := filepath.Join(root.taskDir, reviewRoundDirName(round))
		if err := root.root.Mkdir(relative, 0o755); err != nil {
			if errors.Is(err, os.ErrExist) {
				continue
			}
			return 0, "", fmt.Errorf("dev-cycle: create review round %03d: %w", round, err)
		}
		return round, relative, nil
	}
}

func discoverReviewRounds(root *reviewArtifactRoot) ([]int, error) {
	entries, err := fs.ReadDir(root.root.FS(), root.taskDir)
	if err != nil {
		return nil, fmt.Errorf("dev-cycle: read review task directory: %w", err)
	}
	rounds := make([]int, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !strings.HasPrefix(name, "reviews-") {
			continue
		}
		round, parseErr := strconv.Atoi(strings.TrimPrefix(name, "reviews-"))
		if parseErr == nil && round > 0 {
			rounds = append(rounds, round)
		}
	}
	sort.Ints(rounds)
	return rounds, nil
}

func writeReviewIssue(
	root *reviewArtifactRoot,
	roundDir string,
	number int,
	round int,
	createdAt time.Time,
	issue ReviewIssue,
) (reviewArtifactFile, error) {
	file := fallbackReviewValue(issue.File, unknownReviewFile)
	meta := reviewFileMeta{
		Round:          round,
		RoundCreatedAt: createdAt,
		Status:         reviewStatusPending,
		File:           file,
		Line:           issue.Line,
		Severity:       strings.TrimSpace(issue.Severity),
		Author:         fallbackReviewValue(issue.Author, "unknown"),
	}
	title := strings.TrimSpace(issue.Title)
	body := strings.TrimSpace(issue.Body)
	contentBody := strings.Join([]string{
		fmt.Sprintf("# Issue %03d: %s", number, title),
		"",
		"## Review Comment",
		"",
		body,
		"",
		"## Triage",
		"",
		"- Decision: `UNREVIEWED`",
		"- Notes:",
		"",
	}, "\n")
	content, err := frontmatter.Format(meta, contentBody)
	if err != nil {
		return reviewArtifactFile{}, fmt.Errorf("dev-cycle: format review issue %03d: %w", number, err)
	}
	path := filepath.Join(roundDir, reviewIssueFileName(number))
	if err := root.root.WriteFile(path, []byte(content), 0o600); err != nil {
		return reviewArtifactFile{}, fmt.Errorf("dev-cycle: write review issue %03d: %w", number, err)
	}
	return reviewArtifactFile{
		Path: path,
		File: file,
	}, nil
}

func batchReviewArtifacts(files []reviewArtifactFile) []reviewArtifactBatch {
	indexes := make(map[string]int)
	batches := make([]reviewArtifactBatch, 0)
	for _, artifact := range files {
		index, ok := indexes[artifact.File]
		if !ok {
			index = len(batches)
			indexes[artifact.File] = index
			batches = append(batches, reviewArtifactBatch{File: artifact.File, IssueFiles: []string{}})
		}
		batches[index].IssueFiles = append(batches[index].IssueFiles, artifact.Path)
	}
	return batches
}

func fallbackReviewValue(value string, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}
