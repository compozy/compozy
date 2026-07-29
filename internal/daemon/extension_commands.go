package daemon

import (
	"context"
	"errors"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
)

type extensionCommandRuntime interface {
	Commands(string) ([]extensionpkg.CommandDescriptor, []extensionpkg.CommandGroupDescriptor, error)
}

func (s *daemonExtensionService) Commands(
	ctx context.Context,
	extensionFilter string,
) (contract.ExtensionCommandsResponse, error) {
	if err := ctx.Err(); err != nil {
		return contract.ExtensionCommandsResponse{}, err
	}
	if err := s.checkReady(); err != nil {
		return contract.ExtensionCommandsResponse{}, err
	}
	return s.extensionCommands("", extensionFilter)
}

func (s *daemonExtensionService) CommandsScoped(
	ctx context.Context,
	extensionFilter string,
	actor taskpkg.ActorContext,
) (contract.ExtensionCommandsResponse, error) {
	if err := ctx.Err(); err != nil {
		return contract.ExtensionCommandsResponse{}, err
	}
	if err := actor.Validate(); err != nil {
		return contract.ExtensionCommandsResponse{}, err
	}
	if !actor.Authority.Read {
		return contract.ExtensionCommandsResponse{}, taskpkg.ErrPermissionDenied
	}
	if err := s.checkReady(); err != nil {
		return contract.ExtensionCommandsResponse{}, err
	}
	return s.extensionCommands(strings.TrimSpace(actor.Scope.WorkspaceID), extensionFilter)
}

func (s *daemonExtensionService) extensionCommands(
	workspaceID string,
	extensionFilter string,
) (contract.ExtensionCommandsResponse, error) {
	runtime, ok := s.runtime.(extensionCommandRuntime)
	if !ok || runtime == nil {
		return contract.ExtensionCommandsResponse{}, errors.New("daemon: extension command runtime is unavailable")
	}
	commands, groups, err := runtime.Commands(workspaceID)
	if err != nil {
		return contract.ExtensionCommandsResponse{}, err
	}
	filter := strings.TrimSpace(extensionFilter)
	response := contract.ExtensionCommandsResponse{
		Commands: make([]contract.ExtensionCommandPayload, 0, len(commands)),
		Groups:   make([]contract.ExtensionCommandGroupPayload, 0, len(groups)),
	}
	for _, command := range commands {
		if filter != "" && command.Extension != filter {
			continue
		}
		response.Commands = append(response.Commands, contract.ExtensionCommandPayload{
			Extension: command.Extension, Verb: command.Verb, ToolID: command.ToolID,
			Summary: command.Summary, Example: command.Example, Flags: command.Flags,
			RiskClass: command.RiskClass, ApprovalRequired: command.ApprovalRequired,
		})
	}
	for _, group := range groups {
		if filter != "" && group.Extension != filter {
			continue
		}
		response.Groups = append(response.Groups, contract.ExtensionCommandGroupPayload{
			Extension: group.Extension, Path: group.Path, Summary: group.Summary,
		})
	}
	return response, nil
}
