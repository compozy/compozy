package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/fileutil"
)

func (s *Service) resolveBaseRef(ctx context.Context, root, requested string) (string, string, error) {
	ref := strings.TrimSpace(requested)
	if ref == "" {
		stdout, _, err := s.runner.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
		if err == nil && strings.TrimSpace(string(stdout)) != "" {
			ref = strings.TrimPrefix(strings.TrimSpace(string(stdout)), "origin/")
		} else {
			stdout, stderr, headErr := s.runner.Run(ctx, root, "symbolic-ref", "--quiet", "--short", "HEAD")
			if headErr != nil {
				return "", "", refusal(ErrBaseRefNotFound, diagnostics.RedactAndBound(string(stderr), 2048))
			}
			ref = strings.TrimSpace(string(stdout))
		}
	}
	stdout, stderr, err := s.runner.Run(ctx, root, "rev-parse", "--verify", ref+"^{commit}")
	if err != nil {
		return "", "", refusal(ErrBaseRefNotFound, diagnostics.RedactAndBound(string(stderr), 2048))
	}
	return ref, strings.TrimSpace(string(stdout)), nil
}

func (s *Service) commonDir(ctx context.Context, root string) (string, error) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", ErrWorkspaceNotGitBacked
		}
		return "", fmt.Errorf("worktree: inspect workspace Git metadata: %w", err)
	}
	stdout, stderr, err := s.runner.Run(ctx, root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("worktree: resolve Git common directory: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err)
	}
	return fileutil.CanonicalExistingDirectory(strings.TrimSpace(string(stdout)))
}

func (s *Service) resolveGitDir(ctx context.Context, root string) (string, error) {
	stdout, stderr, err := s.runner.Run(ctx, root, "rev-parse", "--path-format=absolute", "--git-dir")
	if err != nil {
		return "", fmt.Errorf("worktree: resolve Git directory: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err)
	}
	return fileutil.CanonicalExistingDirectory(strings.TrimSpace(string(stdout)))
}

func (s *Service) readWorktreeList(ctx context.Context, root, commonDir string) ([]GitWorktree, error) {
	stdout, stderr, err := s.runner.Run(ctx, root, "--git-dir", commonDir, "worktree", "list", "--porcelain", "-z")
	if err != nil {
		return nil, fmt.Errorf("worktree: list Git worktrees: %s: %w", diagnostics.RedactAndBound(string(stderr), 2048), err)
	}
	return ParseWorktreeList(stdout)
}
