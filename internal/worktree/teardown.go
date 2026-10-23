package worktree

import (
	"context"
	"strings"
	"sync"
)

const (
	EventTeardownOutput = "worktree.teardown_output"
	EventTeardownFailed = "worktree.teardown_failed"
)

func (s *Service) runTeardown(ctx context.Context, workspace Workspace, item Worktree) bool {
	settings, _ := s.worktreeSettings(workspace)
	command := strings.TrimSpace(settings.TeardownCommand)
	if command == "" {
		return false
	}
	operation := ExitOperation{ProfileID: item.ProfileID, WorkspaceID: item.WorkspaceID, WorktreeID: item.ID}
	var outputMu sync.Mutex
	remaining := map[GitOutputStream]int{GitOutputStdout: setupDiagnosticLimit, GitOutputStderr: setupDiagnosticLimit}
	state, detail := s.runWorktreeCommand(
		ctx,
		workspace,
		item,
		command,
		settings.TeardownTimeout,
		func(output GitOutput) {
			outputMu.Lock()
			defer outputMu.Unlock()
			if remaining[output.Stream] <= 0 {
				return
			}
			remaining[output.Stream] -= len(output.Chunk)
			s.emitExit(ctx, EventTeardownOutput, operation, ExitEventPayload{
				Action: ExitAction(exitRemoveAction), Stream: string(output.Stream), Chunk: string(output.Chunk),
			})
		},
	)
	if state == SetupFailed {
		s.logger.WarnContext(
			ctx,
			"worktree teardown failed; continuing removal",
			"worktree_id",
			item.ID,
			"error",
			detail,
		)
		s.emitExit(context.WithoutCancel(ctx), EventTeardownFailed, operation, ExitEventPayload{
			Action: ExitAction(exitRemoveAction), State: exitStepFailed, Message: detail,
		})
	}
	return true
}
