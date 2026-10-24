package worktree

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/redact"
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
	operation := ExitOperation{
		RunID:       item.RunID,
		ProfileID:   item.ProfileID,
		WorkspaceID: item.WorkspaceID,
		WorktreeID:  item.ID,
	}
	state, detail := s.runWorktreeCommand(
		ctx,
		workspace,
		item,
		command,
		settings.TeardownTimeout,
		func(output GitOutput) {
			s.emitTeardownOutput(ctx, operation, output)
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

func (s *Service) prepareFencedRemoval(
	ctx context.Context, workspace Workspace, item Worktree, commonDir string, force bool,
) (bool, RemovalRisk, *RemovalRefusal, error) {
	terminal, risk, refusalResult, err := s.evaluateFencedRemoval(ctx, workspace, item, commonDir, force)
	if terminal || err != nil || refusalResult != nil {
		return terminal, risk, refusalResult, err
	}
	if s.runTeardown(ctx, workspace, item) {
		return s.evaluateFencedRemoval(ctx, workspace, item, commonDir, force)
	}
	return false, risk, nil, nil
}

func (s *Service) emitTeardownOutput(ctx context.Context, operation ExitOperation, output GitOutput) {
	// Redact before splitting so secrets spanning event boundaries remain protected.
	remaining := redact.String(strings.ToValidUTF8(string(output.Chunk), "�"))
	for remaining != "" {
		end := min(len(remaining), exitHookOutputLimit)
		if end < len(remaining) {
			for !utf8.RuneStart(remaining[end]) {
				end--
			}
		}
		s.emitExit(ctx, EventTeardownOutput, operation, ExitEventPayload{
			Action: ExitAction(exitRemoveAction), Stream: string(output.Stream), Chunk: remaining[:end],
		})
		remaining = remaining[end:]
	}
}
