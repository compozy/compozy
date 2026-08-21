// Package corecmds provides the complete daemon-owned core command inventory.
package corecmds

import (
	"context"
	"fmt"
	"sort"

	"github.com/compozy/compozy/internal/cmdpalette"
)

// Provider is the daemon-owned core command inventory.
type Provider struct {
	commands []cmdpalette.Descriptor
}

// New builds the workspace-independent core command catalog.
func New() (*Provider, error) {
	commands := make([]cmdpalette.Descriptor, 0, 128)
	commands = append(commands, shellCommands()...)
	commands = append(commands, windowManagerCommands()...)
	commands = append(commands, appCommands()...)
	commands = append(commands, settingsCommands()...)
	commands = append(commands, viewCommands()...)
	commands = append(commands, profileCommands()...)
	sort.Slice(commands, func(left, right int) bool { return commands[left].ID < commands[right].ID })
	seen := make(map[cmdpalette.CommandID]struct{}, len(commands))
	for _, command := range commands {
		if _, exists := seen[command.ID]; exists {
			return nil, fmt.Errorf("core command %q is duplicated", command.ID)
		}
		seen[command.ID] = struct{}{}
		if err := cmdpalette.ValidateDescriptor(command); err != nil {
			return nil, fmt.Errorf("validate core command %q: %w", command.ID, err)
		}
	}
	return &Provider{commands: commands}, nil
}

// ProvideCommands returns a defensive copy of the core inventory and ignores workspace.
func (p *Provider) ProvideCommands(
	context.Context,
	cmdpalette.WorkspaceID,
) ([]cmdpalette.Descriptor, error) {
	return cloneCommands(p.commands), nil
}

// StaticCommands returns a defensive copy of the core inventory for boot validation.
func (p *Provider) StaticCommands() []cmdpalette.Descriptor { return cloneCommands(p.commands) }

func cloneCommands(source []cmdpalette.Descriptor) []cmdpalette.Descriptor {
	cloned := make([]cmdpalette.Descriptor, 0, len(source))
	for _, command := range source {
		cloned = append(cloned, cmdpalette.CloneDescriptor(command))
	}
	return cloned
}

func coreDescriptor(
	id cmdpalette.CommandID,
	title string,
	section string,
	icon string,
	action cmdpalette.Action,
) cmdpalette.Descriptor {
	return cmdpalette.Descriptor{
		ID: id, Title: title, Section: section, Icon: icon,
		Source:    cmdpalette.Source{Kind: cmdpalette.SourceKindCore},
		Action:    action,
		Arguments: []cmdpalette.Argument{},
		Policy: cmdpalette.ExecutionPolicy{
			SingleFlight: action.Kind == cmdpalette.ActionKindClientOp,
			RetrySafe:    action.Kind != cmdpalette.ActionKindClientOp,
		},
	}
}

func clientCommand(id cmdpalette.CommandID, title, section, icon string) cmdpalette.Descriptor {
	return coreDescriptor(id, title, section, icon, cmdpalette.Action{
		Kind: cmdpalette.ActionKindClientOp,
		Op:   string(id),
	})
}
