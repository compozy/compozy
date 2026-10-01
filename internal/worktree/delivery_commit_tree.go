package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (s *Service) deliveryExpectedTree(ctx context.Context, item Worktree, paths []string) (tree string, err error) {
	runner, ok := s.runner.(GitIndexRunner)
	if !ok {
		return "", refusal(ErrExitActionInvalid, "The Git runner cannot prepare an isolated delivery tree.")
	}
	for _, path := range paths {
		if err := validateCommitPath(item.Path, path); err != nil {
			return "", err
		}
	}
	directory, err := os.MkdirTemp("", "compozy-delivery-index-")
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(directory)) }()
	index := filepath.Join(directory, "index")
	if _, stderr, err := runner.RunWithIndex(ctx, item.Path, index, "read-tree", "HEAD"); err != nil {
		return "", fmt.Errorf("worktree: prepare delivery index: %s: %w", exitCommandOutput(nil, stderr, err), err)
	}
	args := []string{gitLiteralPathspecs, "add", "-A"}
	if len(paths) > 0 {
		args = append(args, "--")
		args = append(args, paths...)
	}
	if _, stderr, err := runner.RunWithIndex(ctx, item.Path, index, args...); err != nil {
		return "", fmt.Errorf(
			"worktree: stage isolated delivery tree: %s: %w",
			exitCommandOutput(nil, stderr, err),
			err,
		)
	}
	output, stderr, err := runner.RunWithIndex(ctx, item.Path, index, "write-tree")
	if err != nil {
		return "", fmt.Errorf("worktree: write delivery tree: %s: %w", exitCommandOutput(nil, stderr, err), err)
	}
	return strings.TrimSpace(string(output)), nil
}
