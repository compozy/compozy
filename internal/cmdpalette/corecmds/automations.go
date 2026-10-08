package corecmds

import "github.com/compozy/compozy/internal/cmdpalette"

const coreAppAutomations = "automations"

type automationCreateDefinition struct {
	start    string
	title    string
	icon     string
	keywords []string
}

// automationCreateDefinitions open the Automations editor with Starts
// preselected; the web route reads `create=1&start=…` as the editor deep link.
var automationCreateDefinitions = []automationCreateDefinition{
	{
		start:    "schedule",
		title:    "New scheduled automation",
		icon:     coreIconPlus,
		keywords: []string{"job", "schedule", "cron", "every"},
	},
	{
		start:    "event",
		title:    "New automation on an event",
		icon:     coreIconPlus,
		keywords: []string{"trigger", "event", "when", "webhook"},
	},
}

func automationCommands() []cmdpalette.Descriptor {
	commands := make([]cmdpalette.Descriptor, 0, len(automationCreateDefinitions))
	for _, definition := range automationCreateDefinitions {
		command := coreDescriptor(
			cmdpalette.CommandID("automation.create."+definition.start),
			definition.title,
			coreSectionApps,
			definition.icon,
			cmdpalette.Action{
				Kind: cmdpalette.ActionKindNavigate,
				App:  coreAppAutomations,
				Args: map[string]any{
					"pathname": "/automations",
					"create":   "1",
					"start":    definition.start,
				},
			},
		)
		command.Keywords = append([]string(nil), definition.keywords...)
		commands = append(commands, command)
	}
	return commands
}
