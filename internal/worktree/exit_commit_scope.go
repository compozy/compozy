package worktree

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const gitLiteralPathspecs = "--literal-pathspecs"

func (s *Service) ExitPlanForPaths(ctx context.Context, workspaceID, ref string, paths []string) (*ExitPlan, error) {
	plan, err := s.ExitPlan(ctx, workspaceID, ref)
	if err != nil || len(paths) == 0 {
		return plan, err
	}
	item, err := s.store.Get(ctx, workspaceID, plan.WorktreeID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, ErrNotFound
	}
	scope, err := s.selectedCommitScope(ctx, item.Path, paths)
	if err != nil {
		return nil, err
	}
	plan.CommitScope = scope
	return plan, nil
}

func (s *Service) selectedCommitScope(ctx context.Context, root string, paths []string) (ExitCommitScope, error) {
	selected := slices.Clone(paths)
	slices.Sort(selected)
	selected = slices.Compact(selected)
	hash := sha256.New()
	head, _, err := s.runner.Run(ctx, root, "rev-parse", "HEAD")
	if err != nil {
		return ExitCommitScope{}, err
	}
	fmt.Fprintf(hash, "HEAD:%s\x00", head)
	branch, _, err := s.runner.Run(ctx, root, "symbolic-ref", "HEAD")
	if err != nil {
		return ExitCommitScope{}, err
	}
	fmt.Fprintf(hash, "branch:%s\x00", branch)
	for _, path := range selected {
		if err := validateCommitPath(root, path); err != nil {
			return ExitCommitScope{}, err
		}
		index, err := s.selectedIndexEntry(ctx, root, path)
		if err != nil {
			return ExitCommitScope{}, err
		}
		fmt.Fprintf(hash, "index:%s\x00", index)
		info, err := os.Lstat(filepath.Join(root, path))
		fmt.Fprintf(hash, "%s\x00", path)
		if errors.Is(err, os.ErrNotExist) {
			if len(index) == 0 {
				tracked, err := s.trackedCommitDeletion(ctx, root, path)
				if err != nil {
					return ExitCommitScope{}, err
				}
				if !tracked {
					return ExitCommitScope{}, refusal(
						ErrExitActionInvalid,
						"Commit paths must name regular files or tracked deletions.",
					)
				}
			}
			fmt.Fprint(hash, "deleted\x00")
			continue
		}
		if err != nil {
			return ExitCommitScope{}, err
		}
		if !info.Mode().IsRegular() {
			return ExitCommitScope{}, refusal(
				ErrExitActionInvalid,
				"Commit paths must be regular files or tracked deletions.",
			)
		}
		fmt.Fprintf(hash, "%d\x00", info.Mode())
		file, err := os.Open(filepath.Join(root, path))
		if err != nil {
			return ExitCommitScope{}, err
		}
		contentHash := sha256.New()
		_, copyErr := io.Copy(contentHash, file)
		closeErr := file.Close()
		if err := errors.Join(copyErr, closeErr); err != nil {
			return ExitCommitScope{}, err
		}
		fmt.Fprintf(hash, "%x\x00", contentHash.Sum(nil))
	}
	return ExitCommitScope{
		IncludePaths: selected,
		Fingerprint:  hex.EncodeToString(hash.Sum(nil)),
		Complete:     true,
		ChangedFiles: len(selected),
	}, nil
}

func (s *Service) selectedIndexEntry(ctx context.Context, root, path string) ([]byte, error) {
	index, _, err := s.runner.Run(ctx, root, gitLiteralPathspecs, "ls-files", "--stage", "-z", "--", path)
	if err != nil {
		return nil, err
	}
	if len(index) > 3*(len(path)+128) {
		return nil, refusal(ErrExitActionInvalid, "Selected index metadata exceeds the exact-file bound.")
	}
	records := strings.Split(string(index), "\x00")
	stages := map[string]bool{}
	for _, record := range records {
		if record == "" {
			continue
		}
		metadata, name, found := strings.Cut(record, "\t")
		fields := strings.Fields(metadata)
		if !found || name != path || len(fields) != 3 || len(fields[0]) != 6 || len(fields[2]) != 1 ||
			fields[2][0] < '0' ||
			fields[2][0] > '3' ||
			stages[fields[2]] {
			return nil, refusal(
				ErrExitActionInvalid,
				"Commit scope must contain exact file paths, not directory pathspecs.",
			)
		}
		oid, decodeErr := hex.DecodeString(fields[1])
		if decodeErr != nil || (len(oid) != 20 && len(oid) != 32) {
			return nil, refusal(ErrExitActionInvalid, "Selected index object identity is invalid.")
		}
		stages[fields[2]] = true
	}
	return index, nil
}

