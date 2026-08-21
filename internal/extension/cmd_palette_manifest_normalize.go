package extensionpkg

import (
	"maps"
	"strings"
)

func normalizeCmdPaletteConfig(config CmdPaletteConfig) CmdPaletteConfig {
	result := CmdPaletteConfig{
		Commands: make([]CmdPaletteCommand, 0, len(config.Commands)),
		Views:    make([]CmdPaletteView, 0, len(config.Views)),
	}
	for _, command := range config.Commands {
		result.Commands = append(result.Commands, normalizeCmdPaletteCommand(command))
	}
	for _, view := range config.Views {
		result.Views = append(result.Views, normalizeCmdPaletteView(view))
	}
	if len(result.Commands) == 0 {
		result.Commands = nil
	}
	if len(result.Views) == 0 {
		result.Views = nil
	}
	return result
}

func normalizeCmdPaletteCommand(command CmdPaletteCommand) CmdPaletteCommand {
	normalized := command
	normalized.ID = strings.TrimSpace(command.ID)
	normalized.Title = strings.TrimSpace(command.Title)
	normalized.Section = strings.TrimSpace(command.Section)
	normalized.Icon = strings.TrimSpace(command.Icon)
	normalized.Keywords = normalizeStrings(command.Keywords)
	normalized.DefaultShortcut = strings.TrimSpace(command.DefaultShortcut)
	normalized.Arguments = make([]CmdPaletteArgument, 0, len(command.Arguments))
	for _, argument := range command.Arguments {
		normalized.Arguments = append(normalized.Arguments, CmdPaletteArgument{
			Name: strings.TrimSpace(argument.Name), Type: strings.TrimSpace(argument.Type),
			Placeholder: strings.TrimSpace(argument.Placeholder), Required: argument.Required,
			Options: normalizeStrings(argument.Options),
		})
	}
	if len(normalized.Arguments) == 0 {
		normalized.Arguments = nil
	}
	normalized.Action.Kind = strings.TrimSpace(command.Action.Kind)
	normalized.Action.Tool = strings.TrimSpace(command.Action.Tool)
	normalized.Action.View = strings.TrimSpace(command.Action.View)
	normalized.Action.App = strings.TrimSpace(command.Action.App)
	normalized.Action.URL = strings.TrimSpace(command.Action.URL)
	if command.Action.Args != nil {
		normalized.Action.Args = make(map[string]any, len(command.Action.Args))
		maps.Copy(normalized.Action.Args, command.Action.Args)
	}
	if command.Confirmation != nil {
		normalized.Confirmation = &CmdPaletteConfirmation{
			Title:   strings.TrimSpace(command.Confirmation.Title),
			Body:    strings.TrimSpace(command.Confirmation.Body),
			Confirm: strings.TrimSpace(command.Confirmation.Confirm),
		}
	}
	if command.Execution != nil {
		normalized.Execution = &CmdPaletteExecutionPolicy{
			SingleFlight: cloneBool(command.Execution.SingleFlight),
			RetrySafe:    cloneBool(command.Execution.RetrySafe),
		}
	}
	return normalized
}

func normalizeCmdPaletteView(view CmdPaletteView) CmdPaletteView {
	normalized := view
	normalized.ID = strings.TrimSpace(view.ID)
	normalized.Title = strings.TrimSpace(view.Title)
	normalized.Kind = strings.TrimSpace(view.Kind)
	if view.Source != nil {
		normalized.Source = &CmdPaletteViewSource{Tool: strings.TrimSpace(view.Source.Tool)}
	}
	return normalized
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
