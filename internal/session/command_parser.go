package session

import (
	"context"
	"fmt"

	commandpkg "github.com/compozy/compozy/internal/command"
)

func (m *Manager) preparePromptSkillInvocations(ctx context.Context, req *promptRequest) error {
	if req == nil {
		return nil
	}
	_, goalMatched, err := ParseGoalCommand(req.authoredMessage)
	if err != nil {
		return fmt.Errorf("session: parse goal command: %w", err)
	}
	if goalMatched {
		return nil
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
