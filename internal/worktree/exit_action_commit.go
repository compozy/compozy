package worktree

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

func (s *Service) runExitCommit(
	ctx context.Context,
	operation ExitOperation,
	action ExitAction,
	item Worktree,
	scope ExitCommitScope,
	message string,
) (ExitStepResult, error) {
	step := ExitStepResult{Phase: ExitPhaseCommit, State: "completed"}
	if _, stderr, err := s.runner.Run(ctx, item.Path, "add", "-A"); err != nil {
		step.State, step.Output = "failed", exitCommandOutput(nil, stderr, err)
		return step, err
	}
	staged, stderr, err := s.runner.Run(ctx, item.Path, "diff", "--cached", "--name-only", "-z")
	if err != nil {
		step.State, step.Output = "failed", exitCommandOutput(nil, stderr, err)
		return step, err
	}
	if len(bytes.Trim(staged, "\x00")) == 0 {
		step.State, step.Reason = "skipped", "nothing to commit"
		return step, nil
	}
	commitMessage := strings.TrimSpace(message)
	if commitMessage == "" {
		count := scope.ChangedFiles
		if count <= 0 {
			count = len(bytes.Split(bytes.Trim(staged, "\x00"), []byte{0}))
		}
		commitMessage = fmt.Sprintf("Update %d files", count)
	}
	stdout, stderr, err := s.runExitCommitCommand(ctx, operation, action, item.Path, commitMessage)
	if err != nil {
		step.State, step.Output = "failed", exitCommandOutput(stdout, stderr, err)
		return step, err
	}
	sha, stderr, err := s.runner.Run(ctx, item.Path, "rev-parse", "HEAD")
	if err != nil {
		step.State, step.Output = "failed", exitCommandOutput(nil, stderr, err)
		return step, err
	}
	step.SHA = strings.TrimSpace(string(sha))
	return step, nil
}

func (s *Service) runExitCommitCommand(
	ctx context.Context,
	operation ExitOperation,
	action ExitAction,
	worktreePath string,
	message string,
) ([]byte, []byte, error) {
	runner, ok := s.runner.(StreamingGitRunner)
	if !ok {
		return s.runner.Run(ctx, worktreePath, "commit", "-m", message)
	}
	return runner.RunStreaming(ctx, worktreePath, func(output GitOutput) {
		chunk := redactExitHookChunk(string(output.Chunk))
		if chunk == "" {
			return
		}
		s.emitExit(ctx, EventExitHookOutput, operation, ExitEventPayload{
			OperationID: operation.ID,
			Action:      action,
			Phase:       ExitPhaseCommit,
			Stream:      string(output.Stream),
			Chunk:       chunk,
		})
	}, "commit", "-m", message)
}