func (s *Service) trackedCommitDeletion(ctx context.Context, root, path string) (bool, error) {
	entry, _, err := s.runner.Run(ctx, root, gitLiteralPathspecs, "ls-tree", "-z", "HEAD", "--", path)
	if err != nil {
		return false, err
	}
	if len(entry) > len(path)+128 {
		return false, refusal(ErrExitActionInvalid, "Selected tracked deletion metadata exceeds the exact-file bound.")
	}
	metadata, name, found := strings.Cut(strings.TrimSuffix(string(entry), "\x00"), "\t")
	fields := strings.Fields(metadata)
	return found && name == path && len(fields) == 3 && fields[0] != "040000", nil
}

func validateCommitPath(root, path string) error {
	if path == "" || len(path) > 4096 || filepath.IsAbs(path) || filepath.Clean(path) != path || path == "." ||
		strings.Contains(path, "\\") ||
		strings.ContainsRune(path, 0) ||
		path == ".." ||
		strings.HasPrefix(path, "../") {
		return refusal(ErrExitActionInvalid, "Commit paths must be exact worktree-relative file paths.")
	}
	current := root
	for component := range strings.SplitSeq(filepath.ToSlash(path), "/") {
		if component == ".git" {
			return refusal(ErrExitActionInvalid, "Git metadata cannot be committed.")
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return refusal(ErrExitActionInvalid, "Commit paths cannot traverse symlinks.")
		}
	}
	return nil
}

func (s *Service) runSelectedExitCommit(
	ctx context.Context,
	operation ExitOperation,
	action ExitAction,
	item Worktree,
	scope ExitCommitScope,
	message string,
) (ExitStepResult, error) {
	step := ExitStepResult{
		Phase:        ExitPhaseCommit,
		State:        exitStepFailed,
		IncludePaths: slices.Clone(scope.IncludePaths),
	}
	current, err := s.selectedCommitScope(ctx, item.Path, scope.IncludePaths)
	if err != nil {
		return step, err
	}
	if current.Fingerprint != scope.Fingerprint {
		return step, refusal(ErrExitActionInvalid, "Reviewed commit scope changed before staging.")
	}
	expectedTree, err := s.deliveryExpectedTree(ctx, item, scope.IncludePaths)
	if err != nil {
		return step, err
	}
	reviewedHead, _, err := s.runner.Run(ctx, item.Path, "rev-parse", "HEAD")
	if err != nil {
		return step, err
	}
	current, err = s.selectedCommitScope(ctx, item.Path, scope.IncludePaths)
	if err != nil {
		return step, err
	}
	if current.Fingerprint != scope.Fingerprint {
		return step, refusal(ErrExitActionInvalid, "Reviewed commit scope changed before staging.")
	}
	if err := s.stageSelectedCommitPaths(ctx, item.Path, scope.IncludePaths); err != nil {
		step.Output = exitCommandOutput(nil, nil, err)
		return step, err
	}
	if strings.TrimSpace(message) == "" {
		message = fmt.Sprintf("Update %d files", len(scope.IncludePaths))
	}
	args := append([]string{gitLiteralPathspecs, "commit", "--only", "-m", message, "--"}, scope.IncludePaths...)
	stdout, stderr, err := s.runExitCommitArgs(ctx, operation, action, item.Path, args...)
	if err != nil {
		step.Output = exitCommandOutput(stdout, stderr, err)
		return step, err
	}
	sha, _, err := s.runner.Run(ctx, item.Path, "rev-parse", "HEAD")
	if err != nil {
		return step, err
	}
	if err := s.verifySelectedCommit(ctx, item.Path, strings.TrimSpace(string(sha)),
		strings.TrimSpace(string(reviewedHead)), expectedTree); err != nil {
		return step, err
	}
	step.State, step.SHA = exitStepCompleted, strings.TrimSpace(string(sha))
	return step, nil
}

func (s *Service) stageSelectedCommitPaths(ctx context.Context, root string, paths []string) error {
	stage := make([]string, 0, len(paths))
	for _, path := range paths {
		_, err := os.Lstat(filepath.Join(root, path))
		if errors.Is(err, os.ErrNotExist) {
			entry, indexErr := s.selectedIndexEntry(ctx, root, path)
			if indexErr != nil {
				return indexErr
			}
			if len(entry) == 0 {
				continue
			}
		} else if err != nil {
			return err
		}
		stage = append(stage, path)
	}
	if len(stage) == 0 {
		return nil
	}
	args := append([]string{gitLiteralPathspecs, "add", "-A", "--"}, stage...)
	_, _, err := s.runner.Run(ctx, root, args...)
	return err
}

func (s *Service) verifySelectedCommit(ctx context.Context, root, sha, reviewedHead, expectedTree string) error {
	actualTree, _, err := s.runner.Run(ctx, root, "rev-parse", sha+"^{tree}")
	if err != nil {
		return err
	}
	parents, _, err := s.runner.Run(ctx, root, "rev-list", "--parents", "-n", "1", sha)
	if err != nil {
		return err
	}
	identity := strings.Fields(string(parents))
	if strings.TrimSpace(string(actualTree)) != expectedTree || len(identity) != 2 ||
		identity[0] != sha || identity[1] != reviewedHead {
		return refusal(ErrSafetyCheckFailed, "Created commit does not match the reviewed delivery tree and parent.")
	}
	return nil
}
