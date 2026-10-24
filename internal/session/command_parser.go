package session

import (
	"context"
	"fmt"
	"strings"
	"unicode"

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
	if standaloneAgentCommand(req.authoredMessage, catalog) != "" {
		return nil
	}
	invocations, err := commandpkg.ParseSkillInvocations(req.authoredMessage, catalog)
	if err != nil {
		return fmt.Errorf("session: parse prompt commands: %w", err)
	}
	req.skillInvocations = invocations
	return nil
}

func standaloneAgentCommand(message string, catalog commandpkg.Catalog) string {
	text := strings.TrimSpace(message)
	if !strings.HasPrefix(text, "/") {
		return ""
	}
	token := text
	if end := strings.IndexFunc(text, unicode.IsSpace); end >= 0 {
		token = text[:end]
	}
	for _, descriptor := range catalog.Commands {
		if descriptor.Lane == commandpkg.LaneAgent && descriptor.Available && descriptor.CanonicalToken == token {
			return token
		}
	}
	return ""
}

func (m *Manager) prepareAgentCommandDelivery(ctx context.Context, req *promptRequest, message string) error {
	if !req.humanIngress || req.meta.Origin != nil || req.delivery != PromptDeliveryNormal {
		return nil
	}
	if !strings.HasPrefix(strings.TrimSpace(req.authoredMessage), "/") {
		return nil
	}
	catalog, err := m.CommandCatalog(ctx, req.target)
	if err != nil {
		return err
	}
	token := standaloneAgentCommand(req.authoredMessage, catalog)
	if token != "" && standaloneAgentCommand(message, catalog) == token {
		req.delivery = PromptDeliveryAgentCommand
		req.skillInvocations = nil
	}
	return nil
}
