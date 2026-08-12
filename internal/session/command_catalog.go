package session

import (
	"context"
	"errors"
	"fmt"
	"strings"

	commandpkg "github.com/compozy/compozy/internal/command"
)

// CommandCatalog returns the authoritative slash command projection for a session.
func (m *Manager) CommandCatalog(ctx context.Context, id string) (commandpkg.Catalog, error) {
	if m == nil {
		return commandpkg.Catalog{}, errors.New("session: manager is required")
	}
	if ctx == nil {
		return commandpkg.Catalog{}, errors.New("session: command catalog context is required")
	}
	info, err := m.Status(ctx, id)
	if err != nil {
		return commandpkg.Catalog{}, err
	}
	agent, _ := m.SessionAgentDefinition(info.ID)
	if m.commandService != nil {
		catalog, err := m.commandService.Catalog(ctx, info, agent)
		if err != nil {
			return commandpkg.Catalog{}, fmt.Errorf("session: build command catalog for %q: %w", info.ID, err)
		}
		return applyWorktreeCommandAvailability(catalog, info, m.IsPrompting(info.ID)), nil
	}
	catalog, err := commandCatalogWithoutSkills(info)
	if err != nil {
		return commandpkg.Catalog{}, err
	}
	return applyWorktreeCommandAvailability(catalog, info, m.IsPrompting(info.ID)), nil
}

func applyWorktreeCommandAvailability(
	catalog commandpkg.Catalog,
	info *Info,
	prompting bool,
) commandpkg.Catalog {
	switch {
	case info == nil || info.State != StateActive:
		return commandpkg.SetBuiltinAvailability(
			catalog,
			"worktree",
			false,
			"Start or resume the session before forking it.",
		)
	case prompting:
		return commandpkg.SetBuiltinAvailability(
			catalog,
			"worktree",
			false,
			"Wait for the current turn to finish.",
		)
	default:
		return commandpkg.SetBuiltinAvailability(catalog, "worktree", true, "")
	}
}

func commandCatalogWithoutSkills(info *Info) (commandpkg.Catalog, error) {
	agents := make([]commandpkg.AgentSpec, 0)
	if info != nil {
		agents = make([]commandpkg.AgentSpec, 0, len(info.AdvertisedCommands))
		for _, advertised := range info.AdvertisedCommands {
			hint := ""
			if advertised.Input != nil {
				hint = strings.TrimSpace(advertised.Input.Hint)
			}
			agents = append(agents, commandpkg.AgentSpec{
				Name:        advertised.Name,
				Description: advertised.Description,
				InputHint:   hint,
				SourceID:    info.AgentName,
			})
		}
	}
	return commandpkg.BuildCatalog(commandpkg.DefaultBuiltins(), agents, nil)
}

func (m *Manager) expandPromptSkillInvocations(
	ctx context.Context,
	session *Session,
	message string,
) (string, error) {
	if m.commandService == nil || session == nil {
		return message, nil
	}
	invocations := session.CurrentSkillInvocations()
	if len(invocations) == 0 {
		return message, nil
	}
	info := session.Info()
	agent := session.AgentDefinition()
	expanded, err := m.commandService.Expand(ctx, info, agent, invocations, message)
	if err != nil {
		return "", fmt.Errorf("session: expand invoked skills: %w", err)
	}
	return expanded, nil
}
