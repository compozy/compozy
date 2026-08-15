package speccycle

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/compozy/compozy/internal/frontmatter"
	toolspkg "github.com/compozy/compozy/internal/tools"
	"gopkg.in/yaml.v3"
)

var reviewIssueFilePattern = regexp.MustCompile(`^issue_([0-9]+)\.md$`)

func (s *reviewArtifactStore) Finalize(
	ctx context.Context,
	scope *toolspkg.ExtensionToolWorkspaceScope,
	input finalizeReviewRoundInput,
) (_ finalizeReviewRoundOutput, retErr error) {
	taskName, err := resolveReviewTask(input.TaskName)
	if err != nil {
		return finalizeReviewRoundOutput{}, err
	}
	if input.Round <= 0 {
		return finalizeReviewRoundOutput{}, errors.New("spec-cycle: review round must be positive")
	}
	root, err := openReviewArtifactRoot(scope, taskName, false)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return finalizeReviewRoundOutput{}, reviewRoundNotFoundError(taskName, input.Round)
		}
		return finalizeReviewRoundOutput{}, err
	}
	defer func() {
		if closeErr := root.close(); closeErr != nil {
			retErr = errors.Join(retErr, closeErr)
		}
	}()

	roundDir := filepath.Join(root.taskDir, reviewRoundDirName(input.Round))
	if err := requireRealReviewDirectory(root.root, roundDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return finalizeReviewRoundOutput{}, reviewRoundNotFoundError(taskName, input.Round)
		}
		return finalizeReviewRoundOutput{}, err
	}
	paths, err := reviewRoundIssuePaths(root, roundDir)
	if err != nil {
		return finalizeReviewRoundOutput{}, err
	}
	if len(paths) == 0 {
		return finalizeReviewRoundOutput{}, reviewRoundNotFoundError(taskName, input.Round)
	}
	output := finalizeReviewRoundOutput{TaskName: taskName, Round: input.Round}
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return finalizeReviewRoundOutput{}, err
		}
		resolved, invalid, pending, finalizeErr := finalizeReviewIssue(root, path, input.Round)
		if finalizeErr != nil {
			return finalizeReviewRoundOutput{}, finalizeErr
		}
		output.Resolved += resolved
		output.Invalid += invalid
		output.Pending += pending
	}
	return output, nil
}

func finalizeReviewIssue(root *reviewArtifactRoot, path string, round int) (int, int, int, error) {
	meta, body, content, err := readReviewIssue(root, path)
	if err != nil {
		return 0, 0, 0, err
	}
	if meta.Round != round {
		return 0, 0, 0, fmt.Errorf(
			"spec-cycle: review issue %q belongs to round %03d, want %03d",
			path,
			meta.Round,
			round,
		)
	}
	status := strings.ToLower(strings.TrimSpace(meta.Status))
	invalid := 0
	if status == reviewStatusInvalid || reviewBodyDecisionIsInvalid(body) {
		invalid = 1
	}
	switch status {
	case reviewStatusPending:
		return 0, invalid, 1, nil
	case reviewStatusResolved:
		return 1, invalid, 0, nil
	case reviewStatusValid, reviewStatusInvalid:
		rewritten, rewriteErr := frontmatter.RewriteStringField(content, "status", reviewStatusResolved)
		if rewriteErr != nil {
			return 0, 0, 0, fmt.Errorf("spec-cycle: rewrite review issue %q: %w", path, rewriteErr)
		}
		if writeErr := root.root.WriteFile(path, []byte(rewritten), 0o600); writeErr != nil {
			return 0, 0, 0, fmt.Errorf("spec-cycle: write review issue %q: %w", path, writeErr)
		}
		return 1, invalid, 0, nil
	default:
		return 0, 0, 0, fmt.Errorf(
			"spec-cycle: review issue %q has unsupported status %q",
			path,
			meta.Status,
		)
	}
}

func reviewRoundIssuePaths(root *reviewArtifactRoot, roundDir string) ([]string, error) {
	entries, err := fs.ReadDir(root.root.FS(), roundDir)
	if err != nil {
		return nil, fmt.Errorf("spec-cycle: read review round: %w", err)
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Type().IsRegular() && reviewIssueFilePattern.MatchString(entry.Name()) {
			paths = append(paths, filepath.Join(roundDir, entry.Name()))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func readReviewIssue(
	root *reviewArtifactRoot,
	path string,
) (reviewFileMeta, string, string, error) {
	info, err := root.root.Lstat(path)
	if err != nil {
		return reviewFileMeta{}, "", "", fmt.Errorf("spec-cycle: inspect review issue %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return reviewFileMeta{}, "", "", fmt.Errorf("spec-cycle: review issue %q must be a regular file", path)
	}
	content, err := root.root.ReadFile(path)
	if err != nil {
		return reviewFileMeta{}, "", "", fmt.Errorf("spec-cycle: read review issue %q: %w", path, err)
	}
	var meta reviewFileMeta
	body, err := frontmatter.Decode(content, func(raw []byte) error {
		return yaml.Unmarshal(raw, &meta)
	})
	if err != nil {
		return reviewFileMeta{}, "", "", fmt.Errorf("spec-cycle: parse review issue %q: %w", path, err)
	}
	return meta, body, string(content), nil
}

func reviewBodyDecisionIsInvalid(body string) bool {
	for line := range strings.SplitSeq(body, "\n") {
		normalized := strings.ToLower(strings.TrimSpace(line))
		if normalized == "- decision: `invalid`" || normalized == "- decision: invalid" {
			return true
		}
	}
	return false
}
