package session

import (
	"context"
	"fmt"

	commandpkg "github.com/compozy/compozy/internal/command"
)

// preparePromptSkillInvocations leaves Goal syntax to its control parser and resolves ordinary skill commands.
func (m *Manager) preparePromptSkillInvocations(ctx context.Context, req *promptRequest) error {
	if req == nil {
		return nil
	}
	_, goalMatched, err := ParseGoalCommand(req.authoredMessage)
	if goalMatched {
		return nil
	}
	if err != nil {
		return fmt.Errorf("session: parse goal command: %w", err)
	}
	catalog, err := m.CommandCatalog(ctx, req.target)
	if err != nil {
		return fmt.Errorf("session: resolve prompt commands: %w", err)
	}
	invocations, err := commandpkg.ParseSkillInvocations(req.authoredMessage, catalog)
	if err != nil {
		return fmt.Errorf("session: parse prompt commands: %w", err)
	}
	req.skillInvocations = invocations
	return nil
}
